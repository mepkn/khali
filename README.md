# Khali

macOS app that frees disk space taken by node_modules, build output, generated native projects and shared developer caches.

## Features

- Scans your project folders (default `~/Desktop/praveen`) and lists each project's regenerable folders with their real size on disk
- Cleans `node_modules`, `dist`, `.expo` and similar, whole Expo-generated `android/` / `ios/` folders, and `.apk` / `.aab` files
- Only touches what git ignores and doesn't track; anything else is shown with a lock
- Pin the project you're working on so it is never cleaned; "Select idle" picks everything in projects untouched for a few days
- Shared caches: npm, Gradle, Android NDK / build tools / platforms, Xcode DerivedData, unavailable simulators, CocoaPods, Go, Homebrew, old Convex local backends, Playwright browsers
- Docker (OrbStack or Docker Desktop): unused images, stopped containers, build cache and unused volumes, removed with `docker ... prune`; volumes are never picked by "Select unused"
- Android NDK, build tools, platform and Gradle versions your projects still use are marked in use and can't be selected
- Delete or move to the Trash, with a log of what went and how to get it back
- Menu bar item with the free space; closing the window keeps it running
- Checks for updates on launch and from Khali → Check for Updates… in the menu bar; installs the latest GitHub release and restarts
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
| `scripts/install.sh` | Build and install to `/Applications/Khali.app`, then relaunch |
| `scripts/check.sh` | Generate bindings, Go vet + tests, frontend typecheck + lint + tests |
| `npm --prefix frontend run check` | Frontend checks only |

## Deployment

No server. `scripts/install.sh` builds the app and replaces `/Applications/Khali.app`. The build is ad-hoc signed, so the first launch may need right-click → Open.

## How it works

- **Projects**: git repos, or folders with a `package.json` / `go.mod` / `app.json` outside one, up to four levels below each folder in Settings.
- **Rules** (`internal/scan/rules.go`): `node_modules` and build folders next to a `package.json`; in an Expo app (`expo` dependency + `app.json`) the whole `android/` and `ios/` folders, since `expo prebuild` makes them; elsewhere only their build folders (`app/build`, `.gradle`, `.cxx`, `Pods` ...). Next to a `go.mod`: `bin/`, and air's `tmp/` when there is an `.air.toml`; these need git to confirm they are ignored. Hidden folders such as `.convex` and plain `build/` are never matched.
- **Safety**: in a git repo a target must pass `git check-ignore` and contain no files from `git ls-files`. If a whole `android/` fails, its build folders are offered instead. The service only deletes paths the last scan marked safe, in unpinned projects, inside the home folder, and never through a symlink.
- **Active projects**: newest of the last commit, git's index and file edits (folder times are ignored so cleaning doesn't count as activity).
- **Android versions** (`internal/scan/versions.go`): read from `node_modules/react-native/gradle/libs.versions.toml`, the gradle files and `gradle-wrapper.properties`. Versions seen are remembered per React Native minor version, so they are still known after `android/` and `node_modules` are cleaned. If any app's versions are unknown, unused versions are shown as "probably unused".
- **Sizes** are allocated blocks, with hard links counted once.
- **State** in `~/Library/Application Support/khali/`: `settings.json`, `versions.json` and `log.jsonl`.
- **Updates** (`update.go`): release builds carry their short commit SHA (`-X main.commit`, set in `build/darwin/Taskfile.yml`). On launch and from Khali → Check for Updates…, the app compares it with the tag of the latest GitHub release; if they differ it downloads `khali-macos.zip` to a temp folder, quits, swaps the `.app` in place (putting the old one back if that fails), deletes the download and relaunches. The app's folder must be writable. Builds without a SHA (`wails3 dev`) skip the check.
- **Code map**: `main.go` (window, menu bar), `service.go` (methods the frontend calls), `internal/scan`, `internal/caches`, `internal/fsx` (size, delete, trash), `internal/store`, `frontend/src` (UI; `frontend/bindings` is generated).
