package librtl

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// The STR$ string routines (docs/planning/PHASE-83, subtask 9): written from the VMS
// 5.0 STR$ manual and held to VMS 7.1's run of testdata/str83. They share
// LIB$'s descriptors (descriptor.go), with their own conditions: where a LIB$
// routine returns LIB$_INVSTRDES, a STR$ routine signals STR$_ILLSTRCLA, and
// truncating a string is the warning STR$_TRU. A status a routine returns has
// bit 0 clear for a warning; ILLSTRPOS and its neighbors are "alternate
// successes", bit 0 set, saying a default stood in for a bad position.
//
// This file has what they share and the routines that build a string; strparts.go
// the routines that take part of one, and strsearch.go those that compare and
// search. The arithmetic routines (STR$ADD and so on) aren't here: they stay
// NOTIMPL (missing.go), by the author's answer 2.

var (
	strIllStrCla = vmsdef.LibrarySymbols["STR$_ILLSTRCLA"]
	strInsVirMem = vmsdef.LibrarySymbols["STR$_INSVIRMEM"]
	strStrTooLon = vmsdef.LibrarySymbols["STR$_STRTOOLON"]
	strWroNumArg = vmsdef.LibrarySymbols["STR$_WRONUMARG"]
	strNegStrLen = vmsdef.LibrarySymbols["STR$_NEGSTRLEN"]
	strIllStrPos = vmsdef.LibrarySymbols["STR$_ILLSTRPOS"]
	strIllStrSpe = vmsdef.LibrarySymbols["STR$_ILLSTRSPE"]
	strInvDelim  = vmsdef.LibrarySymbols["STR$_INVDELIM"]
	strNoElem    = vmsdef.LibrarySymbols["STR$_NOELEM"]
	strMatch     = vmsdef.LibrarySymbols["STR$_MATCH"]
	strNoMatch   = vmsdef.LibrarySymbols["STR$_NOMATCH"]
)

// strArg is argument i of a routine that doesn't look at its argument count.
// VMS's STR$ routines take a missing argument from whatever follows the
// argument list on the stack, as any VAX routine reading n(AP) does: for a call
// from a routine with a handler, that's the caller's frame (the probe's "no
// args" and "one arg" cases, which fail by that frame's contents). So does this.
func strArg(env *corevms.Environment, argv []uint32, i int) uint32 {
	if i < len(argv) {
		return argv[i]
	}

	v, err := env.Memory().LoadLongword(env.CPU(), env.CPU().GPR(registers.AP)+4*uint32(i+1))
	if err != nil {
		return 0
	}

	return v
}

// strFailure is what a STR$ routine does with an error from the descriptor
// helpers: a condition it signals, with the status to return if a handler
// continues it. A memory error is an access violation (memoryFault).
func strFailure(env *corevms.Environment, err error) (uint32, error) {
	switch {
	case errors.Is(err, errInvStrDes):
		return env.SignalFailure([]uint32{strIllStrCla}, strIllStrCla)
	case errors.Is(err, errInsVirMem):
		return env.SignalFailure([]uint32{strInsVirMem}, strInsVirMem)
	default:
		return memoryFault(env, err)
	}
}

// strSource reads the input string the descriptor at addr describes, with the
// address of its text. Its class byte is read first, as VMS's routines read
// it: an unreadable descriptor's access violation names that byte.
func strSource(env *corevms.Environment, addr uint32) (string, uint32, error) {
	if _, err := env.Memory().LoadByte(env.CPU(), addr+3); err != nil {
		return "", 0, err
	}

	d, err := readDescriptor(env, addr)
	if err != nil {
		return "", 0, err
	}

	s, err := readText(env, d.text, d.length)

	return s, d.text, err
}

// strTooLong signals STR$_STRTOOLON for a result too long for any descriptor,
// the destination left as it was. A destination of no string class is still
// STR$_ILLSTRCLA.
func strTooLong(env *corevms.Environment, dst uint32) (uint32, error) {
	if _, err := readDescriptor(env, dst); err != nil {
		return strFailure(env, err)
	}

	return env.SignalFailure([]uint32{strStrTooLon}, strStrTooLon)
}

// strStore writes s to the destination the descriptor at dst describes, by its
// class (writeOut), and returns SS$_NORMAL or STR$_TRU, with how many bytes
// were stored.
//
// regs, for a JSB entry, gets what a MOVC5 from text (where s came from) to the
// destination leaves: R1 past the bytes moved, R2 0, R3 past the destination's
// text, R4 and R5 0; and R6 the destination's descriptor (the probe; the copy
// routines' JSB entries in LIB$ have the status in R6 instead).
func strStore(env *corevms.Environment, dst uint32, s string, text uint32, regs *corevms.JSBRegisters) (uint32, int, error) {
	if len(s) > maxStringLength {
		status, err := strTooLong(env, dst)

		return status, 0, err
	}

	moved, truncated, end, err := writeOut(env, dst, s, ' ', true)
	if err != nil {
		status, err := strFailure(env, err)

		return status, 0, err
	}

	if regs != nil {
		regs[registers.R1] = text + uint32(moved)
		regs[registers.R2], regs[registers.R3] = 0, end
		regs[registers.R4], regs[registers.R5] = 0, 0
		regs[registers.R6] = dst
	}

	if truncated {
		return strTru, moved, nil
	}

	return ssNormal, moved, nil
}

// strPut is strStore for a routine that returns only the status.
func strPut(env *corevms.Environment, dst uint32, s string) (uint32, error) {
	status, _, err := strStore(env, dst, s, 0, nil)

	return status, err
}

// strWithStatus is strStore for a routine that found alternate, a success
// status of its own (or SS$_NORMAL), before storing: a truncation, being a
// warning, is what it returns if the store truncates (unconfirmed: the probe
// has no case with both).
func strWithStatus(env *corevms.Environment, dst uint32, s string, text uint32, alternate uint32, regs *corevms.JSBRegisters) (uint32, error) {
	status, _, err := strStore(env, dst, s, text, regs)
	if err != nil || status != ssNormal {
		return status, err
	}

	return alternate, nil
}

// wrongArgCount signals STR$_WRONUMARG, whose message takes the count and the
// routine's name.
func wrongArgCount(env *corevms.Environment, argv []uint32, name string) (uint32, error) {
	desc, err := stackString(env, name)
	if err != nil {
		return memoryFault(env, writing(err))
	}

	return env.SignalFailure([]uint32{strWroNumArg, 2, uint32(len(argv)), desc}, strWroNumArg)
}

// copyDx is STR$COPY_DX's work: any input string to any destination.
func copyDx(env *corevms.Environment, dst, src uint32, regs *corevms.JSBRegisters) (uint32, error) {
	s, text, err := strSource(env, src)
	if err != nil {
		return strFailure(env, err)
	}

	return strWithStatus(env, dst, s, text, ssNormal, regs)
}

// strCopyDx is STR$COPY_DX: destination-string, source-string.
func strCopyDx(env *corevms.Environment, argv []uint32) (uint32, error) {
	return copyDx(env, strArg(env, argv, 0), strArg(env, argv, 1), nil)
}

// strCopyDxR8 is STR$COPY_DX_R8: R0 the destination, R1 the source.
func strCopyDxR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := copyDx(env, regs[registers.R0], regs[registers.R1], regs)

	return jsbStatus(regs, status, err)
}

// copyR is STR$COPY_R's work: length bytes at src to the destination.
func copyR(env *corevms.Environment, dst, length, src uint32, regs *corevms.JSBRegisters) (uint32, error) {
	s, err := readText(env, src, length)
	if err != nil {
		return strFailure(env, err)
	}

	return strWithStatus(env, dst, s, src, ssNormal, regs)
}

// strCopyR is STR$COPY_R: destination-string, word-integer-source-length (by
// reference), source-string-address.
func strCopyR(env *corevms.Environment, argv []uint32) (uint32, error) {
	length, err := env.Memory().LoadWord(env.CPU(), strArg(env, argv, 1))
	if err != nil {
		return strFailure(env, err)
	}

	return copyR(env, strArg(env, argv, 0), uint32(length), strArg(env, argv, 2), nil)
}

// strCopyRR8 is STR$COPY_R_R8: R0 the destination, R1<15:0> the length, R2 the
// source's address.
func strCopyRR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := copyR(env, regs[registers.R0], regs[registers.R1]&0xFFFF, regs[registers.R2], regs)

	return jsbStatus(regs, status, err)
}

// joinToEnd is STR$APPEND's and STR$PREFIX's work. Only a string that can
// change its length can be added to: a dynamic or varying destination, any
// other being STR$_ILLSTRCLA with nothing changed (the probe). join makes the
// new text from the old and the source.
func joinToEnd(env *corevms.Environment, argv []uint32, join func(old, source string) string) (uint32, error) {
	dst := strArg(env, argv, 0)

	d, err := readDescriptor(env, dst)
	if err != nil {
		return strFailure(env, err)
	}

	if d.class != dscClassD && d.class != dscClassVS {
		return strFailure(env, errInvStrDes)
	}

	old, err := readText(env, d.text, d.length)
	if err != nil {
		return strFailure(env, err)
	}

	source, _, err := strSource(env, strArg(env, argv, 1))
	if err != nil {
		return strFailure(env, err)
	}

	return strPut(env, dst, join(old, source))
}

// strAppend is STR$APPEND: destination-string, source-string.
func strAppend(env *corevms.Environment, argv []uint32) (uint32, error) {
	return joinToEnd(env, argv, func(old, source string) string { return old + source })
}

// strPrefix is STR$PREFIX: destination-string, source-string.
func strPrefix(env *corevms.Environment, argv []uint32) (uint32, error) {
	return joinToEnd(env, argv, func(old, source string) string { return source + old })
}

// strConcat is STR$CONCAT: destination-string, then 1 to 254 source strings.
// It's the one that counts its arguments.
func strConcat(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 || len(argv) > 255 {
		return wrongArgCount(env, argv, "STR$CONCAT")
	}

	var b strings.Builder

	for _, addr := range argv[1:] {
		s, _, err := strSource(env, addr)
		if err != nil {
			return strFailure(env, err)
		}

		b.WriteString(s)
	}

	return strPut(env, argv[0], b.String())
}

// duplicate is STR$DUPL_CHAR's work. A negative count is STR$_NEGSTRLEN with an
// empty string. The count is checked before the string is made, since it may be
// anything up to 2,147,483,647.
func duplicate(env *corevms.Environment, dst uint32, count int32, ch byte, regs *corevms.JSBRegisters) (uint32, error) {
	alternate := ssNormal

	if count < 0 {
		count, alternate = 0, strNegStrLen
	}

	if count > maxStringLength {
		return strTooLong(env, dst)
	}

	return strWithStatus(env, dst, strings.Repeat(string([]byte{ch}), int(count)), 0, alternate, regs)
}

// strDuplChar is STR$DUPL_CHAR: destination-string, [repetition-count], and
// [ASCII-character], the last two by reference, 1 and a blank when omitted.
func strDuplChar(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	count, ch := int32(1), byte(' ')

	if addr := arg(argv, 1); addr != 0 {
		v, err := mem.LoadLongword(cpu, addr)
		if err != nil {
			return strFailure(env, err)
		}

		count = int32(v)
	}

	if addr := arg(argv, 2); addr != 0 {
		var err error
		if ch, err = mem.LoadByte(cpu, addr); err != nil {
			return strFailure(env, err)
		}
	}

	return duplicate(env, strArg(env, argv, 0), count, ch, nil)
}

// strDuplCharR8 is STR$DUPL_CHAR_R8, DUPL_CHARR8 in the vector: R0 the
// destination, R1 the count, R2<7:0> the character, both by value.
func strDuplCharR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := duplicate(env, regs[registers.R0], int32(regs[registers.R1]), byte(regs[registers.R2]), regs)

	return jsbStatus(regs, status, err)
}

// strUpcase is STR$UPCASE: destination-string, source-string. Its capitals are
// the Multinational Character Set's, LIB$AB_UPCASE: the probe's E0 and E9 come
// out C0 and C9.
func strUpcase(env *corevms.Environment, argv []uint32) (uint32, error) {
	s, _, err := strSource(env, strArg(env, argv, 1))
	if err != nil {
		return strFailure(env, err)
	}

	return strPut(env, strArg(env, argv, 0), upcased(s))
}

// upcased is s through LIB$AB_UPCASE, a byte at a time. (Go's strings.ToUpper
// reads a string as UTF-8, which VMS's text isn't.)
func upcased(s string) string {
	b := []byte(s)

	for i, c := range b {
		b[i] = upcase[c]
	}

	return string(b)
}

// strTrim is STR$TRIM: destination-string, source-string, and
// [resultant-length], a word given how many bytes were stored. Trailing blanks
// and tabs go; leading ones stay.
func strTrim(env *corevms.Environment, argv []uint32) (uint32, error) {
	s, text, err := strSource(env, strArg(env, argv, 1))
	if err != nil {
		return strFailure(env, err)
	}

	status, moved, err := strStore(env, strArg(env, argv, 0), strings.TrimRight(s, " \t"), text, nil)
	if err != nil {
		return status, err
	}

	if addr := arg(argv, 2); addr != 0 {
		if err := env.Memory().StoreWord(env.CPU(), addr, uint16(moved)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	return status, nil
}

// strTranslate is STR$TRANSLATE: destination-string, source-string,
// translation-string, match-string. A source character found in the match
// string becomes the translation string's character at that place, or a blank
// past its end; any other stays. The first place in the match string counts
// (the manual's MOVTC-style table doesn't say; unconfirmed).
func strTranslate(env *corevms.Environment, argv []uint32) (uint32, error) {
	s, _, err := strSource(env, strArg(env, argv, 1))
	if err != nil {
		return strFailure(env, err)
	}

	translation, _, err := strSource(env, strArg(env, argv, 2))
	if err != nil {
		return strFailure(env, err)
	}

	match, _, err := strSource(env, strArg(env, argv, 3))
	if err != nil {
		return strFailure(env, err)
	}

	out := []byte(s)

	for i, c := range out {
		place := strings.IndexByte(match, c)

		switch {
		case place < 0:
		case place < len(translation):
			out[i] = translation[place]
		default:
			out[i] = ' '
		}
	}

	return strPut(env, strArg(env, argv, 0), string(out))
}

// strElement is STR$ELEMENT: destination-string, element-number (by reference;
// from 0, a negative one taken as 0, as VMS's run shows), delimiter-string,
// source-string. A delimiter that isn't one character long is STR$_INVDELIM, and
// too few elements STR$_NOELEM, each leaving the destination alone (unconfirmed:
// the probe's destinations start empty).
func strElement(env *corevms.Environment, argv []uint32) (uint32, error) {
	number, err := env.Memory().LoadLongword(env.CPU(), strArg(env, argv, 1))
	if err != nil {
		return strFailure(env, err)
	}

	delimiter, _, err := strSource(env, strArg(env, argv, 2))
	if err != nil {
		return strFailure(env, err)
	}

	s, _, err := strSource(env, strArg(env, argv, 3))
	if err != nil {
		return strFailure(env, err)
	}

	if len(delimiter) != 1 {
		return strInvDelim, nil
	}

	elements := strings.Split(s, delimiter)
	n := max(int32(number), 0)

	if int(n) >= len(elements) {
		return strNoElem, nil
	}

	return strPut(env, strArg(env, argv, 0), elements[n])
}

// analyzeStr is STR$ANALYZE_SDESC's work: the length and the text's address.
func analyzeStr(env *corevms.Environment, addr uint32) (length, text uint32, err error) {
	if _, err := env.Memory().LoadByte(env.CPU(), addr+3); err != nil {
		return 0, 0, err
	}

	d, err := readDescriptor(env, addr)
	if err != nil {
		return 0, 0, err
	}

	return d.length, d.text, nil
}

// strAnalyzeSdesc is STR$ANALYZE_SDESC: input-descriptor, word-integer-length,
// data-address. It returns the length in R0, not a status (VMS's run; the
// manual calls the routine's value nothing).
func strAnalyzeSdesc(env *corevms.Environment, argv []uint32) (uint32, error) {
	length, text, err := analyzeStr(env, strArg(env, argv, 0))
	if err != nil {
		return strFailure(env, err)
	}

	mem, cpu := env.Memory(), env.CPU()

	if err := mem.StoreWord(cpu, strArg(env, argv, 1), uint16(length)); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, strArg(env, argv, 2), text); err != nil {
		return memoryFault(env, writing(err))
	}

	return length, nil
}

// strAnalyzeSdescR1 is STR$ANALYZE_SDESC_R1: R0 the descriptor; it returns the
// length in R0 and the text's address in R1.
func strAnalyzeSdescR1(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	length, text, err := analyzeStr(env, regs[registers.R0])
	if err != nil {
		status, err := strFailure(env, err)

		return jsbStatus(regs, status, err)
	}

	regs[registers.R0], regs[registers.R1] = length, text

	return nil
}

// requireDynamic is the check STR$GET1_DX and STR$FREE1_DX make, which their
// LIB$ neighbors don't: the descriptor must be class D.
func requireDynamic(env *corevms.Environment, desc uint32) error {
	class, err := env.Memory().LoadByte(env.CPU(), desc+3)
	if err != nil {
		return err
	}

	if class != dscClassD {
		return errInvStrDes
	}

	return nil
}

// getDx is STR$GET1_DX's work: LIB$SGET1_DD's, on a class D string, with the
// facility's own condition.
func getDx(env *corevms.Environment, length, desc uint32) (uint32, error) {
	if err := requireDynamic(env, desc); err != nil {
		return strFailure(env, err)
	}

	status, err := getDynamic(env, desc, length)
	if err == nil && status == libInsVirMem {
		return strFailure(env, errInsVirMem)
	}

	return status, err
}

// strGet1Dx is STR$GET1_DX: word-integer-length (by reference), character-string.
func strGet1Dx(env *corevms.Environment, argv []uint32) (uint32, error) {
	length, err := env.Memory().LoadWord(env.CPU(), strArg(env, argv, 0))
	if err != nil {
		return strFailure(env, err)
	}

	return getDx(env, uint32(length), strArg(env, argv, 1))
}

// strGet1DxR4 is STR$GET1_DX_R4: R0<15:0> the length, R1 the descriptor.
func strGet1DxR4(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	status, err := getDx(env, regs[registers.R0]&0xFFFF, regs[registers.R1])

	return jsbStatus(regs, status, err)
}

// freeDx is STR$FREE1_DX's work: LIB$SFREE1_DD's, on a class D string; a block
// that isn't the string zone's is signaled here.
func freeDx(env *corevms.Environment, desc uint32) (uint32, error) {
	if err := requireDynamic(env, desc); err != nil {
		return strFailure(env, err)
	}

	status, err := freeDynamic(env, desc)
	if err == nil && status == strErrFreDyn {
		return env.SignalFailure([]uint32{strErrFreDyn}, strErrFreDyn)
	}

	return status, err
}

// strFree1Dx is STR$FREE1_DX: character-string.
func strFree1Dx(env *corevms.Environment, argv []uint32) (uint32, error) {
	return freeDx(env, strArg(env, argv, 0))
}

// strFree1DxR4 is STR$FREE1_DX_R4: R0 the descriptor. R2 ends 0 and R4 the
// descriptor, as VMS's leaves them for a string with nothing to free.
func strFree1DxR4(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	desc := regs[registers.R0]
	status, err := freeDx(env, desc)

	if err == nil {
		regs[registers.R2], regs[registers.R4] = 0, desc
	}

	return jsbStatus(regs, status, err)
}
