// Package main provides the entry point for the Joblin CLI application.
// Joblin is a tool for deploying and managing Python script jobs on Kubernetes clusters.
package main

import (
	"fmt"
	"os"

	"github.com/berkunal/joblin/src/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
