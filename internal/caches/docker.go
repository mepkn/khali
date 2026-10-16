package caches

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"khali/internal/fsx"
)

// dockerBin finds the docker CLI; apps opened from Finder don't get the
// shell's PATH, so the usual install places are tried first.
func dockerBin(home string) string {
	for _, p := range []string{
		filepath.Join(home, ".orbstack/bin/docker"),
		"/usr/local/bin/docker",
		"/opt/homebrew/bin/docker",
	} {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	p, _ := exec.LookPath("docker")
	return p
}

// dfRow is one line of `docker system df --format '{{json .}}'`.
type dfRow struct {
	Type        string
	Reclaimable string // "1.2GB (50%)"
}

var dockerKinds = map[string]struct {
	id, label, status, note string
	prune                   []string
}{
	"Images":        {"images", "Unused images", Cache, "Images no container uses; pulled or built again when needed", []string{"image", "prune", "-a", "-f"}},
	"Containers":    {"containers", "Stopped containers", Unused, "Their own files are lost; images and volumes stay", []string{"container", "prune", "-f"}},
	"Build Cache":   {"builder", "Build cache", Cache, "The next docker build is slower", []string{"builder", "prune", "-a", "-f"}},
	"Local Volumes": {"volumes", "Unused volumes", Maybe, "Volumes no container uses; may hold database data you want", []string{"volume", "prune", "-a", "-f"}},
}

// docker lists what Docker can reclaim. When the engine isn't running (for
// OrbStack, the app is closed) the OrbStack disk is shown, locked.
func docker(home string, add func(Item)) {
	bin := dockerBin(home)
	if bin == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "system", "df", "--format", "{{json .}}").Output()
	if err != nil {
		orbStackDisk(home, add)
		return
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		var r dfRow
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		k, ok := dockerKinds[r.Type]
		n := parseDockerSize(r.Reclaimable)
		if !ok || n == 0 {
			continue
		}
		add(Item{
			ID: "docker:" + k.id, Group: "Docker", Label: k.label, Bytes: n, Status: k.status,
			Note:    k.note + ". Docker's disk shrinks over time, not straight away",
			Command: append([]string{bin}, k.prune...),
		})
	}
}

func orbStackDisk(home string, add func(Item)) {
	matches, _ := filepath.Glob(filepath.Join(home, "Library/Group Containers/*.dev.orbstack/data"))
	for _, p := range matches {
		add(Item{
			ID: "docker:orbstack", Group: "Docker", Label: "OrbStack disk", Bytes: fsx.NewSizer().Size(p).Bytes,
			Status: InUse, Note: "Docker isn't running. Open OrbStack and rescan to see what can be cleaned inside it",
		})
	}
}

var reDockerSize = regexp.MustCompile(`^([\d.]+)\s*([kKMGTP]?B)`)

// parseDockerSize reads Docker's decimal sizes ("1.2GB", "512kB", "0B").
func parseDockerSize(s string) int64 {
	m := reDockerSize.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	mult := map[string]float64{"B": 1, "kB": 1e3, "KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12, "PB": 1e15}[m[2]]
	return int64(v * mult)
}
