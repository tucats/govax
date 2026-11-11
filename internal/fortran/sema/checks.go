package sema

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/fortran/diag"
)

// Checks is what /CHECK asks the compiled code to look out for (section 1.4.3 and
// appendix D of the language manual give the forms, the same on the FORTRAN command
// and in an OPTIONS statement, and docs/planning/PHASE-85 - FORTRAN compiler usability.md the
// rest). The zero value checks nothing, the default here: nothing to hand states VAX
// FORTRAN's, as MANUAL.md's section 14.7 says.
type Checks struct {
	// Bounds checks each array subscript against its dimension, each substring's
	// bounds against the value's length, and an adjustable array's bounds when the
	// subprogram is entered.
	Bounds bool

	// Overflow makes integer overflow a trap, which the VAX does for a routine whose
	// entry mask sets its IV bit.
	Overflow bool

	// Underflow makes floating underflow a fault, not a quiet zero, which the VAX does
	// when the PSW's FU bit is set.
	Underflow bool
}

// checkKeywords are /CHECK's keywords, each what it does to a setting.
var checkKeywords = map[string]func(*Checks){
	"ALL":         func(c *Checks) { *c = Checks{Bounds: true, Overflow: true, Underflow: true} },
	"NONE":        func(c *Checks) { *c = Checks{} },
	"BOUNDS":      func(c *Checks) { c.Bounds = true },
	"NOBOUNDS":    func(c *Checks) { c.Bounds = false },
	"OVERFLOW":    func(c *Checks) { c.Overflow = true },
	"NOOVERFLOW":  func(c *Checks) { c.Overflow = false },
	"UNDERFLOW":   func(c *Checks) { c.Underflow = true },
	"NOUNDERFLOW": func(c *Checks) { c.Underflow = false },
}

// checkKeywordList says what the keywords are, for a message.
const checkKeywordList = "ALL, NONE, [NO]BOUNDS, [NO]OVERFLOW, or [NO]UNDERFLOW"

// With is the setting with /CHECK's keywords applied in order. No keywords is /CHECK
// alone, which the manual's example calls full checking: ALL. A keyword may be cut
// short, while it stays one keyword's start.
func (c Checks) With(keywords []string) (Checks, error) {
	if len(keywords) == 0 {
		keywords = []string{"ALL"}
	}

	for _, word := range keywords {
		apply, err := checkKeyword(word)
		if err != nil {
			return c, err
		}

		apply(&c)
	}

	return c, nil
}

// checkKeyword finds the keyword word is, whole or cut short.
func checkKeyword(word string) (func(*Checks), error) {
	found, matches := diag.FindKeyword(checkKeywords, word)

	switch matches {
	case 0:
		return nil, fmt.Errorf("/CHECK has no keyword %s; give %s", strings.ToUpper(word), checkKeywordList)
	case 1:
		return found, nil
	}

	return nil, fmt.Errorf("/CHECK's keyword %s is ambiguous; give %s", strings.ToUpper(word), checkKeywordList)
}

// OpenEnded reports whether dimension i of dims has no upper bound a check can use: an
// assumed-size array's last, or the last dimension of (1) of s, a dummy argument, the way
// FORTRAN-66 wrote (*), which such programs index far past. dims is s.Dims, or a record's
// field's, s then saying nothing, as it may be nil. The manual gives no rule for either
// (section 6.1.1.2 says only that the size isn't known); this is Decision 4 of
// docs/planning/PHASE-85 - FORTRAN compiler usability.md, ours.
func OpenEnded(s *Symbol, dims []Dim, i int) bool {
	d := dims[i]
	if d.Assumed {
		return true
	}

	return s != nil && s.Dummy != 0 && len(s.Dims) > 0 && &dims[0] == &s.Dims[0] && i == len(dims)-1 &&
		d.LowerExpr == nil && d.UpperExpr == nil && d.Lower == 1 && d.Upper == 1
}

// checkConstantSubscripts warns of a constant subscript outside its dimension, in any
// compilation (Decision 9 of the phase's document, ours): the program is surely wrong,
// though it may never run the statement. A bound only known at run time says nothing.
func (a *analyzer) checkConstantSubscripts(s *Symbol, name string, dims []Dim, subs []Expr) {
	for i, sub := range subs {
		v, ok := constantInt(sub)
		if !ok || i >= len(dims) {
			continue
		}

		d := dims[i]

		switch {
		case d.LowerExpr == nil && v < d.Lower:
			a.report("SUBSCRIPTRANGE", sub.Position(), v, name, "lower", d.Lower)
		case d.UpperExpr == nil && !OpenEnded(s, dims, i) && v > d.Upper:
			a.report("SUBSCRIPTRANGE", sub.Position(), v, name, "upper", d.Upper)
		}
	}
}
