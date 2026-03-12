# StackGet

[![npm](https://img.shields.io/npm/v/stackget?color=cb3837&logo=npm)](https://www.npmjs.com/package/stackget)
[![platforms](https://img.shields.io/badge/platforms-Windows%20%7C%20macOS%20%7C%20Linux-blue)](https://github.com/sriram-ravichandran/stackget/releases/latest)
[![license](https://img.shields.io/badge/license-MIT-green)](https://github.com/sriram-ravichandran/stackget/blob/main/LICENSE)

> Detect every development tool installed on your machine — CLI + GUI apps, with exact versions.

**Current version: v1.0.3**

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

| Platform            | Supported |
|---------------------|-----------|
| Windows x64         | ✅ |
| Windows ARM64       | ✅ |
| macOS Intel         | ✅ |
| macOS Apple Silicon | ✅ |
| Linux x64           | ✅ |
| Linux ARM64         | ✅ |

## Links

- [GitHub](https://github.com/sriram-ravichandran/stackget)
- [Issues](https://github.com/sriram-ravichandran/stackget/issues)
- [Releases](https://github.com/sriram-ravichandran/stackget/releases/latest)
