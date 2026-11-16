package corevms

import (
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmsdef"
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

	if status := s.AddUser(UserAccount{Name: "prot93c", UIC: 0x00C00001, Rights: []security.Identifier{team}}); status != ssNoSuchID {
		t.Errorf("a right no identifier has: %#x", status)
	}

	if status := s.AddIdentifier("PROT93_TEAM", team.Value, 0); status != ssNormal {
		t.Fatalf("AddIdentifier: %#x", status)
	}

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

// TestAuthorizationFile: with a host file, AddUser and AddIdentifier
// write it, another System reads what they wrote, a change made to the
// file outside is seen, and an account gets a UIC identifier unless its
// UIC or name has one.
func TestAuthorizationFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sysuaf.json")
	if _, err := uaf.Ensure(path); err != nil {
		t.Fatal(err)
	}

	s := &System{}
	s.SetAuthorizationFile(path)

	if status := s.AddIdentifier("TEAM", 0x80010005, 0); status != ssNormal {
		t.Fatalf("AddIdentifier: %#x", status)
	}

	if status := s.AddIdentifier("team", 0x80010006, 0); status != vmsdef.Symbols["SS$_DUPIDENT"] {
		t.Errorf("a second TEAM: %#x", status)
	}

	if status := s.AddUser(UserAccount{Name: "smith", UIC: 0x00C00002, Rights: []security.Identifier{{Value: 0x80010005}}}); status != ssNormal {
		t.Fatalf("AddUser: %#x", status)
	}

	other := &System{}
	other.SetAuthorizationFile(path)

	u, ok := other.User("SMITH")
	if !ok || u.UIC != 0x00C00002 || len(u.Rights) != 1 || u.Rights[0].Value != 0x80010005 {
		t.Fatalf("SMITH from the file: %+v %v", u, ok)
	}

	f, err := uaf.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	smith, _ := f.Find("SMITH")
	if held := f.HeldBy(smith.UIC); len(held) != 1 || held[0].Identifier.Name != "TEAM" || smith.Flags&uaf.FlagDISUSER != 0 || smith.Quotas.PRCLM != 2 {
		t.Errorf("SMITH's record %+v", smith)
	}

	if id, ok := f.IdentifierNamed("SMITH"); !ok || id.Value != 0x00C00002 {
		t.Errorf("SMITH's UIC identifier %+v %v", id, ok)
	}

	// A change by someone else (another session's AUTHORIZE) is seen.
	if err := uaf.Update(path, func(f *uaf.File) error { f.Remove("SMITH"); return nil }); err != nil {
		t.Fatal(err)
	}

	if _, ok := s.User("SMITH"); ok {
		t.Error("SMITH still found after the file lost it")
	}

	// SYSTEM is found even when the file has lost it.
	if err := uaf.Update(path, func(f *uaf.File) error { f.Remove("SYSTEM"); return nil }); err != nil {
		t.Fatal(err)
	}

	if u, ok := s.User("SYSTEM"); !ok || u.Privileges != security.AllPrivileges {
		t.Errorf("SYSTEM without its record: %+v %v", u, ok)
	}
}
