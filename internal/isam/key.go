package isam

import (
	"bytes"
	"encoding/binary"
)

// Type is a key's data type, by VMS's values (KEY$C_, the same numbers
// as XAB$C_): a kind in the low bits, and 0x20 for the descending form.
type Type uint8

// The key types. COL, a collated key, needs the National Character Set
// utility and is left out (the phase doc's Decision 7).
const (
	String   Type = 0 // STG: bytes, compared unsigned
	Int2     Type = 1 // IN2
	Bin2     Type = 2 // BN2
	Int4     Type = 3 // IN4
	Bin4     Type = 4 // BN4
	Packed   Type = 5 // PAC: packed decimal
	Int8     Type = 6 // IN8
	Bin8     Type = 7 // BN8
	Collated Type = 8 // COL

	Descending Type = 0x20 // added to any of them
)

// Kind is the type without its direction.
func (t Type) Kind() Type { return t &^ Descending }

// IsDescending reports whether the key sorts from high to low.
func (t Type) IsDescending() bool { return t&Descending != 0 }

// Compare orders two whole values of the key, in the key's own order:
// negative if a comes before b, whichever way the key runs. Records with
// equal keys keep the order they were written in, which isn't Compare's
// to say.
func (k *Key) Compare(a, b []byte) int {
	c := k.compareAscending(a, b)
	if k.Type.IsDescending() {
		return -c
	}

	return c
}

// compareAscending orders two values from low to high.
func (k *Key) compareAscending(a, b []byte) int {
	switch k.Type.Kind() {
	case Int2, Int4, Int8:
		x, y := signedValue(a), signedValue(b)

		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}

		return 0

	case Bin2, Bin4, Bin8:
		x, y := unsignedValue(a), unsignedValue(b)

		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}

		return 0

	case Packed:
		return comparePacked(a, b)
	}

	return bytes.Compare(a, b)
}

// unsignedValue reads a little-endian integer of 2, 4, or 8 bytes: the
// VAX keeps a number's lowest byte first.
func unsignedValue(b []byte) uint64 {
	var buf [8]byte

	copy(buf[:], b)

	return binary.LittleEndian.Uint64(buf[:])
}

// signedValue is unsignedValue with the top byte's sign carried up.
func signedValue(b []byte) int64 {
	var fill byte
	if len(b) > 0 && b[len(b)-1]&0x80 != 0 {
		fill = 0xFF
	}

	buf := [8]byte{fill, fill, fill, fill, fill, fill, fill, fill}
	copy(buf[:], b)

	return int64(binary.LittleEndian.Uint64(buf[:]))
}

// comparePacked orders two packed decimal strings by value. Packed
// decimal holds a digit in each 4 bits, most significant first, and the
// last 4 bits are the sign: B and D are negative, any other positive
// (the VAX Architecture's rule).
func comparePacked(a, b []byte) int {
	an, ad := packedDigits(a)
	bn, bd := packedDigits(b)

	switch {
	case an && !bn:
		return -1
	case !an && bn:
		return 1
	case an:
		return bytes.Compare(bd, ad)
	}

	return bytes.Compare(ad, bd)
}

// packedDigits splits a packed value into its sign and a digit per byte.
// Digits compare byte by byte, since a key's values are all one length. A
// value of zero is never negative, so -0 and +0 compare alike.
func packedDigits(p []byte) (bool, []byte) {
	if len(p) == 0 {
		return false, nil
	}

	digits := make([]byte, 0, 2*len(p))
	zero := true

	for i, b := range p {
		digits = append(digits, b>>4)

		if i < len(p)-1 {
			digits = append(digits, b&0x0F)
		}
	}

	for _, d := range digits {
		if d != 0 {
			zero = false

			break
		}
	}

	sign := p[len(p)-1] & 0x0F

	return !zero && (sign == 0x0B || sign == 0x0D), digits
}

// comparePrefix orders a whole stored key against a search value. A
// string search value may be shorter than the key: then only that many
// leading bytes compare, which is RMS's generic match.
func (k *Key) comparePrefix(stored, value []byte) int {
	if len(value) < len(stored) {
		stored = stored[:len(value)]
	}

	return k.Compare(stored, value)
}

// checkValue refuses a search value of a size this key can't take.
func (k *Key) checkValue(value []byte) error {
	switch {
	case len(value) == 0 || len(value) > k.Size:
		return ErrKeySize
	case k.Type.Kind() != String && len(value) != k.Size:
		return ErrKeySize
	}

	return nil
}

// Value takes this key's value out of a record, joining its segments. It
// returns false when the record is too short to hold the whole key.
func (k *Key) Value(record []byte) ([]byte, bool) {
	if len(record) < k.MinRecord {
		return nil, false
	}

	value := make([]byte, 0, k.Size)
	for _, s := range k.Segments {
		value = append(value, record[s.Position:s.Position+s.Size]...)
	}

	return value, true
}

// IsNull reports whether a value is this key's null value, for a key
// that has one: such a record isn't in the key's index. A string key's is
// every byte the null character, and a numeric key's zero.
func (k *Key) IsNull(value []byte) bool {
	if k.Flags&FlagNull == 0 {
		return false
	}

	want := k.Null
	if k.Type.Kind() != String {
		want = 0
	}

	for _, b := range value {
		if b != want {
			return false
		}
	}

	return true
}

// Indexes reports whether a record belongs in this key's index, and the
// value it's filed under: a record must hold the whole key, and its value
// mustn't be null. The primary key must always hold.
func (k *Key) Indexes(record []byte) ([]byte, bool) {
	value, ok := k.Value(record)
	if !ok || k.IsNull(value) {
		return nil, false
	}

	return value, true
}

// assemble rebuilds a whole record from a primary key's value and the
// rest of the record's bytes. A primary key is stored first whatever its
// position, and its segments' bytes are left out of the rest, so they
// go back where the descriptor says. A deleted record may hold fewer
// bytes than its key's positions: what's there is laid out regardless.
func (k *Key) assemble(value, rest []byte) []byte {
	record := make([]byte, 0, len(value)+len(rest))

	for {
		at := len(record)
		placed := false
		used := 0

		for _, s := range k.Segments {
			if at >= s.Position && at < s.Position+s.Size && used+at-s.Position < len(value) {
				record = append(record, value[used+at-s.Position])
				placed = true

				break
			}

			used += s.Size
		}

		if placed {
			continue
		}

		if len(rest) == 0 {
			return record
		}

		record = append(record, rest[0])
		rest = rest[1:]
	}
}
