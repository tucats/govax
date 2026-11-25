package diff

import (
	"fmt"
	"strings"
)

// The hexadecimal and octal listings (/MODE=HEXADECIMAL and OCTAL), as VMS
// 7.1 wrote them (testdata/dif116's F19 to F23, F33, F36, F48, F49, I33,
// N64, R13). Each record is a heading and a dump, as DUMP lays one out:
//
//	(a blank line)
//	RECORD NUMBER 5 (00000005) LENGTH 11 (0000000B)
//	(a blank line)
//	            352065 6E696C20 61746164 data line 5..... 000000
//
// Each dump line is a run of longwords, the highest first, each a blank and
// its digits (8 in hexadecimal, 11 in octal, zero-filled; a longword the
// record only partly fills shows only its bytes' digits, right-aligned, and
// one it doesn't reach is blank), then a blank, the bytes as text (Printable,
// four a longword, the record's end filled with periods), a blank, and the
// line's offset in the record, six digits in the radix. The heading's two
// numbers in parentheses are in the radix too, eight digits; the heading ends
// with a blank.
//
// A line holds the most longwords that fit in one less than the width, an
// even number of them (a multiple of 8 bytes): 8 hexadecimal longwords at
// 132, 4 at 80, 6 octal ones at 132. The rule is fitted to those three
// widths; another width's choice is unconfirmed, as is an octal offset past
// the first line (no record in the probe was long enough).

// Radix is a listing's radix, /MODE's keywords.
type Radix int

const (
	// ASCII is the records as text, the default.
	ASCII Radix = iota

	// Hexadecimal and Octal are the records dumped in that radix.
	Hexadecimal
	Octal
)

// dumpLayout is a radix's dump line: the digits of a longword, and how many
// longwords a line holds at a width.
type dumpLayout struct {
	radix     Radix
	digits    int // a whole longword's digits
	longwords int // longwords a line
}

func newDumpLayout(radix Radix, width int) dumpLayout {
	d := dumpLayout{radix: radix, digits: 8}
	if radix == Octal {
		d.digits = 11
	}

	d.longwords = 2
	for n := 4; d.lineLength(n) <= width-1; n += 2 {
		d.longwords = n
	}

	return d
}

// lineLength is a dump line's length with n longwords: each a blank and its
// digits, a blank, four characters a longword, a blank, and six digits.
func (d dumpLayout) lineLength(n int) int {
	return n*(d.digits+1) + 1 + 4*n + 1 + 6
}

// number is a value in the radix, zero-filled to width digits.
func (d dumpLayout) number(v uint64, width int) string {
	if d.radix == Octal {
		return fmt.Sprintf("%0*o", width, v)
	}

	return fmt.Sprintf("%0*X", width, v)
}

// heading is a record's heading line.
func (d dumpLayout) heading(number int, length int) string {
	return fmt.Sprintf("RECORD NUMBER %d (%s) LENGTH %d (%s) ",
		number, d.number(uint64(number), 8), length, d.number(uint64(length), 8))
}

// dumpRecord lists one record in the radix: a blank line, its heading
// (with " ***CHANGE***" after it when a change bar marks the record, N64),
// a blank line, and its dump lines. The heading is cut at a dump line's
// length (N64's "***CHANGE**" at /WIDTH=80).
func dumpRecord(l *Listing, d dumpLayout, number int, text []byte, changed bool) {
	heading := d.heading(number, len(text))
	if changed {
		heading += " ***CHANGE***"
	}

	if n := d.lineLength(d.longwords); len(heading) > n {
		heading = heading[:n]
	}

	l.Lines = append(l.Lines, "")
	l.Cut(heading)
	l.Lines = append(l.Lines, "")

	per := 4 * d.longwords

	for at := 0; at < len(text); at += per {
		l.Cut(d.line(text[at:min(at+per, len(text))], at))
	}
}

// line is one dump line of bytes, at offset at in its record.
func (d dumpLayout) line(bytes []byte, at int) string {
	var b strings.Builder

	for k := d.longwords - 1; k >= 0; k-- {
		b.WriteByte(' ')

		lo := 4 * k
		if lo >= len(bytes) {
			b.WriteString(strings.Repeat(" ", d.digits))

			continue
		}

		part := bytes[lo:min(lo+4, len(bytes))]

		var v uint64
		for i := len(part) - 1; i >= 0; i-- {
			v = v<<8 | uint64(part[i])
		}

		digits := 2 * len(part)
		if d.radix == Octal {
			digits = (8*len(part) + 2) / 3
		}

		fmt.Fprintf(&b, "%*s", d.digits, d.number(v, digits))
	}

	b.WriteByte(' ')

	for _, c := range bytes {
		b.WriteByte(Printable(c))
	}

	b.WriteString(strings.Repeat(".", 4*d.longwords-len(bytes)))
	b.WriteByte(' ')
	b.WriteString(d.number(uint64(at), 6))

	return b.String()
}

// Printable is a byte as a dump shows it: itself, or a period for a control
// character, DEL, a C1 control (%X80 to %X9F), or %XFF (R13, I33); the rest
// of DEC's multinational set is kept.
func Printable(c byte) byte {
	if c < 0x20 || c == 0x7F || (c >= 0x80 && c < 0xA0) || c == 0xFF {
		return '.'
	}

	return c
}
