Bundled file-proxy worker binaries are stored in platform subdirectories:

- darwin-arm64/file-proxy
- darwin-arm64/file-proxy-web
- darwin-amd64/file-proxy
- windows-amd64/file-proxy.exe
- windows-amd64/file-proxy-web.exe
- linux-amd64/file-proxy
- linux-amd64/file-proxy-web
- linux-arm64/file-proxy
- linux-arm64/file-proxy-web

The binary payloads are local build inputs and are ignored by git. Refresh them
with:

```sh
UPDATE_WORKERS_ONLY=1 TARGETS="windows/amd64" scripts/build_all.sh
```

Keep Windows support files in the same directory as `file-proxy.exe` and
`file-proxy-web.exe`. Current
Windows release bundles may include:

- file-proxy-client.exe
- file-proxy-web.exe
- libffi-8.dll
- libgmp-10.dll
- libmcfgthread-1.dll

The Wails app embeds `bin/` and extracts both executables plus platform support
files at startup.
