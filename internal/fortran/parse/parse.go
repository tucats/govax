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
// Units nest in Fortran 90: a unit's internal procedures follow its
// CONTAINS statement, and an interface block holds interface bodies, each a
// unit with its own END (units.go). And it gathers structure declarations'
// statements into the declarations (blocks.go).
func Parse(statements []*lex.Statement, messages *diag.List) *ast.File {
	u := &unitStack{file: &ast.File{}, messages: messages, blocks: &blockStack{messages: messages}}

	// One statement per turn. Each is parsed on its own (statement), then
	// placed: in the prologue waiting for a unit, as a new unit's header,
	// or in the open unit (place).
	for _, ls := range statements {
		if ls.Kind == lex.Empty {
			if ls.Label != 0 {
				messages.Report("EMPTYSTMT", ls.Source.LabelPosition(), ls.Label)
			}

			continue
		}

		s := statement(ls, messages, u.records())

		if u.top() == nil && (ls.Kind == lex.Options || ls.Kind == lex.Directive) {
			u.prologue = append(u.prologue, s)

			continue
		}

		u.place(s, ls)
		u.last = ls
	}

	u.finish()
	u.file.Trailing = u.prologue

	return u.file
}

// isHeader reports whether statements of kind k start a program unit.
func isHeader(k lex.Kind) bool {
	switch k {
	case lex.Program, lex.Function, lex.Subroutine, lex.BlockData, lex.Module:
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
		lex.Common:      parseCommon,
		lex.Equivalence: parseEquivalence,
		lex.Data:        parseData,
		lex.Save:        parseAttr,
		lex.External:    parseAttr,
		lex.Intrinsic:   parseAttr,
		lex.Volatile:    parseAttr,
		lex.Automatic:   parseAttr,
		lex.Static:      parseAttr,

		// Fortran 90's attribute statements (Phase 100); ALLOCATABLE,
		// POINTER, PUBLIC, and PRIVATE wait for their phases, below.
		lex.Intent:   parseAttr,
		lex.Optional: parseAttr,
		lex.Target:   parseAttr,

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

		// Fortran 90's statements (f90.go): recognized, and NOTIMPL until
		// their phases parse them.
		lex.Module:            parseModule,
		lex.Allocatable:       parsePending,
		lex.Allocate:          parsePending,
		lex.Case:              parseCase,
		lex.Contains:          parseContains,
		lex.Cycle:             parseLoopExit,
		lex.Deallocate:        parsePending,
		lex.ElseWhere:         parseElseWhere,
		lex.EndInterface:      parseEndInterface,
		lex.EndSelect:         parseEndSelect,
		lex.EndType:           parsePending,
		lex.EndWhere:          parseEndWhere,
		lex.Exit:              parseLoopExit,
		lex.Interface:         parseInterface,
		lex.ModuleProcedure:   parseModuleProcedure,
		lex.Nullify:           parsePending,
		lex.PointerAssignment: parsePending,
		lex.Pointer:           parsePending,
		lex.Private:           parsePending,
		lex.Public:            parsePending,
		lex.SelectCase:        parseSelectCase,
		lex.Sequence:          parsePending,
		lex.TypeDef:           parsePending,
		lex.TypeDecl:          parsePending,
		lex.Use:               parsePending,
		lex.Where:             parseWhere,
		lex.WhereBlock:        parseWhereBlock,
	}
}

// parser is the parser's state for one statement: the statement and a
// cursor over its tokens.
type parser struct {
	messages *diag.List
	st       *lex.Statement
	tokens   []lex.Token
	i        int // the next token's index

	// records are the names declared records where the statement is, which
	// decide what A.B.C is under /F90 (fieldNext); nil outside any unit.
	records *recordScope
}

// bailout is what a parsing function panics with when it has reported an
// error, to abandon the statement: statement recovers it and returns a
// BadStmt. (A panic unwinds every parsing function between the error and
// the statement at once, which spares each of them checking for an error
// after every call; Go's own parser, go/parser, works the same way.)
type bailout struct{}

// statement parses one statement.
func statement(ls *lex.Statement, messages *diag.List, records *recordScope) (s ast.Stmt) {
	p := &parser{messages: messages, st: ls, tokens: ls.Tokens, records: records}
	base := p.base()

	if ls.Kind == lex.Unknown {
		// The lexer reported it (UNRECSTMT).
		return &ast.BadStmt{StmtBase: base}
	}

	// recover, called in a deferred function, stops a panic that is
	// unwinding the stack and returns the value it was raised with (nil if
	// there's no panic). s is a named result, so setting it here replaces
	// what the function returns. Only our own bailout is caught: any other
	// panic, such as an index out of range, is a bug, and is raised again
	// so that it isn't hidden.
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r) // a bug in the parser, not an error in the program
			}

			s = &ast.BadStmt{StmtBase: base}
		}
	}()

	// Look up the kind's parsing function in the table and call it.
	return parsers[ls.Kind](p)
}

// base is the statement's label and position, which every node made from
// it starts with.
func (p *parser) base() ast.StmtBase {
	b := ast.StmtBase{Pos: p.st.Position(p.st.Offset), Label: p.st.Label, Kind: p.st.Kind, Source: p.st.Source}

	if p.st.Construct != "" {
		b.Construct, b.ConstructPos = p.st.Construct, p.st.Position(p.st.ConstructOffset)
	}

	return b
}

// peek is the next token, or nil at the end of the statement. It returns
// a pointer so that "no token" can be nil; the pointer is into the token
// slice, so the caller mustn't change what it points at. peek doesn't
// move on: that is next's job. A parser that can look ahead without
// consuming is called a predictive (or recursive descent) parser.
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
		if t.Prefixed {
			return "a binary, octal, or hexadecimal constant"
		}

		return "an octal or hexadecimal constant"
	case lex.Radix50Const:
		return "a RADIX-50 constant"
	}

	return quote(t.Text)
}

// quote is an operator as messages show it: '('.
func quote(text string) string {
	return "'" + text + "'"
}

// article is "a" or "an", for the word that follows it.
func article(word string) string {
	if word != "" && strings.ContainsRune("AEIOUaeiou", rune(word[0])) {
		return "an"
	}

	return "a"
}
