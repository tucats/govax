package dbgsym

import "fmt"

// The debug module table (DMT) is LINK/DEBUG's index to the DST
// (docs/DEBUG-RECORDS.md 2.3): an entry for each module with DST
// records, in link order, giving where the module's records are and the
// address ranges of its psect contributions. The debugger reads it to
// find which module an address is in without reading every module's
// records (it loads a module's symbols only when SET MODULE asks).
//
// An entry is a 12-byte header (DBG$K_DMT_HEADER_SIZE): the offset of
// the module's module begin record in the DST, the size of its records,
// a word count of psects and a zero word. Then each psect's start
// address and length, 8 bytes each (DBG$K_DMT_PSECT_SIZE).
const (
	dmtHeaderSize = 12
	dmtPsectSize  = 8
)

// applyDMT reads the debug module table dmt and gives each module its
// psect ranges (Module.Ranges), adding base to the addresses. Each entry
// must name a module the DST holds, by the offset of its module begin
// record, and give that module's size.
func (p *Program) applyDMT(dmt []byte, base uint32) error {
	byOffset := map[uint32]*Module{}
	for _, m := range p.Modules {
		byOffset[m.DSTOffset] = m
	}

	for at := 0; at < len(dmt); {
		if at+dmtHeaderSize > len(dmt) {
			return fmt.Errorf("debug module table entry at %#x: %w", at, errShort)
		}

		offset, size, count := le.Uint32(dmt[at:]), le.Uint32(dmt[at+4:]), int(le.Uint16(dmt[at+8:]))

		m, ok := byOffset[offset]
		if !ok {
			return fmt.Errorf("debug module table entry at %#x: no module begins at DST offset %#x", at, offset)
		}

		if m.DSTSize != 0 && m.DSTSize != size {
			return fmt.Errorf("debug module table: module %s's records are %d bytes, not %d", m.Name, m.DSTSize, size)
		}

		at += dmtHeaderSize
		if at+dmtPsectSize*count > len(dmt) {
			return fmt.Errorf("debug module table: module %s's psects: %w", m.Name, errShort)
		}

		m.Ranges = make([]Range, count)
		for i := range m.Ranges {
			m.Ranges[i] = Range{Address: le.Uint32(dmt[at:]) + base, Size: le.Uint32(dmt[at+4:])}
			at += dmtPsectSize
		}
	}

	return nil
}
