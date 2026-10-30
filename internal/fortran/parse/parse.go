package parse

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Parse builds the syntax tree of the statements the lexer lexed,
// reporting each statement's errors to messages and going on with the
// next statement, so that one compilation reports every syntax error.
//
// It also divides the statements into program units: a PROGRAM,
// FUNCTION, SUBROUTINE, or BLOCK DATA statement starts one, as does any
// other statement when no unit is open (a main program without a PROGRAM
// statement), and END ends one. A unit the next unit's header or the
// file's end interrupts has no END: NOEND.
func Parse(statements []*lex.Statement, messages *diag.List) *ast.File {
	file := &ast.File{}

	var (
		unit *ast.Unit      // the unit being read; nil between units
		last *lex.Statement // the unit's last statement so far, for NOEND
	)

	for _, ls := range statements {
		switch {
		case ls.Kind == lex.Directive, ls.Kind == lex.Options:
			// These come before a unit, or between statements, and say
			// how to compile; none is implemented yet, and none belongs
			// to a unit's statements.
			messages.Report("NOTIMPL", ls.Position(ls.Offset), what(ls.Kind))

			continue

		case ls.Kind == lex.Empty:
			if ls.Label != 0 {
				messages.Report("EMPTYSTMT", ls.Source.LabelPosition(), ls.Label)
			}

			continue
		}

		s := statement(ls, messages)

		if isHeader(ls.Kind) {
			if unit != nil {
				noEnd(unit, last, messages)
			}

			unit = &ast.Unit{Header: s}
			file.Units = append(file.Units, unit)
		} else {
			if unit == nil {
				unit = &ast.Unit{}
				file.Units = append(file.Units, unit)
			}

			unit.Body = append(unit.Body, s)
		}

		last = ls

		// The lexer's kind, not the parser's node, decides that a unit
		// has ended: END with an error in it still ends its unit, as it
		// did for the lexer (which looks for typed FUNCTION statements
		// only at a unit's start).
		if ls.Kind == lex.End {
			unit = nil
		}
	}

	if unit != nil {
		noEnd(unit, last, messages)
	}

	return file
}

// isHeader reports whether statements of kind k start a program unit.
func isHeader(k lex.Kind) bool {
	switch k {
	case lex.Program, lex.Function, lex.Subroutine, lex.BlockData:
		return true
	}

	return false
}

// noEnd reports that unit, whose last statement is last, has no END.
func noEnd(unit *ast.Unit, last *lex.Statement, messages *diag.List) {
	what := unit.Kind().String()

	switch {
	case unit.Name() != "" && unit.Kind() == ast.MainProgram:
		what = "the main program " + unit.Name()
	case unit.Name() != "":
		what += " " + unit.Name()
	case unit.Kind() == ast.MainProgram:
		what = "the main program"
	}

	messages.Report("NOEND", last.Position(len(last.Source.Text)), what)
}

// parsers is the function that parses each kind of statement the
// compiler can compile so far; a kind not here is NOTIMPL. (It is filled
// in by init, not in its declaration, because parseLogicalIf parses its
// own statement through it: Go won't initialize a variable whose value
// refers back to the variable.)
var parsers map[lex.Kind]func(*parser) ast.Stmt

func init() {
	parsers = map[lex.Kind]func(*parser) ast.Stmt{
		lex.Program:    parseProgram,
		lex.Function:   parseFunction,
		lex.Subroutine: parseSubroutine,
		lex.Integer:    parseTypeStmt,
		lex.Real:       parseTypeStmt,
		lex.Logical:    parseTypeStmt,
		lex.Dimension:  parseDimension,
		lex.Assignment: parseAssignment,
		lex.Call:       parseCall,
		lex.GoTo:       parseGoTo,
		lex.LogicalIf:  parseLogicalIf,
		lex.IfThen:     parseIfThen,
		lex.ElseIf:     parseElseIf,
		lex.Else:       parseElse,
		lex.EndIf:      parseEndIf,
		lex.Do:         parseDo,
		lex.Continue:   parseContinue,
		lex.Stop:       parseStop,
		lex.Return:     parseReturn,
		lex.End:        parseEnd,
		lex.Print:      parsePrint,
		lex.Write:      parseWrite,
	}
}

// what is how a NOTIMPL message names a kind of statement.
func what(k lex.Kind) string {
	if k == lex.Directive {
		return "a CDEC$ directive"
	}

	return "the " + k.String() + " statement"
}

// parser is the parser's state for one statement: the statement and a
// cursor over its tokens.
type parser struct {
	messages *diag.List
	st       *lex.Statement
	tokens   []lex.Token
	i        int // the next token's index
}

// bailout is what a parsing function panics with when it has reported an
// error, to abandon the statement: statement recovers it and returns a
// BadStmt, or a NotImplStmt if what is set. (A panic unwinds every
// parsing function between the error and the statement at once, which
// spares each of them checking for an error after every call; Go's own
// parser, go/parser, works the same way.)
type bailout struct {
	what string // the NOTIMPL message's subject, or "" for an error
}

// statement parses one statement.
func statement(ls *lex.Statement, messages *diag.List) (s ast.Stmt) {
	p := &parser{messages: messages, st: ls, tokens: ls.Tokens}
	base := p.base()

	if ls.Kind == lex.Unknown {
		// The lexer reported it (UNRECSTMT).
		return &ast.BadStmt{StmtBase: base, Kind: ls.Kind}
	}

	parse, ok := parsers[ls.Kind]
	if !ok {
		messages.Report("NOTIMPL", base.Pos, what(ls.Kind))

		return &ast.NotImplStmt{StmtBase: base, What: what(ls.Kind), Kind: ls.Kind}
	}

	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(bailout)
			if !ok {
				panic(r) // a bug in the parser, not an error in the program
			}

			if b.what != "" {
				s = &ast.NotImplStmt{StmtBase: base, What: b.what, Kind: ls.Kind}
			} else {
				s = &ast.BadStmt{StmtBase: base, Kind: ls.Kind}
			}
		}
	}()

	return parse(p)
}

// base is the statement's label and position, which every node made from
// it starts with.
func (p *parser) base() ast.StmtBase {
	return ast.StmtBase{Pos: p.st.Position(p.st.Offset), Label: p.st.Label, Source: p.st.Source}
}

// peek is the next token, or nil at the end of the statement.
func (p *parser) peek() *lex.Token {
	if p.i < len(p.tokens) {
		return &p.tokens[p.i]
	}

	return nil
}

// peekAt is the token n after the next one, or nil past the end.
func (p *parser) peekAt(n int) *lex.Token {
	if p.i+n < len(p.tokens) {
		return &p.tokens[p.i+n]
	}

	return nil
}

// atEnd reports whether every token has been read.
func (p *parser) atEnd() bool {
	return p.i >= len(p.tokens)
}

// next reads the next token; there must be one.
func (p *parser) next() lex.Token {
	t := p.tokens[p.i]
	p.i++

	return t
}

// at reports whether the next token is the operator or keyword text.
func (p *parser) at(text string) bool {
	t := p.peek()

	return t != nil && t.Is(text)
}

// atKind reports whether the next token is of kind k.
func (p *parser) atKind(k lex.TokenKind) bool {
	t := p.peek()

	return t != nil && t.Kind == k
}

// accept reads the next token if it is the operator or keyword text,
// and reports whether it was.
func (p *parser) accept(text string) bool {
	if p.at(text) {
		p.i++

		return true
	}

	return false
}

// expect reads the operator or keyword text, which must be next.
func (p *parser) expect(text string) lex.Token {
	if !p.at(text) {
		p.syntax(quote(text))
	}

	return p.next()
}

// name reads a name, which must be next; expected says what it is, for
// the message if it isn't there.
func (p *parser) name(expected string) *ast.Name {
	if !p.atKind(lex.Name) {
		p.syntax(expected)
	}

	t := p.next()

	return &ast.Name{ExprBase: ast.ExprBase{Pos: p.pos(t)}, Name: t.Text}
}

// end checks that every token has been read.
func (p *parser) end() {
	if t := p.peek(); t != nil {
		p.messages.Report("EXTRATEXT", p.pos(*t), describe(t))
		panic(bailout{})
	}
}

// pos is where token t is.
func (p *parser) pos(t lex.Token) diag.Position {
	return p.st.Position(t.Offset)
}

// here is where the next token is, or the end of the statement.
func (p *parser) here() diag.Position {
	if t := p.peek(); t != nil {
		return p.pos(*t)
	}

	return p.st.Position(len(p.st.Source.Text))
}

// syntax reports that the parser expected something else where the next
// token is, and abandons the statement.
func (p *parser) syntax(expected string) {
	p.messages.Report("SYNTAX", p.here(), expected, describe(p.peek()))
	panic(bailout{})
}

// notImpl reports that what, at pos, is not implemented yet, and
// abandons the statement, which becomes a NotImplStmt.
func (p *parser) notImpl(what string, pos diag.Position) {
	p.messages.Report("NOTIMPL", pos, what)
	panic(bailout{what: what})
}

// labelRef reads a reference to a statement label: an integer constant
// of 1 to 5 digits, not all zeros (section 1.1.4).
func (p *parser) labelRef() ast.LabelRef {
	if !p.atKind(lex.IntegerConst) {
		p.syntax("a statement label")
	}

	t := p.next()
	n, err := strconv.Atoi(t.Text)

	if err != nil || len(t.Text) > 5 || n == 0 {
		p.messages.Report("BADLABELREF", p.pos(t), t.Text)
		panic(bailout{})
	}

	return ast.LabelRef{Label: n, Pos: p.pos(t)}
}

// describe says what a token is, for a message: "the name X", "','";
// nil is the end of the statement.
func describe(t *lex.Token) string {
	if t == nil {
		return "the end of the statement"
	}

	switch t.Kind {
	case lex.Name:
		return "the name " + t.Text
	case lex.Keyword:
		return "the keyword " + t.Text
	case lex.IntegerConst, lex.RealConst:
		return "the number " + t.Text
	case lex.CharConst:
		return "a character constant"
	case lex.HollerithConst:
		return "a Hollerith constant"
	case lex.TypelessConst:
		return "an octal or hexadecimal constant"
	}

	return quote(t.Text)
}

// quote is an operator as messages show it: '('.
func quote(text string) string {
	return "'" + text + "'"
}

// article is "a" or "an", for the word that follows it.
func article(word string) string {
	if word != "" && strings.ContainsRune("AEIOU", rune(word[0])) {
		return "an"
	}

	return "a"
}
