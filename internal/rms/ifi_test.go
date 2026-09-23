package rms

import (
	"bytes"
	"testing"
)

// TestFileTable_consoleSeeded confirms NewFileTable pre-seeds IFI 1 with
// the console writer it was given (see FileTable's own doc comment on
// why slot 1 is special — it's SYS$OUTPUT), and that the seeded handle
// actually writes through to that same io.Writer.
func TestFileTable_consoleSeeded(t *testing.T) {
	var out bytes.Buffer

	ft := NewFileTable(&out)

	h, ok := ft.Lookup(1)
	if !ok {
		t.Fatal("Lookup(1) = not found, want the console handle NewFileTable seeds")
	}

	if !h.IsConsole() {
		t.Error("IsConsole() = false for the console-seeded slot 1")
	}

	if _, err := h.Console.Write([]byte("hello")); err != nil {
		t.Fatalf("writing through the seeded console handle: %v", err)
	}

	if got := out.String(); got != "hello" {
		t.Errorf("console output = %q, want %q", got, "hello")
	}
}

// TestFileTable_nilConsoleLeavesSlotUnusable confirms a nil consoleOut
// (valid for a test, or any other case with no real console) simply
// leaves slot 1 empty rather than seeding a handle with a nil Console
// writer that would panic the first time something tried to write
// through it.
func TestFileTable_nilConsoleLeavesSlotUnusable(t *testing.T) {
	ft := NewFileTable(nil)

	if _, ok := ft.Lookup(1); ok {
		t.Error("Lookup(1) with a nil consoleOut = found, want not found")
	}
}

// TestFileTable_allocSkipsReservedSlots confirms Alloc never hands out
// one of the four reserved IFIs (0-3 — see FileTable's own doc comment),
// even when — as here, with a nil console — slot 1 happens to be free.
func TestFileTable_allocSkipsReservedSlots(t *testing.T) {
	ft := NewFileTable(nil)

	id := ft.Alloc(&FileHandle{})
	if id < firstUserIFI {
		t.Errorf("Alloc returned reserved IFI %d, want an IFI >= %d", id, firstUserIFI)
	}
}

// TestFileTable_allocSkipsConsoleSlot confirms Alloc still skips IFI 1
// even when a real console handle occupies it (as opposed to it simply
// being below firstUserIFI).
func TestFileTable_allocSkipsConsoleSlot(t *testing.T) {
	var out bytes.Buffer

	ft := NewFileTable(&out)

	for range 5 {
		if id := ft.Alloc(&FileHandle{}); id == 1 {
			t.Fatal("Alloc handed out IFI 1, which NewFileTable already seeded with the console")
		}
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
