package asm

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// Tests for the message and listing-control directives
// (docs/PHASE-28.md, subtask 4).

// requireMessage fails the test unless err is the status code's, and its
// text ends with want.
func requireMessage(t *testing.T, err error, code uint32, want string) {
	t.Helper()

	requireCode(t, err, code)

	if !strings.HasSuffix(err.Error(), want) {
		t.Errorf("error = %q, want it to end with %q", err.Error(), want)
	}
}

// TestErrorDirective is the manual's .ERROR example: the value, a blank,
// then the comment as written, its leading blank and a library comment's
// closing ";" included (real MACRO keeps both;
// testdata/mar/list/vax/errors.lis and testdata/mar/macros/vax/macros.log).
func TestErrorDirective(t *testing.T) {
	src := `
LONG_MESS = 1
WORK_AREA = 900
	.IF	DEFINED	LONG_MESS
	.IF	GREATER	1000-WORK_AREA
	.ERROR	25		; Need larger WORK_AREA;
	.ENDC
	.ENDC`

	err := macroErr(t, src)
	requireMessage(t, err, vmserrors.VAX_GENERR, "Generated ERROR: 25  Need larger WORK_AREA;")

	var located *Error
	if !errors.As(err, &located) || located.Line != 6 {
		t.Errorf("error = %v, want it on line 6", err)
	}
}

// TestErrorInMacro: a macro's arguments are substituted into the comment,
// and a zero value isn't shown. The comment keeps its case.
func TestErrorInMacro(t *testing.T) {
	src := `
	.MACRO	CHECK	ARG1
	.IIF	NE,%LENGTH(ARG1)-3,	.ERROR	0 ; Argument ARG1 is not 3 characters;
	.ENDM	CHECK
	CHECK	ABC
	CHECK	WXYZ`

	requireMessage(t, macroErr(t, src), vmserrors.VAX_GENERR, "Generated ERROR:  Argument WXYZ is not 3 characters;")
}

// TestErrorSkipped: a message directive in a conditional's false branch,
// as in STARLET.MLB's argument checks, does nothing.
func TestErrorSkipped(t *testing.T) {
	src := `
	.PSECT	DATA
	.IF	DEFINED	NOPE
	.ERROR	; not assembled
	.WARN	; not assembled
	.ENDC
	.BYTE	1`

	a := macroAssembler()

	out, err := a.Assemble(src)
	if err != nil {
		t.Fatal(err)
	}

	requireBytes(t, out, 1)

	if len(a.Warnings()) != 0 {
		t.Errorf("warnings = %v, want none", a.Warnings())
	}
}

// TestWarnDirective is the manual's .WARN example: a warning, and
// assembly goes on.
func TestWarnDirective(t *testing.T) {
	src := `
FULL = 1
DOUBLE_PREC = 1
	.PSECT	DATA
	.IF	DEFINED	FULL
	.IF	DEFINED	DOUBLE_PREC
	.WARN		; This combination not tested
	.ENDC
	.ENDC
	.BYTE	2`

	a := macroAssembler()

	out, err := a.Assemble(src)
	if err != nil {
		t.Fatal(err)
	}

	requireBytes(t, out, 2)

	if len(a.Warnings()) != 1 {
		t.Fatalf("warnings = %v, want one", a.Warnings())
	}

	requireMessage(t, a.Warnings()[0], vmserrors.VAX_GENWRN, "Generated WARNING:  This combination not tested")
}

// TestPrintDirective is the manual's .PRINT example: an informational
// message, neither an error nor a warning.
func TestPrintDirective(t *testing.T) {
	src := `
	.PRINT	2	; The sine routine has been changed
	.PRINT	1+ -
		2	; Continued`

	a := macroAssembler()

	if _, err := a.Assemble(src); err != nil {
		t.Fatal(err)
	}

	if len(a.Warnings()) != 0 {
		t.Errorf("warnings = %v, want none", a.Warnings())
	}

	msgs := a.Messages()
	if len(msgs) != 2 {
		t.Fatalf("messages = %v, want two", msgs)
	}

	// Displayed bare: MACRO adds no prefix of its own. The comment keeps
	// its leading blank after the value.
	if msgs[0] != "2  The sine routine has been changed" || msgs[1] != "3  Continued" {
		t.Errorf("messages = %q", msgs)
	}
}

// TestAlignmentCheck is a macro that checks its block is longword
// aligned, as the RMS block macros do: a conditional tests a relocatable
// value by its offset in its psect, and the .PRINT comment carries its own
// message prefix (the message real MACRO prints for a misaligned $FAB).
func TestAlignmentCheck(t *testing.T) {
	src := `
	.MACRO	CHKALIGN
	.IIF NE .&3, .print ;%MACRO-I-GENINFO, Generated INFO: RMS BLOCK NOT LONGWORD ALIGNED;
	.LONG	0
	.ENDM	CHKALIGN
	.PSECT	DATA,LONG
	CHKALIGN
	.BYTE	1
	CHKALIGN
	.ALIGN	LONG
	CHKALIGN`

	a := macroAssembler()

	if _, err := a.Assemble(src); err != nil {
		t.Fatal(err)
	}

	want := "%MACRO-I-GENINFO, Generated INFO: RMS BLOCK NOT LONGWORD ALIGNED;"
	if msgs := a.Messages(); len(msgs) != 1 || msgs[0] != want {
		t.Errorf("messages = %q, want [%q]", msgs, want)
	}
}

// TestMessageErrors: the expression must be absolute and defined, with
// nothing after it but the comment.
func TestMessageErrors(t *testing.T) {
	for _, src := range []string{
		"\t.ERROR\tUNDEFINED\t; message",
		"\t.WARN\t1 2\t; message",
	} {
		if err := macroErr(t, src); strings.Contains(err.Error(), "GENERR") {
			t.Errorf("%q: error = %v, want an expression error", src, err)
		}
	}
}

// TestListingDirectives: the listing-control directives are accepted,
// with their arguments, and assemble nothing.
func TestListingDirectives(t *testing.T) {
	src := `
	.PSECT	DATA
	.LIST	MEB
	.NLIST	CND,ME
	.SHOW	EXPANSIONS
	.NOSHOW
	.CROSS
	.NOCROSS	A,B
	.PAGE
	.BYTE	3`

	requireBytes(t, macroBytes(t, src), 3)
}

// TestConsoleDialectMessages: the console dialect has .ERROR and .WARN,
// and keeps eVAX's .PRINT.
func TestConsoleDialectMessages(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble(`.PRINT "HELLO", 12`); err != nil {
		t.Fatal(err)
	}

	if got := a.Prints(); len(got) != 1 || got[0] != "HELLO12" {
		t.Errorf("prints = %q, want [HELLO12]", got)
	}

	_, err := New(false).Assemble(".ERROR 5 ; stop here")
	requireMessage(t, err, vmserrors.VAX_GENERR, "Generated ERROR: 5  stop here")
}
