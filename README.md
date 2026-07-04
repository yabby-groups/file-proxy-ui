# Myna File Proxy

Wails desktop GUI for starting a bundled `file-proxy` worker.

The app logs into `https://iot.huabot.com`, creates or fetches the current
user's sandbox periodic config, downloads the periodic certificate bundle,
asks the user to select a local root directory, and starts `file-proxy`.

## Bundled binaries

Place platform binaries here before building a release:

- `bin/darwin-arm64/file-proxy`
- `bin/darwin-amd64/file-proxy`
- `bin/windows-amd64/file-proxy.exe`
- `bin/linux-amd64/file-proxy`
- `bin/linux-arm64/file-proxy`

Only the current runtime target is required for local development. Missing
targets fail at worker start with a clear error.

## Development

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

The script checks that each target has a matching bundled worker binary before
building. Override the target list for partial builds:

```bash
TARGETS="darwin/arm64 linux/amd64" ./scripts/build_all.sh
```

Useful environment overrides:

- `OUT_DIR=dist-release` changes the copied release artifact directory.
- `RUN_TESTS=0` skips `go test ./...`.
- `SKIP_UNSUPPORTED=1` skips missing worker binaries and Wails targets that cannot be built on the current host.
- `WAILS_FLAGS="-trimpath"` appends extra `wails build` flags.
- `GO_ENV_PREFIX="GOSUMDB=off GOPROXY=off"` runs Go/Wails with an explicit Go environment.

Release artifacts are copied to `dist/` after each target build.

Wails can build Windows targets from macOS, but Linux GUI targets must be built
on a Linux host or Linux CI runner. The script reports those targets before
release builds when the current host cannot build them.
