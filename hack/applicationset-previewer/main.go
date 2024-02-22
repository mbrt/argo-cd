// Generate Applications from ApplicationSets locally. This only requires
// cluster secrets and ApplicationSets manifests to be available locally.
//
// Usage: applicationset-previewer --appsets=<appset1.yaml,appset2.yaml> --cluster-secrets=<secrets1.yaml,secrets2.yaml>
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	appsets        = flag.String("appsets", "", "comma separated file URLs to load application sets from")
	clusterSecrets = flag.String("cluster-secrets", "", "comma separated file names to load cluster secrets from")
)

func main() {
	flag.Parse()
	if err := do(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func do() error {
	apps, err := Generate(strings.Split(*appsets, ","), strings.Split(*clusterSecrets, ","))
	fmt.Println(DumpApps(apps))
	return err
}
