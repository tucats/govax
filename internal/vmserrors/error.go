package vmserrors

import (
	"strconv"
	"strings"
)

func (e VMSError) Error() string {
	msg, ok := Messages[e.Status]
	if !ok {
		return "SYS$UNKNOWN, Unknown error" + strconv.FormatUint(uint64(e.Status), 16)
	}

	if len(e.Arguments) == 0 {
		return msg
	}

	// Scan over the message, and insert positional substitutions from
	// the error string using arguments. The argument must be either an
	// integer value or a string. If the argument is encoded as !S for a
	// string, but the argument is a number, that will have to be resolved
	// by the caller, since the error string is not aware of the memory
	// contents. The error string will leave that marker for later processing.
	var b strings.Builder

	argIndex := 0

	for i := 0; i < len(msg); i++ {
		// A marker is '!' followed by a type character; anything else is
		// copied through verbatim.
		if msg[i] != '!' || i+1 >= len(msg) || argIndex >= len(e.Arguments) {
			b.WriteByte(msg[i])

			continue
		}

		arg := e.Arguments[argIndex]
		replaced := true

		switch msg[i+1] {
		case 'X':
			// Hexadecimal argument
			if v, ok := arg.(uint32); ok {
				b.WriteString(strconv.FormatUint(uint64(v), 16))
			} else {
				replaced = false
			}
		case 'D':
			// Decimal argument
			if v, ok := arg.(uint32); ok {
				b.WriteString(strconv.FormatUint(uint64(v), 10))
			} else {
				replaced = false
			}
		case 'S':
			// String argument
			if v, ok := arg.(string); ok {
				b.WriteString(v)
			} else {
				replaced = false
			}
		default:
			replaced = false
		}

		if !replaced {
			b.WriteByte(msg[i])

			continue
		}

		argIndex++
		i++ // consume the marker's type character too
	}

	return b.String()
}
