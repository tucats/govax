package obj

// This file packs the line-number table into debugger (DBG) records as
// real MACRO does (docs/PHASE-29.md, subtask 12), which isn't the way
// Builder packs TIR, TBT, or a module's DBG symbol records:
//
//   - The table's commands go into DSTLineNumbers records of at most
//     lineRecordMax bytes (the type and data, as the length byte counts
//     them), never splitting a command.
//   - A DBG record is filled to lineFill bytes, and the line-number
//     record being built is cut short to end where the DBG record does,
//     the table going on in a new one in the next DBG record.
//   - Its data is stored by STORE IMMEDIATEs of at most lineImmediate
//     bytes, each run of data cut from its start: the first after the
//     record's start or an address.
//   - A record of the table's last part (the line count, and the end of
//     the last segment) goes in a new DBG record unless the one being
//     filled has lineReserve bytes left.
//
// FORTH's table (testdata/mar/dst/vax/forth.obj) shows the first three:
// two DBG records of exactly 453 bytes, each a line-number record of 248
// bytes and one cut short. The fourth is a guess that fits both FORTH,
// whose last part starts a record of its own, and every smaller table,
// whose last part shares the record (unconfirmed).
//
// The table's DBG records go out as the code does: the caller adds each
// command as the code it describes is written, and puts each record
// Add returns among the TIR records there.
const (
	lineRecordMax = 248
	lineFill      = 453
	lineImmediate = 120
	lineReserve   = 256
)

// LineTable packs a module's line-number table into DBG records.
type LineTable struct {
	// parts are the current DBG record's DST data: bytes, and the
	// addresses between them.
	parts []DSTItem
	// open is the line-number record being built: its commands.
	open []DSTItem
}

// NewLineTable starts a line-number table: the DBG record it starts with
// holds the source file's record, and the table's first commands.
func NewLineTable(source DSTRecord) *LineTable {
	t := &LineTable{}
	t.commit(dstItems(source)...)

	return t
}

// dstItems returns a DST record's bytes as items: its length and type,
// then its data, with each address in place.
func dstItems(r DSTRecord) []DSTItem {
	items := []DSTItem{{Bytes: []byte{byte(len(r.Data) + 1), byte(r.Type)}}}
	at := 0

	for _, a := range r.Addresses {
		items = append(items, DSTItem{Bytes: r.Data[at:a.Offset], Address: a.Commands})
		at = a.Offset + 4
	}

	return append(items, DSTItem{Bytes: r.Data[at:]})
}

// commit adds items to the current DBG record.
func (t *LineTable) commit(items ...DSTItem) {
	t.parts = append(t.parts, items...)
}

// openRecord returns the line-number record being built as DST items.
func (t *LineTable) openRecord() []DSTItem {
	n := 0
	for _, it := range t.open {
		n += it.size()
	}

	return append([]DSTItem{{Bytes: []byte{byte(n + 1), byte(DSTLineNumbers)}}}, t.open...)
}

// closeOpen moves the line-number record being built, if any, into the
// current DBG record.
func (t *LineTable) closeOpen() {
	if len(t.open) > 0 {
		t.commit(t.openRecord()...)
		t.open = nil
	}
}

// recordSize is how many bytes a DBG record of items takes: its type,
// each STORE IMMEDIATE's opcode and data, and each address's commands.
func recordSize(items []DSTItem) int {
	size, run := 1, 0

	end := func() {
		size += run + (run+lineImmediate-1)/lineImmediate
		run = 0
	}

	for _, it := range items {
		run += len(it.Bytes)

		if it.Address != nil {
			end()

			for _, c := range it.Address {
				enc, _ := c.encode(nil)
				size += len(enc)
			}
		}
	}

	end()

	return size
}

// Add adds one command to the table, returning the DBG records it
// filled, in order.
func (t *LineTable) Add(cmd DSTItem) []*TIR {
	var out []*TIR

	for {
		n := 0
		for _, it := range t.open {
			n += it.size()
		}

		withCmd := append(append([]DSTItem(nil), t.parts...), t.openRecordWith(cmd)...)

		switch {
		case recordSize(withCmd) > lineFill:
			// The DBG record is full: the line-number record being
			// built ends with it.
			t.closeOpen()
			out = append(out, t.flush())

			if len(t.parts) == 0 && len(t.open) == 0 && recordSize(t.openRecordWith(cmd)) > lineFill {
				// A command too long for any record: it goes alone.
				t.open = []DSTItem{cmd}

				return out
			}

			continue

		case len(t.open) > 0 && n+cmd.size()+1 > lineRecordMax:
			t.closeOpen()

			continue
		}

		t.open = append(t.open, cmd)

		return out
	}
}

// openRecordWith returns the line-number record being built, with cmd
// added, as DST items.
func (t *LineTable) openRecordWith(cmd DSTItem) []DSTItem {
	saved := t.open
	t.open = append(append([]DSTItem(nil), saved...), cmd)
	items := t.openRecord()
	t.open = saved

	return items
}

// Finish ends the table: the line-number record being built, the source's
// line count, and a last line-number record of end (the last segment's
// LineEnd). It returns the DBG records left.
func (t *LineTable) Finish(lines int, end DSTItem) []*TIR {
	var out []*TIR

	t.closeOpen()

	count := dstItems(DSTLineCountRecord(lines))
	if len(t.parts) > 0 && recordSize(t.parts)+lineReserve > lineFill {
		out = append(out, t.flush())
	}

	t.commit(count...)
	t.open = []DSTItem{end}
	t.closeOpen()

	return append(out, t.flush())
}

// flush returns the current DBG record, starting a new one.
func (t *LineTable) flush() *TIR {
	rec := &TIR{Type: RecDBG}

	var run []byte

	store := func() {
		for len(run) > 0 {
			n := min(len(run), lineImmediate)
			rec.Commands = append(rec.Commands, Command{Op: OpStoreImmediate, Data: append([]byte(nil), run[:n]...)})
			run = run[n:]
		}
	}

	for _, it := range t.parts {
		run = append(run, it.Bytes...)

		if it.Address != nil {
			store()
			rec.Commands = append(rec.Commands, it.Address...)
		}
	}

	store()

	t.parts = nil

	return rec
}
