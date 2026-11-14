package security

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The probe's [PROT93.F] matrix (testdata/prot93, Phase 93's subtask 2):
// 26 files, each a different combination of protection, owner, and ACL,
// which every identity tried to open every way. TestProbeMatrix holds
// Check to every one of VMS 7.1's answers.

// The probe's identifiers.
var (
	uicSystem  = NewUIC(1, 4)
	uicA       = NewUIC(0o200, 1)
	uicB       = NewUIC(0o200, 2)
	uicC       = NewUIC(0o300, 1)
	idTeam     = uint32(0x80010001) // PROT93_TEAM, held by PROT93C
	idInteract = uint32(0x80000003) // INTERACTIVE
	idLocal    = uint32(0x80000004) // LOCAL
)

// probeFile is one of [PROT93.F]'s files.
type probeFile struct {
	name  string
	owner UIC
	prot  string
	acl   []ACE
}

// identifier ACEs for the table, by the identifier's value.
func grant(id uint32, a Access) []ACE { return []ACE{NewIdentifierACE([]uint32{id}, a, 0)} }

// probeFiles are testdata/prot93/gen.go's fFiles but FACLCTL, whose
// protection the probe changed between runs (it is a case of its own in
// check_test.go).
var probeFiles = []probeFile{
	{"FNONE", uicA, "(S,O,G,W)", nil},
	{"FALL", uicA, "(S:RWED,O:RWED,G:RWED,W:RWED)", nil},
	{"FR", uicA, "(S:R,O:R,G:R,W:R)", nil},
	{"FW", uicA, "(S:W,O:W,G:W,W:W)", nil},
	{"FRW", uicA, "(S:RW,O:RW,G:RW,W:RW)", nil},
	{"FE", uicA, "(S:E,O:E,G:E,W:E)", nil},
	{"FD", uicA, "(S:D,O:D,G:D,W:D)", nil},
	{"FROT1", uicA, "(S:R,O:W,G:E,W:D)", nil},
	{"FROT2", uicA, "(S:W,O:E,G:D,W:R)", nil},
	{"FROT3", uicA, "(S:E,O:D,G:R,W:W)", nil},
	{"FROT4", uicA, "(S:D,O:R,G:W,W:E)", nil},
	{"FOWNER", uicA, "(S,O:RWED,G,W)", nil},
	{"FGROUP", uicA, "(S,O,G:RWED,W)", nil},
	{"FWORLD", uicA, "(S,O,G,W:RWED)", nil},
	{"FSYS", uicA, "(S:RWED,O,G,W)", nil},
	{"FUIC0", 0, "(S,O,G,W)", nil},
	{"FUIC0ACL", 0, "(S,O,G,W)", grant(uint32(uicB), Read)},
	{"FACLDENY", uicA, "(S:RWED,O:RWED,G:RWED,W:RWED)", grant(uint32(uicB), 0)},
	{"FACLGRANT", uicA, "(S,O,G,W)", grant(uint32(uicC), Read|Write)},
	{"FACLTEAM", uicA, "(S,O,G,W)", grant(idTeam, Read)},
	{"FACLOWN", uicA, "(S:RWED,O:RWED,G,W)", grant(uint32(uicA), 0)},
	{"FACLWILD", uicA, "(S,O,G,W)", grant(uint32(NewUIC(0o200, WildMember)), Read)},
	{"FACLSYS", uicA, "(S:RWED,O,G,W)", grant(uint32(uicSystem), 0)},
	{"FACLINT", uicA, "(S,O,G,W)", grant(idInteract, Read)},
	{"FACLTWO", uicA, "(S,O,G,W)", append(grant(uint32(uicC), 0), grant(idTeam, Read)...)},
}

// parseCode reads a protection code in DCL's form, "(S:RW,O,G:E,W)"; a
// category not named gives nothing, as in every code the probe used.
func parseCode(t *testing.T, text string) Protection {
	t.Helper()

	p := NoAccess

	for _, field := range strings.Split(strings.Trim(text, "()"), ",") {
		cat, letters, _ := strings.Cut(field, ":")

		c := map[string]Category{"S": System, "O": Owner, "G": Group, "W": World}[cat]

		var a Access

		for _, l := range letters {
			a |= map[rune]Access{'R': Read, 'W': Write, 'E': Execute, 'D': Delete}[l]
		}

		p = p.With(c, a)
	}

	return p
}

// probeIdentity makes a pass's credentials from its tag: the account's
// letter and the privilege pass (0 none, 1 READALL, 2 GRPPRV, 3 SYSPRV,
// 4 BYPASS; SYSTEM's M4 all four).
func probeIdentity(tag string) Credentials {
	c := Credentials{Rights: []Identifier{{Value: idInteract}, {Value: idLocal}}}

	switch tag[0] {
	case 'M':
		c.UIC = uicSystem
	case 'A':
		c.UIC = uicA
	case 'B':
		c.UIC = uicB
	case 'C':
		c.UIC = uicC
		c.Rights = append(c.Rights, Identifier{Value: idTeam})
	}

	switch {
	case tag == "M4":
		c.Privileges = PrivREADALL | PrivGRPPRV | PrivSYSPRV | PrivBYPASS
	case tag[1] == '1':
		c.Privileges = PrivREADALL
	case tag[1] == '2':
		c.Privileges = PrivGRPPRV
	case tag[1] == '3':
		c.Privileges = PrivSYSPRV
	case tag[1] == '4':
		c.Privileges = PrivBYPASS
	}

	return c
}

// probeAnswer is one case of the log: an identity, an operation on a
// file, and whether VMS allowed it.
type probeAnswer struct {
	tag, op, file string
	ok            bool
}

// readProbeLog finds every [PROT93.F] case in a log of the probe's
// program, with the tag of the pass it ran in.
func readProbeLog(t *testing.T, path string) []probeAnswer {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	const prefix = "PROT93$DISK:[PROT93.F]"

	var (
		answers []probeAnswer
		tag     string
		pending *probeAnswer
	)

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()

		switch {
		case strings.HasPrefix(line, "tag "):
			tag = strings.TrimSpace(line[4:])
		case strings.HasPrefix(line, "== "):
			pending = nil

			op, rest, _ := strings.Cut(line[3:], " ")
			if !strings.HasPrefix(rest, prefix) {
				continue
			}

			file, flags, _ := strings.Cut(strings.TrimPrefix(rest, prefix), "|")
			pending = &probeAnswer{tag: tag, op: op + "|" + flags, file: strings.TrimSuffix(file, ".DAT")}
		case pending != nil && strings.Contains(line, "$OPEN sts="):
			pending.ok = strings.Contains(line, "sts=00010001")
			answers = append(answers, *pending)
			pending = nil
		case pending != nil && strings.Contains(line, "$QIOW "):
			pending.ok = strings.HasSuffix(line, "iosb=0001")
			answers = append(answers, *pending)
			pending = nil
		}
	}

	return answers
}

// allowed is what each of the probe's operations needs, by Check.
func allowed(c Credentials, p Profile, op string) bool {
	switch op {
	case "O-|G", "O-|B", "A-|", "QR|":
		// A read, block I/O for read, $DISPLAY after an open for read,
		// and the ACP's access for read.
		return Check(c, p, Read).Granted
	case "O-|P", "O-|GP", "QW|":
		// Writing needs read and write (the guide, 4.8.4.3).
		return Check(c, p, Read|Write).Granted
	case "QA|":
		// The ACP's read of the attributes: read or control (A.4).
		return Check(c, p, Read).Granted || Check(c, p, Control).Granted
	}

	panic("no operation " + op)
}

func TestProbeMatrix(t *testing.T) {
	vax := filepath.Join("..", "..", "testdata", "prot93", "vax")
	logs := []string{
		"prot93.log", "prot93u_prot93b.log", "prot93u_prot93c.log",
		filepath.Join("round2", "prot93v_prot93a.log"),
		filepath.Join("round2", "prot93v_prot93b.log"),
		filepath.Join("round2", "prot93v_prot93c.log"),
	}

	files := map[string]Profile{}
	for _, f := range probeFiles {
		files[f.name] = Profile{Owner: f.owner, Protection: parseCode(t, f.prot), ACL: f.acl}
	}

	tags := map[string]bool{}
	checked := 0

	for _, name := range logs {
		for _, a := range readProbeLog(t, filepath.Join(vax, name)) {
			p, ok := files[a.file]
			if !ok {
				continue
			}

			tags[a.tag] = true
			checked++

			if got := allowed(probeIdentity(a.tag), p, a.op); got != a.ok {
				t.Errorf("%s %s %s: allowed %v, VMS %v", a.tag, a.op, a.file, got, a.ok)
			}
		}
	}

	// Every identity but M4, whose matrix wasn't run, and A1 to A4 (the
	// owner has no privileges to turn on).
	for _, tag := range []string{"M0", "M1", "A0", "B0", "B1", "B2", "B3", "B4", "C0", "C1", "C2", "C3", "C4"} {
		if !tags[tag] {
			t.Errorf("no cases for %s", tag)
		}
	}

	t.Logf("%d cases on %d files under %d identities", checked, len(files), len(tags))
}
