# Home Assistant App: HA Mac Mini Power On Restart
#
# Build:
#   docker buildx build --platform linux/amd64,linux/arm64 -t <image>:<tag> .
#
# This addon uses a static Go binary built from scratch — no Alpine,
# no bash, no pciutils. The binary talks directly to sysfs PCI.
#
# Multi-stage:
#   go-builder   — Go compilation (scratch, fully static binary)
#   production    — HA base image with the binary + s6-overlay services

ARG BUILD_FROM=ghcr.io/home-assistant/base:3.23
FROM ${BUILD_FROM} AS builder

# BuildKit sets TARGETARCH automatically from --platform.
ARG TARGETARCH

# Copy Go source into the build stage.
COPY rootfs/usr/bin/go.mod /build/go.mod
COPY rootfs/usr/bin/main.go /build/main.go

# Compile: fully static, stripped, trimpath for reproducible builds.
FROM scratch AS go-builder
ARG TARGETARCH
COPY --from=builder /usr/local/go /usr/local/go
WORKDIR /build
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH} \
    GOPATH=/go \
    GOCACHE=/tmp/go-cache
RUN go mod download && \
    go build -trimpath -ldflags '-s -w -extldflags "-static"' \
      -o /usr/bin/poweron main.go

# === Production stage ===
FROM ${BUILD_FROM}

# Copy the Go binary and s6-overlay service scripts.
COPY --from=go-builder /usr/bin/poweron  /usr/bin/poweron
COPY rootfs/ /

# Ensure executable.
RUN chmod +x /usr/bin/poweron

LABEL \
    org.opencontainers.image.title="Home Assistant App: HA Mac Mini Power On Restart" \
    org.opencontainers.image.description="Allow your MacMini-based HAOS to restart on return of power" \
    org.opencontainers.image.source="https://github.com/chickenandpork/ha-macmini-poweron-restart" \
    org.opencontainers.image.licenses="Apache License 2.0"
