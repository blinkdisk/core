# BlinkDisk Core

**This is the core library used by the [BlinkDisk](https://github.com/blinkdisk/blinkdisk) desktop app.**

## Fork of BlinkDisk

This repository is a fork of the incredible [BlinkDisk](https://github.com/blinkdisk/core) project, which handles the heavy lifting behind BlinkDisk — including snapshot management, deduplication, encryption, compression, and efficient backup operations.

>  **A huge thank you to the BlinkDisk team** — this project would not be possible without their amazing work. BlinkDisk is one of the most thoughtfully designed backup engines out there, and we're proud to build on top of it.

## Differences from BlinkDisk

BlinkDisk Core is kept very close to upstream [BlinkDisk](https://github.com/blinkdisk/core) in order to benefit from upstream improvements and bug fixes. Our fork mainly introduces:

- **Removal of unused features** like BlinkDisk’s built-in UI, server functionality, and other CLI tools not used by the BlinkDisk desktop app.
- **Rebranding** from BlinkDisk to BlinkDisk for internal consistency and integration into the desktop app.
- **Feature additions** such as support for the BlinkDisk Cloud Proxy, which enables seamless backups through our managed infrastructure.
