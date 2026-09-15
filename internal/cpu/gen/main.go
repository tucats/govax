// Command gen parses reference/eVAX/eVAX/Headers/instruction_table.h and
// emits internal/cpu/instructions_table.go. Run via `go generate` from
// internal/cpu (see the go:generate directive in instruction.go) rather than
// hand-transcribing ~284 instruction table entries — see docs/PHASE-03.md's
// design notes on why.
//
// The parser doesn't try to be a general C parser: it relies on
// instruction_table.h's own documented convention (each entry is a fixed,
// machine-generated 9-field sequence) and fails loudly if that shape isn't
// found, rather than silently emitting a wrong or partial table.
package main

import (
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type field struct {
	scale  [6]int
	typ    string
	ext    int
	opcode int
	count  int
	access [6]string
	name   string
}

// entryRE matches one instruction_table.h entry. The table's own header
// comment says it's machine-generated in a fixed format, and every entry in
// the file (checked against all ~284 of them) follows this exact field
// order, so a single structural regex is more reliable than a hand-rolled
// tokenizer for a one-off generator like this.
var entryRE = regexp.MustCompile(`(?s)\{[^{]*?` +
	`\{\s*(-?\d+)\s*,\s*(-?\d+)\s*,\s*(-?\d+)\s*,\s*(-?\d+)\s*,\s*(-?\d+)\s*,\s*(-?\d+)\s*\}\s*,.*?` +
	`(OP_TYPE_\w+)\s*,.*?` +
	`0L\s*,.*?` +
	`(0[Xx][0-9A-Fa-f]+)\s*,\s*(0[Xx][0-9A-Fa-f]+)\s*,.*?` +
	`(\d+)\s*,.*?` +
	`ACCESS\(\s*(OP_\w+)\s*,\s*(OP_\w+)\s*,\s*(OP_\w+)\s*,\s*(OP_\w+)\s*,\s*(OP_\w+)\s*,\s*(OP_\w+)\s*\)\s*,.*?` +
	`0L\s*,\s*0L\s*,.*?` +
	`"([^"]*)"\s*` +
	`\}`)

func parseInt(s string) int {
	n, err := strconv.ParseInt(s, 0, 32)
	if err != nil {
		log.Fatalf("gen: bad integer %q: %v", s, err)
	}
	return int(n)
}

func parse(src string) []field {
	matches := entryRE.FindAllStringSubmatch(src, -1)
	if len(matches) == 0 {
		log.Fatalf("gen: no entries matched in source")
	}

	var out []field
	for _, m := range matches {
		f := field{
			typ:    m[7],
			ext:    parseInt(m[8]),
			opcode: parseInt(m[9]),
			count:  parseInt(m[10]),
			access: [6]string{m[11], m[12], m[13], m[14], m[15], m[16]},
			name:   m[17],
		}
		for i := 0; i < 6; i++ {
			f.scale[i] = parseInt(m[1+i])
		}
		if f.name == "" {
			// The table's end-of-array sentinel entry (empty name); not a
			// real instruction.
			continue
		}
		out = append(out, f)
	}
	return out
}

// knownTableFixes patches instruction_table.h entries that are themselves
// wrong in the C reference (not a parsing issue): seven D-floating opcodes
// (SUBD2, CVTDB, CVTDW, CVTDL, CVTRDL, CMPD, TSTD) have all-zero (or, for
// SUBD2, zero operand-count-only) scale/access/count columns in the C
// header, and in most cases no init_emulators.c dispatch entry either — they
// were never correctly wired up in the C reference at all, confirmed via
// docs/PHASE-05.md's design notes. Applied here, at generation time, rather
// than hand-edited into the generated instructions_table.go, so the fix
// survives a future `go generate` instead of being silently reverted by it.
// See docs/DEVIATIONS.md for the full writeup.
//
// Each fix mirrors the corresponding already-correct sibling row: SUBD2
// mirrors MULD2/DIVD2's shape; the CVTDx conversions mirror CVTFB/CVTFW/
// CVTFL/CVTRFL with the source scaled to 8 bytes; CMPD/TSTD mirror CMPF/TSTF.
var knownTableFixes = map[string]field{
	"SUBD2": {
		scale: [6]int{8, 8, 0, 0, 0, 0}, typ: "OP_TYPE_FLOAT", count: 2,
		access: [6]string{"OP_RD", "OP_MD", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	"CVTDB": {
		scale: [6]int{8, 1, 0, 0, 0, 0}, typ: "OP_TYPE_FLOAT", count: 2,
		access: [6]string{"OP_RD", "OP_WR", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	"CVTDW": {
		scale: [6]int{8, 2, 0, 0, 0, 0}, typ: "OP_TYPE_FLOAT", count: 2,
		access: [6]string{"OP_RD", "OP_WR", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	"CVTDL": {
		scale: [6]int{8, 4, 0, 0, 0, 0}, typ: "OP_TYPE_FLOAT", count: 2,
		access: [6]string{"OP_RD", "OP_WR", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	"CVTRDL": {
		scale: [6]int{8, 4, 0, 0, 0, 0}, typ: "OP_TYPE_FLOAT", count: 2,
		access: [6]string{"OP_RD", "OP_WR", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	"CMPD": {
		scale: [6]int{8, 8, 0, 0, 0, 0}, typ: "OP_TYPE_FLOAT", count: 2,
		access: [6]string{"OP_RD", "OP_RD", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	"TSTD": {
		scale: [6]int{8, 0, 0, 0, 0, 0}, typ: "OP_TYPE_FLOAT", count: 1,
		access: [6]string{"OP_RD", "OP_NL", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	// CRC's C header entry has all-zero operand count/scale/access, matching
	// emul_crc.c's own empty-stub implementation (see docs/PHASE-06.md's
	// design notes and docs/DEVIATIONS.md) -- there was never a working
	// operand shape to preserve. Scale/access below is transcribed directly
	// from vax_instr_set.pdf's CRC format line: `tbl.ab, inicrc.rl,
	// strlen.rw, stream.ab`.
	"CRC": {
		scale: [6]int{1, 4, 2, 1, 0, 0}, typ: "OP_TYPE_INT", count: 4,
		access: [6]string{"OP_AD", "OP_RD", "OP_RD", "OP_AD", "OP_NL", "OP_NL"},
	},
	// REMQHI/REMQTI's C header rows are byte-for-byte copies of
	// INSQHI/INSQTI's ({1,8}/OP_AD,OP_AD) despite having a different operand
	// order and second-operand type: vax_instr_set.pdf's format line is
	// `header.aq, addr.wl` (header first, an address operand -- matches
	// OP_AD/scale irrelevant; addr second, a *write longword* destination,
	// not an address operand at all), and emul_remqhi.c/emul_remqti.c's own
	// handlers already call `put_operand(opcode, 1, OP_WR, ...)` explicitly
	// -- the handler's own intent already disagrees with its table row.
	// Fixed here to OP_WR/scale 4 for operand 1 (matching both the manual
	// and the handler), OP_AD/scale 8 for operand 0 (matching INSQHI/
	// INSQTI's own already-correct header.aq). See docs/DEVIATIONS.md.
	"REMQHI": {
		scale: [6]int{8, 4, 0, 0, 0, 0}, typ: "OP_TYPE_INT", count: 2,
		access: [6]string{"OP_AD", "OP_WR", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	"REMQTI": {
		scale: [6]int{8, 4, 0, 0, 0, 0}, typ: "OP_TYPE_INT", count: 2,
		access: [6]string{"OP_AD", "OP_WR", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	// REMQUE has the identical OP_AD-should-be-OP_WR mistake for its second
	// operand (`entry.ab, addr.wl` per the manual; emul_remque.c also calls
	// `put_operand(opcode, 1, OP_WR, &entry)` explicitly). Scale was already
	// correct (4, matching a longword write) -- only access needed fixing.
	"REMQUE": {
		scale: [6]int{4, 4, 0, 0, 0, 0}, typ: "OP_TYPE_INT", count: 2,
		access: [6]string{"OP_AD", "OP_WR", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	// ADWC/SBWC's C header row scales both operands as word (2), and
	// emul_integer_math.c special-cases these two opcodes to dsize=5
	// (word) rather than falling through to the longword case its opcode
	// range would otherwise select -- but vax_instr_set.pdf's format line
	// is `add.rl, sum.ml` (SBWC: `sub.rl, dif.ml`), longword throughout.
	// Fixed here in Phase 12, per user direction, rather than left
	// deferred as a generated-table matter out of a handler phase's own
	// scope. See docs/DEVIATIONS.md.
	"ADWC": {
		scale: [6]int{4, 4, 0, 0, 0, 0}, typ: "OP_TYPE_INT", count: 2,
		access: [6]string{"OP_RD", "OP_MD", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	"SBWC": {
		scale: [6]int{4, 4, 0, 0, 0, 0}, typ: "OP_TYPE_INT", count: 2,
		access: [6]string{"OP_RD", "OP_MD", "OP_NL", "OP_NL", "OP_NL", "OP_NL"},
	},
	// BISB3's C header row declares its destination (third) operand
	// longword-sized (4) despite every sibling Bxx3 instruction of the
	// identical shape (ADDB3, SUBB3, MULB3, DIVB3, BICB3, XORB3)
	// correctly declaring all three operands byte-sized -- a plain
	// transcription error, not a deliberate design choice (there's no ISA
	// reading under which BISB3 alone would have a wider destination than
	// BISB3's own 2-operand form or its Bxx3 siblings). Fixed here in
	// Phase 12, per user direction. See docs/DEVIATIONS.md.
	"BISB3": {
		scale: [6]int{1, 1, 1, 0, 0, 0}, typ: "OP_TYPE_INT", count: 3,
		access: [6]string{"OP_RD", "OP_RD", "OP_WR", "OP_NL", "OP_NL", "OP_NL"},
	},
}

// applyKnownFixes patches fields in place per knownTableFixes, preserving
// each entry's parsed name/ext/opcode (identity), and fails loudly if an
// expected name isn't found — the fix list is stale, which is a bug in the
// generator, not something to silently skip.
func applyKnownFixes(fields []field) {
	remaining := make(map[string]bool, len(knownTableFixes))
	for name := range knownTableFixes {
		remaining[name] = true
	}
	for i, f := range fields {
		fix, ok := knownTableFixes[f.name]
		if !ok {
			continue
		}
		fix.name, fix.ext, fix.opcode = f.name, f.ext, f.opcode
		fields[i] = fix
		delete(remaining, f.name)
	}
	for name := range remaining {
		log.Fatalf("gen: knownTableFixes entry %q not found in parsed table", name)
	}
}

var accessName = map[string]string{
	"OP_NL": "AccessNone",
	"OP_RD": "AccessRead",
	"OP_WR": "AccessWrite",
	"OP_MD": "AccessModify",
	"OP_AD": "AccessAddress",
	"OP_VA": "AccessVarField",
	"OP_BR": "AccessBranch",
	"OP_IM": "AccessImmediate",
}

var typeName = map[string]string{
	"OP_TYPE_INT":   "ShortLiteralInt",
	"OP_TYPE_FLOAT": "ShortLiteralFloat",
}

func generate(fields []field, sourcePath string) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "// Code generated by internal/cpu/gen from %s. DO NOT EDIT.\n", sourcePath)
	fmt.Fprintf(&b, "// Run `go generate ./internal/cpu` to regenerate.\n\n")
	fmt.Fprintf(&b, "package cpu\n\n")
	fmt.Fprintf(&b, "var instructionTable = newTable([]*Instruction{\n")

	for _, f := range fields {
		access := make([]string, 6)
		for i, a := range f.access {
			name, ok := accessName[a]
			if !ok {
				log.Fatalf("gen: unknown access kind %q in entry %q", a, f.name)
			}
			access[i] = name
		}
		typ, ok := typeName[f.typ]
		if !ok {
			log.Fatalf("gen: unknown short-literal type %q in entry %q", f.typ, f.name)
		}

		fmt.Fprintf(&b, "\t{\n")
		fmt.Fprintf(&b, "\t\tName:         %q,\n", f.name)
		fmt.Fprintf(&b, "\t\tOpcode:       Opcode{Extended: 0x%02X, Function: 0x%02X},\n", f.ext, f.opcode)
		fmt.Fprintf(&b, "\t\tOperandCount: %d,\n", f.count)
		fmt.Fprintf(&b, "\t\tScale:        [6]int{%d, %d, %d, %d, %d, %d},\n",
			f.scale[0], f.scale[1], f.scale[2], f.scale[3], f.scale[4], f.scale[5])
		fmt.Fprintf(&b, "\t\tAccess:       [6]AccessKind{%s, %s, %s, %s, %s, %s},\n",
			access[0], access[1], access[2], access[3], access[4], access[5])
		fmt.Fprintf(&b, "\t\tType:         %s,\n", typ)
		fmt.Fprintf(&b, "\t},\n")
	}

	fmt.Fprintf(&b, "})\n")

	out, err := format.Source([]byte(b.String()))
	if err != nil {
		log.Fatalf("gen: generated source doesn't compile: %v", err)
	}
	return out
}

func main() {
	in := flag.String("in", "", "path to instruction_table.h")
	out := flag.String("out", "", "path to write the generated Go source")
	flag.Parse()

	if *in == "" || *out == "" {
		log.Fatal("gen: -in and -out are required")
	}

	src, err := os.ReadFile(*in)
	if err != nil {
		log.Fatalf("gen: %v", err)
	}

	fields := parse(string(src))
	applyKnownFixes(fields)
	code := generate(fields, *in)

	if err := os.WriteFile(*out, code, 0o644); err != nil {
		log.Fatalf("gen: %v", err)
	}

	fmt.Fprintf(os.Stderr, "gen: wrote %d instructions to %s\n", len(fields), *out)
}
