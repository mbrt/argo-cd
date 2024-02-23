// Generate Applications from ApplicationSets locally. This only requires
// cluster secrets and ApplicationSets manifests to be available locally.
//
// Accepts inputs from stdin or from files.
//
// Usage: applicationset-previewer <appset1.yaml,appset2.yaml> <secrets1.yaml,secrets2.yaml>
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Parse()
	if err := do(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func do() error {
	apps, err := Generate(flag.Args())
	DumpApps(os.Stdout, apps)
	return err
}
