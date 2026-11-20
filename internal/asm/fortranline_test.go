package asm

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/fortran/dbgtable"
	"github.com/tucats/govax/internal/obj"
)

// fortranLineSource is a MACRO file as the FORTRAN compiler writes one: FORTRAN
// lines as comments before their code, the marker at the top unless marker is
// false, and .ENABLE DEBUG if debug.
func fortranLineSource(marker, debug bool) string {
	var b strings.Builder

	b.WriteString("\t.TITLE\tSUB\n\t.IDENT\t/01/\n")

	if debug {
		b.WriteString("\t.ENABLE\tDEBUG\n")
	}

	if marker {
		b.WriteString(dbgtable.TracebackMarker + "\n")
	}

	b.WriteString(`; Storage map for SUB, a subroutine
	.PSECT	$LOCAL,PIC,CON,REL,LCL,NOSHR,NOEXE,RD,WRT,LONG
SUB.I:	.BLKL	1
	.PSECT	$CODE,PIC,CON,REL,LCL,SHR,EXE,RD,NOWRT,LONG
;    3        SUBROUTINE SUB
	.ENTRY	SUB,^M<>
;    4        INTEGER I
;    5        I = 1
	MOVL	#1,SUB.I
	INCL	SUB.I
;    9        CALL OTHER(I)
	PUSHAL	SUB.I
	CALLS	#1,G^OTHER
;   10        END
	RET
	.END
`)

	return b.String()
}

// tracebackLineRecords returns the line-number records among m's traceback records.
func tracebackLineRecords(t *testing.T, m *obj.Module) []obj.DSTRecord {
	t.Helper()

	var groups [][]obj.Command

	for _, r := range m.Records {
		if tir, ok := r.(*obj.TIR); ok && tir.Type == obj.RecTBT {
			groups = append(groups, tir.Commands)
		}
	}

	recs, _, err := obj.DecodeDST(groups)
	if err != nil {
		t.Fatal(err)
	}

	var lines []obj.DSTRecord

	for _, r := range recs {
		if r.Type == obj.DSTLineNumbers {
			lines = append(lines, r)
		}
	}

	return lines
}

// fortranLineObject assembles src and returns its object.
func fortranLineObject(t *testing.T, src string) *obj.Module {
	t.Helper()

	m, err := macroAssemble(t, src).Object(ObjectOptions{Created: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// TestFortranTracebackTable holds the line-number table govax's MACRO gives a
// compiled FORTRAN program's traceback records (fortranline.go): rows numbered by
// the FORTRAN lines, none without the marker or under /DEBUG.
func TestFortranTracebackTable(t *testing.T) {
	lines := tracebackLineRecords(t, fortranLineObject(t, fortranLineSource(true, false)))
	if len(lines) != 1 {
		t.Fatalf("%d line-number records in the traceback records, want 1", len(lines))
	}

	// The table's start; the .ENTRY's row at line 3 (its address the linker's:
	// zeros here); line 5's MOVL and INCL one row, after the mask's 2 bytes; line
	// 9 four lines on, after their 13 (each operand in $LOCAL a longword
	// displacement); line 10 the next line, after PUSHAL's and CALLS's 13; and the
	// RET's 1 byte ending the segment.
	want := []byte{
		0x13, 0x00, // SET_LINUM_B 0
		0x10, 0, 0, 0, 0, 0x13, 0x02, 0x00, // SET_ABS_PC, SET_LINUM_B 2, a row of line 3
		0x02, 0x01, 0xFE, // INCR_LINUM 1, a row of line 5 after 2 bytes
		0x02, 0x03, 0xF3, // INCR_LINUM 3, a row of line 9 after 13 bytes
		0xF3,       // a row of line 10 after 13 bytes
		0x0E, 0x01, // TERM: the last row is 1 byte
	}
	if !bytes.Equal(lines[0].Data, want) {
		t.Errorf("the table is % X, want % X", lines[0].Data, want)
	}

	if n := len(lines[0].Addresses); n != 1 {
		t.Errorf("%d addresses in the table, want 1", n)
	}

	if got := tracebackLineRecords(t, fortranLineObject(t, fortranLineSource(false, false))); len(got) != 0 {
		t.Errorf("without the marker, %d line-number records", len(got))
	}

	if got := tracebackLineRecords(t, fortranLineObject(t, fortranLineSource(true, true))); len(got) != 0 {
		t.Errorf("under /DEBUG, %d line-number records in the traceback records", len(got))
	}
}

// TestFortranMarkerChangesOnlyTraceback holds an object with the marker to the
// same object without it but for the traceback records, so that a MACRO file
// without one assembles as VMS's MACRO assembles it.
func TestFortranMarkerChangesOnlyTraceback(t *testing.T) {
	strip := func(m *obj.Module) []obj.Record {
		var out []obj.Record

		for _, r := range m.Records {
			if tir, ok := r.(*obj.TIR); ok && tir.Type == obj.RecTBT {
				continue
			}

			out = append(out, r)
		}

		return out
	}

	with := strip(fortranLineObject(t, fortranLineSource(true, false)))
	without := strip(fortranLineObject(t, fortranLineSource(false, false)))

	a, err := obj.Encode(&obj.Module{Records: with})
	if err != nil {
		t.Fatal(err)
	}

	b, err := obj.Encode(&obj.Module{Records: without})
	if err != nil {
		t.Fatal(err)
	}

	if !slices.EqualFunc(a, b, bytes.Equal) {
		t.Error("the marker changed more than the traceback records")
	}
}
