package debugger

import (
	"encoding/binary"
	"regexp"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/fortran/dbgtable"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmserrors"
)

// How a location that may be a FORTRAN variable is read (docs/planning/PHASE-110 -
// Fortran 90 compiler usability.md, subtask 7a). It is a name, which may have a unit
// (UNIT.X) and a path before it (MOD\), then any chain of three things:
//
//   - parentheses: an array's subscripts, V(2) or M(2,3); a section, V(2:5), M(1:2,1),
//     or V(1:9:2), a subscript being lower:upper[:stride] with either bound left out
//     for the array's; or after a CHARACTER value its substring, S(2:4) or NAMES(2)(1:3);
//   - % and a component's name, a Fortran 90 derived type's: PT%X, PTS(2)%Y;
//   - a period and a component's name, a DEC record's (R.F), or a derived type's too.
//
// A period is also what divides a unit's name from its variable's, so a dotted name is
// tried as a unit's variable first, the longest unit first (as a module procedure's
// unit is M.P), and only then as a variable of the routine the program is in with its
// components after it. Each step of the chain turns a list of things (arrays or single
// values) into another: a section gives an element for each subscript it names, first
// varying fastest, and a component of an array is that component of every element, as
// Fortran 90 reads PTS%Y.

// fortranPath is the path a location may start with: MOD\ or MOD\ROUTINE\.
var fortranPath = regexp.MustCompile(`^(?i)(?:[A-Z$_][A-Z0-9$_.]*\\)*`)

// itemPart is one piece of a location: a name, a period, a %, or parentheses and what's
// in them.
type itemPart struct {
	kind byte // 'n' a name, '.', '%', or '(' a group, its text with the parentheses
	text string
}

// scanItem splits what follows a location's path into its parts, or says it isn't a
// FORTRAN location: anything but names, periods, %s, and balanced parentheses, or those
// in an order no chain has.
func scanItem(text string) ([]itemPart, bool) {
	var parts []itemPart

	isFirst := func(c byte) bool { return c == '$' || c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }
	isName := func(c byte) bool { return isFirst(c) || c >= '0' && c <= '9' }

	for i := 0; i < len(text); {
		switch c := text[i]; {
		case isFirst(c):
			j := i
			for j < len(text) && isName(text[j]) {
				j++
			}

			parts = append(parts, itemPart{'n', strings.ToUpper(text[i:j])})
			i = j

		case c == '.' || c == '%':
			parts = append(parts, itemPart{c, string(c)})
			i++

		case c == '(':
			depth, j := 0, i
			for ; j < len(text); j++ {
				if text[j] == '(' {
					depth++
				} else if text[j] == ')' {
					depth--
					if depth == 0 {
						break
					}
				}
			}

			if j == len(text) {
				return nil, false
			}

			parts = append(parts, itemPart{'(', text[i : j+1]})
			i = j + 1

		default:
			return nil, false
		}
	}

	// A name first; after a name or a group, anything but a name; after a period or
	// a %, a name; and nothing left hanging.
	for i, p := range parts {
		var after byte
		if i > 0 {
			after = parts[i-1].kind
		}

		switch {
		case i == 0 && p.kind != 'n',
			p.kind == 'n' && i > 0 && after != '.' && after != '%',
			p.kind != 'n' && (after == '.' || after == '%'):
			return nil, false
		}
	}

	if len(parts) == 0 || parts[len(parts)-1].kind == '.' || parts[len(parts)-1].kind == '%' {
		return nil, false
	}

	return parts, true
}

// fortranObject is one thing a step of the chain has: an array (bounds) or one value.
type fortranObject struct {
	place  fortranPlace
	bounds []dbgtable.Bound
}

// fortranPlaces says what a location means if it's a FORTRAN variable: ok is false if it
// isn't one, and the location is anyone else's to read. A whole array is each of its
// elements, first subscript varying fastest, as the program stores it. A record is one
// place; examineFortran shows it a component at a time.
func (d *Debugger) fortranPlaces(item string) (places []fortranPlace, ok bool, err error) {
	item = strings.TrimSpace(item)
	path := fortranPath.FindString(item)

	parts, ok := scanItem(item[len(path):])
	if !ok {
		return nil, false, nil
	}

	// The dotted name at the front, and which of its names is the variable's.
	head := 1
	for head+1 < len(parts) && parts[head].kind == '.' && parts[head+1].kind == 'n' {
		head += 2
	}

	for k := head - 1; k >= 0; k -= 2 {
		names := make([]string, 0, k/2+1)
		for i := 0; i <= k; i += 2 {
			names = append(names, parts[i].text)
		}

		if v, mod, prog, found := d.findFortranVar(path, strings.Join(names, ".")); found {
			return d.fortranChain(v, mod, prog, parts[k+1:])
		}
	}

	return nil, false, nil
}

// fortranChain follows a variable's chain of subscripts, substrings, and components.
func (d *Debugger) fortranChain(v *dbgtable.Var, mod *dbgsym.Module, prog *dbgsym.Program, chain []itemPart) ([]fortranPlace, bool, error) {
	name := mod.Name + `\` + v.Unit + "." + v.Name

	// A pointer is what it points to.
	if v.Pointer {
		p := fortranPlace{name: name, size: 4, v: v, module: mod, prog: prog, pointer: true}

		if v.Place.Kind == dbgtable.InRegister {
			p.addr, p.reg = uint32(v.Place.N), true
		} else {
			addr, err := d.storageAddr(v, mod, prog)
			if err != nil {
				return nil, true, err
			}

			p.addr = addr
		}

		o, err := d.follow(p, len(v.Bounds))
		if err != nil {
			return nil, true, err
		}

		return d.fortranSteps(o, chain)
	}

	base, length, err := d.fortranBase(v, mod, prog)
	if err != nil {
		return nil, true, err
	}

	bounds, err := d.fortranBounds(v, mod, prog)
	if err != nil {
		return nil, true, err
	}

	// S(2:4) of a scalar is a substring, not a subscript, and any other group after a
	// scalar means the location isn't a FORTRAN variable's.
	if len(chain) > 0 && chain[0].kind == '(' && len(bounds) == 0 && (v.Type != dbgtable.Char || !strings.Contains(chain[0].text, ":")) {
		return nil, false, nil
	}

	size := fortranElementSize(v, length)
	place := fortranPlace{name: name, addr: base, size: size, v: v, reg: v.Place.Kind == dbgtable.InRegister, module: mod, prog: prog}

	if place.strides, place.a0, err = d.describedStrides(v, mod, prog, bounds, size); err != nil {
		return nil, true, err
	}

	return d.fortranSteps(fortranObject{place, bounds}, chain)
}

// fortranSteps follows a chain of subscripts, substrings, and components from a thing,
// and gives each place it ends at, an array's being each of its elements.
func (d *Debugger) fortranSteps(start fortranObject, chain []itemPart) ([]fortranPlace, bool, error) {
	var err error

	objects := []fortranObject{start}

	for i := 0; i < len(chain); i++ {
		// A component is the separator and the name after it.
		step := chain[i]
		if step.kind != '(' {
			i++
			step = chain[i]
		}

		var next []fortranObject

		for _, o := range objects {
			var made []fortranObject

			if step.kind == '(' {
				made, err = d.fortranGroup(o, step.text)
			} else {
				made, err = d.fortranComponent(o, step.text)
			}

			if err != nil {
				return nil, true, err
			}

			next = append(next, made...)
		}

		objects = next
	}

	var places []fortranPlace

	for _, o := range objects {
		if len(o.bounds) > 0 {
			places = append(places, wholeArray(o.place, o.bounds)...)
		} else {
			places = append(places, o.place)
		}
	}

	return places, true, nil
}

// fortranGroup is what parentheses after a thing mean: an array's subscripts or
// section, or a CHARACTER value's substring.
func (d *Debugger) fortranGroup(o fortranObject, text string) ([]fortranObject, error) {
	if len(o.bounds) == 0 {
		if !strings.Contains(text, ":") {
			return nil, vmserrors.New(vmserrors.DBG_NOSYMBOL, o.place.name+text)
		}

		places, _, err := d.fortranSubstring(o.place, text)
		if err != nil {
			return nil, err
		}

		return []fortranObject{{place: places[0]}}, nil
	}

	places, err := d.fortranSubscripts(o.place, o.bounds, text)
	if err != nil {
		return nil, err
	}

	objects := make([]fortranObject, len(places))
	for i, p := range places {
		objects[i] = fortranObject{place: p}
	}

	return objects, nil
}

// fortranComponent is a record's component: of every element, if the record is an
// array of them.
func (d *Debugger) fortranComponent(o fortranObject, name string) ([]fortranObject, error) {
	if len(o.bounds) > 0 {
		var objects []fortranObject

		for _, p := range wholeArray(o.place, o.bounds) {
			made, err := d.fortranComponent(fortranObject{place: p}, name)
			if err != nil {
				return nil, err
			}

			objects = append(objects, made...)
		}

		return objects, nil
	}

	st := d.placeStructure(o.place)
	if st == nil {
		return nil, vmserrors.New(vmserrors.DBG_NOSYMBOL, o.place.name+"%"+name)
	}

	c := st.Component(name)
	if c == nil {
		return nil, vmserrors.New(vmserrors.DBG_NOSYMBOL, o.place.name+st.Separator()+name)
	}

	made := componentObject(o.place, st, c)
	if c.Pointer {
		followed, err := d.follow(made.place, len(c.Bounds))
		if err != nil {
			return nil, err
		}

		made = followed
	}

	return []fortranObject{made}, nil
}

// follow is what a pointer points to. p is the pointer's own storage: its address, or
// for a scalar pointer the optimizer keeps in a register, p.reg and the register's number.
// A scalar's storage holds its target's address, a CHARACTER one's is a class S
// descriptor (the length, then the address), and an array pointer's (rank more than 0)
// a class NCA descriptor, whose POINTER is the first element's address, its strides and
// bounds the target's: an array pointer's elements are found as an assumed-shape
// dummy's are. An address of 0 is a pointer not associated.
func (d *Debugger) follow(p fortranPlace, rank int) (fortranObject, error) {
	target := p
	target.pointer, target.reg, target.strides = false, false, nil
	target.size = uint32(max(p.v.Size, 0))

	notAssociated := vmserrors.New(vmserrors.DBG_NOTASSOC, p.name)

	switch {
	case rank > 0:
		head, err := d.Machine.ReadBytes(p.addr, 20)
		if err != nil {
			return fortranObject{}, err
		}

		if target.addr = binary.LittleEndian.Uint32(head[4:]); target.addr == 0 {
			return fortranObject{}, notAssociated
		}

		if p.v.Type == dbgtable.Char {
			target.size = uint32(binary.LittleEndian.Uint16(head))
		}

		bounds := make([]dbgtable.Bound, rank)
		strides := make([]int64, rank)
		contiguous, expect := true, int64(target.size)

		for i := range bounds {
			stride, err := d.long(p.addr + 20 + 4*uint32(i))
			if err != nil {
				return fortranObject{}, err
			}

			at := p.addr + 20 + 4*uint32(rank) + 8*uint32(i)

			low, err := d.long(at)
			if err != nil {
				return fortranObject{}, err
			}

			high, err := d.long(at + 4)
			if err != nil {
				return fortranObject{}, err
			}

			bounds[i] = dbgtable.Bound{Lower: int(int32(low)), Upper: int(int32(high)), Known: true}
			strides[i] = int64(int32(stride))
			contiguous = contiguous && strides[i] == expect
			expect *= int64(bounds[i].Upper - bounds[i].Lower + 1)
		}

		if !contiguous {
			target.strides, target.a0 = strides, binary.LittleEndian.Uint32(head[16:])
		}

		return fortranObject{target, bounds}, nil

	case p.reg:
		view, err := d.registerView()
		if err != nil {
			return fortranObject{}, err
		}

		reg := registers.Reg(p.addr)
		if target.addr, err = view.get(reg, registerName(reg)); err != nil {
			return fortranObject{}, err
		}

	case p.v.Type == dbgtable.Char:
		data, err := d.Machine.ReadBytes(p.addr, 8)
		if err != nil {
			return fortranObject{}, err
		}

		target.size = uint32(binary.LittleEndian.Uint16(data))
		target.addr = binary.LittleEndian.Uint32(data[4:])

	default:
		addr, err := d.long(p.addr)
		if err != nil {
			return fortranObject{}, err
		}

		target.addr = addr
	}

	if target.addr == 0 {
		return fortranObject{}, notAssociated
	}

	return fortranObject{place: target}, nil
}

// placeStructure is the layout of the record at a place, or nil if it isn't a record
// or the table doesn't give it.
func (d *Debugger) placeStructure(p fortranPlace) *dbgtable.Structure {
	if p.v.Type != dbgtable.Record || p.v.Struct == "" {
		return nil
	}

	table := d.fortranTable(p.module)
	if table == nil || table.Units[p.v.Unit] == nil {
		return nil
	}

	return table.Units[p.v.Unit].Structures[p.v.Struct]
}

// componentObject is a component of the record at p. Its Var is made for it: the unit's,
// so its structure is found in the same table, with no place of its own, the place
// being p's address and the component's offset. Its name is what the debugger shows.
//
// A pointer component's object is its storage, not yet followed (place.pointer), with
// no bounds: follow has them from its descriptor.
func componentObject(p fortranPlace, st *dbgtable.Structure, c *dbgtable.Component) fortranObject {
	v := &dbgtable.Var{Unit: p.v.Unit, Name: p.v.Name + st.Separator() + c.Name, Type: c.Type, Struct: c.Struct, Size: c.Size, Pointer: c.Pointer}

	if c.Pointer {
		return fortranObject{place: fortranPlace{
			name:    p.name + st.Separator() + c.Name,
			addr:    p.addr + uint32(c.Offset),
			size:    4,
			v:       v,
			module:  p.module,
			prog:    p.prog,
			pointer: true,
		}}
	}

	return fortranObject{
		place: fortranPlace{
			name:   p.name + st.Separator() + c.Name,
			addr:   p.addr + uint32(c.Offset),
			size:   uint32(c.Size),
			v:      v,
			module: p.module,
			prog:   p.prog,
		},
		bounds: c.Bounds,
	}
}

// recordLeaves is what EXAMINE shows of a place: a record's every component that isn't
// a record, in order, each element of an array component, and anything else itself.
func (d *Debugger) recordLeaves(p fortranPlace) []fortranPlace {
	st := d.placeStructure(p)
	if st == nil {
		return []fortranPlace{p}
	}

	var leaves []fortranPlace

	for _, c := range st.Components {
		o := componentObject(p, st, c)

		// A pointer component is shown as where it points, not followed: a record
		// may point to another of its type, or to itself. The address is the
		// storage's first longword, or a descriptor's POINTER.
		if c.Pointer {
			if c.Type == dbgtable.Char || len(c.Bounds) > 0 {
				o.place.addr += 4
			}

			leaves = append(leaves, o.place)

			continue
		}

		elements := []fortranPlace{o.place}
		if len(o.bounds) > 0 {
			elements = wholeArray(o.place, o.bounds)
		}

		for _, e := range elements {
			leaves = append(leaves, d.recordLeaves(e)...)
		}
	}

	return leaves
}

// fortranSubscripts finds the elements subscripts name: one element, or a section's,
// first subscript varying fastest.
func (d *Debugger) fortranSubscripts(place fortranPlace, bounds []dbgtable.Bound, text string) ([]fortranPlace, error) {
	words := strings.Split(strings.Trim(text, "()"), ",")

	dimensions := strconv.Itoa(len(bounds)) + " dimension"
	if len(bounds) != 1 {
		dimensions += "s"
	}

	switch {
	case len(words) > len(bounds):
		return nil, vmserrors.New(vmserrors.DBG_TOOMANSUB, dimensions)
	case len(words) < len(bounds):
		return nil, vmserrors.New(vmserrors.DBG_TOOFEWSUB, dimensions)
	}

	// Each dimension's subscripts, in order.
	lists := make([][]int, len(words))

	for i, word := range words {
		list, err := d.subscriptList(strings.TrimSpace(word), i, bounds[i])
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return nil, nil // an empty section has no elements
		}

		lists[i] = list
	}

	var places []fortranPlace

	at := make([]int, len(lists)) // where each dimension is in its list
	subscripts := make([]int, len(lists))

	for {
		for i, list := range lists {
			subscripts[i] = list[at[i]]
		}

		places = append(places, elementAt(place, bounds, subscripts))

		dimension := 0
		for ; dimension < len(lists); dimension++ {
			if at[dimension]++; at[dimension] < len(lists[dimension]) {
				break
			}

			at[dimension] = 0
		}

		if dimension == len(lists) {
			return places, nil
		}
	}
}

// subscriptList is one dimension's subscripts: a subscript, or a triplet's values. A
// triplet's bound left out is the dimension's; an assumed-size array's last upper bound,
// which nothing has, is its lower, as an EXAMINE of the whole array shows only that.
func (d *Debugger) subscriptList(word string, dimension int, b dbgtable.Bound) ([]int, error) {
	value := func(text string, missing int, known bool) (int, error) {
		if text = strings.TrimSpace(text); text == "" {
			if !known {
				return 0, vmserrors.New(vmserrors.DBG_SYNTAX, word)
			}

			return missing, nil
		}

		v, err := d.Machine.EvalWholeIn(d.inputRadix, text)

		return int(int32(v)), err
	}

	inBounds := func(n int) error {
		if b.Known && (n < b.Lower || !b.Assumed && n > b.Upper) {
			return vmserrors.New(vmserrors.DBG_SUBOUTBND, strconv.Itoa(dimension+1), strconv.Itoa(n), b.String())
		}

		return nil
	}

	fields := strings.Split(word, ":")
	if len(fields) == 1 {
		n, err := value(word, 0, true)
		if err != nil {
			return nil, err
		}

		return []int{n}, inBounds(n)
	}

	if len(fields) > 3 {
		return nil, vmserrors.New(vmserrors.DBG_SYNTAX, word)
	}

	upper := b.Upper
	if b.Assumed {
		upper = b.Lower
	}

	low, err := value(fields[0], b.Lower, b.Known)
	if err != nil {
		return nil, err
	}

	high, err := value(fields[1], upper, b.Known)
	if err != nil {
		return nil, err
	}

	stride := 1
	if len(fields) == 3 {
		if stride, err = value(fields[2], 1, true); err != nil {
			return nil, err
		}

		if stride == 0 {
			return nil, vmserrors.New(vmserrors.DBG_SYNTAX, word)
		}
	}

	var list []int

	for n := low; stride > 0 && n <= high || stride < 0 && n >= high; n += stride {
		if err := inBounds(n); err != nil {
			return nil, err
		}

		list = append(list, n)
	}

	return list, nil
}

// elementAt is one element of an array, by FORTRAN's rule: the first subscript varies
// fastest. A class NCA descriptor's strides, if the place has them, say where it is.
func elementAt(place fortranPlace, bounds []dbgtable.Bound, subscripts []int) fortranPlace {
	offset, stride := int64(0), int64(1)
	strided := int64(place.a0)
	shown := make([]string, len(subscripts))

	for i, n := range subscripts {
		b := bounds[i]
		shown[i] = strconv.Itoa(n)
		offset += int64(n-b.Lower) * stride
		stride *= int64(b.Upper - b.Lower + 1)

		if place.strides != nil {
			strided += int64(n) * place.strides[i]
		}
	}

	element := place
	element.addr = uint32(int64(place.addr) + offset*int64(place.size))

	if place.strides != nil {
		element.addr = uint32(strided)
	}

	element.strides = nil
	element.name += "(" + strings.Join(shown, ",") + ")"

	return element
}
