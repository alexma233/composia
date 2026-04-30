# AUR Packaging

This directory contains the source-build `PKGBUILD` used for publishing the `composia` AUR package. Official releases also publish the `composia-bin` AUR package from release binaries through GoReleaser.

Release checklist:

1. Update `pkgver` to the release version without the `v` prefix.
2. Replace `sha256sums=('SKIP')` with the checksum from `makepkg -g`.
3. Build locally with `makepkg -sf`.
4. Publish the resulting `PKGBUILD` and `.SRCINFO` to AUR.

The package installs:

- `/usr/bin/composia`
- `/usr/bin/composia-controller`
- `/usr/bin/composia-agent`
