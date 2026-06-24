# Home Assistant App: HA MacMini PowerOn Restart

Allow your MacMini-based HAOS to restart on return of power

On successful startup (or start of startup) in HAOS under Home-Assistant, this App will set the SMC
in Apple devices (ie Mac Minis) to reboot the next time power is removed and restored.  This allows
HAOS on a Mac Mini to survive a power-cycle due to  UPS going critically offline and shutting
things down, then returning power when the mains power returns.

![Supports aarch64 Architecture][aarch64-shield]
![Supports amd64 Architecture][amd64-shield]

[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
