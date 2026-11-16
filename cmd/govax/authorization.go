package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmserrors"
	"golang.org/x/term"
)

// loginUser is --user's account, "" for SYSTEM (docs/planning/PHASE-95 -
// authentication.md, Decision 6).
var loginUser string

// loginAccount is the account process 1 logs in as: --user's, in
// capitals, or SYSTEM.
func loginAccount() string {
	if name := strings.ToUpper(strings.TrimSpace(loginUser)); name != "" {
		return name
	}

	return uaf.SystemRecord().Username
}

// checkLogin is LOGINOUT's check of the account before anything runs:
// one the file doesn't have, or one with DISUSER, is its message on out
// ("%LOGIN-F-DISUSER, account is disabled") and an error already shown,
// which makes govax exit with status 1. An account with a password must
// be given it (checkPassword); realStdin says whether standard input is
// govax's own, which may be a terminal, rather than a test's.
func checkLogin(path string, out io.Writer, realStdin bool) error {
	f, err := uaf.Load(path)
	if err != nil {
		return err
	}

	r, err := corevms.LoginRecord(f, loginAccount())
	if err != nil {
		fmt.Fprintln(out, "%"+err.Error())

		return vmserrors.InhibitMessage(err)
	}

	return checkPassword(r, out, realStdin)
}

// The password (docs/planning/PHASE-95 - authentication.md, Decision 6's
// "The password"). An account with a password must be given it before it
// logs in: at a terminal govax asks, with echo off, three times at most;
// otherwise it takes GOVAX_PASSWORD, never reading standard input, whose
// lines are the session's commands.

// passwordVariable is the environment variable that gives the password
// when standard input isn't a terminal.
const passwordVariable = "GOVAX_PASSWORD"

// passwordTries is how many times govax asks at a terminal.
const passwordTries = 3

// authorizationFailure is LOGINOUT's one message for a login it refuses
// for its username or password, which never says which was wrong.
const authorizationFailure = "User authorization failure"

// passwordReader reads one password typed at the terminal, echo off.
type passwordReader func() (string, error)

// passwordInput returns the reader for a password typed at the terminal,
// and false when standard input isn't a terminal. The tests replace it.
var passwordInput = func(out io.Writer) (passwordReader, bool) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, false
	}

	return func() (string, error) {
		typed, err := term.ReadPassword(fd)

		// The Return typed wasn't echoed either.
		fmt.Fprintln(out)

		return string(typed), err
	}, true
}

// checkPassword asks for r's password when it has one, and returns an
// error already shown when it isn't given: three wrong answers at a
// terminal, a wrong GOVAX_PASSWORD, or neither a terminal nor the
// variable. An account with no password logs in asked for nothing.
func checkPassword(r uaf.Record, out io.Writer, realStdin bool) error {
	if r.Password == "" {
		return nil
	}

	refused := errors.New(authorizationFailure)

	matches := func(typed string) bool {
		ok, err := uaf.CheckPassword(r.Password, typed)

		return err == nil && ok
	}

	if realStdin {
		if read, ok := passwordInput(out); ok {
			for range passwordTries {
				fmt.Fprint(out, "Password: ")

				typed, err := read()
				if err != nil {
					break
				}

				if matches(typed) {
					return nil
				}

				fmt.Fprintln(out, authorizationFailure)
			}

			return vmserrors.InhibitMessage(refused)
		}
	}

	typed, ok := os.LookupEnv(passwordVariable)
	if !ok {
		fmt.Fprintf(out, "%%GOVAX-F-PASSWORD, account %s has a password; standard input isn't a terminal, so set %s to it\n",
			r.Username, passwordVariable)

		return vmserrors.InhibitMessage(refused)
	}

	if !matches(typed) {
		fmt.Fprintln(out, authorizationFailure)

		return vmserrors.InhibitMessage(refused)
	}

	return nil
}

// authorizationFile, when set, is the authorization file's path in place
// of vax.authorization.file's and the default: the tests' (TestMain
// gives each run a temporary one), never the author's own.
var authorizationFile string

// ensureAuthorizationFile returns the authorization file's host path
// (authorizationFile, else vax.authorization.file, else
// ~/.govax/sysuaf.json, "~/" the home directory), making the file with
// uaf.Default's accounts if it isn't there (docs/planning/PHASE-95 -
// authentication.md, Decision 2). A file that exists but can't be read
// is an error naming the file and the line.
func ensureAuthorizationFile() (string, error) {
	path := authorizationFile
	if path == "" {
		path = strings.TrimSpace(settings.Get("vax.authorization.file"))
	}

	if path == "" {
		path = uaf.DefaultPath
	}

	path, err := uaf.ExpandHome(path)
	if err != nil {
		return "", err
	}

	if _, err := uaf.Ensure(path); err != nil {
		return "", err
	}

	return path, nil
}
