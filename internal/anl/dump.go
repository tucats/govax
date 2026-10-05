package anl

import (
	"fmt"
	"strings"
)

// dumpRowBytes is how many bytes one row of a hex dump shows.
const dumpRowBytes = 8

// The hex dump's two heading lines. The column numbers count bytes from
// the right, the way a VAX longword is read: the byte at the lowest
// address is on the right.
const (
	dumpHeading   = "  7  6  5  4  3  2  1  0          01234567"
	dumpUnderline = "------------------------          --------"
)

// hexDump adds a hex dump of data, indented by indent: the two heading
// lines, then a row per eight bytes. A row shows its bytes highest
// address first (right to left, as a VAX reads memory), then their offset
// in data, then the bytes as characters, lowest address first:
//
//	 77 20 2C 6F 6C 6C 65 48|  0000  |Hello, w|
//	          21 64 6C 72 6F|  0008  |orld!   |
func (r *report) hexDump(indent string, data []byte) {
	r.keep(keepDump, indent+dumpHeading)
	r.line(indent + dumpUnderline)

	for off := 0; off < len(data); off += dumpRowBytes {
		var hex, chars strings.Builder

		for i := dumpRowBytes - 1; i >= 0; i-- {
			if off+i < len(data) {
				fmt.Fprintf(&hex, " %02X", data[off+i])
			} else {
				hex.WriteString("   ")
			}
		}

		for i := range dumpRowBytes {
			if off+i < len(data) {
				chars.WriteByte(dumpChar(data[off+i]))
			} else {
				chars.WriteByte(' ')
			}
		}

		r.line(fmt.Sprintf("%s%s|  %04X  |%s|", indent, hex.String(), off, chars.String()))
	}
}

// dumpChar is how a hex dump shows a byte as a character: printable ASCII
// and the DEC Multinational characters (%X'A1' to %X'FE') as themselves,
// anything else as a period. The rule is read off real ANALYZE's dumps;
// %X'A0', which they never show, is taken as unprintable (unconfirmed).
func dumpChar(b byte) byte {
	switch {
	case b >= 0x20 && b <= 0x7E:
		return b
	case b >= 0xA1 && b <= 0xFE:
		return b
	}

	return '.'
}
