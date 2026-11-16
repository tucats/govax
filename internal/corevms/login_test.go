package corevms

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmsdef"
)

// loginSystem returns process 1 on a System whose authorization file (in
// memory) has SMITH, holding PROJX, and DISABLED, with DISUSER.
func loginSystem(t *testing.T) *Environment {
	t.Helper()

	base, _ := fixture()
	sys := NewSystem(base.cpu, base.mem, base.Devices, base.Mounts)
	sys.SetSysgen(DefaultSysgen())

	env, err := NewEnvironment(sys, base.Logicals, bytes.NewReader(nil), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	tmpmbx, _ := uaf.PrivilegeBit("TMPMBX")
	sysprv, _ := uaf.PrivilegeBit("SYSPRV")

	err = sys.UpdateAuthorization(func(f *uaf.File) error {
		f.Identifiers = append(f.Identifiers, uaf.Identifier{Name: "PROJX", Value: uaf.FirstGeneral, Holders: []uaf.Holder{{UIC: uaf.NewUIC(0o300, 1)}}})

		smith := uaf.DefaultRecord()
		smith.Username, smith.Account, smith.Flags = "SMITH", "GOVAX", 0
		smith.UIC = uaf.NewUIC(0o300, 1)
		smith.Priority = 3
		smith.Privileges = uaf.Privileges(1<<tmpmbx | 1<<sysprv)
		smith.DefaultPrivileges = uaf.Privileges(1 << tmpmbx)
		smith.Quotas.ASTLM, smith.Quotas.MAXJOBS = 33, 7
		smith.LastNonInteractive = time.Date(2026, 11, 15, 17, 19, 48, 0, time.Local)
		f.Put(smith)

		disabled := uaf.DefaultRecord()
		disabled.Username = "DISABLED"
		f.Put(disabled)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return env
}

// TestLogIn: a login gives process 1 the account's identity, privileges,
// priority, quotas, and rights, the last-login facts the record had, and
// notes this login's time in the record; DISUSER and a missing account
// are refused with LOGINOUT's messages.
func TestLogIn(t *testing.T) {
	env := loginSystem(t)
	sys := env.System

	l, err := sys.BeginLogin("smith", true)
	if err != nil {
		t.Fatal(err)
	}

	env.LogIn(l)

	p := env.Process
	if p.Username != "SMITH" || p.Name != "SMITH" || p.Account != "GOVAX" || p.UIC != 0x00C00001 {
		t.Errorf("identity %s %s %s %#x", p.Username, p.Name, p.Account, p.UIC)
	}

	if env.Logicals.UIC() != 0x00C00001 {
		t.Errorf("logical names' UIC %#x", env.Logicals.UIC())
	}

	if p.AuthorizedPrivileges&security.PrivSYSPRV == 0 || p.CurrentPrivileges&security.PrivSYSPRV != 0 || p.ProcessPrivileges != p.CurrentPrivileges {
		t.Errorf("privileges auth %#x process %#x current %#x", p.AuthorizedPrivileges, p.ProcessPrivileges, p.CurrentPrivileges)
	}

	if p.BasePriority != 3 || p.AuthorizedPriority != 3 || p.ASTLimit != 33 || p.Job.SubprocessLimit != 2 || p.Job.Pooled.BYTLM != 32768 {
		t.Errorf("priority %d/%d ASTLM %d PRCLM %d BYTLM %d", p.BasePriority, p.AuthorizedPriority, p.ASTLimit, p.Job.SubprocessLimit, p.Job.Pooled.BYTLM)
	}

	var values []uint32
	for _, r := range p.Rights {
		values = append(values, r.Value)
	}

	want := []uint32{vmsdef.Symbols["KGB$K_INTERACTIVE_ID"], vmsdef.Symbols["KGB$K_LOCAL_ID"], uaf.FirstGeneral}
	if len(values) != 3 || values[0] != want[0] || values[1] != want[1] || values[2] != want[2] {
		t.Errorf("rights %x, want %x", values, want)
	}

	if p.LoginTime != l.Time || p.LastLogin.Interactive != 0 || p.LastLogin.NonInteractive == 0 || p.LastLogin.MaxJobs != 7 {
		t.Errorf("login %d, last %+v", p.LoginTime, p.LastLogin)
	}

	if got := vmsdef.GoTime(p.LastLogin.NonInteractive).Format("02-Jan-2006 15:04:05"); got != "15-Nov-2026 17:19:48" {
		t.Errorf("last non-interactive %s", got)
	}

	if text := p.LastLogin.LastLoginText(); text != "    Last non-interactive login on Sunday, 15-NOV-2026 17:19\n" {
		t.Errorf("banner %q", text)
	}

	// The login's time is in the record now, so the next login reports it.
	l2, err := sys.BeginLogin("SMITH", false)
	if err != nil {
		t.Fatal(err)
	}

	if got := l2.Previous().Interactive; got != l.Time {
		t.Errorf("recorded interactive login %d, want %d", got, l.Time)
	}

	env.LogIn(l2)

	if p.Rights[0].Value != vmsdef.Symbols["KGB$K_BATCH_ID"] {
		t.Errorf("a non-interactive login's rights %+v", p.Rights)
	}

	for name, ident := range map[string]string{"DISABLED": "DISUSER", "NOBODY": "NOSUCHUSER"} {
		_, err := sys.BeginLogin(name, true)

		var le *LoginError
		if !errors.As(err, &le) || le.Ident != ident || le.Status != vmsdef.LibrarySymbols["LGI$_"+ident] {
			t.Errorf("%s: %v", name, err)
		}
	}

	if _, err := sys.BeginLogin("DISABLED", true); !strings.HasPrefix(err.Error(), "LOGIN-F-DISUSER, account is disabled") {
		t.Errorf("DISUSER's message %q", err)
	}
}

// TestLoginItems: F$GETJPI's account items (JPIItem by name), and that
// $GETJPI's code table doesn't need them.
func TestLoginItems(t *testing.T) {
	env := loginSystem(t)

	if data, ok := env.JPIItem("JPI$_LAST_LOGIN_I"); !ok || ItemQuadword(data) != 0 {
		t.Errorf("LAST_LOGIN_I before login %q %v", data, ok)
	}

	l, err := env.System.BeginLogin("SMITH", true)
	if err != nil {
		t.Fatal(err)
	}

	env.LogIn(l)

	for name, want := range map[string]uint32{"JPI$_MAXJOBS": 7, "JPI$_MAXDETACH": 0, "JPI$_SHRFILLM": 0, "JPI$_UAF_FLAGS": 0, "JPI$_LOGIN_FAILURES": 0, "JPI$_LOGIN_FLAGS": 0} {
		if data, ok := env.JPIItem(name); !ok || ItemLongword(data) != want {
			t.Errorf("%s = %q %v, want %d", name, data, ok, want)
		}
	}

	if data, _ := env.JPIItem("JPI$_LAST_LOGIN_N"); ItemQuadword(data) != env.Process.LastLogin.NonInteractive {
		t.Errorf("LAST_LOGIN_N %q", data)
	}

	if text := env.ItemText(JPIItemForm("JPI$_LAST_LOGIN_I", StyleDCL), string(make([]byte, 8)), StyleDCL); !strings.HasPrefix(text, "17-NOV-1858") {
		t.Errorf("no last login written %q", text)
	}
}
