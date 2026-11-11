package debugger

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/fortran/dbgtable"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmserrors"
)

// A FORTRAN program's variables, in a program compiled with /DEBUG (docs/planning/PHASE-85 -
// FORTRAN compiler usability.md, subtask 10).
//
// The debugger's language stays MACRO: the compiler writes no debugger records of
// FORTRAN's, and every name and rule MACRO gives still stands. What this adds, from the
// table the compiler leaves in the MACRO file (package dbgtable), is what a MACRO symbol
// can't say. In a routine, a bare X is that routine's X, and everywhere UNIT.X is
// shown as what it is, REAL as a REAL, whatever directive it was labeled over. V(2) and M(2,3)
// are elements, in FORTRAN's bounds and order, and S(2:4) a substring. A dummy argument
// is found through its frame's argument list, not at the small number its MACRO symbol
// stands for, and a CHARACTER one through its descriptor. A module with no table is
// unchanged by any of this.
//
// A subscript is an expression read in the debugger's radix, like anything else typed
// here, so V(10) under the default radix is V(16), and SET RADIX DECIMAL is the way to
// think in FORTRAN's numbers.

// fortranPlace is one thing to show or change: a variable, or one element of an array.
type fortranPlace struct {
	name   string // as the debugger shows it: MOD\UNIT.V(2)
	addr   uint32
	size   uint32 // its bytes: a CHARACTER value's length
	v      *dbgtable.Var
	reg    bool            // addr is a register's number, the value not in memory
	module *dbgsym.Module  // the module it's in
	prog   *dbgsym.Program // the image it's in
}

// fortranItem is what a location that may be a FORTRAN variable looks like: a path, a name
// that may have a unit, and up to two parts in parentheses. Anything else is an expression.
var fortranItem = regexp.MustCompile(`^(?i)((?:[A-Z$_][A-Z0-9$_.]*\\)*)([A-Z$_][A-Z0-9$_.]*)(\([^()]*(?:\([^()]*\)[^()]*)*\))?(\([^()]*\))?$`)

// fortranTable is a module's table, read once.
func (d *Debugger) fortranTable(mod *dbgsym.Module) *dbgtable.Table {
	if t, seen := d.fortranTables[mod]; seen {
		return t
	}

	if d.fortranTables == nil {
		d.fortranTables = map[*dbgsym.Module]*dbgtable.Table{}
	}

	t := dbgtable.Parse(d.Machine.ModuleSource(mod))
	d.fortranTables[mod] = t

	return t
}

// fortranFrame is one call frame's registers: where it's running, and its AP and FP.
type fortranFrame struct{ pc, ap, fp uint32 }

// fortranFrames lists the active frames, innermost first, following the saved FPs.
func (d *Debugger) fortranFrames() []fortranFrame {
	cpu := d.Machine.CPU
	frames := []fortranFrame{{cpu.GPR(registers.PC), cpu.GPR(registers.AP), cpu.GPR(registers.FP)}}

	const most = 256

	for len(frames) < most {
		fp := frames[len(frames)-1].fp
		if fp == 0 {
			break
		}

		data, err := d.Machine.ReadBytes(fp+8, 12)
		if err != nil {
			break
		}

		ap := binary.LittleEndian.Uint32(data)
		next := binary.LittleEndian.Uint32(data[4:])
		pc := binary.LittleEndian.Uint32(data[8:])

		if next <= fp {
			frames = append(frames, fortranFrame{pc, ap, 0})

			break
		}

		frames = append(frames, fortranFrame{pc, ap, next})
	}

	return frames
}

// frameUnit is the unit of the routine a frame's PC is in, with its module and image.
func (d *Debugger) frameUnit(f fortranFrame) (*dbgtable.Unit, *dbgsym.Module, *dbgsym.Program) {
	prog := d.Machine.DebugProgramAt(f.pc)
	if prog == nil {
		return nil, nil, nil
	}

	routine, mod, ok := prog.RoutineAt(f.pc)
	if !ok {
		return nil, nil, nil
	}

	table := d.fortranTable(mod)
	if table == nil {
		return nil, nil, nil
	}

	return table.Entries[routine.Name], mod, prog
}

// findFortranVar finds what a name means: in the unit a dotted name says, or for a bare
// name in the routine the program is in, or if that isn't FORTRAN's, the nearest
// routine on the stack that is. path is a module to look in, if the item gave one.
func (d *Debugger) findFortranVar(path, name string) (*dbgtable.Var, *dbgsym.Module, *dbgsym.Program, bool) {
	name = strings.ToUpper(name)

	wanted := ""
	if parts := strings.Split(strings.TrimSuffix(path, `\`), `\`); path != "" {
		wanted = strings.ToUpper(parts[0])
	}

	if unitName, varName, dotted := cutLastDot(name); dotted {
		for _, prog := range d.Machine.DebugPrograms() {
			for _, mod := range prog.Modules {
				if wanted != "" && !strings.EqualFold(mod.Name, wanted) {
					continue
				}

				if table := d.fortranTable(mod); table != nil {
					if u := table.Units[unitName]; u != nil {
						if v := u.Vars[varName]; v != nil {
							return v, mod, prog, true
						}
					}
				}
			}
		}

		return nil, nil, nil, false
	}

	if wanted != "" {
		return nil, nil, nil, false
	}

	for _, f := range d.fortranFrames() {
		if u, mod, prog := d.frameUnit(f); u != nil {
			v, ok := u.Vars[name]

			return v, mod, prog, ok
		}
	}

	return nil, nil, nil, false
}

// cutLastDot splits UNIT.NAME. A unit's name may hold no period, but MACRO's may, so the
// last one divides them.
func cutLastDot(name string) (string, string, bool) {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 || i == len(name)-1 {
		return "", "", false
	}

	return name[:i], name[i+1:], true
}

// fortranFrameOf finds the frame a dummy argument's or a frame variable's place is in:
// the innermost active frame of a routine in its unit, starting at the call level the scope
// chose (SET SCOPE 2, SET SCOPE/CURRENT 2, 2\K), so a recursive unit's outer activations
// can be looked at (Phase 89).
func (d *Debugger) fortranFrameOf(v *dbgtable.Var, mod *dbgsym.Module) (fortranFrame, bool) {
	table := d.fortranTable(mod)

	frames := d.fortranFrames()
	if level, _, ok := d.registerPlace(d.callLevels()); ok && level < len(frames) {
		frames = frames[level:]
	}

	for _, f := range frames {
		prog := d.Machine.DebugProgramAt(f.pc)
		if prog == nil {
			continue
		}

		routine, m, ok := prog.RoutineAt(f.pc)
		if ok && m == mod && table.Entries[routine.Name] == table.Units[v.Unit] {
			return f, true
		}
	}

	return fortranFrame{}, false
}

// long reads a longword.
func (d *Debugger) long(addr uint32) (uint32, error) {
	data, err := d.Machine.ReadBytes(addr, 4)
	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint32(data), nil
}

// fortranBase is where the variable's first element is, and a CHARACTER variable's length
// as the place says: for a CHARACTER dummy, what its descriptor says.
func (d *Debugger) fortranBase(v *dbgtable.Var, mod *dbgsym.Module, prog *dbgsym.Program) (addr, length uint32, err error) {
	length = uint32(max(v.Size, 0))

	symbol := func(name string) (uint32, error) {
		if s, ok := prog.Lookup(mod.Name + `\` + name); ok {
			return s.Value, nil
		}

		return 0, vmserrors.New(vmserrors.DBG_NOSYMBOL, name)
	}

	switch v.Place.Kind {
	case dbgtable.Static:
		addr, err = symbol(v.Place.Symbol)

		return addr, length, err

	case dbgtable.Through:
		cell, err := symbol(v.Place.Symbol)
		if err != nil {
			return 0, 0, err
		}

		if addr, err = d.long(cell); err != nil {
			return 0, 0, err
		}

	case dbgtable.Argument, dbgtable.Frame, dbgtable.ThroughFrame:
		frame, ok := d.fortranFrameOf(v, mod)
		if !ok {
			return 0, 0, vmserrors.New(vmserrors.DBG_NOTACTIVE, v.Unit+"."+v.Name)
		}

		switch v.Place.Kind {
		case dbgtable.Frame:
			return frame.fp + uint32(int32(v.Place.N)), length, nil
		case dbgtable.ThroughFrame:
			addr, err = d.long(frame.fp + uint32(int32(v.Place.N)))
		default:
			addr, err = d.long(frame.ap + 4*uint32(v.Place.N))
		}

		if err != nil {
			return 0, 0, err
		}

	case dbgtable.InRegister:
		return uint32(v.Place.N), length, nil
	}

	if v.DescriptorPassed() {
		data, err := d.Machine.ReadBytes(addr, 8)
		if err != nil {
			return 0, 0, err
		}

		return binary.LittleEndian.Uint32(data[4:]), uint32(binary.LittleEndian.Uint16(data)), nil
	}

	return addr, length, nil
}

// fortranBounds are the variable's bounds now: those the compiler knew, and those a class
// A descriptor holds (an adjustable array's, filled in by the entry point). The bounds
// follow the descriptor's multipliers, one for each dimension, after its first 20 bytes.
func (d *Debugger) fortranBounds(v *dbgtable.Var, mod *dbgsym.Module, prog *dbgsym.Program) ([]dbgtable.Bound, error) {
	bounds := append([]dbgtable.Bound(nil), v.Bounds...)

	// The descriptor's address: a symbol's, or in a RECURSIVE unit a displacement
	// from the frame of the activation the scope names.
	var descriptor uint32

	switch {
	case v.DescriptorInFrame:
		frame, ok := d.fortranFrameOf(v, mod)
		if !ok {
			return bounds, nil
		}

		descriptor = frame.fp + uint32(int32(v.DescriptorOffset))

	case v.Descriptor != "":
		s, ok := prog.Lookup(mod.Name + `\` + v.Descriptor)
		if !ok {
			return bounds, nil
		}

		descriptor = s.Value

	default:
		return bounds, nil
	}

	const multipliersAt = 20

	at := descriptor + multipliersAt + 4*uint32(len(bounds))

	for i := range bounds {
		low, err := d.long(at + 8*uint32(i))
		if err != nil {
			return nil, err
		}

		high, err := d.long(at + 8*uint32(i) + 4)
		if err != nil {
			return nil, err
		}

		if !bounds[i].Known {
			bounds[i].Lower, bounds[i].Upper = int(int32(low)), int(int32(high))
			bounds[i].Known = true
		}
	}

	return bounds, nil
}

// fortranElementSize is the size of one of the variable's elements.
func fortranElementSize(v *dbgtable.Var, length uint32) uint32 {
	if v.Type == dbgtable.Char {
		return length
	}

	return uint32(v.Size)
}

// fortranPlaces says what a location means if it's a FORTRAN variable: ok is false if it
// isn't one, and the location is anyone else's to read. A whole array is each of its
// elements, first subscript varying fastest, as the program stores it.
func (d *Debugger) fortranPlaces(item string) (places []fortranPlace, ok bool, err error) {
	m := fortranItem.FindStringSubmatch(strings.TrimSpace(item))
	if m == nil {
		return nil, false, nil
	}

	v, mod, prog, found := d.findFortranVar(m[1], m[2])
	if !found {
		return nil, false, nil
	}

	base, length, err := d.fortranBase(v, mod, prog)
	if err != nil {
		return nil, true, err
	}

	bounds, err := d.fortranBounds(v, mod, prog)
	if err != nil {
		return nil, true, err
	}

	subscripts, substring := m[3], m[4]

	// S(2:4) of a scalar is a substring, not a subscript.
	if len(bounds) == 0 && substring == "" && strings.Contains(subscripts, ":") {
		subscripts, substring = "", subscripts
	}

	if subscripts != "" && len(bounds) == 0 {
		return nil, false, nil
	}

	size := fortranElementSize(v, length)
	name := mod.Name + `\` + v.Unit + "." + v.Name
	place := fortranPlace{name: name, addr: base, size: size, v: v, reg: v.Place.Kind == dbgtable.InRegister, module: mod, prog: prog}

	if len(bounds) == 0 {
		if substring != "" {
			return d.fortranSubstring(place, substring)
		}

		return []fortranPlace{place}, true, nil
	}

	if subscripts == "" {
		return wholeArray(place, bounds), true, nil
	}

	element, err := d.fortranElement(place, bounds, subscripts)
	if err != nil {
		return nil, true, err
	}

	if substring != "" {
		return d.fortranSubstring(element, substring)
	}

	return []fortranPlace{element}, true, nil
}

// fortranElement finds one element of an array, by FORTRAN's rule: the first subscript
// varies fastest.
func (d *Debugger) fortranElement(place fortranPlace, bounds []dbgtable.Bound, text string) (fortranPlace, error) {
	words := strings.Split(strings.Trim(text, "()"), ",")

	dimensions := strconv.Itoa(len(bounds)) + " dimension"
	if len(bounds) != 1 {
		dimensions += "s"
	}

	switch {
	case len(words) > len(bounds):
		return place, vmserrors.New(vmserrors.DBG_TOOMANSUB, dimensions)
	case len(words) < len(bounds):
		return place, vmserrors.New(vmserrors.DBG_TOOFEWSUB, dimensions)
	}

	offset, stride := int64(0), int64(1)
	shown := make([]string, len(words))

	for i, word := range words {
		value, err := d.Machine.EvalWholeIn(d.inputRadix, strings.TrimSpace(word))
		if err != nil {
			return place, err
		}

		n, b := int(int32(value)), bounds[i]

		if !b.Assumed && b.Known && (n < b.Lower || n > b.Upper) || b.Assumed && b.Known && n < b.Lower {
			return place, vmserrors.New(vmserrors.DBG_SUBOUTBND, strconv.Itoa(i+1), strconv.Itoa(n), b.String())
		}

		shown[i] = strconv.Itoa(n)
		offset += int64(n-b.Lower) * stride
		stride *= int64(b.Upper - b.Lower + 1)
	}

	place.addr = uint32(int64(place.addr) + offset*int64(place.size))
	place.name += "(" + strings.Join(shown, ",") + ")"

	return place, nil
}

// fortranSubstring is part of a CHARACTER value, by its bounds, either of which may be
// left out.
func (d *Debugger) fortranSubstring(place fortranPlace, text string) ([]fortranPlace, bool, error) {
	if place.v.Type != dbgtable.Char {
		return nil, true, vmserrors.New(vmserrors.DBG_NOSYMBOL, place.name+text)
	}

	low, high, _ := strings.Cut(strings.Trim(text, "()"), ":")
	first, last := 1, int(place.size)

	if strings.TrimSpace(low) != "" {
		value, err := d.Machine.EvalWholeIn(d.inputRadix, strings.TrimSpace(low))
		if err != nil {
			return nil, true, err
		}

		first = int(int32(value))
	}

	if strings.TrimSpace(high) != "" {
		value, err := d.Machine.EvalWholeIn(d.inputRadix, strings.TrimSpace(high))
		if err != nil {
			return nil, true, err
		}

		last = int(int32(value))
	}

	if first < 1 || last > int(place.size) || first > last {
		return nil, true, vmserrors.New(vmserrors.DBG_SUBOUTBND, "of the substring", fmt.Sprintf("%d:%d", first, last), fmt.Sprintf("1:%d", place.size))
	}

	place.addr += uint32(first - 1)
	place.size = uint32(last - first + 1)
	place.name += fmt.Sprintf("(%d:%d)", first, last)

	return []fortranPlace{place}, true, nil
}

// wholeArray is each element of an array. An assumed-size array's last dimension has no
// end to go to, so that one shows only its first element, which is all that's known to be
// there.
func wholeArray(place fortranPlace, bounds []dbgtable.Bound) []fortranPlace {
	subscripts := make([]int, len(bounds))
	for i, b := range bounds {
		subscripts[i] = b.Lower
		if !b.Known {
			return []fortranPlace{place}
		}
	}

	var places []fortranPlace

	for at := place.addr; ; at += place.size {
		words := make([]string, len(subscripts))
		for i, n := range subscripts {
			words[i] = strconv.Itoa(n)
		}

		element := place
		element.addr = at
		element.name = place.name + "(" + strings.Join(words, ",") + ")"
		places = append(places, element)

		dimension := 0

		for ; dimension < len(bounds); dimension++ {
			if bounds[dimension].Assumed {
				return places
			}

			subscripts[dimension]++
			if subscripts[dimension] <= bounds[dimension].Upper {
				break
			}

			subscripts[dimension] = bounds[dimension].Lower
		}

		if dimension == len(bounds) {
			return places
		}
	}
}

// fortranFloat is the floating format of a real's or complex's type code.
func fortranFloat(code string) vaxfloat.Format {
	switch code {
	case dbgtable.RealD, dbgtable.ComplxD:
		return vaxfloat.D
	case dbgtable.RealG, dbgtable.ComplxG:
		return vaxfloat.G
	case dbgtable.RealH:
		return vaxfloat.H
	}

	return vaxfloat.F
}

// fortranBytes reads a place's value.
func (d *Debugger) fortranBytes(p fortranPlace) ([]byte, error) {
	if p.reg {
		// The register as the scope's call level has it.
		view, err := d.registerView()
		if err != nil {
			return nil, err
		}

		reg := registers.Reg(p.addr)

		value, err := view.get(reg, registerName(reg))
		if err != nil {
			return nil, err
		}

		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data, value)

		return data, nil
	}

	return d.Machine.ReadBytes(p.addr, p.size)
}

// fortranText is a value as text, by its FORTRAN type.
func (d *Debugger) fortranText(p fortranPlace, data []byte, radix int) string {
	switch p.v.Type {
	case dbgtable.Integer:
		text, _ := d.fixedText(dataType{kind: typeInteger, size: uint32(len(data))}, data, radix)

		return text

	case dbgtable.Logical:
		if data[0]&1 != 0 {
			return ".TRUE."
		}

		return ".FALSE."

	case dbgtable.Char:
		return "'" + printable(data) + "'"

	case dbgtable.RealF, dbgtable.RealD, dbgtable.RealG, dbgtable.RealH:
		return formatFloat(fortranFloat(p.v.Type), data)

	case dbgtable.ComplxF, dbgtable.ComplxD, dbgtable.ComplxG:
		half := len(data) / 2
		format := fortranFloat(p.v.Type)

		return "(" + formatFloat(format, data[:half]) + "," + formatFloat(format, data[half:]) + ")"
	}

	// A record, which the debugger doesn't take apart: its longwords.
	var words []string

	for i := 0; i+4 <= len(data); i += 4 {
		words = append(words, fmt.Sprintf("%08X", binary.LittleEndian.Uint32(data[i:])))
	}

	return strings.Join(words, " ")
}

// examineFortran shows each place, by its FORTRAN type, or by the type a command's
// qualifier gives, as a qualifier does for any location.
func (d *Debugger) examineFortran(r *dcl.Result, places []fortranPlace) error {
	radix := d.examineRadix(r)

	t, typed, err := d.commandType(r)
	if err != nil {
		return err
	}

	for _, p := range places {
		if typed && !p.reg {
			d.namedPlace = p.name
			err := d.examineTyped(p.addr, t, nil, radix)
			d.namedPlace = ""

			if err != nil {
				return err
			}

			continue
		}

		data, err := d.fortranBytes(p)
		if err != nil {
			return err
		}

		if !p.reg {
			d.examined = examineState{addr: p.addr, size: max(p.size, 1), set: true}
			d.Machine.DepositAddr = p.addr + p.size
		}

		d.setCurrentValue(data)
		d.Machine.Printf("%s%s\n", tabPad(p.name+":"), d.fortranText(p, data, radix))
		d.showLoopRegister(p, radix)
	}

	return nil
}

// showLoopRegister says when /OPTIMIZE keeps a variable in a register during a loop,
// when what's in memory may be out of date. Whether the program is in that loop the
// table doesn't say, so this gives what's there and leaves it to whoever is reading.
func (d *Debugger) showLoopRegister(p fortranPlace, radix int) {
	table := d.fortranTable(p.module)
	if table == nil || table.Units[p.v.Unit] == nil {
		return
	}

	n, ok := table.Units[p.v.Unit].Loops[p.v.Name]
	if !ok {
		return
	}

	// The register as the scope's call level has it.
	view, err := d.registerView()
	if err != nil {
		return
	}

	value, err := view.get(registers.Reg(n), registerName(registers.Reg(n)))
	if err != nil {
		return
	}

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, value)

	text, _ := d.fixedText(dataType{kind: typeInteger, size: 4}, data, radix)
	d.Machine.Printf("%sduring a loop, R%d holds it: %s\n", tabPad(""), n, text)
}

// fortranLineAt is which line of the FORTRAN source the code at pc came from, in a
// program with a table: the source line last written above that instruction in the MACRO
// file. The number is told apart by the record, so a line of an included file, whose
// number means nothing here, is a line too.
func (d *Debugger) fortranLineAt(pc uint32) (string, bool) {
	prog := d.Machine.DebugProgramAt(pc)
	if prog == nil {
		return "", false
	}

	_, mod, ok := prog.LineAt(pc)
	if !ok || d.fortranTable(mod) == nil {
		return "", false
	}

	records, index, ok := d.Machine.SourceAt(pc)
	if !ok {
		return "", false
	}

	for ; index >= 0; index-- {
		if _, isLine := dbgtable.FortranLine(records[index]); isLine {
			return records[index], true
		}
	}

	return "", false
}

// showFortranLine shows, above the MACRO line a stop is at, the FORTRAN line it came from.
func (d *Debugger) showFortranLine(pc uint32) {
	record, ok := d.fortranLineAt(pc)
	if !ok {
		return
	}

	number, _ := dbgtable.FortranLine(record)
	text := ""

	if len(record) > dbgtable.FortranLinePrefix {
		text = record[dbgtable.FortranLinePrefix:]
	}

	d.Machine.Printf("FORTRAN line %d: %s\n", number, strings.TrimSpace(expandTabs(text)))
}

// evaluateFortran is EVALUATE of a variable: what EXAMINE shows, without the name.
func (d *Debugger) evaluateFortran(places []fortranPlace, radix int) error {
	for _, p := range places {
		data, err := d.fortranBytes(p)
		if err != nil {
			return err
		}

		d.setCurrentValue(data)
		d.Machine.Printf("%s\n", d.fortranText(p, data, radix))
	}

	return nil
}

// depositFortran changes one place, reading the value as its FORTRAN type, or as the type
// a command's qualifier gives.
func (d *Debugger) depositFortran(places []fortranPlace, t dataType, typed bool, value string) error {
	if len(places) != 1 {
		return vmserrors.New(vmserrors.DBG_OPTNOTALLOW, "DEPOSIT")
	}

	p := places[0]

	data, err := d.fortranValue(p, t, typed, value)
	if err != nil {
		return err
	}

	d.setCurrentValue(data)

	if p.reg {
		if len(data) != 4 {
			return vmserrors.New(vmserrors.DBG_OPTNOTALLOW, "DEPOSIT")
		}

		view, err := d.registerView()
		if err != nil {
			return err
		}

		reg := registers.Reg(p.addr)

		return d.put(view, reg, registerName(reg), binary.LittleEndian.Uint32(data))
	}

	d.examined = examineState{addr: p.addr, size: max(uint32(len(data)), 1), set: true}

	return d.Machine.WriteBytes(p.addr, data)
}

// fortranValue is the bytes a value typed for a place makes.
func (d *Debugger) fortranValue(p fortranPlace, t dataType, typed bool, value string) ([]byte, error) {
	if typed {
		return d.depositBytes(p.addr, t, value)
	}

	switch p.v.Type {
	case dbgtable.Integer:
		return d.depositBytes(p.addr, dataType{kind: typeInteger, size: p.size}, value)

	case dbgtable.RealF, dbgtable.RealD, dbgtable.RealG, dbgtable.RealH:
		return d.parseFloatValue(fortranFloat(p.v.Type), value)

	case dbgtable.Char:
		data := []byte(unquote(value))
		for uint32(len(data)) < p.size {
			data = append(data, ' ')
		}

		return data[:p.size], nil

	case dbgtable.Logical:
		data := make([]byte, p.size)

		switch strings.ToUpper(strings.TrimSpace(value)) {
		case ".TRUE.":
			for i := range data {
				data[i] = 0xFF
			}
		case ".FALSE.":
		default:
			return d.depositBytes(p.addr, dataType{kind: typeInteger, size: p.size}, value)
		}

		return data, nil
	}

	return nil, vmserrors.New(vmserrors.DBG_OPTNOTALLOW, "DEPOSIT")
}

// fortranName is the evaluator's way into all this: inside an expression, a name is its
// variable's address, as a MACRO label is, or under EVALUATE, as with a data label, what
// it holds (d.evaluating).
func (d *Debugger) fortranName(name string) (uint32, bool) {
	// $LINE30 is the label on line 30 of the routine the program is in.
	if strings.HasPrefix(strings.ToUpper(name), "$LINE") {
		if addr, ok := d.fortranLabel(strings.ToUpper(name)); ok {
			return addr, true
		}
	}

	places, ok, err := d.fortranPlaces(name)
	if !ok || err != nil || len(places) == 0 {
		return 0, false
	}

	p := places[0]

	// A dotted name of a static variable is a MACRO label, whose contents the evaluator
	// fetches itself.
	label := strings.Contains(name, ".") && p.v.Place.Kind == dbgtable.Static

	if d.evaluating && !label && len(places) == 1 && p.size <= 4 {
		data, err := d.fortranBytes(p)
		if err != nil {
			return 0, false
		}

		var value uint32

		for i := len(data) - 1; i >= 0; i-- {
			value = value<<8 | uint32(data[i])
		}

		return value, true
	}

	if p.reg {
		return 0, false
	}

	return p.addr, true
}

// fortranLabel finds a line's label in the routine the program is in.
func (d *Debugger) fortranLabel(name string) (uint32, bool) {
	for _, f := range d.fortranFrames() {
		if u, _, prog := d.frameUnit(f); u != nil {
			// A label is the routine's symbol, not the module's, so its path would need
			// the routine, and its whole name is enough.
			if s, ok := prog.Lookup(u.Name + "." + name); ok {
				return s.Value, true
			}

			return 0, false
		}
	}

	return 0, false
}
