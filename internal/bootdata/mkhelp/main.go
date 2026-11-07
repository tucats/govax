// Command mkhelp builds govax's help libraries, HELPLIB.HLB and
// DBG$HELP.HLB, from their help files in internal/bootdata/files/help and
// files/dbghelp. It's run by "go generate ./internal/bootdata", from that
// directory.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tucats/govax/internal/bootdata"
)

func main() {
	files := os.DirFS("files")

	names := make([]string, 0, len(bootdata.HelpLibraries))
	for name := range bootdata.HelpLibraries {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		data, err := bootdata.BuildHelp(files, bootdata.HelpLibraries[name])
		if err != nil {
			fail(err)
		}

		if err := os.WriteFile(filepath.Join("files", name), data, 0o644); err != nil {
			fail(err)
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mkhelp:", err)
	os.Exit(1)
}
