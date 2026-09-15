package asm

// image is the assembler's output store: bytes written at arbitrary VAX
// addresses, keyed by address rather than backed by one contiguous buffer.
// A real assembly can jump around — a .REGION switch between the P0 and S0
// deposit counters leaves a multi-gigabyte gap between them — so a sparse
// map is the simplest representation that doesn't force this package to
// model how large a real VAX's address space is. An address never written
// reads back as 0, which is what a Go map already does for a missing key.
type image struct {
	bytes map[uint32]byte
}

func newImage() *image { return &image{bytes: make(map[uint32]byte)} }

func (im *image) storeByte(addr uint32, b byte) error {
	im.bytes[addr] = b

	return nil
}

func (im *image) storeWord(addr uint32, w uint16) error {
	im.bytes[addr] = byte(w)
	im.bytes[addr+1] = byte(w >> 8)

	return nil
}

func (im *image) storeLongword(addr uint32, l uint32) error {
	im.bytes[addr] = byte(l)
	im.bytes[addr+1] = byte(l >> 8)
	im.bytes[addr+2] = byte(l >> 16)
	im.bytes[addr+3] = byte(l >> 24)

	return nil
}

func (im *image) loadByte(addr uint32) byte { return im.bytes[addr] }

// Bytes returns the contiguous region [from, to), zero-filling any address
// in that range that was never written. Callers own the choice of range —
// this package never guesses at one, since a sparse image can legitimately
// have gigabyte gaps (see the type doc above) that would make a naive
// "everything ever written" extraction enormous.
func (im *image) Bytes(from, to uint32) []byte {
	if to <= from {
		return nil
	}

	out := make([]byte, to-from)
	for a := from; a < to; a++ {
		out[a-from] = im.bytes[a]
	}
	
	return out
}
