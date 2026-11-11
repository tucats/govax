package rms

import (
	"bytes"
	"testing"
)

// TestFileTable_noPreassignedIFIs confirms a new FileTable has no files,
// with a terminal or without: no IFI is preassigned, so a FAB that was
// never opened is RMS$_IFI to $CONNECT whatever its FAB$W_IFI says (IFI 1
// was once the terminal, a stray a program could connect to).
func TestFileTable_noPreassignedIFIs(t *testing.T) {
	for _, ft := range []*FileTable{NewFileTable(nil), NewFileTable(&bytes.Buffer{})} {
		for ifi := range uint16(firstUserIFI) {
			if _, ok := ft.Lookup(ifi); ok {
				t.Errorf("Lookup(%d) found a file in a new table", ifi)
			}
		}
	}

	f := newCreateFixture(t, true)
	f.ctx.Console = &bytes.Buffer{}
	f.ctx.Files = NewFileTable(f.ctx.Console)

	putWord(t, f.ctx, testFabAddr+fabIFI, 1)
	putRAB(t, f.ctx, testFabAddr)

	r0, err := SysConnect(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysConnect: %v", err)
	}

	if r0 != rmsInvalidIFI {
		t.Errorf("$CONNECT with FAB$W_IFI 1, never opened: r0 = %08X, want RMS$_IFI (%08X)", r0, rmsInvalidIFI)
	}
}

// TestFileTable_rundownEndsOpenLine confirms Rundown ends a terminal line
// a prompt left open, on the table's own terminal output.
func TestFileTable_rundownEndsOpenLine(t *testing.T) {
	var out bytes.Buffer

	ft := NewFileTable(&out)
	ft.lineOpen = true

	if _, err := ft.Rundown(); err != nil {
		t.Fatalf("Rundown: %v", err)
	}

	if out.String() != "\n" || ft.lineOpen {
		t.Errorf("after Rundown: output %q, line open %v; want a newline, closed", out.String(), ft.lineOpen)
	}
}

// TestFileTable_allocSkipsReservedSlots confirms Alloc never hands out
// an IFI below firstUserIFI.
func TestFileTable_allocSkipsReservedSlots(t *testing.T) {
	ft := NewFileTable(nil)

	id := ft.Alloc(&FileHandle{})
	if id < firstUserIFI {
		t.Errorf("Alloc returned reserved IFI %d, want an IFI >= %d", id, firstUserIFI)
	}
}

// TestFileTable_allocUnique confirms repeated Alloc calls never hand out
// the same IFI twice while nothing has been Released.
func TestFileTable_allocUnique(t *testing.T) {
	ft := NewFileTable(nil)

	seen := make(map[uint16]bool)

	for range 10 {
		id := ft.Alloc(&FileHandle{})
		if seen[id] {
			t.Fatalf("Alloc returned duplicate IFI %d", id)
		}

		seen[id] = true
	}
}

// TestFileTable_releaseFreesSlotForReuse confirms Release actually
// forgets the entry (a later Lookup fails) and that Alloc will hand the
// freed slot back out again — specifically the *lowest* free slot,
// matching Alloc's own doc comment on why it always scans from
// firstUserIFI rather than remembering a monotonically increasing
// cursor.
func TestFileTable_releaseFreesSlotForReuse(t *testing.T) {
	ft := NewFileTable(nil)

	id := ft.Alloc(&FileHandle{})
	ft.Release(id)

	if _, ok := ft.Lookup(id); ok {
		t.Errorf("Lookup(%d) after Release = found, want not found", id)
	}

	if again := ft.Alloc(&FileHandle{}); again != id {
		t.Errorf("Alloc after Release returned %d, want the freed slot %d back", again, id)
	}
}

// TestFileTable_releaseUnknownIFIIsNoop confirms Release on an IFI that
// was never allocated (including a reserved 0-3 slot) doesn't panic —
// SYS$CLOSE (a later docs/PHASE-22.md subtask) should be able to call
// this defensively without first having to prove the IFI is valid.
func TestFileTable_releaseUnknownIFIIsNoop(t *testing.T) {
	ft := NewFileTable(nil)

	ft.Release(999)
	ft.Release(0)
}

// TestFileTable_lookupMiss confirms Lookup on a table with nothing
// allocated reports "not found" rather than returning a zero-value
// *FileHandle that would panic the moment a caller dereferenced it.
func TestFileTable_lookupMiss(t *testing.T) {
	ft := NewFileTable(nil)

	if _, ok := ft.Lookup(42); ok {
		t.Error("Lookup(42) on an empty table = found, want not found")
	}
}

// TestFileHandle_isConsoleFalseForRealFile confirms IsConsole only
// reports true when Console is actually set — a handle representing a
// real ODS-2 file (Console left nil) must not be mistaken for the
// console pseudo-device.
func TestFileHandle_isConsoleFalseForRealFile(t *testing.T) {
	h := &FileHandle{}
	if h.IsConsole() {
		t.Error("IsConsole() = true for a handle with no Console writer set")
	}
}
