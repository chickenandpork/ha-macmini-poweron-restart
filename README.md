# Home-Assistant MacMini Power On (restore) Restart

There's a small number of people running Home Assistant as HAOS on a Mac Mini.  I'm certain it
cannot be very large.

In that group, some of us have frequent power blips, during which HAOS stops, the power is
immediately on, but the UP protecting HAOS still runs down and turns off.  On return of mains
power, Home Assistant doesn't restart, and it's frustratingC, but it's not Home Assistant's fault.

Apple makes desktop devices (Oh, XServe, we miss you).  These devices always have a person at them
when in-use, or so the design would suggest, so if the system might not reboot cleanly, just wait
for the user.  The way the SMC works in Mac is that when the system boots to a stable OS, the OS
tells the SMC "yeah, OK, you can reboot this one on powerfail and return".  The counter example to
this is when a desktop system is being tested or used, and it gets some bad boot config, and
starts, fails, starts, fails, starts, ... so in this case -- likely expected ore often than an
Apple running without a user -- in this case, the system would never get to that steady-state, so
would not keep rebooting. The user can investigate the error when they return.

We want it to start automatically, so we set the "safe to reboot" setting as soon as Home Assistant starts.


<!--
  Apps documentation: <https://developers.home-assistant.io/docs/apps>
 -->

[![Open your Home Assistant instance and show the app store with a specific repository URL pre-filled.](https://my.home-assistant.io/badges/supervisor_store.svg)](https://my.home-assistant.io/redirect/supervisor_store/?repository_url=https%3A%2F%2Fgithub.com%2Fchickenandpork%2Fha-macmini-poweron-restart)

## Apps

This repository contains the following app(s)

### [HA MacMini PowerOn Restart](./ha-macmini-poweron)

![Supports aarch64 Architecture][aarch64-shield]
![Supports amd64 Architecture][amd64-shield]


<!--

Notes to developers after forking or using the github template feature:
- While developing comment out the 'image' key from 'ha-macmini-poweron/config.yaml' to make the supervisor build the app locally.
  - Remember to put this back when pushing up your changes.
- When you merge to the 'main' branch of your repository a new build will be triggered.
  - The first time this runs you might need to adjust the image configuration on github container registry to make it public.
- Share your repository on the forums https://community.home-assistant.io/c/projects/9
 -->

[aarch64-shield]: https://img.shields.io/badge/aarch64-yes-green.svg
[amd64-shield]: https://img.shields.io/badge/amd64-yes-green.svg
