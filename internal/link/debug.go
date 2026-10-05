package link

import (
	"bytes"
	"encoding/binary"
	"sort"

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

// gstMaxRecord is the longest record the global symbol table's header
// gives (its MHD's maximum record size), which its GSD records are kept
// within.
const gstMaxRecord = 512

// gstPsectName is the one psect a global symbol table defines, absolute
// and empty: every symbol in it has its final value.
const gstPsectName = ".$$ABS$$."

// globalSymbolTable returns the global symbol table (GST) of an image
// linked /DEBUG, which the debugger falls back on where the DST says
// nothing: an object module named for the image, as real LINK writes it
// (docs/PHASE-29.md, "What real LINK/DEBUG writes"). Its records are a
// main header (the image's name and ident, the link time), the linker's
// name, a GSD record defining the absolute psect .$$ABS$$., GSD records
// of the global symbols, each absolute with its final value, and an end
// of module record. It returns the records in ODS-2's variable-length
// layout, and their count.
func (l *linker) globalSymbolTable() ([]byte, int, error) {
	date := mapDate(l.opts.Time)

	m := &obj.Module{Records: []obj.Record{
		&obj.MainHeader{
			MaxRecordSize: gstMaxRecord,
			Name:          l.opts.ImageName,
			Version:       l.imageID,
			Created:       date,
			Patched:       date,
		},
		&obj.TextHeader{Type: obj.HdrLNM, Text: "Linker " + l.opts.LinkerID},
		&obj.GSD{Subrecords: []obj.Subrecord{
			&obj.Psect{Flags: obj.PsectPIC | obj.PsectLIB | obj.PsectRD, Name: gstPsectName},
		}},
	}}

	gsd, size := &obj.GSD{}, 1

	for _, g := range l.gstSymbols() {
		s := &obj.Symbol{Type: obj.GSDSymbol, Flags: obj.SymDEF, Value: g.value, Name: g.name}
		if g.entry {
			s.Type, s.Mask = obj.GSDEntry, g.mask
		}

		b, err := obj.EncodeRecord(&obj.GSD{Subrecords: []obj.Subrecord{s}})
		if err != nil {
			return nil, 0, err
		}

		// Each GSD record holds as many symbols as fit; how real LINK
		// splits a long table isn't known (FORTH's eight fit in one).
		if n := len(b) - 1; size+n > gstMaxRecord && len(gsd.Subrecords) > 0 {
			m.Records = append(m.Records, gsd)
			gsd, size = &obj.GSD{}, 1
		}

		gsd.Subrecords = append(gsd.Subrecords, s)
		size += len(b) - 1
	}

	if len(gsd.Subrecords) > 0 {
		m.Records = append(m.Records, gsd)
	}

	m.Records = append(m.Records, &obj.EOM{})

	records, err := obj.Encode(m)
	if err != nil {
		return nil, 0, err
	}

	var buf bytes.Buffer
	if err := obj.WriteRecords(&buf, records); err != nil {
		return nil, 0, err
	}

	return buf.Bytes(), len(records), nil
}

// gstSymbols returns the global symbols the global symbol table lists, in
// its order: every symbol the link defines but those in shareable images.
// That's each one a module defines, the ones a library module defines
// that the link used (a library module searched selectively defines no
// others), and those govax's own tables define in its place.
//
// Their order is real LINK's for both images that show it, by a rule
// that isn't confirmed (docs/PHASE-29.md, subtask 18): symbols before
// entry points; within each, those a library or symbol source defined,
// in name order, and then the modules' own, the last defined first.
// TRACE's GST is SYS$IMGSTA, LEVEL, GLOBDATA, then SECOND, FIRST, TRACE;
// FORTH's is its seven SYS$ services by name, then FORTH.
func (l *linker) gstSymbols() []*global {
	var out []*global

	for _, g := range l.symbols {
		if g.defined && g.image == "" {
			out = append(out, g)
		}
	}

	// fromLibrary says a library module or a symbol source defined g,
	// rather than a module the link was given.
	fromLibrary := func(g *global) bool {
		return g.fromSource || g.option || (g.module != nil && g.module.library)
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]

		if a.entry != b.entry {
			return !a.entry
		}

		if la, lb := fromLibrary(a), fromLibrary(b); la != lb {
			return la
		} else if la {
			return a.name < b.name
		}

		return a.defSeq > b.defSeq
	})

	return out
}
