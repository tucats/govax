package uaf

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestDefault checks a new file's accounts: SYSTEM with process 1's
// values, DEFAULT with VMS 7.1's, and SYSTEM's UIC identifier.
func TestDefault(t *testing.T) {
	f := Default()
	if err := f.Check(); err != nil {
		t.Fatal(err)
	}

	sys, ok := f.Find("system  ")
	if !ok {
		t.Fatal("no SYSTEM")
	}

	if sys.UIC != NewUIC(1, 4) || sys.Privileges != AllPrivileges || sys.Priority != 4 ||
		sys.Quotas.WSEXTENT != 16400 || sys.Quotas.PGFLQUOTA != 40960 || sys.Device != "SYS$SYSROOT:" || sys.Directory != "[SYSMGR]" {
		t.Errorf("SYSTEM = %+v", *sys)
	}

	def, ok := f.Find("DEFAULT")
	if !ok {
		t.Fatal("no DEFAULT")
	}

	if def.Flags != FlagDISUSER || def.UIC.String() != "[200,200]" || !slices.Equal(def.Privileges.Names(), []string{"TMPMBX", "NETMBX"}) {
		t.Errorf("DEFAULT = %+v, privileges %v", *def, def.Privileges.Names())
	}

	if id, ok := f.IdentifierNamed("system"); !ok || id.Value != IdentifierValue(NewUIC(1, 4)) {
		t.Errorf("SYSTEM's identifier = %+v, %v", id, ok)
	}

	if f.NextGeneral() != FirstGeneral {
		t.Errorf("NextGeneral = %s", f.NextGeneral())
	}
}

// TestRoundTrip writes a file with every kind of field and reads it back
// the same, and checks the text uses names.
func TestRoundTrip(t *testing.T) {
	f := Default()
	team := Identifier{Name: "PROJECTX", Value: f.NextGeneral(), Attributes: mustAttr(t, "RESOURCE")}
	f.Identifiers = append(f.Identifiers, team)

	r := DefaultRecord()
	r.Username, r.Owner, r.Account = "SMITH", "JOE SMITH", "GOVAX"
	r.UIC = NewUIC(0o300, 0o10)
	r.Flags = 0
	r.LastInteractive = time.Date(2026, 11, 15, 10, 30, 0, 0, time.UTC)
	r.Identifiers = []Held{{Name: "PROJECTX", Attributes: mustAttr(t, "RESOURCE")}, {Name: "%X80020001"}}
	f.Put(r)

	data, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	text := string(data)
	for _, want := range []string{`"uic": "[300,10]"`, `"ALL"`, `"value": "%X80010000"`, `"RESOURCE"`, `"DISUSER"`, `"TMPMBX"`, `"version": 1`} {
		if !strings.Contains(text, want) {
			t.Errorf("file lacks %s:\n%s", want, text)
		}
	}

	if strings.Contains(text, "password_changed") {
		t.Errorf("a zero time was written:\n%s", text)
	}

	back, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v\n%s", err, text)
	}

	again, _ := back.Marshal()
	if string(again) != text {
		t.Errorf("round trip changed the file:\n%s\n---\n%s", text, again)
	}

	smith, _ := back.Find("SMITH")
	if v, ok := back.HeldValue(smith.Identifiers[1]); !ok || v != 0x80020001 {
		t.Errorf("held by value = %s, %v", v, ok)
	}
}

func mustAttr(t *testing.T, name string) Attributes {
	t.Helper()

	a, ok := AttributeMask(name)
	if !ok {
		t.Fatalf("no attribute %s", name)
	}

	return a
}

// TestParseErrors checks that each kind of mistake is refused with the
// line it's on.
func TestParseErrors(t *testing.T) {
	good, _ := Default().Marshal()
	text := string(good)

	cases := []struct {
		name, from, to, want string
	}{
		{"syntax", `"version": 1,`, `"version": 1`, "invalid character"},
		{"unknown field", `"astlm": 50`, `"astlmx": 50`, "unknown field"},
		{"octal UIC", `"uic": "[200,200]"`, `"uic": "[200,8]"`, "not a UIC"},
		{"big group", `"uic": "[200,200]"`, `"uic": "[400000,1]"`, "not a UIC"},
		{"privilege", `"TMPMBX"`, `"NOSUCHPRIV"`, "no privilege"},
		{"flag", `"DISUSER"`, `"NOSUCHFLAG"`, "no flag"},
		{"type", `"priority": 4,`, `"priority": "four",`, "cannot unmarshal"},
		{"username", `"username": "DEFAULT"`, `"username": "BAD-NAME"`, "can't be a username"},
		{"twice", `"username": "DEFAULT"`, `"username": "SYSTEM"`, "twice"},
		{"version", `"version": 1`, `"version": 2`, "version 2"},
		{"held", `"identifiers": []`, `"identifiers": [{"name": "NOSUCHID"}]`, "no identifier"},
	}

	for _, c := range cases {
		if !strings.Contains(text, c.from) {
			t.Fatalf("%s: the default file has no %s", c.name, c.from)
		}

		bad := strings.Replace(text, c.from, c.to, 1)
		line := strings.Count(bad[:strings.Index(bad, c.to)], "\n") + 1

		path := filepath.Join(t.TempDir(), "sysuaf.json")
		if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := Load(path)

		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("%s: Load = %v, want a ParseError", c.name, err)

			continue
		}

		if !strings.Contains(pe.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, pe, c.want)
		}

		// A syntax error is found where the parser gave up, the line
		// after the missing comma.
		if c.name != "syntax" && pe.Line != line {
			t.Errorf("%s: line %d, want %d (%v)", c.name, pe.Line, line, pe)
		}
	}
}

// TestSaveAndUpdate checks that Save writes a 0600 file and leaves no
// temporary behind, that Update changes it, that Ensure makes a file
// once, and that concurrent Updates lose nothing.
func TestSaveAndUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "sysuaf.json")

	if _, err := Load(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load of a missing file = %v", err)
	}

	made, err := Ensure(path)
	if err != nil || !made {
		t.Fatalf("Ensure = %v, %v", made, err)
	}

	if made, err := Ensure(path); err != nil || made {
		t.Fatalf("second Ensure = %v, %v", made, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode().Perm())
	}

	var wg sync.WaitGroup

	for i := range 8 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			err := Update(path, func(f *File) error {
				r := DefaultRecord()
				r.Username = "USER" + string(rune('A'+i))
				f.Put(r)

				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}

	wg.Wait()

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(f.Accounts) != 10 {
		t.Errorf("%d accounts after 8 concurrent adds, want 10", len(f.Accounts))
	}

	// A change that fails writes nothing.
	if err := Update(path, func(f *File) error { f.Remove("SYSTEM"); return errors.New("no") }); err == nil {
		t.Error("Update ignored change's error")
	}

	if f, _ := Load(path); !hasAccount(f, "SYSTEM") {
		t.Error("a failed Update was written")
	}

	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if e.Name() != "sysuaf.json" && e.Name() != "sysuaf.json.lock" {
			t.Errorf("left behind: %s", e.Name())
		}
	}

	// An invalid file can't be saved.
	f.Accounts[0].Username = "bad name"
	if err := f.Save(path); err == nil {
		t.Error("Save wrote an invalid file")
	}
}

func hasAccount(f *File, name string) bool {
	_, ok := f.Find(name)

	return ok
}

// TestPasswords checks the hash's form, that it checks the password it
// was made from in any case, and refuses others and bad hashes.
func TestPasswords(t *testing.T) {
	h, err := HashPassword("Secret1")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Errorf("hash %s", h)
	}

	if h2, _ := HashPassword("Secret1"); h2 == h {
		t.Error("two hashes of one password are the same: no salt")
	}

	for pw, want := range map[string]bool{"Secret1": true, "SECRET1": true, " secret1 ": true, "secret2": false, "": false} {
		if got, err := CheckPassword(h, pw); err != nil || got != want {
			t.Errorf("CheckPassword(%q) = %v, %v; want %v", pw, got, err, want)
		}
	}

	if ok, err := CheckPassword("", "x"); ok || err != nil {
		t.Errorf("no password matched: %v, %v", ok, err)
	}

	for _, bad := range []string{"$argon2i$v=19$m=1,t=1,p=1$AA$AA", "$argon2id$v=19$m=x$AA$AA", "plain"} {
		if _, err := CheckPassword(bad, "x"); !errors.Is(err, ErrBadHash) {
			t.Errorf("CheckPassword(%q) = %v", bad, err)
		}
	}

	// A hash made with other parameters still checks: they're in it.
	r := SystemRecord()
	r.Password = h
	if err := r.Check(); err != nil {
		t.Error(err)
	}
}

// TestNames checks the names' forms both ways.
func TestNames(t *testing.T) {
	if u, err := ParseUIC(" [ 37776 , 177776 ] "); err != nil || u.Group() != MaxGroup || u.Member() != MaxMember {
		t.Errorf("ParseUIC = %v, %v", u, err)
	}

	for _, bad := range []string{"[300]", "300,1", "[37777,1]", "[1,177777]", "[a,1]"} {
		if _, err := ParseUIC(bad); err == nil {
			t.Errorf("ParseUIC(%q) succeeded", bad)
		}
	}

	p, err := ParsePrivileges([]string{"detach", "SYSPRV", "noacnt"})
	if err != nil {
		t.Fatal(err)
	}

	if got := p.Names(); !slices.Equal(got, []string{"IMPERSONATE", "ACNT", "SYSPRV"}) {
		t.Errorf("names %v", got)
	}

	if v, err := ParseIdentifierValue("%x8001000a"); err != nil || v != 0x8001000A || v.String() != "%X8001000A" {
		t.Errorf("identifier value %v, %v", v, err)
	}

	for _, name := range []string{"1PROJ", "A_B$1"} {
		if !ValidIdentifierName(name) {
			t.Errorf("%s refused", name)
		}
	}

	for _, name := range []string{"123", "", "a", strings.Repeat("A", 32)} {
		if ValidIdentifierName(name) {
			t.Errorf("%q accepted", name)
		}
	}

	if !ValidUsername("TOOLONGUSERN") || ValidUsername("TOOLONGUSERNA") {
		t.Error("username length")
	}
}
