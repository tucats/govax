package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
)

// The C library's heap routines in LIBRTL: LIB$VM_MALLOC, LIB$VM_FREE,
// LIB$VM_CALLOC, and LIB$VM_REALLOC (docs/PHASE-59, subtask 8). They are
// in VMS 7.3's transfer vector but in neither LIB$ manual. They are C's
// malloc, free, calloc, and realloc, with C's conventions, as VMS 7.1
// showed (testdata/lib59): every argument by value (a size or a
// pointer), the result in R0, 0 (NULL) when there's no room. Their
// blocks are a heap of their own (corevms's CAllocate, CFree, CSize),
// which LIB$FIND_VM_ZONE doesn't find and LIB$STAT_VM doesn't count, and
// LIB$FREE_VM refuses them. LIB$VM_FREE returns 1. The _P forms
// (VM_MALLOC_P and the rest, and VM_MALLOC_INIT_P), whose extra argument
// no probe alone can explain, stay missing-routine stubs.

// moveLimit is the most bytes cpu.MoveCharacters moves at a time: its
// lengths are words, as MOVC3's and MOVC5's are.
const moveLimit = 0xFFFF

// libVMMalloc is LIB$VM_MALLOC:
//
//	address = LIB$VM_MALLOC(size)
//
// It allocates size bytes and returns the block's address, or 0 when
// there's no room. A size of 0 gets a block too, as on VMS 7.1.
func libVMMalloc(env *corevms.Environment, argv []uint32) (uint32, error) {
	return env.CAllocate(arg(argv, 0)), nil
}

// libVMFree is LIB$VM_FREE:
//
//	LIB$VM_FREE(address)
//
// It frees the block LIB$VM_MALLOC, LIB$VM_CALLOC, or LIB$VM_REALLOC
// returned. Freeing 0 (NULL) does nothing. It returns 1, as VMS 7.1's
// does (testdata/lib59), or, by govax's choice, 0 for an address that
// isn't a block's, freeing nothing (unconfirmed).
func libVMFree(env *corevms.Environment, argv []uint32) (uint32, error) {
	addr := arg(argv, 0)
	if addr == 0 || env.CFree(addr) {
		return 1, nil
	}

	return 0, nil
}

// libVMCalloc is LIB$VM_CALLOC:
//
//	address = LIB$VM_CALLOC(count, size)
//
// It allocates an array of count elements of size bytes each, every byte
// 0, and returns its address; or 0 when there's no room or count times
// size doesn't fit in a longword.
func libVMCalloc(env *corevms.Environment, argv []uint32) (uint32, error) {
	total := uint64(arg(argv, 0)) * uint64(arg(argv, 1))
	if total > 0xFFFFFFFF {
		return 0, nil
	}

	addr := env.CAllocate(uint32(total))
	if addr == 0 {
		return 0, nil
	}

	// A block can be one freed before, so its bytes are cleared here.
	if err := moveBytes(env, 0, 0, addr, uint32(total)); err != nil {
		env.CFree(addr)

		return memoryFault(env, writing(err))
	}

	return addr, nil
}

// libVMRealloc is LIB$VM_REALLOC:
//
//	address = LIB$VM_REALLOC(address, size)
//
// It changes the size of the block at address to size bytes, returning
// the block's address, which may be a new one: a block already large
// enough (as the heap rounded it) stays where it is, as VMS 7.1's does,
// and otherwise a new block gets the old one's bytes and the old one is
// freed. An address of 0 makes it LIB$VM_MALLOC; a size of 0 keeps the
// block (VMS 7.1 returns an address, not NULL). It returns 0, changing
// nothing, when there's no room for the new block, or address isn't a
// block's (unconfirmed). Where VMS puts a moved block isn't govax's
// (testdata/lib59's expected differences).
func libVMRealloc(env *corevms.Environment, argv []uint32) (uint32, error) {
	addr, size := arg(argv, 0), arg(argv, 1)
	if addr == 0 {
		return env.CAllocate(size), nil
	}

	old, ok := env.CSize(addr)
	if !ok {
		return 0, nil
	}

	if size <= old {
		return addr, nil
	}

	moved := env.CAllocate(size)
	if moved == 0 {
		return 0, nil
	}

	if err := moveBytes(env, old, addr, moved, old); err != nil {
		env.CFree(moved)

		return memoryFault(env, err)
	}

	env.CFree(addr)

	return moved, nil
}

// moveBytes copies the srcLen bytes at src to the dstLen bytes at dst,
// filling the rest of them with 0, as MOVC5 does but without its 64K
// limit: srcLen is at most dstLen, and the two don't overlap.
func moveBytes(env *corevms.Environment, srcLen, src, dst, dstLen uint32) error {
	for dstLen > 0 {
		n := min(dstLen, moveLimit)
		m := min(srcLen, n)

		if _, err := cpu.MoveCharacters(env.CPU(), env.Memory(), uint16(m), src, 0, uint16(n), dst, nil); err != nil {
			return err
		}

		src, srcLen = src+m, srcLen-m
		dst, dstLen = dst+n, dstLen-n
	}

	return nil
}
