package dclshell

import (
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmserrors"
)

// The login procedures (docs/planning/PHASE-95 - authentication.md,
// Decision 8). After a process logs in, DCL runs two command procedures
// before it reads the user's first command:
//
//   - the system's, which the logical name SYS$SYLOGIN names (STARTUP.COM
//     defines it as SYS$MANAGER:SYLOGIN), for every account;
//   - the account's own: the file its LGICMD names, or, when it names
//     none, LOGIN.COM in the home directory (SYS$LOGIN:LOGIN.COM). Never
//     both.
//
// VMS 7.1 (the probe's round 3, testdata/uaf95/vax/round3) puts the
// account's procedure at command level 1 and runs SYLOGIN above it, at
// level 2, before the account's first line, so F$ENVIRONMENT("DEPTH") is
// 2 in SYLOGIN and 1 in the account's procedure; when the account has no
// procedure that can run, SYLOGIN is at level 1. A procedure that isn't
// there, or that the user may not read, is passed over without a message
// (round 2). Each follows DCL's default ON action: a warning goes on, an
// error ends that procedure, and the login goes on to the prompt.
// Verification is off in both.

// The names the login procedures are found by. SYLOGIN is opened by the
// logical name alone, which RMS translates whole, as a name with no
// punctuation (rms.Session.Locate), before @ gives it its default type.
const (
	syloginName    = "SYS$SYLOGIN"
	loginProcedure = "SYS$LOGIN:LOGIN.COM"
)

// RunLoginProcedures runs the login procedures of the process whose DCL
// runs now (process 1's, or a batch job's), SYLOGIN and then
// lgicmd (an account's LGICMD; "" for SYS$LOGIN:LOGIN.COM), sending their
// lines to dispatch. A failure the procedures have already shown isn't
// returned again; any other error is.
func (c *Interpreter) RunLoginProcedures(lgicmd string, dispatch func(string) error) error {
	c.Verify = false

	runSylogin := func() {
		if !c.loginNameDefined(syloginName) {
			return
		}

		if _, err := c.loginProcedure(syloginName, nil, dispatch); err != nil && !vmserrors.MessageInhibited(err) {
			c.Printf("%%%s\n", err.Error())
		}
	}

	account := strings.TrimSpace(lgicmd)
	if account == "" {
		// With no home (the host's directory, as before accounts) there
		// is no SYS$LOGIN, and so no LOGIN.COM to look for.
		if !c.loginNameDefined(loginName) {
			runSylogin()

			return nil
		}

		account = loginProcedure
	}

	ran, err := c.loginProcedure(account, runSylogin, dispatch)
	if !ran {
		runSylogin()
	}

	if err != nil && !vmserrors.MessageInhibited(err) {
		return err
	}

	return nil
}

// loginProcedure runs the procedure file at a new command level, first
// running before (if any) once the level is there, as SYLOGIN runs above
// the account's procedure. ran is false, and nothing is shown, when the
// file can't be opened: a login procedure that is missing or unreadable is
// passed over.
func (c *Interpreter) loginProcedure(file string, before func(), dispatch func(string) error) (ran bool, err error) {
	if len(c.levels) >= MaxCommandLevels {
		return false, nil
	}

	source, err := c.openProcedure(file, false)
	if err != nil {
		return false, nil
	}

	return true, c.finishProcedure(c.runLevel(source, procedureCommand{file: file, before: before}, dispatch))
}

// loginNameDefined reports whether name translates as a file name would
// be translated: SYS$SYLOGIN, which a system without a system volume
// doesn't define, or SYS$LOGIN, which a login with no home doesn't.
func (c *Interpreter) loginNameDefined(name string) bool {
	_, err := c.Logicals.Translate(lnm.FileDevName, name, lnm.User, 0)

	return err == nil
}
