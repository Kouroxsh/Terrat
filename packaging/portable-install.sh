#!/bin/sh
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_SRC="${SCRIPT_DIR}/terrat"
ICON_SRC="${SCRIPT_DIR}/icon.png"
DESKTOP_SRC="${SCRIPT_DIR}/terrat.desktop"

INSTALL_BIN_DIR="${HOME}/.local/bin"
INSTALL_APP_DIR="${HOME}/.local/share/applications"
INSTALL_ICON_DIR="${HOME}/.local/share/icons/hicolor/1024x1024/apps"

if [ "${1:-}" = "--uninstall" ] || [ "${1:-}" = "-u" ]; then
    echo "Uninstalling TerraTerminal from user directory..."
    rm -f "${INSTALL_BIN_DIR}/terrat"
    rm -f "${INSTALL_APP_DIR}/terrat.desktop"
    rm -f "${INSTALL_APP_DIR}/terraterminal.desktop"
    rm -f "${INSTALL_ICON_DIR}/terrat.png"
    rm -f "${INSTALL_ICON_DIR}/terraterminal.png"

    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database "${INSTALL_APP_DIR}" || true
    fi
    if command -v gtk-update-icon-cache >/dev/null 2>&1; then
        gtk-update-icon-cache -q -t -f "${HOME}/.local/share/icons/hicolor" 2>/dev/null || true
    fi
    echo "TerraTerminal uninstalled successfully."
    exit 0
fi

if [ ! -f "${BIN_SRC}" ]; then
    echo "Error: terrat binary not found in ${SCRIPT_DIR}"
    exit 1
fi

echo "Installing TerraTerminal for current user ($(whoami))..."

mkdir -p "${INSTALL_BIN_DIR}"
mkdir -p "${INSTALL_APP_DIR}"
mkdir -p "${INSTALL_ICON_DIR}"

install -m 755 "${BIN_SRC}" "${INSTALL_BIN_DIR}/terrat"

if [ -f "${ICON_SRC}" ]; then
    install -m 644 "${ICON_SRC}" "${INSTALL_ICON_DIR}/terrat.png"
fi

# L54: use the bundled terrat.desktop when it is shipped alongside the
# tarball instead of generating a second copy of it here; the generated
# entry below is only a fallback for a bare binary.
# (L55: a single launcher — no duplicate terraterminal.desktop.)
if [ -f "${DESKTOP_SRC}" ]; then
    # Same substitution as packaging/install.sh, via a line rewrite so
    # special characters in the paths cannot break it.
    {
        while IFS= read -r line; do
            case "$line" in
                Exec=*)  printf '%s\n' "Exec=${INSTALL_BIN_DIR}/terrat" ;;
                Icon=*)  printf '%s\n' "Icon=${INSTALL_ICON_DIR}/terrat.png" ;;
                *)       printf '%s\n' "$line" ;;
            esac
        done < "${DESKTOP_SRC}"
    } > "${INSTALL_APP_DIR}/terrat.desktop"
    chmod 644 "${INSTALL_APP_DIR}/terrat.desktop"
else
    cat << EOF > "${INSTALL_APP_DIR}/terrat.desktop"
[Desktop Entry]
Version=1.0
Type=Application
Name=TerraTerminal
GenericName=Terminal Emulator
Comment=Linux terminal emulator written in Go
Exec=${INSTALL_BIN_DIR}/terrat
Icon=${INSTALL_ICON_DIR}/terrat.png
Terminal=false
Categories=System;TerminalEmulator;Utility;
Keywords=shell;prompt;command;commandline;cmd;terrat;terminal;
StartupNotify=true
StartupWMClass=terrat
EOF
    chmod 644 "${INSTALL_APP_DIR}/terrat.desktop"
fi

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "${INSTALL_APP_DIR}" || true
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q -t -f "${HOME}/.local/share/icons/hicolor" 2>/dev/null || true
fi

echo "TerraTerminal installed to ${INSTALL_BIN_DIR}/terrat"
if ! echo "$PATH" | tr ':' '\n' | grep -qx "${INSTALL_BIN_DIR}"; then
    echo "Note: Make sure ${INSTALL_BIN_DIR} is in your PATH."
fi
echo "You can also run ./terrat directly from this folder without installing."
