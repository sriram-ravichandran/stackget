package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/sriram-ravichandran/stackget/internal/detector"
	"github.com/sriram-ravichandran/stackget/internal/output"
)

// defaultRegistryURL is the canonical remote overlay source.
// Override at build time: -ldflags "-X cmd.defaultRegistryURL=https://..."
var defaultRegistryURL = "https://raw.githubusercontent.com/sriram-ravichandran/stackget/main/registry.json"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Fetch the latest tool registry overlay from the remote server",
	Long: `Download a registry overlay JSON and save it locally so that
subsequent scans include any new tools or corrected definitions.

The overlay is ADDITIVE — it only extends or overrides the built-in
tool list.  The original built-in definitions are never replaced and
are always used as the base.

OVERLAY FORMAT
  {
    "version": "1",
    "categories": [
      {
        "name": "Languages",
        "tools": [
          {
            "name":        "MyNewLang",
            "commands":    ["mynewlang"],
            "version_args": ["--version"]
          }
        ]
      }
    ]
  }

  Fields mirror the built-in ToolDef:
    name, commands, version_args, version_filter, version_regex,
    no_version, timeout_ms (integer milliseconds), stdin,
    multi_version, gui_app { win_exe, win_hints, mac_app,
    linux_bin, linux_hints, env_var }

EXAMPLES
  stackget update
  stackget update --url https://example.com/my-registry.json
  stackget update --url https://example.com/registry.json --sha256 <hex>

LOCAL FILE
  The overlay is saved to ~/.stackget/registry.json and loaded
  automatically on every subsequent scan.  Delete that file to
  revert to the built-in tool list.`,
	SilenceUsage: true,
	RunE:         runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().String("url", "", fmt.Sprintf(
		"Registry URL to fetch (default: %s)", defaultRegistryURL))
	updateCmd.Flags().String("sha256", "",
		"Expected SHA-256 hex digest — update is rejected if the download does not match")
}

func runUpdate(cmd *cobra.Command, _ []string) error {
	noColor, _ := cmd.Flags().GetBool("no-color")
	if noColor {
		pterm.DisableColor()
	}

	output.PrintBanner("Registry Update")

	url, _ := cmd.Flags().GetString("url")
	if url == "" {
		url = defaultRegistryURL
	}
	expectedSHA, _ := cmd.Flags().GetString("sha256")

	pterm.Info.Printf("Fetching %s\n", url)

	data, err := fetchURL(url)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	// Verify checksum when provided.
	if expectedSHA != "" {
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if !strings.EqualFold(got, expectedSHA) {
			return fmt.Errorf("SHA-256 mismatch\n  expected: %s\n  got:      %s", expectedSHA, got)
		}
		pterm.Success.Println("Checksum verified.")
	}

	// Validate JSON structure before writing anything to disk.
	var reg detector.OverlayRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return fmt.Errorf("invalid registry JSON: %w", err)
	}
	if len(reg.Categories) == 0 {
		return fmt.Errorf("registry JSON is valid but contains no categories — aborting")
	}

	totalTools := 0
	for _, c := range reg.Categories {
		totalTools += len(c.Tools)
	}
	pterm.Info.Printf("Registry: %d categories, %d tool definitions\n",
		len(reg.Categories), totalTools)

	// Atomic write: temp file → rename so a crash never leaves a partial file.
	path, err := detector.OverlayPath()
	if err != nil {
		return fmt.Errorf("resolving registry path: %w", err)
	}
	if err := atomicWriteFile(path, data); err != nil {
		return fmt.Errorf("saving registry: %w", err)
	}

	pterm.Success.Printf("Saved to %s\n", path)
	pterm.Info.Println("Run 'stackget scan' to use the updated registry.")
	return nil
}

func fetchURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func atomicWriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
