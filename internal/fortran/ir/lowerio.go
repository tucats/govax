package ir

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// The I/O statements (Decision 12 of docs/PHASE-74 - FORTRAN plan.md, and
// docs/PHASE-80 - FORTRAN formatted IO.md). Each becomes FTNRTL calls: a
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
// ERR=, END=, and IOSTAT= the statement has, and whether its format is a
// run-time one), the unit (a number, by value; an internal file's or
// ENCODE's buffer's descriptor), and the format (a FORMAT statement's
// block, the address an ASSIGNed variable holds, a run-time format's
// descriptor, or a NAMELIST group's block); a list-directed statement has
// no format argument.
//
// FTN$IO_END's status is 0 for success, negative at an end of file, and
// positive (the FOR number) for an error. A statement with none of ERR=,
// END=, and IOSTAT= never sees a failure, since FTNRTL signals it; one
// with them gets its status, stores it in IOSTAT's variable, and goes to
// END= (or ERR= without it) when it is negative and ERR= when positive.

// The FTNRTL routines the I/O statements call.
const (
	ftnIOItem = "FTN$IO_ITEM"
	ftnIOEnd  = "FTN$IO_END"
)

// io lowers an I/O statement.
func (l *lowerer) io(s *sema.IO) {
	c := Control{}

	if s.Err != 0 {
		c.Flags |= ControlErr
	}

	if s.End != 0 {
		c.Flags |= ControlEnd
	}

	if s.IOStat != nil {
		c.Flags |= ControlIOStat
	}

	if s.RunFormat != nil {
		c.Flags |= ControlRunFormat
	}

	args := []Arg{{Ref: l.control(c)}, l.ioUnit(s)}

	if s.Form != sema.ListForm {
		args = append(args, l.ioFormat(s))
	}

	l.emit(&Call{origin: l.origin(), Routine: beginRoutine(s), Args: args})
	l.ioItems(s.Items)

	end := &Call{origin: l.origin(), Routine: ftnIOEnd}

	if c.Flags&(ControlErr|ControlEnd|ControlIOStat) == 0 {
		l.emit(end)

		return
	}

	// The status, kept in a temporary while the IOSTAT variable is set
	// and the statement's last work is done (both may use R0).
	var status Expr = &CallValue{}

	if s.IOStat != nil || len(l.after) > 0 {
		end.Result = l.temp(Long)
		status = &Load{end.Result}
	}

	l.emit(end)

	if s.IOStat != nil {
		ios := l.mem(s.IOStat)

		value := status
		if ios.T != Long {
			value = arith(Cvt, ios.T, status, nil)
		}

		l.emit(&Assign{l.origin(), ios, value})
	}

	if s.Err == 0 && s.End == 0 {
		return
	}

	next := l.newBlock()
	neg, pos := next, next

	switch {
	case s.End != 0:
		neg = l.labelBlock(s.End)
	case s.Err != 0:
		neg = l.labelBlock(s.Err)
	}

	if s.Err != 0 {
		pos = l.labelBlock(s.Err)
	}

	l.terminate(&Sign{l.origin(), status, neg, next, pos})
	l.begin(next)
}

// beginRoutine is the FTNRTL routine that begins statement s: FTN$, the
// direction, and the access and form, as VMS's FORTRAN run-time library
// names its own: S sequential or I internal, then F formatted, L
// list-directed, or N namelist (FTN$WRITE_SF, FTN$READ_IL); or FTN$ENCODE
// and FTN$DECODE.
func beginRoutine(s *sema.IO) string {
	switch {
	case s.Code && s.Read:
		return "FTN$DECODE"
	case s.Code:
		return "FTN$ENCODE"
	}

	name := "FTN$WRITE_"
	if s.Read {
		name = "FTN$READ_"
	}

	if s.Internal {
		name += "I"
	} else {
		name += "S"
	}

	return name + [...]string{sema.ListForm: "L", sema.FormattedForm: "F", sema.NamelistForm: "N"}[s.Form]
}

// ioUnit is the begin call's unit argument: a unit's number by value (a
// default unit's from sema), or an internal file's descriptor, or
// ENCODE's or DECODE's buffer's, a text of Count characters at the
// buffer's address.
func (l *lowerer) ioUnit(s *sema.IO) Arg {
	switch {
	case s.Code:
		return Arg{Ref: l.bufferDescriptor(s)}
	case s.Internal:
		return Arg{Ref: l.item(s.Unit)}
	case s.Unit == nil:
		return Arg{Value: LongConst(int32(s.Default))}
	}

	return Arg{Value: l.expr(s.Unit)}
}

// bufferDescriptor is a descriptor of ENCODE's or DECODE's record: text
// whose length is the count, at the buffer's address, whatever the
// buffer's type (appendix A.1).
func (l *lowerer) bufferDescriptor(s *sema.IO) *Mem {
	var at *Mem

	switch b := s.Unit.(type) {
	case *sema.ArrayRef:
		at = l.symbolMem(b.Sym)
	case *sema.Substring:
		at = l.chars(b).Mem
	default:
		if b.Type().Kind == sema.Character {
			at = l.chars(b).Mem
		} else {
			at = l.mem(b)
		}
	}

	d := &Descriptor{DType: DTypeT, Class: ClassS}
	c := l.descriptorCell(d)
	l.emit(&Assign{l.origin(), &Mem{Cell: c, T: Word}, arith(Cvt, Word, l.expr(s.Count), nil)})
	l.emit(&Assign{l.origin(), &Mem{Cell: c, Offset: PointerOffset, T: Long}, &Addr{at}})

	return &Mem{Cell: c, T: Long}
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

		l.emit(&Assign{l.origin(), &Mem{Cell: c, T: Word}, arith(Cvt, Word, length, nil)})
		l.emit(&Assign{l.origin(), &Mem{Cell: c, Offset: PointerOffset, T: Long}, &Addr{m}})

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

		l.emit(&Call{origin: l.origin(), Routine: ftnIOItem, Args: []Arg{{Ref: l.item(it.Value)}}})
	}
}

// impliedDo lowers an implied-DO list (section 7.1.2.2) as a DO loop
// whose body is its items' calls. Its temporaries stay busy until the
// statement's end: the statement isn't over when the loop is.
func (l *lowerer) impliedDo(d *sema.ImpliedDo) {
	name := "the implied DO (line " + strconv.Itoa(d.Pos.Line) + ")"

	l.countedLoop(name, d.Var, d.Start, d.Limit, d.Step, false, func() {
		l.ioItems(d.Items)
	})
}

// formatBlock is the datum holding FORMAT statement f's block, with its
// text and the cell FTNRTL keeps its parsed form in.
func (l *lowerer) formatBlock(f *sema.Format) *Mem {
	d := l.datum("format "+strconv.Itoa(f.Label), func() *Datum {
		label := strconv.Itoa(f.Label)
		cache := l.newCell("$FC"+label, FormatCell, Long, 4)
		text := l.text(f.Encoded).Datum

		return &Datum{Name: "$F" + label, Kind: FormatDatum, Format: &FormatBlock{Label: f.Label, Text: text, Cache: cache, VFE: l.vfeRoutine(f)}}
	})

	return &Mem{Datum: d, T: Byte}
}

// namelist is the datum holding NAMELIST group n's block: its name, and
// each entity's name and descriptor, all in the routine's read-only data
// (a group holds no dummy argument, so each descriptor is a constant).
func (l *lowerer) namelist(n *sema.Namelist) *Mem {
	d := l.datum("namelist "+n.Name, func() *Datum {
		b := &NamelistBlock{Group: n.Name, Name: l.counted(n.Name)}

		for i, s := range n.Items {
			b.Entities = append(b.Entities, NamelistEntity{Name: l.counted(s.Name), Desc: l.item(n.Refs[i]).Datum})
		}

		return &Datum{Name: "$N" + n.Name, Kind: NamelistDatum, Namelist: b}
	})

	return &Mem{Datum: d}
}

// counted is the datum holding name as a counted string (.ASCIC): a byte
// of its length, then its characters.
func (l *lowerer) counted(name string) *Datum {
	return l.text(string(rune(len(name))) + name).Datum
}
