// Command mkfcalc builds FCALC.EXE from its sources and writes it to
// fcalc.exe, beside them; "go generate ./internal/fcalc" runs it.
package main

import (
	"fmt"
	"os"

	"github.com/tucats/govax/internal/fcalc"
	"github.com/tucats/govax/internal/fcalc/build"
)

func main() {
	// go generate runs a command in the directory of the file naming it,
	// internal/fcalc, so the sources are in ".". Reading them from the
	// directory rather than the embedded copy builds from the sources as
	// they are now.
	image, err := build.Build(os.DirFS("."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkfcalc: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(fcalc.ImageFile, image, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "mkfcalc: %v\n", err)
		os.Exit(1)
	}
}
