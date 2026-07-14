# Repository Guidelines

## Project Structure & Module Organization

This repository contains a Wails v2 desktop application. Go backend code lives at the repository root: `main.go` boots the app, `app.go` contains application behavior, and `binaries.go` embeds platform-specific workers from `bin/<os>-<arch>/`. Platform resource-limit implementations use `rlimit_unix.go` and `rlimit_windows.go`.

The React/Vite interface is under `frontend/src/`; static and packaging assets live in `frontend/src/assets/` and `build/`. Treat `frontend/wailsjs/` as generated bindings. Go tests are colocated in `app_test.go`. Release automation is in `scripts/build_all.sh`, with artifacts written to `dist/`.

## Build, Test, and Development Commands

- `go test ./...` runs all backend unit tests.
- `npm install --prefix frontend` installs pinned frontend dependencies.
- `npm run build --prefix frontend` creates a production frontend bundle and catches Vite build errors.
- `wails dev` runs the desktop app with frontend hot reload.
- `wails build` builds the current platform application.
- `TARGETS="darwin/arm64 linux/amd64" ./scripts/build_all.sh` builds selected release targets. Linux GUI targets require a Linux host.
- `UPDATE_WORKERS_ONLY=1 TARGETS=darwin/arm64 ./scripts/build_all.sh` refreshes an embedded worker without producing the app bundle.

## Coding Style & Naming Conventions

Format Go changes with `gofmt`; use standard Go naming (`PascalCase` exports, `camelCase` internals) and keep platform-specific code in suffixed files. Frontend code uses ES modules, functional React components, single quotes, semicolons, and two-space indentation. Use `PascalCase` for components and `camelCase` for hooks, handlers, and state. Keep CSS class names consistent with the existing camelCase convention.

## Testing Guidelines

Add focused Go tests beside the implementation using `TestXxx` names and `t.TempDir()` for filesystem isolation. Cover success and failure paths for worker extraction, settings, and process lifecycle changes. There is no configured coverage threshold or frontend test runner; for UI changes, run the production frontend build and manually exercise the affected Wails flow.

## Commit & Pull Request Guidelines

Recent commits use short, imperative subjects, sometimes with Conventional Commit scope, for example `fix(worker): bundle Windows runtime dependencies`. Keep each commit focused and explain platform or packaging implications in the body when relevant. Pull requests should summarize behavior changes, list validation commands, identify tested operating systems, link related issues, and include screenshots for visible UI changes. Do not commit credentials, local settings, caches, or generated `dist/` artifacts unless the release process explicitly requires them.
