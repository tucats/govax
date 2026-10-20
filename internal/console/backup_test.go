package console

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/dclshell"
	"github.com/tucats/govax/internal/vmserrors"
)

// backupSets is where the probe's save sets are (testdata/backup69).
const backupSets = "../../testdata/backup69/vax/"

// TestBackupCommandParse: the probe's BACKUP commands (testdata/
// backup69/backup69.com) read into BackupCommands, each qualifier by
// where it stands.
func TestBackupCommandParse(t *testing.T) {
	g := loadEvaxGrammar(t)

	parse := func(line string) dclcmd.BackupCommand {
		t.Helper()

		r, err := g.Parse(line)
		if err != nil {
			t.Fatalf("Parse(%q): %v", line, err)
		}

		cmd, err := backupCommand(r, line)
		if err != nil {
			t.Fatalf("backupCommand(%q): %v", line, err)
		}

		return cmd
	}

	cmd := parse("BACKUP/LOG [B69...]*.* [SETS]B69.BCK/SAVE_SET")
	if !cmd.Log || cmd.Input.SaveSet || cmd.Output == nil || !cmd.Output.SaveSet ||
		cmd.Input.Text != "[B69...]*.*" || cmd.Output.Text != "[SETS]B69.BCK" || cmd.Command != "BACKUP/LOG [B69...]*.* [SETS]B69.BCK/SAVE_SET" {
		t.Errorf("a save: %+v", cmd)
	}

	cmd = parse(`BACKUP [B69...]*.* [SETS]B69K8.BCK/SAVE_SET/BLOCK_SIZE=8192/GROUP_SIZE=0/CRC/COMMENT="Phase 69's probe, with a comment"`)
	if cmd.BlockSize == nil || *cmd.BlockSize != 8192 || cmd.GroupSize == nil || *cmd.GroupSize != 0 ||
		!cmd.WriteCRC || cmd.CheckCRC || cmd.Comment != "Phase 69's probe, with a comment" {
		t.Errorf("output save-set qualifiers: %+v", cmd)
	}

	cmd = parse("BACKUP [B69...]*.*/EXCLUDE=(*.DAT,[B69.SUB1.SUB2]*.*)/BY_OWNER=[200,201]/SINCE=TODAY/CREATED [SETS]B69EXC.BCK/SAVE_SET/BY_OWNER=[1,4]")
	if !slices.Equal(cmd.Exclude, []string{"*.DAT", "[B69.SUB1.SUB2]*.*"}) || cmd.InputOwner == nil || *cmd.InputOwner != "[200,201]" ||
		cmd.Since != "TODAY" || cmd.DateKind != dclcmd.BackupCreated || cmd.OutputOwner == nil || *cmd.OutputOwner != "[1,4]" {
		t.Errorf("input file-selection qualifiers: %+v", cmd)
	}

	cmd = parse("BACKUP [B69]TEXT.TXT,[B69.SUB1]A.TXT [SETS]LIST.BCK/SAVE_SET")
	if !slices.Equal(cmd.Inputs, []string{"[B69]TEXT.TXT", "[B69.SUB1]A.TXT"}) || cmd.Input.Text != "[B69]TEXT.TXT" {
		t.Errorf("an input list: %+v", cmd)
	}

	cmd = parse("BACKUP [B69]NOBACK.TXT/IGNORE=NOBACKUP [SETS]B69IGN.BCK/SAVE_SET")
	if !cmd.IgnoreNoBackup || cmd.IgnoreInterlock {
		t.Errorf("/IGNORE=NOBACKUP: %+v", cmd)
	}

	cmd = parse("BACKUP/LIST=[SETS]B69FULL.LIS/FULL [SETS]B69.BCK/SAVE_SET/SELECT=([B69]TEXT.TXT,[B69]OWNED.TXT)/CRC")
	if !cmd.List || cmd.ListFile != "[SETS]B69FULL.LIS" || !cmd.Full || cmd.Output != nil || !cmd.Input.SaveSet ||
		!slices.Equal(cmd.Select, []string{"[B69]TEXT.TXT", "[B69]OWNED.TXT"}) || !cmd.CheckCRC {
		t.Errorf("a listing: %+v", cmd)
	}

	cmd = parse("BACKUP/LIST [SETS]B69.BCK/SAVE_SET")
	if !cmd.List || cmd.ListFile != "" || cmd.Full || cmd.Brief {
		t.Errorf("BACKUP/LIST: %+v", cmd)
	}

	cmd = parse("BACKUP/LOG [SETS]B69.BCK/SAVE_SET/SELECT=([B69]TEXT.TXT,[B69]OWNED.TXT) [R10A]/BY_OWNER=ORIGINAL")
	if cmd.OutputOwner == nil || *cmd.OutputOwner != "ORIGINAL" || cmd.InputOwner != nil || cmd.Existing != dclcmd.BackupKeepExisting {
		t.Errorf("a restore's owner: %+v", cmd)
	}

	cmd = parse("BACKUP/LOG [SETS]B69.BCK/SAVE_SET/SELECT=[B69]TEXT.TXT [*...]/NEW_VERSION")
	if cmd.Existing != dclcmd.BackupNewVersion || cmd.Output.Text != "[*...]" {
		t.Errorf("/NEW_VERSION: %+v", cmd)
	}

	for line, want := range map[string]dclcmd.BackupExisting{
		"BACKUP X.BCK/SAVE_SET [R7]/OVERLAY": dclcmd.BackupOverlay,
		"BACKUP X.BCK/SAVE_SET [R7]/REPLACE": dclcmd.BackupReplace,
	} {
		if cmd := parse(line); cmd.Existing != want {
			t.Errorf("%s: Existing %d, want %d", line, cmd.Existing, want)
		}
	}

	if cmd := parse("BACKUP/COMPARE/LOG [SETS]TEXT.BCK/SAVE_SET [CMP]"); !cmd.Compare || !cmd.Log {
		t.Errorf("/COMPARE: %+v", cmd)
	}

	// Two of one kind's qualifiers.
	for _, line := range []string{
		"BACKUP X.BCK/SAVE_SET [R7]/NEW_VERSION/OVERLAY",
		"BACKUP [B69]*.*/SINCE=TODAY/CREATED/MODIFIED X.BCK/SAVE_SET",
	} {
		r, err := g.Parse(line)
		if err != nil {
			t.Fatalf("Parse(%q): %v", line, err)
		}

		if _, err := backupCommand(r, line); !errors.Is(err, vmserrors.New(vmserrors.CLI_BADQUALIFIERCOMBO)) {
			t.Errorf("%s: %v, want BADQUALIFIERCOMBO", line, err)
		}
	}

	if _, err := g.Parse("BACKUP/LIST/FULL/BRIEF X.BCK/SAVE_SET"); err == nil {
		t.Error("/FULL with /BRIEF parsed")
	}
}

// TestBackupMessages: BACKUP's failures are VMS's messages, with the
// condition values VMS V7.1's $STATUS showed for them (testdata/
// backup69/vax/backup69.log), as far as this subtask goes: the
// /SAVE_SET rule, and the input save set opened and read.
func TestBackupMessages(t *testing.T) {
	d, c := newTestDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.cmd().SetDefault("DUA0:"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"pi.bck", "pi.mar"} {
		if err := d.Dispatch("COPY/BINARY " + `"` + backupSets + name + `" ` + strings.ToUpper(name)); err != nil {
			t.Fatalf("COPY %s: %v", name, err)
		}
	}

	cases := []struct {
		line   string
		text   string
		status uint32
	}{
		{"BACKUP/LIST PI.BCK", "BACKUP-F-LISINPSAV, /LIST requires save set as input", 0x10A3808C},
		{"BACKUP/LIST NONE.BCK/SAVE_SET",
			"BACKUP-F-OPENIN, error opening DUA0:[000000]NONE.BCK; as input\n-RMS-E-FNF, file not found", 0x10A3800C},
		{"BACKUP/LIST PI.MAR/SAVE_SET", "BACKUP-F-NOTSAVESET, DUA0:[000000]PI.MAR;1 is not a BACKUP save set", 0x10A38104},
		{`BACKUP/LIST "` + backupSets + `pi.mar"/SAVE_SET`, "BACKUP-F-NOTSAVESET, " + backupSets + "pi.mar is not a BACKUP save set", 0x10A38104},

		// govax's messages (their codes too).
		{"BACKUP PI.BCK/SAVE_SET OTHER.BCK/SAVE_SET", "BACKUP-F-BOTHSAVSET, input and output cannot both be save sets", 0x10A3FFC4},
		{"BACKUP *.MAR [R1]", "BACKUP-F-NOSAVESET, input or output must be a save set", 0x10A3FFBC},
		{"BACKUP/COMPARE *.MAR OTHER.BCK/SAVE_SET", "BACKUP-F-CMPINPSAV, /COMPARE requires save set as input", 0x10A3FFB4},

		// The operations, until their subtasks: the save set is read
		// first.
		{"BACKUP PI.BCK/SAVE_SET [R1...]", "BACKUP-F-NOTYET, a BACKUP restore is not implemented yet", 0x10A3FFAC},
		{"BACKUP/COMPARE PI.BCK/SAVE_SET [R1...]", "BACKUP-F-NOTYET, BACKUP/COMPARE is not implemented yet", 0x10A3FFAC},
		{"BACKUP *.MAR OTHER.BCK/SAVE_SET", "BACKUP-F-NOTYET, a BACKUP save is not implemented yet", 0x10A3FFAC},
	}

	for _, tc := range cases {
		err := d.Dispatch(tc.line)
		if err == nil {
			t.Errorf("%s: no error", tc.line)

			continue
		}

		if err.Error() != tc.text {
			t.Errorf("%s:\n%s\nwant\n%s", tc.line, err, tc.text)
		}

		if got := dclshell.ConditionValue(err); got != tc.status {
			t.Errorf("%s: status %%X%08X, want %%X%08X", tc.line, got, tc.status)
		}
	}

	// No output specifier without /LIST.
	if err := d.Dispatch("BACKUP PI.BCK/SAVE_SET"); !errors.Is(err, vmserrors.New(vmserrors.CLI_INSFPRM)) {
		t.Errorf("BACKUP with no output: %v, want INSFPRM", err)
	}
}

// TestBackupMessageShown: in a procedure, BACKUP's failure is shown as
// VMS shows it, "%" first, and ends the procedure with its status.
func TestBackupMessageShown(t *testing.T) {
	err, out, status := runStatusProcedure(t, "$ BACKUP/LIST NOSUCH.BCK", `$ PRINT "after"`)
	if out != "%BACKUP-F-LISINPSAV, /LIST requires save set as input\n" || status != 0x10A3808C || !vmserrors.MessageInhibited(err) {
		t.Errorf("output %q, $STATUS %%X%08X, %v", out, status, err)
	}
}
