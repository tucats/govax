// Package bootdata embeds the small set of files govax needs to boot a
// working console with no external testdata/ checkout: the DCL command
// grammar (evax.dcl), HELP text (vax.help), the startup script (vax.init),
// and the microkernel source vax.init assembles (kernel.asm, which
// .INCLUDEs ssdef.asm). These are copies of testdata/dcl and testdata/asm's
// own files (see docs/PHASE-15.md) — go:embed can only embed files under
// this package's own directory, which is why a copy exists here rather than
// embedding testdata/ in place.
//
// FS is the last, implicit entry in every internal/respath.Resolver's
// search order: a "govax" invocation with no "-path" flags at all still
// boots correctly, purely off these embedded copies.
package bootdata

import (
	"embed"
	"io/fs"
)

//go:embed files
var embedded embed.FS

// FS serves the embedded files by their bare name (e.g. "vax.init"), rather
// than "files/vax.init").
var FS fs.FS = mustSub(embedded, "files")

func mustSub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	
	return sub
}
