package detector

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// guiAppInfo holds OS-reported metadata for an installed GUI application.
// Populated from the native app registry; never guessed from file paths.
type guiAppInfo struct {
	DisplayName string
	Version     string // from registry DisplayVersion / plist / desktop file
	InstallPath string // directory or .app bundle path
}

var (
	nativeRegistryOnce  sync.Once
	nativeRegistryCache map[string]guiAppInfo // key: strings.ToLower(DisplayName)
)

// nativeScanTimeout caps the single OS-inventory subprocess call.
// PowerShell registry query: ~1-3 s. macOS directory stat: <100 ms. Linux: <50 ms.
const nativeScanTimeout = 15 * time.Second

// ─────────────────────────────────────────────────────────────────────────────
// Public lookup API
// ─────────────────────────────────────────────────────────────────────────────

// lookupNativeApp queries the OS native app registry for a tool.
// The registry is populated exactly once via sync.Once and shared across all
// concurrent tool-detection goroutines — zero repeated OS calls.
//
// Matching is two-pass:
//
//	Pass 1 — case-insensitive exact match of toolName or any alias.
//	Pass 2 — toolName/alias is a substring of the registry entry (≥ 6 chars).
//	          Handles versioned registry names: "MySQL Workbench" matches
//	          "MySQL Workbench 8.0 CE"; "DataGrip" matches "DataGrip 2024.3".
func lookupNativeApp(toolName string, aliases ...string) (guiAppInfo, bool) {
	nativeRegistryOnce.Do(func() {
		nativeRegistryCache = scanNativeApps()
	})

	candidates := make([]string, 0, 1+len(aliases))
	if s := strings.ToLower(strings.TrimSpace(toolName)); s != "" {
		candidates = append(candidates, s)
	}
	for _, a := range aliases {
		if s := strings.ToLower(strings.TrimSpace(a)); s != "" {
			candidates = append(candidates, s)
		}
	}

	// Pass 1 — exact match
	for _, c := range candidates {
		if info, ok := nativeRegistryCache[c]; ok {
			return info, true
		}
	}

	// Pass 2 — our candidate is a substring of the registry entry name.
	// Minimum 6 chars to prevent short names ("git") from matching unrelated
	// entries ("GitHub Desktop"), while allowing "DBeaver" to match
	// "DBeaver Community" and "datagrip" to match "DataGrip 2024.3".
	for _, c := range candidates {
		if len(c) < 6 {
			continue
		}
		for key, info := range nativeRegistryCache {
			if strings.Contains(key, c) {
				return info, true
			}
		}
	}

	return guiAppInfo{}, false
}

// ─────────────────────────────────────────────────────────────────────────────
// OS dispatcher
// ─────────────────────────────────────────────────────────────────────────────

func scanNativeApps() map[string]guiAppInfo {
	switch runtime.GOOS {
	case "windows":
		return scanWindowsRegistry()
	case "darwin":
		return scanDarwinApps()
	case "linux":
		return scanLinuxDesktopFiles()
	}
	return make(map[string]guiAppInfo)
}

// ─────────────────────────────────────────────────────────────────────────────
// Windows: PowerShell registry query
// ─────────────────────────────────────────────────────────────────────────────

// scanWindowsRegistry reads all three Windows Uninstall registry hives in a
// single PowerShell invocation and returns every installed application keyed by
// lower-cased DisplayName.
//
// Hives queried (covers 100% of normally installed apps):
//
//	HKLM\...\Uninstall              — 64-bit machine-wide installs
//	HKLM\...\WOW6432Node\Uninstall  — 32-bit machine-wide installs
//	HKCU\...\Uninstall              — per-user installs (Electron apps: MongoDB
//	                                   Compass, Postman, Insomnia, GitHub Desktop …)
//
// PowerShell startup cost (~1-3 s) is amortised across all GUI tool lookups
// via sync.Once and runs concurrently with the CLI path-check goroutines.
//
// The @() wrapper forces ConvertTo-Json to always emit a JSON array, even when
// exactly one entry matches (PowerShell 5 emits a bare object otherwise).
func scanWindowsRegistry() map[string]guiAppInfo {
	const script = `` +
		`$p=@(` +
		`'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',` +
		`'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',` +
		`'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*'` +
		`);` +
		`$a=@();` +
		`foreach($x in $p){if(Test-Path $x){$a+=Get-ItemProperty $x}};` +
		`@($a|Where-Object{$_.DisplayName}|` +
		`Select-Object DisplayName,DisplayVersion,InstallLocation)|` +
		`ConvertTo-Json -Compress`

	ctx, cancel := context.WithTimeout(context.Background(), nativeScanTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx,
		"powershell", "-NoProfile", "-NonInteractive", "-Command", script,
	).Output()
	if err != nil || len(out) == 0 {
		return make(map[string]guiAppInfo)
	}

	type winEntry struct {
		Name     string `json:"DisplayName"`
		Version  string `json:"DisplayVersion"`
		InstPath string `json:"InstallLocation"`
	}

	// Unmarshal as array; fall back to single-object if the @() wrapper was
	// somehow stripped (defensive — should not happen in practice).
	var entries []winEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		var single winEntry
		if json.Unmarshal(out, &single) == nil && single.Name != "" {
			entries = []winEntry{single}
		} else {
			return make(map[string]guiAppInfo)
		}
	}

	result := make(map[string]guiAppInfo, len(entries))
	for _, e := range entries {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		// HKLM entries appear before HKCU in the PowerShell output;
		// keep first occurrence so machine-wide versions take priority.
		if _, exists := result[key]; exists {
			continue
		}
		result[key] = guiAppInfo{
			DisplayName: name,
			Version:     cleanVersion(strings.TrimSpace(e.Version)),
			InstallPath: strings.TrimSpace(e.InstPath),
		}
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// macOS: /Applications directory scan (fast, no system_profiler subprocess)
// ─────────────────────────────────────────────────────────────────────────────

// scanDarwinApps lists /Applications and ~/Applications and records every .app
// bundle by its display name (bundle name with ".app" stripped).
//
// Version is intentionally NOT read here — calling PlistBuddy for every app
// in /Applications would cost ~100 × 10 ms = 1 s upfront. Instead, versions
// are fetched lazily by discoverGUIApp only for tools we actually detect,
// keeping the one-time scan cost to a single ReadDir call (<1 ms).
//
// Fuzzy matching allows "MongoDB Compass" to match the "MongoDB Compass.app"
// bundle and "MySQLWorkbench" to match a lookup for "MySQL Workbench" via the
// MacApp alias passed by the caller.
func scanDarwinApps() map[string]guiAppInfo {
	result := make(map[string]guiAppInfo, 128)
	home, _ := os.UserHomeDir()
	for _, dir := range []string{"/Applications", filepath.Join(home, "Applications")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			displayName := strings.TrimSuffix(e.Name(), ".app")
			key := strings.ToLower(displayName)
			if _, exists := result[key]; !exists {
				result[key] = guiAppInfo{
					DisplayName: displayName,
					InstallPath: filepath.Join(dir, e.Name()),
					// Version populated lazily in discoverGUIApp via versionFromMacApp.
				}
			}
		}
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Linux: XDG .desktop file scan
// ─────────────────────────────────────────────────────────────────────────────

// scanLinuxDesktopFiles parses XDG .desktop files from standard application
// directories. The Name= field is the display name; Exec= provides the binary
// path used for version probing.
//
// Directories scanned (in order; first occurrence of each name wins):
//
//	/usr/share/applications               — system-wide (apt, rpm, pacman)
//	/usr/local/share/applications         — manually installed
//	~/.local/share/applications           — per-user (AppImage, snap, user installs)
//	/var/lib/flatpak/…/applications       — Flatpak system
//	~/.local/share/flatpak/…/applications — Flatpak user
func scanLinuxDesktopFiles() map[string]guiAppInfo {
	result := make(map[string]guiAppInfo, 64)
	home, _ := os.UserHomeDir()
	dirs := []string{
		"/usr/share/applications",
		"/usr/local/share/applications",
		filepath.Join(home, ".local", "share", "applications"),
		"/var/lib/flatpak/exports/share/applications",
		filepath.Join(home, ".local", "share", "flatpak", "exports", "share", "applications"),
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			name, execPath := parseDesktopFile(filepath.Join(dir, e.Name()))
			if name == "" {
				continue
			}
			key := strings.ToLower(name)
			if _, exists := result[key]; !exists {
				result[key] = guiAppInfo{DisplayName: name, InstallPath: execPath}
			}
		}
	}
	return result
}

// parseDesktopFile extracts Name= and Exec= from the [Desktop Entry] section
// of an XDG .desktop file. Both fields are read in a single pass.
// %U and other field codes are stripped from the Exec= value.
func parseDesktopFile(path string) (name, execPath string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	inEntry := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "[Desktop Entry]":
			inEntry = true
		case strings.HasPrefix(line, "["):
			inEntry = false
		case inEntry && name == "" && strings.HasPrefix(line, "Name="):
			name = strings.TrimPrefix(line, "Name=")
		case inEntry && execPath == "" && strings.HasPrefix(line, "Exec="):
			exec := strings.TrimPrefix(line, "Exec=")
			if parts := strings.Fields(exec); len(parts) > 0 {
				execPath = parts[0] // drop %U, %F, and other field codes
			}
		}
		if name != "" && execPath != "" {
			break // both fields found; stop reading
		}
	}
	return
}
