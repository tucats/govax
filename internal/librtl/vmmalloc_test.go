package librtl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// mustMalloc calls LIB$VM_MALLOC(size) and fails the test unless it
// returns a block.
func mustMalloc(t *testing.T, env *corevms.Environment, size uint32) uint32 {
	t.Helper()

	addr := call(t, env, "LIB$VM_MALLOC", size)
	if addr == 0 {
		t.Fatalf("LIB$VM_MALLOC(%d) = 0", size)
	}

	return addr
}

// TestLibVMMalloc: blocks are the default zone's, distinct and quadword
// aligned (size 0 too); LIB$VM_FREE frees each once, ignores NULL, and
// refuses anything else; no room is NULL; none of it counts in
// LIB$STAT_VM.
func TestLibVMMalloc(t *testing.T) {
	env, _ := statFixture(t)

	a := mustMalloc(t, env, 10)
	b := mustMalloc(t, env, 0)
	c := mustMalloc(t, env, 0)

	for _, p := range []uint32{a, b, c} {
		if p%8 != 0 {
			t.Errorf("block %#x isn't quadword aligned", p)
		}
	}

	if a == b || b == c || a == c {
		t.Errorf("blocks %#x, %#x, %#x aren't distinct", a, b, c)
	}

	if size, ok := env.BlockSize(a); !ok || size != 16 {
		t.Errorf("BlockSize(%#x) = %d, %v; want the default zone's 16", a, size, ok)
	}

	if r0 := call(t, env, "LIB$VM_FREE", 0); r0 != 0 {
		t.Errorf("LIB$VM_FREE(NULL) = %#x, want 0", r0)
	}

	if r0 := call(t, env, "LIB$VM_FREE", a+8); r0 != 0xFFFFFFFF {
		t.Errorf("LIB$VM_FREE of an interior address = %#x, want -1", r0)
	}

	if r0 := call(t, env, "LIB$VM_FREE", a); r0 != 0 {
		t.Errorf("LIB$VM_FREE(%#x) = %#x, want 0", a, r0)
	}

	if r0 := call(t, env, "LIB$VM_FREE", a); r0 != 0xFFFFFFFF {
		t.Errorf("LIB$VM_FREE of a freed block = %#x, want -1", r0)
	}

	// First fit: the freed block is the next one of its size.
	if p := mustMalloc(t, env, 16); p != a {
		t.Errorf("LIB$VM_MALLOC(16) after the free = %#x, want %#x", p, a)
	}

	if p := call(t, env, "LIB$VM_MALLOC", 0x40000000); p != 0 {
		t.Errorf("LIB$VM_MALLOC with no room = %#x, want 0", p)
	}

	gets, frees, pages := env.PageStatistics()
	wantStats(t, env, "the C routines", [6]uint32{0, 0, 0, gets, frees, pages})
}

// TestLibVMCalloc: the array is all zeros, even in a block that held
// something before; an overflowing product is NULL.
func TestLibVMCalloc(t *testing.T) {
	env, _ := statFixture(t)

	a := mustMalloc(t, env, 64)
	putBytes(t, env, a, strings.Repeat("\xAA", 64))

	if r0 := call(t, env, "LIB$VM_FREE", a); r0 != 0 {
		t.Fatalf("LIB$VM_FREE = %#x", r0)
	}

	p := call(t, env, "LIB$VM_CALLOC", 8, 8)
	if p != a {
		t.Fatalf("LIB$VM_CALLOC(8, 8) = %#x, want the freed block %#x", p, a)
	}

	if got := bytesAt(t, env, p, 64); got != strings.Repeat("\x00", 64) {
		t.Errorf("LIB$VM_CALLOC's block holds % x", got)
	}

	if p := call(t, env, "LIB$VM_CALLOC", 0x10000, 0x10000); p != 0 {
		t.Errorf("LIB$VM_CALLOC overflowing = %#x, want 0", p)
	}

	if p := call(t, env, "LIB$VM_CALLOC", 0, 4); p == 0 || p == a {
		t.Errorf("LIB$VM_CALLOC(0, 4) = %#x, want a block of its own", p)
	}
}

// TestLibVMRealloc: NULL is malloc; a block big enough stays; a larger
// one moves with its bytes, past 64K too, and the old block is freed;
// size 0 frees; an address that isn't a block's, or no room, is NULL
// with nothing changed.
func TestLibVMRealloc(t *testing.T) {
	env, _ := statFixture(t)

	a := call(t, env, "LIB$VM_REALLOC", 0, 20)
	if size, ok := env.BlockSize(a); a == 0 || !ok || size != 24 {
		t.Fatalf("LIB$VM_REALLOC(NULL, 20) = %#x (size %d, %v); want a 24-byte block", a, size, ok)
	}

	putBytes(t, env, a, "abcdefghijklmnopqrstuvwx")

	if p := call(t, env, "LIB$VM_REALLOC", a, 24); p != a {
		t.Errorf("LIB$VM_REALLOC to the block's size = %#x, want %#x", p, a)
	}

	if p := call(t, env, "LIB$VM_REALLOC", a, 4); p != a {
		t.Errorf("LIB$VM_REALLOC smaller = %#x, want %#x", p, a)
	}

	if p := call(t, env, "LIB$VM_REALLOC", a+8, 100); p != 0 {
		t.Errorf("LIB$VM_REALLOC of an interior address = %#x, want 0", p)
	}

	if p := call(t, env, "LIB$VM_REALLOC", a, 0x40000000); p != 0 {
		t.Errorf("LIB$VM_REALLOC with no room = %#x, want 0", p)
	}

	if _, ok := env.BlockSize(a); !ok {
		t.Fatal("a failed LIB$VM_REALLOC freed the block")
	}

	b := call(t, env, "LIB$VM_REALLOC", a, 100)
	if b == 0 || b == a {
		t.Fatalf("LIB$VM_REALLOC larger = %#x, want a new block", b)
	}

	if got := bytesAt(t, env, b, 24); got != "abcdefghijklmnopqrstuvwx" {
		t.Errorf("the moved block holds %q", got)
	}

	if _, ok := env.BlockSize(a); ok {
		t.Error("LIB$VM_REALLOC kept the old block")
	}

	// Past MOVC3's 64K: the bytes at the end move too.
	big := mustMalloc(t, env, 0x18000)
	putBytes(t, env, big, "head")
	putBytes(t, env, big+0x17FFC, "tail")

	moved := call(t, env, "LIB$VM_REALLOC", big, 0x20000)
	if moved == 0 || moved == big {
		t.Fatalf("LIB$VM_REALLOC of the large block = %#x", moved)
	}

	if bytesAt(t, env, moved, 4) != "head" || bytesAt(t, env, moved+0x17FFC, 4) != "tail" {
		t.Error("the large block's bytes didn't all move")
	}

	if p := call(t, env, "LIB$VM_REALLOC", b, 0); p != 0 {
		t.Errorf("LIB$VM_REALLOC(p, 0) = %#x, want 0", p)
	}

	if _, ok := env.BlockSize(b); ok {
		t.Error("LIB$VM_REALLOC(p, 0) didn't free the block")
	}
}
