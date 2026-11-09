package sysvol

import (
	"sort"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lbr"
)

// helpLibrary builds a help library of modules, each a module's text
// ("1 A\nText of A."), as LIBRARY/CREATE/HELP would.
func helpLibrary(t *testing.T, modules ...string) []byte {
	t.Helper()

	b, err := lbr.Create(lbr.TypeHelp)
	if err != nil {
		t.Fatal(err)
	}

	for _, text := range modules {
		var lines [][]byte
		for _, l := range strings.Split(text, "\n") {
			lines = append(lines, []byte(l))
		}

		entries, bad := b.HelpModules(lines)
		if bad != nil {
			t.Fatal(bad)
		}

		for _, e := range entries {
			if _, err := b.Replace(e); err != nil {
				t.Fatal(err)
			}
		}
	}

	return b.Bytes()
}

// libraryOn returns the modules of the volume's highest version of the
// help library dir+name, each as "NAME: first line of text".
func (r *refreshTest) libraryOn(dir, name string) string {
	r.t.Helper()

	records, found, _ := r.v.ReadFile(dir + name)
	if !found {
		return "(none)"
	}

	_, modules, _, err := libraryModules(Join(records, Binary))
	if err != nil {
		r.t.Fatal(err)
	}

	var out []string

	for key, e := range modules {
		text := ""
		if len(e.Records) > 1 {
			text = string(e.Records[1])
		}

		out = append(out, key+":"+text)
	}

	sort.Strings(out)

	return strings.Join(out, " ")
}

// TestRefreshHelpLibrary walks a help library through the refresh's
// cases by module (docs/PHASE-90 - VMS help.md, Decision 7), a refresh
// after each change.
func TestRefreshHelpLibrary(t *testing.T) {
	r := newRefreshTest(t)

	govax := func(modules ...string) {
		r.copies["H.HLB"] = string(helpLibrary(t, modules...))
	}

	files := func() []File {
		read := func() ([]byte, error) { return []byte(r.copies["H.HLB"]), nil }

		return []File{{Directory: SysHlp, Name: "H.HLB", Format: Binary, HelpLibrary: true, read: read}}
	}

	expect := func(want string) {
		t.Helper()

		report, err := refreshFiles(r.v, files())
		if err != nil {
			t.Fatal(err)
		}

		c := report.Changes[0]
		got := c.Action.String()

		if c.Notify {
			got += "!"
		}

		if len(c.KeptModules) > 0 {
			got += "[" + strings.Join(c.KeptModules, ",") + "]"
		}

		if got != want {
			t.Errorf("refresh: %s, want %s", got, want)
		}
	}

	user := func(modules ...string) {
		records, _, _ := r.v.ReadFile(SysHlp + "H.HLB")

		l, err := lbr.Open(Join(records, Binary))
		if err != nil {
			t.Fatal(err)
		}

		b, err := lbr.Edit(l)
		if err != nil {
			t.Fatal(err)
		}

		for _, text := range modules {
			if name, ok := strings.CutPrefix(text, "-"); ok {
				if err := b.Delete(name); err != nil {
					t.Fatal(err)
				}

				continue
			}

			var lines [][]byte
			for _, line := range strings.Split(text, "\n") {
				lines = append(lines, []byte(line))
			}

			entries, _ := b.HelpModules(lines)
			for _, e := range entries {
				if _, err := b.Replace(e); err != nil {
					t.Fatal(err)
				}
			}
		}

		_ = r.v.WriteFile(SysHlp+"H.HLB", Records(Canonical(b.Bytes(), Binary), Binary), Binary)
	}

	on := func(want string) {
		t.Helper()

		if got := r.libraryOn(SysHlp, "H.HLB"); got != want {
			t.Errorf("library: %s\n    want %s", got, want)
		}
	}

	// A new volume: the library written whole.
	govax("1 A\nA one.", "1 B\nB one.", "1 C\nC one.")
	expect("written")
	expect("current")

	// The user adds a module of their own and changes B; govax hasn't
	// changed: the library is theirs now, and nothing is said.
	user("1 U\nMine.", "1 B\nMy B.")
	expect("kept")

	// A newer govax changes A and B, drops C, and adds D: A is replaced, C
	// taken out, D inserted, and B, the user's, kept and said once; U stays.
	govax("1 A\nA two.", "1 B\nB two.", "1 D\nD one.")
	expect("updated[B]")
	on("A:A two. B:My B. D:D one. U:Mine.")
	expect("kept")

	// The user deletes D, govax's: it stays deleted. The user puts B back
	// as govax has it, which makes it govax's again.
	user("-D", "1 B\nB two.")
	expect("kept")
	on("A:A two. B:B two. U:Mine.")

	govax("1 A\nA two.", "1 B\nB three.", "1 D\nD one.")
	expect("updated")
	on("A:A two. B:B three. U:Mine.")

	// A file that isn't a library is the user's, whole, said once.
	_ = r.v.WriteFile(SysHlp+"H.HLB", Records(Canonical([]byte("not a library"), Binary), Binary), Binary)
	expect("kept!")
	expect("kept")
}

// TestReportUpdates checks Report.Updates, what the update log is told: a
// file written or updated whole by its specification, and a help library
// refreshed module by module as each module, LIBNAME(MODULE), never the
// library.
func TestReportUpdates(t *testing.T) {
	r := newRefreshTest(t)
	r.copies["H.HLB"] = string(helpLibrary(t, "1 A\nA one.", "1 C\nC one."))

	files := func() []File {
		read := func() ([]byte, error) { return []byte(r.copies["H.HLB"]), nil }

		return append(r.files(), File{Directory: SysHlp, Name: "H.HLB", Format: Binary, HelpLibrary: true, read: read})
	}

	updates := func() string {
		t.Helper()

		report, err := refreshFiles(r.v, files())
		if err != nil {
			t.Fatal(err)
		}

		var lines []string
		for _, u := range report.Updates() {
			lines = append(lines, u.Action+" "+u.Name)
		}

		return strings.Join(lines, "; ")
	}

	want := "written [SYSTEM.SYSMGR]A.COM; written [SYSTEM.SYSLIB]B.EXE; written [SYSTEM.SYSMGR]SITE.COM; " +
		"written [SYSTEM.SYSHLP]H.HLB"
	if got := updates(); got != want {
		t.Errorf("first refresh: %s\n    want %s", got, want)
	}

	if got := updates(); got != "" {
		t.Errorf("second refresh: %s, want nothing", got)
	}

	// A newer govax: B.EXE whole; in the library, A replaced, B inserted,
	// C taken out.
	r.copies["B.EXE"] = "image two"
	r.copies["H.HLB"] = string(helpLibrary(t, "1 A\nA two.", "1 B\nB one."))

	want = "updated [SYSTEM.SYSLIB]B.EXE; replaced [SYSTEM.SYSHLP]H.HLB(A); inserted [SYSTEM.SYSHLP]H.HLB(B); " +
		"removed [SYSTEM.SYSHLP]H.HLB(C)"
	if got := updates(); got != want {
		t.Errorf("newer govax: %s\n    want %s", got, want)
	}
}
