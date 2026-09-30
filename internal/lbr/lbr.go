// Package lbr reads VMS librarian files: object libraries (.OLB),
// shareable image symbol table libraries (IMAGELIB.OLB), macro libraries
// (.MLB), help libraries (.HLB), and text libraries (.TLB) all share one
// format (docs/PHASE-30.md, subtask 3). It is a leaf package: it reads a
// library from its bytes, and callers find and read the file.
//
// The format comes from VMS 7.3's librarian sources
// (vmssrc_archive/v73/lbr/lis/lbr.sdl, getput.lis, openclose.lis, subs.lis,
// and vest_lbr/lis/lbrusr.sdl):
//
//   - Block 1 is the library header ($LHDDEF), which ends with one index
//     descriptor ($IDDDEF) for each of the library's indexes. An object
//     library has two: module names, and the global symbols the modules
//     define.
//   - Each index is a B-tree of 512-byte index blocks ($INDEXDEF). An entry
//     is an RFA (a VBN and a byte offset) and a key. In an upper-level
//     block, the RFA's offset is ^XFFFF and its VBN is the child block,
//     whose highest key the entry's key is. In a leaf block, the RFA is
//     where the key's module starts.
//   - Modules are stored as records in a chain of data blocks ($DATADEF):
//     a record is a length word and its bytes, word-aligned, and may run
//     on into the next block of the chain. A module starts with the
//     librarian's module header ($MHDDEF), then its own records, and ends
//     with a three-byte end-of-text record.
//   - A data-reduced library (LHD$L_DCXMAPVBN nonzero) keeps every record
//     after the module header compressed with the DCX facility (dcx.go).
//
// The old (VMS V1) library format isn't supported.
package lbr

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// Type is a library's type (LBR$C_TYP_xxx).
type Type byte

// Library types.
const (
	TypeUnknown   Type = 0 // LBR$C_TYP_UNK
	TypeObject    Type = 1 // LBR$C_TYP_OBJ: object modules
	TypeMacro     Type = 2 // LBR$C_TYP_MLB
	TypeHelp      Type = 3 // LBR$C_TYP_HLP
	TypeText      Type = 4 // LBR$C_TYP_TXT
	TypeShareable Type = 5 // LBR$C_TYP_SHSTB: shareable image symbol tables
	TypeNCS       Type = 6 // LBR$C_TYP_NCS
)

// String names the type as LIBRARIAN/LIST does.
func (t Type) String() string {
	switch t {
	case TypeObject:
		return "OBJECT"
	case TypeMacro:
		return "MACRO"
	case TypeHelp:
		return "HELP"
	case TypeText:
		return "TEXT"
	case TypeShareable:
		return "SHAREABLE IMAGE"
	case TypeNCS:
		return "NCS"
	}

	return fmt.Sprintf("type %d", byte(t))
}

// Header layout ($LHDDEF).
const (
	blockSize = 512

	lhdType      = 0x00
	lhdNIndex    = 0x01
	lhdSanity    = 0x04
	lhdMajorID   = 0x08
	lhdMinorID   = 0x0A
	lhdLbrVer    = 0x0C // counted, in 32 bytes
	lhdCreDat    = 0x2C
	lhdUpdTim    = 0x34
	lhdMhdUsz    = 0x3C
	lhdModCnt    = 0x6E
	lhdDcxMapVBN = 0x8C
	lhdIdxDesc   = 0xC4

	iddLength = 8 // $IDDDEF: flags word, key length word, VBN longword

	// Sanity longwords: a V3 library, and a V3 library whose data is DCX
	// data-reduced. LHD$C_SANEID, the V2 format's, isn't supported.
	saneID3 = 233579905
	saneIDC = 319232342

	indexEntries = 12     // INDEX$C_ENTRIES: after the used word, parent VBN, and 6 reserved bytes
	rfaIndex     = 0xFFFF // RFA$C_INDEX: an upper-level entry's offset

	dataLink = 2 // DATA$L_LINK
	dataData = 6 // DATA$C_DATA: the first record's offset in a data block

	maxRecSize    = 2048           // LBR$C_MAXRECSIZ
	maxDCXRecSize = 2 * maxRecSize // LBR_DCX$C_MAXRECSIZ

	mhdID  = 173 // MHD$C_MHDID
	mhdLen = 16  // MHD$C_MHDLEN: the fixed part of a module header
)

// Index flags (IDD$V_xxx).
const (
	IndexASCII     = 0x01 // keys are ASCII
	IndexLocked    = 0x02
	IndexVarLength = 0x04 // entries are only as long as their keys
	IndexNoCaseCmp = 0x08
	IndexNoCaseEnt = 0x10
	IndexUpcase    = 0x20
)

// eot is the end-of-text record that ends every module.
var eot = []byte{0x77, 0x00, 0x77}

// RFA is a record's address in a library: a virtual block number and a
// byte offset in the block.
type RFA struct {
	VBN    uint32
	Offset uint16
}

// Key is one index entry: a key, and where its module starts.
type Key struct {
	Name string
	RFA  RFA
}

// Index is one of a library's indexes.
type Index struct {
	Flags  uint16
	KeyLen uint16 // the longest key
	// Keys are the index's entries, in the index's (sorted) order.
	Keys  []Key
	byKey map[string]RFA
	byRFA map[RFA]string // built when first needed (ModuleName)
}

// Lookup returns where key's module starts.
func (x *Index) Lookup(key string) (RFA, bool) {
	r, ok := x.byKey[key]

	return r, ok
}

// Library is a librarian file, read into memory.
type Library struct {
	Type    Type
	MajorID uint16
	MinorID uint16
	// Librarian is the version of the librarian that created the library
	// ("Librarian T09-20").
	Librarian string
	// Created and Updated are VMS times (100 ns units since 17-Nov-1858).
	Created uint64
	Updated uint64
	// Modules is how many modules the library's first index holds.
	Modules uint32
	// Indexes are the library's indexes. Index 1 (Indexes[0]) names the
	// modules; an object library's index 2 holds its global symbols.
	Indexes []*Index

	data        []byte
	mhdUserSize int
	dcx         *dcxMap // nil unless the library is data-reduced
}

// Module is one library module.
type Module struct {
	Header ModuleHeader
	// Records are the module's own records, expanded if the library is
	// data-reduced: for an object library, its object records.
	Records [][]byte
}

// ModuleHeader is the librarian's header on a module ($MHDDEF).
type ModuleHeader struct {
	Flags    byte   // MHD$B_LBRFLAG
	RefCount uint32 // how many index entries refer to the module
	Inserted uint64 // a VMS time
	// UserData is the library type's own header data (LHD$B_MHDUSZ bytes).
	// An object library's holds the module's status and ident.
	UserData []byte
}

// SelectiveSearch reports whether an object library module is searched
// selectively (MHD$V_SELSRC, from LIBRARY/INSERT/SELECTIVE_SEARCH): the
// linker takes from it only the definitions of symbols already referred
// to.
func (h ModuleHeader) SelectiveSearch() bool {
	return len(h.UserData) > 0 && h.UserData[0]&mhdSelectiveSearch != 0
}

// mhdSelectiveSearch is MHD$M_SELSRC, in an object library module
// header's MHD$B_OBJSTAT (its user data's first byte).
const mhdSelectiveSearch = 1

// ObjectIdent is an object or shareable image library module's ident
// (MHD$B_OBJIDLNG and MHD$T_OBJIDENT), from its header's user data.
func (h ModuleHeader) ObjectIdent() string {
	if len(h.UserData) < 2 {
		return ""
	}

	n := int(h.UserData[1])

	return string(h.UserData[2:min(2+n, len(h.UserData))])
}

// Open reads a library from its bytes: the file's blocks, in order.
func Open(data []byte) (*Library, error) {
	if len(data) < blockSize {
		return nil, fmt.Errorf("lbr: file is too short to be a library")
	}

	h := data[:blockSize]

	switch sanity := le32(h, lhdSanity); sanity {
	case saneID3, saneIDC:
	default:
		return nil, fmt.Errorf("lbr: not a library, or an unsupported library format (sanity %08X)", sanity)
	}

	l := &Library{
		Type:        Type(h[lhdType]),
		MajorID:     le16(h, lhdMajorID),
		MinorID:     le16(h, lhdMinorID),
		Librarian:   counted(h[lhdLbrVer : lhdLbrVer+32]),
		Created:     binary.LittleEndian.Uint64(h[lhdCreDat:]),
		Updated:     binary.LittleEndian.Uint64(h[lhdUpdTim:]),
		Modules:     le32(h, lhdModCnt),
		data:        data,
		mhdUserSize: int(h[lhdMhdUsz]),
	}

	n := int(h[lhdNIndex])
	if lhdIdxDesc+n*iddLength > blockSize {
		return nil, fmt.Errorf("lbr: header claims %d indexes", n)
	}

	for i := range n {
		d := h[lhdIdxDesc+i*iddLength:]

		x, err := l.readIndex(le16(d, 0), le16(d, 2), le32(d, 4))
		if err != nil {
			return nil, fmt.Errorf("lbr: index %d: %w", i+1, err)
		}

		l.Indexes = append(l.Indexes, x)
	}

	if vbn := le32(h, lhdDcxMapVBN); vbn != 0 {
		m, err := l.readDCXMap(vbn)
		if err != nil {
			return nil, err
		}

		l.dcx = m
	}

	return l, nil
}

// block returns block vbn.
func (l *Library) block(vbn uint32) ([]byte, error) {
	if vbn == 0 || int64(vbn)*blockSize > int64(len(l.data)) {
		return nil, fmt.Errorf("block %d is outside the file", vbn)
	}

	return l.data[(vbn-1)*blockSize : vbn*blockSize], nil
}

// readIndex reads an index's B-tree, from its root block.
func (l *Library) readIndex(flags, keyLen uint16, root uint32) (*Index, error) {
	if flags&IndexASCII == 0 {
		return nil, fmt.Errorf("binary keys aren't supported")
	}

	x := &Index{Flags: flags, KeyLen: keyLen, byKey: map[string]RFA{}}
	seen := map[uint32]bool{}

	var walk func(vbn uint32) error

	walk = func(vbn uint32) error {
		if seen[vbn] {
			return fmt.Errorf("index block %d is reached twice", vbn)
		}

		seen[vbn] = true

		b, err := l.block(vbn)
		if err != nil {
			return err
		}

		end := indexEntries + int(le16(b, 0))
		if end > blockSize {
			return fmt.Errorf("index block %d claims %d bytes in use", vbn, end-indexEntries)
		}

		for pos := indexEntries; pos < end; {
			if pos+7 > end {
				return fmt.Errorf("index block %d: entry at %d is cut short", vbn, pos)
			}

			r := RFA{VBN: le32(b, pos), Offset: le16(b, pos+4)}
			n := int(b[pos+6])

			size := 7 + n
			if flags&IndexVarLength == 0 {
				size = 6 + int(keyLen)
			}

			if pos+7+n > end || size < 7+n {
				return fmt.Errorf("index block %d: key at %d is cut short", vbn, pos)
			}

			if r.Offset == rfaIndex {
				if err := walk(r.VBN); err != nil {
					return err
				}
			} else {
				k := Key{Name: string(b[pos+7 : pos+7+n]), RFA: r}
				x.Keys = append(x.Keys, k)
				x.byKey[k.Name] = r
			}

			pos += size
		}

		return nil
	}

	if err := walk(root); err != nil {
		return nil, err
	}

	if !sort.SliceIsSorted(x.Keys, func(i, j int) bool { return x.Keys[i].Name < x.Keys[j].Name }) && flags&(IndexNoCaseCmp|IndexUpcase) == 0 {
		return nil, fmt.Errorf("keys are out of order")
	}

	return x, nil
}

// Lookup finds a module by name, through index 1.
func (l *Library) Lookup(name string) (RFA, bool) {
	if len(l.Indexes) == 0 {
		return RFA{}, false
	}

	return l.Indexes[0].Lookup(name)
}

// ModuleName returns the name of the module that starts at rfa: the key
// index 1 has for it. Another index's entry for a symbol a module defines
// has the module's RFA.
func (l *Library) ModuleName(rfa RFA) (string, bool) {
	if len(l.Indexes) == 0 {
		return "", false
	}

	x := l.Indexes[0]
	if x.byRFA == nil {
		x.byRFA = make(map[RFA]string, len(x.Keys))
		for _, k := range x.Keys {
			x.byRFA[k.RFA] = k.Name
		}
	}

	name, ok := x.byRFA[rfa]

	return name, ok
}

// Module reads the module that starts at rfa.
func (l *Library) Module(rfa RFA) (*Module, error) {
	b, next, err := l.readRecord(rfa)
	if err != nil {
		return nil, fmt.Errorf("lbr: module at %d.%d: %w", rfa.VBN, rfa.Offset, err)
	}

	if len(b) != mhdLen+l.mhdUserSize || b[1] != mhdID {
		return nil, fmt.Errorf("lbr: no module header at %d.%d", rfa.VBN, rfa.Offset)
	}

	m := &Module{Header: ModuleHeader{
		Flags:    b[0],
		RefCount: le32(b, 4),
		Inserted: binary.LittleEndian.Uint64(b[8:]),
		UserData: append([]byte(nil), b[mhdLen:]...),
	}}

	for {
		rec, after, err := l.readRecord(next)
		if err != nil {
			return nil, fmt.Errorf("lbr: module at %d.%d, record %d: %w", rfa.VBN, rfa.Offset, len(m.Records)+1, err)
		}

		if string(rec) == string(eot) {
			return m, nil
		}

		if l.dcx != nil {
			if rec, err = l.dcx.expand(rec); err != nil {
				return nil, fmt.Errorf("lbr: module at %d.%d, record %d: %w", rfa.VBN, rfa.Offset, len(m.Records)+1, err)
			}
		} else {
			rec = append([]byte(nil), rec...)
		}

		m.Records = append(m.Records, rec)
		next = after
	}
}

// readRecord reads the record at rfa, and returns it and where the next
// record starts (getput.lis, read_record). A record that runs on past its
// block continues at the next block of the chain, after its link.
func (l *Library) readRecord(rfa RFA) ([]byte, RFA, error) {
	b, err := l.block(rfa.VBN)
	if err != nil {
		return nil, rfa, err
	}

	if rfa.Offset == 0 {
		rfa.Offset = dataData
	}

	if int(rfa.Offset) > blockSize-2 {
		return nil, rfa, fmt.Errorf("bad record offset %d", rfa.Offset)
	}

	count := int(le16(b, int(rfa.Offset)))

	limit := maxRecSize
	if l.dcx != nil {
		limit = maxDCXRecSize
	}

	if count > limit {
		return nil, rfa, fmt.Errorf("record at %d.%d claims %d bytes", rfa.VBN, rfa.Offset, count)
	}

	// advance moves past n bytes, word-aligned (subs.lis, incr_rfa), and
	// follows the chain at the end of a block.
	advance := func(n int) error {
		off := (int(rfa.Offset) + n + 1) &^ 1
		if off < blockSize {
			rfa.Offset = uint16(off)

			return nil
		}

		rfa = RFA{VBN: le32(b, dataLink), Offset: dataData}

		b, err = l.block(rfa.VBN)

		return err
	}

	start := int(rfa.Offset) + 2
	if start+count <= blockSize {
		rec := b[start : start+count]

		if err := advance(count + 2); err != nil && !isLast(rec) {
			return nil, rfa, err
		}

		return rec, rfa, nil
	}

	if err := advance(2); err != nil {
		return nil, rfa, err
	}

	rec := make([]byte, 0, count)

	for left := count; left > 0; {
		n := min(left, blockSize-int(rfa.Offset))
		rec = append(rec, b[rfa.Offset:int(rfa.Offset)+n]...)
		left -= n

		if err := advance(n); err != nil && left > 0 {
			return nil, rfa, err
		}
	}

	return rec, rfa, nil
}

// isLast reports whether rec ends a module, so that nothing follows it:
// the chain may end at the block it fills.
func isLast(rec []byte) bool { return string(rec) == string(eot) }

func le16(b []byte, off int) uint16 { return binary.LittleEndian.Uint16(b[off:]) }

func le32(b []byte, off int) uint32 { return binary.LittleEndian.Uint32(b[off:]) }

// counted reads a counted string from a fixed-length field.
func counted(b []byte) string {
	n := min(int(b[0]), len(b)-1)

	return string(b[1 : 1+n])
}
