// Command mkftnrtl builds FTNRTL.EXE from its sources and writes it to
// ftnrtl.exe, beside them; "go generate ./internal/ftnrtl" runs it.
package main

import (
	"fmt"
	"os"

	"github.com/tucats/govax/internal/ftnrtl"
	"github.com/tucats/govax/internal/ftnrtl/build"
)

func main() {
	// go generate runs a command in the directory of the file naming it,
	// internal/ftnrtl, so the sources are in ".". Reading them from the
	// directory rather than the embedded copy builds from the sources as
	// they are now.
	image, err := build.Build(os.DirFS("."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkftnrtl: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(ftnrtl.ImageFile, image, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "mkftnrtl: %v\n", err)
		os.Exit(1)
	}
}
