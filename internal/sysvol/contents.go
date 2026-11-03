package sysvol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// BlockSize is the size of a Binary file's records: a disk block.
const BlockSize = 512

// Canonical returns data in the one form this package compares and sums
// contents in, for format f.
//
// A text file's records are its lines, so its canonical form is its lines
// each ended by a line feed: a carriage return before a line feed is
// dropped, and a last line without one gets one. That is exactly what
// joining the records read back from the volume gives (Join), so a file
// govax wrote compares equal to itself however the host's copy ended its
// lines.
//
// A binary file's canonical form is its bytes padded with zeros to a
// whole number of blocks, as its fixed-length records hold them.
func Canonical(data []byte, f Format) []byte {
	if f == Binary {
		if extra := len(data) % BlockSize; extra != 0 {
			padded := make([]byte, len(data)+BlockSize-extra)
			copy(padded, data)

			return padded
		}

		return data
	}

	text := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if len(text) > 0 && text[len(text)-1] != '\n' {
		text = append(text, '\n')
	}

	return text
}

// Records splits canonical contents into the records that hold them on
// the volume: a text file's lines, without their line feeds, or a binary
// file's blocks.
func Records(contents []byte, f Format) [][]byte {
	var records [][]byte

	if f == Binary {
		for start := 0; start < len(contents); start += BlockSize {
			end := min(start+BlockSize, len(contents))
			records = append(records, contents[start:end])
		}

		return records
	}

	for len(contents) > 0 {
		i := bytes.IndexByte(contents, '\n')
		if i < 0 {
			records = append(records, contents)

			break
		}

		records = append(records, contents[:i])
		contents = contents[i+1:]
	}

	return records
}

// Join is Records' inverse: the canonical contents of the records read
// from a file of format f.
func Join(records [][]byte, f Format) []byte {
	var b bytes.Buffer

	for _, r := range records {
		b.Write(r)

		if f == Text {
			b.WriteByte('\n')
		}
	}

	return Canonical(b.Bytes(), f)
}

// Sum is a SHA-256 digest of a file's canonical contents: what the
// manifest records of each file govax supplied.
type Sum [sha256.Size]byte

// SumOf returns the digest of canonical contents.
func SumOf(contents []byte) Sum {
	return sha256.Sum256(contents)
}

// String is the digest in lowercase hexadecimal, as the manifest holds it.
func (s Sum) String() string {
	return hex.EncodeToString(s[:])
}

// ParseSum reads a digest written by String.
func ParseSum(text string) (Sum, error) {
	var s Sum

	b, err := hex.DecodeString(text)
	if err != nil || len(b) != len(s) {
		return s, fmt.Errorf("%q is not a SHA-256 digest", text)
	}

	copy(s[:], b)

	return s, nil
}
