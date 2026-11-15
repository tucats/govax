package corevms

import (
	"os"
	"time"

	"github.com/tucats/govax/internal/uaf"
)

// The authorization file (docs/planning/PHASE-95 - authentication.md,
// Decisions 1 and 2): govax's SYSUAF.DAT and RIGHTSLIST.DAT, the accounts
// and identifiers internal/uaf keeps. A System reads it whenever it needs
// to know about an account or an identifier (users.go, rights.go).
//
// cmd/govax names a host file (~/.govax/sysuaf.json, or
// vax.authorization.file's), which every System of the session reads, so
// what AUTHORIZE writes is there for the next VMINIT and the next session
// too. With no file named (the tests, mostly) the System keeps one in
// memory, starting as uaf.Default's, and like the mailboxes it goes with
// the System. Either way AddUser and AddIdentifier change it, standing
// for AUTHORIZE's ADD and ADD/IDENTIFIER, which is how the tests make a
// probe's accounts.

// authorization is a System's authorization file.
type authorization struct {
	// path is the host file, "" for one in memory.
	path string

	// mem is the file in memory when there's no path; nil until first
	// needed.
	mem *uaf.File

	// cached is the host file as last read, and its size and time then,
	// so that it's read again only when it has changed: the rights
	// database is asked for at every $IDTOASC call.
	cached      *uaf.File
	cachedSize  int64
	cachedMTime time.Time
}

// SetAuthorizationFile makes path, a host file, the System's
// authorization file; "" keeps one in memory. cmd/govax has made the file
// (uaf.Ensure) before it starts the machine.
func (s *System) SetAuthorizationFile(path string) {
	s.auth = authorization{path: path}
}

// AuthorizationFile is the System's authorization file's host path, ""
// when it keeps the file in memory.
func (s *System) AuthorizationFile() string { return s.auth.path }

// Authorization returns a copy of the authorization file, to read: the
// host file as it is now, or the one in memory.
func (s *System) Authorization() (*uaf.File, error) {
	a := &s.auth

	if a.path == "" {
		if a.mem == nil {
			a.mem = uaf.Default()
		}

		return a.mem.Clone(), nil
	}

	info, err := os.Stat(a.path)
	if err != nil {
		return nil, err
	}

	if a.cached == nil || info.Size() != a.cachedSize || !info.ModTime().Equal(a.cachedMTime) {
		f, err := uaf.Load(a.path)
		if err != nil {
			return nil, err
		}

		a.cached, a.cachedSize, a.cachedMTime = f, info.Size(), info.ModTime()
	}

	return a.cached.Clone(), nil
}

// UpdateAuthorization calls change on the authorization file and keeps
// what it did if it returns nil: in the host file, under its lock
// (uaf.Update), or in memory. A change whose file fails uaf's checks is
// refused, the file left as it was.
func (s *System) UpdateAuthorization(change func(*uaf.File) error) error {
	a := &s.auth

	if a.path != "" {
		a.cached = nil

		return uaf.Update(a.path, change)
	}

	f, err := s.Authorization()
	if err != nil {
		return err
	}

	if err := change(f); err != nil {
		return err
	}

	if err := f.Check(); err != nil {
		return err
	}

	a.mem = f

	return nil
}
