package rms

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

var systemUIC = ondisk.Uic{Group: 1, Member: 4}

// newCreateDirSession is a session with a freshly initialized volume
// mounted writable on DUA0 and set as the default.
func newCreateDirSession(t *testing.T) (*Session, *volume.Volume) {
	t.Helper()

	mounts := NewMountTable()
	if err := mounts.Mount("DUA0", newTestVolumeFile(t, "CREDIR"), true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	vol, _ := mounts.Lookup("DUA0")

	s := NewSession(mounts)
	if err := s.SetDefault("DUA0:[000000]"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	return s, vol
}

func mustCreateDir(t *testing.T, s *Session, spec string, opts CreateDirectoryOptions) []CreatedDirectory {
	t.Helper()

	created, err := s.CreateDirectory(spec, opts)
	if err != nil {
		t.Fatalf("CreateDirectory(%s): %v", spec, err)
	}

	return created
}

func dirHeader(t *testing.T, vol *volume.Volume, dirs ...string) ondisk.FileHeader {
	t.Helper()

	d, err := filespec.ResolveDirectory(vol, dirs)
	if err != nil {
		t.Fatalf("ResolveDirectory(%v): %v", dirs, err)
	}

	return d.Header
}

func TestCreateDirectoryDefaults(t *testing.T) {
	s, vol := newCreateDirSession(t)

	created := mustCreateDir(t, s, "[A.B]", CreateDirectoryOptions{ProcessUIC: systemUIC})

	want := []CreatedDirectory{{Name: "DUA0:[A]", Created: true}, {Name: "DUA0:[A.B]", Created: true}}
	if len(created) != 2 || created[0] != want[0] || created[1] != want[1] {
		t.Errorf("created = %+v, want %+v", created, want)
	}

	mfd := dirHeader(t, vol)

	for _, dirs := range [][]string{{"A"}, {"A", "B"}} {
		h := dirHeader(t, vol, dirs...)

		if h.Owner != systemUIC {
			t.Errorf("%v owner = %v, want the process's [1,4]", dirs, h.Owner)
		}

		if want := mfd.FileProtection | ondisk.ProtectionNoDeleteAll; h.FileProtection != want {
			t.Errorf("%v protection = %#x, want the parent's less delete, %#x", dirs, h.FileProtection, want)
		}

		if h.RecordAttributes.VersionLimit != mfd.RecordAttributes.VersionLimit {
			t.Errorf("%v version limit = %d, want the MFD's %d", dirs, h.RecordAttributes.VersionLimit, mfd.RecordAttributes.VersionLimit)
		}
	}

	// Again: both exist.
	created = mustCreateDir(t, s, "[A.B]", CreateDirectoryOptions{ProcessUIC: systemUIC})
	if len(created) != 2 || created[0].Created || created[1].Created {
		t.Errorf("second CreateDirectory = %+v, want both existing", created)
	}
}

func TestCreateDirectoryOwner(t *testing.T) {
	s, vol := newCreateDirSession(t)

	owner := ondisk.Uic{Group: 0o200, Member: 0o201}

	mustCreateDir(t, s, "[OWNED]", CreateDirectoryOptions{ProcessUIC: systemUIC, Owner: &owner})
	mustCreateDir(t, s, "[OWNED.PCHILD]", CreateDirectoryOptions{ProcessUIC: systemUIC, OwnerParent: true})
	mustCreateDir(t, s, "[OWNED.CHILD]", CreateDirectoryOptions{ProcessUIC: systemUIC})

	for dir, want := range map[string]ondisk.Uic{"OWNED": owner, "PCHILD": owner, "CHILD": systemUIC} {
		dirs := []string{"OWNED", dir}
		if dir == "OWNED" {
			dirs = dirs[:1]
		}

		if got := dirHeader(t, vol, dirs...).Owner; got != want {
			t.Errorf("%v owner = %v, want %v", dirs, got, want)
		}
	}
}

func TestCreateDirectoryVersionLimitAndProtection(t *testing.T) {
	s, vol := newCreateDirSession(t)

	limit := uint16(3)

	mustCreateDir(t, s, "[LIMITED]", CreateDirectoryOptions{VersionLimit: &limit, Protection: "(S:RWED,O:RWED,G:RWED,W:RWED)"})
	mustCreateDir(t, s, "[LIMITED.INHERIT]", CreateDirectoryOptions{})
	mustCreateDir(t, s, "[LIMITED.PARTIAL]", CreateDirectoryOptions{Protection: "(G:R)"})

	if got := dirHeader(t, vol, "LIMITED").FileProtection; got != 0 {
		t.Errorf("[LIMITED] protection = %#x, want 0", got)
	}

	inherit := dirHeader(t, vol, "LIMITED", "INHERIT")
	if inherit.RecordAttributes.VersionLimit != 3 || inherit.FileProtection != ondisk.ProtectionNoDeleteAll {
		t.Errorf("[LIMITED.INHERIT] limit %d, protection %#x; want 3, %#x",
			inherit.RecordAttributes.VersionLimit, inherit.FileProtection, ondisk.ProtectionNoDeleteAll)
	}

	// Only the group field changes from the inherited default.
	if got := dirHeader(t, vol, "LIMITED", "PARTIAL").FileProtection; got != 0x8E88 {
		t.Errorf("[LIMITED.PARTIAL] protection = %#x, want 0x8e88", got)
	}
}

func TestCreateDirectoryRelative(t *testing.T) {
	s, vol := newCreateDirSession(t)

	mustCreateDir(t, s, "[PLAIN]", CreateDirectoryOptions{})

	if err := s.SetDefault("[PLAIN]"); err != nil {
		t.Fatal(err)
	}

	sub := mustCreateDir(t, s, "[.SUB]", CreateDirectoryOptions{})
	sib := mustCreateDir(t, s, "[-.SIBLING]", CreateDirectoryOptions{})

	if last := sub[len(sub)-1]; last.Name != "DUA0:[PLAIN.SUB]" || !last.Created {
		t.Errorf("[.SUB] = %+v, want DUA0:[PLAIN.SUB] created", sub)
	}

	if last := sib[len(sib)-1]; last.Name != "DUA0:[SIBLING]" || !last.Created {
		t.Errorf("[-.SIBLING] = %+v, want DUA0:[SIBLING] created", sib)
	}

	dirHeader(t, vol, "PLAIN", "SUB")
	dirHeader(t, vol, "SIBLING")
}

func TestCreateDirectoryErrors(t *testing.T) {
	s, vol := newCreateDirSession(t)

	tooBig := uint16(40000)

	for spec, opts := range map[string]CreateDirectoryOptions{
		"[PLAIN]FILE.DAT": {},
		"[WILD*]":         {},
		"[PLAIN...]":      {},
		"DUB0:[X]":        {},
		"[BADLIMIT]":      {VersionLimit: &tooBig},
		"[BADPROT]":       {Protection: "(X:R)"},
	} {
		if _, err := s.CreateDirectory(spec, opts); err == nil {
			t.Errorf("CreateDirectory(%s): want an error, got none", spec)
		}
	}

	if _, err := s.CreateDirectory("[PLAIN]FILE.DAT", CreateDirectoryOptions{}); !errors.Is(err, ErrNotDirectorySpec) {
		t.Errorf("CreateDirectory([PLAIN]FILE.DAT): err = %v, want ErrNotDirectorySpec", err)
	}

	if _, err := s.CreateDirectory("DUB0:[X]", CreateDirectoryOptions{}); !errors.As(err, new(*NotMountedError)) {
		t.Errorf("CreateDirectory(DUB0:[X]): err = %v, want a NotMountedError", err)
	}

	for _, dir := range []string{"PLAIN", "WILD*", "BADLIMIT", "BADPROT"} {
		if _, err := filespec.ResolveDirectory(vol, []string{dir}); err == nil {
			t.Errorf("[%s] was made", dir)
		}
	}
}

func TestCreateDirectoryReadOnly(t *testing.T) {
	mounts := NewMountTable()
	if err := mounts.Mount("DUA0", newTestVolumeFile(t, "CREDIR"), false); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	s := NewSession(mounts)

	if _, err := s.CreateDirectory("DUA0:[X]", CreateDirectoryOptions{}); err == nil {
		t.Error("CreateDirectory on a read-only volume: want an error, got none")
	}
}

// TestCreateDirectoryHost: with no default on a volume, a directory is a
// host directory, relative to the current directory.
func TestCreateDirectoryHost(t *testing.T) {
	t.Chdir(t.TempDir())

	s := NewSession(NewMountTable())

	created, err := s.CreateDirectory("[.WORK.OUT]", CreateDirectoryOptions{})
	if err != nil {
		t.Fatalf("CreateDirectory([.WORK.OUT]): %v", err)
	}

	want := []CreatedDirectory{{Name: "WORK", Created: true}, {Name: filepath.Join("WORK", "OUT"), Created: true}}
	if len(created) != 2 || created[0] != want[0] || created[1] != want[1] {
		t.Errorf("created = %+v, want %+v", created, want)
	}

	if info, err := os.Stat(filepath.Join("WORK", "OUT")); err != nil || !info.IsDir() {
		t.Errorf("WORK/OUT: %v", err)
	}

	created, err = s.CreateDirectory("WORK/OUT/DEEP", CreateDirectoryOptions{})
	if err != nil || len(created) != 3 || created[1].Created || !created[2].Created {
		t.Errorf("CreateDirectory(WORK/OUT/DEEP) = %+v, %v, want only DEEP created", created, err)
	}

	if err := os.Chdir("WORK"); err != nil {
		t.Fatal(err)
	}

	created, err = s.CreateDirectory("[-.SIBLING]", CreateDirectoryOptions{})
	if err != nil || len(created) != 1 || created[0].Name != filepath.Join("..", "SIBLING") {
		t.Errorf("CreateDirectory([-.SIBLING]) = %+v, %v", created, err)
	}

	if _, err := os.Stat(filepath.Join("..", "SIBLING")); err != nil {
		t.Errorf("../SIBLING: %v", err)
	}

	for _, spec := range []string{"[]", "[*]", "[A...]", "[A.]", "X.DAT"} {
		if _, err := s.CreateDirectory(spec, CreateDirectoryOptions{}); err == nil {
			t.Errorf("CreateDirectory(%s) on the host: want an error, got none", spec)
		}
	}
}

// TestDirectoryOwnerProtection: DIRECTORY/OWNER and /PROTECTION show a new
// directory's owner and protection, and /FULL shows both.
func TestDirectoryOwnerProtection(t *testing.T) {
	s, _ := newCreateDirSession(t)

	owner := ondisk.Uic{Group: 0o200, Member: 0o201}
	mustCreateDir(t, s, "[OWNED]", CreateDirectoryOptions{Owner: &owner, Protection: "(S:RWE,O:RWE,G:RE,W:E)"})

	for _, opts := range []DirectoryOptions{{Owner: true, Protection: true}, {Full: true}} {
		out, err := s.Directory("[000000]OWNED.DIR", opts)
		if err != nil {
			t.Fatalf("Directory(%+v): %v", opts, err)
		}

		if !strings.Contains(out, "[200,201]") || !strings.Contains(out, "(RWE,RWE,RE,E)") {
			t.Errorf("Directory(%+v) = %q, want the owner [200,201] and protection (RWE,RWE,RE,E)", opts, out)
		}
	}

	out, err := s.Directory("[000000]OWNED.DIR", DirectoryOptions{Protection: true})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out, "[200,201]") {
		t.Errorf("Directory/PROTECTION = %q, want no owner column", out)
	}
}
