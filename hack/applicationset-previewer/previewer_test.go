package main

import (
	"bytes"
	"flag"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

// update is useful to regenerate the golden test files.
// Make sure the new version makes sense!!
var update = flag.Bool("update", false, "update golden files")

func TestGenerate(t *testing.T) {
	apps, err := Generate([]string{"testdata/applicationsets.yaml", "testdata/secrets.yaml"})
	assert.NoError(t, err)
	var out bytes.Buffer
	DumpApps(&out, apps)

	if *update {
		err := os.WriteFile("testdata/expected.yaml", out.Bytes(), 0644)
		assert.NoError(t, err)
		t.Skip("update flag is set")
		return
	}

	want := mustReadFile(t, "testdata/expected.yaml")
	if diff := cmp.Diff(want, out.String()); diff != "" {
		t.Errorf("Unexpected diff:\n%s\n", diff)
	}
}

func mustReadFile(t *testing.T, path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
