package asm

// outKind is the kind of an outEvent.
type outKind int

const (
	// evSwitch: assembly moved to psect sect, at offset. The first time,
	// the object defines the psect there.
	evSwitch outKind = iota
	// evSet: ". =" moved the location to offset.
	evSet
	// evData: size bytes were stored at offset. The object takes them
	// from the psect's final contents, and any relocation among them
	// from the relocation list.
	evData
	// evGap: the location moved size bytes without storing anything
	// (.BLKx, or .ALIGN without a fill).
	evGap
	// evConst: the size-byte constant value was stored at offset through
	// the linker's stack, as real MACRO stores .ASCID's first longword.
	evConst
	// evEntry: .ENTRY stored the entry mask of sym at offset.
	evEntry
	// evPatch: the size-byte constant value was stored back at offset,
	// behind the location, as real MACRO fills in .ASCID's length.
	evPatch
)

// outEvent is one step of a MACRO-dialect assembly's output, in source
// order. Real MACRO writes its object's records as its second pass reads
// the source, so the order of what it defines and stores follows the
// source, not the psects (docs/PHASE-27.md, subtask 3's log). The object
// emitter (see Object) replays these events to write the same records.
type outEvent struct {
	kind   outKind
	sect   *section
	offset uint32
	size   uint32
	value  uint32
	sym    *symbol
	// implicit marks the switch into . BLANK . that code or data before
	// any .PSECT makes (see useBlankPsect).
	implicit bool
}

// logEvent records an output event, in the MACRO dialect only. A data
// event that continues the one before it extends it.
func (a *Assembler) logEvent(e outEvent) {
	if a.dialect != DialectMACRO {
		return
	}

	if n := len(a.events); n > 0 && e.kind == evData {
		last := &a.events[n-1]
		if last.kind == evData && last.sect == e.sect && last.offset+last.size == e.offset {
			last.size += e.size

			return
		}
	}

	a.events = append(a.events, e)
}

// enterSection makes s the current section, logging the switch.
func (a *Assembler) enterSection(s *section) {
	a.cur = s
	a.logEvent(outEvent{kind: evSwitch, sect: s, offset: s.loc})
}
