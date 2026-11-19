package sema

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Modules (the Fortran 90 standard's 11.3; docs/planning/PHASE-106 - Fortran
// 90 modules.md). A module is a program unit that holds declarations for other
// units to share: data, constants, derived types, interface blocks, and, after
// CONTAINS, module procedures. A unit that says USE M has M's entities as if it
// had declared them itself (use association, use.go).
//
//	MODULE CONSTS                       PROGRAM AREA
//	  REAL, PARAMETER :: PI = 3.14159     USE CONSTS
//	  INTEGER :: CALLS = 0                CALLS = CALLS + 1
//	END MODULE CONSTS                     PRINT *, PI * R**2
//	                                    END PROGRAM AREA
//
// Compiling a module does two things. Its variables are storage: one program
// section, $MOD$CONSTS, laid out as a COMMON block's is (finding 1), so that
// every unit using the module reaches the same bytes, as COMMON's units do; the
// module's own object holds the section's initial values. And its entities are
// written as the module's text: Fortran 90 declarations of everything a user
// can reach, with !GOVAX$ comment lines for what Fortran can't say (each
// variable's offset, each type's layout). The text is the module file,
// CONSTS.F90$MOD, and every USE reads it, whether the module was compiled
// earlier in this compilation (its text in memory) or before (finding 6). So a
// module in the same file and one compiled separately are one path, and a user
// learns of a module only what its file says (modwrite.go, modread.go).
//
// A module's specification part may not hold what only runs (11.3's
// constraints): executable statements, statement functions, ENTRY, FORMAT, and
// automatic objects; and by the HP manual, AUTOMATIC, INTENT, and OPTIONAL
// (MODSTMT). Its variables are static, whatever /RECURSIVE says: they are SAVE
// in practice (the design note; 14.7.6 lets a processor keep them).

// modulePsectPrefix starts the name of a module's program section, $MOD$M:
// a name no COMMON block of a program can share, since no FORTRAN name starts
// with a $.
const modulePsectPrefix = "$MOD$"

// ModuleText is a module this compilation compiled: its name and its text, the
// module file's lines, which the FORTRAN command writes as NAME.F90$MOD
// (finding 7). Digest is the text's SHA-256 without its source line, in
// hexadecimal: what says whether the file changed (finding 9).
type ModuleText struct {
	Name   string
	Lines  []string
	Digest string
}

// ModuleFinder finds the text of a module this compilation doesn't compile: a
// module file (subtask 5's search). Find returns the file's lines and its
// specification, as messages name it, or ok false and the places it looked.
type ModuleFinder interface {
	Find(name string) (lines []string, spec string, ok bool, looked []string)
}

// modules is a compilation's table of modules: those it compiles, by name with
// their text, those its USE statements have read (each read once, so that a
// type reached by two paths is one Structure, finding 3), and those later in
// the file, for MODLATER.
type modules struct {
	compiled map[string]*ModuleText
	loaded   map[string]*moduleScope
	later    map[string]diag.Position
	finder   ModuleFinder

	// current is the module being compiled, which nothing it uses may use
	// (MODCYCLE).
	current string

	// opts are what a module file's text is analyzed with: the compilation's
	// options, which the file's own lines change.
	opts Options
}

// moduleScope is what a USE of a module reaches: the module as its text gives
// it (modread.go). unit is the module read from the text, its Symbols the
// entities; block is its variables' program section, nil if it has none.
type moduleScope struct {
	name      string
	unit      *Unit
	block     *CommonBlock
	gFloating bool

	// failed is set when the text couldn't be read; the USE that found it
	// reported it, and later USEs say nothing more.
	failed bool
}

// newModules makes the table for the file's compilation, noting where each of
// its modules is, so that a USE before one can say it comes later.
func newModules(file *ast.File, opts Options) *modules {
	m := &modules{
		compiled: map[string]*ModuleText{},
		loaded:   map[string]*moduleScope{},
		later:    map[string]diag.Position{},
		finder:   opts.Modules,
		opts:     opts,
	}

	for _, au := range file.Units {
		if h, ok := au.Header.(*ast.ModuleStmt); ok {
			if _, seen := m.later[h.Name]; !seen {
				m.later[h.Name] = h.Pos
			}
		}
	}

	return m
}

// ModulePsect is the program section of module name's variables: $MOD$NAME, or
// for a name too long for MACRO's 31 characters, the stable hashed form
// (LongName).
func ModulePsect(name string) string {
	return LongName(modulePsectPrefix + name)
}

// LongName is a global name as MACRO can hold it: the name itself if it has at
// most 31 characters, else its first 22, a $, and 8 hexadecimal digits of the
// whole name's SHA-256 (finding 5). It is stable, so a module and each of its
// users, compiled apart, make the same name from the same full name.
func LongName(name string) string {
	if len(name) <= 31 {
		return name
	}

	sum := sha256.Sum256([]byte(name))

	return name[:22] + "$" + strings.ToUpper(hex.EncodeToString(sum[:4]))
}

// moduleStatements reports each statement a module's specification part can't
// hold (MODSTMT): an executable statement, a statement function, and FORMAT.
// What else 11.3 and the HP manual forbid there has its message already: ENTRY
// outside a subprogram (ENTRYUNIT), INTENT and OPTIONAL on what isn't a dummy
// argument (ATTRDUMMY), an array of bounds not constant (ADJNOTDUMMY), and
// AUTOMATIC on a variable its section holds (AUTOCOMMON).
func (a *analyzer) moduleStatements(body []ast.Stmt) {
	for _, s := range body {
		what := ""

		switch s.(type) {
		case *ast.EndStmt, *ast.BadStmt, *ast.ContainsStmt, *ast.EntryStmt:
		case *ast.AssignStmt:
			what = "an assignment or a statement function"
		case *ast.FormatStmt:
			what = "a FORMAT statement"
		default:
			if ast.Executable(s) && s.Base().Kind != lex.Unknown {
				what = "an executable statement"
			}
		}

		if what != "" {
			a.report("MODSTMT", s.Position(), what)
		}
	}
}

// moduleBlock makes the module's program section a common block of its unit,
// $MOD$M, holding every variable and array the module declares and no COMMON
// statement placed, in order of first appearance; layoutCommon then places
// them, by the module's /ALIGN as a COMMON statement's names are (finding 1).
// A module with no variables has no section.
func (a *analyzer) moduleBlock() {
	u := a.unit

	var members []*Symbol

	for _, s := range u.Symbols {
		if moduleVariable(s) {
			members = append(members, s)
		}
	}

	if len(members) == 0 {
		return
	}

	b := &CommonBlock{Module: u.Name, Psect: ModulePsect(u.Name), NamePos: u.AST.Position(), Attrs: PsectAttrs{Align: DefaultAlign}}
	u.Commons = append(u.Commons, b)

	for _, s := range members {
		s.Common, s.commonAlign, s.commonPos = b, a.align, s.Pos
		if s.Class == Unclassified {
			s.Class = Variable
		}

		b.Members = append(b.Members, s)
	}
}

// moduleVariable reports whether s is one of a module's variables, which its
// program section holds: a name with storage of its own (not a constant, a
// procedure, a generic name, or a dummy argument), in no COMMON block.
func moduleVariable(s *Symbol) bool {
	switch {
	case s.Common != nil, s.Dummy != 0, s.Use != nil, s.External, s.broken:
		return false
	case s.Generic != nil, s.Interface != nil, s.Internal != nil:
		return false
	}

	return s.Class == Unclassified || s.Class == Variable || s.Class == Array
}
