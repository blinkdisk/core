# BlinkDisk Core

**This is the core library used by the [BlinkDisk desktop app](https://github.com/blinkdisk/blinkdisk).**

## Fork of Kopia

This repository is a fork of the incredible [Kopia](https://github.com/kopia/kopia) project, which handles the heavy lifting behind BlinkDisk — including snapshot management, deduplication, encryption, compression, and efficient backup operations.

>  **A huge thank you to the Kopia team** — this project would not be possible without their amazing work. Kopia is one of the most thoughtfully designed backup engines out there, and we're proud to build on top of it.

## Differences from Kopia

BlinkDisk Core is kept very close to upstream [Kopia](https://github.com/kopia/kopia) in order to benefit from upstream improvements and bug fixes. Our fork mainly introduces:

- **Removal of unused features** like Kopija’s built-in UI, server functionality, and other CLI tools not used by the BlinkDisk desktop app.
- **Rebranding** from Kopia to BlinkDisk for internal consistency and integration into the desktop app.
- **Feature additions** such as support for the BlinkCloud Proxy, which enables seamless backups through our managed infrastructure.
