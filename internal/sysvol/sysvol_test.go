package sysvol

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
)

// mapVolume is a Volume in memory: each file's versions, the newest
// last, by specification in uppercase, and the directories made.
type mapVolume struct {
	dirs  []string
	files map[string][]mapFile
	fail  string // a specification WriteFile refuses, for the error paths
}

type mapFile struct {
	records [][]byte
	format  Format
}

func newMapVolume() *mapVolume {
	return &mapVolume{files: map[string][]mapFile{}}
}

func (v *mapVolume) CreateDirectory(dir string) error {
	// A directory's parent must exist, as on a volume: [SYSTEM.SYSLIB]
	// needs [SYSTEM].
	if i := strings.LastIndex(dir, "."); i >= 0 {
		parent := dir[:i] + "]"
		if !v.hasDir(parent) {
			return errors.New("no parent directory " + parent)
		}
	}

	if !v.hasDir(dir) {
		v.dirs = append(v.dirs, strings.ToUpper(dir))
	}

	return nil
}

func (v *mapVolume) hasDir(dir string) bool {
	for _, d := range v.dirs {
		if strings.EqualFold(d, dir) {
			return true
		}
	}

	return false
}

func (v *mapVolume) WriteFile(spec string, records [][]byte, f Format) error {
	if strings.EqualFold(spec, v.fail) {
		return errors.New("refused")
	}

	dir := spec[:strings.Index(spec, "]")+1]
	if !v.hasDir(dir) {
		return errors.New("no directory " + dir)
	}

	copied := make([][]byte, len(records))
	for i, r := range records {
		copied[i] = append([]byte(nil), r...)
	}

	k := strings.ToUpper(spec)
	v.files[k] = append(v.files[k], mapFile{records: copied, format: f})

	return nil
}

func (v *mapVolume) ReadFile(spec string) ([][]byte, bool, error) {
	versions := v.files[strings.ToUpper(spec)]
	if len(versions) == 0 {
		return nil, false, nil
	}

	return versions[len(versions)-1].records, true, nil
}

// TestFileTable checks the layout: every file has a directory on the
// volume, a name ODS-2 allows, a unique specification, and contents that
// read, in their format's canonical form.
func TestFileTable(t *testing.T) {
	// ODS-2's names: up to 39 characters, a period, up to 39 more, of
	// letters, digits, "$", "-", and "_".
	odsName := regexp.MustCompile(`^[A-Z0-9$_-]{1,39}\.[A-Z0-9$_-]{0,39}$`)

	seen := map[string]bool{}
	seeded := 0

	for _, f := range Files() {
		spec := f.Spec()

		if seen[spec] {
			t.Errorf("%s is in the table twice", spec)
		}

		seen[spec] = true

		found := false

		for _, d := range Directories {
			found = found || d == f.Directory
		}

		if !found {
			t.Errorf("%s: directory %s isn't one of Directories", spec, f.Directory)
		}

		if !odsName.MatchString(f.Name) {
			t.Errorf("%s: %q isn't an uppercase ODS-2 file name", spec, f.Name)
		}

		contents, err := f.Contents()
		if err != nil {
			t.Errorf("%s: %v", spec, err)

			continue
		}

		if len(contents) == 0 {
			t.Errorf("%s is empty", spec)
		}

		if !bytes.Equal(contents, Canonical(contents, f.Format)) {
			t.Errorf("%s: contents aren't in canonical form", spec)
		}

		if f.Kind == Seeded {
			seeded++
		}

		if got, ok := Lookup(strings.ToLower(spec)); !ok || got.Spec() != spec {
			t.Errorf("Lookup(%q) = %v, %v", strings.ToLower(spec), got.Spec(), ok)
		}
	}

	if seeded != 1 || !seen[SysMgr+"SYSTARTUP_VMS.COM"] {
		t.Errorf("want SYSTARTUP_VMS.COM the one seeded file; %d seeded", seeded)
	}

	for _, want := range []string{
		SysMgr + "STARTUP.COM", SysMgr + "CONSOLE.DCL", SysMgr + "DEBUG.DCL",
		SysHlp + "CONSOLE.HLP", SysHlp + "DEBUG.HLP",
		SysLib + "FTNRTL.EXE", SysLib + "FORSYSDEF.TLB",
		SysExe + "DEFAULT.ROM",
		Sources + "FTN_VECTOR.MAR", Sources + "FTNRTL.OPT", Sources + "BUILDFTN.COM",
		Sources + "KERNEL.ASM", Sources + "SSDEF.ASM", Sources + "BUILDSTARLET.COM",
	} {
		if !seen[want] {
			t.Errorf("%s is missing from the table", want)
		}
	}

	if _, ok := Lookup(Root + "NOSUCH.FILE"); ok {
		t.Error("Lookup found a file that isn't there")
	}
}

// TestContentsRepeat checks that every file's contents are the same each
// time they're made: a library stamped with the time it was built would
// look like a new version to every later start.
func TestContentsRepeat(t *testing.T) {
	for _, f := range Files() {
		a, errA := f.Contents()
		b, errB := f.Contents()

		if errA != nil || errB != nil {
			t.Fatalf("%s: %v, %v", f.Spec(), errA, errB)
		}

		if SumOf(a) != SumOf(b) {
			t.Errorf("%s differs from one making to the next", f.Spec())
		}
	}
}

// TestBuildProcedureModules checks that BUILDFTN.COM assembles exactly
// the objects FTNRTL.OPT links, in its order, and that every FTNRTL
// source on the volume is one of them: a module added to the library
// without a line in the procedure would build an FTNRTL without it.
func TestBuildProcedureModules(t *testing.T) {
	read := func(spec string) string {
		f, ok := Lookup(spec)
		if !ok {
			t.Fatalf("%s isn't on the volume", spec)
		}

		data, err := f.Contents()
		if err != nil {
			t.Fatal(err)
		}

		return string(data)
	}

	var assembled []string

	for line := range strings.SplitSeq(read(Sources+"BUILDFTN.COM"), "\n") {
		if fields := strings.Fields(strings.ToUpper(line)); len(fields) == 3 && fields[0] == "$" && fields[1] == "MACRO/NOLIST" {
			assembled = append(assembled, fields[2])
		}
	}

	var linked []string

	objects := regexp.MustCompile(`([A-Z0-9_]+)\.OBJ`)

	for line := range strings.SplitSeq(read(Sources+"FTNRTL.OPT"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "!") {
			continue
		}

		for _, m := range objects.FindAllStringSubmatch(strings.ToUpper(line), -1) {
			linked = append(linked, m[1])
		}
	}

	if strings.Join(assembled, ",") != strings.Join(linked, ",") {
		t.Errorf("BUILDFTN.COM assembles\n  %v\nbut FTNRTL.OPT links\n  %v", assembled, linked)
	}

	modules := map[string]bool{}
	for _, m := range assembled {
		modules[m] = true
	}

	for _, f := range Files() {
		if f.Directory == Sources && strings.HasPrefix(f.Name, "FTN_") && strings.HasSuffix(f.Name, ".MAR") {
			if name := strings.TrimSuffix(f.Name, ".MAR"); !modules[name] {
				t.Errorf("%s isn't assembled by BUILDFTN.COM", f.Name)
			}
		}
	}
}

// TestCanonicalText checks a text file's canonical form and its records,
// both ways.
func TestCanonicalText(t *testing.T) {
	tests := []struct {
		in, want string
		records  int
	}{
		{"", "", 0},
		{"one\n", "one\n", 1},
		{"one", "one\n", 1},
		{"one\r\ntwo\r\n", "one\ntwo\n", 2},
		{"one\n\nthree", "one\n\nthree\n", 3},
		{"\n", "\n", 1},
	}

	for _, tt := range tests {
		got := Canonical([]byte(tt.in), Text)
		if string(got) != tt.want {
			t.Errorf("Canonical(%q) = %q, want %q", tt.in, got, tt.want)
		}

		records := Records(got, Text)
		if len(records) != tt.records {
			t.Errorf("Records(%q) has %d records, want %d", got, len(records), tt.records)
		}

		if back := Join(records, Text); !bytes.Equal(back, got) {
			t.Errorf("Join(Records(%q)) = %q", got, back)
		}
	}
}

// TestCanonicalBinary checks a binary file's padding and blocks.
func TestCanonicalBinary(t *testing.T) {
	for _, n := range []int{0, 1, 511, 512, 513, 1024, 1500} {
		data := bytes.Repeat([]byte{0xA5}, n)
		got := Canonical(data, Binary)

		if len(got)%BlockSize != 0 || len(got) < n || len(got)-n >= BlockSize {
			t.Errorf("Canonical of %d bytes is %d bytes", n, len(got))
		}

		if !bytes.Equal(got[:n], data) || bytes.ContainsFunc(got[n:], func(r rune) bool { return r != 0 }) {
			t.Errorf("Canonical of %d bytes changed them, or padded with other than zeros", n)
		}

		records := Records(got, Binary)
		for _, r := range records {
			if len(r) != BlockSize {
				t.Errorf("a record of %d bytes, not %d", len(r), BlockSize)
			}
		}

		if back := Join(records, Binary); !bytes.Equal(back, got) {
			t.Errorf("Join(Records()) of %d bytes differs", n)
		}
	}
}

// TestManifestRoundTrip checks that a manifest read back from its
// records is the same manifest, specifications in any case.
func TestManifestRoundTrip(t *testing.T) {
	m := NewManifest()
	m.Set("[system.syslib]ftnrtl.exe", SumOf([]byte("image")))
	m.Set(SysMgr+"STARTUP.COM", SumOf([]byte("startup")))
	m.Set(SysMgr+"GONE.COM", SumOf([]byte("gone")))
	m.Delete(SysMgr + "gone.com")

	records := m.Records()

	if !strings.HasPrefix(string(records[0]), "!") {
		t.Errorf("the manifest starts %q, not a comment", records[0])
	}

	back, err := ParseManifest(records)
	if err != nil {
		t.Fatal(err)
	}

	if got := strings.Join(back.Specs(), ","); got != SysLib+"FTNRTL.EXE,"+SysMgr+"STARTUP.COM" {
		t.Errorf("Specs() = %s", got)
	}

	for _, spec := range m.Specs() {
		want, _ := m.Sum(spec)
		if got, ok := back.Sum(strings.ToLower(spec)); !ok || got != want {
			t.Errorf("%s: %v, %v after the round trip", spec, got, ok)
		}
	}

	if !bytes.Equal(bytes.Join(back.Records(), nil), bytes.Join(records, nil)) {
		t.Error("the manifest's records changed in the round trip")
	}
}

// TestManifestDamaged checks that a damaged record is an error, not a
// file left out.
func TestManifestDamaged(t *testing.T) {
	good := SumOf([]byte("x")).String()

	for _, record := range []string{
		SysLib + "FTNRTL.EXE",
		SysLib + "FTNRTL.EXE " + good + " extra",
		SysLib + "FTNRTL.EXE " + good[:63],
		SysLib + "FTNRTL.EXE " + strings.Repeat("g", 64),
	} {
		_, err := ParseManifest([][]byte{[]byte("! a comment"), {}, []byte(record)})
		if err == nil || !strings.Contains(err.Error(), "record 3") {
			t.Errorf("ParseManifest(%q) = %v, want an error naming record 3", record, err)
		}
	}
}

// TestPopulate fills an empty volume and checks what's on it: every
// directory, in order; every file, with its format and its contents; and
// a manifest of the supplied files only, whose digests are the files'.
func TestPopulate(t *testing.T) {
	v := newMapVolume()

	m, err := Populate(v)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Join(v.dirs, ",") != strings.Join(Directories, ",") {
		t.Errorf("directories made: %v", v.dirs)
	}

	for _, f := range Files() {
		versions := v.files[f.Spec()]
		if len(versions) != 1 {
			t.Errorf("%s: %d versions written", f.Spec(), len(versions))

			continue
		}

		want, _ := f.Contents()
		if got := Join(versions[0].records, f.Format); !bytes.Equal(got, want) {
			t.Errorf("%s: what was written doesn't read back as its contents", f.Spec())
		}

		if versions[0].format != f.Format {
			t.Errorf("%s written as %v, not %v", f.Spec(), versions[0].format, f.Format)
		}

		sum, listed := m.Sum(f.Spec())

		switch {
		case f.Kind == Seeded && listed:
			t.Errorf("%s is the user's, but in the manifest", f.Spec())
		case f.Kind == Supplied && !listed:
			t.Errorf("%s is govax's, but not in the manifest", f.Spec())
		case f.Kind == Supplied && sum != SumOf(want):
			t.Errorf("%s: the manifest's digest isn't its contents'", f.Spec())
		}
	}

	read, found, err := ReadManifest(v)
	if err != nil || !found {
		t.Fatalf("ReadManifest: %v, %v", found, err)
	}

	if strings.Join(read.Specs(), ",") != strings.Join(m.Specs(), ",") {
		t.Error("the manifest read back lists other files")
	}
}

// TestPopulateKeepsSeeded checks that Populate leaves a seeded file that
// is already there alone: a volume rebuilt around the user's own startup.
func TestPopulateKeepsSeeded(t *testing.T) {
	v := newMapVolume()
	spec := SysMgr + "SYSTARTUP_VMS.COM"
	mine := [][]byte{[]byte("$ show time")}

	for _, dir := range Directories {
		_ = v.CreateDirectory(dir)
	}

	_ = v.WriteFile(spec, mine, Text)

	if _, err := Populate(v); err != nil {
		t.Fatal(err)
	}

	versions := v.files[spec]
	if len(versions) != 1 || string(versions[0].records[0]) != "$ show time" {
		t.Errorf("SYSTARTUP_VMS.COM was rewritten: %d versions", len(versions))
	}
}

// TestPopulateFailure checks that a file the volume refuses ends Populate
// with an error naming it, and that a volume with no manifest reads as
// an empty one.
func TestPopulateFailure(t *testing.T) {
	v := newMapVolume()
	v.fail = SysLib + "FTNRTL.EXE"

	if _, err := Populate(v); err == nil || !strings.Contains(err.Error(), "FTNRTL.EXE") {
		t.Errorf("Populate = %v, want FTNRTL.EXE's failure", err)
	}

	m, found, err := ReadManifest(v)
	if err != nil || found || len(m.Specs()) != 0 {
		t.Errorf("ReadManifest of a volume without one = %v, %v, %v", m.Specs(), found, err)
	}
}

// TestManifestNotes checks that KEPT= and SEEDED survive a round trip,
// that a digest set again drops a KEPT note, and that each is refused
// where it doesn't belong.
func TestManifestNotes(t *testing.T) {
	m := NewManifest()
	m.Set(SysMgr+"CONSOLE.DCL", SumOf([]byte("old")))
	m.SetKept(SysMgr+"CONSOLE.DCL", SumOf([]byte("new")))
	m.SetSeeded(SysMgr + "SYSTARTUP_VMS.COM")

	back, err := ParseManifest(m.Records())
	if err != nil {
		t.Fatal(err)
	}

	if kept, ok := back.Kept(SysMgr + "console.dcl"); !ok || kept != SumOf([]byte("new")) {
		t.Errorf("KEPT after the round trip: %v, %v", kept, ok)
	}

	if !back.Seeded(SysMgr+"SYSTARTUP_VMS.COM") || back.Seeded(SysMgr+"CONSOLE.DCL") {
		t.Error("SEEDED isn't where it was put")
	}

	if _, ok := back.Sum(SysMgr + "SYSTARTUP_VMS.COM"); ok {
		t.Error("a seeded file has a digest")
	}

	// A KEPT note alone: a file govax never wrote.
	only, err := ParseManifest([][]byte{[]byte(SysMgr + "X.COM KEPT=" + SumOf([]byte("x")).String())})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := only.Sum(SysMgr + "X.COM"); ok {
		t.Error("a KEPT note alone has a digest")
	}

	if _, ok := only.Kept(SysMgr + "X.COM"); !ok {
		t.Error("a KEPT note alone was lost")
	}

	back.Set(SysMgr+"CONSOLE.DCL", SumOf([]byte("new")))

	if _, ok := back.Kept(SysMgr + "CONSOLE.DCL"); ok {
		t.Error("Set kept the KEPT note")
	}

	good := SumOf([]byte("x")).String()

	for _, record := range []string{
		SysMgr + "X.COM SEEDED " + good,
		SysMgr + "X.COM " + good + " SEEDED",
		SysMgr + "X.COM " + good + " KEPT=" + good[:10],
		SysMgr + "X.COM KEPT=" + good + " " + good,
		SysMgr + "X.COM KEPT=" + good + " KEPT=" + good,
	} {
		if _, err := ParseManifest([][]byte{[]byte(record)}); err == nil {
			t.Errorf("ParseManifest(%q) took it", record)
		}
	}
}

// refreshTest is a volume and a stand-in for govax's files: a supplied
// text file, a supplied binary file, and a seeded file, whose contents a
// test changes as a later govax would.
type refreshTest struct {
	t      *testing.T
	v      *mapVolume
	copies map[string]string // each file's name, and govax's contents of it
}

func newRefreshTest(t *testing.T) *refreshTest {
	return &refreshTest{t: t, v: newMapVolume(), copies: map[string]string{
		"A.COM":    "$ write sys$output \"a\"\n",
		"B.EXE":    "image one",
		"SITE.COM": "$ ! yours\n",
	}}
}

// files returns the stand-in file table, reading the current copies.
func (r *refreshTest) files() []File {
	read := func(name string) func() ([]byte, error) {
		return func() ([]byte, error) { return []byte(r.copies[name]), nil }
	}

	return []File{
		{Directory: SysMgr, Name: "A.COM", Format: Text, read: read("A.COM")},
		{Directory: SysLib, Name: "B.EXE", Format: Binary, read: read("B.EXE")},
		{Directory: SysMgr, Name: "SITE.COM", Format: Text, Kind: Seeded, read: read("SITE.COM")},
	}
}

// refresh runs a refresh and returns each file's action, notified ones
// marked "!", as "A.COM=written B.EXE=kept! ...".
func (r *refreshTest) refresh() (string, *Report) {
	r.t.Helper()

	report, err := refreshFiles(r.v, r.files())
	if err != nil {
		r.t.Fatal(err)
	}

	var parts []string

	for _, c := range report.Changes {
		part := c.File.Name + "=" + c.Action.String()
		if c.Notify {
			part += "!"
		}

		parts = append(parts, part)
	}

	return strings.Join(parts, " "), report
}

// expect runs a refresh and checks its actions and whether it wrote the
// manifest.
func (r *refreshTest) expect(actions string, manifestWritten bool) {
	r.t.Helper()

	got, report := r.refresh()
	if got != actions {
		r.t.Errorf("refresh: %s\n                want %s", got, actions)
	}

	if report.ManifestWritten != manifestWritten {
		r.t.Errorf("refresh (%s): manifest written %v, want %v", got, report.ManifestWritten, manifestWritten)
	}
}

// versions returns how many versions of the file name there are.
func (r *refreshTest) versions(dir, name string) int {
	return len(r.v.files[dir+name])
}

// text returns the highest version of the text file dir+name.
func (r *refreshTest) text(dir, name string) string {
	records, _, _ := r.v.ReadFile(dir + name)

	return string(Join(records, Text))
}

// edit writes a new version of dir+name with the user's text.
func (r *refreshTest) edit(dir, name, text string) {
	_ = r.v.WriteFile(dir+name, Records(Canonical([]byte(text), Text), Text), Text)
}

// TestRefresh walks one volume through the refresh's cases (Decision 3),
// a refresh after each change.
func TestRefresh(t *testing.T) {
	r := newRefreshTest(t)

	// A new volume: everything written, and the manifest.
	r.expect("A.COM=written B.EXE=written SITE.COM=written", true)

	// Nothing changed: nothing written, not even the manifest.
	r.expect("A.COM=current B.EXE=current SITE.COM=left", false)

	if r.versions(SysMgr, "A.COM") != 1 || r.versions(Root, ManifestName) != 1 {
		t.Error("a refresh with nothing to do wrote a version")
	}

	// A newer govax changes both supplied files: each is a new version,
	// the old one kept below it.
	r.copies["A.COM"] = "$ write sys$output \"a2\"\n"
	r.copies["B.EXE"] = "image two"
	r.expect("A.COM=updated B.EXE=updated SITE.COM=left", true)

	if r.versions(SysMgr, "A.COM") != 2 || r.text(SysMgr, "A.COM") != r.copies["A.COM"] {
		t.Errorf("A.COM: %d versions, the highest %q", r.versions(SysMgr, "A.COM"), r.text(SysMgr, "A.COM"))
	}

	// The user changes A.COM; govax hasn't: kept, and nothing said.
	r.edit(SysMgr, "A.COM", "$ mine\n")
	r.expect("A.COM=kept B.EXE=current SITE.COM=left", false)

	// govax changes it too: kept, and said once.
	r.copies["A.COM"] = "$ write sys$output \"a3\"\n"
	r.expect("A.COM=kept! B.EXE=current SITE.COM=left", true)
	r.expect("A.COM=kept B.EXE=current SITE.COM=left", false)

	// And again: said again, for the new copy.
	r.copies["A.COM"] = "$ write sys$output \"a4\"\n"
	r.expect("A.COM=kept! B.EXE=current SITE.COM=left", true)

	if r.text(SysMgr, "A.COM") != "$ mine\n" {
		t.Errorf("the user's A.COM was replaced: %q", r.text(SysMgr, "A.COM"))
	}

	// The user goes back to govax's copy (a PURGE and a COPY, say): it is
	// govax's again, and the next new copy replaces it.
	r.edit(SysMgr, "A.COM", r.copies["A.COM"])
	r.expect("A.COM=current B.EXE=current SITE.COM=left", true)

	if _, kept := r.v.manifest(t).Kept(SysMgr + "A.COM"); kept {
		t.Error("the KEPT note outlived the user's change")
	}

	r.copies["A.COM"] = "$ write sys$output \"a5\"\n"
	r.expect("A.COM=updated B.EXE=current SITE.COM=left", true)

	// A supplied file deleted is written again; the seeded one, deleted,
	// is left deleted.
	delete(r.v.files, SysLib+"B.EXE")
	delete(r.v.files, SysMgr+"SITE.COM")
	r.expect("A.COM=current B.EXE=written SITE.COM=left", false)

	if r.versions(SysMgr, "SITE.COM") != 0 {
		t.Error("the seeded file came back")
	}
}

// TestRefreshInterrupted checks a refresh that stopped after writing a
// file but before writing the manifest: the file holds govax's copy, the
// manifest the old digest. The next refresh takes the file as current and
// corrects the manifest; nothing is written or reported.
func TestRefreshInterrupted(t *testing.T) {
	r := newRefreshTest(t)
	r.expect("A.COM=written B.EXE=written SITE.COM=written", true)

	r.copies["A.COM"] = "$ write sys$output \"a2\"\n"
	r.edit(SysMgr, "A.COM", r.copies["A.COM"]) // the write that happened

	r.expect("A.COM=current B.EXE=current SITE.COM=left", true)

	if r.versions(SysMgr, "A.COM") != 2 {
		t.Errorf("A.COM has %d versions, want 2", r.versions(SysMgr, "A.COM"))
	}

	if sum, _ := r.v.manifest(t).Sum(SysMgr + "A.COM"); sum != SumOf([]byte(r.copies["A.COM"])) {
		t.Error("the manifest's digest wasn't corrected")
	}

	// A failure part way: B.EXE refused. Nothing after it is done, the
	// manifest isn't written, and the next refresh (B.EXE allowed) finishes
	// the job, A.COM, already written, taken as current.
	r.copies["A.COM"] = "$ write sys$output \"a3\"\n"
	r.copies["B.EXE"] = "image two"
	r.v.fail = SysLib + "B.EXE"

	if _, err := refreshFiles(r.v, r.files()); err == nil || !strings.Contains(err.Error(), "B.EXE") {
		t.Fatalf("refresh = %v, want B.EXE's failure", err)
	}

	if r.versions(Root, ManifestName) != 2 {
		t.Errorf("the failed refresh wrote the manifest: %d versions", r.versions(Root, ManifestName))
	}

	r.v.fail = ""
	r.expect("A.COM=current B.EXE=updated SITE.COM=left", true)
}

// TestRefreshNoManifest checks a volume whose manifest is gone (deleted,
// or a volume made by hand): a file that is govax's copy is taken as
// govax's, one that differs as the user's, said once; a missing seeded
// file is made, the manifest not saying it was before.
func TestRefreshNoManifest(t *testing.T) {
	r := newRefreshTest(t)
	r.expect("A.COM=written B.EXE=written SITE.COM=written", true)

	r.edit(SysMgr, "A.COM", "$ mine\n")
	delete(r.v.files, SysMgr+"SITE.COM")
	delete(r.v.files, Root+ManifestName)

	r.expect("A.COM=kept! B.EXE=current SITE.COM=written", true)
	r.expect("A.COM=kept B.EXE=current SITE.COM=left", false)
}

// TestRefreshDamagedManifest checks that a manifest that can't be read
// stops the refresh before anything is written, so no file whose owner
// can't be told is replaced.
func TestRefreshDamagedManifest(t *testing.T) {
	r := newRefreshTest(t)
	r.expect("A.COM=written B.EXE=written SITE.COM=written", true)

	_ = r.v.WriteFile(ManifestSpec, [][]byte{[]byte("[SYSTEM.SYSMGR]A.COM not-a-digest")}, Text)
	r.copies["A.COM"] = "$ write sys$output \"a2\"\n"

	if _, err := refreshFiles(r.v, r.files()); err == nil || !strings.Contains(err.Error(), "record 1") {
		t.Errorf("refresh = %v, want the manifest's record 1", err)
	}

	if r.versions(SysMgr, "A.COM") != 1 {
		t.Error("A.COM was written past a damaged manifest")
	}
}

// TestRefreshBuiltIn runs the real refresh twice on a volume Populate
// made: the second finds every file current and writes nothing.
func TestRefreshBuiltIn(t *testing.T) {
	v := newMapVolume()
	if _, err := Populate(v); err != nil {
		t.Fatal(err)
	}

	report, err := Refresh(v)
	if err != nil {
		t.Fatal(err)
	}

	if report.ManifestWritten || report.Count(Current)+report.Count(Left) != len(Files()) {
		t.Errorf("a second refresh: %d current, %d left of %d, manifest written %v",
			report.Count(Current), report.Count(Left), len(Files()), report.ManifestWritten)
	}
}

// manifest reads the volume's manifest.
func (v *mapVolume) manifest(t *testing.T) *Manifest {
	t.Helper()

	m, _, err := ReadManifest(v)
	if err != nil {
		t.Fatal(err)
	}

	return m
}
