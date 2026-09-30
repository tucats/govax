package asm

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestMacroDialect_reportsEveryError checks that a MACRO-dialect assembly
// goes on after an error and reports every one, in source order, with the
// errors found at the end (an undefined local label here) last.
func TestMacroDialect_reportsEveryError(t *testing.T) {
	a := New(false)
	a.SetDialect(DialectMACRO)

	_, err := a.Assemble(strings.Join([]string{
		"\t.PSECT\tCODE",
		"\tBOGUS",      // 2
		"\t.LONG\t1",   //
		"\tMOVL\t#1,",  // 4
		"\t.ALIGN\t12", // 5
		"\tBRB\t10$",
	}, "\n"))

	var list *Errors
	if !errors.As(err, &list) {
		t.Fatalf("Assemble error = %v, want an *Errors", err)
	}

	lines := []int{2, 4, 5}
	if len(list.List) != len(lines)+1 {
		t.Fatalf("got %d errors, want %d:\n%v", len(list.List), len(lines)+1, err)
	}

	for i, want := range lines {
		var e *Error
		if !errors.As(list.List[i], &e) || e.Line != want {
			t.Errorf("error %d = %v, want one at line %d", i, list.List[i], want)
		}
	}

	if !errors.Is(err, vmserrors.New(vmserrors.VAX_BADOPCODE)) {
		t.Error("errors.Is doesn't find the first error through *Errors")
	}

	if got := len(strings.Split(err.Error(), "\n")); got != len(list.List) {
		t.Errorf("Error() has %d lines, want one per error", got)
	}
}

// TestMacroDialect_oneErrorAlone checks that one error comes back as
// itself, not in a list.
func TestMacroDialect_oneErrorAlone(t *testing.T) {
	a := New(false)
	a.SetDialect(DialectMACRO)

	_, err := a.Assemble("\t.PSECT\tCODE\n\tBOGUS\n\tRSB")

	var e *Error
	if !errors.As(err, &e) || e.Line != 2 {
		t.Fatalf("Assemble error = %v, want line 2's", err)
	}

	var list *Errors
	if errors.As(err, &list) {
		t.Error("a single error came back as an *Errors")
	}

	// The next assembly starts with no errors.
	if _, err := a.Assemble("\tRSB"); err != nil {
		t.Errorf("second Assemble = %v", err)
	}
}

// TestMacroDialect_errorsInIncludes checks that an error in an included
// file names the .INCLUDE line that led to it, and that assembly of the
// including file goes on past it.
func TestMacroDialect_errorsInIncludes(t *testing.T) {
	a := New(false)
	a.SetDialect(DialectMACRO)
	a.SetIncludeResolver(func(name string) (string, error) {
		switch name {
		case "INNER.MAR":
			return "\t.LONG\t1\n\tBOGUS", nil
		case "OUTER.MAR":
			return "\t.LONG\t2\n\t.INCLUDE \"INNER.MAR\"\n\t.ENABLE\tTRUNCATION", nil
		}

		return "", errors.New("no such file")
	})

	_, err := a.Assemble(strings.Join([]string{
		"\t.PSECT\tDATA",
		"\t.INCLUDE \"OUTER.MAR\"", // 2
		"\tALSO_BOGUS",             // 3
	}, "\n"))

	var list *Errors
	if !errors.As(err, &list) || len(list.List) != 2 {
		t.Fatalf("Assemble error = %v, want two errors", err)
	}

	if got := list.List[0].Error(); !strings.HasPrefix(got, "line 2: line 2: line 2: ") {
		t.Errorf("included error = %q, want it at line 2 of INNER, from line 2 of OUTER, from line 2", got)
	}

	if got := list.List[1].Error(); !strings.HasPrefix(got, "line 3: ") {
		t.Errorf("second error = %q, want it at line 3", got)
	}

	// A warning in an included file names its .INCLUDE line too.
	if w := a.Warnings(); len(w) != 1 || !strings.HasPrefix(w[0].Error(), "line 2: line 3: ") {
		t.Errorf("warnings = %v, want TRUNCATION's at line 3 of OUTER, from line 2", w)
	}
}

// TestConsoleDialect_stopsAtFirstError checks that the console dialect
// still stops at its first error, as eVAX's assembler does.
func TestConsoleDialect_stopsAtFirstError(t *testing.T) {
	a := New(false)

	_, err := a.Assemble("\tBOGUS\n\tALSO_BOGUS")

	var list *Errors
	if errors.As(err, &list) {
		t.Fatalf("console dialect returned %d errors, want only the first", len(list.List))
	}

	var e *Error
	if !errors.As(err, &e) || e.Line != 1 {
		t.Errorf("Assemble error = %v, want line 1's", err)
	}
}
