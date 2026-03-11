package hardware

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// hwTimeout caps each OS subprocess call. Hardware queries run concurrently
// with the tool scan so they add zero wall-clock time to the total scan.
const hwTimeout = 5 * time.Second

// Info holds the hardware profile of the current machine.
type Info struct {
	OSName   string // "Windows 11 Pro", "macOS Sonoma 14.4.1", "Ubuntu 22.04.3 LTS"
	CPUModel string // "AMD Ryzen 9 5900X", "Apple M3 Max"
	CPUCores int    // logical (hardware thread) count
	GPUModel string // "NVIDIA GeForce RTX 4090"; empty if undetectable
}

// Collect fetches OS, CPU, and GPU info concurrently.
// All three queries start at the same time; total latency ≈ slowest query.
func Collect() Info {
	info := Info{}
	var wg sync.WaitGroup
	wg.Add(3)

	go func() { defer wg.Done(); info.OSName = fetchOS() }()
	go func() { defer wg.Done(); info.CPUModel, info.CPUCores = fetchCPU() }()
	go func() { defer wg.Done(); info.GPUModel = fetchGPU() }()

	wg.Wait()
	return info
}

// ─────────────────────────────────────────────────────────────────────────────
// OS dispatcher
// ─────────────────────────────────────────────────────────────────────────────

func fetchOS() string {
	switch runtime.GOOS {
	case "windows":
		return fetchWinOS()
	case "darwin":
		return fetchMacOS()
	case "linux":
		return fetchLinuxOS()
	}
	return runtime.GOOS
}

func fetchCPU() (model string, cores int) {
	switch runtime.GOOS {
	case "windows":
		return fetchWinCPU()
	case "darwin":
		return fetchMacCPU()
	case "linux":
		return fetchLinuxCPU()
	}
	return "", 0
}

func fetchGPU() string {
	switch runtime.GOOS {
	case "windows":
		return fetchWinGPU()
	case "darwin":
		return fetchMacGPU()
	case "linux":
		return fetchLinuxGPU()
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Windows — wmic (much lower startup cost than PowerShell)
// ─────────────────────────────────────────────────────────────────────────────

func fetchWinOS() string {
	// Try wmic first (fast, ~100 ms). Falls back to Get-CimInstance when wmic
	// is absent (removed in Windows 11 24H2+ on some editions).
	if out := wmicGetValue("os", "get", "Caption", "/value"); out != "" {
		if v := strings.TrimSpace(wmicFirstValue(out, "Caption")); v != "" {
			return v
		}
	}
	// Fallback: PowerShell Get-CimInstance (available Windows 8+)
	return strings.TrimSpace(psGetValue("(Get-CimInstance Win32_OperatingSystem).Caption"))
}

func fetchWinCPU() (string, int) {
	// Try wmic first.
	if out := wmicGetValue("cpu", "get", "Name,NumberOfLogicalProcessors", "/value"); out != "" {
		model := strings.TrimSpace(wmicFirstValue(out, "Name"))
		cores, _ := strconv.Atoi(strings.TrimSpace(wmicFirstValue(out, "NumberOfLogicalProcessors")))
		if model != "" {
			return model, cores
		}
	}
	// Fallback: PowerShell Get-CimInstance
	model := strings.TrimSpace(psGetValue("(Get-CimInstance Win32_Processor).Name"))
	coresStr := strings.TrimSpace(psGetValue("(Get-CimInstance Win32_Processor).NumberOfLogicalProcessors"))
	cores, _ := strconv.Atoi(coresStr)
	return model, cores
}

func fetchWinGPU() string {
	// Try wmic first.
	if out := wmicGetValue("path", "Win32_VideoController", "get", "Name", "/value"); out != "" {
		if gpus := wmicAllValues(out, "Name"); len(gpus) > 0 {
			return pickGPU(gpus)
		}
	}
	// Fallback: PowerShell Get-CimInstance
	out := strings.TrimSpace(psGetValue("(Get-CimInstance Win32_VideoController).Name"))
	if out == "" {
		return ""
	}
	// May return multiple lines if there are multiple adapters.
	var gpus []string
	for _, line := range strings.Split(out, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			gpus = append(gpus, name)
		}
	}
	return pickGPU(gpus)
}

// wmicGetValue runs: cmd /c wmic <args> and returns stdout.
// All queries use the /value flag so output is "KEY=value\r\n" pairs —
// unambiguous to parse and immune to locale-specific column spacing.
func wmicGetValue(args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), hwTimeout)
	defer cancel()
	fullArgs := append([]string{"/c", "wmic"}, args...)
	out, _ := exec.CommandContext(ctx, "cmd", fullArgs...).Output()
	// Strip UTF-16 BOM that some Windows versions prepend.
	s := strings.TrimPrefix(string(out), "\xff\xfe")
	return s
}

// wmicFirstValue extracts the first occurrence of "KEY=value" from wmic /value output.
func wmicFirstValue(output, key string) string {
	prefix := strings.ToLower(key) + "="
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.ReplaceAll(raw, "\r", ""))
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return ""
}

// wmicAllValues extracts every occurrence of "KEY=value" — used when multiple
// adapters each emit their own "Name=..." line in the same wmic output block.
func wmicAllValues(output, key string) []string {
	prefix := strings.ToLower(key) + "="
	var vals []string
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.ReplaceAll(raw, "\r", ""))
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			v := strings.TrimSpace(line[len(prefix):])
			if v != "" {
				vals = append(vals, v)
			}
		}
	}
	return vals
}

// psGetValue runs a single PowerShell expression and returns its stdout.
// Used as a fallback when wmic is unavailable (Windows 11 24H2+).
func psGetValue(expr string) string {
	ctx, cancel := context.WithTimeout(context.Background(), hwTimeout)
	defer cancel()
	out, _ := exec.CommandContext(ctx,
		"powershell", "-NoProfile", "-NonInteractive", "-Command", expr,
	).Output()
	return strings.TrimSpace(string(out))
}

// ─────────────────────────────────────────────────────────────────────────────
// macOS — sysctl (instant) for CPU; sw_vers for OS; system_profiler for GPU
// ─────────────────────────────────────────────────────────────────────────────

func fetchMacOS() string {
	name := strings.TrimSpace(runCmd("sw_vers", "-productName"))
	ver := strings.TrimSpace(runCmd("sw_vers", "-productVersion"))
	if name == "" {
		name = "macOS"
	}
	codename := macOSCodename(ver)
	if codename != "" {
		return name + " " + codename + " " + ver
	}
	return name + " " + ver
}

// macOSCodename maps a ProductVersion major to the Apple marketing name.
func macOSCodename(ver string) string {
	major := ver
	if idx := strings.Index(ver, "."); idx >= 0 {
		major = ver[:idx]
	}
	switch major {
	case "26":
		return "Tahoe"
	case "15":
		return "Sequoia"
	case "14":
		return "Sonoma"
	case "13":
		return "Ventura"
	case "12":
		return "Monterey"
	case "11":
		return "Big Sur"
	case "10":
		// Distinguish by minor version.
		if strings.HasPrefix(ver, "10.15") {
			return "Catalina"
		} else if strings.HasPrefix(ver, "10.14") {
			return "Mojave"
		} else if strings.HasPrefix(ver, "10.13") {
			return "High Sierra"
		}
	}
	return ""
}

func fetchMacCPU() (string, int) {
	model := strings.TrimSpace(runCmd("sysctl", "-n", "machdep.cpu.brand_string"))
	if model == "" {
		// Apple Silicon exposes brand string differently on some kernels.
		model = strings.TrimSpace(runCmd("sysctl", "-n", "hw.model"))
	}
	coresStr := strings.TrimSpace(runCmd("sysctl", "-n", "hw.logicalcpu"))
	cores, _ := strconv.Atoi(coresStr)
	return model, cores
}

func fetchMacGPU() string {
	// system_profiler SPDisplaysDataType returns display/GPU info.
	// We parse the "Chipset Model:" line from its output.
	out := runCmd("system_profiler", "SPDisplaysDataType")
	var gpus []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Chipset Model:") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "Chipset Model:"))
			if name != "" {
				gpus = append(gpus, name)
			}
		}
	}
	return pickGPU(gpus)
}

// ─────────────────────────────────────────────────────────────────────────────
// Linux — file reads (near-instant) + lspci for GPU
// ─────────────────────────────────────────────────────────────────────────────

func fetchLinuxOS() string {
	// /etc/os-release is the canonical source (systemd-era distros).
	f, err := os.Open("/etc/os-release")
	if err != nil {
		// Fallback: RHEL/CentOS older style.
		if data, err := os.ReadFile("/etc/redhat-release"); err == nil {
			return strings.TrimSpace(string(data))
		}
		return "Linux"
	}
	defer f.Close()

	vals := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.IndexByte(line, '='); idx > 0 {
			k := line[:idx]
			v := strings.Trim(line[idx+1:], `"'`)
			vals[k] = v
		}
	}
	if name := vals["PRETTY_NAME"]; name != "" {
		return name
	}
	if name := vals["NAME"]; name != "" {
		if ver := vals["VERSION"]; ver != "" {
			return name + " " + ver
		}
		return name
	}
	return "Linux"
}

func fetchLinuxCPU() (string, int) {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", 0
	}
	defer f.Close()

	var model string
	cores := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "model name") {
			if model == "" {
				if idx := strings.IndexByte(line, ':'); idx >= 0 {
					model = strings.TrimSpace(line[idx+1:])
				}
			}
		}
		if strings.HasPrefix(line, "processor") {
			cores++
		}
	}
	return model, cores
}

func fetchLinuxGPU() string {
	// Prefer lspci — most reliable for listing PCI graphics adapters.
	if out := runCmd("lspci"); out != "" {
		var gpus []string
		for _, line := range strings.Split(out, "\n") {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "vga") ||
				strings.Contains(lower, "3d controller") ||
				strings.Contains(lower, "display controller") {
				// lspci format: "00:02.0 VGA compatible controller: Intel ... [description]"
				// Extract the part after the last ": ".
				if idx := strings.LastIndex(line, ": "); idx >= 0 {
					name := strings.TrimSpace(line[idx+2:])
					// Strip "(rev XX)" suffix.
					if i := strings.LastIndex(name, " (rev"); i > 0 {
						name = strings.TrimSpace(name[:i])
					}
					gpus = append(gpus, name)
				}
			}
		}
		if len(gpus) > 0 {
			return pickGPU(gpus)
		}
	}

	// Fallback: /sys/class/drm/card*/device/product_name (common on discrete cards).
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return ""
	}
	var gpus []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "card") || strings.Contains(e.Name(), "-") {
			continue
		}
		path := "/sys/class/drm/" + e.Name() + "/device/product_name"
		if data, err := os.ReadFile(path); err == nil {
			if name := strings.TrimSpace(string(data)); name != "" {
				gpus = append(gpus, name)
			}
		}
	}
	return pickGPU(gpus)
}

// ─────────────────────────────────────────────────────────────────────────────
// GPU selection — prefer discrete over integrated
// ─────────────────────────────────────────────────────────────────────────────

// gpuScore ranks an adapter name: higher = more likely to be the primary GPU.
// NVIDIA or AMD discrete cards score highest; Intel/AMD integrated score lowest.
func gpuScore(name string) int {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "nvidia"):
		return 5
	case strings.Contains(lower, "amd") && !strings.Contains(lower, "radeon") ||
		strings.Contains(lower, "radeon") && !strings.Contains(lower, "vega") &&
			!strings.Contains(lower, "graphics"):
		return 4
	case strings.Contains(lower, "radeon"):
		return 3
	case strings.Contains(lower, "apple"):
		return 2
	case strings.Contains(lower, "intel arc"):
		return 2
	default:
		// Intel integrated (UHD, HD, Iris), VMware/VirtualBox adapters, etc.
		return 1
	}
}

// pickGPU selects the best GPU from a list of adapter names.
// If multiple GPUs share the top score (e.g. dual NVIDIA), they are joined
// with " / " so nothing is silently dropped.
func pickGPU(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}

	best := 0
	for _, n := range names {
		if s := gpuScore(n); s > best {
			best = s
		}
	}

	var top []string
	for _, n := range names {
		if gpuScore(n) == best {
			top = append(top, n)
		}
	}
	// Deduplicate (some systems report the same adapter twice).
	seen := make(map[string]bool, len(top))
	var deduped []string
	for _, n := range top {
		if !seen[n] {
			seen[n] = true
			deduped = append(deduped, n)
		}
	}
	return strings.Join(deduped, " / ")
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// runCmd executes a command with hwTimeout and returns its stdout as a string.
func runCmd(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), hwTimeout)
	defer cancel()
	out, _ := exec.CommandContext(ctx, name, args...).Output()
	return string(out)
}
