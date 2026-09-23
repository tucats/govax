package rtl

import "github.com/tucats/govax/internal/p1vector"

// p1VectorByMatchAddr indexes p1vector.Table by the exact pc value
// call_service matches against — Addr for a CALL-reached entry, Addr-2 for
// a JMP-reached one (see p1vector.Entry.Jmp) — built once at package init
// rather than linearly scanned per lookup.
var p1VectorByMatchAddr = func() map[uint32]p1vector.Entry {
	m := make(map[uint32]p1vector.Entry, len(p1vector.Table))
	for _, e := range p1vector.Table {
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
func lookupP1Vector(pc uint32) (p1vector.Entry, bool) {
	e, ok := p1VectorByMatchAddr[pc]

	return e, ok
}
