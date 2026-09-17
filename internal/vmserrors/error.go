package vmserrors

import (
	"fmt"
	"strconv"
	"strings"
)

// argFormatters maps a message-text substitution marker (the character(s)
// following '!') to a function that renders one Argument as text. Two-
// character markers are tried before one-character ones (see Error below),
// so "!XL"/"!XW"/"!XB" are matched in preference to the bare "!X".
//
// This is deliberately a table, not a switch, so a new marker is one map
// entry rather than another branch (see feedback on table-driven dispatch
// for the instruction/RTL/shim tables elsewhere in this codebase).
var argFormatters = map[string]func(a any) (string, bool){
	// Hexadecimal, no padding.
	"X": func(a any) (string, bool) {
		v, ok := toUint64(a)
		if !ok {
			return "", false
		}

		return strconv.FormatUint(v, 16), true
	},
	// Hexadecimal, zero-padded to a byte/word/longword width -- VMS FAO-
	// style !XB/!XW/!XL, used for addresses and register/status dumps
	// where fixed width matters more than a shortest-form fidelity.
	"XB": func(a any) (string, bool) { return fixedHex(a, 2) },
	"XW": func(a any) (string, bool) { return fixedHex(a, 4) },
	"XL": func(a any) (string, bool) { return fixedHex(a, 8) },
	// Decimal, no padding.
	"D": func(a any) (string, bool) {
		v, ok := toInt64(a)
		if !ok {
			return "", false
		}

		return strconv.FormatInt(v, 10), true
	},
	// Unsigned decimal (longword) -- VMS FAO-style !UL.
	"UL": func(a any) (string, bool) {
		v, ok := toUint64(a)
		if !ok {
			return "", false
		}

		return strconv.FormatUint(v, 10), true
	},
	// String, verbatim.
	"S": func(a any) (string, bool) {
		v, ok := a.(string)

		return v, ok
	},
	// String, double-quoted -- for token/symbol/name arguments where the
	// original Go error text used %q.
	"Q": func(a any) (string, bool) {
		v, ok := a.(string)
		if !ok {
			return "", false
		}

		return strconv.Quote(v), true
	},
	// A single character, from a rune/byte/integer argument.
	"C": func(a any) (string, bool) {
		v, ok := toInt64(a)
		if !ok {
			return "", false
		}

		return string(rune(v)), true
	},
}

// fixedHex renders a as zero-padded lowercase hex, width hex digits wide
// (wider values are not truncated, matching Go's own %0*x behavior).
func fixedHex(a any, width int) (string, bool) {
	v, ok := toUint64(a)
	if !ok {
		return "", false
	}

	return fmt.Sprintf("%0*x", width, v), true
}

// toUint64 converts any of Go's integer types (signed or unsigned, any
// width) to a uint64 for formatting. This is deliberately more permissive
// than requiring callers to construct a uint32 argument by hand.
func toUint64(a any) (uint64, bool) {
	switch v := a.(type) {
	case int:
		return uint64(v), true
	case int8:
		return uint64(v), true
	case int16:
		return uint64(v), true
	case int32:
		return uint64(v), true
	case int64:
		return uint64(v), true
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	case uintptr:
		return uint64(v), true
	default:
		return 0, false
	}
}

// toInt64 converts any of Go's integer types to an int64 for formatting.
func toInt64(a any) (int64, bool) {
	switch v := a.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		return int64(v), true
	case uintptr:
		return int64(v), true
	default:
		return 0, false
	}
}

func (e VMSError) Error() string {
	msg, ok := Messages[e.Status]
	if !ok {
		msg = "SYSTEM-F-UNKNOWNERR, Unknown error " + fmt.Sprintf("%08X", e.Status)

		if len(e.Arguments) > 0 {
			msg = msg + " ["

			for idx, arg := range e.Arguments {
				if idx > 0 {
					msg = msg + ", "
				}

				if t, ok := arg.(string); ok {
					msg = msg + strconv.Quote(t)
				} else {
					msg = msg + fmt.Sprintf("%v", arg)
				}
			}

			msg = msg + "]"
		}

		return appendCause(msg, e.Cause)
	}

	if len(e.Arguments) == 0 {
		return appendCause(msg, e.Cause)
	}

	// Scan over the message, and insert positional substitutions from
	// the error string using arguments. The argument must be a type
	// accepted by the marker's formatter (see argFormatters); a mismatch
	// (e.g. a string argument for a numeric marker) leaves the marker
	// text in place rather than substituting garbage, since the error
	// string has no way to know the memory contents a real VMS !S/!AS
	// argument would dereference.
	var b strings.Builder

	argIndex := 0

	for i := 0; i < len(msg); i++ {
		if msg[i] != '!' || i+1 >= len(msg) || argIndex >= len(e.Arguments) {
			b.WriteByte(msg[i])

			continue
		}

		arg := e.Arguments[argIndex]

		// Try the longest marker first (two characters after '!') so
		// "!XL" isn't misread as "!X" followed by a literal "L".
		markerLen := 0
		rendered := ""
		matched := false

		if i+2 < len(msg) {
			if fn, ok := argFormatters[msg[i+1:i+3]]; ok {
				if text, ok := fn(arg); ok {
					rendered, matched, markerLen = text, true, 2
				}
			}
		}

		if !matched {
			if fn, ok := argFormatters[msg[i+1:i+2]]; ok {
				if text, ok := fn(arg); ok {
					rendered, matched, markerLen = text, true, 1
				}
			}
		}

		if !matched {
			b.WriteByte(msg[i])

			continue
		}

		b.WriteString(rendered)

		argIndex++
		i += markerLen
	}

	return appendCause(b.String(), e.Cause)
}

// appendCause appends a wrapped Go error's own message, when present, so
// the original failure (e.g. an *os.PathError) stays visible for
// debugging even though it's now reported as a VMS-style status code.
func appendCause(msg string, cause error) string {
	if cause == nil {
		return msg
	}

	return msg + ": " + cause.Error()
}
