# Khali

macOS app that frees disk space taken by node_modules, build output, generated native projects and shared developer caches.

## Features

- Scans your project folders (default `~/Desktop/praveen`) and lists each project's regenerable folders with their real size on disk
- Cleans `node_modules`, `dist`, `.expo` and similar, whole Expo-generated `android/` / `ios/` folders, and `.apk` / `.aab` files
- Only touches what git ignores and doesn't track; anything else is shown with a lock
- Pin the project you're working on so it is never cleaned; "Select idle" picks everything in projects untouched for a few days
- Shared caches: npm, Gradle, Android NDK / build tools / platforms, Xcode DerivedData, unavailable simulators, CocoaPods, Go, Homebrew, old Convex local backends
- Android NDK, build tools, platform and Gradle versions your projects still use are marked in use and can't be selected
- Delete or move to the Trash, with a log of what went and how to get it back
- Menu bar item with the free space; closing the window keeps it running
- Single user, local only: no account or sign-in

## Stack

Go · Wails v3 · React 19 · TypeScript · Vite · Tailwind CSS v4 · shadcn/ui (Base UI) · Vitest

## Development

Needs Go 1.25+, Node 22.18+ (`.nvmrc`) and the Wails v3 CLI, pinned to the beta this repo uses.

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.30
npm --prefix frontend install
wails3 dev          # app with hot reload
```

## Scripts

| Command | What it does |
|---|---|
| `wails3 dev` | Run the app with frontend hot reload |
| `wails3 task package` | Build `bin/khali.app` (ad-hoc signed) |
| `wails3 task run` | Build and run a dev `.app` |
| `scripts/check.sh` | Generate bindings, Go vet + tests, frontend typecheck + lint + tests |
| `npm --prefix frontend run check` | Frontend checks only |

## Deployment

No server. `wails3 task package`, then copy `bin/khali.app` to `/Applications`. The build is ad-hoc signed, so the first launch may need right-click → Open.

## How it works

- **Projects**: git repos, or folders with a `package.json` / `go.mod` / `app.json` outside one, up to four levels below each folder in Settings.
- **Rules** (`internal/scan/rules.go`): `node_modules` and build folders next to a `package.json`; in an Expo app (`expo` dependency + `app.json`) the whole `android/` and `ios/` folders, since `expo prebuild` makes them; elsewhere only their build folders (`app/build`, `.gradle`, `.cxx`, `Pods` ...). Hidden folders such as `.convex` and plain `build/` are never matched.
- **Safety**: in a git repo a target must pass `git check-ignore` and contain no files from `git ls-files`. If a whole `android/` fails, its build folders are offered instead. The service only deletes paths the last scan marked safe, in unpinned projects, inside the home folder, and never through a symlink.
- **Active projects**: newest of the last commit, git's index and file edits (folder times are ignored so cleaning doesn't count as activity).
- **Android versions** (`internal/scan/versions.go`): read from `node_modules/react-native/gradle/libs.versions.toml`, the gradle files and `gradle-wrapper.properties`. Versions seen are remembered per React Native minor version, so they are still known after `android/` and `node_modules` are cleaned. If any app's versions are unknown, unused versions are shown as "probably unused".
- **Sizes** are allocated blocks, with hard links counted once.
- **State** in `~/Library/Application Support/khali/`: `settings.json`, `versions.json` and `log.jsonl`.
- **Code map**: `main.go` (window, menu bar), `service.go` (methods the frontend calls), `internal/scan`, `internal/caches`, `internal/fsx` (size, delete, trash), `internal/store`, `frontend/src` (UI; `frontend/bindings` is generated).
