// Command mkstarlet builds govax's STARLET.MLB from its source, both in
// internal/bootdata/files. It's run by "go generate ./internal/bootdata",
// from that directory.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tucats/govax/internal/bootdata"
)

func main() {
	src, err := os.ReadFile(filepath.Join("files", bootdata.StarletSource))
	if err != nil {
		fail(err)
	}

	data, err := bootdata.BuildStarlet(string(src))
	if err != nil {
		fail(err)
	}

	if err := os.WriteFile(filepath.Join("files", bootdata.StarletLibrary), data, 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mkstarlet:", err)
	os.Exit(1)
}
