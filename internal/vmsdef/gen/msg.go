package main

import (
	"fmt"
	"go/format"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
)

// Parsing a VMS message-file listing (docs/PHASE-26.md subtask 25).
//
// VMS keeps the text of every condition value ("%SYSTEM-F-ACCVIO, access
// violation, ...") in message files, compiled from sources written for
// the MESSAGE utility. reference/vms/sysmsg.txt holds part of the VMS 7.3
// system message file's *listing*: the source lines, each prefixed by
// the value the MESSAGE compiler assigned. The two kinds of line this
// parser reads look like this (tabs and spaces vary):
//
//	00000000  ****   .FACILITY  SYSTEM,0 /SHARED /SYSTEM /PREFIX=SS$_
//	0000000C  ****   ACCVIO  <access violation, reason mask=!XB, ...> /FAO=4
//
// The first starts a facility: its name and number. The second defines
// one message: the full condition value (facility, message number, and
// severity), the message's identifier, its text between <> (or ""), and
// qualifiers: /FAO=n, how many $FAO parameters the text consumes, and
// /ID=name, an identifier to show other than the symbol's own.
//
// Every other line (comments, .SEVERITY and .BASE directives, page
// headers) is ignored: the values in the listing already account for
// them.
//
// The listing cuts lines at 132 columns, so a long message loses its end:
// its closing delimiter, and sometimes its /FAO qualifier. Such a text is
// kept as far as it goes, and its parameter count is worked out from the
// directives that remain (faoParamCount).

var (
	msgFacilityRE = regexp.MustCompile(`^\s+[0-9A-F]{8}\s+(?:\d+|\*{4})\s+\.FACILITY\s+([A-Za-z0-9_$]+)\s*,\s*(\d+)`)
	msgDefRE      = regexp.MustCompile(`^\s+([0-9A-F]{8})\s+(?:\d+|\*{4})\s+([A-Za-z][A-Za-z0-9_$]*)\s+([<"])(.*)$`)
	msgFAORE      = regexp.MustCompile(`(?i)/\s*FAO(?:_COUNT)?\s*=\s*(\d+)`)
	msgIDRE       = regexp.MustCompile(`(?i)/\s*ID\s*=\s*([A-Za-z0-9_$]+)`)
)

// message is one parsed message definition.
type message struct {
	code     uint32 // the full condition value from the listing
	facility string
	ident    string
	text     string
	faoCount int
}

// parseMessages reads every message definition in a message listing,
// returning them and the facility names by number. A message number
// defined twice is an error.
func parseMessages(src string) ([]message, map[uint32]string, error) {
	var (
		facilities = map[uint32]string{}
		facility   string
		seen       = map[uint32]string{}
	)

	splits := strings.Split(src, "\n")
	out := make([]message, 0, len(splits))

	for n, line := range strings.Split(src, "\n") {
		line = strings.TrimRight(line, "\r")

		if m := msgFacilityRE.FindStringSubmatch(line); m != nil {
			num, err := strconv.ParseUint(m[2], 10, 12)
			if err != nil {
				return nil, nil, fmt.Errorf("line %d: bad facility number %q", n+1, m[2])
			}

			facility = strings.ToUpper(m[1])
			facilities[uint32(num)] = facility

			continue
		}

		m := msgDefRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		if facility == "" {
			return nil, nil, fmt.Errorf("line %d: message %s before any .FACILITY", n+1, m[2])
		}

		code, err := strconv.ParseUint(m[1], 16, 32)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: bad value %q", n+1, m[1])
		}

		closing := ">"
		if m[3] == `"` {
			closing = `"`
		}

		text, qualifiers, complete := strings.Cut(m[4], closing)

		msg := message{code: uint32(code), facility: facility, ident: strings.ToUpper(m[2]), text: text}

		if id := msgIDRE.FindStringSubmatch(qualifiers); id != nil {
			msg.ident = strings.ToUpper(id[1])
		}

		if fao := msgFAORE.FindStringSubmatch(qualifiers); fao != nil {
			msg.faoCount, _ = strconv.Atoi(fao[1])
		} else if !complete || strings.Contains(qualifiers, "/") {
			// The listing cut the line before the count (or through it):
			// count the parameters the directives left consume.
			msg.faoCount = faoParamCount(text)
		}

		key := msg.code & messageKeyMask
		if prev, dup := seen[key]; dup {
			return nil, nil, fmt.Errorf("line %d: %s has the same message number as %s", n+1, msg.ident, prev)
		}

		seen[key] = msg.ident
		out = append(out, msg)
	}

	return out, facilities, nil
}

// messageKeyMask selects the bits of a condition value that identify a
// message: the facility and message number (bits 3-27). The severity
// (bits 0-2) and control bits (28-31) don't. It must match
// vmsdef.messageKeyMask.
const messageKeyMask = 0x0FFFFFF8

// faoParamCount counts how many $FAO parameters the directives in text
// consume, for a message whose /FAO qualifier the listing cut off: one
// for most directives, two for !AD and !AF, none for the layout
// directives. It's only an estimate for texts that use # or !-, which
// no truncated message in the listing does.
func faoParamCount(text string) int {
	re := regexp.MustCompile(`!(?:\d+|#)?\(?(?:\d+|#)?(%[A-Z]|[A-Z]{2}|[^A-Z%])`)
	n := 0

	for _, m := range re.FindAllStringSubmatch(text, -1) {
		switch d := m[1]; {
		case d == "AD" || d == "AF":
			n += 2
		case len(d) == 2 && d != "%S":
			n++
		}
	}

	return n
}

// messagesFile is the generated file that holds Messages and
// MessageFacilities, in internal/vmsdef.
const messagesFile = "messages_generated.go"

// mergeMessages merges the messages and facilities of one message listing
// into the tables, by the rules mergeSymbols follows: a message (by its
// masked condition value) or facility (by number) that's new is added,
// one that's the same is left alone, and one that differs is a conflict,
// changed only if replace is true. A message's facility, ident, text, and
// $FAO parameter count must all match for it to be the same.
func mergeMessages(messages map[uint32]vmsdef.Message, facilities map[uint32]string,
	msgs []message, facs map[uint32]string, replace bool,
) mergeResult {
	var r mergeResult

	nums := make([]uint32, 0, len(facs))
	for n := range facs {
		nums = append(nums, n)
	}

	sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })

	for _, n := range nums {
		name := facs[n]
		old, ok := facilities[n]

		switch {
		case !ok:
			facilities[n] = name
			r.added = append(r.added, fmt.Sprintf("facility %d %s", n, name))

		case old == name:
			r.same++

		case replace:
			facilities[n] = name
			r.changed = append(r.changed, fmt.Sprintf("facility %d: %s -> %s", n, old, name))

		default:
			r.conflicts = append(r.conflicts, fmt.Sprintf("facility %d: %s, not %s", n, old, name))
		}
	}

	sort.Slice(msgs, func(i, j int) bool { return msgs[i].code&messageKeyMask < msgs[j].code&messageKeyMask })

	for _, m := range msgs {
		key := m.code & messageKeyMask
		msg := vmsdef.Message{Facility: m.facility, Ident: m.ident, Text: m.text, FAOCount: m.faoCount}
		old, ok := messages[key]

		switch {
		case !ok:
			messages[key] = msg
			r.added = append(r.added, fmt.Sprintf("%%%s-%s %q", msg.Facility, msg.Ident, msg.Text))

		case old == msg:
			r.same++

		case replace:
			messages[key] = msg
			r.changed = append(r.changed, fmt.Sprintf("%#x: %+v -> %+v", key, old, msg))

		default:
			r.conflicts = append(r.conflicts, fmt.Sprintf("%#x: %+v, not %+v", key, old, msg))
		}
	}

	return r
}

// generateMessages returns the Go source of messages_generated.go: the
// MessageSources list, the Messages map (keyed by the masked condition
// value), and the MessageFacilities map.
func generateMessages(messages map[uint32]vmsdef.Message, facilities map[uint32]string, sources []string) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "// Code generated by internal/vmsdef/gen. DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package vmsdef\n\n")

	fmt.Fprintf(&b, "// MessageSources names the message listings Messages was built from,\n")
	fmt.Fprintf(&b, "// in the order they were first merged.\n")
	fmt.Fprintf(&b, "var MessageSources = []string{\n")

	for _, s := range sources {
		fmt.Fprintf(&b, "\t%q,\n", s)
	}

	fmt.Fprintf(&b, "}\n\n")

	keys := make([]uint32, 0, len(messages))
	for k := range messages {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	fmt.Fprintf(&b, "// Messages is the text of every VMS message govax knows, from the\n")
	fmt.Fprintf(&b, "// facilities MessageFacilities names, keyed by the message's condition\n")
	fmt.Fprintf(&b, "// value without its severity or control bits (see LookupMessage).\n")
	fmt.Fprintf(&b, "var Messages = map[uint32]Message{\n")

	for _, k := range keys {
		m := messages[k]
		fmt.Fprintf(&b, "\t%#x: {Facility: %q, Ident: %q, Text: %q, FAOCount: %d},\n",
			k, m.Facility, m.Ident, m.Text, m.FAOCount)
	}

	fmt.Fprintf(&b, "}\n\n")

	nums := make([]uint32, 0, len(facilities))
	for n := range facilities {
		nums = append(nums, n)
	}

	sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })

	fmt.Fprintf(&b, "// MessageFacilities names the facilities Messages covers, by facility\n")
	fmt.Fprintf(&b, "// number (bits 16-27 of a condition value).\n")
	fmt.Fprintf(&b, "var MessageFacilities = map[uint32]string{\n")

	for _, n := range nums {
		fmt.Fprintf(&b, "\t%d: %q,\n", n, facilities[n])
	}

	fmt.Fprintf(&b, "}\n")

	out, err := format.Source([]byte(b.String()))
	if err != nil {
		log.Fatalf("gen: generated messages don't compile: %v", err)
	}

	return out
}
