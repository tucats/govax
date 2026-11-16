package machine

import "github.com/tucats/govax/internal/corevms"

// LogIn logs process 1 in as username (docs/planning/PHASE-95 -
// authentication.md, Decision 7): LOGINOUT's look at the authorization
// file (corevms's BeginLogin, which refuses a missing or disabled
// account and records the login's time), then the process made that
// user's (Environment.LogIn). The machine keeps the login, and each new
// process 1 a VMINIT makes is the same user. The Login is returned for
// DCL's side (the home directory) and the banner.
func (m *Machine) LogIn(username string, interactive bool) (*corevms.Login, error) {
	l, err := m.processOne.System.BeginLogin(username, interactive)
	if err != nil {
		return nil, err
	}

	m.processOne.LogIn(l)
	m.login = l

	return l, nil
}
