package main

import (
	"os"

	"github.com/ozanturksever/convex-backend-ops/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
