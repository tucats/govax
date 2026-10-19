package dclshell

import (
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"
)

// F$CONFIG is govax's own lexical function, not one of VMS's: it reads
// govax's configuration (the settings `govax config` shows and sets, and
// HELP CONFIG KEYS lists), so a command procedure can ask how govax is
// set up. Configuration keys are lowercase ("vax.startup.procedure"); the
// key is lowercased before it is looked up, so DCL's habit of uppercasing
// doesn't matter.

// configOperations are F$CONFIG's operation keywords: what each returns
// about the configuration item key.
var configOperations = map[string]func(key string) dclValue{
	// VALUE is the item's value as a string, or "" if it isn't set.
	"VALUE": func(key string) dclValue { return dclString(settings.Get(key)) },

	// EXISTS is 1 if the item is set (even to ""), 0 if it isn't.
	"EXISTS": func(key string) dclValue {
		if settings.Exists(key) {
			return dclInteger(1)
		}

		return dclInteger(0)
	},

	// BOOLEAN is 1 if the item's value is one govax reads as true (TRUE,
	// T, YES, Y, or 1, in any case), 0 if it's anything else or not set:
	// how govax itself reads a true/false item, such as
	// vax.sysgen.paging.enable.
	"BOOLEAN": func(key string) dclValue {
		if settings.GetBool(key) {
			return dclInteger(1)
		}

		return dclInteger(0)
	},
}

// lexConfig is F$CONFIG(key [,operation]): the configuration item key's
// value, or what operation (VALUE if not given; configOperations) asks
// about it. An operation the table doesn't have is CLI_IVKEYW.
func lexConfig(_ *dclExpression, args []lexicalArg) (dclValue, error) {
	key := strings.ToLower(strings.TrimSpace(args[0].str()))

	operation := configOperations["VALUE"]

	if args[1].present {
		op, _, err := lexicalKeyword(configOperations, args[1])
		if err != nil {
			return dclValue{}, err
		}

		operation = op
	}

	return operation(key), nil
}
