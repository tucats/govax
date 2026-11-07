package lbr

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
)

// help90Dir is the Phase 90 probe, whose part A is LIBRARY/HELP on its own
// help files (testdata/help90/help90.com). Round 1b's save set holds the
// libraries VMS 7.1 made of them.
var help90Dir = filepath.Join("..", "..", "testdata", "help90")

// help90SaveSet reads round 1b's save set, its files by name ("H90A.HLB;1").
func help90SaveSet(t *testing.T) map[string][]byte {
	t.Helper()

	f, err := os.Open(filepath.Join(help90Dir, "vax", "round1b", "h90.bck"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	set, err := backup.Read(f, info.Size(), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	files := map[string][]byte{}
	for _, file := range set.Files {
		files[strings.TrimPrefix(file.Name, "[000000]")] = file.Data()
	}

	return files
}

// helpLines reads one of the probe's files as its records: a help file by
// its name alone ("h90a1"), another with its type.
func helpLines(t *testing.T, name string) [][]byte {
	t.Helper()

	if filepath.Ext(name) == "" {
		name += ".hlp"
	}

	data := readFixture(t, filepath.Join(help90Dir, name))

	var out [][]byte
	for _, line := range strings.SplitAfter(strings.TrimSuffix(string(data), "\n"), "\n") {
		out = append(out, []byte(strings.TrimSuffix(line, "\n")))
	}

	return out
}

// TestHelp90NewLibraries makes each help library one LIBRARY/CREATE/HELP
// command of part A made, and matches VMS's byte for byte, taking only the
// times and the librarian's name from VMS's: the modules, the key length,
// the index, an abandoned module's records (H90X2, H90X6), and a library
// left with nothing (H90X4).
func TestHelp90NewLibraries(t *testing.T) {
	cases := []struct {
		file    string
		keySize int
		inputs  []string
		problem HelpProblem // the last input's
	}{
		{"H90A.HLB;1", 0, []string{"h90a1", "h90a2"}, 0},
		{"H90R.HLB;1", 0, []string{"h90a1", "h90a2"}, 0},
		{"H90B.HLB;1", 0, []string{"h90b"}, 0},
		{"H90C.HLB;1", 0, []string{"h90c"}, 0},
		{"H90G.HLB;1", 0, []string{"h90g"}, 0},
		{"H90S.HLB;1", 0, []string{"h90s"}, 0},
		{"H90D.HLB;1", 0, []string{"h90d"}, 0},
		{"H90L.HLB;1", 80, []string{"h90a1"}, 0},
		{"H90X1.HLB;1", 0, []string{"badtext"}, 0},
		{"H90X2.HLB;1", 0, []string{"skip"}, HelpBadLevel},
		{"H90X3.HLB;1", 0, []string{"noblank"}, 0},
		{"H90X4.HLB;1", 0, []string{"toolong"}, HelpKeyTooLong},
		{"H90K.HLB;1", 40, []string{"toolong"}, 0},
		{"H90X5.HLB;1", 0, []string{"lower"}, 0},
		{"H90X6.HLB;1", 0, []string{"digits"}, HelpBadLevel},
		{"H90X7.HLB;1", 0, []string{"zero"}, 0},
	}

	files := help90SaveSet(t)

	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			data, ok := files[c.file]
			if !ok {
				t.Fatalf("the save set has no %s", c.file)
			}

			real, err := Open(data)
			if err != nil {
				t.Fatal(err)
			}

			b, err := CreateWith(TypeHelp, CreateOptions{KeySize: c.keySize})
			if err != nil {
				t.Fatal(err)
			}

			var (
				all     []*Entry
				partial *Entry
			)

			for i, in := range c.inputs {
				mods, bad := b.HelpModules(helpLines(t, in))

				switch {
				case i == len(c.inputs)-1 && c.problem != 0:
					if bad == nil || bad.Problem != c.problem {
						t.Fatalf("%s: problem %v, want %d", in, bad, c.problem)
					}

					partial = bad.Partial
				case bad != nil:
					t.Fatalf("%s: %v", in, bad)
				}

				all = append(all, mods...)
			}

			withRealTimes(t, b, real, all)

			for _, e := range all {
				if err := b.Insert(e); err != nil {
					t.Fatal(err)
				}
			}

			if partial != nil {
				partial.Inserted = real.Updated
				b.Abandon(partial)
			}

			requireSameLibrary(t, b.Bytes(), data)
		})
	}
}

// TestHelp90DataReduced reads the library LIBRARY/DATA=REDUCE/HELP made of
// H90A's modules: every module's records are H90A's.
func TestHelp90DataReduced(t *testing.T) {
	files := help90SaveSet(t)

	reduced, err := Open(files["H90R.HLB;2"])
	if err != nil {
		t.Fatal(err)
	}

	plain, err := Open(files["H90A.HLB;1"])
	if err != nil {
		t.Fatal(err)
	}

	if !reduced.DataReduced() || len(reduced.Indexes[0].Keys) != 16 {
		t.Fatalf("reduced %v, %d modules", reduced.DataReduced(), len(reduced.Indexes[0].Keys))
	}

	for _, k := range plain.Indexes[0].Keys {
		rfa, ok := reduced.Lookup(k.Name)
		if !ok {
			t.Fatalf("H90R has no %s", k.Name)
		}

		got, err := reduced.Module(rfa)
		if err != nil {
			t.Fatal(err)
		}

		want, err := plain.Module(k.RFA)
		if err != nil {
			t.Fatal(err)
		}

		if !sameRecords(got.Records, want.Records) {
			t.Errorf("%s: records differ", k.Name)
		}
	}
}

// TestHelpKeyLine checks the key line's form against the probe's.
func TestHelpKeyLine(t *testing.T) {
	cases := []struct {
		line  string
		level int
		key   string
	}{
		{"1 ALPHA", 1, "ALPHA"},
		{"2  Two blanks", 2, "Two"},
		{"10 apples is a line", 10, "apples"},
		{"2 /DIRECT", 2, "/DIRECT"},
		{"2Nospace", 0, ""},
		{"2", 0, ""},
		{"2\tTabbed", 0, ""},
		{"0 zero is a line", 0, ""},
		{"   1 indented", 0, ""},
		{"/BRIEF", 0, ""},
	}

	for _, c := range cases {
		level, key, ok := HelpKeyLine([]byte(c.line))
		if ok != (c.level > 0) || level != c.level || key != c.key {
			t.Errorf("%q: %d %q %v, want %d %q", c.line, level, key, ok, c.level, c.key)
		}
	}
}

// TestHelpModulesRecords checks what a module's records are: its lines as
// written, a line of blanks an empty record, the text before the first key
// dropped.
func TestHelpModulesRecords(t *testing.T) {
	b, err := Create(TypeHelp)
	if err != nil {
		t.Fatal(err)
	}

	lines := [][]byte{[]byte("before"), []byte("1 A"), []byte("   text"), []byte("   "), []byte("\ttab"), []byte("2 Sub"), []byte("1 b")}

	mods, bad := b.HelpModules(lines)
	if bad != nil || len(mods) != 2 {
		t.Fatalf("%d modules, %v", len(mods), bad)
	}

	want := [][]byte{[]byte("1 A"), []byte("   text"), {}, []byte("\ttab"), []byte("2 Sub")}
	if mods[0].Name != "A" || !sameRecords(mods[0].Records, want) {
		t.Errorf("A: %q", mods[0].Records)
	}

	if mods[1].Name != "b" || len(mods[1].Records) != 1 || !bytes.Equal(mods[1].Records[0], []byte("1 b")) {
		t.Errorf("b: %q %q", mods[1].Name, mods[1].Records)
	}

	// A help library's names are one module whatever their case.
	for _, e := range mods {
		if err := b.Insert(e); err != nil {
			t.Fatal(err)
		}
	}

	if err := b.Insert(&Entry{Name: "a"}); err == nil {
		t.Error("inserted a beside A")
	}

	if m, ok := b.Module("B"); !ok || m.Name != "b" {
		t.Errorf("Module(B): %v %v", m, ok)
	}
}

// TestHelpModulesBadLevel checks a key more than a level deeper, and a file
// whose first key isn't level 1.
func TestHelpModulesBadLevel(t *testing.T) {
	b, _ := Create(TypeHelp)

	mods, bad := b.HelpModules([][]byte{[]byte("1 GOOD"), []byte("1 BAD"), []byte("text"), []byte("3 THREE"), []byte("1 NEVER")})
	if len(mods) != 1 || mods[0].Name != "GOOD" || bad == nil || bad.Problem != HelpBadLevel ||
		bad.Level != 3 || bad.Key != "THREE" || bad.Line != 4 || bad.Partial == nil || bad.Partial.Name != "BAD" || len(bad.Partial.Records) != 2 {
		t.Fatalf("%v %+v", mods, bad)
	}

	if mods, bad = b.HelpModules([][]byte{[]byte("2 FIRST")}); len(mods) != 0 || bad == nil || bad.Partial != nil {
		t.Errorf("%v %+v", mods, bad)
	}
}

// TestHelp90OtherLibraries matches part A's other new libraries byte for
// byte: H90Q, the object library LIBRARY/CREATE made of a help file (no
// module goes in, RECTYP), and the text and macro libraries the LBR$
// routines read.
func TestHelp90OtherLibraries(t *testing.T) {
	files := help90SaveSet(t)

	cases := []struct {
		file string
		typ  Type
		mods func(b *Builder) []*Entry
	}{
		{"H90Q.HLB;1", TypeObject, func(*Builder) []*Entry { return nil }},
		{"H90T.TLB;1", TypeText, func(*Builder) []*Entry {
			return []*Entry{TextModule("T1", helpLines(t, "t1.txt")), TextModule("T2", helpLines(t, "t2.txt"))}
		}},
		{"H90M.MLB;1", TypeMacro, func(b *Builder) []*Entry {
			var lines []string
			for _, r := range helpLines(t, "h90m.mar") {
				lines = append(lines, string(r))
			}

			mods, warns, err := b.MacroModules(lines, true)
			if err != nil || len(warns) > 0 {
				t.Fatalf("MacroModules: %v %v", warns, err)
			}

			return mods
		}},
	}

	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			data := files[c.file]

			real, err := Open(data)
			if err != nil {
				t.Fatal(err)
			}

			b, err := Create(c.typ)
			if err != nil {
				t.Fatal(err)
			}

			mods := c.mods(b)
			withRealTimes(t, b, real, mods)

			for _, e := range mods {
				if err := b.Insert(e); err != nil {
					t.Fatal(err)
				}
			}

			requireSameLibrary(t, b.Bytes(), data)
		})
	}
}
