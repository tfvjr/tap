package main

import (
	"os"

	"github.com/tap-dev/tap/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
