package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// A repo in the backend/web/mobile layout, plus an Expo-less repo whose
// android/ is committed.
func fixture(t *testing.T) string {
	root := t.TempDir()

	app := filepath.Join(root, "app")
	write(t, app, ".gitignore", "node_modules/\ndist/\n.expo/\n/mobile/android\n")
	write(t, app, "backend/package.json", `{"dependencies":{"convex":"1"}}`)
	write(t, app, "backend/node_modules/convex/index.js", "x")
	write(t, app, "backend/.convex/local/db.sqlite", "data")
	write(t, app, "backend/.env.local", "SECRET=1")
	write(t, app, "web/package.json", `{}`)
	write(t, app, "web/src/main.tsx", "x")
	write(t, app, "web/dist/index.html", "x")
	write(t, app, "web/build/notes.txt", "kept: plain build/ is never matched")
	write(t, app, "mobile/package.json", `{"dependencies":{"expo":"~57.0.0","react-native":"0.86.3"}}`)
	write(t, app, "mobile/app.json", `{}`)
	write(t, app, "mobile/.expo/cache", "x")
	write(t, app, "mobile/node_modules/react-native/package.json", `{"version":"0.86.3"}`)
	write(t, app, "mobile/node_modules/react-native/gradle/libs.versions.toml", "[versions]\nndkVersion = \"27.1.12297006\"\nbuildTools = \"36.0.0\"\ncompileSdk = \"36\"\n")
	write(t, app, "mobile/android/settings.gradle", "x")
	write(t, app, "mobile/android/gradle/wrapper/gradle-wrapper.properties", "distributionUrl=https\\://services.gradle.org/distributions/gradle-9.3.1-bin.zip\n")
	write(t, app, "mobile/android/app/build/out.apk", "x")
	write(t, app, "mobile/preview.apk", "x")
	git(t, app, "init", "-q")
	git(t, app, "add", ".")
	git(t, app, "commit", "-qm", "init")

	bare := filepath.Join(root, "bare")
	write(t, bare, ".gitignore", "node_modules/\nbuild/\n.gradle/\n")
	write(t, bare, "package.json", `{"dependencies":{"react-native":"0.77.3"}}`)
	write(t, bare, "android/settings.gradle", "x")
	write(t, bare, "android/app/build.gradle", "android { ndkVersion \"28.0.13004108\"\n compileSdkVersion 35 }\n")
	write(t, bare, "android/app/build/x", "x")
	write(t, bare, "android/.gradle/x", "x")
	git(t, bare, "init", "-q")
	git(t, bare, "add", ".")
	git(t, bare, "commit", "-qm", "init")
	// A Go backend: bin/ ignored, air's tmp/ not.
	write(t, bare, "server/go.mod", "module server\n")
	write(t, bare, "server/.air.toml", "x")
	write(t, bare, "server/bin/server", "x")
	write(t, bare, "server/tmp/main", "x")
	write(t, bare, ".gitignore", "node_modules/\nbuild/\n.gradle/\nbin/\n")
	// Ignored files that are committed anyway stay protected.
	write(t, bare, "android/.cxx/x", "x")
	return root
}

func TestScan(t *testing.T) {
	root := fixture(t)
	res := Run(Options{Roots: []string{root}, Versions: SeedVersions})
	if len(res.Projects) != 2 {
		t.Fatalf("want 2 projects, got %d", len(res.Projects))
	}
	byName := map[string]Project{}
	for _, p := range res.Projects {
		byName[p.Name] = p
	}

	got := map[string]bool{}
	for _, tg := range byName["app"].Targets {
		got[tg.Rel] = tg.Safe
	}
	want := map[string]bool{
		"backend/node_modules": true,
		"web/dist":             true,
		"mobile/.expo":         true,
		"mobile/node_modules":  true,
		"mobile/android":       true,  // whole generated folder, as one
		"mobile/preview.apk":   false, // committed, so git tracks it
	}
	if !equal(got, want) {
		t.Errorf("app targets:\n got %v\nwant %v", got, want)
	}

	// android/ is committed: only its ignored build folders are offered,
	// and .cxx (not ignored) is shown but locked.
	got = map[string]bool{}
	for _, tg := range byName["bare"].Targets {
		got[tg.Rel] = tg.Safe
	}
	want = map[string]bool{
		"node_modules":      false, // doesn't exist; must not appear
		"android/app/build": true,
		"android/.gradle":   true,
		"android/.cxx":      false,
		"server/bin":        true,
		"server/tmp":        false, // not ignored
	}
	delete(want, "node_modules")
	if !equal(got, want) {
		t.Errorf("bare targets:\n got %v\nwant %v", got, want)
	}

	apps := byName["app"].Android
	if len(apps) != 1 || !apps[0].Resolved || apps[0].Versions.NDK != "27.1.12297006" || apps[0].Versions.Gradle != "9.3.1" {
		t.Errorf("app android versions: %+v", apps)
	}
	bare := byName["bare"].Android
	if len(bare) != 1 || bare[0].Versions.NDK != "28.0.13004108" || bare[0].Versions.CompileSdk != "35" {
		t.Errorf("bare android versions: %+v", bare)
	}
	if res.Observed["0.86"].NDK != "27.1.12297006" {
		t.Errorf("observed: %+v", res.Observed)
	}
}

// Once android/ and node_modules are gone, versions come from the table.
func TestVersionsFromTable(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"dependencies":{"react-native":"~0.90.1"}}`)
	app, _, ok := readAndroidApp(dir, VersionTable{"0.90": {NDK: "29.0.1", BuildTools: "37.0.0", CompileSdk: "37", Gradle: "9.9"}})
	if !ok || !app.Resolved || app.Versions.NDK != "29.0.1" || app.RN != "0.90.1" {
		t.Errorf("got %+v", app)
	}
	app, _, _ = readAndroidApp(dir, VersionTable{})
	if app.Resolved {
		t.Errorf("unknown RN version must not resolve: %+v", app)
	}
}

func TestOutsideGit(t *testing.T) {
	root := t.TempDir()
	write(t, root, "site/package.json", `{}`)
	write(t, root, "site/node_modules/a/index.js", "x")
	write(t, root, "site/.convex/db", "x")
	write(t, root, "tool/go.mod", "module tool\n")
	write(t, root, "tool/bin/run.sh", "x")
	res := Run(Options{Roots: []string{root}})
	for _, p := range res.Projects {
		if p.Name == "tool" && (len(p.Targets) != 1 || p.Targets[0].Safe) {
			t.Errorf("go bin/ outside git must be locked: %+v", p.Targets)
		}
	}
	var rels []string
	for _, tg := range projectNamed(res, "site").Targets {
		rels = append(rels, tg.Rel)
		if !tg.Safe {
			t.Errorf("%s should be cleanable outside git", tg.Rel)
		}
	}
	sort.Strings(rels)
	if len(rels) != 1 || rels[0] != "node_modules" {
		t.Errorf("got %v", rels)
	}
}

func equal(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func projectNamed(res Result, name string) Project {
	for _, p := range res.Projects {
		if p.Name == name {
			return p
		}
	}
	return Project{}
}
