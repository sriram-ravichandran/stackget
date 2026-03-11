package detector

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sriram-ravichandran/stackget/internal/hardware"
	"github.com/sriram-ravichandran/stackget/internal/schema"
)

// semver3Regex matches a strict 3-component version (MAJOR.MINOR.PATCH).
// Tried first — avoids matching 2-component timestamps.
var semver3Regex = regexp.MustCompile(`\d+\.\d+\.\d+`)

// semver2Regex matches a 2-component version where the MAJOR is not zero-prefixed
// (e.g. "1.21" but NOT "07.109" which is a timestamp fragment).
var semver2Regex = regexp.MustCompile(`[1-9]\d*\.\d+`)

// defaultVersionArgs are tried in order when a ToolDef has no VersionArgs.
// defaultVersionArgs are tried in order for tools with no explicit VersionArgs.
// "version" and "-version" are omitted — any tool that needs them has explicit
// VersionArgs already (e.g. go, kubectl, kafka-topics).  Keeping the list short
// reduces the number of subprocess round-trips per installed tool.
var defaultVersionArgs = []string{
	"--version",
	"-v",
	"-V",
}

const (
	// maxConcurrency caps simultaneous subprocess invocations.
	// 32 (down from 64) halves the Windows process-creation storm so that
	// Defender/AV scanning doesn't delay individual tools beyond their timeout.
	maxConcurrency = 32
)

// cmdTimeout is the per-subprocess deadline for tools with no explicit Timeout.
// Windows needs 5 s to absorb AV/Defender scanning latency when ~32 processes
// start concurrently.  macOS and Linux have no such contention so 3 s is ample.
var cmdTimeout = func() time.Duration {
	if runtime.GOOS == "windows" {
		return 5 * time.Second
	}
	return 3 * time.Second
}()

type toolWork struct {
	catIdx  int
	toolIdx int
	def     ToolDef
	catName string
}

type indexedResult struct {
	catIdx  int
	toolIdx int
	result  schema.ToolResult
}

// DetectAll runs all tool detections concurrently and returns a full ScanResult.
func DetectAll() *schema.ScanResult {
	start := time.Now()

	// Start the native OS registry pre-warm immediately — it runs for the
	// entire scan (PowerShell ~13 s on Windows) so we want the clock ticking
	// as early as possible.
	go func() {
		nativeRegistryOnce.Do(func() { nativeRegistryCache = scanNativeApps() })
	}()

	// Collect hardware BEFORE spawning tool goroutines.
	//
	// On Windows, hardware.Collect() launches three concurrent wmic processes
	// (~1 s total).  If those processes overlap with the 64 tool goroutines
	// also spawning subprocesses, Windows process-creation contention causes
	// slow Python-based tools (Alembic, Uvicorn, Hugging Face CLI, …) to
	// exceed their 2 s cmdTimeout and report "version unknown".
	//
	// Running hardware collection here adds ≈ 0 s to the total scan time
	// because the registry pre-warm (the real bottleneck at ~13 s) is already
	// running in the background — the 1 s wmic window is fully hidden.
	hw := hardware.Collect()

	// Apply overlay (if any) on top of the built-in categories.
	effectiveCategories := AllCategories
	if overlay, err := LoadOverlay(); err == nil && len(overlay) > 0 {
		effectiveCategories = MergeCategories(AllCategories, overlay)
	}

	var works []toolWork
	for catIdx, cat := range effectiveCategories {
		for toolIdx, tool := range cat.Tools {
			works = append(works, toolWork{
				catIdx:  catIdx,
				toolIdx: toolIdx,
				def:     tool,
				catName: cat.Name,
			})
		}
	}

	results := make(chan indexedResult, len(works))
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for _, w := range works {
		wg.Add(1)
		go func(work toolWork) {
			defer wg.Done()
			// GUIApp-only tools (no CLI commands) never spawn subprocesses —
			// they only do registry/plist/desktop-file lookups.  Skip the
			// subprocess semaphore so they don't starve real CLI detections.
			if len(work.def.Commands) > 0 {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			r := detectTool(work.def, work.catName)
			results <- indexedResult{catIdx: work.catIdx, toolIdx: work.toolIdx, result: r}
		}(w)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	catResults := make([]schema.CategoryResult, len(effectiveCategories))
	for i, cat := range effectiveCategories {
		catResults[i] = schema.CategoryResult{
			Name:  cat.Name,
			Emoji: cat.Emoji,
			Tools: make([]schema.ToolResult, len(cat.Tools)),
			Total: len(cat.Tools),
		}
	}

	totalInstalled := 0
	totalTools := 0

	for r := range results {
		catResults[r.catIdx].Tools[r.toolIdx] = r.result
		totalTools++
		if r.result.Installed {
			catResults[r.catIdx].Installed++
			totalInstalled++
		}
	}

	hostname, _ := os.Hostname()

	return &schema.ScanResult{
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		Hostname:       hostname,
		Timestamp:      time.Now(),
		Hardware: schema.HardwareInfo{
			OSName:   hw.OSName,
			CPUModel: hw.CPUModel,
			CPUCores: hw.CPUCores,
			GPUModel: hw.GPUModel,
		},
		Categories:     catResults,
		TotalInstalled: totalInstalled,
		TotalTools:     totalTools,
		ScanDuration:   time.Since(start).Round(time.Millisecond).String(),
	}
}

// detectTool attempts to detect a single tool and returns its result.
func detectTool(def ToolDef, category string) schema.ToolResult {
	result := schema.ToolResult{
		Name:     def.Name,
		Category: category,
	}

	for _, cmdName := range def.Commands {
		path, err := exec.LookPath(cmdName)
		if err != nil {
			continue
		}
		result.Path = path

		// Presence-only tool — no version extraction.
		if def.NoVersion {
			result.Installed = true
			result.Version = ""
			return result
		}

		timeout := def.Timeout
		if timeout == 0 {
			timeout = cmdTimeout
		}

		var version string
		if def.Stdin != "" {
			// REPL-style: pipe a script to stdin with no args.
			version = tryGetVersion(cmdName, nil, def.VersionFilter, def.Stdin, def.VersionRegex, timeout)
		} else {
			for _, args := range buildArgSets(def.VersionArgs) {
				version = tryGetVersion(cmdName, args, def.VersionFilter, "", def.VersionRegex, timeout)
				if version != "" {
					break
				}
			}
		}
		if version != "" {
			result.Installed = true
			result.Version = version
			if def.MultiVersion {
				if all := DetectMultiVersions(def.Name, version); len(all) >= 2 {
					result.AllVersions = all
				}
			}
			return result
		}

		// Binary exists but we couldn't extract a version.
		// Only mark installed if there's no VersionFilter requirement —
		// a filter miss means this binary is the wrong variant (e.g. python2 vs python3).
		if def.VersionFilter == "" {
			result.Installed = true
			// GUIApp version fallback: when the CLI binary is present but its version
			// flag is unreliable (e.g. xcodebuild CLT-only, VirtualBox on ARM), try
			// reading the version from the native OS app record (plist / registry).
			// This runs only on tools that define a GUIApp; short-circuits immediately
			// if the cache lookup misses so there is no meaningful extra latency.
			if def.GUIApp != nil {
				if guiResult := discoverGUIApp(def, category); guiResult.Version != "" {
					result.Version = guiResult.Version
					return result
				}
			}
			result.Version = "unknown"
			return result
		}
		// VersionFilter didn't match on any arg set — continue trying next command.
	}

	// All $PATH commands failed — try scalable OS app discovery.
	if def.GUIApp != nil {
		return discoverGUIApp(def, category)
	}

	return result
}

// buildArgSets converts VersionArgs strings into ready-to-use [][]string.
func buildArgSets(versionArgs []string) [][]string {
	src := versionArgs
	if len(src) == 0 {
		src = defaultVersionArgs
	}
	out := make([][]string, 0, len(src))
	for _, s := range src {
		if parts := strings.Fields(s); len(parts) > 0 {
			out = append(out, parts)
		}
	}
	return out
}

// tryGetVersion runs cmdName with args up to timeout.
// It extracts a version from stdout+stderr, checking VersionFilter if non-empty.
// When versionRegex is non-empty it is used instead of the generic semver scan;
// it must contain exactly one capturing group that yields the version string.
// Prefers a 3-component semver match; falls back to a 2-component non-zero-major match.
func tryGetVersion(cmdName string, args []string, filter, stdin, versionRegex string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdName, args...) //nolint:gosec
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	_ = cmd.Run()

	combined := stdout.String() + "\n" + stderr.String()

	// Some Windows-native tools (e.g. wsl.exe) output UTF-16LE text, which
	// embeds a null byte after every character.  Strip them so version regexes
	// can match the resulting ASCII bytes.
	if strings.ContainsRune(combined, '\x00') {
		combined = strings.ReplaceAll(combined, "\x00", "")
	}

	// If a content filter is required, bail out early if output doesn't contain it.
	if filter != "" && !strings.Contains(combined, filter) {
		return ""
	}

	// Tool-specific regex overrides the generic semver scan.
	// Used when the output embeds unrelated version strings that semver3/semver2
	// would pick up first (e.g. nmap embeds its Darwin SDK version before its own).
	if versionRegex != "" {
		re, err := regexp.Compile(versionRegex)
		if err == nil {
			if m := re.FindStringSubmatch(combined); len(m) >= 2 {
				return cleanVersion(m[1])
			}
		}
		return ""
	}

	// Prefer exact 3-component semver (stops greedy match from eating extra dot segments).
	if m := semver3Regex.FindString(combined); m != "" {
		return cleanVersion(m)
	}
	// Fall back to 2-component where the major is ≥ 1 (avoids timestamp fragments like 07.109).
	if m := semver2Regex.FindString(combined); m != "" {
		return cleanVersion(m)
	}
	return ""
}

// cleanVersion trims trailing dots and surrounding whitespace from a version string.
// e.g. "2.51.0." → "2.51.0", " 3.11 " → "3.11".
func cleanVersion(v string) string {
	return strings.TrimRight(strings.TrimSpace(v), ".")
}
