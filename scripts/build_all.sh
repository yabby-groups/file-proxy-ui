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
DOWNLOAD_WORKERS="${DOWNLOAD_WORKERS:-1}"
WORKER_CACHE_REFRESH="${WORKER_CACHE_REFRESH:-1}"
UPDATE_WORKERS_ONLY="${UPDATE_WORKERS_ONLY:-0}"
LOCAL_WORKER_BIN="${LOCAL_WORKER_BIN:-}"
LOCAL_WORKER_BUNDLE="${LOCAL_WORKER_BUNDLE:-}"
FILE_PROXY_VERSION="${FILE_PROXY_VERSION:-v1.1.0.0}"
FILE_PROXY_RELEASE_BASE="${FILE_PROXY_RELEASE_BASE:-https://github.com/Lupino/file-proxy/releases/download}"
WORKER_CACHE_DIR="${WORKER_CACHE_DIR:-$ROOT/.cache/file-proxy-workers/$FILE_PROXY_VERSION}"

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

clear_quarantine() {
  if [[ "$(uname -s)" != "Darwin" ]]; then
    return
  fi
  command -v xattr >/dev/null 2>&1 || return
  xattr -dr com.apple.quarantine "$@" 2>/dev/null || true
}

install_file() {
  local src="$1"
  local dst="$2"

  rm -f "$dst"
  cp "$src" "$dst"
}

copy_sibling_files() {
  local source_dir="$1"
  local target_dir="$2"
  local main_name="$3"
  local source

  while IFS= read -r source; do
    install_file "$source" "$target_dir/$(basename "$source")"
  done < <(find "$source_dir" -maxdepth 1 -type f ! -name "$main_name" -print)
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

worker_archive_for_target() {
  case "$1" in
    darwin/arm64) printf 'file-proxy-macos-aarch64-%s.tar.bz2' "$FILE_PROXY_VERSION" ;;
    windows/amd64) printf 'file-proxy-windows-%s.tar.bz2' "$FILE_PROXY_VERSION" ;;
    linux/amd64) printf 'file-proxy-linux-%s.tar.bz2' "$FILE_PROXY_VERSION" ;;
    linux/arm64) printf 'file-proxy-linux-aarch64-%s.tar.bz2' "$FILE_PROXY_VERSION" ;;
    *) printf '' ;;
  esac
}

copy_local_worker_source() {
  local target="$1"
  local target_dir="bin/$(target_slug "$target")"
  local worker
  local source_bin=""

  worker="$(worker_binary_for_target "$target")"
  case "$target" in
    windows/*) source_bin="file-proxy.exe" ;;
    *) source_bin="file-proxy" ;;
  esac

  mkdir -p "$target_dir"
  if [[ -n "$LOCAL_WORKER_BUNDLE" ]]; then
    [[ -f "$LOCAL_WORKER_BUNDLE/bin/$source_bin" ]] || fail "local worker bundle missing bin/$source_bin: $LOCAL_WORKER_BUNDLE"
    install_file "$LOCAL_WORKER_BUNDLE/bin/$source_bin" "$worker"
    if [[ "$target" == windows/* ]]; then
      copy_sibling_files "$LOCAL_WORKER_BUNDLE/bin" "$target_dir" "$source_bin"
    fi
    if [[ "$target" == darwin/* && -d "$LOCAL_WORKER_BUNDLE/lib/file-proxy" ]]; then
      rm -rf "$target_dir/lib"
      mkdir -p "$target_dir/lib"
      cp -R "$LOCAL_WORKER_BUNDLE/lib/file-proxy" "$target_dir/lib/file-proxy"
    fi
  else
    [[ -f "$LOCAL_WORKER_BIN" ]] || fail "local worker binary not found: $LOCAL_WORKER_BIN"
    install_file "$LOCAL_WORKER_BIN" "$worker"
  fi
  case "$target" in
    windows/*) ;;
    *) chmod 0755 "$worker" ;;
  esac
  clear_quarantine "$target_dir"
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

download_worker_archive() {
  local target="$1"
  local archive="$2"
  local cached="$WORKER_CACHE_DIR/$archive"
  local url="$FILE_PROXY_RELEASE_BASE/$FILE_PROXY_VERSION/$archive"
  local tmp="$cached.tmp.$$"

  mkdir -p "$WORKER_CACHE_DIR"
  if [[ -f "$cached" && "$WORKER_CACHE_REFRESH" != "1" ]]; then
    printf 'cached: %s\n' "$cached"
    return
  fi

  if [[ -f "$cached" ]]; then
    log "Refresh file-proxy worker $target $url"
  else
    log "Download file-proxy worker $target $url"
  fi
  if curl -L --fail --show-error --output "$tmp" "$url"; then
    if [[ -f "$cached" ]] && cmp -s "$tmp" "$cached"; then
      rm -f "$tmp"
      printf 'cached unchanged: %s\n' "$cached"
    else
      mv "$tmp" "$cached"
      printf 'updated: %s\n' "$cached"
    fi
    return
  fi

  rm -f "$tmp"
  if [[ -f "$cached" ]]; then
    printf 'warning: refresh failed, using cached worker archive: %s\n' "$cached" >&2
    return
  fi
  return 1
}

extract_worker_archive() {
  local target="$1"
  local archive="$2"
  local cached="$WORKER_CACHE_DIR/$archive"
  local target_dir="bin/$(target_slug "$target")"
  local tmp_dir

  tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/file-proxy-worker.XXXXXX")"
  tar -xjf "$cached" -C "$tmp_dir"

  mkdir -p "$target_dir"
  case "$target" in
    windows/*)
      [[ -f "$tmp_dir/file-proxy.exe" ]] || fail "file-proxy.exe not found in $archive"
      install_file "$tmp_dir/file-proxy.exe" "$target_dir/file-proxy.exe"
      copy_sibling_files "$tmp_dir" "$target_dir" "file-proxy.exe"
      ;;
    darwin/arm64)
      [[ -f "$tmp_dir/bin/file-proxy" ]] || fail "bin/file-proxy not found in $archive"
      install_file "$tmp_dir/bin/file-proxy" "$target_dir/file-proxy"
      chmod 0755 "$target_dir/file-proxy"
      if [[ -d "$tmp_dir/lib/file-proxy" ]]; then
        rm -rf "$target_dir/lib"
        mkdir -p "$target_dir/lib"
        cp -R "$tmp_dir/lib/file-proxy" "$target_dir/lib/file-proxy"
      fi
      clear_quarantine "$target_dir"
      ;;
    linux/*)
      [[ -f "$tmp_dir/file-proxy" ]] || fail "file-proxy not found in $archive"
      install_file "$tmp_dir/file-proxy" "$target_dir/file-proxy"
      chmod 0755 "$target_dir/file-proxy"
      ;;
    *)
      fail "no extraction rule for $target"
      ;;
  esac

  rm -rf "$tmp_dir"
}

ensure_worker_binary() {
  local target="$1"
  local worker
  local archive

  worker="$(worker_binary_for_target "$target")"
  archive="$(worker_archive_for_target "$target")"
  if [[ -n "$LOCAL_WORKER_BIN" || -n "$LOCAL_WORKER_BUNDLE" ]]; then
    copy_local_worker_source "$target"
    [[ -f "$worker" ]] || fail "expected local worker binary not found after copy: $worker"
    return 0
  fi

  if [[ "$DOWNLOAD_WORKERS" == "1" && -n "$archive" ]]; then
    download_worker_archive "$target" "$archive"
    extract_worker_archive "$target" "$archive"
    [[ -f "$worker" ]] || fail "expected worker binary not found after extraction: $worker"
    return 0
  fi

  if [[ -f "$worker" ]]; then
    return 0
  fi

  if [[ -z "$archive" ]]; then
    printf 'skip: %s has no configured worker archive\n' "$target" >&2
    return 1
  fi
  if [[ "$DOWNLOAD_WORKERS" != "1" ]]; then
    printf 'skip: %s missing %s and DOWNLOAD_WORKERS=%s\n' "$target" "$worker" "$DOWNLOAD_WORKERS" >&2
    return 1
  fi
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
    env $GO_ENV_PREFIX wails build -platform "$target" -webview2 embed $WAILS_FLAGS
  else
    # shellcheck disable=SC2086
    wails build -platform "$target" -webview2 embed $WAILS_FLAGS
  fi
}

if [[ "$DOWNLOAD_WORKERS" == "1" ]]; then
  require_cmd curl
  require_cmd tar
fi
if [[ "$UPDATE_WORKERS_ONLY" != "1" ]]; then
  require_cmd go
  require_cmd npm
  require_cmd wails
fi

HOST_GOOS=""
if [[ "$UPDATE_WORKERS_ONLY" != "1" ]]; then
  HOST_GOOS="$(go env GOOS)"
fi
missing=()
unsupported=()
build_targets=()
for target in $TARGETS; do
  worker="$(worker_binary_for_target "$target")"
  if [[ "$UPDATE_WORKERS_ONLY" != "1" ]] && ! host_supports_target "$target" "$HOST_GOOS"; then
    if [[ "$SKIP_UNSUPPORTED" == "1" ]]; then
      printf 'skip: %s cannot be built on host GOOS=%s with Wails\n' "$target" "$HOST_GOOS" >&2
      continue
    else
      unsupported+=("$target on host GOOS=$HOST_GOOS")
      continue
    fi
  fi
  if ! ensure_worker_binary "$target"; then
    continue
  fi
  if [[ ! -f "$worker" ]]; then
    missing+=("$target -> $worker")
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

if [[ "$UPDATE_WORKERS_ONLY" == "1" ]]; then
  log "Updated worker binaries"
  for target in "${build_targets[@]}"; do
    worker="$(worker_binary_for_target "$target")"
    printf 'worker: %s\n' "$worker"
  done
  exit 0
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
