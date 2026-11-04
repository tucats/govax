package dclcmd

import (
	"errors"
	"io/fs"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fdl"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// CREATE/FDL (docs/PHASE-88 - keyed index files.md, subtask 10b; the Record
// Management Utilities manual's chapter 5): an empty file made from an FDL
// file's description. The FDL file is read (its default type .FDL) and
// parsed (internal/fdl), its messages shown, and then, unless one was an
// error, RMS makes the file from it (rms.Session.CreateFDL).
//
// What VMS 7.1 showed (round 3 of the probe, testdata/idx88's IDX88C.COM,
// sections C2 to C4 and C13):
//
//	%FDL-F-OPENFDL, error opening DUA1:[R3]NOSUCH.FDL;     $STATUS %X10B4808C
//	-RMS-E-FNF, file not found
//
//	%FDL-E-UNPRIKW, unrecognized primary keyword in statement 1
//	 \FIEL\
//	%FDL-E-FDLERROR, error parsing FDL file                $STATUS %X00B48072
//
//	%FDL-W-UNQUAKW, unrecognized qualifier keyword in statement 2
//	 \SQUENTIAL\
//	%FDL-W-WARNING, parsed with warnings                   $STATUS %X00010001
//
//	%FDL-F-CREATE, error creating DUA1:[R3]C34.IDX;        $STATUS %X10B4806C
//	-RMS-F-NPK, no primary key defined for indexed file
//
//	%FDL-I-CREATED, DUA1:[R3]C23.DAT;1 created             (/LOG)
//
// A file made is $STATUS %X00010001. FDLERROR is shown by DCL, from the
// status, so its value hasn't STS$M_INHIB_MSG; the others are the utility's
// own, and have.

// The FDL facility's status values, as VMS 7.1's $STATUS showed them.
const (
	fdlFacility = 180 << 16 // VMS's facility field, bits 16 to 27

	fdlStatusCreate   = fdlFacility | 0x806C
	fdlStatusError    = fdlFacility | 0x8072
	fdlStatusOpenFDL  = fdlFacility | 0x808C
	fdlStatusNormal   = 0x00010001
	fdlSeverityFailed = 'F'
)

// CreateFDLRequest is CREATE/FDL's parameter and qualifiers.
type CreateFDLRequest struct {
	// FDL is /FDL's file; File the file to make, "" for the FDL file's
	// FILE NAME.
	FDL, File string

	// Log is /LOG: the file made is reported.
	Log bool
}

// CreateFDL implements CREATE/FDL.
func (c *Context) CreateFDL(req CreateFDLRequest) error {
	s := c.ContainerSession

	text, failure := c.readFDLFile(req.FDL)
	if failure != nil {
		return failure
	}

	def, msgs := fdl.Parse(text, fdl.Options{ParseTime: c.ParseTime})
	c.showFDLMessages(msgs)

	if fdl.Failed(msgs) {
		return vmserrors.ConditionError{Status: fdlStatusError, Text: fdl.Text("FDLERROR", 'E', "")}
	}

	if len(msgs) > 0 {
		c.Printf("%%%s\n", fdl.Text("WARNING", 'W', ""))
	}

	made := s.CreateFDL(rms.FDLCreate{Definition: def, Name: req.File, Owner: c.ownerUIC})
	if made.Status&1 == 0 {
		return vmserrors.ConditionError{
			Status: fdlStatusCreate | stsInhibitMsg,
			Text:   fdl.Text("CREATE", fdlSeverityFailed, made.Name) + "\n" + conditionLineWith(made.Status, made.STV),
		}
	}

	if req.Log {
		c.Printf("%%%s\n", fdl.Text("CREATED", 'I', made.Name))
	}

	if c.SetStatus != nil {
		c.SetStatus(fdlStatusNormal)
	}

	return nil
}

// showFDLMessages shows a parse's messages.
func (c *Context) showFDLMessages(msgs []fdl.Message) {
	for _, m := range msgs {
		for i, line := range m.Lines() {
			if i == 0 {
				line = "%" + line
			}

			c.Printf("%s\n", line)
		}
	}
}

// readFDLFile reads the FDL file spec names, its default type .FDL, as one
// text; a file that isn't there is OPENFDL with RMS's reason.
func (c *Context) readFDLFile(spec string) (string, error) {
	s := c.ContainerSession

	loc, err := s.Locate(spec, false)
	if err != nil {
		return "", FileFailure(err, spec)
	}

	loc = loc.WithDefaultType("FDL")

	records, _, err := s.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		var notFound *rms.NotFoundError

		switch {
		case loc.Host && errors.Is(err, fs.ErrNotExist):
			return "", openFDLFailure(loc.Name, vmsdef.Symbols["RMS$_FNF"], 0)
		case !loc.Host && errors.As(err, &notFound):
			missing := s.NotFound(loc.Name)

			return "", openFDLFailure(missing.Name, missing.Status, missing.STV)
		}

		return "", FileFailure(err, spec)
	}

	var b strings.Builder

	for _, r := range records {
		b.Write(r)
		b.WriteByte('\n')
	}

	return b.String(), nil
}

// openFDLFailure is OPENFDL for name, with RMS's status and any secondary
// status under it.
func openFDLFailure(name string, status, stv uint32) error {
	text := fdl.Text("OPENFDL", fdlSeverityFailed, name) + "\n" + ConditionLine(status)
	if stv != 0 {
		text += "\n" + ConditionLine(stv)
	}

	return vmserrors.ConditionError{Status: fdlStatusOpenFDL | stsInhibitMsg, Text: text}
}

// conditionLineWith is ConditionLine for an RMS status whose message takes
// the STV, "-RMS-F-FLG, invalid key definition flags for key = 0".
func conditionLineWith(code, stv uint32) string {
	return strings.Replace(ConditionLine(code), "!UL", strconv.FormatUint(uint64(stv), 10), 1)
}

// ownerUIC is FILE OWNER's UIC: "[200,201]" in octal, "[SYSTEM]" an
// identifier the rights database has.
func (c *Context) ownerUIC(text string) (uint32, bool) {
	inner := strings.TrimSuffix(strings.TrimPrefix(text, "["), "]")

	if group, member, ok := strings.Cut(inner, ","); ok {
		g, err1 := strconv.ParseUint(strings.TrimSpace(group), 8, 16)
		m, err2 := strconv.ParseUint(strings.TrimSpace(member), 8, 16)

		return uint32(g)<<16 | uint32(m), err1 == nil && err2 == nil
	}

	if c.RTL == nil {
		return 0, false
	}

	value, status := c.RTL.IdentifierValue(inner)

	return value, status == 0
}
