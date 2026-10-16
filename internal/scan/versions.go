package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AndroidVersions are the SDK and tool versions a React Native app builds with.
type AndroidVersions struct {
	NDK        string `json:"ndk"`
	BuildTools string `json:"buildTools"`
	CompileSdk string `json:"compileSdk"`
	Gradle     string `json:"gradle"`
}

func (v AndroidVersions) complete() bool {
	return v.NDK != "" && v.BuildTools != "" && v.CompileSdk != "" && v.Gradle != ""
}

// fill copies the fields v is missing from o.
func (v AndroidVersions) fill(o AndroidVersions) AndroidVersions {
	if v.NDK == "" {
		v.NDK = o.NDK
	}
	if v.BuildTools == "" {
		v.BuildTools = o.BuildTools
	}
	if v.CompileSdk == "" {
		v.CompileSdk = o.CompileSdk
	}
	if v.Gradle == "" {
		v.Gradle = o.Gradle
	}
	return v
}

// VersionTable maps a React Native minor version ("0.86") to what it uses.
// Once android/ and node_modules are cleaned there is nothing left to read,
// so versions seen in earlier scans are remembered here.
type VersionTable map[string]AndroidVersions

// SeedVersions are known defaults, used until a scan has seen the real files.
var SeedVersions = VersionTable{
	"0.86": {NDK: "27.1.12297006", BuildTools: "36.0.0", CompileSdk: "36", Gradle: "9.3.1"},
}

// AndroidApp is one React Native app and the versions it uses.
type AndroidApp struct {
	Project  string          `json:"project"`
	Rel      string          `json:"rel"`
	RN       string          `json:"rn"`
	Versions AndroidVersions `json:"versions"`
	// Resolved is false when some version could not be worked out; global
	// cleanup then treats every installed version of that tool as maybe used.
	Resolved bool `json:"resolved"`
}

var (
	reTOML        = regexp.MustCompile(`(?m)^\s*(ndkVersion|buildTools|compileSdk)\s*=\s*"([^"]+)"`)
	reNDK         = regexp.MustCompile(`ndkVersion\s*[=:]?\s*["']([0-9.]+)["']`)
	reBuildTools  = regexp.MustCompile(`buildToolsVersion\s*[=:]?\s*["']([0-9.]+)["']`)
	reCompileSdk  = regexp.MustCompile(`compileSdk(?:Version)?\s*[=:]?\s*(\d+)\b`)
	reGradleDist  = regexp.MustCompile(`gradle-([0-9][0-9A-Za-z.\-]*?)-(?:bin|all)\.zip`)
	reVersionNums = regexp.MustCompile(`\d+\.\d+`)
)

// readAndroidApp works out the versions of the React Native app in dir.
// observed holds only what was read from files, for the VersionTable.
func readAndroidApp(dir string, table VersionTable) (app AndroidApp, observed AndroidVersions, ok bool) {
	d, _, err := readDirInfo(dir)
	if err != nil || !d.hasDep("react-native") {
		return app, observed, false
	}
	app.RN = rnVersion(dir, d.pkgJSON)

	if b, err := os.ReadFile(filepath.Join(dir, "node_modules/react-native/gradle/libs.versions.toml")); err == nil {
		for _, m := range reTOML.FindAllStringSubmatch(string(b), -1) {
			switch m[1] {
			case "ndkVersion":
				observed.NDK = m[2]
			case "buildTools":
				observed.BuildTools = m[2]
			case "compileSdk":
				observed.CompileSdk = m[2]
			}
		}
	}

	android := filepath.Join(dir, "android")
	if b, err := os.ReadFile(filepath.Join(android, "gradle/wrapper/gradle-wrapper.properties")); err == nil {
		if m := reGradleDist.FindStringSubmatch(string(b)); m != nil {
			observed.Gradle = m[1]
		}
	}
	// Literal versions in the gradle files override React Native's defaults.
	for _, f := range []string{"build.gradle", "build.gradle.kts", "app/build.gradle", "app/build.gradle.kts"} {
		b, err := os.ReadFile(filepath.Join(android, f))
		if err != nil {
			continue
		}
		s := string(b)
		if m := reNDK.FindStringSubmatch(s); m != nil {
			observed.NDK = m[1]
		}
		if m := reBuildTools.FindStringSubmatch(s); m != nil {
			observed.BuildTools = m[1]
		}
		if m := reCompileSdk.FindStringSubmatch(s); m != nil {
			observed.CompileSdk = m[1]
		}
	}

	app.Versions = observed
	if known, ok := table[rnMinor(app.RN)]; ok {
		app.Versions = app.Versions.fill(known)
	}
	app.Resolved = app.Versions.complete()
	return app, observed, true
}

func rnVersion(dir, pkgJSON string) string {
	if b, err := os.ReadFile(filepath.Join(dir, "node_modules/react-native/package.json")); err == nil {
		var p struct{ Version string }
		if json.Unmarshal(b, &p) == nil && p.Version != "" {
			return p.Version
		}
	}
	var p struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	_ = json.Unmarshal([]byte(pkgJSON), &p)
	return strings.TrimLeft(p.Dependencies["react-native"], "^~>=v ")
}

func rnMinor(v string) string {
	return reVersionNums.FindString(v)
}
