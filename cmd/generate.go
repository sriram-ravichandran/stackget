package cmd

import (
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/sriram-ravichandran/stackget/internal/detector"
	"gopkg.in/yaml.v3"
)

var generateCmd = &cobra.Command{
	Use:   "generate [output-file]",
	Short: "Scan and save full results to a YAML file",
	Long: `Scan the current machine and save the complete results to a YAML file.
The snapshot records OS, architecture, hostname, timestamp, and every
detected tool with its version and install path.

EXAMPLES
  stackget generate                        Save to stackget.yaml (default)
  stackget generate ~/envs/laptop.yaml     Save to a specific path
  stackget generate ci-baseline.yaml       Name it anything you like

WHAT TO DO WITH THE FILE
  stackget check [file]          Verify a machine matches this snapshot.
                                 Auto-discovers stackget.yaml if no file given.
  stackget diff <file1> <file2>  Compare two snapshots side-by-side.

TIP
  Commit stackget.yaml to your repo so teammates and CI can run
  'stackget check' to verify their environment matches the project baseline.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outFile := "stackget.yaml"
		if len(args) == 1 {
			outFile = args[0]
		}

		noColor, _ := cmd.Flags().GetBool("no-color")
		if noColor {
			pterm.DisableColor()
		}

		var spinner *pterm.SpinnerPrinter
		if !noColor {
			spinner, _ = pterm.DefaultSpinner.
				WithText("Scanning developer tools…").
				WithRemoveWhenDone(true).
				Start()
		}

		result := detector.DetectAll()
		if spinner != nil {
			_ = spinner.Stop()
		}

		data, err := yaml.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshalling YAML: %w", err)
		}

		if err := os.WriteFile(outFile, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", outFile, err)
		}

		pterm.Success.Printf(
			"Saved %d/%d tools to %s  (scan time: %s)\n",
			result.TotalInstalled, result.TotalTools, outFile, result.ScanDuration,
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
