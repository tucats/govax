package lbr

import (
	"encoding/binary"
	"errors"
)

// This file is a library's update history: the records LBR$PUT_HISTORY
// adds and LBR$GET_HISTORY returns (LIBRARY writes one for each change it
// makes, and LIBRARY/LIST/HISTORY shows them). The layout is what VMS
// 7.1's LBRSHR left in round 3's libraries (testdata/help90, W5 and H90T;
// docs/PHASE-90 - VMS help.md, subtask 11c):
//
//   - The records live in data blocks of their own, whose record count
//     (the block's first byte) stays 0. Each record is a marker word
//     (historyMark, the bytes BA AB), a length word, and the record's
//     bytes, packed with no alignment, one after another; a block that
//     fills goes on in the next (unconfirmed: no case filled one, and
//     govax links it as a module's data blocks are linked).
//   - The header holds two RFAs: the oldest record kept (lhdHisHead) and
//     where the next one goes (lhdHisTail). Their offsets count from the
//     block's data (DATA$C_DATA), not from its start: the first record of
//     a block is offset 0. Both are 0 in a library that has never had a
//     history record; the first one takes a new block, at the next VBN.
//   - LHD$W_NUMLUHREC counts the records kept and LHD$W_MAXLUHREC is the
//     most there may be. A record added when the history is full takes
//     the oldest off first, by moving the head past it: the block isn't
//     cleared and isn't given back (unconfirmed).
//
// A record is any bytes; LIBRARY's own are a time, a module count, the
// operation, a counted user name, and the modules' counted names.
const (
	lhdHisHead = 0x80 // RFA: the oldest record kept
	lhdHisTail = 0x86 // RFA: where the next record goes

	historyMark = 0xABBA // the word before each record's length
)

// The history's conditions.
var (
	ErrNoHistory  = errors.New("lbr: library has no update history") // LBR$_NOHISTORY
	ErrBadHistory = errors.New("lbr: update history damaged")        // LBR$_INTRNLERR
)

// UpdateHistory returns the library's update history records, the oldest first,
// as LBR$GET_HISTORY returns them. A library that keeps none is
// ErrNoHistory; one whose records can't be read ErrBadHistory.
func (l *Library) UpdateHistory() ([][]byte, error) {
	h := l.data[:blockSize]
	if le16(h, lhdMaxLUHRec) == 0 {
		return nil, ErrNoHistory
	}

	c := chain{data: l.data, pos: historyRFA(h, lhdHisHead)}

	var recs [][]byte

	for range le16(h, lhdNumLUHRec) {
		head, err := c.read(4)
		if err != nil || le16(head, 0) != historyMark {
			return recs, ErrBadHistory
		}

		rec, err := c.read(int(le16(head, 2)))
		if err != nil {
			return recs, ErrBadHistory
		}

		recs = append(recs, rec)
	}

	return recs, nil
}

// PutHistory adds rec to the library's update history, after taking the
// oldest record off if the history is full. As VMS 7.1's did (round 3's
// W5), the oldest goes before rec's length is checked, so a record longer
// than LBR$C_MAXRECSIZ, which is ErrRecordLength, still costs the oldest.
func (u *Updater) PutHistory(rec []byte) error {
	most := le16(u.data, lhdMaxLUHRec)
	if most == 0 {
		return ErrNoHistory
	}

	n := le16(u.data, lhdNumLUHRec)

	if n >= most {
		c := chain{data: u.data, pos: historyRFA(u.data, lhdHisHead)}

		head, err := c.read(4)
		if err != nil || le16(head, 0) != historyMark {
			return ErrBadHistory
		}

		if _, err := c.read(int(le16(head, 2))); err != nil {
			return ErrBadHistory
		}

		u.setHistoryRFA(lhdHisHead, c.pos)
		n--
		binary.LittleEndian.PutUint16(u.data[lhdNumLUHRec:], n)
	}

	if len(rec) > maxRecSize {
		return ErrRecordLength
	}

	tail := historyRFA(u.data, lhdHisTail)
	if tail.VBN == 0 {
		tail = RFA{VBN: u.reuseBlock(), Offset: dataData}
		u.setHistoryRFA(lhdHisHead, tail)
	}

	out := binary.LittleEndian.AppendUint16(nil, historyMark)
	out = binary.LittleEndian.AppendUint16(out, uint16(len(rec)))
	out = append(out, rec...)

	for len(out) > 0 {
		if tail.Offset >= blockSize {
			v := u.reuseBlock()
			binary.LittleEndian.PutUint32(u.block(tail.VBN)[dataLink:], v)
			tail = RFA{VBN: v, Offset: dataData}
		}

		k := copy(u.block(tail.VBN)[tail.Offset:], out)
		out = out[k:]
		tail.Offset += uint16(k)
	}

	u.setHistoryRFA(lhdHisTail, tail)
	binary.LittleEndian.PutUint16(u.data[lhdNumLUHRec:], n+1)

	return nil
}

// reuseBlock takes a block for the update history: the first of the
// deleted blocks' list if it has one, cleared, or else the next VBN. VMS
// 7.1's LIBRARIAN put H90E.HLB's first history record (round 1b) in the
// block the replacement before it had just freed; whether module data
// reuses deleted blocks too is unconfirmed (round 3's W3 showed a module
// going on in its session's block rather than into one).
func (u *Updater) reuseBlock() uint32 {
	v := u.hdr(lhdFreeVBN)
	if v == 0 || v > u.blocks() || u.hdr(lhdFreeBlk) == 0 {
		return u.newBlock()
	}

	u.setHdr(lhdFreeVBN, le32(u.block(v), dataLink))
	u.addHdr(lhdFreeBlk, -1)
	clear(u.block(v))

	return v
}

// historyRFA reads one of the header's history RFAs, its offset made one
// from the block's start.
func historyRFA(h []byte, off int) RFA {
	rfa := RFA{VBN: le32(h, off), Offset: le16(h, off+4)}
	if rfa.VBN != 0 {
		rfa.Offset += dataData
	}

	return rfa
}

// setHistoryRFA writes one of the header's history RFAs, its offset
// counted from the block's data.
func (u *Updater) setHistoryRFA(off int, rfa RFA) {
	u.setHdr(off, rfa.VBN)
	binary.LittleEndian.PutUint16(u.data[off+4:], rfa.Offset-dataData)
}

// chain reads bytes from a library's data blocks, from pos on, going on
// through each block's link.
type chain struct {
	data []byte
	pos  RFA // the offset from the block's start
}

// read returns the next n bytes.
func (c *chain) read(n int) ([]byte, error) {
	out := make([]byte, 0, n)

	for len(out) < n {
		if c.pos.VBN < 2 || int(c.pos.VBN)*blockSize > len(c.data) {
			return nil, ErrBadHistory
		}

		b := c.data[int(c.pos.VBN-1)*blockSize : int(c.pos.VBN)*blockSize]

		if c.pos.Offset >= blockSize {
			c.pos = RFA{VBN: le32(b, dataLink), Offset: dataData}

			continue
		}

		k := min(n-len(out), blockSize-int(c.pos.Offset))
		out = append(out, b[c.pos.Offset:int(c.pos.Offset)+k]...)
		c.pos.Offset += uint16(k)
	}

	return out, nil
}
