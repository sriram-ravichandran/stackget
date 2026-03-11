package detector

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sriram-ravichandran/stackget/internal/schema"
)

// guiExecTimeout caps how long we spend trying to run a GUI binary for version
// output. Many GUI launchers open a window instead of printing to stdout, so we
// keep this very short and fall back gracefully.
const guiExecTimeout = 500 * time.Millisecond

// envVarProbeTimeout is used when running a CLI binary located via an env-var
// hint (e.g. $CATALINA_HOME/bin/catalina.sh version). These are real CLI tools,
// not GUI launchers, so a longer timeout is appropriate.
const envVarProbeTimeout = 3 * time.Second

// ─────────────────────────────────────────────────────────────────────────────
// Main entry point
// ─────────────────────────────────────────────────────────────────────────────

// discoverGUIApp is called when all Commands[] PATH lookups have failed.
// It tries discovery in priority order:
//  1. All    — native OS registry (Windows: Uninstall hives via PowerShell;
//              macOS: /Applications scan; Linux: XDG .desktop files).
//              Runs once via sync.Once; zero repeated OS calls.
//  1.5 All   — env-var probe (Deep Heuristics): if GUIApp.EnvVar is set,
//              check $EnvVar/bin/<binary> (e.g. $CATALINA_HOME, $HADOOP_HOME).
//              Uses a 3 s timeout — these are real CLI tools, not GUI launchers.
//  2. All    — OS path search (file walker) for portable/unregistered installs.
//  3. macOS  — PlistBuddy reads CFBundleShortVersionString from Info.plist
//  4. Linux  — run the binary with the first version arg (500 ms cap)
//  5. All    — parse a semver from the directory name in the resolved path
//  6. All    — return with empty Version; printer renders "installed"
func discoverGUIApp(def ToolDef, category string) schema.ToolResult {
	result := schema.ToolResult{Name: def.Name, Category: category}
	app := def.GUIApp
	if app == nil {
		return result
	}

	// ── Strategy 1: native OS registry ───────────────────────────────────────
	// Build aliases: on macOS pass the bundle name without ".app" so that
	// "MySQL Workbench.app" matches a lookup for "MySQL Workbench".
	//
	// Skip native lookup on macOS when no MacApp is defined — without a
	// specific .app target the fuzzy name match can produce false positives
	// (e.g. "Visual Studio" matching "Visual Studio Code").
	// Likewise skip on Linux when no LinuxBin is defined.
	skipNativeLookup := (runtime.GOOS == "darwin" && app.MacApp == "") ||
		(runtime.GOOS == "linux" && app.LinuxBin == "")
	var aliases []string
	if runtime.GOOS == "darwin" && app.MacApp != "" {
		aliases = append(aliases, strings.TrimSuffix(app.MacApp, ".app"))
	}
	if !skipNativeLookup {
		if info, found := lookupNativeApp(def.Name, aliases...); found {
			result.Installed = true
			if info.InstallPath != "" {
				result.Path = info.InstallPath
			}
			if def.NoVersion {
				return result
			}
			result.Version = info.Version
			// macOS: version intentionally not stored in cache (lazy PlistBuddy).
			if runtime.GOOS == "darwin" && result.Version == "" && info.InstallPath != "" {
				result.Version = versionFromMacApp(info.InstallPath)
			}
			// Linux: .desktop files carry no version field; probe the binary.
			if runtime.GOOS == "linux" && result.Version == "" && info.InstallPath != "" {
				argSets := buildArgSets(def.VersionArgs)
				if len(argSets) == 0 {
					argSets = [][]string{{"--version"}}
				}
				result.Version = tryGetVersion(info.InstallPath, argSets[0], def.VersionFilter, "", def.VersionRegex, guiExecTimeout)
			}
			// Windows: version comes from the registry DisplayVersion field.
			// If absent, fall back to directory-name parsing.
			if runtime.GOOS == "windows" && result.Version == "" && info.InstallPath != "" {
				result.Version = versionFromPath(info.InstallPath)
			}
			return result
		}
	}

	// ── Strategy 1.5: Environment variable probe (Deep Heuristics) ──────────
	// Handles tools like Apache Tomcat (CATALINA_HOME) and Hadoop (HADOOP_HOME)
	// that install to a custom directory pointed to by a well-known env var.
	// We probe $EnvVar/bin/<binary> before falling back to the file walker.
	if app.EnvVar != "" {
		if root := os.Getenv(app.EnvVar); root != "" {
			var candidate string
			switch runtime.GOOS {
			case "windows":
				if app.WinExe != "" {
					candidate = filepath.Join(root, "bin", app.WinExe)
				}
			default:
				if app.LinuxBin != "" {
					candidate = filepath.Join(root, "bin", app.LinuxBin)
				}
			}
			if candidate != "" && fileExists(candidate) {
				result.Path = candidate
				result.Installed = true
				if def.NoVersion {
					return result
				}
				argSets := buildArgSets(def.VersionArgs)
				if len(argSets) == 0 {
					argSets = [][]string{{"--version"}}
				}
				if v := tryGetVersion(candidate, argSets[0], def.VersionFilter, "", def.VersionRegex, envVarProbeTimeout); v != "" {
					result.Version = v
				}
				return result
			}
		}
	}

	// ── Strategy 2: OS path search (file walker fallback) ────────────────────
	var found string
	switch runtime.GOOS {
	case "windows":
		found = findWinApp(app.WinExe, app.WinHints)
	case "darwin":
		found = findMacApp(app.MacApp)
	case "linux":
		found = findLinuxApp(app.LinuxBin, app.LinuxHints)
	}
	if found == "" {
		return result
	}

	result.Path = found
	result.Installed = true
	if def.NoVersion {
		return result
	}

	switch runtime.GOOS {
	case "darwin":
		// Info.plist read is a tiny subprocess — typically < 5 ms.
		if v := versionFromMacApp(found); v != "" {
			result.Version = v
			return result
		}
	case "linux":
		// Try running the binary with the first version arg.
		argSets := buildArgSets(def.VersionArgs)
		if len(argSets) == 0 {
			argSets = [][]string{{"--version"}}
		}
		if v := tryGetVersion(found, argSets[0], def.VersionFilter, "", def.VersionRegex, guiExecTimeout); v != "" {
			result.Version = v
			return result
		}
	// Windows: skip exec entirely — .exe launchers open the GUI rather than
	// printing to stdout. versionFromPath handles installers that embed the
	// version in the directory name (e.g. "MySQL Workbench 8.0" → "8.0",
	// "DataGrip 2024.3" → "2024.3").
	}

	result.Version = versionFromPath(found)
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Windows discovery
// ─────────────────────────────────────────────────────────────────────────────

// findWinApp locates exeName on Windows without filepath.Glob.
//
// Motivation: filepath.Glob silently returns no matches on Windows paths whose
// non-wildcard prefix contains spaces (e.g. "C:\Program Files\*\*\foo.exe").
// Using os.ReadDir avoids this and is equally fast — it is just directory listing.
//
// Search strategy
// ──────────────────────────────────────────────────────────────────────────────
//
//	Standard roots (resolved at runtime from env vars — never hardcoded):
//	  %ProgramFiles%             depth-2 search:  root\<App>\exe
//	  %ProgramFiles(x86)%        depth-2 search:  root\<App>\exe
//	  %LOCALAPPDATA%\Programs    depth-2 search:  root\<App>\exe
//	    → covers all Electron per-user installs (Compass, Postman, Insomnia,
//	      GitHub Desktop, …) which install at exactly one directory deep.
//
//	WinHints (caller-supplied):
//	  Static dirs  e.g. "C:\Program Files\pgAdmin 4"
//	    depth-1: hint\exe             (hint IS the app dir, e.g. pgAdmin 4\runtime)
//	    depth-2: hint\<Sub>\exe       (hint is vendor dir, e.g. MySQL\Workbench 8.0\)
//	  Glob patterns  e.g. "C:\Program Files\JetBrains\DataGrip *\bin"
//	    expanded first via filepath.Glob (glob on a short vendor path with no
//	    spaces before the wildcard works reliably), then depth-1 and depth-2.
//
// Returns the last candidate collected (entries are read in alphabetical order,
// so the last match is the lexicographically newest — i.e. highest version).
func findWinApp(exeName string, hints []string) string {
	if exeName == "" {
		return ""
	}

	found := ""

	// ── Standard roots: depth-2 ──────────────────────────────────────────────
	for _, root := range winStandardRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if c := filepath.Join(root, e.Name(), exeName); fileExists(c) {
				found = c
			}
		}
	}

	// ── WinHints: depth-1 and depth-2 ────────────────────────────────────────
	// Hints may be glob patterns (e.g. "C:\PF\JetBrains\DataGrip *\bin").
	// filepath.Glob works correctly here because the wildcard comes AFTER the
	// vendor path — no spaces in the fixed prefix before the first *.
	for _, hint := range hints {
		for _, dir := range expandHint(hint) {
			// depth-1: exe lives directly inside this dir (e.g. pgAdmin 4\runtime\)
			if c := filepath.Join(dir, exeName); fileExists(c) {
				found = c
			}
			// depth-2: exe is one sub-directory deeper
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				if c := filepath.Join(dir, e.Name(), exeName); fileExists(c) {
					found = c
				}
			}
		}
	}

	return found
}

// winStandardRoots returns the standard Windows application install directories.
// Resolved at call time — never hardcoded — so correct on any localisation.
func winStandardRoots() []string {
	roots := make([]string, 0, 3)
	if d := os.Getenv("ProgramFiles"); d != "" {
		roots = append(roots, d)
	}
	// The env var really is named "ProgramFiles(x86)" — parentheses included.
	if d := os.Getenv("ProgramFiles(x86)"); d != "" {
		roots = append(roots, d)
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		roots = append(roots, filepath.Join(local, "Programs"))
	}
	return roots
}

// expandHint returns the set of directories matched by a path or glob pattern.
//   - Static paths  (no wildcard): returned as-is if the directory exists.
//   - Glob patterns (contain * or ?): expanded via filepath.Glob.
//     This is safe because well-formed hints have no spaces before the first *
//     (e.g. "C:\Program Files\JetBrains\DataGrip *\bin"), so filepath.Glob
//     resolves the fixed prefix correctly before applying the wildcard.
func expandHint(hint string) []string {
	if !strings.ContainsAny(hint, "*?") {
		if info, err := os.Stat(hint); err == nil && info.IsDir() {
			return []string{hint}
		}
		return nil
	}
	matches, _ := filepath.Glob(hint)
	return matches
}

// fileExists reports whether path refers to an existing regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ─────────────────────────────────────────────────────────────────────────────
// macOS discovery
// ─────────────────────────────────────────────────────────────────────────────

// findMacApp checks /Applications and ~/Applications for the named .app bundle.
// If not found there, falls back to mdfind (Spotlight) which locates .app bundles
// installed anywhere on the filesystem — Setapp, external drives, custom paths, etc.
func findMacApp(appBundleName string) string {
	if appBundleName == "" {
		return ""
	}
	home, _ := os.UserHomeDir()
	for _, dir := range []string{
		"/Applications",
		filepath.Join(home, "Applications"),
		"/Applications/Setapp", // Setapp installs here
	} {
		p := filepath.Join(dir, appBundleName)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	// Spotlight fallback: finds .app bundles installed anywhere on the system.
	return mdfindApp(appBundleName)
}

// mdfindApp uses macOS Spotlight (mdfind) to locate an .app bundle by its
// CFBundleName, regardless of where it is installed on the filesystem.
// Returns the first valid .app path found, or "" if not found / Spotlight disabled.
func mdfindApp(appBundleName string) string {
	name := strings.TrimSuffix(appBundleName, ".app")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "mdfind",
		"kMDItemCFBundleName == '"+name+"'",
	).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".app") {
			if st, err := os.Stat(line); err == nil && st.IsDir() {
				return line
			}
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Linux discovery
// ─────────────────────────────────────────────────────────────────────────────

// findLinuxApp searches for binName on Linux in PATH, standard non-PATH locations,
// and caller-supplied hints (which may be glob patterns).
func findLinuxApp(binName string, hints []string) string {
	if binName == "" {
		return ""
	}
	if p, err := exec.LookPath(binName); err == nil {
		return p
	}
	patterns := []string{
		filepath.Join("/opt", "*", binName),
		filepath.Join("/opt", "*", "bin", binName),
		filepath.Join("/usr", "share", "*", binName),
		filepath.Join("/usr", "share", "*", "bin", binName),
	}
	for _, hint := range hints {
		patterns = append(patterns, filepath.Join(hint, binName))
	}
	for _, pat := range patterns {
		if matches, _ := filepath.Glob(pat); len(matches) > 0 {
			return matches[len(matches)-1]
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Version extraction helpers
// ─────────────────────────────────────────────────────────────────────────────

// versionFromMacApp reads CFBundleShortVersionString from an .app bundle's
// Info.plist using /usr/libexec/PlistBuddy, which ships with every macOS.
func versionFromMacApp(appBundlePath string) string {
	plist := filepath.Join(appBundlePath, "Contents", "Info.plist")
	if _, err := os.Stat(plist); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), guiExecTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx,
		"/usr/libexec/PlistBuddy", "-c", "Print CFBundleShortVersionString", plist,
	).Output()
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	if semver3Regex.MatchString(v) || semver2Regex.MatchString(v) {
		return v
	}
	return ""
}

// versionFromPath extracts the first semver-like string embedded in a file path.
// Back-slashes are normalised to forward-slashes before matching.
//
// Examples:
//
//	"C:\Program Files\MySQL\MySQL Workbench 8.0\MySQLWorkbench.exe"  → "8.0"
//	"C:\Program Files\JetBrains\DataGrip 2024.3\bin\datagrip64.exe" → "2024.3"
//	"/opt/datagrip-2024.1/bin/datagrip.sh"                           → "2024.1"
//	"C:\Program Files\pgAdmin 4\runtime\pgadmin4.exe"               → ""
func versionFromPath(p string) string {
	norm := strings.ReplaceAll(p, `\`, "/")
	if m := semver3Regex.FindString(norm); m != "" {
		return cleanVersion(m)
	}
	if m := semver2Regex.FindString(norm); m != "" {
		return cleanVersion(m)
	}
	return ""
}
