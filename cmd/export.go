package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/sriram-ravichandran/stackget/internal/detector"
	"github.com/sriram-ravichandran/stackget/internal/schema"
)

// devcontainerConfig mirrors the schema of a .devcontainer/devcontainer.json file.
type devcontainerConfig struct {
	Name     string                       `json:"name"`
	Image    string                       `json:"image"`
	Features map[string]map[string]string `json:"features,omitempty"`
}

// featureMapping relates a detected tool to its Microsoft devcontainer feature.
type featureMapping struct {
	// ToolName matches schema.ToolResult.Name exactly.
	ToolName string
	// Category is used to disambiguate tools with the same name in different categories.
	Category string
	// Feature is the ghcr.io feature URI (without trailing version).
	Feature string
	// VersionType controls how the version string is formatted in the JSON:
	//   "major"       → "20"  (from "20.17.0")
	//   "major.minor" → "3.11" (from "3.11.0")
	//   "full"        → "1.9.3"
	//   "latest"      → always "latest"
	VersionType string
}

// devcontainerFeatures is the ordered lookup table of supported mappings.
// Order matters: the first match per feature URI wins.
var devcontainerFeatures = []featureMapping{
	// Languages
	{ToolName: "Node.js", Category: "Languages", Feature: "ghcr.io/devcontainers/features/node:1", VersionType: "major"},
	{ToolName: "Python 3", Category: "Languages", Feature: "ghcr.io/devcontainers/features/python:1", VersionType: "major.minor"},
	{ToolName: "Go", Category: "Languages", Feature: "ghcr.io/devcontainers/features/go:1", VersionType: "major.minor"},
	{ToolName: "Rust (rustc)", Category: "Languages", Feature: "ghcr.io/devcontainers/features/rust:1", VersionType: "latest"},
	{ToolName: "Java", Category: "Languages", Feature: "ghcr.io/devcontainers/features/java:1", VersionType: "major"},
	{ToolName: "Ruby", Category: "Languages", Feature: "ghcr.io/devcontainers/features/ruby:1", VersionType: "major.minor"},
	{ToolName: ".NET", Category: "Languages", Feature: "ghcr.io/devcontainers/features/dotnet:2", VersionType: "major"},
	{ToolName: "PHP", Category: "Languages", Feature: "ghcr.io/devcontainers/features/php:1", VersionType: "major.minor"},
	{ToolName: "Swift", Category: "Languages", Feature: "ghcr.io/devcontainers/features/swift:1", VersionType: "major.minor"},
	{ToolName: "Dart", Category: "Languages", Feature: "ghcr.io/devcontainers/features/dart:1", VersionType: "major.minor"},
	{ToolName: "Flutter", Category: "Languages", Feature: "ghcr.io/devcontainers/features/flutter:1", VersionType: "major.minor"},
	{ToolName: "Kotlin", Category: "Languages", Feature: "ghcr.io/devcontainers/features/java:1", VersionType: "major"}, // Kotlin runs on JVM
	// DevOps
	{ToolName: "Docker", Category: "DevOps & Infrastructure", Feature: "ghcr.io/devcontainers/features/docker-in-docker:2", VersionType: "latest"},
	{ToolName: "kubectl", Category: "DevOps & Infrastructure", Feature: "ghcr.io/devcontainers/features/kubectl-helm-minikube:1", VersionType: "latest"},
	{ToolName: "Terraform", Category: "DevOps & Infrastructure", Feature: "ghcr.io/devcontainers/features/terraform:1", VersionType: "major.minor"},
	{ToolName: "AWS CLI", Category: "DevOps & Infrastructure", Feature: "ghcr.io/devcontainers/features/aws-cli:1", VersionType: "latest"},
	{ToolName: "Azure CLI (az)", Category: "DevOps & Infrastructure", Feature: "ghcr.io/devcontainers/features/azure-cli:1", VersionType: "latest"},
	{ToolName: "Google Cloud (gcloud)", Category: "DevOps & Infrastructure", Feature: "ghcr.io/devcontainers/features/gcloud-cli:1", VersionType: "latest"},
	// CI/CD
	{ToolName: "GitHub CLI (gh)", Category: "CI/CD Tools", Feature: "ghcr.io/devcontainers/features/github-cli:1", VersionType: "latest"},
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the current environment to a target config format",
	Long: `Scan the current machine and convert the results into a target
configuration file format. Each target maps detected tools to the
conventions of that ecosystem.

TARGETS
  devcontainer
    Generates a .devcontainer/devcontainer.json using official Microsoft
    Dev Container features (ghcr.io/devcontainers/features/…).
    Detected tool versions are mapped to the nearest supported feature
    version (major, major.minor, or "latest").

    Supported tools: Node.js, Python 3, Go, Rust (rustc), Java, Kotlin,
    Ruby, .NET, PHP, Swift, Dart, Flutter, Docker, kubectl, Terraform,
    AWS CLI, Azure CLI (az), Google Cloud (gcloud), GitHub CLI (gh).

EXAMPLES
  stackget export --target devcontainer
      Print devcontainer.json to stdout.

  stackget export --target devcontainer --output .devcontainer/devcontainer.json
      Write directly to the devcontainer config file.

  stackget export --target devcontainer | pbcopy
      Copy to clipboard (macOS).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		target, _ := cmd.Flags().GetString("target")
		outFile, _ := cmd.Flags().GetString("output")

		switch strings.ToLower(target) {
		case "devcontainer":
			return runExportDevcontainer(outFile)
		case "":
			return fmt.Errorf("--target is required (e.g. --target devcontainer)")
		default:
			return fmt.Errorf("unknown target %q — supported: devcontainer", target)
		}
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().String("target", "", "Export target format (devcontainer)")
	exportCmd.Flags().String("output", "", "Output file path (default: stdout)")
	_ = exportCmd.MarkFlagRequired("target")
}

// ─────────────────────────────────────────────────────────────────────────────
// devcontainer export
// ─────────────────────────────────────────────────────────────────────────────

func runExportDevcontainer(outFile string) error {
	spinner, _ := pterm.DefaultSpinner.
		WithText("Scanning environment…").
		WithRemoveWhenDone(true).
		Start()

	result := detector.DetectAll()
	_ = spinner.Stop()

	cfg := buildDevcontainer(result)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	if outFile == "" {
		// Use \r\n so Windows VT terminal resets to column 0 on each new line.
		fmt.Print(strings.ReplaceAll(string(data), "\n", "\r\n") + "\r\n")
		pterm.Info.Printf("Mapped %d devcontainer features from %d installed tools.\r\n",
			len(cfg.Features), result.TotalInstalled)
		return nil
	}

	// Write to file — create directories if necessary.
	if dir := filepath.Dir(outFile); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(outFile, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outFile, err)
	}
	pterm.Success.Printf("Wrote devcontainer.json to %s (%d features)\n", outFile, len(cfg.Features))
	return nil
}

// buildDevcontainer maps detected tools to a devcontainerConfig.
func buildDevcontainer(result *schema.ScanResult) devcontainerConfig {
	// Build a fast lookup: "category/toolname" → ToolResult.
	installed := make(map[string]schema.ToolResult, 64)
	for _, cat := range result.Categories {
		for _, t := range cat.Tools {
			if t.Installed {
				key := strings.ToLower(cat.Name + "/" + t.Name)
				installed[key] = t
			}
		}
	}

	features := make(map[string]map[string]string)

	for _, m := range devcontainerFeatures {
		// Skip if feature URI already added (e.g. both Java + Kotlin → same feature).
		if _, exists := features[m.Feature]; exists {
			continue
		}

		key := strings.ToLower(m.Category + "/" + m.ToolName)
		t, found := installed[key]
		if !found {
			continue
		}

		ver := formatFeatureVersion(t.Version, m.VersionType)
		features[m.Feature] = map[string]string{"version": ver}
	}

	cfg := devcontainerConfig{
		Name:  "StackGet Auto-Generated Environment",
		Image: "mcr.microsoft.com/devcontainers/base:ubuntu",
	}
	if len(features) > 0 {
		cfg.Features = features
	}
	return cfg
}

// formatFeatureVersion converts a detected version string to the format
// expected by devcontainer features.
func formatFeatureVersion(version, versionType string) string {
	if versionType == "latest" || version == "" || version == "unknown" {
		return "latest"
	}

	parts := strings.Split(version, ".")
	switch versionType {
	case "major":
		if len(parts) >= 1 {
			return parts[0]
		}
	case "major.minor":
		if len(parts) >= 2 {
			return parts[0] + "." + parts[1]
		}
		if len(parts) == 1 {
			return parts[0]
		}
	}
	return version // "full" or fallback
}
