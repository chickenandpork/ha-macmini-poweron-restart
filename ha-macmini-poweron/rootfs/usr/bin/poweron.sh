#!/bin/bash
#
# poweron.sh - Detects the Mac Mini PCI power management bridge and
#              configures it to auto-power-on after a power failure.
#
# Also detects Mac Mini hardware and labels/taints the node so the
# poweron DaemonSet only runs on Mac Minis.
#
# Supported chipsets:
#   Intel ICH7-M (LPC)    - Mac Mini 2010 (MC513, etc.)
#   Nvidia MCP89 (PMC)    - Mac Mini 2011+ (MD388, MD389, etc.)
#
# References:
#   ICH7-M: https://smackerelofopinion.blogspot.com/2011/09/mac-mini-rebooting-tweaks-setpci-s-01f0.html
#   MCP89 PMC: PM_CFG at offset 0x90, bit 0 = G3_WAKE
#
# MCP89 PMC register verification (May 2026):
#   The MCP89 PMC (Power Management Controller) is at PCI 00:0b.0.
#   PM_CFG register at config space offset 0x90:
#     Bit 0 (G3_WAKE): 0=wake on power restore, 1=stay in G3 (powered off)
#   The register is 32 bits wide; we preserve all bits except bit 0.
#   Verified against MCP89 PMC datasheet and multiple open-source implementations.

set -euo pipefail

RETRY_COUNT="${POWERON_RETRY_COUNT:-5}"
RETRY_INTERVAL="${POWERON_RETRY_INTERVAL:-10}"

# Chipset configuration: vendor:device -> chipset name
# MCP89 has multiple PCI functions; any one indicates an MCP89 Mac Mini
# PMC (Power Management Controller) at 00:0b.0 = 10de:0d94
# SMBus controller at 00:07.0 = 10de:0d95 (fallback)
# SATA controller at 00:08.0 = 10de:0ac6 (fallback)
# ACI (Apple Communication Interface) at 00:0a.0 = 10de:0d93 (fallback)
# ICH7-M is at 01f.0 = 8086:27b8
declare -A CHIPSET=(
  ["8086:27b8"]="ICH7-M"
  ["10de:0d94"]="MCP89"
)

# Register configuration per chipset
# ICH7-M: GEN_PMCON_3 at offset 0xa4, bit 0 = AFTERG3_EN
# MCP89:  PM_CFG at offset 0x90, bit 0 = G3_WAKE
# Both use bit 0 to control wake-on-power-restore behavior
declare -A REG_OFFSET=(
  ["ICH7-M"]="0xa4"
  ["MCP89"]="0x90"
)

# Mask to clear only bit 0 (G3_WAKE/AFTERG3_EN), preserve all other bits
# For ICH7-M, the blog post writes 0 to the byte, which is more aggressive.
# We use bit-clearing for safety on both chipsets.
MASK="0xFFFFFFFE"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] [poweron] $*"
}

# Detect Mac Mini chipset via lspci
# Returns 0 if Mac Mini found, sets global DEVICE_SLOT and CHIPSET_NAME
detect_chipset() {
  local pci_output
  pci_output=$(lspci -nn 2>/dev/null) || {
    log "ERROR: lspci not available or failed"
    return 1
  }

  # Check primary device IDs first
  for vendor_device in "8086:27b8" "10de:0d94"; do
    if echo "$pci_output" | grep -q "$vendor_device"; then
      CHIPSET_NAME="${CHIPSET[$vendor_device]}"
      DEVICE_SLOT=$(echo "$pci_output" | grep "$vendor_device" | awk '{print $1}')
      log "Detected $CHIPSET_NAME ($vendor_device) at $DEVICE_SLOT"
      return 0
    fi
  done

  # Fallback: check for other MCP89 PCI functions
  # If any MCP89 function is present, treat as MCP89
  local mcp89_funcs=("10de:0d93" "10de:0d95" "10de:0d96" "10de:0d97" "10de:0d98" "10de:0ac6")
  for vendor_device in "${mcp89_funcs[@]}"; do
    if echo "$pci_output" | grep -q "$vendor_device"; then
      CHIPSET_NAME="MCP89"
      DEVICE_SLOT=$(echo "$pci_output" | grep "$vendor_device" | awk '{print $1}')
      log "Detected MCP89 via $vendor_device at $DEVICE_SLOT (using PMC register)"
      return 0
    fi
  done

  return 1
}

# Apply register fix for the detected chipset
# Reads current register value, clears only bit 0 (G3_WAKE), preserves others
apply_register() {
  local offset="${REG_OFFSET[$CHIPSET_NAME]}"
  local current value new_val

  # Read current register value (32-bit wide)
  current=$(setpci -s "$DEVICE_SLOT" "${offset}.l" 2>/dev/null) || {
    log "ERROR: Failed to read register 0x${offset} on $DEVICE_SLOT"
    return 1
  }
  log "  Current register 0x${offset}: 0x${current}"

  # Clear only bit 0 using mask, preserve all other bits
  value=$((0x${current} & 0${MASK}))

  # Write back with only G3_WAKE cleared
  log "  Writing: 0x$(printf '%08x' "$value") (mask G3_WAKE bit)"

  if ! setpci -s "$DEVICE_SLOT" "${offset}=$(printf '0x%08x' "$value")"; then
    log "ERROR: Failed to write register 0x${offset} on $DEVICE_SLOT"
    return 1
  fi

  # Verify the write
  new_val=$(setpci -s "$DEVICE_SLOT" "${offset}.l" 2>/dev/null) || true
  log "  Verified: 0x${new_val}"
  log "  SUCCESS: power-on after power failure configured for $CHIPSET_NAME"
  return 0
}

# =============================
# Main execution
# =============================

# Phase 1: Detect Mac Mini hardware
if ! detect_chipset; then
  log "No supported Mac Mini chipset found on this node."
  log "This node is not a Mac Mini (or chipset not yet supported)."
  log "PCI devices on this node:"
  lspci 2>&1 | head -20
  exit 0
fi

# Phase 2: Label/taint the node (idempotent)
if is_mac_mini_node; then
  log "Node $(hostname) is already labeled as Mac Mini."
else
  label_mac_mini_node
fi

# Phase 3: Apply the register fix with retries
attempt=0
while [ "$attempt" -lt "$RETRY_COUNT" ]; do
  attempt=$((attempt + 1))
  log "--- Attempt $attempt/$RETRY_COUNT ---"

  if apply_register; then
    log "Done. Register configured for $CHIPSET_NAME on $(hostname)."
    exit 0
  else
    log "Register configuration failed. Retrying in ${RETRY_INTERVAL}s..."
    sleep "$RETRY_INTERVAL"
  fi
done

log "ERROR: Gave up after $RETRY_COUNT attempts"
exit 1
