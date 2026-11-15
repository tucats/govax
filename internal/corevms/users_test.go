package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/security"
)

// TestUsers: the authorization file's accounts, found in any case and
// with trailing blanks, SYSTEM always there and replaceable, a bad name
// refused, and an account's credentials its authorized privileges and
// granted identifiers.
func TestUsers(t *testing.T) {
	s := &System{}

	if u, ok := s.User("system"); !ok || u.UIC != uint32(security.SystemUIC) || u.Privileges != security.AllPrivileges {
		t.Errorf("SYSTEM: %+v %v", u, ok)
	}

	team := security.Identifier{Value: 0x80010001}

	if status := s.AddUser(UserAccount{Name: "prot93c", UIC: 0x00C00001, Privileges: security.PrivSYSPRV, Rights: []security.Identifier{team}}); status != ssNormal {
		t.Fatalf("AddUser: %#x", status)
	}

	u, ok := s.User("PROT93C   ")
	if !ok || u.Name != "PROT93C" {
		t.Fatalf("PROT93C: %+v %v", u, ok)
	}

	c := u.AccountCredentials(0)
	if c.UIC != security.NewUIC(0o300, 1) || !c.Has(security.PrivSYSPRV) || !c.Holds(team.Value) || c.Holds(0x80000003) {
		t.Errorf("credentials %+v", c)
	}

	for _, name := range []string{"", "NOSUCH"} {
		if _, ok := s.User(name); ok {
			t.Errorf("%q found", name)
		}
	}

	if status := s.AddUser(UserAccount{Name: "A NAME"}); status == ssNormal {
		t.Error("a name with a blank was taken")
	}

	s.AddUser(UserAccount{Name: "SYSTEM", UIC: 0x00010004})

	if u, _ := s.User("SYSTEM"); u.Privileges != 0 {
		t.Errorf("SYSTEM not replaced: %+v", u)
	}
}
