package obj

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
)

// sampleModule builds a small module like the ones MACRO produces: a code
// psect with an entry point, a data psect, an external reference, a
// relocated longword, and a transfer address.
func sampleModule(t *testing.T) *Module {
	t.Helper()

	b := &Builder{
		Name:     "HELLO",
		Version:  "V1.0",
		Language: "govax MACRO V1.0",
		Created:  time.Date(2026, 9, 30, 9, 5, 0, 0, time.UTC),
		Title:    "Say hello",
	}

	code := b.AddPsect(Psect{Align: 2, Flags: PsectPIC | PsectREL | PsectSHR | PsectEXE | PsectRD, Alloc: 12, Name: "$CODE"})
	data := b.AddPsect(Psect{Align: 2, Flags: PsectREL | PsectRD | PsectWRT, Alloc: 4, Name: "$DATA"})

	b.AddSymbol(Symbol{Type: GSDEntry, Flags: SymDEF | SymREL, Psect: code, Value: 0, Mask: 0x0004, Name: "MAIN"})
	b.AddSymbol(Symbol{Type: GSDSymbol, Name: "SYS$EXIT"})

	b.SetLocation(code, 0)
	b.Store([]byte{0x04, 0x00}) // the entry mask
	b.Store([]byte{0xfb, 0x00}) // CALLS #0,
	b.Emit(Command{Op: OpStackGlobal, Name: "SYS$EXIT"}, Command{Op: OpStorePICodeRef})
	b.Store([]byte{0x04}) // RET
	b.SetLocation(data, 0)
	b.Emit(Command{Op: OpStackPsectLong, Psect: code, Value: 2}, Command{Op: OpStoreLong})
	b.SetTransfer(code, 2, false)

	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	return m
}

func TestBuilder_sampleModuleChecksClean(t *testing.T) {
	m := sampleModule(t)

	if problems := Check(m); len(problems) != 0 {
		t.Errorf("Check found problems: %v", problems)
	}

	if got := m.Name(); got != "HELLO" {
		t.Errorf("Name() = %q", got)
	}

	if ps := m.Psects(); len(ps) != 2 || ps[0].Name != "$CODE" || ps[1].Name != "$DATA" {
		t.Errorf("Psects() = %v", ps)
	}

	if syms := m.Symbols(); len(syms) != 2 || !syms[0].Defined() || syms[1].Defined() {
		t.Errorf("Symbols() = %v", syms)
	}
}

// TestRoundTrip checks that decoding an encoded module gives the same
// module back, and that encoding it again gives the same bytes.
func TestRoundTrip(t *testing.T) {
	m := sampleModule(t)

	raw, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}

	back, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(m, back) {
		t.Errorf("Decode(Encode(m)) differs:\n%#v\nwant\n%#v", back, m)
	}

	again, err := Encode(back)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(raw, again) {
		t.Errorf("re-encoding changed the bytes")
	}
}

// TestEncode_layouts pins the bytes of individual records against the
// manual's record diagrams, so the encoder can't drift in step with the
// decoder.
func TestEncode_layouts(t *testing.T) {
	cases := []struct {
		name string
		rec  Record
		want []byte
	}{
		{
			"main header",
			&MainHeader{StructureLevel: 0, MaxRecordSize: 2048, Name: "AB", Version: "V1", Created: "30-SEP-2026 09:05", Patched: "\x00"},
			append(append([]byte{0, 0, 0, 0x00, 0x08, 2, 'A', 'B', 2, 'V', '1'}, "30-SEP-2026 09:05"...), 0),
		},
		{
			"language header",
			&TextHeader{Type: HdrLNM, Text: "MACRO"},
			[]byte{0, 1, 'M', 'A', 'C', 'R', 'O'},
		},
		{
			// GSD type, alignment, flags word, allocation longword,
			// counted name (section 7.3.1).
			"psect",
			&GSD{Subrecords: []Subrecord{&Psect{Align: 2, Flags: 0x01c8, Alloc: 0x10, Name: "X"}}},
			[]byte{1, 0, 2, 0xc8, 0x01, 0x10, 0, 0, 0, 1, 'X'},
		},
		{
			// Type, data type, flags, psect byte, value, counted name
			// (section 7.3.2.1).
			"symbol definition",
			&GSD{Subrecords: []Subrecord{&Symbol{Type: GSDSymbol, Flags: SymDEF | SymREL, Psect: 1, Value: 0x20, Name: "S"}}},
			[]byte{1, 1, 0, 0x0a, 0, 1, 0x20, 0, 0, 0, 1, 'S'},
		},
		{
			// A reference has only the type, data type, flags, and name
			// (section 7.3.2.2).
			"symbol reference",
			&GSD{Subrecords: []Subrecord{&Symbol{Type: GSDSymbol, Flags: SymWEAK, Name: "R"}}},
			[]byte{1, 1, 0, 0x01, 0, 1, 'R'},
		},
		{
			// ... psect byte, value, entry mask, counted name (7.3.3).
			"entry point",
			&GSD{Subrecords: []Subrecord{&Symbol{Type: GSDEntry, Flags: SymDEF | SymREL, Psect: 0, Value: 4, Mask: 0x0ffc, Name: "E"}}},
			[]byte{1, 2, 0, 0x0a, 0, 0, 4, 0, 0, 0, 0xfc, 0x0f, 1, 'E'},
		},
		{
			"TIR commands",
			&TIR{Type: RecTIR, Commands: []Command{
				{Op: OpStackPsectLong, Psect: 1, Value: 8},
				{Op: OpSetRelocBase},
				{Op: OpStoreImmediate, Data: []byte{0xaa, 0xbb}},
				{Op: OpStackGlobal, Name: "G"},
				{Op: OpStorePICodeRef},
			}},
			[]byte{2, 6, 1, 8, 0, 0, 0, 80, 0xfe, 0xaa, 0xbb, 0, 1, 'G', 28},
		},
		{
			"end of module without a transfer address",
			&EOM{Severity: SeverityWarning},
			[]byte{3, 1},
		},
		{
			"end of module with a weak transfer address",
			&EOM{HasTransfer: true, Psect: 2, Transfer: 0x100, HasFlags: true, Flags: EOMWeakTransfer},
			[]byte{3, 0, 2, 0, 1, 0, 0, 1},
		},
		{
			"end of module with word psect",
			&EOM{Word: true, HasTransfer: true, Psect: 0x102, Transfer: 1},
			[]byte{7, 0, 2, 1, 1, 0, 0, 0},
		},
	}

	for _, c := range cases {
		got, err := EncodeRecord(c.rec)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)

			continue
		}

		if !bytes.Equal(got, c.want) {
			t.Errorf("%s: EncodeRecord = % x, want % x", c.name, got, c.want)
		}

		back, err := decodeRecord(got)
		if err != nil {
			t.Errorf("%s: decoding: %v", c.name, err)
		} else if !reflect.DeepEqual(back, c.rec) {
			t.Errorf("%s: decoded %#v, want %#v", c.name, back, c.rec)
		}
	}
}

// TestCommands_roundTrip encodes and decodes a command of every TIR type.
func TestCommands_roundTrip(t *testing.T) {
	for op, info := range ops {
		c := Command{Op: op}

		switch info.format {
		case opName:
			c.Name = "SYM"
		case opByte:
			c.Value = 0x80
		case opWord:
			c.Value = 0x8001
		case opLong:
			c.Value = 0x12345678
		case opPsectB, opPsectW, opPsectL:
			c.Psect, c.Value = 3, 0x7f
		case opWPsectB, opWPsectW, opWPsectL:
			c.Psect, c.Value = 0x123, 0x7f
		case opEnvName:
			c.Env, c.Name = 2, "LOCAL"
		case opIndex:
			c.Index = 7
		case opField:
			c.Pos, c.Size = 3, 4
		case opBytes:
			c.Data = []byte{1, 2, 3}
		case opCheckArg:
			c.Name, c.Index, c.Data = "PROC", 1, []byte{2, 1, 9}
		}

		b, err := c.encode(nil)
		if err != nil {
			t.Errorf("%s: %v", op, err)

			continue
		}

		back, n, err := decodeCommand(b)
		if err != nil || n != len(b) || !reflect.DeepEqual(back, c) {
			t.Errorf("%s: decodeCommand(% x) = %#v, %d, %v; want %#v", op, b, back, n, err, c)
		}
	}

	if len(ops) != 65 {
		t.Errorf("%d TIR commands, want the 65 the manual numbers 0-19, 20-42, 50-66, and 80-84", len(ops))
	}
}

func TestStoreImmediate_lengths(t *testing.T) {
	for _, n := range []int{1, 2, 127, 128} {
		c := Command{Op: OpStoreImmediate, Data: bytes.Repeat([]byte{0x5a}, n)}

		b, err := c.encode(nil)
		if err != nil {
			t.Fatalf("%d bytes: %v", n, err)
		}

		if int(b[0]) != 0x100-n {
			t.Errorf("%d bytes: command byte %#x", n, b[0])
		}

		back, used, err := decodeCommand(b)
		if err != nil || used != n+1 || !bytes.Equal(back.Data, c.Data) {
			t.Errorf("%d bytes: decoded %d bytes, %v", n, used, err)
		}
	}

	for _, n := range []int{0, 129} {
		if _, err := (Command{Op: OpStoreImmediate, Data: make([]byte, n)}).encode(nil); err == nil {
			t.Errorf("%d bytes: encode succeeded", n)
		}
	}
}

func TestStackedValue_signExtension(t *testing.T) {
	cases := []struct {
		c    Command
		want uint32
	}{
		{Command{Op: opsByName["STA_SB"], Value: 0xff}, 0xffffffff},
		{Command{Op: opsByName["STA_UB"], Value: 0xff}, 0xff},
		{Command{Op: opsByName["STA_SW"], Value: 0x8000}, 0xffff8000},
		{Command{Op: opsByName["STA_UW"], Value: 0x8000}, 0x8000},
		{Command{Op: opsByName["STA_PB"], Value: 0xfe}, 0xfffffffe},
		{Command{Op: OpStackPsectLong, Value: 0xfffffffe}, 0xfffffffe},
	}

	for _, c := range cases {
		if got := c.c.StackedValue(); got != c.want {
			t.Errorf("%s %#x: StackedValue = %#x, want %#x", c.c.Op, c.c.Value, got, c.want)
		}
	}
}

// TestSubrecords_roundTrip encodes and decodes every GSD subrecord type.
func TestSubrecords_roundTrip(t *testing.T) {
	subs := make([]Subrecord, 0, len(symbolLayouts)+4)

	subs = append(subs,
		&Psect{Align: 9, Flags: PsectOVR | PsectGBL, Alloc: 512, Name: "COMMON"},
		&Psect{Shared: true, Align: 2, Flags: PsectLIB, Alloc: 8, Base: 0x200, Name: "SHR"},
		&IdentCheck{Flags: 1, Name: "ENT", Ident: []byte{1, 2, 3, 4}, Object: "OBJ"},
		&Environment{Flags: 3, Parent: 1, Name: "ENV"},
	)

	for t2, layout := range symbolLayouts {
		s := &Symbol{Type: t2, DataType: 5, Flags: SymDEF | SymREL, Psect: 1, Value: 0x40, Name: "N" + t2.String()}
		if layout.local {
			s.Env = 2
		}

		if layout.wordPsect {
			s.Psect = 0x101
		}

		if layout.extra {
			s.Extra = 0x55
		}

		if layout.entry {
			s.Mask = 0x0ffc
		}

		if layout.procedure {
			s.Formals = &Formals{Min: 1, Max: 2, Args: []FormalArg{{ValCtl: 1}, {ValCtl: 2, Detail: []byte{9, 9}}}}
		}

		subs = append(subs, s)

		if !layout.alwaysDefines {
			subs = append(subs, &Symbol{Type: t2, Flags: SymWEAK, Name: "R" + t2.String()})
		}
	}

	for _, s := range subs {
		b, err := encodeGSDEntry(nil, s)
		if err != nil {
			t.Errorf("%s: %v", s.GSDType(), err)

			continue
		}

		r := reader{b: b}

		back, err := decodeGSDEntry(&r)
		if err != nil || !r.done() || !reflect.DeepEqual(back, s) {
			t.Errorf("%s: decoded %#v, %v (%d of %d bytes); want %#v", s.GSDType(), back, err, r.pos, len(b), s)
		}
	}
}

func TestBuilder_packsRecords(t *testing.T) {
	b := &Builder{Name: "BIG", Created: time.Now(), RecordLimit: 200}
	p := b.AddPsect(Psect{Flags: PsectREL, Alloc: 1000, Name: "D"})

	for i := range 40 {
		b.AddSymbol(Symbol{Type: GSDSymbol, Name: strings.Repeat("S", 20) + string(rune('A'+i%26)) + string(rune('A'+i/26))})
	}

	b.SetLocation(p, 0)
	b.Store(bytes.Repeat([]byte{0x11}, 1000))

	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	raw, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}

	gsds, tirs, stored := 0, 0, 0

	for i, r := range raw {
		if len(r) > 200 {
			t.Errorf("record %d is %d bytes, over the 200-byte limit", i+1, len(r))
		}
	}

	for _, rec := range m.Records {
		switch rec := rec.(type) {
		case *GSD:
			gsds++
		case *TIR:
			tirs++

			for _, c := range rec.Commands {
				if c.Op == OpStoreImmediate {
					if len(c.Data) > MaxImmediate {
						t.Errorf("STORE IMMEDIATE of %d bytes", len(c.Data))
					}

					stored += len(c.Data)
				}
			}
		}
	}

	if gsds < 2 || tirs < 2 {
		t.Errorf("%d GSD and %d TIR records, want several of each", gsds, tirs)
	}

	if stored != 1000 {
		t.Errorf("stored %d bytes, want 1000", stored)
	}

	if problems := Check(m); len(problems) != 0 {
		t.Errorf("Check: %v", problems)
	}
}

func TestBuilder_defaults(t *testing.T) {
	b := &Builder{Created: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)}
	b.AddPsect(Psect{Name: "P"})

	m, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	h := m.Records[0].(*MainHeader)
	if h.Name != ".MAIN." || h.Created != " 2-JAN-2026 03:04" || h.MaxRecordSize != DefaultRecordLimit || h.Patched != "                 " {
		t.Errorf("main header %+v", h)
	}

	if _, err := (&Builder{}).Build(); err == nil {
		t.Error("Build with no psects or symbols succeeded")
	}
}

func TestRecords_hostLayout(t *testing.T) {
	recs := [][]byte{{1}, {2, 3}, {4, 5, 6}, {}}

	var buf bytes.Buffer
	if err := WriteRecords(&buf, recs); err != nil {
		t.Fatal(err)
	}

	want := []byte{1, 0, 1, 0, 2, 0, 2, 3, 3, 0, 4, 5, 6, 0, 0, 0}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("WriteRecords = % x, want % x", buf.Bytes(), want)
	}

	back, err := ReadRecords(&buf)
	if err != nil || !reflect.DeepEqual(back, recs) {
		t.Errorf("ReadRecords = %v, %v", back, err)
	}
}

// TestReadRecords_endOfBlock reads a block whose rest is marked unused by a
// 0xFFFF length, as RMS writes when records may not span blocks.
func TestReadRecords_endOfBlock(t *testing.T) {
	block := make([]byte, blockSize)
	copy(block, []byte{2, 0, 0xaa, 0xbb, 0xff, 0xff})

	data := append(block, 1, 0, 0xcc, 0)

	got, err := ReadRecords(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	want := [][]byte{{0xaa, 0xbb}, {0xcc}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadRecords = % x, want % x", got, want)
	}

	if _, err := ReadRecords(bytes.NewReader([]byte{5, 0, 1, 2})); err == nil {
		t.Error("a truncated record read without error")
	}
}

func TestDecode_errors(t *testing.T) {
	cases := map[string][]byte{
		"empty record":          {},
		"short header":          {0, 0, 0},
		"empty GSD":             {1},
		"unknown GSD type":      {1, 99},
		"truncated psect":       {1, 0, 2, 0},
		"unknown TIR command":   {2, 45},
		"short immediate":       {2, 0xfd, 1},
		"truncated name":        {2, 0, 5, 'A'},
		"EOM with extra bytes":  {3, 0, 1, 0, 0, 0, 0, 0, 9},
		"EOM with partial addr": {3, 0, 1, 0},
	}

	for name, rec := range cases {
		if _, err := Decode([][]byte{rec}); err == nil {
			t.Errorf("%s: Decode(% x) succeeded", name, rec)
		}
	}

	m, err := Decode([][]byte{{99, 1, 2}})
	if err != nil {
		t.Fatal(err)
	}

	if u, ok := m.Records[0].(*Unknown); !ok || u.Type != 99 || !bytes.Equal(u.Data, []byte{1, 2}) {
		t.Errorf("unknown record decoded as %#v", m.Records[0])
	}
}

func TestCheck_problems(t *testing.T) {
	good := func() *Module { return sampleModule(t) }

	cases := []struct {
		name   string
		mutate func(m *Module)
		want   string
	}{
		{"missing EOM", func(m *Module) { m.Records = m.Records[:len(m.Records)-1] }, "no end of module"},
		{"EOM not last", func(m *Module) {
			n := len(m.Records)
			m.Records[n-1], m.Records[n-2] = m.Records[n-2], m.Records[n-1]
		}, "not the last record"},
		{"LNM not second", func(m *Module) { m.Records[1], m.Records[2] = m.Records[2], m.Records[1] }, "not the language processor header"},
		{"bad creation time", func(m *Module) { m.Records[0].(*MainHeader).Created = "yesterday at noon" }, "creation time"},
		{"long name", func(m *Module) { m.Records[0].(*MainHeader).Name = strings.Repeat("N", 32) }, "module name"},
		{"bad psect index", func(m *Module) {
			m.Records[len(m.Records)-1].(*EOM).Psect = 7
		}, "refers to psect 7"},
		{"stack underflow", func(m *Module) {
			tir := m.Records[len(m.Records)-2].(*TIR)
			tir.Commands = append(tir.Commands, Command{Op: OpStoreLong})
		}, "needs 1 stack longwords"},
		{"stack left over", func(m *Module) {
			tir := m.Records[len(m.Records)-2].(*TIR)
			tir.Commands = append(tir.Commands, Command{Op: OpStackLong, Value: 1})
		}, "holds 1 longwords"},
		{"undeclared global", func(m *Module) {
			tir := m.Records[len(m.Records)-2].(*TIR)
			tir.Commands = append(tir.Commands, Command{Op: OpStackGlobal, Name: "NOWHERE"}, Command{Op: OpStoreLong})
		}, "NOWHERE"},
		{"reserved severity", func(m *Module) { m.Records[len(m.Records)-1].(*EOM).Severity = 5 }, "reserved"},
		{"record too long", func(m *Module) { m.Records[0].(*MainHeader).MaxRecordSize = 10 }, "more than the maximum of 10"},
		{"header after text", func(m *Module) {
			n := len(m.Records)
			m.Records = append(m.Records[:n-1], &TextHeader{Type: HdrTTL, Text: "late"}, m.Records[n-1])
		}, "header record after"},
	}

	for _, c := range cases {
		m := good()
		c.mutate(m)

		problems := Check(m)

		found := false

		for _, p := range problems {
			if strings.Contains(p.String(), c.want) {
				found = true
			}
		}

		if !found {
			t.Errorf("%s: Check = %v, want a problem containing %q", c.name, problems, c.want)
		}
	}
}

func TestDump(t *testing.T) {
	var buf bytes.Buffer
	if err := Dump(&buf, sampleModule(t)); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`1. HDR MHD: module "HELLO", version "V1.0"`,
		`created "30-SEP-2026 09:05", structure level 0, maximum record size 512`,
		`2. HDR LNM: "govax MACRO V1.0"`,
		`3. HDR TTL: "Say hello"`,
		`PSC 0: "$CODE", alignment LONG, PIC,CON,REL,LCL,SHR,EXE,RD,NOWRT,NOVEC, 12 bytes`,
		`PSC 1: "$DATA"`,
		`EPM "MAIN" = 0x0 in psect 0, DEF,REL, mask 0x0004`,
		`SYM "SYS$EXIT" (reference)`,
		`STA_PL psect 0 offset 0x0`,
		`CTL_SETRB`,
		`STO_IMM 4 bytes: 04 00 fb 00`,
		`STA_GBL "SYS$EXIT"`,
		`STO_PICR`,
		`EOM: severity SUCCESS, transfer address psect 0 offset 0x2`,
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("Dump output lacks %q:\n%s", want, buf.String())
		}
	}
}

// TestOtherRecords round-trips and dumps the records a MACRO module rarely
// or never has: link options, debugger and traceback records, word-psect
// ends of module, and a record type this package doesn't know.
func TestOtherRecords(t *testing.T) {
	cmds := make([]Command, 0, len(ops))

	for op := range ops {
		c := Command{Op: op}

		switch ops[op].format {
		case opName, opEnvName:
			c.Name = "X"
		case opBytes:
			c.Data = []byte{1}
		case opCheckArg:
			c.Name, c.Data = "P", []byte{1, 0}
		}

		cmds = append(cmds, c)
	}

	m := &Module{Records: []Record{
		&LNK{Type: 3, Flags: 1, Name: "SYS$LIBRARY:STARLET", Rest: []byte{9}},
		&TIR{Type: RecDBG, Commands: cmds},
		&TIR{Type: RecTBT, Commands: []Command{{Op: OpStoreImmediate, Data: []byte{7}}}},
		&EOM{Word: true, Severity: SeverityAbort},
		&Unknown{Type: 42, Data: []byte{1, 2}},
		&TextHeader{Type: HeaderType(200), Text: "ignored"},
	}}

	raw, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}

	back, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(back, m) {
		t.Errorf("round trip changed the module")
	}

	var buf bytes.Buffer
	if err := Dump(&buf, back); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`1. LNK type 3, flags 0x1: "SYS$LIBRARY:STARLET"`,
		"2. DBG", "3. TBT", "4. EOMW: severity ABORT",
		"5. record type 42: 2 bytes 01 02", `6. HDR header type 200: "ignored"`,
		"STO_VPS bits 0 to -1", "STA_LIT literal 0", `STA_LSY environment 0 "X"`,
		`STA_CKARG "P" argument 0`, "STO_RIVB 01", "OPR_ADD", "CTL_AUGRB 0x0",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("Dump lacks %q:\n%s", want, buf.String())
		}
	}

	if op, ok := OpByName("sta_pl"); !ok || op != OpStackPsectLong {
		t.Errorf("OpByName(sta_pl) = %v, %v", op, ok)
	}

	if s := Op(99).String(); s != "TIR command 99" {
		t.Errorf("Op(99) = %q", s)
	}

	for _, a := range []byte{0, 1, 2, 3, 4, 9, 5} {
		if alignmentName(a) == "" {
			t.Errorf("alignment %d has no name", a)
		}
	}

	if got := PsectFlagNames(PsectGBL | PsectOVR); got != "NOPIC,OVR,ABS,GBL,NOSHR,NOEXE,NORD,NOWRT,NOVEC" {
		t.Errorf("PsectFlagNames = %q", got)
	}
}
