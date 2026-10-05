package anl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmsdef"
)

// ANALYZE/OBJECT reports each error on a line of its own, "***  " and a
// sentence, right after what it's about, and counts them at the end. Real
// ANALYZE's output shows two of its messages (testdata's dbgsrc.anl):
//
//	***  End of module record is missing from previous module.
//	***  The stack still contains 1 longword.
//
// The rest are govax's, in the same style (unconfirmed), for the rules of
// the object language that internal/obj.Check also applies.

// linkerStackDepth is the deepest the linker's stack is promised to go,
// in longwords (section 7.4 of the Linker Utility Manual).
const linkerStackDepth = 25

// checkCommand checks a TIR command's effect on the linker's stack (an
// underflow is found by command), and the psect it names.
func (a *objectAnalyzer) checkCommand(c obj.Command, underflow bool) {
	switch {
	case underflow:
		a.fail("The stack underflowed.")

	case a.depth > linkerStackDepth:
		a.fail("The stack is deeper than %d longwords.", linkerStackDepth)
	}

	if commandNamesPsect(c.Op) {
		a.checkPsect(c.Psect)
	}

	if c.Name != "" {
		a.checkName("symbol", c.Name)
	}
}

// commandNamesPsect reports whether a TIR command has a psect operand:
// STA_PB, STA_PW, STA_PL, and their word-psect forms, STA_WPB, STA_WPW,
// and STA_WPL.
func commandNamesPsect(op obj.Op) bool {
	name := op.String()

	return strings.HasPrefix(name, "STA_P") || strings.HasPrefix(name, "STA_WP")
}

// checkSubrecord checks a GSD subrecord's names, alignment, and psect.
func (a *objectAnalyzer) checkSubrecord(s obj.Subrecord) {
	switch s := s.(type) {
	case *obj.Psect:
		a.checkName("psect", s.Name)

		if s.Align > obj.MaxPsectAlignment {
			a.fail("Psect alignment %d is larger than a page.", s.Align)
		}

	case *obj.Symbol:
		a.checkName("symbol", s.Name)

		if s.Defined() && s.Flags&obj.SymREL != 0 {
			a.checkPsect(s.Psect)
		}

	case *obj.Environment:
		a.checkName("environment", s.Name)
	}
}

// checkPsect checks that the module defines psect index.
func (a *objectAnalyzer) checkPsect(index uint16) {
	if int(index) >= a.modulePsects {
		a.fail("Psect %d is undefined.", index)
	}
}

// checkName checks a name's length.
func (a *objectAnalyzer) checkName(what, name string) {
	if len(name) == 0 || len(name) > obj.MaxNameLength {
		a.fail("The %s name must be 1 to %d characters.", what, obj.MaxNameLength)
	}
}

// unwrap is an error's innermost message, without the record and offset
// prefixes internal/obj adds.
func unwrap(err error) error {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}

		err = next
	}
}

// objConst returns a VAX object language constant from vmsdef.Symbols.
func objConst(name string) uint32 {
	v, ok := vmsdef.Symbols[name]
	if !ok {
		panic("anl: no object language constant " + name)
	}

	return v
}

// linkTypes are the link option record types' descriptions (unconfirmed:
// no fixture has an LNK record).
var linkTypes = map[byte]string{
	byte(objConst("LNK$C_OLB")): "object library",
	byte(objConst("LNK$C_SHR")): "shareable image",
	byte(objConst("LNK$C_SHA")): "shareable image library",
	byte(objConst("LNK$C_OBJ")): "object file",
	byte(objConst("LNK$C_OLI")): "object library with inclusion list",
}

// linkOption describes a link option specification record.
func (a *objectAnalyzer) linkOption(l *obj.LNK) {
	what := linkTypes[l.Type]
	if what == "" {
		what = "unknown"
	}

	a.line(fmt.Sprintf("\tlink option type: %s (%d)", what, l.Type))
	a.line(fmt.Sprintf("\tflags: %d (%%X'%04X')", l.Flags, l.Flags))
	a.line("\tfile name: " + quote(l.Name))

	if len(l.Rest) > 0 {
		a.hexDump("\t\t", l.Rest)
	}
}
