package bootdata

import (
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/vmsdef"
)

//go:generate go run ./mkdefs
//go:generate go run ./mkstarlet

// govax's own STARLET.MLB, MACRO's system macro library when no real one
// is found, and the sources it's built from (docs/PHASE-28.md): the
// hand-written macros, and the $xxxDEF definition macros, which mkdefs
// generates (docs/PHASE-32.md). The library is generated from the sources
// and committed beside them; a test rebuilds it and checks that the two
// match.
const (
	StarletLibrary   = "starlet.mlb"
	StarletSource    = "starlet.mar"
	StarletDefSource = "starletdef.mar"
)

// StarletSources returns the text of the library's sources, in fsys,
// joined in the order they're built.
func StarletSources(fsys fs.FS) (string, error) {
	var b strings.Builder

	for _, name := range []string{StarletSource, StarletDefSource} {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return "", err
		}

		b.Write(data)
	}

	return b.String(), nil
}

// starletTime is the library's creation, revision, and insertion time,
// fixed so that the generated library doesn't change unless its source
// does.
var starletTime = time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)

// BuildStarlet builds STARLET.MLB from its source as LIBRARY/CREATE/MACRO
// would: one module per macro, squeezed. A warning from the librarian is
// an error here, since govax's own source should draw none.
func BuildStarlet(src string) ([]byte, error) {
	b, err := lbr.Create(lbr.TypeMacro)
	if err != nil {
		return nil, err
	}

	when := vmsdef.Time(starletTime)
	b.Created, b.Updated = when, when

	lines := strings.Split(strings.TrimSuffix(strings.ReplaceAll(src, "\r\n", "\n"), "\n"), "\n")

	entries, warns, err := b.MacroModules(lines, true)
	if err != nil {
		return nil, err
	}

	if len(warns) > 0 {
		return nil, fmt.Errorf("%s: %w", StarletSource, warns[0])
	}

	for _, e := range entries {
		e.Inserted = when

		if err := b.Insert(e); err != nil {
			return nil, fmt.Errorf("%s: %w", StarletSource, err)
		}
	}

	return b.Bytes(), nil
}
