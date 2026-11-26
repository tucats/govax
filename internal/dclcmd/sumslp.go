package dclcmd

import (
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sumslp"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// EDIT/SUM (docs/planning/PHASE-117 - SUMSLP.md): VMS's SUMSLP, a batch
// editor. One input file is updated by the edit commands of one or more
// update files, and the result written as a new file, with a listing of what
// changed if asked for. The editing and the listing's layout are
// internal/sumslp's; this is the command: the files and their names, the
// messages, and the status, as VMS 7.1 had them in the phase's probe
// (testdata/sum117's COMMAND section, the phase doc's A.4.1).
//
// The names:
//
//   - The input has no defaults; a wildcard in it is %SUM-E-OPENIN with
//     RMS's WLD, and a file that can't be opened %SUM-I-OPENIN, either
//     ending the command with RMS's status (C17, C20).
//   - The first update file takes the input's device, directory, and name,
//     and the type .UPD (C01, C02, C14, C28); each later one the device and
//     directory of the one before, but not its type (C04, C16). A file that
//     can't be opened is %SUM-I-OPENIN, and the others are applied (C18,
//     C19); a wildcard ends the command, as the input's does (C21).
//     /UPDATE=SYS$INPUT reads the lines after the command in a procedure, to
//     a "/" line or the data's end (C29, C30).
//   - The output is the input's name and type, a new version, unless
//     /OUTPUT names another (its missing fields the input's, C08, C10, C12,
//     C13); /NOOUTPUT makes none (C11).
//   - /HEADER makes the output VFC records with a 12-byte fixed control,
//     sumslp.Head's (U11 to U14).
//   - The listing is made only with /LISTING, by default the input's name
//     with the type .LIS (C09, C12, C24).
//
// $STATUS is an RMS status with STS$M_INHIB_MSG set: success (%X10010001)
// after any run that finished, a failure's RMS status after one that didn't.
//
// Unconfirmed (every probe case ran in the input's directory): the output
// goes beside the input, as an editor's new version does, and the listing in
// the default directory, as a compiler's does.

// EditSumRequest is EDIT/SUM's parameter and qualifiers.
type EditSumRequest struct {
	// Input is the file to update.
	Input string

	// Update says /UPDATE was given, and UpdateList is its files ("" for a
	// bare /UPDATE).
	Update     bool
	UpdateList []string

	// Listing, Output, and Header are the qualifiers as given.
	Listing, Output, Header DifferencesValue
}

// The statuses EDIT/SUM leaves.
const (
	sumSuccess = 0x10010001 // RMS$_NORMAL, the message inhibited
	sumInput   = "SYS$INPUT:.;"
)

// EditSum runs EDIT/SUM.
func (c *Context) EditSum(req EditSumRequest) error {
	s := c.ContainerSession

	// No file is DCL's INSFPRM: SUMSLP's parameter has no prompt (the
	// probe's terminal session).
	if req.Input == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	if hasWildcard(req.Input) {
		return c.sumWildcard(req.Input, "")
	}

	in, err := s.ReadConvertInput(req.Input, 0, false)
	if err != nil {
		f, ok := c.openFailureOf(err, req.Input, "")
		if !ok {
			return FileFailure(err, req.Input)
		}

		return vmserrors.ConditionError{
			Status: f.sts | stsInhibitMsg,
			Text:   "SUM-I-OPENIN, " + sumOpenText(f.name) + "\n" + f.lines(),
		}
	}

	input := make([][]byte, len(in.Records))
	for i, r := range in.Records {
		input[i] = r.Data
	}

	parsed, _, _ := s.Parse(in.Name, "", "", true, false)

	var updates []sumslp.Update

	if req.Update {
		updates, err = c.sumUpdates(req.UpdateList, parsed)
		if err != nil {
			return err
		}
	}

	r := sumslp.Apply(input, updates)

	// The warnings are the listing's lines; with no listing they go to
	// SYS$OUTPUT (the probe's terminal session, Y08).
	if !on(req.Listing) {
		for _, m := range r.Messages() {
			c.Printf("%s\n", m)
		}
	}

	// An input record past SUMSLP's buffer stopped the pass: READERR with
	// RMS's RTB, which is the status, and the output and listing hold what
	// came before it (X00256 to X32767).
	status := uint32(sumSuccess)
	if r.Long > 0 {
		c.Printf("%%SUM-E-READERR, %s\n%s\n", sumReadText(in.Name), rtbLine(r.Long))
		status = vmsdef.Symbols["RMS$_RTB"] | stsInhibitMsg
	}

	if !(req.Output.Given && req.Output.Negated) {
		if err := c.sumOutput(req.Output.Text, parsed, r, on(req.Header)); err != nil {
			return err
		}
	}

	if on(req.Listing) {
		if err := c.sumListing(req.Listing.Text, in.Name, parsed, r.Lines); err != nil {
			return err
		}
	}

	if c.SetStatus != nil {
		c.SetStatus(status)
	}

	return nil
}

// sumUpdates reads the update files, each named with the defaults the one
// before it leaves (the first the input's, in): a file that can't be opened
// is reported and passed over, a wildcard ends the command.
func (c *Context) sumUpdates(specs []string, in rms.ParsedName) ([]sumslp.Update, error) {
	s := c.ContainerSession
	place := in.Node + in.Device + in.Directory

	var updates []sumslp.Update

	for _, spec := range specs {
		dna := place + in.Name + ".UPD"

		if spec != "" && c.InputStream != nil && c.InputStream(spec) {
			updates = append(updates, sumslp.Update{Name: sumInput, Records: c.sumInputRecords()})

			continue
		}

		if hasWildcard(spec) {
			return nil, c.sumWildcard(spec, dna)
		}

		named := spec
		if named == "" {
			named = dna
		}

		// The next file's device and directory are this one's, found or
		// not (C04: C04C.UPD looked for where C04B.ENH was).
		if p, _, err := s.Parse(named, dna, "", true, false); err == nil {
			place = p.Node + p.Device + p.Directory
		}

		u, err := s.ReadInputDefaulted(named, dna)
		if err != nil {
			f, ok := c.openFailureOf(err, named, dna)
			if !ok {
				return nil, FileFailure(err, named)
			}

			c.Printf("%%SUM-I-OPENIN, %s\n%s\n", sumOpenText(f.name), f.lines())

			continue
		}

		update := sumslp.Update{Name: u.Name}
		for _, r := range u.Records {
			update.Records = append(update.Records, r.Data)
		}

		// A record past SUMSLP's buffer is RMS's RTB, and the file's edits
		// are none of them applied; the run goes on (B30, W256).
		if long := longRecord(update.Records); long > 0 {
			c.Printf("%%SUM-E-READERR, %s\n%s\n", sumReadText(u.Name), rtbLine(long))

			continue
		}

		updates = append(updates, update)
	}

	return updates, nil
}

// sumInputRecords reads an update file from SYS$INPUT: a procedure's data
// lines after the command, to the update's "/" or the data's end. VMS's
// SUMSLP reads an update file twice, which a procedure's log shows; govax
// reads it once (Decision 8).
func (c *Context) sumInputRecords() [][]byte {
	var records [][]byte

	for {
		line, ok := c.inputLine()
		if !ok {
			return records
		}

		records = append(records, []byte(line))

		if strings.HasPrefix(line, "/") {
			return records
		}
	}
}

// sumWildcard is a wildcard in the input's or an update file's name, spec
// with default name dna: %SUM-E-OPENIN, and the command ends with RMS's WLD
// (C20, C21).
//
//	%SUM-E-OPENIN, error opening DUA1:[SUM117.IN]S1*.SRC; as input
//	-RMS-F-WLD, invalid wildcard operation
func (c *Context) sumWildcard(spec, dna string) error {
	name := strings.ToUpper(spec)
	if p, _, err := c.ContainerSession.Parse(spec, dna, "", true, false); err == nil {
		name = p.String()
	}

	wld := vmsdef.Symbols["RMS$_WLD"]

	return vmserrors.ConditionError{
		Status: wld | stsInhibitMsg,
		Text:   "SUM-E-OPENIN, " + sumOpenText(name) + "\n" + ConditionLine(wld),
	}
}

// longRecord is the length of the first record past sumslp.MaxRecord, or 0.
func longRecord(records [][]byte) int {
	for _, r := range records {
		if len(r) > sumslp.MaxRecord {
			return len(r)
		}
	}

	return 0
}

// rtbLine is RMS's RTB as READERR's second line:
//
//	-RMS-W-RTB, 300 byte record too large for user's buffer
func rtbLine(n int) string {
	return strings.Replace(ConditionLine(vmsdef.Symbols["RMS$_RTB"]), "!UL", strconv.Itoa(n), 1)
}

// sumReadText is READERR's text for a file's name.
func sumReadText(name string) string {
	return strings.Replace(vmsdef.SUMMessages["READERR"], "!AS", name, 1)
}

// sumOpenText is OPENIN's text for a file's name.
func sumOpenText(name string) string {
	return strings.Replace(vmsdef.SUMMessages["OPENIN"], "!AS", name, 1)
}

// sumOutput writes the output file: spec's name, its missing fields the
// input's name and type (beside the input), a new version. With /HEADER its
// records are VFC, each with SUMSLP's 12-byte fixed control (U11 to U14).
func (c *Context) sumOutput(spec string, in rms.ParsedName, r *sumslp.Result, headed bool) error {
	dna := in.Node + in.Device + in.Directory + in.Name + in.Type

	if headed {
		return c.sumWrite(spec, dna, r.Headed(), sumslp.HeaderSize)
	}

	return c.sumWrite(spec, dna, r.Output, 0)
}

// sumListing writes the listing: spec's name, its missing fields the input's
// name and the type .LIS, in the default directory.
func (c *Context) sumListing(spec, inName string, in rms.ParsedName, lines []sumslp.Line) error {
	h := sumslp.Heading{Run: time.Now(), Input: inName}

	if a, err := c.ContainerSession.FileAttributes(inName); err == nil && a.Created != 0 {
		h.Created = vmsdef.GoTime(a.Created)
	}

	return c.sumWrite(spec, in.Name+".LIS", sumslp.Listing(lines, h), 0)
}

// sumWrite makes a new file of records, spec named with default name dna:
// variable-length records with carriage return carriage control, whatever the
// input's form (the probe's FORMATS section), or VFC records whose first
// control bytes are their fixed control if control isn't 0.
func (c *Context) sumWrite(spec, dna string, records [][]byte, control int) error {
	s := c.ContainerSession

	name := spec
	if p, _, err := s.Parse(spec, dna, "", true, false); err == nil && !s.DefaultOnHost() {
		name = p.String()
	}

	loc, err := s.LocateOutput(name, false)
	if err != nil {
		return FileFailure(err, name)
	}

	if control > 0 {
		_, err = s.CreateVFCFile(loc, control, records)
	} else {
		_, err = s.CreateRecordFile(loc, rms.TextRecords, records)
	}

	if err != nil {
		return objectFailureAs(vmserrors.CLI_BADFILESPEC, err, loc.Name)
	}

	return nil
}

// hasWildcard says a file name has a wildcard.
func hasWildcard(spec string) bool {
	return strings.ContainsAny(spec, "*%") || strings.Contains(spec, "...")
}
