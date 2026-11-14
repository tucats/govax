package security

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// probeACLs reads the ACLs $DISPLAY's XABPRO returned in the probe's log
// (testdata/prot93/vax/prot93.log's "== A-" cases): each file's name
// (the part after the last "]") and its ACL's bytes.
func probeACLs(t *testing.T) map[string][]byte {
	t.Helper()

	f, err := os.Open(filepath.Join("..", "..", "testdata", "prot93", "vax", "prot93.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	acls := map[string][]byte{}

	var (
		name   string
		length int
	)

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()

		switch {
		case strings.HasPrefix(line, "== A- "):
			name = line[strings.LastIndex(line, "]")+1:]
			length = 0
		case name != "" && strings.Contains(line, " acllen="):
			field := strings.Fields(line[strings.Index(line, "acllen="):])[0]
			length, _ = strconv.Atoi(strings.TrimPrefix(field, "acllen="))
		case name != "" && strings.HasPrefix(line, "  acl "):
			b, err := hex.DecodeString(strings.ReplaceAll(line[6:], " ", ""))
			if err != nil {
				t.Fatalf("%s: %v", line, err)
			}

			acls[name] = append(acls[name], b...)
			if len(acls[name]) >= length {
				acls[name] = acls[name][:length]
				name = ""
			}
		case strings.HasPrefix(line, "== "):
			name = ""
		}
	}

	return acls
}

func TestProbeACEs(t *testing.T) {
	acls := probeACLs(t)

	// The ACEs this package can make, as VMS made them from the probe's
	// SET SECURITY/ACL text (testdata/prot93/gen.go's aceFiles).
	a := uint32(uicA)
	made := map[string][]ACE{
		"ACE01.DAT": {NewIdentifierACE([]uint32{a}, Read, 0)},
		"ACE02.DAT": {NewIdentifierACE([]uint32{a}, All, 0)},
		"ACE03.DAT": {NewIdentifierACE([]uint32{a}, Read, 0)},
		"ACE04.DAT": {NewIdentifierACE([]uint32{uint32(NewUIC(0o200, WildMember))}, Read, 0)},
		"ACE05.DAT": {NewIdentifierACE([]uint32{uint32(NewUIC(WildGroup, WildMember))}, Read, 0)},
		"ACE06.DAT": {NewIdentifierACE([]uint32{idTeam}, 0, 0)},
		"ACE07.DAT": {NewIdentifierACE([]uint32{a, idTeam}, Read, 0)},
		"ACE08.DAT": {NewIdentifierACE([]uint32{idInteract}, Read, 0)},
		"ACE09.DAT": {NewIdentifierACE([]uint32{a}, Read, OptionProtected)},
		"ACE10.DAT": {NewIdentifierACE([]uint32{a}, Read, OptionNoPropagate)},
		"ACE12.DAT": {NewIdentifierACE([]uint32{a}, Read, OptionDefault)},
		"ACE13.DAT": {NewIdentifierACE([]uint32{a}, Read, OptionProtected|OptionNoPropagate)},
		"ACE14.DIR": {NewDefaultProtectionACE(NoAccess.With(System, All).With(Owner, All).With(Group, Read|Execute), 0)},
		"ACE15.DIR": {NewDefaultProtectionACE(NoAccess.With(System, All).With(Owner, All), OptionProtected)},
		"ACE20.DAT": {NewIdentifierACE([]uint32{a}, Read, 0), NewIdentifierACE([]uint32{uint32(uicB)}, Write, 0)},
		"ACE21.DIR": {NewIdentifierACE([]uint32{a}, Read, OptionDefault)},
		"DP.DIR": {
			NewDefaultProtectionACE(NoAccess.With(System, All).With(Owner, All).With(Group, Read).With(World, Execute), 0),
			NewIdentifierACE([]uint32{idTeam}, Read, OptionDefault),
			NewIdentifierACE([]uint32{uint32(uicB)}, Read|Write, OptionDefault|OptionNoPropagate),
			NewIdentifierACE([]uint32{a}, Read, 0),
		},
	}

	for name, want := range made {
		raw, ok := acls[name]
		if !ok {
			t.Errorf("%s: not in the log", name)

			continue
		}

		if got := EncodeACL(want); !bytes.Equal(got, raw) {
			t.Errorf("%s: made % X, VMS % X", name, got, raw)
		}

		decoded, err := DecodeACL(raw)
		if err != nil {
			t.Errorf("%s: %v", name, err)

			continue
		}

		if len(decoded) != len(want) {
			t.Errorf("%s: %d ACEs, want %d", name, len(decoded), len(want))

			continue
		}

		for i, d := range decoded {
			w := want[i]
			if d.Type != w.Type || d.Flags != w.Flags || d.Access != w.Access || !equalIDs(d.Identifiers, w.Identifiers) {
				t.Errorf("%s ACE %d: decoded %+v, want %+v", name, i, d, w)
			}
		}
	}

	// The default protection ACE reads back as its code.
	if d, _ := DecodeACL(acls["DP.DIR"]); d[0].DefaultProtection() != NoAccess.With(System, All).With(Owner, All).With(Group, Read).With(World, Execute) {
		t.Errorf("DP.DIR's default protection: %#04x", uint16(d[0].DefaultProtection()))
	}

	// The ACEs this package only carries: a creator ACE, an alarm, and
	// an audit, each decoded with its header and kept whole.
	for _, c := range []struct {
		name   string
		typ    byte
		flags  uint16
		access Access
	}{
		{"ACE16.DIR", TypeCreator, 0, Read | Write},
		{"ACE17.DAT", TypeAlarm, 3, Read | Write},
		{"ACE18.DAT", TypeAudit, 2, Delete},
	} {
		d, err := DecodeACL(acls[c.name])
		if err != nil || len(d) != 1 || d[0].Type != c.typ || d[0].Flags != c.flags || d[0].Access != c.access {
			t.Errorf("%s: %+v, %v", c.name, d, err)

			continue
		}

		if !bytes.Equal(EncodeACL(d), acls[c.name]) {
			t.Errorf("%s: not kept whole", c.name)
		}
	}
}

// equalIDs compares two lists of identifiers.
func equalIDs(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestDecodeACLErrors(t *testing.T) {
	if _, err := DecodeACL([]byte{0x0C, 1, 0, 0, 1, 0, 0, 0}); err == nil {
		t.Error("an ACE past the end decoded")
	}

	if _, err := DecodeACL([]byte{2, 1}); err == nil {
		t.Error("an ACE too short decoded")
	}

	// A length of 0 ends the list: a header's unused ACL area.
	acl, err := DecodeACL(append(EncodeACL(grant(idTeam, Read)), 0, 0, 0, 0))
	if err != nil || len(acl) != 1 {
		t.Errorf("trailing zeros: %v, %v", acl, err)
	}
}
