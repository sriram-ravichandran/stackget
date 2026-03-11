package schema

import "time"

// ToolResult holds the detection result for a single tool.
type ToolResult struct {
	Name        string   `yaml:"name"                         json:"name"`
	Category    string   `yaml:"category"                     json:"category"`
	Version     string   `yaml:"version,omitempty"            json:"version,omitempty"`
	AllVersions []string `yaml:"all_versions,omitempty"       json:"all_versions,omitempty"`
	Installed   bool     `yaml:"installed"                    json:"installed"`
	Path        string   `yaml:"path,omitempty"               json:"path,omitempty"`
}

// CategoryResult groups tool results under a named category.
type CategoryResult struct {
	Name      string       `yaml:"name"      json:"name"`
	Emoji     string       `yaml:"emoji"     json:"emoji"`
	Tools     []ToolResult `yaml:"tools"     json:"tools"`
	Installed int          `yaml:"installed" json:"installed"`
	Total     int          `yaml:"total"     json:"total"`
}

// HardwareInfo holds OS, CPU, and GPU details collected at scan time.
type HardwareInfo struct {
	OSName   string `yaml:"os_name"             json:"os_name"`
	CPUModel string `yaml:"cpu_model"           json:"cpu_model"`
	CPUCores int    `yaml:"cpu_cores"           json:"cpu_cores"`
	GPUModel string `yaml:"gpu_model,omitempty" json:"gpu_model,omitempty"`
}

// ScanResult is the top-level result of a full machine scan.
type ScanResult struct {
	OS             string           `yaml:"os"              json:"os"`
	Arch           string           `yaml:"arch"            json:"arch"`
	Hostname       string           `yaml:"hostname"        json:"hostname"`
	Timestamp      time.Time        `yaml:"timestamp"       json:"timestamp"`
	Hardware       HardwareInfo     `yaml:"hardware"        json:"hardware"`
	Categories     []CategoryResult `yaml:"categories"      json:"categories"`
	TotalInstalled int              `yaml:"total_installed" json:"total_installed"`
	TotalTools     int              `yaml:"total_tools"     json:"total_tools"`
	ScanDuration   string           `yaml:"scan_duration"   json:"scan_duration"`
}
