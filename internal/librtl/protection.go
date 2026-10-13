package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The protection-text routines (docs/PHASE-55, the survey's "Protection
// text"; VMS 7.x routines the 5.0 manual lacks, written from the VSI
// LIB$ manual): LIB$FORMAT_SOGW_PROT turns a protection mask into text,
// LIB$PARSE_SOGW_PROT text into a mask, and LIB$PARSE_ACCESS_CODE a
// string of access letters into one category's access mask.
//
// A protection mask is the 16-bit SOGW word: four bits for each
// ownership category, System in bits 0-3, Owner 4-7, Group 8-11, and
// World 12-15, a set bit meaning that access is *denied*. An access-name
// table names each bit of an access mask (a vector of 32 quadword string
// descriptors, an undefined access a zero-length name), and the routines
// use each name's first letter: R, W, E, D for a file. Only the first
// four entries matter here, as each category has four bits.

var strTru = vmsdef.LibrarySymbols["STR$_TRU"]

// categories is the number of ownership categories, and categoryBits the
// bits each has in a protection mask.
const (
	categories   = 4
	categoryBits = 4
)

// fileAccessNames is the FILE class's access-name table, the default
// when a routine's access-names argument is omitted: the DCL
// Dictionary's read, write, execute, and delete, in XAB$W_PRO's bit
// order. Only the first letters are used.
var fileAccessNames = [categoryBits]string{"READ", "WRITE", "EXECUTE", "DELETE"}

// ownershipNames are the categories' names, the default for
// FORMAT_SOGW_PROT's ownership-names and the names PARSE_SOGW_PROT
// accepts, in category order.
var ownershipNames = [categories]string{"System", "Owner", "Group", "World"}

// The routines' default separators: an ownership name's from its
// access letters, and one category's from the next.
const (
	defaultOwnershipSeparator = ": "
	defaultListSeparator      = ", "
)

// descriptorSize is the size of a quadword string descriptor, an entry
// of an access-name or ownership-name vector.
const descriptorSize = 8

// readNameVector reads the first n strings of the vector of quadword
// descriptors at addr, or returns fallback when addr is 0 (omitted).
func readNameVector(env *corevms.Environment, addr uint32, fallback []string) ([]string, error) {
	if addr == 0 {
		return fallback, nil
	}

	names := make([]string, len(fallback))

	for i := range names {
		s, err := readString(env, addr+uint32(i)*descriptorSize)
		if err != nil {
			return nil, err
		}

		names[i] = s
	}

	return names, nil
}

// accessLetters returns each access bit's letter, uppercase: the first
// letter of its name in the table at addr (FILE's when addr is 0), or 0
// for a bit whose name is empty.
func accessLetters(env *corevms.Environment, addr uint32) ([categoryBits]byte, error) {
	var letters [categoryBits]byte

	names, err := readNameVector(env, addr, fileAccessNames[:])
	if err != nil {
		return letters, err
	}

	for i, name := range names {
		if name != "" {
			letters[i] = upper(name[0])
		}
	}

	return letters, nil
}

// upper returns the ASCII letter ch in uppercase, any other byte as it
// is.
func upper(ch byte) byte {
	if ch >= 'a' && ch <= 'z' {
		return ch - 'a' + 'A'
	}

	return ch
}

// isBlank reports whether ch is a blank or a tab, which the parsers
// ignore ("spacing is ignored").
func isBlank(ch byte) bool {
	return ch == ' ' || ch == '\t'
}

// optionalText reads the string the descriptor at addr describes, or
// returns fallback when addr is 0 (omitted).
func optionalText(env *corevms.Environment, addr uint32, fallback string) (string, error) {
	if addr == 0 {
		return fallback, nil
	}

	return readString(env, addr)
}

// storeOptionalWord writes v to the word at addr, unless addr is 0 (an
// omitted output).
func storeOptionalWord(env *corevms.Environment, addr uint32, v uint16) error {
	if addr == 0 {
		return nil
	}

	return env.Memory().StoreWord(env.CPU(), addr, v)
}

// libFormatSogwProt is LIB$FORMAT_SOGW_PROT:
//
//	LIB$FORMAT_SOGW_PROT protection-mask.rwu.r [,access-names.rq.ra]
//	                     [,ownership-names.rq.ra] [,ownership-separator.rt.dx]
//	                     [,list-separator.rt.dx] ,protection-string.wt.dx
//	                     [,protection-length.ww.r]
//
// For System, Owner, Group, and World in turn it writes the category's
// name and, when the mask grants it any access (clear bits), the
// ownership separator and the letters of those accesses in bit order,
// with the list separator between categories:
// "System: RWED, Owner: RWED, Group: RW, World: R". A category with no
// access is its name alone (the separator "introduce[s] a nonempty set
// of access names"; unconfirmed). The names and separators default to
// the categories' full names, ": ", and ", ". The output is written by
// writeString's class rules, truncation being STR$_TRU (the entry's
// status, a warning); protection-length gets the length written.
// LIB$_WRONUMARG for fewer than six arguments or more than seven;
// LIB$_INVARG when protection-mask or protection-string is omitted.
func libFormatSogwProt(env *corevms.Environment, argv []uint32) (uint32, error) {
	if tooFewArgs(argv, 6) || len(argv) > 7 {
		return libWroNumArg, nil
	}

	maskAddr, dest := arg(argv, 0), arg(argv, 5)
	if maskAddr == 0 || dest == 0 {
		return libInvArg, nil
	}

	mask, err := env.Memory().LoadWord(env.CPU(), maskAddr)
	if err != nil {
		return memoryFault(env, err)
	}

	letters, err := accessLetters(env, arg(argv, 1))
	if err != nil {
		return stringFailure(env, err)
	}

	names, err := readNameVector(env, arg(argv, 2), ownershipNames[:])
	if err != nil {
		return stringFailure(env, err)
	}

	ownSep, err := optionalText(env, arg(argv, 3), defaultOwnershipSeparator)
	if err != nil {
		return stringFailure(env, err)
	}

	listSep, err := optionalText(env, arg(argv, 4), defaultListSeparator)
	if err != nil {
		return stringFailure(env, err)
	}

	text := formatProtection(mask, letters, names, ownSep, listSep)

	stored, truncated, err := writeString(env, dest, text)
	if err != nil {
		return stringFailure(env, err)
	}

	if err := storeOptionalWord(env, arg(argv, 6), uint16(stored)); err != nil {
		return memoryFault(env, writing(err))
	}

	if truncated {
		return strTru, nil
	}

	return ssNormal, nil
}

// formatProtection is FORMAT_SOGW_PROT's text for mask, with the access
// letters, ownership names, and separators given.
func formatProtection(mask uint16, letters [categoryBits]byte, names []string, ownSep, listSep string) string {
	var b strings.Builder

	for c := range categories {
		if c > 0 {
			b.WriteString(listSep)
		}

		b.WriteString(names[c])

		var granted []byte

		for bit := range categoryBits {
			if mask>>(c*categoryBits+bit)&1 == 0 && letters[bit] != 0 {
				granted = append(granted, letters[bit])
			}
		}

		if len(granted) > 0 {
			b.WriteString(ownSep)
			b.Write(granted)
		}
	}

	return b.String()
}

// protectionError is a parse failure at offset pos of the string.
type protectionError struct{ pos int }

// parseAccess reads access letters from s starting at i, skipping
// blanks, up to a comma or the end, and returns the access mask they
// grant (bit n for the access whose letter is letters[n]) and where it
// stopped. A character that isn't a blank, a comma, or some access's
// letter (case blind; unconfirmed) is an error at its offset.
func parseAccess(s string, i int, letters [categoryBits]byte) (uint16, int, *protectionError) {
	var granted uint16

	for ; i < len(s) && s[i] != ','; i++ {
		if isBlank(s[i]) {
			continue
		}

		bit := -1

		for n, letter := range letters {
			if letter != 0 && upper(s[i]) == letter {
				bit = n

				break
			}
		}

		if bit < 0 {
			return 0, i, &protectionError{i}
		}

		granted |= 1 << bit
	}

	return granted, i, nil
}

// parseProtection is PARSE_SOGW_PROT's work on s: the protection mask
// (bits set for the accesses not granted to a category named) and the
// ownership mask (each named category's four bits set). A category not
// named has 0 in both (unconfirmed), and one named twice takes its last
// value (unconfirmed). On an error it returns the offending character's
// offset.
func parseProtection(s string, letters [categoryBits]byte) (protection, ownership uint16, perr *protectionError) {
	if strings.TrimLeft(s, " \t") == "" {
		return 0, 0, nil
	}

	i := 0

	for {
		c, next, perr := parseOwnership(s, i)
		if perr != nil {
			return 0, 0, perr
		}

		i = next

		var granted uint16

		if i < len(s) && (s[i] == ':' || s[i] == '=') {
			if granted, i, perr = parseAccess(s, i+1, letters); perr != nil {
				return 0, 0, perr
			}
		}

		shift := c * categoryBits
		nibble := uint16(1<<categoryBits-1) << shift
		ownership |= nibble
		protection = protection&^nibble | ^(granted<<shift)&nibble

		if i >= len(s) {
			return protection, ownership, nil
		}

		i++ // the comma
	}
}

// parseOwnership reads an ownership name from s starting at i, blanks
// skipped, up to a separator (":", "=", ","), or the end: one of the
// four category names in full or abbreviated to any length, case blind.
// It returns the category's number and where the name stopped. An empty
// name is an error at the character after it; a name that's no
// category's is an error at the first character that leaves no category
// it could abbreviate.
func parseOwnership(s string, i int) (int, int, *protectionError) {
	var typed []byte

	for ; i < len(s) && s[i] != ':' && s[i] != '=' && s[i] != ','; i++ {
		if isBlank(s[i]) {
			continue
		}

		typed = append(typed, upper(s[i]))

		if matchOwnership(string(typed)) < 0 {
			return 0, i, &protectionError{i}
		}
	}

	if len(typed) == 0 {
		return 0, i, &protectionError{i}
	}

	return matchOwnership(string(typed)), i, nil
}

// matchOwnership returns the number of the category that typed (in
// uppercase) abbreviates, or -1. The four names' first letters differ,
// so no abbreviation is ambiguous.
func matchOwnership(typed string) int {
	for c, name := range ownershipNames {
		if strings.HasPrefix(strings.ToUpper(name), typed) {
			return c
		}
	}

	return -1
}

// libParseSogwProt is LIB$PARSE_SOGW_PROT:
//
//	LIB$PARSE_SOGW_PROT protection-string.rt.dx [,access-names.rq.ra]
//	                    ,protection-mask.wwu.r ,ownership-mask.wwu.r
//	                    [,end-position.ww.r]
//
// It reads "owner[:|=][letters]" items separated by commas, such as
// "SYSTEM=RWED,OWNER:RWED,GROUP,WORLD:R" (parseProtection): a missing or
// empty access part is no access. protection-mask gets the mask (a set
// bit is an access denied) and ownership-mask the categories named.
// end-position gets the characters processed, the whole string, or on
// an error the offset of the character that made the string invalid;
// an error is LIB$_INVARG with neither mask written (unconfirmed).
// LIB$_WRONUMARG for fewer than four arguments or more than five;
// LIB$_INVARG when a mask's address is omitted.
func libParseSogwProt(env *corevms.Environment, argv []uint32) (uint32, error) {
	if tooFewArgs(argv, 4) || len(argv) > 5 {
		return libWroNumArg, nil
	}

	protAddr, ownAddr, endAddr := arg(argv, 2), arg(argv, 3), arg(argv, 4)
	if protAddr == 0 || ownAddr == 0 {
		return libInvArg, nil
	}

	s, err := readString(env, arg(argv, 0))
	if err != nil {
		return stringFailure(env, err)
	}

	letters, err := accessLetters(env, arg(argv, 1))
	if err != nil {
		return stringFailure(env, err)
	}

	protection, ownership, perr := parseProtection(s, letters)
	if perr != nil {
		if err := storeOptionalWord(env, endAddr, uint16(perr.pos)); err != nil {
			return memoryFault(env, writing(err))
		}

		return libInvArg, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	if err := mem.StoreWord(cpu, protAddr, protection); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreWord(cpu, ownAddr, ownership); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := storeOptionalWord(env, endAddr, uint16(len(s))); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// libParseAccessCode is LIB$PARSE_ACCESS_CODE:
//
//	LIB$PARSE_ACCESS_CODE access-string.rt.dx [,access-names.rq.ra]
//	                      ,ownership-category.rwu.r ,access-mask.wwu.r
//	                      [,end-position.ww.r]
//
// It reads access letters ("RWE"; blanks ignored, as PARSE_SOGW_PROT
// ignores them, unconfirmed) and writes to access-mask the accesses they
// grant, a set bit meaning granted ("the opposite of the standard
// protection format"), in the nibble of the category ownership-category
// names: RWE for Group is 0x0700 (govax's reading of "a mask for the
// requested ownership category"; unconfirmed). ownership-category must
// be exactly one category's four bits (0x000F, 0x00F0, 0x0F00, 0xF000),
// else LIB$_INVARG (unconfirmed). A character that isn't an access's
// letter is LIB$_INVARG, end-position getting its offset, else the
// characters processed. LIB$_WRONUMARG for fewer than four arguments or
// more than five; LIB$_INVARG when ownership-category or access-mask is
// omitted.
func libParseAccessCode(env *corevms.Environment, argv []uint32) (uint32, error) {
	if tooFewArgs(argv, 4) || len(argv) > 5 {
		return libWroNumArg, nil
	}

	catAddr, maskAddr, endAddr := arg(argv, 2), arg(argv, 3), arg(argv, 4)
	if catAddr == 0 || maskAddr == 0 {
		return libInvArg, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	category, err := mem.LoadWord(cpu, catAddr)
	if err != nil {
		return memoryFault(env, err)
	}

	shift := -1

	for c := range categories {
		if category == uint16(1<<categoryBits-1)<<(c*categoryBits) {
			shift = c * categoryBits
		}
	}

	if shift < 0 {
		return libInvArg, nil
	}

	s, err := readString(env, arg(argv, 0))
	if err != nil {
		return stringFailure(env, err)
	}

	letters, err := accessLetters(env, arg(argv, 1))
	if err != nil {
		return stringFailure(env, err)
	}

	// A comma isn't an access letter here: parseAccess stops at one, so
	// the string must end there.
	granted, end, perr := parseAccess(s, 0, letters)
	if perr == nil && end < len(s) {
		perr = &protectionError{end}
	}

	if perr != nil {
		if err := storeOptionalWord(env, endAddr, uint16(perr.pos)); err != nil {
			return memoryFault(env, writing(err))
		}

		return libInvArg, nil
	}

	if err := mem.StoreWord(cpu, maskAddr, granted<<shift); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := storeOptionalWord(env, endAddr, uint16(len(s))); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}
