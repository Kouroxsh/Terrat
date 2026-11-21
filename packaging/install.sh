#!/usr/bin/env bash
set -euo pipefail

# H15: resolve the repo root from this script's own location instead of
# assuming the caller runs from the repo root.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/1024x1024/apps"
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

# L52: uninstall mode (usage: install.sh --uninstall [PREFIX] [BINARY]).
if [[ "${1:-}" == "--uninstall" || "${1:-}" == "-u" ]]; then
  PREFIX=${2:-"$HOME/go/bin"}
  BINARY=${3:-terrat}
  BIN_NAME="$(basename "$BINARY")"
  echo "Uninstalling TerraTerminal..."
  rm -f "$PREFIX/$BIN_NAME" "$PREFIX/$BIN_NAME.bak"
  rm -f "$DESKTOP_DIR/terrat.desktop" "$DESKTOP_DIR/terraterminal.desktop"
  rm -f "$ICON_DIR/terrat.png" "$ICON_DIR/terraterminal.png"
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$DESKTOP_DIR" || true
  command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t "$ICON_ROOT" 2>/dev/null || true
  echo "TerraTerminal uninstalled."
  exit 0
fi

# L56: the VERSION file is the single source of truth; the argument still
# wins when given explicitly.
VERSION=${1:-$(cat "${ROOT_DIR}/VERSION" 2>/dev/null || echo 3.0.0)}
# M34: accept an optional leading `v` (git tags are v3.0.0); the banner
# already adds its own `v`.
VERSION=${VERSION#v}
PREFIX=${2:-"$HOME/go/bin"}
BINARY=${3:-terrat}
# L61: a BINARY containing `/` used to corrupt the install destination
# ("$PREFIX/$BINARY"); install under its basename instead.
BIN_NAME="$(basename "$BINARY")"
case "$BINARY" in
  /*) BUILD_BIN="$BINARY" ;;
  *)  BUILD_BIN="${ROOT_DIR}/${BIN_NAME}" ;;
esac

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
  # L52: keep a backup of the previous binary instead of overwriting it
  # silently.
  if [[ -f "$PREFIX/$BIN_NAME" ]]; then
    cp -f "$PREFIX/$BIN_NAME" "$PREFIX/$BIN_NAME.bak"
  fi
  install -m 755 "$BUILD_BIN" "$PREFIX/$BIN_NAME"
  cp "$ROOT_DIR/assets/icon.png" "$ICON_DIR/terrat.png"
  # L53: rewrite the Exec/Icon lines line-by-line instead of sed, so `&`,
  # `|`, spaces and backslashes in PREFIX/BIN_NAME cannot break the
  # replacement (bash ${var//pat/rep} treats `&` as "the matched text").
  # (L55: a single launcher — no duplicate terraterminal.desktop.)
  {
    while IFS= read -r line; do
      case "$line" in
        Exec=*) printf '%s\n' "Exec=$PREFIX/$BIN_NAME" ;;
        Icon=*) printf '%s\n' "Icon=$ICON_DIR/terrat.png" ;;
        *)      printf '%s\n' "$line" ;;
      esac
    done < "$ROOT_DIR/packaging/terrat.desktop"
  } > "$DESKTOP_DIR/terrat.desktop"
}

refresh_desktop() {
  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$DESKTOP_DIR" || true
  command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t "$ICON_ROOT" 2>/dev/null || true
}

if ! run_step "Building terrat" env CGO_ENABLED=0 go -C "${ROOT_DIR}" build \
  -ldflags="-s -w -X main.Version=$VERSION" -o "$BUILD_BIN" ./cmd/terrat; then
  exit 1
fi

if ! run_step "Installing files" install_files; then
  exit 1
fi

run_step "Updating app menu" refresh_desktop || true
if [[ "$ANIMATE" == true ]]; then
  printf '\r\033[2K\033[32m✓\033[0m Installed · run: terrat\n'
else
  printf 'Installed to %s/%s · run: terrat\n' "$PREFIX" "$BIN_NAME"
fi
