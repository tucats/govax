package librtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// The character string routines (docs/PHASE-53, subtask 10): LIB$LOCC,
// LIB$SKPC, LIB$MATCHC, LIB$SCANC, LIB$SPANC, LIB$MOVC3, LIB$MOVC5,
// LIB$MOVTC, and LIB$MOVTUC make the VAX's character string instructions
// callable.
//
// Most take their strings by descriptor where the instructions take a
// length and an address, and the searches return a position where the
// instructions leave a count in R0: the 1-based index in the source of
// the character they stopped at, or 0 when they ran off the end (an
// empty source gives 0). The scanning is the CPU's own
// (cpu.LocateCharacter, cpu.MatchCharacters, cpu.ScanTable,
// cpu.MoveCharacters), so the routines and the instructions follow one
// set of rules.
//
// A single-character argument (LIB$LOCC's and LIB$SKPC's character-string,
// the fill and stop characters) is the first byte of its string, whose
// length "is not checked": a zero-length one still gives the byte at its
// address.
//
// The searches return a position, not a status, so an invalid descriptor
// has no status to return: govax signals LIB$_INVSTRDES for one
// (unconfirmed). LIB$MOVTUC returns -1 instead, as it does when its
// destination can't be allocated.

var libStrTru = vmsdef.Symbols["LIB$_STRTRU"]

// notFound is the position a search returns when it ran off the end.
const notFound = 0

// position turns an instruction's count of bytes left (counting the one
// it stopped at, 0 for none) into the routines' 1-based position in a
// length-byte string.
func position(length uint32, left uint16) uint32 {
	if left == 0 {
		return notFound
	}

	return length - uint32(left) + 1
}

// searchFailure is what a search routine does with a descriptor helper's
// error: an invalid descriptor is signaled as LIB$_INVSTRDES (if a
// handler continues it, the routine returns it); a memory error is an
// access violation (memoryFault).
func searchFailure(env *corevms.Environment, err error) (uint32, error) {
	if errors.Is(err, errInvStrDes) {
		return env.SignalFailure([]uint32{libInvStrDes}, libInvStrDes)
	}

	return memoryFault(env, err)
}

// firstChar returns the first byte of the string the descriptor at addr
// describes, its length not checked.
func firstChar(env *corevms.Environment, addr uint32) (byte, error) {
	d, err := readDescriptor(env, addr)
	if err != nil {
		return 0, err
	}

	return env.Memory().LoadByte(env.CPU(), d.text)
}

// libLocc is LIB$LOCC (LIB-258): the position in source-string of the
// first character equal to character-string's first, or 0.
//
//	LIB$LOCC character-string ,source-string
func libLocc(env *corevms.Environment, argv []uint32) (uint32, error) {
	return locate(env, argv, true)
}

// libSkpc is LIB$SKPC (LIB-376): the position in source-string of the
// first character not equal to character-string's first, or 0.
//
//	LIB$SKPC character-string ,source-string
func libSkpc(env *corevms.Environment, argv []uint32) (uint32, error) {
	return locate(env, argv, false)
}

// locate is LIB$LOCC and LIB$SKPC.
func locate(env *corevms.Environment, argv []uint32, equal bool) (uint32, error) {
	ch, err := firstChar(env, arg(argv, 0))
	if err != nil {
		return searchFailure(env, err)
	}

	src, err := readDescriptor(env, arg(argv, 1))
	if err != nil {
		return searchFailure(env, err)
	}

	left, err := cpu.LocateCharacter(env.CPU(), env.Memory(), ch, uint16(src.length), src.text, equal)
	if err != nil {
		return memoryFault(env, err)
	}

	return position(src.length, left), nil
}

// libMatchc is LIB$MATCHC (LIB-270): the position in source-string of the
// first occurrence of sub-string, or 0. An empty sub-string is found at
// position 1, as MATCHC finds an empty object at once (unconfirmed for
// an empty source too).
//
//	LIB$MATCHC sub-string ,source-string
func libMatchc(env *corevms.Environment, argv []uint32) (uint32, error) {
	sub, err := readDescriptor(env, arg(argv, 0))
	if err != nil {
		return searchFailure(env, err)
	}

	src, err := readDescriptor(env, arg(argv, 1))
	if err != nil {
		return searchFailure(env, err)
	}

	at, found, err := cpu.MatchCharacters(env.CPU(), env.Memory(), uint16(sub.length), sub.text, uint16(src.length), src.text)
	if err != nil {
		return memoryFault(env, err)
	}

	if !found {
		return notFound, nil
	}

	return uint32(at) + 1, nil
}

// libScanc is LIB$SCANC (LIB-334): the position in source-string of the
// first character whose entry in table-array, ANDed with the mask byte,
// is nonzero, or 0. (The manual's description says "equal to 1"; its
// RETURNS text and the instruction say nonzero, which govax follows.)
//
//	LIB$SCANC source-string ,table-array ,byte-integer-mask
func libScanc(env *corevms.Environment, argv []uint32) (uint32, error) {
	return scan(env, argv, true)
}

// libSpanc is LIB$SPANC (LIB-378): LIB$SCANC, stopping at the first
// character whose AND is zero.
//
//	LIB$SPANC source-string ,table-array ,byte-integer-mask
func libSpanc(env *corevms.Environment, argv []uint32) (uint32, error) {
	return scan(env, argv, false)
}

// scan is LIB$SCANC and LIB$SPANC. The table is 256 bytes at the
// table-array argument; the mask is a byte by reference.
func scan(env *corevms.Environment, argv []uint32, scanc bool) (uint32, error) {
	src, err := readDescriptor(env, arg(argv, 0))
	if err != nil {
		return searchFailure(env, err)
	}

	mask, err := env.Memory().LoadByte(env.CPU(), arg(argv, 2))
	if err != nil {
		return memoryFault(env, err)
	}

	left, err := cpu.ScanTable(env.CPU(), env.Memory(), uint16(src.length), src.text, arg(argv, 1), mask, scanc)
	if err != nil {
		return memoryFault(env, err)
	}

	return position(src.length, left), nil
}

// libMovc3 is LIB$MOVC3 (LIB-275): length bytes from source to
// destination, by address, not descriptor; overlap doesn't matter. The
// manual gives it no value; govax returns what MOVC3 leaves in R0, 0
// (unconfirmed).
//
//	LIB$MOVC3 word-integer-length ,source ,destination
func libMovc3(env *corevms.Environment, argv []uint32) (uint32, error) {
	length, err := env.Memory().LoadWord(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	return moveChars(env, length, arg(argv, 1), 0, length, arg(argv, 2))
}

// libMovc5 is LIB$MOVC5 (LIB-276): the source replaces the destination,
// padded with the fill byte or truncated, by address, not descriptor.
// The manual gives it no value; govax returns what MOVC5 leaves in R0,
// the source bytes not moved (unconfirmed).
//
//	LIB$MOVC5 word-integer-source-length ,source ,fill ,word-integer-destination-length ,destination
func libMovc5(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, c := env.Memory(), env.CPU()

	srcLen, err := mem.LoadWord(c, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	fill, err := mem.LoadByte(c, arg(argv, 2))
	if err != nil {
		return memoryFault(env, err)
	}

	dstLen, err := mem.LoadWord(c, arg(argv, 3))
	if err != nil {
		return memoryFault(env, err)
	}

	return moveChars(env, srcLen, arg(argv, 1), fill, dstLen, arg(argv, 4))
}

// moveChars is LIB$MOVC3's and LIB$MOVC5's move: the source bytes not
// moved. A fault in the destination is a write's.
func moveChars(env *corevms.Environment, srcLen uint16, src uint32, fill byte, dstLen uint16, dst uint32) (uint32, error) {
	left, err := cpu.MoveCharacters(env.CPU(), env.Memory(), srcLen, src, fill, dstLen, dst, nil)
	if err != nil {
		return memoryFault(env, faultIn(err, dst, uint32(dstLen)))
	}

	return uint32(left), nil
}

// faultIn marks a memory error as a write's (writing) when the address
// it names is in the length bytes at base: a routine that reads one
// area and writes another can tell which failed.
func faultIn(err error, base, length uint32) error {
	var (
		tf *vm.TranslationFault
		pe *vm.PhysicalAddressError
		va uint32
	)

	switch {
	case errors.As(err, &tf):
		va = tf.Addr
	case errors.As(err, &pe):
		va = pe.Addr
	default:
		return err
	}

	if va-base < length {
		return writing(err)
	}

	return err
}

// translate returns s with each byte replaced by its entry in the
// 256-byte table at table, stopping before the first whose entry is stop
// when stop is given (LIB$MOVTUC); at is then that byte's index, or -1
// when there was none.
func translate(env *corevms.Environment, s string, table uint32, stop *byte) (out string, at int, err error) {
	mem, c := env.Memory(), env.CPU()
	buf := make([]byte, 0, len(s))

	for i := range len(s) {
		t, err := mem.LoadByte(c, table+uint32(s[i]))
		if err != nil {
			return "", 0, err
		}

		if stop != nil && t == *stop {
			return string(buf), i, nil
		}

		buf = append(buf, t)
	}

	return string(buf), -1, nil
}

// libMovtc is LIB$MOVTC (LIB-278): source-string, each character
// translated through translation-table (256 bytes from its descriptor's
// address, its length not checked), written to destination-string as
// any output string is, a fixed one padded with fill-character's first
// byte; a dynamic one gets the source's length. SS$_NORMAL, or
// LIB$_STRTRU when the source was longer than the destination; an invalid
// descriptor is LIB$_INVSTRDES and a dynamic string that can't be
// allocated LIB$_INSVIRMEM. Overlap of the source and destination
// doesn't matter: the source is read first.
//
//	LIB$MOVTC source-string ,fill-character ,translation-table ,destination-string
func libMovtc(env *corevms.Environment, argv []uint32) (uint32, error) {
	src, err := readString(env, arg(argv, 0))
	if err != nil {
		return stringFailure(env, err)
	}

	fill, err := firstChar(env, arg(argv, 1))
	if err != nil {
		return stringFailure(env, err)
	}

	table, err := readDescriptor(env, arg(argv, 2))
	if err != nil {
		return stringFailure(env, err)
	}

	out, _, err := translate(env, src, table.text, nil)
	if err != nil {
		return memoryFault(env, err)
	}

	_, truncated, err := writePadded(env, arg(argv, 3), out, fill, true)
	if err != nil {
		return stringFailure(env, err)
	}

	if truncated {
		return libStrTru, nil
	}

	return ssNormal, nil
}

// movtucFailed is LIB$MOVTUC's value when its destination can't be
// allocated, -1.
const movtucFailed = 0xFFFFFFFF

// libMovtuc is LIB$MOVTUC (LIB-295): source-string translated through
// translation-table into destination-string until a character translates
// to stop-character's first byte. The value is that character's position
// in the source, or 0 when the source or the destination ran out first,
// or -1 when the destination can't be allocated (and, in govax, when a
// descriptor is invalid: unconfirmed). The destination is written as any
// output string is: with fill-character, a fixed one's rest is filled
// with its first byte; without it, left as it was. A dynamic one gets
// the translated characters' length (unconfirmed).
//
//	LIB$MOVTUC source-string ,stop-character ,translation-table ,destination-string [,fill-character]
func libMovtuc(env *corevms.Environment, argv []uint32) (uint32, error) {
	src, err := readString(env, arg(argv, 0))
	if err != nil {
		return movtucFailure(env, err)
	}

	stop, err := firstChar(env, arg(argv, 1))
	if err != nil {
		return movtucFailure(env, err)
	}

	table, err := readDescriptor(env, arg(argv, 2))
	if err != nil {
		return movtucFailure(env, err)
	}

	dst, err := readDescriptor(env, arg(argv, 3))
	if err != nil {
		return movtucFailure(env, err)
	}

	// The instruction stops when the destination is full, before it
	// looks at the next source character: a fixed or varying string
	// holds its limit; a dynamic one grows to the source's length.
	if dst.class != dscClassD && uint32(len(src)) > dst.limit {
		src = src[:dst.limit]
	}

	out, at, err := translate(env, src, table.text, &stop)
	if err != nil {
		return memoryFault(env, err)
	}

	var (
		fill byte
		pad  bool
	)

	if fillAddr := arg(argv, 4); fillAddr != 0 {
		if fill, err = firstChar(env, fillAddr); err != nil {
			return movtucFailure(env, err)
		}

		pad = true
	}

	if _, _, err := writePadded(env, arg(argv, 3), out, fill, pad); err != nil {
		return movtucFailure(env, err)
	}

	return uint32(at + 1), nil
}

// movtucFailure is what LIB$MOVTUC does with a descriptor helper's error:
// a memory error is an access violation; any other is -1.
func movtucFailure(env *corevms.Environment, err error) (uint32, error) {
	if errors.Is(err, errInvStrDes) || errors.Is(err, errInsVirMem) {
		return movtucFailed, nil
	}

	return memoryFault(env, err)
}
