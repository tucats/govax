package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// LIB$LOOKUP_KEY (docs/PHASE-54, "Keywords"; LIB-260): find a keyword, or
// an abbreviation of one, in a table of keywords and values:
//
//	LIB$LOOKUP_KEY search-string ,key-table-array [,key-value]
//	               [,keyword-string] [,resultant-length]
//
// The table (the manual's Figure LIB-5, which the $LIB_KEY_TABLE macro
// builds) is a longword count of the longwords that follow, then, for
// each keyword, the address of its counted ASCII string (a length byte,
// then the text) and its longword value.

var (
	libAmbKey = vmsdef.LibrarySymbols["LIB$_AMBKEY"]
	libUnrKey = vmsdef.LibrarySymbols["LIB$_UNRKEY"]
)

// keyword is one entry of a keyword table.
type keyword struct {
	text  string
	value uint32
}

// readKeyword reads entry i of the keyword table at table.
func readKeyword(env *corevms.Environment, table uint32, i uint32) (keyword, error) {
	mem, cpu := env.Memory(), env.CPU()
	entry := table + 4 + 8*i

	addr, err := mem.LoadLongword(cpu, entry)
	if err != nil {
		return keyword{}, err
	}

	value, err := mem.LoadLongword(cpu, entry+4)
	if err != nil {
		return keyword{}, err
	}

	length, err := mem.LoadByte(cpu, addr)
	if err != nil {
		return keyword{}, err
	}

	text, err := readText(env, addr+1, uint32(length))

	return keyword{text: text, value: value}, err
}

// libLookupKey is LIB$LOOKUP_KEY. The search string matches a keyword
// that starts with it (the comparison is exact: a program upcases what
// it reads, the keywords being uppercase; unconfirmed). An exact match,
// the lengths equal, ends the search with SS$_NORMAL, wherever it is in
// the table (unconfirmed: the manual has the rest of the table scanned
// "after a match has been found", without saying whether an exact match
// later on wins); otherwise one match is SS$_NORMAL, two or more
// LIB$_AMBKEY, and none LIB$_UNRKEY. An empty search string matches every
// keyword (unconfirmed).
//
// The outputs are the match's (the first, for LIB$_AMBKEY: unconfirmed),
// each written when its argument is present: key-value the keyword's
// value, keyword-string the keyword (any class; cut short with no other
// status, as VMS does), and resultant-length the characters written
// there. Fewer than two arguments is LIB$_INVARG ("invalid arguments, not
// enough arguments, and/or bad keyword table"). The table isn't checked:
// VMS reads count/2 entries whatever count is (an odd count of 3, its
// first entry's string at 0, was an access violation there).
func libLookupKey(env *corevms.Environment, argv []uint32) (uint32, error) {
	if tooFewArgs(argv, 2) {
		return libInvArg, nil
	}

	search, err := readString(env, arg(argv, 0))
	if err != nil {
		return stringFailure(env, err)
	}

	table := arg(argv, 1)

	count, err := env.Memory().LoadLongword(env.CPU(), table)
	if err != nil {
		return memoryFault(env, err)
	}

	var (
		match   keyword
		matches int
	)

	for i := range count / 2 {
		k, err := readKeyword(env, table, i)
		if err != nil {
			return memoryFault(env, err)
		}

		if !strings.HasPrefix(k.text, search) {
			continue
		}

		if len(k.text) == len(search) {
			match, matches = k, 1

			break
		}

		if matches == 0 {
			match = k
		}

		matches++
	}

	status := ssNormal

	switch {
	case matches == 0:
		return libUnrKey, nil
	case matches > 1:
		status = libAmbKey
	}

	return keywordOutputs(env, argv, match, status)
}

// keywordOutputs writes LIB$LOOKUP_KEY's optional outputs for match and
// returns status.
func keywordOutputs(env *corevms.Environment, argv []uint32, match keyword, status uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	if addr := arg(argv, 2); addr != 0 {
		if err := mem.StoreLongword(cpu, addr, match.value); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	stored := len(match.text)

	if addr := arg(argv, 3); addr != 0 {
		n, _, err := writeString(env, addr, match.text)
		if err != nil {
			return stringFailure(env, err)
		}

		stored = n
	}

	if addr := arg(argv, 4); addr != 0 {
		if err := mem.StoreWord(cpu, addr, uint16(stored)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	return status, nil
}
