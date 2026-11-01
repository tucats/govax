package ir

import (
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/sema"
)

// The file statements (docs/PHASE-81 - FORTRAN files.md, Decision 4): one
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
		case k.Answer != nil:
			args = append(args, l.fileAnswer(k)...)
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

	l.ioFinish(&Call{origin: l.origin(), Routine: routine, Args: args}, c, s.Err, 0, s.IOStat)
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
