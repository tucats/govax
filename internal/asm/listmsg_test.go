package asm

import (
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/vmserrors"
)

// This file checks how a listing shows errors and warnings (macmsg.go,
// listpage.go's listNotedLine) beyond what the probe's errors.lis, which
// TestFixtureListings compares whole, shows.

// sourcePageLines returns the lines of a's listing from its first source
// line up to the closing pages, without page headings.
func sourcePageLines(a *Assembler) []string {
	var out []string

	lines := a.Listing(ListingOptions{Assembled: time.Now(), Assembler: "govax MACRO V0.0-0"})

	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "\f") {
			if i+1 < len(lines) && (strings.HasPrefix(lines[i+1], labelSymbols) || strings.HasPrefix(lines[i+1], labelSynopsis)) {
				break
			}

			i += listHeadLines - 1

			continue
		}

		out = append(out, lines[i])
	}

	return out
}

// TestTextColumn checks the columns a "!" can mark: a character's, a
// tab's last, and the end of the line's.
func TestTextColumn(t *testing.T) {
	cases := []struct {
		text string
		i    int
		want int
	}{
		{"DUP:\t.LONG", 3, 3},
		{"DUP:\t.LONG", 4, 7},
		{"\t.ENDR", 0, 7},
		{"\tMOVL\tR0", 6, 16},
		{"ABC", 3, 3},
		{"\t", 5, 8},
	}

	for _, c := range cases {
		if got := textColumn(c.text, c.i); got != c.want {
			t.Errorf("textColumn(%q, %d) = %d, want %d", c.text, c.i, got, c.want)
		}
	}
}

// TestMacroMessageFor checks MACRO's words for govax's errors: an
// operand's error seen through the error naming the operand, the errors
// the statement tells apart, and an error MACRO has no message for, which
// keeps govax's own.
func TestMacroMessageFor(t *testing.T) {
	instruction := &listLine{instruction: true}
	show := &listLine{op: ".SHOW"}
	data := &listLine{op: ".LONG"}

	cases := []struct {
		line   *listLine
		err    error
		want   string
		column bool
	}{
		{instruction, vmserrors.Wrap(vmserrors.VAX_OPERANDERR, vmserrors.New(vmserrors.VAX_SHORTRANGE), "MOVL", 1),
			"%MACRO-W-DATATRUNC, Data truncation error", false},
		{instruction, vmserrors.New(vmserrors.VAX_EXTRATEXT, "R2"), "%MACRO-E-TOOMNYOPND, Too many operands for instruction", true},
		{data, vmserrors.New(vmserrors.VAX_EXTRATEXT, "X"), "%MACRO-E-DIRSYNX, Directive syntax error", true},
		{show, vmserrors.New(vmserrors.VAX_BADKEYWORD, ".SHOW", "XX"), "%MACRO-E-NOTLGLISOP, Not a legal listing option", true},
		{data, vmserrors.New(vmserrors.VAX_UNDEFSYM, "X"), "%MACRO-E-UNDEFSYM, Undefined symbol", true},
		{data, vmserrors.New(vmserrors.VAX_GENWRN, " careful"), "%MACRO-W-GENWRN, Generated WARNING:  careful", false},
		{data, vmserrors.New(vmserrors.VAX_NOENDM, "OPEN"), "%VAX-E-NOENDM, Missing .ENDM for macro OPEN", false},
	}

	for _, c := range cases {
		m := c.line.macroMessageFor(listNote{err: c.err, column: 10})
		if m.text != c.want || m.column != c.column {
			t.Errorf("%v: %q (column %v), want %q (column %v)", c.err, m.text, m.column, c.want, c.column)
		}
	}
}

// TestListingTwoMessages checks a line with two messages: each is listed
// where it was raised, with the bytes stored between them on a line of
// their own. No real listing has two on one line, so this is the rule
// errors.lis shows for one, applied twice.
func TestListingTwoMessages(t *testing.T) {
	a := recordListing(t, "\t.PSECT\tD\n\t.BYTE\t300, 400\n", true)

	got := sourcePageLines(a)
	want := []string{
		"                                 00000000     1 \t.PSECT\tD",
		"                                     0000     2 \t.BYTE\t300, 400",
		"%MACRO-W-DATATRUNC, Data truncation error",
		"                                 2C  0000       ",
		"%MACRO-W-DATATRUNC, Data truncation error",
		"                                 90  0001       ",
	}

	compareLines(t, "source pages", got, want)
}

// TestListingEndErrors checks the errors found only at the end of a
// source: listed after its last line, and counted on that line in the
// summary. Real MACRO ran out of memory on the probe's source for them
// (errend.mar), so this is govax's own layout.
func TestListingEndErrors(t *testing.T) {
	a := recordListing(t, "\t.PSECT\tD\n\t.IF\tEQ 0\n\t.MACRO\tOPEN\n\t.LONG\t1\n", true)

	got := sourcePageLines(a)
	want := []string{
		"                                 00000000     1 \t.PSECT\tD",
		"                           00000000  0000     2 \t.IF\tEQ 0",
		"                                     0000     3 \t.MACRO\tOPEN",
		"                                     0000     4 \t.LONG\t1",
		"%VAX-E-NOENDM, Missing .ENDM for macro OPEN",
		"%MACRO-E-UNTERMCOND, Unterminated conditional",
	}

	compareLines(t, "source pages", got, want)

	listing := strings.Join(a.Listing(ListingOptions{}), "\n")
	if !strings.Contains(listing, "There were 2 errors, 0 warnings and 0 information messages, on lines:\n    4 (1)     ") {
		t.Errorf("summary doesn't count 2 errors on line 4:\n%s", listing)
	}
}

// TestRecoveryIsMACROOnly checks that the errors real MACRO assembles
// past still end the statement in the console dialect, and that the
// checks real MACRO makes and eVAX didn't (PC as a register, an index
// register that's the base's) are the MACRO dialect's only.
func TestRecoveryIsMACROOnly(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble(".BYTE 300"); err == nil {
		t.Error(".BYTE 300: no error in the console dialect")
	}

	for _, src := range []string{"CLRL PC", "MOVL (R0)+[R0], R1"} {
		if _, err := New(true).Assemble(src); err != nil {
			t.Errorf("%s: %v in the console dialect", src, err)
		}

		if _, err := macroAssembler().Assemble("\t" + src + "\n"); err == nil {
			t.Errorf("%s: no error in the MACRO dialect", src)
		}
	}
}
