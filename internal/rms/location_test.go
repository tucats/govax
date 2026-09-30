package rms

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/lnm"
)

// TestClassifyName covers the syntax rules (2 and 3) of location.go, by
// themselves.
func TestClassifyName(t *testing.T) {
	cases := []struct {
		name string
		want nameKind
	}{
		{"HELLO.MAR", nameBare},
		{"hello.mar", nameBare},
		{"HELLO", nameBare},
		{"HELLO.MAR;3", nameBare},
		{"foo[1", nameBare},
		{"[DIR]HELLO.MAR", nameVolume},
		{"[DIR.SUB]HELLO.MAR", nameVolume},
		{"[.SUB]HELLO.MAR", nameVolume},
		{"<DIR>HELLO.MAR", nameVolume},
		{"DUA0:HELLO.MAR", nameVolume},
		{"DUA0:[X]HELLO.MAR;2", nameVolume},
		{"SRC:HELLO.MAR", nameVolume},
		{"_DUA0:", nameVolume},
		{"C:x.mar", nameHost},
		{`C:\src\hello.mar`, nameHost},
		{"C:/src/hello.mar", nameHost},
		{"./hello.mar", nameHost},
		{"src/hello.mar", nameHost},
		{"/abs/hello.mar", nameHost},
		{`src\hello.mar`, nameHost},
		{"DUA0:/x", nameHost},
		{"dir/[x]/y.mar", nameHost},
	}

	for _, c := range cases {
		if got := classifyName(c.name); got != c.want {
			t.Errorf("classifyName(%q) = %d, want %d", c.name, got, c.want)
		}
	}
}

// newLocateSession is a session with DUA0 mounted (read-only is enough)
// and no SET DEFAULT.
func newLocateSession(t *testing.T) *Session {
	t.Helper()

	mounts := NewMountTable()
	if err := mounts.Mount("DUA0", newTestVolumeFile(t, "LOCVOL"), false); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	return NewSession(mounts)
}

func defineLogical(t *testing.T, s *Session, name, value string) {
	t.Helper()

	if _, err := s.Logicals.Define(lnm.ProcessTableName, name, lnm.Supervisor, 0, []lnm.Equivalence{{Value: value}}); err != nil {
		t.Fatalf("Define(%s): %v", name, err)
	}
}

func TestLocate(t *testing.T) {
	s := newLocateSession(t)
	defineLogical(t, s, "SRC", "DUA0:[X]")
	defineLogical(t, s, "ELSEWHERE", "DUB0:[X]")

	cases := []struct {
		name     string
		host     bool
		wantHost bool
	}{
		{"HELLO.MAR", false, true},               // bare, no SET DEFAULT
		{"DUA0:[X]HELLO.MAR", true, true},        // /HOST wins over VMS syntax
		{"hello.mar", true, true},                // /HOST on a bare name
		{"DUA0:[X]HELLO.MAR", false, false},      // VMS syntax, mounted
		{"DUA0:HELLO.MAR", false, false},         // device only
		{"SRC:HELLO.MAR", false, false},          // logical name onto a mounted device
		{"./hello.mar", false, true},             // host path
		{`C:\src\hello.mar`, false, true},        // Windows path
		{"C:hello.mar", false, true},             // Windows drive-relative
		{"_DUA0:[X]HELLO.MAR", false, false},     // physical device marker
		{"dua0:[x]hello.mar", false, false},      // lowercase VMS syntax
		{"SRC:[.SUB]HELLO.MAR", false, false},    // logical name plus more directory
		{"DUA0:[X]HELLO.MAR/extra", false, true}, // has a slash: host
		{"DUA0:[X]HELLO.MAR;*", false, false},    // wildcards are the reader's concern
		{"DUA0:[X]*.MAR", false, false},          // likewise
		{"DUA0:[X...]HELLO.MAR", false, false},   // likewise
		{"SRC:HELLO.MAR;1", false, false},        // version
	}

	for _, c := range cases {
		loc, err := s.Locate(c.name, c.host)
		if err != nil {
			t.Errorf("Locate(%q, %v) error = %v", c.name, c.host, err)

			continue
		}

		if loc.Host != c.wantHost || loc.Name != c.name {
			t.Errorf("Locate(%q, %v) = %+v, want Host=%v Name=%q", c.name, c.host, loc, c.wantHost, c.name)
		}
	}

	// VMS syntax that doesn't end on a mounted device is an error, never
	// a quiet fall back to the host.
	for _, name := range []string{"DUB0:[X]HELLO.MAR", "ELSEWHERE:HELLO.MAR", "NOSUCH:HELLO.MAR"} {
		var notMounted *NotMountedError

		if _, err := s.Locate(name, false); !errors.As(err, &notMounted) {
			t.Errorf("Locate(%q) error = %v, want a *NotMountedError", name, err)
		}
	}

	if _, err := s.Locate("[X]HELLO.MAR", false); err == nil {
		t.Error("Locate([X]HELLO.MAR) with no default device succeeded, want an error")
	}

	if _, err := s.Locate("", false); err == nil {
		t.Error("Locate(\"\") succeeded, want an error")
	}
}

// TestLocate_bareNameFollowsSetDefault covers rule 4: a bare name is on
// the volume only while a SET DEFAULT onto a mounted one is in effect.
func TestLocate_bareNameFollowsSetDefault(t *testing.T) {
	s := newLocateSession(t)

	if s.DefaultOnVolume() {
		t.Error("DefaultOnVolume() = true before any SET DEFAULT")
	}

	if err := s.SetDefault("DUB0:[X]"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	if loc, err := s.Locate("HELLO.MAR", false); err != nil || !loc.Host {
		t.Errorf("Locate(HELLO.MAR) with the default on an unmounted device = %+v, %v; want the host", loc, err)
	}

	if err := s.SetDefault("DUA0:[X]"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	if !s.DefaultOnVolume() {
		t.Error("DefaultOnVolume() = false after SET DEFAULT DUA0:[X]")
	}

	if loc, err := s.Locate("HELLO.MAR", false); err != nil || loc.Host {
		t.Errorf("Locate(HELLO.MAR) with the default on DUA0 = %+v, %v; want the volume", loc, err)
	}

	// With a default device, a directory alone is enough.
	if loc, err := s.Locate("[Y]HELLO.MAR", false); err != nil || loc.Host {
		t.Errorf("Locate([Y]HELLO.MAR) = %+v, %v; want the volume", loc, err)
	}

	// /HOST and host paths still win.
	if loc, _ := s.Locate("HELLO.MAR", true); !loc.Host {
		t.Error("Locate(HELLO.MAR, /HOST) went to the volume")
	}

	if loc, _ := s.Locate("./hello.mar", false); !loc.Host {
		t.Error("Locate(./hello.mar) went to the volume")
	}

	// A search list default counts when any element is mounted.
	defineLogical(t, s, "SYS$DISK", "DUB0:")
	if s.DefaultOnVolume() {
		t.Error("DefaultOnVolume() = true with SYS$DISK on an unmounted device")
	}

	if _, err := s.Logicals.Define(lnm.ProcessTableName, "SYS$DISK", lnm.Supervisor, 0,
		[]lnm.Equivalence{{Value: "DUB0:"}, {Value: "DUA0:"}}); err != nil {
		t.Fatalf("Define: %v", err)
	}

	if !s.DefaultOnVolume() {
		t.Error("DefaultOnVolume() = false with SYS$DISK a search list including DUA0")
	}
}

func TestLocateRelated(t *testing.T) {
	s := newLocateSession(t)

	hostSource := FileLocation{Host: true, Name: filepath.Join("src", "dir", "hello.mar")}
	volSource := FileLocation{Name: "DUA0:[X.Y]HELLO.MAR;3"}

	cases := []struct {
		name    string
		host    bool
		related FileLocation
		want    FileLocation
	}{
		// A bare name follows the related file.
		{"other.obj", false, hostSource, FileLocation{Host: true, Name: filepath.Join("src", "dir", "other.obj")}},
		{"OTHER.OBJ", false, volSource, FileLocation{Name: "DUA0:[X.Y]OTHER.OBJ"}},
		{"OTHER", false, volSource, FileLocation{Name: "DUA0:[X.Y]OTHER"}},
		// Anything else is Locate's.
		{"other.obj", true, volSource, FileLocation{Host: true, Name: "other.obj"}},
		{"./other.obj", false, volSource, FileLocation{Host: true, Name: "./other.obj"}},
		{"DUA0:[Z]OTHER.OBJ", false, hostSource, FileLocation{Name: "DUA0:[Z]OTHER.OBJ"}},
	}

	for _, c := range cases {
		got, err := s.LocateRelated(c.name, c.host, c.related)
		if err != nil {
			t.Errorf("LocateRelated(%q, %v, %v) error = %v", c.name, c.host, c.related, err)

			continue
		}

		if got != c.want {
			t.Errorf("LocateRelated(%q, %v, %v) = %+v, want %+v", c.name, c.host, c.related, got, c.want)
		}
	}
}
