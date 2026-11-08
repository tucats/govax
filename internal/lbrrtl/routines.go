package lbrrtl

import (
	"fmt"
	"slices"
	"sync"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/vmsdef"
)

// Library is the name of the shareable image these routines stand for, as a
// program's image names it and SHIM$<library>_<offset> spells it.
const Library = "LBRSHR"

// codeBase is the XFC$SHIM code of the transfer vector's offset 0. LBRSHR's
// entries are 5 bytes apart (an entry mask and a BRW, where MTHRTL's are 8),
// and its two data cells are at 200 and 204 (hex), so a code is codeBase plus
// the entry's offset: 5000 to 5139, above MTHRTL's (4000 on) and below the
// image activator's stubs for other libraries (machine's unresolvedCodeBase,
// 10000).
const codeBase = 5000

// Routine is one LBR$ routine govax provides.
type Routine struct {
	// Name is the routine's universal symbol, which must be one of
	// LBRSHR.EXE's.
	Name string

	// Fn is the routine.
	Fn corevms.ShimFunc

	// CallsBack says the routine may call a procedure the program passed
	// it, so it runs on a goroutine of its own
	// (corevms.ShimTable.RegisterCallback).
	CallsBack bool
}

// Routines are the routines this package provides: those that read a
// library (docs/PHASE-90 - VMS help.md, Decision 8) and those that write
// one (subtask 11). The transfer vector's other entries signal NOTIMPL.
var Routines = []Routine{
	{Name: "LBR$INI_CONTROL", Fn: iniControl},
	{Name: "LBR$OPEN", Fn: open},
	{Name: "LBR$CLOSE", Fn: closeLibrary},
	{Name: "LBR$GET_HEADER", Fn: getHeader},
	{Name: "LBR$SET_INDEX", Fn: setIndex},
	{Name: "LBR$LOOKUP_KEY", Fn: lookupKey},
	{Name: "LBR$FIND", Fn: find},
	{Name: "LBR$GET_RECORD", Fn: getRecord},
	{Name: "LBR$SET_LOCATE", Fn: setLocate},
	{Name: "LBR$SET_MOVE", Fn: setMove},
	{Name: "LBR$GET_INDEX", Fn: getIndex, CallsBack: true},
	{Name: "LBR$GET_HELP", Fn: getHelp, CallsBack: true},
	{Name: "LBR$OUTPUT_HELP", Fn: outputHelp, CallsBack: true},
	{Name: "LBR$RET_RMSSTV", Fn: retRMSSTV},
	{Name: "LBR$PUT_RECORD", Fn: putRecord},
	{Name: "LBR$PUT_END", Fn: putEnd},
	{Name: "LBR$INSERT_KEY", Fn: insertKey},
	{Name: "LBR$DELETE_KEY", Fn: deleteKey},
	{Name: "LBR$REPLACE_KEY", Fn: replaceKey},
	{Name: "LBR$DELETE_DATA", Fn: deleteData},
	{Name: "LBR$SET_MODULE", Fn: setModule},
}

// dataCells are the transfer vector's data: two longwords a program may
// read (LBR$GL_CONTROL, LBR$GL_RMSSTV). govax keeps both in Go, and the
// cells hold 0; LBR$RET_RMSSTV is the way to the RMS status value.
var dataCells = map[string]bool{"LBR$GL_CONTROL": true, "LBR$GL_RMSSTV": true}

// entry is one entry of the transfer vector.
type entry struct {
	name   string
	offset uint32
}

// entries are the transfer vector's entries, in offset order, made once
// from vmsdef's copy of LBRSHR.EXE's global symbol table.
var entries = sync.OnceValue(func() []entry {
	var list []entry

	for name, sym := range vmsdef.ImageSymbols {
		if sym.Image == Library {
			list = append(list, entry{name: name, offset: sym.Value})
		}
	}

	slices.SortFunc(list, func(a, b entry) int { return int(a.offset) - int(b.offset) })

	return list
})

// codeOf returns the XFC$SHIM code of name's stub. It panics for a name
// LBRSHR.EXE doesn't have, a mistake in the tables that TestTables reports.
func codeOf(name string) uint32 {
	sym, ok := vmsdef.ImageSymbols[name]
	if !ok || sym.Image != Library {
		panic(fmt.Sprintf("lbrrtl: %s is not in %s's transfer vector", name, Library))
	}

	return codeBase + sym.Value
}

// Register installs every routine entry's routine in t: the table's, and
// for each other entry one that signals NOTIMPL. The machine does this for
// each new System.
func Register(t *corevms.ShimTable) {
	for _, e := range entries() {
		if !dataCells[e.name] {
			t.Register(codeBase+e.offset, e.name, librtl.NotImplemented(e.name))
		}
	}

	for _, r := range Routines {
		if r.CallsBack {
			t.RegisterCallback(codeOf(r.Name), r.Name, r.Fn)
		} else {
			t.Register(codeOf(r.Name), r.Name, r.Fn)
		}
	}
}

// Links are the transfer vector's entries as corevms.ShimLinks, for their
// stubs, in offset order: a stub for each routine, whatever the table holds,
// and a longword of 0 for each data cell.
func Links() []corevms.ShimLink {
	var links []corevms.ShimLink

	for _, e := range entries() {
		l := corevms.ShimLink{Name: e.name, Library: Library, Offset: e.offset, Code: codeBase + e.offset}

		if dataCells[e.name] {
			l.Code, l.Data = 0, make([]byte, 4)
		}

		links = append(links, l)
	}

	return links
}

// arg returns argument i of argv, or 0 when the call passed fewer: an
// omitted optional argument is the same as a 0 address.
func arg(argv []uint32, i int) uint32 {
	if i < len(argv) {
		return argv[i]
	}

	return 0
}
