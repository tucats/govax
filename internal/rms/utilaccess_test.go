package rms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/security"
)

// Phase 93's subtask 9: the DCL utilities checked against the session's
// process (utilaccess.go), each rule one VMS 7.1 gave the probe's Part B
// (A.4). console's TestProt93UtilitiesOracle holds the utilities'
// messages to the probe's own logs; these need no local volume.

// actAs makes the session's utilities act for creds.
func (f *accessFixture) actAs(creds security.Credentials) {
	f.s.Credentials = func() security.Credentials { return creds }
}

// deniedOf returns err's DeniedError, failing the test if it has none.
func deniedOf(t *testing.T, what string, err error) *DeniedError {
	t.Helper()

	denied, ok := IsDenied(err)
	if !ok || denied == nil {
		t.Fatalf("%s: %v, want a refusal", what, err)
	}

	return denied
}

// A file the process may not read is refused to every utility that reads
// it, named with its version; one it may read isn't.
func TestUtilityReads(t *testing.T) {
	f := newAccessFixture(t, "U")
	f.file("DUA0:[U]NONE.DAT", uicA, protNone)
	f.file("DUA0:[U]WLD.DAT", uicA, everyone(security.Read))

	f.actAs(account(uicC, 0))

	_, err := f.s.Type("DUA0:[U]NONE.DAT")
	if d := deniedOf(t, "TYPE", err); d.Search || d.Name != "DUA0:[U]NONE.DAT;1" {
		t.Errorf("TYPE: %+v, want the file DUA0:[U]NONE.DAT;1", d)
	}

	if _, _, err := f.s.ReadRecordFile(FileLocation{Name: "DUA0:[U]NONE.DAT"}, TextRecords); err == nil {
		t.Error("ReadRecordFile of NONE.DAT: allowed")
	}

	if _, err := f.s.FileAttributes("DUA0:[U]NONE.DAT"); !errors.Is(err, ErrNoPrivilege) {
		t.Errorf("FileAttributes of NONE.DAT: %v, want ErrNoPrivilege", err)
	}

	var fe *DCLFileError
	if _, err := f.s.OpenDump("DUA0:[U]NONE.DAT"); !errors.As(err, &fe) || fe.Status != rmsPrivilegeViolation {
		t.Errorf("DUMP of NONE.DAT: %v, want RMS$_PRV", err)
	}

	if text, err := f.s.Type("DUA0:[U]WLD.DAT"); err != nil || text != "x\n" {
		t.Errorf("TYPE WLD.DAT: %q, %v", text, err)
	}

	// READALL reads it.
	f.actAs(account(uicC, security.PrivREADALL))

	if _, err := f.s.Type("DUA0:[U]NONE.DAT"); err != nil {
		t.Errorf("TYPE with READALL: %v", err)
	}
}

// A command procedure is opened with R or E (A.4, answer 4), any other
// input with R.
func TestProcedureExecuteOnly(t *testing.T) {
	f := newAccessFixture(t, "P")
	f.file("DUA0:[P]E.COM", uicA, everyone(security.Execute))
	f.file("DUA0:[P]N.COM", uicA, protNone)

	f.actAs(account(uicC, 0))

	if _, _, err := f.s.ReadProcedure(FileLocation{Name: "DUA0:[P]E.COM"}); err != nil {
		t.Errorf("@ of E.COM: %v", err)
	}

	if _, _, err := f.s.ReadRecordFile(FileLocation{Name: "DUA0:[P]E.COM"}, TextRecords); !errors.Is(err, ErrNoPrivilege) {
		t.Errorf("reading E.COM: %v, want ErrNoPrivilege", err)
	}

	if _, _, err := f.s.ReadProcedure(FileLocation{Name: "DUA0:[P]N.COM"}); !errors.Is(err, ErrNoPrivilege) {
		t.Errorf("@ of N.COM: %v, want ErrNoPrivilege", err)
	}
}

// A directory giving E alone lets a utility look a name up, a version
// wildcard and all, but not list it; one giving nothing, neither (the
// probe's [.UDE] and [.UDN]).
func TestUtilitySearch(t *testing.T) {
	f := newAccessFixture(t, "UDE", "UDN")
	f.file("DUA0:[UDE]A.DAT", uicA, protAll)
	f.file("DUA0:[UDN]A.DAT", uicA, protAll)
	f.protect([]string{"UDE"}, everyone(security.Execute))
	f.protect([]string{"UDN"}, protNone)

	f.actAs(account(uicC, 0))

	if _, found, err := f.s.Directory("DUA0:[UDE]A.DAT", DirectoryOptions{}); err != nil || !found {
		t.Errorf("DIRECTORY [UDE]A.DAT: %v, found %v", err, found)
	}

	if _, _, err := f.s.Directory("DUA0:[UDE]*.DAT", DirectoryOptions{}); !deniedOf(t, "DIRECTORY [UDE]*.DAT", err).Search {
		t.Errorf("DIRECTORY [UDE]*.DAT: %v, want a search refused", err)
	}

	if names, err := f.s.Search("DUA0:[UDE]*.DAT"); err != nil || len(names) != 0 {
		t.Errorf("F$SEARCH [UDE]*.DAT: %q, %v, want nothing", names, err)
	}

	if _, err := f.s.Type("DUA0:[UDE]A.DAT"); err != nil {
		t.Errorf("TYPE [UDE]A.DAT: %v", err)
	}

	if _, err := f.s.Type("DUA0:[UDN]A.DAT"); !deniedOf(t, "TYPE [UDN]A.DAT", err).Search {
		t.Errorf("TYPE [UDN]A.DAT: %v, want a search refused", err)
	}
}

// DIRECTORY/FULL of a file the process may neither read nor control
// shows the reason in place of its facts, and counts no blocks (A.4,
// answer 12).
func TestDirectoryFullUnreadable(t *testing.T) {
	f := newAccessFixture(t, "U")
	f.file("DUA0:[U]NONE.DAT", uicA, protNone)

	f.actAs(account(uicC, 0))

	text, _, err := f.s.Directory("DUA0:[U]NONE.DAT", DirectoryOptions{Full: true})
	if err != nil {
		t.Fatal(err)
	}

	want := "\nDirectory DUA0:[U]\n\nNONE.DAT;1          insufficient privilege or object protection violation\n\nTotal of 1 file, 0/0 blocks.\n"
	if text != want {
		t.Errorf("got:\n%s\nwant:\n%s", text, want)
	}
}

// DELETE and PURGE pass over a file without D, or in a directory without
// R and W, and delete the rest (the probe's DEL_ and PUR_ files).
func TestUtilityDeletePurge(t *testing.T) {
	f := newAccessFixture(t, "D", "NW")

	d := security.Read | security.Write | security.Delete
	f.file("DUA0:[D]KEEP.DAT", uicA, everyone(security.Read|security.Write))
	f.file("DUA0:[D]GONE.DAT", uicA, everyone(d))
	f.file("DUA0:[D]P.DAT", uicA, everyone(security.Read))
	f.file("DUA0:[D]P.DAT", uicA, everyone(d))
	f.file("DUA0:[D]P.DAT", uicA, everyone(d))
	f.file("DUA0:[NW]X.DAT", uicA, protAll)
	f.protect([]string{"NW"}, everyone(security.Read|security.Execute))

	f.actAs(account(uicC, 0))

	deleted, err := f.s.Delete("DUA0:[D]*.DAT;*")

	var notDeleted *NotDeletedError
	if !errors.As(err, &notDeleted) {
		t.Fatalf("DELETE [D]*.DAT;*: %v", err)
	}

	gone := map[string]bool{}
	for _, x := range deleted {
		gone[x.Full] = true
	}

	if !gone["DUA0:[D]GONE.DAT;1"] || gone["DUA0:[D]KEEP.DAT;1"] || gone["DUA0:[D]P.DAT;1"] {
		t.Errorf("DELETE deleted %v; refused %v", deleted, notDeleted.Names)
	}

	if _, err := f.s.Delete("DUA0:[NW]X.DAT;*"); !errors.As(err, &notDeleted) || len(notDeleted.Names) != 1 {
		t.Errorf("DELETE in [NW]: %v, want X.DAT refused", err)
	}

	// PURGE keeping one of Q.DAT's three versions: ;1 gives no D, ;2 does.
	f.actAs(security.SystemCredentials())
	f.file("DUA0:[D]Q.DAT", uicA, everyone(security.Read))
	f.file("DUA0:[D]Q.DAT", uicA, everyone(d))
	f.file("DUA0:[D]Q.DAT", uicA, everyone(d))
	f.actAs(account(uicC, 0))

	purged, err := f.s.Purge("DUA0:[D]Q.DAT", 1)
	if !errors.As(err, &notDeleted) || len(notDeleted.Names) != 1 || notDeleted.Names[0] != "DUA0:[D]Q.DAT;1" {
		t.Errorf("PURGE: %v, want Q.DAT;1 refused", err)
	}

	if len(purged) != 1 || purged[0].Version != 2 {
		t.Errorf("PURGE purged %v, want Q.DAT;2", purged)
	}
}

// A file made by a utility needs R and W on its directory, and a new
// version W on the highest version: a refusal names the output, and for
// COPY its input (the probe's CPY_ and N_ files).
func TestUtilityCreate(t *testing.T) {
	f := newAccessFixture(t, "IN", "RO", "V")
	f.file("DUA0:[IN]SRC.DAT", uicA, everyone(security.Read))
	f.file("DUA0:[V]OLD.DAT", uicA, everyone(security.Read))
	f.protect([]string{"RO"}, everyone(security.Read|security.Execute))

	f.actAs(account(uicC, 0))

	_, err := f.s.CreateRecordFile(FileLocation{Name: "DUA0:[RO]NEW.DAT"}, TextRecords, nil)
	if d := deniedOf(t, "CREATE in [RO]", err); !d.Output || d.Name != "DUA0:[RO]NEW.DAT;" {
		t.Errorf("CREATE in [RO]: %+v", d)
	}

	if err := f.s.CreateAllowed(FileLocation{Name: "DUA0:[RO]NEW.DAT"}); err == nil {
		t.Error("CreateAllowed in [RO]: allowed")
	}

	_, err = f.s.Copy("DUA0:[IN]SRC.DAT", false, "DUA0:[V]OLD.DAT", false, CopyOptions{})
	if d := deniedOf(t, "COPY over OLD.DAT", err); !d.Output || d.Name != "DUA0:[V]OLD.DAT;" || d.Source != "DUA0:[IN]SRC.DAT;1" {
		t.Errorf("COPY over OLD.DAT: %+v", d)
	}

	if _, err := f.s.Copy("DUA0:[IN]SRC.DAT", false, "DUA0:[V]NEW.DAT", false, CopyOptions{}); err != nil {
		t.Errorf("COPY to [V]NEW.DAT: %v", err)
	}

	if _, err := f.s.CreateDirectory("DUA0:[RO.SUB]", CreateDirectoryOptions{}); !errors.Is(err, ErrNoPrivilege) {
		t.Errorf("CREATE/DIRECTORY [RO.SUB]: %v, want ErrNoPrivilege", err)
	}
}
