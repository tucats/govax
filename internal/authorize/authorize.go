package authorize

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// Store is where the authorization file is: corevms's System, which
// keeps a host file or one in memory.
type Store interface {
	// Authorization returns a copy of the file, to read.
	Authorization() (*uaf.File, error)

	// UpdateAuthorization calls change on the file and keeps what it did
	// if it returns nil and the file passes uaf's checks.
	UpdateAuthorization(change func(*uaf.File) error) error
}

// Utility is one run of AUTHORIZE.
type Utility struct {
	// Store is the authorization file.
	Store Store

	// Credentials are the process's, which ask for the file.
	Credentials security.Credentials

	// Out is where messages go: SYS$OUTPUT.
	Out io.Writer

	// Read reads a session's next command after the prompt: a command
	// procedure's data lines, or the terminal. ok is false at the end of
	// the input (Ctrl/Z), which ends the session as EXIT does.
	Read func(prompt string) (line string, ok bool)

	grammar *dcl.Grammar

	// What the command being run has done: the status it leaves, and
	// whether it changed the accounts or the identifiers.
	status          uint32
	changedAccounts bool
	changedRights   bool
}

// prompt is the session's prompt.
const prompt = "UAF> "

// fileProfile is the authorization file's security profile, SYSUAF.DAT's
// on VMS 7.1 (the probe's DIRECTORY/SECURITY: [SYSTEM], (RWE,RWE,,)).
var fileProfile = security.Profile{
	Owner:      security.SystemUIC,
	Protection: security.NoAccess.With(security.System, security.Read|security.Write|security.Execute).With(security.Owner, security.Read|security.Write|security.Execute),
	Name:       "SYSUAF.DAT;1",
}

// Run runs AUTHORIZE with command, the rest of the line that started it:
// that one command, or a session at the UAF> prompt when command is
// blank. It returns the condition value DCL is to leave in $STATUS.
func (u *Utility) Run(command string) uint32 {
	g, err := loadGrammar()
	if err != nil {
		fmt.Fprintf(u.Out, "%%%s\n", err)

		return vmsdef.Symbols["SS$_ABORT"]
	}

	u.grammar = g
	u.bind()

	if command = strings.TrimSpace(command); command != "" {
		return u.one(command)
	}

	return u.session()
}

// one runs a single command, AUTHORIZE's other way of being run: it
// prints only the command's messages (no DONEMSG at the end, as VMS
// 7.1's didn't) and leaves the command's status.
func (u *Utility) one(command string) uint32 {
	if !u.mayRead() {
		return ssNormal
	}

	return u.command(command)
}

// session reads commands until EXIT or the end of the input, then says
// whether the file and the rights database changed.
func (u *Utility) session() uint32 {
	if !u.mayRead() {
		return ssNormal
	}

	changedAccounts, changedRights := false, false

	for {
		line, ok := u.Read(prompt)
		if !ok {
			break
		}

		if isExit(line) {
			break
		}

		u.command(line)
		changedAccounts = changedAccounts || u.changedAccounts
		changedRights = changedRights || u.changedRights
	}

	if changedAccounts {
		u.message("DONEMSG", "")
	} else {
		u.message("NOMODS", "")
	}

	if changedRights {
		u.message("RDBDONEMSG", "")
	} else {
		u.message("RDBNOMODS", "")
	}

	return ssNormal
}

// isExit reports whether line is EXIT, abbreviated or not, which ends a
// session.
func isExit(line string) bool {
	word := strings.ToUpper(strings.TrimSpace(line))

	return word != "" && len(word) <= len("EXIT") && strings.HasPrefix("EXIT", word)
}

// command parses and runs one command line, and returns the status it
// leaves. A blank line or a comment does nothing.
func (u *Utility) command(line string) uint32 {
	u.status, u.changedAccounts, u.changedRights = ssNormal, false, false

	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "!") {
		return ssNormal
	}

	r, err := u.grammar.Parse(line)
	if err != nil {
		return u.parseFailure(err)
	}

	if err := u.grammar.Dispatch(r); err != nil {
		fmt.Fprintf(u.Out, "%%%s\n", err)

		return vmsdef.Symbols["SS$_ABORT"]
	}

	return u.status
}

// parseFailure shows a command the grammar couldn't parse as VMS 7.1's
// AUTHORIZE did, with CLI's messages (shown as CLI, not DCL), and
// returns CLI's condition with the inhibit bit: an unknown verb IVVERB,
// an unknown qualifier IVQUAL, each with the word as its segment, and a
// missing parameter INSFPRM.
func (u *Utility) parseFailure(err error) uint32 {
	ident, segment := "", ""

	var ve vmserrors.VMSError
	if errors.As(err, &ve) {
		switch {
		case ve.Status == vmserrors.CLI_MISSINGPARAMETER:
			ident = "INSFPRM"
		case ve.Status == vmserrors.CLI_UNRECOGNIZED && len(ve.Arguments) == 2:
			word := strings.TrimPrefix(fmt.Sprint(ve.Arguments[1]), "/")

			switch ve.Arguments[0] {
			case "verb":
				ident, segment = "IVVERB", word
			case "qualifier":
				ident, segment = "IVQUAL", word
			case "keyword":
				ident, segment = "IVKEYW", word
			}
		case ve.Status == vmserrors.CLI_AMBIGUOUS && len(ve.Arguments) == 2 && ve.Arguments[0] == "verb":
			ident, segment = "ABVERB", fmt.Sprint(ve.Arguments[1])
		case ve.Status == vmserrors.CLI_NOQUALIFIERVALUE && len(ve.Arguments) == 1:
			ident, segment = "NOVALU", fmt.Sprint(ve.Arguments[0])
		}
	}

	if ident == "" {
		fmt.Fprintf(u.Out, "%%%s\n", err)

		return vmsdef.Symbols["SS$_ABORT"]
	}

	return u.cliError(ident, segment)
}

// cliError shows CLI's warning ident, with segment on the line under it
// if not "", and returns its condition value with the inhibit bit, as
// AUTHORIZE leaves it.
func (u *Utility) cliError(ident, segment string) uint32 {
	code := vmsdef.LibrarySymbols["CLI$_"+ident]

	text := ident
	if m, ok := vmsdef.LookupMessage(code); ok {
		text = m.Text
	}

	fmt.Fprintf(u.Out, "%%CLI-%c-%s, %s\n", severityLetters[code&7], ident, text)

	if segment != "" {
		fmt.Fprintln(u.Out, vmserrors.SegmentLine(segment))
	}

	return code | stsInhibitMsg
}

// mayRead reports whether the process may read the authorization file,
// saying NAOFIL with RMS's PRV if not.
func (u *Utility) mayRead() bool { return u.mayAccess(security.Read) }

// mayWrite reports whether the process may change the authorization
// file, saying NAOFIL with RMS's PRV if not. VMS's AUTHORIZE opens the
// file for both at its start; govax asks each command for what it needs,
// so an account allowed only to read the file could SHOW it.
func (u *Utility) mayWrite() bool { return u.mayAccess(security.Read | security.Write) }

// mayAccess asks internal/security's rule for want of the file.
func (u *Utility) mayAccess(want security.Access) bool {
	if security.Check(u.Credentials, fileProfile, want).Granted {
		return true
	}

	u.message("NAOFIL", "")
	u.cause(rmsPRV)

	return false
}

// loadGrammar parses authorize.dcl, govax's own copy.
func loadGrammar() (*dcl.Grammar, error) {
	src, err := fs.ReadFile(bootdata.FS, "authorize.dcl")
	if err != nil {
		return nil, err
	}

	return dcl.ParseGrammar(string(src))
}

// bind binds each verb's handler. A handler sets u.status and the
// changed flags; an error it returns is govax's own failure.
func (u *Utility) bind() {
	g := u.grammar

	g.Bind("EXIT", func(int64, *dcl.Result) error { return nil })
	g.Bind("ADD", func(_ int64, r *dcl.Result) error { return u.add(r) })
	g.Bind("COPY", func(_ int64, r *dcl.Result) error { return u.copyAccount(r) })
	g.Bind("MODIFY", func(_ int64, r *dcl.Result) error { return u.modify(r, r.String("USERNAME")) })
	g.Bind("DEFAULT", func(_ int64, r *dcl.Result) error { return u.modify(r, "DEFAULT") })
	g.Bind("REMOVE", func(_ int64, r *dcl.Result) error { return u.remove(r) })
	g.Bind("RENAME", func(_ int64, r *dcl.Result) error { return u.rename(r) })
}
