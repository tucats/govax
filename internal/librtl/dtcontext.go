package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmsdef"
)

// The date and time formatting context (docs/PHASE-56, subtask 5; the
// survey's "Formatting: the logical names and the context"). Every
// formatting routine (LIB$FORMAT_DATE_TIME, LIB$CONVERT_DATE_STRING,
// LIB$GET_DATE_FORMAT, LIB$GET_MAXIMUM_DATE_LENGTH) takes an optional
// user-context argument, the address of a longword that is 0 at first.
// The first call allocates a context and writes its address there; later
// calls find the same context through it. Without the argument a routine
// uses the image's one static context.
//
// A context holds what the routines format and parse by: the language's
// spellings (month and weekday names, AM and PM, ...), the output format
// (a date part and a time part, in the order they are written), and the
// input format. These come from one of two places, never both:
//
//   - LIB$INIT_DATE_TIME_CONTEXT, one component a call. A context so
//     initialized uses only what it was given: a spelling it lacks is
//     its language's (LIB$K_LANGUAGE's, else English), and a format it
//     lacks is VMS's standard one (unconfirmed, both).
//   - Otherwise, the logical names: SYS$LANGUAGE names the language,
//     LIB$DT_FORMAT the output format's parts (names in
//     LNM$DT_FORMAT_TABLE), and LIB$DT_INPUT_FORMAT the input format.
//     They are read on the context's first use and kept (unconfirmed:
//     VMS may read them on every call). A name that doesn't translate
//     falls back to English (LIB$_ENGLUSED) or the standard format
//     (LIB$_DEFFORUSE), as the manual's section 3.3.2.2 says.
//
// The context's block is on the heap (LIB$GET_VM's, so it's freed with
// the image), holding a tag that tells it from other memory; the rest of
// it is kept in Go, per image (corevms.Environment.ImageData), found by
// the block's address. VMS doesn't document its block.

var (
	libIllComponent = vmsdef.LibrarySymbols["LIB$_ILLCOMPONENT"]
	libIllIniStr    = vmsdef.LibrarySymbols["LIB$_ILLINISTR"]
	libNumElements  = vmsdef.LibrarySymbols["LIB$_NUMELEMENTS"]
	libUnrForCod    = vmsdef.LibrarySymbols["LIB$_UNRFORCOD"]
	libIllFormat    = vmsdef.LibrarySymbols["LIB$_ILLFORMAT"]
	libDefForUse    = vmsdef.LibrarySymbols["LIB$_DEFFORUSE"]
)

// The components LIB$INIT_DATE_TIME_CONTEXT sets, by their LIB$K_ codes.
var (
	componentMonthName    = int32(vmsdef.LibrarySymbols["LIB$K_MONTH_NAME"])
	componentOutputFormat = int32(vmsdef.LibrarySymbols["LIB$K_OUTPUT_FORMAT"])
	componentInputFormat  = int32(vmsdef.LibrarySymbols["LIB$K_INPUT_FORMAT"])
	componentMonthAbb     = int32(vmsdef.LibrarySymbols["LIB$K_MONTH_NAME_ABB"])
	componentLanguage     = int32(vmsdef.LibrarySymbols["LIB$K_LANGUAGE"])
	componentWeekday      = int32(vmsdef.LibrarySymbols["LIB$K_WEEKDAY_NAME"])
	componentWeekdayAbb   = int32(vmsdef.LibrarySymbols["LIB$K_WEEKDAY_NAME_ABB"])
	componentMeridiem     = int32(vmsdef.LibrarySymbols["LIB$K_MERIDIEM_INDICATOR"])
	componentRelativeDay  = int32(vmsdef.LibrarySymbols["LIB$K_RELATIVE_DAY_NAME"])
	componentMnemonics    = int32(vmsdef.LibrarySymbols["LIB$K_FORMAT_MNEMONICS"])
)

// dtComponent describes one component: how many elements its init-string
// has, and, for a spelled one, the language's spellings. A spelled
// component's logical name in a language's table (LNM$LANGUAGE_name) is
// its langName with _U, _L, or _C for the three cases, or langName alone
// when the spellings have no case (the legible mnemonics).
type dtComponent struct {
	elements int
	spelled  bool
	langName string
	cased    bool
	english  []string // English's spellings, in upper case
}

// dtComponents is every component, by code. The codes between them (5,
// 7, 8, 10, ...) and past them have no names, and are LIB$_ILLCOMPONENT
// (unconfirmed). LIB$K_LANGUAGE is the VSI manual's, not the 5.0
// manual's; VMS 7.3's definitions have it.
var dtComponents = map[int32]*dtComponent{
	componentOutputFormat: {elements: 2},
	componentInputFormat:  {elements: 1},
	componentLanguage:     {elements: 1},
	componentMonthName: {
		elements: 12, spelled: true, langName: "LIB$MONTHS", cased: true,
		english: []string{
			"JANUARY", "FEBRUARY", "MARCH", "APRIL", "MAY", "JUNE",
			"JULY", "AUGUST", "SEPTEMBER", "OCTOBER", "NOVEMBER", "DECEMBER",
		},
	},
	componentMonthAbb: {
		elements: 12, spelled: true, langName: "LIB$MONTH_ABBREVIATIONS", cased: true,
		english: []string{
			"JAN", "FEB", "MAR", "APR", "MAY", "JUN",
			"JUL", "AUG", "SEP", "OCT", "NOV", "DEC",
		},
	},
	componentWeekday: {
		elements: 7, spelled: true, langName: "LIB$WEEKDAYS", cased: true,
		english: []string{"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY", "SATURDAY", "SUNDAY"},
	},
	componentWeekdayAbb: {
		elements: 7, spelled: true, langName: "LIB$WEEKDAY_ABBREVIATIONS", cased: true,
		english: []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"},
	},
	componentMeridiem: {
		elements: 2, spelled: true, langName: "LIB$MI", cased: true,
		english: []string{"AM", "PM"},
	},
	componentRelativeDay: {
		elements: 3, spelled: true, langName: "LIB$RELATIVE_DAYS", cased: true,
		english: []string{"YESTERDAY", "TODAY", "TOMORROW"},
	},
	// The legible mnemonics LIB$GET_DATE_FORMAT shows (the manual's Table
	// 3-5): year, numeric month, day, hours, minutes, seconds, fraction,
	// meridiem, and alphabetic month.
	componentMnemonics: {
		elements: 9, spelled: true, langName: "LIB$FORMAT_MNEMONICS",
		english: []string{"YYYY", "MM", "DD", "HH", "MM", "SS", "CC", "AM/PM", "MONTH"},
	},
}

// english is the language whose spellings are built in: the manual says
// English's "are not taken from logical name translations, but rather
// are looked up in an internal table".
const english = "ENGLISH"

// The standard formats, VMS's own ($ASCTIM's text): the predefined date
// format 001 and time format 001, the date first. The input format is
// the same pair.
var (
	standardDateFormat  = lnm.DateFormats[0]
	standardTimeFormat  = lnm.TimeFormats[0]
	standardInputFormat = standardDateFormat + " " + standardTimeFormat
)

// dtElement is one element of an output format: its text, parsed, and
// the part it gives.
type dtElement struct {
	text  string
	items []dtItem
	part  dtPart
}

// newElement parses an output format's element. Its status is
// LIB$_UNRFORCOD for an unknown mnemonic, and LIB$_ILLFORMAT for an
// element that gives no part, or both (unconfirmed: the manual gives no
// status for either).
func newElement(text string) (dtElement, uint32) {
	items, ok := parseDateFormat(text)
	if !ok {
		return dtElement{}, libUnrForCod
	}

	e := dtElement{text: text, items: items, part: partOf(items)}
	if e.part == partNone || e.part == partMixed {
		return dtElement{}, libIllFormat
	}

	return e, ssNormal
}

// dtSettings is what a context formats and parses by, wherever it came
// from: each spelled component's spellings in the three cases (indexed
// by dtCase), the output format's elements in order, and the input
// format. A warning status is kept for the language and for each format
// that fell back (0 when none did).
type dtSettings struct {
	spellings map[int32][caseCount][]string
	output    []dtElement
	input     []dtItem
	inputText string

	languageStatus, outputStatus, inputStatus uint32
}

// outputWarning is the status a routine that writes by these settings
// returns: LIB$_ENGLUSED when the language fell back to English, else
// LIB$_DEFFORUSE when the output format did, else SS$_NORMAL. Which wins
// when both fall back is unconfirmed.
func (s *dtSettings) outputWarning() uint32 {
	return s.warning(s.outputStatus)
}

// inputWarning is outputWarning for a routine that reads by the input
// format.
func (s *dtSettings) inputWarning() uint32 {
	return s.warning(s.inputStatus)
}

func (s *dtSettings) warning(format uint32) uint32 {
	switch {
	case s.languageStatus != 0:
		return s.languageStatus
	case format != 0:
		return format
	}

	return ssNormal
}

// dateContext is one context's state in Go. initialized is set once
// LIB$INIT_DATE_TIME_CONTEXT has given it a component, after which the
// logical names aren't read. resolved caches the settings it gives.
type dateContext struct {
	initialized bool

	language  string             // LIB$K_LANGUAGE's, upper case
	spelled   map[int32][]string // INIT's spellings, as given
	output    []dtElement        // INIT's output format (nil: none given)
	inputText string             // INIT's input format ("": none given)
	input     []dtItem

	resolved *dtSettings
}

// settings is what the context formats and parses by, worked out on its
// first use and kept until INIT changes it.
func (c *dateContext) settings(env *corevms.Environment) *dtSettings {
	if c.resolved == nil {
		if c.initialized {
			c.resolved = c.initializedSettings(env)
		} else {
			c.resolved = namedSettings(env)
		}
	}

	return c.resolved
}

// initializedSettings are an initialized context's settings: INIT's
// components, its language's spellings for the rest, and the standard
// formats for a format it wasn't given (with LIB$_DEFFORUSE,
// unconfirmed).
func (c *dateContext) initializedSettings(env *corevms.Environment) *dtSettings {
	language := c.language
	if language == "" {
		language = english
	}

	s := languageSettings(env, language)

	for code, given := range c.spelled {
		s.spellings[code] = casedSpellings(given)
	}

	if c.output != nil {
		s.output = c.output
	} else {
		s.output, s.outputStatus = standardOutput(), libDefForUse
	}

	if c.inputText != "" {
		s.input, s.inputText = c.input, c.inputText
	} else {
		s.input, s.inputText, s.inputStatus = standardInput(), standardInputFormat, libDefForUse
	}

	return s
}

// namedSettings are the settings the logical names give (the manual's
// section 3.3.2.2): SYS$LANGUAGE's language, LIB$DT_FORMAT's output
// format, and LIB$DT_INPUT_FORMAT's input format. The user's names are
// translated through LNM$FILE_DEV, as LIB$GET_USERS_LANGUAGE translates
// SYS$LANGUAGE (unconfirmed which tables VMS searches).
func namedSettings(env *corevms.Environment) *dtSettings {
	language, ok := userName(env, "SYS$LANGUAGE")

	var s *dtSettings

	if ok {
		s = languageSettings(env, strings.ToUpper(language[0]))
	} else {
		s = languageSettings(env, english)
		s.languageStatus = libEnglUsed
	}

	s.output, s.outputStatus = namedOutput(env)

	if input, ok := userName(env, "LIB$DT_INPUT_FORMAT"); ok {
		if items, ok := parseDateFormat(input[0]); ok {
			s.input, s.inputText = items, input[0]

			return s
		}
	}

	s.input, s.inputText, s.inputStatus = standardInput(), standardInputFormat, libDefForUse

	return s
}

// namedOutput is the output format LIB$DT_FORMAT gives: each of its
// equivalence strings names a format in LNM$DT_FORMAT_TABLE
// (LIB$DATE_FORMAT_006, LIB$TIME_FORMAT_012), executive mode, whose
// element comes out in the list's order. A name that doesn't translate,
// or a format that doesn't parse or gives no single part, is left out
// with LIB$_DEFFORUSE (unconfirmed for the format; the manual speaks of
// translations), and so is a second element of a part already given
// (unconfirmed). Without LIB$DT_FORMAT, or with nothing left, it is the
// standard format, with LIB$_DEFFORUSE.
func namedOutput(env *corevms.Environment) ([]dtElement, uint32) {
	names, ok := userName(env, "LIB$DT_FORMAT")
	if !ok {
		return standardOutput(), libDefForUse
	}

	var (
		out    []dtElement
		status uint32
	)

	for _, name := range names {
		format, ok := translateIn(env, lnm.DTFormatTableName, name)
		if !ok {
			status = libDefForUse

			continue
		}

		e, st := newElement(format[0])
		if st != ssNormal || hasPart(out, e.part) {
			status = libDefForUse

			continue
		}

		out = append(out, e)
	}

	if len(out) == 0 {
		return standardOutput(), libDefForUse
	}

	return out, status
}

// hasPart reports whether one of elements gives part.
func hasPart(elements []dtElement, part dtPart) bool {
	for _, e := range elements {
		if e.part == part {
			return true
		}
	}

	return false
}

// standardOutput is the standard output format's two elements.
func standardOutput() []dtElement {
	date, _ := newElement(standardDateFormat)
	time, _ := newElement(standardTimeFormat)

	return []dtElement{date, time}
}

// standardInput is the standard input format, parsed.
func standardInput() []dtItem {
	items, _ := parseDateFormat(standardInputFormat)

	return items
}

// languageSettings are settings holding language's spellings for every
// spelled component: English's built-in ones, or those of the language's
// table, LNM$LANGUAGE_language. A component whose names that table lacks
// (or has with the wrong number of strings) is English's, with
// LIB$_ENGLUSED (the manual: "any logical name relating to text").
func languageSettings(env *corevms.Environment, language string) *dtSettings {
	s := &dtSettings{spellings: map[int32][caseCount][]string{}}

	for code, c := range dtComponents {
		if !c.spelled {
			continue
		}

		if language != english {
			if sp, ok := tableSpellings(env, "LNM$LANGUAGE_"+language, c); ok {
				s.spellings[code] = sp

				continue
			}

			s.languageStatus = libEnglUsed
		}

		s.spellings[code] = casedSpellings(c.english)
	}

	return s
}

// tableSpellings reads a spelled component's names from a language's
// table, executive mode: one per case, or one alone for an uncased
// component. Each must have the component's number of strings.
func tableSpellings(env *corevms.Environment, table string, c *dtComponent) ([caseCount][]string, bool) {
	var out [caseCount][]string

	if !c.cased {
		values, ok := translateIn(env, table, c.langName)
		if !ok || len(values) != c.elements {
			return out, false
		}

		for i := range out {
			out[i] = values
		}

		return out, true
	}

	for i, suffix := range []string{"_U", "_L", "_C"} {
		values, ok := translateIn(env, table, c.langName+suffix)
		if !ok || len(values) != c.elements {
			return out, false
		}

		out[i] = values
	}

	return out, true
}

// casedSpellings makes the three cases from one set of spellings: upper,
// lower, and capitalized (the first letter upper, the rest lower). INIT
// gives a component's spellings once, and the format's mnemonics choose
// the case; that VMS derives the cases this way is unconfirmed.
func casedSpellings(given []string) [caseCount][]string {
	var out [caseCount][]string

	for _, s := range given {
		lower := strings.ToLower(s)

		out[caseUpper] = append(out[caseUpper], strings.ToUpper(s))
		out[caseLower] = append(out[caseLower], lower)
		out[caseCapital] = append(out[caseCapital], capitalized(lower))
	}

	return out
}

// capitalized is s with its first letter in upper case.
func capitalized(s string) string {
	if s == "" {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
}

// userName translates one of the user's names through LNM$FILE_DEV, at
// any mode, giving its equivalence strings.
func userName(env *corevms.Environment, name string) ([]string, bool) {
	return translateAt(env, lnm.FileDevName, name, lnm.User)
}

// translateIn translates a name that must be executive mode in table.
func translateIn(env *corevms.Environment, table, name string) ([]string, bool) {
	return translateAt(env, table, name, lnm.Executive)
}

// translateAt is $TRNLNM's lookup of name in table at mode, giving the
// name's equivalence strings; ok is false when it has none.
func translateAt(env *corevms.Environment, table, name string, mode lnm.Mode) ([]string, bool) {
	db := env.Logicals
	if db == nil {
		return nil, false
	}

	e, err := db.Translate(table, name, mode, 0)
	if err != nil || len(e.Equivalences) == 0 {
		return nil, false
	}

	values := make([]string, len(e.Equivalences))
	for i, q := range e.Equivalences {
		values[i] = q.Value
	}

	return values, true
}

// dtContextTag is a context block's first longword, "DTCX" in ASCII: how
// the routines tell a context from other memory.
const dtContextTag = 0x58435444

// dtContextSize is a context block's size: the tag and a reserved
// longword.
const dtContextSize = 8

// dtContexts is one image's contexts: the static one (nil until first
// used) and the heap ones, by their blocks' addresses.
type dtContexts struct {
	static *dateContext
	heap   map[uint32]*dateContext
}

// dtContextsKey is the corevms.Environment.ImageData key of the image's
// dtContexts.
type dtContextsKey struct{}

// imageContexts returns the running image's contexts.
func imageContexts(env *corevms.Environment) *dtContexts {
	return env.ImageData(dtContextsKey{}, func() any {
		return &dtContexts{heap: map[uint32]*dateContext{}}
	}).(*dtContexts)
}

// dateContextFor finds the context a user-context argument names: the
// static one when cell (the argument) is 0; a new one, its block's
// address written to the longword at cell, when that longword is 0; else
// the context whose block it holds. Its status is SS$_NORMAL,
// LIB$_INSVIRMEM with no room for a new block, or LIB$_INVARG when the
// longword holds the address of no context (unconfirmed). err is a
// memory error, which the caller signals.
func dateContextFor(env *corevms.Environment, cell uint32) (*dateContext, uint32, error) {
	all := imageContexts(env)

	if cell == 0 {
		if all.static == nil {
			all.static = &dateContext{}
		}

		return all.static, ssNormal, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	block, err := mem.LoadLongword(cpu, cell)
	if err != nil {
		return nil, 0, err
	}

	if block != 0 {
		tag, err := mem.LoadLongword(cpu, block)
		if err != nil {
			return nil, 0, err
		}

		if c, ok := all.heap[block]; ok && tag == dtContextTag {
			return c, ssNormal, nil
		}

		return nil, libInvArg, nil
	}

	block, err = env.AllocateVM(dtContextSize, 0)
	if err != nil {
		return nil, 0, err
	}

	if block == 0 {
		return nil, libInsVirMem, nil
	}

	for _, w := range []struct{ addr, v uint32 }{{block, dtContextTag}, {block + 4, 0}, {cell, block}} {
		if err := mem.StoreLongword(cpu, w.addr, w.v); err != nil {
			env.FreeVM(block)

			return nil, 0, writing(err)
		}
	}

	c := &dateContext{}
	all.heap[block] = c

	return c, ssNormal, nil
}

// splitInitString splits an init-string into its elements: its first
// character is the delimiter, which must end it too, and which separates
// the elements between ("|JAN|FEB|...|DEC|"). ok is false
// (LIB$_ILLINISTR) for a string that doesn't end with its delimiter, or
// is too short to have one at each end (an empty string, or one
// character; unconfirmed).
func splitInitString(s string) (elements []string, ok bool) {
	if len(s) < 2 || s[len(s)-1] != s[0] {
		return nil, false
	}

	return strings.Split(s[1:len(s)-1], s[:1]), true
}

// libInitDateTimeContext is LIB$INIT_DATE_TIME_CONTEXT (LIB-231):
//
//	LIB$INIT_DATE_TIME_CONTEXT user-context.mlu.r, component.rl.r,
//	                           init-string.rt.dx
//
// It sets one component of the context user-context names
// (dateContextFor), allocating the context if the cell holds 0, from the
// init-string's elements (splitInitString). After it the context no
// longer reads the logical names. Its statuses:
//
//   - LIB$_ILLCOMPONENT for a component with no name (dtComponents);
//   - LIB$_ILLINISTR for a badly formed init-string, and LIB$_NUMELEMENTS
//     for the wrong number of elements;
//   - LIB$_UNRFORCOD for an unknown mnemonic in a format;
//   - LIB$_ILLFORMAT for an output format element that gives no part or
//     both, or two of one part (unconfirmed; an empty element is a part
//     left out);
//   - LIB$_INVSTRDES for a bad descriptor, and LIB$_INSVIRMEM or
//     LIB$_INVARG from finding the context.
//
// The init-string is checked before the context is touched, so a failed
// call allocates nothing (unconfirmed). An input format's fields are
// checked when it's used (LIB$_ILLFORMAT there). Without a user-context
// it sets the static context (unconfirmed: the manual requires the
// argument).
func libInitDateTimeContext(env *corevms.Environment, argv []uint32) (uint32, error) {
	code, err := readArg(env, arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	component, ok := dtComponents[code]
	if !ok {
		return libIllComponent, nil
	}

	text, err := readString(env, arg(argv, 2))
	if err != nil {
		return stringFailure(env, err)
	}

	elements, ok := splitInitString(text)
	if !ok {
		return libIllIniStr, nil
	}

	if len(elements) != component.elements {
		return libNumElements, nil
	}

	apply, status := initComponent(code, elements)
	if status != ssNormal {
		return status, nil
	}

	c, status, err := dateContextFor(env, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if status != ssNormal {
		return status, nil
	}

	apply(c)

	c.initialized = true
	c.resolved = nil

	return ssNormal, nil
}

// initComponent checks one component's elements and returns the change
// to make to a context, or the status of what's wrong with them.
func initComponent(code int32, elements []string) (func(*dateContext), uint32) {
	switch code {
	case componentOutputFormat:
		var out []dtElement

		for _, text := range elements {
			if text == "" {
				continue
			}

			e, status := newElement(text)
			if status != ssNormal {
				return nil, status
			}

			if hasPart(out, e.part) {
				return nil, libIllFormat
			}

			out = append(out, e)
		}

		if len(out) == 0 {
			return nil, libIllFormat
		}

		return func(c *dateContext) { c.output = out }, ssNormal

	case componentInputFormat:
		items, ok := parseDateFormat(elements[0])
		if !ok {
			return nil, libUnrForCod
		}

		if len(items) == 0 {
			return nil, libIllFormat
		}

		return func(c *dateContext) { c.input, c.inputText = items, elements[0] }, ssNormal

	case componentLanguage:
		language := strings.ToUpper(strings.TrimSpace(elements[0]))
		if language == "" {
			return nil, libIllIniStr
		}

		return func(c *dateContext) { c.language = language }, ssNormal
	}

	return func(c *dateContext) {
		if c.spelled == nil {
			c.spelled = map[int32][]string{}
		}

		c.spelled[code] = elements
	}, ssNormal
}

// libFreeDateTimeContext is LIB$FREE_DATE_TIME_CONTEXT (LIB-172):
//
//	LIB$FREE_DATE_TIME_CONTEXT [user-context.mlu.r]
//
// It frees the context user-context names, and zeroes the cell, or the
// static context without the argument; image exit frees them all anyway.
// A cell holding 0 frees nothing (unconfirmed). A cell holding the
// address of no context, or of a block the heap no longer has, is
// LIB$_BADBLOADR, LIB$FREE_VM's status (unconfirmed); the block's tag is
// cleared first, so a copy of the cell no longer names a context.
func libFreeDateTimeContext(env *corevms.Environment, argv []uint32) (uint32, error) {
	all := imageContexts(env)

	cell := arg(argv, 0)
	if cell == 0 {
		all.static = nil

		return ssNormal, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	block, err := mem.LoadLongword(cpu, cell)
	if err != nil {
		return memoryFault(env, err)
	}

	if block == 0 {
		return ssNormal, nil
	}

	tag, err := mem.LoadLongword(cpu, block)
	if err != nil {
		return memoryFault(env, err)
	}

	if _, ok := all.heap[block]; !ok || tag != dtContextTag {
		return libBadBlockAddress, nil
	}

	delete(all.heap, block)

	if err := mem.StoreLongword(cpu, block, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, cell, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	if !env.FreeVM(block) {
		return libBadBlockAddress, nil
	}

	return ssNormal, nil
}
