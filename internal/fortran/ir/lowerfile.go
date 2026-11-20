package ir

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/sema"
)

// The file statements (docs/planning/PHASE-81 - FORTRAN files.md, Decision 4): one
// call each, which returns the status in R0 as FTN$IO_END does.
//
//	OPEN (UNIT=3, FILE=NAME, STATUS='OLD', RECL=N, ERR=99)
//
//	  call FTN$OPEN($IO1, 3, 1806, <descriptor of NAME>, 1039, 1, 1030, N, ...) -> $T1
//	  sign $T1 L99 B2 L99
//
// After the control block and the unit, each keyword is two arguments: a
// longword of its code (bits 0 to 7) and its mechanism (bits 8 to 15), and
// its value. The codes and the mechanisms are VMS's FORTRAN run-time
// library's, as far as FTNRTL needs them: 4 for the value itself, 7 for a
// descriptor's address.
//
// INQUIRE has no unit argument, since it may ask by file: its unit is a
// keyword, code 1. Its answers are keywords too, each followed by where to
// store it: by the place's width for a number or logical, and a descriptor's
// address for a text, which FORRTL gives mechanism 4.
const (
	mechanismWord       = 3
	mechanismValue      = 4
	mechanismLong       = 5
	mechanismDescriptor = 7
	mechanismByte       = 10
)

// fileStatement lowers OPEN, CLOSE, INQUIRE, REWIND, BACKSPACE, ENDFILE,
// UNLOCK, DELETE, or FIND, a record number following the unit, by value: FIND's,
// and DELETE's if it has one (FTN$DELETE_D; FTN$DELETE without).
func (l *lowerer) fileStatement(s *sema.FileStmt) {
	c := controlFor(s.Err, 0, s.IOStat)
	kind := s.AST.Base().Kind

	args := []Arg{{Ref: l.control(c)}}

	if kind != lex.Inquire {
		var unit Expr = LongConst(0)
		if s.Unit != nil {
			unit = l.expr(s.Unit)
		}

		args = append(args, Arg{Value: unit})
	}

	if s.Rec != nil {
		args = append(args, Arg{Value: l.expr(s.Rec)})
	}

	for _, k := range s.Keys {
		switch {
		case k.KeyList != nil:
			args = append(args, keyword(k.Code, mechanismValue), Arg{Ref: l.keyListBlock(k.KeyList)})
		case k.Answer != nil:
			args = append(args, l.fileAnswer(k)...)
		case k.Proc != nil:
			args = append(args, keyword(k.Code, mechanismValue), l.procedureArg(k.Proc, sema.ByRef))
		case k.Known:
			args = append(args, keyword(k.Code, mechanismValue), Arg{Value: LongConst(int32(k.Number))})
		case k.Value != nil:
			args = append(args, keyword(k.Code, mechanismValue), Arg{Value: l.expr(k.Value)})
		default:
			args = append(args, keyword(k.Code, mechanismDescriptor), Arg{Ref: l.item(k.Text)})
		}
	}

	routine := "FTN$OPEN"

	switch kind {
	case lex.Close:
		routine = "FTN$CLOSE"
	case lex.Inquire:
		routine = "FTN$INQUIRE"
	case lex.Rewind:
		routine = "FTN$REWIND"
	case lex.Backspace:
		routine = "FTN$BACKSPACE"
	case lex.EndFile:
		routine = "FTN$ENDFILE"
	case lex.Find:
		routine = "FTN$FIND"
	case lex.Delete:
		routine = "FTN$DELETE"
		if s.Rec != nil {
			routine = "FTN$DELETE_D"
		}
	case lex.Unlock:
		routine = "FTN$UNLOCK"
	}

	l.ioFinish(&Call{origin: l.origin(), Routine: routine, Args: args}, c, s.Err, 0, s.IOStat, nil)
}

// ioLength lowers INQUIRE (IOLENGTH=n) list (sema's IOLength): the items'
// bytes added up in a temporary, then rounded up to longwords. No call.
//
//	INQUIRE (IOLENGTH=N) X, C(1:K)        REAL X; CHARACTER*8 C(20)
//
//	  $T1 <- 0
//	  $T1 <- add.L($T1, 4)
//	  $T1 <- add.L($T1, mul.L(K, 8))       (C(1:K)'s extent, times 8)
//	  N <- div.L(add.L($T1, 3), 4)
//
// An item whose bytes aren't a count of elements of a known length (an array
// of a run-time CHARACTER length) is added an element at a time, through the
// scalarizer's loops, as an output list's section is written; an implied DO
// is a loop, as an output list's is. Without /OPTIMIZE a list of constant
// lengths is still added up when the program runs; the optimizer folds it.
func (l *lowerer) ioLength(s *sema.IOLength) {
	total := l.temp(Long)

	add := func(x Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: total, Src: arith(Add, Long, &Load{total}, x)})
	}

	l.emit(&Assign{origin: l.origin(), Dst: total, Src: LongConst(0)})

	var items func([]*sema.Item)

	items = func(list []*sema.Item) {
		for _, it := range list {
			if d := it.Loop; d != nil {
				name := "the implied DO (line " + strconv.Itoa(d.Pos.Line) + ")"

				l.countedLoop(name, d.Var, d.Start, d.Limit, d.Step, false, func(*Block) {
					items(d.Items)
				})

				continue
			}

			l.itemLength(it.Value, add)
		}
	}

	items(s.Items)

	value := arith(Div, Long, arith(Add, Long, &Load{total}, LongConst(3)), LongConst(4))
	dst := l.mem(s.Var)
	l.emit(&Assign{origin: l.origin(), Dst: dst, Src: convertTo(dst.T, value)})
}

// itemLength adds the bytes of v, an IOLENGTH list's item, with add.
func (l *lowerer) itemLength(v sema.Expr, add func(Expr)) {
	v = sema.Shaped(v) // a whole array named in the list has its shape
	t := v.Type()
	shape := v.Shape()

	if len(shape) == 0 {
		if t.Kind == sema.Character && t.Size == 0 {
			add(convertTo(Long, l.chars(v).Len))
		} else {
			add(LongConst(int32(t.Size)))
		}

		return
	}

	if t.Kind == sema.Character && t.Size == 0 {
		l.eachElement(v, func(e sema.Expr) { l.itemLength(e, add) })

		return
	}

	if n, known := shape.Size(); known {
		add(LongConst(int32(n * t.Size)))

		return
	}

	add(arith(Mul, Long, l.elementCount(v), LongConst(int32(t.Size))))
}

// A key's type in OPEN's key list block: a descriptor's type, with descending
// added. These are VMS's FORRTL's (testdata/for88).
const (
	keyTypeString     = 14  // DSC$K_DTYPE_T: a CHARACTER key
	keyTypeInteger    = 8   // DSC$K_DTYPE_L: an INTEGER key, signed, of the key's length
	keyTypeDescending = 512 // DESCENDING, not ASCENDING
)

// keyListBlock is the block OPEN's KEY= passes, a cell of its own that the
// statement fills in, since a key's bounds may be expressions (section 9.1.15).
// Its form is VMS's FORRTL's: a word no one reads, then a word counting the
// longwords after it, three per key:
//
//	OPEN (UNIT=3, ..., KEY=(1:10, N:N+1:INTEGER:DESCENDING))
//
//	  $KL1+0  <- 6 * 65536     the words: 0, and 2 keys' 6 longwords
//	  $KL1+4  <- 14, 1, 10     the first key: type, first byte, last byte
//	  $KL1+16 <- 520, N, N+1   the second
func (l *lowerer) keyListBlock(list []*sema.KeyDef) *Mem {
	const header, perKey = 4, 12

	l.keyLists++
	c := l.newCell("$KL"+strconv.Itoa(l.keyLists), KeyListCell, Long, header+perKey*len(list))

	store := func(offset int, x Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, Offset: offset, T: Long}, Src: x})
	}

	store(0, LongConst(int32(3*len(list))<<16))

	for i, k := range list {
		kind := keyTypeString
		if k.Integer {
			kind = keyTypeInteger
		}

		if k.Descending {
			kind += keyTypeDescending
		}

		base := header + perKey*i
		store(base, LongConst(int32(kind)))
		store(base+4, l.expr(k.First))
		store(base+8, l.expr(k.Last))
	}

	return &Mem{Cell: c, T: Long}
}

// fileAnswer is the two arguments of one of INQUIRE's answers.
func (l *lowerer) fileAnswer(k *sema.FileKey) []Arg {
	t := k.Answer.Type()

	if t.Kind == sema.Character {
		return []Arg{keyword(k.Code, mechanismValue), {Ref: l.item(k.Answer)}}
	}

	mechanism := mechanismLong

	switch t.Size {
	case 1:
		mechanism = mechanismByte
	case 2:
		mechanism = mechanismWord
	}

	return []Arg{keyword(k.Code, mechanism), {Ref: l.mem(k.Answer)}}
}

// defineFile lowers DEFINE FILE: a call for each file, with no specifiers to
// take a failure, so FTNRTL signals one.
//
//	DEFINE FILE 3 (1000, 48, U, NREC)
//
//	  call FTN$DEF_FILE($IO1, 3, 1000, 48, NREC)
//
// The unit and the two numbers are by value, the associated variable by
// reference, FTN$DEF_FILE_W's a word, as VMS's FORTRAN run-time library has
// two routines for the two widths.
func (l *lowerer) defineFile(s *sema.DefineFile) {
	for _, f := range s.Files {
		routine := "FTN$DEF_FILE"
		if f.Var.Type().Size == 2 {
			routine = "FTN$DEF_FILE_W"
		}

		l.emit(&Call{origin: l.origin(), Routine: routine, Args: []Arg{
			{Ref: l.control(Control{})},
			{Value: l.expr(f.Unit)},
			{Value: l.expr(f.Records)},
			{Value: l.expr(f.Length)},
			{Ref: l.mem(f.Var)},
		}})
	}
}

// keyword is a keyword's first argument: its code and mechanism.
func keyword(code, mechanism int) Arg {
	return Arg{Value: LongConst(int32(code | mechanism<<8))}
}
