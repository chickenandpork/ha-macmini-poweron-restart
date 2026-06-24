# Home Assistant App: HA MacMini PowerOn Restart

## How to use

This app is non-interactive: it does what's needed.

Simply install it, and when the system boots, it'll see whether you're running on a Mac with the
power adapter controllers it expects.  If it finds one it understands, it'll configure the power
adapter controller to power on when the mains power goes low-to-high or similar transitions
depending on the specific power controller.  The enxt time power is retored, the power adapter will
signal the SMC, the SMC will then cause the system to boot.  Simples!
