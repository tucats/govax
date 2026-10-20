package backup

import "hash/crc32"

// A block carries two checks. Both were found by solving for them over
// VMS's own save sets; neither is described in DIGITAL's manuals.
//
//   - The header checksum, a word at the header's end, is the CRC-16 of
//     the 256-byte header (the reflected polynomial %XA001, the VAX CRC
//     instruction's "CRC-16" table, starting from 0, nothing XORed at
//     the end), taken with the checksum word and the data CRC longword
//     as zero. BACKUP always writes it.
//   - The data CRC, a longword in the header, is the common CRC-32
//     (AUTODIN-II, reflected %XEDB88320, starting from all ones, the
//     result complemented: Go's crc32.ChecksumIEEE) of the whole block,
//     header and all, with the same two fields as zero. BACKUP writes it
//     only with /CRC, and otherwise sets the header's NOCRC flag.

// crc16Table is the table-driven form of the reflected CRC-16: entry n is
// the remainder of the byte n shifted through the polynomial eight
// times.
var crc16Table = func() (table [256]uint16) {
	for n := range table {
		crc := uint16(n)
		for range 8 {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0xA001
			} else {
				crc >>= 1
			}
		}

		table[n] = crc
	}

	return table
}()

// crc16 is the CRC-16 of data, starting from 0.
func crc16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc = crc>>8 ^ crc16Table[byte(crc)^b]
	}

	return crc
}

// HeaderChecksum is the checksum block's header should carry: the CRC-16
// of its first 256 bytes with the checksum word and the CRC longword
// taken as zero. block must hold at least the header.
func HeaderChecksum(block []byte) uint16 {
	var header [HeaderSize]byte

	copy(header[:], block[:HeaderSize])
	clearChecks(header[:])

	return crc16(header[:])
}

// DataCRC is the data CRC block should carry: the CRC-32 of the whole
// block with the checksum word and the CRC longword taken as zero.
func DataCRC(block []byte) uint32 {
	crc := crc32.NewIEEE()

	var header [HeaderSize]byte

	copy(header[:], block[:HeaderSize])
	clearChecks(header[:])
	crc.Write(header[:])
	crc.Write(block[HeaderSize:])

	return crc.Sum32()
}

// clearChecks zeroes the two fields neither check covers.
func clearChecks(header []byte) {
	for i := range 4 {
		header[bhdCRC+i] = 0
	}

	header[bhdChecksum] = 0
	header[bhdChecksum+1] = 0
}
