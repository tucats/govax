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
//
// GSD and TIR content goes into the module in the order it's added, as
// real MACRO writes it: a run of AddPsect and AddSymbol calls fills GSD
// records, and a run of Emit, SetLocation, and Store calls fills TIR
// records. Break ends the current record early, so the next content
// starts a new one.
type Builder struct {
	Name     string
	Version  string
	Language string    // the LNM record's text
	Created  time.Time // the MHD creation time
	// Source and Title, when not empty, become SRC and TTL header
	// records. Real MACRO puts its command line in the SRC record, and
	// .TITLE's comment in the TTL record.
	Source string
	Title  string
	// RecordLimit is the longest record Build writes, and the MHD maximum
	// record size; 0 means DefaultRecordLimit.
	RecordLimit int
	// Severity is the EOM completion code.
	Severity byte

	chunks []chunk
	psects int

	transfer         bool
	transferPsect    uint16
	transferAddr     uint32
	transferWeak     bool
	pendingImmediate []byte
}

// DefaultRecordLimit is the record size Build uses when RecordLimit is 0:
// 512 bytes, the maximum record size VAX MACRO V5.4-3 writes in its
// objects' main headers (docs/PHASE-27.md), rather than the object
// language's own 2048-byte limit.
const DefaultRecordLimit = 512

// chunk is a run of GSD subrecords or TIR commands that Build packs into
// records of one type, starting a new record at the chunk's start.
type chunk struct {
	gsd      bool
	subs     []Subrecord
	commands []Command
}

// current returns the chunk content of the given type goes into, starting
// a new one if the last is of the other type or Break ended it.
func (b *Builder) current(gsd bool) *chunk {
	if n := len(b.chunks); n > 0 && b.chunks[n-1].gsd == gsd {
		return &b.chunks[n-1]
	}

	b.chunks = append(b.chunks, chunk{gsd: gsd})

	return &b.chunks[len(b.chunks)-1]
}

// AddPsect adds a psect definition, returning its index: the order it was
// added in, which is how the linker numbers psects.
func (b *Builder) AddPsect(p Psect) uint16 {
	b.flushImmediate()

	c := b.current(true)
	c.subs = append(c.subs, &p)
	b.psects++

	return uint16(b.psects - 1)
}

// AddSymbol adds a symbol subrecord (a definition, reference, or entry
// point).
func (b *Builder) AddSymbol(s Symbol) {
	b.flushImmediate()

	c := b.current(true)
	c.subs = append(c.subs, &s)
}

// Emit appends TIR commands.
func (b *Builder) Emit(cmds ...Command) {
	b.flushImmediate()

	c := b.current(false)
	c.commands = append(c.commands, cmds...)
}

// Break ends the current record: the next content added starts a new one.
func (b *Builder) Break() {
	b.flushImmediate()

	if len(b.chunks) > 0 {
		b.chunks = append(b.chunks, chunk{gsd: !b.chunks[len(b.chunks)-1].gsd})
	}
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
	if len(b.pendingImmediate) == 0 {
		return
	}

	c := b.current(false)

	for data := b.pendingImmediate; len(data) > 0; {
		n := min(len(data), MaxImmediate)
		c.commands = append(c.commands, Command{Op: OpStoreImmediate, Data: append([]byte(nil), data[:n]...)})
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
		limit = DefaultRecordLimit
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
			// The linker ignores the patch time. The manual says to pad
			// it "with 17 zeros", but real MACRO writes 17 spaces.
			Patched: strings.Repeat(" ", 17),
		},
		&TextHeader{Type: HdrLNM, Text: b.Language},
	)

	if b.Source != "" {
		m.Records = append(m.Records, &TextHeader{Type: HdrSRC, Text: b.Source})
	}

	if b.Title != "" {
		m.Records = append(m.Records, &TextHeader{Type: HdrTTL, Text: b.Title})
	}

	hasGSD := false

	for _, c := range b.chunks {
		hasGSD = hasGSD || len(c.subs) > 0

		records, err := packChunk(c, limit)
		if err != nil {
			return nil, err
		}

		m.Records = append(m.Records, records...)
	}

	if !hasGSD {
		return nil, fmt.Errorf("module %s has no psects or symbols", name)
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

// packChunk packs one chunk's subrecords or commands into as few records
// as fit in limit bytes.
func packChunk(c chunk, limit int) ([]Record, error) {
	var (
		out  []Record
		gsd  *GSD
		tir  *TIR
		size int
	)

	start := func() {
		if c.gsd {
			gsd = &GSD{}
			out = append(out, gsd)
		} else {
			tir = &TIR{Type: RecTIR}
			out = append(out, tir)
		}

		size = 1
	}

	add := func(n int, what string) error {
		if 1+n > limit {
			return fmt.Errorf("%s is too long for a %d-byte record", what, limit)
		}

		if len(out) == 0 || size+n > limit {
			start()
		}

		size += n

		return nil
	}

	for _, s := range c.subs {
		enc, err := encodeGSDEntry(nil, s)
		if err != nil {
			return nil, err
		}

		if err := add(len(enc), s.GSDType().String()+" subrecord"); err != nil {
			return nil, err
		}

		gsd.Subrecords = append(gsd.Subrecords, s)
	}

	for _, cmd := range c.commands {
		enc, err := cmd.encode(nil)
		if err != nil {
			return nil, err
		}

		if err := add(len(enc), cmd.Op.String()+" command"); err != nil {
			return nil, err
		}

		tir.Commands = append(tir.Commands, cmd)
	}

	return out, nil
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
