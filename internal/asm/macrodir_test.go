package asm

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

func requireMACROError(t *testing.T, src string, code uint32) {
	t.Helper()

	_, err := macroAssembler().Assemble(src)
	requireCode(t, err, code)
}

// requirePsect checks a psect's index, attributes, and alignment.
func requirePsect(t *testing.T, a *Assembler, name string, index int, flags, align uint32) *section {
	t.Helper()

	s := a.findSection(name)
	if s == nil {
		t.Fatalf("no psect %q", name)
	}

	if s.index != index || s.flags != flags || s.align != align {
		t.Fatalf("psect %s: index %d flags %03X align %d, want %d %03X %d", name, s.index, s.flags, s.align, index, flags, align)
	}

	return s
}

func requireSymbol(t *testing.T, a *Assembler, name string, flags SymFlag) *symbol {
	t.Helper()

	s, ok := a.symbols.find(name)
	if !ok {
		t.Fatalf("no symbol %s", name)
	}

	if s.flags&flags != flags {
		t.Fatalf("symbol %s flags %b, want %b set", name, s.flags, flags)
	}

	return s
}

// TestTitleAndIdent checks the module name and version: the text after
// .TITLE's name, and .IDENT's string, keep their case; the name is cut to
// 31 characters and the comment to 40; the last of each wins.
func TestTitleAndIdent(t *testing.T) {
	a := macroAssemble(t, `	.title	first
	.TITLE	hello	Say "Hello", Politely ; a comment
	.IDENT	/v1.0-a/
	.SBTTL	Anything at all
	.SUBTITLE More`)

	if name, text := a.Title(); name != "HELLO" || text != `Say "Hello", Politely` {
		t.Errorf("Title() = %q, %q", name, text)
	}

	if got := a.Ident(); got != "v1.0-a" {
		t.Errorf("Ident() = %q", got)
	}

	a = macroAssemble(t, ".TITLE ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "+
		"0123456789012345678901234567890123456789EXTRA")
	if name, text := a.Title(); name != "ABCDEFGHIJKLMNOPQRSTUVWXYZ01234" || text != "0123456789012345678901234567890123456789" {
		t.Errorf("Title() = %q, %q", name, text)
	}

	if name, _ := macroAssemble(t, ".END").Title(); name != ".MAIN." {
		t.Errorf("default module name = %q", name)
	}

	requireMACROError(t, ".IDENT /0123456789012345678901234567890123/", vmserrors.VAX_DATARANGE)
	requireMACROError(t, ".IDENT /open", vmserrors.VAX_NOCLOSE)
}

// TestPsectAttributes checks a named psect's defaults, the attributes and
// alignment a .PSECT list changes, and a name with a ".".
func TestPsectAttributes(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE, NOWRT, EXE, LONG
	.PSECT $DATA.RW, WRT, NOEXE, PIC, SHR, GBL, OVR, LIB, VEC, NORD, 9
	.PSECT PLAIN
	.PSECT`)

	requirePsect(t, a, absPsect, 0, 0, 0)
	requirePsect(t, a, "CODE", 1, gpsREL|gpsEXE|gpsRD, 2)
	requirePsect(t, a, "$DATA.RW", 2, gpsREL|gpsWRT|gpsPIC|gpsSHR|gpsGBL|gpsOVR|gpsLIB|gpsVEC, 9)
	requirePsect(t, a, "PLAIN", 3, gpsREL|gpsEXE|gpsRD|gpsWRT, 0)
	requirePsect(t, a, blankPsect, 4, gpsREL|gpsEXE|gpsRD|gpsWRT, 0)

	requireMACROError(t, ".PSECT CODE, FAST", vmserrors.VAX_PSECTATTR)
	requireMACROError(t, ".PSECT CODE, 10", vmserrors.VAX_PSECTATTR)
}

// TestPsectContinuation checks that a continued psect may repeat its
// attributes, but not change them.
func TestPsectContinuation(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE, NOWRT, LONG
	.BYTE 1
	.PSECT DATA
	.PSECT CODE, LONG, NOWRT, EXE
	.BYTE 2`)
	requireBytes(t, psectBytes(t, a, "CODE"), 1, 2)

	requireMACROError(t, ".PSECT CODE, NOWRT\n.PSECT CODE, WRT", vmserrors.VAX_PSECTCONFLICT)
	requireMACROError(t, ".PSECT CODE, LONG\n.PSECT CODE, QUAD", vmserrors.VAX_PSECTCONFLICT)
	requireMACROError(t, ".PSECT CODE\n.PSECT CODE, NOEXE", vmserrors.VAX_PSECTCONFLICT)
}

// TestDefaultPsects checks where statements before any .PSECT go: a
// symbol definition stays in . ABS ., and . BLANK . is defined only when
// a label, code, or data needs it.
func TestDefaultPsects(t *testing.T) {
	a := macroAssemble(t, "LIMIT = 100\n.PSECT DATA\n.BYTE 1")
	if a.findSection(blankPsect) != nil {
		t.Error(". BLANK . defined for a module that never uses it")
	}

	requirePsect(t, a, "DATA", 1, defaultPsectFlags, 0)

	for _, src := range []string{"START: .PSECT DATA", ".BYTE 1\n.PSECT DATA", ".BLKB 4\n.PSECT DATA", "NOP\n.PSECT DATA"} {
		a = macroAssemble(t, src)
		requirePsect(t, a, blankPsect, 1, defaultPsectFlags, 0)
		requirePsect(t, a, "DATA", 2, defaultPsectFlags, 0)
	}

	// .RESTORE returns to . ABS . as it was, so code afterwards still
	// goes to . BLANK .: STARLET's $xxxDEF macros (through $DEFINI and
	// $DEFEND) save, switch to $ABS$, and restore this way.
	a = macroAssemble(t, ".SAVE LOCAL_BLOCK\n.PSECT $ABS$,ABS\nX = 1\n.RESTORE\nNOP")
	requirePsect(t, a, blankPsect, 2, defaultPsectFlags, 0)
	requireBytes(t, psectBytes(t, a, blankPsect), 1)
}

// TestAbsolutePsect checks a psect with the ABS attribute: it defines
// offsets (its labels are absolute), and holds no code or data.
func TestAbsolutePsect(t *testing.T) {
	a := macroAssemble(t, `.PSECT OFFSETS, ABS
FIRST:	.BLKL 1
SECOND:	.BLKW 1
	.PSECT DATA
	.LONG SECOND`)

	requirePsect(t, a, "OFFSETS", 1, gpsEXE|gpsRD|gpsWRT, 0)
	requireRelocations(t, a)
	requireBytes(t, psectBytes(t, a, "DATA"), 4, 0, 0, 0)

	if s := requireSymbol(t, a, "SECOND", SymLabel); s.sect != nil || s.value != 4 {
		t.Errorf("SECOND = %d in %v, want absolute 4", s.value, s.sect)
	}

	requireMACROError(t, ".PSECT OFFSETS, ABS\n.BYTE 1", vmserrors.VAX_ABSDATA)
	requireMACROError(t, ".PSECT OFFSETS, ABS\nNOP", vmserrors.VAX_ABSDATA)
	requireMACROError(t, ".PSECT OFFSETS, ABS\n.ASCII /A/", vmserrors.VAX_ABSDATA)
}

func TestTooManyPsects(t *testing.T) {
	src := ""
	for i := 0; i < maxUserPsects; i++ {
		src += ".PSECT P" + string(rune('A'+i/26%26)) + string(rune('A'+i%26)) + "\n"
	}

	// 254 named psects, and the blank one, are fine; one more isn't.
	macroAssemble(t, src+".BYTE 1\n.PSECT\n.BYTE 2")
	requireMACROError(t, src+".PSECT ONE_MORE", vmserrors.VAX_TOOMANYPSECTS)
}

// TestSaveRestorePsect checks that .RESTORE_PSECT returns to the saved
// psect and location, and with LOCAL_BLOCK to the saved local label
// block, whose labels a .PSECT in between doesn't end.
func TestSaveRestorePsect(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE
START:	BRB 30$
	.SAVE_PSECT LOCAL_BLOCK
	.PSECT TEXT
MESSAGE::
	.ASCIC /Hi/
	.RESTORE_PSECT
	NOP
30$:	RSB
	.SAVE
	.PSECT TEXT
	.BYTE 9
	.RESTORE
	HALT`)

	requireRelocations(t, a)
	requireBytes(t, psectBytes(t, a, "CODE"), 0x11, 0x01, 0x01, 0x05, 0x00)
	requireBytes(t, psectBytes(t, a, "TEXT"), 2, 'H', 'i', 9)

	requireMACROError(t, ".RESTORE_PSECT", vmserrors.VAX_PSECTSTACK)
	requireMACROError(t, ".SAVE_PSECT LOCALS", vmserrors.VAX_BADKEYWORD)

	src := ""
	for i := 0; i <= maxPsectStack; i++ {
		src += ".SAVE_PSECT\n"
	}

	requireMACROError(t, src, vmserrors.VAX_PSECTSTACK)

	// Without LOCAL_BLOCK, the .PSECT ends the block, and 30$ with it.
	requireMACROError(t, ".PSECT CODE\nBRB 30$\n.SAVE\n.PSECT TEXT\n.RESTORE\n30$: RSB", vmserrors.VAX_UNDEFSYM)
}

// TestAlignMACRO checks MACRO-32's .ALIGN: a keyword or a power of two,
// no more than the psect's alignment, leaving a gap or filling it.
func TestAlignMACRO(t *testing.T) {
	a := macroAssemble(t, `.PSECT DATA, PAGE
	.BYTE 1
	.ALIGN LONG
	.BYTE 2
	.ALIGN 3, ^A/ /
	.BYTE 3
	.ALIGN WORD
	.ALIGN BYTE
	.BYTE 4
	.ALIGN PAGE
	.BYTE 5`)

	s := a.findSection("DATA")
	requireBytes(t, s.img.Bytes(0, 11), 1, 0, 0, 0, 2, ' ', ' ', ' ', 3, 0, 4)

	if s.hi != 513 {
		t.Errorf("allocation = %d, want 513", s.hi)
	}

	requireMACROError(t, ".PSECT DATA, LONG\n.ALIGN QUAD", vmserrors.VAX_ALIGNPSECT)
	requireMACROError(t, ".PSECT DATA\n.ALIGN WORD", vmserrors.VAX_ALIGNPSECT)
	requireMACROError(t, ".PSECT DATA, PAGE\n.ALIGN 10", vmserrors.VAX_DATARANGE)
	requireMACROError(t, ".PSECT DATA, PAGE\n.ALIGN LATER\nLATER=2", vmserrors.VAX_UNDEFSYM)
}

// TestGlobalSymbols checks every way to make a symbol global: "::", "==",
// .ENTRY, and .GLOBAL of a symbol defined here, and that a symbol the
// module declares but never defines is external.
func TestGlobalSymbols(t *testing.T) {
	a := macroAssemble(t, `	.GLOBL	TABLE, OTHER
	.EXTERNAL EXT
	.WEAK	MAYBE, HERE
LIMIT	== 100
	.PSECT	DATA
TABLE:	.LONG	LIMIT, OTHER, MAYBE
HERE::	.LONG	0
LOCAL:	.LONG	0
	.PSECT	CODE
	.ENTRY	MAIN, ^M<R2,R11>
	RET`)

	requireSymbol(t, a, "TABLE", SymGlobal)
	requireSymbol(t, a, "LIMIT", SymGlobal)
	requireSymbol(t, a, "HERE", SymGlobal|SymWeak)
	requireSymbol(t, a, "OTHER", SymGlobal|SymExternal)
	requireSymbol(t, a, "MAYBE", SymGlobal|SymWeak|SymExternal)
	requireSymbol(t, a, "EXT", SymGlobal|SymExternal)

	if s := requireSymbol(t, a, "MAIN", SymGlobal|SymEntry); s.mask != 0x0804 {
		t.Errorf("MAIN's mask = %04X, want 0804", s.mask)
	}

	if s := requireSymbol(t, a, "LOCAL", SymLabel); s.flags&SymGlobal != 0 {
		t.Error("LOCAL is global")
	}

	if got := a.Externals(); got != "EXT,MAYBE,OTHER" {
		t.Errorf("externals = %s", got)
	}

	requireRelocations(t, a, "DATA+4 L OTHER", "DATA+8 L MAYBE")

	requireMACROError(t, "10$:: NOP", vmserrors.VAX_NOTGLOBAL)
	requireMACROError(t, ".GLOBL 10$", vmserrors.VAX_NOTGLOBAL)
	requireMACROError(t, ".GLOBL A B", vmserrors.VAX_EXTRATEXT)
	requireMACROError(t, ".GLOBL A\nA: NOP\nA: NOP", vmserrors.VAX_DUPSYM)
}

// TestDeclaredSymbolIsNotDefined checks that a symbol .GLOBAL named isn't
// usable where a value must already be defined.
func TestDeclaredSymbolIsNotDefined(t *testing.T) {
	requireMACROError(t, ".GLOBL X\n.BLKB X", vmserrors.VAX_UNDEFSYM)
	requireMACROError(t, ".GLOBL X\nY = X", vmserrors.VAX_UNDEFSYM)

	// Once defined, it is.
	a := macroAssemble(t, ".GLOBL X\nX = 3\n.PSECT DATA\n.BLKB X") //nolint:dupword
	if got := a.findSection("DATA").hi; got != 3 {
		t.Errorf("allocation = %d, want 3", got)
	}
}

// TestEnableGlobal checks .DISABLE GLOBAL: an undefined symbol is an
// error unless .EXTERNAL (or .GLOBAL) names it.
func TestEnableGlobal(t *testing.T) {
	requireMACROError(t, ".DISABLE GLOBAL\n.PSECT DATA\n.LONG EXT", vmserrors.VAX_UNDEFSYM)

	a := macroAssemble(t, ".DSABL GBL\n.EXTRN EXT\n.PSECT DATA\n.LONG EXT")
	requireRelocations(t, a, "DATA+0 L EXT")

	macroAssemble(t, ".DISABLE GLOBAL\n.ENABLE GLOBAL\n.PSECT DATA\n.LONG EXT")
}

// TestEnableArguments checks .ENABLE and .DISABLE's argument lists, in
// long and short forms, and the warning for a function govax lacks.
func TestEnableArguments(t *testing.T) {
	a := macroAssemble(t, ".ENABLE ABSOLUTE, DBG SUP\n.DISABLE TRACEBACK")

	if want := enableAbsolute | enableDebug | enableSuppression | enableGlobal; a.enabled != want {
		t.Errorf("enabled = %b, want %b", a.enabled, want)
	}

	if len(a.Warnings()) != 0 {
		t.Errorf("warnings: %v", a.Warnings())
	}

	a = macroAssemble(t, ".PSECT DATA\n\n.ENABLE TRUNCATION")

	warnings := a.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("warnings: %v", warnings)
	}

	var e *Error
	if !errors.As(warnings[0], &e) || e.Line != 3 {
		t.Errorf("warning %v, want one on line 3", warnings[0])
	}

	requireCode(t, warnings[0], vmserrors.VAX_IGNORED)
	requireMACROError(t, ".ENABLE SPEED", vmserrors.VAX_BADKEYWORD)
}

// TestEnableLocalBlock checks that .ENABLE LOCAL_BLOCK holds a local
// label block open across labels and .PSECTs, until .DISABLE LOCAL_BLOCK
// and the next label.
func TestEnableLocalBlock(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE
	.ENABLE LOCAL_BLOCK
A:	BRB 10$
B:	NOP
	.PSECT MORE
	.PSECT CODE
10$:	RSB
	.DISABLE LOCAL_BLOCK
C:	BRB 10$
10$:	RSB`)
	requireBytes(t, psectBytes(t, a, "CODE"), 0x11, 0x01, 0x01, 0x05, 0x11, 0x00, 0x05)

	requireMACROError(t, ".PSECT CODE\nA: BRB 10$\nB: NOP\n10$: RSB", vmserrors.VAX_UNDEFSYM)
}

func TestDefaultDisplacement(t *testing.T) {
	if a := macroAssemble(t, ""); a.defaultDisp != 4 {
		t.Errorf("default displacement = %d, want 4", a.defaultDisp)
	}

	if a := macroAssemble(t, ".DEFAULT DISPLACEMENT, WORD"); a.defaultDisp != 2 {
		t.Errorf("displacement = %d, want 2", a.defaultDisp)
	}

	if a := macroAssemble(t, ".DEFAULT DISPLACEMENT BYTE"); a.defaultDisp != 1 {
		t.Errorf("displacement = %d, want 1", a.defaultDisp)
	}

	requireMACROError(t, ".DEFAULT DISPLACEMENT, QUAD", vmserrors.VAX_BADKEYWORD)
	requireMACROError(t, ".DEFAULT SIZE, LONG", vmserrors.VAX_BADKEYWORD)
}

// TestEntryMACRO checks MACRO-32's .ENTRY mask: any absolute expression,
// but not R0, R1, AP, or FP.
func TestEntryMACRO(t *testing.T) {
	a := macroAssemble(t, ".PSECT CODE\n.ENTRY A, 0\nRET\n.ENTRY B, ^M<R2>!^M<IV>\nRET")
	requireBytes(t, psectBytes(t, a, "CODE"), 0, 0, 4, 0x04, 0x80, 4)

	requireMACROError(t, ".PSECT CODE\n.ENTRY A, ^M<R0>", vmserrors.VAX_ENTRYMASK)
	requireMACROError(t, ".PSECT CODE\n.ENTRY A, ^X1000", vmserrors.VAX_ENTRYMASK)
	requireMACROError(t, ".PSECT CODE\n.ENTRY A, MASK\nMASK = 4", vmserrors.VAX_UNDEFSYM) //nolint:dupword
}

// TestAddress checks .ADDRESS: position-independent longword addresses.
func TestAddress(t *testing.T) {
	a := macroAssemble(t, `.PSECT DATA
ITEM:	.LONG 0
	.ADDRESS ITEM, LATER, EXT+4, 16
LATER:`)

	requireRelocations(t, a,
		"DATA+4 PIDR DATA:0",
		"DATA+8 PIDR DATA:14",
		"DATA+C PIDR EXT 4 +",
	)
	requireBytes(t, psectBytes(t, a, "DATA")[16:], 16, 0, 0, 0)
}

// TestMaskMACRO checks MACRO-32's .MASK: a word the linker fills from an
// entry point's mask, ORed with an expression.
func TestMaskMACRO(t *testing.T) {
	a := macroAssemble(t, `.PSECT CODE
	.ENTRY SUB, ^M<R2>
	RET
	.PSECT VECTOR
	.MASK SUB
	.MASK EXT, ^M<R3>`)

	requireRelocations(t, a, "VECTOR+0 W MASK(SUB)", "VECTOR+2 W MASK(EXT) 8 !")
	requireBytes(t, psectBytes(t, a, "VECTOR"), 0, 0, 0, 0)
	requireSymbol(t, a, "EXT", SymExternal)

	requireMACROError(t, ".PSECT CODE\nNOTENTRY: NOP\n.MASK NOTENTRY", vmserrors.VAX_NOTENTRY)
}

// TestFloatingMACRO checks MACRO-32's names for the floating-point data
// directives.
func TestFloatingMACRO(t *testing.T) {
	a := macroAssemble(t, `.PSECT DATA
	.F_FLOATING 1.0
	.FLOAT 1.0
	.D_FLOATING 1.0
	.DOUBLE 1.0`)

	f := []byte{0x80, 0x40, 0, 0}
	d := []byte{0x80, 0x40, 0, 0, 0, 0, 0, 0}

	var want []byte
	want = append(want, f...)
	want = append(want, f...)
	want = append(want, d...)
	want = append(want, d...)

	requireBytes(t, psectBytes(t, a, "DATA"), want...)
}

// TestConsoleKeepsItsForms checks that the console dialect still has
// eVAX's .ALIGN and .MASK, and takes .TITLE and .IDENT.
func TestConsoleKeepsItsForms(t *testing.T) {
	requireBytes(t, assembleBytes(t, `.TITLE X
	.IDENT /1/
	.BYTE 1
	.ALIGN 4
	.MASK <R2>`), 1, 0, 0, 0, 4, 0)
}
