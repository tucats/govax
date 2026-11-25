package dclcmd

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/diff"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// DIFFERENCES (docs/planning/PHASE-116 - DIFFERENCES.md): two files compared
// record by record, and the records that don't match listed. The comparison
// and the listing are internal/diff's; this is the command: its qualifiers,
// the files (the second's missing fields from the first, or with one file
// its next lower version), the listing's file, the messages, and the status,
// as VMS 7.1 had them in the phase's probe (testdata/dif116).
//
// The order things are checked in follows VMS's: what DCL itself refuses
// first (a number that isn't one, /SLP with another form, a keyword it
// doesn't know), then what the utility refuses of its qualifiers (/WINDOW=0, a
// comment delimiter), its warnings about /MERGED and /PARALLEL, then the
// files, and only then the listing's file, so a missing input makes none
// (O07).

// DifferencesValue is a qualifier's value as the command gave it.
type DifferencesValue struct {
	// Given says the qualifier was given, Negated that it was its /NO
	// form, and Text is its value ("" for none).
	Given, Negated bool
	Text           string
}

// DifferencesRequest is DIFFERENCES' parameters and qualifiers.
type DifferencesRequest struct {
	// Master and Revision are the two files; Revision is "" for one.
	Master, Revision string

	// MasterBar and RevisionBar are each file's /CHANGE_BAR, its values.
	MasterBar, RevisionBar DifferencesValue
	MasterBarList          []string
	RevisionBarList        []string

	Ignore           DifferencesValue
	IgnoreList       []string
	CommentDelimiter DifferencesValue
	CommentList      []string
	Mode             DifferencesValue
	ModeList         []string
	Separated        DifferencesValue
	SeparatedList    []string

	Match, Merged, MaximumDifferences, Parallel, Width, Window DifferencesValue

	Number, Output, SLP DifferencesValue
}

// differences is one run of the command.
type differences struct {
	c   *Context
	req DifferencesRequest

	match, merged, parallel int32
	width                   int
	window, maxDifferences  uint32

	ignore     []string // /IGNORE's keywords as the echo shows them
	delimiters []string // /COMMENT_DELIMITER's, as the echo shows them
	modes      []string // /MODE's, as the echo shows them
	separated  []string // /SEPARATED's, MASTER and REVISION, in that order
}

// Differences runs DIFFERENCES.
func (c *Context) Differences(req DifferencesRequest) error {
	d := &differences{c: c, req: req, match: 3, merged: 1, width: 132}

	if err := d.qualifiers(); err != nil {
		return err
	}

	if strings.ContainsAny(req.Master, "*%") || strings.Contains(req.Master, "...") {
		return vmserrors.ConditionError{Status: vmsdef.DIFNoWild, Text: "DIFF-F-NOWILD, no wildcard permitted"}
	}

	master, revision, err := d.inputs()
	if err != nil {
		return err
	}

	res := diff.Compare(master.Records, revision.Records, diff.Options{
		Match: d.match, Window: int(d.window), MaxDifferences: int(d.maxDifferences),
	})

	l := diff.NewListing(d.width)
	diff.List(l, master, revision, res, d.forms())
	diff.Summary(l, res)

	outName, err := d.output(master.Name)
	if err != nil {
		return err
	}

	// A comparison /MAXIMUM_DIFFERENCES stopped lists its last section with
	// no matched records after it, and the echo says /MERGED=0 (testdata/
	// dif116's L01_1 to L01_9, XG211).
	if res.Stopped {
		d.merged = 0
	}

	diff.Echo(l, d.echo(outName), []string{
		master.Name + changeBarEcho(req.MasterBar, req.MasterBarList),
		revision.Name + changeBarEcho(req.RevisionBar, req.RevisionBarList),
	})

	if err := d.write(outName, l); err != nil {
		return err
	}

	status := uint32(vmsdef.DIFIdentical)

	switch {
	case res.Stopped:
		// After the listing, even one written to SYS$OUTPUT (L04).
		c.Printf("%%DIF-W-MAXDIF, %s\n", vmsdef.DifferencesMessages["MAXDIF"])

		status = vmsdef.DIFMaxDif
	case len(res.Sections) > 0:
		status = vmsdef.DIFDifferent
	}

	if c.SetStatus != nil {
		c.SetStatus(status)
	}

	return nil
}

// qualifiers reads and checks the qualifiers, in VMS's order.
func (d *differences) qualifiers() error {
	req := d.req

	numbers := []struct {
		v   DifferencesValue
		set func(int32)
	}{
		{req.Match, func(n int32) { d.match = n }},
		{req.Merged, func(n int32) { d.merged = n }},
		{req.MaximumDifferences, func(n int32) { d.maxDifferences = uint32(n) }},
		{req.Parallel, func(n int32) { d.parallel = n }},
		{req.Width, func(n int32) { d.width = max(int(n), diff.MinWidth) }},
		{req.Window, func(n int32) { d.window = uint32(n) }},
	}

	for _, q := range numbers {
		if !q.v.Given || q.v.Negated || q.v.Text == "" {
			continue
		}

		n, err := differencesNumber(q.v.Text)
		if err != nil {
			return err
		}

		q.set(n)
	}

	// /SLP is an edit script, no listing: DCL refuses it with a listing's
	// form (N41 to N45).
	if on(req.SLP) && (on(req.Merged) || on(req.Parallel) || on(req.Separated) || on(req.Mode)) {
		return vmserrors.NewSegment(vmserrors.CLI_CONFLICT, "SLP")
	}

	if err := d.keywords(); err != nil {
		return err
	}

	if req.Window.Given && req.Window.Text != "" && d.window == 0 {
		return vmserrors.ConditionError{
			Status: vmsdef.DIFBadValue,
			Text:   "DIFF-F-BADVALUE, '" + req.Window.Text + "' is an invalid keyword value",
		}
	}

	if err := d.commentDelimiters(); err != nil {
		return err
	}

	// /MERGED and /PARALLEL no greater than /MATCH, compared as signed
	// numbers (N30, N31, N34, N36): a warning, and the value lowered.
	if d.merged > d.match {
		d.warnGreater("MRGGTMTCH", "MERGE", d.merged)
		d.merged = d.match
	}

	if d.parallel > d.match {
		d.warnGreater("PARGTMTCH", "PARALLEL", d.parallel)
		d.parallel = d.match
	}

	return nil
}

// warnGreater writes MRGGTMTCH or PARGTMTCH, as VMS wrote them:
//
//	%DIF-W-MRGGTMTCH, value of 4 for /MERGE is greater than /MATCH value of 3
//		/MERGE set to 3
func (d *differences) warnGreater(ident, name string, value int32) {
	text := strings.NewReplacer("!UL", "%d").Replace(vmsdef.DifferencesMessages[ident])
	d.c.Printf("%%DIF-W-%s, %s\n\t/%s set to %d\n", ident,
		fmt.Sprintf(text, uint32(value), uint32(d.match)), name, uint32(d.match))
}

// on says a qualifier was given in its positive form.
func on(v DifferencesValue) bool { return v.Given && !v.Negated }

// differencesNumber reads a qualifier's number as DCL reads one: decimal,
// with a sign; another is %DCL-W-NUMBER with the value as its segment (N32).
// It is kept in 32 bits, so /MATCH=-1 is 4294967295 shown, -1 compared.
func differencesNumber(text string) (int32, error) {
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, vmserrors.NewSegment(vmserrors.CLI_NUMBER, text)
	}

	return int32(uint32(n)), nil
}

// The keywords /IGNORE, /MODE, and /SEPARATED take, each in the order the
// echo shows them (N63, N64; A.4's answer 1; /MODE's by F22 and R13).
var (
	ignoreKeywords    = []string{"SPACING", "TRAILING_SPACES", "HEADER", "FORM_FEEDS", "CASE", "EXACT", "PRETTY", "COMMENTS", "BLANK_LINES", "EDITED"}
	modeKeywords      = []string{"OCTAL", "HEXADECIMAL", "ASCII"}
	separatedKeywords = []string{"MASTER", "REVISION"}
)

// keywords checks /IGNORE's, /MODE's, and /SEPARATED's keywords, any
// abbreviation of one, and puts them in the echo's order; a keyword none
// names is DCL's IVKEYW (N46 to N48).
func (d *differences) keywords() error {
	ignore, err := matchKeywords(d.req.IgnoreList, ignoreKeywords, true)
	if err != nil {
		return err
	}

	if d.req.CommentDelimiter.Given && !d.req.CommentDelimiter.Negated {
		ignore["COMMENTS"] = ""
	}

	for _, k := range ignoreKeywords {
		v, ok := ignore[k]

		switch {
		case !ok || k == "EDITED":
		case k == "HEADER" && v == "":
			d.ignore = append(d.ignore, "HEADER=2")
		case k == "HEADER":
			d.ignore = append(d.ignore, "HEADER="+v)
		default:
			d.ignore = append(d.ignore, k)
		}
	}

	modes, err := matchKeywords(d.req.ModeList, modeKeywords, false)
	if err != nil {
		return err
	}

	// The parallel form is ASCII's, so /PARALLEL adds it to the modes
	// given (F34: /PARALLEL/MODE=HEX echoes /MODE=(HEXADECIMAL,ASCII)).
	if len(modes) > 0 && on(d.req.Parallel) {
		modes["ASCII"] = ""
	}

	for _, k := range modeKeywords {
		if _, ok := modes[k]; ok {
			d.modes = append(d.modes, k)
		}
	}

	separated, err := matchKeywords(d.req.SeparatedList, separatedKeywords, false)
	if err != nil {
		return err
	}

	for _, k := range separatedKeywords {
		if _, ok := separated[k]; ok {
			d.separated = append(d.separated, k)
		}
	}

	return nil
}

// matchKeywords matches each value to one of keywords (any abbreviation),
// with "=value" after it if withValue, and returns them by keyword.
func matchKeywords(values, keywords []string, withValue bool) (map[string]string, error) {
	out := map[string]string{}

	for _, v := range values {
		v = strings.ToUpper(strings.TrimSpace(v))
		if v == "" {
			continue
		}

		name, value := v, ""
		if withValue {
			name, value, _ = strings.Cut(v, "=")
		}

		var found []string

		for _, k := range keywords {
			if strings.HasPrefix(k, name) {
				found = append(found, k)
			}
		}

		switch len(found) {
		case 0:
			return nil, vmserrors.NewSegment(vmserrors.CLI_IVKEYW, name)
		case 1:
			out[found[0]] = value
		default:
			return nil, vmserrors.NewSegment(vmserrors.CLI_ABKEYW, name)
		}
	}

	return out, nil
}

// commentNames are the characters /COMMENT_DELIMITER names with a keyword,
// and the keyword the echo shows for each (";" is shown quoted though
// SEMI_COLON names it: IE04).
var commentNames = map[string]byte{
	"COLON": ':', "COMMA": ',', "EXCLAMATION": '!', "FORM_FEED": '\f', "LEFT": '[',
	"RIGHT": ']', "SEMI_COLON": ';', "SLASH": '/', "SPACE": ' ', "TAB": '\t',
}

// commentShown is the echo's name for a comment character.
var commentShown = map[byte]string{
	':': "COLON", ',': "COMMA", '!': "EXCLAMATION", '\f': "FORM_FEED", '[': "LEFT",
	']': "RIGHT", '/': "SLASH", ' ': "SPACE", '\t': "TAB",
}

// commentDelimiters reads /COMMENT_DELIMITER's values: a character, or a
// keyword naming one; up to 32. More is TOOMNYDLM (N51), a value of more
// than one character that no keyword names is CMDSYNTAX (N50).
func (d *differences) commentDelimiters() error {
	if !d.req.CommentDelimiter.Given || d.req.CommentDelimiter.Negated {
		return nil
	}

	var chars []byte

	for _, v := range d.req.CommentList {
		if v == "" {
			continue
		}

		var ch byte

		switch {
		case len(v) == 1:
			ch = v[0]
		default:
			k, ok := uniqueKeyword(strings.ToUpper(v))
			if !ok {
				return vmserrors.ConditionError{
					Status: vmsdef.DIFCmdSyntax,
					Text: "DIF-F-CMDSYNTAX, " + vmsdef.DifferencesMessages["CMDSYNTAX"] +
						"\n-DIF-F-CMDLIN, " + vmsdef.DifferencesMessages["CMDLIN"] + "\n\t\t\\" + v + "\\",
				}
			}

			ch = commentNames[k]
		}

		chars = append(chars, ch)
	}

	if len(chars) > 32 {
		return vmserrors.ConditionError{
			Status: vmsdef.DIFTooMnyDlm,
			Text:   "DIF-F-TOOMNYDLM, " + vmsdef.DifferencesMessages["TOOMNYDLM"],
		}
	}

	for _, ch := range chars {
		if name, ok := commentShown[ch]; ok {
			d.delimiters = append(d.delimiters, name)
		} else {
			d.delimiters = append(d.delimiters, `"`+string(ch)+`"`)
		}
	}

	return nil
}

// uniqueKeyword is the comment keyword text abbreviates, if only one.
func uniqueKeyword(text string) (string, bool) {
	found := ""

	for k := range commentNames {
		if strings.HasPrefix(k, text) {
			if found != "" {
				return "", false
			}

			found = k
		}
	}

	return found, found != ""
}

// inputs reads the two files: the first as named; the second with the
// first's name (but not its version) as its default name, or with no second,
// the next lower version of the first (N01 to N09).
func (d *differences) inputs() (master, revision diff.Input, err error) {
	s := d.c.ContainerSession

	first, err := s.ReadConvertInput(d.req.Master, 0, false)
	if err != nil {
		return master, revision, d.openFailure(err, d.req.Master, "")
	}

	master = diffInput(first)

	if d.req.Revision == "" {
		lower, err := d.lowerVersion(first.Name)
		if err != nil {
			return master, revision, err
		}

		second, err := s.ReadConvertInput(lower, 0, false)
		if err != nil {
			return master, revision, d.openFailure(err, lower, "")
		}

		return master, diffInput(second), nil
	}

	dna, _ := rms.SplitVersion(first.Name)

	second, err := s.ReadInputDefaulted(d.req.Revision, dna)
	if err != nil {
		return master, revision, d.openFailure(err, d.req.Revision, dna)
	}

	return master, diffInput(second), nil
}

// diffInput is a file read as the listing takes it.
func diffInput(in *rms.ConvertInput) diff.Input {
	out := diff.Input{Name: in.Name}

	for _, r := range in.Records {
		out.Records = append(out.Records, r.Data)
	}

	return out
}

// lowerVersion is the full name of the highest version of name's file below
// name's own, or NOLOWVER.
func (d *differences) lowerVersion(name string) (string, error) {
	base, version := rms.SplitVersion(name)
	own, _ := strconv.Atoi(strings.TrimPrefix(version, ";"))

	names, _ := d.c.ContainerSession.Search(base + ";*")

	best, bestVersion := "", 0

	for _, n := range names {
		_, v := rms.SplitVersion(n)

		ver, err := strconv.Atoi(strings.TrimPrefix(v, ";"))
		if err == nil && ver < own && ver > bestVersion {
			best, bestVersion = n, ver
		}
	}

	if best == "" {
		return "", vmserrors.ConditionError{
			Status: vmsdef.DIFNoLowVer,
			Text:   "DIF-F-NOLOWVER, " + strings.Replace(vmsdef.DifferencesMessages["NOLOWVER"], "!AS", name, 1),
		}
	}

	return best, nil
}

// openFailure is an input that couldn't be opened, spec with default name
// dna, as VMS showed it (N12, N20 to N25): the name RMS expanded, RMS's
// status, and its secondary status for a directory not found.
//
//	%DIFF-F-OPENIN, error opening DUA1:[DIF116.IN]NOSUCH.TXT; as input
//	-RMS-E-FNF, file not found
func (d *differences) openFailure(err error, spec, dna string) error {
	var (
		failure    *rms.ConvertOpenError
		notMounted *rms.NotMountedError
		bad        *rms.NameError
	)

	name, sts, stv := strings.ToUpper(spec), uint32(0), uint32(0)

	switch {
	case errors.As(err, &failure):
		name, sts, stv = failure.Name, failure.Status, failure.STV
	case errors.Is(err, fs.ErrNotExist):
		sts = vmsdef.Symbols["RMS$_FNF"]
	case errors.As(err, &notMounted):
		// A device with no volume: RMS's DEV, the name as $PARSE writes
		// it (N23: DUA99:[X]Y.Z;).
		if parsed, _, perr := d.c.ContainerSession.Parse(spec, dna, "", true, false); perr == nil {
			name = parsed.String()
		}

		sts = vmsdef.Symbols["RMS$_DEV"]
	case errors.As(err, &bad):
		sts = bad.Status
	default:
		return FileFailure(err, spec)
	}

	text := "DIFF-F-OPENIN, error opening " + name + " as input\n" + ConditionLine(sts)
	if stv != 0 && sts == vmsdef.Symbols["RMS$_DNF"] {
		text += "\n" + ConditionLine(stv)
	}

	return vmserrors.ConditionError{Status: vmsdef.DIFOpenIn, Text: text}
}

// output is the listing's file: none (SYS$OUTPUT) without /OUTPUT or with
// /NOOUTPUT (N56); with /OUTPUT, its value with the master's name and type
// .DIF as defaults, in the default directory (O01 to O03). The file is made
// empty here, so its name and version can be in the echo, and written by
// write.
//
// /SLP's script goes to a file whatever /OUTPUT says: the master's name with
// type .DIF unless /OUTPUT names another (O04, O05).
func (d *differences) output(master string) (string, error) {
	if !on(d.req.Output) && !on(d.req.SLP) {
		return "", nil
	}

	name, _ := rms.SplitVersion(rms.NameType(master))
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[:i]
	}

	s := d.c.ContainerSession

	spec := ""
	if on(d.req.Output) {
		spec = d.req.Output.Text
	}

	if parsed, _, err := s.Parse(spec, name+".DIF", "", true, false); err == nil && !s.DefaultOnHost() {
		spec = parsed.String()
	}

	loc, err := s.LocateOutput(spec, false)
	if err != nil {
		return "", FileFailure(err, spec)
	}

	loc = loc.WithDefaultType("DIF")

	created, err := s.CreateRecordFile(loc, rms.TextRecords, nil)
	if err != nil {
		return "", objectFailureAs(vmserrors.CLI_BADFILESPEC, err, loc.Name)
	}

	return created.Name, nil
}

// write writes the listing to its file, or to SYS$OUTPUT: a user-mode
// SYS$OUTPUT naming a file, the image's own, gets it there, as VMS's image
// wrote it (O09).
func (d *differences) write(name string, l *diff.Listing) error {
	if name == "" {
		name = d.userOutput()
	}

	if name == "" {
		d.c.Printf("%s", l.Text())

		return nil
	}

	records := make([][]byte, len(l.Lines))
	for i, line := range l.Lines {
		records[i] = []byte(line)
	}

	_, err := d.c.ContainerSession.RewriteRecordFile(rms.FileLocation{Name: name}, rms.TextRecords, records)
	if err != nil {
		return objectFailureAs(vmserrors.CLI_BADFILESPEC, err, name)
	}

	return nil
}

// userOutput is the file a user-mode SYS$OUTPUT names for the image DCL
// would run, made empty here, or "" for none.
func (d *differences) userOutput() string {
	if d.c.Logicals == nil {
		return ""
	}

	e, err := d.c.Logicals.Translate("LNM$FILE_DEV", "SYS$OUTPUT", lnm.User, 0)
	if err != nil || e.Mode != lnm.User || len(e.Equivalences) == 0 {
		return ""
	}

	s := d.c.ContainerSession

	loc, err := s.LocateOutput(e.Equivalences[0].Value, false)
	if err != nil {
		return ""
	}

	created, err := s.CreateRecordFile(loc, rms.TextRecords, nil)
	if err != nil {
		return ""
	}

	return created.Name
}

// echo is the echo's qualifiers, each a piece or a list's pieces, in VMS's
// order (A.4's answer 1): each written only if given, or a default the
// listing uses (/IGNORE always, /MERGED for the merged form).
func (d *differences) echo(outName string) []string {
	req := d.req

	pieces := listPieces("/IGNORE=", d.ignore, true)

	if req.CommentDelimiter.Given && !req.CommentDelimiter.Negated {
		if len(d.delimiters) == 0 {
			pieces = append(pieces, "/COMMENT_DELIMITERS")
		} else {
			pieces = append(pieces, listPieces("/COMMENT_DELIMITERS=", d.delimiters, true)...)
		}
	}

	if on(req.Width) {
		pieces = append(pieces, fmt.Sprintf("/WIDTH=%d", d.width))
	}

	if on(req.Match) {
		pieces = append(pieces, fmt.Sprintf("/MATCH=%d", uint32(d.match)))
	}

	if on(req.MaximumDifferences) {
		pieces = append(pieces, fmt.Sprintf("/MAXIMUM_DIFFERENCES=%d", d.maxDifferences))
	}

	if d.mergedForm() {
		pieces = append(pieces, fmt.Sprintf("/MERGED=%d", uint32(d.merged)))
	}

	if len(d.modes) > 0 && !(len(d.modes) == 1 && d.modes[0] == "ASCII") {
		pieces = append(pieces, listPieces("/MODE=", d.modes, true)...)
	}

	// "/OUTPUT=" and the name are pieces of their own (F14 at /WIDTH=40).
	if outName != "" {
		pieces = append(pieces, "/OUTPUT=", outName)
	}

	if on(req.Parallel) {
		if req.Parallel.Text == "" {
			pieces = append(pieces, "/PARALLEL")
		} else {
			pieces = append(pieces, fmt.Sprintf("/PARALLEL=%d", uint32(d.parallel)))
		}
	}

	// /SEPARATED of both files, by default or named, is shown bare (F09).
	if on(req.Separated) {
		if len(d.separated) == 1 {
			pieces = append(pieces, "/SEPARATED="+d.separated[0])
		} else {
			pieces = append(pieces, "/SEPARATED")
		}
	}

	if on(req.Window) {
		pieces = append(pieces, fmt.Sprintf("/WINDOW=%d", d.window))
	}

	if req.Number.Given && req.Number.Negated {
		pieces = append(pieces, "/NONUMBER")
	}

	return pieces
}

// listPieces is a list qualifier's pieces: "/NAME=(A," "B," "C)", or with one
// value and !paren "/NAME=A". An empty list is "/NAME=()".
func listPieces(prefix string, values []string, paren bool) []string {
	if len(values) == 0 {
		return []string{prefix + "()"}
	}

	if len(values) == 1 && !paren {
		return []string{prefix + values[0]}
	}

	pieces := make([]string, len(values))

	for i, v := range values {
		end := ","
		if i == len(values)-1 {
			end = ")"
		}

		pieces[i] = v + end
	}

	pieces[0] = prefix + "(" + pieces[0]

	return pieces
}

// changeBarEcho is a file's /CHANGE_BAR in the echo: bare when given no
// value (F40), else its character quoted, `/CHANGE_BAR=("#")` (F41), and
// NUMBER or NONUMBER if given (unconfirmed: no case showed them).
func changeBarEcho(v DifferencesValue, values []string) string {
	if !on(v) {
		return ""
	}

	bar, number := "!", ""
	given := false

	for _, value := range values {
		switch {
		case value == "":
		case strings.EqualFold(value, "NUMBER"):
			number, given = ",NUMBER", true
		case strings.EqualFold(value, "NONUMBER"):
			number, given = ",NONUMBER", true
		default:
			bar, given = value, true
		}
	}

	if !given {
		return "/CHANGE_BAR"
	}

	return `/CHANGE_BAR=("` + bar + `"` + number + ")"
}

// mergedForm says the merged form is listed: asked for, or no other form
// was (F40: a file's /CHANGE_BAR alone lists only its change bars).
func (d *differences) mergedForm() bool {
	req := d.req

	return on(req.Merged) ||
		!(on(req.Parallel) || on(req.Separated) || on(req.MasterBar) || on(req.RevisionBar))
}

// forms is the listings the qualifiers ask for (internal/diff's List writes
// them in VMS's order).
func (d *differences) forms() diff.Forms {
	req := d.req
	noNumber := req.Number.Given && req.Number.Negated

	f := diff.Forms{
		Merged:        d.mergedForm(),
		MergedStyle:   diff.Style{After: d.merged, NoNumber: noNumber},
		Parallel:      on(req.Parallel),
		ParallelStyle: diff.Style{After: d.parallel, NoNumber: noNumber},
		NoNumber:      noNumber,
		MasterBar:     changeBar(req.MasterBar, req.MasterBarList, noNumber),
		RevisionBar:   changeBar(req.RevisionBar, req.RevisionBarList, noNumber),
	}

	if on(req.Separated) {
		f.SeparatedMaster, f.SeparatedRevision = len(d.separated) != 1, len(d.separated) != 1

		for _, k := range d.separated {
			f.SeparatedMaster = f.SeparatedMaster || k == "MASTER"
			f.SeparatedRevision = f.SeparatedRevision || k == "REVISION"
		}
	}

	return f
}

// changeBar is a file's /CHANGE_BAR: its character ("!" unless given), and
// its numbers, by its own NUMBER or NONUMBER, else by /[NO]NUMBER.
func changeBar(v DifferencesValue, values []string, noNumber bool) *diff.Bar {
	if !on(v) {
		return nil
	}

	bar := &diff.Bar{Char: '!', NoNumber: noNumber}

	for _, value := range values {
		switch {
		case value == "":
		case strings.EqualFold(value, "NUMBER"):
			bar.NoNumber = false
		case strings.EqualFold(value, "NONUMBER"):
			bar.NoNumber = true
		default:
			bar.Char = value[0]
		}
	}

	return bar
}
