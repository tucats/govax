package dclcmd

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// SET SECURITY and VMS's obsolete forms of it, SET FILE/PROTECTION and
// /OWNER_UIC, SET PROTECTION=(code) file, and SET ACL
// (docs/planning/PHASE-93 - file protection.md, subtask 11, Decision 10,
// and A.4's "Changing profiles").
//
// A file's profile is its owner (a UIC, or an identifier), its protection
// code, and its access control list. Changing any of it needs control
// access to the file: the system and owner categories have it, and so
// does anyone an ACE gives CONTROL. Giving the file a different owner
// needs more, security.CanChangeOwner's rule: an owner can't give a file
// away without privilege, though one with CONTROL may change everything
// else.
//
// Every form runs the same code, SetSecurity, with a SecurityForm that
// decides only the messages each form shows, as VMS 7.1's showed them.

// SecurityForm is which command is changing a profile.
type SecurityForm int

// The forms.
const (
	// FormSecurity is SET SECURITY.
	FormSecurity SecurityForm = iota

	// FormFile is SET FILE/PROTECTION and /OWNER_UIC.
	FormFile

	// FormProtection is SET PROTECTION[=(code)] file.
	FormProtection

	// FormACL is SET ACL.
	FormACL
)

// SecurityOptions is one SET SECURITY command (or an obsolete form of
// it), as the console's grammar read it.
type SecurityOptions struct {
	Form SecurityForm

	// Files are the files named: SET SECURITY's one specification, SET
	// FILE's list.
	Files []string

	// Protection is /PROTECTION's code, an element per category as typed
	// ("S:RWED", "W"); with HasProtection and no elements, the process's
	// default protection (SET FILE/PROTECTION and SET PROTECTION with no
	// code, A.4 answer 11).
	Protection    []string
	HasProtection bool

	// Owner is /OWNER's (or /OWNER_UIC's) UIC or identifier; with
	// HasOwner and no text, the process's UIC (SET FILE/OWNER_UIC).
	Owner    string
	HasOwner bool

	// Default is /DEFAULT: the process's default protection (for a
	// directory, its parent's less delete), the process's UIC, and only
	// the protected ACEs kept.
	Default bool

	// Like is /LIKE=NAME=file's file, and CopyAttributes /COPY_ATTRIBUTE's
	// keywords, of which OWNER also copies the owner.
	Like           string
	CopyAttributes []string

	// ACL is /ACL's entries' text, "(ace)" or "((ace),(ace))", HasACL
	// whether /ACL was given (with no value it names no entries). After,
	// Replace, and Delete are /AFTER's and /REPLACE's entries and
	// /DELETE's value ("" or "ALL"), HasDelete whether it was given.
	ACL       string
	HasACL    bool
	After     string
	Replace   string
	Delete    string
	HasDelete bool

	// Log is /LOG: each file changed is reported, %SET-I-MODIFIED.
	Log bool

	// Exclude are /EXCLUDE's names, ByOwner /BY_OWNER's UIC: which of the
	// files found are changed.
	Exclude []string
	ByOwner string

	// Class is /CLASS's object class; only FILE is supported.
	Class string
}

// setFacility is the SET utility's facility number, which its condition
// values carry (unconfirmed: the probe's logs show SET's messages, not
// their values).
const setFacility = 119

// setStatus is a SET message's condition value: a shared message's (SHR$)
// number in SET's facility, or for one of SET's own a number of its own
// (bit 15, the facility-specific bit), each with its severity.
func setStatus(shr string, own uint32, severity uint32) uint32 {
	if shr != "" {
		return setFacility<<16 | vmsdef.LibrarySymbols["SHR$_"+shr]&^7 | severity
	}

	return setFacility<<16 | 0x8000 | own<<3 | severity
}

// SET's messages: the severities are the logs' (W 0, S 1, E 2, I 3, F 4).
var (
	setOpenIn    = setStatus("OPENIN", 0, 2)
	setOpenOut   = setStatus("OPENOUT", 0, 2)
	setWriteErr  = setStatus("WRITEERR", 0, 4)
	setSyntax    = setStatus("SYNTAX", 0, 4)
	setModified  = setStatus("", 1, 3)
	setProNotChg = setStatus("", 2, 2)
	setProErr    = setStatus("", 3, 2)
	setNoHidden  = setStatus("", 4, 2)
	setNoSuchACE = setStatus("", 5, 0)
)

// The secondary statuses SET's messages show.
var (
	libSyntaxErr = vmsdef.Symbols["LIB$_SYNTAXERR"]
	libInvArg    = vmsdef.Symbols["LIB$_INVARG"]
	ssIVACL      = vmsdef.Symbols["SS$_IVACL"]
)

// securityReport collects a command's messages as it goes, in order, and
// the condition value of the last failure: SET goes on to the next file
// after one it couldn't change, as VMS's did.
type securityReport struct {
	c      *Context
	status uint32
}

// message shows a message, "%SET-E-OPENIN, ...", and the secondary
// statuses' lines under it, and remembers status if it is a failure.
func (r *securityReport) message(status uint32, text string, secondary ...uint32) {
	r.c.Printf("%%%s\n", text)

	for _, s := range secondary {
		r.c.Printf("%s\n", ConditionLine(s))
	}

	if status&1 == 0 {
		r.status = status
	}
}

// result is the command's error: nil if nothing failed, else the last
// failure's status, its messages already shown.
func (r *securityReport) result() error {
	if r.status == 0 {
		return nil
	}

	return vmserrors.InhibitMessage(vmserrors.ConditionError{Status: r.status | stsInhibitMsg, Text: "SET failed"})
}

// syntaxError is SET's refusal of a value: "%SET-F-SYNTAX, error parsing
// 'text'", with the secondary status's line if there is one.
func syntaxError(text string, secondary ...uint32) error {
	lines := []string{"SET-F-SYNTAX, error parsing '" + text + "'"}
	for _, s := range secondary {
		lines = append(lines, ConditionLine(s))
	}

	return vmserrors.ConditionError{Status: setSyntax, Text: strings.Join(lines, "\n")}
}

// securityChange is what SetSecurity has read from its options before it
// looks at any file.
type securityChange struct {
	opts SecurityOptions

	creds security.Credentials
	names security.Names

	owner    security.UIC
	byOwner  security.UIC
	acl      [][]byte // /ACL's entries
	after    []byte   // /AFTER's entry
	replace  [][]byte // /REPLACE's entries
	like     *rms.SecurityObject
	copyOwner bool // /LIKE copies the owner too
}

// SetSecurity is SET SECURITY, and with opts.Form its obsolete forms:
// each file named has its profile changed as opts say. A value the
// command can't read stops it before any file is changed; a file it may
// not change (or can't find) is reported, and the others are changed.
func (c *Context) SetSecurity(opts SecurityOptions) error {
	if c.ContainerSession == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	if opts.Class != "" && !strings.HasPrefix("FILE", strings.ToUpper(opts.Class)) {
		// Only files have profiles here (Decision 10; other classes
		// unconfirmed: VMS 7.1 has them).
		return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, strings.ToUpper(opts.Class))
	}

	ch := &securityChange{opts: opts, creds: c.ContainerSession.Identity(), names: rightsNames{c}}
	if err := ch.read(c); err != nil {
		return err
	}

	report := &securityReport{c: c}

	for _, spec := range opts.Files {
		objects, err := c.ContainerSession.SecurityObjects(spec)
		if err != nil {
			ch.notFound(report, c, spec, err)

			continue
		}

		for _, o := range objects {
			if ch.selected(o) {
				ch.change(report, o)
			}
		}
	}

	return report.result()
}

// read reads the options' values: the protection code's letters, the
// UICs, the ACEs, and the /LIKE file, each refused with SET's own
// message, as VMS 7.1's were (A.4, "Forms and their errors").
func (ch *securityChange) read(c *Context) error {
	opts := ch.opts

	if code := codeText(opts.Protection); code != "" {
		if _, err := security.ParseProtection(code, 0); err != nil {
			return protectionSyntax(err, code)
		}
	}

	if opts.HasOwner {
		ch.owner = ch.creds.UIC

		if opts.Owner != "" {
			uic, err := security.ParseUIC(opts.Owner, c.identifierLookup())
			if err != nil {
				return syntaxError(opts.Owner)
			}

			ch.owner = uic
		}
	}

	if opts.ByOwner != "" {
		uic, err := security.ParseUICPattern(opts.ByOwner, c.identifierLookup())
		if err != nil {
			return syntaxError(opts.ByOwner)
		}

		ch.byOwner = uic
	}

	var err error

	if ch.acl, err = ch.entries(opts.ACL); err != nil {
		return err
	}

	if ch.replace, err = ch.entries(opts.Replace); err != nil {
		return err
	}

	after, err := ch.entries(opts.After)
	if err != nil {
		return err
	}

	if len(after) > 0 {
		ch.after = after[0]
	}

	for _, ace := range append(append([][]byte{}, ch.acl...), ch.replace...) {
		if security.DecodeACE(ace).Has(security.OptionHidden) {
			return vmserrors.ConditionError{Status: setNoHidden, Text: "SET-E-NOHIDDEN, cannot modify hidden ACEs"}
		}
	}

	for _, kw := range opts.CopyAttributes {
		if strings.HasPrefix("OWNER", strings.ToUpper(kw)) {
			ch.copyOwner = true
		}
	}

	if opts.Like != "" {
		like, err := c.ContainerSession.FindSecurityObject(opts.Like)
		if err != nil || !like.MayRead() {
			// The /LIKE file's refusals are unconfirmed: the probe's
			// were all readable.
			if err == nil {
				err = rms.ErrNoPrivilege
			}

			report := &securityReport{c: c}
			ch.notFound(report, c, opts.Like, err)

			return report.result()
		}

		ch.like = like
	}

	return nil
}

// codeText is a protection code's elements as one text, "S:R,W:RE"; ""
// for none (a /PROTECTION with no value).
func codeText(code []string) string {
	var parts []string

	for _, p := range code {
		if p != "" {
			parts = append(parts, p)
		}
	}

	return strings.Join(parts, ",")
}

// protectionSyntax is SET's refusal of a protection code's letters:
// %SET-F-SYNTAX naming the code, with LIB$TPARSE's SYNTAXERR for letters
// that aren't R, W, E, and D, and INVARG for a category given twice.
func protectionSyntax(err error, code string) error {
	var unknown security.ErrUnknownCategory
	if errors.As(err, &unknown) {
		return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, unknown.Name)
	}

	if errors.Is(err, security.ErrRepeatedCategory) {
		return syntaxError(code, libInvArg)
	}

	return syntaxError(code, libSyntaxErr)
}

// entries reads an $ACL value's entries, "(ace)" or "((ace),(ace),...)",
// each by $PARSE_ACL's grammar. A refusal is %SET-F-SYNTAX quoting the
// entry from the item that failed to its end, with SS$_NOSUCHID for an
// unknown identifier and SS$_IVACL (or SS$_IVIDENT) otherwise, as VMS
// 7.1 showed it.
func (ch *securityChange) entries(value string) ([][]byte, error) {
	var aces [][]byte

	for _, text := range splitEntries(value) {
		ace, err := security.ParseACE(text, ch.names, nil)
		if err != nil {
			var bad *security.ACEError
			if !errors.As(err, &bad) {
				return nil, err
			}

			status := ssIVACL

			switch bad.Status {
			case security.NoSuchIdentifier:
				status = ssNOSUCHID
			case security.BadIdentifier:
				status = ssIVIDENT
			}

			pos := min(bad.Pos, len(text))

			return nil, syntaxError(text[pos:], status)
		}

		aces = append(aces, ace)
	}

	return aces, nil
}

// splitEntries is an $ACL value's entries' texts: the value itself for
// one entry, "(ace)", or each parenthesized entry of a list, "((a),(b))".
// An empty value has none.
func splitEntries(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "("), ")"))
	if !strings.HasPrefix(value, "(") || !strings.HasPrefix(inner, "(") {
		return []string{value}
	}

	var (
		out   []string
		depth int
		start int
	)

	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '(':
			if depth == 0 {
				start = i
			}

			depth++
		case ')':
			if depth--; depth == 0 {
				out = append(out, inner[start:i+1])
			}
		}
	}

	return out
}

// notFound reports a file SET couldn't find or reach, as VMS 7.1's SET
// SECURITY reported a missing file or directory (A.4):
//
//	%SET-E-OPENOUT, error opening DUA1:[PROT93.SEC]NOSUCH.DAT; as output
//	-RMS-E-FNF, file not found
//
// A directory the process may not search is OPENIN over RMS$_PRV
// (unconfirmed).
func (ch *securityChange) notFound(report *securityReport, c *Context, spec string, err error) {
	var fileErr *rms.DCLFileError
	if errors.As(err, &fileErr) {
		secondary := []uint32{fileErr.Status}
		if fileErr.STV != 0 {
			secondary = append(secondary, fileErr.STV)
		}

		report.message(setOpenOut, "SET-E-OPENOUT, error opening "+fileErr.Name+" as output", secondary...)

		return
	}

	if denied, ok := rms.IsDenied(err); ok || errors.Is(err, rms.ErrNoPrivilege) {
		name := c.expandedName(spec)
		if denied != nil && denied.Name != "" {
			name = denied.Name
		}

		report.message(setOpenIn, "SET-E-OPENIN, error opening "+name+" as input", rmsPRV)

		return
	}

	report.message(setOpenOut, "SET-E-OPENOUT, error opening "+c.expandedName(spec)+" as output", rmsFileNotFound)
}

// rmsFileNotFound is RMS$_FNF.
var rmsFileNotFound = vmsdef.Symbols["RMS$_FNF"]

// selected reports whether /EXCLUDE and /BY_OWNER leave the file to be
// changed: its name and type (and version, if a name gives one) match no
// /EXCLUDE name, and its owner is /BY_OWNER's UIC.
func (ch *securityChange) selected(o *rms.SecurityObject) bool {
	file := o.Name
	if i := strings.LastIndexAny(file, "]>"); i >= 0 {
		file = file[i+1:]
	}

	nameType, version, _ := strings.Cut(file, ";")

	for _, pattern := range ch.opts.Exclude {
		pattern = strings.ToUpper(pattern)
		if i := strings.LastIndexAny(pattern, "]>"); i >= 0 {
			pattern = pattern[i+1:]
		}

		pName, pVersion, hasVersion := strings.Cut(pattern, ";")
		if !strings.Contains(pName, ".") {
			pName += ".*"
		}

		if rms.WildMatch(pName, nameType) && (!hasVersion || pVersion == "" || pVersion == "*" || rms.WildMatch(pVersion, version)) {
			return false
		}
	}

	if ch.opts.ByOwner != "" && !o.Profile().Owner.Matches(ch.byOwner) {
		return false
	}

	return true
}

// change makes one file's change, or reports why it couldn't.
func (ch *securityChange) change(report *securityReport, o *rms.SecurityObject) {
	opts := ch.opts

	if !o.MayChange() {
		ch.refused(report, o.Name)

		return
	}

	profile := o.Profile()
	owner, protection := profile.Owner, profile.Protection

	acl, err := security.DecodeACL(o.ACL())
	if err != nil {
		acl = nil
	}

	if opts.Default {
		// The process's defaults; a directory gets what a new directory
		// in its parent would (A.4, round 2's SD1.DIR).
		protection = security.Protection(ch.creds.DefaultProtection)
		if o.IsDirectory() {
			if p, ok := o.DirectoryDefault(); ok {
				protection = p
			}
		}

		owner = ch.creds.UIC
		acl = protectedOnly(acl)
	}

	if ch.like != nil {
		like := ch.like.Profile()
		protection = like.Protection

		if ch.copyOwner {
			owner = like.Owner
		}

		likeACL, _ := security.DecodeACL(ch.like.ACL())
		acl = append(protectedOnly(acl), propagating(likeACL)...)
	}

	if opts.HasProtection {
		if code := codeText(opts.Protection); code == "" {
			protection = security.Protection(ch.creds.DefaultProtection)
		} else {
			protection, _ = security.ParseProtection(code, protection)
		}
	}

	if opts.HasOwner {
		owner = ch.owner
	}

	aclChanged := opts.Default || ch.like != nil

	if opts.HasACL || opts.HasDelete {
		edited, missing, ok := ch.editACL(acl)
		if !ok {
			report.message(setNoSuchACE, "SET-W-NOSUCHACE, ACE for object "+o.Name+", does not exist")
			report.c.Printf("    %s\n", security.FormatACE(missing, ch.names, nil))
			report.message(setWriteErr, "SET-F-WRITEERR, error writing "+o.Name)

			return
		}

		acl, aclChanged = edited, true
	}

	if owner != profile.Owner || protection != profile.Protection || opts.HasProtection || opts.HasOwner || opts.Default || ch.like != nil {
		if err := o.SetOwnerProtection(owner, protection); err != nil {
			report.message(setWriteErr, "SET-F-WRITEERR, error writing "+o.Name, ssNOPRIV)

			return
		}
	}

	if aclChanged {
		if err := o.SetACL(security.EncodeACL(acl)); err != nil {
			report.message(setWriteErr, "SET-F-WRITEERR, error writing "+o.Name, ssNOPRIV)

			return
		}
	}

	if opts.Log {
		report.message(setModified, "SET-I-MODIFIED, "+o.Name+" modified")
	}
}

// refused reports a file the process may not change, each form as VMS
// 7.1's showed it (A.4, "Statuses and messages"): SET SECURITY's OPENIN,
// SET FILE/PROTECTION's PRONOTCHG and PROERR, SET PROTECTION's PRONOTCHG
// alone. SET FILE/OWNER_UIC and SET ACL take SET SECURITY's
// (unconfirmed).
func (ch *securityChange) refused(report *securityReport, name string) {
	switch {
	case ch.opts.Form == FormProtection:
		report.message(setProNotChg, "SET-E-PRONOTCHG, protection on "+name+" not changed", ssNOPRIV)
	case ch.opts.Form == FormFile && ch.opts.HasProtection:
		report.message(setProNotChg, "SET-E-PRONOTCHG, protection on "+name+" not changed", ssNOPRIV)
		report.message(setProErr, "SET-E-PROERR, error protecting "+name, ssNOPRIV)
	default:
		report.message(setOpenIn, "SET-E-OPENIN, error opening "+name+" as input", ssNOPRIV)
	}
}

// protectedOnly is acl's protected entries, which /DEFAULT and /LIKE keep
// (A.4: L1.DAT kept its PROTECTED ACE; L2.DAT kept its own before L1's).
func protectedOnly(acl []security.ACE) []security.ACE {
	var out []security.ACE

	for _, a := range acl {
		if a.Has(security.OptionProtected) {
			out = append(out, a)
		}
	}

	return out
}

// propagating is acl without its NOPROPAGATE entries, what /LIKE copies.
func propagating(acl []security.ACE) []security.ACE {
	var out []security.ACE

	for _, a := range acl {
		if !a.Has(security.OptionNoPropagate) {
			out = append(out, a)
		}
	}

	return out
}

// editACL makes /ACL's change to acl (A.4, "Changing profiles"):
//
//   - with /DELETE=ALL, every entry goes, and /ACL's entries are added;
//   - with /DELETE and /ACL's entries, each is deleted, matched as
//     $CHANGE_ACL matches (security.MatchACE), and one not there stops
//     the change (missing is it); with /DELETE alone, every entry but the
//     protected ones goes;
//   - with /REPLACE, each of /ACL's entries is replaced in place by
//     /REPLACE's, in turn;
//   - otherwise /ACL's entries are added, in order, before the first
//     entry or after /AFTER's; an entry already there for the same
//     identifiers (or of the same type, for any other kind) is taken out
//     first, as VMS 7.1 replaced (PROT93A,PROTECTED,EXECUTE) by
//     (PROT93A,NOPROPAGATE,DELETE).
func (ch *securityChange) editACL(acl []security.ACE) (out []security.ACE, missing []byte, ok bool) {
	opts := ch.opts

	switch {
	case opts.HasDelete && strings.HasPrefix("ALL", strings.ToUpper(opts.Delete)) && opts.Delete != "":
		acl = nil

	case opts.HasDelete:
		if len(ch.acl) == 0 {
			return protectedOnly(acl), nil, true
		}

		for _, want := range ch.acl {
			i := findACE(acl, want, security.MatchACE)
			if i < 0 {
				return nil, want, false
			}

			acl = append(acl[:i:i], acl[i+1:]...)
		}

		return acl, nil, true

	case opts.Replace != "":
		for k, want := range ch.acl {
			i := findACE(acl, want, security.MatchACE)
			if i < 0 {
				return nil, want, false
			}

			if k < len(ch.replace) {
				acl[i] = security.DecodeACE(ch.replace[k])
			}
		}

		return acl, nil, true
	}

	if len(ch.acl) == 0 {
		return acl, nil, true
	}

	for _, add := range ch.acl {
		if i := findACE(acl, add, sameSubject); i >= 0 {
			acl = append(acl[:i:i], acl[i+1:]...)
		}
	}

	at := 0

	if ch.after != nil {
		i := findACE(acl, ch.after, security.MatchACE)
		if i < 0 {
			return nil, ch.after, false
		}

		at = i + 1
	}

	added := make([]security.ACE, 0, len(ch.acl))
	for _, add := range ch.acl {
		added = append(added, security.DecodeACE(add))
	}

	out = append(append(append([]security.ACE{}, acl[:at]...), added...), acl[at:]...)

	return out, nil, true
}

// findACE is the index of acl's first entry match says is want, or -1.
func findACE(acl []security.ACE, want []byte, match func(ace, want []byte) bool) int {
	for i, a := range acl {
		if match(a.Data, want) {
			return i
		}
	}

	return -1
}

// sameSubject reports whether ace is an entry an added entry want takes
// the place of: an identifier entry for the same identifiers, whatever
// its options and access; a default protection entry or a creator entry,
// of which an ACL has one; any other kind by $CHANGE_ACL's matching
// (unconfirmed beyond identifier entries).
func sameSubject(ace, want []byte) bool {
	a, w := security.DecodeACE(ace), security.DecodeACE(want)
	if a.Type != w.Type {
		return false
	}

	switch w.Type {
	case security.TypeIdentifier:
		if len(a.Identifiers) != len(w.Identifiers) {
			return false
		}

		for i := range a.Identifiers {
			if a.Identifiers[i] != w.Identifiers[i] {
				return false
			}
		}

		return true
	case security.TypeDefaultProtection, security.TypeCreator:
		return true
	}

	return security.MatchACE(ace, want)
}

// rightsNames is the process's rights database as security.Names, for
// an ACE's identifiers; SYSTEM's names alone without a process.
type rightsNames struct{ c *Context }

func (n rightsNames) IdentifierValue(name string) (uint32, bool) {
	if n.c.RTL == nil {
		return 0, false
	}

	v, status := n.c.RTL.IdentifierValue(name)

	return v, status == 0
}

func (n rightsNames) IdentifierName(id uint32) (string, bool) {
	if n.c.RTL == nil {
		return "", false
	}

	return n.c.RTL.IdentifierName(id)
}
