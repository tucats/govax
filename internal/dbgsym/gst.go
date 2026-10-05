package dbgsym

import (
	"bytes"
	"fmt"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/symtab"
	"github.com/tucats/govax/internal/vmsimage"
)

// The global symbol table (GST) an image linked /DEBUG carries is an
// object module named for the image (docs/PHASE-29.md, "What real
// LINK/DEBUG writes"): a main header, the linker's name, a GSD record
// defining one absolute psect, GSD records of the image's global
// symbols, each with its final value, and an end of module record. It
// holds every global the link defined but those in shareable images:
// the modules' own, and the system services and other symbols the
// libraries supplied (SYS$OPEN). It's the debugger's fallback where no
// module's DST names an address (docs/DEBUG-RECORDS.md 1.3): the probe's
// sessions show globals naming addresses no data record covers
// (L^GLIMIT+24D, @#SYS$OPEN).
//
// Its records are in the variable-length layout of a record file, from
// the GST's first block (IHS$L_GSTVBN) on; the IHS gives their count.
// They run to the end of the file, which real LINK ends mid-block.

// readGST reads the image's global symbol table into p.Globals, adding
// base to each value.
//
// Unconfirmed: whether the debugger relocates a shareable image's
// globals by where it's loaded. A GST records no relocatability (its
// symbols are all absolute), so a constant (GLIMIT) and an address
// (GLOBDATA) look alike; every value is relocated here, which changes
// nothing for a main image (base 0), the only kind the fixtures have.
func (p *Program) readGST(img *vmsimage.Image, data []byte, base uint32) error {
	start := uint64(img.GSTVBN-1) * vmsimage.BlockSize
	if start >= uint64(len(data)) {
		return fmt.Errorf("the global symbol table (VBN %d) isn't in the file", img.GSTVBN)
	}

	records, err := obj.ReadRecords(bytes.NewReader(data[start:]))
	if err != nil {
		return fmt.Errorf("global symbol table: %w", err)
	}

	count := int(img.GSTRecordCount())
	if len(records) < count {
		return fmt.Errorf("global symbol table: %d records, not %d", len(records), count)
	}

	// What follows the GST's records is padding to the end of a block
	// (govax's LINK pads it, Phase 29's Decision 7).
	mod, err := obj.Decode(records[:count])
	if err != nil {
		return fmt.Errorf("global symbol table: %w", err)
	}

	p.Globals = symtab.New()

	for _, r := range mod.Records {
		gsd, ok := r.(*obj.GSD)
		if !ok {
			continue
		}

		for _, sub := range gsd.Subrecords {
			s, ok := sub.(*obj.Symbol)
			if !ok || !s.Defined() {
				continue // the psect, or (in no GST seen) a reference
			}

			flags := symtab.Global
			if isEntry(s.Type) {
				flags |= symtab.Entry
			}

			p.Globals.Set(symtab.Symbol{Name: s.Name, Value: s.Value + base, Flags: flags})
		}
	}

	return nil
}

// isEntry reports whether a GSD subrecord type defines an entry point
// (a CALLS/CALLG routine, with its register-save mask) or a procedure.
func isEntry(t obj.GSDType) bool {
	switch t {
	case obj.GSDEntry, obj.GSDEntryW, obj.GSDEntryV, obj.GSDEntryM,
		obj.GSDProcedure, obj.GSDProcedureW, obj.GSDProcedureV, obj.GSDProcedureM:
		return true
	}

	return false
}
