package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/testproc"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmserrors"
)

// loginSession is a console set up as the probe's VAX was for round 3
// (testdata/uaf95's README): a volume UAF95 as DUA1, UAF95$DISK, the
// probe's procedures on it, SYS$SYLOGIN naming UAF95S.COM, and the
// accounts UAF95 ([200,5], no LGICMD) and UAF95L ([200,7], LGICMD
// UAF95X.COM), whose home is UAF95$DISK:[UAF95], owned by [200,5].
type loginSession struct {
	t        *testing.T
	c        *console.Console
	dispatch func(string) string
	run      func(string) error
	buf      interface {
		String() string
		Reset()
	}
}

func newLoginSession(t *testing.T) *loginSession {
	t.Helper()

	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
	s := &loginSession{t: t, c: c, run: d.Dispatch, buf: buf}

	s.dispatch = func(line string) string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		return strings.ReplaceAll(buf.String(), "\r", "")
	}

	dsk := filepath.Join(t.TempDir(), "uaf95.dsk")

	for _, line := range []string{
		`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD53 UAF95`,
		`MOUNT/WRITE DUA1 "` + dsk + `"`,
		"DEFINE/SYSTEM/EXECUTIVE_MODE UAF95$DISK DUA1:",
		"CREATE/DIRECTORY DUA1:[UAF95]",
		"SET DEFAULT DUA1:[000000]",
	} {
		s.dispatch(line)
	}

	for _, f := range []struct{ name, dir string }{
		{"uaf95s.com", "[000000]"}, {"login.com", "[UAF95]"}, {"uaf95x.com", "[UAF95]"},
	} {
		src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "uaf95", f.name))
		if err != nil {
			t.Fatal(err)
		}

		if out := s.dispatch(`COPY "` + src + `"/HOST DUA1:` + f.dir + strings.ToUpper(f.name) + "/QUIET"); out != "" {
			t.Fatalf("COPY %s: %s", f.name, out)
		}
	}

	// UAF95ACCT.COM's protection: SYLOGIN's file readable by everyone, the
	// home and its files the account's.
	for _, line := range []string{
		"SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) DUA1:[000000]UAF95S.COM",
		"SET SECURITY/OWNER=[200,5]/PROTECTION=(S:RWE,O:RWE,G:RWE,W) DUA1:[000000]UAF95.DIR",
		"SET SECURITY/OWNER=[200,5]/PROTECTION=(S:RWED,O:RWED,G:RWED,W) DUA1:[UAF95]*.COM;*",
		`DEFINE/SYSTEM/EXECUTIVE_MODE SYS$SYLOGIN UAF95$DISK:[000000]UAF95S.COM`,
	} {
		if out := s.dispatch(line); out != "" {
			t.Fatalf("%s: %s", line, out)
		}
	}

	err := c.RTL.System.UpdateAuthorization(func(f *uaf.File) error {
		for _, a := range []struct {
			name   string
			member uint32
			lgicmd string
		}{{"UAF95", 5, ""}, {"UAF95L", 7, "UAF95$DISK:[UAF95]UAF95X.COM"}} {
			r := uaf.DefaultRecord()
			r.Username, r.Flags, r.UIC = a.name, 0, uaf.NewUIC(0o200, a.member)
			r.Owner, r.Account = "GOVAX PROBE", "GOVAX"
			r.Device, r.Directory, r.LGICMD = "UAF95$DISK:", "[UAF95]", a.lgicmd
			f.Put(r)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return s
}

// logIn logs process 1 in as user and runs its login procedures,
// returning what they wrote.
func (s *loginSession) logIn(user string) string {
	s.t.Helper()

	l, err := s.c.LogIn(user, "")
	if err != nil {
		s.t.Fatal(err)
	}

	s.buf.Reset()

	if err := s.c.RunLoginProcedures(l.Record.LGICMD, s.run); err != nil {
		s.t.Fatalf("login procedures: %v", err)
	}

	return strings.ReplaceAll(s.buf.String(), "\r", "")
}

// probeSection returns the lines of a log between the line starting with
// heading and the next "$ " line.
func probeSection(t *testing.T, log, heading string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "uaf95", "vax", "round3", log))
	if err != nil {
		t.Fatal(err)
	}

	var lines []string

	in := false

	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		switch {
		case strings.HasPrefix(line, heading):
			in = true
		case in && strings.HasPrefix(line, "$ "):
			return lines
		case in:
			lines = append(lines, line)
		}
	}

	return lines
}

// TestUAF95LoginOracle holds the login procedures to VMS 7.1's (the
// probe's round 3, testdata/uaf95/vax/round3): SYLOGIN first, at depth 2,
// then LOGIN.COM or the LGICMD (never both) at depth 1, each with verify
// off, P1 empty, the home as the default and SYS$LOGIN, the account's
// privileges; the LGICMD's warning goes on and its error ends it, with
// VMS's messages, and the login goes on.
func TestUAF95LoginOracle(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	for _, tc := range []struct{ user, log string }{
		{"UAF95", "uaf95i_uaf95_interactive.log"},
		{"UAF95L", "uaf95i_uaf95l_interactive.log"},
	} {
		s := newLoginSession(t)
		said := s.logIn(tc.user)

		if tc.user == "UAF95L" {
			want := []string{
				"%DCL-W-IVVERB, unrecognized command verb - check validity and spelling",
				` \NOSUCHVERB\`,
				"%DCL-E-OPENIN, error opening DUA1:[UAF95]NOSUCH.FILE; as input",
				"-RMS-E-FNF, file not found",
			}
			if got := strings.TrimSpace(said); got != strings.Join(want, "\n") {
				t.Errorf("%s's login said:\n%s\nwant (VMS 7.1's terminal.log):\n%s", tc.user, got, strings.Join(want, "\n"))
			}
		} else if strings.TrimSpace(said) != "" {
			t.Errorf("%s's login said:\n%s", tc.user, said)
		}

		got := strings.Split(strings.TrimRight(s.dispatch("SHOW SYMBOL/GLOBAL UAF95_*"), "\n"), "\n")
		want := probeSection(t, tc.log, "######## WHAT THE LOGIN PROCEDURES SAW")

		if len(got) != len(want) {
			t.Errorf("%s: %d symbols, want %d:\n%s", tc.user, len(got), len(want), strings.Join(got, "\n"))

			continue
		}

		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: %q\n          want %q", tc.user, got[i], want[i])
			}
		}
	}
}

// TestLoginProceduresSkipped: a login whose procedures aren't there, or
// can't be read, goes on without a message, SYLOGIN then running at
// level 1; with no SYS$SYLOGIN only the account's runs.
func TestLoginProceduresSkipped(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	s := newLoginSession(t)

	// UAF95's LOGIN.COM unreadable to it.
	s.dispatch("SET SECURITY/OWNER=[1,4]/PROTECTION=(S:RWED,O:RWED,G,W) DUA1:[UAF95]LOGIN.COM")

	if said := s.logIn("UAF95"); strings.TrimSpace(said) != "" {
		t.Errorf("the login said:\n%s", said)
	}

	out := s.dispatch("SHOW SYMBOL/GLOBAL UAF95_ORDER")
	depth := s.dispatch("SHOW SYMBOL/GLOBAL UAF95_SYLOGIN_DEPTH")

	if !strings.Contains(out, `",SYLOGIN"`) || !strings.Contains(depth, "== 1 ") {
		t.Errorf("SYLOGIN alone:\n%s%s", out, depth)
	}

	s = newLoginSession(t)
	s.dispatch("DEASSIGN/SYSTEM/EXECUTIVE_MODE SYS$SYLOGIN")
	s.logIn("UAF95")

	if out := s.dispatch("SHOW SYMBOL/GLOBAL UAF95_ORDER"); !strings.Contains(out, `",LOGIN"`) {
		t.Errorf("no SYS$SYLOGIN:\n%s", out)
	}
}

// ctrlYReader is a terminal whose first read types Ctrl/Y, as
// dclcontroly_test.go's keyReader does.
type ctrlYReader struct {
	r     *strings.Reader
	first func()
}

func (k *ctrlYReader) Read(p []byte) (int, error) {
	if k.first != nil {
		k.first()
		k.first = nil
	}

	return k.r.Read(p)
}

// TestLoginProceduresControlY: Ctrl/Y in SYLOGIN gives DCL's Ctrl/Y
// level, as in any procedure: CONTINUE goes on, SYLOGIN and then the
// account's procedure; EXIT ends the login procedures, as it ends every
// procedure (the User's Manual, 13.7.1), and the login goes on to the
// prompt.
func TestLoginProceduresControlY(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	for _, tc := range []struct{ typed, want string }{
		{"y\nCONTINUE\n", `UAF95_ORDER == "SYLOGIN WENT ON,LOGIN"`},
		{"y\nEXIT\n", "UNDSYM"},
	} {
		s := newLoginSession(t)

		proc := filepath.Join(t.TempDir(), "ylog.com")
		if err := os.WriteFile(proc, []byte("$ INQUIRE X\n$ UAF95_ORDER == \"SYLOGIN WENT ON\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		for _, line := range []string{
			`COPY "` + proc + `"/HOST DUA1:[000000]YLOG.COM/QUIET`,
			"SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) DUA1:[000000]YLOG.COM",
			"DEFINE/SYSTEM/EXECUTIVE_MODE SYS$SYLOGIN UAF95$DISK:[000000]YLOG.COM",
		} {
			if out := s.dispatch(line); out != "" && !strings.Contains(out, "SUPERSEDE") {
				t.Fatalf("%s: %s", line, out)
			}
		}

		s.c.In = &ctrlYReader{r: strings.NewReader(tc.typed), first: func() {
			if s.c.ControlYEnabled() {
				s.c.Engine.AttentionKey(cpu.AttentionCtrlY)
			}
		}}

		said := s.logIn("UAF95")

		if out := s.dispatch("SHOW SYMBOL/GLOBAL UAF95_ORDER"); !strings.Contains(out, tc.want) {
			t.Errorf("Ctrl/Y, then %q: %s%s", tc.typed, said, out)
		}

		if s.c.CommandLevel() != 0 {
			t.Errorf("level %d after the login", s.c.CommandLevel())
		}
	}
}
