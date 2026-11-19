package sema

import (
	"fmt"
	"math/big"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Initial values (section 4.3): the DATA statement, and its other spelling,
// the values in slashes after a name in a type statement (sections 4.4.1,
// 4.4.2). Nothing here makes code. An initial value is bytes in the image,
// so every rule of section 4.3 comes down to which bytes an item gets:
//
//	INTEGER*2 K / '12345'X /      a hexadecimal constant is cut on the left
//	                              to the item's size: 45 23
//	CHARACTER*6 STARS / '****' /  a character constant is padded on the
//	                              right with blanks: **** and two blanks
//	REAL X / 'AB' /               so is a character constant for a number:
//	                              'A' 'B' and two blanks, whatever that
//	                              means as a REAL
//	CHARACTER BELL / 7 /          an integer is one character's code
//	REAL Y / 1 /                  a number becomes the item's type first:
//	                              1.0, in F_floating, 80 40 00 00
//
// A list of names and a list of values are walked together (dataSet): each
// item an array name or an implied-DO list expands to takes the next value,
// and a repeat count just holds the values' place for a while. Walking them
// together, rather than expanding both and comparing, means that a list with
// too many items stops at the first it has no value for, however many the
// implied-DO lists would have made.
//
// The values are worked out when every declaration has been read, since DATA
// may sit among the specification statements and come before a name's type or
// dimensions (Figure 1-1). Where each value goes in storage waits for layout
// (placeInitials), when the names have their places.

// Initial is one item's initial value.
type Initial struct {
	// Sym is the variable or array the item is, or is part of.
	Sym *Symbol

	// Offset is the item's place in Sym's common block if it is in one,
	// and otherwise in the unit's $LOCAL.
	Offset int

	// Type is the item's type: a substring's is CHARACTER of its length.
	Type Type

	// Bytes are the value, as the VAX holds it in memory.
	Bytes []byte

	// Item is the item as FORTRAN writes it, A(1,2), and Source the
	// constant as the program writes it, for the MACRO file's comments.
	Item, Source string

	// Pos is where the item is in its list.
	Pos diag.Position
}

// dataSet is one list of names with its list of values: nlist/clist/ of a
// DATA statement, or a type statement's name and values.
//
// A Fortran 90 declaration's = value is one too (initialValue), with two
// differences: the one value is every element's, if the name is an array
// (broadcast; the standard's 7.5.1.4), and it must be one an assignment
// could store (assign), so REAL :: X = 'AB' is an error where DATA's
// REAL X / 'AB' / isn't.
type dataSet struct {
	names     []ast.Expr
	values    []*ast.DataValue
	broadcast bool
	assign    bool

	// array is an initial value that is an array constant, whose elements are
	// the items' values in turn (initialValue).
	array *ArrayConst
}

// dataStatement keeps a DATA statement's lists for dataSets.
func (a *analyzer) dataStatement(s *ast.DataStmt) {
	for _, set := range s.Sets {
		a.dataSets = append(a.dataSets, dataSet{names: set.Names, values: set.Values})
	}
}

// initialValues keeps the values a type statement gives the name d declares.
// They are DATA under another name, with the same rules (sections 4.4.1 and
// 4.4.2 say "as in a DATA statement"), and one value for a scalar and one for
// each element of an array follows from them.
func (a *analyzer) initialValues(d *ast.Declarator) {
	name := &ast.Name{ExprBase: ast.ExprBase{Pos: d.Pos}, Name: d.Name}
	a.dataSets = append(a.dataSets, dataSet{names: []ast.Expr{name}, values: d.Init})
}

// initialValue keeps the value a Fortran 90 declaration gives the name d
// declares, sym, after its =, and notes that the name is initialized, which
// gives it SAVE (impliedSaves).
//
// An array may be given an array, an array constructor of constants or an
// array PARAMETER (constarray.go), of its shape (NOTCONFORM): each element its
// element of the value.
func (a *analyzer) initialValue(sym *Symbol, d *ast.Declarator) {
	name := &ast.Name{ExprBase: ast.ExprBase{Pos: d.Pos}, Name: d.Name}
	value := &ast.DataValue{Value: d.Value, Pos: d.ValuePos}
	set := dataSet{[]ast.Expr{name}, []*ast.DataValue{value}, true, true, nil}

	a.initialized = append(a.initialized, sym)

	if ev := a.declEval(); ev.isArrayForm(d.Value) {
		arr, err := ev.evalArray(d.Value)
		if err != nil {
			if err.kind != constFailed {
				a.report("DATAVALUE", err.pos, err.why)
			}

			return
		}

		if !a.initialShape(sym, arr, d.Value.Position()) {
			return
		}

		set.broadcast, set.array = false, arr
	}

	a.dataSets = append(a.dataSets, set)
}

// initialShape reports whether arr, an initial value, has sym's shape:
// NOTSCALAR for a scalar, NOTCONFORM for an array of another shape.
func (a *analyzer) initialShape(sym *Symbol, arr *ArrayConst, pos diag.Position) bool {
	if sym.Class != Array {
		a.report("NOTSCALAR", pos)

		return false
	}

	extents := make([]int, len(sym.Dims))

	for i, dim := range sym.Dims {
		if !dim.known() {
			return true // DATASIZE, from the items' walk
		}

		extents[i] = max(dim.Extent(), 0)
	}

	if !slices.Equal(extents, arr.extents()) {
		a.report("NOTCONFORM", pos, knownShape(extents...).String(), arr.S.String())

		return false
	}

	return true
}

// impliedSaves gives SAVE to each name a Fortran 90 declaration initialized
// (the standard's 5.1: "initialization ... implies the SAVE attribute"), but
// one in a common block, whose storage is the block's. The storage rule
// already keeps an initialized name static (storage.go), so this changes
// only what the name is said to be.
func (a *analyzer) impliedSaves() {
	for _, s := range a.initialized {
		if !s.broken && s.Common == nil && !s.Automatic {
			s.saved, s.f90Saved = true, true
		}
	}
}

// dataStatements works out every list's values, stopping a list at its first
// error: its other items would only be counted wrong.
func (a *analyzer) dataStatements() {
	for _, set := range a.dataSets {
		vals := &dataValues{a: a, list: set.values, broadcast: set.broadcast, assign: set.assign, array: set.array}
		ok := true

		for _, e := range set.names {
			if ok = a.dataItems(e, nil, vals); !ok {
				break
			}
		}

		if ok {
			if pos, more := vals.more(); more {
				a.report("DATACOUNT", pos, "more values than names")
			}
		}
	}
}

// dataValues walks a list of constants, repeat counts and all. Each next is
// one item's constant.
type dataValues struct {
	a    *analyzer
	list []*ast.DataValue

	// i is the next item of list to read, and left how many more times cur
	// is to be used.
	i    int
	left int64
	cur  dataValue

	// broadcast says the list's one value is every item's, and assign that
	// it must be one an assignment could store: a Fortran 90 declaration's
	// = value (initialValue).
	broadcast, assign bool

	// array, if not nil, is the value whose elements are the items' in turn,
	// and taken how many of them have been.
	array *ArrayConst
	taken int
}

// dataValue is one constant of a list. Exactly one of its fields says what it
// is: a Hollerith or character constant's bytes (text, with isText set), an
// octal or hexadecimal constant (typeless), a RADIX-50 constant's bytes (radix),
// or a constant of any other type (c).
//
// character is set for a character constant's text, not a Hollerith
// constant's, and assign for a value given with = (dataValues.assign).
type dataValue struct {
	text      string
	isText    bool
	character bool
	assign    bool
	typeless  *ast.TypelessConst
	radix     []byte
	c         *Const
	source    string
	pos       diag.Position
}

// more reports whether values are left, and where the next is.
func (v *dataValues) more() (diag.Position, bool) {
	if v.array != nil {
		return v.array.Pos, v.taken < len(v.array.Elems)
	}

	if v.broadcast {
		return diag.Position{}, false
	}

	if v.left > 0 {
		return v.cur.pos, true
	}

	if v.i < len(v.list) {
		return v.list[v.i].Pos, true
	}

	return diag.Position{}, false
}

// next is the next constant. pos is the item it is for, where running out of
// constants is reported. It returns false after an error, reported.
func (v *dataValues) next(pos diag.Position) (*dataValue, bool) {
	if v.array != nil {
		if v.taken >= len(v.array.Elems) {
			v.a.report("DATACOUNT", pos, "more names than values")

			return nil, false
		}

		c := v.array.Elems[v.taken]
		v.taken++

		v.cur = dataValue{source: DumpExpr(c), pos: v.array.Pos, assign: v.assign}
		if c.T.Kind == Character {
			v.cur.text, v.cur.isText, v.cur.character = c.Text, true, true
		} else {
			v.cur.c = c
		}

		return &v.cur, true
	}

	if v.left == 0 && v.broadcast && v.i > 0 {
		// The one value again, for the next element.
		v.left = 1
	}

	if v.left == 0 {
		if v.i >= len(v.list) {
			v.a.report("DATACOUNT", pos, "more names than values")

			return nil, false
		}

		item := v.list[v.i]
		v.i++

		n, ok := v.a.repeatCount(item)
		if !ok {
			return nil, false
		}

		v.left = n

		if v.cur, ok = v.a.dataConstant(item); !ok {
			return nil, false
		}

		v.cur.assign = v.assign
	}

	v.left--

	return &v.cur, true
}

// repeatCount is an item's repeat count, 1 without one: "a nonzero, unsigned
// integer constant or the symbolic name of an integer constant".
func (a *analyzer) repeatCount(item *ast.DataValue) (int64, bool) {
	if item.Repeat == nil {
		return 1, true
	}

	switch item.Repeat.(type) {
	case *ast.IntConst, *ast.Name:
	default:
		a.report("DATAREPEAT", item.Repeat.Position())

		return 0, false
	}

	c, err := a.declEval().eval(item.Repeat)

	switch {
	case err != nil && err.kind == constFailed:
		return 0, false
	case err != nil || c.T.Kind != Integer || c.Int <= 0:
		a.report("DATAREPEAT", item.Repeat.Position())

		return 0, false
	}

	return int64(c.Int), true
}

// dataConstant reads the constant of a list's item. A Hollerith constant has
// only its bytes, and an octal or hexadecimal one takes its size from the item
// it is for, so neither is a Const yet.
func (a *analyzer) dataConstant(item *ast.DataValue) (dataValue, bool) {
	v := dataValue{source: constantText(item.Value), pos: item.Pos}

	switch e := item.Value.(type) {
	case *ast.HollerithConst:
		v.text, v.isText = e.Value, true

		return v, true

	case *ast.TypelessConst:
		v.typeless = e

		_, err := a.declEval().typelessValue(e)

		return v, err == nil

	case *ast.Radix50Const:
		var ok bool
		v.radix, ok = a.radix50Bytes(e)

		return v, ok
	}

	c, err := a.declEval().eval(item.Value)

	switch {
	case err != nil && err.kind == constFailed:
		return v, false

	case err != nil:
		a.report("DATAVALUE", err.pos, err.why)

		return v, false

	case c.T.Kind == Character:
		v.text, v.isText, v.character = c.Text, true, true

		return v, true
	}

	v.c = c

	return v, true
}

// constantText is a constant as the program writes it, a sign and all.
func constantText(e ast.Expr) string {
	if u, ok := e.(*ast.Unary); ok {
		if u.Op == ast.Sub {
			return "-" + constantText(u.X)
		}

		return "+" + constantText(u.X)
	}

	return ast.DumpExpr(e)
}

// dataItems takes the items e expands to, giving each the next constant: a
// variable is one item, an array every element in storage order, an implied-DO
// list its items once for each value of its variable. vars are the variables
// of the implied-DO lists e is inside. It returns false after an error,
// reported.
func (a *analyzer) dataItems(e ast.Expr, vars map[string]int32, vals *dataValues) bool {
	switch e := a.substringForm(e).(type) {
	case *ast.Name:
		s, ok := a.dataName(e.Name, e.Pos)
		if !ok {
			return false
		}

		if s.Class != Array {
			return a.dataItem(s, 0, s.Type, s.Name, e.Pos, vals)
		}

		if s.Size() == 0 {
			a.report("DATASIZE", e.Pos, s.Name)

			return false
		}

		for n := range s.Elements() {
			if !a.dataItem(s, n*s.Type.Size, s.Type, s.elementText(n), e.Pos, vals) {
				return false
			}
		}

		return true

	case *ast.Apply:
		s, number, ok := a.dataElement(e, vars)
		if !ok {
			return false
		}

		return a.dataItem(s, number*s.Type.Size, s.Type, s.elementText(number), e.Pos, vals)

	case *ast.Substring:
		return a.dataSubstring(e, vars, vals)

	case *ast.ImpliedDo:
		return a.dataImpliedDo(e, vars, vals)

	case *ast.FieldRef:
		if a.derivedField(e) {
			return a.dataComponent(e, vars, vals)
		}
	}

	a.report("DATAFORM", e.Position())

	return false
}

// dataItem gives one item the next constant.
func (a *analyzer) dataItem(s *Symbol, offset int, t Type, text string, pos diag.Position, vals *dataValues) bool {
	if t.Kind == Record {
		return a.dataRecordItem(s, offset, t, text, pos, vals)
	}

	v, ok := vals.next(pos)
	if !ok {
		return false
	}

	b, ok := a.initialBytes(v, t)
	if !ok {
		return false
	}

	a.unit.Initials = append(a.unit.Initials, &Initial{
		Sym: s, Offset: offset, Type: t, Bytes: b, Item: text, Source: v.source, Pos: pos,
	})

	return true
}

// dataName is the symbol of a name in a list, if it may be given an initial
// value. Dummy arguments, functions, and blank common may not (the standard's
// rule, which the manual doesn't state), and a block data subprogram
// initializes only what is in a named block (section 4.1 says "the entities in
// named common blocks", and nothing else could reach a program from one).
func (a *analyzer) dataName(name string, pos diag.Position) (*Symbol, bool) {
	s := a.local(name, pos)
	a.note(&s.refs, pos, RefAssigned)

	switch {
	case s.broken || s.Type.Kind == Invalid:
		return s, false

	case s.Dummy != 0:
		a.report("DATADUMMY", pos, name)

		s.broken = true

		return s, false

	case s.Allocatable:
		a.report("ALLOCATTR", pos, name, "be given an initial value")

		s.broken = true

		return s, false

	case s.AutomaticObject:
		a.autoAttr(s, pos, name, "be given an initial value")

		s.broken = true

		return s, false

	case s.Class == UnitName || s.Result || s.Class == Constant || s.Class == StmtFunction ||
		s.Class == External || s.Class == Intrinsic:
		a.badUse(s, pos, "be given an initial value")

		s.broken = true

		return s, false

	case s.Struct != nil && !s.Struct.Derived:
		// Section 4.13: its initial values are its structure's. A derived
		// type's object may be (datatypes.go).
		a.badUse(s, pos, "be in a DATA statement")

		s.broken = true

		return s, false

	case s.Common != nil && s.Common.Name == "":
		a.report("DATABLANK", pos, name)

		return s, false

	case s.Common == nil && a.unit.Kind == ast.BlockDataUnit:
		a.report("DATABLOCK", pos, name)

		return s, false
	}

	if s.Class == Unclassified {
		s.Class = Variable
	}

	return s, true
}

// dataElement is an array element in a list: its array and its number in
// storage order.
func (a *analyzer) dataElement(e *ast.Apply, vars map[string]int32) (*Symbol, int, bool) {
	s, ok := a.dataName(e.Name, e.Pos)
	if !ok {
		return s, 0, false
	}

	if s.Class != Array {
		a.report("NOTARRAY", e.Pos, e.Name)

		return s, 0, false
	}

	if hasTriplet(e.Args) {
		// Fortran 90's DATA may name a section (the standard's R537).
		a.notImplForm(e.Pos, "an array section")

		return s, 0, false
	}

	if len(e.Args) != len(s.Dims) {
		a.report("SUBSCRIPTS", e.Pos, e.Name, count(len(s.Dims), "subscript"), len(e.Args))

		return s, 0, false
	}

	if a.nullSubscript(e.Pos, e.Args) {
		return s, 0, false
	}

	subs := make([]int, len(e.Args))

	for i, arg := range e.Args {
		v, ok := a.dataInt(arg, vars)
		if !ok {
			return s, 0, false
		}

		subs[i] = int(v)
	}

	number, ok := s.ElementNumber(subs)
	if !ok {
		a.report("DATARANGE", e.Pos, e.Name)

		return s, 0, false
	}

	return s, number, true
}

// dataSubstring is a substring in a list: one item, of its own length.
func (a *analyzer) dataSubstring(e *ast.Substring, vars map[string]int32, vals *dataValues) bool {
	var (
		s    *Symbol
		item dataSlot
		ok   bool
	)

	switch x := e.X.(type) {
	case *ast.Name:
		if s, ok = a.dataName(x.Name, x.Pos); ok && s.Class == Array {
			a.report("NEEDSUBSCRIPT", x.Pos, x.Name)

			return false
		}

		item = dataSlot{0, s.Type, s.Name}

	case *ast.Apply:
		var number int

		if s, number, ok = a.dataElement(x, vars); ok {
			item = dataSlot{number * s.Type.Size, s.Type, s.elementText(number)}
		}

	case *ast.FieldRef:
		// ORIGIN%NAME(2:4), a component's substring (datatypes.go).
		if !a.derivedField(x) {
			a.report("DATAFORM", e.Pos)

			return false
		}

		s, item, ok = a.dataComponentSubstring(x, vars)

	default:
		a.report("DATAFORM", e.Pos)
	}

	if !ok {
		return false
	}

	// The messages name an array's element by its array, a component by
	// itself.
	named := s.Name
	if _, ok := e.X.(*ast.FieldRef); ok {
		named = item.text
	}

	if item.t.Kind != Character {
		a.report("DATASUBSTR", e.Pos, named, item.t.String())

		return false
	}

	first, last := 1, item.t.Size

	if e.Lower != nil {
		v, ok := a.dataInt(e.Lower, vars)
		if !ok {
			return false
		}

		first = int(v)
	}

	if e.Upper != nil {
		v, ok := a.dataInt(e.Upper, vars)
		if !ok {
			return false
		}

		last = int(v)
	}

	if first < 1 || first > last || last > item.t.Size {
		a.report("DATASUBRANGE", e.Pos, first, last, named, item.t.Size)

		return false
	}

	return a.dataItem(s, item.offset+first-1, Type{Kind: Character, Size: last - first + 1},
		substringText(item.text, first, last), e.Pos, vals)
}

// dataImpliedDo is an implied-DO list, (dlist, i=n1,n2[,n3]). Its variable
// exists only inside the list: it is not the unit's variable of that name, and
// isn't entered in the symbol table. The bounds may use the variables of the
// lists it is inside. The count is section 5.3.1's, as in a DO statement.
func (a *analyzer) dataImpliedDo(e *ast.ImpliedDo, vars map[string]int32, vals *dataValues) bool {
	name := e.Var.Name

	if _, inUse := vars[name]; inUse {
		a.report("DATADOVAR", e.Var.Pos, name, "is already an enclosing implied-DO list's variable")

		return false
	}

	if !a.integerName(name) {
		a.report("DATADOVAR", e.Var.Pos, name, "must be an INTEGER variable")

		return false
	}

	start, ok := a.dataInt(e.Start, vars)
	if !ok {
		return false
	}

	limit, ok := a.dataInt(e.Limit, vars)
	if !ok {
		return false
	}

	step := int32(1)

	if e.Step != nil {
		if step, ok = a.dataInt(e.Step, vars); !ok {
			return false
		}

		if step == 0 {
			a.report("DATADOSTEP", e.Step.Position())

			return false
		}
	}

	// The loop's variable is added to a copy of the outer loops' variables,
	// not to vars itself: a Go map is a reference, so changing vars would
	// change the caller's too, and the variable would still seem in use
	// after this list.
	inner := make(map[string]int32, len(vars)+1)
	for k, v := range vars {
		inner[k] = v
	}

	// FORTRAN's trip count for DO (section 5.3.1.1): (limit - start +
	// step) / step, truncated, and none if that is 0 or less. It's computed
	// once, before the first trip, as for an executable DO loop.
	//
	// A count in 64 bits, since the difference of two INTEGERs mightn't fit
	// in 32.
	n := (int64(limit) - int64(start) + int64(step)) / int64(step)
	v := int64(start)

	for ; n > 0; n-- {
		inner[name] = int32(v)

		for _, item := range e.Items {
			if _, bare := item.(*ast.Name); bare {
				a.report("DATAFORM", item.Position())

				return false
			}

			if !a.dataItems(item, inner, vals) {
				return false
			}
		}

		v += int64(step)
	}

	return true
}

// integerName reports whether the name, as an implied-DO variable, has an
// INTEGER type: its declaration's, or its first letter's. A constant, or an
// array, can't be one.
func (a *analyzer) integerName(name string) bool {
	if s := a.scope.find(name); s != nil {
		return s.Type.Kind == Integer && s.Class != Constant && s.Class != Array
	}

	t, _ := a.scope.rules().typeOf(name)

	return t.Kind == Integer
}

// dataInt is the value of a subscript, a substring bound, or an implied-DO
// limit: an integer constant expression that may use the variables of the
// implied-DO lists it is inside.
func (a *analyzer) dataInt(e ast.Expr, vars map[string]int32) (int32, bool) {
	ev := a.declEval()
	ev.vars = vars

	c, err := ev.eval(e)

	switch {
	case err != nil && err.kind == constFailed:
		return 0, false
	case err != nil || c.T.Kind != Integer:
		a.report("DATACONST", e.Position())

		return 0, false
	}

	return c.Int, true
}

// initialBytes is the bytes the constant v gives an item of type t, by section
// 4.3's rules.
func (a *analyzer) initialBytes(v *dataValue, t Type) ([]byte, bool) {
	if v.assign && !assignable(v, t) {
		what := "a CHARACTER"
		if v.c != nil {
			what = article(v.c.T.String())
		}

		a.report("INITTYPE", v.pos, what, article(t.String()))

		return nil, false
	}

	switch {
	case v.isText:
		// On the right: blanks if it's short, cut if it's long, whichever
		// type the item is.
		b := make([]byte, t.Size)

		for i := range b {
			b[i] = ' '
		}

		copy(b, v.text)

		return b, true

	case v.radix != nil:
		// Appendix B.3: zero bytes on the right if it's short, and if it's
		// long, the rightmost characters not used.
		b := make([]byte, t.Size)
		copy(b, v.radix)

		return b, true

	case v.typeless != nil:
		value, ok := new(big.Int).SetString(v.typeless.Digits, v.typeless.Radix)
		if !ok {
			return nil, false
		}

		if t.Kind == Character && (t.Size != 1 || value.Cmp(big.NewInt(255)) > 0) {
			a.report("DATACHAR", v.pos)

			return nil, false
		}

		return littleEndian(value, t.Size), true

	case t.Kind == Character:
		if t.Size != 1 || v.c.T.Kind != Integer || v.c.Int < 0 || v.c.Int > 255 {
			a.report("DATACHAR", v.pos)

			return nil, false
		}

		return []byte{byte(v.c.Int)}, true
	}

	c, err := a.declEval().convert(v.c, t, v.pos)

	switch {
	case err == incompatible:
		a.report("DATATYPE", v.pos, article(v.c.T.String()), article(t.String()))

		return nil, false

	case err != nil:
		return nil, false
	}

	return a.constBytes(c), true
}

// assignable reports whether an assignment could store v in an item of type
// t, as a value given with = must be (the standard's 7.5.1.4): a character
// constant only in a CHARACTER item, and a number or logical value only in
// another. Hollerith, octal, hexadecimal, and RADIX-50 constants are DEC's,
// typeless, and go anywhere DATA lets them.
func assignable(v *dataValue, t Type) bool {
	switch {
	case v.character:
		return t.Kind == Character
	case v.c != nil:
		return t.Kind != Character
	}

	return true
}

// littleEndian is v's low size bytes, lowest first: extended on the left with
// zeros, or cut on the left. A VAX number's bytes are lowest first, and a
// floating value's bits are the same longwords: '00004080'X is 1.0.
func littleEndian(v *big.Int, size int) []byte {
	b := make([]byte, size)
	digits := v.Bytes() // highest first

	for i := 0; i < size && i < len(digits); i++ {
		b[i] = digits[len(digits)-1-i]
	}

	return b
}

// constBytes is a constant's bytes as the VAX holds it in memory.
func (a *analyzer) constBytes(c *Const) []byte {
	switch c.T.Kind {
	case Integer, Logical:
		// The VAX is little-endian: the lowest byte goes at the lowest
		// address. Byte i is the value shifted right by 8*i bits, its low
		// 8 bits kept by the conversion to byte.
		b := make([]byte, c.T.Size)

		for i := range b {
			b[i] = byte(uint32(c.Int) >> (8 * i))
		}

		return b

	case Real:
		return floatBytes(a.format(c.T), c.Real)

	case Complex:
		return append(floatBytes(a.format(c.T), c.Real), floatBytes(a.format(c.T), c.Imag)...)
	}

	return []byte(c.Text)
}

// floatBytes is a floating value's bytes in format f, lowest address first.
func floatBytes(f vaxfloat.Format, v vaxfloat.Value) []byte {
	// Pack gives the bytes the value occupies in memory, read as a
	// little-endian integer in two 64-bit halves, Lo the first 8 bytes and
	// Hi the next 8 (the VAX's floating formats store their 16-bit words in
	// an order of their own, which vaxfloat has already dealt with). Spread
	// them back out a byte at a time, then keep as
	// many bytes as the format has: 4 for F, 8 for D and G, 16 for H.
	// "range 8" (Go 1.22 on) counts i from 0 to 7.
	bits, _ := vaxfloat.Pack(f, v)
	b := make([]byte, 16)

	for i := range 8 {
		b[i] = byte(bits.Lo >> (8 * i))
		b[8+i] = byte(bits.Hi >> (8 * i))
	}

	return b[:f.Size()]
}

// elementText is the array element number n, from 0, as FORTRAN writes it:
// A(1,2).
func (s *Symbol) elementText(n int) string {
	subs := make([]string, len(s.Dims))

	for i, d := range s.Dims {
		extent := d.Extent()
		if i == len(s.Dims)-1 {
			extent = n + 1 // the last dimension's extent is never needed
		}

		subs[i] = strconv.Itoa(d.Lower + n%extent)
		n /= extent
	}

	return s.Name + "(" + strings.Join(subs, ",") + ")"
}

// placeInitials gives each initial value its place in storage, now that its
// name has one. Two values for one byte are an error (DATATWICE), but only
// $LOCAL's are checked here: a common block's are checked across the file's
// units (checkCommonInitials), as its one image is shared.
func (a *analyzer) placeInitials() {
	var local []*Initial

	for _, in := range a.unit.Initials {
		in.Offset += in.Sym.Offset

		if in.Sym.Common == nil {
			local = append(local, in)
		}
	}

	// $LOCAL's first, then each block's, each in order of place.
	section := func(in *Initial) string {
		if in.Sym.Common == nil {
			return ""
		}

		return in.Sym.Common.Psect
	}

	sort.SliceStable(a.unit.Initials, func(i, j int) bool {
		x, y := a.unit.Initials[i], a.unit.Initials[j]
		if section(x) != section(y) {
			return section(x) < section(y)
		}

		return x.Offset < y.Offset
	})

	reportTwice(local, a.messages)
}

// checkCommonInitials reports two values for one byte of a common block,
// among all the file's units (their names may differ). A value another unit has
// given already, at the same place with the same bytes, is that value again, as
// when each unit INCLUDEs one declaration, so it is no error, and it leaves its
// unit's list: the block has one image, with one value per byte (ours; each unit
// of VAX FORTRAN's own was a module, and LINK kept the last). Whether two
// separately compiled modules may differ is LINK's business too.
func checkCommonInitials(prog *Program, messages *diag.List) {
	blocks := map[string][]*Initial{}

	var names []string

	dropRepeatedInitials(prog)

	for _, u := range prog.Units {
		for _, in := range u.Initials {
			if b := in.Sym.Common; b != nil {
				if blocks[b.Psect] == nil {
					names = append(names, b.Psect)
				}

				blocks[b.Psect] = append(blocks[b.Psect], in)
			}
		}
	}

	for _, name := range names {
		reportTwice(blocks[name], messages)
	}
}

// dropRepeatedInitials takes out of each unit's list the common block values an
// earlier unit gives identically. Only the first of a unit's own repeats goes: a
// unit that says the same thing twice has given two values, as it has in $LOCAL.
func dropRepeatedInitials(prog *Program) {
	type value struct {
		psect  string
		offset int
		bytes  string
	}

	seen := map[value]bool{}

	for _, u := range prog.Units {
		mine := map[value]bool{}
		kept := u.Initials[:0:0]

		for _, in := range u.Initials {
			if in.Sym.Common == nil {
				kept = append(kept, in)

				continue
			}

			v := value{in.Sym.Common.Psect, in.Offset, string(in.Bytes)}

			if !seen[v] || mine[v] {
				kept = append(kept, in)
			}

			mine[v] = true
		}

		u.Initials = kept

		for v := range mine {
			seen[v] = true
		}
	}
}

// reportTwice reports the initial values that share a byte with another, in
// the list given, each place in the source once. The later of the two in the
// source gets the message, naming the earlier's line.
func reportTwice(list []*Initial, messages *diag.List) {
	sort.SliceStable(list, func(i, j int) bool { return list[i].Offset < list[j].Offset })

	reported := map[diag.Position]bool{}

	var last *Initial

	for _, in := range list {
		if last != nil && in.Offset < last.Offset+len(last.Bytes) {
			first, second := last, in
			if before(second.Pos, first.Pos) {
				first, second = second, first
			}

			if !reported[second.Pos] {
				reported[second.Pos] = true

				messages.Report("DATATWICE", second.Pos, second.Item, first.Pos.Line)
			}
		}

		if last == nil || in.Offset+len(in.Bytes) > last.Offset+len(last.Bytes) {
			last = in
		}
	}
}

// before reports whether x comes before y in the source.
func before(x, y diag.Position) bool {
	if x.Line != y.Line {
		return x.Line < y.Line
	}

	return x.Column < y.Column
}

// dumpInitial is an initial value's line in the dump: where it goes, the item,
// and its bytes.
func dumpInitial(in *Initial) string {
	section := "$LOCAL"
	if in.Sym.Common != nil {
		section = in.Sym.Common.Psect
	}

	return fmt.Sprintf("  initial %s+%d %s %s = % X", section, in.Offset, in.Item, in.Type, in.Bytes)
}
