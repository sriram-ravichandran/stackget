package detector

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// mvTimeout caps each subprocess call made during multi-version discovery.
const mvTimeout = 2 * time.Second

// DetectMultiVersions returns all installed versions of a supported tool,
// sorted ascending, with activeVersion included. Returns nil if fewer than
// 2 distinct versions are found (caller should not populate AllVersions).
func DetectMultiVersions(toolName, activeVersion string) []string {
	var versions []string
	switch toolName {
	case "Node.js":
		versions = detectNodeVersions(activeVersion)
	case "Python 3":
		versions = detectPythonVersions(activeVersion)
	case "Java":
		versions = detectJavaVersions(activeVersion)
	case "Go":
		versions = detectGoVersions(activeVersion)
	}
	if len(versions) < 2 {
		return nil
	}
	return versions
}

// ─────────────────────────────────────────────────────────────────────────────
// Node.js
// ─────────────────────────────────────────────────────────────────────────────

func detectNodeVersions(active string) []string {
	seen := make(map[string]bool)
	add := func(v string) {
		v = strings.TrimPrefix(strings.TrimSpace(v), "v")
		v = cleanVersion(v)
		if v != "" && semver3Regex.MatchString(v) {
			seen[v] = true
		}
	}
	if active != "" {
		add(active)
	}

	// Tier 1 — nvm / nvm-windows: "nvm list" or "nvm ls"
	for _, args := range [][]string{{"list"}, {"ls"}} {
		if out := mvRun("nvm", args...); out != "" && strings.Contains(out, ".") {
			for _, line := range strings.Split(out, "\n") {
				if m := semver3Regex.FindString(line); m != "" {
					add(m)
				}
			}
		}
	}

	// Tier 1 — fnm: "fnm list"
	if out := mvRun("fnm", "list"); out != "" {
		for _, line := range strings.Split(out, "\n") {
			if m := semver3Regex.FindString(line); m != "" {
				add(m)
			}
		}
	}

	// Tier 2 — nvm-windows dir: %NVM_HOME%, %APPDATA%\nvm, %LOCALAPPDATA%\nvm
	for _, base := range nvmWinDirs() {
		mvGlobVersionDirs(base, "v*", add)
	}

	// Tier 2 — nvm Unix: $NVM_DIR or ~/.nvm/versions/node
	nvmDir := os.Getenv("NVM_DIR")
	if nvmDir == "" {
		nvmDir = filepath.Join(mvHomeDir(), ".nvm")
	}
	mvGlobVersionDirs(filepath.Join(nvmDir, "versions", "node"), "v*", add)

	// Tier 2 — fnm dirs
	for _, d := range fnmNodeDirs() {
		mvGlobVersionDirs(d, "v*", add)
	}

	return buildVersionList(seen)
}

func nvmWinDirs() []string {
	var dirs []string
	if h := os.Getenv("NVM_HOME"); h != "" {
		dirs = append(dirs, h)
	}
	if a := os.Getenv("APPDATA"); a != "" {
		dirs = append(dirs, filepath.Join(a, "nvm"))
	}
	if l := os.Getenv("LOCALAPPDATA"); l != "" {
		dirs = append(dirs, filepath.Join(l, "nvm"))
	}
	return dirs
}

func fnmNodeDirs() []string {
	h := mvHomeDir()
	l := os.Getenv("LOCALAPPDATA")
	return []string{
		filepath.Join(h, ".local", "share", "fnm", "node-versions"),
		filepath.Join(l, "fnm", "node-versions"),
		filepath.Join(h, "AppData", "Roaming", "fnm", "node-versions"),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Python 3
// ─────────────────────────────────────────────────────────────────────────────

func detectPythonVersions(active string) []string {
	seen := make(map[string]bool)
	add := func(v string) {
		v = cleanVersion(strings.TrimSpace(v))
		if v != "" && (semver3Regex.MatchString(v) || semver2Regex.MatchString(v)) {
			seen[v] = true
		}
	}
	if active != "" {
		add(active)
	}

	// Tier 1 — pyenv / pyenv-win
	if out := mvRun("pyenv", "versions", "--bare"); out != "" {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if m := semver3Regex.FindString(line); m != "" {
				add(m)
			} else if m := semver2Regex.FindString(line); m != "" {
				add(m)
			}
		}
	}

	// Tier 2 — filesystem scan
	if runtime.GOOS == "windows" {
		// %LOCALAPPDATA%\Programs\Python\Python3*
		localApp := os.Getenv("LOCALAPPDATA")
		if localApp != "" {
			mvRunBinaryVersions(filepath.Join(localApp, "Programs", "Python"), "Python3*", "python.exe", "Python 3", add)
		}
		// C:\Python3* (legacy global installs)
		mvRunBinaryVersions(`C:\`, "Python3*", "python.exe", "Python 3", add)
	} else {
		// /usr/bin/python3.*, /usr/local/bin/python3.*, /opt/homebrew/bin/python3.*
		for _, dir := range []string{"/usr/bin", "/usr/local/bin", "/opt/homebrew/bin"} {
			entries, _ := filepath.Glob(filepath.Join(dir, "python3.*"))
			for _, e := range entries {
				if v := mvBinaryVersion(e, []string{"--version"}, "Python 3"); v != "" {
					add(v)
				}
			}
		}
		// ~/.pyenv/versions/ (filesystem mirror of pyenv)
		mvGlobBinaryVersions(filepath.Join(mvHomeDir(), ".pyenv", "versions"), "*", filepath.Join("bin", "python3"), "Python 3", add)
	}

	return buildVersionList(seen)
}

// ─────────────────────────────────────────────────────────────────────────────
// Java
// ─────────────────────────────────────────────────────────────────────────────

func detectJavaVersions(active string) []string {
	seen := make(map[string]bool)
	addJava := func(dir string) {
		// Try directory name first (fast, no subprocess).
		if v := javaVersionFromDir(filepath.Base(dir)); v != "" {
			seen[v] = true
			return
		}
		// Fallback: run java -version from this JDK's bin/.
		javaBin := filepath.Join(dir, "bin", "java")
		if runtime.GOOS == "windows" {
			javaBin += ".exe"
		}
		if _, err := os.Stat(javaBin); err == nil {
			if v := mvBinaryVersion(javaBin, []string{"-version"}, ""); v != "" {
				seen[v] = true
			}
		}
	}
	if active != "" {
		seen[active] = true
	}

	// Tier 1 — sdkman candidates dir (~/.sdkman/candidates/java/*)
	sdkDir := filepath.Join(mvHomeDir(), ".sdkman", "candidates", "java")
	if entries, err := os.ReadDir(sdkDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && e.Name() != "current" {
				addJava(filepath.Join(sdkDir, e.Name()))
			}
		}
	}

	// Tier 1 — jenv command: "jenv versions --bare"
	if out := mvRun("jenv", "versions", "--bare"); out != "" {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line == "system" {
				continue
			}
			if m := semver3Regex.FindString(line); m != "" {
				seen[m] = true
			} else if m := semver2Regex.FindString(line); m != "" {
				seen[m] = true
			}
		}
	}
	// Tier 1 — jenv filesystem: ~/.jenv/versions/* names are version strings
	// (e.g. "21.0.5", "11.0") — no subprocess needed.
	jenvDir := filepath.Join(mvHomeDir(), ".jenv", "versions")
	if entries, err := os.ReadDir(jenvDir); err == nil {
		for _, e := range entries {
			if v := javaVersionFromDir(e.Name()); v != "" {
				seen[v] = true
			}
		}
	}

	// Tier 2 — OS-specific install directories
	if runtime.GOOS == "windows" {
		for _, base := range []string{
			`C:\Program Files\Java`,
			`C:\Program Files\Eclipse Adoptium`,
			`C:\Program Files\Microsoft`,
			`C:\Program Files\Amazon Corretto`,
			`C:\Program Files\Zulu`,
			`C:\Program Files\BellSoft`,
			`C:\Program Files\OpenJDK`,
			filepath.Join(mvHomeDir(), ".jdks"),
		} {
			if entries, err := os.ReadDir(base); err == nil {
				for _, e := range entries {
					if e.IsDir() {
						addJava(filepath.Join(base, e.Name()))
					}
				}
			}
		}
	} else {
		for _, base := range []string{
			"/usr/lib/jvm",
			"/usr/local/lib/jvm",
			filepath.Join(mvHomeDir(), ".jdks"),
		} {
			if entries, err := os.ReadDir(base); err == nil {
				for _, e := range entries {
					if e.IsDir() {
						addJava(filepath.Join(base, e.Name()))
					}
				}
			}
		}
		// macOS: /Library/Java/JavaVirtualMachines/*.jdk/Contents/Home
		if macEntries, err := os.ReadDir("/Library/Java/JavaVirtualMachines"); err == nil {
			for _, e := range macEntries {
				if e.IsDir() {
					addJava(filepath.Join("/Library/Java/JavaVirtualMachines", e.Name(), "Contents", "Home"))
				}
			}
		}
	}

	return buildVersionList(seen)
}

// javaVersionFromDir extracts a version string from common JDK directory names.
// Examples: "jdk-21.0.5" → "21.0.5", "jdk1.8.0_401" → "1.8.0",
//
//	"temurin-21.0.5+11" → "21.0.5", "corretto-21.0.5.11.1" → "21.0.5"
func javaVersionFromDir(name string) string {
	lower := strings.ToLower(name)
	// Strip vendor prefixes (longest first to avoid partial matches).
	for _, pfx := range []string{
		"temurin-", "corretto-", "liberica-", "semeru-", "microsoft-", "zulu-",
		"bellsoft-liberica-", "openjdk-", "java-", "jdk-", "jdk",
	} {
		if strings.HasPrefix(lower, pfx) {
			name = name[len(pfx):]
			lower = strings.ToLower(name)
			break
		}
	}
	// Strip architecture/OS suffix after first non-version separator.
	// e.g. "21.0.5+9-amd64" → "21.0.5", "21-lts" → "21"
	for i, c := range name {
		if c == '+' || (c == '-' && i > 0) {
			name = name[:i]
			break
		}
	}
	// "1.8.0_401" → strip _update suffix
	if i := strings.Index(name, "_"); i > 0 {
		name = name[:i]
	}

	if m := semver3Regex.FindString(name); m != "" {
		return cleanVersion(m)
	}
	if m := semver2Regex.FindString(name); m != "" {
		return cleanVersion(m)
	}
	// Plain major version number like "21"
	if len(name) > 0 && name[0] >= '1' && name[0] <= '9' {
		if _, err := strconv.Atoi(name); err == nil {
			return name
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Go
// ─────────────────────────────────────────────────────────────────────────────

func detectGoVersions(active string) []string {
	seen := make(map[string]bool)
	add := func(v string) {
		v = cleanVersion(strings.TrimPrefix(strings.TrimSpace(v), "go"))
		if v != "" && semver3Regex.MatchString(v) {
			seen[v] = true
		}
	}
	if active != "" {
		add(active)
	}

	// Tier 1 — goenv
	if out := mvRun("goenv", "versions", "--bare"); out != "" {
		for _, line := range strings.Split(out, "\n") {
			if m := semver3Regex.FindString(line); m != "" {
				add(m)
			}
		}
	}

	// Tier 2 — ~/sdk/go* (populated by `go install golang.org/dl/goX.Y.Z@latest`)
	sdkDir := filepath.Join(mvHomeDir(), "sdk")
	if entries, err := filepath.Glob(filepath.Join(sdkDir, "go*")); err == nil {
		for _, e := range entries {
			add(filepath.Base(e)) // "go1.21.0" → strips "go" prefix inside add()
		}
	}

	// Tier 2 — Windows: C:\Program Files\Go (check for version via binary)
	if runtime.GOOS == "windows" {
		for _, pat := range []string{`C:\Go*`, `C:\Program Files\Go*`} {
			if matches, err := filepath.Glob(pat); err == nil {
				for _, m := range matches {
					goBin := filepath.Join(m, "bin", "go.exe")
					if _, err := os.Stat(goBin); err == nil {
						if v := mvBinaryVersion(goBin, []string{"version"}, ""); v != "" {
							add(v)
						}
					}
				}
			}
		}
	}

	return buildVersionList(seen)
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared helpers
// ─────────────────────────────────────────────────────────────────────────────

// mvRun executes a command with mvTimeout and returns combined output.
func mvRun(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), mvTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// mvBinaryVersion runs an absolute binary path with args and extracts a version.
func mvBinaryVersion(binaryPath string, args []string, filter string) string {
	return tryGetVersion(binaryPath, args, filter, "", "", mvTimeout)
}

// mvHomeDir returns the user's home directory (safe on Windows and Unix).
func mvHomeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// mvGlobVersionDirs globs pattern inside base, strips leading "v", and calls add.
func mvGlobVersionDirs(base, pattern string, add func(string)) {
	if base == "" {
		return
	}
	matches, err := filepath.Glob(filepath.Join(base, pattern))
	if err != nil {
		return
	}
	for _, m := range matches {
		name := strings.TrimPrefix(filepath.Base(m), "v")
		if v := semver3Regex.FindString(name); v != "" {
			add(v)
		}
	}
}

// mvRunBinaryVersions globs dirPattern inside base, finds binary inside each dir,
// and runs it to extract a version.
func mvRunBinaryVersions(base, dirPattern, binary, filter string, add func(string)) {
	dirs, err := filepath.Glob(filepath.Join(base, dirPattern))
	if err != nil {
		return
	}
	for _, d := range dirs {
		bin := filepath.Join(d, binary)
		if _, err := os.Stat(bin); err == nil {
			if v := mvBinaryVersion(bin, []string{"--version"}, filter); v != "" {
				add(v)
			}
		}
	}
}

// mvGlobBinaryVersions scans subdirs of base, appends binSuffix to each, and runs it.
func mvGlobBinaryVersions(base, dirPattern, binSuffix, filter string, add func(string)) {
	dirs, err := filepath.Glob(filepath.Join(base, dirPattern))
	if err != nil {
		return
	}
	for _, d := range dirs {
		bin := filepath.Join(d, binSuffix)
		if _, err := os.Stat(bin); err == nil {
			if v := mvBinaryVersion(bin, []string{"--version"}, filter); v != "" {
				add(v)
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Version sorting and deduplication
// ─────────────────────────────────────────────────────────────────────────────

// buildVersionList deduplicates and returns a sorted (ascending) version slice.
// Major-only versions (e.g. "21") are dropped when a full semver with the same
// major already exists (e.g. "21.0.5"), preventing redundant display.
// Returns nil if fewer than 2 distinct versions remain.
func buildVersionList(seen map[string]bool) []string {
	// Remove major-only entries that are superseded by a full semver.
	for v := range seen {
		if !strings.Contains(v, ".") {
			for other := range seen {
				if strings.HasPrefix(other, v+".") {
					delete(seen, v)
					break
				}
			}
		}
	}
	if len(seen) < 2 {
		return nil
	}
	vs := make([]string, 0, len(seen))
	for v := range seen {
		if v != "" {
			vs = append(vs, v)
		}
	}
	sort.Slice(vs, func(i, j int) bool {
		return mvCmpVersions(vs[i], vs[j]) < 0
	})
	return vs
}

// mvCmpVersions compares two version strings numerically component by component.
// Returns negative if a < b, positive if a > b, 0 if equal.
func mvCmpVersions(a, b string) int {
	pa, pb := mvSplitVersion(a), mvSplitVersion(b)
	for k := 0; k < len(pa) && k < len(pb); k++ {
		if pa[k] != pb[k] {
			if pa[k] < pb[k] {
				return -1
			}
			return 1
		}
	}
	if len(pa) < len(pb) {
		return -1
	}
	if len(pa) > len(pb) {
		return 1
	}
	return 0
}

// mvSplitVersion splits a version string into integer components.
// Non-numeric suffixes on components (e.g. "1p1") are stripped.
func mvSplitVersion(v string) []int {
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		end := 0
		for end < len(p) && p[end] >= '0' && p[end] <= '9' {
			end++
		}
		n, _ := strconv.Atoi(p[:end])
		nums = append(nums, n)
	}
	return nums
}
