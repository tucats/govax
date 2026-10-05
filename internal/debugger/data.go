package debugger

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is the debugger's data commands (docs/PHASE-42.md, subtask 10):
// EXAMINE of data (a register or memory), DEPOSIT, EVALUATE, and SYMBOLIZE.
// EXAMINE/INSTRUCTION, the other half of EXAMINE, is in examine.go.
//
// Background for a reader new to the VMS debugger. The debugger shows a
// location in memory as
//
//	DBGCMD\WATCHL:  00000000
//
// the location's name (module, then symbol), a colon, a tab, and the
// contents. What the contents mean depends on the *type* of the data: the
// assembler's data directives (.LONG, .BYTE, .ASCII, ...) give each label a
// type in the program's debug symbol table, and the debugger shows a label
// by its type: a longword as eight hex digits, a byte as two, a string in
// quotes, an array one element to a line. An address that no label names
// is shown as a longword, the debugger's default type, and named by the
// nearest label before it and the distance (DBGCMD\WATCHL+3).
//
// The registers are locations too: EXAMINE R2 is named by the routine the
// program is in (DBGCMD\FACT\%R2).

// defaultSize is the size in bytes of an untyped location: a longword.
const defaultSize = 4

// examineState is the debugger's idea of the "current location", which
// EXAMINE with no location (the next one), "." (this one), and "^" (the
// one before) are relative to: where the last EXAMINE looked and how big
// the thing it showed was.
type examineState struct {
	addr uint32
	size uint32
	set  bool
}

// registerLocations are the names of the machine's general registers, as
// a user types them (with or without the "%"), and their numbers.
var registerLocations = map[string]vax.Reg{
	"R0": vax.R0, "R1": vax.R1, "R2": vax.R2, "R3": vax.R3,
	"R4": vax.R4, "R5": vax.R5, "R6": vax.R6, "R7": vax.R7,
	"R8": vax.R8, "R9": vax.R9, "R10": vax.R10, "R11": vax.R11,
	"R12": vax.R12, "R13": vax.R13, "R14": vax.R14, "R15": vax.R15,
	"AP": vax.AP, "FP": vax.FP, "SP": vax.SP, "PC": vax.PC,
}

// registerOf reports which register text names, if it names one. The PSL
// isn't a general register, so isPSL says it separately.
func registerOf(text string) (reg vax.Reg, name string, isPSL, ok bool) {
	name = strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(text), "%"))

	if name == "PSL" {
		return 0, name, true, true
	}

	reg, ok = registerLocations[name]

	return reg, name, false, ok
}

// tabPad pads text with blanks to the next tab stop (a multiple of 8
// columns), which is the layout a tab character gives and the one the
// debugger's logs show: the value after "DBGCMD\WATCHL:" starts in column
// 16. A label that already ends at a tab stop gets a whole tab.
func tabPad(text string) string {
	return text + strings.Repeat(" ", 8-len(text)%8)
}

// formatRadix is v, a value of size bytes, in radix: hexadecimal padded
// to the size (two digits a byte), octal padded to its width (11 digits
// for a longword), binary in groups of 16 bits, and decimal unpadded.
func formatRadix(v uint64, size uint32, radix int) string {
	bits := int(size) * 8

	switch radix {
	case 10:
		return fmt.Sprintf("%d", v)
	case 8:
		return fmt.Sprintf("%0*o", (bits+2)/3, v)
	case 2:
		text := fmt.Sprintf("%0*b", bits, v)

		var groups []string

		for len(text) > 16 {
			groups = append(groups, text[:16])
			text = text[16:]
		}

		return strings.Join(append(groups, text), " ")
	}

	return fmt.Sprintf("%0*X", size*2, v)
}

// examineRadix is the radix a data value is shown in: the one the command
// asked for (/DECIMAL, ...), else the session's output radix.
func (d *Debugger) examineRadix(r *dcl.Result) int {
	switch {
	case r.Present("HEXADECIMAL"):
		return 16
	case r.Present("DECIMAL"):
		return 10
	case r.Present("OCTAL"):
		return 8
	case r.Present("BINARY"):
		return 2
	}

	return d.outputRadix
}

// registerScope is the path the register names are written under: the
// module and routine the PC is in (DBGCMD\FACT), or "" when it isn't in a
// routine the debug symbols know.
func (d *Debugger) registerScope() string {
	pc := d.Console.CPU.GPR(vax.PC)

	prog := d.Console.DebugProgramAt(pc)
	if prog == nil {
		return ""
	}

	routine, module, ok := prog.RoutineAt(pc)
	if !ok {
		return ""
	}

	return dbgsym.DisplayScope(module.Name + `\` + routine.Name)
}

// locationName is how EXAMINE labels the memory at addr: the data symbol
// that names it (with the offset past it), or the program's symbol for the
// address (a routine's code), or just the address in hex.
func (d *Debugger) locationName(addr uint32) string {
	if prog := d.Console.DebugProgramAt(addr); prog != nil {
		if name, ok := prog.DataName(addr, d.symbolRadix()); ok {
			return name
		}

		if name, ok := prog.Symbolize(addr, d.symbolRadix()); ok {
			return name
		}
	}

	return fmt.Sprintf("%08X", addr)
}

// examineData runs EXAMINE for the data forms: each location in the list
// (or the current one) is shown by its type.
func (d *Dispatcher) examineData(r *dcl.Result) error {
	dbg := d.Debugger

	items := []string{""}

	if text := strings.TrimSpace(r.String("LOCATION")); text != "" {
		var err error

		if items, err = splitList(text); err != nil {
			return err
		}
	}

	for _, item := range items {
		if err := dbg.examineItem(r, item); err != nil {
			return err
		}
	}

	return nil
}

// examineItem shows one item of EXAMINE's list: a register, a location, a
// range of them (start:end), or, with nothing, the next location after
// the last one shown ("." the same one again, "^" the one before it).
func (d *Debugger) examineItem(r *dcl.Result, item string) error {
	if reg, name, isPSL, ok := registerOf(item); ok {
		return d.examineRegister(r, reg, name, isPSL)
	}

	cur := d.examined

	switch item {
	case "", ".", "^":
		if !cur.set {
			cur = examineState{addr: d.Console.DepositAddr, size: defaultSize, set: true}
		}

		switch item {
		case "":
			return d.examineAt(r, cur.addr+cur.size, false)
		case ".":
			return d.examineAt(r, cur.addr, false)
		default:
			return d.examineAt(r, cur.addr-cur.size, false)
		}
	}

	if lo, hi, ok := splitRange(item); ok {
		first, err := d.evalWhole(lo)
		if err != nil {
			return err
		}

		last, err := d.evalWhole(hi)
		if err != nil {
			return err
		}

		// A range steps by the size the command gives, or, without one, by a
		// longword, however big each thing in it is, as the probe shows:
		// EXAMINE 200:20C lists WATCHL (4 bytes), then WATCHB (1 byte), and
		// the byte-array BUFFER's elements 0 and 4, four bytes apart.
		step := uint32(defaultSize)
		if size, typed := d.typedSize(r); typed && size > 0 {
			step = size
		}

		for addr := first; addr <= last; addr += step {
			if err := d.examineAt(r, addr, true); err != nil {
				return err
			}

			if addr+step < addr {
				break
			}
		}

		return nil
	}

	addr, err := d.evalWhole(item)
	if err != nil {
		return err
	}

	// A subscripted name (BUFFER[2]) is one element, not the array.
	return d.examineAt(r, addr, strings.Contains(item, "["))
}

// typedSize is the size a type qualifier gives the items shown: /BYTE,
// /WORD, /LONGWORD, /QUADWORD, or (for /PSL and /PTE) a longword. typed is
// false with none of them. /ASCII has no size of its own; its count is
// asciiCount's.
func (d *Debugger) typedSize(r *dcl.Result) (size uint32, typed bool) {
	switch {
	case r.Present("BYTE"):
		return 1, true
	case r.Present("WORD"):
		return 2, true
	case r.Present("LONGWORD"), r.Present("PSL"), r.Present("PTE"):
		return 4, true
	case r.Present("QUADWORD"):
		return 8, true
	}

	return 0, false
}

// asciiGiven reports whether the command has /ASCII. (A qualifier with a
// default value always counts as present, and Defaulted says it was left
// out.)
func asciiGiven(r *dcl.Result) bool { return r.Present("ASCII") && !r.Defaulted("ASCII") }

// asciiCount is /ASCII:n's count of characters, read in the input radix
// (so "/ASCII:16" is 22 characters by default, as in the probe); ok is
// false when /ASCII has no count.
func (d *Debugger) asciiCount(text string) (n uint32, ok bool, err error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false, nil
	}

	n, err = d.evalWhole(text)

	return n, err == nil, err
}

// readValue reads a size-byte little-endian number (a VAX stores the
// least significant byte first) at addr.
func (d *Debugger) readValue(addr, size uint32) (uint64, error) {
	data, err := d.Console.ReadBytes(addr, size)
	if err != nil {
		return 0, err
	}

	var v uint64

	for i := len(data) - 1; i >= 0; i-- {
		v = v<<8 | uint64(data[i])
	}

	return v, nil
}

// examineRegister shows a register as "DBGCMD\FACT\%R2:  00000004", or the
// processor status longword as a table of its fields.
func (d *Debugger) examineRegister(r *dcl.Result, reg vax.Reg, name string, isPSL bool) error {
	c := d.Console

	label := "%" + name
	if scope := d.registerScope(); scope != "" {
		label = scope + `\` + label
	}

	label = tabPad(label + ":")

	if isPSL || r.Present("PSL") {
		value := uint32(c.CPU.PSL())
		if !isPSL {
			value = c.CPU.GPR(reg)
		}

		c.Printf("%s\n%s", label, pslTable(value))

		return nil
	}

	value := uint64(c.CPU.GPR(reg))

	if r.Present("PTE") {
		c.Printf("%s%s\n", label, c.FormatPTE(uint32(value)))

		return nil
	}

	size := uint32(defaultSize)
	if s, typed := d.typedSize(r); typed {
		size = s
	}

	if size < 4 {
		value &= 1<<(size*8) - 1
	}

	c.Printf("%s%s\n", label, formatRadix(value, size, d.examineRadix(r)))

	return nil
}

// pslNames are the access modes' names in the PSL's two mode fields
// (current and previous), and the layout the debugger shows the PSL in:
// the first line names each field, the second gives its value under the
// name. Each value sits at a fixed column below its name's, as the
// probe's log shows.
var pslModes = [4]string{"KERNEL", "EXEC", "SUPER", "USER"}

// pslTable is the PSL's fields in the debugger's table. The processor
// status longword holds the condition codes (N, Z, V, C), the trap
// enables (T, IV, FU, DV), the interrupt priority level (IPL), the
// current and previous access modes, and the flags for the interrupt
// stack (IS), first part done (FPD), trace pending (TP), and compatibility
// mode (CM).
func pslTable(psl uint32) string {
	bit := func(n uint) uint32 { return psl >> n & 1 }
	mode := func(shift uint) string { return pslModes[psl>>shift&3] }

	// Each field's text is placed so that it ends at (or, for the mode
	// names, is centered under) the column its name does.
	row := make([]byte, 58)
	for i := range row {
		row[i] = ' '
	}

	put := func(col int, text string) { copy(row[col:], text) }

	put(9, fmt.Sprintf("%d", bit(31)))    // CMP
	put(13, fmt.Sprintf("%d", bit(30)))   // TP
	put(16, fmt.Sprintf("%d", bit(27)))   // FPD
	put(20, fmt.Sprintf("%d", bit(26)))   // IS
	put(22+(6-len(mode(24)))/2, mode(24)) // CURMOD
	put(29+(6-len(mode(22)))/2, mode(22)) // PRVMOD
	ipl := fmt.Sprintf("%X", psl>>16&31)  // IPL, in hex, ending under IPL's last letter
	put(39-len(ipl), ipl)
	put(41, fmt.Sprintf("%d", bit(7))) // DV
	put(44, fmt.Sprintf("%d", bit(6))) // FU
	put(47, fmt.Sprintf("%d", bit(5))) // IV
	put(49, fmt.Sprintf("%d", bit(4))) // T
	put(51, fmt.Sprintf("%d", bit(3))) // N
	put(53, fmt.Sprintf("%d", bit(2))) // Z
	put(55, fmt.Sprintf("%d", bit(1))) // V
	put(57, fmt.Sprintf("%d", bit(0))) // C

	return "        CMP TP FPD IS CURMOD PRVMOD IPL DV FU IV T N Z V C\n" + string(row) + "\n"
}

// examineAt shows the memory at addr, by its type: the one a type
// qualifier gives, else the debug symbols' for a label exactly at addr,
// else a longword. inRange is true for an item of a range (start:end),
// which shows an array's single element, not the whole array.
func (d *Debugger) examineAt(r *dcl.Result, addr uint32, inRange bool) error {
	c := d.Console
	prog := c.DebugProgramAt(addr)

	var datum *dbgsym.Datum

	if prog != nil {
		datum, _, _ = prog.DatumAt(addr)

		// An address inside an array is typed by its elements.
		if datum == nil {
			datum, _ = prog.ElementDatumAt(addr)
			inRange = true
		}
	}

	// /ASCII, or a label of a string: show text.
	if asciiGiven(r) || (datum != nil && datum.IsText() && !r.Present("BYTE") &&
		!r.Present("WORD") && !r.Present("LONGWORD") && !r.Present("QUADWORD") && !r.Present("PTE") && !r.Present("PSL")) {
		return d.examineText(r, addr, datum)
	}

	size, typed := d.typedSize(r)

	switch {
	case typed:
	case datum != nil && datum.ElementSize() > 0 && datum.ElementSize() <= 8:
		size = datum.ElementSize()
	default:
		size = defaultSize
	}

	// A label of an array, named alone, shows every element.
	if !typed && !inRange && datum != nil && datum.IsArray() && len(datum.Descriptor.Bounds) == 1 {
		return d.examineArray(r, addr, datum)
	}

	value, err := d.readValue(addr, size)
	if err != nil {
		return err
	}

	d.examined = examineState{addr: addr, size: size, set: true}
	c.DepositAddr = addr + size

	text := formatRadix(value, size, d.examineRadix(r))
	if r.Present("PTE") {
		text = c.FormatPTE(uint32(value))
	}

	if r.Present("PSL") {
		c.Printf("%s\n%s", tabPad(d.locationName(addr)+":"), pslTable(uint32(value)))

		return nil
	}

	c.Printf("%s%s\n", tabPad(d.locationName(addr)+":"), text)

	return nil
}

// examineArray shows an array one element to a line, under a heading of
// its name and bounds:
//
//	DBGCMD\BUFFER[0:15]
//	    [0]:        00
//	    [1]:        00
func (d *Debugger) examineArray(r *dcl.Result, addr uint32, datum *dbgsym.Datum) error {
	c := d.Console
	bounds := datum.Descriptor.Bounds[0]
	size := datum.ElementSize()
	name := strings.SplitN(d.locationName(addr), "[", 2)[0]

	c.Printf("%s[%d:%d]\n", name, bounds.Lower, bounds.Upper)

	for i := bounds.Lower; i <= bounds.Upper; i++ {
		at := addr + uint32(i-bounds.Lower)*size

		value, err := d.readValue(at, size)
		if err != nil {
			return err
		}

		c.Printf("%s%s\n", tabPad(fmt.Sprintf("    [%d]:", i)), formatRadix(value, size, d.examineRadix(r)))
	}

	d.examined = examineState{addr: addr, size: size * uint32(bounds.Upper-bounds.Lower+1), set: true}

	return nil
}

// examineText shows data as a string in quotes: the count /ASCII:n gives,
// else the length the label's descriptor has (.ASCID's label names a
// descriptor, whose pointer says where the text is), else one character.
// A byte that isn't a printable character is shown as a period.
func (d *Debugger) examineText(r *dcl.Result, addr uint32, datum *dbgsym.Datum) error {
	c := d.Console
	name := d.locationName(addr)
	start := addr

	count, haveCount, err := d.asciiCount(r.String("ASCII"))
	if err != nil {
		return err
	}

	length := uint32(1)

	if datum != nil {
		switch {
		case datum.Kind == dbgsym.DescriptorAddress:
			// The label is a descriptor: length in its first word, the
			// text's address in its second longword.
			header, err := c.ReadBytes(addr, 8)
			if err != nil {
				return err
			}

			length = uint32(header[0]) | uint32(header[1])<<8
			start = uint32(header[4]) | uint32(header[5])<<8 | uint32(header[6])<<16 | uint32(header[7])<<24
		case datum.IsText() && datum.Descriptor != nil:
			length = uint32(datum.Descriptor.Length)
		case datum.IsArray() && len(datum.Descriptor.Bounds) == 1:
			b := datum.Descriptor.Bounds[0]
			length = uint32(b.Upper-b.Lower+1) * max(datum.ElementSize(), 1)
		}

		if datum.IsArray() && len(datum.Descriptor.Bounds) == 1 {
			b := datum.Descriptor.Bounds[0]
			name = fmt.Sprintf("%s[%d:%d]", strings.SplitN(name, "[", 2)[0], b.Lower, b.Upper)
		}
	}

	if haveCount {
		length = count
	}

	data, err := c.ReadBytes(start, length)
	if err != nil {
		return err
	}

	for i, b := range data {
		if b < ' ' || b > '~' {
			data[i] = '.'
		}
	}

	d.examined = examineState{addr: addr, size: max(length, 1), set: true}
	c.DepositAddr = addr + max(length, 1)

	c.Printf("%s'%s'\n", tabPad(name+":"), data)

	return nil
}

// evaluate runs EVALUATE[/radix] expression and EVALUATE/ADDRESS
// expression, which shows an expression's value: for a name of data, what
// is stored there, or with /ADDRESS where it is.
func (d *Dispatcher) evaluate(r *dcl.Result) error {
	dbg := d.Debugger

	if err := dbg.Console.RequireInit(); err != nil {
		return err
	}

	text := strings.TrimSpace(r.String("EXPRESSION"))

	value, err := dbg.Console.EvalWholeMode(dbg.inputRadix, text, !r.Present("ADDRESS"))
	if err != nil {
		return err
	}

	radix := dbg.examineRadix(r)
	out := formatRadix(uint64(value), 4, radix)

	// A hexadecimal number that begins with a letter is written with a
	// leading zero, so that it reads as a number (0FFFFFFFF).
	if radix == 16 && out[0] >= 'A' {
		out = "0" + out
	}

	dbg.Console.Printf("%s\n", out)

	return nil
}

// symbolize runs SYMBOLIZE address, which lists every name the debugger has
// for an address, as the probe's logs show:
//
//	address 00000402:
//	    DBGDIS\START+2
//	    DBGDIS\START\%LINE 42
//	address 00000402: (global)
//	    START+2
//
// The first block is the module symbols and lines; the second, only for an
// image linked with a global symbol table (LINK/DEBUG), is the global
// symbol table's.
func (d *Dispatcher) symbolize(r *dcl.Result) error {
	dbg := d.Debugger

	if err := dbg.Console.RequireInit(); err != nil {
		return err
	}

	addr, err := dbg.evalWhole(r.String("ADDRESS"))
	if err != nil {
		return err
	}

	names, global := dbg.Console.DebugProgramAt(addr).SymbolizeNames(addr, dbg.symbolRadix())

	// An address no module names gets only the heading, as does an
	// address in no image (govax's choice; the probe has no such case).
	dbg.Console.Printf("address %08X: \n", addr)

	for _, name := range names {
		dbg.Console.Printf("    %s\n", name)
	}

	if global != "" {
		dbg.Console.Printf("address %08X: (global)\n    %s\n", addr, global)
	}

	return nil
}

// deposit runs DEPOSIT[/type] location = value: stores a number (read in
// the input radix) of the location's type, or, with /ASCII, a string, at a
// register or an address.
func (d *Dispatcher) deposit(r *dcl.Result) error {
	dbg := d.Debugger
	c := dbg.Console

	if err := c.RequireInit(); err != nil {
		return err
	}

	target, value, ok := splitAssignment(r.String("ASSIGNMENT"))
	if !ok {
		return vmserrors.New(vmserrors.CLI_NEEDEXPR)
	}

	// A register takes a longword.
	if reg, _, isPSL, isReg := registerOf(target); isReg && !isPSL {
		v, err := dbg.Console.EvalWholeIn(dbg.inputRadix, value)
		if err != nil {
			return err
		}

		c.CPU.SetGPR(reg, v)

		return nil
	}

	addr, err := dbg.evalWhole(target)
	if err != nil {
		return err
	}

	// The type of what's being stored: from the command, else from the
	// label at the address (DEPOSIT WATCHB = 1 stores a byte), else a
	// longword.
	size, typed := dbg.typedSize(r)

	if !typed {
		size = defaultSize

		if prog := c.DebugProgramAt(addr); prog != nil {
			if datum, _, found := prog.DatumAt(addr); found && datum.ElementSize() > 0 && datum.ElementSize() <= 8 {
				size = datum.ElementSize()
			}
		}
	}

	if asciiGiven(r) {
		text := strings.TrimSpace(value)
		if len(text) >= 2 && (text[0] == '"' || text[0] == '\'') && text[len(text)-1] == text[0] {
			text = text[1 : len(text)-1]
		}

		data := []byte(text)

		if count, have, err := dbg.asciiCount(r.String("ASCII")); err != nil {
			return err
		} else if have {
			// The count says how many characters are stored: pad with
			// blanks, or cut short.
			for uint32(len(data)) < count {
				data = append(data, ' ')
			}

			data = data[:count]
		}

		dbg.examined = examineState{addr: addr, size: max(uint32(len(data)), 1), set: true}

		return c.WriteBytes(addr, data)
	}

	v, err := dbg.Console.EvalWholeIn(dbg.inputRadix, value)
	if err != nil {
		return err
	}

	data := make([]byte, size)
	for i := range data {
		if i < 4 {
			data[i] = byte(v >> (8 * i))
		}
	}

	dbg.examined = examineState{addr: addr, size: size, set: true}

	return c.WriteBytes(addr, data)
}

// splitAssignment splits DEPOSIT's "location = value" at its equal sign,
// the first one outside quotes and parentheses.
func splitAssignment(text string) (target, value string, ok bool) {
	depth := 0
	quote := byte(0)

	for i := 0; i < len(text); i++ {
		switch ch := text[i]; {
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == '"' || ch == '\'':
			quote = ch
		case ch == '(' || ch == '[':
			depth++
		case ch == ')' || ch == ']':
			depth--
		case ch == '=' && depth == 0:
			target, value = strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+1:])

			return target, value, target != "" && value != ""
		}
	}

	return "", "", false
}
