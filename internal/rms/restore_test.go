package rms

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/ods2/ondisk"
)

// The probe's save sets and VMS's reports of what it restored from them
// (docs/PHASE-69 - backup save sets.md, testdata/backup69).
const backup69Dir = "../../testdata/backup69/vax"

// readSaveSet reads the probe's save set name.
func readSaveSet(t *testing.T, name string) *backup.SaveSet {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(backup69Dir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}

	set, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{})
	if err != nil {
		t.Fatalf("backup.Read(%s): %v", name, err)
	}

	return set
}

// savedFile is the file named name ("[B69]TEXT.TXT;1") in set.
func savedFile(t *testing.T, set *backup.SaveSet, name string) *backup.File {
	t.Helper()

	for _, f := range set.Files {
		if f.Name == name {
			return f
		}
	}

	t.Fatalf("no %s in the save set", name)

	return nil
}

// newRestoreSession mounts a fresh volume of blocks blocks with clusters
// of cluster blocks as DUA1, writable, and returns a session on it.
func newRestoreSession(t *testing.T, blocks uint32, cluster uint16) *Session {
	t.Helper()

	path := filepath.Join(t.TempDir(), "restore.dsk")
	if err := InitializeContainer(path, blocks, "RESTORE", cluster, "RD54"); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	mounts := NewMountTable()
	if err := mounts.Mount("DUA1", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	t.Cleanup(func() { _ = mounts.DismountAll() })

	return NewSession(mounts)
}

// underR1 is a saved file's name, "[B69.SUB1]A.TXT;1", restored under
// [R1] on DUA1: "DUA1:[R1.B69.SUB1]A.TXT;1".
func underR1(name string) string {
	return "DUA1:[R1." + strings.TrimPrefix(name, "[")
}

// systemOwner names the probe's owner [1,4] as VMS's DIRECTORY did.
func systemOwner(uic uint32) string {
	if uic == 0x00010004 {
		return "[SYSTEM]"
	}

	return ownerUIC(uic)
}

// ownerUIC writes a UIC as "[g,m]", octal.
func ownerUIC(uic uint32) string {
	return "[" + strconvOctal(uic>>16) + "," + strconvOctal(uic&0xFFFF) + "]"
}

func strconvOctal(n uint32) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	for ; n > 0; n /= 8 {
		digits = append([]byte{byte('0' + n%8)}, digits...)
	}

	return string(digits)
}

// fileIDs and dirDates mask what a restore makes anew: every file's ID,
// and a directory's dates, which are the restore's own.
var fileIDs = regexp.MustCompile(`File ID:  \(\d+,\d+,\d+\) *`)

// TestRestoreTreeOracle restores B69.BCK as the probe's BACKUP/LOG
// [SETS]B69.BCK/SAVE_SET [R1...] did, onto a volume with VMS's 3-block
// clusters, and matches DIRECTORY/FULL of [R1...] with VMS's, r1.lst,
// line for line. The file IDs are masked, and so are the dates of the
// directories, which the restore made. The owner is [1,4], the one VMS's
// restore gave every file (OWNED.TXT's [200,201] included: /BY_OWNER's
// default, subtask 9's).
func TestRestoreTreeOracle(t *testing.T) {
	s := newRestoreSession(t, 4000, 3)
	set := readSaveSet(t, "b69.bck")
	owner := ondisk.Uic{Group: 1, Member: 4}

	var made []string

	for _, f := range set.Files {
		target, err := s.Locate(underR1(f.Name), false)
		if err != nil {
			t.Fatalf("Locate(%s): %v", f.Name, err)
		}

		r, err := s.RestoreFile(target, f, RestoreOptions{Owner: &owner})
		if err != nil {
			t.Fatalf("RestoreFile(%s): %v", f.Name, err)
		}

		for _, d := range r.Directories {
			made = append(made, d.Name)
		}

		if !f.Directory() && r.Name != underR1(f.Name) {
			t.Errorf("RestoreFile(%s) made %s, want %s", f.Name, r.Name, underR1(f.Name))
		}
	}

	// [R1] and [R1.B69] are made for the first file, [R1.B69.SUB1] and
	// [R1.B69.SUB1.SUB2] from their saved directory files, in their
	// places. (VMS's /LOG showed CREDIR for the last three: one line for
	// the path made with the first file, naming its deepest level.)
	wantMade := []string{"DUA1:[R1]", "DUA1:[R1.B69]", "DUA1:[R1.B69.SUB1]", "DUA1:[R1.B69.SUB1.SUB2]"}
	if strings.Join(made, " ") != strings.Join(wantMade, " ") {
		t.Errorf("directories made = %q, want %q", made, wantMade)
	}

	listing, _, err := s.Directory("DUA1:[R1...]", DirectoryOptions{Full: true, OwnerName: systemOwner})
	if err != nil {
		t.Fatalf("Directory: %v", err)
	}

	want, err := os.ReadFile(filepath.Join(backup69Dir, "r1.lst"))
	if err != nil {
		t.Fatal(err)
	}

	compareRestoredListing(t, listing, string(want))
}

// expectedRestoreDifferences are the lines of VMS's DIRECTORY/FULL that
// govax's doesn't write, each with govax's line: an indexed file's prolog
// and key count come from the file's own prolog, which govax's DIRECTORY
// doesn't read.
var expectedRestoreDifferences = map[string]string{
	"File organization:  Indexed, Prolog: 3, Using 1 key": "File organization:  Indexed",
}

// compareRestoredListing matches got, govax's DIRECTORY/FULL, with want,
// VMS's, line for line, with the file IDs masked, and the creation and
// revision dates of the directory files.
func compareRestoredListing(t *testing.T, got, want string) {
	t.Helper()

	mask := func(text string) []string {
		lines := strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r", ""), "\n"), "\n")
		dir := false

		for i, line := range lines {
			if strings.Contains(line, "File ID:") {
				dir = strings.Contains(line, ".DIR;")
				lines[i] = fileIDs.ReplaceAllString(line, "File ID:  (*)")
			}

			if dir && (strings.HasPrefix(line, "Created:") || strings.HasPrefix(line, "Revised:")) {
				// The revision count, "(1)", stays.
				count := ""
				if strings.HasPrefix(line, "Revised:") {
					count = line[strings.LastIndexByte(line, ' '):]
				}

				lines[i] = line[:len("Created:")] + " (the restore's)" + count
			}

			if difference, ok := expectedRestoreDifferences[line]; ok {
				lines[i] = difference
			}
		}

		return lines
	}

	g, w := mask(got), mask(want)

	for i := 0; i < max(len(g), len(w)); i++ {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}

		if i < len(w) {
			wl = w[i]
		}

		if gl != wl {
			t.Errorf("line %d:\n got  %q\n want %q", i+1, gl, wl)
		}
	}
}

// TestRestoreData checks that restored files hold the saved data up to
// their ends of file, across save set blocks (BIG.DAT) and for an image
// (PI.EXE against VMS's own copy).
func TestRestoreData(t *testing.T) {
	s := newRestoreSession(t, 4000, 1)
	set := readSaveSet(t, "b69.bck")

	for _, name := range []string{"[B69.SUB1.SUB2]BIG.DAT;1", "[B69]PI.EXE;1", "[B69]VFC.TXT;1", "[B69]EMPTY.DAT;1"} {
		f := savedFile(t, set, name)

		target, _ := s.Locate(underR1(name), false)
		if _, err := s.RestoreFile(target, f, RestoreOptions{}); err != nil {
			t.Fatalf("RestoreFile(%s): %v", name, err)
		}

		got, _, err := s.ReadRawFile(target)
		if err != nil {
			t.Fatalf("ReadRawFile(%s): %v", name, err)
		}

		want := f.Data()
		saved, _ := ondisk.DecodeRecAttr(f.RecordAttributes[:])
		want = want[:fileBytes(saved)]

		if !bytes.Equal(got, want) {
			t.Errorf("%s: %d bytes restored, want the saved %d", name, len(got), len(want))
		}
	}

	// pi.bck's PI.EXE is VMS's pi.exe (B69.BCK's was linked again by the
	// probe).
	pi := savedFile(t, readSaveSet(t, "pi.bck"), "[XFER.PI]PI.EXE;1")
	target, _ := s.Locate("DUA1:[PI]PI.EXE;1", false)

	if _, err := s.RestoreFile(target, pi, RestoreOptions{}); err != nil {
		t.Fatalf("RestoreFile(PI.EXE): %v", err)
	}

	exe, _ := os.ReadFile(filepath.Join(backup69Dir, "pi.exe"))
	got, _, _ := s.ReadRawFile(target)

	if !bytes.Equal(got, exe) {
		t.Error("the restored PI.EXE isn't VMS's pi.exe")
	}
}

// fileBytes is a file's length by its end of file.
func fileBytes(attr ondisk.RecAttr) int {
	if attr.EndOfFileBlock == 0 {
		return 0
	}

	return int(attr.EndOfFileBlock-1)*ondisk.BlockSize + int(attr.FirstFreeByte)
}

// TestRestoreOwnerKept: without an owner, the saved one is kept.
func TestRestoreOwnerKept(t *testing.T) {
	s := newRestoreSession(t, 2000, 1)
	f := savedFile(t, readSaveSet(t, "b69.bck"), "[B69]OWNED.TXT;1")

	target, _ := s.Locate("DUA1:[R]OWNED.TXT", false)
	if _, err := s.RestoreFile(target, f, RestoreOptions{}); err != nil {
		t.Fatalf("RestoreFile: %v", err)
	}

	a, err := s.FileAttributes("DUA1:[R]OWNED.TXT")
	if err != nil {
		t.Fatalf("FileAttributes: %v", err)
	}

	if a.Owner != 0o200<<16|0o201 {
		t.Errorf("owner %s, want [200,201]", ownerUIC(a.Owner))
	}
}

// TestRestoreVersions: with no version, the next is made; an explicit
// version that exists is RMS$_FEX's *OutputExistsError, with the file's
// full name, and nothing is changed.
func TestRestoreVersions(t *testing.T) {
	s := newRestoreSession(t, 2000, 1)
	f := savedFile(t, readSaveSet(t, "b69.bck"), "[B69]TEXT.TXT;1")

	restore := func(name string) (RestoreResult, error) {
		target, err := s.Locate(name, false)
		if err != nil {
			t.Fatalf("Locate(%s): %v", name, err)
		}

		return s.RestoreFile(target, f, RestoreOptions{})
	}

	if r, err := restore("DUA1:[R7]TEXT.TXT;1"); err != nil || r.Name != "DUA1:[R7]TEXT.TXT;1" {
		t.Fatalf("first restore: %+v, %v", r, err)
	}

	_, err := restore("DUA1:[R7]TEXT.TXT;1")

	var exists *OutputExistsError
	if !errors.As(err, &exists) || exists.Spec != "DUA1:[R7]TEXT.TXT;1" {
		t.Fatalf("second restore of ;1: %v, want an OutputExistsError naming DUA1:[R7]TEXT.TXT;1", err)
	}

	r, err := restore("DUA1:[R7]TEXT.TXT")
	if err != nil || r.Name != "DUA1:[R7]TEXT.TXT;2" || len(r.Directories) != 0 {
		t.Fatalf("restore with no version: %+v, %v; want ;2 and no directory made", r, err)
	}
}

// TestRestoreHeaderOnly: a file saved without its data (NOBACKUP) gets
// its allocation and end of file, and its blocks read as zeros.
func TestRestoreHeaderOnly(t *testing.T) {
	s := newRestoreSession(t, 2000, 3)
	f := savedFile(t, readSaveSet(t, "b69.bck"), "[B69]NOBACK.TXT;1")

	if !f.HeaderOnly() {
		t.Fatal("NOBACK.TXT isn't header-only in the save set")
	}

	target, _ := s.Locate("DUA1:[R]NOBACK.TXT;1", false)
	if _, err := s.RestoreFile(target, f, RestoreOptions{}); err != nil {
		t.Fatalf("RestoreFile: %v", err)
	}

	a, err := s.FileAttributes("DUA1:[R]NOBACK.TXT;1")
	if err != nil {
		t.Fatalf("FileAttributes: %v", err)
	}

	if a.Used() != 1 || a.Allocated != 3 || !a.NoBackup() {
		t.Errorf("size %d/%d, NOBACKUP %v; want 1/3, set", a.Used(), a.Allocated, a.NoBackup())
	}

	data, _, err := s.ReadRawFile(target)
	if err != nil {
		t.Fatalf("ReadRawFile: %v", err)
	}

	if !bytes.Equal(data, make([]byte, len(data))) {
		t.Error("a header-only file's data isn't zeros")
	}
}

// TestRestoreContiguousNoRoom: a contiguous file no free run can hold is
// ErrNoContiguousSpace, and nothing of it is left.
func TestRestoreContiguousNoRoom(t *testing.T) {
	s := newRestoreSession(t, 100, 1)
	f := savedFile(t, readSaveSet(t, "b69.bck"), "[B69]CONTIG.DAT;1")

	target, _ := s.Locate("DUA1:[000000]CONTIG.DAT;1", false)

	_, err := s.RestoreFile(target, f, RestoreOptions{})
	if !errors.Is(err, ErrNoContiguousSpace) || !errors.Is(err, ErrACPDeviceFull) {
		t.Fatalf("RestoreFile: %v, want ErrNoContiguousSpace", err)
	}

	if _, err := s.FileAttributes("DUA1:[000000]CONTIG.DAT;1"); err == nil {
		t.Error("the file that couldn't be restored is there")
	}
}

// TestRestoreDirectoryExists: a saved directory file whose directory
// exists already leaves it as it is.
func TestRestoreDirectoryExists(t *testing.T) {
	s := newRestoreSession(t, 2000, 1)
	set := readSaveSet(t, "b69.bck")

	a := savedFile(t, set, "[B69.SUB1]A.TXT;1")
	sub1 := savedFile(t, set, "[B69]SUB1.DIR;1")

	target, _ := s.Locate("DUA1:[R.SUB1]A.TXT;1", false)
	if _, err := s.RestoreFile(target, a, RestoreOptions{}); err != nil {
		t.Fatalf("RestoreFile(A.TXT): %v", err)
	}

	target, _ = s.Locate("DUA1:[R]SUB1.DIR;1", false)

	r, err := s.RestoreFile(target, sub1, RestoreOptions{})
	if err != nil || r.Created || len(r.Directories) != 0 || r.Name != "DUA1:[R.SUB1]" {
		t.Errorf("RestoreFile(SUB1.DIR): %+v, %v; want DUA1:[R.SUB1], not created", r, err)
	}

	if _, err := s.FileAttributes("DUA1:[R.SUB1]A.TXT;1"); err != nil {
		t.Errorf("A.TXT is gone: %v", err)
	}

	target, _ = s.Locate("DUA1:[R]SUB1.TXT;1", false)
	if _, err := s.RestoreFile(target, sub1, RestoreOptions{}); err == nil {
		t.Error("a directory restored as SUB1.TXT")
	}
}

// TestRestoreReadOnly: a volume mounted read-only is ErrACPWriteLocked.
func TestRestoreReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ro.dsk")
	if err := InitializeContainer(path, 400, "RO", 1, "RD54"); err != nil {
		t.Fatal(err)
	}

	mounts := NewMountTable()
	if err := mounts.Mount("DUA1", path, false); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = mounts.DismountAll() })

	s := NewSession(mounts)
	f := savedFile(t, readSaveSet(t, "b69.bck"), "[B69]TEXT.TXT;1")

	target, _ := s.Locate("DUA1:[000000]TEXT.TXT", false)
	if _, err := s.RestoreFile(target, f, RestoreOptions{}); !errors.Is(err, ErrACPWriteLocked) {
		t.Errorf("RestoreFile: %v, want ErrACPWriteLocked", err)
	}
}

// TestRestoreToHost restores to the host by COPY's rule: a text file's
// records as lines, an image's bytes, an object module's records in the
// host's object layout, a directory as a host directory; the directories
// made are reported, and a file there already is RMS$_FEX's error.
func TestRestoreToHost(t *testing.T) {
	s := NewSession(NewMountTable())
	set := readSaveSet(t, "b69.bck")
	root := filepath.Join(t.TempDir(), "out")

	restore := func(name, path string) RestoreResult {
		t.Helper()

		r, err := s.RestoreFile(FileLocation{Host: true, Name: path}, savedFile(t, set, name), RestoreOptions{})
		if err != nil {
			t.Fatalf("RestoreFile(%s): %v", name, err)
		}

		return r
	}

	r := restore("[B69]TEXT.TXT;1", filepath.Join(root, "b69", "text.txt"))
	if len(r.Directories) != 2 || r.Directories[0].Name != root || r.Directories[1].Name != filepath.Join(root, "b69") {
		t.Errorf("directories made = %+v, want %s and its b69", r.Directories, root)
	}

	text, _ := os.ReadFile(filepath.Join(root, "b69", "text.txt"))
	if !bytes.HasSuffix(text, []byte("\n")) || bytes.Contains(text, []byte{0}) || bytes.Count(text, []byte("\n")) < 2 {
		t.Errorf("text.txt = %q, want lines", text)
	}

	// VFC.TXT is TEXT.TXT's lines in a print file: the same lines.
	restore("[B69]VFC.TXT;1", filepath.Join(root, "b69", "vfc.txt"))
	restore("[B69]STMLF.TXT;1", filepath.Join(root, "b69", "stmlf.txt"))

	for _, name := range []string{"vfc.txt", "stmlf.txt"} {
		got, _ := os.ReadFile(filepath.Join(root, "b69", name))
		if !bytes.Equal(got, text) {
			t.Errorf("%s = %q, want text.txt's lines %q", name, got, text)
		}
	}

	// pi.bck's files are VMS's pi.exe and pi.obj (B69.BCK's were made
	// again by the probe).
	pi := readSaveSet(t, "pi.bck")

	if _, err := s.RestoreFile(FileLocation{Host: true, Name: filepath.Join(root, "pi.exe")}, savedFile(t, pi, "[XFER.PI]PI.EXE;1"), RestoreOptions{}); err != nil {
		t.Fatalf("RestoreFile(PI.EXE): %v", err)
	}

	exe, _ := os.ReadFile(filepath.Join(backup69Dir, "pi.exe"))
	if got, _ := os.ReadFile(filepath.Join(root, "pi.exe")); !bytes.Equal(got, exe) {
		t.Error("pi.exe isn't VMS's bytes")
	}

	// An object module's host layout is ODS-2's variable-length records,
	// so the host file is VMS's pi.obj up to its end of file.
	if _, err := s.RestoreFile(FileLocation{Host: true, Name: filepath.Join(root, "pi.obj")}, savedFile(t, pi, "[XFER.PI]PI.OBJ;1"), RestoreOptions{}); err != nil {
		t.Fatalf("RestoreFile(PI.OBJ): %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(root, "pi.obj"))
	vaxObj, _ := os.ReadFile(filepath.Join(backup69Dir, "pi.obj"))

	saved, _ := ondisk.DecodeRecAttr(savedFile(t, pi, "[XFER.PI]PI.OBJ;1").RecordAttributes[:])
	if want := vaxObj[:fileBytes(saved)]; !bytes.Equal(got, want) {
		t.Errorf("pi.obj: %d bytes, want VMS's %d up to its end of file", len(got), len(want))
	}

	r = restore("[B69]SUB1.DIR;1", filepath.Join(root, "sub1"))
	if info, err := os.Stat(filepath.Join(root, "sub1")); err != nil || !info.IsDir() || !r.Created {
		t.Errorf("SUB1.DIR: %+v, %v; want a host directory made", r, err)
	}

	_, err := s.RestoreFile(FileLocation{Host: true, Name: filepath.Join(root, "pi.exe")}, savedFile(t, set, "[B69]PI.EXE;1"), RestoreOptions{})

	var exists *OutputExistsError
	if !errors.As(err, &exists) {
		t.Errorf("restoring over pi.exe: %v, want an OutputExistsError", err)
	}
}

// TestACLText lays out access control lists as DIRECTORY/FULL shows them.
func TestACLText(t *testing.T) {
	acl := []byte{
		12, 1, 0, 0, 3, 0, 0, 0, 4, 0, 1, 0,
		12, 1, 0, 0, 1, 0, 0, 0, 0o201, 0, 0o200, 0,
	}

	want := "(IDENTIFIER=[SYSTEM],ACCESS=READ+WRITE)\n                    (IDENTIFIER=[200,201],ACCESS=READ)"
	if got := aclText(acl, DirectoryOptions{OwnerName: systemOwner}); got != want {
		t.Errorf("aclText = %q, want %q", got, want)
	}

	if got := aclText(nil, DirectoryOptions{}); got != "None" {
		t.Errorf("aclText(nil) = %q, want None", got)
	}

	if got := aclText([]byte{4, 9, 1, 2}, DirectoryOptions{}); got != "(ACE type 9: 04090102)" {
		t.Errorf("aclText of another kind of ACE = %q", got)
	}
}

// TestRestoreExisting: the output file qualifiers' rules for a version
// that's there. /OVERLAY writes the saved file over it, keeping its file
// ID and extending it when the saved file is larger, the overlaid file's
// own ACL gone; /REPLACE makes it anew (VMS's r7.lst: a new file ID);
// /NEW_VERSION makes the next version when the version or a higher one
// is there, and the saved version otherwise; and a file someone has open
// can be neither overlaid nor replaced.
func TestRestoreExisting(t *testing.T) {
	s := newRestoreSession(t, 4000, 3)
	set := readSaveSet(t, "b69.bck")
	text := savedFile(t, set, "[B69]TEXT.TXT;1")
	medium := savedFile(t, set, "[B69.SUB1]MEDIUM.DAT;1")
	acl := savedFile(t, set, "[B69]ACL.TXT;1")

	restore := func(name string, f *backup.File, rule ExistingFile) (RestoreResult, error) {
		t.Helper()

		target, err := s.Locate(name, false)
		if err != nil {
			t.Fatalf("Locate(%s): %v", name, err)
		}

		return s.RestoreFile(target, f, RestoreOptions{Existing: rule})
	}

	attributes := func(name string) FileAttributes {
		t.Helper()

		a, err := s.FileAttributes(name)
		if err != nil {
			t.Fatalf("FileAttributes(%s): %v", name, err)
		}

		return a
	}

	data := func(name string) []byte {
		t.Helper()

		target, _ := s.Locate(name, false)

		b, _, err := s.ReadRawFile(target)
		if err != nil {
			t.Fatalf("ReadRawFile(%s): %v", name, err)
		}

		return b
	}

	const x = "DUA1:[R]X.TXT;1"

	if _, err := restore(x, acl, ExistingRefused); err != nil {
		t.Fatal(err)
	}

	first := attributes(x)

	// /OVERLAY of a larger file: the same file ID, extended, the saved
	// data and attributes, and no ACL.
	if r, err := restore(x, medium, ExistingOverlay); err != nil || r.Name != x || !r.Created {
		t.Fatalf("overlay: %+v, %v", r, err)
	}

	if a := attributes(x); a.FID != first.FID || a.Used() != 74 || a.Allocated < 74 {
		t.Errorf("overlaid: %s %d/%d, want %s 74 blocks", a.FID, a.Used(), a.Allocated, first.FID)
	}

	if !bytes.Equal(data(x), medium.Data()[:len(data(x))]) {
		t.Error("overlaid data isn't MEDIUM.DAT's")
	}

	listing, _, err := s.Directory(x, DirectoryOptions{Full: true})
	if err != nil || !strings.Contains(listing, "Access Cntrl List:  None") {
		t.Errorf("overlaid file's ACL:\n%s%v", listing, err)
	}

	// /REPLACE: a new file ID, the saved file.
	if _, err := restore(x, text, ExistingReplace); err != nil {
		t.Fatalf("replace: %v", err)
	}

	if a := attributes(x); a.FID == first.FID || a.Used() != 1 {
		t.Errorf("replaced: %s %d blocks, want a new file ID and 1 block", a.FID, a.Used())
	}

	// /NEW_VERSION: ;1 is there, so ;2; ;1 again, ;3; ;5, which isn't
	// there and is higher, ;5.
	for _, tc := range []struct{ name, want string }{
		{x, "DUA1:[R]X.TXT;2"},
		{x, "DUA1:[R]X.TXT;3"},
		{"DUA1:[R]X.TXT;5", "DUA1:[R]X.TXT;5"},
		{"DUA1:[R]X.TXT;4", "DUA1:[R]X.TXT;6"},
	} {
		if r, err := restore(tc.name, text, ExistingNewVersion); err != nil || r.Name != tc.want {
			t.Errorf("new version of %s: %s, %v; want %s", tc.name, r.Name, err, tc.want)
		}
	}

	// Open, so neither /OVERLAY nor /REPLACE.
	target, _ := s.Locate(x, false)

	open, err := s.OpenSaveSet(target)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = open.Close() }()

	for _, rule := range []ExistingFile{ExistingOverlay, ExistingReplace} {
		if _, err := restore(x, text, rule); !errors.Is(err, ErrACPAccessConflict) {
			t.Errorf("rule %d of an open file: %v, want ErrACPAccessConflict", rule, err)
		}
	}
}

// TestRestoreParentOwner: /BY_OWNER=PARENT gives a file, and a saved
// directory, the owner of the directory it's restored to (VMS's [R10C]).
func TestRestoreParentOwner(t *testing.T) {
	s := newRestoreSession(t, 2000, 3)
	set := readSaveSet(t, "b69.bck")

	owner := ondisk.Uic{Group: 0o100, Member: 0o100}
	if _, err := s.CreateDirectory("DUA1:[R10C]", CreateDirectoryOptions{Owner: &owner}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ saved, target, check string }{
		{"[B69]OWNED.TXT;1", "DUA1:[R10C]OWNED.TXT;1", "DUA1:[R10C]OWNED.TXT;1"},
		{"[B69]SUB1.DIR;1", "DUA1:[R10C]SUB1.DIR;1", "DUA1:[R10C]SUB1.DIR;1"},
	} {
		target, _ := s.Locate(tc.target, false)

		if _, err := s.RestoreFile(target, savedFile(t, set, tc.saved), RestoreOptions{ParentOwner: true, Owner: &ondisk.Uic{Group: 1, Member: 4}}); err != nil {
			t.Fatalf("%s: %v", tc.saved, err)
		}

		a, err := s.FileAttributes(tc.check)
		if err != nil {
			t.Fatal(err)
		}

		if a.Owner != 0o100<<16|0o100 {
			t.Errorf("%s: owner %X, want [100,100]", tc.saved, a.Owner)
		}
	}
}
