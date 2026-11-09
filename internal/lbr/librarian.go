package lbr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// This file is what LIBRARY (VMS's LIBRARIAN) does to a library that
// already exists, which it changes in place through LBRSHR rather than
// laying out afresh (docs/PHASE-90 - VMS help.md, subtask 11e): a module
// written whole by an Updater, LIBRARIAN's update history records and
// their listing, and LIBRARY/DATA's rebuilt libraries. Round 3 of
// testdata/help90 showed each, in VMS 7.1's H90T.TLB, WD1.HLB, and
// WD8.HLB.

// PutModule writes e as a module where the next record goes: its header
// (e's flags, insertion time, and user data, no keys counted yet), its
// records, and its end. It returns where the module starts, for
// InsertKey.
func (u *Updater) PutModule(e *Entry) RFA {
	rfa := u.putHeader(e)

	for _, r := range e.Records {
		u.write(r)
	}

	u.write(eot)
	u.inModule = false

	return rfa
}

// PutAbandoned writes e as a module whose insertion failed partway, as
// Builder.Abandon does: its header and records, and no end.
func (u *Updater) PutAbandoned(e *Entry) {
	u.putHeader(e)

	for _, r := range e.Records {
		u.write(r)
	}

	u.inModule = false
}

func (u *Updater) putHeader(e *Entry) RFA {
	mhd := make([]byte, mhdLen+u.userSize())
	mhd[0] = e.Flags
	mhd[1] = mhdID
	binary.LittleEndian.PutUint64(mhd[8:], e.Inserted)
	copy(mhd[mhdLen:], e.UserData)

	rfa := u.write(mhd)
	u.inModule = true
	u.addHdr(lhdModHdrs, 1)

	return rfa
}

// LIBRARIAN's history record operations: the byte after a record's module
// count (round 1b's H90E.HLB: ALPS deleted, CONLY inserted by /REPLACE,
// ALPHA replaced).
const (
	HistoryDeleted  = 1
	HistoryInserted = 2
	HistoryReplaced = 3
)

// historyUserLen is the user name's length in LIBRARIAN's history record.
const historyUserLen = 12

// historyOperations are the words LIBRARY/LIST/HISTORY shows for each
// operation.
var historyOperations = map[byte]string{
	HistoryInserted: "inserted",
	HistoryDeleted:  "deleted",
	HistoryReplaced: "replaced",
}

// LibrarianHistory is one of LIBRARIAN's update history records: the
// time, a module count word, the operation, the user name counted and
// blank-padded to 12, then each module's name counted (round 3's H90T:
// "SYSTEM replaced 1 module", T2).
func LibrarianHistory(now uint64, op byte, user string, names []string) []byte {
	rec := binary.LittleEndian.AppendUint64(nil, now)
	rec = binary.LittleEndian.AppendUint16(rec, uint16(len(names)))
	rec = append(rec, op, historyUserLen)
	rec = append(rec, field(strings.ToUpper(user), historyUserLen)...)

	for _, n := range names {
		rec = append(rec, byte(len(n)))
		rec = append(rec, n...)
	}

	return rec
}

// historyEntry is a LIBRARIAN history record read back.
type historyEntry struct {
	time  uint64
	op    byte
	user  string
	names []string
}

// readLibrarianHistory reads rec as one of LIBRARIAN's records, or reports
// that it isn't one: a record the LBR$ routines wrote, which VMS 7.1's
// LIBRARY/LIST/HISTORY couldn't format (round 3's W5 and H90T).
func readLibrarianHistory(rec []byte) (historyEntry, bool) {
	if len(rec) < 12 {
		return historyEntry{}, false
	}

	h := historyEntry{time: binary.LittleEndian.Uint64(rec), op: rec[10]}
	count := int(le16(rec, 8))

	if _, ok := historyOperations[h.op]; !ok || int(rec[11]) > len(rec)-12 {
		return historyEntry{}, false
	}

	h.user = strings.TrimRight(string(rec[12:12+int(rec[11])]), " ")

	at := 12 + int(rec[11])
	for range count {
		if at >= len(rec) || at+1+int(rec[at]) > len(rec) {
			return historyEntry{}, false
		}

		h.names = append(h.names, string(rec[at+1:at+1+int(rec[at])]))
		at += 1 + int(rec[at])
	}

	if at != len(rec) {
		return historyEntry{}, false
	}

	return h, true
}

// HistoryFormatError is LIBRARY/LIST/HISTORY's failure on a history record
// that isn't LIBRARIAN's: the listing's lines up to it are in the listing
// List returns, and the messages are VMS 7.1's (FAOFAIL and HISTERR, each
// with the access violation its FAO call met; round 3's W5 and H90T).
type HistoryFormatError struct{}

func (HistoryFormatError) Error() string { return "lbr: an update history record isn't LIBRARIAN's" }

// historyLines is LIBRARY/LIST/HISTORY's update history section: a heading,
// then each record's line and its modules' names, each followed by a blank
// line. The names fill lines of width, in columns a key and two blanks wide,
// as /NAMES's symbols do (unconfirmed: round 3's records have one name).
func (l *Library) historyLines(width int) ([]string, error) {
	if l.HistoryRecords == 0 {
		return nil, nil
	}

	out := []string{"", "Library Update History", ""}

	recs, err := l.UpdateHistory()
	if err != nil {
		return out, HistoryFormatError{}
	}

	for _, rec := range recs {
		h, ok := readLibrarianHistory(rec)
		if !ok {
			return out, HistoryFormatError{}
		}

		plural := "s"
		if len(h.names) == 1 {
			plural = ""
		}

		out = append(out, fmt.Sprintf("%s %-8s %3d module%s on %s", field(h.user, historyUserLen), historyOperations[h.op],
			len(h.names), plural, listDate(h.time)))

		column := l.KeySize() + 2
		line := ""

		for _, n := range h.names {
			if line != "" && len(line)+column > width {
				out = append(out, line)
				line = ""
			}

			line += field(n, column)
		}

		out = append(out, line, "")
	}

	return out, nil
}

// Rebuild returns l laid out afresh, as LIBRARY/DATA=REDUCE (reduce) and
// /DATA=EXPAND write it (round 3's WD8.HLB;2 and ;3): its modules in index
// order, the index blocks only those used, created and updated at now. A
// reduced library has the sanity longword of DCX data and a DCX map after
// its index blocks. govax's map has no sub-maps, so the records go in as
// they are, which is what VMS 7.1's DCX made of WD8's few records; what it
// makes of more is unconfirmed.
func Rebuild(l *Library, now uint64, reduce bool) ([]byte, error) {
	b, err := Edit(l)
	if err != nil {
		return nil, err
	}

	b.preallocate = 0
	b.Created, b.Updated = now, now
	b.reduced = reduce

	names := b.Names()

	b.order = make([]*Entry, 0, len(names))
	for _, n := range names {
		b.order = append(b.order, b.entries[b.fold(n)])
	}

	return b.Bytes(), nil
}

// emptyDCXMap is the DCX map block VMS 7.1's LIBRARY/DATA=REDUCE wrote for
// WD8.HLB: the map's length, then a map of 20 bytes with the sanity
// longword and no sub-maps. The two bytes after it, 02 FF, were in VMS's
// block too, and are copied unexplained.
func emptyDCXMap() []byte {
	b := make([]byte, blockSize)
	binary.LittleEndian.PutUint32(b, dcxMapLength)
	binary.LittleEndian.PutUint32(b[4:], dcxMapLength)
	binary.LittleEndian.PutUint32(b[4+dcxMapSanityOff:], dcxMapSanity)
	b[4+dcxMapLength], b[4+dcxMapLength+1] = 0x02, 0xFF

	return b
}

// ModuleHeaderError is LIBRARY/LIST's failure on a key whose module header
// can't be read (round 3's W4, whose key Z was entered with a bad RFA):
// the listing's lines before it are in the listing List returns.
type ModuleHeaderError struct {
	Name string
}

func (e ModuleHeaderError) Error() string { return "lbr: module header error for " + e.Name }

// ErrNoIndex is LIBRARY/LIST's failure on a library with no indexes (round
// 3's W15, made with every create option 0 but the type).
var ErrNoIndex = errors.New("lbr: the library has no indexes") // LBR$_ILLIDXNUM
