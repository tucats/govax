package lbr

import (
	"encoding/binary"
	"fmt"
)

// This file expands the records of a data-reduced library: the DCX
// facility's type 0 expansion (vmssrc_archive/v73/dcx/lis/expand.lis,
// dcxdef.sdl). DCX codes each byte with a binary tree picked by the byte
// before it, so the map is a set of sub-maps, each a tree:
//
//   - the flags bit vector says whether each node is a leaf;
//   - an inner node's nodes byte is half the index of its "0" child, whose
//     "1" sibling follows it; a leaf's is the byte it stands for;
//   - the next vector, if the sub-map has one, gives the sub-map that
//     codes the byte after each leaf's byte.
//
// Bits are read from each input byte low bit first. A 0 bit goes to the
// current node's "0" child and a 1 bit to its "1" child, and an inner node
// whose child index is 0 marks the end of the record.

// Map layout ($DCXMAPDEF, $DCXSBMDEF).
const (
	dcxMapSanity = 1542824871 // DCXMAP$C_SANITY

	dcxMapSanityOff = 8
	dcxMapNSubs     = 16
	dcxMapSub0      = 18
	dcxMapLength    = 20

	sbmSize    = 0
	sbmMinChar = 2
	sbmFlags   = 6
	sbmNodes   = 8
	sbmNext    = 10
	sbmLength  = 12
)

// dcxMap is a data-reduced library's DCX map.
type dcxMap struct {
	subs []dcxSub
}

// dcxSub is one sub-map, as slices of the map.
type dcxSub struct {
	flags   []byte
	nodes   []byte
	next    []byte // nil when the sub-map has no next vector
	minChar int
}

// readDCXMap reads the library's DCX map: a length longword at the start of
// block vbn, then the map, in consecutive blocks (openclose.lis,
// lbr$dcx_map).
func (l *Library) readDCXMap(vbn uint32) (*dcxMap, error) {
	if _, err := l.block(vbn); err != nil {
		return nil, fmt.Errorf("lbr: DCX map: %w", err)
	}

	start := int64(vbn-1)*blockSize + 4
	size := int64(binary.LittleEndian.Uint32(l.data[start-4:]))

	if start+size > int64(len(l.data)) {
		return nil, fmt.Errorf("lbr: DCX map at block %d runs past the end of the file", vbn)
	}

	m, err := parseDCXMap(l.data[start : start+size])
	if err != nil {
		return nil, fmt.Errorf("lbr: DCX map at block %d: %w", vbn, err)
	}

	return m, nil
}

// parseDCXMap checks a DCX map and finds its sub-maps (expand.lis,
// dcx$expand_init).
func parseDCXMap(b []byte) (*dcxMap, error) {
	if len(b) < dcxMapLength || binary.LittleEndian.Uint32(b[dcxMapSanityOff:]) != dcxMapSanity {
		return nil, fmt.Errorf("not a DCX map")
	}

	if int(binary.LittleEndian.Uint32(b)) > len(b) {
		return nil, fmt.Errorf("map claims %d bytes", binary.LittleEndian.Uint32(b))
	}

	n := int(binary.LittleEndian.Uint16(b[dcxMapNSubs:]))
	m := &dcxMap{subs: make([]dcxSub, 0, n)}

	off := int(binary.LittleEndian.Uint16(b[dcxMapSub0:]))

	for i := range n {
		if off+sbmLength > len(b) {
			return nil, fmt.Errorf("sub-map %d is outside the map", i)
		}

		s := b[off:]
		size := int(binary.LittleEndian.Uint16(s[sbmSize:]))

		if size < sbmLength || off+size > len(b) {
			return nil, fmt.Errorf("sub-map %d claims %d bytes", i, size)
		}

		s = s[:size]
		flags := int(binary.LittleEndian.Uint16(s[sbmFlags:]))
		nodes := int(binary.LittleEndian.Uint16(s[sbmNodes:]))
		next := int(binary.LittleEndian.Uint16(s[sbmNext:]))

		if flags > size || nodes > size || next > size {
			return nil, fmt.Errorf("sub-map %d's vectors are outside it", i)
		}

		sub := dcxSub{flags: s[flags:], nodes: s[nodes:], minChar: int(s[sbmMinChar])}
		if next != 0 {
			sub.next = s[next:]
		}

		m.subs = append(m.subs, sub)
		off += size
	}

	return m, nil
}

// expand expands one record (expand.lis, dcx$do_expansion_0).
func (m *dcxMap) expand(in []byte) ([]byte, error) {
	if len(m.subs) == 0 {
		return append([]byte(nil), in...), nil
	}

	out := make([]byte, 0, 2*len(in))
	cur := &m.subs[0]
	node := 0

	for _, c := range in {
		for bit := range 8 {
			if c>>bit&1 != 0 {
				node++
			}

			if node >= len(cur.nodes) || node/8 >= len(cur.flags) {
				return nil, fmt.Errorf("DCX data leaves its sub-map")
			}

			if cur.flags[node/8]>>(node%8)&1 == 0 {
				node = 2 * int(cur.nodes[node])
				if node == 0 {
					return out, nil
				}

				continue
			}

			ch := cur.nodes[node]
			out = append(out, ch)

			if cur.next != nil {
				i := 2 * (int(ch) - cur.minChar)
				if i < 0 || i+2 > len(cur.next) {
					return nil, fmt.Errorf("DCX data has a byte its sub-map can't follow")
				}

				n := int(binary.LittleEndian.Uint16(cur.next[i:]))
				if n >= len(m.subs) {
					return nil, fmt.Errorf("DCX sub-map %d doesn't exist", n)
				}

				cur = &m.subs[n]
			}

			node = 0
		}
	}

	return nil, fmt.Errorf("DCX data has no end of record")
}
