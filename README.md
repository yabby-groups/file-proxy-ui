# Myna File Proxy

Wails desktop GUI for starting bundled `file-proxy` and `file-proxy-web` binaries.

The app logs into `https://iot.huabot.com`, creates or fetches the current
user's sandbox periodic config, downloads the periodic certificate bundle,
asks the user to select a local root directory, and starts each service independently.
The web gateway listens on `127.0.0.1` and provides a browser UI for a running
worker.

## Bundled binaries

Release builds download the bundled `file-proxy` workers from the upstream
GitHub release before invoking Wails. The default worker release is `v1.2.0.0`.

Downloaded worker binaries are extracted into:

- `bin/darwin-arm64/file-proxy`
- `bin/darwin-amd64/file-proxy`
- `bin/windows-amd64/file-proxy.exe`
- `bin/linux-amd64/file-proxy`
- `bin/linux-arm64/file-proxy`

Every supported v1.2.0.0 archive also provides the matching
`file-proxy-web` executable in the same directory. The desktop application
extracts and manages the worker and web gateway independently.

The macOS aarch64 worker archive also includes `lib/file-proxy/*.dylib`
runtime libraries, which are bundled and extracted next to the app-managed
worker at startup. Only the current runtime target is required for local
development. Missing targets fail at worker start with a clear error.

## Development

Refresh the bundled worker used by Go embed before running Wails dev:

```bash
UPDATE_WORKERS_ONLY=1 TARGETS=darwin/arm64 ./scripts/build_all.sh
```

To test a locally built worker instead of the GitHub release worker:

```bash
LOCAL_WORKER_BIN=/Users/lmj/.local/bin/file-proxy UPDATE_WORKERS_ONLY=1 TARGETS=darwin/arm64 ./scripts/build_all.sh
```

For a full local bundle with macOS support libraries:

```bash
LOCAL_WORKER_BUNDLE=/Users/lmj/yuntan/apps/file-proxy/dist/bundle UPDATE_WORKERS_ONLY=1 TARGETS=darwin/arm64 ./scripts/build_all.sh
```

```bash
npm install --prefix frontend
npm run build --prefix frontend
wails build
```

## Multi-platform release builds

Build all configured desktop targets:

```bash
./scripts/build_all.sh
```

The default target list is:

```text
darwin/arm64 darwin/amd64 windows/amd64 linux/amd64 linux/arm64
```

The script downloads each target's configured worker archive before building.
Set `DOWNLOAD_WORKERS=0` or `LOCAL_WORKER_BIN`/`LOCAL_WORKER_BUNDLE` to build
with a local worker instead.
`darwin/amd64` is kept in the default target list but skipped until a matching
worker archive is configured. Override the target list for partial builds:

```bash
TARGETS="darwin/arm64 linux/amd64" ./scripts/build_all.sh
```

Useful environment overrides:

- `OUT_DIR=dist-release` changes the copied release artifact directory.
- `RUN_TESTS=0` skips `go test ./...`.
- `DOWNLOAD_WORKERS=0` disables upstream worker downloads before building.
- `WORKER_CACHE_REFRESH=0` reuses an existing downloaded worker archive without checking the remote asset.
- `UPDATE_WORKERS_ONLY=1` updates `bin/<target>/file-proxy` and `file-proxy-web` and exits before Wails builds.
- `FILE_PROXY_VERSION=v1.2.0.0` changes the upstream worker release version.
- `FILE_PROXY_RELEASE_BASE=https://github.com/Lupino/file-proxy/releases/download` changes the release base URL.
- `LOCAL_WORKER_BIN=/path/to/file-proxy` copies it and a sibling `file-proxy-web` executable into the selected target's `bin/`.
- `LOCAL_WORKER_BUNDLE=/path/to/bundle` copies both executables and `lib/file-proxy` from a local bundle into the selected target's `bin/`.
- `SKIP_UNSUPPORTED=1` skips missing worker binaries and Wails targets that cannot be built on the current host.
- `WAILS_FLAGS="-trimpath"` appends extra `wails build` flags.
- `GO_ENV_PREFIX="GOSUMDB=off GOPROXY=off"` runs Go/Wails with an explicit Go environment.

Release artifacts are copied to `dist/` after each target build.

Wails can build Windows targets from macOS, but Linux GUI targets must be built
on a Linux host or Linux CI runner. The script reports those targets before
release builds when the current host cannot build them.
