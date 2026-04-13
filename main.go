package main

import "github.com/idvoretskyi/cncf-github-maintainers/cmd"

// These variables are set at build time via -ldflags by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersion(version, commit, date)
	cmd.Execute()
}
