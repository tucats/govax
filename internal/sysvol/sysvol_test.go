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
