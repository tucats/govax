package lbr

import (
	"encoding/binary"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// This file writes libraries: a Builder holds a library's modules in
// memory, and Bytes lays the whole file out as VMS 7.3's librarian would
// have written it by creating the library and inserting the modules in
// order (lbr/lis/openclose.lis, getput.lis, index.lis, subs.lis):
//
//   - Block 1 is the header. LBR$OPEN's prealloc_index then reserves index
//     blocks 2 on, enough for /CREATE's default entries (128 modules, plus
//     512 globals in an object library), chained as a free list through
//     each block's first longword. The index blocks come from that list, a
//     B-tree of full blocks built bottom-up, with each block's parent VBN
//     set, and the rest of the list stays free for later insertions.
//   - The data blocks follow, one chain holding every module in turn, as
//     write_record fills it: each record's length word and bytes,
//     word-aligned, running on into a new block when one fills. A data
//     block's DATA$B_RECS counts the records with any part in it.
//
// It writes the V3 format without DCX data reduction, and no update
// history records.

// Entry is one module of a library being built.
type Entry struct {
	// Name is the module's key in index 1.
	Name string
	// Records are the module's own records: a macro's lines, or an object
	// module's object records.
	Records [][]byte
	// Symbols are the module's keys in index 2: for an object module, the
	// global symbols it defines.
	Symbols []string
	// Flags is MHD$B_LBRFLAG, and Inserted the insertion time, a VMS time.
	Flags    byte
	Inserted uint64
	// UserData is the library type's own module header data (see
	// ModuleHeader), cut or zero-filled to the library's size for it.
	UserData []byte
}

// Builder is a library being created or changed, held in memory: its
// modules, and the header fields the librarian keeps.
type Builder struct {
	Type Type
	// KeySize is the longest key the library allows (/CREATE=KEYSIZE).
	KeySize int
	// History is how many update history records the library may keep
	// (LHD$W_MAXLUHREC). govax writes none.
	History int
	// Librarian names the librarian that created the library.
	Librarian string
	// Created and Updated are VMS times. The librarian sets Updated to
	// the time the last module was inserted.
	Created uint64
	Updated uint64

	flags    []uint16 // each index's IDD$W_FLAGS
	userSize int      // LHD$B_MHDUSZ
	entries  map[string]*Entry
	order    []string          // the modules in the order their data is written
	symbols  map[string]string // index 2's keys, and the modules they name
}

// Library defaults by type (librar/lis/database.lis: LIB$AL_ASCBINF,
// LIB$AL_HDRLEN, LIB$AL_IDXOPT, and LIB$AL_NUMIDX; lib.sdl's DEFMODS and
// DEFGBLS). The index options are CRE$M_NOCASECMP, CRE$M_NOCASENTR, and
// CRE$M_UPCASNTRY, which the index flags hold three bits higher.
var typeDefaults = map[Type]struct {
	indexes, keySize, userSize int
	caseFlags                  uint16
	entries                    int // CRE$L_ENTALL
}{
	TypeObject:    {2, 31, 33, IndexNoCaseCmp | IndexNoCaseEnt, 128 + 512},
	TypeMacro:     {1, 31, 0, 0, 128},
	TypeHelp:      {1, 15, 0, IndexNoCaseEnt | IndexUpcase, 128},
	TypeText:      {1, 39, 0, 0, 128},
	TypeShareable: {2, 39, 33, IndexNoCaseCmp | IndexNoCaseEnt, 128 + 512},
}

const (
	// LibrarianName is what govax puts in LHD$T_LBRVER.
	LibrarianName = "govax Librarian"

	defaultHistory = 20 // LIB$C_DEFLUHREC
	indexSpace     = blockSize - indexEntries
	rfaLength      = 6 // RFA$C_LENGTH
	keyOverhead    = 7 // an index entry's RFA and key length byte
)

// Create starts a new, empty library of type t, with LIBRARY/CREATE's
// defaults for the type.
func Create(t Type) (*Builder, error) {
	d, ok := typeDefaults[t]
	if !ok {
		return nil, fmt.Errorf("lbr: can't create a %s library", t)
	}

	b := &Builder{
		Type:      t,
		KeySize:   d.keySize,
		History:   defaultHistory,
		Librarian: LibrarianName,
		userSize:  d.userSize,
		entries:   map[string]*Entry{},
		symbols:   map[string]string{},
	}

	for range d.indexes {
		b.flags = append(b.flags, IndexASCII|IndexVarLength|d.caseFlags)
	}

	return b, nil
}

// Edit starts a change to an existing library: a Builder holding its
// modules, in the order of their data, and its header's settings. A
// data-reduced library's records are expanded, and Bytes writes them
// unreduced.
func Edit(l *Library) (*Builder, error) {
	if len(l.Indexes) < 1 || len(l.Indexes) > 2 {
		return nil, fmt.Errorf("lbr: can't change a library with %d indexes", len(l.Indexes))
	}

	b := &Builder{
		Type:      l.Type,
		KeySize:   int(l.Indexes[0].KeyLen) - 1,
		History:   int(l.History),
		Librarian: l.Librarian,
		Created:   l.Created,
		Updated:   l.Updated,
		userSize:  l.mhdUserSize,
		entries:   map[string]*Entry{},
		symbols:   map[string]string{},
	}

	for _, x := range l.Indexes {
		b.flags = append(b.flags, x.Flags)
	}

	keys := slices.Clone(l.Indexes[0].Keys)
	sort.SliceStable(keys, func(i, j int) bool { return rfaLess(keys[i].RFA, keys[j].RFA) })

	for _, k := range keys {
		m, err := l.Module(k.RFA)
		if err != nil {
			return nil, err
		}

		e := &Entry{
			Name:     k.Name,
			Records:  m.Records,
			Flags:    m.Header.Flags,
			Inserted: m.Header.Inserted,
			UserData: m.Header.UserData,
		}
		b.entries[e.Name] = e
		b.order = append(b.order, e.Name)
	}

	if len(l.Indexes) > 1 {
		for _, k := range l.Indexes[1].Keys {
			name, ok := l.ModuleName(k.RFA)
			if !ok {
				return nil, fmt.Errorf("lbr: index 2's key %s names no module", k.Name)
			}

			e := b.entries[name]
			e.Symbols = append(e.Symbols, k.Name)
			b.symbols[k.Name] = name
		}
	}

	return b, nil
}

func rfaLess(a, b RFA) bool {
	return a.VBN < b.VBN || a.VBN == b.VBN && a.Offset < b.Offset
}

// Module returns the module named name.
func (b *Builder) Module(name string) (*Entry, bool) {
	e, ok := b.entries[name]

	return e, ok
}

// Names returns the modules' names in index order.
func (b *Builder) Names() []string {
	names := slices.Clone(b.order)
	b.sortKeys(0, names)

	return names
}

// SymbolModule returns the module that defines symbol, a key in index 2.
func (b *Builder) SymbolModule(symbol string) (string, bool) {
	name, ok := b.symbols[symbol]

	return name, ok
}

// Insert adds a module. A module of the same name, or a symbol another
// module already defines, is an error.
func (b *Builder) Insert(e *Entry) error {
	if _, ok := b.entries[e.Name]; ok {
		return fmt.Errorf("lbr: module %s is already in the library", e.Name)
	}

	return b.add(e)
}

// Replace adds a module, taking the place of any module of the same name
// (whose symbols go with it), and reports whether it replaced one.
func (b *Builder) Replace(e *Entry) (bool, error) {
	old, ok := b.entries[e.Name]
	if !ok {
		return false, b.add(e)
	}

	pos := slices.Index(b.order, e.Name)
	b.remove(old)

	if err := b.add(e); err != nil {
		b.entries[old.Name] = old
		b.order = slices.Insert(b.order, pos, old.Name)

		for _, s := range old.Symbols {
			b.symbols[s] = old.Name
		}

		return false, err
	}

	return true, nil
}

// Delete removes a module and its symbols.
func (b *Builder) Delete(name string) error {
	e, ok := b.entries[name]
	if !ok {
		return fmt.Errorf("lbr: no module %s in the library", name)
	}

	b.remove(e)

	return nil
}

func (b *Builder) remove(e *Entry) {
	delete(b.entries, e.Name)
	b.order = slices.DeleteFunc(b.order, func(n string) bool { return n == e.Name })

	for _, s := range e.Symbols {
		delete(b.symbols, s)
	}
}

// add checks e and adds it, its data last.
func (b *Builder) add(e *Entry) error {
	if err := b.checkKey("module name", e.Name); err != nil {
		return err
	}

	if len(e.Symbols) > 0 && len(b.flags) < 2 {
		return fmt.Errorf("lbr: a %s library has no symbol index", b.Type)
	}

	seen := map[string]bool{}

	for _, s := range e.Symbols {
		if err := b.checkKey("symbol", s); err != nil {
			return err
		}

		if seen[s] {
			return fmt.Errorf("lbr: module %s lists symbol %s twice", e.Name, s)
		}

		seen[s] = true

		if other, ok := b.symbols[s]; ok {
			return fmt.Errorf("lbr: symbol %s of module %s is already defined by module %s", s, e.Name, other)
		}
	}

	for i, r := range e.Records {
		if len(r) > maxRecSize {
			return fmt.Errorf("lbr: module %s, record %d: %d bytes is too long (the limit is %d)", e.Name, i+1, len(r), maxRecSize)
		}
	}

	b.entries[e.Name] = e
	b.order = append(b.order, e.Name)

	for _, s := range e.Symbols {
		b.symbols[s] = e.Name
	}

	return nil
}

func (b *Builder) checkKey(what, key string) error {
	switch {
	case key == "":
		return fmt.Errorf("lbr: a %s is empty", what)
	case len(key) > b.KeySize:
		return fmt.Errorf("lbr: %s %s is longer than the library's %d-character keys", what, key, b.KeySize)
	}

	return nil
}

// sortKeys sorts keys into index i's order: the librarian's CH$COMPARE,
// on the keys as they are, or upper-cased if the index says so.
func (b *Builder) sortKeys(i int, keys []string) {
	if b.flags[i]&IndexUpcase != 0 {
		sort.SliceStable(keys, func(p, q int) bool { return strings.ToUpper(keys[p]) < strings.ToUpper(keys[q]) })
	} else {
		sort.Strings(keys)
	}
}

// indexTree is one index's B-tree: its blocks, level by level from the
// leaves up.
type indexTree struct {
	levels [][]*indexBlock
}

type indexBlock struct {
	keys   []Key
	vbn    uint32
	parent *indexBlock
}

// pack groups keys into full index blocks.
func pack(keys []Key) []*indexBlock {
	var (
		out  []*indexBlock
		used int
	)

	for _, k := range keys {
		n := keyOverhead + len(k.Name)
		if len(out) == 0 || used+n > indexSpace {
			out = append(out, &indexBlock{})
			used = 0
		}

		last := out[len(out)-1]
		last.keys = append(last.keys, k)
		used += n
	}

	return out
}

// buildTree builds the B-tree over keys, sorted: each upper-level entry
// names its child block and the child's highest key. It numbers the
// blocks from *next on.
func buildTree(keys []Key, next *uint32) *indexTree {
	t := &indexTree{}
	if len(keys) == 0 {
		return t
	}

	level := pack(keys)

	for {
		for _, blk := range level {
			blk.vbn = *next
			*next++
		}

		t.levels = append(t.levels, level)
		if len(level) == 1 {
			return t
		}

		up := make([]Key, len(level))
		for i, blk := range level {
			up[i] = Key{Name: blk.keys[len(blk.keys)-1].Name, RFA: RFA{VBN: blk.vbn, Offset: rfaIndex}}
		}

		upper := pack(up)

		n := 0
		for _, u := range upper {
			for range u.keys {
				level[n].parent = u
				n++
			}
		}

		level = upper
	}
}

func (t *indexTree) root() uint32 {
	if len(t.levels) == 0 {
		return 0
	}

	return t.levels[len(t.levels)-1][0].vbn
}

// dataWriter writes records into a chain of data blocks, as write_record
// does.
type dataWriter struct {
	first  uint32
	blocks [][]byte
	off    int
}

func (w *dataWriter) vbn() uint32 { return w.first + uint32(len(w.blocks)) - 1 }

// newBlock starts the next data block, linking the last one to it, and
// sets its record count to recs.
func (w *dataWriter) newBlock(recs byte) {
	if n := len(w.blocks); n > 0 {
		binary.LittleEndian.PutUint32(w.blocks[n-1][dataLink:], w.first+uint32(n))
	}

	blk := make([]byte, blockSize)
	blk[0] = recs
	w.blocks = append(w.blocks, blk)
	w.off = dataData
}

// advance moves past n bytes, word-aligned (incr_rfa), starting a new
// block when this one is full: the record goes on into it (recs 1), or,
// if done is set, it's empty so far.
func (w *dataWriter) advance(n int, done bool) {
	w.off = (w.off + n + 1) &^ 1
	if w.off < blockSize {
		return
	}

	if done {
		w.newBlock(0)
	} else {
		w.newBlock(1)
	}
}

// write writes rec and returns where it starts.
func (w *dataWriter) write(rec []byte) RFA {
	if len(w.blocks) == 0 {
		w.newBlock(0)
	}

	blk := w.blocks[len(w.blocks)-1]
	blk[0]++

	at := RFA{VBN: w.vbn(), Offset: uint16(w.off)}
	binary.LittleEndian.PutUint16(blk[w.off:], uint16(len(rec)))

	// A record whose length word ends a block has begun in that block and
	// goes on in the next, even if it's empty.
	w.advance(2, false)

	for left := rec; ; {
		blk = w.blocks[len(w.blocks)-1]
		n := copy(blk[w.off:], left)
		left = left[n:]

		w.advance(n, len(left) == 0)

		if len(left) == 0 {
			return at
		}
	}
}

// Bytes lays out and returns the library file.
func (b *Builder) Bytes() []byte {
	// The index keys, sorted, with their modules' names for now.
	keys := make([][]Key, len(b.flags))

	names := b.Names()
	for _, n := range names {
		keys[0] = append(keys[0], Key{Name: n})
	}

	if len(b.flags) > 1 {
		syms := make([]string, 0, len(b.symbols))
		for s := range b.symbols {
			syms = append(syms, s)
		}

		b.sortKeys(1, syms)

		for _, s := range syms {
			keys[1] = append(keys[1], Key{Name: s})
		}
	}

	// How many index blocks the trees take, and so how many to
	// preallocate: LIBRARY/CREATE's default (prealloc_index's, from the
	// entries to allow for and how many keys of the longest length fit in
	// a block), or more if needed.
	next := uint32(2)
	for _, k := range keys {
		buildTree(k, &next)
	}

	used := int(next - 2)

	d := typeDefaults[b.Type]
	entall := d.entries

	if entall == 0 {
		entall = 300 // LBR$C_DEFENTALL
	}

	prealloc := max(entall/(indexSpace/(b.KeySize+rfaLength)), 1, used)

	// The data, after the preallocated index blocks.
	w := &dataWriter{first: uint32(prealloc) + 2}
	rfas := map[string]RFA{}

	for _, name := range b.order {
		e := b.entries[name]

		mhd := make([]byte, mhdLen+b.userSize)
		mhd[0] = e.Flags
		mhd[1] = mhdID
		binary.LittleEndian.PutUint32(mhd[4:], uint32(1+len(e.Symbols)))
		binary.LittleEndian.PutUint64(mhd[8:], e.Inserted)
		copy(mhd[mhdLen:], e.UserData)

		rfas[name] = w.write(mhd)

		for _, r := range e.Records {
			w.write(r)
		}

		w.write(eot)
	}

	// The index trees, now that the modules' RFAs are known.
	for i := range keys {
		for j, k := range keys[i] {
			if i == 0 {
				keys[i][j].RFA = rfas[k.Name]
			} else {
				keys[i][j].RFA = rfas[b.symbols[k.Name]]
			}
		}
	}

	next = 2
	trees := make([]*indexTree, len(keys))

	for i, k := range keys {
		trees[i] = buildTree(k, &next)
	}

	nblocks := 1 + prealloc + len(w.blocks)
	out := make([]byte, nblocks*blockSize)

	overhead := 0

	for _, t := range trees {
		for li, level := range t.levels {
			for _, blk := range level {
				p := out[(blk.vbn-1)*blockSize:]
				if blk.parent != nil {
					binary.LittleEndian.PutUint32(p[indexParent:], blk.parent.vbn)
				}

				pos := indexEntries
				for _, k := range blk.keys {
					binary.LittleEndian.PutUint32(p[pos:], k.RFA.VBN)
					binary.LittleEndian.PutUint16(p[pos+4:], k.RFA.Offset)
					p[pos+6] = byte(len(k.Name))
					pos += keyOverhead + copy(p[pos+keyOverhead:], k.Name)
				}

				binary.LittleEndian.PutUint16(p, uint16(pos-indexEntries))

				if li > 0 {
					overhead += len(blk.keys)
				}
			}
		}
	}

	// The unused preallocated index blocks, chained.
	for v := next; v <= uint32(prealloc)+1; v++ {
		if v < uint32(prealloc)+1 {
			binary.LittleEndian.PutUint32(out[(v-1)*blockSize:], v+1)
		}
	}

	for i, blk := range w.blocks {
		copy(out[(int(w.first)-1+i)*blockSize:], blk)
	}

	// The header.
	h := out[:blockSize]
	h[lhdType] = byte(b.Type)
	h[lhdNIndex] = byte(len(b.flags))
	binary.LittleEndian.PutUint32(h[lhdSanity:], saneID3)
	binary.LittleEndian.PutUint16(h[lhdMajorID:], 3)
	h[lhdLbrVer] = byte(copy(h[lhdLbrVer+1:lhdLbrVer+32], b.Librarian))
	binary.LittleEndian.PutUint64(h[lhdCreDat:], b.Created)
	binary.LittleEndian.PutUint64(h[lhdUpdTim:], b.Updated)
	h[lhdMhdUsz] = byte(b.userSize)

	nextRFA := RFA{VBN: w.first}
	if len(w.blocks) > 0 {
		nextRFA = RFA{VBN: w.vbn(), Offset: uint16(w.off)}
	}

	binary.LittleEndian.PutUint32(h[lhdNextRFA:], nextRFA.VBN)
	binary.LittleEndian.PutUint16(h[lhdNextRFA+4:], nextRFA.Offset)
	binary.LittleEndian.PutUint32(h[lhdNextVBN:], w.first+uint32(len(w.blocks)))

	if free := prealloc - used; free > 0 {
		binary.LittleEndian.PutUint32(h[lhdFreIdxBlk:], uint32(free))
		binary.LittleEndian.PutUint32(h[lhdFreeIdx:], next)
	}

	binary.LittleEndian.PutUint32(h[lhdHiPreAl:], uint32(prealloc)+1)

	if used > 0 {
		binary.LittleEndian.PutUint32(h[lhdHiPrUsd:], next-1)
	}

	total := 0
	for _, k := range keys {
		total += len(k)
	}

	binary.LittleEndian.PutUint32(h[lhdIdxBlks:], uint32(used))
	binary.LittleEndian.PutUint32(h[lhdIdxCnt:], uint32(total))
	binary.LittleEndian.PutUint32(h[lhdModCnt:], uint32(len(keys[0])))
	binary.LittleEndian.PutUint32(h[lhdModHdrs:], uint32(len(b.order)))
	binary.LittleEndian.PutUint32(h[lhdIdxOvh:], uint32(overhead))
	binary.LittleEndian.PutUint16(h[lhdMaxLUHRec:], uint16(b.History))

	for i, f := range b.flags {
		d := h[lhdIdxDesc+i*iddLength:]
		binary.LittleEndian.PutUint16(d, f)
		binary.LittleEndian.PutUint16(d[2:], uint16(b.KeySize+1))
		binary.LittleEndian.PutUint32(d[4:], trees[i].root())
	}

	return out
}
