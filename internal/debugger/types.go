package debugger

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is the debugger's data types (docs/PHASE-42.md, subtask 18):
// what EXAMINE and DEPOSIT take a location to hold, by a type qualifier
// (/FLOAT, /ASCIC, /PACKED:5, ...), by SET TYPE's default for locations
// the program's symbols don't type, or by SET TYPE/OVERRIDE's type for
// every location.
//
// Background for a reader new to the VAX. Memory is just bytes; what they
// mean depends on how a program uses them, and the debugger can show the
// same bytes several ways:
//
//   - integers of 1, 2, 4, 8, or 16 bytes (byte, word, longword, quadword,
//     octaword), least significant byte first;
//   - floating-point numbers in one of the VAX's four formats (F, D, G, and
//     H_floating; internal/vaxfloat describes them);
//   - packed decimal: two decimal digits a byte, most significant first,
//     the last half byte (nibble) holding the sign;
//   - text, which VMS lays out several ways: a fixed number of characters
//     (ASCII), a count byte or word followed by the characters (ASCIC,
//     ASCIW), characters ended by a zero byte (ASCIZ), or a *descriptor*
//     (ASCID: an 8-byte record holding the text's length and address);
//   - a VMS time: a quadword count of 100-nanosecond ticks since 17
//     November 1858;
//   - a condition value: a longword status code, shown as its message;
//   - the processor status longword (PSL) or its low word (PSW).

// typeKind is the family a dataType belongs to.
type typeKind int

const (
	typeInteger     typeKind = iota // size bytes, shown in a radix
	typeFloat                       // a vaxfloat.Format
	typePacked                      // digits decimal digits
	typeASCII                       // count characters (or the location's own length)
	typeASCIC                       // a count byte, then the text
	typeASCIW                       // a count word, then the text
	typeASCIZ                       // text ended by a zero byte
	typeASCID                       // a string descriptor
	typeDateTime                    // a quadword VMS time
	typeCondition                   // a longword condition value
	typePSL                         // the processor status longword's fields
	typePSW                         // the processor status word's fields
	typePTE                         // a page table entry (govax's own)
	typeInstruction                 // a machine instruction
	typeSymbol                      // the type of a datum the program defines (TYPE=)
)

// dataType is one type a location can be examined or deposited as.
type dataType struct {
	kind typeKind

	// size is an integer's size in bytes (1, 2, 4, 8, or 16).
	size uint32

	// count is ASCII's number of characters; counted says one was given
	// (ASCII alone takes the location's own length, or 4 for SET TYPE).
	count   uint32
	counted bool

	// digits is PACKED's number of decimal digits.
	digits uint32

	// float is a typeFloat's format.
	float vaxfloat.Format

	// datum is the datum TYPE=(name) named, and symbol that name as typed.
	datum  *dbgsym.Datum
	symbol string
}

// longwordType is the type of a location nothing else types.
var longwordType = dataType{kind: typeInteger, size: 4}

// fixedSize returns the number of bytes a value of the type occupies, for
// a type whose size doesn't depend on what is stored (an ASCIC string's
// does). ok is false for the others.
func (t dataType) fixedSize() (uint32, bool) {
	switch t.kind {
	case typeInteger:
		return t.size, true
	case typeFloat:
		return uint32(t.float.Size()), true
	case typePacked:
		return t.digits/2 + 1, true
	case typeASCII:
		return t.count, t.counted
	case typeDateTime:
		return 8, true
	case typeCondition, typePSL, typePTE:
		return 4, true
	case typePSW:
		return 2, true
	case typeSymbol:
		if t.datum.IsText() && t.datum.Descriptor != nil {
			return uint32(t.datum.Descriptor.Length), true
		}

		if size := t.datum.ElementSize(); size > 0 {
			return size, true
		}
	}

	return 0, false
}

// floatFormats are the floating qualifiers and keywords, and their
// formats. FLOAT is the VMS debugger's name for F_floating; F_FLOAT is
// accepted too, as the other three are spelled that way.
var floatFormats = []struct {
	name   string
	format vaxfloat.Format
}{
	{"FLOAT", vaxfloat.F},
	{"F_FLOAT", vaxfloat.F},
	{"D_FLOAT", vaxfloat.D},
	{"G_FLOAT", vaxfloat.G},
	{"H_FLOAT", vaxfloat.H},
}

// integerSizes are the integer qualifiers and keywords, and their sizes.
var integerSizes = []struct {
	name string
	size uint32
}{
	{"BYTE", 1},
	{"WORD", 2},
	{"LONGWORD", 4},
	{"QUADWORD", 8},
	{"OCTAWORD", 16},
}

// simpleTypes are the qualifiers and keywords that name a type with no
// value: the text layouts and the other fixed types.
var simpleTypes = []struct {
	name string
	kind typeKind
}{
	{"ASCIC", typeASCIC},
	{"ASCIW", typeASCIW},
	{"ASCIZ", typeASCIZ},
	{"ASCID", typeASCID},
	{"DATE_TIME", typeDateTime},
	{"CONDITION_VALUE", typeCondition},
	{"PSL", typePSL},
	{"PSW", typePSW},
	{"PTE", typePTE},
}

// qualifierType returns the type a command's qualifiers give its
// locations (EXAMINE/FLOAT, DEPOSIT/ASCIC, ...), and whether one was
// given. The grammar allows only one type qualifier on a command.
// /INSTRUCTION isn't among them: EXAMINE handles it apart (examine.go),
// and DEPOSIT/INSTRUCTION is subtask 19's.
func (d *Debugger) qualifierType(r *dcl.Result) (dataType, bool, error) {
	for _, q := range integerSizes {
		if r.Present(q.name) {
			return dataType{kind: typeInteger, size: q.size}, true, nil
		}
	}

	for _, q := range floatFormats {
		if r.Present(q.name) {
			return dataType{kind: typeFloat, float: q.format}, true, nil
		}
	}

	for _, q := range simpleTypes {
		if r.Present(q.name) {
			return dataType{kind: q.kind}, true, nil
		}
	}

	if asciiGiven(r) {
		t := dataType{kind: typeASCII}

		count, have, err := d.asciiCount(r.String("ASCII"))
		if err != nil {
			return dataType{}, false, err
		}

		t.count, t.counted = count, have

		return t, true, nil
	}

	if r.Present("PACKED") {
		t, err := packedType(r.Int("PACKED"))

		return t, err == nil, err
	}

	if r.Present("TYPE") {
		t, err := d.symbolType(strings.Join(r.List("TYPE"), ","))

		return t, err == nil, err
	}

	return dataType{}, false, nil
}

// packedType is PACKED:n, n decimal digits: the VAX's packed decimal
// strings hold 0 to 31.
func packedType(n int64) (dataType, error) {
	if n < 0 || n > 31 {
		return dataType{}, vmserrors.New(vmserrors.DBG_SYNTAX, fmt.Sprint(n))
	}

	return dataType{kind: typePacked, digits: uint32(n)}, nil
}

// symbolType is TYPE=(name): the type of the datum name labels.
func (d *Debugger) symbolType(name string) (dataType, error) {
	name = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(name), "("), ")"))

	addr, err := d.evalWhole(name)
	if err != nil {
		return dataType{}, err
	}

	if prog := d.Console.DebugProgramAt(addr); prog != nil {
		if datum, _, ok := prog.DatumAt(addr); ok {
			return dataType{kind: typeSymbol, datum: datum, symbol: strings.ToUpper(name)}, nil
		}
	}

	return dataType{}, vmserrors.New(vmserrors.DBG_NOTYPE, strings.ToUpper(name))
}

// parseTypeKeyword reads SET TYPE's keyword: an integer, floating, text,
// or other type name (BYTE, D_FLOAT, ASCIC, ...), ASCII:n, PACKED:n,
// INSTRUCTION, or TYPE=(name). A keyword may be abbreviated to anything
// that picks out one name, and AC, AD, AW, and AZ are the text types'
// short names. ASCII without a count is 4 characters.
func (d *Debugger) parseTypeKeyword(text string) (dataType, error) {
	text = strings.TrimSpace(text)
	word, value, _ := strings.Cut(text, "=")
	word, count, hasCount := strings.Cut(word, ":")
	word = strings.ToUpper(strings.TrimSpace(word))

	if word == "" {
		return dataType{}, vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "type")
	}

	names := []string{"ASCII", "PACKED", "INSTRUCTION", "TYPE"}
	for _, q := range integerSizes {
		names = append(names, q.name)
	}

	for _, q := range floatFormats {
		names = append(names, q.name)
	}

	for _, q := range simpleTypes {
		names = append(names, q.name)
	}

	if short, ok := map[string]string{"AC": "ASCIC", "AD": "ASCID", "AW": "ASCIW", "AZ": "ASCIZ"}[word]; ok {
		word = short
	}

	name, err := uniqueKeyword(word, names)
	if err != nil {
		return dataType{}, err
	}

	switch name {
	case "TYPE":
		return d.symbolType(value)

	case "INSTRUCTION":
		return dataType{kind: typeInstruction}, nil

	case "ASCII":
		t := dataType{kind: typeASCII, count: 4, counted: true}

		if hasCount {
			n, err := d.typeCount(count)
			if err != nil {
				return dataType{}, err
			}

			t.count = n
		}

		return t, nil

	case "PACKED":
		if !hasCount {
			return dataType{}, vmserrors.New(vmserrors.DBG_SYNTAX, word)
		}

		n, err := d.typeCount(count)
		if err != nil {
			return dataType{}, err
		}

		return packedType(int64(n))
	}

	if hasCount || value != "" {
		return dataType{}, vmserrors.New(vmserrors.DBG_SYNTAX, strings.ToUpper(text))
	}

	for _, q := range integerSizes {
		if q.name == name {
			return dataType{kind: typeInteger, size: q.size}, nil
		}
	}

	for _, q := range floatFormats {
		if q.name == name {
			return dataType{kind: typeFloat, float: q.format}, nil
		}
	}

	for _, q := range simpleTypes {
		if q.name == name {
			return dataType{kind: q.kind}, nil
		}
	}

	return dataType{}, vmserrors.New(vmserrors.DBG_SYNTAX, word)
}

// typeCount reads the count of SET TYPE ASCII:n or PACKED:n, which the
// manual says is decimal whatever the radix.
func (d *Debugger) typeCount(text string) (uint32, error) {
	return d.Console.EvalWholeIn(10, strings.TrimSpace(text))
}

// uniqueKeyword finds the name word spells, or abbreviates uniquely.
func uniqueKeyword(word string, names []string) (string, error) {
	var matches []string

	for _, name := range names {
		if name == word {
			return name, nil
		}

		if strings.HasPrefix(name, word) {
			matches = append(matches, name)
		}
	}

	switch len(matches) {
	case 0:
		return "", vmserrors.New(vmserrors.DBG_SYNTAX, word)
	case 1:
		return matches[0], nil
	}

	return "", vmserrors.New(vmserrors.CLI_AMBIGUOUS, "keyword", word)
}

// typeName is how SHOW TYPE words a type. The manual shows "long integer"
// and "quadword integer"; the other names follow them (unconfirmed).
func (t dataType) typeName() string {
	switch t.kind {
	case typeInteger:
		return map[uint32]string{
			1: "byte integer", 2: "word integer", 4: "long integer",
			8: "quadword integer", 16: "octaword integer",
		}[t.size]
	case typeFloat:
		return t.float.String() + "_floating"
	case typePacked:
		return fmt.Sprintf("packed decimal, %d digits", t.digits)
	case typeASCII:
		return fmt.Sprintf("ascii string, length %d", t.count)
	case typeASCIC:
		return "counted ascii string"
	case typeASCIW:
		return "word counted ascii string"
	case typeASCIZ:
		return "zero terminated ascii string"
	case typeASCID:
		return "ascii string descriptor"
	case typeDateTime:
		return "date and time"
	case typeCondition:
		return "condition value"
	case typePSL:
		return "processor status longword"
	case typePSW:
		return "processor status word"
	case typePTE:
		return "page table entry"
	case typeInstruction:
		return "instruction"
	case typeSymbol:
		return "type of " + t.symbol
	}

	return "unknown"
}

// datumType is the type of a datum the program's symbols describe, for
// the types this file can show that the older code (data.go) doesn't: a
// floating-point number, and an octaword. ok is false for the others,
// which data.go shows by their size as it always has.
func datumType(datum *dbgsym.Datum) (dataType, bool) {
	if datum == nil || datum.IsArray() || datum.IsText() {
		return dataType{}, false
	}

	switch datum.Type {
	case dtypeF:
		return dataType{kind: typeFloat, float: vaxfloat.F}, true
	case dtypeD:
		return dataType{kind: typeFloat, float: vaxfloat.D}, true
	case dtypeG:
		return dataType{kind: typeFloat, float: vaxfloat.G}, true
	case dtypeH:
		return dataType{kind: typeFloat, float: vaxfloat.H}, true
	case dtypeO, dtypeOU:
		return dataType{kind: typeInteger, size: 16}, true
	case dtypeP:
		if datum.Descriptor != nil {
			return dataType{kind: typePacked, digits: uint32(datum.Descriptor.Length)}, true
		}
	}

	return dataType{}, false
}

// The calling standard's data type codes (DSC$K_DTYPE_*) datumType reads.
const (
	dtypeF  = 10 // F_floating
	dtypeD  = 11 // D_floating
	dtypeP  = 21 // packed decimal
	dtypeOU = 25 // unsigned octaword
	dtypeO  = 26 // signed octaword
	dtypeG  = 27 // G_floating
	dtypeH  = 28 // H_floating
)

// littleEndian reads data as an unsigned little-endian integer of any
// length (an octaword is 16 bytes, more than a uint64 holds).
func littleEndian(data []byte) *big.Int {
	v := new(big.Int)

	for i := len(data) - 1; i >= 0; i-- {
		v.Lsh(v, 8).Or(v, big.NewInt(int64(data[i])))
	}

	return v
}

// formatBig is v, a value of size bytes, in radix, laid out as
// formatRadix lays out the shorter integers.
func formatBig(v *big.Int, size uint32, radix int) string {
	if v.IsUint64() && size <= 8 {
		return formatRadix(v.Uint64(), size, radix)
	}

	bits := int(size) * 8

	switch radix {
	case 10:
		return v.String()
	case 8:
		return fmt.Sprintf("%0*s", (bits+2)/3, v.Text(8))
	case 2:
		text := fmt.Sprintf("%0*s", bits, v.Text(2))

		var groups []string

		for len(text) > 16 {
			groups = append(groups, text[:16])
			text = text[16:]
		}

		return strings.Join(append(groups, text), " ")
	}

	return fmt.Sprintf("%0*s", size*2, strings.ToUpper(v.Text(16)))
}

// floatBits packs data, a floating value's bytes as they are in memory,
// into the form vaxfloat reads.
func floatBits(data []byte) vaxfloat.Bits {
	var b vaxfloat.Bits

	for i := len(data) - 1; i >= 0; i-- {
		if i >= 8 {
			b.Hi = b.Hi<<8 | uint64(data[i])
		} else {
			b.Lo = b.Lo<<8 | uint64(data[i])
		}
	}

	return b
}

// bitsBytes is the reverse of floatBits: a format's value as size bytes.
func bitsBytes(b vaxfloat.Bits, size int) []byte {
	data := make([]byte, size)

	for i := range data {
		if i >= 8 {
			data[i] = byte(b.Hi >> (8 * (i - 8)))
		} else {
			data[i] = byte(b.Lo >> (8 * i))
		}
	}

	return data
}

// floatDigits is the number of significant digits the debugger shows a
// format's value with in fixed notation; in exponent notation it shows one
// more. The manual's examples show F_floating as 5350.000, 0.2407524, and
// 1.9117807E-38, D_floating as 2.35600000000000, and G_floating as
// 1.509506018605227E-300; H's is chosen by the same rule (unconfirmed).
func floatDigits(f vaxfloat.Format) int {
	switch f {
	case vaxfloat.F:
		return 7
	case vaxfloat.H:
		return 33
	}

	return 15
}

// formatFloat shows a floating value the way the manual's examples do, as
// FORTRAN's G format would: in fixed notation with floatDigits significant
// digits when its magnitude is from 0.1 up to 10 to that power, and in
// exponent notation with one more digit otherwise (1.9117807E-38). A
// reserved operand (the sign bit set with a zero exponent, which no
// arithmetic produces) is named, as the manual names one.
func formatFloat(f vaxfloat.Format, data []byte) string {
	v, err := vaxfloat.Unpack(f, floatBits(data))
	if err != nil {
		return "Reserved operand"
	}

	digits := floatDigits(f)

	if v.IsZero() {
		return "0." + strings.Repeat("0", digits)
	}

	x := new(big.Float).SetPrec(f.Precision() + 8)
	if _, ok := x.SetString(v.String()); !ok {
		return v.String()
	}

	abs := new(big.Float).Abs(x)
	low := big.NewFloat(0.1)
	high := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil))

	if abs.Cmp(low) >= 0 && abs.Cmp(high) < 0 {
		// The number of digits before the point decides how many go after
		// it, so that digits are shown in all (0.2407524 counts none
		// before).
		whole := len(new(big.Int).Abs(intPart(x)).String())
		if intPart(x).Sign() == 0 {
			whole = 0
		}

		return x.Text('f', digits-whole)
	}

	text := x.Text('E', digits)

	// Go writes the exponent with at least two digits and its sign
	// (E-38, E+10), which is the manual's layout too.
	return text
}

// intPart is x truncated toward zero.
func intPart(x *big.Float) *big.Int {
	i, _ := x.Int(nil)

	return i
}

// parseFloatValue reads the value DEPOSIT/FLOAT and its kind store: a
// floating literal (1.5, -2.5E3) or, failing that, an expression in the
// input radix, whose integer value is converted.
func (d *Debugger) parseFloatValue(f vaxfloat.Format, text string) ([]byte, error) {
	text = strings.TrimSpace(text)

	v, err := vaxfloat.Parse(text)
	if err != nil {
		n, evalErr := d.Console.EvalWholeIn(d.inputRadix, text)
		if evalErr != nil {
			return nil, evalErr
		}

		v = vaxfloat.FromInt(int64(int32(n)))
	}

	rounded, err := vaxfloat.Round(f, v)
	if err != nil {
		return nil, vmserrors.New(vmserrors.DBG_IVALOUTBNDS, strings.ToUpper(text))
	}

	b, err := vaxfloat.Pack(f, rounded)
	if err != nil {
		return nil, vmserrors.New(vmserrors.DBG_IVALOUTBNDS, strings.ToUpper(text))
	}

	return bitsBytes(b, f.Size()), nil
}

// formatPacked shows a packed decimal string of digits digits: its value
// as a signed decimal number, without leading zeros (unconfirmed). Each
// byte holds two digits, the most significant first; the last byte's low
// nibble is the sign, where 0xB and 0xD mean minus and the others plus. A
// nibble over 9 isn't a digit; it counts its own value, as the VAX's
// decimal instructions read it.
func formatPacked(data []byte, digits uint32) string {
	nibbles := make([]byte, 0, len(data)*2)

	for _, b := range data {
		nibbles = append(nibbles, b>>4, b&0xF)
	}

	sign := nibbles[len(nibbles)-1]
	nibbles = nibbles[len(nibbles)-1-int(digits) : len(nibbles)-1]

	v := new(big.Int)
	for _, n := range nibbles {
		v.Mul(v, big.NewInt(10)).Add(v, big.NewInt(int64(n)))
	}

	if (sign == 0xB || sign == 0xD) && v.Sign() != 0 {
		v.Neg(v)
	}

	return v.String()
}

// packDecimal lays v out as a packed decimal string of digits digits,
// with the preferred signs (0xC plus, 0xD minus). A value with more digits
// than that doesn't fit, and is %DEBUG-I-IVALOUTBNDS.
func packDecimal(v int64, digits uint32, text string) ([]byte, error) {
	neg := v < 0
	if neg {
		v = -v
	}

	s := fmt.Sprint(v)
	if v == 0 {
		s = ""
	}

	if uint32(len(s)) > digits {
		return nil, vmserrors.New(vmserrors.DBG_IVALOUTBNDS, strings.ToUpper(text))
	}

	nibbles := make([]byte, digits/2*2+1-uint32(len(s)), digits/2*2+2)
	for _, ch := range s {
		nibbles = append(nibbles, byte(ch-'0'))
	}

	sign := byte(0xC)
	if neg {
		sign = 0xD
	}

	nibbles = append(nibbles, sign)

	data := make([]byte, len(nibbles)/2)
	for i := range data {
		data[i] = nibbles[2*i]<<4 | nibbles[2*i+1]
	}

	return data, nil
}

// pswTable is the processor status word's fields in the debugger's
// layout: the low word of the PSL, the trap enables and condition codes,
// under the same headings as the PSL's table (pslTable) uses for them
// (unconfirmed: the manual says only that /PSW is /PSL's low word).
func pswTable(psw uint32) string {
	bit := func(n uint) uint32 { return psw >> n & 1 }

	return "        DV FU IV T N Z V C\n" +
		fmt.Sprintf("         %d  %d  %d %d %d %d %d %d\n",
			bit(7), bit(6), bit(5), bit(4), bit(3), bit(2), bit(1), bit(0))
}

// fixedText is what EXAMINE shows for data, the bytes of a type with a
// fixed size (fixedSize), in radix where the type is an integer. The
// PSL's and PSW's tables have several lines; multiline says to put them
// below the location's name rather than after it.
func (d *Debugger) fixedText(t dataType, data []byte, radix int) (text string, multiline bool) {
	switch t.kind {
	case typeFloat:
		return formatFloat(t.float, data), false

	case typePacked:
		return formatPacked(data, t.digits), false

	case typeDateTime:
		v := littleEndian(data).Uint64()

		text, ok := corevms.FormatTime(v, false)
		if !ok {
			return formatRadix(v, 8, radix), false
		}

		return strings.TrimLeft(text, " "), false

	case typeCondition:
		return d.Console.StatusText(uint32(littleEndian(data).Uint64())), false

	case typePSL:
		return pslTable(uint32(littleEndian(data).Uint64())), true

	case typePSW:
		return pswTable(uint32(littleEndian(data).Uint64())), true

	case typePTE:
		return d.Console.FormatPTE(uint32(littleEndian(data).Uint64())), false
	}

	return formatBig(littleEndian(data), uint32(len(data)), radix), false
}

// bindTypes binds SET TYPE, SHOW TYPE, and CANCEL TYPE.
func (d *Dispatcher) bindTypes() {
	g := d.Grammar

	g.Bind("SET_TYPE", func(id int64, r *dcl.Result) error { return d.Debugger.setType(r) })
	g.Bind("SHOW_TYPE", func(id int64, r *dcl.Result) error { return d.Debugger.showType(r) })
	g.Bind("CANCEL_TYPE", func(id int64, r *dcl.Result) error {
		// CANCEL TYPE/OVERRIDE is the manual's form. Without /OVERRIDE
		// govax does the same (unconfirmed): the default type has no
		// CANCEL, only SET TYPE LONGWORD.
		d.Debugger.overrideType = nil

		return nil
	})
}

// setType implements SET TYPE[/OVERRIDE] keyword: the type of the
// locations no program symbol types, or, with /OVERRIDE, of every
// location.
func (d *Debugger) setType(r *dcl.Result) error {
	t, err := d.parseTypeKeyword(r.String("KEYWORD"))
	if err != nil {
		return err
	}

	if r.Present("OVERRIDE") {
		d.overrideType = &t

		return nil
	}

	// LONGWORD is the default, which other code takes nil to mean.
	if t.kind == typeInteger && t.size == 4 {
		d.defaultType = nil
	} else {
		d.defaultType = &t
	}

	return nil
}

// showType implements SHOW TYPE[/OVERRIDE], in the manual's words:
//
//	type: long integer
//	type/override: none
func (d *Debugger) showType(r *dcl.Result) error {
	if r.Present("OVERRIDE") {
		name := "none"
		if d.overrideType != nil {
			name = d.overrideType.typeName()
		}

		d.Console.Printf("type/override: %s\n", name)

		return nil
	}

	t := longwordType
	if d.defaultType != nil {
		t = *d.defaultType
	}

	d.Console.Printf("type: %s\n", t.typeName())

	return nil
}
