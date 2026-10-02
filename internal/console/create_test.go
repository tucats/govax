package console

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// newCreateTestDispatcher is a dispatcher on the real console grammar,
// its console's output, and a fresh volume mounted writable on DUA0, whose
// [000000] is the default directory.
func newCreateTestDispatcher(t *testing.T) (*Dispatcher, *bytes.Buffer) {
	t.Helper()

	c, out := newTestConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)

	mountFreshContainer(t, c, "DUA0")

	if err := d.Dispatch("SET DEFAULT DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	return d, out
}

// createdHeader is the file header of the directory dirs on DUA0.
func createdHeader(t *testing.T, d *Dispatcher, dirs ...string) ondisk.FileHeader {
	t.Helper()

	vol, _ := d.Console.Mounts.Lookup("DUA0")

	dir, err := filespec.ResolveDirectory(vol, dirs)
	if err != nil {
		t.Fatalf("ResolveDirectory(%v): %v", dirs, err)
	}

	return dir.Header
}

func TestCreateDirectory_logLevelsAndExisting(t *testing.T) {
	d, out := newCreateTestDispatcher(t)

	lines, err := renameLines(t, d, out, "CREATE/DIRECTORY/LOG [A.B],[C]")
	if err != nil {
		t.Fatal(err)
	}

	// As on VMS, /LOG reports the directory asked for, not the ones made
	// above it.
	wantLines(t, "CREATE/DIRECTORY/LOG [A.B],[C]", lines,
		"%CREATE-I-CREATED, DUA0:[A.B] created",
		"%CREATE-I-CREATED, DUA0:[C] created")

	if _, err := filespec.ResolveDirectory(mustVolume(t, d), []string{"A"}); err != nil {
		t.Errorf("[A]: %v", err)
	}

	// Without /LOG, a directory made says nothing, but one that already
	// existed is still reported, by the name typed.
	lines, err = renameLines(t, d, out, "CREATE/DIRECTORY [A.B.NEW],[C]")
	if err != nil {
		t.Fatal(err)
	}

	wantLines(t, "CREATE/DIRECTORY [A.B.NEW],[C]", lines, "%CREATE-I-EXISTS, [C] already exists")

	// Owners come from the parent, down from the MFD's.
	if got, mfd := createdHeader(t, d, "A", "B", "NEW").Owner, createdHeader(t, d, "A").Owner; got != mfd {
		t.Errorf("[A.B.NEW] owner = %v, want [A]'s %v", got, mfd)
	}
}

func TestCreateDirectory_qualifiers(t *testing.T) {
	d, _ := newCreateTestDispatcher(t)

	for _, cmd := range []string{
		"CREATE/DIRECTORY/OWNER_UIC=[200,201]/VERSION_LIMIT=3/ALLOCATION=4 [Q]",
		"CREATE/DIRECTORY/OWNER_UIC=PARENT/PROTECTION=(S:RWED,O:RWED,G:RE,W) [Q.P]",
		"CREATE/DIRECTORY/PROTECTION=W:RE [Q.R]",
	} {
		if err := d.Dispatch(cmd); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
	}

	q := createdHeader(t, d, "Q")
	if q.Owner != (ondisk.Uic{Group: 0o200, Member: 0o201}) || q.RecordAttributes.VersionLimit != 3 || q.RecordAttributes.HighestBlock < 4 {
		t.Errorf("[Q]: owner %v, limit %d, HIBLK %d; want [200,201], 3, at least 4",
			q.Owner, q.RecordAttributes.VersionLimit, q.RecordAttributes.HighestBlock)
	}

	p := createdHeader(t, d, "Q", "P")
	if p.Owner != q.Owner || p.FileProtection != 0xFA00 || p.RecordAttributes.VersionLimit != 3 {
		t.Errorf("[Q.P]: owner %v, protection %#x, limit %d; want [200,201], 0xfa00, 3",
			p.Owner, p.FileProtection, p.RecordAttributes.VersionLimit)
	}

	// W:RE changes only the world field of [Q]'s protection less delete.
	want, _ := ondisk.ParseProtection("W:RE", q.FileProtection|ondisk.ProtectionNoDeleteAll)
	// With no /OWNER_UIC, [Q.R] has its parent's owner.
	if r := createdHeader(t, d, "Q", "R"); r.FileProtection != want || r.Owner != q.Owner {
		t.Errorf("[Q.R]: protection %#x, owner %v; want %#x, [Q]'s %v", r.FileProtection, r.Owner, want, q.Owner)
	}
}

func TestCreateDirectory_relativeAndLogical(t *testing.T) {
	d, out := newCreateTestDispatcher(t)

	for _, cmd := range []string{"CREATE/DIRECTORY [PLAIN]", "SET DEFAULT [PLAIN]", "DEFINE CREDEV DUA0:"} {
		if err := d.Dispatch(cmd); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
	}

	lines, err := renameLines(t, d, out, "CREATE/DIRECTORY/LOG [.SUB],[-.SIBLING],CREDEV:[LOGICAL]")
	if err != nil {
		t.Fatal(err)
	}

	wantLines(t, "relative and logical", lines,
		"%CREATE-I-CREATED, DUA0:[PLAIN.SUB] created",
		"%CREATE-I-CREATED, DUA0:[SIBLING] created",
		"%CREATE-I-CREATED, DUA0:[LOGICAL] created")
}

// TestCreateDirectory_errors: each failure prints VMS 7.3's messages
// (testdata/credir's CREDIR.LOG) and fails the command with its message
// already shown. A bad /OWNER_UIC or /PROTECTION makes nothing; a bad
// /VERSION_LIMIT is reported and ignored, and the directory still made.
func TestCreateDirectory_errors(t *testing.T) {
	d, out := newCreateTestDispatcher(t)

	tests := []struct {
		cmd   string
		lines []string
	}{
		{"CREATE/DIRECTORY/OWNER_UIC=[400000,1] [BAD]", []string{
			"%CREATE-F-SYNTAX, error parsing '[400000,1]'",
			"-SYSTEM-F-IVIDENT, invalid identifier format",
		}},
		{"CREATE/DIRECTORY/OWNER_UIC=SYSTEM [BAD]", []string{
			"%CREATE-F-SYNTAX, error parsing 'SYSTEM'",
			"-SYSTEM-F-IVIDENT, invalid identifier format",
		}},
		{"CREATE/DIRECTORY/PROTECTION=(X:RWED) [BAD]", []string{
			"%CREATE-F-SYNTAX, error parsing '(X:RWED)'",
		}},
		{"CREATE/DIRECTORY [BAD]FILE.DAT", []string{
			"%CREATE-E-DIRNOTCRE, [BAD]FILE.DAT directory file not created",
			"-LIB-F-INVFILSPE, invalid file specification",
		}},
		{"CREATE/DIRECTORY DUB0:[BAD]", []string{
			"%CREATE-E-DIRNOTCRE, DUB0:[BAD] directory file not created",
			"-SYSTEM-W-NOSUCHDEV, no such device available",
		}},
		{"CREATE/DIRECTORY [L1.L2.L3.L4.L5.L6.L7.L8.L9]", []string{
			"%CREATE-E-DIRNOTCRE, [L1.L2.L3.L4.L5.L6.L7.L8.L9] directory file not created",
			"-RMS-F-DIR, error in directory name",
		}},
	}

	for _, tt := range tests {
		lines, err := renameLines(t, d, out, tt.cmd)
		if err == nil || !vmserrors.MessageInhibited(err) {
			t.Errorf("%s: err = %v, want a failure already displayed", tt.cmd, err)
		}

		wantLines(t, tt.cmd, lines, tt.lines...)
	}

	vol := mustVolume(t, d)
	for _, dir := range []string{"BAD", "L1"} {
		if _, err := filespec.ResolveDirectory(vol, []string{dir}); err == nil {
			t.Errorf("[%s] was made", dir)
		}
	}

	// A bad /VERSION_LIMIT is reported, then ignored.
	lines, err := renameLines(t, d, out, "CREATE/DIRECTORY/LOG/VERSION_LIMIT=40000 [BADLIMIT]")
	if err == nil || !vmserrors.MessageInhibited(err) {
		t.Errorf("/VERSION_LIMIT=40000: err = %v, want a failure already displayed", err)
	}

	wantLines(t, "/VERSION_LIMIT=40000", lines,
		"%CREATE-E-BADVALUE, '40000' is an invalid keyword value",
		"%CREATE-I-CREATED, DUA0:[BADLIMIT] created")

	if got := createdHeader(t, d, "BADLIMIT").RecordAttributes.VersionLimit; got != 0 {
		t.Errorf("[BADLIMIT] version limit = %d, want the MFD's, 0", got)
	}

	// In a list, the rest are still made.
	lines, err = renameLines(t, d, out, "CREATE/DIRECTORY/LOG [OK1],[*],[OK2]")
	if err == nil || !vmserrors.MessageInhibited(err) {
		t.Errorf("list with a bad directory: err = %v, want a failure already displayed", err)
	}

	wantLines(t, "list with a bad directory", lines,
		"%CREATE-I-CREATED, DUA0:[OK1] created",
		"%CREATE-E-DIRNOTCRE, [*] directory file not created",
		"-LIB-F-INVFILSPE, invalid file specification",
		"%CREATE-I-CREATED, DUA0:[OK2] created")

	// A bare CREATE is still a missing qualifier.
	if err := d.Dispatch("CREATE"); !errors.Is(err, vmserrors.New(vmserrors.CLI_MISSINGPARAMETER)) {
		t.Errorf("CREATE: err = %v, want CLI_MISSINGPARAMETER", err)
	}
}

// mustVolume is the volume mounted on DUA0.
func mustVolume(t *testing.T, d *Dispatcher) *volume.Volume {
	t.Helper()

	vol, ok := d.Console.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("DUA0 isn't mounted")
	}

	return vol
}
