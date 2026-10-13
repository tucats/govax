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

var (
	strTru        = vmsdef.LibrarySymbols["STR$_TRU"]
	ssNoClass     = vmsdef.Symbols["SS$_NOCLASS"]
	libSyntaxErr  = vmsdef.LibrarySymbols["LIB$_SYNTAXERR"]
	protectionAll = uint16(0xFFFF) // every access denied to every category
)

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
// letter of its name in the table at addr (FILE's when addr is 0). A
// bit whose name is empty takes FILE's letter (VMS 7.1 wrote a table of
// ALPHA, BETA, "", DELTA as "ABED": testdata/lib55).
func accessLetters(env *corevms.Environment, addr uint32) ([categoryBits]byte, error) {
	var letters [categoryBits]byte

	names, err := readNameVector(env, addr, fileAccessNames[:])
	if err != nil {
		return letters, err
	}

	for i, name := range names {
		if name == "" {
			name = fileAccessNames[i]
		}

		letters[i] = upper(name[0])
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

// isLetter reports whether ch is an ASCII letter: the parsers' names
// and access letters are runs of them. Anything else, a blank included,
// ends one (VMS 7.1 refused "System: RWED" at its blank, though the VSI
// manual says spacing is ignored: testdata/lib55).
func isLetter(ch byte) bool {
	return upper(ch) >= 'A' && upper(ch) <= 'Z'
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
// status, a warning); protection-length gets the whole text's length,
// even when it was cut (VMS 7.1). LIB$_WRONUMARG for more than seven
// arguments (unconfirmed); LIB$_INVARG when protection-mask or
// protection-string is omitted, fewer arguments included (VMS 7.1 with
// five).
func libFormatSogwProt(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) > 7 {
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

	_, truncated, err := writeString(env, dest, text)
	if err != nil {
		return stringFailure(env, err)
	}

	if err := storeOptionalWord(env, arg(argv, 6), uint16(len(text))); err != nil {
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
			if mask>>(c*categoryBits+bit)&1 == 0 {
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

// protectionError is a parse failure at offset pos of the string, with
// its status: LIB$_SYNTAXERR for a malformed string, LIB$_UNRKEY for a
// name that's no category's, LIB$_INVARG for a category named twice or
// a bad letter in LIB$PARSE_ACCESS_CODE (VMS 7.1's statuses:
// testdata/lib55).
type protectionError struct {
	pos    int
	status uint32
}

// letterRun returns the end of the run of letters in s starting at i.
func letterRun(s string, i int) int {
	for i < len(s) && isLetter(s[i]) {
		i++
	}

	return i
}

// accessBits returns the access mask the letters of word grant (bit n
// for the access whose letter is letters[n], case blind), and false if
// one of them is no access's letter.
func accessBits(word string, letters [categoryBits]byte) (uint16, bool) {
	var granted uint16

	for i := range len(word) {
		bit := -1

		for n, letter := range letters {
			if upper(word[i]) == letter {
				bit = n

				break
			}
		}

		if bit < 0 {
			return 0, false
		}

		granted |= 1 << bit
	}

	return granted, true
}

// parseProtection is PARSE_SOGW_PROT's work on s, read as VMS 7.1 read
// it (testdata/lib55): items separated by commas, each a category's name
// (letters, any abbreviation, case blind), then, optionally, ":" or "="
// and a run of access letters. It returns the protection mask, a set bit
// for each access denied, every category not named denied everything,
// and the ownership mask, each named category's four bits set. An empty
// string names nothing; a comma at the end is accepted.
//
// The errors, and where end-position puts them: a name that's no
// category's is LIB$_UNRKEY at the name; a category named twice
// LIB$_INVARG at the second name; access letters not all some access's
// LIB$_SYNTAXERR at the first of them; and anything else out of place (a
// blank, an empty item, a stray character) LIB$_SYNTAXERR at it.
func parseProtection(s string, letters [categoryBits]byte) (protection, ownership uint16, perr *protectionError) {
	protection = protectionAll
	i := 0

	for i < len(s) {
		end := letterRun(s, i)
		if end == i {
			return 0, 0, &protectionError{i, libSyntaxErr}
		}

		c := matchOwnership(strings.ToUpper(s[i:end]))
		if c < 0 {
			return 0, 0, &protectionError{i, libUnrKey}
		}

		shift := c * categoryBits
		nibble := uint16(1<<categoryBits-1) << shift

		if ownership&nibble != 0 {
			return 0, 0, &protectionError{i, libInvArg}
		}

		ownership |= nibble
		i = end

		if i < len(s) && (s[i] == ':' || s[i] == '=') {
			start := i + 1
			i = letterRun(s, start)

			granted, ok := accessBits(s[start:i], letters)
			if !ok {
				return 0, 0, &protectionError{start, libSyntaxErr}
			}

			protection &^= granted << shift
		}

		if i == len(s) {
			break
		}

		if s[i] != ',' {
			return 0, 0, &protectionError{i, libSyntaxErr}
		}

		i++ // the comma
	}

	return protection, ownership, nil
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
// It reads items such as "SYSTEM=RWED,OWNER:RWED,GROUP,WORLD:R"
// (parseProtection): protection-mask gets the mask (a set bit is an
// access denied; a category not named is denied everything) and
// ownership-mask the categories named. end-position gets the characters
// processed, the whole string, or on an error the offset parseProtection
// gives; an error writes neither mask. LIB$_WRONUMARG for more than five
// arguments (unconfirmed); LIB$_INVARG when a mask's address is omitted,
// fewer arguments included (VMS 7.1 with three).
func libParseSogwProt(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) > 5 {
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

		return perr.status, nil
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
// It reads access letters ("RWE", case blind) and puts the accesses they
// grant, a set bit meaning granted ("the opposite of the standard
// protection format"), into access-mask where ownership-category's bits
// are, leaving its other bits as they were: the mask becomes
// (mask AND NOT category) OR (granted shifted to category's lowest set
// bit), so RWE for Group (0x0F00) puts 7 in bits 8-11. That is what VMS
// 7.1 did (testdata/lib55), for any category word: 0x00FF clears the low
// byte, and 0 places the bits at bit 16, past the word, which VMS wrote
// into the word after it. A character that isn't an access's letter (a
// blank, a comma, a letter beyond the four) is LIB$_INVARG with the mask
// unchanged, end-position getting its offset; on success end-position is
// the characters processed. LIB$_WRONUMARG for more than five arguments
// (unconfirmed); LIB$_INVARG when ownership-category or access-mask is
// omitted.
func libParseAccessCode(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) > 5 {
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

	s, err := readString(env, arg(argv, 0))
	if err != nil {
		return stringFailure(env, err)
	}

	letters, err := accessLetters(env, arg(argv, 1))
	if err != nil {
		return stringFailure(env, err)
	}

	for i := range len(s) {
		if _, ok := accessBits(s[i:i+1], letters); !ok {
			if err := storeOptionalWord(env, endAddr, uint16(i)); err != nil {
				return memoryFault(env, writing(err))
			}

			return libInvArg, nil
		}
	}

	granted, _ := accessBits(s, letters)

	old, err := mem.LoadWord(cpu, maskAddr)
	if err != nil {
		return memoryFault(env, err)
	}

	// The category's lowest set bit; 16 when it has none.
	shift := 16

	for bit := range 16 {
		if category>>bit&1 != 0 {
			shift = bit

			break
		}
	}

	result := uint32(old&^category) | uint32(granted)<<shift

	if err := mem.StoreWord(cpu, maskAddr, uint16(result)); err != nil {
		return memoryFault(env, writing(err))
	}

	if high := uint16(result >> 16); high != 0 {
		if err := mem.StoreWord(cpu, maskAddr+2, high); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if err := storeOptionalWord(env, endAddr, uint16(len(s))); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// fileClassNames is the FILE class's whole access-name table, as VMS
// 7.1's LIB$GET_ACCNAM returned it (testdata/lib55): bits 0 to 4 named,
// the other 27 entries empty.
var fileClassNames = []string{"READ", "WRITE", "EXECUTE", "DELETE", "CONTROL"}

// accessNameEntries is how many descriptors an access-name table has,
// one per bit of an access mask.
const accessNameEntries = 32

// accnamKey is the corevms.Environment.ImageData key of the image's
// FILE access-name table's address (0 until it's made).
type accnamKey struct{}

// libGetAccnam is LIB$GET_ACCNAM:
//
//	LIB$GET_ACCNAM [clsnam.rt.dx] ,[objnam.rt.dx] ,accnam.wlu.r
//
// It writes to accnam the address of the access-name table of the
// protected object class clsnam names (FILE when omitted): a vector of
// 32 class S descriptors, one per bit of an access mask. govax has the
// FILE class's only, which it lays out on the heap the first time the
// image asks (fileClassNames); another class is SS$_NOCLASS. objnam,
// which names an object of the class, isn't looked at (VMS checks that
// it exists; unconfirmed how). The 7.x routine is a simplified interface
// to $GET_SECURITY (the VSI manual), which govax lacks.
// LIB$_WRONUMARG for fewer than three arguments; LIB$_INVARG when
// accnam is omitted.
func libGetAccnam(env *corevms.Environment, argv []uint32) (uint32, error) {
	if tooFewArgs(argv, 3) || len(argv) > 3 {
		return libWroNumArg, nil
	}

	out := arg(argv, 2)
	if out == 0 {
		return libInvArg, nil
	}

	class, err := optionalText(env, arg(argv, 0), "FILE")
	if err != nil {
		return stringFailure(env, err)
	}

	if !strings.EqualFold(strings.TrimSpace(class), "FILE") {
		return ssNoClass, nil
	}

	table := env.ImageData(accnamKey{}, func() any { return new(uint32) }).(*uint32)
	if *table == 0 {
		if *table, err = buildAccessNameTable(env); err != nil {
			return memoryFault(env, writing(err))
		}

		if *table == 0 {
			return libInsVirMem, nil
		}
	}

	if err := env.Memory().StoreLongword(env.CPU(), out, *table); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// buildAccessNameTable lays the FILE access-name table out in a heap
// block: 32 descriptors, then the names' text. It returns 0 when the
// heap has no room.
func buildAccessNameTable(env *corevms.Environment) (uint32, error) {
	size := uint32(accessNameEntries * descriptorSize)
	for _, name := range fileClassNames {
		size += uint32(len(name))
	}

	table, err := env.AllocateVM(size, 0)
	if err != nil || table == 0 {
		return 0, err
	}

	mem, cpu := env.Memory(), env.CPU()
	text := table + accessNameEntries*descriptorSize

	for i := range accessNameEntries {
		var name string
		if i < len(fileClassNames) {
			name = fileClassNames[i]
		}

		desc := table + uint32(i)*descriptorSize
		ptr := uint32(0)

		if name != "" {
			ptr = text
		}

		if err := mem.StoreLongword(cpu, desc, uint32(len(name))|dscDTypeT<<16|dscClassS<<24); err != nil {
			return 0, err
		}

		if err := mem.StoreLongword(cpu, desc+4, ptr); err != nil {
			return 0, err
		}

		for j := range len(name) {
			if err := mem.StoreByte(cpu, text+uint32(j), name[j]); err != nil {
				return 0, err
			}
		}

		text += uint32(len(name))
	}

	return table, nil
}
