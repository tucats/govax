package console

import (
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/vmsdef"
)

// This file is govax's own symbol source for LINK (docs/PHASE-30.md,
// "Where external symbols come from"): the symbols govax can run without
// any VMS files. The routines its shims stand for (shimTable) are in their
// shareable images at their real offsets, so an image linked against them
// runs against the shims in govax and against the real images on VMS. The
// system services are at their addresses in the P1 vector.

// sharedImages is what is known of the shareable images the shims stand
// for: what an image's global section ISD records about each (docs/
// PHASE-30.md). LIBRTL's comes from ANALYZE/IMAGE of an image real LINK
// linked against VMS 7.3's LIBRTL. The other images' aren't known, so an
// image records them by name only, with a match control that accepts any
// ident; govax's RUN doesn't check them.
var sharedImages = map[string]link.SharedImage{
	"LIBRTL": {Pages: 264, MajorID: 1, MinorID: 0x0E, Match: link.MatchLEQ},
}

// govaxSymbols returns govax's own symbol source.
func govaxSymbols() link.SymbolSource {
	src := &link.TableSource{Symbols: map[string]link.Definition{}, Images: sharedImages}

	for _, e := range vmsdef.P1VectorTable {
		src.Symbols[e.Name] = link.Definition{Value: e.Addr}
	}

	for _, e := range shimTable {
		src.Symbols[e.name] = link.Definition{Image: e.library, Value: e.offset}
	}

	return src
}
