package dclshell

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The lexical functions about the process and its command environment
// (docs/PHASE-50 - DCL command procedures.md, subtask 14; the User's
// Manual, 15.2 and 15.3): F$ENVIRONMENT, F$VERIFY, F$MODE, F$DIRECTORY,
// F$USER, F$IDENTIFIER, F$PROCESS, F$PID, F$GETJPI, F$GETSYI,
// F$PRIVILEGE, and F$SETPRV. The process is the console's, process 1; F$GETJPI and F$PID
// reach the others. F$GETJPI and F$GETSYI read the same items $GETJPI
// and $GETSYI return (corevms.Environment.JPIItem, SYIItem), each
// keyword's item and how its value is written being an entry in a table.

// environmentItems are F$ENVIRONMENT's keywords, and what each returns.
// What govax has no counterpart for is answered as a VMS login with
// DCL's defaults would answer it (unconfirmed against VMS: CONTROL,
// NOCONTROL, and ON_SEVERITY at the terminal).
var environmentItems = map[string]func(c *Interpreter) dclValue{
	"CAPTIVE": func(c *Interpreter) dclValue { return dclTrueFalse(false) },

	// CONTROL and NOCONTROL are the control characters SET CONTROL
	// enables and disables (dclcontroly.go).
	"CONTROL":   func(c *Interpreter) dclValue { return dclString(c.controlString(true)) },
	"NOCONTROL": func(c *Interpreter) dclValue { return dclString(c.controlString(false)) },

	// DEFAULT is the default device and directory, as SHOW DEFAULT shows
	// it (a search list's name, without the lines for its elements).
	"DEFAULT": func(c *Interpreter) dclValue {
		s, _, _ := strings.Cut(c.ContainerSession.DefaultString(), "\n")

		return dclString(s)
	},

	// DEPTH is the command level: 0 at the terminal (13.7). MAX_DEPTH is
	// the limit, the terminal's level among them.
	"DEPTH":     func(c *Interpreter) dclValue { return dclInteger(int32(c.CommandLevel())) },
	"MAX_DEPTH": func(c *Interpreter) dclValue { return dclInteger(MaxCommandLevels + 1) },

	"INTERACTIVE": func(c *Interpreter) dclValue { return dclTrueFalse(true) },
	"KEY_STATE":   func(c *Interpreter) dclValue { return dclString("DEFAULT") },
	"MESSAGE":     func(c *Interpreter) dclValue { return dclString("/FACILITY/SEVERITY/IDENTIFICATION/TEXT") },

	// ON_CONTROL_Y says an ON CONTROL_Y is in effect at this level, and
	// ON_SEVERITY is the severity ON acts at, NONE after SET NOON or at
	// the terminal, where no status is acted on.
	"ON_CONTROL_Y": func(c *Interpreter) dclValue {
		level := c.currentLevel()

		return dclTrueFalse(level != nil && level.on.controlY != "")
	},
	"ON_SEVERITY": func(c *Interpreter) dclValue {
		level := c.currentLevel()
		if level == nil || level.on.off {
			return dclString("NONE")
		}

		if level.on.command == "" {
			return dclString("ERROR")
		}

		return dclString(severityNames[level.on.rank])
	},

	// PROCEDURE is the file the current command procedure came from, ""
	// at the terminal.
	"PROCEDURE": func(c *Interpreter) dclValue {
		if level := c.currentLevel(); level != nil {
			return dclString(level.source.name)
		}

		return dclString("")
	},

	"PROMPT":         func(c *Interpreter) dclValue { return dclString(c.Prompt()) },
	"PROMPT_CONTROL": func(c *Interpreter) dclValue { return dclTrueFalse(true) },

	// PROTECTION is the default file protection, in SET PROTECTION's
	// syntax (15.2.2): VMS's default, as govax has no SET
	// PROTECTION/DEFAULT.
	"PROTECTION": func(c *Interpreter) dclValue { return dclString("SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD") },

	// SYMBOL_SCOPE and VERB_SCOPE are SET SYMBOL/SCOPE's settings
	// (12.11.1), which govax doesn't have: every symbol is seen.
	"SYMBOL_SCOPE": func(c *Interpreter) dclValue { return dclString("LOCAL,GLOBAL") },
	"VERB_SCOPE":   func(c *Interpreter) dclValue { return dclString("LOCAL,GLOBAL") },

	"VERIFY_IMAGE":     func(c *Interpreter) dclValue { return dclTrueFalse(c.verifyImage) },
	"VERIFY_PREFIX":    func(c *Interpreter) dclValue { return dclString(c.verifyPrefix) },
	"VERIFY_PROCEDURE": func(c *Interpreter) dclValue { return dclTrueFalse(c.Verify) },
}

// severityNames are ON's severities as F$ENVIRONMENT("ON_SEVERITY")
// writes them.
var severityNames = map[int]string{
	rankWarning: "WARNING",
	rankError:   "ERROR",
	rankSevere:  "SEVERE",
}

// lexEnvironment is F$ENVIRONMENT(item): what environmentItems says the
// item is.
func lexEnvironment(e *dclExpression, args []lexicalArg) (dclValue, error) {
	item, _, err := lexicalKeyword(environmentItems, args[0])
	if err != nil {
		return dclValue{}, err
	}

	return item(e.console), nil
}

// lexVerify is F$VERIFY([procedure-value] [,image-value]): whether
// procedure verification was on (1) or off (0), and then, with an
// argument, a change: procedure-value (odd for on) sets procedure
// verification, and image verification too unless image-value is given
// to set it (15.2.1). Without procedure-value, image-value changes
// nothing: VMS 7.3's F$VERIFY(,1) left image verification off.
// Verification itself is dclverify.go's.
func lexVerify(e *dclExpression, args []lexicalArg) (dclValue, error) {
	c := e.console
	was := dclBool(c.Verify)

	if !args[0].present {
		return was, nil
	}

	c.Verify = args[0].int()&1 != 0
	c.verifyImage = c.Verify

	if args[1].present {
		c.verifyImage = args[1].int()&1 != 0
	}

	return was, nil
}

// lexMode is F$MODE(): the mode the process runs in. govax's is always
// an interactive one.
func lexMode(_ *dclExpression, _ []lexicalArg) (dclValue, error) {
	return dclString("INTERACTIVE"), nil
}

// lexDirectory is F$DIRECTORY(): the default directory, "[WORK]".
func lexDirectory(e *dclExpression, _ []lexicalArg) (dclValue, error) {
	return dclString(e.console.ContainerSession.DefaultDirectory()), nil
}

// lexUser is F$USER(): the process's UIC, as an identifier where the
// rights database names it ("[SYSTEM]"), as numbers otherwise ("[1,4]").
func lexUser(e *dclExpression, _ []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	return jpiItems["UIC"](env), nil
}

// identifierConversions are F$IDENTIFIER's conversion types: each
// converts the identifier argument, a name or a number, to the other.
var identifierConversions = map[string]func(env *corevms.Environment, id lexicalArg) dclValue{
	// NAME_TO_NUMBER is the identifier's value, as $ASCTOID gives it,
	// or 0 for a name the rights database doesn't have.
	"NAME_TO_NUMBER": func(env *corevms.Environment, id lexicalArg) dclValue {
		value, status := env.IdentifierValue(id.str())
		if status != 0 {
			return dclInteger(0)
		}

		return dclInteger(int32(value))
	},

	// NUMBER_TO_NAME is the identifier's name, as $IDTOASC gives it (a
	// username without brackets), or "" for a value the database
	// doesn't have.
	"NUMBER_TO_NAME": func(env *corevms.Environment, id lexicalArg) dclValue {
		name, _ := env.IdentifierName(uint32(id.int()))

		return dclString(name)
	},
}

// lexIdentifier is F$IDENTIFIER(identifier, conversion-type): a rights
// identifier's name converted to its value, or its value to its name
// (identifierConversions). An identifier the rights database doesn't
// have is 0 or "", as the DCL Dictionary says, whether or not it could
// be one (unconfirmed against VMS: that a name that can't be an
// identifier, SS$_IVIDENT to $ASCTOID, is 0 rather than a message).
func lexIdentifier(e *dclExpression, args []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	convert, _, err := lexicalKeyword(identifierConversions, args[1])
	if err != nil {
		return dclValue{}, err
	}

	return convert(env, args[0]), nil
}

// lexProcess is F$PROCESS(): the process's name.
func lexProcess(e *dclExpression, _ []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	return jpiItems["PRCNAM"](env), nil
}

// lexPID is F$PID(context-symbol): the PID of the next process, in the
// process table's order, and "" after the last (15.3.3). The context
// symbol, set to "" before the first call, keeps the place: govax keeps
// in it how many processes have been returned, and puts "" back after
// the last, so the next call starts again. An undefined context symbol
// is CLI_UNDSYM. A symbol F$CONTEXT made a context symbol returns only
// the processes that meet its criteria (nextContextPID,
// dcllexcontext.go).
func lexPID(e *dclExpression, args []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	name := args[0].name

	sym, ok := e.symbols.lookup(name)
	if !ok {
		return dclValue{}, vmserrors.NewSegment(vmserrors.CLI_UNDSYM, name)
	}

	if sym.context != "" {
		id, ctx, err := e.console.contextOf(e.symbols, name, false)
		if err != nil {
			return dclValue{}, err
		}

		return e.console.nextContextPID(e.symbols, name, id, ctx), nil
	}

	next := int(sym.dclValue().Int())
	procs := env.Processes()

	if next < 0 || next >= len(procs) {
		e.symbols.replaceValue(name, dclString(""))

		return dclString(""), nil
	}

	e.symbols.replaceValue(name, dclInteger(int32(next+1)))

	return dclString(fmt.Sprintf("%08X", procs[next].Process.PID)), nil
}

// replaceValue gives the symbol word names a new value, in the table it
// is in, keeping its name and scope. It does nothing if there's no such
// symbol.
func (t *dclSymbolTable) replaceValue(word string, v dclValue) {
	for _, table := range t.searchOrder() {
		if sym, ok := table.lookup(word); ok {
			sym.value, sym.integer = v.String(), v.integer
			table[sym.name] = sym

			return
		}
	}
}

// jpiItem returns one of F$GETJPI's items for a process.
type jpiItem func(env *corevms.Environment) dclValue

// jpiItems are F$GETJPI's keywords: each one's $GETJPI item, written as
// corevms's item forms say in DCL's style (lexItemValue). Built by init
// from jpiKeywords.
var jpiItems = map[string]jpiItem{
	// STATE is the scheduling state as SHOW SYSTEM names it ("CUR",
	// "LEF", ..., or the resource of an MWAIT: "RWMBX").
	"STATE": func(env *corevms.Environment) dclValue { return dclString(env.ProcessState(env)) },
}

// jpiKeywords are F$GETJPI's other keywords, each the name of its
// $GETJPI item without JPI$_. (MODE is a keyword for the JPI$K_ codes;
// JOBTYPE is the code itself, an integer: VMS 7.3 gave 3 for a local
// terminal's, testdata/dcl50's round 5.)
var jpiKeywords = []string{
	"ACCOUNT", "ASTACT", "ASTCNT", "ASTEN", "ASTLM", "AUTHPRI", "AUTHPRIV",
	"BIOLM", "BUFIO", "BYTLM", "CLINAME", "CPULIM", "CPUTIM", "CREPRC_FLAGS",
	"CURPRIV", "DFWSCNT", "DIOLM", "DIRIO", "EFCS", "EFCU", "ENQCNT", "ENQLM",
	"FILLM", "GPGCNT", "GRP", "IMAGPRIV", "JOBPRCCNT", "JOBTYPE", "LOGINTIM",
	"MASTER_PID", "MEM", "MODE", "OWNER", "PAGEFLTS", "PGFLQUOTA", "PID", "PPGCNT",
	"PRCCNT", "PRCLM", "PRCNAM", "PRI", "PRIB", "PROCPRIV", "TERMINAL", "TMBU",
	"TQLM", "UIC", "USERNAME", "WSAUTH", "WSAUTHEXT", "WSEXTENT", "WSPEAK",
	"WSQUOTA", "WSSIZE",
}

func init() {
	for _, name := range jpiKeywords {
		item := "JPI$_" + name
		form := corevms.JPIItemForm(item, corevms.StyleDCL)

		jpiItems[name] = func(env *corevms.Environment) dclValue {
			data, _ := env.JPIItem(item)

			return lexItemValue(env, form, data)
		}
	}
}

// lexItemValue is an item's value as a lexical function returns it: a
// number as an integer, anything else as the text corevms writes for its
// form in DCL's style.
func lexItemValue(env *corevms.Environment, form corevms.ItemForm, data string) dclValue {
	if form == corevms.FormNumber {
		return dclInteger(int32(itemLongword(data)))
	}

	return dclString(env.ItemText(form, data, corevms.StyleDCL))
}

// itemLongword and itemQuadword read an item's value (corevms's
// ItemLongword and ItemQuadword).
func itemLongword(data string) uint32 { return corevms.ItemLongword(data) }

func itemQuadword(data string) uint64 { return corevms.ItemQuadword(data) }

// PrivilegeList writes a privilege mask as F$GETJPI does
// (corevms.PrivilegeList).
func PrivilegeList(mask uint64) string { return corevms.PrivilegeList(mask) }

// lexGetJPI is F$GETJPI(pid, item): the item (jpiItems) for the process
// whose PID is pid, in hexadecimal, or the console's own for "". A PID
// that names no process is SS$_NONEXPR.
func lexGetJPI(e *dclExpression, args []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	item, _, err := lexicalKeyword(jpiItems, args[1])
	if err != nil {
		return dclValue{}, err
	}

	if pid := strings.TrimSpace(args[0].str()); pid != "" {
		n, perr := strconv.ParseUint(pid, 16, 32)

		target, found := env.FindProcess(uint32(n))
		if perr != nil || !found {
			return dclValue{}, e.console.statusFailure(vmsdef.Symbols["SS$_NONEXPR"])
		}

		env = target
	}

	return item(env), nil
}

// syiKeywords are F$GETSYI's keywords, each the $GETSYI item of the
// same name, written as corevms's item forms say in DCL's style.
var syiKeywords = map[string]bool{
	"BOOTTIME": true, "CLUSTER_MEMBER": true, "CPU": true, "MINWSCNT": true,
	"NODE_CSID": true, "NODE_SWTYPE": true, "NODE_SWVERS": true,
	"NODENAME": true, "SID": true, "VERSION": true,
	"GROWLIM": true, "FREELIM": true, "FREEGOAL": true, "TBSKIPWSL": true,
	"MPW_HILIMIT": true, "MPW_LOLIMIT": true, "PQL_DWSDEFAULT": true,
	"PQL_DWSQUOTA": true, "PQL_DWSEXTENT": true, "PQL_DPGFLQUOTA": true,
}

// lexGetSYI is F$GETSYI(item [,node]): the item (syiItems) for this
// system, the only node (15.3.1). Another node's name is
// SS$_NOSUCHNODE.
func lexGetSYI(e *dclExpression, args []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	_, key, err := lexicalKeyword(syiKeywords, args[0])
	if err != nil {
		return dclValue{}, err
	}

	if node := strings.TrimSpace(args[1].str()); node != "" && !strings.EqualFold(node, env.NodeName) {
		return dclValue{}, e.console.statusFailure(vmsdef.Symbols["SS$_NOSUCHNODE"])
	}

	item := "SYI$_" + key
	data, _ := env.SYIItem(item)

	return lexItemValue(env, corevms.SYIItemForm(item, corevms.StyleDCL), data), nil
}

// privilegeKeyword reads one of F$PRIVILEGE's or F$SETPRV's keywords: a
// privilege's name, or NO and one, as a mask bit and whether it is
// negated. NOACNT is a privilege's own name; NONOACNT negates it. One
// that names no privilege is CLI_IVKEYW (unconfirmed against VMS).
func privilegeKeyword(key string) (mask uint64, negated bool, err error) {
	if mask, ok := corevms.PrivilegeMask(key); ok {
		return mask, false, nil
	}

	if rest, ok := strings.CutPrefix(key, "NO"); ok {
		if mask, ok := corevms.PrivilegeMask(rest); ok {
			return mask, true, nil
		}
	}

	return 0, false, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
}

// lexPrivilege is F$PRIVILEGE(priv-list): "TRUE" if every privilege the
// list names is enabled now, and every one it names with NO is not
// (15.2), "FALSE" otherwise.
func lexPrivilege(e *dclExpression, args []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	data, _ := env.JPIItem("JPI$_CURPRIV")
	current := itemQuadword(data)
	all := true

	for _, key := range lexicalKeywords(args[0]) {
		mask, negated, err := privilegeKeyword(key)
		if err != nil {
			return dclValue{}, err
		}

		if (current&mask != 0) == negated {
			all = false
		}
	}

	return dclTrueFalse(all), nil
}

// lexSetPrv is F$SETPRV(priv-list): it enables each privilege the list
// names, and disables each named with NO (ALL and NOALL are every
// privilege), as SET PROCESS/PRIVILEGES does, and returns what they were
// before: for each one named, in the list's order, its name, or NO and
// its name if it was disabled (15.2). Privileges the process isn't
// authorized for aren't enabled.
func lexSetPrv(e *dclExpression, args []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	type change struct {
		mask   uint64
		enable bool
	}

	var changes []change

	for _, key := range lexicalKeywords(args[0]) {
		switch key {
		case "ALL":
			changes = append(changes, change{corevms.AllPrivileges, true})
		case "NOALL":
			changes = append(changes, change{corevms.AllPrivileges, false})
		default:
			mask, negated, err := privilegeKeyword(key)
			if err != nil {
				return dclValue{}, err
			}

			changes = append(changes, change{mask, !negated})
		}
	}

	data, _ := env.JPIItem("JPI$_CURPRIV")
	before := itemQuadword(data)

	var was []string

	for _, ch := range changes {
		for bit, name := range corevms.PrivilegeNames {
			if name == "" || ch.mask&(1<<bit) == 0 {
				continue
			}

			if before&(1<<bit) == 0 {
				name = "NO" + name
			}

			was = append(was, name)
		}

		env.SetProcessPrivileges(ch.mask, ch.enable)
	}

	return dclString(strings.Join(was, ",")), nil
}
