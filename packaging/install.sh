#!/usr/bin/env bash
set -uo pipefail

VERSION=${1:-3.0.0}
PREFIX=${2:-"$HOME/go/bin"}
BINARY=${3:-terrat}
DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/apps"
ICON_ROOT="$HOME/.local/share/icons/hicolor"
LOG_FILE=$(mktemp "${TMPDIR:-/tmp}/terrat-install.XXXXXX") || exit 1
SPINNER='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
SPINNER_INDEX=0
ANIMATE=false

if [[ -t 1 && ${TERM:-dumb} != dumb ]]; then
  ANIMATE=true
fi

cleanup() {
  rm -f "$LOG_FILE"
}
trap cleanup EXIT

run_step() {
  local label=$1
  shift

  ("$@" >"$LOG_FILE" 2>&1) &
  local pid=$!

  if [[ "$ANIMATE" == true ]]; then
    while kill -0 "$pid" 2>/dev/null; do
      local frame=${SPINNER:SPINNER_INDEX:1}
      printf '\r\033[2K\033[36m%s\033[0m %s' "$frame" "$label"
      SPINNER_INDEX=$(((SPINNER_INDEX + 1) % ${#SPINNER}))
      sleep 0.08
    done
  else
    printf '• %s...\n' "$label"
  fi

  wait "$pid"
  local status=$?
  if [[ $status -eq 0 ]]; then
    if [[ "$ANIMATE" != true ]]; then
      printf '  done\n'
    fi
    return 0
  fi

  if [[ "$ANIMATE" == true ]]; then
    printf '\r\033[2K\033[31m✗\033[0m %s\n' "$label"
  else
    printf '  failed\n'
  fi
  cat "$LOG_FILE" >&2
  return "$status"
}

install_files() {
  set -e
  mkdir -p "$PREFIX" "$DESKTOP_DIR" "$ICON_DIR"
  install -m 755 "$BINARY" "$PREFIX/$BINARY"
  cp assets/icon.png "$PREFIX/icon.png"
  cp assets/icon.png "$ICON_DIR/terraterminal.png"
  cp assets/icon.png "$ICON_DIR/terrat.png"
  cp packaging/terrat.desktop "$DESKTOP_DIR/terraterminal.desktop"
  cp packaging/terrat.desktop "$DESKTOP_DIR/terrat.desktop"
  sed -i "s|Exec=.*|Exec=$PREFIX/$BINARY|g" "$DESKTOP_DIR/terraterminal.desktop"
  sed -i "s|Icon=.*|Icon=$ICON_DIR/terraterminal.png|g" "$DESKTOP_DIR/terraterminal.desktop"
  sed -i "s|Exec=.*|Exec=$PREFIX/$BINARY|g" "$DESKTOP_DIR/terrat.desktop"
  sed -i "s|Icon=.*|Icon=$ICON_DIR/terrat.png|g" "$DESKTOP_DIR/terrat.desktop"
}

refresh_desktop() {
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$DESKTOP_DIR" || true
  command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t "$ICON_ROOT" 2>/dev/null || true
}

if ! run_step "Building terrat" env GOTOOLCHAIN=local go build \
  -ldflags="-s -w -X main.Version=$VERSION" -o "$BINARY" ./cmd/terrat; then
  exit 1
fi

if ! run_step "Installing files" install_files; then
  exit 1
fi

run_step "Updating app menu" refresh_desktop || true
if [[ "$ANIMATE" == true ]]; then
  printf '\r\033[2K\033[32m✓\033[0m Installed · run: terrat\n'
else
  printf 'Installed to %s/%s · run: terrat\n' "$PREFIX" "$BINARY"
fi
