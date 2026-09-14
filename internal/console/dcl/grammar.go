package dcl

// ValueType identifies the kind of value a parameter or qualifier accepts,
// matching dclrtl.h's DCL_ANY/DCL_NAME/DCL_STRING/DCL_INTEGER/DCL_KEYWORD
// datatype codes plus dcldef.h's DCL_KEYWORD__REST_OF_LINE — restricted to
// the subset testdata/dcl/evax.dcl actually uses (see doc.go).
type ValueType int

// Value type constants. TypeSwitch marks a qualifier that takes no value at
// all (a plain on/off switch or a /SYNTAX= redirector), which has no
// corresponding DCL_* datatype code in the C source since such qualifiers
// never call DCLdefault/set a type.
const (
	TypeSwitch ValueType = iota
	TypeAny
	TypeName
	TypeString
	TypeInteger
	TypeRestOfLine
	TypeKeyword
)

// Keyword is one named value of a Type, matching a grammar "keyword"
// statement. A keyword whose grammar statement carried /syntax=X redirects
// parsing into that named Entry when matched, exactly as a Qualifier's own
// Syntax field does (see DCLkeysearch's shared redirect behavior, ported in
// parse.go).
type Keyword struct {
	Name   string
	ID     int64
	Syntax string // target Entry name, or "" for no redirect
}

// Type is a named list of keywords, matching a grammar "type" statement
// (e.g. testdata/dcl/evax.dcl's show_types, dev_class).
type Type struct {
	Name     string
	Keywords []*Keyword
}

func (t *Type) lookup(name string) (kw *Keyword, negated bool, err error) {
	return matchKeyword(t.Keywords, name)
}

// Value holds a literal default (or matched) value for a Parameter or
// Qualifier, matching struct DCL_VALUE.
type Value struct {
	IsString bool
	Str      string
	Int      int64
}

// Parameter is one positional parameter of an Entry, matching a grammar
// "parameter" statement.
type Parameter struct {
	Name     string
	ID       int64
	Type     ValueType
	TypeName string // when Type == TypeKeyword, the referenced Type's name
	Prompt   string
	Default  *Value

	typeRef *Type // resolved by validate()
}

// required reports whether this parameter must be supplied, matching
// DCLprompt's DCL_REQ side effect: a parameter becomes required exactly when
// its grammar statement carries a /prompt= clause (see dclrtl.c's
// DCLprompt).
func (p *Parameter) required() bool { return p.Prompt != "" }

// Qualifier is one /-prefixed switch of an Entry, matching a grammar
// "qualifier" statement.
type Qualifier struct {
	Name     string
	ID       int64
	Type     ValueType // TypeSwitch if the qualifier takes no value
	TypeName string    // when Type == TypeKeyword, the referenced Type's name
	Syntax   string    // target Entry name to redirect to when matched, or ""
	Alias    string    // name of another qualifier in the same Entry this stands in for
	NoNegate bool
	Default  *Value

	typeRef  *Type      // resolved by validate()
	aliasRef *Qualifier // resolved by validate()
}

// hasValue reports whether this qualifier expects a "=value" suffix,
// matching the C source's DCL_REQ flag on a qualifier: any typed,
// non-redirecting qualifier requires a value (defaulted or explicit) once
// specified.
func (q *Qualifier) hasValue() bool { return q.Type != TypeSwitch && q.Syntax == "" }

// Disallow records one grammar "disallow" statement: an illegal combination
// of two qualifier presence/negation states within the same Entry.
type Disallow struct {
	Qual1    string
	Negated1 bool
	Qual2    string
	Negated2 bool
}

// Entry is a verb or a syntax — the C source's shared struct DCL_VERB,
// unified here because dclrtl.c itself treats them identically once parsing
// is underway (see DCLdispatch's single "whichever entry has state
// DCL_PRESENT" walk, and DCLkeysearch's verb<->syntax redirect). A verb is a
// grammar-top-level "verb" statement; a syntax is a "syntax" statement,
// reachable only via a Qualifier's or Keyword's Syntax redirect.
type Entry struct {
	Name       string
	ID         int64
	IsVerb     bool
	Alias      string // e.g. "quit" is an alias for "exit"
	EntryPoint string // /entry=, a VAX microkernel routine name (Phase 10/RTL, not dispatched here)
	Parameters []*Parameter
	Qualifiers []*Qualifier
	Disallows  []*Disallow

	aliasRef *Entry // resolved by validate()
}

func (e *Entry) qualifier(name string) (q *Qualifier, negated bool, err error) {
	return matchQualifier(e.Qualifiers, name)
}

// Handler is a routine bound to an Entry name via Grammar.Bind, called by
// Grammar.Dispatch with the matched Entry's ID — the Go equivalent of a
// DCLbind-registered console_*_dcl routine, which the C source always calls
// with a single `long id` argument (see DCLdispatch).
type Handler func(id int64, r *Result) error

// Grammar is a parsed DCL-style grammar: the named verbs/syntaxes it
// defines, its named keyword Types, and any Handlers bound to an Entry name.
type Grammar struct {
	Name string

	entries   map[string]*Entry // by upcased name, verbs and syntaxes together
	verbOrder []*Entry          // verbs only, in declaration order
	types     map[string]*Type

	handlers map[string]Handler
}

func newGrammar(name string) *Grammar {
	return &Grammar{
		Name:     name,
		entries:  map[string]*Entry{},
		types:    map[string]*Type{},
		handlers: map[string]Handler{},
	}
}

// Bind registers h to be called by Dispatch when the named verb or syntax
// entry (case-insensitive) ends up active after a Parse.
func (g *Grammar) Bind(name string, h Handler) {
	g.handlers[upcase(name)] = h
}
