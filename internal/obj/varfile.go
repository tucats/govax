package obj

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// On VMS an object module is an RMS variable-length record file, and the
// record boundaries are part of the format. A host file has no records, so
// govax stores an object module in a host file the way ODS-2 lays out a
// variable-length record file on disk: each record is a 2-byte
// little-endian length, then the record's bytes, then a zero pad byte if
// the length is odd, so every record starts on an even offset. Those are
// the same bytes as a raw copy of the file's blocks, so an object module
// copied out of an ODS-2 container block by block reads the same way.
//
// On disk, a length of 0xFFFF means the rest of the 512-byte block holds
// no more records (RMS writes it when a record won't fit and records may
// not span blocks); the next record starts at the next block. ReadRecords
// honors it, and WriteRecords never needs it.

// blockSize is an ODS-2 disk block's size.
const blockSize = 512

// endOfBlock is the record length that marks the rest of a block unused.
const endOfBlock = 0xFFFF

// ReadRecords reads records in the variable-length layout described above
// until the end of r.
func ReadRecords(r io.Reader) ([][]byte, error) {
	br := bufio.NewReader(r)

	var (
		out    [][]byte
		offset int64
		lenBuf [2]byte
	)

	for {
		if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}

			return nil, fmt.Errorf("record length at offset %d: %w", offset, err)
		}

		offset += 2
		n := int(binary.LittleEndian.Uint16(lenBuf[:]))

		if n == endOfBlock {
			skip := (blockSize - offset%blockSize) % blockSize
			if _, err := br.Discard(int(skip)); err != nil && !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("skipping to the next block at offset %d: %w", offset, err)
			}

			offset += skip

			continue
		}

		rec := make([]byte, n)
		if _, err := io.ReadFull(br, rec); err != nil {
			return nil, fmt.Errorf("record of %d bytes at offset %d: %w", n, offset, err)
		}

		offset += int64(n)
		
		out = append(out, rec)

		if n%2 != 0 {
			if _, err := br.ReadByte(); err != nil && !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("pad byte at offset %d: %w", offset, err)
			}

			offset++
		}
	}
}

// WriteRecords writes records in the variable-length layout described
// above.
func WriteRecords(w io.Writer, records [][]byte) error {
	bw := bufio.NewWriter(w)

	for i, rec := range records {
		if len(rec) >= endOfBlock {
			return fmt.Errorf("record %d is %d bytes, too long for a variable-length record", i+1, len(rec))
		}

		var lenBuf [2]byte

		binary.LittleEndian.PutUint16(lenBuf[:], uint16(len(rec)))

		if _, err := bw.Write(lenBuf[:]); err != nil {
			return err
		}

		if _, err := bw.Write(rec); err != nil {
			return err
		}

		if len(rec)%2 != 0 {
			if err := bw.WriteByte(0); err != nil {
				return err
			}
		}
	}

	return bw.Flush()
}
