package asm

// This file handles a store over bytes an earlier statement stored, as
// the RMS control block macros do: $FAB stores FAB$L_FNA as .LONG FNA
// (0 by default), and then, given FNM=, goes back with ". =" and stores
// .ADDRESS of the name string there. The later store wins, as it does
// when the linker runs real MACRO's object, so:
//
//   - a fixup of the earlier statement whose field is stored over is
//     never applied (it would otherwise overwrite the later value when
//     its symbol is defined, or at the end of the assembly), and its
//     relocation, if it already has one, is dropped;
//   - in the object, a relocation is written by the output event that
//     last stored its field (see emitter.data); an earlier event that
//     stored the same bytes writes the final bytes there. Real MACRO's
//     earlier event writes what was stored at the time, so only a
//     constant stored over with a different constant differs from its
//     object, and the linked image is the same either way.

// claim records that the current statement, in the latest output event,
// stored the n bytes at offset in s, and cancels what an earlier
// statement left to fill in among them.
func (a *Assembler) claim(s *section, offset, n uint32) {
	if s.owners == nil {
		s.owners = map[uint32]byteOwner{}
	}

	event := -1
	if a.dialect == DialectMACRO {
		event = len(a.events) - 1
	}

	overwritten := false

	for p := offset; p < offset+n; p++ {
		if o, ok := s.owners[p]; ok && o.stmt != a.stmt {
			overwritten = true
		}

		s.owners[p] = byteOwner{stmt: a.stmt, event: event}
	}

	if overwritten {
		a.cancelOverwritten(s, offset, offset+n)
	}
}

// cancelOverwritten cancels the fixups and relocations of earlier
// statements whose fields overlap [lo, hi) in s.
func (a *Assembler) cancelOverwritten(s *section, lo, hi uint32) {
	overlaps := func(sect *section, loc uint32, kind fixupKind, stmt int) bool {
		return sect == s && stmt != a.stmt && loc < hi && loc+uint32(fixupSize(kind)) > lo
	}

	for _, f := range a.ready {
		if overlaps(f.sect, f.location, f.kind, f.stmt) {
			f.dead = true
		}
	}

	for _, sym := range a.symbols.byName {
		for _, f := range sym.forward {
			if overlaps(f.sect, f.location, f.kind, f.stmt) {
				f.dead = true
			}
		}
	}

	kept := a.relocs[:0]

	for _, r := range a.relocs {
		if !overlaps(r.sect, r.offset, r.kind, r.stmt) {
			kept = append(kept, r)
		}
	}

	a.relocs = kept
}
