package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLibrary_fixtureExtracts extracts every module of the Phase 28
// fixture macro libraries (testdata/mar/macros/vax) with LIBRARY/EXTRACT
// and compares the output with real LIBRARIAN's LIBRARY/EXTRACT=* of the
// same library: its own LIBMAC.MLB, govax's GV_LIBMAC.MLB, and the copy
// of that LIBRARIAN changed.
func TestLibrary_fixtureExtracts(t *testing.T) {
	vax := filepath.Join("..", "..", "testdata", "mar", "macros", "vax")

	for _, name := range []string{"libmac", "gv_libmac", "gv_libmac2"} {
		t.Run(name, func(t *testing.T) {
			c, _ := newTestConsole(t)
			out := filepath.Join(t.TempDir(), name+".mar")

			if err := c.Library(LibraryOptions{Library: filepath.Join(vax, name+".mlb"), Extract: []string{"*"}, Output: out}); err != nil {
				t.Fatalf("LIBRARY/EXTRACT: %v", err)
			}

			got, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}

			want, err := os.ReadFile(filepath.Join(vax, name+".ext"))
			if err != nil {
				t.Fatal(err)
			}

			if g, w := strings.TrimRight(string(got), "\n"), strings.TrimRight(string(want), "\n"); g != w {
				t.Errorf("extracted:\n%s\nwant:\n%s", g, w)
			}
		})
	}
}
