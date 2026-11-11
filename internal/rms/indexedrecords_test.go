package rms

import (
	"fmt"
	"testing"
)

// The record services on an indexed file where the probe's run doesn't reach
// (Phase 88, subtask 8). The probe's own cases, 120 to 153, are
// internal/console's TestIdx88Oracle.

// keyBufferAddr is where these tests put a search key.
const keyBufferAddr uint32 = 0x7000

// recordsFixture makes DUA0:R.IDX, a file of variable records with a string
// primary key of 8 bytes at 0 and an alternate with duplicates at 8, opened
// with access fac and connected.
func recordsFixture(t *testing.T, fac byte) *createFixture {
	t.Helper()

	f := newCreateFixture(t, true)
	putXABKey(t, f.ctx, xabKey0Addr, xabKey1Addr, 0, 0, 0, 8, 0)
	putXABKey(t, f.ctx, xabKey1Addr, 0, 1, 0, 8, 4, xabFlagDUP)
	indexedFAB(t, f.ctx, "DUA0:R.IDX", 100, xabKey0Addr)
	putByte(t, f.ctx, testFabAddr+fabFAC, fac)

	if r0 := run(t, f.ctx, SysCreate, testFabAddr); r0&1 == 0 {
		t.Fatalf("$CREATE: %s", StatusName(r0))
	}

	connectRAB(t, f.ctx, testFabAddr)

	if sts := readLongword(t, f.ctx, testRabAddr+rabSTS); sts != rmsNormal {
		t.Fatalf("$CONNECT: %s", StatusName(sts))
	}

	return f
}

// putN puts record n: its key P000000n, and an alternate of n/2.
func putN(t *testing.T, f *createFixture, n int) uint32 {
	t.Helper()

	putByte(t, f.ctx, testRabAddr+rabRAC, racKey)
	putLongwordAt(t, f.ctx, testRabAddr+rabROP, 0)
	putRecord(t, f.ctx, testRabAddr, fmt.Appendf(nil, "P%07dA%03d the rest of record %d", n, n/2, n))

	return run(t, f.ctx, SysPut, testRabAddr)
}

// getKey is a keyed $GET (or $FIND) of key krf's value, by match bits rop.
func getKey(t *testing.T, f *createFixture, service func(*Context, []uint32) (uint32, error), krf byte, value string, rop uint32) uint32 {
	t.Helper()

	for i, b := range []byte(value) {
		putByte(t, f.ctx, keyBufferAddr+uint32(i), b)
	}

	putByte(t, f.ctx, testRabAddr+rabRAC, racKey)
	putByte(t, f.ctx, testRabAddr+rabKRF, krf)
	putLongwordAt(t, f.ctx, testRabAddr+rabKBF, keyBufferAddr)
	putByte(t, f.ctx, testRabAddr+rabKSZ, byte(len(value)))
	putLongwordAt(t, f.ctx, testRabAddr+rabROP, rop)
	putLongwordAt(t, f.ctx, testRabAddr+rabUBF, testRecordAddr)
	putWord(t, f.ctx, testRabAddr+rabUSZ, 512)

	return run(t, f.ctx, service, testRabAddr)
}

// getSeq is a sequential $GET, returning the status and the record's key.
func getSeq(t *testing.T, f *createFixture) (uint32, string) {
	t.Helper()

	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putLongwordAt(t, f.ctx, testRabAddr+rabROP, 0)
	putLongwordAt(t, f.ctx, testRabAddr+rabUBF, testRecordAddr)
	putWord(t, f.ctx, testRabAddr+rabUSZ, 512)

	r0 := run(t, f.ctx, SysGet, testRabAddr)
	if r0&1 == 0 {
		return r0, ""
	}

	key, err := f.ctx.loadFixedString(testRecordAddr, 8)
	if err != nil {
		t.Fatal(err)
	}

	return r0, key
}

// TestIndexedAccessRefused is each record service on a stream without the
// access it needs: RMS$_FAC, as on the other organizations.
func TestIndexedAccessRefused(t *testing.T) {
	f := recordsFixture(t, facPut|facGet)

	if r0 := putN(t, f, 1); r0 != rmsNormal {
		t.Fatalf("$PUT: %s", StatusName(r0))
	}

	if r0 := getKey(t, f, SysGet, 0, "P0000001", 0); r0 != rmsNormal {
		t.Fatalf("$GET: %s", StatusName(r0))
	}

	for name, service := range map[string]func(*Context, []uint32) (uint32, error){
		"$UPDATE": SysUpdate, "$DELETE": SysDelete, "$TRUNCATE": SysTruncate,
	} {
		if r0 := run(t, f.ctx, service, testRabAddr); r0 != rmsFACNotAllowed {
			t.Errorf("%s: %s, want RMS$_FAC", name, StatusName(r0))
		}
	}

	// UIF over an existing record needs update access too.
	putLongwordAt(t, f.ctx, testRabAddr+rabROP, ropUIF)
	putByte(t, f.ctx, testRabAddr+rabRAC, racKey)

	if r0 := run(t, f.ctx, SysPut, testRabAddr); r0 != rmsFACNotAllowed {
		t.Errorf("$PUT with UIF: %s, want RMS$_FAC", StatusName(r0))
	}
}

// TestIndexedFindThenChange is a $FIND by key, then $UPDATE and $DELETE of the
// record it found, and the sequence going on after the deleted record among
// its duplicates, which isam alone couldn't do once it had gone.
func TestIndexedFindThenChange(t *testing.T) {
	f := recordsFixture(t, facPut|facGet|facUpd|facDel)

	for _, n := range []int{10, 11, 12, 13, 20} {
		if r0 := putN(t, f, n); r0&1 == 0 {
			t.Fatalf("$PUT %d: %s", n, StatusName(r0))
		}
	}

	// Key 1's A005 is 10 and 11, A006 12 and 13.
	if r0 := getKey(t, f, SysFind, 1, "A006", 0); r0 != rmsNormal {
		t.Fatalf("$FIND: %s", StatusName(r0))
	}

	putRecord(t, f.ctx, testRabAddr, []byte("P0000012A006 changed"))

	if r0 := run(t, f.ctx, SysUpdate, testRabAddr); r0 != rmsNormal {
		t.Fatalf("$UPDATE: %s", StatusName(r0))
	}

	if r0 := run(t, f.ctx, SysUpdate, testRabAddr); r0 != rmsNoCurrent {
		t.Errorf("a second $UPDATE: %s, want RMS$_CUR", StatusName(r0))
	}

	if r0 := getKey(t, f, SysGet, 1, "A005", 0); r0 != rmsNormal {
		t.Fatalf("$GET: %s", StatusName(r0))
	}

	if r0 := run(t, f.ctx, SysDelete, testRabAddr); r0 != rmsNormal {
		t.Fatalf("$DELETE: %s", StatusName(r0))
	}

	if r := readLongword(t, f.ctx, testRabAddr+rabRFA); r != 0 {
		t.Errorf("RFA after $DELETE: %d, want 0", r)
	}

	// Its duplicate, then the next value's two.
	for _, want := range []string{"P0000011", "P0000012", "P0000013", "P0000020"} {
		if r0, key := getSeq(t, f); r0 != rmsNormal || key != want {
			t.Errorf("$GET: %s %q, want %q", StatusName(r0), key, want)
		}
	}

	if r0, _ := getSeq(t, f); r0 != rmsEOF {
		t.Errorf("$GET at the end: %s, want RMS$_EOF", StatusName(r0))
	}
}

// TestIndexedGetByRFA is a $GET by RFA: the record, the stream then on the
// primary key, and RMS$_DEL once the record has gone.
func TestIndexedGetByRFA(t *testing.T) {
	f := recordsFixture(t, facPut|facGet|facDel)

	for _, n := range []int{1, 2, 3} {
		putN(t, f, n)
	}

	getKey(t, f, SysGet, 1, "A001", 0) // record 2, by key 1

	vbn := readLongword(t, f.ctx, testRabAddr+rabRFA)
	id := readWord(t, f.ctx, testRabAddr+rabRFA+4)

	getKey(t, f, SysGet, 0, "P0000001", 0)
	putLongwordAt(t, f.ctx, testRabAddr+rabRFA, vbn)
	putWord(t, f.ctx, testRabAddr+rabRFA+4, id)
	putByte(t, f.ctx, testRabAddr+rabRAC, racRFA)

	if r0 := run(t, f.ctx, SysGet, testRabAddr); r0 != rmsNormal {
		t.Fatalf("$GET by RFA: %s", StatusName(r0))
	}

	if r0, key := getSeq(t, f); key != "P0000003" {
		t.Errorf("$GET after the RFA access: %s %q, want P0000003 by the primary key", StatusName(r0), key)
	}

	getKey(t, f, SysGet, 0, "P0000002", 0)
	run(t, f.ctx, SysDelete, testRabAddr)
	putLongwordAt(t, f.ctx, testRabAddr+rabRFA, vbn)
	putWord(t, f.ctx, testRabAddr+rabRFA+4, id)
	putByte(t, f.ctx, testRabAddr+rabRAC, racRFA)

	if r0 := run(t, f.ctx, SysGet, testRabAddr); r0 != rmsRecordDeleted {
		t.Errorf("$GET by a deleted record's RFA: %s, want RMS$_DEL", StatusName(r0))
	}
}

// TestIndexedEndOfFile is the header's end of file: 0 while the records fit
// the first allocation, and the block after the allocation once RMS has
// extended the file, as VMS's DIRECTORY/FULL showed (testdata/idx88's
// vax/idx88dir.txt: "Size: 9/9").
func TestIndexedEndOfFile(t *testing.T) {
	f := recordsFixture(t, facPut|facGet)

	h, _ := f.ctx.Files.Lookup(readWord(t, f.ctx, testFabAddr+fabIFI))
	before := h.File.Blocks()

	for n := 1; h.File.Blocks() == before; n++ {
		if r0 := putN(t, f, n); r0&1 == 0 {
			t.Fatalf("$PUT %d: %s", n, StatusName(r0))
		}

		if n > 500 {
			t.Fatal("the file never grew")
		}
	}

	run(t, f.ctx, SysClose, testFabAddr)

	attr, err := NewSession(f.ctx.Mounts).FileAttributes("DUA0:R.IDX")
	if err != nil {
		t.Fatal(err)
	}

	if attr.EndBlock != attr.Allocated+1 {
		t.Errorf("end of file %d over %d blocks, want the block after them", attr.EndBlock, attr.Allocated)
	}
}

// TestIndexedRewindAndConnect is $REWIND's and $CONNECT's key of reference:
// RMS$_KRF past the file's keys (unconfirmed), and the sequence by the key
// $REWIND names.
func TestIndexedRewindAndConnect(t *testing.T) {
	f := recordsFixture(t, facPut|facGet)

	for _, n := range []int{5, 2, 9} {
		putN(t, f, n)
	}

	putByte(t, f.ctx, testRabAddr+rabKRF, 2)

	if r0 := run(t, f.ctx, SysRewind, testRabAddr); r0 != rmsInvalidKeyRef {
		t.Errorf("$REWIND by key 2: %s, want RMS$_KRF", StatusName(r0))
	}

	putByte(t, f.ctx, testRabAddr+rabKRF, 1)

	if r0 := run(t, f.ctx, SysRewind, testRabAddr); r0 != rmsNormal {
		t.Fatalf("$REWIND by key 1: %s", StatusName(r0))
	}

	for _, want := range []string{"P0000002", "P0000005", "P0000009"} {
		if r0, key := getSeq(t, f); key != want {
			t.Errorf("$GET: %s %q, want %q", StatusName(r0), key, want)
		}
	}
}
