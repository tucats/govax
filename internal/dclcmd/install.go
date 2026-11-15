package dclcmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// INSTALL (docs/planning/PHASE-94 - privileges.md, Decision 9): the known
// file list (corevms/knownfiles.go). ADD (CREATE) makes an image known,
// with privileges of its own if /PRIVILEGED names some; REMOVE (DELETE)
// forgets it; LIST shows the list, or one file's entry. ADD and REMOVE
// need CMKRNL; LIST needs nothing. VMS's INSTALL is an image, so its
// $STATUS has STS$M_INHIB_MSG set, success too (%X10000001), as VMS 7.1's
// run showed (testdata/priv94's INSTALL sections).

// InstallOptions are an INSTALL command's subcommand, file, and qualifiers.
type InstallOptions struct {
	// Subcommand is ADD, CREATE, REMOVE, DELETE, or LIST.
	Subcommand string

	// File is the image's file specification; LIST may have none.
	File string

	// Full is LIST's /FULL.
	Full bool

	// Privileges is /PRIVILEGED's list of privilege names.
	Privileges []string

	// Attributes are ADD's other qualifiers that were given, by name
	// (OPEN, HEADER_RESIDENT, SHARED, ...), each true unless negated.
	Attributes map[string]bool
}

// InstallAttributes are ADD's qualifiers other than /PRIVILEGED, which
// the grammar has and the known file list records; each has the column
// INSTALL LIST shows it in, the attribute's abbreviation. Only Open, Hdr,
// Shar, and Prv's columns are VMS 7.1's (the probe's INSTALL LIST); the
// others' places are govax's, after them (unconfirmed).
var InstallAttributes = []struct {
	Qualifier, Column string
}{
	{"OPEN", "Open"},
	{"HEADER_RESIDENT", "Hdr"},
	{"SHARED", "Shar"},
	{"", "Prv"},
	{"PROTECTED", "Prot"},
	{"WRITEABLE", "Wrt"},
	{"ACCOUNTING", "Acnt"},
	{"EXECUTE_ONLY", "Exe"},
	{"RESIDENT", "Resid"},
}

// INSTALL's own messages. INSTALL's facility number and the values are
// govax's (unconfirmed: the probe's logs show the messages, not their
// values); FAIL is a warning and NOKFEFND an error, as the logs show them.
const installFacility uint32 = 0x10f

var (
	installFail     = installFacility<<16 | 0x8000 | 1<<3 // severity W, 0
	installNoKFE    = installFacility<<16 | 0x8000 | 2<<3 | 2
	installKFExists = installFacility<<16 | 0x8000 | 3<<3 | 2
)

// installSuccess is what a successful INSTALL leaves in $STATUS:
// SS$_NORMAL with STS$M_INHIB_MSG set, an image's exit status.
const installSuccess = 0x10000001

// Install is the INSTALL command.
func (c *Context) Install(opts InstallOptions) error {
	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	verb := strings.ToUpper(opts.Subcommand)

	switch verb {
	case "ADD", "CREATE":
		verb = "CREATE"
	case "REMOVE", "DELETE":
		verb = "DELETE"
	case "LIST":
	default:
		return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, verb)
	}

	if verb != "LIST" {
		if st := c.RTL.CommandPrivilege("INSTALL"); st != 0 {
			return installCondition(st, ConditionLine(st)[1:])
		}
	}

	if verb == "LIST" && opts.File == "" {
		c.listKnownFiles(c.RTL.KnownFiles.Entries(), opts.Full)
		c.installed()

		return nil
	}

	kf, err := c.knownFileFor(opts.File)
	if err != nil {
		return installFailure(verb, opts.File, err)
	}

	switch verb {
	case "CREATE":
		if err := c.installAdd(kf, opts); err != nil {
			return err
		}

	case "DELETE":
		if !c.RTL.KnownFiles.Remove(kf.Name) {
			return installFailed(verb, kf, installNoKFE)
		}

	case "LIST":
		entry, ok := c.RTL.KnownFiles.Find(kf.Name)
		if !ok {
			return installFailed(verb, kf, installNoKFE)
		}

		c.listKnownFiles([]corevms.KnownFile{entry}, opts.Full)
	}

	c.installed()

	return nil
}

// installed sets INSTALL's successful $STATUS.
func (c *Context) installed() {
	if c.SetStatus != nil {
		c.SetStatus(installSuccess)
	}
}

// installAdd puts kf on the known file list with opts' privileges and
// attributes.
func (c *Context) installAdd(kf corevms.KnownFile, opts InstallOptions) error {
	for _, item := range opts.Privileges {
		for _, name := range strings.Split(strings.Trim(item, "()"), ",") {
			name = strings.ToUpper(strings.TrimSpace(name))

			switch {
			case name == "":
			case name == "ALL":
				kf.Privileges |= corevms.AllPrivileges
			default:
				mask, ok := corevms.PrivilegeMask(name)
				if !ok {
					return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, name)
				}

				kf.Privileges |= mask
			}
		}
	}

	kf.Attributes = map[string]bool{}
	for name, on := range opts.Attributes {
		if on {
			kf.Attributes[name] = true
		}
	}

	if !c.RTL.KnownFiles.Add(kf) {
		return installFailed("CREATE", kf, installKFExists)
	}

	return nil
}

// knownFileFor finds the file spec names (its type .EXE by default) on a
// mounted volume and returns an entry for it with no privileges or
// attributes: its key, version, and the names INSTALL shows. INSTALL
// knows only volume files: a host file is RMS$_DEV (govax's choice).
func (c *Context) knownFileFor(spec string) (corevms.KnownFile, error) {
	s := c.ContainerSession
	if s == nil {
		return corevms.KnownFile{}, rmsStatusError(vmsdef.Symbols["RMS$_DEV"])
	}

	loc, err := s.Locate(spec, false)
	if err != nil {
		return corevms.KnownFile{}, err
	}

	if loc.Host {
		return corevms.KnownFile{}, rmsStatusError(vmsdef.Symbols["RMS$_DEV"])
	}

	found, err := s.LocateImageFile(s.ImageLocation(loc))
	if err != nil {
		var notFound *rms.NotFoundError
		if errors.As(err, &notFound) {
			return corevms.KnownFile{}, rmsStatusError(vmsdef.Symbols["RMS$_FNF"])
		}

		return corevms.KnownFile{}, err
	}

	return c.knownFileParts(found.Physical), nil
}

// knownFileParts is an entry for the file whose physical specification is
// physical ("DUA1:[000000]PRIV94I.EXE;1"): its key, its version, and its
// volume (DISK$label, or the device), directory, name, and type. A rooted
// directory ("[A.][B]") is shown as one ("A.B").
func (c *Context) knownFileParts(physical string) corevms.KnownFile {
	kf := corevms.KnownFile{Name: corevms.KnownFileKey(physical)}

	spec := strings.TrimPrefix(strings.ToUpper(physical), "_")

	device, rest, _ := strings.Cut(spec, ":")
	kf.Volume = device

	if name, ok := c.volumeLogicalName(device); ok {
		kf.Volume = name
	}

	if open := strings.IndexAny(rest, "[<"); open >= 0 {
		if end := strings.LastIndexAny(rest, "]>"); end > open {
			kf.Directory = strings.ReplaceAll(strings.ReplaceAll(rest[open+1:end], ".][", "."), ".><", ".")
			rest = rest[end+1:]
		}
	}

	file, version, _ := strings.Cut(rest, ";")
	kf.File, kf.Type, _ = strings.Cut(file, ".")

	var v uint16

	_, _ = fmt.Sscanf(version, "%d", &v)
	kf.Version = v

	return kf
}

// listKnownFiles writes INSTALL LIST's report of entries, as VMS 7.1
// laid it out: a blank line and a heading for each volume, directory, and
// type ("DISK$PRIV94:<000000>.EXE"), and under it a line per file, the
// name and version in 17 columns after three blanks, then the attributes'
// columns up to the last one it has. /FULL adds the entry's counts and
// privileges and a blank line.
//
// The access count is the activations since INSTALL ADD; "Current /
// Maximum shared" is 1 for an open file, 0 otherwise, over 0 (govax has
// no shared images), what VMS showed for an open, unshared image
// (unconfirmed for others).
func (c *Context) listKnownFiles(entries []corevms.KnownFile, full bool) {
	heading := ""

	for _, kf := range entries {
		h := kf.Volume + ":<" + kf.Directory + ">." + kf.Type
		if h != heading {
			fmt.Fprintf(c.Out, "\n%s\n", h)
			heading = h
		}

		fmt.Fprintf(c.Out, "   %-17s%s\n", fmt.Sprintf("%s;%d", kf.File, kf.Version), installColumns(kf))

		if !full {
			continue
		}

		open := 0
		if kf.Attributes["OPEN"] {
			open = 1
		}

		fmt.Fprintf(c.Out, "        Entry access count         = %d\n", kf.Accesses)
		fmt.Fprintf(c.Out, "        Current / Maximum shared   = %d / 0\n", open)

		if names := kf.PrivilegeNames(); len(names) > 0 {
			fmt.Fprintf(c.Out, "        Privileges = %s \n", strings.Join(names, " "))
		}

		fmt.Fprintln(c.Out)
	}
}

// installColumns is an entry's attribute columns: each column the entry
// has is its abbreviation and a blank, each it hasn't as many blanks,
// ending after the last it has ("Open Hdr      Prv ").
func installColumns(kf corevms.KnownFile) string {
	var b strings.Builder

	end := 0

	for _, a := range InstallAttributes {
		has := kf.Attributes[a.Qualifier]
		if a.Qualifier == "" {
			has = kf.Privileges != 0
		}

		if has {
			b.WriteString(a.Column + " ")
			end = b.Len()
		} else {
			b.WriteString(strings.Repeat(" ", len(a.Column)+1))
		}
	}

	return b.String()[:end]
}

// installFailed is INSTALL's failure for kf's entry: %INSTALL-W-FAIL,
// naming the subcommand and the file as LIST's heading shows it, over
// status's message.
func installFailed(verb string, kf corevms.KnownFile, status uint32) error {
	return installFailedText(verb, kf.Volume+":<"+kf.Directory+">"+kf.File+"."+kf.Type, installMessage(status))
}

// installFailure is INSTALL's failure for a file it couldn't find: FAIL
// naming the file as it was given, over err's message.
func installFailure(verb, spec string, err error) error {
	var se vmsStatusError
	if errors.As(err, &se) {
		return installFailedText(verb, strings.ToUpper(spec), ConditionLine(uint32(se)))
	}

	return installFailedText(verb, strings.ToUpper(spec), "-"+err.Error())
}

// installFailedText is %INSTALL-W-FAIL for file with line under it.
func installFailedText(verb, file, line string) error {
	return installCondition(installFail,
		"INSTALL-W-FAIL, failed to "+verb+" entry for "+file+"\n"+line)
}

// installMessage is the line of one of INSTALL's own errors.
func installMessage(status uint32) string {
	switch status {
	case installNoKFE:
		return "-INSTALL-E-NOKFEFND, Known File Entry not found"
	case installKFExists:
		return "-INSTALL-E-KFEXISTS, Known File Entry already exists"
	}

	return ConditionLine(status)
}

// installCondition is a failure of INSTALL's with text as its message:
// shown, and its status with STS$M_INHIB_MSG set, as an image's exit
// status is.
func installCondition(status uint32, text string) error {
	return vmserrors.ConditionError{Status: status | 0x10000000, Text: text}
}

// vmsStatusError is a failure that is a VMS condition value (RMS$_FNF),
// whose message is the system message file's.
type vmsStatusError uint32

func (e vmsStatusError) Error() string { return ConditionLine(uint32(e))[1:] }

// rmsStatusError is the failure whose condition value is status.
func rmsStatusError(status uint32) error { return vmsStatusError(status) }
