#!/usr/bin/env bash
set -euo pipefail

# Installs the Rad CLI for CI use (Linux amd64). kan dogfoods Rad for CI-only
# scripting - see ci/*.rad.
#
# Pinned to an exact version AND its tarball SHA256. GitHub release tags are mutable,
# so the version tag alone doesn't guarantee integrity - the checksum does. This
# matters because the comment workflow that runs Rad holds `pull-requests: write`.
# When bumping RAD_VERSION, recompute RAD_SHA256:
#   shasum -a 256 rad_linux_amd64.tar.gz   (or sha256sum on Linux)

RAD_VERSION="v0.10.1"
RAD_SHA256="c87adc2e8e70a6b6374fbef35e27a720ed97699c4d5e9116a40757d1712a4efd"
INSTALL_DIR="/usr/local/bin"
PLATFORM="linux_amd64"
URL="https://github.com/amterp/rad/releases/download/${RAD_VERSION}/rad_${PLATFORM}.tar.gz"

echo "Installing Rad ${RAD_VERSION} from ${URL}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$URL" -o "${tmp}/rad.tar.gz"
echo "${RAD_SHA256}  ${tmp}/rad.tar.gz" | sha256sum -c -
tar -xz -C "${tmp}" -f "${tmp}/rad.tar.gz"
sudo mv "${tmp}/rad" "${INSTALL_DIR}/rad"
sudo chmod +x "${INSTALL_DIR}/rad"

echo "Rad installed: $(rad -v)"
