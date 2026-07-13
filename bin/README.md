Bundled file-proxy worker binaries are stored in platform subdirectories:

- darwin-arm64/file-proxy
- darwin-amd64/file-proxy
- windows-amd64/file-proxy.exe
- linux-amd64/file-proxy
- linux-arm64/file-proxy

The binary payloads are local build inputs and are ignored by git. Refresh them
with:

```sh
UPDATE_WORKERS_ONLY=1 TARGETS="windows/amd64" scripts/build_all.sh
```

Keep Windows support files in the same directory as `file-proxy.exe`. Current
Windows release bundles may include:

- file-proxy-client.exe
- libffi-8.dll
- libgmp-10.dll
- libmcfgthread-1.dll

The Wails app embeds `bin/` and extracts the worker plus platform support files
at startup.
