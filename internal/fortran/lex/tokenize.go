package lex

import (
	"strconv"
	"strings"
)

// tokenMode is what the tokenizer knows of the statement it is splitting
// up, which it needs where FORTRAN's text alone is ambiguous.
type tokenMode int

const (
	// modeNone: no special knowledge.
	modeNone tokenMode = iota

	// modeLabel: the text starts with a statement label, as DO's does.
	// Squeezed, DO 10 E1 = 1, 5 is DO10E1=1,5, and without being told,
	// the tokenizer would read 10E1 as the real constant 100.0.
	modeLabel

	// modeLengths: a type declaration, where *n after the type keyword or
	// a name is a length. REAL*8 D1 squeezes to REAL*8D1, whose 8D1 would
	// otherwise be the REAL*8 constant 80.0. A * after a number is still
	// a repeat count, whose value may be any constant (X/3*1.5E2/).
	modeLengths
)

// tokenize splits the squeezed text from i to j into tokens, the
// lexer's third step, reporting a character FORTRAN doesn't use.
func (l *lexer) tokenize(i, j int, mode tokenMode) []Token {
	var tokens []Token

	// text is the squeezed statement (blanks gone, letters capitals), and
	// lit[i] says whether text[i] is inside a constant, where an = or a
	// period is just a character. See squeeze.go.
	text, lit := l.sq.text, l.sq.lit

	// add appends one token. It's a closure: a function literal that can
	// use tokens and l from the function around it. from is the token's
	// index in the squeezed text, which l.offset turns back into its place
	// in the original statement, for messages.
	add := func(kind TokenKind, from int, s string) {
		tokens = append(tokens, Token{Kind: kind, Text: s, Offset: l.offset(from)})
	}

	// One token per turn: look at the first character, which says what
	// kind of token starts here, and let that case move i past it. A
	// switch with no value after the word switch tries each case's
	// condition in order, like a chain of if ... else if.
	for i < j {
		c := text[i]

		// start and before are where this turn's token begins and how many
		// tokens there were, for free form's checks of blanks after it.
		start, before := i, len(tokens)

		switch {
		case (c == '\'' || c == '"') && lit[i]:
			// A character constant: in apostrophes, or under /F90 in
			// quotation marks (squeeze marked which quotation marks start
			// one).
			i = l.character(&tokens, i, j, "")

		case isLetter(c):
			k := i
			for k < j && isNameChar(text[k]) && !lit[k] {
				k++
			}

			switch {
			case l.quoteAt(k, j) && k == i+1 && strings.IndexByte("BOZ", c) >= 0:
				// Fortran 90's B'101', O'17', Z'1F': a constant in a base,
				// its letter first (the standard's R407).
				i = l.prefixed(&tokens, i, j)

			case l.quoteAt(k, j) && k > i+1 && text[k-1] == '_':
				// K_'text': a character constant with a kind parameter
				// named K before it (R420).
				i = l.character(&tokens, k, j, string(text[i:k-1]))
				tokens[len(tokens)-1].Offset = l.offset(start)

			default:
				add(Name, i, string(text[i:k]))
				i = k
			}

		case isDigit(c), c == '.' && i+1 < j && isDigit(text[i+1]) && !closesWord(tokens):
			// A number, including one starting with a decimal point (.5).
			// plain says it must be digits only: a DO statement's label, or
			// a type length after *.
			plain := mode == modeLabel && len(tokens) == 0 ||
				mode == modeLengths && afterLengthStar(tokens)
			i = l.number(&tokens, i, j, plain)

		case c == '.':
			// FORTRAN's operators and logical constants that are words
			// between periods: .EQ., .AND., .TRUE., and the rest; and
			// Fortran 90's defined operators, .CROSS. (definedOperator).
			switch word, kind, end := l.dotted(i, j); {
			case word != "":
				add(kind, i, "."+word+".")
				i = l.kindAfter(&tokens, end, j)

			case l.definedOperator(tokens, i, j) > i:
				end := l.definedOperator(tokens, i, j)
				add(Operator, i, string(text[i:end]))
				i = end

			default:
				add(Operator, i, ".")
				i++
			}

		case c == '"':
			// Appendix A.5's octal integer constant, "17: an INTEGER
			// written in octal, not a typeless constant like '17'O. (Under
			// /F90 the quotation mark starts a character constant instead,
			// which the first case took.)
			k := i + 1
			for k < j && text[k] >= '0' && text[k] <= '7' {
				k++
			}

			tokens = append(tokens, Token{Kind: IntegerConst, Text: string(text[i+1 : k]), Radix: 8, Offset: l.offset(i)})
			i = k

		default:
			if op := l.operator(i, j); op != "" {
				add(Operator, i, op)
				i += len(op)

				break
			}

			// Report the character and skip it, so the rest of the
			// statement still gets tokens and the later passes can look
			// for more errors.
			l.messages.Report("BADCHAR", l.src.Position(l.offset(i)), string(c))
			i++
		}

		if l.src != nil && l.src.Free && len(tokens) > before {
			l.checkBlanks(tokens[len(tokens)-1], start, i)
		}
	}

	return tokens
}

// operators are the operators of more than one character, longest first
// within each first character, so that ** is found before *. The ones on
// the second line are Fortran 90's (the standard's 3.2.3 and 3.2.5): the
// relational operators' new spellings, the array constructor's (/ and /), the
// declarations' ::, and the pointer assignment's =>. None of them means
// anything in VAX FORTRAN, so reading them as one token changes no old
// program (TestNoF90Unchanged).
var operators = []string{
	"**", "//",
	"(/", "/)", "==", "/=", "<=", ">=", "::", "=>",
}

// operator returns the operator that starts at i, before j: one of operators,
// or a single character FORTRAN uses; "" if the character is none of them.
// Neither character may be inside a constant.
func (l *lexer) operator(i, j int) string {
	text, lit := l.sq.text, l.sq.lit

	for _, op := range operators {
		if i+1 < j && text[i] == op[0] && text[i+1] == op[1] && !lit[i+1] {
			return op
		}
	}

	if strings.IndexByte("+-*/(),=:%&'<>", text[i]) >= 0 {
		return string(text[i])
	}

	return ""
}

// quoteAt reports whether a character constant starts at k (before j): an
// apostrophe or quotation mark squeeze marked as a constant's.
func (l *lexer) quoteAt(k, j int) bool {
	return k < j && l.sq.lit[k] && (l.sq.text[k] == '\'' || l.sq.text[k] == '"')
}

// prefixed adds Fortran 90's binary, octal, or hexadecimal constant whose
// letter, B, O, or Z, is at i: a TypelessConst, Prefixed, with its base. Its
// digits are checked where it is given a value, as DEC's '1F'X are.
func (l *lexer) prefixed(tokens *[]Token, i, j int) int {
	value, k := l.quoted(i+1, j)

	radix := map[byte]int{'B': 2, 'O': 8, 'Z': 16}[l.sq.text[i]]
	*tokens = append(*tokens, Token{Kind: TypelessConst, Text: value, Radix: radix, Prefixed: true, Offset: l.offset(i)})

	return k
}

// definedOperator returns where the defined operator starting with the period
// at i ends (past its closing period), or i if there is none: a word of
// letters between periods that isn't one of FORTRAN's own (the standard's
// R704 and R724: .CROSS., .INVERSE.).
//
// After a name or a ), A.B.C is VAX FORTRAN's record field reference as
// well as A .B. C (docs/planning/PHASE-99 - Fortran 90.md, "Where Fortran 90 and
// VAX FORTRAN disagree"), and the lexer can't tell which: there it leaves the
// periods and the name as separate tokens, as it always has, for the parser
// and the symbols to settle (Phase 103). Anywhere else, at an expression's
// start, after an operator, or after a constant, no field can follow, and the
// word is the operator.
func (l *lexer) definedOperator(tokens []Token, i, j int) int {
	if n := len(tokens); n > 0 && (tokens[n-1].Kind == Name || tokens[n-1].Is(")")) {
		return i
	}

	if end := l.dottedWord(i, j); end > 0 {
		return end
	}

	return i
}

// closesWord reports whether a period after tokens closes a word another
// period opened after a name or a ): in R.AT.2.0, the period before 2 ends
// .AT., which definedOperator left as a period and a name, R.AT perhaps being
// a field (a constant never follows one), so it doesn't start the number .2.
func closesWord(tokens []Token) bool {
	n := len(tokens)

	return n >= 3 && tokens[n-1].Kind == Name && tokens[n-2].Is(".") &&
		(tokens[n-3].Kind == Name || tokens[n-3].Is(")"))
}

// dottedWord returns where a word of letters between periods, the first at i,
// ends (past the second period), or 0 if there is no such word.
func (l *lexer) dottedWord(i, j int) int {
	text, lit := l.sq.text, l.sq.lit

	k := i + 1
	for k < j && isLetter(text[k]) && !lit[k] {
		k++
	}

	if k == i+1 || k >= j || text[k] != '.' || lit[k] {
		return 0
	}

	return k + 1
}

// kindAfter reads the kind parameter after the constant just added to tokens,
// if an underscore follows it at i: digits or a name (the standard's R404:
// 1.5_8, 2_K, .TRUE._1). It returns where the text after it starts.
func (l *lexer) kindAfter(tokens *[]Token, i, j int) int {
	text, lit := l.sq.text, l.sq.lit

	if i >= j || text[i] != '_' || lit[i] {
		return i
	}

	k := i + 1
	for k < j && isNameChar(text[k]) && !lit[k] {
		k++
	}

	kind := string(text[i+1 : k])
	if !validKind(kind) {
		l.messages.Report("BADKIND", l.src.Position(l.offset(i)))

		return k
	}

	(*tokens)[len(*tokens)-1].KindParam = kind

	return k
}

// validKind reports whether text can be a kind parameter: digits, or a name.
func validKind(text string) bool {
	if text == "" {
		return false
	}

	if isLetter(text[0]) {
		return true
	}

	for i := 0; i < len(text); i++ {
		if !isDigit(text[i]) {
			return false
		}
	}

	return true
}

// compoundNames are the words that are two keywords written together, which
// may have a blank between them in free form though they are read as one name
// in a statement's tokens (the standard's 3.3.1.2): IMPLICIT DOUBLE PRECISION
// (A-H), INTENT(IN OUT). The number is where the blank may be.
var compoundNames = map[string]int{"DOUBLEPRECISION": 6, "DOUBLECOMPLEX": 6, "INOUT": 2}

// checkBlanks applies free form's rules for blanks (the standard's 3.3.1) to
// tok, which runs from start to end in the squeezed text: a blank inside it is
// BLANKINNAME, and none between it and a name, keyword, constant, or label
// just before it is NEEDBLANK. squeeze recorded where blanks were (gap).
func (l *lexer) checkBlanks(tok Token, start, end int) {
	text, lit, gap := l.sq.text, l.sq.lit, l.sq.gap

	word := func(m int) bool { return isNameChar(text[m]) && !lit[m] }

	if start > 0 && word(start-1) && word(start) && !gap[start] {
		k := start - 1
		for k > 0 && word(k-1) {
			k--
		}

		l.messages.Report("NEEDBLANK", l.src.Position(l.offset(start)), string(text[k:start]), tok.Text)
	}

	allowed := -1
	if split, ok := compoundNames[tok.Text]; ok && tok.Kind == Name {
		allowed = start + split
	}

	for m := start + 1; m < end; m++ {
		if gap[m] && m != allowed {
			l.messages.Report("BLANKINNAME", l.src.Position(l.offset(m)), tokenWhat(tok))

			return
		}
	}
}

// tokenWhat names what kind of word tok is, for BLANKINNAME.
func tokenWhat(tok Token) string {
	switch tok.Kind {
	case Name:
		return "a name"
	case Keyword:
		return "a keyword"
	case Operator:
		return "an operator"
	}

	return "a constant"
}

// afterLengthStar reports whether the next number is a type length: it
// follows a * that follows the type keyword (nothing yet), a name, a
// keyword, or the parenthesis closing a CHARACTER array's bounds
// (REAL*8, X*4, FUNCTION F*8, CHARACTER A(10)*8).
func afterLengthStar(tokens []Token) bool {
	n := len(tokens)

	// The token just before must be the *; and the one before it (if
	// any) says whether the * is a length's or a multiplication's.
	if n == 0 || !tokens[n-1].Is("*") {
		return false
	}

	return n == 1 || tokens[n-2].Kind == Name || tokens[n-2].Kind == Keyword || tokens[n-2].Is(")")
}

// character adds the character constant whose opening apostrophe (or
// quotation mark) is at i, with the kind parameter kind ("" for none), and
// returns where the text after it starts. A constant followed at once by X
// or O is a hexadecimal or octal constant ('1F'X).
//
// tokens is a pointer to the caller's slice because append may have to
// move a slice to a bigger array, giving back a new slice header. Writing
// *tokens = append(*tokens, ...) puts that new header back in the
// caller's variable, so the caller sees the token.
func (l *lexer) character(tokens *[]Token, i, j int, kind string) int {
	text, lit := l.sq.text, l.sq.lit

	value, k := l.quoted(i, j)
	tok := Token{Kind: CharConst, Text: value, KindParam: kind, Offset: l.offset(i)}

	// '1F'X and '17'O are VAX FORTRAN's typeless constants: bits written
	// in hexadecimal or octal, which take on the type of whatever they are
	// used with. The X or O must not begin a longer name ('A'XYZ isn't
	// one), hence the check of the character after it.
	if kind == "" && k < j && !lit[k] && (text[k] == 'X' || text[k] == 'O') && (k+1 == j || !isNameChar(text[k+1])) {
		tok.Kind, tok.Radix = TypelessConst, 16

		if text[k] == 'O' {
			tok.Radix = 8
		}

		k++
	}

	*tokens = append(*tokens, tok)

	return k
}

// quoted reads the quoted text whose opening apostrophe or quotation mark is
// at i, returning its value and where the text after the closing one starts.
// The delimiter doubled inside the constant is one of it in the value
// ('IT''S', "SAY ""HI""").
func (l *lexer) quoted(i, j int) (string, int) {
	text, lit := l.sq.text, l.sq.lit
	delimiter := text[i]

	var value []byte

	k := i + 1

	for k < j {
		if text[k] == delimiter {
			if k+1 < j && text[k+1] == delimiter && lit[k+1] {
				value = append(value, delimiter)
				k += 2

				continue
			}

			k++ // the closing delimiter

			break
		}

		value = append(value, text[k])
		k++
	}

	return string(value), k
}

// number adds the number, or Hollerith or RADIX-50 constant, starting at i, and
// returns where the text after it starts. With plain set the number is
// digits only (a label or a type length).
//
// A real constant is digits with a decimal point, an exponent, or both
// (section 2.2.1.2). The hard case is a period after digits: in 1.5 it is
// a decimal point, but in I.EQ.1.AND.J it starts the operator .AND., so
// a period that starts a dotted operator ends the integer.
func (l *lexer) number(tokens *[]Token, i, j int, plain bool) int {
	text, lit := l.sq.text, l.sq.lit

	// k runs over the leading digits; text[i:k] is them.
	k := i
	for k < j && isDigit(text[k]) && !lit[k] {
		k++
	}

	// nH: a Hollerith constant, which squeeze marked. 5HHELLO is the five
	// characters after the H. min keeps a count too large for the text
	// from running past the end (squeeze has reported that already).
	if k > i && k < j && lit[k] && text[k] == 'H' {
		n, _ := strconv.Atoi(string(text[i:k]))
		end := min(k+1+n, j)
		*tokens = append(*tokens, Token{Kind: HollerithConst, Text: string(text[k+1 : end]), Offset: l.offset(i)})

		return end
	}

	// nR: a RADIX-50 constant, which squeeze marked too. RADIX-50 is a DEC
	// code packing three characters from a 40-character set into 16 bits
	// (40 is 50 in octal, hence the name).
	if k > i && k < j && lit[k] && text[k] == 'R' {
		n, _ := strconv.Atoi(string(text[i:k]))
		end := min(k+1+n, j)
		*tokens = append(*tokens, Token{Kind: Radix50Const, Text: strings.ToUpper(string(text[k+1 : end])), Offset: l.offset(i)})

		return end
	}

	kind := IntegerConst

	// Otherwise it's an integer, unless a decimal point or an exponent
	// makes it a real. E is REAL*4's exponent letter, D REAL*8's, and Q
	// REAL*16's: 1.5D0 is the REAL*8 value 1.5.
	if !plain {
		// A period followed by a word and a period starts an operator:
		// .AND., or a defined operator (2.CROSS.X), so it ends an integer.
		if k < j && text[k] == '.' && !lit[k] {
			if l.dottedWord(k, j) == 0 {
				kind = RealConst
				k++

				for k < j && isDigit(text[k]) {
					k++
				}
			}
		}

		if k < j && strings.IndexByte("EDQ", text[k]) >= 0 && !lit[k] {
			m := k + 1
			if m < j && (text[m] == '+' || text[m] == '-') {
				m++
			}

			// Only a letter followed by digits is an exponent: in 2EQ the
			// E starts a name. The for loop with an empty body just moves
			// k past the exponent's digits.
			if m < j && isDigit(text[m]) {
				kind = RealConst

				for k = m; k < j && isDigit(text[k]); k++ {
				}
			}
		}
	}

	// An integer before _'text' is a character constant's kind parameter
	// (1_'text'); otherwise an underscore after a number starts the number's
	// own (1.5_8).
	if !plain && kind == IntegerConst && k+1 < j && text[k] == '_' && !lit[k] && l.quoteAt(k+1, j) {
		end := l.character(tokens, k+1, j, string(text[i:k]))
		(*tokens)[len(*tokens)-1].Offset = l.offset(i)

		return end
	}

	*tokens = append(*tokens, Token{Kind: kind, Text: string(text[i:k]), Offset: l.offset(i)})

	if plain {
		return k
	}

	return l.kindAfter(tokens, k, j)
}

// dotted reads the dotted operator or logical constant whose first period
// is at i (.EQ., .TRUE.), returning its word, its token kind, and where
// the text after it starts; the word is empty if there is none there.
func (l *lexer) dotted(i, j int) (string, TokenKind, int) {
	text, lit := l.sq.text, l.sq.lit

	k := i + 1
	for k < j && isLetter(text[k]) && !lit[k] {
		k++
	}

	if k == i+1 || k >= j || text[k] != '.' || lit[k] {
		return "", 0, 0
	}

	word := string(text[i+1 : k])

	// dotted (the map in kinds.go, not this method) has every word that
	// may stand between periods. A letter run that isn't one (.X.) means
	// the first period is something else, such as a decimal point.
	kind, ok := dotted[word]
	if !ok {
		return "", 0, 0
	}

	return word, kind, k + 1
}
