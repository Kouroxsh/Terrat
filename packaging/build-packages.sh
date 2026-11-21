#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"

# L56: the VERSION file is the single source of truth; the argument and
# the VERSION env var still win when given explicitly.
VERSION="${1:-${VERSION:-$(cat "${ROOT_DIR}/VERSION" 2>/dev/null || echo 3.0.0)}}"
# M34: accept an optional leading `v` (git tags are v3.0.0) and reject
# anything that is not a sane package version before building anything.
VERSION="${VERSION#v}"
if [[ ! "${VERSION}" =~ ^[0-9][0-9A-Za-z.+~]*$ ]]; then
    echo "error: invalid version '${VERSION}' (expected e.g. 3.0.0)" >&2
    exit 1
fi

# L61: temp files/dirs from failed runs must not leak.
NFPM_CONFIG=""
PORTABLE_DIR=""
cleanup() {
    [ -n "${NFPM_CONFIG}" ] && rm -f "${NFPM_CONFIG}"
    [ -n "${PORTABLE_DIR}" ] && rm -rf "${PORTABLE_DIR}"
}
trap cleanup EXIT

# M35: start from a clean dist/ so a failed run never leaves stale
# artifacts (notably checksums.txt) behind to be mistaken for fresh ones.
rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

# H14: resolve nfpm the way `go install` lays it out (GOBIN, else
# GOPATH/bin) instead of assuming ${HOME}/go/bin.
# L51: pin nfpm instead of tracking @latest.
NFPM_VERSION="${NFPM_VERSION:-v2.49.0}"
GO_BIN_DIR="$(go env GOBIN 2>/dev/null || true)"
if [ -z "${GO_BIN_DIR}" ]; then
    GO_BIN_DIR="$(go env GOPATH 2>/dev/null || echo "${HOME}/go")/bin"
fi
resolve_nfpm() {
    if command -v nfpm >/dev/null 2>&1; then
        NFPM="nfpm"
        return 0
    fi
    if [ -x "${GO_BIN_DIR}/nfpm" ]; then
        NFPM="${GO_BIN_DIR}/nfpm"
        return 0
    fi
    return 1
}
if ! resolve_nfpm; then
    echo "Installing nfpm ${NFPM_VERSION} for packaging..."
    go install "github.com/goreleaser/nfpm/v2/cmd/nfpm@${NFPM_VERSION}"
    if ! resolve_nfpm; then
        echo "error: nfpm still not found after install (looked in PATH and ${GO_BIN_DIR})" >&2
        exit 1
    fi
fi

echo "Building Linux distribution packages for v${VERSION}..."

for ARCH in amd64 arm64; do
    BIN_SRC="${DIST_DIR}/terrat-linux-${ARCH}"
    echo "Compiling Linux binary for ${ARCH}..."
    CGO_ENABLED=0 GOOS=linux GOARCH="${ARCH}" go build -ldflags="-s -w -X main.Version=${VERSION}" -o "${BIN_SRC}" "${ROOT_DIR}/cmd/terrat"

    for PACKAGER in deb rpm archlinux; do
        case "${PACKAGER}" in
            deb)
                # M36: Debian expects the copyright file here, not under
                # /usr/share/licenses (which is the rpm/Arch location).
                LICENSE_DST="/usr/share/doc/terrat/copyright"
                LABEL=".deb"
                ;;
            rpm)
                LICENSE_DST="/usr/share/licenses/terrat/LICENSE"
                LABEL=".rpm"
                ;;
            archlinux)
                LICENSE_DST="/usr/share/licenses/terrat/LICENSE"
                LABEL="Arch Linux (.pkg.tar.zst)"
                ;;
        esac

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
  - src: ${ROOT_DIR}/assets/icon.png
    dst: /usr/share/icons/hicolor/1024x1024/apps/terrat.png
    file_info:
      mode: 0644
  - src: ${ROOT_DIR}/LICENSE
    dst: ${LICENSE_DST}
    file_info:
      mode: 0644
scripts:
  postinstall: ${ROOT_DIR}/packaging/postinstall.sh
  postremove: ${ROOT_DIR}/packaging/postremove.sh
EOF

        echo "Packaging ${LABEL} for ${ARCH}..."
        "${NFPM}" package -f "${NFPM_CONFIG}" --packager "${PACKAGER}" --target "${DIST_DIR}/"

        rm -f "${NFPM_CONFIG}"
    done

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
    shopt -s nullglob
    artifacts=( *.deb *.rpm *.pkg.tar.zst *.tar.gz )
    if [ "${#artifacts[@]}" -eq 0 ]; then
        echo "error: no package artifacts found in ${DIST_DIR}" >&2
        exit 1
    fi
    sha256sum "${artifacts[@]}" > checksums.txt
)

echo "Release assets successfully built in ${DIST_DIR}:"
ls -lh "${DIST_DIR}"
