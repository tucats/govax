package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmserrors"
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
// which makes govax exit with status 1.
func checkLogin(path string, out io.Writer) error {
	f, err := uaf.Load(path)
	if err != nil {
		return err
	}

	if _, err := corevms.LoginRecord(f, loginAccount()); err != nil {
		fmt.Fprintln(out, "%"+err.Error())

		return vmserrors.InhibitMessage(err)
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
