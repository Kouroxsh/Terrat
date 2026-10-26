#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-${VERSION:-3.0.0}}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"

mkdir -p "${DIST_DIR}"

if command -v nfpm >/dev/null 2>&1; then
    NFPM="nfpm"
elif [ -x "${HOME}/go/bin/nfpm" ]; then
    NFPM="${HOME}/go/bin/nfpm"
else
    echo "Installing nfpm for packaging..."
    go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
    NFPM="${HOME}/go/bin/nfpm"
fi

echo "Building Linux distribution packages for v${VERSION}..."

for ARCH in amd64 arm64; do
    BIN_SRC="${DIST_DIR}/terrat-linux-${ARCH}"
    echo "Compiling Linux binary for ${ARCH}..."
    GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -ldflags="-s -w -X main.Version=${VERSION}" -o "${BIN_SRC}" "${ROOT_DIR}/cmd/terrat"

    NFPM_CONFIG=$(mktemp --suffix=.yaml)
    cat << EOF > "${NFPM_CONFIG}"
name: "terrat"
arch: "${ARCH}"
platform: "linux"
version: "${VERSION}"
section: "utils"
priority: "optional"
maintainer: "Kouroxsh <nightlandcompany@proton.me>"
description: "Linux terminal emulator written in Go"
homepage: "https://github.com/Kouroxsh/Terrat"
license: "MIT"
contents:
  - src: ${BIN_SRC}
    dst: /usr/bin/terrat
    file_info:
      mode: 0755
  - src: ${ROOT_DIR}/packaging/terrat.desktop
    dst: /usr/share/applications/terrat.desktop
    file_info:
      mode: 0644
  - src: ${ROOT_DIR}/packaging/terrat.desktop
    dst: /usr/share/applications/terraterminal.desktop
    file_info:
      mode: 0644
  - src: ${ROOT_DIR}/assets/icon.png
    dst: /usr/share/icons/hicolor/512x512/apps/terrat.png
    file_info:
      mode: 0644
  - src: ${ROOT_DIR}/assets/icon.png
    dst: /usr/share/icons/hicolor/512x512/apps/terraterminal.png
    file_info:
      mode: 0644
  - src: ${ROOT_DIR}/LICENSE
    dst: /usr/share/licenses/terrat/LICENSE
    file_info:
      mode: 0644
scripts:
  postinstall: ${ROOT_DIR}/packaging/postinstall.sh
  postremove: ${ROOT_DIR}/packaging/postremove.sh
EOF

    echo "Packaging .deb for ${ARCH}..."
    "${NFPM}" package -f "${NFPM_CONFIG}" --packager deb --target "${DIST_DIR}/"

    echo "Packaging .rpm for ${ARCH}..."
    "${NFPM}" package -f "${NFPM_CONFIG}" --packager rpm --target "${DIST_DIR}/"

    echo "Packaging Arch Linux (.pkg.tar.zst) for ${ARCH}..."
    "${NFPM}" package -f "${NFPM_CONFIG}" --packager archlinux --target "${DIST_DIR}/"

    rm -f "${NFPM_CONFIG}"

    echo "Packaging Linux portable tarball for ${ARCH}..."
    PORTABLE_DIR=$(mktemp -d)
    TARGET_SUBDIR="${PORTABLE_DIR}/terrat-v${VERSION}-linux-${ARCH}"
    mkdir -p "${TARGET_SUBDIR}"
    cp "${BIN_SRC}" "${TARGET_SUBDIR}/terrat"
    chmod 755 "${TARGET_SUBDIR}/terrat"
    cp "${ROOT_DIR}/packaging/terrat.desktop" "${TARGET_SUBDIR}/"
    cp "${ROOT_DIR}/assets/icon.png" "${TARGET_SUBDIR}/"
    cp "${ROOT_DIR}/packaging/portable-install.sh" "${TARGET_SUBDIR}/install.sh"
    chmod +x "${TARGET_SUBDIR}/install.sh"
    cp "${ROOT_DIR}/LICENSE" "${TARGET_SUBDIR}/"

    tar -czf "${DIST_DIR}/terrat-v${VERSION}-linux-${ARCH}.tar.gz" -C "${PORTABLE_DIR}" "terrat-v${VERSION}-linux-${ARCH}"
    rm -rf "${PORTABLE_DIR}"
done

echo "Generating SHA-256 checksums..."
(
    cd "${DIST_DIR}"
    rm -f checksums.txt
    sha256sum *.deb *.rpm *.pkg.tar.zst *.tar.gz > checksums.txt
)

echo "Release assets successfully built in ${DIST_DIR}:"
ls -lh "${DIST_DIR}"
