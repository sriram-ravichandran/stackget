package cmd

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/sriram-ravichandran/stackget/internal/output"
	"github.com/sriram-ravichandran/stackget/internal/schema"
	"gopkg.in/yaml.v3"
)

var diffCmd = &cobra.Command{
	Use:   "diff <env1.yaml> <env2.yaml>",
	Short: "Side-by-side diff of two saved environment snapshots",
	Long: `Load two previously saved stackget.yaml snapshots and display them
side-by-side, highlighting every version difference.

EXAMPLES
  stackget diff laptop.yaml   desktop.yaml
  stackget diff dev.yaml      ci.yaml
  stackget diff before.yaml   after.yaml

OUTPUT
  Same version on both sides  → shown without colour
  Versions differ             → left file in cyan, right file in magenta
  Tool missing on one side    → shown as "-" for the absent environment

TYPICAL WORKFLOWS
  Onboarding    diff your machine against a teammate's snapshot to find gaps.
  Upgrades      snapshot before and after an upgrade to confirm what changed.
  CI vs local   diff ci-baseline.yaml against your local stackget.yaml to
                understand why builds pass in CI but fail on your machine.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		noColor, _ := cmd.Flags().GetBool("no-color")
		if noColor {
			pterm.DisableColor()
		}

		output.PrintBanner("Environment Diff")

		leftFile := args[0]
		rightFile := args[1]

		left, err := loadYAML(leftFile)
		if err != nil {
			return err
		}
		right, err := loadYAML(rightFile)
		if err != nil {
			return err
		}

		output.PrintDiff(left, right, leftFile, rightFile)
		return nil
	},
}

func loadYAML(path string) (*schema.ScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var result schema.ScanResult
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &result, nil
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
