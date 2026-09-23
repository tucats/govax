package rtl

import "github.com/tucats/govax/internal/vmsdef"

// p1VectorByMatchAddr indexes vmsdef.P1VectorTable by the exact pc value
// call_service matches against — Addr for a CALL-reached entry, Addr-2 for
// a JMP-reached one (see vmsdef.P1VectorEntry.Jmp) — built once at package
// init rather than linearly scanned per lookup.
var p1VectorByMatchAddr = func() map[uint32]vmsdef.P1VectorEntry {
	m := make(map[uint32]vmsdef.P1VectorEntry, len(vmsdef.P1VectorTable))
	for _, e := range vmsdef.P1VectorTable {
		addr := e.Addr
		if e.Jmp {
			addr -= 2
		}
		m[addr] = e
	}

	return m
}()

// lookupP1Vector finds the SYS$ service name whose vector address matches
// pc, matching call_service's own linear search.
func lookupP1Vector(pc uint32) (vmsdef.P1VectorEntry, bool) {
	e, ok := p1VectorByMatchAddr[pc]

	return e, ok
}
