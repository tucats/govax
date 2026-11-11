// Package sysdef is govax's own definition modules for FORTRAN: what a program
// gets from INCLUDE '($SSDEF)' when no text library has such a module. VAX
// FORTRAN's come from SYS$LIBRARY:FORSYSDEF.TLB, a library that comes with the
// compiler; govax makes its from the values internal/vmsdef already holds, as
// FORTRAN source, when a compilation first asks for one
// (docs/planning/PHASE-83 - FORTRAN and VMS.md, Decision 3).
//
// Most modules are a family of constants, the names VMS's definition file of
// that name defines. Each name is declared and given its value in two
// statements, so the module means the same under /NOI4 and IMPLICIT NONE:
//
//	      INTEGER*4 SS$_ABORT
//	      PARAMETER (SS$_ABORT = '0000002C'X)
//
// A module is a set of name prefixes, and a name belongs to the module with its
// longest matching prefix, so that LIB$_ is $LIBDEF's and LIB$K_DCF $LIBDCFDEF's
// and no name is in two modules. Three modules are made differently:
//
//   - $SYSSRVNAM declares each system service an INTEGER*4 function, from
//     vmsdef's vector of them.
//   - $FORIOSDEF has each FOR$_ name's number, what IOSTAT= returns for that
//     error (for an end of file, -1 instead), as FOR$IOS_ names. There is no
//     FORSYSDEF to read, so those names are unconfirmed.
//   - $CHFDEF has structures as well as constants: the handler's argument list,
//     signal array, and mechanism array.
//
// A program pays nothing for the names it doesn't use. A PARAMETER makes no
// code or storage, and an EXTERNAL from a text library's module makes no .EXTRN
// unless the unit uses the name (sema's Symbol.Linked).
package sysdef

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/tucats/govax/internal/vmsdef"
)

// families are the modules of constants, with each one's name prefixes.
var families = map[string][]string{
	"$ACCDEF":    {"ACC$"},
	"$ATRDEF":    {"ATR$"},
	"$BRKDEF":    {"BRK$"},
	"$CHFDEF":    {"CHF$"},
	"$CLIDEF":    {"CLI$"},
	"$DCDEF":     {"DC$", "DT$"},
	"$DEVDEF":    {"DEV$"},
	"$DSCDEF":    {"DSC$"},
	"$DVIDEF":    {"DVI$"},
	"$FABDEF":    {"FAB$"},
	"$FIBDEF":    {"FIB$"},
	"$FORDEF":    {"FOR$"},
	"$IODEF":     {"IO$"},
	"$JPIDEF":    {"JPI$"},
	"$LCKDEF":    {"LCK$"},
	"$LIBCLIDEF": {"LIB$K_CLI", "LIB$M_CLI", "LIB$V_CLI", "LIB$S_CLI"},
	"$LIBDCFDEF": {"LIB$B_DCF", "LIB$K_DCF", "LIB$M_DCF", "LIB$S_DCF", "LIB$S_LIB$B_DCF", "LIB$V_DCF"},
	"$LIBDEF":    {"LIB$_"},
	"$LIBDTDEF":  {"LIB$K_", "LIB$M_", "LIB$S_DT", "LIB$V_"},
	"$LIBVMDEF":  {"LIB$K_VM", "LIB$M_VM"},
	"$LKIDEF":    {"LKI$", "LKISND$", "LKIUSR$"},
	"$LNMDEF":    {"LNM$"},
	"$MSGDEF":    {"MSG$"},
	"$MTHDEF":    {"MTH$"},
	"$NAMDEF":    {"NAM$"},
	"$OPCDEF":    {"OPC$"},
	"$OTSDEF":    {"OTS$"},
	"$PQLDEF":    {"PQL$"},
	"$PRCDEF":    {"PRC$"},
	"$PRTDEF":    {"PRT$"},
	"$PRVDEF":    {"PRV$"},
	"$PSLDEF":    {"PSL$"},
	"$RABDEF":    {"RAB$"},
	"$RMSDEF":    {"RMS$"},
	"$SECDEF":    {"SEC$"},
	"$SHRDEF":    {"SHR$"},
	"$SMGDEF":    {"SMG$"},
	"$SSDEF":     {"SS$"},
	"$STATEDEF":  {"SCH$C_"},
	"$STRDEF":    {"STR$"},
	"$STSDEF":    {"STS$"},
	"$SYIDEF":    {"SYI$"},
	"$TT2DEF":    {"TT2$"},
	"$TTDEF":     {"TT$"},
	"$XABDEF":    {"XAB$"},
}

// The modules made some other way than by prefix.
const (
	serviceModule = "$SYSSRVNAM"
	iostatModule  = "$FORIOSDEF"
	handlerModule = "$CHFDEF"

	// forPrefix and iostatPrefix are the condition names $FORIOSDEF reads, and
	// what it calls the same names.
	forPrefix    = "FOR$_"
	iostatPrefix = "FOR$IOS_"

	// maxName is the most characters a FORTRAN symbolic name may have.
	maxName = 31
)

// constant is one name and its value.
type constant struct {
	name  string
	value uint32
}

// built is every module's text, made the first time one is asked for: most
// compilations ask for none.
var built = sync.OnceValue(func() map[string][]string {
	values := allValues()
	modules := map[string][]string{}

	for module, list := range constantsByModule(values) {
		modules[module] = constantLines(module, list)
	}

	modules[handlerModule] = append(modules[handlerModule], handlerStructures()...)
	modules[serviceModule] = serviceLines()
	modules[iostatModule] = constantLines(iostatModule, iostatConstants(values))

	return modules
})

// Module returns the text of the module named name, in any case, and whether
// there is one. The lines are shared, so a caller mustn't change them.
func Module(name string) ([]string, bool) {
	lines, ok := built()[strings.ToUpper(name)]

	return lines, ok
}

// Names returns every module's name, sorted.
func Names() []string {
	names := make([]string, 0, len(built()))
	for name := range built() {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// allValues is every definition vmsdef has, in one table. The two tables differ
// in a few end-of-list markers, where Symbols, from the later definitions, wins.
func allValues() map[string]uint32 {
	values := make(map[string]uint32, len(vmsdef.LibrarySymbols)+len(vmsdef.Symbols))

	for name, value := range vmsdef.LibrarySymbols {
		values[name] = value
	}

	for name, value := range vmsdef.Symbols {
		values[name] = value
	}

	return values
}

// constantsByModule sorts values into their modules, each module's in order of
// name. A name no module's prefix matches is in none.
func constantsByModule(values map[string]uint32) map[string][]constant {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}

	sort.Strings(names)

	owners := prefixOwners()
	modules := map[string][]constant{}
	seen := map[string]bool{}

	for _, name := range names {
		// Names differing only in case are one name to FORTRAN, and a name
		// past FORTRAN's limit is none. The sort puts capitals first, so
		// TT$_PRO_SERIES stays and TT$_Pro_Series goes.
		upper := strings.ToUpper(name)
		if len(name) > maxName || seen[upper] {
			continue
		}

		if module := owners.moduleOf(name); module != "" {
			seen[upper] = true
			modules[module] = append(modules[module], constant{upper, values[name]})
		}
	}

	return modules
}

// prefixTable says which module owns each prefix.
type prefixTable map[string]string

// prefixOwners makes the table from families.
func prefixOwners() prefixTable {
	owners := prefixTable{}

	for module, prefixes := range families {
		for _, prefix := range prefixes {
			owners[prefix] = module
		}
	}

	return owners
}

// moduleOf is the module with the longest prefix of name, or "" if none has one.
func (t prefixTable) moduleOf(name string) string {
	for length := len(name); length > 0; length-- {
		if module, ok := t[name[:length]]; ok {
			return module
		}
	}

	return ""
}

// constantLines is a module of constants as FORTRAN source.
func constantLines(module string, list []constant) []string {
	lines := make([]string, 0, 2*len(list)+1)
	lines = append(lines, "C     "+module+": govax's definitions, from its tables of VMS's values")

	for _, c := range list {
		lines = append(lines, "      INTEGER*4 "+c.name, fmt.Sprintf("      PARAMETER (%s = '%08X'X)", c.name, c.value))
	}

	return lines
}

// iostatConstants is $FORIOSDEF's names. A FOR$_ condition value's message
// number is the number IOSTAT= returns for it.
func iostatConstants(values map[string]uint32) []constant {
	var list []constant

	for name, value := range values {
		if strings.HasPrefix(name, forPrefix) {
			list = append(list, constant{iostatPrefix + name[len(forPrefix):], iostatNumber(value)})
		}
	}

	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })

	return list
}

// iostatNumber is a condition value's message number: bits 3 to 14, above the
// severity and below the facility's flag bit.
func iostatNumber(condition uint32) uint32 {
	return condition >> 3 & 0xFFF
}

// serviceLines is $SYSSRVNAM: each service an INTEGER*4 function named in an
// EXTERNAL statement. Left out of vmsdef's vector are what no program calls by
// name: its data cells, its spare entries, the second halves of services with
// two (SYS$QIOW_2), and SYS$SRCHANDLER, reached by a JMP.
func serviceLines() []string {
	var names []string

	for _, entry := range vmsdef.P1VectorTable {
		if !entry.DataCell() && !entry.Jmp && !strings.HasSuffix(entry.Name, "_2") && !strings.Contains(entry.Name, "SPARE") {
			names = append(names, entry.Name)
		}
	}

	sort.Strings(names)

	lines := make([]string, 0, 2*len(names)+1)
	lines = append(lines, "C     "+serviceModule+": the system services, from govax's table of them")

	for _, name := range names {
		lines = append(lines, "      INTEGER*4 "+name, "      EXTERNAL "+name)
	}

	return lines
}

// handlerStructure is one of $CHFDEF's structures: its name, the constant that
// says its size, and its longwords' names, "" for one with no name.
type handlerStructure struct {
	name   string
	size   string
	fields []string
}

// handlerLayouts are a handler's own argument list (its count, then the two
// arrays' addresses), its signal array, and its mechanism array, which VSI's
// LIB$MATCH_COND example reads the depth from. A test holds each field to
// vmsdef's offset.
var handlerLayouts = []handlerStructure{
	{"CHFDEF", "CHF$S_CHFDEF", []string{"", "CHF$L_SIGARGLST", "CHF$L_MCHARGLST"}},
	{"CHFDEF1", "CHF$S_CHFDEF1", []string{"CHF$L_SIG_ARGS", "CHF$L_SIG_NAME", "CHF$L_SIG_ARG1"}},
	{"CHFDEF2", "CHF$S_CHFDEF2", []string{"CHF$L_MCH_ARGS", "CHF$L_MCH_FRAME", "CHF$L_MCH_DEPTH", "CHF$L_MCH_SAVR0", "CHF$L_MCH_SAVR1"}},
}

// handlerStructures is $CHFDEF's structure declarations. Field names have a
// structure's own name space, so each may also be the PARAMETER giving its
// offset.
func handlerStructures() []string {
	var lines []string

	for _, s := range handlerLayouts {
		lines = append(lines, "      STRUCTURE /"+s.name+"/")

		for _, field := range s.fields {
			if field == "" {
				field = "%FILL"
			}

			lines = append(lines, "        INTEGER*4 "+field)
		}

		lines = append(lines, "      END STRUCTURE")
	}

	return lines
}
