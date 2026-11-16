package corevms

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
)

// TestHostAllowed holds the gate to Decisions 1 and 3 of
// docs/planning/PHASE-97 - host file access.md: with the key off every
// process may use a host file and nothing is traced; with it on only a
// process holding SYSPRV may, the privileges read at the moment of the
// access, and each decision is traced under SET DEBUG PRIVILEGES.
func TestHostAllowed(t *testing.T) {
	env, _ := fixture()
	p := env.Process

	var trace bytes.Buffer

	env.cpu.SetDebugWriter(&trace)
	env.cpu.SetDebug(registers.DebugPrivileges)
	setMode(env, registers.User, registers.User, 0x9000)

	p.CurrentPrivileges = privTMPMBX

	if st := env.HostAllowed(); st != 0 {
		t.Errorf("key off: %#x, want 0", st)
	}

	if trace.Len() != 0 {
		t.Errorf("key off traced: %q", trace.String())
	}

	env.HostRequiresSYSPRV = true

	if st := env.HostAllowed(); st != ssNoPriv {
		t.Errorf("key on, no SYSPRV: %#x, want SS$_NOPRIV", st)
	}

	p.CurrentPrivileges = privTMPMBX | privSYSPRV

	if st := env.HostAllowed(); st != 0 {
		t.Errorf("key on, SYSPRV: %#x, want 0", st)
	}

	lines := strings.Split(strings.TrimSpace(trace.String()), "\n")
	want := []string{
		"USER mode: host file: open, create, or run (needs SYSPRV): refused, SS$_NOPRIV",
		"USER mode: host file: open, create, or run (needs SYSPRV): holds SYSPRV",
	}

	if len(lines) != len(want) {
		t.Fatalf("trace:\n%s", trace.String())
	}

	for i, line := range lines {
		if !strings.HasSuffix(line, want[i]) {
			t.Errorf("trace line %d %q, want ...%q", i, line, want[i])
		}
	}
}

// TestUseSessionHostAccess: the session UseSession gives a process asks
// the process's gate, so rms.Session.Locate refuses a host file to a
// process without SYSPRV while the key is on, and a privilege turned on
// counts at once (Decision 6).
func TestUseSessionHostAccess(t *testing.T) {
	env, _ := fixture()
	p := env.Process

	s := rms.NewSession(rms.NewMountTable())
	env.UseSession(s)

	p.CurrentPrivileges = privTMPMBX

	if _, err := s.Locate("/tmp/notes.txt", false); err != nil {
		t.Fatalf("key off: %v", err)
	}

	env.HostRequiresSYSPRV = true

	_, err := s.Locate("/tmp/notes.txt", false)

	var denied *rms.DeniedError
	if !errors.As(err, &denied) || !denied.Host || denied.Name != "/tmp/notes.txt" {
		t.Fatalf("key on, no SYSPRV: %v, want a host DeniedError", err)
	}

	p.CurrentPrivileges |= privSYSPRV

	if loc, err := s.Locate("/tmp/notes.txt", false); err != nil || !loc.Host {
		t.Errorf("key on, SYSPRV: %+v, %v", loc, err)
	}
}

// TestExeOpenHostAccess: EXE$OPEN names a host file, so with the key on
// a process without SYSPRV gets -1, a failed open, and nothing is made
// (docs/planning/PHASE-97 - host file access.md, subtask 4); with
// SYSPRV, or the key off, it opens as before.
func TestExeOpenHostAccess(t *testing.T) {
	env, _ := fixture()
	p := env.Process
	fn := t.TempDir() + "/made.txt"

	nameAddr := uint32(0x1000)
	putString(t, env, nameAddr, fn)

	create := []uint32{nameAddr, posixOWronly | posixOCreat | posixOTrunc}

	setMode(env, registers.User, registers.User, 0x9000)

	p.CurrentPrivileges = privTMPMBX
	env.HostRequiresSYSPRV = true

	if fid, err := shimExeOpen(env, create); err != nil || fid != 0xFFFFFFFF {
		t.Fatalf("key on, no SYSPRV: %#x, %v, want -1", fid, err)
	}

	if _, err := os.Stat(fn); err == nil {
		t.Fatal("key on, no SYSPRV: the file was made")
	}

	p.CurrentPrivileges |= privSYSPRV

	fid, err := shimExeOpen(env, create)
	if err != nil || fid == 0xFFFFFFFF {
		t.Fatalf("key on, SYSPRV: %#x, %v", fid, err)
	}

	env.closeHostFile(fid)

	p.CurrentPrivileges = privTMPMBX
	env.HostRequiresSYSPRV = false

	fid, err = shimExeOpen(env, []uint32{nameAddr, 0})
	if err != nil || fid == 0xFFFFFFFF {
		t.Fatalf("key off: %#x, %v", fid, err)
	}

	env.closeHostFile(fid)
}
