package securertl

import (
	"fmt"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
)

// Library is the name of the shareable image these routines stand for, as a
// program's image names it and SHIM$<library>_<offset> spells it.
const Library = "SECURESHR"

// codeBase is the XFC$SHIM code of the transfer vector's offset 0: a code is
// codeBase plus the entry's offset, 6000 to 6120, above LBRSHR's (5000 on)
// and below the image activator's stubs for other libraries (machine's
// unresolvedCodeBase, 10000).
const codeBase = 6000

// entry is one entry of SECURESHR's transfer vector, at its offset.
type entry struct {
	name   string
	offset uint32
	fn     corevms.ShimFunc // nil: NOTIMPL until its subtask
}

// entries are the transfer vector's entries govax knows. govax has no copy
// of SECURESHR.EXE's global symbol table, so the offsets come from what a
// program linked against VMS 7.1's holds: testdata/prot93/vax/round3's
// SEC93.EXE, whose fixup vector gave $FORMAT_ACL %X48, $PARSE_ACL %X50,
// $CHANGE_ACL %X58, and $CHECK_ACCESS %X78. A program linked by govax's
// LINK gets these offsets, and one VMS's LINK made runs the same routines.
// $CHECK_ACCESS is subtask 13's.
var entries = []entry{
	{name: "SYS$FORMAT_ACL", offset: 0x48, fn: formatACL},
	{name: "SYS$PARSE_ACL", offset: 0x50, fn: parseACL},
	{name: "SYS$CHANGE_ACL", offset: 0x58, fn: changeACL},
	{name: "SYS$CHECK_ACCESS", offset: 0x78},
}

// Register installs every entry's routine in t, NOTIMPL for an entry with
// none. The machine does this for each new System.
func Register(t *corevms.ShimTable) {
	for _, e := range entries {
		fn := e.fn
		if fn == nil {
			fn = librtl.NotImplemented(e.name)
		}

		t.Register(codeBase+e.offset, e.name, fn)
	}
}

// Links are the transfer vector's entries as corevms.ShimLinks, for their
// stubs.
func Links() []corevms.ShimLink {
	links := make([]corevms.ShimLink, len(entries))

	for i, e := range entries {
		links[i] = corevms.ShimLink{Name: e.name, Library: Library, Offset: e.offset, Code: codeBase + e.offset}
	}

	return links
}

// Code returns the XFC$SHIM code of name's stub, for the tests; it panics
// for a name that isn't an entry.
func Code(name string) uint32 {
	for _, e := range entries {
		if e.name == name {
			return codeBase + e.offset
		}
	}

	panic(fmt.Sprintf("securertl: %s is not in %s's transfer vector", name, Library))
}

// arg is argument i of a call, 0 if the call had fewer.
func arg(argv []uint32, i int) uint32 {
	if i < len(argv) {
		return argv[i]
	}

	return 0
}
