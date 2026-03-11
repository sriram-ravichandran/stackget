package main

import "github.com/sriram-ravichandran/stackget/cmd"

// version is injected at build time by GoReleaser:
//
//	-ldflags "-X main.version=v1.2.3"
//
// Falls back to "dev" when built locally with `go build`.
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
