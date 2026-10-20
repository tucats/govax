package backup

import (
	"encoding/binary"
	"errors"
	"io"
)

// Writing a save set (docs/PHASE-69 - backup save sets.md, subtask 11),
// laid out as VMS V7.1's BACKUP V6.2 lays one out. Every rule here was
// read off the probe's save sets (testdata/backup69/vax), which the
// tests rewrite byte for byte from what Read makes of them:
//
//   - Block 1 starts with the summary record. Then each file is its
//     attribute record followed by its data records, which hold whole
//     virtual blocks and never cross a block's end: a file's data goes on
//     in the next block in a new data record whose address is the next
//     VBN. A record that doesn't fit where the block is starts the next
//     block.
//   - A record's body is padded with zeros to a multiple of 16 bytes, and
//     a null record (type 0) fills the rest of each block; its size runs
//     exactly to the block's end, 0 when only its header fits. govax
//     always leaves room for it, which no save set contradicts and none
//     settles (with every size a multiple of 16, a block can only end
//     with no room left at all, which the probe never shows).
//   - A block's header names the file whose attribute record was written
//     last before the block began (zero in block 1).
//   - After every GroupSize data blocks (and after the last, when the
//     group is short) comes their redundancy block: the XOR of the
//     group's blocks, its own size, kind, number, flags, and checks
//     written over (reader.go's repair is its inverse).
//
// VMS's BACKUP reads files ahead of writing them, and reports a file
// copied (/LOG's COPIED) only once the block holding its last record has
// gone out; it keeps up to the summary's buffer count of blocks before
// writing the oldest. The probe's log fits that rule (every NOBACKUP
// notice before the first COPIED, the whole tree fitting in eight
// buffers), which Writer follows; how VMS's buffers really cycle is
// unconfirmed.

// DefaultBlockSize is a disk save set's block size when /BLOCK_SIZE isn't
// given (the System Management Utilities Reference Manual), and
// DefaultGroupSize its redundancy group's.
const (
	DefaultBlockSize = 32256
	DefaultGroupSize = 10
)

// BufferBytes is what BACKUP's buffers hold together: the summary's
// buffer count is BufferBytes over the block size (8 for 32256-byte
// blocks, 32 for 8192, as VMS's save sets show; the rule between them is
// govax's).
const BufferBytes = 256 * 1024

// BuffersFor is the buffer count a save set of blockSize-byte blocks
// records.
func BuffersFor(blockSize int) int {
	return max(BufferBytes/blockSize, 1)
}

// StructureLevel is the word that starts every attribute list, and the
// structure level of every block.
const StructureLevel = 0x0101

// userNameSize is the summary's user name field, blank-padded.
const userNameSize = 12

// ErrBlockSize is a block size outside MinBlockSize to MaxBlockSize.
var ErrBlockSize = errors.New("backup: the block size must be 2048 to 65535 bytes")

// Writer writes a save set to an io.Writer, a block at a time.
type Writer struct {
	out     io.Writer
	summary Summary
	crc     bool

	blockSize, group, buffers int

	// Saved, if set, is called with each file's name (its attributes'
	// Name) once the block holding the file's last record is written.
	Saved func(name string)

	block   []byte   // the block being filled, nil before one starts
	at      int      // where its next record goes
	done    []string // the files whose last record is in it
	number  uint32   // the last block number given out
	current CurrentFile

	parity  []byte // the XOR of the group's blocks so far
	inGroup int

	pending []pendingBlock
	err     error
}

// pendingBlock is a block waiting in BACKUP's buffers.
type pendingBlock struct {
	data []byte
	done []string
}

// NewWriter starts a save set on out with summary s: its name, the
// command, and the rest, with BlockSize, GroupSize (0 for no redundancy
// blocks), and Buffers saying how it's laid out. With crc each block
// carries a data CRC; without, the NOCRC flag.
func NewWriter(out io.Writer, s Summary, crc bool) (*Writer, error) {
	if s.BlockSize < MinBlockSize || s.BlockSize > MaxBlockSize {
		return nil, ErrBlockSize
	}

	w := &Writer{
		out: out, summary: s, crc: crc,
		blockSize: int(s.BlockSize), group: int(s.GroupSize), buffers: max(int(s.Buffers), 1),
	}

	w.put(RecordSummary, 0, 0, EncodeSummary(s))

	return w, w.err
}

// Add writes one file: its attributes, then data, its used blocks in VBN
// order (a whole number of blocks; a short last block is padded with
// zeros). A file saved without its data (NOBACKUP) has none. flags are
// the records' flags: RecordDirectory for a directory file.
func (w *Writer) Add(attrs Attributes, flags RecordFlag, data []byte) error {
	if w.err != nil {
		return w.err
	}

	w.current = currentFile(attrs)
	w.put(RecordFile, flags, 0, EncodeAttributes(attrs))

	if rem := len(data) % VBNSize; rem != 0 {
		data = append(data[:len(data):len(data)], make([]byte, VBNSize-rem)...)
	}

	vbn := uint32(1)
	for len(data) > 0 && w.err == nil {
		blocks := w.room() / VBNSize
		if blocks == 0 {
			w.endBlock()

			continue
		}

		n := min(blocks*VBNSize, len(data))
		w.put(RecordVBN, flags, vbn, data[:n])
		vbn += uint32(n / VBNSize)
		data = data[n:]
	}

	w.done = append(w.done, attrs.Name)

	return w.err
}

// EndBlock ends the block being filled, so the next file starts a new
// one, as VMS's BACKUP starts each element of an input list in a block
// of its own (testdata/backup69's LIST.BCK).
func (w *Writer) EndBlock() error {
	if w.block != nil {
		w.endBlock()
	}

	return w.err
}

// Close ends the last block and its group, and writes every block still
// held.
func (w *Writer) Close() error {
	if w.block != nil {
		w.endBlock()
	}

	if w.inGroup > 0 {
		w.endGroup()
	}

	w.flush(0)

	return w.err
}

// Blocks is how many blocks the save set has so far, redundancy blocks
// too.
func (w *Writer) Blocks() int { return int(w.number) }

// room is how much of the current block a record's body may take: what
// is left after its record header and the closing null record's header.
// With no block started it's a whole block's.
func (w *Writer) room() int {
	at := w.at
	if w.block == nil {
		at = HeaderSize
	}

	return max(w.blockSize-at-2*RecordHeaderSize, 0)
}

// put writes one record, starting a new block first when it doesn't fit
// in this one. body is padded to a multiple of 16 bytes.
func (w *Writer) put(kind RecordType, flags RecordFlag, address uint32, body []byte) {
	size := (len(body) + 15) &^ 15

	if w.block != nil && size > w.room() {
		w.endBlock()
	}

	if w.block == nil {
		w.startBlock()
	}

	if size > w.room() {
		w.err = errors.New("backup: a record is larger than a save set block")

		return
	}

	le := binary.LittleEndian
	r := w.block[w.at:]
	le.PutUint16(r[brhSize:], uint16(size))
	le.PutUint16(r[brhType:], uint16(kind))
	le.PutUint32(r[brhFlags:], uint32(flags))
	le.PutUint32(r[brhAddress:], address)
	copy(r[RecordHeaderSize:], body)

	w.at += RecordHeaderSize + size
}

// startBlock begins the next data block.
func (w *Writer) startBlock() {
	w.block = make([]byte, w.blockSize)
	w.at = HeaderSize
	w.number++
	w.writeHeader(w.block, ApplicationData)
}

// writeHeader fills block's header for a block of kind application
// numbered w.number, naming w.current, without its checks.
func (w *Writer) writeHeader(block []byte, application uint16) {
	le := binary.LittleEndian
	le.PutUint16(block[bhdSize:], HeaderSize)
	le.PutUint16(block[bhdOpSys:], w.summary.OpSys)
	le.PutUint16(block[bhdSubsystem:], SubsystemBackup)
	le.PutUint16(block[bhdApplication:], application)
	le.PutUint32(block[bhdNumber:], w.number)
	le.PutUint16(block[bhdStrucLev:], StructureLevel)
	le.PutUint16(block[bhdVolume:], 1)
	le.PutUint32(block[bhdBlockSize:], uint32(w.blockSize))
	le.PutUint32(block[bhdFlags:], w.flags())
	putCounted(block[bhdSaveSetName:bhdSaveSetName+32], w.summary.Name)

	c := w.current
	for i := range 3 {
		le.PutUint16(block[bhdFID+2*i:], c.FID[i])
		le.PutUint16(block[bhdDID+2*i:], c.DID[i])
	}

	putCounted(block[bhdFileName:bhdFileName+128], c.Name)
	block[bhdRecordType] = c.RecordType
	block[bhdRecordAttr] = c.RecordAttr
	le.PutUint16(block[bhdRecordSize:], c.RecordSize)
	block[bhdBucketSize] = c.BucketSize
	block[bhdVFCSize] = c.VFCSize
	le.PutUint16(block[bhdMaxRecord:], c.MaxRecord)
	le.PutUint32(block[bhdFileSize:], c.Allocation)
}

// flags is a block header's flags.
func (w *Writer) flags() uint32 {
	if w.crc {
		return 0
	}

	return FlagNoCRC
}

// seal writes block's data CRC (with /CRC) and header checksum.
func (w *Writer) seal(block []byte) {
	le := binary.LittleEndian
	le.PutUint32(block[bhdCRC:], 0)

	if w.crc {
		le.PutUint32(block[bhdCRC:], DataCRC(block))
	}

	le.PutUint16(block[bhdChecksum:], HeaderChecksum(block))
}

// endBlock closes the current block with its null record, seals it, adds
// it to its group, and queues it.
func (w *Writer) endBlock() {
	block := w.block
	le := binary.LittleEndian
	le.PutUint16(block[w.at+brhSize:], uint16(w.blockSize-w.at-RecordHeaderSize))
	le.PutUint16(block[w.at+brhType:], uint16(RecordNull))
	w.seal(block)

	w.queue(block, w.done)
	w.block, w.done = nil, nil

	if w.group == 0 {
		return
	}

	if w.parity == nil {
		w.parity = make([]byte, w.blockSize)
	}

	for i, b := range block {
		w.parity[i] ^= b
	}

	if w.inGroup++; w.inGroup == w.group {
		w.endGroup()
	}
}

// endGroup queues the group's redundancy block.
func (w *Writer) endGroup() {
	block := w.parity
	w.parity, w.inGroup = nil, 0
	w.number++

	le := binary.LittleEndian
	le.PutUint16(block[bhdSize:], HeaderSize)
	le.PutUint16(block[bhdOpSys:], w.summary.OpSys)
	le.PutUint16(block[bhdSubsystem:], SubsystemBackup)
	le.PutUint16(block[bhdApplication:], ApplicationXOR)
	le.PutUint32(block[bhdNumber:], w.number)
	le.PutUint32(block[bhdFlags:], w.flags())
	w.seal(block)

	w.queue(block, nil)
}

// queue holds block in a buffer, writing the oldest when every buffer is
// in use.
func (w *Writer) queue(block []byte, done []string) {
	w.pending = append(w.pending, pendingBlock{data: block, done: done})
	w.flush(w.buffers)
}

// flush writes the oldest held blocks until keep are left, and reports
// the files they finish.
func (w *Writer) flush(keep int) {
	for len(w.pending) > keep {
		b := w.pending[0]
		w.pending = w.pending[1:]

		if w.err == nil {
			_, w.err = w.out.Write(b.data)
		}

		if w.Saved != nil {
			for _, name := range b.done {
				w.Saved(name)
			}
		}
	}
}

// currentFile is the block header's description of the file attrs
// describes: its file ID and its directory's with the relative volume
// number 1 (as VMS writes them there; a backlink may have 0), its name,
// its record attributes, and its allocation.
func currentFile(attrs Attributes) CurrentFile {
	fat := attrs.RecordAttributes

	return CurrentFile{
		FID:        onVolume(attrs.FID),
		DID:        onVolume(attrs.BackLink),
		Name:       attrs.Name,
		RecordType: fat[fatRecordType],
		RecordAttr: fat[fatRecordAttr],
		RecordSize: attrs.RecordSize(),
		BucketSize: attrs.BucketSize(),
		VFCSize:    attrs.VFCSize(),
		MaxRecord:  attrs.MaxRecord(),
		Allocation: attrs.Allocated,
	}
}

// onVolume is a file ID with relative volume number 1 when it has 0, as
// a single volume's files have.
func onVolume(id [3]uint16) [3]uint16 {
	if id[2] == 0 {
		id[2] = 1
	}

	return id
}

// putCounted writes s into field as a counted string, cut to fit.
func putCounted(field []byte, s string) {
	n := min(len(s), len(field)-1)
	field[0] = byte(n)
	copy(field[1:], s[:n])
}

// attributeWriter builds an attribute list.
type attributeWriter struct{ b []byte }

func newAttributeWriter() *attributeWriter {
	return &attributeWriter{b: binary.LittleEndian.AppendUint16(nil, StructureLevel)}
}

func (a *attributeWriter) bytes(kind AttributeType, value []byte) {
	a.b = binary.LittleEndian.AppendUint16(a.b, uint16(len(value)))
	a.b = binary.LittleEndian.AppendUint16(a.b, uint16(kind))
	a.b = append(a.b, value...)
}

func (a *attributeWriter) text(kind AttributeType, s string) { a.bytes(kind, []byte(s)) }
func (a *attributeWriter) byte1(kind AttributeType, v uint8) { a.bytes(kind, []byte{v}) }

func (a *attributeWriter) word(kind AttributeType, v uint16) {
	a.bytes(kind, binary.LittleEndian.AppendUint16(nil, v))
}

func (a *attributeWriter) long(kind AttributeType, v uint32) {
	a.bytes(kind, binary.LittleEndian.AppendUint32(nil, v))
}

func (a *attributeWriter) quad(kind AttributeType, v uint64) {
	a.bytes(kind, binary.LittleEndian.AppendUint64(nil, v))
}

func (a *attributeWriter) fileID(kind AttributeType, id [3]uint16) {
	var v []byte
	for _, w := range id {
		v = binary.LittleEndian.AppendUint16(v, w)
	}

	a.bytes(kind, v)
}

// EncodeSummary is a summary record's body: its attributes in the order
// VMS writes them, the comment and node name only when there are any,
// the user name blank-padded to 12.
func EncodeSummary(s Summary) []byte {
	a := newAttributeWriter()
	a.text(AttrSaveSetName, s.Name)
	a.text(AttrCommand, s.Command)

	if s.Comment != "" {
		a.text(AttrComment, s.Comment)
	}

	user := s.UserName
	for len(user) < userNameSize {
		user += " "
	}

	a.text(AttrUserName, user)
	a.long(AttrUserUIC, s.UserUIC)
	a.quad(AttrDate, s.Date)
	a.word(AttrOpSys, s.OpSys)
	a.text(AttrSystemVersion, s.SystemVersion)

	if s.NodeName != "" {
		a.text(AttrNodeName, s.NodeName)
	}

	a.long(AttrCPUID, s.CPUID)
	a.text(AttrDevice, s.Device)
	a.text(AttrBackupVersion, s.BackupVersion)
	a.long(AttrBlockSize, s.BlockSize)
	a.word(AttrGroupSize, s.GroupSize)
	a.word(AttrBuffers, s.Buffers)

	return a.b
}

// EncodeAttributes is a file record's body: the file's attributes in the
// order VMS writes them (Part A's table), then DIR_STATUS for a directory
// file and the ACL for a file with one.
func EncodeAttributes(f Attributes) []byte {
	a := newAttributeWriter()
	a.text(AttrFileName, f.Name)
	a.word(AttrStrucLev, f.StructureLevel)
	a.fileID(AttrFID, f.FID)
	a.long(AttrFileSize, f.Allocated)
	a.long(AttrOwner, f.Owner)
	a.bytes(AttrRecordAttrs, f.RecordAttributes[:])
	a.fileID(AttrBackLink, f.BackLink)
	a.word(AttrProtection, f.Protection)
	a.word(AttrRecordProt, f.RecordProt)
	a.byte1(AttrAccessLevel, f.AccessLevel)
	a.long(AttrCharacteristics, f.Characteristics)
	a.word(AttrVersionLimit, f.VersionLimit)
	a.byte1(AttrJournal, f.Journal)
	a.byte1(AttrRUActive, f.RUActive)
	a.long(AttrHighwater, f.Highwater)
	a.word(AttrRevision, f.Revision)
	a.quad(AttrCreated, f.Created)
	a.quad(AttrRevised, f.Revised)
	a.quad(AttrExpires, f.Expires)
	a.quad(AttrBackedUp, f.Backup)
	a.long(AttrDirOwner, f.DirOwner)
	a.word(AttrDirProtection, f.DirProtection)
	a.word(AttrDirVersionLimit, f.DirVersionLimit)

	if f.Directory() {
		a.byte1(AttrDirStatus, uint8(f.DirStatus))
	}

	if len(f.ACL) > 0 {
		a.bytes(AttrACLSegment, f.ACL)
	}

	return a.b
}
