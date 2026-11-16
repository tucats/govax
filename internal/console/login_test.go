package console

import (
	"testing"

	"github.com/tucats/govax/internal/uaf"
)

// TestLogInSurvivesVMInit: process 1 logged in as an account stays that
// account through INITIALIZE/MEMORY, which makes a new System and a new
// process 1, and no second login is recorded.
func TestLogInSurvivesVMInit(t *testing.T) {
	c := bootedConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t))

	err := c.RTL.System.UpdateAuthorization(func(f *uaf.File) error {
		r := uaf.DefaultRecord()
		r.Username, r.Flags, r.UIC = "SMITH", 0, uaf.NewUIC(0o300, 1)
		f.Put(r)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	l, err := c.LogIn("SMITH", "")
	if err != nil {
		t.Fatal(err)
	}

	before := c.RTL
	if before.Process.Username != "SMITH" {
		t.Fatalf("logged in as %s", before.Process.Username)
	}

	mustDispatch(t, d, "INIT/VAX ^D8192", "INITIALIZE/MEMORY")

	p := c.RTL.Process
	if c.RTL == before || p.Username != "SMITH" || p.UIC != 0x00C00001 || p.LoginTime != l.Time {
		t.Errorf("after VMINIT: new %v, user %s, UIC %#x, login %d (want %d)", c.RTL != before, p.Username, p.UIC, p.LoginTime, l.Time)
	}
}
