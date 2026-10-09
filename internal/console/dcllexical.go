package console

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// DCL's lexical functions (docs/PHASE-50 - DCL command procedures.md,
// subtasks 9 and 14): F$name(arguments), in an expression, returns an
// integer or a string (User's Manual, 12.8.4). Each function is an entry
// in lexicalFunctions, the table the expression evaluator calls through,
// with how many arguments it takes. The arguments are expressions,
// separated by commas; an optional one may be left out, keeping its
// comma, and a function with none still takes "()". A name starting with
// F$ is a function only when "(" follows it; otherwise it's a symbol.
//
// A function's name may be abbreviated to any prefix that names only one
// of VMS's lexical functions (lexicalNames): F$LEN is F$LENGTH, and F$L
// is CLI_ABFNAM, as VMS 7.3 answered in testdata/dcl50. A name that
// matches none is CLI_IVFNAM. Both show the name and its "(" as their
// segment. A required argument left out is CLI_ARGREQ, an argument too
// many or a missing ")" CLI_SYMDEL.
//
// Subtask 9 brings the syntax and three functions; subtask 14 the rest.
// Until then a call of one of the others is CLI_LEXNOTIMPL.

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

	// call computes the function's value. args has maxArgs entries; one
	// left out, or beyond the call's arguments, is absent.
	call func(e *dclExpression, args []lexicalArg) (dclValue, error)
}

// lexicalArg is one argument of a lexical function call: its value, if
// it was given.
type lexicalArg struct {
	present bool
	value   dclValue
}

// lexicalFunctions are DCL's lexical functions, by name.
var lexicalFunctions = map[string]lexicalFunction{
	// F$INTEGER(expression): the expression's value as an integer,
	// converted by the rules for strings (12.9.1).
	"F$INTEGER": {1, 1, func(_ *dclExpression, args []lexicalArg) (dclValue, error) {
		return dclInteger(args[0].value.Int()), nil
	}},

	// F$LENGTH(string): the string's length.
	"F$LENGTH": {1, 1, func(_ *dclExpression, args []lexicalArg) (dclValue, error) {
		return dclInteger(int32(len(args[0].value.String()))), nil
	}},

	// F$STRING(expression): the expression's value as a string.
	"F$STRING": {1, 1, func(_ *dclExpression, args []lexicalArg) (dclValue, error) {
		return dclString(args[0].value.String()), nil
	}},
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

// lexicalCall evaluates a call of the lexical function name, whose name
// has been read: its argument list, in parentheses, is next.
func (e *dclExpression) lexicalCall(name string) (dclValue, error) {
	f, err := findLexicalFunction(name)
	if err != nil {
		return dclValue{}, err
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
