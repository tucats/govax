package anl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// encode turns records into bytes for AnalyzeObject.
func encode(t *testing.T, recs ...obj.Record) [][]byte {
	t.Helper()

	out := make([][]byte, len(recs))

	for i, r := range recs {
		b, err := obj.EncodeRecord(r)
		if err != nil {
			t.Fatal(err)
		}

		out[i] = b
	}

	return out
}

// The records of a small, correct module.
var (
	mhd = &obj.MainHeader{MaxRecordSize: 512, Name: "TEST", Version: "V1.0", Created: " 4-OCT-2026 09:00", Patched: strings.Repeat(" ", 17)}
	lnm = &obj.TextHeader{Type: obj.HdrLNM, Text: "VAX MACRO V5.4-3"}
	gsd = &obj.GSD{Subrecords: []obj.Subrecord{&obj.Psect{Align: 2, Flags: obj.PsectREL, Alloc: 4, Name: "DATA"}}}
	tir = &obj.TIR{Type: obj.RecTIR, Commands: []obj.Command{
		{Op: obj.OpStackPsectLong, Psect: 0},
		{Op: obj.OpSetRelocBase},
		{Op: obj.OpStoreImmediate, Data: []byte{1, 2, 3, 4}},
	}}
	eom = &obj.EOM{}
)

// errorLines returns a report's error lines.
func errorLines(rep ObjectReport) []string {
	var out []string

	for _, l := range rep.Lines {
		if strings.HasPrefix(l.Text, "***  ") {
			out = append(out, strings.TrimPrefix(l.Text, "***  "))
		}
	}

	return out
}

func TestObjectErrors(t *testing.T) {
	stoL, _ := obj.OpByName("STO_L")

	tests := []struct {
		name    string
		records [][]byte
		want    []string
	}{
		{"correct", encode(t, mhd, lnm, gsd, tir, eom), nil},
		{"two modules", encode(t, mhd, lnm, gsd, tir, eom, mhd, lnm, gsd, tir, eom), nil},
		{"no main header", encode(t, lnm, gsd, tir, eom), []string{"The module header record is missing."}},
		{"no end of module", encode(t, mhd, lnm, gsd, tir), []string{"End of module record is missing from previous module."}},
		{"no end before the next module", encode(t, mhd, lnm, gsd, tir, mhd, lnm, gsd, tir, eom),
			[]string{"End of module record is missing from previous module."}},
		{"stack left", encode(t, mhd, lnm, gsd, &obj.TIR{Type: obj.RecTIR, Commands: []obj.Command{{Op: obj.OpStackLong, Value: 1}}}, eom),
			[]string{"The stack still contains 1 longword."}},
		{"stack underflow", encode(t, mhd, lnm, gsd, &obj.TIR{Type: obj.RecTIR, Commands: []obj.Command{{Op: stoL}}}, eom),
			[]string{"The stack underflowed."}},
		{"undefined psect", encode(t, mhd, lnm, gsd, &obj.TIR{Type: obj.RecTIR, Commands: []obj.Command{{Op: obj.OpStackPsectLong, Psect: 3}, {Op: obj.OpSetRelocBase}}}, eom),
			[]string{"Psect 3 is undefined."}},
		{"psect defined later", encode(t, mhd, lnm, tir, gsd, eom), nil},
		{"undefined transfer psect", encode(t, mhd, lnm, gsd, &obj.EOM{HasTransfer: true, Psect: 1}), []string{"Psect 1 is undefined."}},
		{"undefined severity", encode(t, mhd, lnm, gsd, &obj.EOM{Severity: 7}), []string{"Severity 7 is undefined."}},
		{"long name", encode(t, mhd, lnm, &obj.GSD{Subrecords: []obj.Subrecord{&obj.Psect{Name: strings.Repeat("P", 32)}}}, eom),
			[]string{"The psect name must be 1 to 31 characters."}},
		{"unknown record", append(encode(t, mhd, lnm, gsd), []byte{99, 1}, encode(t, eom)[0]), []string{"Record type 99 is undefined."}},
		{"too long", encode(t, &obj.MainHeader{MaxRecordSize: 20, Name: "TEST", Version: "V1.0", Created: mhd.Created}, lnm, gsd, tir, eom),
			[]string{"The record is longer than the maximum record size, 20 bytes."}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rep := AnalyzeObject(tt.records, ObjectOptions{})
			got := errorLines(rep)

			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") || rep.Errors != len(tt.want) {
				t.Errorf("errors %q (count %d), want %q", got, rep.Errors, tt.want)
			}
		})
	}
}

// TestObjectMalformedCommand checks that a record's commands are shown up
// to a malformed one, which is reported in its place.
func TestObjectMalformedCommand(t *testing.T) {
	records := encode(t, mhd, lnm, gsd, eom)

	good, err := obj.EncodeRecord(tir)
	if err != nil {
		t.Fatal(err)
	}

	// STA_LW (3) with only two of its four operand bytes.
	bad := append(append([]byte(nil), good...), 3, 0xAA, 0xBB)
	records = append(records[:3], bad, records[3])

	rep := AnalyzeObject(records, ObjectOptions{})

	var b strings.Builder
	for _, l := range rep.Lines {
		b.WriteString(l.Text + "\n")
	}

	text := b.String()
	if !strings.Contains(text, "\t3)  Store Immediate, 4 bytes:") {
		t.Errorf("the good commands aren't shown:\n%s", text)
	}

	if got := errorLines(rep); len(got) != 1 || !strings.HasPrefix(got[0], "Command 4 is malformed: ") {
		t.Errorf("errors %q, want command 4 malformed", got)
	}
}
