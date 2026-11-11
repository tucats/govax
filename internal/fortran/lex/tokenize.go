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

		switch {
		case c == '\'' && lit[i]:
			i = l.character(&tokens, i, j)

		case isLetter(c):
			k := i
			for k < j && isNameChar(text[k]) && !lit[k] {
				k++
			}

			add(Name, i, string(text[i:k]))
			i = k

		case isDigit(c), c == '.' && i+1 < j && isDigit(text[i+1]):
			// A number, including one starting with a decimal point (.5).
			// plain says it must be digits only: a DO statement's label, or
			// a type length after *.
			plain := mode == modeLabel && len(tokens) == 0 ||
				mode == modeLengths && afterLengthStar(tokens)
			i = l.number(&tokens, i, j, plain)

		case c == '.':
			// FORTRAN's operators and logical constants that are words
			// between periods: .EQ., .AND., .TRUE., and the rest.
			if word, kind, end := l.dotted(i, j); word != "" {
				add(kind, i, "."+word+".")
				i = end
			} else {
				add(Operator, i, ".")
				i++
			}

		case c == '"':
			// Appendix A.5's octal integer constant, "17: an INTEGER
			// written in octal, not a typeless constant like '17'O.
			k := i + 1
			for k < j && text[k] >= '0' && text[k] <= '7' {
				k++
			}

			tokens = append(tokens, Token{Kind: IntegerConst, Text: string(text[i+1 : k]), Radix: 8, Offset: l.offset(i)})
			i = k

		case c == '*' || c == '/':
			if i+1 < j && text[i+1] == c && !lit[i+1] {
				add(Operator, i, string([]byte{c, c})) // ** or //
				i += 2
			} else {
				add(Operator, i, string(c))
				i++
			}

		case strings.IndexByte("+-(),=:%&'<>", c) >= 0:
			add(Operator, i, string(c))
			i++

		default:
			// Report the character and skip it, so the rest of the
			// statement still gets tokens and the later passes can look
			// for more errors.
			l.messages.Report("BADCHAR", l.src.Position(l.offset(i)), string(c))
			i++
		}
	}

	return tokens
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

// character adds the character constant whose opening apostrophe is at i,
// and returns where the text after it starts. An apostrophe doubled inside
// the constant is one apostrophe of its value. A constant followed at
// once by X or O is a hexadecimal or octal constant ('1F'X).
//
// tokens is a pointer to the caller's slice because append may have to
// move a slice to a bigger array, giving back a new slice header. Writing
// *tokens = append(*tokens, ...) puts that new header back in the
// caller's variable, so the caller sees the token.
func (l *lexer) character(tokens *[]Token, i, j int) int {
	text, lit := l.sq.text, l.sq.lit

	var value []byte

	k := i + 1

	for k < j {
		if text[k] == '\'' {
			if k+1 < j && text[k+1] == '\'' && lit[k+1] {
				value = append(value, '\'')
				k += 2

				continue
			}

			k++ // the closing apostrophe

			break
		}

		value = append(value, text[k])
		k++
	}

	tok := Token{Kind: CharConst, Text: string(value), Offset: l.offset(i)}

	// '1F'X and '17'O are VAX FORTRAN's typeless constants: bits written
	// in hexadecimal or octal, which take on the type of whatever they are
	// used with. The X or O must not begin a longer name ('A'XYZ isn't
	// one), hence the check of the character after it.
	if k < j && !lit[k] && (text[k] == 'X' || text[k] == 'O') && (k+1 == j || !isNameChar(text[k+1])) {
		tok.Kind, tok.Radix = TypelessConst, 16

		if text[k] == 'O' {
			tok.Radix = 8
		}

		k++
	}

	*tokens = append(*tokens, tok)

	return k
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
		if k < j && text[k] == '.' && !lit[k] {
			if word, _, _ := l.dotted(k, j); word == "" {
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

	*tokens = append(*tokens, Token{Kind: kind, Text: string(text[i:k]), Offset: l.offset(i)})

	return k
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
