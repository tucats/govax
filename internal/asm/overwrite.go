package asm

// This file handles a store over bytes an earlier statement stored, as
// the RMS control block macros do: $FAB stores FAB$L_FNA as .ADDRESS FNA
// (0 by default), and then, given FNM=, goes back with ". =" and stores
// .ADDRESS of the name string there. Real MACRO writes both stores, in
// source order, and the linker's last store wins. So:
//
//   - each byte records the statements, and the output events, that
//     stored it (claim), and each relocation is written by the event that
//     stored its field (emitter.data), even when a later event stores over
//     it;
//   - a relocation or fixup whose field a later statement stores over is
//     superseded: it's still written, but it's no longer the field's
//     value, so it isn't one of Relocations(), and a fixup never stores
//     its value into the assembler's image (it would otherwise overwrite
//     the later value when its symbol is defined, or at the end of the
//     assembly).
//
// An event writes the bytes it stored: the image's, which a fixup may
// have finished since, unless a later event stored over them, and then
// the values it stored (byteOwner.value).

// claim records that the current statement, in the latest output event,
// stored the n bytes at offset in s, and supersedes what an earlier
// statement left to fill in among them.
func (a *Assembler) claim(s *section, offset, n uint32) {
	if s.owners == nil {
		s.owners = map[uint32][]byteOwner{}
	}

	event := -1
	if a.dialect == DialectMACRO {
		event = len(a.events) - 1
	}

	overwritten := false

	for p := offset; p < offset+n; p++ {
		history := s.owners[p]
		if len(history) > 0 && history[len(history)-1].stmt != a.stmt {
			overwritten = true
		}

		s.owners[p] = append(history, byteOwner{stmt: a.stmt, event: event, value: s.img.loadByte(s.base + p)})
	}

	if overwritten {
		a.supersede(s, offset, offset+n)
	}
}

// supersede marks the fixups and relocations of earlier statements whose
// fields overlap [lo, hi) in s.
func (a *Assembler) supersede(s *section, lo, hi uint32) {
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

	for i := range a.relocs {
		if r := &a.relocs[i]; overlaps(r.sect, r.offset, r.kind, r.stmt) {
			r.superseded = true
		}
	}
}
