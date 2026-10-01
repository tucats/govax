package console

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
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

	wantLines(t, "CREATE/DIRECTORY/LOG [A.B],[C]", lines,
		"%CREATE-I-CREATED, DUA0:[A] created",
		"%CREATE-I-CREATED, DUA0:[A.B] created",
		"%CREATE-I-CREATED, DUA0:[C] created")

	// Without /LOG, a directory made says nothing, but one that already
	// existed is still reported.
	lines, err = renameLines(t, d, out, "CREATE/DIRECTORY [A.B.NEW],[C]")
	if err != nil {
		t.Fatal(err)
	}

	wantLines(t, "CREATE/DIRECTORY [A.B.NEW],[C]", lines, "%CREATE-I-EXISTS, DUA0:[C] already exists")

	// The process UIC before INIT is SYSTEM's, [1,4].
	if got := createdHeader(t, d, "A", "B", "NEW").Owner; got != (ondisk.Uic{Group: 1, Member: 4}) {
		t.Errorf("[A.B.NEW] owner = %v, want [1,4]", got)
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
	if r := createdHeader(t, d, "Q", "R"); r.FileProtection != want || r.Owner != (ondisk.Uic{Group: 1, Member: 4}) {
		t.Errorf("[Q.R]: protection %#x, owner %v; want %#x, [1,4]", r.FileProtection, r.Owner, want)
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

// TestCreateDirectory_errors: a bad qualifier fails before anything is
// made; in a list, a directory that can't be made is reported and the
// rest are still made, and the command fails with its message already
// shown.
func TestCreateDirectory_errors(t *testing.T) {
	d, out := newCreateTestDispatcher(t)

	for cmd, status := range map[string]uint32{
		"CREATE/DIRECTORY/OWNER_UIC=[400000,1] [BAD]": vmserrors.CLI_BADQUALIFIER,
		"CREATE/DIRECTORY/OWNER_UIC=SYSTEM [BAD]":     vmserrors.CLI_BADQUALIFIER,
		"CREATE/DIRECTORY/VERSION_LIMIT=40000 [BAD]":  vmserrors.CLI_BADQUALIFIER,
		"CREATE/DIRECTORY/PROTECTION=(X:RWED) [BAD]":  vmserrors.CLI_BADQUALIFIER,
		"CREATE/DIRECTORY [BAD]FILE.DAT":              vmserrors.CREATE_DIRNOTCRE,
		"CREATE/DIRECTORY DUB0:[BAD]":                 vmserrors.SS_DEVNOTMOUNT,
		"CREATE":                                      vmserrors.CLI_MISSINGPARAMETER,
	} {
		err := d.Dispatch(cmd)
		if !errors.Is(err, vmserrors.New(status)) {
			t.Errorf("%s: err = %v, want %s", cmd, err, vmserrors.New(status))
		}
	}

	vol, _ := d.Console.Mounts.Lookup("DUA0")
	if _, err := filespec.ResolveDirectory(vol, []string{"BAD"}); err == nil {
		t.Error("[BAD] was made")
	}

	lines, err := renameLines(t, d, out, "CREATE/DIRECTORY/LOG [OK1],[*],[OK2]")
	if err == nil || !vmserrors.MessageInhibited(err) {
		t.Errorf("list with a bad directory: err = %v, want a failure already displayed", err)
	}

	if len(lines) != 3 || lines[0] != "%CREATE-I-CREATED, DUA0:[OK1] created" || lines[2] != "%CREATE-I-CREATED, DUA0:[OK2] created" {
		t.Errorf("list with a bad directory printed %q", lines)
	}
}
