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
// comma, and a function with none still takes "()".
//
// A function's name may be abbreviated to any prefix that names only it
// (an ambiguous one is CLI_ABFNAM, one naming none CLI_IVFNAM), as DCL's
// messages for the two imply. Unconfirmed against VMS: whether there is
// a shortest abbreviation, and that too many arguments are CLI_MAXPARM.
//
// Subtask 9 brings the syntax and three functions; subtask 14 the rest.

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
	name = strings.ToUpper(name)

	if f, ok := lexicalFunctions[name]; ok {
		return f, nil
	}

	var matches []string

	for full := range lexicalFunctions {
		if strings.HasPrefix(full, name) {
			matches = append(matches, full)
		}
	}

	switch len(matches) {
	case 0:
		return lexicalFunction{}, vmserrors.New(vmserrors.CLI_IVFNAM, name)
	case 1:
		return lexicalFunctions[matches[0]], nil
	}

	return lexicalFunction{}, vmserrors.New(vmserrors.CLI_ABFNAM, name)
}

// lexicalCall evaluates a call of the lexical function name, whose name
// has been read: its argument list, in parentheses, is next.
func (e *dclExpression) lexicalCall(name string) (dclValue, error) {
	f, err := findLexicalFunction(name)
	if err != nil {
		return dclValue{}, err
	}

	if e.skipBlanks(); e.atEnd() || e.text[e.pos] != '(' {
		return dclValue{}, vmserrors.New(vmserrors.CLI_NOPAREN, strings.ToUpper(name))
	}

	e.pos++

	args, err := e.lexicalArguments()
	if err != nil {
		return dclValue{}, err
	}

	if len(args) > f.maxArgs {
		return dclValue{}, vmserrors.New(vmserrors.CLI_MAXPARM)
	}

	for len(args) < f.maxArgs {
		args = append(args, lexicalArg{})
	}

	for _, arg := range args[:f.minArgs] {
		if !arg.present {
			return dclValue{}, vmserrors.New(vmserrors.CLI_ARGREQ, strings.ToUpper(name))
		}
	}

	return f.call(e, args)
}

// lexicalArguments reads a lexical function's arguments, after its "(",
// through the closing ")". "()" is no arguments; otherwise each comma
// separates two, either of which may be empty (absent).
func (e *dclExpression) lexicalArguments() ([]lexicalArg, error) {
	if e.skipBlanks(); e.pos < len(e.text) && e.text[e.pos] == ')' {
		e.pos++

		return nil, nil
	}

	var args []lexicalArg

	for {
		var arg lexicalArg

		if e.skipBlanks(); e.pos < len(e.text) && e.text[e.pos] != ',' && e.text[e.pos] != ')' {
			v, err := e.binary(precOr)
			if err != nil {
				return nil, err
			}

			arg = lexicalArg{present: true, value: v}
		}

		args = append(args, arg)

		if e.skipBlanks(); e.pos >= len(e.text) {
			return nil, vmserrors.New(vmserrors.CLI_NOPAREN, e.rest())
		}

		switch e.text[e.pos] {
		case ',':
			e.pos++
		case ')':
			e.pos++

			return args, nil
		default:
			return nil, e.syntaxError()
		}
	}
}
