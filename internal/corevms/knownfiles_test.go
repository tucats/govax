package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// TestKnownFileList: an entry is found by its file's name in any case,
// with or without a version or a leading "_"; a second ADD of the same file
// is refused; REMOVE takes it away; Entries is in LIST's order.
func TestKnownFileList(t *testing.T) {
	l := NewKnownFileList()

	if !l.Add(KnownFile{Name: "_DUA1:[000000]PRIV94I.EXE;1", Version: 1, Volume: "DISK$B", Directory: "000000", File: "PRIV94I", Type: "EXE"}) {
		t.Fatal("first ADD refused")
	}

	if l.Add(KnownFile{Name: "dua1:[000000]priv94i.exe;2"}) {
		t.Error("a second ADD of the file was taken")
	}

	l.Add(KnownFile{Name: "DUA1:[000000]AAA.EXE", Volume: "DISK$B", Directory: "000000", File: "AAA", Type: "EXE"})
	l.Add(KnownFile{Name: "DUA0:[SYS]ZZZ.EXE", Volume: "DISK$A", Directory: "SYS", File: "ZZZ", Type: "EXE"})

	for _, spec := range []string{"DUA1:[000000]PRIV94I.EXE", "dua1:[000000]priv94i.exe;7", "_DUA1:[000000]PRIV94I.EXE;1"} {
		if kf, ok := l.Find(spec); !ok || kf.Name != "DUA1:[000000]PRIV94I.EXE" {
			t.Errorf("Find(%q) = %+v, %v", spec, kf, ok)
		}
	}

	var order []string
	for _, kf := range l.Entries() {
		order = append(order, kf.File)
	}

	if got := len(order); got != 3 || order[0] != "ZZZ" || order[1] != "AAA" || order[2] != "PRIV94I" {
		t.Errorf("Entries' order %v", order)
	}

	if !l.Remove("DUA1:[000000]PRIV94I.EXE;1") || l.Remove("DUA1:[000000]PRIV94I.EXE") {
		t.Error("REMOVE didn't take the entry exactly once")
	}
}

// TestInstalledImagePrivileges: an image on the list, at its installed
// version, runs with IMAGPRIV its privileges and CURPRIV = PROCPRIV |
// IMAGPRIV, counting the access; another version, or another image, gets
// nothing; rundown takes them away (testdata/priv94's INSTALL sections).
func TestInstalledImagePrivileges(t *testing.T) {
	env, _ := fixture()
	p := env.Process
	tmp := privilegeBit("TMPMBX") | privilegeBit("NETMBX")
	imag := privilegeBit("SYSNAM") | privilegeBit("OPER")

	p.AuthorizedPrivileges, p.ProcessPrivileges, p.CurrentPrivileges = tmp, tmp, tmp

	env.KnownFiles.Add(KnownFile{Name: "DUA1:[000000]PRIV94I.EXE", Version: 1, Privileges: imag})

	env.StartImage("DUA1:[000000]PRIV94I.EXE;1")

	if p.ImagePrivileges != imag || p.CurrentPrivileges != tmp|imag || p.ImageName != "DUA1:[000000]PRIV94I.EXE;1" {
		t.Errorf("installed: IMAGPRIV %#x CURPRIV %#x name %q", p.ImagePrivileges, p.CurrentPrivileges, p.ImageName)
	}

	if kf, _ := env.KnownFiles.Find("DUA1:[000000]PRIV94I.EXE"); kf.Accesses != 1 {
		t.Errorf("access count %d", kf.Accesses)
	}

	env.resetImagePrivileges()

	if p.ImagePrivileges != 0 || p.CurrentPrivileges != tmp {
		t.Errorf("after rundown: IMAGPRIV %#x CURPRIV %#x", p.ImagePrivileges, p.CurrentPrivileges)
	}

	for _, name := range []string{"DUA1:[000000]PRIV94I.EXE;2", "DUA1:[000000]OTHER.EXE;1", "/host/priv94i.exe"} {
		env.StartImage(name)

		if p.ImagePrivileges != 0 || p.CurrentPrivileges != tmp {
			t.Errorf("%s: IMAGPRIV %#x CURPRIV %#x", name, p.ImagePrivileges, p.CurrentPrivileges)
		}
	}
}

// TestSetprvImagePrivileges: while an installed image runs, $SETPRV may
// enable its privileges again temporarily, though the process isn't
// authorized for them, but not permanently (the System Services manual's
// $SETPRV).
func TestSetprvImagePrivileges(t *testing.T) {
	env, _ := fixture()
	p := env.Process
	tmp := privilegeBit("TMPMBX")
	sysnam := privilegeBit("SYSNAM")

	p.AuthorizedPrivileges, p.ProcessPrivileges = tmp, tmp
	p.ImagePrivileges = sysnam
	p.CurrentPrivileges = tmp

	if st := env.setPrivileges(sysnam, true, false, registers.User); st != ssNormal || p.CurrentPrivileges != tmp|sysnam {
		t.Errorf("temporary: %#x, CURPRIV %#x", st, p.CurrentPrivileges)
	}

	p.CurrentPrivileges = tmp

	if st := env.setPrivileges(sysnam, true, true, registers.User); st != ssNotAllPriv || p.ProcessPrivileges != tmp {
		t.Errorf("permanent: %#x, PROCPRIV %#x", st, p.ProcessPrivileges)
	}
}
