package lbr

import (
	"fmt"
	"strconv"
	"strings"
)

// This file turns a help file's lines into help modules, as VMS 7.1's
// LIBRARY/HELP does. The rules come from testdata/help90's probe (docs/
// PHASE-90 - VMS help.md, "What the probe settled", rounds 1 and 1b), which
// ran LIBRARY/HELP on files written to ask each question:
//
//   - A key line is a level number, then a blank, then the key: "1 ALPHA",
//     "2  Parameters", "10 apples is a line" (whose key is "apples"). A number with no blank after it ("2Nospace",
//     a bare "2", "2" and a tab) or a level of 0 is a line of text.
//   - A level-1 key line starts a module, named by its key in the case the
//     file has it. The module's records are its lines as written, key lines
//     too, with tabs and long lines kept, but a line holding only blanks is
//     stored as an empty record.
//   - Lines before the first key line are dropped, with no message.
//   - Within a module, a key may be at most one level deeper than the key
//     before it. A deeper one ends the file's reading with ILLKEYLVL. So does
//     a first key line that isn't level 1 (unconfirmed: the probe's files
//     all start at 1).
//   - A level-1 key longer than the library's keys ends it with KEYNAMLNG.

// HelpProblem is what stopped LIBRARY/HELP reading a help file.
type HelpProblem int

const (
	// HelpBadLevel is ILLKEYLVL: a key more than one level below the key
	// before it.
	HelpBadLevel HelpProblem = iota + 1
	// HelpKeyTooLong is KEYNAMLNG: a module's name longer than the
	// library's keys.
	HelpKeyTooLong
)

// HelpError is a help file's bad key line. LIBRARY/HELP inserts the
// modules before it, and reads no more of the file.
type HelpError struct {
	Problem HelpProblem
	// Level and Key are the bad key line's.
	Level int
	Key   string
	// Line is the bad line's number in the file, from 1.
	Line int
	// Partial is the module the bad line was in, as far as it went, for
	// Builder.Abandon: VMS's LIBRARY has written that much of it to the
	// library by the time it reads the bad line. It's nil when the bad
	// line is the module's own key line.
	Partial *Entry
}

func (e *HelpError) Error() string {
	switch e.Problem {
	case HelpBadLevel:
		return fmt.Sprintf("lbr: line %d: key %s is at level %d, more than one level below the key before it", e.Line, e.Key, e.Level)
	case HelpKeyTooLong:
		return fmt.Sprintf("lbr: line %d: key %s is longer than the library's keys", e.Line, e.Key)
	}

	return fmt.Sprintf("lbr: line %d: bad key line", e.Line)
}

// HelpKeyLine reports whether line is a help module's key line, and if it
// is, its level and key. The key is the line's first word after the level:
// ILLKEYLVL names "apples" for the line "10 apples is a line ..." (testdata/
// help90, round 1b), and a module's name is its level-1 key's first word
// (unconfirmed: the probe's level-1 keys are single words). A level number
// and blanks with no key is a line of text (unconfirmed: the probe had no
// such line).
func HelpKeyLine(line []byte) (level int, key string, ok bool) {
	digits := 0
	for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
		digits++
	}

	if digits == 0 || digits >= len(line) || line[digits] != ' ' {
		return 0, "", false
	}

	level, err := strconv.Atoi(string(line[:digits]))
	if err != nil || level == 0 {
		return 0, "", false
	}

	fields := strings.Fields(string(line[digits:]))
	if len(fields) == 0 {
		return 0, "", false
	}

	return level, fields[0], true
}

// HelpModules splits a help file's lines into modules for b, a help
// library. If a bad key line stops it, it returns the modules before that
// line and the HelpError.
func (b *Builder) HelpModules(lines [][]byte) ([]*Entry, *HelpError) {
	if b.Type != TypeHelp {
		return nil, &HelpError{Problem: HelpBadLevel, Line: 1}
	}

	var (
		out  []*Entry
		cur  *Entry
		last int // the level of the last key line in cur
	)

	for i, line := range lines {
		level, key, ok := HelpKeyLine(line)

		switch {
		case !ok && cur == nil:
			// Text before the first key line.
			continue
		case !ok:
			cur.Records = append(cur.Records, helpRecord(line))

			continue
		case level == 1:
			if len(key) > b.KeySize {
				return out, &HelpError{Problem: HelpKeyTooLong, Level: level, Key: key, Line: i + 1}
			}

			cur = &Entry{Name: key, Records: [][]byte{helpRecord(line)}}
			out = append(out, cur)
		case cur == nil || level > last+1:
			e := &HelpError{Problem: HelpBadLevel, Level: level, Key: key, Line: i + 1}

			if cur != nil {
				out = out[:len(out)-1]
				e.Partial = cur
			}

			return out, e
		default:
			cur.Records = append(cur.Records, helpRecord(line))
		}

		last = level
	}

	return out, nil
}

// helpRecord is line as a module's record: a line of nothing but blanks
// is an empty one.
func helpRecord(line []byte) []byte {
	if strings.Trim(string(line), " ") == "" {
		return []byte{}
	}

	return line
}
