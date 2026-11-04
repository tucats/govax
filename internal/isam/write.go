package isam

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// The errors a writer can get. Each maps onto an RMS status in internal/rms.
var (
	// ErrReadOnly means the Blocks the file was opened on can't be written.
	ErrReadOnly = errors.New("isam: file isn't open for writing")

	// ErrBadSpec is a Spec that can't make a file. RMS's own checks come
	// first and give each mistake its status (the phase doc's "$CREATE's
	// statuses"), so this is only what would otherwise make nonsense.
	ErrBadSpec = errors.New("isam: bad file specification")

	// ErrDuplicate means a key that forbids duplicates has this value
	// already (RMS$_DUP).
	ErrDuplicate = errors.New("isam: duplicate key")

	// ErrRecordSize is a record too short for the primary key, the wrong
	// size for a fixed format file, or too long for a bucket.
	ErrRecordSize = errors.New("isam: bad record size")
)

// WritableBlocks is a file Create and Put can change. Extend adds at least
// count blocks to the file's end and says where they start and how many it
// added: a volume's cluster size rounds a request up, and the area's
// descriptor records what it got.
type WritableBlocks interface {
	Blocks
	WriteBlock(vbn uint32, block []byte) error
	Extend(count uint32) (first, got uint32, err error)
}

// AreaSpec is what Create needs of one area: an XABALL's fields, or the
// FAB's for a file with no XABALL. Area 0 also holds the prolog.
type AreaSpec struct {
	BucketSize int    // in blocks
	Allocation uint32 // blocks to start with; 0 for none until one is needed
	Extend     int    // the default extend quantity, DEQ
	Alignment  uint8
	Options    uint8 // AREA$B_AOP: XAB$V_CBT, CTG, and the rest
	Volume     int
	Location   uint32
}

// KeySpec is what Create needs of one key. Flags are in the descriptor's
// sense (FlagDuplicates and the rest), so the compression bits are set for
// what is compressed: mapping an XABKEY onto them is internal/rms's.
type KeySpec struct {
	Segments                        []Segment
	Type                            Type
	Flags                           uint8
	Null                            uint8
	IndexArea, Level1Area, DataArea int
	IndexFill, DataFill             int // bytes; 0 is the whole bucket
	Name                            string

	// Named says the key was given a name, if only an empty one, which
	// is written as blanks: VMS 7.1 wrote 32 for CREATE/FDL's NAME ""
	// (testdata/idx88's round 3, C51). A key with neither has zeros.
	Named bool
}

// Spec describes a new file. Keys[0] is the primary key.
type Spec struct {
	Attributes Attributes
	Keys       []KeySpec
	Areas      []AreaSpec
}

// The prolog's layout. Key 0's descriptor shares VBN 1 with the file-level
// fields, and the other keys' follow from VBN 2, keyStride apart. VMS's
// files show up to 3 in a block; 5 is as many as fit, and past that a block
// more is unconfirmed by any file here. The areas come next, in blocks of
// their own.
const (
	keyStride    = 0x66
	keysPerBlock = 5
	prologLevel  = 3
)

// Create makes the prolog of a new file on empty blocks: the descriptors,
// and each area's first allocation. There are no buckets until the first
// record needs them, so a key's descriptor starts with FlagNoIndexYet. The
// blocks are asked for in area order, which is how VMS's file lays out
// (area 0 first, with the prolog at its start: S62).
func Create(blocks WritableBlocks, spec Spec) (*File, error) {
	if err := spec.check(); err != nil {
		return nil, err
	}

	f := &File{
		blocks:  blocks,
		out:     blocks,
		attr:    spec.Attributes,
		Version: prologLevel,
		AreaVBN: uint32(2 + (len(spec.Keys)-2+keysPerBlock)/keysPerBlock),
	}

	for i := range spec.Keys {
		f.Keys = append(f.Keys, spec.key(i))
	}

	prolog := f.AreaVBN + uint32((len(spec.Areas)-1)/(BlockSize/areaDescSize))

	for i := range spec.Areas {
		if err := f.startArea(&spec, i, prolog); err != nil {
			return nil, err
		}
	}

	if err := f.writeProlog(); err != nil {
		return nil, err
	}

	return f, nil
}

// PrologBlocks is how many blocks the prolog of a file with keys keys and
// areas areas takes: VBN 1 (key 0 and the file's fields), the blocks of the
// other keys' descriptors, and the blocks of the area descriptors. RMS
// asks for at least this much in area 0, and reports it as the FAB's
// allocation when a $CREATE fails after the areas are known.
func PrologBlocks(keys, areas int) uint32 {
	areaVBN := uint32(2 + (keys-2+keysPerBlock)/keysPerBlock)

	return areaVBN + uint32((areas-1)/(BlockSize/areaDescSize))
}

// ExtendArea gives area i count more blocks, as RMS's $EXTEND does with a
// XABALL: the file grows, and the blocks become the area's current extent
// if it has none, or else the extent to come (AREA$L_NXT and NXBLK), which
// allocate moves to when the current one is used up. Blocks that continue
// the extent to come lengthen it. It returns the blocks the volume gave.
// That $EXTEND fills the extent to come is govax's reading of the
// descriptor, which no VMS run here shows; an area with an extent to come
// that the new blocks don't continue is ErrBadSpec.
func (f *File) ExtendArea(i int, count uint32) (uint32, error) {
	if f.out == nil {
		return 0, ErrReadOnly
	}

	if i < 0 || i >= len(f.Areas) {
		return 0, ErrBadSpec
	}

	a := &f.Areas[i]
	if a.ExtentSize != 0 && a.NextExtent != 0 && a.NextExtent+a.NextExtentSize != f.fileEnd() {
		return 0, ErrBadSpec
	}

	first, got, err := f.out.Extend(count)
	if err != nil {
		return 0, err
	}

	a.TotalAllocation += got

	switch {
	case a.ExtentSize == 0:
		a.ExtentVBN, a.ExtentSize, a.Used, a.NextVBN = first, got, 0, first
	case a.NextExtent == 0:
		a.NextExtent, a.NextExtentSize = first, got
	default:
		a.NextExtentSize += got
	}

	return got, f.updateArea(i)
}

// SizedBlocks is a file that can say how many blocks it has: where its next
// extension starts. ExtendArea asks for it; without it, the end is taken to
// be the last block any area has.
type SizedBlocks interface {
	Size() uint32
}

// fileEnd is the VBN past the file's last block.
func (f *File) fileEnd() uint32 {
	if s, ok := f.out.(SizedBlocks); ok {
		return s.Size() + 1
	}

	end := uint32(1)

	for _, a := range f.Areas {
		end = max(end, a.ExtentVBN+a.ExtentSize, a.NextExtent+a.NextExtentSize)
	}

	return end
}

// check refuses a Spec the code below couldn't lay out.
func (s *Spec) check() error {
	if len(s.Keys) == 0 || len(s.Keys) > 255 || len(s.Areas) == 0 || len(s.Areas) > 255 {
		return ErrBadSpec
	}

	for i := range s.Areas {
		if s.Areas[i].BucketSize < 1 || s.Areas[i].BucketSize > 63 {
			return ErrBadSpec
		}
	}

	for i := range s.Keys {
		k := &s.Keys[i]
		if len(k.Segments) == 0 || len(k.Segments) > 8 || len(k.Name) > keyNameSize {
			return ErrBadSpec
		}

		for _, area := range []int{k.IndexArea, k.Level1Area, k.DataArea} {
			if area < 0 || area >= len(s.Areas) {
				return ErrBadSpec
			}
		}
	}

	return nil
}

// key makes a key's descriptor from its spec. A bucket's size is its area's.
// A fill under half a bucket is taken as half, the RMS manual's rule, and
// VMS's (S57: DFL 200 and IFL 300 stored as 512 of 1024).
func (s *Spec) key(i int) *Key {
	in := &s.Keys[i]

	k := &Key{
		Ref:             i,
		IndexArea:       in.IndexArea,
		Level1Area:      in.Level1Area,
		DataArea:        in.DataArea,
		IndexBucketSize: s.Areas[in.IndexArea].BucketSize,
		DataBucketSize:  s.Areas[in.DataArea].BucketSize,
		Flags:           in.Flags | FlagNoIndexYet,
		Type:            in.Type,
		Null:            in.Null,
		Segments:        in.Segments,
		Name:            in.Name,
		Named:           in.Named || in.Name != "",
	}

	for _, seg := range in.Segments {
		k.Size += seg.Size
		k.MinRecord = max(k.MinRecord, seg.Position+seg.Size)
	}

	k.IndexFill = fillSize(in.IndexFill, k.IndexBucketSize)
	k.DataFill = fillSize(in.DataFill, k.DataBucketSize)
	k.DescriptorVBN, k.DescriptorOffset = descriptorPlace(i)

	if i+1 < len(s.Keys) {
		k.NextDescriptorVBN, k.NextDescriptorOffset = descriptorPlace(i + 1)
	}

	return k
}

// fillSize is a key's fill quantity in bytes.
func fillSize(asked, bucketBlocks int) int {
	whole := bucketBlocks * BlockSize

	switch {
	case asked == 0, asked > whole:
		return whole
	case asked < whole/2:
		return whole / 2
	}

	return asked
}

// descriptorPlace says where key i's descriptor goes.
func descriptorPlace(i int) (uint32, int) {
	if i == 0 {
		return 1, 0
	}

	return uint32(2 + (i-1)/keysPerBlock), (i - 1) % keysPerBlock * keyStride
}

// startArea gets an area its first blocks. Area 0 holds the prolog at its
// start, so it needs that much whatever it asked for, and counts it as used.
func (f *File) startArea(spec *Spec, i int, prolog uint32) error {
	in := &spec.Areas[i]

	a := Area{
		ID:         i,
		BucketSize: in.BucketSize,
		Volume:     in.Volume,
		Alignment:  in.Alignment,
		Options:    in.Options,
		ExtendSize: in.Extend,
		Location:   in.Location,
	}

	want := in.Allocation

	var used uint32

	if i == 0 {
		used = prolog
		want = max(want, prolog)
	}

	if want > 0 {
		first, got, err := f.out.Extend(want)
		if err != nil {
			return err
		}

		a.ExtentVBN, a.ExtentSize, a.Used = first, got, used
		a.NextVBN = first + used
		a.TotalAllocation = got
	}

	f.Areas = append(f.Areas, a)

	return nil
}

// writeProlog writes every prolog block of a new file.
func (f *File) writeProlog() error {
	first := make([]byte, BlockSize)
	first[plgAreaVBN] = byte(f.AreaVBN)
	first[plgAreaCount] = byte(len(f.Areas))
	binary.LittleEndian.PutUint16(first[plgVersion:], prologLevel)

	blocks := map[uint32][]byte{1: first}

	for _, k := range f.Keys {
		block := blocks[k.DescriptorVBN]
		if block == nil {
			block = make([]byte, BlockSize)
			blocks[k.DescriptorVBN] = block
		}

		encodeKey(block[k.DescriptorOffset:], k)
	}

	perBlock := BlockSize / areaDescSize

	for i := range f.Areas {
		vbn := f.AreaVBN + uint32(i/perBlock)

		block := blocks[vbn]
		if block == nil {
			block = make([]byte, BlockSize)
			blocks[vbn] = block
		}

		encodeArea(block[(i%perBlock)*areaDescSize:], &f.Areas[i])
	}

	for vbn, block := range blocks {
		if err := f.writePrologBlock(vbn, block); err != nil {
			return err
		}
	}

	return nil
}

// writePrologBlock sets a block's sum and writes it.
func (f *File) writePrologBlock(vbn uint32, block []byte) error {
	var sum uint16

	for i := 0; i < blockSumOffset; i += 2 {
		sum += binary.LittleEndian.Uint16(block[i:])
	}

	binary.LittleEndian.PutUint16(block[blockSumOffset:], sum)

	return f.out.WriteBlock(vbn, block)
}

// encodeKey writes a whole key descriptor, parseKey's opposite. A key's name
// is blank filled, an unnamed key's left zero (S37).
func encodeKey(d []byte, k *Key) {
	binary.LittleEndian.PutUint32(d[keyNextVBN:], k.NextDescriptorVBN)
	binary.LittleEndian.PutUint16(d[keyNextOffset:], uint16(k.NextDescriptorOffset))
	d[keyIndexArea] = byte(k.IndexArea)
	d[keyLevel1Area] = byte(k.Level1Area)
	d[keyDataArea] = byte(k.DataArea)
	d[keyIndexSize] = byte(k.IndexBucketSize)
	d[keyDataSize] = byte(k.DataBucketSize)
	d[keyDataType] = byte(k.Type)
	d[keySegments] = byte(len(k.Segments))
	d[keyNullChar] = k.Null
	d[keyTotalSize] = byte(k.Size)
	d[keyReference] = byte(k.Ref)
	binary.LittleEndian.PutUint16(d[keyMinRecord:], uint16(k.MinRecord))
	binary.LittleEndian.PutUint16(d[keyIndexFill:], uint16(k.IndexFill))
	binary.LittleEndian.PutUint16(d[keyDataFill:], uint16(k.DataFill))

	for i, s := range k.Segments {
		binary.LittleEndian.PutUint16(d[keyPositions+2*i:], uint16(s.Position))
		d[keySizes+i] = byte(s.Size)
		d[keyTypes+i] = byte(s.Type)
	}

	if k.Named || k.Name != "" {
		copy(d[keyName:keyName+keyNameSize], k.Name)

		for i := len(k.Name); i < keyNameSize; i++ {
			d[keyName+i] = ' '
		}
	}

	encodeKeyState(d, k)
}

// encodeKeyState writes the fields that change when a key's index is made.
// Until then the root field holds the areas the index will use, by round
// 1 and RIGHTSLIST.DAT's unmade key.
func encodeKeyState(d []byte, k *Key) {
	d[keyRootLevel] = byte(k.RootLevel)
	d[keyFlags] = k.Flags
	binary.LittleEndian.PutUint32(d[keyFirstData:], k.FirstData)

	if k.Flags&FlagNoIndexYet != 0 {
		d[keyRootVBN], d[keyRootVBN+1], d[keyRootVBN+2], d[keyRootVBN+3] =
			byte(k.DataArea), byte(k.Level1Area), byte(k.IndexArea), 0

		return
	}

	binary.LittleEndian.PutUint32(d[keyRootVBN:], k.RootVBN)
}

// encodeArea writes a whole area descriptor, parseArea's opposite.
func encodeArea(d []byte, a *Area) {
	d[areaID] = byte(a.ID)
	d[areaBucketSize] = byte(a.BucketSize)
	binary.LittleEndian.PutUint16(d[areaVolume:], uint16(a.Volume))
	d[areaAlignment] = a.Alignment
	d[areaOptions] = a.Options
	binary.LittleEndian.PutUint32(d[areaAvailable:], a.Available)
	binary.LittleEndian.PutUint16(d[areaExtend:], uint16(a.ExtendSize))
	binary.LittleEndian.PutUint32(d[areaLocation:], a.Location)
	encodeAreaState(d, a)
}

// encodeAreaState writes the fields that change as an area is used, and
// by $EXTEND (the extent to come).
func encodeAreaState(d []byte, a *Area) {
	binary.LittleEndian.PutUint32(d[areaNextExtent:], a.NextExtent)
	binary.LittleEndian.PutUint32(d[areaNextSize:], a.NextExtentSize)
	binary.LittleEndian.PutUint32(d[areaExtentVBN:], a.ExtentVBN)
	binary.LittleEndian.PutUint32(d[areaExtentSize:], a.ExtentSize)
	binary.LittleEndian.PutUint32(d[areaUsed:], a.Used)
	binary.LittleEndian.PutUint32(d[areaNextVBN:], a.NextVBN)
	binary.LittleEndian.PutUint32(d[areaTotal:], a.TotalAllocation)
}

// updateKey rewrites a key's changed fields in its descriptor's block,
// leaving every other byte as it was: a VMS-made block may have anything in
// its unused bytes.
func (f *File) updateKey(k *Key) error {
	if f.holdProlog {
		return nil
	}

	block, err := f.readPrologBlock(k.DescriptorVBN)
	if err != nil {
		return err
	}

	encodeKeyState(block[k.DescriptorOffset:], k)

	return f.writePrologBlock(k.DescriptorVBN, block)
}

// updateArea rewrites an area's changed fields in its descriptor's block.
func (f *File) updateArea(i int) error {
	if f.holdProlog {
		return nil
	}

	perBlock := BlockSize / areaDescSize
	vbn := f.AreaVBN + uint32(i/perBlock)

	block, err := f.readPrologBlock(vbn)
	if err != nil {
		return err
	}

	encodeAreaState(block[(i%perBlock)*areaDescSize:], &f.Areas[i])

	return f.writePrologBlock(vbn, block)
}

// allocate takes a bucket from an area. What's left of an extent too small
// for the bucket stays unused: VMS goes to a new extent (S1: VBN 6 is never
// used, the bucket after it at 7). A new extent is the area's default extend
// quantity, or the bucket; which of them wins when DEQ is the larger has no
// VMS file to settle it, since a 3-block cluster hides small requests.
func (f *File) allocate(i int) (uint32, error) {
	a := &f.Areas[i]
	size := uint32(a.BucketSize)

	// An extent to come, which $EXTEND gave the area (ExtendArea), is used
	// before the file grows again.
	if (a.NextVBN+size > a.ExtentVBN+a.ExtentSize || a.ExtentSize == 0) && a.NextExtentSize >= size {
		a.ExtentVBN, a.ExtentSize, a.Used, a.NextVBN = a.NextExtent, a.NextExtentSize, 0, a.NextExtent
		a.NextExtent, a.NextExtentSize = 0, 0
	}

	if a.NextVBN+size > a.ExtentVBN+a.ExtentSize || a.ExtentSize == 0 {
		first, got, err := f.out.Extend(max(size, uint32(a.ExtendSize)))
		if err != nil {
			return 0, err
		}

		a.ExtentVBN, a.ExtentSize, a.Used, a.NextVBN = first, got, 0, first
		a.TotalAllocation += got
	}

	vbn := a.NextVBN
	a.NextVBN += size
	a.Used += size

	if err := f.updateArea(i); err != nil {
		return 0, err
	}

	return vbn, nil
}

// newBucket makes an empty bucket: alone at its level, so its chain's next
// is itself. A bucket's first write counts as its check character's first
// for a data bucket, and not for an index bucket, which VMS writes with 0
// (touch is the difference).
func newBucket(vbn uint32, blocks, ref, level int, control uint8) *bucket {
	return &bucket{
		vbn:     vbn,
		data:    make([]byte, blocks*BlockSize),
		indexNo: ref,
		sample:  uint16(vbn),
		free:    bktOverhead,
		nextID:  1,
		next:    vbn,
		level:   level,
		control: control,
	}
}

// touch counts a write in the check character, which wraps.
func (b *bucket) touch() { b.check++ }

// writeBucket puts a bucket's header fields in its bytes and writes it.
func (f *File) writeBucket(b *bucket) error {
	d := b.data
	d[bktCheck] = b.check
	d[len(d)-1] = b.check
	d[bktIndexNo] = byte(b.indexNo)
	binary.LittleEndian.PutUint16(d[bktSample:], b.sample)
	binary.LittleEndian.PutUint16(d[bktFree:], uint16(b.free))
	binary.LittleEndian.PutUint16(d[bktNextID:], b.nextID)
	binary.LittleEndian.PutUint32(d[bktNext:], b.next)
	d[bktLevel] = byte(b.level)
	d[bktControl] = b.control

	for i := 0; i*BlockSize < len(d); i++ {
		if err := f.out.WriteBlock(b.vbn+uint32(i), d[i*BlockSize:(i+1)*BlockSize]); err != nil {
			return err
		}
	}

	return nil
}

// makeIndex makes a key's tree for its first record: an empty level 0
// bucket in the data area, and over it an index bucket holding one entry,
// the highest key of all. With a level 1 area apart from the index area, the
// root is a level 2 bucket in the index area over a level 1 bucket in that
// area (S63). It returns the level 0 bucket, not yet written.
func (f *File) makeIndex(k *Key) (*bucket, error) {
	vbn, err := f.allocate(k.DataArea)
	if err != nil {
		return nil, err
	}

	data := newBucket(vbn, k.DataBucketSize, k.Ref, 0, bktLast)

	root, err := f.allocate(k.Level1Area)
	if err != nil {
		return nil, err
	}

	level := 1

	if err := f.writeIndexBucket(k, root, level, vbn, k.IndexArea == k.Level1Area); err != nil {
		return nil, err
	}

	if k.IndexArea != k.Level1Area {
		lower := root

		root, err = f.allocate(k.IndexArea)
		if err != nil {
			return nil, err
		}

		level = 2

		if err := f.writeIndexBucket(k, root, level, lower, true); err != nil {
			return nil, err
		}
	}

	k.RootVBN, k.RootLevel, k.FirstData = root, level, vbn
	k.Flags &^= FlagNoIndexYet

	if err := f.updateKey(k); err != nil {
		return nil, err
	}

	return data, nil
}

// writeIndexBucket writes a new index bucket with one entry, the highest
// key of all, pointing a level down.
func (f *File) writeIndexBucket(k *Key, vbn uint32, level int, pointer uint32, isRoot bool) error {
	control := uint8(bktLast)
	if isRoot {
		control |= bktRoot
	}

	b := newBucket(vbn, k.IndexBucketSize, k.Ref, level, control)

	high := k.highestValue()

	stored := high
	if k.Flags&FlagIndexCompress != 0 {
		stored = compressKey(high, nil)
	}

	b.free += copy(b.data[b.free:], stored)

	// The pointers run down from the bucket's end, the first nearest it,
	// above a word that says where free space ends.
	size := pointerBytes(pointer)
	b.control |= uint8(size-2) << bktPtrShift

	at := len(b.data) - bktEndOffset - size
	writePointer(b.data[at:at+size], pointer)
	binary.LittleEndian.PutUint16(b.data[len(b.data)-bktEndOffset:], uint16(at-1))

	return f.writeBucket(b)
}

// highestValue is the value that sorts last in a key's order, which a
// level's last index entry holds. VMS writes it by the key's type (S19 to
// S30): the largest number of its kind, or for a descending key the smallest.
func (k *Key) highestValue() []byte {
	v := make([]byte, k.Size)

	switch k.Type {
	case String, Bin2, Bin4, Bin8:
		for i := range v {
			v[i] = 0xFF
		}
	case Int2, Int4, Int8:
		for i := range v {
			v[i] = 0xFF
		}

		v[k.Size-1] = 0x7F
	case Int2 | Descending, Int4 | Descending, Int8 | Descending:
		v[k.Size-1] = 0x80
	case Packed, Packed | Descending:
		for i := range v {
			v[i] = 0x99
		}

		v[k.Size-1] = 0x9C
		if k.Type.IsDescending() {
			v[k.Size-1] = 0x9B
		}
	}

	return v
}

// pointerBytes is the fewest bytes that hold a VBN, at least 2. Every
// pointer in VMS's files here is 2 bytes (data records' 4 aside), so what
// wider ones look like is unconfirmed.
func pointerBytes(vbn uint32) int {
	switch {
	case vbn <= 0xFFFF:
		return 2
	case vbn <= 0xFFFFFF:
		return 3
	}

	return 4
}

// writePointer writes a little-endian VBN, readPointer's opposite.
func writePointer(b []byte, vbn uint32) {
	for i := range b {
		b[i] = byte(vbn >> (8 * uint(i)))
	}
}

// PutResult says how a record went in.
type PutResult struct {
	RFA RFA

	// Duplicate means an allowed duplicate was taken for some key: RMS's
	// success status RMS$_OK_DUP rather than RMS$_SUC.
	Duplicate bool

	// Split means an Update's record no longer fitted its bucket, which split
	// for it. VMS's $UPDATE left an STV of 1 then (testdata/idx88, U137).
	Split bool
}

// dataPointerControl is a data record's control byte: its pointer is always
// 4 bytes, whatever the VBN.
const dataPointerControl = 2

// dataPointerBytes is that pointer's size.
const dataPointerBytes = 4

// Put adds a record, by its keys, as RMS's $PUT does: in the primary key's
// data bucket, then in each alternate's. So VMS's order of events can be
// seen in its files: a primary duplicate that isn't allowed is refused
// before anything is written, but an alternate's is found only after the
// record went in, which is then taken out again, so the bucket counts two
// writes and has spent an ID (S41).
func (f *File) Put(record []byte) (PutResult, error) {
	return f.put(record, false)
}

// PutLoad is Put with RMS's ROP LOA: a bucket takes records in key order only
// up to its key's fill size, leaving room for records to come (G119).
func (f *File) PutLoad(record []byte) (PutResult, error) {
	return f.put(record, true)
}

// put is Put and PutLoad.
func (f *File) put(record []byte, load bool) (PutResult, error) {
	f.loading = load

	if f.out == nil {
		return PutResult{}, ErrReadOnly
	}

	k := f.Keys[0]

	value, ok := k.Value(record)
	if !ok || f.attr.Format == Fixed && len(record) != f.attr.Size {
		return PutResult{}, ErrRecordSize
	}

	stored, err := f.encodeData(k, value, record)
	if err != nil {
		return PutResult{}, err
	}

	var result PutResult

	if k.HasIndex() {
		_, err := f.Find(0, value, Equal)

		switch {
		case err == nil && k.Flags&FlagDuplicates == 0:
			return PutResult{}, ErrDuplicate
		case err == nil:
			result.Duplicate = true
		case !errors.Is(err, ErrNotFound):
			return PutResult{}, err
		}
	}

	var b *bucket

	if k.HasIndex() {
		vbn, err := f.descend(k, value)
		if err != nil {
			return PutResult{}, err
		}

		b, err = f.lastFor(k, vbn, value)
		if err != nil {
			return PutResult{}, err
		}
	} else {
		b, err = f.makeIndex(k)
		if err != nil {
			return PutResult{}, err
		}
	}

	rfa, err := f.insertRecord(b, k, value, stored)
	if err != nil {
		return PutResult{}, err
	}

	result.RFA = rfa

	for i := 1; i < len(f.Keys); i++ {
		dup, err := f.insertAlternate(f.Keys[i], record, rfa)
		if err != nil {
			f.backOut(i, record, rfa)

			return PutResult{}, err
		}

		result.Duplicate = result.Duplicate || dup
	}

	return result, nil
}

// encodeData makes the parts of a data record after its pointer but for the
// key, which depends on the record before it: the compressed or plain bytes
// after the key. It refuses a record no bucket of its size could hold.
func (f *File) encodeData(k *Key, value, record []byte) ([]byte, error) {
	rest := k.recordRest(record)
	if k.Flags&FlagRecordCompress != 0 {
		rest = compressData(rest)
	}

	header := 3 + 2 + dataPointerBytes
	if !f.fixedStorage(k) {
		header += 2
	}

	size := header + len(storedKey(k, value, nil)) + len(rest)
	if size > k.DataBucketSize*BlockSize-bktOverhead-1 {
		return nil, ErrRecordSize
	}

	return rest, nil
}

// storedKey is a key as a record or SIDR holds it.
func storedKey(k *Key, value, previous []byte) []byte {
	if k.Flags&FlagKeyCompress != 0 {
		return compressKey(value, previous)
	}

	return value
}

// room says whether a bucket can take more bytes: all but its last byte, the
// second check character, can hold records.
func (b *bucket) room(more int) bool {
	return b.free+more <= len(b.data)-1
}

// limit is how far records may fill a level 0 bucket. ROP LOA stops a record
// going past the bucket's last at the key's fill size, and one going between
// others may use the whole bucket: no VMS case loads out of order, so that part
// is unconfirmed. A size of 0 means the whole bucket.
func (f *File) limit(k *Key, atEnd bool) int {
	if f.loading && atEnd {
		return k.DataFill
	}

	return 0
}

// roomFor is room with a limit from limit.
func (b *bucket) roomFor(limit, more int) bool {
	if limit > 0 && b.free+more > limit {
		return false
	}

	return b.room(more)
}

// replace swaps the bytes at an offset for others, moving what follows up
// to the free offset. Whatever a shorter result uncovers stays as it was:
// VMS leaves stale bytes past the free offset too.
func (b *bucket) replace(at, old int, with []byte) {
	copy(b.data[at+len(with):], b.data[at+old:b.free])
	copy(b.data[at:], with)
	b.free += len(with) - old
}

// insertRecord puts a record in its place in a data bucket, after any with
// the same key, and writes the bucket. A compressed key depends on the key
// before it, so the record after the new one has its key stored again.
func (f *File) insertRecord(b *bucket, k *Key, value, rest []byte) (RFA, error) {
	list, err := f.dataRecords(b, k)
	if err != nil {
		return RFA{}, err
	}

	// A deleted record's stub with this key gives way to it, in the same write:
	// U139's P0000003 went where its stub was, with a new ID, the bucket counting
	// one write.
	for i := range list {
		if r := &list[i]; r.isDeleted() && !r.isRRV() && k.Compare(r.key, value) == 0 {
			f.closeUp(b, k, list, i)

			if list, err = f.dataRecords(b, k); err != nil {
				return RFA{}, err
			}

			break
		}
	}

	at, previous, next := b.free, []byte(nil), (*dataRecord)(nil)

	for i := range list {
		r := &list[i]
		if r.isRRV() {
			continue
		}

		if k.Compare(r.key, value) > 0 {
			at, next = r.offset, r

			break
		}

		previous = r.key
	}

	rfa := RFA{VBN: b.vbn, ID: b.nextID}
	stored := f.encodeRecord(k, b.nextID, rfa, value, previous, rest)

	var again []byte

	var keyAt, oldKey int

	if next != nil && k.Flags&FlagKeyCompress != 0 {
		keyAt, oldKey = f.dataKeyPlace(b, k, next)
		again = compressKey(next.key, value)
	}

	if !b.roomFor(f.limit(k, next == nil), len(stored)+len(again)-oldKey) {
		return f.splitData(b, k, list, value, rest)
	}

	// The next record's key first, then the new record, so nothing's left past
	// the free offset (S6). A SIDR goes the other way round: insertSIDR.
	if again != nil {
		f.restoreDataKey(b, keyAt, oldKey, again)
	}

	b.replace(at, 0, stored)
	b.nextID++
	b.touch()

	return rfa, f.writeBucket(b)
}

// encodeRecord lays out one data record: control, ID, the RFA's ID and VBN,
// a size word unless the file's fixed and uncompressed, the key, the rest.
func (f *File) encodeRecord(k *Key, id uint16, rfa RFA, value, previous, rest []byte) []byte {
	key := storedKey(k, value, previous)

	out := make([]byte, 0, 13+len(key)+len(rest))
	out = append(out, dataPointerControl)
	out = binary.LittleEndian.AppendUint16(out, id)
	out = binary.LittleEndian.AppendUint16(out, rfa.ID)
	out = binary.LittleEndian.AppendUint32(out, rfa.VBN)

	if !f.fixedStorage(k) {
		out = binary.LittleEndian.AppendUint16(out, uint16(len(key)+len(rest)))
	}

	out = append(out, key...)

	return append(out, rest...)
}

// dataKeyPlace finds a stored record's key: its offset, and its compressed
// size. Only a file with a size word compresses keys.
func (f *File) dataKeyPlace(b *bucket, k *Key, r *dataRecord) (int, int) {
	at := r.offset + 3 + 2 + int(r.control&ircPtrSize) + 2
	if !f.fixedStorage(k) {
		at += 2
	}

	return at, 2 + int(b.data[at])
}

// restoreDataKey changes a stored record's key for another form of it,
// keeping its size word right.
func (f *File) restoreDataKey(b *bucket, keyAt, old int, with []byte) {
	size := keyAt - 2
	binary.LittleEndian.PutUint16(b.data[size:],
		uint16(int(binary.LittleEndian.Uint16(b.data[size:]))+len(with)-old))
	b.replace(keyAt, old, with)
}

// insertAlternate files a record under an alternate key: nothing if the
// record doesn't belong in that index, a pointer added to the SIDR for its
// value, or a new SIDR.
func (f *File) insertAlternate(k *Key, record []byte, rfa RFA) (bool, error) {
	value, belongs := k.Indexes(record)
	if !belongs {
		return false, nil
	}

	var b *bucket

	if k.HasIndex() {
		vbn, err := f.descend(k, value)
		if err != nil {
			return false, err
		}

		b, err = f.lastFor(k, vbn, value)
		if err != nil {
			return false, err
		}
	} else {
		var err error

		b, err = f.makeIndex(k)
		if err != nil {
			return false, err
		}
	}

	return f.insertSIDR(b, k, value, rfa)
}

// insertSIDR adds a pointer to a value's SIDR, the last in its array, or
// makes a SIDR in its place. A value's first pointer has IRC$M_FIRST_KEY.
func (f *File) insertSIDR(b *bucket, k *Key, value []byte, rfa RFA) (bool, error) {
	list, err := f.sidrs(b, k)
	if err != nil {
		return false, err
	}

	var previous []byte

	var next *sidr

	for i := range list {
		s := &list[i]

		switch c := k.Compare(s.key, value); {
		case c == 0:
			// Only a live record makes it a duplicate, not a pointer a fast delete
			// left (valueHeld).
			held, err := f.valueHeld(k, value)
			if err != nil {
				return false, err
			}

			if held && k.Flags&FlagDuplicates == 0 {
				return false, ErrDuplicate
			}

			return held, f.addPointer(b, k, list, i, rfa)
		case c > 0:
			next = s
		default:
			previous = s.key

			continue
		}

		break
	}

	key := storedKey(k, value, previous)
	pointer := sidrPointerBytes(rfa, true)

	stored := binary.LittleEndian.AppendUint16(nil, uint16(len(key)+len(pointer)))
	stored = append(stored, key...)
	stored = append(stored, pointer...)

	at := b.free

	var again []byte

	var oldKey int

	if next != nil {
		at = next.offset

		if k.Flags&FlagKeyCompress != 0 {
			oldKey = 2 + int(b.data[next.offset+2])
			again = compressKey(next.key, value)
		}
	}

	if !b.roomFor(f.limit(k, next == nil), len(stored)+len(again)-oldKey) {
		return false, f.growSIDRs(b, k, list, value, stored, next == nil)
	}

	// The new SIDR first, then the next one's key, as in insertRecord.
	if again != nil && !b.room(len(stored)) {
		restoreSIDRKey(b, next.offset, oldKey, again)
		b.replace(at, 0, stored)
	} else {
		b.replace(at, 0, stored)

		if again != nil {
			restoreSIDRKey(b, next.offset+len(stored), oldKey, again)
		}
	}

	b.touch()

	return false, f.writeBucket(b)
}

// restoreSIDRKey changes the key of the SIDR at an offset for another form
// of it, keeping its size word right.
func restoreSIDRKey(b *bucket, at, old int, with []byte) {
	binary.LittleEndian.PutUint16(b.data[at:],
		uint16(int(binary.LittleEndian.Uint16(b.data[at:]))+len(with)-old))
	b.replace(at+2, old, with)
}

// sidrPointerBytes lays out one SIDR pointer: a control byte, the record's
// ID, and its VBN in as few bytes as hold it.
func sidrPointerBytes(rfa RFA, first bool) []byte {
	size := pointerBytes(rfa.VBN)

	control := uint8(size - 2)
	if first {
		control |= ircFirstKey
	}

	out := append(make([]byte, 0, 3+size), control)
	out = binary.LittleEndian.AppendUint16(out, rfa.ID)

	vbn := make([]byte, size)
	writePointer(vbn, rfa.VBN)

	return append(out, vbn...)
}

// growSIDRs makes room for a new SIDR in a bucket that hasn't any. One going
// past the bucket's last has a new bucket to itself; otherwise half of them move.
func (f *File) growSIDRs(b *bucket, k *Key, list []sidr, value, stored []byte, atEnd bool) error {
	items := b.sidrItems(k, list)

	high := []byte(nil)
	if len(items) > 0 {
		high = items[len(items)-1].key
	}

	if atEnd {
		right, err := f.startRight(k.DataArea, k.DataBucketSize, k.Ref, 0)
		if err != nil {
			return err
		}

		pointers := bytes.Clone(stored[2+len(storedKey(k, value, high)):])
		right.putBytes(sidrBytes(k, value, nil, pointers))
		chainAfter(b, right)

		return f.linkNew(b, right, k, high, value)
	}

	at := 0
	for at < len(items) && k.Compare(items[at].key, value) < 0 {
		at++
	}

	var previous []byte
	if at > 0 {
		previous = items[at-1].key
	}

	pointers := bytes.Clone(stored[2+len(storedKey(k, value, previous)):])

	grown := make([]sidrItem, 0, len(items)+1)
	grown = append(grown, items[:at]...)
	grown = append(grown, sidrItem{key: value, pointers: pointers})
	grown = append(grown, items[at:]...)

	n, ok := chooseSIDRs(k, grown)
	if !ok {
		return b.corrupt(bktOverhead, "a SIDR too long for any split")
	}

	_, err := f.splitSIDRs(b, k, grown, n)

	return err
}

// addPointer adds a pointer to the end of a SIDR. With no room, a SIDR alone in
// its bucket goes on in a new bucket holding its key again and the new pointer,
// which has no FIRST_KEY bit and no index entry (G114). A SIDR that's the last of
// several moves whole to a new bucket, pointer and all (G115, where a value's
// third record moves the value's first two). One with others after it splits
// the bucket like a new SIDR would: unconfirmed until a VMS case shows it.
func (f *File) addPointer(b *bucket, k *Key, list []sidr, i int, rfa RFA) error {
	s := &list[i]

	pointer := sidrPointerBytes(rfa, false)
	if !b.room(len(pointer)) {
		items := b.sidrItems(k, list)

		if len(items) == 1 {
			right, err := f.startRight(k.DataArea, k.DataBucketSize, k.Ref, 0)
			if err != nil {
				return err
			}

			right.putBytes(sidrBytes(k, items[0].key, nil, pointer))
			chainAfter(b, right)

			return f.linkNew(b, right, k, items[0].key, items[0].key)
		}

		items[i].pointers = append(items[i].pointers, pointer...)

		n := i
		if i+1 < len(items) {
			var ok bool

			n, ok = chooseSIDRs(k, items)
			if !ok {
				return b.corrupt(bktOverhead, "a SIDR too long for any split")
			}
		}

		_, err := f.splitSIDRs(b, k, items, n)

		return err
	}

	size := int(binary.LittleEndian.Uint16(b.data[s.offset:]))
	end := s.offset + 2 + size

	binary.LittleEndian.PutUint16(b.data[s.offset:], uint16(size+len(pointer)))
	b.replace(end, 0, pointer)
	b.touch()

	return f.writeBucket(b)
}

// backOut takes a record out again after key failed: from the alternates
// done so far, last first, then the data bucket. A failure here leaves the
// record in the file, which is as RMS would leave it (the manual's
// RMS$_OK_IDX has a like case), so errors here aren't Put's to report.
func (f *File) backOut(failed int, record []byte, rfa RFA) {
	for i := failed - 1; i >= 1; i-- {
		k := f.Keys[i]
		if v, belongs := k.Indexes(record); belongs {
			_ = f.removePointer(k, v, rfa)
		}
	}

	_ = f.removeRecord(f.Keys[0], rfa)
}
