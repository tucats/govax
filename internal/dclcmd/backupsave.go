package dclcmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/ondisk"
)

// BACKUP's save (docs/PHASE-69 - backup save sets.md, subtask 11): the
// files the input specifier names on a mounted volume, chosen by the
// input file-selection qualifiers, written into a new save set
// (internal/backup's Writer) as VMS V7.1's BACKUP V6.2 writes one, with
// its messages (testdata/backup69's log, Part 2).
//
// # What's saved
//
// Each element of the input list is searched in turn, taking the device
// and directory the element before it gave when it has none (rms.SaveSearch:
// a directory's entries in order, every version unless one is given, a
// [dir...] search going into each subdirectory where its entry is), and
// each starts a new save set block, as VMS's LIST.BCK shows. Of the
// files found, /EXCLUDE, /BY_OWNER, and /SINCE or /BEFORE (on the date
// /CREATED, /MODIFIED, /EXPIRED, or /BACKUP names, /MODIFIED by default)
// choose; a directory a [dir...] search goes into is saved whatever they
// say (B69NONE.BCK and B69BYO.BCK hold SUB1.DIR and SUB2.DIR), and the
// manual says /EXCLUDE never excludes a directory file. A file marked
// NOBACKUP is saved without its data, with a NOBACKUP notice whether or
// not /LOG is given, unless /IGNORE=NOBACKUP.
//
// # The save set
//
// The summary records the command as DCL hands it to BACKUP: VMS wrote
// /COMMENT="Phase 69's probe, with a comment" as /COMMENT=Phase 69's
// probe, with a comment; govax drops the quotes, uppercases what's
// outside them, and makes each run of blanks one (the last two
// unconfirmed). The user, UIC, time, system version, CPU ID, block and
// group sizes, and buffer count are the process's and the save's; the
// device written on is the output's full name ("_SIMVAX$DUA1:"), and a
// node name is recorded only on a network node, which govax isn't.
// BACKUP's version is VMS 7.1's, V6.2, the BACKUP govax follows.
// /BLOCK_SIZE is 2048 to 65535, rounded up to a multiple of 512 (the
// manual: 10000 is 10240), 32256 by default; /GROUP_SIZE 0 to 100, 10 by
// default; /CRC writes data CRCs, /NOCRC (the default, as VMS 7.1's
// was) none. The save set's file is owned by the process, or by the
// output's /BY_OWNER UIC.
//
// # Messages
//
// With /LOG, COPIED for each file once the save set block holding its
// end is written, HEADCOPIED for a file saved without its data (so every
// NOBACKUP notice comes before them, as in VMS's log). A file another
// process has open for writing is OPENIN with -SYSTEM-W-ACCONFLICT, not
// saved, or with /IGNORE=INTERLOCK saved after an ACCONFLICT warning. An
// input element that finds nothing is SEARCHFAIL with RMS's reason; a
// save that saves no file at all ends NOFILES (both unconfirmed).

// Saving's defaults and limits (the manual's /BLOCK_SIZE and
// /GROUP_SIZE).
const (
	maxGroupSize = 100

	// backupVersion is the BACKUP version a save set records: VMS
	// V7.1's, whose save sets govax's follow.
	backupVersion = "V6.2"
)

// saveSaveSet is a BACKUP save: the input files into the output save set.
func (c *Context) saveSaveSet(cmd BackupCommand) error {
	summary, err := c.saveSummary(cmd)
	if err != nil {
		return err
	}

	filter, err := c.saveFilter(cmd)
	if err != nil {
		return err
	}

	loc, err := c.ContainerSession.Locate(cmd.Output.Text, cmd.Output.Host)
	if err != nil {
		return FileFailure(err, cmd.Output.Text)
	}

	owner, err := c.saveSetOwner(cmd)
	if err != nil {
		return err
	}

	out, err := c.ContainerSession.CreateSaveSet(loc, rms.SaveSetOptions{BlockSize: int(summary.BlockSize), Owner: &owner})
	if err != nil {
		return backupMessageWith(vmsdef.BackupOpenOut, fileErrorCauses(err), saveSetDisplayName(c.ContainerSession, loc))
	}

	summary.Name = saveSetName(out.Name, loc.Host)
	summary.Device = c.writtenOn(out.Device)

	var data bytes.Buffer

	w, err := backup.NewWriter(&data, summary, cmd.WriteCRC)
	if err != nil {
		_ = out.Finish(nil)

		return err
	}

	s := &backupSave{backupRestore: backupRestore{c: c, log: cmd.Log}, cmd: cmd, filter: filter, w: w}
	w.Saved = s.saved

	previous := ""

	for i, input := range cmd.Inputs {
		if i > 0 {
			if err := w.EndBlock(); err != nil {
				break
			}
		}

		input = stickyDefaults(input, previous)
		previous = input

		s.search(input)
	}

	if err := w.Close(); err != nil {
		_ = out.Finish(data.Bytes())

		return err
	}

	if err := out.Finish(data.Bytes()); err != nil {
		return err
	}

	return s.finish()
}

// backupSave is one save's progress.
type backupSave struct {
	backupRestore

	cmd    BackupCommand
	filter saveFilter
	w      *backup.Writer

	// queued are the files written to the save set and not yet reported
	// saved, oldest first: the Writer reports them in the order they
	// were added.
	queued []queuedFile

	// any is set once a file is written.
	any bool
}

// queuedFile is a file waiting for its block to be written: its name
// in messages and whether only its header was saved.
type queuedFile struct {
	name       string
	headerOnly bool
}

// search saves the files input names.
func (s *backupSave) search(input string) {
	loc, err := s.c.ContainerSession.Locate(input, s.cmd.Input.Host)
	if err == nil && loc.Host {
		err = rms.ErrSaveFromHost
	}

	if err == nil {
		err = s.c.ContainerSession.SaveSearch(input, s.file)
	}

	if err == nil {
		return
	}

	var fileErr *rms.DCLFileError
	if errors.As(err, &fileErr) {
		s.messageWith(vmsdef.BackupSearchFail, fileErrorCauses(err), fileErr.Name)

		return
	}

	failure := BackupLine(vmsdef.BackupSearchFail, strings.ToUpper(input))
	s.c.Printf("%s\n-%s\n", failure, strings.TrimPrefix(FileFailure(err, input).Error(), "%"))
	s.note(vmsdef.BackupSearchFail)
}

// file saves f if the selection takes it.
func (s *backupSave) file(f *rms.SaveFile) error {
	a := f.Attributes

	if !f.Walked && !s.filter.takes(a) {
		return nil
	}

	var flags backup.RecordFlag
	if a.Directory() {
		flags = backup.RecordDirectory
	}

	headerOnly := a.NoBackup() && !s.cmd.IgnoreNoBackup

	var data []byte

	if headerOnly {
		s.message(vmsdef.BackupNoBackup, f.Name)
	} else {
		read, conflict, err := f.Read(s.cmd.IgnoreInterlock)

		switch {
		case err != nil && conflict:
			s.messageWith(vmsdef.BackupOpenIn&^7|2, []uint32{vmsdef.Symbols["SS$_ACCONFLICT"]}, f.Name)

			return nil
		case err != nil:
			return err
		case conflict:
			s.message(vmsdef.BackupAccConflict, f.Name)
		}

		data = read
	}

	s.queued = append(s.queued, queuedFile{name: f.Name, headerOnly: headerOnly})
	s.any = true

	return s.w.Add(a, flags, data)
}

// saved is the Writer's report that a file's last record is written.
func (s *backupSave) saved(string) {
	q := s.queued[0]
	s.queued = s.queued[1:]
	s.files++

	if !s.log {
		return
	}

	if q.headerOnly {
		s.message(vmsdef.BackupHeadCopied, q.name)
	} else {
		s.message(vmsdef.BackupCopied, q.name)
	}
}

// messageWith shows one of BACKUP's messages with the conditions causes
// on the lines after it.
func (s *backupSave) messageWith(code uint32, causes []uint32, args ...any) {
	s.message(code, args...)

	for _, cause := range causes {
		s.c.Printf("%s\n", ConditionLine(cause))
	}
}

// note counts code toward the command's status without showing it.
func (s *backupSave) note(code uint32) {
	if backupSeverityRank(code) > backupSeverityRank(s.status) {
		s.status = code
	}
}

// finish ends a save: NOFILES when the searches found files and none
// was saved (/SINCE=TOMORROW), naming the first input specifier as
// expanded, then the status.
func (s *backupSave) finish() error {
	if !s.any && s.status == 0 {
		s.message(vmsdef.BackupNoFiles, saveSetDisplayName(s.c.ContainerSession, rms.FileLocation{Name: s.cmd.Inputs[0]}))
	}

	if s.status == 0 {
		if s.c.SetStatus != nil {
			s.c.SetStatus(ssNormal | stsInhibitMsg)
		}

		return nil
	}

	return vmserrors.InhibitMessage(vmserrors.ConditionError{
		Status: s.status | stsInhibitMsg, Text: vmsdef.FormatMessage(s.status),
	})
}

// fileErrorCauses are the RMS conditions an error carries for BACKUP's
// message lines ("-RMS-E-DNF, directory not found"), none if it has no
// RMS status.
func fileErrorCauses(err error) []uint32 {
	var fileErr *rms.DCLFileError
	if !errors.As(err, &fileErr) {
		return nil
	}

	var causes []uint32
	if fileErr.Status != 0 {
		causes = append(causes, fileErr.Status)
	}

	if fileErr.STV != 0 && fileErr.STV != fileErr.Status {
		causes = append(causes, fileErr.STV)
	}

	return causes
}

// stickyDefaults gives an input list's element with no device or
// directory of its own the ones the element before it named, as DCL's
// lists of file specifications take them ("[B69]TEXT.TXT,PI.*" is
// [B69]PI.*; unconfirmed for BACKUP, the probe's list naming both).
func stickyDefaults(spec, previous string) string {
	if previous == "" || strings.ContainsAny(spec, ":[<\"/") {
		return spec
	}

	if i := strings.LastIndexAny(previous, "]>"); i >= 0 {
		return previous[:i+1] + spec
	}

	if i := strings.LastIndexByte(previous, ':'); i >= 0 {
		return previous[:i+1] + spec
	}

	return spec
}

// saveSetDisplayName is the name OPENOUT shows for an output save set
// that couldn't be made.
func saveSetDisplayName(s *rms.Session, loc rms.FileLocation) string {
	if loc.Host {
		return loc.Name
	}

	name, err := s.ExpandName(loc.Name)
	if err != nil {
		return strings.ToUpper(loc.Name)
	}

	return name
}

// saveSetName is the save set's name its summary and blocks record: its
// file's name and type, "B69.BCK".
func saveSetName(full string, host bool) string {
	if host {
		return strings.ToUpper(filepath.Base(full))
	}

	if i := strings.LastIndexAny(full, "]>:"); i >= 0 {
		full = full[i+1:]
	}

	name, _, _ := strings.Cut(full, ";")

	return name
}

// writtenOn is the summary's device: the output's full device name
// ("_SIMVAX$DUA1:", as $GETDVI's FULLDEVNAM writes it, with a colon), or
// for a host save set govax's HOST in its place.
func (c *Context) writtenOn(device string) string {
	if device == "" {
		device = "HOST"
	}

	if node := c.nodeName(); node != "" {
		return "_" + node + "$" + device + ":"
	}

	return "_" + device + ":"
}

// nodeName is the node's name, "" without a process.
func (c *Context) nodeName() string {
	if c.RTL == nil {
		return ""
	}

	return c.RTL.NodeName
}

// saveSummary is the summary of the save cmd asks for, all but its
// name and device, which the output save set's file gives.
func (c *Context) saveSummary(cmd BackupCommand) (backup.Summary, error) {
	blockSize := backup.DefaultBlockSize
	if cmd.BlockSize != nil {
		n := *cmd.BlockSize
		if n < backup.MinBlockSize || n > backup.MaxBlockSize {
			return backup.Summary{}, backupMessage(vmsdef.BackupBlockSize)
		}

		blockSize = min((n+511)&^511, backup.MaxBlockSize)
	}

	groupSize := backup.DefaultGroupSize
	if cmd.GroupSize != nil {
		groupSize = *cmd.GroupSize
		if groupSize < 0 || groupSize > maxGroupSize {
			return backup.Summary{}, backupMessage(vmsdef.BackupGroupSize)
		}
	}

	user := "SYSTEM"
	if c.RTL != nil && c.RTL.Process != nil && c.RTL.Process.Username != "" {
		user = c.RTL.Process.Username
	}

	var cpuID uint32
	if c.RTL != nil {
		cpuID = c.RTL.SystemID()
	}

	date := c.now()
	if date == 0 {
		date = vmsdef.Time(time.Now())
	}

	return backup.Summary{
		Command:       recordedCommand(cmd.Command),
		Comment:       cmd.Comment,
		UserName:      user,
		UserUIC:       c.processUIC(),
		Date:          date,
		OpSys:         backup.OpSysVAX,
		SystemVersion: corevms.VersionString,
		CPUID:         cpuID,
		BackupVersion: backupVersion,
		BlockSize:     uint32(blockSize),
		GroupSize:     uint16(groupSize),
		Buffers:       uint16(backup.BuffersFor(blockSize)),
	}, nil
}

// saveSetOwner is the save set file's owner: the output's /BY_OWNER UIC,
// or the process's.
func (c *Context) saveSetOwner(cmd BackupCommand) (ondisk.Uic, error) {
	if cmd.OutputOwner == nil || strings.TrimSpace(*cmd.OutputOwner) == "" {
		return savedUIC(c.processUIC()), nil
	}

	uic, err := ondisk.ParseUic(*cmd.OutputOwner)
	if err != nil {
		return uic, vmserrors.ConditionError{Status: ssIVIDENT, Text: strings.TrimPrefix(ConditionLine(ssIVIDENT), "-")}
	}

	return uic, nil
}

// recordedCommand is a command line as BACKUP records it: as DCL hands
// it over, its quoted strings' quotes gone ("" a quote), the rest upper
// case, each run of blanks one blank.
func recordedCommand(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "$"))

	var (
		b      strings.Builder
		quoted bool
		blank  bool
	)

	for i := 0; i < len(line); i++ {
		ch := line[i]

		if !quoted && (ch == ' ' || ch == '\t') {
			blank = true

			continue
		}

		if blank && b.Len() > 0 {
			b.WriteByte(' ')
		}

		blank = false

		switch {
		case ch == '"' && quoted && i+1 < len(line) && line[i+1] == '"':
			b.WriteByte('"')
			i++
		case ch == '"':
			quoted = !quoted
		case quoted:
			b.WriteByte(ch)
		default:
			b.WriteString(strings.ToUpper(string(ch)))
		}
	}

	return b.String()
}

// saveFilter is a save's input file-selection qualifiers.
type saveFilter struct {
	exclude backupSelection
	owner   *uicPattern

	// since and before are /SINCE's and /BEFORE's times, 0 when not
	// given; date picks the file's date they test.
	since, before uint64
	date          BackupDateKind
}

// saveFilter reads cmd's input file-selection qualifiers.
func (c *Context) saveFilter(cmd BackupCommand) (saveFilter, error) {
	f := saveFilter{exclude: newBackupSelection(nil, cmd.Exclude), date: cmd.DateKind}

	if cmd.InputOwner != nil {
		p, err := parseUICPattern(*cmd.InputOwner, c.processUIC())
		if err != nil {
			return f, vmserrors.ConditionError{Status: ssIVIDENT, Text: strings.TrimPrefix(ConditionLine(ssIVIDENT), "-")}
		}

		f.owner = &p
	}

	for _, t := range []struct {
		text string
		to   *uint64
	}{{cmd.Since, &f.since}, {cmd.Before, &f.before}} {
		if t.text == "" {
			continue
		}

		v, ok := c.parseTime(t.text)
		if !ok {
			return f, vmserrors.New(vmserrors.CLI_IVATIME)
		}

		*t.to = v
	}

	return f, nil
}

// parseTime reads a DCL time, through the Context's ParseTime.
func (c *Context) parseTime(text string) (uint64, bool) {
	if c.ParseTime == nil {
		return 0, false
	}

	return c.ParseTime(text)
}

// takes reports whether the selection saves a file with attributes a.
// /BACKUP's /BEFORE also takes a file never backed up, and its /SINCE
// never does (the manual's /BACKUP).
func (f saveFilter) takes(a backup.Attributes) bool {
	if _, ok := f.exclude.selected(a.Name); !ok && !a.Directory() {
		return false
	}

	if f.owner != nil && !f.owner.matches(a.Owner) {
		return false
	}

	date := map[BackupDateKind]uint64{
		BackupModified: a.Revised, BackupBackedUp: a.Backup,
		BackupCreated: a.Created, BackupExpired: a.Expires,
	}[f.date]

	if f.since != 0 && (date < f.since || f.date == BackupBackedUp && date == 0) {
		return false
	}

	if f.before != 0 && date >= f.before && !(f.date == BackupBackedUp && date == 0) {
		return false
	}

	return true
}

// uicPattern is /BY_OWNER's UIC: a group and member, either "*".
type uicPattern struct {
	group, member       uint16
	anyGroup, anyMember bool
}

// parseUICPattern reads /BY_OWNER's value: "[g,m]" in octal, either part
// "*", or a UIC ondisk reads; "" is the process's own (own).
func parseUICPattern(text string, own uint32) (uicPattern, error) {
	text = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(text), " ", ""))
	if text == "" {
		return uicPattern{group: uint16(own >> 16), member: uint16(own)}, nil
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(text, "["), "]")
	if g, m, ok := strings.Cut(inner, ","); ok && (g == "*" || m == "*") {
		var p uicPattern

		for _, part := range []struct {
			text     string
			value    *uint16
			wildcard *bool
		}{{g, &p.group, &p.anyGroup}, {m, &p.member, &p.anyMember}} {
			if part.text == "*" {
				*part.wildcard = true

				continue
			}

			n, err := strconv.ParseUint(part.text, 8, 16)
			if err != nil {
				return p, err
			}

			*part.value = uint16(n)
		}

		return p, nil
	}

	uic, err := ondisk.ParseUic(text)
	if err != nil {
		return uicPattern{}, err
	}

	return uicPattern{group: uic.Group, member: uic.Member}, nil
}

// matches reports whether uic, group in the high word, is one p names.
func (p uicPattern) matches(uic uint32) bool {
	return (p.anyGroup || uint16(uic>>16) == p.group) && (p.anyMember || uint16(uic) == p.member)
}
