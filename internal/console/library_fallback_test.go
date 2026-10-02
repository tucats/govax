package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/respath"
)

// TestLibrary_listEmbeddedFallback lists govax's own system macro library
// by its bare name, with no copy of it on the host: an input host file
// that isn't in the current directory is found along c.Paths, ending with
// the embedded files (internal/rms/hostfile.go). With no type, /MACRO
// supplies it, and an upper-case name finds the lower-case embedded file.
func TestLibrary_listEmbeddedFallback(t *testing.T) {
	c := New(&bytes.Buffer{})
	c.Paths = respath.New(nil, bootdata.FS)
	out := c.Out.(*bytes.Buffer)

	base := strings.TrimSuffix(bootdata.StarletLibrary, ".mlb")

	for _, opts := range []LibraryOptions{
		{Library: bootdata.StarletLibrary, List: true},
		{Library: strings.ToUpper(base), Macro: true, List: true},
	} {
		out.Reset()

		if err := c.Library(opts); err != nil {
			t.Fatalf("LIBRARY/LIST %s: %v", opts.Library, err)
		}

		if !strings.Contains(out.String(), "Directory of MACRO library") {
			t.Errorf("LIBRARY/LIST %s:\n%s", opts.Library, out.String())
		}
	}

	// With no search path, a bare name is only the current directory's.
	c.Paths = nil
	if err := c.Library(LibraryOptions{Library: bootdata.StarletLibrary, List: true}); err == nil {
		t.Error("found the embedded library with no search path")
	}
}
