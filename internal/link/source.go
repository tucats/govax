package link

// This file is docs/PHASE-30.md's "symbol sources": where LINK finds the
// global symbols the object modules refer to but don't define. A source
// can be govax's own tables (its shims and the P1 vector), a shareable
// image's global symbol table, or an object library; LINK searches its
// sources in order. The image it writes is the same whichever source
// resolved a symbol: a routine in a shareable image is referred to by the
// image's name and the routine's offset in it, which govax's RUN resolves
// against the real image if it's there, and against the shim registered
// for that offset otherwise.

// Definition is a global symbol a source defines.
type Definition struct {
	// Image names the shareable image the symbol is in, and Value is its
	// offset there. With no Image, Value is the symbol's absolute value,
	// as a system service's address in the P1 vector is.
	Image string
	Value uint32
}

// Match controls how the image activator checks a shareable image's
// global section ident against the one the image was linked with (the
// Linker manual, GSMATCH=).
type Match uint32

// Match controls, as ISD$K_MATxxx.
const (
	MatchAlways Match = 0 // ISD$K_MATALL: any ident
	MatchEqual  Match = 1 // ISD$K_MATEQU: the same ident
	MatchLEQ    Match = 2 // ISD$K_MATLEQ: the same major ident, and a minor ident at least as high
	MatchNever  Match = 3 // ISD$K_MATNEV: never
)

// SharedImage is what an image records about a shareable image it refers
// to, in the global section ISD real VMS maps it through: its size, and
// the global section ident and match control the image activator checks.
// govax's RUN doesn't use these, but real VMS does.
type SharedImage struct {
	Name    string
	Pages   uint32
	MajorID uint8
	MinorID uint32 // 24 bits
	Match   Match
}

// SymbolSource finds global symbols the object modules don't define.
type SymbolSource interface {
	// Lookup returns the definition of the global symbol name.
	Lookup(name string) (Definition, bool)
	// Image returns what the source knows about the shareable image
	// name.
	Image(name string) (SharedImage, bool)
}

// TableSource is a SymbolSource made from tables.
type TableSource struct {
	Symbols map[string]Definition
	Images  map[string]SharedImage
}

// Lookup implements SymbolSource.
func (t *TableSource) Lookup(name string) (Definition, bool) {
	d, ok := t.Symbols[name]

	return d, ok
}

// Image implements SymbolSource.
func (t *TableSource) Image(name string) (SharedImage, bool) {
	i, ok := t.Images[name]

	return i, ok
}
