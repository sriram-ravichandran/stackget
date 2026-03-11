package detector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OverlayRegistry is the top-level structure of a registry.json overlay file.
// Version is reserved for future schema evolution; currently unused.
type OverlayRegistry struct {
	Version    string            `json:"version,omitempty"`
	Categories []OverlayCategory `json:"categories"`
}

// OverlayCategory is a JSON-serialisable Category for overlay use.
type OverlayCategory struct {
	Name  string           `json:"name"`
	Emoji string           `json:"emoji,omitempty"`
	Tools []OverlayToolDef `json:"tools"`
}

// OverlayToolDef is a JSON-serialisable version of ToolDef.
// Timeout is expressed in milliseconds (0 = use the built-in default).
type OverlayToolDef struct {
	Name          string      `json:"name"`
	Commands      []string    `json:"commands,omitempty"`
	VersionArgs   []string    `json:"version_args,omitempty"`
	VersionFilter string      `json:"version_filter,omitempty"`
	VersionRegex  string      `json:"version_regex,omitempty"`
	NoVersion     bool        `json:"no_version,omitempty"`
	TimeoutMs     int         `json:"timeout_ms,omitempty"`
	Stdin         string      `json:"stdin,omitempty"`
	MultiVersion  bool        `json:"multi_version,omitempty"`
	GUIApp        *OverlayGUI `json:"gui_app,omitempty"`
}

// OverlayGUI is a JSON-serialisable GUIApp.
type OverlayGUI struct {
	WinExe     string   `json:"win_exe,omitempty"`
	WinHints   []string `json:"win_hints,omitempty"`
	MacApp     string   `json:"mac_app,omitempty"`
	LinuxBin   string   `json:"linux_bin,omitempty"`
	LinuxHints []string `json:"linux_hints,omitempty"`
	EnvVar     string   `json:"env_var,omitempty"`
}

// OverlayPath returns the canonical path for the local registry overlay file.
func OverlayPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".stackget", "registry.json"), nil
}

// LoadOverlay reads the local registry overlay (if present) and returns its
// categories. A missing file is not an error — it returns nil, nil.
func LoadOverlay() ([]OverlayCategory, error) {
	path, err := OverlayPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var reg OverlayRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	return reg.Categories, nil
}

// MergeCategories returns a merged category list where overlay entries override
// built-in entries by category+tool name (case-insensitive). New categories and
// new tools within existing categories are appended.
// AllCategories is never mutated — a deep copy is taken before merging.
func MergeCategories(base []Category, overlay []OverlayCategory) []Category {
	if len(overlay) == 0 {
		return base
	}

	// Deep-copy so AllCategories is never mutated.
	merged := make([]Category, len(base))
	for i, cat := range base {
		tools := make([]ToolDef, len(cat.Tools))
		copy(tools, cat.Tools)
		merged[i] = Category{Name: cat.Name, Emoji: cat.Emoji, Tools: tools}
	}

	for _, oc := range overlay {
		catIdx := -1
		for i, mc := range merged {
			if strings.EqualFold(mc.Name, oc.Name) {
				catIdx = i
				break
			}
		}

		if catIdx == -1 {
			// Brand-new category — append at the end.
			nc := Category{Name: oc.Name, Emoji: oc.Emoji, Tools: make([]ToolDef, 0, len(oc.Tools))}
			for _, ot := range oc.Tools {
				nc.Tools = append(nc.Tools, toToolDef(ot))
			}
			merged = append(merged, nc)
			continue
		}

		if oc.Emoji != "" {
			merged[catIdx].Emoji = oc.Emoji
		}

		for _, ot := range oc.Tools {
			toolIdx := -1
			for j, t := range merged[catIdx].Tools {
				if strings.EqualFold(t.Name, ot.Name) {
					toolIdx = j
					break
				}
			}
			td := toToolDef(ot)
			if toolIdx == -1 {
				merged[catIdx].Tools = append(merged[catIdx].Tools, td)
			} else {
				merged[catIdx].Tools[toolIdx] = td
			}
		}
	}

	return merged
}

// toToolDef converts an OverlayToolDef into a ToolDef usable by the detector.
func toToolDef(o OverlayToolDef) ToolDef {
	td := ToolDef{
		Name:          o.Name,
		Commands:      o.Commands,
		VersionArgs:   o.VersionArgs,
		VersionFilter: o.VersionFilter,
		VersionRegex:  o.VersionRegex,
		NoVersion:     o.NoVersion,
		Stdin:         o.Stdin,
		MultiVersion:  o.MultiVersion,
	}
	if o.TimeoutMs > 0 {
		td.Timeout = time.Duration(o.TimeoutMs) * time.Millisecond
	}
	if o.GUIApp != nil {
		td.GUIApp = &GUIApp{
			WinExe:     o.GUIApp.WinExe,
			WinHints:   o.GUIApp.WinHints,
			MacApp:     o.GUIApp.MacApp,
			LinuxBin:   o.GUIApp.LinuxBin,
			LinuxHints: o.GUIApp.LinuxHints,
			EnvVar:     o.GUIApp.EnvVar,
		}
	}
	return td
}
