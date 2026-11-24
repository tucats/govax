package ir

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// The I/O statements (Decision 12 of docs/planning/PHASE-74 - FORTRAN plan.md, and
// docs/planning/PHASE-80 - FORTRAN formatted IO.md). Each becomes FTNRTL calls: a
// begin call chosen by the statement's kind, one FTN$IO_ITEM call per
// item (an implied-DO list a loop around its items' calls), and
// FTN$IO_END, which finishes the statement and returns its status in R0.
//
//	WRITE (6, 10, ERR=99) I, (A(J), J = 1, 3)
//
//	  call FTN$WRITE_SF($IO1, 6, $F10)	the begin call
//	  call FTN$IO_ITEM(<descriptor of I>)
//	  ...					the loop: J, its count, and
//	  call FTN$IO_ITEM(<descriptor of A(J)>)	 an item call each pass
//	  ...
//	  call FTN$IO_END -> $T1
//	  sign $T1 B3 B3 L99			an error goes to 99
//
// The begin call's arguments are the control block (its flags: which of
// ERR=, END=, and IOSTAT= the statement has, whether its format is a
// run-time one, and whether its unit was compiled /F90), the unit (a
// number, by value; an internal file's descriptor; ENCODE's or DECODE's
// buffer's address), and the format (a
// FORMAT statement's block, the address an ASSIGNed variable holds, a
// run-time format's descriptor, or a NAMELIST group's block); a
// list-directed or unformatted statement has no format argument. ENCODE and DECODE have a
// fourth, the count of characters, by value: a longword, since VMS's take
// counts past a descriptor's word. A direct access statement's last argument
// is its record number, by value (docs/planning/PHASE-81 - FORTRAN files.md, Decision
// 4), and a keyed READ's last three its key's descriptor, its key number by value,
// -1 with no KEYID= so that the unit goes on with its last (section 7.1.1.8), and
// how a record's key must compare with it (sema's Key constants, by value): VMS's
// FORRTL's arguments (testdata/for88). REWRITE's begin call is FTN$REWRITE_SF or _SU, a keyed
// READ's FTN$READ_KF or _KU.
//
// FTN$IO_END's status is 0 for success, negative at an end of file, and
// positive (the FOR number) for an error. A statement with none of ERR=,
// END=, and IOSTAT= never sees a failure, since FTNRTL signals it; one
// with them gets its status, stores it in IOSTAT's variable, and goes to
// END= (or ERR= without it) when it is negative and ERR= when positive.

// The FTNRTL routines the I/O statements call.
const (
	ftnIOItem  = "FTN$IO_ITEM"
	ftnIOEnd   = "FTN$IO_END"
	ftnAdvance = "FTN$ADVANCE"
)

// io lowers an I/O statement.
func (l *lowerer) io(s *sema.IO) {
	c := controlFor(s.Err, s.End, s.IOStat)

	if s.RunFormat != nil {
		c.Flags |= ControlRunFormat
	}

	if l.unit.F90 {
		c.Flags |= ControlF90
	}

	if s.Nonadvancing {
		c.Flags |= ControlNonadvancing
	}

	if s.EOR != 0 {
		c.Flags |= ControlEOR
	}

	// An assumed-shape array as the internal file, the buffer, or the format
	// goes through a temporary unless its elements are one after another
	// (lowersequence.go); the file's or buffer's is copied back once the
	// statement is done.
	var backs []func()

	begin := func(c Control) {
		control := Arg{Ref: l.control(c)}

		unit, back := l.ioUnit(s)
		if back != nil {
			backs = append(backs, back)
		}

		args := []Arg{control, unit}

		if s.Form != sema.ListForm && s.Form != sema.UnformattedForm {
			args = append(args, l.ioFormat(s))
		}

		if s.Code {
			args = append(args, Arg{Value: l.expr(s.Count)})
		}

		if s.Rec != nil {
			args = append(args, Arg{Value: l.expr(s.Rec)})
		}

		if s.Key != nil {
			var id Expr = LongConst(-1)
			if s.KeyID != nil {
				id = l.expr(s.KeyID)
			}

			args = append(args, Arg{Ref: l.item(s.Key)}, Arg{Value: id}, Arg{Value: LongConst(int32(s.KeyMatch))})
		}

		l.emit(&Call{origin: l.origin(), Routine: beginRoutine(s), Args: args})
	}

	if s.Advance != nil {
		l.advanceBegin(s.Advance, c, begin)
	} else {
		begin(c)
	}

	l.byComponents = s.Form != sema.UnformattedForm
	l.ioItems(s.Items)
	l.byComponents = false

	// SIZE='s count FTN$IO_END stores in a longword, which is then stored in
	// the variable, of whatever INTEGER type it is, with the copies back.
	end := &Call{origin: l.origin(), Routine: ftnIOEnd}

	if s.Size != nil {
		count := l.temp(Long)
		end.Args = []Arg{{Ref: count}}

		backs = append(backs, func() {
			size := l.mem(s.Size)

			var value Expr = &Load{count}
			if size.T != Long {
				value = arith(Cvt, size.T, value, nil)
			}

			l.emit(&Assign{origin: l.origin(), Dst: size, Src: value})
		})
	}

	mayEOR := s.Read && (s.Nonadvancing || s.Advance != nil)

	l.ioFinish(end, c, s.Err, s.End, s.IOStat, backs, s.EOR, mayEOR)
}

// advanceBegin begins a statement whose ADVANCE= only the program knows
// (docs/planning/PHASE-109 - Fortran 90 IO.md, finding 2): FTNRTL's
// FTN$ADVANCE reads the value, 0 for YES, 1 for NO, 2 for anything else, and a
// Switch on it chooses one of three begin calls, each with a control block of
// its own, so that FTNRTL knows from the start whether the statement continues
// a record. The items and the end are the three's.
//
//	  call FTN$ADVANCE(<descriptor of the value>) -> $T1
//	  switch $T1 base 0 [B2 B3] else B4
//	B2: call FTN$READ_SF($IO1, ...)		advancing
//	    jump B5
//	B3: call FTN$READ_SF($IO2, ...)		nonadvancing
//	    jump B5
//	B4: call FTN$READ_SF($IO3, ...)		FOR$_BADADVANCE
//	B5: ...the items
func (l *lowerer) advanceBegin(advance sema.Expr, c Control, begin func(Control)) {
	which := l.temp(Long)
	l.emit(&Call{origin: l.origin(), Routine: ftnAdvance, Args: []Arg{{Ref: l.item(advance)}}, Result: which})

	yes, no, bad, next := l.newBlock(), l.newBlock(), l.newBlock(), l.newBlock()

	// The statement isn't over, so what is to be done at its end waits.
	l.holdingAfter(func() {
		l.terminate(&Switch{l.origin(), &Load{which}, 0, []*Block{yes, no}, bad})
	})

	for i, b := range []*Block{yes, no, bad} {
		l.begin(b)
		begin(Control{Flags: c.Flags | []int{0, ControlNonadvancing, ControlBadAdvance}[i]})

		if b != bad {
			l.holdingAfter(func() { l.jump(next) })
		}
	}

	l.begin(next)
}

// controlFor is the control block of a statement with these specifiers.
func controlFor(errLabel, endLabel int, iostat sema.Expr) Control {
	c := Control{}

	if errLabel != 0 {
		c.Flags |= ControlErr
	}

	if endLabel != 0 {
		c.Flags |= ControlEnd
	}

	if iostat != nil {
		c.Flags |= ControlIOStat
	}

	return c
}

// ioFinish emits the call that ends an I/O statement and returns its
// status in R0, then backs (copies back to an assumed-shape array, done once
// the statement is, lowersequence.go; SIZE='s count), and what the statement
// does with the status: IOSTAT=, END=, ERR=, and EOR=.
//
// mayEOR is set for a READ that may be nonadvancing, whose status may be -2, an
// end of record, which FTNRTL returns only when EOR= or IOSTAT= is given. It
// goes to EOR=, or with IOSTAT= alone on to the next statement, never to END=
// or ERR=, which the Sign would send a negative status to.
func (l *lowerer) ioFinish(end *Call, c Control, errLabel, endLabel int, iostat sema.Expr, backs []func(),
	eorLabel int, mayEOR bool,
) {
	if c.Flags&(ControlErr|ControlEnd|ControlIOStat|ControlEOR) == 0 {
		l.emit(end)
		copyBack(backs)

		return
	}

	// eorBranch is set when -2 needs a branch of its own.
	eorBranch := mayEOR && (eorLabel != 0 || (iostat != nil && (errLabel != 0 || endLabel != 0)))

	// The status, kept in a temporary while the IOSTAT variable is set
	// and the statement's last work is done (both may use R0).
	var status Expr = &CallValue{}

	if iostat != nil || len(l.after) > 0 || len(backs) > 0 || eorBranch {
		end.Result = l.temp(Long)
		status = &Load{end.Result}
	}

	l.emit(end)
	copyBack(backs)

	if iostat != nil {
		ios := l.mem(iostat)

		value := status
		if ios.T != Long {
			value = arith(Cvt, ios.T, status, nil)
		}

		l.emit(&Assign{origin: l.origin(), Dst: ios, Src: value})
	}

	if !eorBranch && errLabel == 0 && endLabel == 0 {
		return
	}

	next := l.newBlock()

	if eorBranch {
		eor := next
		if eorLabel != 0 {
			eor = l.labelBlock(eorLabel)
		}

		rest := next
		if errLabel != 0 || endLabel != 0 {
			rest = l.newBlock()
		}

		l.branch(&Cmp{Op: Eq, X: status, Y: LongConst(-2)}, eor, rest)

		if rest == next {
			l.begin(next)

			return
		}

		l.begin(rest)
	}

	neg, pos := next, next

	switch {
	case endLabel != 0:
		neg = l.labelBlock(endLabel)
	case errLabel != 0:
		neg = l.labelBlock(errLabel)
	}

	if errLabel != 0 {
		pos = l.labelBlock(errLabel)
	}

	l.terminate(&Sign{l.origin(), status, neg, next, pos})
	l.begin(next)
}

// beginRoutine is the FTNRTL routine that begins statement s: FTN$, the
// direction, and the access and form, as VMS's FORTRAN run-time library
// names its own: S sequential, D direct, or I internal, then F formatted, L
// list-directed, N namelist, or U unformatted (FTN$WRITE_SF, FTN$READ_IL,
// FTN$WRITE_SU, FTN$READ_DF); or FTN$ENCODE and FTN$DECODE.
func beginRoutine(s *sema.IO) string {
	switch {
	case s.Code && s.Read:
		return "FTN$DECODE"
	case s.Code:
		return "FTN$ENCODE"
	}

	name := "FTN$WRITE_"

	switch {
	case s.Rewrite:
		name = "FTN$REWRITE_"
	case s.Read:
		name = "FTN$READ_"
	}

	switch {
	case s.Key != nil:
		name += "K"
	case s.Internal:
		name += "I"
	case s.Rec != nil:
		name += "D"
	default:
		name += "S"
	}

	return name + [...]string{sema.ListForm: "L", sema.FormattedForm: "F", sema.NamelistForm: "N", sema.UnformattedForm: "U"}[s.Form]
}

// ioUnit is the begin call's unit argument: a unit's number by value (a
// default unit's from sema), an internal file's descriptor, or ENCODE's
// or DECODE's buffer's address.
//
// An assumed-shape array or a section as the internal file or buffer goes as
// its elements one after another (lowersequence.go: in place if they are, else
// through a temporary), and back is what copies them back when the statement
// writes them; nil if nothing does.
func (l *lowerer) ioUnit(s *sema.IO) (unit Arg, back func()) {
	if s.Code || s.Internal {
		r, ok := s.Unit.(*sema.ArrayRef)
		if ok && r.Sym.Strided() || !ok && len(s.Unit.Shape()) > 0 {
			seq := l.sequence(s.Unit, true, !s.Read, s.Internal)

			return Arg{Value: seq.value}, seq.back
		}
	}

	switch {
	case s.Code:
		return Arg{Value: l.bufferAddress(s.Unit)}, nil
	case s.Internal:
		return Arg{Ref: l.item(s.Unit)}, nil
	case s.Unit == nil:
		return Arg{Value: LongConst(int32(s.Default))}, nil
	}

	return Arg{Value: l.expr(s.Unit)}, nil
}

// bufferAddress is the address of ENCODE's or DECODE's buffer, whatever
// its type (appendix A.1).
func (l *lowerer) bufferAddress(e sema.Expr) Expr {
	switch b := e.(type) {
	case *sema.ArrayRef:
		return &Addr{l.symbolMem(b.Sym)}
	case *sema.Substring:
		return &Addr{l.chars(b).Mem}
	}

	if e.Type().Kind == sema.Character {
		return &Addr{l.chars(e).Mem}
	}

	return &Addr{l.mem(e)}
}

// ioFormat is the begin call's format argument.
func (l *lowerer) ioFormat(s *sema.IO) Arg {
	switch {
	case s.Form == sema.NamelistForm:
		return Arg{Ref: l.namelist(s.Group)}
	case s.Format != nil:
		return Arg{Ref: l.formatBlock(s.Format)}
	case s.FormatVar != nil:
		// The address ASSIGN stored, passed as it is.
		return Arg{Value: l.expr(s.FormatVar)}
	}

	// An assumed-shape array's text, its elements one after another
	// (lowersequence.go).
	if r, ok := s.RunFormat.(*sema.ArrayRef); ok && r.Sym.Strided() {
		return Arg{Value: l.sequence(r, true, false, true).value}
	}

	return Arg{Ref: l.runFormat(s.RunFormat)}
}

// runFormat is the descriptor of a run-time format (section 8.7): a
// CHARACTER value's, an array's (whose elements, of any type, are its text
// one after another), or for an element of a numeric array, a text from
// the element to the array's end. An array whose size isn't known has its
// element's text run as far as a descriptor's length can say: FTNRTL reads
// a format only to its last parenthesis.
func (l *lowerer) runFormat(e sema.Expr) *Mem {
	switch e := e.(type) {
	case *sema.ArrayRef:
		return l.arrayDescriptor(e.Sym)

	case *sema.Elem:
		if e.Type().Kind == sema.Character {
			break
		}

		m := l.mem(e)
		d := &Descriptor{DType: DTypeT, Class: ClassS}
		c := l.descriptorCell(d)

		var length Expr = LongConst(65535)

		if n := e.Sym.Elements(); n > 0 {
			// The array's bytes less those before the element.
			before := arith(Sub, Long, &Addr{m}, &Addr{l.symbolMem(e.Sym)})
			length = arith(Sub, Long, LongConst(int32(n*e.Sym.Type.Size)), before)
		}

		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, T: Word}, Src: arith(Cvt, Word, length, nil)})
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, Offset: PointerOffset, T: Long}, Src: &Addr{m}})

		return &Mem{Cell: c, T: Long}
	}

	return l.item(e)
}

// ioItems lowers an I/O list: an FTN$IO_ITEM call for each item, with its
// descriptor (item: an input item's is where it is, never a copy), and a
// loop for each implied-DO list.
func (l *lowerer) ioItems(items []*sema.Item) {
	for _, it := range items {
		if it.Loop != nil {
			l.impliedDo(it.Loop)

			continue
		}

		l.ioItem(it.Value)
	}
}

// ioItem lowers one item of an I/O list. A scalar, or a whole array of the
// program's (one class A descriptor, all of it in one call), is one call; so is
// a section or an assumed-shape array, by a class NCA descriptor (ncaItem). Any
// other array (a vector subscript, an array expression, an array constant) is a call
// for each element, in array element order, through the scalarizer's loops
// (docs/planning/PHASE-102 - Fortran 90 array expressions.md, finding 14): an
// input element is a place, read where it is, and an output
// expression's element a value computed into a temporary, as a scalar
// expression's is. A constructor's values are items in turn, an implied DO of
// it a loop around its own: the constructor is never built. The extents and
// subscripts are computed when the item is reached, so READ *, N, A(1:N) uses
// the N just read.
func (l *lowerer) ioItem(v sema.Expr) {
	if l.byComponents && v.Type().Kind == sema.Record {
		l.objectItems(v)

		return
	}

	call := func(e sema.Expr) {
		l.emit(&Call{origin: l.origin(), Routine: ftnIOItem, Args: []Arg{{Ref: l.item(e)}}})
	}

	// An array whose elements needn't be one after another is one call too,
	// with a class NCA descriptor, which FTNRTL walks.
	if d := l.ncaItem(v); d != nil {
		l.emit(&Call{origin: l.origin(), Routine: ftnIOItem, Args: []Arg{{Ref: d}}})

		return
	}

	if len(v.Shape()) == 0 {
		call(v)

		return
	}

	switch v := v.(type) {
	case *sema.ArrayRef:
		call(v)
	case *sema.Constructor:
		l.fillItems(v.Items, call)
	default:
		l.eachElement(v, call)
	}
}

// ncaItem is the class NCA descriptor of the I/O item v, or nil if v isn't
// one an NCA descriptor gives (docs/planning/PHASE-109 - Fortran 90 IO.md,
// finding 11): a whole assumed-shape array or array pointer's target (whose
// elements may not be one after another, which a class A descriptor would say
// they are), or a section, a ranked component, or a substring of either,
// without a vector subscript, of an intrinsic type. FTN$IO_ITEM gives the
// statement's module each element in array element order, as the element
// calls did. A derived type's goes a component at a time (objectItems), and a
// record's, a vector subscript's, or an expression's an element at a time.
func (l *lowerer) ncaItem(v sema.Expr) *Mem {
	// A whole array in a list may have no shape of its own (sema's name):
	// whether it has a rank is its symbol's.
	r, whole := v.(*sema.ArrayRef)
	if whole && len(r.Sym.Dims) == 0 || !whole && len(v.Shape()) == 0 ||
		v.Type().Kind == sema.Record || !sema.Described(sema.Arg{Expr: v}) {
		return nil
	}

	sub, _ := v.(*sema.Substring)

	base := v
	if sub != nil {
		base = sub.X
	}

	switch e := base.(type) {
	case *sema.ArrayRef:
		switch {
		case sub != nil:
			return l.arrayNCA(sema.NewArrayRef(e.Sym, e.Pos), sub)
		case e.Sym.Strided():
			return l.wholeNCA(sema.NewArrayRef(e.Sym, e.Pos))
		}

	case *sema.Section:
		if e.Sym.Class != sema.Constant {
			return l.sectionNCA(e, sub)
		}

	case *sema.FieldRef:
		if sub != nil || !e.WholeArray() {
			return l.sectionNCA(e, sub)
		}
	}

	return nil
}

// objectItems lowers a formatted statement's item of a derived type (sema's
// derivedio.go): an item for each of its ultimate components in turn, or for
// an array of a type, each element's in array element order. An object that
// isn't a place, a function's value or MERGE's, is put in one first, a symbol of
// lowering's own bound to it while its components are lowered.
func (l *lowerer) objectItems(v sema.Expr) {
	v = sema.Shaped(v)

	if len(v.Shape()) > 0 {
		if c, ok := v.(*sema.Constructor); ok {
			l.fillItems(c.Items, l.objectItems)
		} else {
			l.eachElement(v, l.objectItems)
		}

		return
	}

	parts, ok := sema.Components(v)
	if !ok {
		sym := sema.NewSymbol("$V", v.Type())
		l.bound[sym] = l.mem(v)

		defer delete(l.bound, sym)

		parts, _ = sema.Components(sema.NewVar(sym, v.Position()))
	}

	for _, p := range parts {
		l.ioItem(p)
	}
}

// impliedDo lowers an implied-DO list (section 7.1.2.2) as a DO loop
// whose body is its items' calls. Its temporaries stay busy until the
// statement's end: the statement isn't over when the loop is.
func (l *lowerer) impliedDo(d *sema.ImpliedDo) {
	name := "the implied DO (line " + strconv.Itoa(d.Pos.Line) + ")"

	l.countedLoop(name, d.Var, d.Start, d.Limit, d.Step, false, func(*Block) {
		l.ioItems(d.Items)
	})
}

// formatBlock is the datum holding FORMAT statement f's block, with its
// text and the cell FTNRTL keeps its parsed form in.
//
// A CHARACTER constant's format (Label 0, sema's compiledFormat) has a block
// of its own, numbered in the routine ($FK1, its cell $FKC1), shared by every
// statement with the same text.
func (l *lowerer) formatBlock(f *sema.Format) *Mem {
	key, label := "format "+strconv.Itoa(f.Label), strconv.Itoa(f.Label)
	if f.Label == 0 {
		key = "format text " + f.Encoded
	}

	d := l.datum(key, func() *Datum {
		name, cell := "$F"+label, "$FC"+label
		if f.Label == 0 {
			l.constFormats++
			n := strconv.Itoa(l.constFormats)
			name, cell = "$FK"+n, "$FKC"+n
		}

		cache := l.newCell(cell, FormatCell, Long, 4)
		text := l.text(f.Encoded).Datum

		return &Datum{Name: name, Kind: FormatDatum, Format: &FormatBlock{Label: f.Label, Text: text, Cache: cache, VFE: l.vfeRoutine(f)}}
	})

	return &Mem{Datum: d, T: Byte}
}

// namelist is NAMELIST group n's block: its name, and each entity's name
// and descriptor. A group holds no dummy argument, so if no entity is in the
// frame each descriptor is a constant, and the block is a datum in the
// routine's read-only data; otherwise it is a cell the statement fills in
// (namelistCell).
func (l *lowerer) namelist(n *sema.Namelist) *Mem {
	if slices.ContainsFunc(n.Items, func(s *sema.Symbol) bool { return s.Automatic }) {
		return l.namelistCell(n)
	}

	d := l.datum("namelist "+n.Name, func() *Datum {
		b := &NamelistBlock{Group: n.Name, Name: l.counted(n.Name)}

		for _, e := range n.Entities {
			b.Entities = append(b.Entities, NamelistEntity{Name: l.counted(e.Name), Desc: l.namelistDescriptor(e).Datum})
		}

		return &Datum{Name: "$N" + n.Name, Kind: NamelistDatum, Namelist: b}
	})

	return &Mem{Datum: d}
}

// namelistCell is NAMELIST group n's block, for a group with an entity in the
// frame (an AUTOMATIC variable, or a recursive unit's unsaved one; Decision 7 of
// docs/planning/PHASE-89 - reentrant FORTRAN.md): the block FTNRTL takes (namelistBlock's
// layout, docs/planning/PHASE-80's "The namelist block") in a cell of its own, which each
// statement that uses the group fills in whole, since an entity's descriptor
// holds an address only this activation knows. The names stay in read-only
// data; the cell holds their addresses, the count, and the descriptors'
// addresses, each descriptor written by item as for any I/O item. In a
// reentrant routine the cell is in the frame too, so another activation's
// statement can't change the block under this one's.
func (l *lowerer) namelistCell(n *sema.Namelist) *Mem {
	const header, perEntity = 8, 8

	c := l.namelists[n]
	if c == nil {
		c = l.newCell("$NL"+n.Name, NamelistCell, Long, header+perEntity*len(n.Entities))
		c.Loop = n.Name

		if l.namelists == nil {
			l.namelists = map[*sema.Namelist]*Cell{}
		}

		l.namelists[n] = c
	}

	store := func(offset int, x Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, Offset: offset, T: Long}, Src: x})
	}

	store(0, &Addr{&Mem{Datum: l.counted(n.Name), T: Byte}})
	store(4, LongConst(int32(len(n.Entities))))

	for i, e := range n.Entities {
		base := header + perEntity*i
		store(base, &Addr{&Mem{Datum: l.counted(e.Name), T: Byte}})
		store(base+4, &Addr{l.namelistDescriptor(e)})
	}

	return &Mem{Cell: c, T: Long}
}

// namelistDescriptor is a group entity's descriptor: an item's, or for an
// object of a derived type's header, a class Z descriptor whose length is how
// many entities after it are the object's (sema's derivedio.go). An entity
// under an array of a derived type is a class NCA descriptor of its elements
// across the arrays, and its header's class Z descriptor points at its shape, a
// class NCA descriptor of the object's elements without an address
// (docs/planning/PHASE-109 - Fortran 90 IO.md, subtask 6b).
func (l *lowerer) namelistDescriptor(e sema.NamelistEntity) *Mem {
	if e.Ref != nil {
		return l.item(e.Ref)
	}

	if e.Ranks == nil {
		return l.descriptor(&Descriptor{DType: DTypeZ, Class: ClassZ, Length: e.Parts})
	}

	d := &Descriptor{DType: l.dtypeOf(e.Type), Class: ClassNCA, Length: e.Type.Size}
	if e.Type.Kind == sema.Record {
		d.DType = DTypeZ
	}

	for _, r := range e.Ranks {
		d.Dims = append(d.Dims, Bound{Lower: r.Lower, Upper: r.Upper})
		d.Strides = append(d.Strides, r.Stride)
	}

	if e.Type.Kind == sema.Record {
		return l.descriptor(&Descriptor{DType: DTypeZ, Class: ClassZ, Length: e.Parts, Shape: l.descriptor(d)})
	}

	m := l.symbolMem(e.Sym)
	m.Offset += e.Offset
	m.T = Byte

	return l.describe(d, m)
}

// counted is the datum holding name as a counted string (.ASCIC): a byte
// of its length, then its characters.
func (l *lowerer) counted(name string) *Datum {
	return l.text(string(rune(len(name))) + name).Datum
}
