package console_test

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/testproc"
)

// TestCreprcInheritsProtection (Phase 93's subtask 5): a process gets its
// creator's default file protection, whatever it is then, and the
// System's MAXSYSGROUP; a subprocess holds its owner's rights, and a
// detached process none.
func TestCreprcInheritsProtection(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	c, _ := scheduledConsole(t, longQuantum, nil)
	one := c.RTL
	one.Process.DefaultProtection = 0xAF00

	sub, st := one.CreateProcess(corevms.CreateRequest{Name: "SUB"})
	if st != 1 {
		t.Fatalf("subprocess: status %08X", st)
	}

	detached, st := one.CreateProcess(corevms.CreateRequest{Name: "DET", UIC: uint32(security.NewUIC(0o200, 1)), Input: "NL:"})
	if st != 1 {
		t.Fatalf("detached: status %08X", st)
	}

	for _, tc := range []struct {
		name   string
		env    *corevms.Environment
		rights int
	}{{"subprocess", sub, len(one.Process.Rights)}, {"detached", detached, 0}} {
		p := tc.env.Process
		if p.DefaultProtection != 0xAF00 || p.MaxSysGroup != one.Sysgen.MaxSysGroup || len(p.Rights) != tc.rights {
			t.Errorf("%s: protection %#04x, MAXSYSGROUP %o, %d rights; want %%XAF00, %o, %d",
				tc.name, p.DefaultProtection, p.MaxSysGroup, len(p.Rights), one.Sysgen.MaxSysGroup, tc.rights)
		}
	}

	if len(one.Process.Rights) != 2 {
		t.Errorf("process 1 holds %d rights, want INTERACTIVE and LOCAL", len(one.Process.Rights))
	}

	// A subprocess's session asks it, not its creator, for credentials.
	if got := detached.Session.Identity().UIC; got != security.NewUIC(0o200, 1) {
		t.Errorf("the detached process's session acts as %s, want [200,1]", got)
	}
}
