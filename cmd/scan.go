package cmd

import (
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/sriram-ravichandran/stackget/internal/detector"
	"github.com/sriram-ravichandran/stackget/internal/output"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Detect and display installed development tools",
	Long: `Scan this machine for development tools and display the results grouped
by category. Only installed tools are shown by default.

EXAMPLES
  stackget scan                        Show all installed tools
  stackget scan --all                  Also show tools that are NOT installed
  stackget scan --missing              Show only what's missing
  stackget scan --only languages       Filter to one category (partial match)
  stackget scan --only "gui database"  Partial name match is case-insensitive
  stackget scan -o json                Raw JSON output (for scripts / CI)
  stackget scan -o yaml                Raw YAML output
  stackget scan --no-color             Plain text, no ANSI colour codes

CATEGORIES (21)
  Languages · Compilers & Build Tools · Package Managers · Databases ·
  GUI Database Clients · DevOps & Infrastructure · CI/CD Tools ·
  Cloud & Serverless · Runtime Managers & Version Tools ·
  Security & Cryptography · Code Quality & Formatting · Editors & IDEs ·
  Terminal & Shell Tools · API & Testing Tools · Data, ML & AI Tools ·
  Mobile & Cross-Platform · Web Servers & Proxies · Networking & Protocols ·
  Documentation Tools · Game Development · Blockchain & Web3

GUI APPS
  Desktop applications (pgAdmin, MySQL Workbench, MongoDB Compass, DBeaver,
  DataGrip, Docker Desktop, Postman, Insomnia, GitHub Desktop, …) are
  detected via the native OS registry — not by guessing install paths.
  Versions come from the registry DisplayVersion field where available.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		noColor, _ := cmd.Flags().GetBool("no-color")
		showAll, _ := cmd.Flags().GetBool("all")
		missingOnly, _ := cmd.Flags().GetBool("missing")
		onlyCat, _ := cmd.Flags().GetString("only")
		outFmt, _ := cmd.Flags().GetString("output")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		yamlFlag, _ := cmd.Flags().GetBool("yaml")

		// Merge: -o flag takes priority, legacy booleans are also accepted.
		outFmt = strings.ToLower(strings.TrimSpace(outFmt))
		jsonOut := jsonFlag || outFmt == "json"
		yamlOut := yamlFlag || outFmt == "yaml"

		if !jsonOut && !yamlOut {
			output.PrintBanner("Developer Environment Scanner")
		}

		var spinner *pterm.SpinnerPrinter
		if !jsonOut && !yamlOut && !noColor {
			spinner, _ = pterm.DefaultSpinner.
				WithText("Scanning developer tools…").
				WithRemoveWhenDone(true).
				Start()
		}

		result := detector.DetectAll()

		if spinner != nil {
			_ = spinner.Stop()
		}

		output.Print(result, output.Options{
			ShowAll:      showAll,
			MissingOnly:  missingOnly,
			OnlyCategory: onlyCat,
			JSONOutput:   jsonOut,
			YAMLOutput:   yamlOut,
			NoColor:      noColor,
		})

		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().Bool("all", false, "Show all tools including those not installed")
	scanCmd.Flags().Bool("missing", false, "Show only tools that are not installed")
	scanCmd.Flags().String("only", "", "Show only a specific category (partial name match)")
	scanCmd.Flags().StringP("output", "o", "", `Machine-readable output format: "json" or "yaml"`)
	scanCmd.Flags().Bool("json", false, "Output results as JSON (alias for -o json)")
	scanCmd.Flags().Bool("yaml", false, "Output results as YAML (alias for -o yaml)")
}
