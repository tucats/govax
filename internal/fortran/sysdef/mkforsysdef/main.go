// Command mkforsysdef writes govax's FORTRAN definition modules as a text
// library, so they can be read with LIBRARY/LIST and LIBRARY/EXTRACT:
//
//	go run ./internal/fortran/sysdef/mkforsysdef [file]
//
// The file is by default FORSYSDEF.TLB in the current directory. Nothing needs
// it to compile: the FORTRAN command has the same modules built in, and searches
// them last (internal/fortran/sysdef).
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/tucats/govax/internal/fortran/sysdef"
)

func main() {
	file := "FORSYSDEF.TLB"
	if len(os.Args) > 1 {
		file = os.Args[1]
	}

	library, err := sysdef.Library(time.Now())
	if err == nil {
		err = os.WriteFile(file, library, 0o644)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "mkforsysdef: %v\n", err)
		os.Exit(1)
	}
}
