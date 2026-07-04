#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_NAME="MynaFileProxy"
OUT_DIR="${OUT_DIR:-$ROOT/dist}"
TARGETS="${TARGETS:-darwin/arm64 darwin/amd64 windows/amd64 linux/amd64 linux/arm64}"
RUN_TESTS="${RUN_TESTS:-1}"
WAILS_FLAGS="${WAILS_FLAGS:-}"
GO_ENV_PREFIX="${GO_ENV_PREFIX:-}"
SKIP_UNSUPPORTED="${SKIP_UNSUPPORTED:-0}"

cd "$ROOT"

log() {
  printf '\n==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

target_slug() {
  printf '%s' "$1" | tr '/' '-'
}

worker_binary_for_target() {
  case "$1" in
    windows/*) printf 'bin/%s/file-proxy.exe' "$(target_slug "$1")" ;;
    *) printf 'bin/%s/file-proxy' "$(target_slug "$1")" ;;
  esac
}

artifact_name_for_target() {
  case "$1" in
    darwin/*) printf '%s-%s.app' "$APP_NAME" "$(target_slug "$1")" ;;
    windows/*) printf '%s-%s.exe' "$APP_NAME" "$(target_slug "$1")" ;;
    linux/*) printf '%s-%s' "$APP_NAME" "$(target_slug "$1")" ;;
    *) printf '%s-%s' "$APP_NAME" "$(target_slug "$1")" ;;
  esac
}

built_path_for_target() {
  case "$1" in
    darwin/*) printf 'build/bin/%s.app' "$APP_NAME" ;;
    windows/*) printf 'build/bin/%s.exe' "$APP_NAME" ;;
    *) printf 'build/bin/%s' "$APP_NAME" ;;
  esac
}

target_os() {
  printf '%s' "${1%%/*}"
}

target_arch() {
  printf '%s' "${1##*/}"
}

host_supports_target() {
  local target="$1"
  local host_goos="$2"
  local os
  os="$(target_os "$target")"

  case "$os" in
    windows)
      return 0
      ;;
    darwin)
      [[ "$host_goos" == "darwin" ]]
      ;;
    linux)
      [[ "$host_goos" == "linux" ]]
      ;;
    *)
      return 1
      ;;
  esac
}

copy_artifact() {
  local target="$1"
  local src
  local dst
  src="$(built_path_for_target "$target")"
  dst="$OUT_DIR/$(artifact_name_for_target "$target")"
  [[ -e "$src" ]] || fail "expected build artifact not found: $src"
  rm -rf "$dst"
  cp -R "$src" "$dst"
  printf 'artifact: %s\n' "$dst"
}

run_go() {
  if [[ -n "$GO_ENV_PREFIX" ]]; then
    # shellcheck disable=SC2086
    env $GO_ENV_PREFIX go "$@"
  else
    go "$@"
  fi
}

run_wails() {
  local target="$1"
  if [[ -n "$GO_ENV_PREFIX" ]]; then
    # shellcheck disable=SC2086
    env $GO_ENV_PREFIX wails build -clean -platform "$target" -webview2 embed $WAILS_FLAGS
  else
    # shellcheck disable=SC2086
    wails build -clean -platform "$target" -webview2 embed $WAILS_FLAGS
  fi
}

require_cmd go
require_cmd npm
require_cmd wails

HOST_GOOS="$(go env GOOS)"
missing=()
unsupported=()
build_targets=()
for target in $TARGETS; do
  worker="$(worker_binary_for_target "$target")"
  if [[ ! -f "$worker" ]]; then
    if [[ "$SKIP_UNSUPPORTED" == "1" ]]; then
      printf 'skip: %s missing %s\n' "$target" "$worker" >&2
      continue
    else
      missing+=("$target -> $worker")
    fi
  elif ! host_supports_target "$target" "$HOST_GOOS"; then
    if [[ "$SKIP_UNSUPPORTED" == "1" ]]; then
      printf 'skip: %s cannot be built on host GOOS=%s with Wails\n' "$target" "$HOST_GOOS" >&2
      continue
    else
      unsupported+=("$target on host GOOS=$HOST_GOOS")
    fi
  else
    build_targets+=("$target")
  fi
done

if (( ${#missing[@]} > 0 )); then
  printf 'Missing bundled file-proxy worker binaries:\n' >&2
  printf '  %s\n' "${missing[@]}" >&2
  printf '\nSet TARGETS to a smaller list or add the missing binaries before release builds.\n' >&2
  exit 1
fi

if (( ${#unsupported[@]} > 0 )); then
  printf 'Unsupported Wails cross-build targets for this host:\n' >&2
  printf '  %s\n' "${unsupported[@]}" >&2
  printf '\nUse SKIP_UNSUPPORTED=1 to build only targets supported by this host, or run Linux targets on a Linux builder.\n' >&2
  exit 1
fi

if (( ${#build_targets[@]} == 0 )); then
  fail "no buildable targets selected"
fi

log "Generate logo assets"
run_go run scripts/generate_logo_assets.go .

log "Install frontend dependencies"
npm install --prefix frontend

log "Build frontend"
npm run build --prefix frontend

if [[ "$RUN_TESTS" == "1" ]]; then
  log "Run Go tests"
  run_go test ./...
fi

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

for target in "${build_targets[@]}"; do
  log "Build $target"
  run_wails "$target"
  copy_artifact "$target"
done

log "Done"
find "$OUT_DIR" -maxdepth 1 -mindepth 1 -print | sort
