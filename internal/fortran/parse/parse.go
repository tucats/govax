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
// file's end interrupts has no END: NOEND. OPTIONS statements and CDEC$
// directives between units begin none: they wait for the next unit, as
// its Prologue (or the file's Trailing statements, after the last).
//
// And it gathers structure declarations' statements into the
// declarations (blocks.go), the one place statements nest.
func Parse(statements []*lex.Statement, messages *diag.List) *ast.File {
	file := &ast.File{}

	var (
		unit     *ast.Unit      // the unit being read; nil between units
		last     *lex.Statement // the unit's last statement so far, for NOEND
		prologue []ast.Stmt     // OPTIONS and directives waiting for a unit
		blocks   = &blockStack{messages: messages}
	)

	for _, ls := range statements {
		if ls.Kind == lex.Empty {
			if ls.Label != 0 {
				messages.Report("EMPTYSTMT", ls.Source.LabelPosition(), ls.Label)
			}

			continue
		}

		s := statement(ls, messages)

		if unit == nil && (ls.Kind == lex.Options || ls.Kind == lex.Directive) {
			prologue = append(prologue, s)

			continue
		}

		// A unit's END, or the next unit's header, ends any structure
		// declaration left open; otherwise a statement inside one goes
		// into it.
		if ls.Kind == lex.End || isHeader(ls.Kind) {
			blocks.closeAll()
		} else if blocks.take(s) {
			last = ls

			continue
		}

		if isHeader(ls.Kind) {
			if unit != nil {
				noEnd(unit, last, messages)
			}

			unit = &ast.Unit{Prologue: prologue, Header: s}
			file.Units = append(file.Units, unit)
			prologue = nil
		} else {
			if unit == nil {
				unit = &ast.Unit{Prologue: prologue}
				file.Units = append(file.Units, unit)
				prologue = nil
			}

			unit.Body = append(unit.Body, blocks.outside(s))
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
		blocks.closeAll()
		noEnd(unit, last, messages)
	}

	file.Trailing = prologue

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

// parsers is the function that parses each kind of statement: every kind
// the lexer knows, so every statement of VAX FORTRAN, has one. (It is
// filled in by init, not in its declaration, because parseLogicalIf
// parses its own statement through it: Go won't initialize a variable
// whose value refers back to the variable.)
var parsers map[lex.Kind]func(*parser) ast.Stmt

func init() {
	parsers = map[lex.Kind]func(*parser) ast.Stmt{
		// Headers (decl.go).
		lex.Program:    parseProgram,
		lex.Function:   parseFunction,
		lex.Subroutine: parseSubroutine,
		lex.BlockData:  parseBlockData,
		lex.Entry:      parseEntry,

		// Declarations (decl.go).
		lex.Integer:         parseTypeStmt,
		lex.Real:            parseTypeStmt,
		lex.Logical:         parseTypeStmt,
		lex.Byte:            parseTypeStmt,
		lex.DoublePrecision: parseTypeStmt,
		lex.Complex:         parseTypeStmt,
		lex.DoubleComplex:   parseTypeStmt,
		lex.Character:       parseTypeStmt,
		lex.Dimension:       parseDimension,
		lex.Virtual:         parseDimension,
		lex.Implicit:        parseImplicit,
		lex.Parameter:       parseParameter,

		// The other specification statements (spec.go).
		lex.Common:       parseCommon,
		lex.Equivalence:  parseEquivalence,
		lex.Data:         parseData,
		lex.Save:         parseAttr,
		lex.External:     parseAttr,
		lex.Intrinsic:    parseAttr,
		lex.Volatile:     parseAttr,
		lex.Automatic:    parseAttr,
		lex.Static:       parseAttr,
		lex.Namelist:     parseNamelist,
		lex.Record:       parseRecord,
		lex.Structure:    parseStructure,
		lex.Union:        parseUnion,
		lex.Map:          parseMap,
		lex.EndStructure: parseEndBlock,
		lex.EndUnion:     parseEndBlock,
		lex.EndMap:       parseEndBlock,
		lex.Dictionary:   parseDictionary,

		// Compilation control (spec.go).
		lex.Options:   parseOptions,
		lex.Directive: parseDirective,
		lex.Format:    parseFormat,
		lex.Include:   parseInclude,

		// Assignment and control (stmt.go).
		lex.Assignment:   parseAssignment,
		lex.Assign:       parseAssignLabel,
		lex.Call:         parseCall,
		lex.GoTo:         parseGoTo,
		lex.LogicalIf:    parseLogicalIf,
		lex.ArithmeticIf: parseArithIf,
		lex.IfThen:       parseIfThen,
		lex.ElseIf:       parseElseIf,
		lex.Else:         parseElse,
		lex.EndIf:        parseEndIf,
		lex.Do:           parseDo,
		lex.DoWhile:      parseDoWhile,
		lex.EndDo:        parseEndDo,
		lex.Continue:     parseContinue,
		lex.Stop:         parseStop,
		lex.Pause:        parseStop,
		lex.Return:       parseReturn,
		lex.End:          parseEnd,

		// Input and output (io.go).
		lex.Print:      parsePrint,
		lex.Type:       parsePrint,
		lex.Accept:     parsePrint,
		lex.Read:       parseRead,
		lex.Write:      parseTransfer,
		lex.Rewrite:    parseTransfer,
		lex.Encode:     parseCode,
		lex.Decode:     parseCode,
		lex.Open:       parseAuxIO,
		lex.Close:      parseAuxIO,
		lex.Inquire:    parseAuxIO,
		lex.Rewind:     parseAuxIO,
		lex.Backspace:  parseAuxIO,
		lex.EndFile:    parseAuxIO,
		lex.Unlock:     parseAuxIO,
		lex.Delete:     parseAuxIO,
		lex.Find:       parseAuxIO,
		lex.DefineFile: parseDefineFile,
	}
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
// BadStmt. (A panic unwinds every parsing function between the error and
// the statement at once, which spares each of them checking for an error
// after every call; Go's own parser, go/parser, works the same way.)
type bailout struct{}

// statement parses one statement.
func statement(ls *lex.Statement, messages *diag.List) (s ast.Stmt) {
	p := &parser{messages: messages, st: ls, tokens: ls.Tokens}
	base := p.base()

	if ls.Kind == lex.Unknown {
		// The lexer reported it (UNRECSTMT).
		return &ast.BadStmt{StmtBase: base}
	}

	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r) // a bug in the parser, not an error in the program
			}

			s = &ast.BadStmt{StmtBase: base}
		}
	}()

	return parsers[ls.Kind](p)
}

// base is the statement's label and position, which every node made from
// it starts with.
func (p *parser) base() ast.StmtBase {
	return ast.StmtBase{Pos: p.st.Position(p.st.Offset), Label: p.st.Label, Kind: p.st.Kind, Source: p.st.Source}
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

// atName reports whether the next token is the name text.
func (p *parser) atName(text string) bool {
	return isName(p.peek(), text)
}

// isName reports whether t is the name text.
func isName(t *lex.Token, text string) bool {
	return t != nil && t.Kind == lex.Name && t.Text == text
}

// atKeyword reports whether the next tokens are a keyword specifier's
// keyword and its =: a name followed by =, as UNIT= or ERR=.
func (p *parser) atKeyword() bool {
	t, eq := p.peek(), p.peekAt(1)

	return t != nil && t.Kind == lex.Name && eq != nil && eq.Is("=")
}

// fail reports the message ident at pos, and abandons the statement.
func (p *parser) fail(ident string, pos diag.Position, args ...any) {
	p.messages.Report(ident, pos, args...)
	panic(bailout{})
}

// labelList reads a parenthesized list of statement labels, (10, 20).
func (p *parser) labelList() []ast.LabelRef {
	p.expect("(")

	var list []ast.LabelRef

	for {
		list = append(list, p.labelRef())

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")

	return list
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
