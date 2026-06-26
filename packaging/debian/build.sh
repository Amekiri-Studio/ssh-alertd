#!/usr/bin/env bash
# Build a .deb for ssh-alertd using dpkg-deb staging (no debhelper required).
# Usage: ./build.sh [version]   (default version: 0.1.0)
set -euo pipefail

VERSION="${1:-0.1.1}"
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"

command -v dpkg-deb >/dev/null || { echo "error: dpkg-deb not found (install dpkg)" >&2; exit 1; }
command -v go >/dev/null || { echo "error: go toolchain not found" >&2; exit 1; }

ARCH="$(dpkg --print-architecture)"
STAGE="$HERE/build/ssh-alertd_${VERSION}_${ARCH}"

echo ":: building ssh-alertd $VERSION ($ARCH)"
rm -rf "$STAGE"
mkdir -p "$STAGE/DEBIAN" \
	"$STAGE/usr/bin" \
	"$STAGE/lib/systemd/system" \
	"$STAGE/usr/lib/sysusers.d" \
	"$STAGE/usr/lib/tmpfiles.d" \
	"$STAGE/etc/ssh-alertd" \
	"$STAGE/usr/share/doc/ssh-alertd"

# Statically-linked Go binary so the package is portable across Debian releases
# (no glibc version coupling); hence no libc6 dependency in control.
( cd "$ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" \
	-o "$STAGE/usr/bin/ssh-alertd" . )

# systemd unit: the shipped unit targets /usr/local/bin for manual installs;
# the packaged binary lives in /usr/bin, so rewrite ExecStart.
sed 's|/usr/local/bin/ssh-alertd|/usr/bin/ssh-alertd|' \
	"$ROOT/deploy/ssh-alertd.service" > "$STAGE/lib/systemd/system/ssh-alertd.service"

# Dedicated system user + config ownership (applied by postinst via
# systemd-sysusers / systemd-tmpfiles).
install -m644 "$ROOT/deploy/ssh-alertd.sysusers" "$STAGE/usr/lib/sysusers.d/ssh-alertd.conf"
install -m644 "$ROOT/deploy/ssh-alertd.tmpfiles" "$STAGE/usr/lib/tmpfiles.d/ssh-alertd.conf"

# Default config (registered as a conffile, mode 0640 to protect the token).
install -m640 "$ROOT/config.example.json" "$STAGE/etc/ssh-alertd/config.json"

# Documentation (Debian policy: README, copyright, gzipped changelog).
install -m644 "$ROOT/README.md"   "$STAGE/usr/share/doc/ssh-alertd/README.md"
install -m644 "$HERE/copyright"   "$STAGE/usr/share/doc/ssh-alertd/copyright"
gzip -9 -n -c "$HERE/changelog" > "$STAGE/usr/share/doc/ssh-alertd/changelog.gz"

# Control metadata + maintainer scripts.
sed -e "s/@VERSION@/$VERSION/" -e "s/@ARCH@/$ARCH/" "$HERE/control" > "$STAGE/DEBIAN/control"
install -m644 "$HERE/conffiles" "$STAGE/DEBIAN/conffiles"
install -m755 "$HERE/postinst" "$STAGE/DEBIAN/postinst"
install -m755 "$HERE/prerm"    "$STAGE/DEBIAN/prerm"
install -m755 "$HERE/postrm"   "$STAGE/DEBIAN/postrm"

OUT="$HERE/ssh-alertd_${VERSION}_${ARCH}.deb"
dpkg-deb --root-owner-group --build "$STAGE" "$OUT"
echo ":: built $OUT"
