package scan

import (
	"os"
	"path/filepath"
	"strings"
)

// Kinds of cleanable folders, also used as filter groups in the UI.
const (
	KindDeps     = "deps"     // node_modules
	KindBuild    = "build"    // dist, .expo, .next ...
	KindNative   = "native"   // mobile/android, ios, or their build folders
	KindArtifact = "artifact" // .apk / .aab / .ipa files
)

// buildDirs are output or cache folders next to a package.json. Plain
// "build" is left out on purpose: too many projects keep sources in it.
var buildDirs = map[string]string{
	"dist":             "Build output",
	"dist-ssr":         "Build output",
	".expo":            "Expo cache",
	".next":            "Next.js build",
	".nuxt":            "Nuxt build",
	".output":          "Build output",
	".svelte-kit":      "SvelteKit build",
	".turbo":           "Turborepo cache",
	".parcel-cache":    "Parcel cache",
	".vite":            "Vite cache",
	"coverage":         "Test coverage",
	"storybook-static": "Storybook build",
}

var artifactExts = map[string]bool{".apk": true, ".aab": true, ".ipa": true}

// androidBuildDirs are what Gradle regenerates inside an android/ project.
var androidBuildDirs = []string{"build", "app/build", ".gradle", ".cxx", "app/.cxx", ".kotlin"}

var iosBuildDirs = []string{"Pods", "build"}

// candidate is a folder or file the rules matched. When fallback is set and
// the candidate itself turns out unsafe (tracked by git), the fallback
// targets are offered instead.
type candidate struct {
	Target
	fallback []candidate
	// needsGit: only safe when git confirms it is ignored (a Go bin/ may
	// hold hand-written scripts outside a repo).
	needsGit bool
}

// dirInfo holds what the rules need to know about one folder.
type dirInfo struct {
	path    string
	names   map[string]bool
	pkgJSON string // contents of package.json, "" if none
}

func readDirInfo(path string) (dirInfo, []os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return dirInfo{}, nil, err
	}
	d := dirInfo{path: path, names: make(map[string]bool, len(entries))}
	for _, e := range entries {
		d.names[e.Name()] = true
	}
	if d.names["package.json"] {
		b, _ := os.ReadFile(filepath.Join(path, "package.json"))
		d.pkgJSON = string(b)
	}
	return d, entries, nil
}

func (d dirInfo) hasPkg() bool { return d.names["package.json"] }

func (d dirInfo) hasDep(name string) bool {
	return strings.Contains(d.pkgJSON, `"`+name+`"`)
}

// isExpo: an Expo app whose native folders come from `expo prebuild`.
func (d dirInfo) isExpo() bool {
	return d.hasPkg() && d.hasDep("expo") &&
		(d.names["app.json"] || d.names["app.config.ts"] || d.names["app.config.js"])
}

func (d dirInfo) installCmd() string {
	switch {
	case d.names["pnpm-lock.yaml"]:
		return "pnpm install"
	case d.names["yarn.lock"]:
		return "yarn install"
	case d.names["bun.lock"], d.names["bun.lockb"]:
		return "bun install"
	}
	return "npm install"
}

// match applies the rules to one entry of folder d. It returns the
// candidates found and whether the walk should descend into the entry.
func match(d dirInfo, e os.DirEntry) ([]candidate, bool) {
	name := e.Name()
	path := filepath.Join(d.path, name)

	if !e.IsDir() {
		if e.Type().IsRegular() && artifactExts[strings.ToLower(filepath.Ext(name))] {
			return []candidate{{Target: Target{Path: path, Kind: KindArtifact, Label: "Built app file", Regen: "build again"}}}, false
		}
		return nil, false
	}
	if e.Type()&os.ModeSymlink != 0 {
		return nil, false
	}

	switch {
	case name == ".git":
		return nil, false
	case name == "node_modules":
		if !d.hasPkg() {
			return nil, false
		}
		return []candidate{{Target: Target{Path: path, Kind: KindDeps, Label: "Dependencies", Regen: d.installCmd()}}}, false
	case d.names["go.mod"] && name == "bin":
		return []candidate{{Target: Target{Path: path, Kind: KindBuild, Label: "Go binaries", Regen: "go build"}, needsGit: true}}, false
	case d.names["go.mod"] && name == "tmp" && (d.names[".air.toml"] || d.names["air.toml"]):
		return []candidate{{Target: Target{Path: path, Kind: KindBuild, Label: "air build output", Regen: "recreated by air"}, needsGit: true}}, false
	case d.hasPkg() && buildDirs[name] != "":
		return []candidate{{Target: Target{Path: path, Kind: KindBuild, Label: buildDirs[name], Regen: regenFor(name)}}}, false
	case name == "android" && isAndroidProject(path):
		return nativeCandidates(d, path, "android", androidBuildDirs, "Gradle build output", "Gradle rebuilds it"), false
	case name == "ios" && isIOSProject(path):
		return nativeCandidates(d, path, "ios", iosBuildDirs, "CocoaPods and build output", "pod install / Xcode rebuild"), false
	case strings.HasPrefix(name, "."):
		// Hidden folders are never touched (.convex is a local database,
		// .eas-token sits next to them); only the names above are matched.
		return nil, false
	}
	return nil, true
}

func regenFor(name string) string {
	switch name {
	case ".expo", ".vite", ".turbo", ".parcel-cache":
		return "recreated automatically"
	case "coverage":
		return "npm test -- --coverage"
	}
	return "npm run build"
}

// nativeCandidates: in an Expo app the whole folder is generated by
// `expo prebuild`, so it goes as one; its build folders are the fallback when
// git tracks the folder. Outside Expo only the build folders are offered.
func nativeCandidates(d dirInfo, path, platform string, buildSubdirs []string, subLabel, subRegen string) []candidate {
	var subs []candidate
	for _, sub := range buildSubdirs {
		p := filepath.Join(path, sub)
		if isRealDir(p) {
			subs = append(subs, candidate{Target: Target{Path: p, Kind: KindNative, Label: subLabel, Regen: subRegen}})
		}
	}
	if !d.isExpo() {
		return subs
	}
	whole := candidate{
		Target: Target{
			Path:  path,
			Kind:  KindNative,
			Label: "Generated " + platform + " project",
			Regen: "npx expo prebuild -p " + platform,
		},
		fallback: subs,
	}
	return []candidate{whole}
}

func isAndroidProject(path string) bool {
	for _, f := range []string{"settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts", "gradlew"} {
		if exists(filepath.Join(path, f)) {
			return true
		}
	}
	return false
}

func isIOSProject(path string) bool {
	if exists(filepath.Join(path, "Podfile")) {
		return true
	}
	matches, _ := filepath.Glob(filepath.Join(path, "*.xcodeproj"))
	return len(matches) > 0
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func isRealDir(p string) bool {
	info, err := os.Lstat(p)
	return err == nil && info.IsDir()
}
