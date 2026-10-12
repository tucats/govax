package librtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// String descriptors (docs/PHASE-53, Decision 3). A VAX procedure passes
// a string "by descriptor": the argument is the address of a small block
// that says where the text is and how long it is. Its first longword is
// the same in every class:
//
//	+0  DSC$W_LENGTH   word: the length (for VS, the maximum length)
//	+2  DSC$B_DTYPE    byte: the data type (DSC$K_DTYPE_T for text)
//	+3  DSC$B_CLASS    byte: the class, which says how to read the rest
//	+4  DSC$A_POINTER  longword: the address of the data
//
// The classes the RTL accepts for a string, as the LIB$ manual's
// LIB$ANALYZE_SDESC (LIB-10) and LIB$SCOPY_DXDX (LIB-337) describe them:
//
//   - Z (unspecified), S (scalar), and SD (decimal scalar): LENGTH bytes
//     at POINTER.
//   - D (dynamic): the same, but the storage is the RTL's, from the heap;
//     a routine that returns a string allocates it to fit.
//   - A (array) and NCA (noncontiguous array): an array of characters,
//     read as one string whose length is DSC$L_ARSIZE (+12), its size in
//     bytes, which must be less than 65,536.
//   - VS (varying string): LENGTH is the most the string may hold
//     (DSC$W_MAXSTRLEN); POINTER addresses a word holding its current
//     length (CURLEN), followed by the text.
//
// Any other class is LIB$_INVSTRDES.

// The DSC$K_CLASS_ codes of the string classes.
const (
	dscClassZ   = 0  // unspecified, read as S
	dscClassS   = 1  // scalar (fixed length): .ASCID's
	dscClassD   = 2  // dynamic: storage from the heap
	dscClassA   = 4  // array
	dscClassSD  = 9  // decimal scalar, read as S
	dscClassNCA = 10 // noncontiguous array, read as A
	dscClassVS  = 11 // varying string
)

// dscDTypeT is DSC$K_DTYPE_T, the data type of character text.
const dscDTypeT = 14

// maxStringLength is the longest string a descriptor describes: its
// lengths are words.
const maxStringLength = 65535

var (
	libInvStrDes = vmsdef.LibrarySymbols["LIB$_INVSTRDES"]
	libInsVirMem = vmsdef.LibrarySymbols["LIB$_INSVIRMEM"]
	libWroNumArg = vmsdef.LibrarySymbols["LIB$_WRONUMARG"]
	errInvStrDes = errors.New("invalid string descriptor")
	errInsVirMem = errors.New("insufficient virtual memory")
)

// stringDesc is a string descriptor read from memory.
type stringDesc struct {
	addr  uint32 // the descriptor's own address
	class byte   // DSC$B_CLASS

	// length is the string's length: DSC$W_LENGTH, an array's
	// DSC$L_ARSIZE, or a varying string's CURLEN.
	length uint32

	// limit is the most it can hold without new storage: length, or a
	// varying string's DSC$W_MAXSTRLEN.
	limit uint32

	// text is the address of the string's first byte.
	text uint32
}

// readDescriptor reads the string descriptor at addr. It returns
// errInvStrDes for a class that isn't a string's, or an array of 65,536
// bytes or more, and a memory error (an access violation) when the
// descriptor, or a varying string's length word, can't be read.
func readDescriptor(env *corevms.Environment, addr uint32) (stringDesc, error) {
	mem, cpu := env.Memory(), env.CPU()
	d := stringDesc{addr: addr}

	length, err := mem.LoadWord(cpu, addr)
	if err != nil {
		return d, err
	}

	if d.class, err = mem.LoadByte(cpu, addr+3); err != nil {
		return d, err
	}

	if d.text, err = mem.LoadLongword(cpu, addr+4); err != nil {
		return d, err
	}

	switch d.class {
	case dscClassZ, dscClassS, dscClassD, dscClassSD:
		d.length, d.limit = uint32(length), uint32(length)

	case dscClassA, dscClassNCA:
		size, err := mem.LoadLongword(cpu, addr+12) // DSC$L_ARSIZE
		if err != nil {
			return d, err
		}

		if size > maxStringLength {
			return d, errInvStrDes
		}

		d.length, d.limit = size, size

	case dscClassVS:
		cur, err := mem.LoadWord(cpu, d.text) // CURLEN
		if err != nil {
			return d, err
		}

		d.length, d.limit = uint32(min(cur, length)), uint32(length)
		d.text += 2

	default:
		return d, errInvStrDes
	}

	return d, nil
}

// readString reads the input string the descriptor at addr describes,
// of any class the RTL accepts. Its errors are readDescriptor's, and a
// memory error when the text can't be read.
func readString(env *corevms.Environment, addr uint32) (string, error) {
	d, err := readDescriptor(env, addr)
	if err != nil {
		return "", err
	}

	mem, cpu := env.Memory(), env.CPU()
	buf := make([]byte, d.length)

	for i := range buf {
		if buf[i], err = mem.LoadByte(cpu, d.text+uint32(i)); err != nil {
			return "", err
		}
	}

	return string(buf), nil
}

// writeString returns s in the output string the descriptor at addr
// describes, by its class (the manual's LIB$SCOPY_DXDX, LIB-337):
//
//   - Z, S, SD, A, NCA: copied into the string's fixed storage,
//     truncated or padded on the right with blanks.
//   - D: copied into its storage if that's long enough, else the old
//     storage is freed and new storage of s's length allocated; the
//     descriptor's length (and address) become s's.
//   - VS: copied up to DSC$W_MAXSTRLEN, unpadded, and CURLEN set to the
//     bytes copied.
//
// stored is how many of s's bytes were stored, truncated whether that's
// fewer than all of them: a routine then returns its "string truncated"
// status (LIB$_STRTRU, or the one its manual entry names). Errors are
// readDescriptor's, errInsVirMem when a dynamic string's storage can't be
// allocated, and a memory error when the storage can't be written
// (marked as a write's, writing).
func writeString(env *corevms.Environment, addr uint32, s string) (stored int, truncated bool, err error) {
	return writePadded(env, addr, s, ' ', true)
}

// writePadded is writeString with fill in place of blanks for a fixed
// string's padding (LIB$MOVTC's fill-character). With pad false, a fixed
// string's bytes past s are left as they were (LIB$MOVTUC without a fill
// character).
func writePadded(env *corevms.Environment, addr uint32, s string, fill byte, pad bool) (stored int, truncated bool, err error) {
	d, err := readDescriptor(env, addr)
	if err != nil {
		return 0, false, err
	}

	mem, cpu := env.Memory(), env.CPU()

	if len(s) > maxStringLength {
		s, truncated = s[:maxStringLength], true
	}

	n := uint32(len(s)) // bytes to copy
	room := d.length    // bytes to fill: the copy, then blanks

	switch d.class {
	case dscClassD:
		if d.text == 0 || n > d.limit {
			if d.text != 0 {
				env.FreeVM(d.text)
			}

			d.text = 0

			if n > 0 {
				if d.text, err = env.AllocateVM(n, 0); err != nil {
					return 0, false, err
				}

				if d.text == 0 {
					return 0, false, errInsVirMem
				}
			}

			if err := mem.StoreLongword(cpu, addr+4, d.text); err != nil {
				return 0, false, writing(err)
			}
		}

		if err := mem.StoreWord(cpu, addr, uint16(n)); err != nil {
			return 0, false, writing(err)
		}

		room = n

	case dscClassVS:
		n = min(n, d.limit)
		room = n

		if err := mem.StoreWord(cpu, d.text-2, uint16(n)); err != nil {
			return 0, false, writing(err)
		}

	default:
		n = min(n, room)
	}

	if !pad {
		room = n
	}

	for i := uint32(0); i < room; i++ {
		ch := fill
		if i < n {
			ch = s[i]
		}

		if err := mem.StoreByte(cpu, d.text+i, ch); err != nil {
			return 0, false, writing(err)
		}
	}

	return int(n), truncated || int(n) < len(s), nil
}

// tooFewArgs reports whether the call passed fewer than n arguments: a
// routine whose manual entry lists LIB$_WRONUMARG returns it then
// (Decision 4). One that doesn't treats a missing argument as a 0 address
// (arg).
func tooFewArgs(argv []uint32, n int) bool {
	return len(argv) < n
}
