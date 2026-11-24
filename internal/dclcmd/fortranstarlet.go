package dclcmd

import (
	"strings"
	"sync"
	"time"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/sysdef"
)

// The definition modules to USE (docs/planning/PHASE-113 - Fortran errata.md,
// subtask 4; its A.3.3 and the author's answer 8). govax's definition modules
// ($SSDEF, $SYSSRVNAM, and the rest; internal/fortran/sysdef) are text to
// INCLUDE, and a name with a $ can't be a module's. So each is also a module,
// STARLET_ and its name without the $: USE STARLET_SSDEF gives a Fortran 90
// program what INCLUDE '($SSDEF)' gives a VAX FORTRAN one.
//
// No file holds such a module. When a USE's module isn't in any directory the
// search looks in, fortranModules.Find asks starletModule, which wraps the
// definition module's lines in MODULE and END MODULE and compiles them: its
// text is the compiler's own, as if a module file had been written from it. A
// file of the same name in a searched directory comes first. The modules hold
// constants, structures, and external procedures, never a variable or a
// procedure body, so a program using one needs no object of it at LINK.

// starletPrefix begins the name of each definition module to USE.
const starletPrefix = "STARLET_"

// starletTexts are the definition modules' texts, each compiled once a
// session for each REAL*8 format, by starletModule.
var (
	starletMutex sync.Mutex
	starletTexts = map[string][]string{}
)

// starletModule is the text of the definition module to USE whose name is
// name (STARLET_SSDEF for $SSDEF), compiled as a unit with REAL*8 in G_floating
// if gFloating is set, and its specification for messages; or false if there is
// no such definition module.
func starletModule(name string, gFloating bool) ([]string, string, bool) {
	rest, ok := strings.CutPrefix(strings.ToUpper(name), starletPrefix)
	if !ok || rest == "" {
		return nil, "", false
	}

	definition := "$" + rest

	body, ok := sysdef.Module(definition)
	if !ok {
		return nil, "", false
	}

	spec := starletPrefix + rest + " (built into govax, from " + definition + ")"
	key := starletPrefix + rest

	if gFloating {
		key += "/G"
	}

	starletMutex.Lock()
	defer starletMutex.Unlock()

	if text, ok := starletTexts[key]; ok {
		return text, spec, true
	}

	unit := starletPrefix + rest
	lines := append([]string{"      MODULE " + unit}, body...)
	lines = append(lines, "      END MODULE "+unit)

	opts := fortran.Options{
		F90: true, F90Given: true, GFloating: gFloating,
		Version:  "built in",
		Compiled: time.Date(2026, time.November, 23, 0, 0, 0, 0, time.UTC),
	}

	r := fortran.Compile(opts, fortran.File{Spec: unit + ".FOR", Name: unit, Lines: lines})
	if r.Errors != 0 || len(r.Modules) != 1 {
		// govax's own text, which its tests compile (TestStarletModules):
		// no program can make it fail.
		return nil, "", false
	}

	starletTexts[key] = r.Modules[0].Lines

	return r.Modules[0].Lines, spec, true
}
