package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"github.com/sriram-ravichandran/stackget/internal/schema"
	"gopkg.in/yaml.v3"
)

// Options controls what the printer renders.
type Options struct {
	// ShowAll shows not-installed tools as well. Default = installed only.
	ShowAll      bool
	MissingOnly  bool   // show only tools that are NOT installed
	OnlyCategory string // partial name match; empty = all categories
	JSONOutput   bool
	YAMLOutput   bool
	NoColor      bool
}

// Column widths (in terminal characters).
const nameColWidth = 30

// divSep / footSep use ASCII so width is always 1 char per rune.
// Unicode box-drawing chars (─ ═) are "Ambiguous" width and render
// as 2-columns on some East-Asian/Windows locales, causing wrapping.
var (
	divSep  = "  " + strings.Repeat("-", 52)
	footSep = "  " + strings.Repeat("=", 52)
)

// PrintBanner prints the StackGet logo to stdout immediately — before any scan
// work begins — so it stays pinned at the top while output scrolls below.
//
// Layout mirrors the project SVG:
//
//	  ● ● ●
//	  ⫸⫸⫸  StackGet
//	         <subtitle>
//
// Traffic-light dots (red/yellow/green), three coloured arrows, bold name, and
// a gray subtitle — no terminal-width detection, always renders correctly.
func PrintBanner(subtitle string) {
	p := func(s string) { fmt.Fprint(os.Stdout, s+"\r\n") }
	p("")
	p("  " +
		pterm.FgLightMagenta.Sprint("⫸") +
		pterm.FgBlue.Sprint("⫸") +
		pterm.FgCyan.Sprint("⫸") +
		"  " + pterm.Bold.Sprint(pterm.FgWhite.Sprint("StackGet")))
	p("         " + pterm.FgGray.Sprint(subtitle))
	p("")
}

// Print renders a ScanResult to stdout according to opts.
func Print(result *schema.ScanResult, opts Options) {
	if opts.NoColor {
		pterm.DisableColor()
	}
	if opts.JSONOutput {
		printJSON(result)
		return
	}
	if opts.YAMLOutput {
		printYAML(result)
		return
	}
	printPretty(result, opts)
}

// ─────────────────────────────────────────────────────────────────────────────
// Pretty terminal output
// ─────────────────────────────────────────────────────────────────────────────

func printPretty(result *schema.ScanResult, opts Options) {
	// p writes a complete line using \r\n.
	// On Windows in VT/ANSI mode, plain \n is LF-only (cursor moves down but
	// stays at the same column). \r resets the cursor to column 0 first, so
	// every subsequent line starts from the left margin.
	// On macOS/Linux \r is a no-op when already at column 0, so this is safe
	// cross-platform.
	p := func(s string) { fmt.Fprint(os.Stdout, s+"\r\n") }

	// (banner already printed by the command before scanning started)

	hw := result.Hardware
	arch := result.Arch

	// Host | OS line
	osDisplay := hw.OSName
	if osDisplay == "" {
		osDisplay = result.OS // fallback to runtime.GOOS if detection failed
	}
	p(pterm.FgGray.Sprint(fmt.Sprintf(
		"  Host: %-20s  |  OS: %s (%s)",
		result.Hostname, osDisplay, arch)))

	// CPU line
	cpuDisplay := hw.CPUModel
	if cpuDisplay == "" {
		cpuDisplay = "unknown"
	}
	if hw.CPUCores > 0 {
		p(pterm.FgGray.Sprint(fmt.Sprintf("  CPU:  %s (%d cores)", cpuDisplay, hw.CPUCores)))
	} else {
		p(pterm.FgGray.Sprint(fmt.Sprintf("  CPU:  %s", cpuDisplay)))
	}

	// GPU line (omitted if undetectable)
	if hw.GPUModel != "" {
		p(pterm.FgGray.Sprint(fmt.Sprintf("  GPU:  %s", hw.GPUModel)))
	}

	p(pterm.FgGray.Sprint(divSep))

	// Scan time + tip on the same line
	tip := ""
	if !opts.ShowAll && !opts.MissingOnly {
		tip = "  |  Tip: pass --all to show not-installed tools"
	}
	p(pterm.FgGray.Sprint(fmt.Sprintf("  Scan: %s%s", result.ScanDuration, tip)))
	p("")

	wantCat := strings.ToLower(opts.OnlyCategory)

	for _, cat := range result.Categories {
		if wantCat != "" && !strings.Contains(strings.ToLower(cat.Name), wantCat) {
			continue
		}

		// Filter rows for this category.
		var rows []schema.ToolResult
		for _, t := range cat.Tools {
			switch {
			case opts.MissingOnly && t.Installed:
				continue
			case !opts.ShowAll && !opts.MissingOnly && !t.Installed:
				continue // default: hide not-installed
			}
			rows = append(rows, t)
		}
		if len(rows) == 0 {
			continue
		}

		// ── Category header ─────────────────────────────────────────────────
		p(pterm.FgCyan.Sprint(fmt.Sprintf("  %s  %s", cat.Emoji, cat.Name)))
		p(pterm.FgGray.Sprint(divSep))

		// ── Tool rows ────────────────────────────────────────────────────────
		for _, t := range rows {
			// Pad the name to a FIXED width BEFORE applying any ANSI codes.
			// pterm's Sprint wraps text in escape sequences; if we apply padding
			// inside Sprint, fmt still sees the correct byte count but the
			// terminal sees only the visible characters — padding must be plain.
			namePadded := fmt.Sprintf("%-*s", nameColWidth, t.Name)

			if t.Installed {
				var ver string
				if len(t.AllVersions) >= 2 {
					ver = formatAllVersions(t.Version, t.AllVersions)
				} else {
					ver = formatInstalledVersion(t.Version)
				}
				p("    " + pterm.FgGreen.Sprint("●") + "  " +
					namePadded + "  " + pterm.FgGreen.Sprint(ver))
			} else {
				p("    " + pterm.FgGray.Sprint("○") + "  " +
					pterm.FgGray.Sprint(namePadded) + "  " +
					pterm.FgGray.Sprint("not installed"))
			}
		}
		p("")
	}

	// ── Footer ───────────────────────────────────────────────────────────────
	p(pterm.FgYellow.Sprint(footSep))
	p(pterm.FgYellow.Sprint(fmt.Sprintf("  Scan: %s", result.ScanDuration)))
	p(pterm.FgYellow.Sprint(footSep))
	p("")
}

// formatAllVersions renders the active version with "(active)" followed by
// the other versions sorted as stored in allVersions.
// The result is truncated to 55 visible characters to preserve column alignment.
func formatAllVersions(active string, allVersions []string) string {
	parts := []string{}
	if active != "" {
		parts = append(parts, active+" (active)")
	}
	for _, v := range allVersions {
		if v != active {
			parts = append(parts, v)
		}
	}
	result := strings.Join(parts, ", ")
	const maxLen = 55
	runes := []rune(result)
	if len(runes) > maxLen {
		result = string(runes[:maxLen-1]) + "…"
	}
	return result
}

// formatInstalledVersion returns a display-friendly version string.
// Both "" (NoVersion tool) and "unknown" (version extraction failed) display
// as "installed" in the terminal — the distinction is preserved in JSON/YAML.
func formatInstalledVersion(v string) string {
	if v == "" || v == "unknown" {
		return "installed"
	}
	return v
}

// ─────────────────────────────────────────────────────────────────────────────
// JSON / YAML
// ─────────────────────────────────────────────────────────────────────────────

func printJSON(result *schema.ScanResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		pterm.Error.Println("JSON encode error:", err)
	}
}

func printYAML(result *schema.ScanResult) {
	data, err := yaml.Marshal(result)
	if err != nil {
		pterm.Error.Println("YAML encode error:", err)
		return
	}
	fmt.Print(string(data))
}

// ─────────────────────────────────────────────────────────────────────────────
// Enforce: CI/CD pass/fail against a manifest
// ─────────────────────────────────────────────────────────────────────────────

// PrintEnforce compares the current scan against a manifest (saved via generate).
// It checks every tool marked installed in the manifest and reports PASS/FAIL.
// Returns true if all requirements are met (caller should exit 0), false if not (exit 1).
func PrintEnforce(manifest, current *schema.ScanResult, manifestFile string) bool {
	p := func(s string) { fmt.Fprint(os.Stdout, s+"\r\n") }

	// (banner already printed by the command before scanning started)

	p(pterm.FgGray.Sprint(fmt.Sprintf(
		"  Manifest: %s   Host: %s (%s/%s)",
		manifestFile, current.Hostname, current.OS, current.Arch)))
	p("")

	currMap := buildToolMap(current)

	const w = 28 // tool name column width
	passCount, failCount := 0, 0
	var failLines []string // collect failures for the summary

	for _, cat := range manifest.Categories {
		for _, req := range cat.Tools {
			if !req.Installed {
				continue // only enforce tools the manifest says must be present
			}

			key := strings.ToLower(cat.Name + "/" + req.Name)
			curr, found := currMap[key]

			namePadded := fmt.Sprintf("%-*s", w, req.Name)

			switch {
			case !found || !curr.Installed:
				line := fmt.Sprintf("  🔴  %s  not installed (required: %s)",
					namePadded, displayVer(req.Version))
				p(pterm.FgRed.Sprint(line))
				failLines = append(failLines, fmt.Sprintf(
					"  %s — not installed (required: %s)", req.Name, displayVer(req.Version)))
				failCount++

			case req.Version != "" && curr.Version != req.Version && req.Version != "unknown":
				line := fmt.Sprintf("  🔴  %s  required %-14s found %s",
					namePadded, req.Version, curr.Version)
				p(pterm.FgRed.Sprint(line))
				failLines = append(failLines, fmt.Sprintf(
					"  %s — required %s, found %s", req.Name, req.Version, curr.Version))
				failCount++

			default:
				ver := curr.Version
				if ver == "" {
					ver = "installed"
				}
				p(pterm.FgGreen.Sprint(fmt.Sprintf("  🟢  %s  %s", namePadded, ver)))
				passCount++
			}
		}
	}

	p("")
	p(pterm.FgGray.Sprint(footSep))

	if failCount == 0 {
		p(pterm.FgGreen.Sprint(fmt.Sprintf(
			"  ✅  PASS — All %d requirements met.", passCount)))
		p(pterm.FgGray.Sprint(footSep))
		p("")
		return true
	}

	p(pterm.FgRed.Sprint(fmt.Sprintf(
		"  ❌  FAIL — %d of %d requirements not met:", failCount, passCount+failCount)))
	for _, line := range failLines {
		p(pterm.FgRed.Sprint(line))
	}
	p(pterm.FgGray.Sprint(footSep))
	p("")
	return false
}

// displayVer formats a version for the enforce output (handles empty/"unknown").
func displayVer(v string) string {
	if v == "" || v == "unknown" {
		return "any version"
	}
	return v
}

// ─────────────────────────────────────────────────────────────────────────────
// Diff: side-by-side comparison of two YAML snapshots
// ─────────────────────────────────────────────────────────────────────────────

// PrintDiff shows a column-aligned diff of two ScanResult snapshots.
func PrintDiff(left, right *schema.ScanResult, leftFile, rightFile string) {
	p := func(s string) { fmt.Fprint(os.Stdout, s+"\r\n") }

	// (banner already printed by the command before work started)

	p(fmt.Sprintf("  %-38s  %s",
		pterm.FgCyan.Sprintf("<< %s (%s)", leftFile, left.Hostname),
		pterm.FgMagenta.Sprintf(">> %s (%s)", rightFile, right.Hostname)))
	p("")

	leftMap := buildToolMap(left)
	rightMap := buildToolMap(right)
	allKeys := mergeKeys(leftMap, rightMap)

	const colW = 22
	p(pterm.FgGray.Sprint(fmt.Sprintf("  %-30s  %-*s  %-*s",
		"Tool", colW, leftFile, colW, rightFile)))
	p(pterm.FgGray.Sprint("  " + strings.Repeat("-", 30+2+colW+2+colW)))

	for _, key := range allKeys {
		l, hasL := leftMap[key]
		r, hasR := rightMap[key]

		toolName := key
		if hasL {
			toolName = l.Name
		} else if hasR {
			toolName = r.Name
		}
		lv := fmtVer(l, hasL)
		rv := fmtVer(r, hasR)

		if lv == rv {
			p(fmt.Sprintf("  %-30s  %-*s  %-*s", toolName, colW, lv, colW, rv))
		} else {
			p(fmt.Sprintf("  %-30s  %s  %s", toolName,
				pterm.FgCyan.Sprintf("%-*s", colW, lv),
				pterm.FgMagenta.Sprintf("%-*s", colW, rv)))
		}
	}

	p("")
	onlyL, onlyR, both, differ := diffStats(leftMap, rightMap)
	p(fmt.Sprintf("  Shared: %d   Only in %s: %d   Only in %s: %d   Version differs: %d",
		both, leftFile, onlyL, rightFile, onlyR, differ))
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func buildToolMap(r *schema.ScanResult) map[string]schema.ToolResult {
	m := make(map[string]schema.ToolResult, 512)
	for _, cat := range r.Categories {
		for _, t := range cat.Tools {
			key := strings.ToLower(cat.Name + "/" + t.Name)
			m[key] = t
		}
	}
	return m
}

func mergeKeys(a, b map[string]schema.ToolResult) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func fmtVer(t schema.ToolResult, exists bool) string {
	if !exists || !t.Installed {
		return "-"
	}
	if t.Version == "" {
		return "installed"
	}
	return t.Version
}

func diffStats(left, right map[string]schema.ToolResult) (onlyLeft, onlyRight, both, differ int) {
	for k, l := range left {
		r, ok := right[k]
		if !ok || !r.Installed {
			if l.Installed {
				onlyLeft++
			}
		} else if l.Installed && r.Installed {
			both++
			if l.Version != r.Version {
				differ++
			}
		}
	}
	for k, r := range right {
		l, ok := left[k]
		if (!ok || !l.Installed) && r.Installed {
			onlyRight++
		}
	}
	return
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
