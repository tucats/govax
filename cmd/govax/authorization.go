package main

import (
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/uaf"
)

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
