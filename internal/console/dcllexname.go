package console

import (
	"strings"

	"github.com/tucats/govax/internal/lnm"
)

// The lexical functions on logical names and messages (docs/PHASE-50 -
// DCL command procedures.md, subtask 14; the User's Manual, 15.3 and
// 15.5): F$TRNLNM, F$LOGICAL, and F$MESSAGE. Logical names are the
// console process's (Console.Logicals), translated as $TRNLNM translates
// them (lnm.Database.Translate).

// trnlnmModes are F$TRNLNM's access-mode keywords: a translation sees
// names at that mode and more privileged ones.
var trnlnmModes = map[string]lnm.Mode{
	"USER":       lnm.User,
	"SUPERVISOR": lnm.Supervisor,
	"EXECUTIVE":  lnm.Executive,
	"KERNEL":     lnm.Kernel,
}

// accessModeNames are the access modes as F$TRNLNM's ACCESS_MODE writes them.
var accessModeNames = map[lnm.Mode]string{
	lnm.User:       "USER",
	lnm.Supervisor: "SUPERVISOR",
	lnm.Executive:  "EXECUTIVE",
	lnm.Kernel:     "KERNEL",
}

// trnlnmCases are F$TRNLNM's case keywords: the attributes each gives
// the translation. INTERLOCKED and NONINTERLOCKED (whether to wait for
// a cluster-wide change) mean nothing on one node.
var trnlnmCases = map[string]uint32{
	"CASE_BLIND":     lnm.AttrCaseBlind,
	"CASE_SENSITIVE": 0,
	"INTERLOCKED":    0,
	"NONINTERLOCKED": 0,
}

// trnlnmItems are F$TRNLNM's item keywords: what each returns about the
// logical name e, found, and its equivalence string number index (eqv,
// nil when it has none at that index).
var trnlnmItems = map[string]func(e *lnm.Entry, eqv *lnm.Equivalence) dclValue{
	"ACCESS_MODE": func(e *lnm.Entry, _ *lnm.Equivalence) dclValue { return dclString(accessModeNames[e.Mode]) },
	"CONCEALED":   func(_ *lnm.Entry, eqv *lnm.Equivalence) dclValue { return dclTrueFalse(eqvHas(eqv, lnm.AttrConcealed)) },
	"CONFINE":     func(e *lnm.Entry, _ *lnm.Equivalence) dclValue { return dclTrueFalse(e.Attrs&lnm.AttrConfine != 0) },
	"CRELOG":      func(e *lnm.Entry, _ *lnm.Equivalence) dclValue { return dclTrueFalse(e.Attrs&lnm.AttrCrelog != 0) },
	"LENGTH": func(_ *lnm.Entry, eqv *lnm.Equivalence) dclValue {
		if eqv == nil {
			return dclInteger(0)
		}

		return dclInteger(int32(len(eqv.Value)))
	},
	"MAX_INDEX":  func(e *lnm.Entry, _ *lnm.Equivalence) dclValue { return dclInteger(int32(len(e.Equivalences) - 1)) },
	"NO_ALIAS":   func(e *lnm.Entry, _ *lnm.Equivalence) dclValue { return dclTrueFalse(e.Attrs&lnm.AttrNoAlias != 0) },
	"TABLE":      func(e *lnm.Entry, _ *lnm.Equivalence) dclValue { return dclTrueFalse(e.IsTable()) },
	"TABLE_NAME": func(e *lnm.Entry, _ *lnm.Equivalence) dclValue { return dclString(e.Table.Name) },
	"TERMINAL":   func(_ *lnm.Entry, eqv *lnm.Equivalence) dclValue { return dclTrueFalse(eqvHas(eqv, lnm.AttrTerminal)) },
	"VALUE": func(_ *lnm.Entry, eqv *lnm.Equivalence) dclValue {
		if eqv == nil {
			return dclString("")
		}

		return dclString(eqv.Value)
	},
}

// eqvHas reports whether eqv, if there is one, has the attribute attr.
func eqvHas(eqv *lnm.Equivalence, attr uint32) bool {
	return eqv != nil && eqv.Attrs&attr != 0
}

// lexTrnlnm is F$TRNLNM(logical-name [,table] [,index] [,mode] [,case]
// [,item]): the logical name's equivalence string number index (0 if
// not given), or the item asked for (trnlnmItems), looked up in table
// (LNM$DCL_LOGICAL if not given: the process, job, group, and system
// tables) at mode (USER if not given), without regard to case unless
// CASE_SENSITIVE is given (15.5). A name that isn't there, in a table
// that isn't, is "", whatever the item.
func lexTrnlnm(e *dclExpression, args []lexicalArg) (dclValue, error) {
	item := trnlnmItems["VALUE"]

	if args[5].present {
		v, _, err := lexicalKeyword(trnlnmItems, args[5])
		if err != nil {
			return dclValue{}, err
		}

		item = v
	}

	mode := lnm.User

	if args[3].present {
		m, _, err := lexicalKeyword(trnlnmModes, args[3])
		if err != nil {
			return dclValue{}, err
		}

		mode = m
	}

	attr := lnm.AttrCaseBlind

	if args[4].present {
		attr = 0

		for _, key := range lexicalKeywords(args[4]) {
			a, _, err := lexicalKeyword(trnlnmCases, lexicalArg{value: dclString(key)})
			if err != nil {
				return dclValue{}, err
			}

			attr |= a
		}
	}

	table := "LNM$DCL_LOGICAL"
	if args[1].present {
		table = strings.TrimSpace(args[1].str())
	}

	entry, err := e.console.Logicals.Translate(table, args[0].str(), mode, attr)
	if err != nil {
		return dclString(""), nil //nolint:nilerr // no such name: ""
	}

	var eqv *lnm.Equivalence

	if index := int(args[2].int()); index >= 0 && index < len(entry.Equivalences) {
		eqv = &entry.Equivalences[index]
	}

	return item(entry, eqv), nil
}

// lexLogical is F$LOGICAL(logical-name): the logical name's first
// equivalence string, from the process, job, group, and system tables,
// or "" (15.5: F$TRNLNM supersedes it).
func lexLogical(e *dclExpression, args []lexicalArg) (dclValue, error) {
	entry, err := e.console.Logicals.Translate(lnm.FileDevName, strings.ToUpper(args[0].str()), lnm.User, 0)
	if err != nil || len(entry.Equivalences) == 0 {
		return dclString(""), nil //nolint:nilerr // no such name: ""
	}

	return dclString(entry.Equivalences[0].Value), nil
}

// lexMessage is F$MESSAGE(status-code): the system message file's text
// for the condition value, as $GETMSG gives it ("%SYSTEM-S-NORMAL,
// normal successful completion"; a message's FAO directives are left as
// they are), or a NOMSG text for one it doesn't have. (The second
// argument later releases have, choosing the message's parts, is not in
// VMS 7.3.)
func lexMessage(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	return dclString("%" + messageText(uint32(args[0].int()))), nil
}
