package main

import (
	"os"

	"github.com/hpkotak/shellbud/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
