package dclshell

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmserrors"
)

// F$GETQUI (docs/planning/PHASE-98 - batch queues.md, subtask 9; the DCL
// Dictionary's entry): information about the queue manager, the queues,
// and their jobs and files, from $GETQUI's own code
// (corevms.Environment.QueueInformation), in DCL's own context, supervisor
// mode's, which an image's $GETQUI doesn't disturb.
//
//	F$GETQUI(function, [item], [object-id], [flags])
//
// function is one of $GETQUI's (corevms.QueueFunctionCode; "" is
// CANCEL_OPERATION). item is one of Table DCLI-9's
// (corevms.QueueItemNamed): its value as an integer, a string in its form
// (a time, a delta time, a UIC, a protection), or TRUE or FALSE for a
// Boolean; with no item the function is done (moving a wildcard search on)
// and the value is "". object-id is the search's name (SEARCH_NAME), or
// for DISPLAY_ENTRY an entry number (SEARCH_NUMBER) unless it holds a
// wildcard; DISPLAY_QUEUE, DISPLAY_MANAGER, DISPLAY_FORM,
// DISPLAY_CHARACTERISTIC, and TRANSLATE_QUEUE must have one (CLI_ARGREQ:
// VMS 7.1's F$GETQUI("DISPLAY_QUEUE","QUEUE_NAME",,"THIS_JOB"), BAT98J's
// view). flags are Table DCLI-8's keywords, separated by commas. Whatever
// error $GETQUI gives (no such queue, no more queues, ...), the value is
// "", as the Dictionary says and BAT98Z.COM relied on.
func lexGetQUI(e *dclExpression, args []lexicalArg) (dclValue, error) {
	env, err := e.console.lexicalEnvironment()
	if err != nil {
		return dclValue{}, err
	}

	function := strings.ToUpper(strings.TrimSpace(args[0].str()))

	code, ok := corevms.QueueFunctionCode(function)
	if !ok {
		return dclValue{}, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, function)
	}

	var item corevms.QueueItem

	itemName := strings.ToUpper(strings.TrimSpace(args[1].str()))
	if args[1].present && itemName != "" {
		if item, ok = corevms.QueueItemNamed(itemName); !ok {
			return dclValue{}, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, itemName)
		}
	}

	q := corevms.QueueQuery{Function: code}

	for _, key := range lexicalKeywords(args[3]) {
		mask, ok := corevms.QueueSearchFlag(key)
		if !ok {
			return dclValue{}, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
		}

		q.Flags |= mask
	}

	switch function {
	case "DISPLAY_QUEUE", "DISPLAY_MANAGER", "DISPLAY_FORM", "DISPLAY_CHARACTERISTIC", "TRANSLATE_QUEUE":
		if !args[2].present {
			return dclValue{}, vmserrors.New(vmserrors.CLI_ARGREQ)
		}
	}

	if args[2].present {
		object := strings.TrimSpace(args[2].str())

		n, nerr := strconv.Atoi(object)

		switch {
		case function == "DISPLAY_ENTRY" && (args[2].value.integer || nerr == nil):
			if args[2].value.integer {
				n = int(args[2].int())
			}

			q.Number, q.HasNumber = n, true
		case function == "DISPLAY_ENTRY" && object == "":
		default:
			q.Name, q.HasName = object, true
		}
	}

	info, status := env.QueueInformation(q)
	if status != corevms.QueueNormal || item.Code == 0 {
		return dclString(""), nil
	}

	data, ok := info.Item(item.Code)
	if !ok {
		return dclString(""), nil
	}

	switch item.Form {
	case corevms.FormBoolean:
		return dclTrueFalse(itemLongword(data)&item.Mask != 0), nil
	case corevms.FormNumber:
		return dclInteger(int32(itemLongword(data))), nil
	case corevms.FormTime:
		if data == "" {
			return dclString(""), nil
		}
	}

	return dclString(env.ItemText(item.Form, data, corevms.StyleDCL)), nil
}
