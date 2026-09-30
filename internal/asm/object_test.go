package asm

import (
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmserrors"
)

// objectDump assembles src in the MACRO dialect, builds its object, checks
// that the object encodes, decodes, and passes Check, and returns its dump
// without the headers (everything before the first GSD record).
func objectDump(t *testing.T, src string) string {
	t.Helper()

	a := macroAssemble(t, src)

	m, err := a.Object(ObjectOptions{Created: time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := obj.Encode(m)
	if err != nil {
		t.Fatal(err)
	}

	back, err := obj.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	if problems := obj.Check(back); len(problems) > 0 {
		t.Fatalf("Check: %v", problems)
	}

	text := dumpText(t, back)

	return text[strings.Index(text, "GSD"):]
}

// requireDump checks that dump holds each of want's lines, in order,
// ignoring leading blanks and record numbers.
func requireDump(t *testing.T, dump string, want ...string) {
	t.Helper()

	var lines []string

	for _, line := range strings.Split(dump, "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, ". "); i > 0 && strings.Trim(line[:i], "0123456789") == "" {
			line = line[i+2:]
		}

		lines = append(lines, line)
	}

	i := 0

	for _, line := range lines {
		if i < len(want) && line == want[i] {
			i++
		}
	}

	if i < len(want) {
		t.Fatalf("dump has no %q after the lines before it:\n%s", want[i], dump)
	}
}

func TestObjectHeaders(t *testing.T) {
	a := macroAssemble(t, ".TITLE HELLO Says hello\n.IDENT /X-1/")

	m, err := a.Object(ObjectOptions{Created: time.Date(2026, 9, 29, 12, 34, 0, 0, time.UTC), Source: "MACRO HELLO"})
	if err != nil {
		t.Fatal(err)
	}

	dump := dumpText(t, m)
	requireDump(t, dump,
		`HDR MHD: module "HELLO", version "X-1"`,
		`created "29-SEP-2026 12:34", structure level 0, maximum record size 512`,
		`HDR LNM: "govax MACRO"`,
		`HDR SRC: "MACRO HELLO"`,
		`HDR TTL: "Says hello"`,
		"EOM: severity SUCCESS")

	if strings.Contains(dump, "transfer") {
		t.Error("a transfer address without .END naming one")
	}
}

// TestObjectGlobals checks the first GSD record: every global symbol but
// the entry points, sorted, with a definition's psect and value, a weak
// symbol's flag, and REL on every reference.
func TestObjectGlobals(t *testing.T) {
	requireDump(t, objectDump(t, `.WEAK MAYBE, SOFT
LIMIT == 100
.PSECT DATA
SOFT::	.LONG EXT, MAYBE
.PSECT CODE
.ENTRY MAIN, ^M<R2>
RET`),
		`SYM "EXT" (reference), REL`,
		`SYM "LIMIT" = 0x64, DEF`,
		`SYM "MAYBE" (reference), WEAK,REL`,
		`SYM "SOFT" = 0x0 in psect 1, WEAK,DEF,REL`,
		`PSC 0: ".  ABS  .", alignment BYTE, NOPIC,CON,ABS,LCL,NOSHR,NOEXE,NORD,NOWRT,NOVEC, 0 bytes`,
		`EPM "MAIN" = 0x0 in psect 2, DEF,REL, mask 0x0004`,
		"EOM: severity SUCCESS")
}

// TestObjectPsectReentry checks going back to a psect already defined: a
// new record sets the location to where it left off.
func TestObjectPsectReentry(t *testing.T) {
	requireDump(t, objectDump(t, `.PSECT A
.BYTE 1
.PSECT B
.BYTE 2
.PSECT A
.BYTE 3`),
		`PSC 1: "A", alignment BYTE, NOPIC,CON,REL,LCL,NOSHR,EXE,RD,WRT,NOVEC, 2 bytes`,
		"STA_PB psect 1 offset 0x0", "CTL_SETRB", "STO_IMM 1 bytes: 01",
		`PSC 2: "B", alignment BYTE, NOPIC,CON,REL,LCL,NOSHR,EXE,RD,WRT,NOVEC, 1 bytes`,
		"STA_PB psect 2 offset 0x0", "CTL_SETRB", "STO_IMM 1 bytes: 02",
		"TIR", "STA_PB psect 1 offset 0x1", "CTL_SETRB", "STO_IMM 1 bytes: 03")
}

// TestObjectLocation checks ". =", the stack forms of a psect offset past
// a byte and a word, and a gap.
func TestObjectLocation(t *testing.T) {
	requireDump(t, objectDump(t, `.PSECT A
. = ^X100
.LONG LATER
.BLKB ^X10000
LATER:	.BYTE 7`),
		"STA_PB psect 1 offset 0x0", "CTL_SETRB",
		"STA_PW psect 1 offset 0x100", "CTL_SETRB",
		"STA_PL psect 1 offset 0x10104", "STO_L",
		"CTL_AUGRB 0x10000",
		"STO_IMM 1 bytes: 07")
}

// TestObjectMask checks that .MASK stacks the entry point's mask with
// STA_EPM.
func TestObjectMask(t *testing.T) {
	requireDump(t, objectDump(t, `.PSECT V
.MASK SUB, ^M<R3>`),
		`SYM "SUB" (reference), REL`,
		`STA_EPM "SUB"`, "STA_UB 0x8", "OPR_IOR", "STO_W")
}

// TestObjectOperands checks that an operand's value is stacked before its
// mode byte is stored, for each mode that can be relocated, and that a
// constant is stacked in its shortest form.
func TestObjectOperands(t *testing.T) {
	requireDump(t, objectDump(t, `.PSECT CODE
	MOVL #EXT, R0
	TSTB EXT(R3)
	CLRL G^EXT
	.LONG EXT+^X1234, EXT-1, EXT+^X12345678`),
		"STO_IMM 1 bytes: d0", `STA_GBL "EXT"`, "STO_IMM 1 bytes: 8f", "STO_L",
		"STO_IMM 2 bytes: 50 95", `STA_GBL "EXT"`, "STO_IMM 1 bytes: c3", "STO_W",
		"STO_IMM 1 bytes: d4", `STA_GBL "EXT"`, "STO_PICR",
		"STA_UW 0x1234", "OPR_ADD", "STO_L",
		"STA_UB 0x1", "OPR_SUB", "STO_L",
		"STA_LW 0x12345678", "OPR_ADD", "STO_L")
}

// TestObjectLongData checks that a long run of data is split into STORE
// IMMEDIATE commands of at most 128 bytes, and records of at most 512.
func TestObjectLongData(t *testing.T) {
	m, err := macroAssemble(t, ".PSECT D\n.BLKB 1\n"+strings.Repeat(".ASCII /0123456789/\n", 200)).Object(ObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}

	stored := 0

	for _, rec := range m.Records {
		raw, err := obj.EncodeRecord(rec)
		if err != nil {
			t.Fatal(err)
		}

		if len(raw) > 512 {
			t.Errorf("a %d-byte record", len(raw))
		}

		if tir, ok := rec.(*obj.TIR); ok {
			for _, c := range tir.Commands {
				if c.Op == obj.OpStoreImmediate {
					stored += len(c.Data)
				}
			}
		}
	}

	if stored != 2000 {
		t.Errorf("stored %d bytes, want 2000", stored)
	}
}

// TestObjectSeverity checks that a warning makes the end of module record
// report WARNING.
func TestObjectSeverity(t *testing.T) {
	requireDump(t, objectDump(t, ".ENABLE TRUNCATION"), "EOM: severity WARNING")
}

// TestObjectAbsolutePsect checks that an absolute psect allocates nothing.
func TestObjectAbsolutePsect(t *testing.T) {
	requireDump(t, objectDump(t, ".PSECT OFFSETS, ABS\nA: .BLKL 4"),
		`PSC 1: "OFFSETS", alignment BYTE, NOPIC,CON,ABS,LCL,NOSHR,EXE,RD,WRT,NOVEC, 0 bytes`,
		"CTL_AUGRB 0x10")
}

func TestObjectNeedsMACRODialect(t *testing.T) {
	a := New(true)
	if _, err := a.Assemble(".BYTE 1"); err != nil {
		t.Fatal(err)
	}

	_, err := a.Object(ObjectOptions{})
	requireCode(t, err, vmserrors.VAX_INTERNAL)
}
