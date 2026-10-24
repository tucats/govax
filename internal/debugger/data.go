package debugger

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/registers"
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
var registerLocations = map[string]registers.Reg{
	"R0": registers.R0, "R1": registers.R1, "R2": registers.R2, "R3": registers.R3,
	"R4": registers.R4, "R5": registers.R5, "R6": registers.R6, "R7": registers.R7,
	"R8": registers.R8, "R9": registers.R9, "R10": registers.R10, "R11": registers.R11,
	"R12": registers.R12, "R13": registers.R13, "R14": registers.R14, "R15": registers.R15,
	"AP": registers.AP, "FP": registers.FP, "SP": registers.SP, "PC": registers.PC,
}

// registerOf reports which register text names, if it names one. The PSL
// isn't a general register, so isPSL says it separately.
func registerOf(text string) (reg registers.Reg, name string, isPSL, ok bool) {
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

// leadingDigit is a number as the debugger writes a value: a hexadecimal
// one that begins with a letter gets a leading zero, so that it reads as
// a number (EVALUATE's 0FFFFFFFF in exam.dlg; EXAMINE's 0FF170FFC and
// 0FF17 in dun72b.dlg).
func leadingDigit(text string) string {
	if text != "" && text[0] >= 'A' && text[0] <= 'F' {
		return "0" + text
	}

	return text
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
	pc := d.Machine.CPU.GPR(registers.PC)

	prog := d.Machine.DebugProgramAt(pc)
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
	if d.numericNames {
		return fmt.Sprintf("%08X", addr)
	}

	if prog := d.Machine.DebugProgramAt(addr); prog != nil {
		// A routine's start is named by the routine, which the data
		// symbols' nearest-label rule would otherwise miss (START is
		// DBGDIS\START, not DBGDIS\NOLAB2+19C).
		if _, isEntry := d.Machine.EntryMaskText(addr); isEntry {
			if name, ok := prog.Symbolize(addr, d.symbolRadix()); ok {
				return name
			}
		}

		if name, ok := prog.DataName(addr, d.symbolRadix()); ok {
			return name
		}

		if name, ok := prog.Symbolize(addr, d.symbolRadix()); ok {
			return name
		}
	}

	if name, ok := d.Machine.SharedName(addr, d.symbolRadix()); ok {
		return name
	}

	return fmt.Sprintf("%08X", addr)
}

// examineData runs EXAMINE for the data forms: each location in the list
// (or the current one) is shown by its type.
func (d *Dispatcher) examineData(r *dcl.Result) error {
	dbg := d.Debugger

	noSymbolic := r.Present("SYMBOLIC") && r.Negated("SYMBOLIC")

	items := []string{""}

	if text := strings.TrimSpace(r.String("LOCATION")); text != "" {
		var err error

		if items, err = splitList(text); err != nil {
			return err
		}
	}

	for _, item := range items {
		// /NOSYMBOLIC names a location typed as a number by its address
		// (the manual's "0000086F: 03020100"); one typed by name keeps its
		// name, as the probe's EXAMINE/NOSYMBOLIC WATCHL shows.
		dbg.numericNames = noSymbolic && numericItem(item)

		err := dbg.examineItem(r, item)

		dbg.numericNames = false

		if err != nil {
			return err
		}
	}

	return nil
}

// numericItem reports whether an EXAMINE item is a number, or a range of
// numbers, rather than a name: each part starts with a digit or a radix
// prefix (^X), as a MACRO number must.
func numericItem(item string) bool {
	parts := []string{item}
	if lo, hi, ok := splitRange(item); ok {
		parts = []string{lo, hi}
	}

	for _, part := range parts {
		if part = strings.TrimSpace(part); part == "" || !(part[0] >= '0' && part[0] <= '9' || part[0] == '^') {
			return false
		}
	}

	return true
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
			cur = examineState{addr: d.Machine.DepositAddr, size: defaultSize, set: true}
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

		// A type the qualifier, or SET TYPE/OVERRIDE, gives steps by its
		// own size where it has a fixed one.
		if t, typed, err := d.commandType(r); err != nil {
			return err
		} else if size, fixed := t.fixedSize(); typed && fixed && size > 0 {
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
	data, err := d.Machine.ReadBytes(addr, size)
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
// processor status longword as a table of its fields. A type qualifier (or
// SET TYPE/OVERRIDE) shows the register's contents as that type; one wider
// than a longword takes the registers after it too (EXAMINE/D_FLOAT R0 is
// R0 and R1, as the VAX keeps a D_floating value in a register pair).
func (d *Debugger) examineRegister(r *dcl.Result, reg registers.Reg, name string, isPSL bool) error {
	c := d.Machine

	label := "%" + name
	if scope := d.registerScope(); scope != "" {
		label = scope + `\` + label
	}

	label = tabPad(label + ":")

	t, typed, err := d.commandType(r)
	if err != nil {
		return err
	}

	if isPSL {
		value := uint32(c.CPU.PSL())

		if typed && t.kind == typePSW {
			c.Printf("%s\n%s", label, pswTable(value))
		} else {
			c.Printf("%s\n%s", label, pslTable(value))
		}

		return nil
	}

	if !typed {
		t = longwordType
	}

	data, err := d.registerBytes(reg, t, "EXAMINE")
	if err != nil {
		return err
	}

	d.setCurrentValue(data)

	text, multiline := d.fixedText(t, data, d.examineRadix(r))
	if multiline {
		c.Printf("%s\n%s", label, text)
	} else {
		c.Printf("%s%s\n", label, text)
	}

	return nil
}

// registerBytes returns the bytes of the registers from reg on that a
// value of type t fills, least significant first: one register for a
// longword or less, two for a quadword, four for an octaword. A type with
// no fixed size (a counted string) can't be in registers, and neither can
// a value that would run past the PC: %DEBUG-E-OPTNOTALLOW, naming the
// command (verb).
func (d *Debugger) registerBytes(reg registers.Reg, t dataType, verb string) ([]byte, error) {
	size, fixed := t.fixedSize()
	if !fixed || size == 0 || t.kind == typeInstruction {
		return nil, vmserrors.New(vmserrors.DBG_OPTNOTALLOW, verb)
	}

	count := (size + 3) / 4
	if uint32(reg)+count-1 > uint32(registers.PC) {
		return nil, vmserrors.New(vmserrors.DBG_OPTNOTALLOW, verb)
	}

	data := make([]byte, 0, count*4)

	for i := range count {
		v := d.Machine.CPU.GPR(reg + registers.Reg(i))
		data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
	}

	return data[:size], nil
}

// depositPSL is DEPOSIT to the PSL: a longword replaces it, and a byte or
// word (DEPOSIT/WORD PSL = 0, the VMS debugger manuals' way to clear the
// condition codes and trap enables) its low byte or word, the PSW, the
// rest left as it is. A type of another size is %DEBUG-E-OPTNOTALLOW.
// Setting T starts tracing: the next instruction runs, and the trace
// comes before the one after it (the VAX/VMS 2.0 Symbolic Debugger
// Reference: "setting the trace trap condition code may cause a T-bit
// pending trap"; traced.go). govax lets every bit be changed, as SET PSL
// does: VMS's debugger, a user-mode program, can't change the privileged
// half (unconfirmed what it does when asked to).
func (d *Debugger) depositPSL(t dataType, typed bool, value string) error {
	size := uint32(4)

	if typed {
		var fixed bool
		if size, fixed = t.fixedSize(); !fixed || t.kind == typeInstruction || (size != 1 && size != 2 && size != 4) {
			return vmserrors.New(vmserrors.DBG_OPTNOTALLOW, "DEPOSIT")
		}
	}

	var data []byte

	if typed {
		var err error
		if data, err = d.depositBytes(0, t, value); err != nil {
			return err
		}
	} else {
		v, err := d.Machine.EvalWholeIn(d.inputRadix, value)
		if err != nil {
			return err
		}

		data = []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
	}

	psl := uint32(d.Machine.CPU.PSL())

	for b := range min(size, uint32(len(data))) {
		psl = psl&^(0xFF<<(8*b)) | uint32(data[b])<<(8*b)
	}

	d.Machine.CPU.SetPSL(registers.PSL(psl))
	d.setCurrentValue(data)

	return nil
}

// setRegisterBytes stores data in the registers from reg on, as
// registerBytes reads them: a register that data only partly fills keeps
// its other bytes.
func (d *Debugger) setRegisterBytes(reg registers.Reg, data []byte) error {
	count := (uint32(len(data)) + 3) / 4
	if uint32(reg)+count-1 > uint32(registers.PC) {
		return vmserrors.New(vmserrors.DBG_OPTNOTALLOW, "DEPOSIT")
	}

	for i := range count {
		v := d.Machine.CPU.GPR(reg + registers.Reg(i))

		for b := range uint32(4) {
			if at := i*4 + b; at < uint32(len(data)) {
				v = v&^(0xFF<<(8*b)) | uint32(data[at])<<(8*b)
			}
		}

		d.Machine.CPU.SetGPR(reg+registers.Reg(i), v)
	}

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

	ipl := fmt.Sprintf("%X", psl>>16&31) // IPL, in hex, ending under IPL's last letter
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
// qualifier gives, else SET TYPE/OVERRIDE's, else the debug symbols' for a
// label exactly at addr, else SET TYPE's, else a longword. inRange is true
// for an item of a range (start:end), which shows an array's single
// element, not the whole array.
func (d *Debugger) examineAt(r *dcl.Result, addr uint32, inRange bool) error {
	c := d.Machine
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

	radix := d.examineRadix(r)

	t, typed, err := d.commandType(r)
	if err != nil {
		return err
	}

	if typed {
		return d.examineTyped(addr, t, datum, radix)
	}

	// A label of a string: show text.
	if datum != nil && datum.IsText() {
		return d.examineText(addr, datum, 0, false)
	}

	// A label of a type data.go's sizes don't show (a floating value).
	if dt, ok := datumType(datum); ok && !inRange {
		return d.examineTyped(addr, dt, datum, radix)
	}

	// A routine's first word is its entry mask, the registers it saves
	// (dbgdis.dlg: "DBGDIS\START:   entry mask ^M<R2,R3,R4>").
	if datum == nil {
		if text, ok := c.EntryMaskText(addr); ok {
			d.examined = examineState{addr: addr, size: 2, set: true}
			c.DepositAddr = addr + 2

			c.Printf("%s%s\n", tabPad(d.locationName(addr)+":"), text)

			return nil
		}

		// SET TYPE's type, for a location no symbol types.
		if d.defaultType != nil {
			return d.examineTyped(addr, *d.defaultType, nil, radix)
		}
	}

	size := uint32(defaultSize)
	if datum != nil && datum.ElementSize() > 0 && datum.ElementSize() <= 8 {
		size = datum.ElementSize()
	}

	// A label of an array, named alone, shows every element.
	if !inRange && datum != nil && datum.IsArray() && len(datum.Descriptor.Bounds) == 1 {
		return d.examineArray(r, addr, datum)
	}

	return d.examineTyped(addr, dataType{kind: typeInteger, size: size}, datum, radix)
}

// commandType is the type a command's qualifier gives its locations, else
// SET TYPE/OVERRIDE's; typed is false when there is neither.
func (d *Debugger) commandType(r *dcl.Result) (dataType, bool, error) {
	t, typed, err := d.qualifierType(r)
	if err != nil || typed {
		return t, typed, err
	}

	if d.overrideType != nil {
		return *d.overrideType, true, nil
	}

	return dataType{}, false, nil
}

// examineTyped shows the memory at addr as type t. datum is the debug
// symbols' datum for addr, if any, which an /ASCII without a count takes
// its length from.
func (d *Debugger) examineTyped(addr uint32, t dataType, datum *dbgsym.Datum, radix int) error {
	c := d.Machine
	name := tabPad(d.locationName(addr) + ":")

	switch t.kind {
	case typeInstruction:
		return d.disassemble(addr, addr, d.instructionOptions())

	case typeSymbol:
		return d.examineSymbolTyped(addr, t.datum, radix)

	case typeASCII:
		return d.examineText(addr, datum, t.count, t.counted)

	case typeASCIC, typeASCIW, typeASCIZ, typeASCID:
		text, size, err := d.readCountedText(addr, t.kind)
		if err != nil {
			return err
		}

		d.examined = examineState{addr: addr, size: max(size, 1), set: true}
		c.DepositAddr = addr + max(size, 1)

		c.Printf("%s'%s'\n", name, printable(text))

		return nil
	}

	size, _ := t.fixedSize()

	data, err := c.ReadBytes(addr, size)
	if err != nil {
		return err
	}

	d.examined = examineState{addr: addr, size: size, set: true}
	c.DepositAddr = addr + size

	d.setCurrentValue(data)

	text, multiline := d.fixedText(t, data, radix)
	if multiline {
		c.Printf("%s\n%s", name, text)
	} else {
		c.Printf("%s%s\n", name, text)
	}

	return nil
}

// examineSymbolTyped shows the memory at addr as EXAMINE/TYPE=(name) asks:
// typed as the datum name labels is, which may be text or an array as
// well as a number.
func (d *Debugger) examineSymbolTyped(addr uint32, datum *dbgsym.Datum, radix int) error {
	switch {
	case datum.IsText():
		length := uint32(1)
		if datum.Descriptor != nil {
			length = uint32(datum.Descriptor.Length)
		}

		return d.examineText(addr, nil, length, true)

	case datum.IsArray() && len(datum.Descriptor.Bounds) == 1:
		return d.examineArray(nil, addr, datum)
	}

	if dt, ok := datumType(datum); ok {
		return d.examineTyped(addr, dt, nil, radix)
	}

	size := datum.ElementSize()
	if size == 0 {
		size = defaultSize
	}

	return d.examineTyped(addr, dataType{kind: typeInteger, size: size}, nil, radix)
}

// maxASCIZ is the longest zero-terminated string EXAMINE/ASCIZ reads
// looking for its end (govax's own limit).
const maxASCIZ = 512

// readCountedText reads the text of a string laid out as kind says at
// addr: after its count byte (ASCIC) or word (ASCIW), up to its zero byte
// (ASCIZ), or where the descriptor at addr points (ASCID). size is how
// many bytes at addr the string occupies (the descriptor's 8 for ASCID).
func (d *Debugger) readCountedText(addr uint32, kind typeKind) (text []byte, size uint32, err error) {
	c := d.Machine

	switch kind {
	case typeASCIC:
		count, err := c.ReadBytes(addr, 1)
		if err != nil {
			return nil, 0, err
		}

		text, err = c.ReadBytes(addr+1, uint32(count[0]))

		return text, uint32(count[0]) + 1, err

	case typeASCIW:
		count, err := c.ReadBytes(addr, 2)
		if err != nil {
			return nil, 0, err
		}

		n := uint32(count[0]) | uint32(count[1])<<8
		text, err = c.ReadBytes(addr+2, n)

		return text, n + 2, err

	case typeASCIZ:
		for n := uint32(0); n < maxASCIZ; n++ {
			b, err := c.ReadBytes(addr+n, 1)
			if err != nil {
				return nil, 0, err
			}

			if b[0] == 0 {
				return text, n + 1, nil
			}

			text = append(text, b[0])
		}

		return text, maxASCIZ, nil
	}

	// ASCID: the descriptor's length word, then its pointer longword.
	length, pointer, err := d.readDescriptor(addr)
	if err != nil {
		return nil, 0, err
	}

	text, err = c.ReadBytes(pointer, length)

	return text, 8, err
}

// readDescriptor reads the string descriptor at addr: the length in its
// first word, and the address of the text in its second longword.
func (d *Debugger) readDescriptor(addr uint32) (length, pointer uint32, err error) {
	header, err := d.Machine.ReadBytes(addr, 8)
	if err != nil {
		return 0, 0, err
	}

	length = uint32(header[0]) | uint32(header[1])<<8
	pointer = uint32(header[4]) | uint32(header[5])<<8 | uint32(header[6])<<16 | uint32(header[7])<<24

	return length, pointer, nil
}

// printable is text with each byte that isn't a printable character shown
// as a period, as the debugger shows strings.
func printable(text []byte) string {
	out := make([]byte, len(text))

	for i, b := range text {
		if b < ' ' || b > '~' {
			b = '.'
		}

		out[i] = b
	}

	return string(out)
}

// examineArray shows an array one element to a line, under a heading of
// its name and bounds:
//
//	DBGCMD\BUFFER[0:15]
//	    [0]:        00
//	    [1]:        00
func (d *Debugger) examineArray(r *dcl.Result, addr uint32, datum *dbgsym.Datum) error {
	c := d.Machine
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

		radix := d.outputRadix
		if r != nil {
			radix = d.examineRadix(r)
		}

		c.Printf("%s%s\n", tabPad(fmt.Sprintf("    [%d]:", i)), leadingDigit(formatRadix(value, size, radix)))
	}

	d.examined = examineState{addr: addr, size: size * uint32(bounds.Upper-bounds.Lower+1), set: true}

	return nil
}

// examineText shows data as a string in quotes: count characters when
// haveCount (/ASCII:n), else the length the label's descriptor has
// (.ASCID's label names a descriptor, whose pointer says where the text
// is), else one character. A byte that isn't a printable character is
// shown as a period.
func (d *Debugger) examineText(addr uint32, datum *dbgsym.Datum, count uint32, haveCount bool) error {
	c := d.Machine
	name := d.locationName(addr)
	start := addr

	length := uint32(1)

	if datum != nil {
		switch {
		case datum.Kind == dbgsym.DescriptorAddress:
			// The label is a descriptor: length in its first word, the
			// text's address in its second longword.
			var err error

			if length, start, err = d.readDescriptor(addr); err != nil {
				return err
			}
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

	d.examined = examineState{addr: addr, size: max(length, 1), set: true}
	c.DepositAddr = addr + max(length, 1)

	c.Printf("%s'%s'\n", tabPad(name+":"), printable(data))

	return nil
}

// evaluate runs EVALUATE[/radix] expression and EVALUATE/ADDRESS
// expression, which shows an expression's value: for a name of data, what
// is stored there, or with /ADDRESS where it is.
func (d *Dispatcher) evaluate(r *dcl.Result) error {
	dbg := d.Debugger

	if err := dbg.Machine.RequireInit(); err != nil {
		return err
	}

	text := strings.TrimSpace(r.String("EXPRESSION"))

	// %EXC_NAME and %EXC_FACILITY are strings, shown in quotes.
	if s, isString, err := dbg.exceptionString(text); isString || err != nil {
		if err == nil {
			dbg.Machine.Printf("%s\n", quotedString(s))
		}

		return err
	}

	value, err := dbg.Machine.EvalWholeMode(dbg.inputRadix, text, !r.Present("ADDRESS"))
	if err != nil {
		return err
	}

	// EVALUATE/ADDRESS makes the address the current location (%CURLOC);
	// EVALUATE's value is the current value (%CURVAL).
	if r.Present("ADDRESS") {
		dbg.examined = examineState{addr: value, size: defaultSize, set: true}
	} else {
		dbg.curval = currentValue{v: value, set: true}
	}

	if r.Present("CONDITION_VALUE") {
		dbg.Machine.Printf("%s\n", dbg.Machine.StatusText(value))

		return nil
	}

	dbg.Machine.Printf("%s\n", leadingDigit(formatRadix(uint64(value), 4, dbg.examineRadix(r))))

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

	if err := dbg.Machine.RequireInit(); err != nil {
		return err
	}

	addr, err := dbg.evalWhole(r.String("ADDRESS"))
	if err != nil {
		return err
	}

	names, global := dbg.Machine.DebugProgramAt(addr).SymbolizeNames(addr, dbg.symbolRadix())

	// An address no module names gets only the heading, as does an
	// address in no image (govax's choice; the probe has no such case).
	dbg.Machine.Printf("address %08X: \n", addr)

	for _, name := range names {
		dbg.Machine.Printf("    %s\n", name)
	}

	if global != "" {
		dbg.Machine.Printf("address %08X: (global)\n    %s\n", addr, global)
	}

	return nil
}

// deposit runs DEPOSIT[/type] location = value: stores a value of the
// location's type at a register or an address. The type is the one a
// qualifier gives, else SET TYPE/OVERRIDE's, else the type of the data the
// location labels (DEPOSIT WATCHB = 1 stores a byte), else SET TYPE's,
// else a longword. A number is read in the input radix; a string is in
// quotes.
func (d *Dispatcher) deposit(r *dcl.Result) error {
	dbg := d.Debugger
	c := dbg.Machine

	if err := c.RequireInit(); err != nil {
		return err
	}

	target, value, ok := splitAssignment(r.String("ASSIGNMENT"))
	if !ok {
		return vmserrors.New(vmserrors.CLI_NEEDEXPR)
	}

	value = d.asTyped(value)

	t, typed, err := dbg.commandType(r)
	if err != nil {
		return err
	}

	if r.Present("INSTRUCTION") {
		t, typed = dataType{kind: typeInstruction}, true
	}

	// The PSL takes a longword, or the low byte or word (depositPSL).
	if _, _, isPSL, isReg := registerOf(target); isReg && isPSL {
		return dbg.depositPSL(t, typed, value)
	}

	// A register takes a longword, or the type given.
	if reg, _, isPSL, isReg := registerOf(target); isReg && !isPSL {
		if !typed {
			v, err := dbg.Machine.EvalWholeIn(dbg.inputRadix, value)
			if err != nil {
				return err
			}

			c.CPU.SetGPR(reg, v)
			dbg.curval = currentValue{v: v, set: true}

			return nil
		}

		if _, err := dbg.registerBytes(reg, t, "DEPOSIT"); err != nil {
			return err
		}

		data, err := dbg.depositBytes(0, t, value)
		if err != nil {
			return err
		}

		dbg.setCurrentValue(data)

		return dbg.setRegisterBytes(reg, data)
	}

	addr, err := dbg.evalWhole(target)
	if err != nil {
		return err
	}

	if !typed {
		t = dbg.locationType(addr, value)
	}

	data, err := dbg.depositBytes(addr, t, value)
	if err != nil {
		return err
	}

	// An ASCID string goes where the descriptor points.
	at := addr
	if t.kind == typeASCID {
		if _, at, err = dbg.readDescriptor(addr); err != nil {
			return err
		}
	}

	dbg.examined = examineState{addr: addr, size: max(uint32(len(data)), 1), set: true}
	dbg.setCurrentValue(data)

	return c.WriteBytes(at, data)
}

// asTyped returns a string value in apostrophes ('abc') as it was typed.
// The grammar makes unquoted text upper case, and to DCL an apostrophe
// isn't a quote, but to the debugger it is, as a double quote is. The
// value ends the command line, so the line's own text of the same length
// is the value as typed.
func (d *Dispatcher) asTyped(value string) string {
	line := strings.TrimSpace(d.line)

	if !strings.HasPrefix(value, "'") || len(line) < len(value) {
		return value
	}

	if raw := line[len(line)-len(value):]; strings.EqualFold(raw, value) {
		return raw
	}

	return value
}

// locationType is the type DEPOSIT stores at addr when the command gives
// none: the type of the data a label there names (a string's, if the value
// is one in quotes), else SET TYPE's, else a longword.
func (d *Debugger) locationType(addr uint32, value string) dataType {
	if prog := d.Machine.DebugProgramAt(addr); prog != nil {
		if datum, _, found := prog.DatumAt(addr); found {
			if datum.IsText() && quoted(value) {
				return dataType{kind: typeSymbol, datum: datum}
			}

			if dt, ok := datumType(datum); ok {
				return dt
			}

			if size := datum.ElementSize(); size > 0 && size <= 8 {
				return dataType{kind: typeInteger, size: size}
			}

			return longwordType
		}
	}

	if d.defaultType != nil {
		return *d.defaultType
	}

	return longwordType
}

// quoted reports whether text is a string in quotes or apostrophes.
func quoted(text string) bool {
	text = strings.TrimSpace(text)

	return len(text) >= 2 && (text[0] == '"' || text[0] == '\'') && text[len(text)-1] == text[0]
}

// unquote removes the quotes around a string, if it has them.
func unquote(text string) string {
	text = strings.TrimSpace(text)
	if quoted(text) {
		return text[1 : len(text)-1]
	}

	return text
}

// fitText pads text with blanks to n characters, or cuts it short, as
// DEPOSIT of a string into a fixed-length place does.
func fitText(text string, n uint32) []byte {
	data := []byte(text)

	for uint32(len(data)) < n {
		data = append(data, ' ')
	}

	return data[:n]
}

// depositBytes is what DEPOSIT stores for value as type t at addr: the
// bytes, least significant first. addr matters only for the types that
// read what is there (an ASCID descriptor's length).
func (d *Debugger) depositBytes(addr uint32, t dataType, value string) ([]byte, error) {
	switch t.kind {
	case typeFloat:
		return d.parseFloatValue(t.float, value)

	case typePacked:
		v, err := d.Machine.EvalWholeIn(d.inputRadix, value)
		if err != nil {
			return nil, err
		}

		return packDecimal(int64(int32(v)), t.digits, value)

	case typeASCII:
		text := unquote(value)
		if !t.counted {
			return []byte(text), nil
		}

		return fitText(text, t.count), nil

	case typeASCIC:
		text := unquote(value)
		if len(text) > 255 {
			return nil, vmserrors.New(vmserrors.DBG_IVALOUTBNDS, strings.ToUpper(value))
		}

		return append([]byte{byte(len(text))}, text...), nil

	case typeASCIW:
		text := unquote(value)
		if len(text) > 65535 {
			return nil, vmserrors.New(vmserrors.DBG_IVALOUTBNDS, strings.ToUpper(value))
		}

		return append([]byte{byte(len(text)), byte(len(text) >> 8)}, text...), nil

	case typeASCIZ:
		return append([]byte(unquote(value)), 0), nil

	case typeASCID:
		// The string goes where the descriptor points, padded or cut to
		// the descriptor's length (the caller writes it there).
		length, _, err := d.readDescriptor(addr)
		if err != nil {
			return nil, err
		}

		return fitText(unquote(value), length), nil

	case typeDateTime:
		v, ok := d.Machine.ProcessOne().ParseTime(unquote(value))
		if !ok {
			return nil, vmserrors.New(vmserrors.DBG_INVTIME, strings.ToUpper(unquote(value)))
		}

		data := make([]byte, 8)
		for i := range data {
			data[i] = byte(v >> (8 * i))
		}

		return data, nil

	case typeSymbol:
		return d.depositSymbolTyped(addr, t.datum, value)

	case typeInstruction:
		return d.depositInstruction(addr, value)
	}

	// The integer types, and the others that are integers underneath
	// (a condition value, the PSL and PSW, a PTE).
	size, _ := t.fixedSize()

	v, err := d.Machine.EvalWholeIn(d.inputRadix, value)
	if err != nil {
		return nil, err
	}

	data := make([]byte, size)
	for i := range data {
		if i < 4 {
			data[i] = byte(v >> (8 * i))
		}
	}

	return data, nil
}

// depositSymbolTyped is depositBytes for DEPOSIT/TYPE=(name): value stored
// as the datum name labels is typed. A string fills the datum's length.
func (d *Debugger) depositSymbolTyped(addr uint32, datum *dbgsym.Datum, value string) ([]byte, error) {
	if datum.IsText() {
		length := uint32(len(unquote(value)))
		if datum.Descriptor != nil {
			length = uint32(datum.Descriptor.Length)
		}

		return fitText(unquote(value), length), nil
	}

	if dt, ok := datumType(datum); ok {
		return d.depositBytes(addr, dt, value)
	}

	size := datum.ElementSize()
	if size == 0 || size > 8 {
		size = defaultSize
	}

	return d.depositBytes(addr, dataType{kind: typeInteger, size: size}, value)
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
