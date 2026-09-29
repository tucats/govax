package obj

import (
	"fmt"
	"strings"
	"time"
)

// Builder assembles an object module from its content: the module's name
// and version, its psects and symbols, the TIR commands that store its
// code and data, and its transfer address. Build packs that content into
// records no longer than RecordLimit, never splitting a GSD subrecord or
// TIR command across records.
type Builder struct {
	Name     string
	Version  string
	Language string    // the LNM record's text
	Created  time.Time // the MHD creation time
	// Title, when not empty, becomes a TTL header record.
	Title string
	// RecordLimit is the longest record Build writes, and the MHD maximum
	// record size; 0 means MaxRecordSize.
	RecordLimit int
	// Severity is the EOM completion code.
	Severity byte

	gsd      []Subrecord
	psects   int
	commands []Command

	transfer         bool
	transferPsect    uint16
	transferAddr     uint32
	transferWeak     bool
	pendingImmediate []byte
}

// AddPsect adds a psect definition, returning its index: the order it was
// added in, which is how the linker numbers psects.
func (b *Builder) AddPsect(p Psect) uint16 {
	b.gsd = append(b.gsd, &p)
	b.psects++

	return uint16(b.psects - 1)
}

// AddSymbol adds a symbol subrecord (a definition, reference, or entry
// point).
func (b *Builder) AddSymbol(s Symbol) {
	b.gsd = append(b.gsd, &s)
}

// Emit appends TIR commands.
func (b *Builder) Emit(cmds ...Command) {
	b.flushImmediate()
	b.commands = append(b.commands, cmds...)
}

// SetLocation points the linker's location counter at an offset in a
// psect: STA_PL, then CTL_SETRB.
func (b *Builder) SetLocation(psect uint16, offset uint32) {
	b.Emit(Command{Op: OpStackPsectLong, Psect: psect, Value: offset}, Command{Op: OpSetRelocBase})
}

// Store stores bytes as they are at the location counter. Consecutive
// Store calls are merged, then split into STORE IMMEDIATE commands of at
// most MaxImmediate bytes.
func (b *Builder) Store(data []byte) {
	b.pendingImmediate = append(b.pendingImmediate, data...)
}

func (b *Builder) flushImmediate() {
	for data := b.pendingImmediate; len(data) > 0; {
		n := min(len(data), MaxImmediate)
		b.commands = append(b.commands, Command{Op: OpStoreImmediate, Data: append([]byte(nil), data[:n]...)})
		data = data[n:]
	}

	b.pendingImmediate = nil
}

// SetTransfer names the module's transfer address: an offset in a psect.
func (b *Builder) SetTransfer(psect uint16, offset uint32, weak bool) {
	b.transfer, b.transferPsect, b.transferAddr, b.transferWeak = true, psect, offset, weak
}

// Build returns the module.
func (b *Builder) Build() (*Module, error) {
	b.flushImmediate()

	limit := b.RecordLimit
	if limit == 0 {
		limit = MaxRecordSize
	}

	if limit > MaxRecordSize {
		return nil, fmt.Errorf("record limit %d is more than the maximum of %d", limit, MaxRecordSize)
	}

	name := b.Name
	if name == "" {
		name = ".MAIN."
	}

	version := b.Version
	if version == "" {
		version = "0"
	}

	m := &Module{}

	m.Records = append(m.Records,
		&MainHeader{
			StructureLevel: StructureLevel,
			MaxRecordSize:  uint16(limit),
			Name:           name,
			Version:        version,
			Created:        FormatTime(b.Created),
			// The linker ignores the patch time; the manual says to
			// pad it "with 17 zeros".
			Patched: strings.Repeat("\x00", 17),
		},
		&TextHeader{Type: HdrLNM, Text: b.Language},
	)

	if b.Title != "" {
		m.Records = append(m.Records, &TextHeader{Type: HdrTTL, Text: b.Title})
	}

	if len(b.gsd) == 0 {
		return nil, fmt.Errorf("module %s has no psects or symbols", name)
	}

	// Pack GSD subrecords, then TIR commands, into as few records as
	// fit.
	gsd := &GSD{}
	size := 1

	for _, s := range b.gsd {
		enc, err := encodeGSDEntry(nil, s)
		if err != nil {
			return nil, err
		}

		if 1+len(enc) > limit {
			return nil, fmt.Errorf("%s subrecord is too long for a %d-byte record", s.GSDType(), limit)
		}

		if size+len(enc) > limit {
			m.Records = append(m.Records, gsd)
			gsd, size = &GSD{}, 1
		}

		gsd.Subrecords = append(gsd.Subrecords, s)
		size += len(enc)
	}

	m.Records = append(m.Records, gsd)

	tir := &TIR{Type: RecTIR}
	size = 1

	for _, c := range b.commands {
		enc, err := c.encode(nil)
		if err != nil {
			return nil, err
		}

		if 1+len(enc) > limit {
			return nil, fmt.Errorf("%s command is too long for a %d-byte record", c.Op, limit)
		}

		if size+len(enc) > limit {
			m.Records = append(m.Records, tir)
			tir, size = &TIR{Type: RecTIR}, 1
		}

		tir.Commands = append(tir.Commands, c)
		size += len(enc)
	}

	if len(tir.Commands) > 0 {
		m.Records = append(m.Records, tir)
	}

	eom := &EOM{Severity: b.Severity}
	if b.transfer {
		eom.HasTransfer, eom.Psect, eom.Transfer = true, b.transferPsect, b.transferAddr
		if b.transferWeak {
			eom.HasFlags, eom.Flags = true, EOMWeakTransfer
		}
	}

	m.Records = append(m.Records, eom)

	return m, nil
}

// FormatTime formats a time in the object language's fixed 17-character
// form, "dd-mmm-yyyy hh:mm", with the month in capitals as VMS writes it.
func FormatTime(t time.Time) string {
	return strings.ToUpper(t.Format("02-Jan-2006 15:04"))
}

// Name returns the module's name from its main header, or "" if it has
// none.
func (m *Module) Name() string {
	for _, rec := range m.Records {
		if h, ok := rec.(*MainHeader); ok {
			return h.Name
		}
	}

	return ""
}

// Psects returns the module's psect definitions in index order.
func (m *Module) Psects() []*Psect {
	var out []*Psect

	for _, rec := range m.Records {
		if g, ok := rec.(*GSD); ok {
			for _, s := range g.Subrecords {
				if p, ok := s.(*Psect); ok {
					out = append(out, p)
				}
			}
		}
	}

	return out
}

// Symbols returns the module's symbol subrecords in order.
func (m *Module) Symbols() []*Symbol {
	var out []*Symbol

	for _, rec := range m.Records {
		if g, ok := rec.(*GSD); ok {
			for _, s := range g.Subrecords {
				if sym, ok := s.(*Symbol); ok {
					out = append(out, sym)
				}
			}
		}
	}

	return out
}
