package anl

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

var update = flag.Bool("update", false, "rewrite testdata/kinds.txt")

// everyCommand returns one of each TIR command, found by decoding each
// command code with zero operands (a counted name of length zero).
func everyCommand(t *testing.T) []obj.Command {
	t.Helper()

	var out []obj.Command

	for code := range 0x80 {
		b := append([]byte{byte(code)}, make([]byte, 16)...)

		c, _, err := obj.DecodeCommand(b)
		if err != nil {
			continue
		}

		// Give each operand something to show.
		c.Name, c.Psect, c.Value, c.Index, c.Pos, c.Size = "SYM", 0, 0xFFFFFFFC, 2, 3, 5
		if c.Op.String() == "STA_CKARG" {
			c.Data = []byte{1, 2, 0xAA, 0xBB}
		} else if c.Op.String() == "STO_RIVB" {
			c.Data = []byte{0x10, 0x20}
		}

		out = append(out, c)
	}

	if len(out) < 60 {
		t.Fatalf("found %d TIR commands, want them all", len(out))
	}

	return out
}

// TestEveryCommandShown checks that ANALYZE/OBJECT names every TIR
// command and shows each operand a command has.
func TestEveryCommandShown(t *testing.T) {
	for _, c := range everyCommand(t) {
		b, err := obj.EncodeRecord(&obj.TIR{Type: obj.RecTIR, Commands: []obj.Command{c}})
		if err != nil {
			t.Fatalf("%s: %v", c.Op, err)
		}

		var a objectAnalyzer
		a.command(true, 1, c)

		heading := a.lines[0].Text
		if !strings.Contains(heading, "TIR$C_"+commandName(c.Op)+" (") {
			t.Errorf("%s: heading %q", c.Op, heading)
		}

		// A command with operands (more than its code byte) shows them.
		if len(b) > 2 && len(a.lines) < 2 {
			t.Errorf("%s has operands but shows none", c.Op)
		}
	}
}

// everyKind is a module with one of every record type, header type, GSD
// subrecord type, and TIR command.
func everyKind(t *testing.T) [][]byte {
	t.Helper()

	formals := &obj.Formals{Min: 1, Max: 3, Args: []obj.FormalArg{{ValCtl: 1}, {ValCtl: 2}, {ValCtl: 3, Detail: []byte{9}}}}
	def := obj.SymDEF | obj.SymREL

	var symbols []obj.Subrecord

	for _, typ := range []obj.GSDType{
		obj.GSDSymbol, obj.GSDEntry, obj.GSDProcedure, obj.GSDSymbolW, obj.GSDEntryW, obj.GSDProcedureW,
		obj.GSDLocalSymbol, obj.GSDLocalEntry, obj.GSDLocalProc, obj.GSDSymbolV, obj.GSDEntryV,
		obj.GSDProcedureV, obj.GSDSymbolM, obj.GSDEntryM, obj.GSDProcedureM,
	} {
		s := &obj.Symbol{Type: typ, Flags: def, Env: 1, Value: 8, Extra: 0x1234, Mask: 0xC00C, Name: typ.String() + "_SYM"}
		if strings.HasPrefix(typ.String(), "PRO") || typ == obj.GSDLocalProc {
			s.Formals = formals
		}

		symbols = append(symbols, s)
	}

	symbols = append(symbols,
		&obj.Symbol{Type: obj.GSDSymbol, DataType: 8, Flags: obj.SymWEAK, Name: "REFERENCE"},
		&obj.IdentCheck{Flags: 1<<1 | 2<<3, Name: "ENTITY", Ident: []byte("V1.0"), Object: "OBJECT"},
		&obj.IdentCheck{Flags: 1, Name: "ENTITY", Ident: []byte{1, 0, 2, 0}, Object: "OBJECT"},
		&obj.Environment{Flags: 3, Parent: 0, Name: "ENVIRONMENT"},
	)

	// Each command, with what it pops pushed before it and what it
	// pushes stored after it, so the stack balances.
	var commands []obj.Command

	for _, c := range everyCommand(t) {
		pop, push := c.Op.StackEffect()

		for range pop {
			commands = append(commands, obj.Command{Op: obj.OpStackLong, Value: 1})
		}

		commands = append(commands, c)

		for range push {
			commands = append(commands, obj.Command{Op: obj.OpStoreLong})
		}
	}

	return encode(t,
		&obj.MainHeader{MaxRecordSize: 512, Name: "KINDS", Version: "V1.0", Created: " 4-OCT-2026 09:00", Patched: " 5-OCT-2026 10:00"},
		&obj.TextHeader{Type: obj.HdrLNM, Text: "VAX MACRO V5.4-3"},
		&obj.TextHeader{Type: obj.HdrCPR, Text: "Copyright text"},
		&obj.TextHeader{Type: obj.HdrMTC, Text: "Maintenance status"},
		&obj.TextHeader{Type: obj.HdrGTX, Text: "General text"},
		&obj.GSD{Subrecords: []obj.Subrecord{
			&obj.Psect{Align: 9, Flags: obj.PsectREL | obj.PsectRD, Alloc: 512, Name: "PAGE"},
			&obj.Psect{Shared: true, Align: 2, Flags: obj.PsectLIB, Alloc: 16, Base: 0x200, Name: "SHARED"},
		}},
		&obj.GSD{Subrecords: symbols},
		&obj.TIR{Type: obj.RecTIR, Commands: commands},
		&obj.LNK{Type: byte(objConst("LNK$C_OLB")), Name: "MYLIB.OLB"},
		&obj.LNK{Type: byte(objConst("LNK$C_OLI")), Name: "MYLIB.OLB", Rest: []byte{3, 'M', 'O', 'D'}},
		&obj.EOM{Word: true, Severity: obj.SeverityWarning, HasTransfer: true, Psect: 0, Transfer: 4, HasFlags: true, Flags: 1},
	)
}

// TestEveryKind analyzes a module with one of everything, checks that
// nothing is shown as unknown, and compares the report with
// testdata/kinds.txt, where the layouts no real ANALYZE output settles
// can be reviewed (go test -update rewrites it).
func TestEveryKind(t *testing.T) {
	rep := AnalyzeObject(everyKind(t), ObjectOptions{})
	if rep.Errors != 0 {
		t.Errorf("%d errors, want none", rep.Errors)
	}

	var b bytes.Buffer
	if err := WriteText(&b, rep.Lines); err != nil {
		t.Fatal(err)
	}

	text := b.String()

	for _, bad := range []string{"UNKNOWN", "Unknown", "unknown", "TIR command"} {
		if strings.Contains(text, bad) {
			t.Errorf("the report shows %q", bad)
		}
	}

	const golden = "testdata/kinds.txt"

	if *update {
		if err := os.WriteFile(golden, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}

	if diff := firstDiff(string(want), text); diff != "" {
		t.Errorf("%s: %s", golden, diff)
	}
}
