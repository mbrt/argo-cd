package main

import (
	"flag"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

// update is useful to regenerate the golden files
// Make sure the new version makes sense!!
var update = flag.Bool("update", false, "update golden files")

func TestGenerate(t *testing.T) {
	apps, err := Generate([]string{"testdata/applicationsets.yaml"}, []string{"testdata/secrets.yaml"})
	assert.NoError(t, err)
	got := DumpApps(apps)

	if *update {
		err := os.WriteFile("testdata/expected.yaml", []byte(got), 0644)
		assert.NoError(t, err)
		t.Skip("update flag is set")
		return
	}

	want := readYaml(t, "testdata/expected.yaml")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected diff:\n%s\n", diff)
	}
}

func readYaml(t *testing.T, path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
