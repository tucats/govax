package console

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// DCL's lexical functions (docs/PHASE-50 - DCL command procedures.md,
// subtasks 9 and 14): F$name(arguments), in an expression, returns an
// integer or a string (User's Manual, 12.8.4 and chapter 15). Each
// function is an entry in lexicalFunctions, the table the expression
// evaluator calls through, with how many arguments it takes; adding one
// is adding an entry. The arguments are expressions, separated by commas;
// an optional one may be left out, keeping its comma, and a function with
// none still takes "()". A name starting with F$ is a function only when
// "(" follows it; otherwise it's a symbol. A few arguments name a symbol
// rather than giving a value (F$TYPE's, F$PID's context): the table says
// which, and the argument is then read as a name, not evaluated.
//
// A function's name may be abbreviated to any prefix that names only one
// of VMS's lexical functions (lexicalNames): F$LEN is F$LENGTH, and F$L
// is CLI_ABFNAM, as VMS 7.3 answered in testdata/dcl50. A name that
// matches none is CLI_IVFNAM. Both show the name and its "(" as their
// segment. A required argument left out is CLI_ARGREQ, an argument too
// many or a missing ")" CLI_SYMDEL.
//
// The functions are grouped by what they work on, each group in a file
// of its own: strings and data types (dcllexstring.go), the process and
// its command environment (dcllexprocess.go), files (dcllexfile.go),
// logical names and messages (dcllexname.go), and time
// (dcllextime.go). Many take a keyword naming what to return, such as
// F$GETJPI's item or F$PARSE's field: each function's keywords are a
// table too (lexicalKeyword), so a new item is one more entry. A keyword
// is spelled in full, in any case; one the table doesn't have is
// CLI_IVKEYW, with the keyword as its segment (unconfirmed against VMS:
// whether DCL takes abbreviations of these keywords).
//
// The functions VMS 7.3 has that govax doesn't (F$CONTEXT, F$CSID,
// F$DEVICE, F$FILE_ATTRIBUTES, F$GETQUI, F$IDENTIFIER, F$LICENSE) are
// CLI_LEXNOTIMPL. So, in a subprocess's command interpreter, which has no
// console to ask, is a function that asks about the system.

// lexicalNames are VMS 7.3's lexical functions (the User's Manual's
// chapter 15 and the DCL Dictionary), for abbreviations.
var lexicalNames = []string{
	"F$CONTEXT", "F$CSID", "F$CVSI", "F$CVTIME", "F$CVUI", "F$DEVICE",
	"F$DIRECTORY", "F$EDIT", "F$ELEMENT", "F$ENVIRONMENT", "F$EXTRACT",
	"F$FAO", "F$FILE_ATTRIBUTES", "F$GETDVI", "F$GETJPI", "F$GETQUI",
	"F$GETSYI", "F$IDENTIFIER", "F$INTEGER", "F$LENGTH", "F$LICENSE",
	"F$LOCATE", "F$LOGICAL", "F$MESSAGE", "F$MODE", "F$PARSE", "F$PID",
	"F$PRIVILEGE", "F$PROCESS", "F$SEARCH", "F$SETPRV", "F$STRING",
	"F$TIME", "F$TRNLNM", "F$TYPE", "F$USER", "F$VERIFY",
}

// lexicalFunction is one lexical function.
type lexicalFunction struct {
	// minArgs and maxArgs are how many arguments it takes: minArgs are
	// required, the rest optional.
	minArgs, maxArgs int

	// symbolArg is the argument, counting from 1, that is a symbol's
	// name rather than an expression; 0 for none.
	symbolArg int

	// console says the function asks the console about the system
	// (processes, files, logical names, the clock), so it needs one.
	console bool

	// call computes the function's value. args has maxArgs entries; one
	// left out, or beyond the call's arguments, is absent.
	call func(e *dclExpression, args []lexicalArg) (dclValue, error)
}

// lexicalArg is one argument of a lexical function call: its value, if
// it was given, or for a symbol-name argument the name, uppercase.
type lexicalArg struct {
	present bool
	value   dclValue
	name    string
}

// str and int are the argument's value as a string and as an integer;
// an absent argument is "" and 0.
func (a lexicalArg) str() string { return a.value.String() }
func (a lexicalArg) int() int32  { return a.value.Int() }

// lexicalFunctions are DCL's lexical functions, by name: how many
// arguments each takes (required, then in all), and the function.
var lexicalFunctions = map[string]lexicalFunction{
	// Strings and data types (dcllexstring.go).
	"F$CVSI":    {minArgs: 3, maxArgs: 3, call: lexCVSI},
	"F$CVUI":    {minArgs: 3, maxArgs: 3, call: lexCVUI},
	"F$EDIT":    {minArgs: 2, maxArgs: 2, call: lexEdit},
	"F$ELEMENT": {minArgs: 3, maxArgs: 3, call: lexElement},
	"F$EXTRACT": {minArgs: 3, maxArgs: 3, call: lexExtract},
	"F$FAO":     {minArgs: 1, maxArgs: 16, console: true, call: lexFAO},
	"F$INTEGER": {minArgs: 1, maxArgs: 1, call: lexInteger},
	"F$LENGTH":  {minArgs: 1, maxArgs: 1, call: lexLength},
	"F$LOCATE":  {minArgs: 2, maxArgs: 2, call: lexLocate},
	"F$STRING":  {minArgs: 1, maxArgs: 1, call: lexString},
	"F$TYPE":    {minArgs: 1, maxArgs: 1, symbolArg: 1, call: lexType},

	// The process and its command environment (dcllexprocess.go).
	"F$DIRECTORY":   {minArgs: 0, maxArgs: 0, console: true, call: lexDirectory},
	"F$ENVIRONMENT": {minArgs: 1, maxArgs: 1, console: true, call: lexEnvironment},
	"F$GETJPI":      {minArgs: 2, maxArgs: 2, console: true, call: lexGetJPI},
	"F$GETSYI":      {minArgs: 1, maxArgs: 2, console: true, call: lexGetSYI},
	"F$MODE":        {minArgs: 0, maxArgs: 0, call: lexMode},
	"F$PID":         {minArgs: 1, maxArgs: 1, symbolArg: 1, console: true, call: lexPID},
	"F$PRIVILEGE":   {minArgs: 1, maxArgs: 1, console: true, call: lexPrivilege},
	"F$PROCESS":     {minArgs: 0, maxArgs: 0, console: true, call: lexProcess},
	"F$SETPRV":      {minArgs: 1, maxArgs: 1, console: true, call: lexSetPrv},
	"F$USER":        {minArgs: 0, maxArgs: 0, console: true, call: lexUser},
	"F$VERIFY":      {minArgs: 0, maxArgs: 2, console: true, call: lexVerify},

	// Files and devices (dcllexfile.go).
	"F$GETDVI": {minArgs: 2, maxArgs: 2, console: true, call: lexGetDVI},
	"F$PARSE":  {minArgs: 1, maxArgs: 5, console: true, call: lexParse},
	"F$SEARCH": {minArgs: 1, maxArgs: 2, console: true, call: lexSearch},

	// Logical names and messages (dcllexname.go).
	"F$LOGICAL": {minArgs: 1, maxArgs: 1, console: true, call: lexLogical},
	"F$MESSAGE": {minArgs: 1, maxArgs: 1, call: lexMessage},
	"F$TRNLNM":  {minArgs: 1, maxArgs: 6, console: true, call: lexTrnlnm},

	// Time (dcllextime.go).
	"F$CVTIME": {minArgs: 0, maxArgs: 3, console: true, call: lexCVTime},
	"F$TIME":   {minArgs: 0, maxArgs: 0, console: true, call: lexTime},
}

// lexicalKeyword returns the entry of table that arg, a keyword argument,
// names: its value, uppercase and without blanks around it, spelled in
// full. One table doesn't have is CLI_IVKEYW, with the keyword as the
// segment.
func lexicalKeyword[T any](table map[string]T, arg lexicalArg) (T, string, error) {
	key := strings.ToUpper(strings.TrimSpace(arg.str()))

	v, ok := table[key]
	if !ok {
		return v, key, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
	}

	return v, key, nil
}

// lexicalKeywords splits arg, a list of keywords separated by commas
// (F$EDIT's edits, F$PRIVILEGE's privileges), into its keywords,
// uppercase and without blanks; empty ones are left out.
func lexicalKeywords(arg lexicalArg) []string {
	var keys []string

	for _, k := range strings.Split(arg.str(), ",") {
		if k = strings.ToUpper(strings.TrimSpace(k)); k != "" {
			keys = append(keys, k)
		}
	}

	return keys
}

// dclTrueFalse is the string a lexical function returns for a truth:
// "TRUE" or "FALSE".
func dclTrueFalse(b bool) dclValue {
	if b {
		return dclString("TRUE")
	}

	return dclString("FALSE")
}

// findLexicalFunction returns the lexical function name (in any case)
// names, or abbreviates.
func findLexicalFunction(name string) (lexicalFunction, error) {
	upper := strings.ToUpper(name)
	segment := name + "("

	var matches []string

	for _, full := range lexicalNames {
		if full == upper {
			matches = []string{full}

			break
		}

		if strings.HasPrefix(full, upper) {
			matches = append(matches, full)
		}
	}

	switch len(matches) {
	case 0:
		return lexicalFunction{}, vmserrors.NewSegment(vmserrors.CLI_IVFNAM, segment)
	case 1:
	default:
		return lexicalFunction{}, vmserrors.NewSegment(vmserrors.CLI_ABFNAM, segment)
	}

	f, ok := lexicalFunctions[matches[0]]
	if !ok {
		return lexicalFunction{}, vmserrors.New(vmserrors.CLI_LEXNOTIMPL, matches[0])
	}

	return f, nil
}

// lexicalName returns the full name of the lexical function name
// abbreviates, for messages; name itself if it abbreviates none or
// several.
func lexicalName(name string) string {
	upper := strings.ToUpper(name)

	var match string

	for _, full := range lexicalNames {
		if full == upper {
			return full
		}

		if strings.HasPrefix(full, upper) {
			if match != "" {
				return upper
			}

			match = full
		}
	}

	if match == "" {
		return upper
	}

	return match
}

// lexicalCall evaluates a call of the lexical function name, whose name
// has been read: its argument list, in parentheses, is next.
func (e *dclExpression) lexicalCall(name string) (dclValue, error) {
	f, err := findLexicalFunction(name)
	if err != nil {
		return dclValue{}, err
	}

	if f.console && e.console == nil {
		return dclValue{}, vmserrors.New(vmserrors.CLI_LEXNOTIMPL, lexicalName(name))
	}

	e.skipBlanks()
	e.pos++ // the "(", which parenNext found

	args, err := e.lexicalArguments(f)
	if err != nil {
		return dclValue{}, err
	}

	return f.call(e, args)
}

// lexicalArguments reads the arguments of a call of f, after its "(",
// through the closing ")". Each comma separates two, and either may be
// empty (absent); a required one that is absent is CLI_ARGREQ, found as
// it's read, and one more than f takes, or no ")", is CLI_SYMDEL. The
// result has an entry for each of f's arguments.
func (e *dclExpression) lexicalArguments(f lexicalFunction) ([]lexicalArg, error) {
	args := make([]lexicalArg, f.maxArgs)

	for n := 0; ; n++ {
		e.skipBlanks()

		present := e.pos < len(e.text) && e.text[e.pos] != ',' && e.text[e.pos] != ')'

		switch {
		case n >= f.maxArgs && (present || (e.pos < len(e.text) && e.text[e.pos] == ',')):
			return nil, vmserrors.New(vmserrors.CLI_SYMDEL)
		case present && n+1 == f.symbolArg:
			name, err := e.symbolNameArgument()
			if err != nil {
				return nil, err
			}

			args[n] = lexicalArg{present: true, name: name}
		case present:
			v, err := e.binary(precOr)
			if err != nil {
				return nil, err
			}

			args[n] = lexicalArg{present: true, value: v}
		case n < f.minArgs:
			return nil, vmserrors.New(vmserrors.CLI_ARGREQ)
		}

		if e.skipBlanks(); e.pos >= len(e.text) {
			return nil, vmserrors.New(vmserrors.CLI_SYMDEL)
		}

		switch e.text[e.pos] {
		case ',':
			e.pos++
		case ')':
			e.pos++

			if n+1 < f.minArgs {
				return nil, vmserrors.New(vmserrors.CLI_ARGREQ)
			}

			return args, nil
		default:
			return nil, vmserrors.New(vmserrors.CLI_SYMDEL)
		}
	}
}

// symbolNameArgument reads an argument that names a symbol: a name, not
// evaluated, returned uppercase. Anything else is CLI_IVSYMB
// (unconfirmed against VMS).
func (e *dclExpression) symbolNameArgument() (string, error) {
	if !isDCLSymbolStart(e.text[e.pos]) {
		return "", vmserrors.New(vmserrors.CLI_IVSYMB)
	}

	return strings.ToUpper(e.name()), nil
}
