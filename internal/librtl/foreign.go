package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

var (
	libInpStrTru = vmsdef.LibrarySymbols["LIB$_INPSTRTRU"]
	rmsEOF       = vmsdef.Symbols["RMS$_EOF"]
)

// dscClassD is DSC$K_CLASS_D, a dynamic string descriptor, whose storage
// a routine returning a string allocates. Every other class (a MACRO
// program's static descriptor is DSC$K_CLASS_S) is fixed length.
const dscClassD = 2

// maxPromptLength is the longest prompt LIB$GET_FOREIGN and LIB$GET_INPUT
// write.
const maxPromptLength = 255

// maxInputLength is the longest line LIB$GET_FOREIGN (when it prompts)
// and LIB$GET_INPUT read: a string descriptor's length is a word.
const maxInputLength = 65535

// libGetForeign is LIB$GET_FOREIGN (RTL Library manual):
//
//	LIB$GET_FOREIGN resultant-string [,prompt-string] [,resultant-length] [,flags]
//
// It returns the text of the foreign command that ran the image, after
// the command's verb (Environment.CommandLine). If there's none, or bit 0
// of the longword at flags is set, and a prompt-string is given, it
// prompts for the text on SYS$INPUT instead, as LIB$GET_INPUT does; it
// then sets flags to 1, so that a program calling it in a loop prompts
// from the second call on. The text goes into resultant-string (see
// storeString), and its length, as a word, into resultant-length.
//
// It returns SS$_NORMAL; LIB$_INPSTRTRU when the text didn't fit a
// fixed-length resultant-string; RMS$_EOF at the end of the input when it
// prompted; LIB$_INVARG with no resultant-string; or SS$_ACCVIO.
//
// Unconfirmed against VMS (docs/PHASE-34.md, 2026-10-04): that flags is
// set to 1 whether or not the routine prompted, and that a prompted line
// is returned as typed, not uppercased.
func libGetForeign(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	result, prompt, lenAddr, flagsAddr := arg(argv, 0), arg(argv, 1), arg(argv, 2), arg(argv, 3)

	if result == 0 {
		return libInvArg, nil
	}

	force := false

	if flagsAddr != 0 {
		flags, err := mem.LoadLongword(cpu, flagsAddr)
		if err != nil {
			return ssAccVio, nil
		}

		force = flags&1 != 0
	}

	text := env.CommandLine
	status := ssNormal

	if (text == "" || force) && prompt != 0 {
		p, _, err := env.StringDescriptor(prompt, maxPromptLength)
		if err != nil {
			return ssAccVio, nil
		}

		line, ok, err := env.ReadInputLine(p, maxInputLength)
		if err != nil {
			return 0, err // waiting for the line: called again
		}

		if !ok {
			status = rmsEOF
		}

		text = line
	}

	if flagsAddr != 0 {
		if err := mem.StoreLongword(cpu, flagsAddr, 1); err != nil {
			return ssAccVio, nil
		}
	}

	stored, truncated, err := storeString(env, result, text)
	if err != nil {
		return ssAccVio, nil
	}

	if lenAddr != 0 {
		if err := mem.StoreWord(cpu, lenAddr, uint16(stored)); err != nil {
			return ssAccVio, nil
		}
	}

	if truncated && status == ssNormal {
		status = libInpStrTru
	}

	return status, nil
}

// storeString returns s in the string descriptor at addr, as the RTL's
// routines return a string. A dynamic descriptor (DSC$K_CLASS_D) gets new
// storage of s's length, its old storage freed. Any other is fixed length:
// s is copied into its storage, truncated, or padded with blanks, to its
// length. stored is how many of s's bytes were stored; truncated is true
// when that's fewer than all of them.
func storeString(env *corevms.Environment, addr uint32, s string) (stored int, truncated bool, err error) {
	mem, cpu := env.Memory(), env.CPU()

	length, err := mem.LoadWord(cpu, addr)
	if err != nil {
		return 0, false, err
	}

	class, err := mem.LoadByte(cpu, addr+3)
	if err != nil {
		return 0, false, err
	}

	ptr, err := mem.LoadLongword(cpu, addr+4)
	if err != nil {
		return 0, false, err
	}

	if class == dscClassD {
		if len(s) > maxInputLength {
			s, truncated = s[:maxInputLength], true
		}

		if ptr != 0 {
			env.FreeVM(ptr)
		}

		ptr = 0

		if len(s) > 0 {
			if ptr, err = env.AllocateVM(uint32(len(s)), 0); err != nil {
				return 0, false, err
			}
		}

		if err := mem.StoreWord(cpu, addr, uint16(len(s))); err != nil {
			return 0, false, err
		}

		if err := mem.StoreLongword(cpu, addr+4, ptr); err != nil {
			return 0, false, err
		}

		length = uint16(len(s))
	}

	for i := 0; i < int(length); i++ {
		ch := byte(' ')
		if i < len(s) {
			ch = s[i]
		}

		if err := mem.StoreByte(cpu, ptr+uint32(i), ch); err != nil {
			return 0, false, err
		}
	}

	stored = min(len(s), int(length))

	return stored, truncated || stored < len(s), nil
}
