package anl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmsdef"
)

// gsdTitles are each GSD subrecord type's description. Real ANALYZE's
// output shows PSC, SYM, and EPM; the rest follow their pattern and the
// names chapter 7 of the Linker Utility Manual gives the subrecords
// (unconfirmed).
var gsdTitles = map[obj.GSDType]string{
	obj.GSDPsect:       "Program Section Definition",
	obj.GSDSymbol:      "Global Symbol Specification",
	obj.GSDEntry:       "Entry Point Symbol and Mask Definition",
	obj.GSDProcedure:   "Procedure Definition with Formal Arguments",
	obj.GSDSymbolW:     "Global Symbol Specification with Word Psect",
	obj.GSDEntryW:      "Entry Point Symbol and Mask Definition with Word Psect",
	obj.GSDProcedureW:  "Procedure Definition with Formal Arguments with Word Psect",
	obj.GSDIdentCheck:  "Entity Ident Consistency Check",
	obj.GSDEnvironment: "Environment Definition/Reference",
	obj.GSDLocalSymbol: "Module-Local Symbol Specification",
	obj.GSDLocalEntry:  "Module-Local Entry Point Definition",
	obj.GSDLocalProc:   "Module-Local Procedure Definition",
	obj.GSDSharedPsect: "Shareable Image Program Section Definition",
	obj.GSDSymbolV:     "Vectored Global Symbol Specification",
	obj.GSDEntryV:      "Vectored Entry Point Symbol and Mask Definition",
	obj.GSDProcedureV:  "Vectored Procedure Definition",
	obj.GSDSymbolM:     "Version Mask Global Symbol Specification",
	obj.GSDEntryM:      "Version Mask Entry Point Symbol and Mask Definition",
	obj.GSDProcedureM:  "Version Mask Procedure Definition",
}

// The named bits of a psect's and a symbol's flags. A symbol's are
// VMS's own GSY$V_ names. A psect's are ANALYZE's labels, which differ
// from VMS 7.3's objfmt.sdl in one place: ANALYZE labels bit 10 COM and
// bit 11 NOMOD, where the SDL has GPS$V_NOMOD = 10 and GPS$V_COM = 11.
// No fixture sets either bit, so whether ANALYZE's labels or its bits are
// out of step is unconfirmed; the labels are shown as ANALYZE shows them,
// over the bit in their position.
var (
	psectFlagBits = []flagBit{
		{0, "GPS$V_PIC"}, {1, "GPS$V_LIB"}, {2, "GPS$V_OVR"}, {3, "GPS$V_REL"},
		{4, "GPS$V_GBL"}, {5, "GPS$V_SHR"}, {6, "GPS$V_EXE"}, {7, "GPS$V_RD"},
		{8, "GPS$V_WRT"}, {9, "GPS$V_VEC"}, {10, "GPS$V_COM"}, {11, "GPS$V_NOMOD"},
	}
	symbolFlagBits = flagBits("GSY$V_")
)

// dataTypeNames are the DSC$K_DTYPE_ names, by value. Where two names
// share a value the shorter is used.
var dataTypeNames = func() map[byte]string {
	names := map[byte]string{}

	keys := make([]string, 0)
	for name := range vmsdef.LibrarySymbols {
		if strings.HasPrefix(name, "DSC$K_DTYPE_") {
			keys = append(keys, name)
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) < len(keys[j])
		}

		return keys[i] < keys[j]
	})

	for _, name := range keys {
		v := byte(vmsdef.LibrarySymbols[name])
		if _, ok := names[v]; !ok {
			names[v] = name
		}
	}

	return names
}()

// subrecord describes one GSD subrecord, numbered k within its record.
// A psect is counted even when it isn't shown, so later psects keep their
// numbers.
func (a *objectAnalyzer) subrecord(show bool, k int, s obj.Subrecord) {
	if !show {
		if _, ok := s.(*obj.Psect); ok {
			a.psects++
		}

		return
	}

	title := gsdTitles[s.GSDType()]
	if title == "" {
		title = "Unknown Subrecord"
	}

	a.line(fmt.Sprintf("\t%d)  %s (GSD$C_%s)", k, title, s.GSDType()))

	switch s := s.(type) {
	case *obj.Psect:
		a.psect(s)
	case *obj.Symbol:
		a.symbol(s)
	case *obj.IdentCheck:
		a.identCheck(s)
	case *obj.Environment:
		a.environment(s)
	}
}

func (a *objectAnalyzer) psect(p *obj.Psect) {
	a.line(fmt.Sprintf("\t\t%-49s<-- psect %d", fmt.Sprintf("alignment: %d-byte boundary", 1<<p.Align), a.psects))
	a.psects++

	a.line("\t\tattribute flags:")
	a.flagLines(psectFlagBits, p.Flags)
	a.line("\t\tallocation: " + unsignedValue(p.Alloc))

	if p.Shared {
		a.line("\t\tbase address: " + unsignedValue(p.Base))
	}

	a.line("\t\tsymbol: " + quote(p.Name))
}

func (a *objectAnalyzer) symbol(s *obj.Symbol) {
	name, ok := dataTypeNames[s.DataType]
	if !ok {
		name = "unknown"
	}

	a.line(fmt.Sprintf("\t\tdata type: %s (%d)", name, s.DataType))
	a.line("\t\tsymbol flags:")
	a.flagLines(symbolFlagBits, s.Flags)

	if s.IsLocal() {
		a.line(fmt.Sprintf("\t\tenvironment: %d", s.Env))
	}

	if s.Defined() {
		a.line(fmt.Sprintf("\t\tpsect: %d", s.Psect))
		a.line("\t\tvalue: " + signedValue(s.Value))

		switch s.Type {
		case obj.GSDSymbolV, obj.GSDEntryV, obj.GSDProcedureV:
			a.line("\t\tvector: " + signedValue(s.Extra))
		case obj.GSDSymbolM, obj.GSDEntryM, obj.GSDProcedureM:
			a.line(fmt.Sprintf("\t\tversion mask: %%X'%08X'", s.Extra))
		}

		if s.HasEntryMask() {
			a.line("\t\tentry mask: " + entryMask(s.Mask))
		}
	}

	a.line("\t\tsymbol: " + quote(s.Name))

	if s.Formals != nil {
		a.formals(s.Formals)
	}
}

// formals describes a procedure's formal arguments (unconfirmed layout).
func (a *objectAnalyzer) formals(f *obj.Formals) {
	a.line(fmt.Sprintf("\t\tminimum arguments: %d", f.Min))
	a.line(fmt.Sprintf("\t\tmaximum arguments: %d", f.Max))

	for i, arg := range f.Args {
		a.line(fmt.Sprintf("\t\targument %d: %s", i+1, mechanismName(arg.ValCtl)))
	}
}

// mechanismName is a formal argument's passing mechanism, from its
// validation control byte's ARG$V_PASSMECH field.
func mechanismName(valctl byte) string {
	switch uint32(valctl & 3) {
	case vmsdef.Symbols["ARG$C_VALUE"]:
		return "by value"
	case vmsdef.Symbols["ARG$C_REF"]:
		return "by reference"
	case vmsdef.Symbols["ARG$C_DESC"]:
		return "by descriptor"
	}

	return "unknown mechanism"
}

func (a *objectAnalyzer) identCheck(c *obj.IdentCheck) {
	a.line(fmt.Sprintf("\t\tflags: %d (%%X'%04X')", c.Flags, c.Flags))
	a.line(fmt.Sprintf("\t\tentity: %q", c.Name))
	a.line(fmt.Sprintf("\t\tident: %q", c.Ident))
	a.line(fmt.Sprintf("\t\tobject: %q", c.Object))
}

func (a *objectAnalyzer) environment(e *obj.Environment) {
	a.line(fmt.Sprintf("\t\tflags: %d (%%X'%04X')", e.Flags, e.Flags))
	a.line(fmt.Sprintf("\t\tparent environment: %d", e.Parent))
	a.line(fmt.Sprintf("\t\tenvironment: %q", e.Name))
}

// entryMask shows an entry mask as the registers it saves: "<R2,R3>". The
// integer and decimal overflow enables, bits 14 and 15, are shown as IV
// and DV (unconfirmed: no fixture sets them).
func entryMask(mask uint16) string {
	var names []string

	for r := range 12 {
		if mask&(1<<r) != 0 {
			names = append(names, fmt.Sprintf("R%d", r))
		}
	}

	if mask&(1<<14) != 0 {
		names = append(names, "IV")
	}

	if mask&(1<<15) != 0 {
		names = append(names, "DV")
	}

	return "<" + strings.Join(names, ",") + ">"
}
