package security

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// An ACE as text (docs/planning/PHASE-93 - file protection.md, subtask 10,
// and A.4's "Round 3"): what $PARSE_ACL reads and $FORMAT_ACL writes, and
// what DCL's SET SECURITY/ACL and SHOW SECURITY share with them.
//
// An entry's text is one parenthesized list of items, the first saying
// what kind of entry it is:
//
//	(IDENTIFIER=[200,1]+INTERACTIVE,OPTIONS=PROTECTED,ACCESS=READ+WRITE)
//	(DEFAULT_PROTECTION,SYSTEM:RWED,OWNER:RWED,GROUP:RE,WORLD:)
//	(CREATOR,ACCESS=READ+WRITE)
//	(ALARM=SECURITY,ACCESS=READ+SUCCESS+FAILURE)
//	(AUDIT=SECURITY,ACCESS=DELETE+FAILURE)
//	(SUBSYSTEM,IDENTIFIER=PROT93_TEAM)
//
// Keywords may be in any case and abbreviated to any prefix that is
// unique where they stand (ID, A, ACC); blanks may stand around "=", ",",
// "+", and the parentheses. Identifiers are read and written through the
// rights database (Names): VMS 7.1 wrote [200,1] as [PROT93,PROT93A], the
// group's identifier's name and the member's, and an identifier the
// database lacks in numbers.

// Names is the rights database an ACE's identifiers are read and written
// with: an identifier's value by its name (any case), and the name of the
// identifier with a value.
type Names interface {
	IdentifierValue(name string) (uint32, bool)
	IdentifierName(id uint32) (string, bool)
}

// AccessNames name the 32 access bits for a caller that has its own
// ($PARSE_ACL's and $FORMAT_ACL's accnam), bit 0 first. nil is the
// default names, READ, WRITE, EXECUTE, DELETE, CONTROL, and BIT_5 to
// BIT_31.
type AccessNames []string

// accessName is bit's name: names' if it has one, else the default.
func (names AccessNames) accessName(bit int) string {
	if bit < len(names) && names[bit] != "" {
		return names[bit]
	}

	return defaultAccessName(bit)
}

// defaultAccessName is the default name of access bit bit.
func defaultAccessName(bit int) string {
	if bit < len(defaultAccessWords) {
		return defaultAccessWords[bit]
	}

	return "BIT_" + strconv.Itoa(bit)
}

// defaultAccessWords are the first five access bits' default names.
var defaultAccessWords = []string{"READ", "WRITE", "EXECUTE", "DELETE", "CONTROL"}

// More ACE types, and the alarm flags, from $ACEDEF.
var (
	TypeSubsystem   = byte(vmsdef.Symbols["ACE$C_SUBSYSTEM_IDS"])
	TypeApplication = byte(vmsdef.Symbols["ACE$C_INFO"])
	TypeReserved    = byte(vmsdef.Symbols["ACE$C_RESERVED"])
	typeRMSBI       = byte(vmsdef.Symbols["ACE$C_RMSJNL_BI"])
	typeRMSAI       = byte(vmsdef.Symbols["ACE$C_RMSJNL_AI"])
	typeRMSRU       = byte(vmsdef.Symbols["ACE$C_RMSJNL_RU"])
	typeRMSRUDef    = byte(vmsdef.Symbols["ACE$C_RMSJNL_RU_DEFAULT"])

	// An alarm or audit ACE's low flag bits say when it fires.
	AlarmSuccess = uint16(vmsdef.Symbols["ACE$M_SUCCESS"])
	AlarmFailure = uint16(vmsdef.Symbols["ACE$M_FAILURE"])

	// optionMask is the options an ACE's text names; reservedMask an
	// identifier ACE's count of reserved longwords before its
	// identifiers.
	optionMask   = OptionDefault | OptionProtected | OptionHidden | OptionNoPropagate
	reservedMask = uint16(1<<vmsdef.Symbols["ACE$S_RESERVED"]-1) << vmsdef.Symbols["ACE$V_RESERVED"]
)

// MaxIdentifiers is how many identifiers an identifier ACE's text may
// name: VMS 7.1's $PARSE_ACL refused a fourth (A.4).
const MaxIdentifiers = 3

// alarmJournal is the only journal an alarm or audit ACE may name in its
// text: $PARSE_ACL refused (ALARM=OTHER,...).
const alarmJournal = "SECURITY"

// ACEStatus says why an ACE's text was refused.
type ACEStatus int

// The ways an ACE's text can be wrong, each its own status.
const (
	BadACE           ACEStatus = iota // SS$_IVACL: not an ACE's text
	NoSuchIdentifier                  // SS$_NOSUCHID: a name the rights database lacks
	BadIdentifier                     // SS$_IVIDENT: an identifier out of range
)

// ACEError is ParseACE's error: what was wrong, and the offset in the
// text of what was (an item, or for a value that isn't an identifier, the
// value), which $PARSE_ACL returns as errpos.
type ACEError struct {
	Status ACEStatus
	Pos    int
}

func (e *ACEError) Error() string {
	return fmt.Sprintf("security: bad ACE text at offset %d (%d)", e.Pos, e.Status)
}

// ParseACE reads one ACE's text, returning its bytes. names is the
// rights database; access, if not nil, are the caller's names for the
// access bits, which are tried before the default names (VMS 7.1 took
// both: READ was READ with the probe's own names given).
//
// A refusal is an *ACEError: the item it couldn't read, or for what reads
// but makes no ACE (a fourth identifier, an alarm whose journal isn't
// SECURITY or that fires on neither success nor failure), the text's end,
// as VMS 7.1's said.
func ParseACE(text string, names Names, access AccessNames) ([]byte, error) {
	p := &aceParser{text: text, names: names, access: access}

	return p.parse()
}

// aceParser reads an ACE's text, through pos.
type aceParser struct {
	text   string
	pos    int
	names  Names
	access AccessNames
}

// fail is an ACEError of status at pos.
func fail(status ACEStatus, pos int) error { return &ACEError{Status: status, Pos: pos} }

// blanks passes over blanks and tabs.
func (p *aceParser) blanks() {
	for p.pos < len(p.text) && (p.text[p.pos] == ' ' || p.text[p.pos] == '\t') {
		p.pos++
	}
}

// accept passes over c, after any blanks, and the blanks after it.
func (p *aceParser) accept(c byte) bool {
	p.blanks()

	if p.pos < len(p.text) && p.text[p.pos] == c {
		p.pos++
		p.blanks()

		return true
	}

	return false
}

// word reads a keyword or value: letters, digits, "$", and "_".
func (p *aceParser) word() string {
	start := p.pos

	for p.pos < len(p.text) && isWordChar(p.text[p.pos]) {
		p.pos++
	}

	return p.text[start:p.pos]
}

// isWordChar reports whether c can be part of a keyword or a name.
func isWordChar(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '$' || c == '_'
}

// keyword returns which of choices word names: an exact match, else the
// one choice it is a prefix of; "" if none or several.
func keyword(word string, choices ...string) string {
	word = strings.ToUpper(word)
	if word == "" {
		return ""
	}

	found := ""

	for _, c := range choices {
		if c == word {
			return c
		}

		if strings.HasPrefix(c, word) {
			if found != "" {
				return ""
			}

			found = c
		}
	}

	return found
}

// The kinds of ACE a text's first item names.
var aceKinds = []string{"IDENTIFIER", "DEFAULT_PROTECTION", "CREATOR", "ALARM", "ALARM_JOURNAL", "AUDIT", "AUDIT_JOURNAL", "SUBSYSTEM"}

// parsed is what the items say, before it is made an ACE.
type parsed struct {
	kind      string
	ids       []uint32
	options   uint16
	access    uint32
	flags     uint16 // an alarm's SUCCESS and FAILURE
	journal   string
	denied    [4]uint32 // a default protection ACE's, by category
	given     [4]bool
	subsystem []uint32
}

// parse reads the whole text.
func (p *aceParser) parse() ([]byte, error) {
	p.blanks()

	if !p.accept('(') {
		return nil, fail(BadACE, p.pos)
	}

	var a parsed

	if err := p.first(&a); err != nil {
		return nil, err
	}

	for {
		if p.accept(')') {
			break
		}

		if !p.accept(',') {
			return nil, fail(BadACE, p.pos)
		}

		if err := p.item(&a); err != nil {
			return nil, err
		}
	}

	if p.pos != len(p.text) {
		return nil, fail(BadACE, p.pos)
	}

	return a.encode(len(p.text))
}

// first reads the first item, the kind of ACE, and what DEFAULT_PROTECTION
// must have after it.
func (p *aceParser) first(a *parsed) error {
	start := p.pos

	a.kind = keyword(p.word(), aceKinds...)

	switch a.kind {
	case "":
		return fail(BadACE, start)

	case "IDENTIFIER":
		if !p.accept('=') {
			return fail(BadACE, p.pos)
		}

		ids, err := p.identifiers()
		if err != nil {
			return err
		}

		a.ids = ids

	case "ALARM", "ALARM_JOURNAL", "AUDIT", "AUDIT_JOURNAL":
		if !p.accept('=') {
			return fail(BadACE, p.pos)
		}

		a.journal = strings.ToUpper(p.word())

	case "DEFAULT_PROTECTION":
		// The categories follow: (DEFAULT_PROTECTION) alone was refused
		// at its ")".
		p.blanks()

		if p.pos >= len(p.text) || p.text[p.pos] != ',' {
			return fail(BadACE, p.pos)
		}
	}

	return nil
}

// item reads one item after the first.
func (p *aceParser) item(a *parsed) error {
	start := p.pos
	w := p.word()

	if a.kind == "DEFAULT_PROTECTION" {
		if c := keyword(w, "SYSTEM", "OWNER", "GROUP", "WORLD"); c != "" {
			return p.category(a, c)
		}
	}

	choices := []string{"ACCESS", "OPTIONS"}
	if a.kind == "DEFAULT_PROTECTION" {
		choices = []string{"OPTIONS"}
	}

	if a.kind == "SUBSYSTEM" {
		choices = []string{"IDENTIFIER"}
	}

	k := keyword(w, choices...)
	if k == "" || !p.accept('=') {
		return fail(BadACE, start)
	}

	switch k {
	case "IDENTIFIER":
		ids, err := p.identifiers()
		if err != nil {
			return err
		}

		a.subsystem = append(a.subsystem, ids...)

	case "OPTIONS":
		opts, ok := p.options()
		if !ok {
			return fail(BadACE, start)
		}

		a.options |= opts

	case "ACCESS":
		if !p.accessList(a) {
			return fail(BadACE, start)
		}
	}

	return nil
}

// identifiers reads id[+id...].
func (p *aceParser) identifiers() ([]uint32, error) {
	var ids []uint32

	for {
		p.blanks()
		start := p.pos
		text := p.identifierText()

		id, err := ParseIdentifier(text, p.names)
		if err != nil {
			var bad *ACEError
			if errors.As(err, &bad) {
				return nil, fail(bad.Status, start)
			}

			return nil, fail(BadACE, start)
		}

		ids = append(ids, id)

		if !p.accept('+') {
			return ids, nil
		}
	}
}

// identifierText reads an identifier's text: up to a "+", ",", or ")" not
// within brackets, its trailing blanks left out.
func (p *aceParser) identifierText() string {
	start, depth := p.pos, 0

	for p.pos < len(p.text) {
		c := p.text[p.pos]

		switch {
		case c == '[' || c == '<':
			depth++
		case c == ']' || c == '>':
			depth--
		case depth <= 0 && (c == '+' || c == ',' || c == ')'):
			return strings.TrimRight(p.text[start:p.pos], " \t")
		}

		p.pos++
	}

	return strings.TrimRight(p.text[start:p.pos], " \t")
}

// ParseIdentifier reads one identifier as an ACE's text gives it: a UIC
// in brackets ([200,1], [200,*], [*,*], [SMITH], [GROUP,SMITH]), an
// identifier's name, or %Xhex. A refusal is an *ACEError, its Pos 0.
func ParseIdentifier(text string, names Names) (uint32, error) {
	upper := strings.ToUpper(text)

	if hex, ok := strings.CutPrefix(upper, "%X"); ok {
		v, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return 0, fail(BadACE, 0)
		}

		return uint32(v), nil
	}

	if inner, bracketed := unbracket(upper); bracketed && strings.Count(inner, ",") > 1 {
		// [200,1,2] isn't a UIC at all (SS$_IVACL), where [200] is a bad
		// one (SS$_IVIDENT).
		return 0, fail(BadACE, 0)
	}

	if upper == "" {
		return 0, fail(BadACE, 0)
	}

	u, err := ParseUICPattern(text, lookupOf(names))

	var missing ErrNoSuchIdentifier

	switch {
	case err == nil:
		return uint32(u), nil
	case errors.As(err, &missing):
		return 0, fail(NoSuchIdentifier, 0)
	}

	if _, bracketed := unbracket(upper); !bracketed && !validIdentifierName(upper) {
		return 0, fail(BadACE, 0)
	}

	return 0, fail(BadIdentifier, 0)
}

// lookupOf is names as an IdentifierLookup.
func lookupOf(names Names) IdentifierLookup {
	if names == nil {
		return nil
	}

	return names.IdentifierValue
}

// options reads OPTIONS's list: DEFAULT, PROTECTED, NOPROPAGATE, HIDDEN,
// and NONE (nothing), joined by "+".
func (p *aceParser) options() (uint16, bool) {
	var opts uint16

	for {
		p.blanks()

		switch keyword(p.word(), "DEFAULT", "PROTECTED", "NOPROPAGATE", "HIDDEN", "NONE") {
		case "DEFAULT":
			opts |= OptionDefault
		case "PROTECTED":
			opts |= OptionProtected
		case "NOPROPAGATE":
			opts |= OptionNoPropagate
		case "HIDDEN":
			opts |= OptionHidden
		case "NONE":
		default:
			return 0, false
		}

		if !p.accept('+') {
			return opts, true
		}
	}
}

// accessList reads ACCESS's list into a: the access names, NONE, and for
// an alarm or audit SUCCESS and FAILURE.
func (p *aceParser) accessList(a *parsed) bool {
	alarm := a.kind != "IDENTIFIER" && a.kind != "CREATOR"

	for {
		p.blanks()
		w := strings.ToUpper(p.word())

		switch bit, ok := p.accessBit(w); {
		case ok:
			a.access |= 1 << bit
		case keyword(w, "NONE") == "NONE":
		case alarm && keyword(w, "SUCCESS", "FAILURE") == "SUCCESS":
			a.flags |= AlarmSuccess
		case alarm && keyword(w, "SUCCESS", "FAILURE") == "FAILURE":
			a.flags |= AlarmFailure
		default:
			return false
		}

		if !p.accept('+') {
			return true
		}
	}
}

// accessBit is the access bit w names: one of the caller's names if it
// gave them, exactly or by a unique prefix, else one of the defaults
// (BIT_n only in full).
func (p *aceParser) accessBit(w string) (int, bool) {
	if w == "" {
		return 0, false
	}

	if p.access != nil {
		if bit, ok := nameBit(w, p.access); ok {
			return bit, true
		}
	}

	if n, ok := strings.CutPrefix(w, "BIT_"); ok {
		bit, err := strconv.Atoi(n)

		return bit, err == nil && bit >= len(defaultAccessWords) && bit < 32
	}

	return nameBit(w, defaultAccessWords)
}

// nameBit is the index in names of the one name w is, or is a prefix of.
func nameBit(w string, names []string) (int, bool) {
	upper := make([]string, len(names))
	for i, n := range names {
		upper[i] = strings.ToUpper(n)
	}

	k := keyword(w, upper...)
	if k == "" {
		return 0, false
	}

	for i, n := range upper {
		if n == k {
			return i, true
		}
	}

	return 0, false
}

// category reads a default protection ACE's category c: "c", "c:", or
// "c:letters" ("=" for ":" too). A category named twice keeps its first
// letters. A letter that isn't R, W, E, D, or C fails at the letters, as
// VMS 7.1's SET SECURITY quoted 'Q)' of (DEFAULT_PROTECTION,S:Q).
func (p *aceParser) category(a *parsed, c string) error {
	i := map[string]int{"SYSTEM": 0, "OWNER": 1, "GROUP": 2, "WORLD": 3}[c]

	var granted uint32

	if p.accept(':') || p.accept('=') {
		letters := p.pos

		for _, l := range strings.ToUpper(p.word()) {
			k := strings.IndexRune("RWEDC", l)
			if k < 0 {
				return fail(BadACE, letters)
			}

			granted |= 1 << k
		}
	}

	if !a.given[i] {
		a.given[i] = true
		a.denied[i] = ^granted & 0x1F
	}

	return nil
}

// encode makes the ACE the items describe; end is the text's length,
// where a refusal of what reads but makes no ACE is placed.
func (a *parsed) encode(end int) ([]byte, error) {
	le := binary.LittleEndian

	switch a.kind {
	case "IDENTIFIER":
		if len(a.ids) > MaxIdentifiers {
			return nil, fail(BadACE, end)
		}

		return NewIdentifierACE(a.ids, Access(a.access), a.options).Data, nil

	case "DEFAULT_PROTECTION":
		data := make([]byte, aceSysPro+16)
		writeHeader(data, TypeDefaultProtection, a.options, 0)

		for i := range 4 {
			denied := a.denied[i]
			if !a.given[i] {
				denied = 0x1F
			}

			le.PutUint32(data[aceSysPro+4*i:], denied)
		}

		return data, nil

	case "CREATOR":
		data := make([]byte, aceHeader)
		writeHeader(data, TypeCreator, a.options, Access(a.access))

		return data, nil

	case "SUBSYSTEM":
		data := make([]byte, aceHeader+8*len(a.subsystem))
		writeHeader(data, TypeSubsystem, a.options, 0)

		for i, id := range a.subsystem {
			le.PutUint32(data[aceHeader+8*i:], id)
		}

		return data, nil
	}

	// An alarm or audit ACE: it must fire on success, failure, or both,
	// and name the SECURITY journal.
	if a.flags == 0 || a.journal != alarmJournal {
		return nil, fail(BadACE, end)
	}

	typ := TypeAlarm
	if strings.HasPrefix(a.kind, "AUDIT") {
		typ = TypeAudit
	}

	data := make([]byte, aceHeader+len(a.journal))
	writeHeader(data, typ, a.options|a.flags, Access(a.access))
	copy(data[aceHeader:], a.journal)

	return data, nil
}

// FormatACE is an ACE's text, as $FORMAT_ACL writes it with no width. ace
// is the entry's bytes, its length its size (FormatPieces).
func FormatACE(ace []byte, names Names, access AccessNames) string {
	return strings.Join(FormatPieces(ace, names, access), "")
}

// FormatPieces is an ACE's text in the pieces $FORMAT_ACL lays out on
// lines (Wrap): it breaks a line only between pieces. VMS 7.1's showed
// them (A.4): an identifier list's identifiers and their "+"s are pieces
// of their own, and so is the "," after them, where an access or option
// name carries the "+", ",", or ")" after it.
//
// ace is the entry's bytes, its length taken as its size; what the
// layout needs past them reads as zeros, as VMS 7.1's read whatever
// followed a short entry.
func FormatPieces(ace []byte, names Names, access AccessNames) []string {
	if len(ace) < aceHeader {
		ace = append(append([]byte(nil), ace...), make([]byte, aceHeader-len(ace))...)
	}

	f := aceFormatter{ace: ace, names: names, access: access}

	switch typ := ace[aceType]; typ {
	case TypeIdentifier:
		return f.identifier()
	case TypeDefaultProtection:
		return f.defaultProtection()
	case TypeCreator:
		return f.creator()
	case TypeAlarm, TypeAudit:
		return f.alarm()
	case TypeSubsystem:
		return f.subsystem()
	case TypeApplication:
		return f.raw("APPLICATION")
	case typeRMSBI, typeRMSAI, typeRMSRU:
		return f.journal()
	case typeRMSRUDef:
		return f.split(fmt.Sprintf("(RMS_RU_DEFAULT,VOLUME_NAME=%s)", f.volumeName()))
	case TypeReserved:
		return f.raw("RESERVED")
	default:
		return f.raw(fmt.Sprintf("UNKNOWN=%%X%02X", typ))
	}
}

// aceFormatter writes one ACE's pieces.
type aceFormatter struct {
	ace    []byte
	names  Names
	access AccessNames
}

// long is the longword at off, zero past the entry's bytes.
func (f *aceFormatter) long(off int) uint32 {
	var b [4]byte

	if off < len(f.ace) {
		copy(b[:], f.ace[off:])
	}

	return binary.LittleEndian.Uint32(b[:])
}

// bytesAt are n bytes at off, zeros past the entry's.
func (f *aceFormatter) bytesAt(off, n int) []byte {
	b := make([]byte, n)

	if off < len(f.ace) {
		copy(b, f.ace[off:])
	}

	return b
}

func (f *aceFormatter) flags() uint16 { return binary.LittleEndian.Uint16(f.ace[aceFlags:]) }

// optionPieces are "OPTIONS=" and its names, the last carrying ",", or
// none for no options. An option the text has no name for (an identifier
// ACE's count of reserved longwords, any other bit) makes it the whole
// flags word in hexadecimal instead (VMS 7.1's OPTIONS=%X1000). known are
// the flags the entry's own type gives a meaning, shown elsewhere.
func (f *aceFormatter) optionPieces(known uint16) []string {
	flags := f.flags()

	if flags&^(optionMask|known) != 0 {
		return []string{"OPTIONS=", fmt.Sprintf("%%X%04X,", flags)}
	}

	var words []string

	for _, o := range []struct {
		bit  uint16
		name string
	}{{OptionDefault, "DEFAULT"}, {OptionHidden, "HIDDEN"}, {OptionProtected, "PROTECTED"}, {OptionNoPropagate, "NOPROPAGATE"}} {
		if flags&o.bit != 0 {
			words = append(words, o.name)
		}
	}

	if len(words) == 0 {
		return nil
	}

	return append([]string{"OPTIONS="}, joinPieces(words, "+", ",")...)
}

// joinPieces are words, each carrying sep after it, the last end.
func joinPieces(words []string, sep, end string) []string {
	pieces := make([]string, len(words))

	for i, w := range words {
		if i == len(words)-1 {
			pieces[i] = w + end
		} else {
			pieces[i] = w + sep
		}
	}

	return pieces
}

// accessPieces are "ACCESS=" and the access's names, extra after them
// (an alarm's SUCCESS and FAILURE), the last carrying ")"; NONE for none.
func (f *aceFormatter) accessPieces(extra ...string) []string {
	access := f.long(aceAccess)

	var words []string

	for bit := range 32 {
		if access&(1<<bit) != 0 {
			words = append(words, f.access.accessName(bit))
		}
	}

	words = append(words, extra...)

	if len(words) == 0 {
		words = []string{"NONE"}
	}

	return append([]string{"ACCESS="}, joinPieces(words, "+", ")")...)
}

// identifier is an identifier ACE: its identifiers after any reserved
// longwords, each of the entry's longwords, the last read with zeros past
// the entry (VMS 7.1 showed a 10-byte entry's as [0,1]).
func (f *aceFormatter) identifier() []string {
	reserved := int(f.flags() & reservedMask)
	first := aceKey + 4*reserved

	pieces := []string{"(", "IDENTIFIER"}

	for i, off := 0, first; off < len(f.ace); i, off = i+1, off+4 {
		if i == 0 {
			pieces[1] = "IDENTIFIER="
		} else {
			pieces = append(pieces, "+")
		}

		pieces = append(pieces, IdentifierText(f.long(off), f.names))
	}

	pieces = append(pieces, ",")
	pieces = append(pieces, f.optionPieces(0)...)

	return append(pieces, f.accessPieces()...)
}

// defaultProtection is a default protection ACE: each category's granted
// letters, C among them (the entry keeps the denied bits).
func (f *aceFormatter) defaultProtection() []string {
	pieces := []string{"(", "DEFAULT_PROTECTION,"}

	for i, name := range []string{"SYSTEM", "OWNER", "GROUP", "WORLD"} {
		if i > 0 {
			pieces = append(pieces, ",")
		}

		pieces = append(pieces, name+":"+grantedLetters(^f.long(aceSysPro+4*i)))
	}

	if opts := f.optionPieces(0); opts != nil {
		last := len(opts) - 1
		opts[last] = strings.TrimSuffix(opts[last], ",")

		pieces = append(pieces, ",")
		pieces = append(pieces, opts...)
	}

	return append(pieces, ")")
}

// grantedLetters are the letters of the access bits granted: R, W, E, D,
// and C.
func grantedLetters(granted uint32) string {
	var b strings.Builder

	for i, l := range "RWEDC" {
		if granted&(1<<i) != 0 {
			b.WriteRune(l)
		}
	}

	return b.String()
}

// creator is a creator ACE.
func (f *aceFormatter) creator() []string {
	pieces := []string{"(", "CREATOR", ","}
	pieces = append(pieces, f.optionPieces(0)...)

	return append(pieces, f.accessPieces()...)
}

// alarm is an alarm or audit ACE: the journal's name is the bytes after
// the access, whatever they are (VMS 7.1 showed JOURNAL1, and nothing).
func (f *aceFormatter) alarm() []string {
	kind := "ALARM="
	if f.ace[aceType] == TypeAudit {
		kind = "AUDIT="
	}

	journal := ""
	if len(f.ace) > aceHeader {
		journal = string(f.ace[aceHeader:])
	}

	pieces := []string{"(", kind, journal, ","}
	pieces = append(pieces, f.optionPieces(AlarmSuccess|AlarmFailure)...)

	var extra []string

	if f.flags()&AlarmSuccess != 0 {
		extra = append(extra, "SUCCESS")
	}

	if f.flags()&AlarmFailure != 0 {
		extra = append(extra, "FAILURE")
	}

	return append(pieces, f.accessPieces(extra...)...)
}

// subsystem is a subsystem ACE: its identifiers and their attributes, in
// whole pairs of longwords after the header (VMS 7.1 showed a 12-byte
// entry as (SUBSYSTEM) alone).
func (f *aceFormatter) subsystem() []string {
	pieces := []string{"(", "SUBSYSTEM"}

	for off := aceHeader; off+8 <= len(f.ace); off += 8 {
		attributes := "NONE"
		if a := f.long(off + 4); a != 0 {
			attributes = fmt.Sprintf("%%X%08X", a) // unconfirmed: no probe gave one
		}

		pieces = append(pieces, ",", "IDENTIFIER=", IdentifierText(f.long(off), f.names), ",", "ATTRIBUTES="+attributes)
	}

	return append(pieces, ")")
}

// raw is an entry the text has no form for: its size, flags, access, and
// each longword after them in hexadecimal.
func (f *aceFormatter) raw(kind string) []string {
	text := fmt.Sprintf("(%s,SIZE=%%D%d,FLAGS=%%X%04X,ACCESS=%%X%08X,DATA", kind, len(f.ace), f.flags(), f.long(aceAccess))

	for i, off := 0, aceHeader; off < len(f.ace); i, off = i+1, off+4 {
		sep := ","
		if i == 0 {
			sep = "="
		}

		text += fmt.Sprintf("%s%%X%08X", sep, f.long(off))
	}

	return f.split(text + ")")
}

// split is text in pieces that each end at a comma.
func (f *aceFormatter) split(text string) []string {
	var pieces []string

	for {
		i := strings.IndexByte(text, ',')
		if i < 0 {
			return append(pieces, text)
		}

		pieces = append(pieces, text[:i+1])
		text = text[i+1:]
	}
}

// The RMS journaling ACEs' fields, from $ACEDEF.
var (
	aceVolNam    = int(vmsdef.Symbols["ACE$T_VOLNAM"])
	aceVolNamMax = int(vmsdef.Symbols["ACE$S_VOLNAM"])
	aceVolNamLen = int(vmsdef.Symbols["ACE$B_VOLNAM_LEN"])
	aceRJRVer    = int(vmsdef.Symbols["ACE$B_RJRVER"])
	aceFIDNum    = int(vmsdef.Symbols["ACE$W_FID_NUM"])
	aceFIDSeq    = int(vmsdef.Symbols["ACE$W_FID_SEQ"])
	aceFIDRVN    = int(vmsdef.Symbols["ACE$B_FID_RVN"])
	aceFIDNMX    = int(vmsdef.Symbols["ACE$B_FID_NMX"])
	aceJnlFlags  = int(vmsdef.Symbols["ACE$W_RMSJNL_FLAGS"])
	aceJnlIdx    = int(vmsdef.Symbols["ACE$L_JNLIDX"])
	aceCDate     = int(vmsdef.Symbols["ACE$Q_CDATE"])
	aceBackupSeq = int(vmsdef.Symbols["ACE$L_BACKUP_SEQNO"])
	aceModTime   = int(vmsdef.Symbols["ACE$Q_MODIFICATION_TIME"])
)

// volumeName is an RMS journaling ACE's volume name.
func (f *aceFormatter) volumeName() string {
	n := min(int(f.bytesAt(aceVolNamLen, 1)[0]), aceVolNamMax)

	return string(f.bytesAt(aceVolNam, n))
}

// journal is an RMS journaling ACE (before-image, after-image, or
// recovery unit), its fields by $ACEDEF's layout; VMS 7.1 showed the
// probe's entries, all zeros, so which field is which past their names is
// unconfirmed, and JOURNALING_OPTIONS other than NONE is govax's.
func (f *aceFormatter) journal() []string {
	le := binary.LittleEndian
	typ := f.ace[aceType]

	kind := map[byte]string{typeRMSBI: "RMS_BI_JOURNAL", typeRMSAI: "RMS_AI_JOURNAL", typeRMSRU: "RMS_RU_JOURNAL"}[typ]
	num := uint32(le.Uint16(f.bytesAt(aceFIDNum, 2))) | uint32(f.bytesAt(aceFIDNMX, 1)[0])<<16
	seq := le.Uint16(f.bytesAt(aceFIDSeq, 2))
	rvn := f.bytesAt(aceFIDRVN, 1)[0]

	options := "NONE"
	if fl := le.Uint16(f.bytesAt(aceJnlFlags, 2)); fl != 0 {
		options = fmt.Sprintf("%%X%04X", fl)
	}

	text := fmt.Sprintf("(%s,JOURNAL_FILE=DISK$%s:(%d,%d,%d),JOURNAL_LEVEL=%d,JOURNAL_CREATION_DATE=%s,JOURNAL_STREAM_INDEX=%d,",
		kind, f.volumeName(), num, seq, rvn, f.bytesAt(aceRJRVer, 1)[0],
		vmsTimeText(le.Uint64(f.bytesAt(aceCDate, 8))), f.long(aceJnlIdx))

	if typ != typeRMSRU {
		text += fmt.Sprintf("BACKUP_SEQUENCE_NUMBER=%d,JOURNAL_CONSISTENCY_DATE=%s,",
			f.long(aceBackupSeq), vmsTimeText(le.Uint64(f.bytesAt(aceModTime, 8))))
	}

	return f.split(text + "JOURNALING_OPTIONS=" + options + ")")
}

// vmsEpoch is VMS's time zero, 17-NOV-1858.
var vmsEpoch = time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC)

// vmsTimeText is a VMS time (100-nanosecond units since 17-NOV-1858) as
// an RMS journaling ACE's text shows it, "17-NOV-1858:00:00:00.00".
func vmsTimeText(t uint64) string {
	const unitsPerSecond = 10_000_000

	at := vmsEpoch.Add(time.Duration(t/unitsPerSecond) * time.Second)
	hundredths := t % unitsPerSecond / 100_000

	return fmt.Sprintf("%d-%s-%d:%02d:%02d:%02d.%02d", at.Day(), strings.ToUpper(at.Format("Jan")), at.Year(),
		at.Hour(), at.Minute(), at.Second(), hundredths)
}

// IdentifierText is an identifier as an ACE's text writes it (VMS 7.1's,
// A.4): a UIC as [group,member] by the names of its identifier and its
// group's ([PROT93,PROT93A]), [member] when the group has none
// ([SYSTEM]), in octal when the UIC has none ([310,1]); [g,*] and [*,m]
// for the wildcards; a general identifier by its name, or in hexadecimal
// (%X8001FFFF) without one, as is any other value.
func IdentifierText(id uint32, names Names) string {
	name := func(v uint32) (string, bool) {
		if names == nil {
			return "", false
		}

		return names.IdentifierName(v)
	}

	switch id >> 30 {
	case 0:
		// A UIC.
	case 2:
		if n, ok := name(id); ok {
			return n
		}

		return fmt.Sprintf("%%X%08X", id)
	default:
		return fmt.Sprintf("%%X%08X", id)
	}

	u := UIC(id)
	group, member := u.Group(), u.Member()

	groupText := fmt.Sprintf("%o", group)
	groupName, haveGroup := name(group<<16 | WildMember)

	if haveGroup {
		groupText = groupName
	}

	switch {
	case group == WildGroup && member == WildMember:
		return "[*,*]"
	case group == WildGroup:
		return fmt.Sprintf("[*,%o]", member)
	case member == WildMember:
		return fmt.Sprintf("[%s,*]", groupText)
	}

	n, ok := name(id)

	switch {
	case !ok:
		return u.String() // unconfirmed with a named group: no probe case
	case haveGroup:
		return fmt.Sprintf("[%s,%s]", groupName, n)
	}

	return "[" + n + "]"
}

// Wrap lays pieces out as $FORMAT_ACL does: indent blanks first, and with
// wrap (a terminator was given) and a width, a new line wherever the next
// piece would take the line, its indentation counted, past width: the
// terminator, then indent blanks again. A line takes at least one piece,
// however long.
func Wrap(pieces []string, width int, terminator string, wrap bool, indent int) string {
	var b strings.Builder

	lead := strings.Repeat(" ", max(indent, 0))
	b.WriteString(lead)

	length, onLine := len(lead), 0

	for _, p := range pieces {
		if wrap && width > 0 && onLine > 0 && length+len(p) > width {
			b.WriteString(terminator)
			b.WriteString(lead)

			length, onLine = len(lead), 0
		}

		b.WriteString(p)

		length += len(p)
		onLine++
	}

	return b.String()
}

// MatchACE reports whether ace matches want as $CHANGE_ACL and
// $SET_SECURITY match them, by want's type (the VMS 5.0 System Services
// manual's $CHANGE_ACL, and VMS 7.1's answers, A.4): a default protection
// ACE by its type alone; an identifier ACE by its flags and identifiers,
// PROTECTED aside (VMS 7.1 had a PROTECTED entry take an unprotected one's
// place; HIDDEN and NOPROPAGATE unconfirmed); an alarm or audit ACE by its
// flags, whatever its access (VMS 7.1's replaced (ALARM...READ+SUCCESS) by
// (ALARM...WRITE+SUCCESS)); any other by all its bytes.
func MatchACE(ace, want []byte) bool {
	if len(ace) < aceHeader || len(want) < aceHeader || ace[aceType] != want[aceType] {
		return false
	}

	le := binary.LittleEndian
	flags, wantFlags := le.Uint16(ace[aceFlags:]), le.Uint16(want[aceFlags:])

	switch want[aceType] {
	case TypeDefaultProtection:
		return true
	case TypeIdentifier:
		return flags&^OptionProtected == wantFlags&^OptionProtected && string(ace[aceKey:]) == string(want[aceKey:])
	case TypeAlarm, TypeAudit:
		return flags == wantFlags
	}

	return string(ace) == string(want)
}

// AlarmLike reports whether ace is an alarm or audit ACE, which
// $CHANGE_ACL adds by replacing a match wherever it is (A.4).
func AlarmLike(ace []byte) bool {
	return len(ace) > aceType && (ace[aceType] == TypeAlarm || ace[aceType] == TypeAudit)
}
