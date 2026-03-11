package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "stackget",
	Short: "Detect all development tools installed on your machine",
	Long: `StackGet scans your developer machine and reports every installed
development tool — CLI utilities AND desktop GUI applications — with
their exact versions, organized into 21 categories.

HOW IT WORKS
  CLI tools   Searched on PATH; version extracted by running the tool
              with its version flag (e.g. node --version, go version).
  GUI apps    Queried from the OS native app registry — no path guessing:
                Windows  → Uninstall registry hives (HKLM + HKCU),
                           catches Electron per-user installs too
                macOS    → /Applications directory
                Linux    → XDG .desktop files
  Multi-ver   Node.js, Python 3, Java, Go show ALL installed versions.
              Checked: nvm/fnm (Node), pyenv (Python), sdkman/jenv (Java),
              goenv + ~/sdk (Go) — plus standard OS install directories.

QUICK START
  # See everything installed right now
  stackget scan

  # Snapshot your environment to a file
  stackget generate

  # Verify a machine matches the snapshot (great for CI)
  stackget check

  # Compare two environments side-by-side
  stackget diff laptop.yaml ci.yaml

  # Generate a devcontainer.json from your environment
  stackget export --target devcontainer

COMMANDS
  scan        Detect and display installed tools
  generate    Save full scan results to a YAML file  (default: stackget.yaml)
  check       Verify current machine matches a saved manifest
  diff        Side-by-side diff of two saved environment snapshots
  export      Convert scan results to another config format
  update      Fetch the latest tool registry overlay from the remote server

Run 'stackget <command> --help' for examples and full flag details.`,
}

// appVersion is set by main via SetVersion before Execute is called.
var appVersion = "dev"

// SetVersion wires the build-time version string into the root command.
// Call this from main() before Execute().
func SetVersion(v string) {
	appVersion = v
}

// Execute is the entry-point called from main.
func Execute() {
	rootCmd.Version = appVersion
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")
}
