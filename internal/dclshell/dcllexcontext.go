package dclshell

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmserrors"
)

// F$CONTEXT (Phase 51; the DCL Dictionary): selection criteria for
// F$PID. A procedure sets a symbol to "" and calls F$CONTEXT("PROCESS",
// symbol, item, value, qualifier) as many times as it has criteria, then
// calls F$PID(symbol) until it returns "": each call returns the next
// process that meets every criterion. The symbol becomes a local symbol
// of the type PROCESS_CONTEXT (F$TYPE says so), whose value is the
// context's number; the context itself (processContext) is kept by the
// Shell, by number. Once F$PID has used a context it is frozen: another
// criterion is CLI_FRZNCTX, and the context goes on. It is deleted when
// F$PID returns "", or by F$CONTEXT's CANCEL item, and the symbol is then
// "", an ordinary string.
//
// Each criterion is one selection item (contextItems) with a list of
// values, separated by commas, and a qualifier: EQL or NEQ for every
// item, and for an integer LSS, LEQ, GTR, and GEQ too, for a privilege
// list ALL and ANY. A process meets a criterion when any value of its
// list does (the Dictionary's "OR"); it must meet every criterion. A
// string value may have "*" and "%" wildcards. What an item is compared
// with is F$GETJPI's item of the same name for the process (or, for
// NODENAME and NODE_CSID, F$GETSYI's), so the two always agree.
//
// govax has every process on its one node, and the console's process
// may see them all: the GROUP and WORLD privileges the Dictionary
// mentions aren't asked for. HW_MODEL, HW_NAME, and STS, which $GETSYI
// and $GETJPI don't answer in govax, are CLI_IVKEYW.

// processContextType is the context type F$CONTEXT knows, and the type
// F$TYPE gives its symbol.
const processContextType = "PROCESS_CONTEXT"

// processContext is one F$CONTEXT context: its criteria, and, once F$PID
// has begun (frozen), the index in the process table of the next process
// to look at.
type processContext struct {
	criteria []contextCriterion
	frozen   bool
	next     int
}

// contextCriterion is one F$CONTEXT call's selection: a test of a
// process.
type contextCriterion func(env *corevms.Environment) bool

// contextKind is how a selection item's values are compared, and which
// qualifiers it takes.
type contextKind int

const (
	contextString    contextKind = iota // EQL, NEQ; wildcards
	contextInteger                      // LSS, LEQ, GTR, GEQ, EQL, NEQ
	contextKeyword                      // EQL, NEQ; no wildcards
	contextPID                          // EQL, NEQ; hexadecimal PIDs
	contextUIC                          // EQL, NEQ; "[group,member]" or a name
	contextPrivilege                    // ALL, ANY, EQL, NEQ
)

// contextItem is one of F$CONTEXT's selection items: how its values are
// compared, and its value for a process.
type contextItem struct {
	kind  contextKind
	value func(env *corevms.Environment) dclValue
}

// jpiContext and syiContext are selection items whose value is the
// F$GETJPI item, or the F$GETSYI item, of the same name.
func jpiContext(kind contextKind, name string) contextItem {
	return contextItem{kind: kind, value: func(env *corevms.Environment) dclValue { return jpiItems[name](env) }}
}

func syiContext(kind contextKind, name string) contextItem {
	return contextItem{kind: kind, value: func(env *corevms.Environment) dclValue {
		data, _ := env.SYIItem("SYI$_" + name)

		return syiItems[name](data)
	}}
}

// contextItems are F$CONTEXT's PROCESS selection items (the Dictionary's
// table), but for CANCEL, which isn't a criterion.
var contextItems = map[string]contextItem{
	"ACCOUNT":    jpiContext(contextString, "ACCOUNT"),
	"AUTHPRI":    jpiContext(contextInteger, "AUTHPRI"),
	"CURPRIV":    jpiContext(contextPrivilege, "CURPRIV"),
	"GRP":        jpiContext(contextInteger, "GRP"),
	"JOBPRCCNT":  jpiContext(contextInteger, "JOBPRCCNT"),
	"MASTER_PID": jpiContext(contextPID, "MASTER_PID"),
	"MEM":        jpiContext(contextInteger, "MEM"),
	"MODE":       jpiContext(contextKeyword, "MODE"),
	"NODE_CSID":  {kind: contextInteger, value: func(env *corevms.Environment) dclValue { return syiInteger(env, "SYI$_NODE_CSID") }},
	"NODENAME":   syiContext(contextString, "NODENAME"),
	"OWNER":      jpiContext(contextPID, "OWNER"),
	"PRCCNT":     jpiContext(contextInteger, "PRCCNT"),
	"PRCNAM":     jpiContext(contextString, "PRCNAM"),
	"PRI":        jpiContext(contextInteger, "PRI"),
	"PRIB":       jpiContext(contextInteger, "PRIB"),
	"STATE":      jpiContext(contextKeyword, "STATE"),
	"TERMINAL":   jpiContext(contextString, "TERMINAL"),
	"UIC":        {kind: contextUIC, value: func(env *corevms.Environment) dclValue { return dclInteger(int32(env.Process.UIC)) }},
	"USERNAME":   jpiContext(contextString, "USERNAME"),

	// JOBTYPE's keywords are the JPI$K_ job types' names.
	"JOBTYPE": {kind: contextKeyword, value: jpiKeyword("JPI$_JOBTYPE", "DETACHED", "NETWORK", "BATCH", "LOCAL", "DIALUP", "REMOTE")},
}

// syiInteger is the $GETSYI item called name, as an integer.
func syiInteger(env *corevms.Environment, name string) dclValue {
	data, _ := env.SYIItem(name)

	return dclInteger(int32(itemLongword(data)))
}

// contextQualifiers are the qualifiers each kind of selection item takes.
var contextQualifiers = map[contextKind][]string{
	contextString:    {"EQL", "NEQ"},
	contextInteger:   {"LSS", "LEQ", "GTR", "GEQ", "EQL", "NEQ"},
	contextKeyword:   {"EQL", "NEQ"},
	contextPID:       {"EQL", "NEQ"},
	contextUIC:       {"EQL", "NEQ"},
	contextPrivilege: {"ALL", "ANY", "EQL", "NEQ"},
}

// lexContext is F$CONTEXT(context-type, context-symbol, selection-item
// [,selection-value] [,value-qualifier]): it adds a criterion to the
// context the symbol names, making a new one when the symbol is "" or
// undefined, and returns "" (see this file's opening comment). The only
// context type is PROCESS (another is CLI_IVKEYW). A symbol with another
// value is CLI_NOTCTX, or, when it names no context, after F$PID or
// CANCEL ended it (unconfirmed against VMS), also CLI_NOTCTX. A selection
// value and qualifier must be given except with CANCEL (CLI_ARGREQ); an
// item or qualifier the item doesn't take is CLI_IVKEYW, as is a keyword
// value or a privilege that doesn't exist, and an integer value that
// isn't one CLI_IVVALU (unconfirmed against VMS: these messages).
func lexContext(e *dclExpression, args []lexicalArg) (dclValue, error) {
	c := e.console

	if _, err := c.lexicalEnvironment(); err != nil {
		return dclValue{}, err
	}

	if t := strings.ToUpper(strings.TrimSpace(args[0].str())); t != "PROCESS" {
		return dclValue{}, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, t)
	}

	name := args[1].name
	item := strings.ToUpper(strings.TrimSpace(args[2].str()))

	id, ctx, err := c.contextOf(e.symbols, name, item != "CANCEL")
	if err != nil {
		return dclValue{}, err
	}

	if item == "CANCEL" {
		delete(c.processContexts, id)
		e.symbols.setContext(name, "", 0)

		return dclString(""), nil
	}

	if ctx.frozen {
		return dclValue{}, vmserrors.New(vmserrors.CLI_FRZNCTX)
	}

	criterion, err := contextCriterionFor(c, item, args[3], args[4])
	if err != nil {
		return dclValue{}, err
	}

	ctx.criteria = append(ctx.criteria, criterion)

	return dclString(""), nil
}

// contextOf returns the context the symbol called name names, and its
// number: a new one, the symbol made a context symbol, when the symbol
// is "" or undefined and create is set.
func (c *Interpreter) contextOf(symbols *dclSymbolTable, name string, create bool) (int32, *processContext, error) {
	sym, ok := symbols.lookup(name)

	switch {
	case ok && sym.context == processContextType:
		id := int32(sym.dclValue().Int())
		if ctx := c.processContexts[id]; ctx != nil {
			return id, ctx, nil
		}

		return 0, nil, vmserrors.New(vmserrors.CLI_NOTCTX)
	case ok && sym.context != "":
		return 0, nil, vmserrors.New(vmserrors.CLI_INVCTXTYP)
	case ok && sym.value != "", !create:
		return 0, nil, vmserrors.New(vmserrors.CLI_NOTCTX)
	}

	if c.processContexts == nil {
		c.processContexts = map[int32]*processContext{}
	}

	c.lastContext++
	id := c.lastContext
	ctx := &processContext{}
	c.processContexts[id] = ctx
	symbols.setContext(name, processContextType, id)

	return id, ctx, nil
}

// contextCriterionFor is the criterion one F$CONTEXT call gives: the
// selection item called item, compared with the values in value by
// qualifier.
func contextCriterionFor(c *Interpreter, item string, value, qualifier lexicalArg) (contextCriterion, error) {
	sel, ok := contextItems[item]
	if !ok {
		return nil, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, item)
	}

	if !value.present || !qualifier.present {
		return nil, vmserrors.New(vmserrors.CLI_ARGREQ)
	}

	q := strings.ToUpper(strings.TrimSpace(qualifier.str()))
	if !containsString(contextQualifiers[sel.kind], q) {
		return nil, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, q)
	}

	values := contextValues(value.str())
	if value.value.integer {
		values = []string{strconv.Itoa(int(value.int()))}
	}

	if sel.kind == contextPrivilege {
		return privilegeCriterion(sel, values, q)
	}

	match, err := contextMatcher(c, sel.kind, values, q)
	if err != nil {
		return nil, err
	}

	// NEQ is true when any value differs (the Dictionary's warning
	// about NEQ with a list); the others when any value matches.
	return func(env *corevms.Environment) bool { return match(sel.value(env)) }, nil
}

// contextValues splits a selection value into its list of values,
// uppercase and without blanks: at the commas outside brackets, so a
// UIC, "[1,4]", is one value.
func contextValues(text string) []string {
	var (
		values []string
		depth  int
		start  int
	)

	add := func(v string) {
		if v = strings.ToUpper(strings.TrimSpace(v)); v != "" {
			values = append(values, v)
		}
	}

	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '[', '<':
			depth++
		case ']', '>':
			depth--
		case ',':
			if depth <= 0 {
				add(text[start:i])
				start = i + 1
			}
		}
	}

	add(text[start:])

	return values
}

// contextMatcher returns the test of a process's value against values by
// qualifier q, for an item of kind.
func contextMatcher(c *Interpreter, kind contextKind, values []string, q string) (func(v dclValue) bool, error) {
	var tests []func(v dclValue) bool

	for _, want := range values {
		test, err := contextTest(c, kind, want, q)
		if err != nil {
			return nil, err
		}

		tests = append(tests, test)
	}

	return func(v dclValue) bool {
		for _, test := range tests {
			if test(v) {
				return true
			}
		}

		return false
	}, nil
}

// contextTest is the test of a process's value against one selection
// value, want, by qualifier q.
func contextTest(c *Interpreter, kind contextKind, want, q string) (func(v dclValue) bool, error) {
	neq := q == "NEQ"

	switch kind {
	case contextInteger:
		n, ok := parseDCLNumber(want)
		if !ok {
			return nil, vmserrors.NewSegment(vmserrors.CLI_IVVALU, want)
		}

		return func(v dclValue) bool { return compareContext(int64(v.Int()), int64(n), q) }, nil

	case contextPID:
		pid, err := strconv.ParseUint(want, 16, 32)
		if err != nil {
			return nil, vmserrors.NewSegment(vmserrors.CLI_IVVALU, want)
		}

		return func(v dclValue) bool {
			got, _ := strconv.ParseUint(v.String(), 16, 32)

			return (got == pid) != neq
		}, nil

	case contextUIC:
		uic, ok := c.contextUIC(want)
		if !ok {
			return nil, vmserrors.NewSegment(vmserrors.CLI_IVVALU, want)
		}

		return func(v dclValue) bool { return (uint32(v.Int()) == uic) != neq }, nil

	case contextKeyword:
		return func(v dclValue) bool { return (strings.TrimSpace(v.String()) == want) != neq }, nil
	}

	return func(v dclValue) bool { return lnm.Match(want, strings.ToUpper(strings.TrimSpace(v.String()))) != neq }, nil
}

// compareContext compares a process's integer, got, with a selection
// value, want, by qualifier q.
func compareContext(got, want int64, q string) bool {
	switch q {
	case "LSS":
		return got < want
	case "LEQ":
		return got <= want
	case "GTR":
		return got > want
	case "GEQ":
		return got >= want
	case "NEQ":
		return got != want
	}

	return got == want
}

// contextUIC reads a UIC selection value: "[group,member]" in octal, or
// a UIC identifier's name, with or without brackets.
func (c *Interpreter) contextUIC(text string) (uint32, bool) {
	inner := strings.TrimSuffix(strings.TrimPrefix(text, "["), "]")

	if group, member, ok := strings.Cut(inner, ","); ok {
		g, err1 := strconv.ParseUint(strings.TrimSpace(group), 8, 16)
		m, err2 := strconv.ParseUint(strings.TrimSpace(member), 8, 16)

		return uint32(g<<16 | m), err1 == nil && err2 == nil
	}

	value, status := c.RTL.IdentifierValue(inner)

	return value, status == 0 && value&0x80000000 == 0
}

// privilegeCriterion is a CURPRIV criterion: the process's privileges
// hold ALL of the named ones, ANY of them, exactly them (EQL), or not
// exactly them (NEQ).
func privilegeCriterion(sel contextItem, names []string, q string) (contextCriterion, error) {
	var mask uint64

	for _, name := range names {
		bit, ok := corevms.PrivilegeMask(name)
		if !ok {
			return nil, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, name)
		}

		mask |= bit
	}

	return func(env *corevms.Environment) bool {
		data, _ := env.JPIItem("JPI$_CURPRIV")
		held := itemQuadword(data)

		switch q {
		case "ALL":
			return held&mask == mask
		case "ANY":
			return held&mask != 0
		case "EQL":
			return held == mask
		}

		return held != mask
	}, nil
}

// containsString reports whether list has s.
func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}

	return false
}

// nextContextPID is F$PID's next process for the context numbered id:
// the next one in the process table that meets every criterion, as
// hexadecimal, or "" after the last, when the context is deleted and
// the symbol called name made "".
func (c *Interpreter) nextContextPID(symbols *dclSymbolTable, name string, id int32, ctx *processContext) dclValue {
	ctx.frozen = true

	procs := c.RTL.Processes()

	for ctx.next < len(procs) {
		env := procs[ctx.next]
		ctx.next++

		if contextMeets(env, ctx.criteria) {
			return dclString(fmt.Sprintf("%08X", env.Process.PID))
		}
	}

	delete(c.processContexts, id)
	symbols.setContext(name, "", 0)

	return dclString("")
}

// contextMeets reports whether the process env meets every criterion.
func contextMeets(env *corevms.Environment, criteria []contextCriterion) bool {
	for _, meets := range criteria {
		if !meets(env) {
			return false
		}
	}

	return true
}
