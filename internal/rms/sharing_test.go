package rms

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/ods2/volume"
)

// These tests are Phase 47's: two VAX processes using one file on one
// volume. Each "process" is its own Context (its own memory and file
// table) over one shared MountTable, which is exactly how
// internal/corevms gives every process its RMS: the volumes are system
// state, the open files are the process's.

// The FAB$B_FAC and FAB$B_SHR bits the tests use, and RAB$M_EOF, which
// asks $CONNECT to position the stream at the end of the file.
var (
	shrPut = byte(vmsConst("FAB$M_SHRPUT"))
	shrGet = byte(vmsConst("FAB$M_SHRGET"))
	ropEOF = vmsConst("RAB$M_EOF")
	fabSHR = fabOffset("SHR")
)

// sharer is one process in a sharing test: its RMS Context, with one FAB
// and one RAB at the fixture's usual addresses.
type sharer struct {
	t    *testing.T
	name string
	ctx  *Context
}

// newSharers returns n processes sharing one MountTable, with DUA0
// mounted writable on a fresh volume.
func newSharers(t *testing.T, n int) ([]*sharer, *MountTable) {
	t.Helper()

	mounts := NewMountTable()
	if err := mounts.Mount("DUA0", newTestVolumeFile(t, "SHARE"), true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	logicals := newTestLogicals(t)

	var out []*sharer

	for i := range n {
		out = append(out, &sharer{
			t:    t,
			name: string(rune('A' + i)),
			ctx: &Context{
				Mem:      vm.NewMemory(1 << 20),
				CPU:      vax.New(),
				Mounts:   mounts,
				Files:    NewFileTable(nil),
				Logicals: logicals,
			},
		})
	}

	return out, mounts
}

// open runs $OPEN (or $CREATE, if create) for DUA0:name with the given
// access and sharing, a variable-length record format, and, if that
// succeeded, $CONNECT with rop. It returns the first failing status, or
// RMS$_NORMAL (or RMS$_CREATED's success) if both succeeded.
func (s *sharer) open(name string, fac, shr byte, create bool, rop uint32) uint32 {
	s.t.Helper()

	ctx := s.ctx
	newFAB(s.t, ctx, "DUA0:"+name)
	putByte(s.t, ctx, testFabAddr+fabFAC, fac)
	putByte(s.t, ctx, testFabAddr+fabSHR, shr)
	putByte(s.t, ctx, testFabAddr+fabRFM, byte(vmsConst("FAB$C_VAR")))
	putWord(s.t, ctx, testFabAddr+fabMRS, 0)

	service := SysOpen
	if create {
		service = SysCreate
	}

	r0, err := service(ctx, []uint32{testFabAddr})
	if err != nil {
		s.t.Fatalf("%s: open %s: %v", s.name, name, err)
	}

	if r0&1 == 0 {
		return r0
	}

	putRAB(s.t, ctx, testFabAddr)
	putByte(s.t, ctx, testRabAddr+rabRAC, racSeq)
	putLongwordAt(s.t, ctx, testRabAddr+rabROP, rop)

	r0, err = SysConnect(ctx, []uint32{testRabAddr})
	if err != nil {
		s.t.Fatalf("%s: connect %s: %v", s.name, name, err)
	}

	return r0
}

// mustOpen is open, failing the test unless it succeeds.
func (s *sharer) mustOpen(name string, fac, shr byte, create bool, rop uint32) {
	s.t.Helper()

	if r0 := s.open(name, fac, shr, create, rop); r0&1 == 0 {
		s.t.Fatalf("%s: open %s: status %#x", s.name, name, r0)
	}
}

// put writes one record, failing the test unless $PUT succeeds.
func (s *sharer) put(record string) {
	s.t.Helper()

	putRecord(s.t, s.ctx, testRabAddr, []byte(record))

	r0, err := SysPut(s.ctx, []uint32{testRabAddr})
	if err != nil || r0&1 == 0 {
		s.t.Fatalf("%s: put %q: status %#x, %v", s.name, record, r0, err)
	}
}

// get reads one record, returning it and $GET's status.
func (s *sharer) get() (string, uint32) {
	s.t.Helper()

	putLongwordAt(s.t, s.ctx, testRabAddr+rabUBF, testRecordAddr)
	putWord(s.t, s.ctx, testRabAddr+rabUSZ, 1024)

	r0, err := SysGet(s.ctx, []uint32{testRabAddr})
	if err != nil {
		s.t.Fatalf("%s: get: %v", s.name, err)
	}

	if r0&1 == 0 {
		return "", r0
	}

	n := readWord(s.t, s.ctx, testRabAddr+rabRSZ)
	buf := make([]byte, n)

	for i := range buf {
		buf[i] = readByte(s.t, s.ctx, testRecordAddr+uint32(i))
	}

	return string(buf), r0
}

// close runs $CLOSE, failing the test unless it succeeds.
func (s *sharer) close() {
	s.t.Helper()

	r0, err := SysClose(s.ctx, []uint32{testFabAddr})
	if err != nil || r0&1 == 0 {
		s.t.Fatalf("%s: close: status %#x, %v", s.name, r0, err)
	}
}

// records reads every record of DUA0:name with a process of its own.
func records(t *testing.T, mounts *MountTable, name string) []string {
	t.Helper()

	r := &sharer{t: t, name: "reader", ctx: &Context{
		Mem: vm.NewMemory(1 << 20), CPU: vax.New(), Mounts: mounts,
		Files: NewFileTable(nil), Logicals: newTestLogicals(t),
	}}
	r.mustOpen(name, facGet, shrGet, false, 0)

	var out []string

	for {
		rec, sts := r.get()
		if sts == rmsEOF {
			break
		}

		if sts&1 == 0 {
			t.Fatalf("reading %s: status %#x after %d records", name, sts, len(out))
		}

		out = append(out, rec)
	}

	r.close()

	return out
}

// writeFile creates DUA0:name holding records, and closes it.
func writeFile(t *testing.T, mounts *MountTable, name string, recs ...string) {
	t.Helper()

	w := &sharer{t: t, name: "writer", ctx: &Context{
		Mem: vm.NewMemory(1 << 20), CPU: vax.New(), Mounts: mounts,
		Files: NewFileTable(nil), Logicals: newTestLogicals(t),
	}}
	w.mustOpen(name, facPut, 0, true, 0)

	for _, r := range recs {
		w.put(r)
	}

	w.close()
}

// checkVolume fails the test unless the volume's storage bitmap agrees
// with its file headers: no block allocated and lost, none in use and
// free.
func checkVolume(t *testing.T, mounts *MountTable) {
	t.Helper()

	vol, _ := mounts.Lookup("DUA0")

	// The bitmap is cached in memory; write it out first so the analysis
	// reads what the volume would hold after a DISMOUNT.
	bm, err := vol.Devices[0].Bitmap()
	if err != nil {
		t.Fatal(err)
	}

	if err := bm.Flush(); err != nil {
		t.Fatal(err)
	}

	report, err := volume.AnalyzeDisk(vol.Devices[0])
	if err != nil {
		t.Fatalf("AnalyzeDisk: %v", err)
	}

	for _, d := range report.Discrepancies {
		t.Errorf("volume: %s", d)
	}
}

// wantRecords fails the test unless got is want.
func wantRecords(t *testing.T, got []string, want ...string) {
	t.Helper()

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("records = %q\nwant      %q", got, want)
	}
}

// numbered returns n records, prefix followed by a number, each padded
// to size bytes.
func numbered(prefix string, n, size int) []string {
	var out []string

	for i := range n {
		r := fmt.Sprintf("%s%03d", prefix, i)
		out = append(out, r+strings.Repeat(".", size-len(r)))
	}

	return out
}

// pending skips a test that shows a sharing defect Phase 47 hasn't fixed
// yet: subtask is the one that fixes it, and removes the call.
func pending(t *testing.T, subtask int) {
	t.Helper()
	t.Skipf("fixed by Phase 47, subtask %d", subtask)
}

// TestSharing_accessConflict: a file opened for writing with no sharing
// (FAB$B_SHR 0, which for a writer means FAB$V_NIL) can't be opened by
// anyone else; RMS$_FLK.
func TestSharing_accessConflict(t *testing.T) {
	pending(t, 3)

	p, _ := newSharers(t, 2)
	a, b := p[0], p[1]

	a.mustOpen("LOCKED.DAT", facPut, 0, true, 0)

	if r0 := b.open("LOCKED.DAT", facGet, shrGet, false, 0); r0 != rmsFileLocked {
		t.Errorf("second open: status %#x, want RMS$_FLK (%#x)", r0, rmsFileLocked)
	}

	a.close()
}

// TestSharing_twoWritersAppend: two processes with write sharing both
// appending to one sequential file. Each $PUT goes at the end of the file
// as it is then, so the records interleave in the order they were put,
// and none is lost (RMS manual, "Inserting Records into Sequential
// Files").
func TestSharing_twoWritersAppend(t *testing.T) {
	pending(t, 5)

	p, mounts := newSharers(t, 2)
	a, b := p[0], p[1]

	a.mustOpen("SHARE.DAT", facPut, shrGet|shrPut, true, 0)
	a.put("A1")

	b.mustOpen("SHARE.DAT", facPut, shrGet|shrPut, false, ropEOF)
	b.put("B1")
	a.put("A2")
	b.put("B2")
	b.close()
	a.put("A3")
	a.close()

	wantRecords(t, records(t, mounts, "SHARE.DAT"), "A1", "B1", "A2", "B2", "A3")
	checkVolume(t, mounts)
}

// TestSharing_extendThenClose: one process extends a file another has
// open for writing; when the second closes it, the first's extension
// (its blocks and its records) must survive.
func TestSharing_extendThenClose(t *testing.T) {
	pending(t, 4)

	p, mounts := newSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "GROW.DAT", "R0")

	a.mustOpen("GROW.DAT", facPut|facGet, shrGet|shrPut, false, ropEOF)

	more := numbered("B", 40, 100)

	b.mustOpen("GROW.DAT", facPut, shrGet|shrPut, false, ropEOF)

	for _, r := range more {
		b.put(r)
	}

	b.close()
	a.close()

	wantRecords(t, records(t, mounts, "GROW.DAT"), append([]string{"R0"}, more...)...)
	checkVolume(t, mounts)
}

// TestSharing_readerSeesAppend: a reader sharing a file with a writer
// reads records the writer appended after the reader opened it.
func TestSharing_readerSeesAppend(t *testing.T) {
	pending(t, 5)

	p, mounts := newSharers(t, 2)
	a, b := p[0], p[1]

	writeFile(t, mounts, "LOG.DAT", "R0")

	a.mustOpen("LOG.DAT", facGet, shrGet|shrPut, false, 0)

	if rec, sts := a.get(); rec != "R0" || sts&1 == 0 {
		t.Fatalf("first get = %q, %#x", rec, sts)
	}

	if _, sts := a.get(); sts != rmsEOF {
		t.Fatalf("second get: status %#x, want RMS$_EOF", sts)
	}

	b.mustOpen("LOG.DAT", facPut, shrGet|shrPut, false, ropEOF)
	b.put("B1")

	if rec, sts := a.get(); rec != "B1" || sts&1 == 0 {
		t.Errorf("get after the append = %q, %#x; want \"B1\"", rec, sts)
	}

	b.close()
	a.close()

	wantRecords(t, records(t, mounts, "LOG.DAT"), "R0", "B1")
}

// TestSharing_appendToExisting: one process, no sharing: $CONNECT with
// RAB$V_EOF positions the stream at the end of the file, and $PUT
// appends there.
func TestSharing_appendToExisting(t *testing.T) {
	pending(t, 4)

	p, mounts := newSharers(t, 1)

	writeFile(t, mounts, "MORE.DAT", "R0", "R1")

	p[0].mustOpen("MORE.DAT", facPut, 0, false, ropEOF)
	p[0].put("R2")
	p[0].close()

	wantRecords(t, records(t, mounts, "MORE.DAT"), "R0", "R1", "R2")
	checkVolume(t, mounts)
}

// TestSharing_deleteWhileOpen: a file deleted while a process has it
// open leaves the directory at once, but stays readable by that process
// until it closes the file; then it's gone, and its space with it.
func TestSharing_deleteWhileOpen(t *testing.T) {
	pending(t, 3)

	p, mounts := newSharers(t, 1)
	a := p[0]

	writeFile(t, mounts, "DOOMED.DAT", "R0", "R1")

	a.mustOpen("DOOMED.DAT", facGet, shrGet, false, 0)

	got, err := mounts.ACPDelete("DUA0", ACPDeleteRequest{Directory: mfd, Name: "DOOMED.DAT;1", DeleteFile: true})
	if err != nil {
		t.Fatalf("ACPDelete: %v", err)
	}

	if !got.Deferred {
		t.Error("the deletion of an open file wasn't deferred")
	}

	// A new file, which would reuse the deleted file's header and
	// blocks if they had been freed.
	writeFile(t, mounts, "NEW.DAT", numbered("N", 20, 100)...)

	for _, want := range []string{"R0", "R1"} {
		if rec, sts := a.get(); rec != want || sts&1 == 0 {
			t.Errorf("get = %q, %#x; want %q", rec, sts, want)
		}
	}

	a.close()

	if !gone(mounts, got.FID) {
		t.Error("the file is still there after its last close")
	}

	checkVolume(t, mounts)
}
