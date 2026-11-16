package console

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// SHOW SECURITY/CLASS=QUEUE and SET SECURITY/CLASS=QUEUE
// (docs/planning/PHASE-98 - batch queues.md, subtask 5), held to VMS
// 7.1's run of the probe (testdata/bat98/vax/bat98.log, cases 9 and 34
// to 43). The queues are made in the database directly: INITIALIZE/QUEUE
// and SET QUEUE are subtask 6's.

// queueSecurityConsole is a console whose System has a queue database
// holding BAT98_A (VMS's defaults) and BAT98_D (owner [200,10],
// (S:M,O:D,G:R,W)), as the probe made them, and the identifier BAT98_ID;
// and a function running a command and returning what it printed with
// $STATUS after it.
func queueSecurityConsole(t *testing.T) (*Console, func(string) string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "qman.json")

	err := jobqueue.Update(path, func(d *jobqueue.Database) error {
		if _, _, err := d.CreateQueue("BAT98_A", nil); err != nil {
			return err
		}

		_, _, err := d.CreateQueue("BAT98_D", func(q *jobqueue.Queue) {
			q.Owner = jobqueue.UIC(security.NewUIC(0o200, 0o10))
			q.Protection, _ = jobqueue.ParseProtection("(S:M,O:D,G:R,W)", 0)
		})

		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	c, buf := newTestConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t))

	c.QueueFile = path
	c.RTL.System.SetQueueFile(path)

	if status := c.RTL.AddIdentifier("BAT98_ID", 0x80010098, 0); status != 1 {
		t.Fatalf("AddIdentifier: %#x", status)
	}

	run := func(line string) string {
		t.Helper()

		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString(dclMessage(err) + "\n")
		}

		out := buf.String()

		buf.Reset()

		if err := d.Dispatch("SHOW SYMBOL $STATUS"); err != nil {
			t.Fatal(err)
		}

		return out + buf.String()
	}

	return c, run
}

func TestQueueSecurityAsSystem(t *testing.T) {
	c, run := queueSecurityConsole(t)

	// Each step is a command and VMS's output, but that the VAX named
	// [200,10] [PROT93,BAT98] (an identifier of its own), which govax,
	// with no such account, shows as the UIC.
	steps := []struct{ command, want string }{
		{"SHOW SECURITY/CLASS=QUEUE BAT98_A", `
BAT98_A object of class QUEUE
     Owner: [SYSTEM]
     Protection: (System: M, Owner: D, Group: R, World: S)
     Access Control List: <empty>
  $STATUS == "%X00000001"
`},
		{"SHOW SECURITY/CLASS=QUEUE BAT98_D", `
BAT98_D object of class QUEUE
     Owner: [200,10]
     Protection: (System: M, Owner: D, Group: R, World)
     Access Control List: <empty>
  $STATUS == "%X00000001"
`},
		{"SET SECURITY/CLASS=QUEUE/PROTECTION=(W:RS) BAT98_D", `  $STATUS == "%X00000001"
`},
		{"SET SECURITY/CLASS=QUEUE/ACL=(IDENTIFIER=BAT98_ID,ACCESS=READ+SUBMIT) BAT98_D", `  $STATUS == "%X00000001"
`},
		{"SHOW SECURITY/CLASS=QUEUE BAT98_D", `
BAT98_D object of class QUEUE
     Owner: [200,10]
     Protection: (System: M, Owner: D, Group: R, World: RS)
     Access Control List:
          (IDENTIFIER=BAT98_ID,ACCESS=READ+SUBMIT)
  $STATUS == "%X00000001"
`},
	}

	for _, s := range steps {
		if got := run(s.command); got != strings.TrimPrefix(s.want, "\n") && got != s.want {
			t.Errorf("%s:\n got %q\nwant %q", s.command, got, s.want)
		}
	}

	// Case 39's SET QUEUE, in the database (subtask 6's command).
	err := jobqueue.Update(c.QueueFile, func(d *jobqueue.Database) error {
		return d.SetQueue("BAT98_D", func(q *jobqueue.Queue) {
			q.Owner = jobqueue.SystemUIC
			q.Protection, _ = jobqueue.ParseProtection("(S:M,O:M,G:M,W:M)", 0)
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	steps = []struct{ command, want string }{
		{"SET SECURITY/CLASS=QUEUE/ACL/DELETE BAT98_D", `  $STATUS == "%X00000001"
`},
		{"SHOW SECURITY/CLASS=QUEUE BAT98_D", `
BAT98_D object of class QUEUE
     Owner: [SYSTEM]
     Protection: (System: M, Owner: M, Group: M, World: M)
     Access Control List: <empty>
  $STATUS == "%X00000001"
`},
	}

	for _, s := range steps {
		if got := run(s.command); got != s.want {
			t.Errorf("%s:\n got %q\nwant %q", s.command, got, s.want)
		}
	}

	if got := run("SHOW SECURITY/CLASS=QUEUE BAT98_NONE"); !strings.HasPrefix(got, "%JBC-E-NOSUCHQUE, no such queue\n") ||
		!strings.Contains(got, `"%X1004803A"`) {
		t.Errorf("a missing queue: %q", got)
	}
}

// TestQueueSecurityRefused is the matrix's cases 23 and 24 as BAT98W,
// [300,10], in state A (BAT98_A, VMS's defaults): world access is submit
// alone, so neither command may see the queue's profile, and each is
// SS$_NOPRIV alone (bat98p_a_bat98w.log); and BAT98, the owner of
// BAT98_D, may change it (state B's case 23).
func TestQueueSecurityRefused(t *testing.T) {
	c, run := queueSecurityConsole(t)

	privilege := func(name string) uint64 { return 1 << vmsdef.Symbols["PRV$V_"+name] }
	p := c.RTL.Process
	p.UIC = uint32(security.NewUIC(0o300, 0o10))
	p.ProcessPrivileges = privilege("NETMBX") | privilege("TMPMBX")
	p.CurrentPrivileges = p.ProcessPrivileges

	const noPriv = "%SYSTEM-F-NOPRIV, insufficient privilege or object protection violation\n  $STATUS == \"%X10000024\"\n"

	for _, command := range []string{
		"SET SECURITY/CLASS=QUEUE/ACL=(IDENTIFIER=[300,10],ACCESS=READ) BAT98_A",
		"SHOW SECURITY/CLASS=QUEUE BAT98_A",
	} {
		if got := run(command); got != noPriv {
			t.Errorf("%s as BAT98W:\n got %q\nwant %q", command, got, noPriv)
		}
	}

	p.UIC = uint32(security.NewUIC(0o200, 0o10))

	if got := run("SET SECURITY/CLASS=QUEUE/ACL=(IDENTIFIER=[300,10],ACCESS=READ) BAT98_D"); got != "  $STATUS == \"%X00000001\"\n" {
		t.Errorf("SET SECURITY as BAT98_D's owner: %q", got)
	}

	// With OPER, anyone may. (The ACE's identifier is shown by name: the
	// process's own UIC is named by its username, SYSTEM's here.)
	p.UIC = uint32(security.NewUIC(0o300, 0o10))
	p.CurrentPrivileges |= privilege("OPER")

	if got := run("SHOW SECURITY/CLASS=QUEUE BAT98_D"); !strings.Contains(got, ",ACCESS=READ)\n") {
		t.Errorf("SHOW SECURITY with OPER: %q", got)
	}
}
