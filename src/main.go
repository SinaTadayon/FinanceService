package main

import (
	"fmt"
)

// Build Information variants filled at build time by compiler through flags
var (
	GitCommit string
	GitBranch string
	BuildDate string
)

func buildInfo() string {
	return fmt.Sprintf(`
	======  Finance-Service =======

	Git Commit: %s	
	Build Date: %s
	Git Branch: %s

	======  Finance-Service =======
	`, GitCommit, BuildDate, GitBranch)
}

func main() {
	fmt.Printf("%v", buildInfo())
}
