package anl

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
)

// This file is ANALYZE/IMAGE's report of an image's fixup section
// (docs/PHASE-40.md): what the image activator patches once it has
// mapped the shareable images the program uses.

// refsPerLine is how many references a reference fixup list shows on a
// line. The fixtures show up to four on one line and never more
// (unconfirmed: four fits ANALYZE's 80 columns with room to spare, so
// the real limit may be higher).
const refsPerLine = 4

// fixupSection describes the fixup section, which starts a new page.
func (a *imageAnalyzer) fixupSection() {
	f := a.img.Fixups
	if f == nil {
		return
	}

	a.page("IMAGE ACTIVATOR FIXUP SECTION")
	a.blank()
	a.blank()

	a.part("Fixed Information")
	a.flagList("\t\t", "Flags:", iafFlagBits, f.Flags)
	a.line(fmt.Sprintf("\t\tshareable image count: %d", f.ShareCount))
	a.line(fmt.Sprintf("\t\textra image count: %d", f.Extra))
	a.blank()

	if f.ShlOffset != 0 {
		a.part("Shareable Image List")

		for i, name := range f.Shared {
			if i == 0 {
				a.line("\t\t0)  this image")
			} else {
				a.line(fmt.Sprintf("\t\t%d)  %s", i, quote(name)))
			}
		}

		a.blank()
	}

	if f.GFixOffset != 0 {
		a.part("G^ Reference Fixups")
		a.refLists(f.GRefs)
	}

	if f.DotAddrOffset != 0 {
		a.part(fmt.Sprintf(".ADDRESS Reference Fixups (relative to %%X'%08X')", f.Base))
		a.refLists(f.DotAddrRefs)
	}

	if f.ChgPrtOffset != 0 {
		a.part(fmt.Sprintf("Protection Change Fixups (relative to %%X'%08X')", f.Base))

		for i, p := range f.Protections {
			if i > 0 {
				a.blank()
			}

			a.line(fmt.Sprintf("\t\taddress: %%X'%08X', page count: %d", p.Address, p.Pages))
			a.line("\t\tprotection: " + protectionName(p.Code))
		}

		a.blank()
	}
}

// refLists shows each shareable image's references, each list followed
// by a blank line.
func (a *imageAnalyzer) refLists(lists []RefList) {
	for _, l := range lists {
		n := len(l.Values)
		a.keep(keepImageItem, fmt.Sprintf("\t\t%d reference%s to image %d:", n, plural(n), l.Image))

		for i := 0; i < n; i += refsPerLine {
			var b strings.Builder

			b.WriteString("\t\t\t")

			for _, v := range l.Values[i:min(i+refsPerLine, n)] {
				fmt.Fprintf(&b, "  %08X", v)
			}

			a.line(b.String())
		}

		a.blank()
	}
}

// protectionName names a page protection code from PRT$C_.
func protectionName(code uint16) string {
	for name, v := range vmsdef.Symbols {
		if strings.HasPrefix(name, "PRT$C_") && v == uint32(code) && name != "PRT$C_RESERVED" {
			return name
		}
	}

	return fmt.Sprintf("unknown (%d)", code)
}
