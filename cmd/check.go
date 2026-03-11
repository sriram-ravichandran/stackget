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
	"github.com/sriram-ravichandran/stackget/internal/output"
	"github.com/sriram-ravichandran/stackget/internal/schema"
	"gopkg.in/yaml.v3"
)

var checkCmd = &cobra.Command{
	Use:   "check [manifest]",
	Short: "Enforce a saved environment manifest against the current machine",
	Long: `Scan the current machine and compare every tool against a saved manifest.
Reports which tools pass or fail, then exits with a code CI can act on.

If no manifest file is given, check auto-discovers stackget.yaml,
stackget.yml, or stackget.json in the current working directory.
Create a manifest with: stackget generate

EXAMPLES
  stackget check                       Auto-discover manifest in current dir
  stackget check stackget.yaml         Explicit path
  stackget check ~/envs/team.yaml      Shared team baseline

OUTPUT
  🟢  Node.js        20.17.0           installed, version matches
  🟢  Docker         28.0.1            installed, version matches
  🔴  Python 3       not installed     tool is missing entirely
  🔴  Go             1.21 ≠ 1.26       installed but wrong version

EXIT CODES
  0   Every required tool is installed at the correct version  (PASS)
  1   One or more tools are missing or on the wrong version    (FAIL)

CI USAGE
  Add to any pipeline as a pre-build gate:
    - run: stackget check ci-baseline.yaml`,
	Args: cobra.MaximumNArgs(1),
	// SilenceUsage prevents cobra from printing usage on failure — we
	// handle all messaging ourselves so the output stays clean for CI.
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestFile, err := resolveManifest(args)
		if err != nil {
			return err
		}

		baseline, err := loadManifest(manifestFile)
		if err != nil {
			return err
		}

		noColor, _ := cmd.Flags().GetBool("no-color")
		if noColor {
			pterm.DisableColor()
		}

		output.PrintBanner("Environment Enforcer")

		var spinner *pterm.SpinnerPrinter
		if !noColor {
			spinner, _ = pterm.DefaultSpinner.
				WithText("Scanning current environment…").
				WithRemoveWhenDone(true).
				Start()
		}
		current := detector.DetectAll()
		if spinner != nil {
			_ = spinner.Stop()
		}

		pass := output.PrintEnforce(baseline, current, filepath.Base(manifestFile))
		if !pass {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

// resolveManifest returns the manifest path, auto-discovering it when not given.
func resolveManifest(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	// Auto-discover in the current working directory.
	for _, name := range []string{"stackget.yaml", "stackget.yml", "stackget.json"} {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf(
		"no manifest found — run 'stackget generate' to create one, " +
			"or pass the path explicitly: stackget check <file>",
	)
}

// loadManifest reads and parses a YAML or JSON manifest file.
func loadManifest(path string) (*schema.ScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var result schema.ScanResult
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	} else {
		if err := yaml.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
	}
	return &result, nil
}
