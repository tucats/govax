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
// input format. Each comes from LIB$INIT_DATE_TIME_CONTEXT, one
// component a call, or, for each component it wasn't given, from the
// logical names, as VMS V7.1 does (testdata/lib56; the 5.0 manual says
// a context is meant to have one or the other): SYS$LANGUAGE (or
// LIB$K_LANGUAGE) names the language, LIB$DT_FORMAT the output format's
// elements (names in LNM$DT_FORMAT_TABLE), and LIB$DT_INPUT_FORMAT the
// input format. They are read on the context's first use and kept
// (unconfirmed: VMS may read them on every call). A name that doesn't
// translate falls back to English (LIB$_ENGLUSED) or the standard
// format (LIB$_DEFFORUSE), as the manual's section 3.3.2.2 says.
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
	libEleTooLon    = vmsdef.LibrarySymbols["LIB$_ELETOOLON"]
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
// the part it stands for (slotOf).
type dtElement struct {
	text  string
	items []dtItem
	slot  dtPart
}

// buildOutput parses an output format's elements (at most two; more are
// ignored, as VMS V7.1 ignores a third name in LIB$DT_FORMAT) and gives
// each its part: the first element's by its first field, and the second
// the other part, whatever its fields (testdata/lib56: "|!D0|!MN0|" is a
// date part, the day, and a time part, the month). Its status is
// LIB$_UNRFORCOD for an unknown mnemonic.
func buildOutput(texts []string) ([]dtElement, uint32) {
	if len(texts) > 2 {
		texts = texts[:2]
	}

	out := make([]dtElement, len(texts))

	for i, text := range texts {
		items, ok := parseDateFormat(text)
		if !ok {
			return nil, libUnrForCod
		}

		out[i] = dtElement{text: text, items: items}
	}

	if len(out) > 0 {
		out[0].slot = slotOf(out[0].items)
	}

	if len(out) > 1 {
		out[1].slot = partDate
		if out[0].slot == partDate {
			out[1].slot = partTime
		}
	}

	return out, ssNormal
}

// dtSettings is what a context formats and parses by, wherever it came
// from: each spelled component's spellings in the three cases (indexed
// by dtCase), the output format's elements in order, and the input
// format. A warning status is kept for the language and for each format
// that fell back (0 when none did), and an error status for a format
// that can't be used (LIB$_UNRFORCOD, LIB$_ELETOOLON; 0 when it can).
type dtSettings struct {
	spellings map[int32][caseCount][]string
	output    []dtElement
	input     []dtItem
	inputText string

	languageStatus, outputStatus, inputStatus uint32
	outputError, inputError                   uint32
}

// outputWarning is the status a routine that writes by these settings
// returns: LIB$_ENGLUSED when the language fell back to English, else
// LIB$_DEFFORUSE when the output format did, else SS$_NORMAL. ENGLUSED
// wins when both fall back, as on VMS V7.1.
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

// dateContext is one context's state in Go: what LIB$INIT_DATE_TIME_CONTEXT
// gave it, a component at a time. What it wasn't given comes from the
// logical names, as an uninitialized context's all does (VMS V7.1:
// testdata/lib56). resolved caches the settings it gives.
type dateContext struct {
	language    string             // LIB$K_LANGUAGE's, upper case
	hasLanguage bool               // LIB$K_LANGUAGE was given
	spelled     map[int32][]string // INIT's spellings, as given
	output      []string           // INIT's output format's elements (nil: none given)
	hasInput    bool               // INIT gave an input format
	inputText   string
	malformed   map[int32]bool // an input format given as one character, LIB$_ELETOOLON when used

	resolved *dtSettings
}

// settings is what the context formats and parses by, worked out on its
// first use and kept until INIT changes it (unconfirmed: VMS may read
// the names on every call).
func (c *dateContext) settings(env *corevms.Environment) *dtSettings {
	if c.resolved == nil {
		c.resolved = c.resolve(env)
	}

	return c.resolved
}

// resolve works out the settings: the language's spellings (LIB$K_LANGUAGE's
// language when INIT gave one, else SYS$LANGUAGE's), INIT's spellings over
// them, used as given in every case (VMS V7.1 writes "Janvier" for !MAU,
// !MAL, and !MAC alike), and INIT's formats, or the logical names'.
func (c *dateContext) resolve(env *corevms.Environment) *dtSettings {
	var s *dtSettings

	if c.hasLanguage {
		s = languageSettings(env, c.language)
	} else {
		s = userLanguageSettings(env)
	}

	for code, given := range c.spelled {
		var all [caseCount][]string
		for i := range all {
			all[i] = given
		}

		s.spellings[code] = all
	}

	switch {
	case c.output != nil:
		s.output, s.outputError = buildOutput(c.output)
		s.outputError = errorOf(s.outputError)
	default:
		s.output, s.outputStatus, s.outputError = namedOutput(env)
	}

	switch {
	case c.malformed[componentInputFormat]:
		s.inputError = libEleTooLon
	case c.hasInput:
		s.inputText = c.inputText
		s.input, s.inputError = parseInputFormat(c.inputText)
		s.inputError = errorOf(s.inputError)
	default:
		s.inputText, s.input, s.inputStatus, s.inputError = namedInput(env)
	}

	return s
}

// errorOf is a status as dtSettings keeps an error: 0 for SS$_NORMAL.
func errorOf(status uint32) uint32 {
	if status == ssNormal {
		return 0
	}

	return status
}

// userLanguageSettings are the spellings of SYS$LANGUAGE's language,
// translated through LNM$FILE_DEV as LIB$GET_USERS_LANGUAGE translates
// it (unconfirmed which tables VMS searches); English, with
// LIB$_ENGLUSED, when it isn't defined.
func userLanguageSettings(env *corevms.Environment) *dtSettings {
	language, ok := userName(env, "SYS$LANGUAGE")
	if !ok {
		s := languageSettings(env, english)
		s.languageStatus = libEnglUsed

		return s
	}

	return languageSettings(env, strings.ToUpper(language[0]))
}

// namedInput is the input format LIB$DT_INPUT_FORMAT gives, a format
// string itself; the standard one, with LIB$_DEFFORUSE, when it isn't
// defined.
func namedInput(env *corevms.Environment) (text string, items []dtItem, status, err uint32) {
	input, ok := userName(env, "LIB$DT_INPUT_FORMAT")
	if !ok {
		return standardInputFormat, standardInput(), libDefForUse, 0
	}

	items, err = parseInputFormat(input[0])
	if err != ssNormal {
		return input[0], nil, 0, err
	}

	return input[0], items, 0, 0
}

// namedOutput is the output format LIB$DT_FORMAT gives: its first two
// equivalence strings each name a format in LNM$DT_FORMAT_TABLE
// (LIB$DATE_FORMAT_006, LIB$TIME_FORMAT_012), executive mode, and those
// formats are the elements, in the list's order (buildOutput). When
// LIB$DT_FORMAT isn't defined, or either name doesn't translate, it is
// the whole standard format, with LIB$_DEFFORUSE (VMS V7.1, given
// LIB$DATE_FORMAT_999 and LIB$TIME_FORMAT_002: testdata/lib56).
func namedOutput(env *corevms.Environment) (elements []dtElement, status, err uint32) {
	names, ok := userName(env, "LIB$DT_FORMAT")
	if !ok {
		return standardOutput(), libDefForUse, 0
	}

	if len(names) > 2 {
		names = names[:2]
	}

	var formats []string

	for _, name := range names {
		format, ok := translateIn(env, lnm.DTFormatTableName, name)
		if !ok {
			return standardOutput(), libDefForUse, 0
		}

		formats = append(formats, format[0])
	}

	elements, err = buildOutput(formats)
	if err != ssNormal {
		return nil, 0, err
	}

	return elements, 0, 0
}

// standardElement is the standard format's element for part.
func standardElement(part dtPart) dtElement {
	if part == partTime {
		return standardOutput()[1]
	}

	return standardOutput()[0]
}

// standardOutput is the standard output format's two elements.
func standardOutput() []dtElement {
	elements, _ := buildOutput([]string{standardDateFormat, standardTimeFormat})

	return elements
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

// dtElementMost is the longest element an init-string may have: longer
// is LIB$_ELETOOLON ("element too long, maximum 255 characters").
const dtElementMost = 255

// splitInitString splits an init-string into its elements: its first
// character is the delimiter, which must end it too, and which separates
// the elements between ("|JAN|FEB|...|DEC|"). Its status is
// LIB$_ILLINISTR for a string that doesn't end with its delimiter, and
// LIB$_ELETOOLON for an element longer than 255 bytes.
func splitInitString(s string) ([]string, uint32) {
	if s[len(s)-1] != s[0] {
		return nil, libIllIniStr
	}

	elements := strings.Split(s[1:len(s)-1], s[:1])
	for _, e := range elements {
		if len(e) > dtElementMost {
			return nil, libEleTooLon
		}
	}

	return elements, ssNormal
}

// libInitDateTimeContext is LIB$INIT_DATE_TIME_CONTEXT (LIB-231):
//
//	LIB$INIT_DATE_TIME_CONTEXT user-context.mlu.r, component.rl.r,
//	                           init-string.rt.dx
//
// It sets one component of the context user-context names
// (dateContextFor), from the init-string's elements (splitInitString).
// VMS V7.1 checks little (testdata/lib56), and govax checks what it
// does, in its order:
//
//   - the context is found or allocated first, so a cell holding 0 gets
//     a context even when the call then fails; a user-context of 0 is
//     the access violation of reading address 0;
//   - LIB$_ILLCOMPONENT for a component with no name (dtComponents);
//   - LIB$_INVSTRDES for an omitted init-string or a bad descriptor;
//   - LIB$_ELETOOLON for an empty init-string or an element over 255
//     bytes, and LIB$_ILLINISTR for one that doesn't end with its
//     delimiter;
//   - LIB$_NUMELEMENTS for too few elements; more are ignored.
//
// A format isn't checked here: an unknown mnemonic is
// LIB$_UNRFORCOD, and an input format without the fields it needs
// LIB$_ILLFORMAT, when it's used. An init-string of its delimiter alone
// is LIB$_ELETOOLON for the output format (an element of length -1, as
// VMS seems to see it), but accepted for the input format, which is
// LIB$_ELETOOLON when used (unconfirmed); other components so given are
// LIB$_NUMELEMENTS (unconfirmed).
func libInitDateTimeContext(env *corevms.Environment, argv []uint32) (uint32, error) {
	cell := arg(argv, 0)
	if cell == 0 {
		if _, err := env.Memory().LoadLongword(env.CPU(), 0); err != nil {
			return memoryFault(env, err)
		}
	}

	c, status, err := dateContextFor(env, cell)
	if err != nil {
		return memoryFault(env, err)
	}

	if status != ssNormal {
		return status, nil
	}

	code, err := readArg(env, arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	component, ok := dtComponents[code]
	if !ok {
		return libIllComponent, nil
	}

	if arg(argv, 2) == 0 {
		return libInvStrDes, nil
	}

	text, err := readString(env, arg(argv, 2))
	if err != nil {
		return stringFailure(env, err)
	}

	switch {
	case text == "":
		return libEleTooLon, nil
	case len(text) == 1 && code == componentOutputFormat:
		return libEleTooLon, nil
	case len(text) == 1 && code == componentInputFormat:
		if c.malformed == nil {
			c.malformed = map[int32]bool{}
		}

		c.malformed[code] = true
		c.resolved = nil

		return ssNormal, nil
	case len(text) == 1:
		return libNumElements, nil
	}

	elements, status := splitInitString(text)
	if status != ssNormal {
		return status, nil
	}

	if len(elements) < component.elements {
		return libNumElements, nil
	}

	initComponent(c, code, elements[:component.elements])

	c.resolved = nil

	return ssNormal, nil
}

// initComponent sets one component of c from its elements.
func initComponent(c *dateContext, code int32, elements []string) {
	delete(c.malformed, code)

	switch code {
	case componentOutputFormat:
		c.output = append([]string{}, elements...)
	case componentInputFormat:
		c.hasInput, c.inputText = true, elements[0]
	case componentLanguage:
		c.hasLanguage, c.language = true, strings.ToUpper(strings.TrimSpace(elements[0]))
	default:
		if c.spelled == nil {
			c.spelled = map[int32][]string{}
		}

		c.spelled[code] = append([]string{}, elements...)
	}
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
