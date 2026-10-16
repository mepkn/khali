// Package caches lists the shared developer caches outside the projects:
// Android SDK parts, Gradle, npm, Xcode, Go and friends. Versioned items
// (an NDK, a Gradle distribution) are marked in use when a project needs them.
package caches

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"khali/internal/fsx"
	"khali/internal/scan"
)

// Status of an item; "inUse" items are shown but can't be selected.
const (
	InUse  = "inUse"  // a project needs this version
	Unused = "unused" // a version no project uses
	Maybe  = "maybe"  // probably unused, but not every project could be read
	Cache  = "cache"  // downloaded or rebuilt again when needed
)

type Item struct {
	ID     string `json:"id"`
	Group  string `json:"group"`
	Label  string `json:"label"`
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	Status string `json:"status"`
	Note   string `json:"note"`
	// Command, when set, is run instead of deleting Path.
	Command []string `json:"command"`
}

// Used is what the scanned projects need, gathered from scan.AndroidApp.
type Used struct {
	NDK, BuildTools, CompileSdk, Gradle map[string][]string // version → projects
	Unresolved                          []string            // apps with unknown versions
}

func UsedBy(apps []scan.AndroidApp) Used {
	u := Used{NDK: map[string][]string{}, BuildTools: map[string][]string{}, CompileSdk: map[string][]string{}, Gradle: map[string][]string{}}
	for _, a := range apps {
		name := a.Project
		if a.Rel != "." {
			name += "/" + a.Rel
		}
		add := func(m map[string][]string, v string) {
			if v != "" {
				m[v] = append(m[v], name)
			}
		}
		add(u.NDK, a.Versions.NDK)
		add(u.BuildTools, a.Versions.BuildTools)
		add(u.CompileSdk, a.Versions.CompileSdk)
		add(u.Gradle, a.Versions.Gradle)
		if !a.Resolved {
			u.Unresolved = append(u.Unresolved, name)
		}
	}
	return u
}

// versioned sets status and note for a version from the used map.
func (u Used) versioned(it *Item, used map[string][]string, version string) {
	if by := used[version]; len(by) > 0 {
		it.Status = InUse
		it.Note = "Used by " + strings.Join(uniq(by), ", ")
		return
	}
	if len(u.Unresolved) > 0 {
		it.Status = Maybe
		it.Note = "No project found using it, but versions are unknown for " + strings.Join(u.Unresolved, ", ")
		return
	}
	it.Status = Unused
	it.Note = "No project uses this version"
}

// List finds the caches on this Mac and sizes them in parallel.
func List(home string, apps []scan.AndroidApp) []Item {
	u := UsedBy(apps)
	var items []Item
	add := func(it Item) {
		if it.ID == "" {
			it.ID = it.Path
		}
		items = append(items, it)
	}
	dir := func(group, label, rel, status, note string) {
		p := filepath.Join(home, rel)
		if fsx.IsDir(p) {
			add(Item{Group: group, Label: label, Path: p, Status: status, Note: note})
		}
	}

	// Android SDK
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		sdk = filepath.Join(home, "Library/Android/sdk")
	}
	for _, v := range subdirs(filepath.Join(sdk, "ndk")) {
		it := Item{Group: "Android SDK", Label: "NDK " + v, Path: filepath.Join(sdk, "ndk", v)}
		u.versioned(&it, u.NDK, v)
		add(it)
	}
	for _, v := range subdirs(filepath.Join(sdk, "build-tools")) {
		it := Item{Group: "Android SDK", Label: "Build tools " + v, Path: filepath.Join(sdk, "build-tools", v)}
		u.versioned(&it, u.BuildTools, v)
		add(it)
	}
	for _, v := range subdirs(filepath.Join(sdk, "platforms")) {
		it := Item{Group: "Android SDK", Label: "Platform " + v, Path: filepath.Join(sdk, "platforms", v)}
		u.versioned(&it, u.CompileSdk, strings.TrimPrefix(v, "android-"))
		add(it)
	}
	for _, v := range subdirs(filepath.Join(sdk, "cmake")) {
		add(Item{Group: "Android SDK", Label: "CMake " + v, Path: filepath.Join(sdk, "cmake", v), Status: Maybe,
			Note: "React Native picks its CMake version at build time; Gradle installs it again if needed"})
	}
	avdImages := avdSystemImages(home)
	for _, v := range subdirs(filepath.Join(sdk, "system-images")) {
		it := Item{Group: "Android SDK", Label: "Emulator image " + v, Path: filepath.Join(sdk, "system-images", v)}
		if by := avdImages[v]; len(by) > 0 {
			it.Status, it.Note = InUse, "Used by emulator "+strings.Join(by, ", ")
		} else {
			it.Status, it.Note = Unused, "No emulator uses it"
		}
		add(it)
	}

	// Gradle
	gradle := filepath.Join(home, ".gradle")
	reDist := regexp.MustCompile(`^gradle-(.+)-(bin|all)$`)
	for _, name := range subdirs(filepath.Join(gradle, "wrapper/dists")) {
		m := reDist.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		it := Item{Group: "Gradle", Label: "Gradle " + m[1] + " (" + m[2] + ")", Path: filepath.Join(gradle, "wrapper/dists", name)}
		u.versioned(&it, u.Gradle, m[1])
		add(it)
	}
	reVer := regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`)
	for _, name := range subdirs(filepath.Join(gradle, "caches")) {
		p := filepath.Join(gradle, "caches", name)
		switch {
		case reVer.MatchString(name):
			it := Item{Group: "Gradle", Label: "Caches of Gradle " + name, Path: p}
			u.versioned(&it, u.Gradle, name)
			add(it)
		case strings.HasPrefix(name, "build-cache-"):
			add(Item{Group: "Gradle", Label: "Build cache", Path: p, Status: Cache, Note: "Task outputs; the next build is slower"})
		case name == "modules-2":
			add(Item{Group: "Gradle", Label: "Downloaded dependencies", Path: p, Status: Cache, Note: "Downloaded again by the next build"})
		case strings.HasPrefix(name, "transforms-") || strings.HasPrefix(name, "jars-"):
			add(Item{Group: "Gradle", Label: "Shared " + name, Path: p, Status: Cache, Note: "Rebuilt by the next build"})
		}
	}
	dir("Gradle", "Daemon logs", ".gradle/daemon", Cache, "Logs of past Gradle daemons")

	// JavaScript
	dir("JavaScript", "npm cache", ".npm/_cacache", Cache, "npm downloads packages again on install")
	dir("JavaScript", "npx cache", ".npm/_npx", Cache, "Packages run with npx")
	dir("JavaScript", "Yarn cache", "Library/Caches/Yarn", Cache, "")
	dir("JavaScript", "pnpm store", "Library/pnpm/store", Cache, "")
	dir("JavaScript", "Bun cache", ".bun/install/cache", Cache, "")
	dir("JavaScript", "TypeScript cache", "Library/Caches/typescript", Cache, "Type definitions for editors")
	dir("JavaScript", "node-gyp headers", "Library/Caches/node-gyp", Cache, "")
	dir("JavaScript", "React Native cache", "Library/Caches/ReactNative", Cache, "Prebuilt React Native downloads")
	dir("JavaScript", "Playwright browsers", "Library/Caches/ms-playwright", Cache, "")
	dir("JavaScript", "Playwright (Go) browsers", "Library/Caches/ms-playwright-go", Cache, "")
	convexBinaries(home, add)

	// Apple
	dir("Xcode", "DerivedData", "Library/Developer/Xcode/DerivedData", Cache, "Xcode build output")
	dir("Xcode", "iOS DeviceSupport", "Library/Developer/Xcode/iOS DeviceSupport", Cache, "Copied again when a device connects")
	dir("Xcode", "CocoaPods cache", "Library/Caches/CocoaPods", Cache, "")
	dir("Xcode", "SwiftPM cache", "Library/Caches/org.swift.swiftpm", Cache, "")
	unavailableSimulators(home, add)

	// Go and the rest
	dir("Go", "Go build cache", "Library/Caches/go-build", Cache, "")
	dir("Go", "Go module cache", "go/pkg/mod", Cache, "Modules are downloaded again on build")
	docker(home, add)
	dir("Other", "Homebrew downloads", "Library/Caches/Homebrew", Cache, "")
	dir("Other", "dotslash cache", "Library/Caches/dotslash", Cache, "")

	sizeAll(items)
	sort.SliceStable(items, func(a, b int) bool { return items[a].Group < items[b].Group })
	return items
}

func sizeAll(items []Item) {
	sizer := fsx.NewSizer()
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range items {
		if items[i].Bytes > 0 || items[i].Path == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(it *Item) {
			defer func() { <-sem; wg.Done() }()
			it.Bytes = sizer.Size(it.Path).Bytes
		}(&items[i])
	}
	wg.Wait()
}

// convexBinaries: the local Convex backend downloads a new binary per
// release; only the newest is used by `convex dev`.
func convexBinaries(home string, add func(Item)) {
	base := filepath.Join(home, ".cache/convex/binaries")
	names := subdirs(base)
	sort.Strings(names) // precompiled-YYYY-MM-DD-...
	for i, n := range names {
		it := Item{Group: "JavaScript", Label: "Convex local backend " + strings.TrimPrefix(n, "precompiled-"), Path: filepath.Join(base, n)}
		if i == len(names)-1 {
			it.Status, it.Note = InUse, "Newest; used by convex dev"
		} else {
			it.Status, it.Note = Unused, "Older release; downloaded again if needed"
		}
		add(it)
	}
}

// unavailableSimulators are simulators whose iOS runtime is gone; they can
// never boot again. `xcrun simctl delete unavailable` removes them.
func unavailableSimulators(home string, add func(Item)) {
	out, err := exec.Command("xcrun", "simctl", "list", "devices", "unavailable", "-j").Output()
	if err != nil {
		return
	}
	var list struct {
		Devices map[string][]struct{ UDID string } `json:"devices"`
	}
	if json.Unmarshal(out, &list) != nil {
		return
	}
	sizer := fsx.NewSizer()
	var total int64
	n := 0
	for _, devs := range list.Devices {
		for _, d := range devs {
			total += sizer.Size(filepath.Join(home, "Library/Developer/CoreSimulator/Devices", d.UDID)).Bytes
			n++
		}
	}
	if n == 0 {
		return
	}
	add(Item{
		ID: "simctl:unavailable", Group: "Xcode", Label: "Unavailable simulators",
		// No Path: this item is only ever removed through Command.
		Bytes: total, Status: Unused,
		Note:    plural(n, "simulator") + " whose iOS runtime is no longer installed",
		Command: []string{"xcrun", "simctl", "delete", "unavailable"},
	})
}

// avdSystemImages maps "android-36" style folders to the emulators using them.
func avdSystemImages(home string) map[string][]string {
	out := map[string][]string{}
	avds, _ := filepath.Glob(filepath.Join(home, ".android/avd/*.avd"))
	for _, avd := range avds {
		b, err := os.ReadFile(filepath.Join(avd, "config.ini"))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), "image.sysdir.1="); ok {
				parts := strings.Split(filepath.ToSlash(v), "/")
				if len(parts) > 1 && parts[0] == "system-images" {
					name := strings.TrimSuffix(filepath.Base(avd), ".avd")
					out[parts[1]] = append(out[parts[1]], name)
				}
			}
		}
	}
	return out
}

func subdirs(p string) []string {
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out
}

func uniq(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
