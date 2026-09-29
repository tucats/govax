package rtl

import (
	"fmt"
	"strconv"
	"strings"
)

// $FAO and $FAOL (docs/PHASE-26.md subtask 24): formatted ASCII output.
//
// # What $FAO is for
//
// A VMS program that wants to print "Found 3 files" can't call printf: the
// VAX has no C library in its operating system. Instead it calls $FAO
// ("formatted ASCII output") with a *control string* — ordinary text with
// *directives* marked by an exclamation point — and a list of
// *parameters*:
//
//	control string:  "Found !UL file!%S"
//	parameters:      3
//	result:          "Found 3 files"
//
// $FAO copies the control string's text to an output buffer, and replaces
// each directive with the formatted value of the next parameter(s): !UL
// takes one longword and writes it in unsigned decimal, !%S writes an "S"
// if the last number formatted wasn't 1. The program then writes the
// result wherever it likes, typically to the terminal with $QIO. The
// system's own messages (%SYSTEM-F-ACCVIO, ...) are $FAO control strings
// too, formatted with the values that describe one occurrence ($GETMSG
// and $PUTMSG, message.go).
//
// The two services differ only in where the parameters come from:
//
//	SYS$FAO  ctrstr ,[outlen] ,outbuf ,[p1]...[p20]   parameters in the call
//	SYS$FAOL ctrstr ,[outlen] ,outbuf ,[prmlst]       parameters in an array
//
// # Directive syntax
//
// Every directive starts with "!" and names what to do in one or two
// upper-case characters (!AS, !UL, !/, ...). Two optional numbers may come
// before the name:
//
//	!DD        the directive DD
//	!mDD       DD in an output field m characters wide
//	!n(DD)     DD repeated n times, each with its own parameters
//	!n(mDD)    both
//
// Either number may be "#", meaning "take the number from the next
// parameter". (The repeat count's parameter comes before the width's.)
// Two directives use the number differently: in !n<...!> it's the width of
// a field holding everything up to the !>, and in !n*c it's how many
// times to write the character c.
//
// # How this file is organized
//
// faoFormatter walks the control string (format), finding each
// directive's repeat count, width, and name (directive). The name is
// looked up in faoDirectives, a registry mapping each directive name to
// the Go function that performs it — so a new directive is one more map
// entry, never another case in a switch. The numeric directives, which
// are the same few rules for five radixes and several data sizes, are
// generated into the registry from two small tables (faoRadixes and
// faoSizes) when the package loads.

// maxFAOParams is the most p1-pn arguments $FAO takes. $FAOL's list has
// no limit.
const maxFAOParams = 20

// maxFAOOutput is the longest string $FAO can produce: its length is
// returned in a word, as is an output descriptor's size.
const maxFAOOutput = 0xFFFF

// faoParamSource returns parameter i (counting from 0) of an $FAO or
// $FAOL call, or ok false if it can't be read (SS$_ACCVIO).
type faoParamSource func(i int) (v uint32, ok bool)

// faoFormatter holds the state of one formatting run: where it is in the
// control string and parameter list, and the output so far.
type faoFormatter struct {
	env    *Environment
	ctrl   string         // the control string
	pos    int            // the next character of ctrl to look at
	param  faoParamSource // where parameters come from
	next   int            // the index of the next parameter to use
	out    []byte         // the output so far
	status uint32         // 0, or the SS$ status that stopped formatting

	// lastNumber is the most recent value a numeric directive formatted,
	// which !%S tests for plurality. It starts at 0 (plural).
	lastNumber uint64

	// fieldStart and fieldWidth describe an open !n<...!> field: where in
	// out its text started, and how wide it must end up. fieldWidth is
	// -1 when no field is open.
	fieldStart int
	fieldWidth int
}

// faoDirective performs one directive, once. width is the explicit output
// field width, or -1 if the directive didn't give one. It reads its
// parameters with f.nextParam, appends its text to f.out, and reports a
// problem with f.fail.
type faoDirective func(f *faoFormatter, width int)

// formatFAO formats the control string ctrl with the parameters param
// supplies, as $FAO does. It returns the complete output (which may be
// longer than any buffer the caller has; the caller truncates) and 0, or
// the output up to the point of failure and the status that stopped it:
// SS$_BADPARAM for a directive $FAO doesn't know, SS$_ACCVIO for a
// parameter or string that can't be read.
func (env *Environment) formatFAO(ctrl string, param faoParamSource) (string, uint32) {
	f := &faoFormatter{env: env, ctrl: ctrl, param: param, fieldWidth: -1}
	f.format()

	return string(f.out), f.status
}

// format is the main loop: copy text, performing each directive met.
func (f *faoFormatter) format() {
	for f.pos < len(f.ctrl) && f.status == 0 {
		c := f.ctrl[f.pos]
		f.pos++

		if c != '!' {
			f.out = append(f.out, c)

			continue
		}

		f.directive()

		if len(f.out) > maxFAOOutput {
			f.out = f.out[:maxFAOOutput]
		}
	}
}

// directive parses and performs the directive whose "!" was just read.
// See this file's opening comment for the syntax.
func (f *faoFormatter) directive() {
	repeat := 1

	// A number (or #) followed by "(" is a repeat count. If what follows
	// isn't "(", the number was the width instead: back up (including
	// over the parameter a "#" used) and read it again as that.
	start, startParam := f.pos, f.next
	if n, ok := f.number(); ok && f.peek() == '(' {
		f.pos++
		repeat = n
	} else {
		f.pos, f.next = start, startParam
	}

	parenthesized := f.pos > start

	width, hasWidth := f.number()
	if !hasWidth {
		width = -1
	}

	name, ok := f.name()
	if !ok || f.status != 0 {
		f.fail(ssBadParam)

		return
	}

	run, known := faoDirectives[name]
	if !known {
		f.fail(ssBadParam)

		return
	}

	// !n*c's character is part of the directive: read it now, before
	// any closing parenthesis.
	var char byte

	if name == "*" {
		if f.pos >= len(f.ctrl) {
			f.fail(ssBadParam)

			return
		}

		char = f.ctrl[f.pos]
		f.pos++
	}

	if parenthesized {
		if f.peek() != ')' {
			f.fail(ssBadParam)

			return
		}

		f.pos++
	}

	if name == "*" {
		f.out = append(f.out, strings.Repeat(string(char), max(width, 0)*repeat)...)

		return
	}

	for i := 0; i < repeat && f.status == 0; i++ {
		run(f, width)
	}
}

// number reads a decimal number or "#" (take the number from the next
// parameter) at the current position. ok is false if there's neither.
func (f *faoFormatter) number() (n int, ok bool) {
	if f.peek() == '#' {
		f.pos++

		v, _ := f.nextParam()

		return int(int32(v)), true
	}

	start := f.pos
	for f.pos < len(f.ctrl) && f.ctrl[f.pos] >= '0' && f.ctrl[f.pos] <= '9' {
		f.pos++
	}

	if f.pos == start {
		return 0, false
	}

	n, err := strconv.Atoi(f.ctrl[start:f.pos])
	if err != nil { // too many digits to be a sensible count
		f.fail(ssBadParam)
	}

	return n, true
}

// name reads a directive's name: "%" and a letter, two upper-case
// letters, or one punctuation character.
func (f *faoFormatter) name() (string, bool) {
	if f.pos >= len(f.ctrl) {
		return "", false
	}

	c := f.ctrl[f.pos]

	n := 1
	if c == '%' || (c >= 'A' && c <= 'Z') {
		n = 2
	}

	if f.pos+n > len(f.ctrl) {
		return "", false
	}

	name := f.ctrl[f.pos : f.pos+n]
	f.pos += n

	return name, true
}

// peek returns the character at the current position, or 0 at the end.
func (f *faoFormatter) peek() byte {
	if f.pos < len(f.ctrl) {
		return f.ctrl[f.pos]
	}

	return 0
}

// fail stops formatting with status (the first failure wins).
func (f *faoFormatter) fail(status uint32) {
	if f.status == 0 {
		f.status = status
	}
}

// nextParam returns the next parameter and moves past it. A parameter
// that can't be read stops formatting with SS$_ACCVIO (and reads as 0).
func (f *faoFormatter) nextParam() (uint32, bool) {
	v, ok := f.param(f.next)
	f.next++

	if !ok {
		f.fail(ssAccVio)
	}

	return v, ok
}

// emit appends s, fitted to width as the string rule says: left-justified
// and blank-filled when width is longer, cut short on the right when it's
// shorter. width -1 (or 0) means s as it is.
func (f *faoFormatter) emit(s string, width int) {
	if width > 0 {
		s = fitLeft(s, width)
	}

	f.out = append(f.out, s...)
}

// fitLeft left-justifies s in width characters: blank-filled, or
// truncated on the right.
func fitLeft(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}

	return s + strings.Repeat(" ", width-len(s))
}

// fitRight right-justifies s in width characters, filled on the left
// with fill (blank or zero).
func fitRight(s string, width int, fill byte) string {
	if len(s) >= width {
		return s
	}

	return strings.Repeat(string(fill), width-len(s)) + s
}

// faoDirectives is the directive registry: every directive name $FAO
// knows, and what it does. The numeric conversions are added by
// init (see faoRadixes and faoSizes).
var faoDirectives = map[string]faoDirective{
	// Character strings.
	"AC": faoCountedString,
	"AD": func(f *faoFormatter, width int) { f.lengthAddressString(width, false) },
	"AF": func(f *faoFormatter, width int) { f.lengthAddressString(width, true) },
	"AS": faoDescriptorString,

	// Layout characters.
	"/": func(f *faoFormatter, width int) { f.out = append(f.out, "\r\n"...) },
	"_": func(f *faoFormatter, width int) { f.out = append(f.out, '\t') },
	"^": func(f *faoFormatter, width int) { f.out = append(f.out, '\f') },
	"!": func(f *faoFormatter, width int) { f.out = append(f.out, '!') },

	// A fixed-width field (!n<...!>), and a repeated character (!n*c,
	// performed by directive itself since it reads a character).
	"<": faoFieldStart,
	">": faoFieldEnd,
	"*": func(f *faoFormatter, width int) {},

	// Moving through the parameter list: !- uses the previous parameter
	// again, !+ skips one.
	"-": func(f *faoFormatter, width int) { f.next = max(f.next-1, 0) },
	"+": func(f *faoFormatter, width int) { f.nextParam() },

	// Special formatting.
	"%S": faoPlural,
	"%T": func(f *faoFormatter, width int) { f.time(width, true) },
	"%D": func(f *faoFormatter, width int) { f.time(width, false) },
	"%U": func(f *faoFormatter, width int) {
		v, _ := f.nextParam()
		f.emit(formatUIC(v), width)
	},
	"%I": faoIdentifier,
}

// faoCountedString is !AC: the parameter is the address of a *counted*
// string, whose first byte is its length (.ASCIC in MACRO-32).
func faoCountedString(f *faoFormatter, width int) {
	addr, ok := f.nextParam()
	if !ok {
		return
	}

	n, err := f.env.mem.LoadByte(f.env.cpu, addr)
	if err != nil {
		f.fail(ssAccVio)

		return
	}

	f.stringAt(addr+1, int(n), width, false)
}

// lengthAddressString is !AD (and, with filter, !AF): two parameters, the
// string's length and then its address. !AF shows each nonprintable
// character as a period, for dumping arbitrary bytes safely.
func (f *faoFormatter) lengthAddressString(width int, filter bool) {
	n, ok := f.nextParam()
	if !ok {
		return
	}

	addr, ok := f.nextParam()
	if !ok {
		return
	}

	f.stringAt(addr, int(n&0xFFFF), width, filter)
}

// faoDescriptorString is !AS: the parameter is the address of a string
// descriptor (.ASCID in MACRO-32).
func faoDescriptorString(f *faoFormatter, width int) {
	desc, ok := f.nextParam()
	if !ok {
		return
	}

	s, _, err := strGet(f.env, desc, maxFAOOutput)
	if err != nil {
		f.fail(ssAccVio)

		return
	}

	f.emit(s, width)
}

// stringAt emits the n bytes at addr, as a string directive.
func (f *faoFormatter) stringAt(addr uint32, n, width int, filter bool) {
	s, err := loadBytes(f.env, addr, n)
	if err != nil {
		f.fail(ssAccVio)

		return
	}

	if filter {
		b := []byte(s)
		for i, c := range b {
			if c < ' ' || c > '~' {
				b[i] = '.'
			}
		}

		s = string(b)
	}

	f.emit(s, width)
}

// faoFieldStart is !n<: everything up to the matching !> goes in a field
// n characters wide (see faoFieldEnd). Fields don't nest.
func faoFieldStart(f *faoFormatter, width int) {
	f.fieldStart, f.fieldWidth = len(f.out), max(width, 0)
}

// faoFieldEnd is !>: the text since the !n< is left-justified in its
// field, blank-filled or truncated to exactly n characters. A !> without
// a !n< does nothing.
func faoFieldEnd(f *faoFormatter, width int) {
	if f.fieldWidth < 0 {
		return
	}

	field := fitLeft(string(f.out[f.fieldStart:]), f.fieldWidth)
	f.out = append(f.out[:f.fieldStart], field...)
	f.fieldWidth = -1
}

// faoPlural is !%S: an "S" if the last number formatted wasn't 1, in the
// case of the character before it ("FILE!%S" gives "FILES", "file!%S"
// gives "files").
func faoPlural(f *faoFormatter, width int) {
	if f.lastNumber == 1 {
		return
	}

	s := "s"
	if n := len(f.out); n > 0 && f.out[n-1] >= 'A' && f.out[n-1] <= 'Z' {
		s = "S"
	}

	f.emit(s, width)
}

// time is !%D (date and time) and, with timeOnly, !%T (time only): the
// parameter is the address of a VMS 64-bit time, or 0 for the current
// time. The text is $ASCTIM's ("dd-mmm-yyyy hh:mm:ss.cc", or
// "hh:mm:ss.cc"); a width shorter than that truncates it, so !11%D is
// just the date and !5%T just hours and minutes.
func (f *faoFormatter) time(width int, timeOnly bool) {
	addr, ok := f.nextParam()
	if !ok {
		return
	}

	t := f.env.Clock()

	if addr != 0 {
		if t, ok = f.env.loadQuad(addr); !ok {
			f.fail(ssAccVio)

			return
		}
	}

	s, ok := formatVMSTime(t, timeOnly)
	if !ok { // a time too far away to write: VMS writes nothing useful
		s = ""
	}

	f.emit(s, width)
}

// formatUIC is !%U's text for a UIC: "[group,member]", both in octal (the
// traditional base for UICs: SYSTEM's [1,4] is 0x00010004).
func formatUIC(uic uint32) string {
	return fmt.Sprintf("[%o,%o]", uic>>16, uic&0xFFFF)
}

// faoIdentifier is !%I: the name of a rights identifier, looked up in
// the rights database (rights.go). A UIC identifier is written in
// brackets ("[SYSTEM]"), a general one as its name ("INTERACTIVE"). An
// identifier the database doesn't have is written as !%U would for a UIC,
// and as "%X" and eight hexadecimal digits for a general identifier (bit
// 31 set), as VMS does for an identifier with no name.
func faoIdentifier(f *faoFormatter, width int) {
	v, ok := f.nextParam()
	if !ok {
		return
	}

	r, found := f.env.identifierByValue(v)

	switch {
	case found && v&0x80000000 == 0:
		f.emit("["+r.name+"]", width)
	case found:
		f.emit(r.name, width)
	case v&0x80000000 == 0:
		f.emit(formatUIC(v), width)
	default:
		f.emit(fmt.Sprintf("%%X%08X", v), width)
	}
}

// The numeric directives: a radix letter, then a size letter. !XL is a
// longword in hexadecimal, !UB a byte in unsigned decimal, and so on.

// faoRadix describes one radix letter.
type faoRadix struct {
	base   int  // 8, 10, or 16
	signed bool // the value is signed (S)
	zero   bool // decimal zero-filled to the width (Z) rather than blank-filled

	// fixed is true for octal and hexadecimal, whose digits are always
	// zero-filled to a size's full width ("0000012C" for !XL of 300).
	fixed bool
}

var faoRadixes = map[byte]faoRadix{
	'O': {base: 8, fixed: true},
	'X': {base: 16, fixed: true},
	'Z': {base: 10, zero: true},
	'U': {base: 10},
	'S': {base: 10, signed: true},
}

// faoSize describes one size letter: how many bytes of the parameter
// count, and how many digits an octal or hexadecimal value of that size
// has. A quadword's parameter is its *address*, since a quadword doesn't
// fit in one longword parameter.
//
// VMS 7 added A (address), I (integer), H, and J for 64-bit Alpha code;
// on a VAX all four are a longword, which is how the VMS 7.3 system
// message texts govax uses ("virtual address=!XH") read on a VAX.
type faoSize struct {
	bytes     int
	octal     int // digits in octal
	hex       int // digits in hexadecimal
	reference bool
}

var faoSizes = map[byte]faoSize{
	'B': {bytes: 1, octal: 3, hex: 2},
	'W': {bytes: 2, octal: 6, hex: 4},
	'L': {bytes: 4, octal: 11, hex: 8},
	'A': {bytes: 4, octal: 11, hex: 8},
	'I': {bytes: 4, octal: 11, hex: 8},
	'H': {bytes: 4, octal: 11, hex: 8},
	'J': {bytes: 4, octal: 11, hex: 8},
	'Q': {bytes: 8, octal: 22, hex: 16, reference: true},
}

// init adds a registry entry for every radix and size pair.
func init() {
	for r, radix := range faoRadixes {
		for s, size := range faoSizes {
			faoDirectives[string([]byte{r, s})] = func(f *faoFormatter, width int) {
				f.numeric(radix, size, width)
			}
		}
	}
}

// numeric performs one numeric directive: it reads the value (the low
// bytes of a longword parameter, or the quadword a parameter points to),
// formats it, and fits it to width as the manual's table says:
//
//   - Octal and hexadecimal are zero-filled to the size's full width. A
//     longer width right-justifies that in blanks; a shorter one keeps
//     only the rightmost digits.
//   - Decimal has as many digits as it needs (and a "-" if negative). A
//     longer width right-justifies it, filled with blanks, or with zeros
//     for Z. A width too short for the number fills the field with
//     asterisks instead of writing a wrong number.
func (f *faoFormatter) numeric(radix faoRadix, size faoSize, width int) {
	p, ok := f.nextParam()
	if !ok {
		return
	}

	v := uint64(p)

	if size.reference {
		if v, ok = f.env.loadQuad(p); !ok {
			f.fail(ssAccVio)

			return
		}
	}

	// Keep only the size's bytes: !XB of 0x1234 is "34".
	bits := uint(size.bytes * 8)
	if bits < 64 {
		v &= 1<<bits - 1
	}

	var s string

	switch {
	case radix.signed:
		// Sign-extend from the size: a byte of 0xFF is -1. Shifting the
		// value's top bit up to bit 63 and back down as a signed number
		// copies it into every bit above.
		n := int64(v<<(64-bits)) >> (64 - bits)
		s = strconv.FormatInt(n, 10)
		f.lastNumber = uint64(n)

	default:
		s = strconv.FormatUint(v, radix.base)
		f.lastNumber = v
	}

	if radix.fixed {
		digits := size.hex
		if radix.base == 8 {
			digits = size.octal
		}

		s = strings.ToUpper(fitRight(s, digits, '0'))

		switch {
		case width > len(s):
			s = fitRight(s, width, ' ')
		case width > 0:
			s = s[len(s)-width:]
		}

		f.out = append(f.out, s...)

		return
	}

	switch {
	case width <= 0:
	case len(s) > width:
		s = strings.Repeat("*", width)
	case radix.zero:
		s = fitRight(s, width, '0')
	default:
		s = fitRight(s, width, ' ')
	}

	f.out = append(f.out, s...)
}

// serviceSysFao is SYS$FAO:
//
//	SYS$FAO ctrstr ,[outlen] ,outbuf ,[p1]...[p20]
//
// It formats the control string ctrstr (by descriptor) with the
// parameters p1-p20 into the buffer outbuf describes, and stores the
// result's length at outlen (a word; optional). See faoOutput for the
// statuses. As the manual says, the argument list's length isn't checked:
// a parameter past the end of the list reads as 0.
func serviceSysFao(env *Environment, argv []uint32) (uint32, error) {
	params := func(i int) (uint32, bool) {
		if i >= maxFAOParams {
			return 0, true
		}

		return optArg(argv, 3+i), true
	}

	return env.faoOutput(optArg(argv, 0), optArg(argv, 1), optArg(argv, 2), params), nil
}

// serviceSysFaol is SYS$FAOL:
//
//	SYS$FAOL ctrstr ,[outlen] ,outbuf ,[prmlst]
//
// It is $FAO with the parameters in an array of longwords at prmlst
// instead of in the call, so there can be any number of them. A
// parameter that can't be read (including any parameter when prmlst is
// omitted) is SS$_ACCVIO.
func serviceSysFaol(env *Environment, argv []uint32) (uint32, error) {
	prmlst := optArg(argv, 3)

	params := func(i int) (uint32, bool) {
		if prmlst == 0 {
			return 0, false
		}

		v, err := env.mem.LoadLongword(env.cpu, prmlst+uint32(i)*4)

		return v, err == nil
	}

	return env.faoOutput(optArg(argv, 0), optArg(argv, 1), optArg(argv, 2), params), nil
}

// faoOutput is the body of both services: read the control string,
// format it, store the result, and return the status:
//
//   - SS$_NORMAL, or SS$_BUFFEROVF if the result didn't fit in the
//     buffer (it's truncated, and outlen is the truncated length).
//   - SS$_BADPARAM for a directive $FAO doesn't know. What was formatted
//     before it is still stored.
//   - SS$_ACCVIO if the control string, a parameter, a string a
//     parameter points to, the output buffer, or outlen can't be
//     accessed.
func (env *Environment) faoOutput(ctrstr, outlen, outbuf uint32, params faoParamSource) uint32 {
	if ctrstr == 0 || outbuf == 0 {
		return ssAccVio
	}

	ctrl, _, err := strGet(env, ctrstr, maxFAOOutput)
	if err != nil {
		return ssAccVio
	}

	text, status := env.formatFAO(ctrl, params)
	if status == ssAccVio {
		return status
	}

	n, truncated, err := storeDescriptor(env, outbuf, text)
	if err != nil {
		return ssAccVio
	}

	if outlen != 0 {
		if err := env.mem.StoreWord(env.cpu, outlen, n); err != nil {
			return ssAccVio
		}
	}

	switch {
	case status != 0:
		return status
	case truncated:
		return ssBufferOvf
	}

	return ssNormal
}

func registerFAOServices(t *ServiceTable) {
	t.Register("SYS$FAO", serviceSysFao)
	t.Register("SYS$FAOL", serviceSysFaol)
}
