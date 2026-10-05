package link

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/obj"
)

// This file builds the tables an image linked /DEBUG has beside its debug
// symbol table (docs/PHASE-29.md, subtasks 17 and 18, and docs/
// DEBUG-RECORDS.md, 1.3 and 2.3), which the debugger reads to find its
// way around the DST.

// DMT entry sizes: an entry's header (DBG$K_DMT_HEADER_SIZE), and each of
// its psects (DBG$K_DMT_PSECT_SIZE).
const (
	dmtHeaderSize = 12
	dmtPsectSize  = 8
)

// debugModuleTable returns the debug module table (DMT): an entry for each
// module that put records in the DST, in link order. An entry is the
// offset of the module's first DST record (its module begin) and the size
// of its records, a word count of its psects and a zero word, then each
// psect's base and length. A module's psects are its own contributions to
// the relocatable psects, in its object's psect order, as in real LINK's
// images (TRACE's DATA, $CODE, $CONST): they're where its code and data
// are, which the debugger uses to find the module an address is in.
func (l *linker) debugModuleTable() []byte {
	var dmt []byte

	le := binary.LittleEndian

	for _, m := range l.modules {
		if m.dstEnd == m.dstStart {
			continue
		}

		var psects []*contribution

		for _, c := range m.contribs {
			if c.psect.flags&obj.PsectREL != 0 {
				psects = append(psects, c)
			}
		}

		entry := make([]byte, dmtHeaderSize+dmtPsectSize*len(psects))
		le.PutUint32(entry[0:], m.dstStart)
		le.PutUint32(entry[4:], m.dstEnd-m.dstStart)
		le.PutUint16(entry[8:], uint16(len(psects)))

		for i, c := range psects {
			at := entry[dmtHeaderSize+dmtPsectSize*i:]
			le.PutUint32(at[0:], c.psect.base+c.offset)
			le.PutUint32(at[4:], c.size)
		}

		dmt = append(dmt, entry...)
	}

	return dmt
}
