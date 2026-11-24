package ir

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/sema"
)

// Recorded stores (docs/planning/PHASE-113 - Fortran errata.md, subtask 3).
// A FORALL's statement whose tuples may read what other tuples write must have
// every value, and every target's subscripts, computed before anything is
// stored (Fortran 95's 7.5.4.4). Temporaries of the index set's shape do that
// for a scalar assignment over a rectangle of tuples (lowerforall.go's
// forallThroughTemps). Everything else (an array or a WHERE's assignment, a
// CHARACTER target of run-time length, a pointer or defined assignment, a
// target whose subscripts read its own storage, a set of tuples that isn't a
// rectangle) is done by recording the stores (sema's Forall.Recorded):
//
//  1. the statement is lowered for every tuple with each store to the
//     program's storage counted instead of done, giving the bytes a record of
//     them needs (sizing);
//  2. a buffer of that size is taken, as a run-time temporary of the
//     statement's, and the statement is lowered again with each such store
//     recorded there instead: the target's address, its length, and the
//     value, converted to the target's type, in an entry of its own;
//  3. a loop over the entries does the stores, each to its address.
//
// An entry is three longwords and the value, rounded up to a longword:
//
//	+0   the target's address
//	+4   the target's length in bytes (a CHARACTER target's characters)
//	+8   the value's length in bytes
//	+12  the value
//
// Nothing the program can see is written until the third step, so each value
// is the one 7.5.4.4 asks for, whatever the statement reads, and the stores
// may be done in any order, since a FORALL may assign an element only once
// (7.5.4.5). The walks are the statement's own lowering (an array assignment's
// scalarizer, a WHERE's masks, an elemental call's loops), interrupted where a
// store would be (assignScalar, moveRecord, call, pointerAssign, nullify): a
// store is the program's when its target's symbol isn't one of the compiler's
// (recordsStore). A pointer assignment's value is the pointer's own storage
// as the assignment would leave it (its address, a CHARACTER pointer's
// descriptor, an array pointer's descriptor), moved in the third step as bytes.
//
// A WHERE's assignments are each done wholly before the next (7.5.4.4's
// WHERE in a FORALL), so a WHERE is a pass of the three steps for each,
// lowered with the others left out (sema.WhereOnly); sema allows that only
// when the masks the passes test again don't read what the assignments write.
//
// The same is done for an elemental defined assignment whose CHARACTER value
// has a length known only when the program runs and overlaps its target
// (sema's ArrayOrder.Recorded): a temporary of the value would need its
// elements' length.

// recordHeader is the bytes of an entry's header: the target's address, its
// length, and the value's length.
const recordHeader = 12

// recorder is a recording in progress: sizing, the first step, adds each
// entry's bytes to total; otherwise each entry goes at next, and count counts
// them.
type recorder struct {
	sizing bool
	total  *Mem
	next   *Mem
	count  *Mem
	length *Cell
}

// recordedStores records the stores walk lowers, and then does them as store
// says (playRecords): the three steps.
func (l *lowerer) recordedStores(walk func(), store sema.Stmt) {
	r := &recorder{sizing: true, total: l.longCell(), next: l.longCell(), count: l.longCell(), length: l.indexCell()}

	saved := l.recording
	l.recording = r

	l.emit(&Assign{origin: l.origin(), Dst: r.total, Src: LongConst(0)})
	walk()

	buffer := l.runtimeTemp(sema.Type{Kind: sema.Integer, Size: 1}, []Expr{&Load{r.total}}, nil)
	start := &Load{&Mem{Cell: l.adjustable[buffer.Sym], Offset: PointerOffset, T: Long}}

	r.sizing = false
	l.emit(&Assign{origin: l.origin(), Dst: r.next, Src: start})
	l.emit(&Assign{origin: l.origin(), Dst: r.count, Src: LongConst(0)})
	walk()

	l.recording = saved

	l.playRecords(r, start, store)
}

// longCell is a Long cell of the statement's, kept until its end.
func (l *lowerer) longCell() *Mem {
	return &Mem{Cell: l.indexCell(), T: Long}
}

// recordsStore reports whether a store to target, made while a recording is
// in progress, is one to record: a store to the program's storage, not to a
// temporary of the compiler's (a symbol bound for the statement, or named with
// a $). A target whose symbol isn't known (through a pointer) is the program's.
func (l *lowerer) recordsStore(target sema.Expr) bool {
	if l.recording == nil {
		return false
	}

	s := sema.StoreSymbol(target)
	if s == nil {
		return true
	}

	if _, bound := l.bound[s]; bound {
		return false
	}

	return !strings.HasPrefix(s.Name, "$")
}

// recordsCall reports whether the call s is a defined assignment's to record:
// a scalar call whose first argument is the program's storage.
func (l *lowerer) recordsCall(s *sema.Call) bool {
	return l.recording != nil && s.Shape == nil && len(s.Args) == 2 && s.Args[0].Expr != nil &&
		s.Args[1].Expr != nil && l.recordsStore(s.Args[0].Expr)
}

// entryField is the longword at offset in the entry at next.
func (r *recorder) entryField(offset int) *Mem {
	return &Mem{Ptr: &Load{r.next}, Offset: offset, T: Long}
}

// rounded is the bytes an entry with a value of length bytes takes.
func rounded(length Expr) Expr {
	return arith(Add, Long, LongConst(recordHeader), arith(And, Long, arith(Add, Long, length, LongConst(3)), LongConst(-4)))
}

// recordStore records target = value, a scalar store. With own the value keeps
// its own length (a defined assignment's argument), else it is converted to the
// target's, which sema has given it but for a CHARACTER length.
func (l *lowerer) recordStore(target, value sema.Expr, own bool) {
	r := l.recording
	tt := target.Type()

	var address, tlen Expr

	if tt.Kind == sema.Character {
		t := l.chars(target)
		address, tlen = &Addr{t.Mem}, t.Len
	} else {
		address, tlen = &Addr{l.mem(target)}, LongConst(int32(tt.Size))
	}

	vt := value.Type()
	vlen := tlen

	var text *Text

	switch {
	case !own:
		vt = tt
	case vt.Kind == sema.Character:
		t := l.chars(value)
		text, vlen = &t, t.Len
	default:
		vlen = LongConst(int32(vt.Size))
	}

	if r.sizing {
		l.emit(&Assign{origin: l.origin(), Dst: r.total, Src: arith(Add, Long, &Load{r.total}, rounded(vlen))})

		return
	}

	l.emit(&Assign{origin: l.origin(), Dst: r.entryField(0), Src: address})
	l.emit(&Assign{origin: l.origin(), Dst: r.entryField(4), Src: tlen})
	l.emit(&Assign{origin: l.origin(), Dst: r.entryField(8), Src: vlen})

	slot := &Mem{Ptr: &Load{r.next}, Offset: recordHeader, T: l.typeOf(vt)}

	switch {
	case vt.Kind == sema.Character && text == nil:
		text := l.chars(value)
		l.emit(&CharMove{l.origin(), Text{slot, &Load{r.entryField(8)}}, text})
	case vt.Kind == sema.Character:
		l.emit(&CharMove{l.origin(), Text{slot, &Load{r.entryField(8)}}, *text})
	default:
		v := l.slotSymbol(vt, slot, nil)
		l.assign(&sema.Assign{Target: sema.NewVar(v, l.here()), Value: value})
		l.unbind(v)
	}

	l.advance(r)
}

// advance moves next past the entry there, and counts it.
func (l *lowerer) advance(r *recorder) {
	l.emit(&Assign{origin: l.origin(), Dst: r.next, Src: arith(Add, Long, &Load{r.next}, rounded(&Load{r.entryField(8)}))})
	l.emit(&Assign{origin: l.origin(), Dst: r.count, Src: arith(Add, Long, &Load{r.count}, LongConst(1))})
}

// recordPointer records the pointer assignment x => target (NULLIFY's for a nil
// target): the pointer's storage as the assignment would leave it.
func (l *lowerer) recordPointer(x, target sema.Expr) {
	r := l.recording

	var (
		storage *Mem
		size    int
	)

	p := sema.PointerSymbol(x)

	switch {
	case p != nil && p.ArrayPointer():
		storage = &Mem{Cell: l.adjustableOf(p), T: Long}
		size = l.ncaDescriptor(p).Size()
	case charPointer(x.Type()):
		storage = l.pointerOf(x)
		storage.Offset -= PointerOffset
		size = 8
	default:
		storage, size = l.pointerOf(x), Long.Size()
	}

	vlen := LongConst(int32(size))

	if r.sizing {
		l.emit(&Assign{origin: l.origin(), Dst: r.total, Src: arith(Add, Long, &Load{r.total}, rounded(vlen))})

		return
	}

	l.emit(&Assign{origin: l.origin(), Dst: r.entryField(0), Src: &Addr{storage}})
	l.emit(&Assign{origin: l.origin(), Dst: r.entryField(4), Src: vlen})
	l.emit(&Assign{origin: l.origin(), Dst: r.entryField(8), Src: vlen})

	slot := func(offset int) *Mem {
		return &Mem{Ptr: &Load{r.next}, Offset: recordHeader + offset, T: Long}
	}

	switch {
	case p != nil && p.ArrayPointer():
		// The descriptor described in a cell of the statement's, then moved.
		d := l.ncaDescriptor(p)
		cell := l.newCell("$Q", DescriptorCell, Long, d.Size())
		cell.Desc = d

		if target == nil {
			l.copyDescriptor(cell, l.adjustableOf(p))
			l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: cell, T: Long, Offset: PointerOffset}, Src: LongConst(0)})
		} else {
			l.associate(cell, p, target)
		}

		l.moveBytes(&Mem{Ptr: &Load{r.next}, Offset: recordHeader, T: Byte}, &Mem{Cell: cell, T: Byte}, size)

	case size == 8:
		// A CHARACTER pointer's descriptor: its length and type, then the
		// address. A passed-length one's first longword is its actual's.
		header := Expr(&Load{storage})
		if n := x.Type().Size; n > 0 {
			header = charPointerHeader(n)
		}

		address := Expr(LongConst(0))
		if target != nil {
			address = l.targetAddress(target)
		}

		l.emit(&Assign{origin: l.origin(), Dst: slot(PointerOffset), Src: address})
		l.emit(&Assign{origin: l.origin(), Dst: slot(0), Src: header})

	default:
		address := Expr(LongConst(0))
		if target != nil {
			address = l.targetAddress(target)
		}

		l.emit(&Assign{origin: l.origin(), Dst: slot(0), Src: address})
	}

	l.advance(r)
}

// slotSymbol is a symbol of the statement's of type t at the place m; a
// CHARACTER one whose length isn't constant has its length in the cell length.
func (l *lowerer) slotSymbol(t sema.Type, m *Mem, length *Cell) *sema.Symbol {
	sym := sema.NewSymbol("$V", t)
	l.bound[sym] = m

	if t.Kind == sema.Character && t.Size == 0 {
		// A length's value only marks it as one known when the program runs
		// (charLength); the cell holds it.
		sym.LengthValue = sema.IntConstAt(0, l.here())

		if l.lengths == nil {
			l.lengths = map[*sema.Symbol]*Cell{}
		}

		l.lengths[sym] = length
	}

	return sym
}

// unbind forgets the statement's symbol sym.
func (l *lowerer) unbind(sym *sema.Symbol) {
	delete(l.bound, sym)
	delete(l.lengths, sym)
}

// playRecords is the third step: each of r's entries, from start, stored as
// store says. An assignment's is its target = its value, a defined
// assignment's its subroutine called with them, a pointer assignment's the
// storage's bytes moved.
func (l *lowerer) playRecords(r *recorder, start Expr, store sema.Stmt) {
	pos := l.here()

	counter := sema.NewSymbol("$I", sema.IntegerType)
	l.bound[counter] = &Mem{Cell: l.indexCell(), T: Long}

	count := sema.NewSymbol("$I", sema.IntegerType)
	l.bound[count] = r.count

	defer l.unbind(counter)
	defer l.unbind(count)

	tlen, vlen := r.length, l.indexCell()

	l.emit(&Assign{origin: l.origin(), Dst: r.next, Src: start})

	l.countedLoop("recorded stores (line "+strconv.Itoa(pos.Line)+")", counter, sema.IntConstAt(1, pos), sema.NewVar(count, pos), sema.IntConstAt(1, pos), false, func(*Block) {
		address := &Load{r.entryField(0)}
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: tlen, T: Long}, Src: &Load{r.entryField(4)}})
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: vlen, T: Long}, Src: &Load{r.entryField(8)}})

		slot := func(t sema.Type) *Mem {
			return &Mem{Ptr: &Load{r.next}, Offset: recordHeader, T: l.typeOf(t)}
		}

		switch s := store.(type) {
		case *sema.Assign:
			t := s.Target.Type()
			target := l.slotSymbol(t, &Mem{Ptr: address, T: l.typeOf(t)}, tlen)
			value := l.slotSymbol(t, slot(t), tlen)

			l.assign(&sema.Assign{Target: sema.NewVar(target, pos), Value: sema.NewVar(value, pos)})
			l.unbind(target)
			l.unbind(value)

		case *sema.Call:
			tt, vt := s.Args[0].Expr.Type(), s.Args[1].Expr.Type()
			target := l.slotSymbol(tt, &Mem{Ptr: address, T: l.typeOf(tt)}, tlen)
			value := l.slotSymbol(vt, slot(vt), vlen)

			c := *s
			c.Shape, c.Order = nil, nil
			c.Args = append([]sema.Arg(nil), s.Args...)
			c.Args[0].Expr = sema.NewVar(target, pos)
			c.Args[1].Expr = sema.NewVar(value, pos)

			l.call(&c)
			l.unbind(target)
			l.unbind(value)

		default:
			// A pointer's storage: its bytes.
			n := &Load{&Mem{Cell: vlen, T: Long}}
			l.emit(&CharMove{l.origin(), Text{&Mem{Ptr: address, T: Byte}, n}, Text{&Mem{Ptr: &Load{r.next}, Offset: recordHeader, T: Byte}, n}})
		}

		l.emit(&Assign{origin: l.origin(), Dst: r.next, Src: arith(Add, Long, &Load{r.next}, rounded(&Load{&Mem{Cell: vlen, T: Long}}))})
	})
}
