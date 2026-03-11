# StackGet

> Detect every development tool installed on your machine — CLI + GUI apps, with exact versions.

## Install

```bash
npm install -g stackget
```

## Usage

```bash
stackget scan                     # Show all installed tools
stackget scan --all               # Include not-installed tools
stackget scan --missing           # Show only what's missing
stackget scan --only languages    # Filter by category
stackget scan -o json             # JSON output
stackget generate                 # Save snapshot to stackget.yaml
stackget check                    # Verify machine matches snapshot
stackget diff laptop.yaml ci.yaml # Compare two snapshots
stackget export --target devcontainer --output .devcontainer/devcontainer.json
```

## Platforms

Windows · macOS (Intel + Apple Silicon) · Linux

## Links

- [GitHub](https://github.com/sriram-ravichandran/stackget)
- [Issues](https://github.com/sriram-ravichandran/stackget/issues)
- [Releases](https://github.com/sriram-ravichandran/stackget/releases)
