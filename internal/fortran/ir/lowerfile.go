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
const (
	mechanismValue      = 4
	mechanismDescriptor = 7
)

// fileStatement lowers OPEN or CLOSE.
func (l *lowerer) fileStatement(s *sema.FileStmt) {
	c := controlFor(s.Err, 0, s.IOStat)

	var unit Expr = LongConst(0)
	if s.Unit != nil {
		unit = l.expr(s.Unit)
	}

	args := []Arg{{Ref: l.control(c)}, {Value: unit}}

	for _, k := range s.Keys {
		switch {
		case k.Known:
			args = append(args, keyword(k.Code, mechanismValue), Arg{Value: LongConst(int32(k.Number))})
		case k.Value != nil:
			args = append(args, keyword(k.Code, mechanismValue), Arg{Value: l.expr(k.Value)})
		default:
			args = append(args, keyword(k.Code, mechanismDescriptor), Arg{Ref: l.item(k.Text)})
		}
	}

	routine := "FTN$OPEN"
	if s.AST.Base().Kind == lex.Close {
		routine = "FTN$CLOSE"
	}

	l.ioFinish(&Call{origin: l.origin(), Routine: routine, Args: args}, c, s.Err, 0, s.IOStat)
}

// keyword is a keyword's first argument: its code and mechanism.
func keyword(code, mechanism int) Arg {
	return Arg{Value: LongConst(int32(code | mechanism<<8))}
}
