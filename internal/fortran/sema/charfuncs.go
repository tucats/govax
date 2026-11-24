package sema

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
)

// Fortran 90's character functions (the standard's 13.13: ADJUSTL, ADJUSTR,
// INDEX with BACK, LEN_TRIM, REPEAT, SCAN, TRIM, VERIFY; docs/planning/
// PHASE-108 - Fortran 90 intrinsic procedures.md, finding 3). Each is compiled
// the cheapest way its arguments allow:
//
//   - of constants, it is folded, here and in constant expressions alike
//     (constCharFuncs), so that a PARAMETER or an initial value may use it;
//   - LEN_TRIM is a loop of a few instructions in line, backward from the end
//     (the VAX has no instruction that skips characters from the right);
//   - TRIM needs no storage of its own: its value is its argument's own text,
//     as long as LEN_TRIM says, a CHARACTER value of a length known only when
//     the program runs, which every use of a value already takes;
//   - SCAN and VERIFY of a SET that is one constant character, and a BACK that
//     is constant, are the same loop in line (SCAN forward is INDEX, MATCHC);
//     any other is a call of FTNRTL's FTN$SCAN or FTN$VERIFY;
//   - INDEX with a BACK other than .FALSE. calls FTNRTL's FTN$INDEX_BACK;
//   - ADJUSTL, ADJUSTR, and REPEAT make new text, in a temporary of the
//     statement's (or a dynamic string when its length isn't known when
//     compiling), by FTNRTL's FTN$ADJUSTL, FTN$ADJUSTR, and FTN$REPEAT.
//
// The elemental ones (all but TRIM and REPEAT) take arrays, a scalar argument
// standing for each element, as any elemental function does.

// characterFunctions are the table's sixth column for these functions.
var characterFunctions = map[string]bool{
	"LENTRIM": true, "TRIM": true, "ADJUSTL": true, "ADJUSTR": true,
	"REPEAT": true, "SCAN": true, "VERIFY": true,
}

// characterFunction is a reference to one of the character functions whose
// row's how is how, or to INDEX with a BACK, of args, the arguments in the
// dummies' order (a BACK left out is nil), with shape the shape of an
// elemental reference's arrays.
func (a *analyzer) characterFunction(e *ast.Apply, how string, args []Expr, shape Shape) Expr {
	// STRING, and SET or SUBSTRING, are CHARACTER.
	for i, x := range args {
		if i < 2 && !(how == "REPEAT" && i == 1) && x.Type().Kind != Character {
			a.report("BADARG", x.Position(), e.Name, "CHARACTER", x.Type().String())

			return a.bad(e.Pos)
		}
	}

	if (how == "TRIM" || how == "REPEAT") && shape != nil {
		// Transformational: a scalar STRING (13.13.84, 13.13.111).
		a.report("BADARG", e.Pos, e.Name, "a scalar", "an array")

		return a.bad(e.Pos)
	}

	ref := func(t Type, op IntrinsicOp, routine string, args ...Expr) Expr {
		return &IntrinsicRef{exprBase{T: t, Pos: e.Pos, S: shape}, e.Name, op, args, routine}
	}

	s := args[0]
	sc, constant := s.(*Const)

	switch how {
	case "LENTRIM":
		if constant {
			return IntConstAt(lenTrim(sc.Text), e.Pos)
		}

		return ref(IntegerType, LenTrimOp, "", s)

	case "TRIM":
		if constant {
			return charConst(sc.Text[:lenTrim(sc.Text)], e.Pos)
		}

		return ref(Type{Kind: Character}, TrimOp, "", s)

	case "ADJUSTL", "ADJUSTR":
		if constant {
			if how == "ADJUSTL" {
				return charConst(adjustLeft(sc.Text), e.Pos)
			}

			return charConst(adjustRight(sc.Text), e.Pos)
		}

		op := AdjustLOp
		if how == "ADJUSTR" {
			op = AdjustROp
		}

		return ref(s.Type(), op, "FTN$"+how, s)

	case "REPEAT":
		return a.repeat(e, s, args[1])
	}

	// SCAN, VERIFY, or INDEX: a BACK, if given, is LOGICAL, made the default
	// LOGICAL for FTNRTL; left out, it is .FALSE.. An array BACK is an
	// elemental argument like the others, its element taken with theirs.
	back := Expr(logicalConst(false, e.Pos))
	if len(args) == 3 && args[2] != nil {
		switch b := args[2]; {
		case b.Type().Kind != Logical:
			a.report("BADARG", b.Position(), e.Name, "a LOGICAL BACK", b.Type().String())

			return a.bad(e.Pos)
		default:
			back = a.convert(b, LogicalType)
		}
	}

	set := args[1]
	bc, backConstant := back.(*Const)
	setc, setConstant := set.(*Const)

	if constant && setConstant && backConstant {
		switch how {
		case "SCAN":
			return IntConstAt(scanText(sc.Text, setc.Text, bc.Truth()), e.Pos)
		case "VERIFY":
			return IntConstAt(verifyText(sc.Text, setc.Text, bc.Truth()), e.Pos)
		}

		return IntConstAt(indexText(sc.Text, setc.Text, bc.Truth()), e.Pos)
	}

	switch {
	case how == "INDEX" && backConstant && !bc.Truth():
		// Searching forward, as INDEX always has (MATCHC).
		return ref(IntegerType, IndexOp, "", s, set)

	case how == "INDEX":
		return ref(IntegerType, CallOp, "FTN$INDEX_BACK", s, set, back)

	case how == "SCAN" && setConstant && len(setc.Text) == 1 && backConstant && !bc.Truth():
		// The first of one character is INDEX's search for it.
		return ref(IntegerType, IndexOp, "", s, set)

	case setConstant && len(setc.Text) == 1 && backConstant:
		op := ScanOp
		if how == "VERIFY" {
			op = VerifyOp
		}

		return ref(IntegerType, op, "", s, set, back)
	}

	return ref(IntegerType, CallOp, "FTN$"+how, s, set, back)
}

// repeat is REPEAT(STRING, NCOPIES) (13.13.88): NCOPIES copies of STRING, one
// after another. NCOPIES is an INTEGER scalar, of 0 or more; the value's
// length, STRING's times NCOPIES, is known when compiling if both are, and
// must fit a descriptor's word. FTNRTL's FTN$REPEAT checks both when the
// program runs (BADREPEAT, FOR number 305).
func (a *analyzer) repeat(e *ast.Apply, s, n Expr) Expr {
	if n.Type().Kind != Integer || rankOf(n) > 0 {
		a.report("BADARG", n.Position(), e.Name, "an INTEGER scalar NCOPIES", n.Type().String())

		return a.bad(e.Pos)
	}

	n = a.convert(n, IntegerType)

	count, known := constantInt(n)
	if known && count < 0 {
		a.report("BADARG", n.Position(), e.Name, "an NCOPIES of 0 or more", strconv.Itoa(count))

		return a.bad(e.Pos)
	}

	length := 0

	switch sc, ok := s.(*Const); {
	case ok && known:
		if len(sc.Text)*count > maxCharLength {
			a.report("BADARG", e.Pos, e.Name, "a value of 65535 characters or fewer", "a longer one")

			return a.bad(e.Pos)
		}

		return charConst(strings.Repeat(sc.Text, count), e.Pos)

	case known && s.Type().Size > 0:
		length = s.Type().Size * count

		if length > maxCharLength {
			a.report("BADARG", e.Pos, e.Name, "a value of 65535 characters or fewer", "a longer one")

			return a.bad(e.Pos)
		}
	}

	return &IntrinsicRef{exprBase{T: Type{Kind: Character, Size: length}, Pos: e.Pos}, e.Name, RepeatOp, []Expr{s, n}, "FTN$REPEAT"}
}

// The functions' values, for folding constants. A blank is the only character
// LEN_TRIM, TRIM, ADJUSTL, and ADJUSTR pass over; positions count from 1, and
// 0 is none.

// lenTrim is LEN_TRIM(s).
func lenTrim(s string) int {
	return len(strings.TrimRight(s, " "))
}

// adjustLeft is ADJUSTL(s): its leading blanks moved to its end.
func adjustLeft(s string) string {
	t := strings.TrimLeft(s, " ")

	return t + strings.Repeat(" ", len(s)-len(t))
}

// adjustRight is ADJUSTR(s): its trailing blanks moved to its start.
func adjustRight(s string) string {
	t := strings.TrimRight(s, " ")

	return strings.Repeat(" ", len(s)-len(t)) + t
}

// scanText is SCAN(s, set, back): the first (or with back the last) position
// of a character of set in s.
func scanText(s, set string, back bool) int {
	return findByte(s, back, func(c byte) bool { return strings.IndexByte(set, c) >= 0 })
}

// verifyText is VERIFY(s, set, back): the first (or last) position of a
// character not in set.
func verifyText(s, set string, back bool) int {
	return findByte(s, back, func(c byte) bool { return strings.IndexByte(set, c) < 0 })
}

// findByte is the first (or with back the last) position of a byte of s that
// match takes, or 0. The bytes are compared as they are: a character past 127
// isn't the start of anything longer.
func findByte(s string, back bool, match func(byte) bool) int {
	if back {
		for i := len(s) - 1; i >= 0; i-- {
			if match(s[i]) {
				return i + 1
			}
		}

		return 0
	}

	for i := range len(s) {
		if match(s[i]) {
			return i + 1
		}
	}

	return 0
}

// indexText is INDEX(s, t, back): the first (or last) position where t
// starts in s. An empty t is at 1, or at LEN(s)+1 from the back.
func indexText(s, t string, back bool) int {
	if back {
		return strings.LastIndex(s, t) + 1
	}

	return strings.Index(s, t) + 1
}
