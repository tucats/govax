package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The routines that work with the command language interpreter that ran
// the image (docs/PHASE-57, the 5.0 manual's section 2.2). Each reaches
// the process's DCL through corevms.CLI (Environment.CLI); a process
// with none (one $CREPRC created to run an image) gets LIB$_NOCLI, which
// each routine checks for before it reads its arguments (unconfirmed:
// the manuals don't give the order).

var (
	libNoCLI     = vmsdef.LibrarySymbols["LIB$_NOCLI"]
	libInvSymNam = vmsdef.LibrarySymbols["LIB$_INVSYMNAM"]
)

// maxSymbolName is the longest symbol name the symbol routines take, and
// maxSymbolValue the longest value LIB$SET_SYMBOL does (the 5.0 manual's
// limit; the VSI manual's is 4096).
const (
	maxSymbolName  = 255
	maxSymbolValue = 255
)

// The characters a symbol name may start with, by the 5.0 manual's
// entries, which differ (docs/PHASE-57, "Symbols"; unconfirmed, a probe
// case): LIB$GET_SYMBOL's "a letter or dollar sign", and LIB$SET_SYMBOL's
// and LIB$DELETE_SYMBOL's letter, underscore, or dollar sign.
func getSymbolStart(ch byte) bool { return isLetter(ch) || ch == '$' }

func setSymbolStart(ch byte) bool { return getSymbolStart(ch) || ch == '_' }

// symbolName reads the symbol name at addr as the symbol routines do:
// uppercased, its trailing blanks removed. It returns the name, or a
// status: LIB$_INVSYMNAM for a name that is empty, longer than
// maxSymbolName, or starts with a character start doesn't allow. Nothing
// is said of the characters after the first, so any is taken
// (unconfirmed). err is readString's.
func symbolName(env *corevms.Environment, addr uint32, start func(byte) bool) (string, uint32, error) {
	text, err := readString(env, addr)
	if err != nil {
		return "", 0, err
	}

	name := []byte(strings.TrimRight(text, " "))
	for i, ch := range name {
		name[i] = upper(ch)
	}

	if len(name) == 0 || len(name) > maxSymbolName || !start(name[0]) {
		return "", libInvSymNam, nil
	}

	return string(name), 0, nil
}

// symbolTable reads a table-type-indicator argument at addr: the local
// table when it's omitted, LIB$_INVARG (as status) for a value other
// than LIB$K_CLI_LOCAL_SYM or LIB$K_CLI_GLOBAL_SYM.
func symbolTable(env *corevms.Environment, addr uint32) (table, status uint32, err error) {
	if addr == 0 {
		return corevms.CLILocalSymbol, 0, nil
	}

	table, err = env.Memory().LoadLongword(env.CPU(), addr)
	if err != nil {
		return 0, 0, err
	}

	if table != corevms.CLILocalSymbol && table != corevms.CLIGlobalSymbol {
		return 0, libInvArg, nil
	}

	return table, 0, nil
}

// libGetSymbol is LIB$GET_SYMBOL (LIB-219):
//
//	LIB$GET_SYMBOL symbol ,resultant-string [,resultant-length] [,table-type-indicator]
//
// symbol.rt.dx, resultant-string.wt.dx, resultant-length.ww.r,
// table-type-indicator.wl.r. It looks the symbol up in the local symbol
// table, then the global one (corevms.CLI's Symbol, which follows DCL's
// own lookup: every command level's local table from the current one
// out, and abbreviations; unconfirmed, probe cases), and writes its value,
// an integer's as a signed decimal number; resultant-length gets the
// number of characters stored, and table-type-indicator 1 (local) or 2
// (global).
//
// It returns SS$_NORMAL; LIB$_STRTRU (a success) when the value didn't
// fit a fixed-length resultant-string; LIB$_NOSUCHSYM; LIB$_INVSYMNAM;
// LIB$_NOCLI; LIB$_INVSTRDES; or LIB$_INSVIRMEM. An argument it can't
// read or write is an access violation, signaled (memoryFault).
func libGetSymbol(env *corevms.Environment, argv []uint32) (uint32, error) {
	if env.CLI == nil {
		return libNoCLI, nil
	}

	name, status, err := symbolName(env, arg(argv, 0), getSymbolStart)
	if err != nil {
		return stringFailure(env, err)
	} else if status != 0 {
		return status, nil
	}

	value, table, status := env.CLI.Symbol(name)
	if status != ssNormal {
		return status, nil
	}

	stored, truncated, err := writeString(env, arg(argv, 1), value)
	if err != nil {
		return stringFailure(env, err)
	}

	mem, cpu := env.Memory(), env.CPU()

	if a := arg(argv, 2); a != 0 {
		if err := mem.StoreWord(cpu, a, uint16(stored)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if a := arg(argv, 3); a != 0 {
		if err := mem.StoreLongword(cpu, a, table); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if truncated {
		return libStrTru, nil
	}

	return ssNormal, nil
}

// libSetSymbol is LIB$SET_SYMBOL (LIB-343):
//
//	LIB$SET_SYMBOL symbol ,value-string [,table-type-indicator]
//
// symbol.rt.dx, value-string.rt.dx, table-type-indicator.rl.r. It
// defines the symbol as a string symbol ("Integer values are not
// allowed"), trailing blanks and all, in the local table of the command
// level that ran the image (the default) or in the global table. An "*"
// in the name marks the shortest abbreviation, as in a DCL assignment
// (unconfirmed, a probe case).
//
// It returns SS$_NORMAL; LIB$_AMBSYMDEF when an abbreviation of the name
// would also mean another symbol of the table (unconfirmed which: see
// corevms.CLI's SetSymbol); LIB$_INVSYMNAM; LIB$_INVARG for a table
// indicator other than 1 or 2, or a value longer than 255 characters
// (the 5.0 manual's limit, which gives no status for passing it; the VSI
// manual's LIB$_INVARG past its own 4096, unconfirmed); LIB$_NOCLI; or
// LIB$_INVSTRDES. An argument it can't read is an access violation,
// signaled (memoryFault).
func libSetSymbol(env *corevms.Environment, argv []uint32) (uint32, error) {
	if env.CLI == nil {
		return libNoCLI, nil
	}

	name, status, err := symbolName(env, arg(argv, 0), setSymbolStart)
	if err != nil {
		return stringFailure(env, err)
	} else if status != 0 {
		return status, nil
	}

	value, err := readString(env, arg(argv, 1))
	if err != nil {
		return stringFailure(env, err)
	}

	table, status, err := symbolTable(env, arg(argv, 2))
	if err != nil {
		return memoryFault(env, err)
	} else if status != 0 {
		return status, nil
	}

	if len(value) > maxSymbolValue {
		return libInvArg, nil
	}

	return env.CLI.SetSymbol(name, value, table), nil
}

// libDeleteSymbol is LIB$DELETE_SYMBOL (LIB-116):
//
//	LIB$DELETE_SYMBOL symbol [,table-type-indicator]
//
// symbol.rt.dx, table-type-indicator.rl.r. It deletes the symbol from
// the local table of the command level that ran the image (the default)
// or from the global table, as DELETE/SYMBOL does: the name may be an
// abbreviation the symbol allows.
//
// It returns SS$_NORMAL; LIB$_NOSUCHSYM when the table has no such
// symbol; LIB$_INVSYMNAM; LIB$_INVARG for a table indicator other than 1
// or 2; LIB$_NOCLI; or LIB$_INVSTRDES. An argument it can't read is an
// access violation, signaled (memoryFault).
func libDeleteSymbol(env *corevms.Environment, argv []uint32) (uint32, error) {
	if env.CLI == nil {
		return libNoCLI, nil
	}

	name, status, err := symbolName(env, arg(argv, 0), setSymbolStart)
	if err != nil {
		return stringFailure(env, err)
	} else if status != 0 {
		return status, nil
	}

	table, status, err := symbolTable(env, arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	} else if status != 0 {
		return status, nil
	}

	return env.CLI.DeleteSymbol(name, table), nil
}

// The control keys (docs/PHASE-57, "Control keys"; the 5.0 manual's
// section 2.2.5): LIB$DISABLE_CTRL and LIB$ENABLE_CTRL are SET NOCONTROL
// and SET CONTROL for Ctrl/T and Ctrl/Y, each key a bit of a mask
// (LIB$M_CLI_CTRLT, LIB$M_CLI_CTRLY). The setting is the DCL's, so it
// outlasts the image.

// validControlKeys are the mask bits the CLI can intercept.
const validControlKeys = corevms.CLICtrlT | corevms.CLICtrlY

// controlKeys is LIB$DISABLE_CTRL (enable false) and LIB$ENABLE_CTRL:
// it reads the mask at argv[0], changes the keys it names, and writes
// the keys enabled before to old-mask at argv[1], if given. Both
// routines write the keys previously *enabled*, so that DISABLE_CTRL's
// old-mask given to ENABLE_CTRL restores them, as the VSI manual says of
// DISABLE_CTRL (unconfirmed for ENABLE_CTRL, a probe case).
//
// It returns SS$_NORMAL; LIB$_NOCLI (checked first); or LIB$_INVARG for
// a bit other than Ctrl/T's and Ctrl/Y's, when nothing is changed and
// old-mask isn't written (unconfirmed: the manuals don't say whether the
// valid bits are acted on). A mask or old-mask it can't reach is an
// access violation, signaled (memoryFault).
func controlKeys(env *corevms.Environment, argv []uint32, enable bool) (uint32, error) {
	if env.CLI == nil {
		return libNoCLI, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	mask, err := mem.LoadLongword(cpu, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if mask&^validControlKeys != 0 {
		return libInvArg, nil
	}

	var old uint32
	if enable {
		old = env.CLI.ControlKeys(mask, 0)
	} else {
		old = env.CLI.ControlKeys(0, mask)
	}

	if a := arg(argv, 1); a != 0 {
		if err := mem.StoreLongword(cpu, a, old); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	return ssNormal, nil
}

// libDisableCtrl is LIB$DISABLE_CTRL (LIB-122):
//
//	LIB$DISABLE_CTRL disable-mask [,old-mask]
//
// disable-mask.rlu.r, old-mask.wlu.r. It stops DCL intercepting the
// control keys disable-mask names, as SET NOCONTROL does: a Ctrl/Y typed
// then is echoed and does nothing else. Disabling Ctrl/Y also cancels
// the ON CONTROL_Y of the command level that ran the image, as SET
// NOCONTROL=Y does (unconfirmed, a probe case). See controlKeys.
func libDisableCtrl(env *corevms.Environment, argv []uint32) (uint32, error) {
	return controlKeys(env, argv, false)
}

// libEnableCtrl is LIB$ENABLE_CTRL (LIB-138):
//
//	LIB$ENABLE_CTRL enable-mask [,old-mask]
//
// enable-mask.rlu.r, old-mask.wlu.r. It has DCL intercept the control
// keys enable-mask names again, as SET CONTROL does. See controlKeys.
func libEnableCtrl(env *corevms.Environment, argv []uint32) (uint32, error) {
	return controlKeys(env, argv, true)
}

// libPause is LIB$PAUSE (LIB-299):
//
//	LIB$PAUSE
//
// It "suspends program execution and returns control to the calling
// command level": DCL reads commands at its Ctrl/Y level with the image
// paused (corevms.CLI's Pause; internal/dclshell/dclpause.go). CONTINUE
// resumes the image, and the routine returns SS$_NORMAL; EXIT or STOP
// end the image, and the routine doesn't return. In batch it does
// nothing and returns SS$_NORMAL. Without a CLI it returns LIB$_NOCLI.
func libPause(env *corevms.Environment, _ []uint32) (uint32, error) {
	if env.CLI == nil {
		return libNoCLI, nil
	}

	return env.CLI.Pause()
}
