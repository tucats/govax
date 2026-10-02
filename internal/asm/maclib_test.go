package asm

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/vmserrors"
)

// Tests for macro libraries (docs/PHASE-28.md, subtask 5).

// mapLibrary is a MacroLibrary of macro sources keyed by name, counting
// its lookups.
type mapLibrary struct {
	macros  map[string]string
	lookups int
}

func newMapLibrary(macros map[string]string) *mapLibrary {
	return &mapLibrary{macros: macros}
}

func (m *mapLibrary) Macro(name string) ([]string, bool, error) {
	m.lookups++

	src, ok := m.macros[name]
	if !ok {
		return nil, false, nil
	}

	return strings.Split(strings.Trim(src, "\n"), "\n"), true, nil
}

// libAssembler returns a MACRO-dialect assembler searching libs.
func libAssembler(libs ...MacroLibrary) *Assembler {
	a := macroAssembler()
	a.SetMacroLibraries(libs...)

	return a
}

// libBytes assembles src with a and returns the bytes of the psect
// assembly ended in.
func libBytes(t *testing.T, a *Assembler, src string) []byte {
	t.Helper()

	out, err := a.Assemble(src)
	if err != nil {
		t.Fatalf("assemble:\n%s\nerror: %v", src, err)
	}

	return out
}

// TestLibraryAutomaticSearch: a name that isn't a directive, macro, or
// opcode is looked up in the libraries, defined, and called; a later
// call uses the definition without searching again. A library macro's
// .MACRO line may be continued, and its body may call another library
// macro.
func TestLibraryAutomaticSearch(t *testing.T) {
	lib := newMapLibrary(map[string]string{
		"TWO": `
	.MACRO	TWO	A,-
			B=7
	ONE	A
	.BYTE	B
	.ENDM	TWO`,
		"ONE": `
	.MACRO	ONE	X
	.BYTE	X
	.ENDM	ONE`,
	})

	a := libAssembler(lib)
	requireBytes(t, libBytes(t, a, "\t.PSECT\tDATA\n\tTWO\t1\n\tTWO\t2,3"), 1, 7, 2, 3)

	if lib.lookups != 2 {
		t.Errorf("lookups = %d, want 2 (one each for TWO and ONE)", lib.lookups)
	}
}

// TestLibrarySearchOrder: .LIBRARY's libraries, the last named first,
// then the caller's, in order.
func TestLibrarySearchOrder(t *testing.T) {
	def := func(v string) string { return "\t.MACRO\tM\n\t.BYTE\t" + v + "\n\t.ENDM" }

	named := map[string]*mapLibrary{
		"first":  newMapLibrary(map[string]string{"M": def("1")}),
		"second": newMapLibrary(map[string]string{"M": def("2")}),
	}

	callers := []MacroLibrary{
		newMapLibrary(map[string]string{"M": def("3"), "N": def("4")}),
		newMapLibrary(map[string]string{"M": def("5")}),
	}

	a := libAssembler(callers...)

	requireBytes(t, libBytes(t, a, "\t.PSECT\tDATA\n\tM"), 3)

	a = libAssembler(callers...)

	var resolved []string

	a.SetLibraryResolver(func(name string) (MacroLibrary, error) {
		resolved = append(resolved, name)

		return named[name], nil
	})

	requireBytes(t, libBytes(t, a, "\t.LIBRARY\t/first/\n\t.LIBRARY\t\\second\\\n\t.PSECT\tDATA\n\t.MDELETE M\n\tM"), 2)

	if strings.Join(resolved, ",") != "first,second" {
		t.Errorf("resolved %q, want the names as written", resolved)
	}
}

// TestLibraryOpcode: an opcode is never looked up, so a library macro
// with an opcode's name is used only after .MCALL.
func TestLibraryOpcode(t *testing.T) {
	lib := newMapLibrary(map[string]string{
		"NOP": "\t.MACRO\tNOP\n\t.BYTE\t9\n\t.ENDM\tNOP",
	})

	a := libAssembler(lib)
	requireBytes(t, libBytes(t, a, "\t.PSECT\tCODE\n\tNOP\n\t.MCALL\tNOP\n\tNOP"), 0x01, 9)
}

// TestMcall: .MCALL defines each macro from the libraries, replacing a
// definition in the source; a name no library has is an error.
func TestMcall(t *testing.T) {
	lib := newMapLibrary(map[string]string{
		"A": "\t.MACRO\tA\n\t.BYTE\t1\n\t.ENDM",
		"B": "\t.MACRO\tB\n\t.BYTE\t2\n\t.ENDM",
	})

	src := `
	.MACRO	A
	.BYTE	0
	.ENDM
	.PSECT	DATA
	A
	.MCALL	A, B
	A
	B`

	requireBytes(t, libBytes(t, libAssembler(lib), src), 0, 1, 2)

	_, err := libAssembler(lib).Assemble("\t.MCALL\tA,C")
	requireCode(t, err, vmserrors.VAX_UNDEFMACRO)
}

// TestLibraryErrors: .LIBRARY needs a resolver, and reports its failure;
// an undefined name no library has is still a bad opcode; an error in a
// library's definition names the macro.
func TestLibraryErrors(t *testing.T) {
	_, err := macroAssembler().Assemble("\t.LIBRARY\t/X/")
	requireCode(t, err, vmserrors.VAX_NOLIBRESOLVER)

	a := macroAssembler()
	a.SetLibraryResolver(func(string) (MacroLibrary, error) { return nil, errors.New("no such file") })

	_, err = a.Assemble("\t.LIBRARY\t/X/")
	requireCode(t, err, vmserrors.VAX_LIBRARY)

	lib := newMapLibrary(map[string]string{"BAD": "\t.MACRO\tBAD\n\t.BYTE\t1"})

	_, err = libAssembler(lib).Assemble("\tNOSUCH")
	requireCode(t, err, vmserrors.VAX_BADOPCODE)

	_, err = libAssembler(lib).Assemble("\tBAD")
	requireCode(t, err, vmserrors.VAX_NOENDM)

	if !strings.Contains(err.Error(), "in library definition of macro BAD") {
		t.Errorf("error = %v, want it to name the library macro", err)
	}
}

// TestConsoleDialectLibrary: the console dialect searches libraries too,
// when it's given any.
func TestConsoleDialectLibrary(t *testing.T) {
	a := New(false)
	a.SetMacroLibraries(newMapLibrary(map[string]string{
		"ONE": "\t.MACRO\tONE\tX\n\t.BYTE\tX\n\t.ENDM",
	}))

	requireBytes(t, libBytes(t, a, "ONE 5\nONE 6"), 5, 6)
}

// openStarlet returns the real STARLET.MLB (testdata/vmslib, not in git:
// it's licensed), skipping the test without it.
func openStarlet(t *testing.T) *lbr.Library {
	t.Helper()

	data, err := os.ReadFile("../../testdata/vmslib/starlet.mlb")
	if err != nil {
		t.Skip("no testdata/vmslib/starlet.mlb:", err)
	}

	l, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	return l
}

// TestStarletLoads: every one of the real STARLET.MLB's macros loads
// through .MCALL, and defines the macro its module is named for.
func TestStarletLoads(t *testing.T) {
	l := openStarlet(t)

	lib, err := NewMacroLibrary(l)
	if err != nil {
		t.Fatal(err)
	}

	keys := l.Indexes[0].Keys
	if len(keys) != 1529 {
		t.Errorf("STARLET.MLB has %d macros, want 1529", len(keys))
	}

	var src strings.Builder
	for _, k := range keys {
		src.WriteString("\t.MCALL\t" + k.Name + "\n")
	}

	a := libAssembler(lib)
	if _, err := a.Assemble(src.String()); err != nil {
		t.Fatal(err)
	}

	for _, k := range keys {
		if _, ok := a.macros[k.Name]; !ok {
			t.Errorf("macro %s not defined", k.Name)
		}
	}
}

// TestStarletCalls: system macro calls expand from the real STARLET.MLB
// and assemble into an object module.
func TestStarletCalls(t *testing.T) {
	lib, err := NewMacroLibrary(openStarlet(t))
	if err != nil {
		t.Fatal(err)
	}

	src := `
	.TITLE	CALLS
	$SSDEF
	$IODEF
	.PSECT	DATA,LONG,NOEXE,WRT
FAB1:	$FAB	FNM=<X.DAT>,FAC=<GET,PUT>,ORG=SEQ,RFM=VAR
RAB1:	$RAB	FAB=FAB1
IOSB:	.BLKQ	1
MSG:	.ASCII	/Hello/
CHAN:	.BLKW	1
	.PSECT	CODE,EXE,NOWRT
	.ENTRY	START,^M<>
	$QIOW_S	CHAN=CHAN,FUNC=#IO$_WRITEVBLK,IOSB=IOSB,P1=MSG,P2=#5
	$EXIT_S	#SS$_NORMAL
	.END	START`

	a := libAssembler(lib)
	libBytes(t, a, src)

	if len(a.Warnings()) != 0 || len(a.Messages()) != 0 {
		t.Errorf("warnings %v, messages %q, want none", a.Warnings(), a.Messages())
	}

	if _, err := a.Object(ObjectOptions{}); err != nil {
		t.Fatal(err)
	}

	for _, sym := range []string{"SS$_NORMAL", "IO$_WRITEVBLK", "FAB$C_BLN", "RAB$C_BLN"} {
		if s, ok := a.symbols.find(sym); !ok || !s.defined() {
			t.Errorf("%s not defined", sym)
		}
	}

	if ext := a.Externals(); !strings.Contains(ext, "SYS$QIOW") || !strings.Contains(ext, "SYS$EXIT") {
		t.Errorf("externals = %s, want SYS$QIOW and SYS$EXIT", ext)
	}
}
