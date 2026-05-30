# AUR Packaging

This directory contains the source-build `PKGBUILD` template for the `composia` AUR package. Official releases publish both `composia` from sources and `composia-bin` from release binaries through GoReleaser.

Release checklist:

1. Update `pkgver` to the release version without the `v` prefix.
2. Replace `sha256sums=('SKIP')` with the checksum from `makepkg -g`.
3. Build locally with `makepkg -sf`.
4. Publish the resulting `PKGBUILD` and `.SRCINFO` to AUR if GoReleaser publishing is not being used.

The package installs:

- `/usr/bin/composia`
- `/usr/bin/composia-controller`
- `/usr/bin/composia-agent`
- `/usr/lib/systemd/system/composia-controller.service`
- `/usr/lib/systemd/system/composia-agent.service`

The systemd units are installed only. They are not enabled or started automatically.
