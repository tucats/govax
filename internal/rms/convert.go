package rms

import (
	"bytes"
	"errors"
	"io/fs"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/isam"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// CONVERT's files (docs/planning/PHASE-88 - keyed index files.md, subtask 10c; the
// command is internal/dclcmd/convert.go). As VMS's CONVERT does, this goes through
// RMS's services on blocks in a memory of its own: $OPEN, $CONNECT, and $GET for
// an input, $CREATE (by CreateFDL or CreateLike) then $OPEN, $CONNECT, and $PUT for
// an output. A fast load is the one thing that isn't a service: the file is
// opened, then its buckets built directly (isam's Load), as the manual has it, "by
// building the indexes directly".
//
// An input is read whole before anything else is done, so a file may be converted
// to a new version of itself.

// The layout of the rest of a conversion's memory, after the FAB and NAM that
// CreateFDL's layout has.
const (
	convRABAddr uint32 = 0x900
	convRHBAddr uint32 = 0xA00
)

// A separate record buffer, past fdlMemSize's blocks.
const convMemSize = 0x20000

// convBufAddr is where a conversion's records pass through.
const convBufAddr uint32 = 0x10000

// ConvertRecord is one record of an input: its data, and a VFC record's fixed
// control apart from it.
type ConvertRecord struct {
	Control []byte
	Data    []byte
}

// ConvertInput is an input file read whole, and what a file with its attributes
// needs (CreateLike).
type ConvertInput struct {
	// Name is the file's resultant name.
	Name string

	Records []ConvertRecord

	// Indexed says it's an indexed file; Keys has its keys then, and Areas its areas.
	Indexed bool
	Keys    []*isam.Key
	Areas   []isam.Area
	Prolog  int

	// VFCSize is a VFC file's fixed control size, 0 for any other.
	VFCSize int

	org, rfm, rat, fsz, bks byte
	mrs, deq, gbc           uint16
	alq, mrn                uint32
}

// ConvertOpenError is an input or output file that couldn't be opened: its name
// as a message shows it, and RMS's status.
type ConvertOpenError struct {
	Name        string
	Status, STV uint32
}

func (e *ConvertOpenError) Error() string { return "rms: can't open " + e.Name }

// convertBlocks starts a context and a FAB and NAM for a file called name.
func (s *Session) convertBlocks(name string, fac, shr byte) (*Context, error) {
	ctx := s.blockContext()
	ctx.Mem = vm.NewMemory(convMemSize) // room for a record of any size

	b := &fdlBlocks{mem: make([]byte, fdlMemSize)}
	fab := fdlFABAddr

	b.byteAt(fab+fabOffset("BID"), fabBIDValue)
	b.byteAt(fab+fabOffset("BLN"), fabBLNValue)
	b.byteAt(fab+fabFAC, fac)
	b.byteAt(fab+fabSHR, shr)

	fdlText(b, fab+fabOffset("FNA"), fab+fabOffset("FNS"), fdlFNAAddr, name)

	nam := fdlNAMAddr
	b.long(fab+fabOffset("NAM"), nam)
	b.byteAt(nam+namBID, namBIDValue)
	b.byteAt(nam+namBLN, namBLNValue)
	b.long(nam+namESA, fdlESAAddr)
	b.byteAt(nam+namESS, fdlNameSize)
	b.long(nam+namRSA, fdlRSAAddr)
	b.byteAt(nam+namRSS, fdlNameSize)

	rab := convRABAddr
	b.byteAt(rab+rabBID, rabBIDValue)
	b.byteAt(rab+rabBLN, rabBLNValue)
	b.long(rab+rabFAB, fab)
	b.long(rab+rabRHB, convRHBAddr)
	b.long(rab+rabUBF, convBufAddr)
	b.word(rab+rabUSZ, 0xFFFF)
	b.long(rab+rabRBF, convBufAddr)

	return ctx, ctx.storeBytes(0, b.mem)
}

// openBlocks calls $OPEN on a context's FAB, returning the failure if any.
func openBlocks(ctx *Context) *ConvertOpenError {
	if _, err := SysOpen(ctx, []uint32{fdlFABAddr}); err != nil {
		return &ConvertOpenError{Status: rmsDeviceError}
	}

	sts, _ := ctx.loadLongword(fdlFABAddr + fabSTS)
	stv, _ := ctx.loadLongword(fdlFABAddr + fabSTV)

	if sts&1 == 0 {
		return &ConvertOpenError{Name: fdlString(ctx, namESL, fdlESAAddr), Status: sts, STV: stv}
	}

	return nil
}

// ReadConvertInput reads the file spec names, in the order of key of reference krf
// on an indexed file (any other ignores it), with sharing for /SHARE. A record it
// can't read ends the reading quietly with the records before it, as VMS's CONVERT
// of damaged files did (C12's X05 and X07: status 1). A host file is text, a record
// a line.
func (s *Session) ReadConvertInput(spec string, krf int, share bool) (*ConvertInput, error) {
	loc, err := s.Locate(spec, false)
	if err != nil {
		return nil, err
	}

	if loc.Host {
		return s.readHostInput(loc)
	}

	shr := facGet
	if share {
		shr |= facPut | facUpd | facDel
	}

	ctx, err := s.convertBlocks(loc.Name, facGet, shr)
	if err != nil {
		return nil, err
	}

	if failure := openBlocks(ctx); failure != nil {
		return nil, failure
	}

	in := &ConvertInput{Name: fdlString(ctx, namRSL, fdlRSAAddr)}

	defer func() { _, _ = SysClose(ctx, []uint32{fdlFABAddr}) }()

	ifi, err := ctx.loadWord(fdlFABAddr + fabIFI)
	if err != nil {
		return nil, err
	}

	h, ok := ctx.Files.Lookup(ifi)
	if !ok {
		return nil, &ConvertOpenError{Name: in.Name, Status: rmsInvalidIFI}
	}

	if err := in.capture(h); err != nil {
		return nil, err
	}

	if in.Indexed && krf >= len(in.Keys) {
		return in, ErrNoSuchKey
	}

	if in.Indexed {
		if err := ctx.storeByte(convRABAddr+rabKRF, byte(krf)); err != nil {
			return nil, err
		}
	}

	if _, err := SysConnect(ctx, []uint32{convRABAddr}); err != nil {
		return nil, err
	}

	for {
		if _, err := SysGet(ctx, []uint32{convRABAddr}); err != nil {
			return nil, err
		}

		sts, _ := ctx.loadLongword(convRABAddr + rabSTS)
		if sts&1 == 0 {
			break
		}

		rsz, _ := ctx.loadWord(convRABAddr + rabRSZ)
		rbf, _ := ctx.loadLongword(convRABAddr + rabRBF)

		data, err := ctx.loadBytes(rbf, int(rsz))
		if err != nil {
			return nil, err
		}

		in.Records = append(in.Records, in.split(ctx, data))
	}

	return in, nil
}

// ErrNoSuchKey is /KEY past an indexed input's keys.
var ErrNoSuchKey = errors.New("rms: no such key")

// split parts a record read from a VFC file: a sequential file's $GET has its
// control in front, and a relative file's leaves it at RAB$L_RHB.
func (in *ConvertInput) split(ctx *Context, data []byte) ConvertRecord {
	if in.VFCSize == 0 {
		return ConvertRecord{Data: data}
	}

	if in.org == orgRel {
		control, _ := ctx.loadBytes(convRHBAddr, in.VFCSize)

		return ConvertRecord{Control: control, Data: data}
	}

	if len(data) < in.VFCSize {
		return ConvertRecord{Control: make([]byte, in.VFCSize), Data: data}
	}

	return ConvertRecord{Control: data[:in.VFCSize], Data: data[in.VFCSize:]}
}

// capture copies what CreateLike needs from an open file.
func (in *ConvertInput) capture(h *FileHandle) error {
	ra := h.File.Header.RecordAttributes
	whole := byte(ra.Format)

	in.org, in.rfm, in.rat = whole&0xF0, whole&0x0F, byte(ra.Attributes)
	in.mrs, in.deq, in.bks, in.gbc = ra.MaxRecordSize, ra.DefaultExtend, ra.BucketSize, ra.GlobalBufferCount
	in.alq = ra.HighestBlock

	if ondisk.RecordFormat(in.rfm) == ondisk.RecordFormatVFC {
		in.fsz = ra.VfcSize
		in.VFCSize = int(ra.VfcSize)
	}

	switch {
	case isIndexed(h.File):
		f, err := openIndexed(h.File, false)
		if err != nil {
			return &ConvertOpenError{Name: "", Status: prologStatus(err)}
		}

		in.Indexed, in.Keys, in.Areas, in.Prolog = true, f.Keys, f.Areas, f.Version

	case isRelative(h.File):
		mrn, err := prologueMRNOf(h.File)
		if err != nil {
			return err
		}

		in.mrn = mrn
	}

	return nil
}

// readHostInput reads a host file as a sequential text file's records.
func (s *Session) readHostInput(loc FileLocation) (*ConvertInput, error) {
	records, _, err := s.ReadRecordFile(loc, TextRecords)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &ConvertOpenError{Name: strings.ToUpper(loc.Name), Status: rmsFileNotFound}
		}

		return nil, err
	}

	in := &ConvertInput{
		Name: loc.Name, org: orgSeq, rfm: byte(ondisk.RecordFormatVariable), rat: ratCR,
	}

	for _, r := range records {
		in.Records = append(in.Records, ConvertRecord{Data: r})
	}

	return in, nil
}

// SameKey says whether a key has the same place and type as an input's primary
// key, whose order the records are then in already: VMS 7.1's CONVERT sorted
// nothing for such a key (C10B's /WORK_FILES=11 was refused only at key 1, when
// key 0 was loaded).
func (in *ConvertInput) SameKey(k *isam.Key) bool {
	if !in.Indexed || len(in.Keys) == 0 || k.Type != in.Keys[0].Type || len(k.Segments) != len(in.Keys[0].Segments) {
		return false
	}

	for i, s := range k.Segments {
		if s != in.Keys[0].Segments[i] {
			return false
		}
	}

	return true
}

// CreateLike makes a file called name with in's attributes, as CONVERT does with
// no FDL file (C6.2's C62.IDX is F122.IDX's, its areas, keys, and fills), and
// closes it. The owner and protection are the creator's and the volume's
// (unconfirmed: every input had those).
func (s *Session) CreateLike(in *ConvertInput, name string, prolog int) FDLCreated {
	ctx := s.blockContext()
	b := &fdlBlocks{mem: make([]byte, fdlMemSize)}
	fab := fdlFABAddr

	b.byteAt(fab+fabOffset("BID"), fabBIDValue)
	b.byteAt(fab+fabOffset("BLN"), fabBLNValue)
	b.byteAt(fab+fabFAC, facPut)
	b.byteAt(fab+fabORG, in.org)
	b.byteAt(fab+fabRFM, in.rfm)
	b.byteAt(fab+fabRAT, in.rat)
	b.word(fab+fabMRS, in.mrs)
	b.byteAt(fab+fabOffset("FSZ"), in.fsz)
	b.long(fab+fabOffset("ALQ"), in.alq)
	b.word(fab+fabOffset("DEQ"), in.deq)
	b.byteAt(fab+fabOffset("BKS"), in.bks)
	b.word(fab+fabOffset("GBC"), in.gbc)
	b.long(fab+fabOffset("MRN"), in.mrn)
	b.long(fab+fabFOP, fopMXV)

	fdlText(b, fab+fabOffset("FNA"), fab+fabOffset("FNS"), fdlFNAAddr, name)

	nam := fdlNAMAddr
	b.long(fab+fabOffset("NAM"), nam)
	b.byteAt(nam+namBID, namBIDValue)
	b.byteAt(nam+namBLN, namBLNValue)
	b.long(nam+namESA, fdlESAAddr)
	b.byteAt(nam+namESS, fdlNameSize)
	b.long(nam+namRSA, fdlRSAAddr)
	b.byteAt(nam+namRSS, fdlNameSize)

	for i := range in.Areas {
		a := &in.Areas[i]
		addr := fdlAllAddr + uint32(i)*fdlXABAlign

		b.xab(addr, xabALL, uint32(xabLengths[xabALL]))
		b.byteAt(addr+xabAllAID, byte(i))
		b.byteAt(addr+xabAllBKZ, byte(a.BucketSize))
		b.long(addr+xabALQ, a.TotalAllocation)
		b.word(addr+xabDEQ, uint16(a.ExtendSize))
		b.byteAt(addr+xabAOP, a.Options)
		b.byteAt(addr+xabAllALN, a.Alignment)
		b.word(addr+xabAllVOL, uint16(a.Volume))
		b.long(addr+xabAllLOC, a.Location)
	}

	for i, k := range in.Keys {
		addr := fdlKeyAddr + uint32(i)*fdlKeyStep

		b.xab(addr, xabKEY, vmsConst("XAB$C_KEYLEN"))
		b.byteAt(addr+xabKeyREF, byte(i))
		b.byteAt(addr+xabKeyDTP, byte(k.Type))
		b.byteAt(addr+xabKeyFLG, xabFlags(k.Flags&^isam.FlagNoIndexYet))
		b.byteAt(addr+xabKeyNUL, k.Null)
		b.byteAt(addr+xabKeyDAN, byte(k.DataArea))
		b.byteAt(addr+xabKeyIAN, byte(k.IndexArea))
		b.byteAt(addr+xabKeyLAN, byte(k.Level1Area))
		b.word(addr+xabKeyDFL, uint16(k.DataFill))
		b.word(addr+xabKeyIFL, uint16(k.IndexFill))

		for n, seg := range k.Segments {
			b.byteAt(addr+xabKeySIZ+uint32(n), byte(seg.Size))
			b.word(addr+xabKeyPOS+2*uint32(n), uint16(seg.Position))
		}

		if k.Named {
			knm := fdlKNMAddr + uint32(i)*keyNameSize
			field := b.mem[knm : knm+keyNameSize]

			for j := range field {
				field[j] = ' '
			}

			copy(field, k.Name)
			b.long(addr+xabKeyKNM, knm)
		}
	}

	if prolog != 0 && len(in.Keys) > 0 {
		b.byteAt(fdlKeyAddr+xabKeyPROLOG, byte(prolog))
	}

	return createFromBlocks(ctx, b)
}

// ConvertOutput is an output file open for records.
type ConvertOutput struct {
	// Name is the file's resultant name.
	Name string

	// Indexed and Relative say what it is; Fixed that its records have one size,
	// MaxRecord; VFCSize its fixed control's size, if it's VFC.
	Indexed, Relative, Sequential bool
	Fixed                         bool
	MaxRecord                     int
	VFCSize                       int

	// PrintControl says a record with no fixed control of its own gets a print
	// file's, a new line before and a carriage return after: 01 8D (C83D, where the
	// manual says null).
	PrintControl bool

	// Undefined says its records are of undefined format.
	Undefined bool

	// Primary is an indexed file's primary key; KeyCount how many it has.
	Primary  *isam.Key
	KeyCount int

	// Empty says an indexed file has no records.
	Empty bool

	ctx  *Context
	h    *FileHandle
	file *isam.File
	load *isam.Load

	// rev is the revision a finished load leaves; the file's first key's end has
	// been reached when primaryDone.
	rev         uint16
	created     bool
	primaryDone bool
	alternates  int
	endBefore   uint32

	// raw and longest are an undefined file's bytes and its longest record.
	raw     []byte
	longest int
}

// OpenConvertOutput opens the file called name for its records. created says
// this conversion just made it; appending positions at the end of a sequential
// file, as /APPEND does.
func (s *Session) OpenConvertOutput(name string, created, appending bool) (*ConvertOutput, error) {
	ctx, err := s.convertBlocks(name, facPut|facGet|facUpd, 0)
	if err != nil {
		return nil, err
	}

	if failure := openBlocks(ctx); failure != nil {
		return nil, failure
	}

	ifi, err := ctx.loadWord(fdlFABAddr + fabIFI)
	if err != nil {
		return nil, err
	}

	h, ok := ctx.Files.Lookup(ifi)
	if !ok {
		return nil, &ConvertOpenError{Name: name, Status: rmsInvalidIFI}
	}

	o := &ConvertOutput{Name: fdlString(ctx, namRSL, fdlRSAAddr), ctx: ctx, h: h, created: created}

	ra := h.File.Header.RecordAttributes
	whole := byte(ra.Format)
	format := ondisk.RecordFormat(whole & 0x0F)

	o.MaxRecord = int(ra.MaxRecordSize)
	o.Fixed = format == ondisk.RecordFormatFixed
	o.Undefined = format == ondisk.RecordFormatUndefined

	if format == ondisk.RecordFormatVFC {
		o.VFCSize = int(ra.VfcSize)
		o.PrintControl = ra.VfcSize == 2 && byte(ra.Attributes)&ratPRN != 0
	}

	switch {
	case isIndexed(h.File):
		o.Indexed = true

		f, err := openIndexed(h.File, true)
		if err != nil {
			return nil, &ConvertOpenError{Name: o.Name, Status: prologStatus(err)}
		}

		o.file, o.Primary, o.KeyCount = f, f.Keys[0], len(f.Keys)
		o.Empty = !f.Keys[0].HasIndex()
	case isRelative(h.File):
		o.Relative = true
	default:
		o.Sequential = true
	}

	if appending {
		if err := ctx.storeLongword(convRABAddr+rabROP, vmsConst("RAB$M_EOF")); err != nil {
			return nil, err
		}
	}

	if _, err := SysConnect(ctx, []uint32{convRABAddr}); err != nil {
		return nil, err
	}

	if sts, _ := ctx.loadLongword(convRABAddr + rabSTS); sts&1 == 0 {
		stv, _ := ctx.loadLongword(convRABAddr + rabSTV)

		return nil, &ConvertOpenError{Name: o.Name, Status: sts, STV: stv}
	}

	return o, nil
}

// Put writes a record by $PUT: on an indexed file by key, with ROP LOA for a
// load's fills unless fill says /FILL_BUCKETS, and on any other in sequence. It
// returns RMS's status and STV.
func (o *ConvertOutput) Put(control, data []byte, loading, fill bool) (uint32, uint32) {
	ctx := o.ctx
	record := data

	// $PUT has no record to frame in a file of undefined records, so CONVERT's are
	// its bytes, one after another, written when the file is closed (C84E).
	if o.Undefined {
		o.raw = append(o.raw, data...)
		o.longest = max(o.longest, len(data))

		return rmsNormal, 0
	}

	// A sequential file's fixed control goes in front of its record ($GET's split
	// has the other side of this); a relative file's at RAB$L_RHB.
	if o.VFCSize != 0 {
		if o.Sequential {
			record = append(append(make([]byte, 0, len(control)+len(data)), control...), data...)
		} else if err := ctx.storeBytes(convRHBAddr, control); err != nil {
			return rmsDeviceError, 0
		}
	}

	if len(record) > 0xFFFF {
		return rmsRecordTooBig, 0
	}

	if err := ctx.storeBytes(convBufAddr, record); err != nil {
		return rmsDeviceError, 0
	}

	var rop uint32

	rac := racSeq
	if o.Indexed {
		rac = racKey

		// A record goes in by the key's value: RAB$L_KBF isn't read for a $PUT.
		if loading && !fill {
			rop = vmsConst("RAB$M_LOA")
		}
	}

	for _, set := range []error{
		ctx.storeByte(convRABAddr+rabRAC, rac),
		ctx.storeWord(convRABAddr+rabRSZ, uint16(len(record))),
		ctx.storeLongword(convRABAddr+rabRBF, convBufAddr),
	} {
		if set != nil {
			return rmsDeviceError, 0
		}
	}

	if o.Indexed {
		if err := ctx.storeLongword(convRABAddr+rabROP, rop); err != nil {
			return rmsDeviceError, 0
		}
	}

	if _, err := SysPut(ctx, []uint32{convRABAddr}); err != nil {
		return rmsDeviceError, 0
	}

	sts, _ := ctx.loadLongword(convRABAddr + rabSTS)
	stv, _ := ctx.loadLongword(convRABAddr + rabSTV)

	return sts, stv
}

// StartFast begins a fast load of an indexed file with no records. The end of
// file stays as it was until the primary key is finished.
func (o *ConvertOutput) StartFast(fill bool) {
	o.endBefore = o.h.File.Header.RecordAttributes.EndOfFileBlock
	o.load = o.file.StartLoad(fill)
}

// AddFast adds a record to a fast load.
func (o *ConvertOutput) AddFast(record []byte) error {
	_, err := o.load.Add(record)

	return err
}

// FinishPrimary ends a fast load's primary key.
func (o *ConvertOutput) FinishPrimary() error {
	if err := o.load.FinishPrimary(); err != nil {
		return err
	}

	o.primaryDone = true

	return nil
}

// FinishAlternate ends a fast load's alternate key i.
func (o *ConvertOutput) FinishAlternate(i int) error {
	if err := o.load.FinishAlternate(i); err != nil {
		return err
	}

	o.alternates++

	return nil
}

// Close ends a conversion's use of the file. A fast load's file ends at its one
// area's next block once the primary key is finished, the blocks after it being
// its own (C71's 139 of 246), and a file of more areas where it did; a load that
// stopped sooner leaves the end of file as it found it (C78D's 0, though the file
// grew). VMS counted a revision for each alternate key loaded: 8 keys are revision
// 8, one key revision 1, and /NOCREATE one more than the file had (C91).
func (o *ConvertOutput) Close() error {
	f := o.h.File

	if o.load != nil {
		o.load.End()

		end := o.endBefore
		if o.primaryDone && len(o.file.Areas) == 1 {
			end = o.file.Areas[0].NextVBN
		}

		if err := volume.UpdateHeader(f, func(h *ondisk.FileHeader, id *ondisk.Ident) {
			h.RecordAttributes.EndOfFileBlock = end
			h.RecordAttributes.FirstFreeByte = 0

			if o.created {
				id.Revision = uint16(o.alternates)
			}
		}); err != nil {
			return err
		}
	}

	if o.Undefined {
		if err := o.writeRaw(f); err != nil {
			return err
		}
	}

	// A sequential or relative file this conversion made is revision 1, as if
	// never closed since its creation (C61, C65).
	if o.load == nil && o.created && !o.Indexed {
		if err := volume.UpdateHeader(f, func(_ *ondisk.FileHeader, id *ondisk.Ident) { id.Revision = 0 }); err != nil {
			return err
		}
	}

	if _, err := SysClose(o.ctx, []uint32{fdlFABAddr}); err != nil {
		return err
	}

	return nil
}

// writeRaw writes an undefined file's bytes by blocks, extending the file as they
// need, and ends it where they end.
func (o *ConvertOutput) writeRaw(f *volume.File) error {
	if len(o.raw) == 0 {
		return nil
	}

	blocks := uint32((len(o.raw) + ondisk.BlockSize - 1) / ondisk.BlockSize)

	if blocks > f.Blocks() {
		if _, _, err := (indexedBlocks{f}).Extend(blocks - f.Blocks()); err != nil {
			return err
		}
	}

	for i := uint32(0); i < blocks; i++ {
		block := make([]byte, ondisk.BlockSize)
		copy(block, o.raw[int(i)*ondisk.BlockSize:])

		if err := f.WriteBlock(i+1, block); err != nil {
			return err
		}
	}

	f.SetEndOfFile(uint32(len(o.raw)/ondisk.BlockSize)+1, uint16(len(o.raw)%ondisk.BlockSize))

	return volume.UpdateHeader(f, func(h *ondisk.FileHeader, _ *ondisk.Ident) {
		h.RecordAttributes.RecordSize = uint16(o.longest)
	})
}

// SortRecords sorts records by a key, keeping equal ones in order, or with
// byRecord (more than one input, VMS's sort being given the records themselves:
// C79's "AAA a duplicate" came before "AAA two") by the whole record next. A
// record too short for the key sorts with what it has, padded with zeros (C79's
// CC before CCC).
func SortRecords(records []ConvertRecord, k *isam.Key, byRecord bool) []ConvertRecord {
	values := make([][]byte, len(records))
	for i := range records {
		values[i] = paddedValue(k, records[i].Data)
	}

	order := make([]int, len(records))
	for i := range order {
		order[i] = i
	}

	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]

		if c := k.Compare(values[a], values[b]); c != 0 {
			return c < 0
		}

		return byRecord && bytes.Compare(records[a].Data, records[b].Data) < 0
	})

	out := make([]ConvertRecord, len(records))
	for i, at := range order {
		out[i] = records[at]
	}

	return out
}

// paddedValue is a key's value in a record, zeros standing for what a short
// record lacks.
func paddedValue(k *isam.Key, record []byte) []byte {
	value := make([]byte, 0, k.Size)

	for _, s := range k.Segments {
		part := make([]byte, s.Size)
		if s.Position < len(record) {
			copy(part, record[s.Position:])
		}

		value = append(value, part...)
	}

	return value
}
