package rms

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/lck"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// DCL's READ and WRITE on an indexed file (docs/PHASE-88 - keyed index
// files.md, subtask 9), and READ's /KEY, /INDEX, /MATCH, and /DELETE on any
// file.
//
// VMS's DCL reads and writes a file through a RAB of its own, as any program
// does, and so does govax's here: a DCLFile that needs one gets a small memory
// of its own, unmapped, holding one RAB, a key buffer, and a record buffer
// (dclMemory), and each operation fills the RAB in and calls the service a
// program would call, SysGet, SysPut, SysUpdate, or SysDelete. The record
// services on an indexed file (indexedrecords.go) are then DCL's too, with
// every status, the stream's key of reference and place, and record locks, and
// nothing about indexed files is written twice. A sequential file's READ and
// WRITE stay DCLFile's own (dclfile.go), as they were before; only a READ with
// /KEY or /DELETE goes through the RAB on one.
//
// What the DCL Dictionary says, as VMS 7.1 showed it in round 4
// (testdata/idx88's IDX88D.COM, whose log TestIdx88DOracle holds govax's to):
//
//   - READ/KEY=string is a $GET by key (RAB$B_RAC KEY), the string's bytes
//     the key, RAB$B_KSZ its length: shorter than a string key is a generic
//     search, and a longer or empty one RMS$_KSZ. "Binary and integer keys
//     are not allowed", the manual says, but a string the key's size is
//     taken as its bytes, and any other length is RMS$_KSZ (D4).
//   - /MATCH=EQ, GE, or GT is ROP's nothing, KGE, or KGT; EQ without /MATCH.
//     A descending key's GT is the next lower value (D4).
//   - /INDEX=n is RAB$B_KRF, and the value stays the file's for later READs
//     until another /INDEX, even after a READ it failed (D4): a READ without
//     /KEY is a $GET in sequence, which follows the key of reference the last
//     keyed READ made the stream's.
//   - READ/DELETE is the $GET, then a $DELETE of the record it read; the
//     symbol gets the record even when the $DELETE fails, and $STATUS is the
//     $DELETE's (D5, D7). On a file opened only to read that's RMS$_FAC, and
//     on a sequential file RMS$_IOP.
//   - /NOLOCK is ROP NLK and RRL: the record isn't locked, and is read even
//     if another stream has it locked (D8). A READ without it locks the
//     record when the file is shared for writing, and the next operation on
//     the stream gives it up, RMS's default; so does WRITE/UPDATE (D8, the
//     record free for the other stream at once), and a WRITE/UPDATE of a
//     record READ/NOLOCK read is RMS$_RNL.
//   - WRITE is a $PUT by key (RAC KEY), so records may come in any order,
//     and $STATUS is the $PUT's success status, RMS$_OK_DUP for a record
//     whose alternate key another has (D1, D9). After OPEN/APPEND ($CONNECT
//     with ROP EOF) the first must be past the file's last key (RMS$_SEQ,
//     D10), and those after it may come in any order.
//   - WRITE/UPDATE is an $UPDATE of the record READ read, its status the
//     $UPDATE's too (D6).
//   - An indexed file opened to write may also delete records: its stream has
//     FAB$V_DEL beside GET, PUT, and UPD (the DCL Dictionary's READ/DELETE
//     needs only /READ/WRITE; D7).

// The layout of a DCLFile's memory: its RAB, the key buffer (a key is at most
// 255 bytes), and the record buffer, which holds the longest record RMS
// allows.
const (
	dclRABAddr    = 0x200
	dclKeyAddr    = 0x300
	dclKeySize    = 256
	dclBufAddr    = 0x400
	dclBufSize    = 32767
	dclMemorySize = 0x8800
)

// RAB fields only this file sets.
var (
	rabBID = rabOffset("BID")
	rabBLN = rabOffset("BLN")

	rabBIDValue = byte(vmsConst("RAB$C_BID"))
	rabBLNValue = byte(vmsConst("RAB$C_BLN"))

	ropNLKBit = vmsConst("RAB$M_NLK")
	ropRRLBit = vmsConst("RAB$M_RRL")
)

// DCLMatch is READ's /MATCH.
type DCLMatch int

// READ's /MATCH options: EQ (the default), GE, GT.
const (
	MatchEqual DCLMatch = iota
	MatchEqualOrNext
	MatchNext
)

// DCLRead is a READ's qualifiers.
type DCLRead struct {
	// Key is /KEY's string; HasKey says /KEY was given.
	Key    []byte
	HasKey bool

	// Index is /INDEX's key of reference, if HasIndex.
	Index    int
	HasIndex bool

	Match  DCLMatch
	Delete bool
	NoLock bool
}

// plain reports whether r is a READ with none of the indexed qualifiers that
// change what it does.
func (r DCLRead) plain() bool { return !r.HasKey && !r.Delete }

// dclMemory gives f's context the memory its RAB is in, made at first need:
// physical addresses, MAPEN off, so nothing is translated.
func (f *DCLFile) dclMemory() {
	if f.ctx.Mem != nil {
		return
	}

	f.ctx.Mem = vm.NewMemory(dclMemorySize)
	f.ctx.CPU = registers.New()
}

// rabRequest is what one operation puts in the RAB besides its stream.
type rabRequest struct {
	rac    byte
	rop    uint32
	krf    byte
	key    []byte
	record []byte // a $PUT's or $UPDATE's
}

// callRAB fills f's RAB in for req and calls service with it, returning the
// status RMS left in RAB$L_STS and, for a $GET, the record read.
func (f *DCLFile) callRAB(service func(*Context, []uint32) (uint32, error), req rabRequest) ([]byte, uint32) {
	f.dclMemory()

	ctx := f.ctx

	// The stream is in the context's own table only for the call: once DCL
	// has made the file a process-permanent one, the process's table has it
	// (ppfstream.go's AddDCLFile).
	isi, ok := ctx.Files.Find(f.handle)
	if !ok {
		isi = ctx.Files.Alloc(f.handle)
		defer ctx.Files.Release(isi)
	}

	rab := make([]byte, rabBLNValue)
	rab[rabBID] = rabBIDValue
	rab[rabBLN] = rabBLNValue
	binary.LittleEndian.PutUint16(rab[rabISI:], isi)
	rab[rabRAC] = req.rac
	binary.LittleEndian.PutUint32(rab[rabROP:], req.rop)
	rab[rabKRF] = req.krf
	rab[rabKSZ] = byte(len(req.key))
	binary.LittleEndian.PutUint32(rab[rabKBF:], dclKeyAddr)
	binary.LittleEndian.PutUint32(rab[rabUBF:], dclBufAddr)
	binary.LittleEndian.PutUint16(rab[rabUSZ:], dclBufSize)
	binary.LittleEndian.PutUint32(rab[rabRBF:], dclBufAddr)
	binary.LittleEndian.PutUint16(rab[rabRSZ:], uint16(len(req.record)))

	if err := ctx.storeBytes(dclRABAddr, rab); err != nil {
		return nil, rmsDeviceError
	}

	if err := ctx.storeBytes(dclKeyAddr, req.key[:min(len(req.key), dclKeySize)]); err != nil {
		return nil, rmsDeviceError
	}

	if err := ctx.storeBytes(dclBufAddr, req.record); err != nil {
		return nil, rmsDeviceError
	}

	if _, err := service(ctx, []uint32{dclRABAddr}); err != nil {
		return nil, rmsDeviceError
	}

	sts, err := ctx.loadLongword(dclRABAddr + rabSTS)
	if err != nil || sts&1 == 0 {
		return nil, sts
	}

	rsz, err := ctx.loadWord(dclRABAddr + rabRSZ)
	if err != nil {
		return nil, rmsDeviceError
	}

	record, err := ctx.loadBytes(dclBufAddr, int(rsz))
	if err != nil {
		return nil, rmsDeviceError
	}

	return record, sts
}

// connectRAB connects an indexed file's stream as $CONNECT would, at the
// primary key's first record, or with appending past the last (ROP EOF).
func (f *DCLFile) connectRAB(appending bool) uint32 {
	f.dclMemory()

	var rop uint32
	if appending {
		rop = ropEOF
	}

	if err := f.ctx.storeBytes(dclRABAddr, make([]byte, rabBLNValue)); err != nil {
		return rmsDeviceError
	}

	sts, err := f.ctx.connectIndexed(dclRABAddr, f.handle, rop)
	if err != nil {
		return rmsDeviceError
	}

	return sts
}

// Read reads a record as READ does with r's qualifiers (see the top of this
// file), returning it with its success status, or nil and the status for why
// not; with r.Delete, the record and the $DELETE's status, whether it worked
// or not.
func (f *DCLFile) Read(r DCLRead) ([]byte, uint32) {
	if r.HasIndex {
		f.index = r.Index
	}

	if f.host != nil || (r.plain() && !f.handle.indexedRecords()) {
		return f.Get()
	}

	req := rabRequest{rac: racSeq, krf: byte(f.index)}

	if r.NoLock {
		req.rop |= ropNLKBit | ropRRLBit
	}

	if r.HasKey {
		req.rac, req.key = racKey, r.Key

		switch r.Match {
		case MatchEqualOrNext:
			req.rop |= ropKGE
		case MatchNext:
			req.rop |= ropKGT
		}
	}

	record, sts := f.callRAB(SysGet, req)

	// A VFC record's fixed control isn't part of what READ reads (RAB$L_RHB,
	// which DCL's RAB has no place for here).
	if n := f.handle.vfcSize(); n > 0 && len(record) >= n {
		record = record[n:]
	}

	if sts&1 == 0 || !r.Delete {
		return record, sts
	}

	// A delete that fails still returns the record, which READ puts in its
	// symbol (VMS 7.1, round 4's D5); one that works returns its own status.
	_, del := f.callRAB(SysDelete, rabRequest{})
	if record == nil {
		record = []byte{}
	}

	return record, del
}

// Indexed reports whether f is an indexed file.
func (f *DCLFile) Indexed() bool { return f.handle != nil && f.handle.indexedRecords() }

// putIndexed is Put on an indexed file: a $PUT by key.
func (f *DCLFile) putIndexed(record []byte) uint32 {
	_, sts := f.callRAB(SysPut, rabRequest{rac: racKey, record: record})

	return sts
}

// updateIndexed is Update on an indexed file: an $UPDATE of the record READ
// read last.
func (f *DCLFile) updateIndexed(record []byte) uint32 {
	_, sts := f.callRAB(SysUpdate, rabRequest{record: record})

	return sts
}

// DCLLocks are the lock database and process an open DCL file's records are
// locked in (DCLAccess.Locks): nil Manager for none.
type DCLLocks struct {
	Manager *lck.Manager
	PID     uint32
	CanLock func() bool
}
