package rms

import (
	"errors"
	"testing"
)

// TestLocateHostAccess holds Locate to Decision 4 of
// docs/planning/PHASE-97 - host file access.md: a name placed on the host
// by any of the rules (host syntax, /HOST, a bare name with the default
// directory on the host) is refused when the session's HostAccess says
// no, as a *DeniedError with Host set, which IsDenied takes for a
// protection refusal; Side answers the same names without asking; and a
// session with no HostAccess, or one that allows, is as before.
func TestLocateHostAccess(t *testing.T) {
	s := NewSession(NewMountTable())

	cases := []struct {
		name string
		host bool
	}{
		{"/tmp/notes.txt", false}, // host syntax
		{"NOTES.TXT", true},       // /HOST
		{"NOTES.TXT", false},      // bare, no default volume
	}

	for _, c := range cases {
		if loc, err := s.Locate(c.name, c.host); err != nil || !loc.Host {
			t.Errorf("no HostAccess, %q /HOST=%v: %+v, %v", c.name, c.host, loc, err)
		}
	}

	allowed := false
	s.HostAccess = func() bool { return allowed }

	for _, c := range cases {
		_, err := s.Locate(c.name, c.host)

		var denied *DeniedError
		if !errors.As(err, &denied) || !denied.Host || denied.Name != c.name {
			t.Errorf("refused, %q /HOST=%v: %v, want a host DeniedError", c.name, c.host, err)
		}

		if d, ok := IsDenied(err); !ok || d == nil {
			t.Errorf("refused, %q: IsDenied %v, %v", c.name, d, ok)
		}

		if loc, err := s.Side(c.name, c.host); err != nil || !loc.Host || loc.Name != c.name {
			t.Errorf("Side, %q /HOST=%v: %+v, %v", c.name, c.host, loc, err)
		}
	}

	allowed = true

	for _, c := range cases {
		if loc, err := s.Locate(c.name, c.host); err != nil || !loc.Host {
			t.Errorf("allowed, %q /HOST=%v: %+v, %v", c.name, c.host, loc, err)
		}
	}
}

// TestLocateVolumeNotAsked: a name on a volume never asks HostAccess, so
// the key can't change anything but host files.
func TestLocateVolumeNotAsked(t *testing.T) {
	s := NewSession(NewMountTable())
	s.HostAccess = func() bool {
		t.Error("HostAccess asked about a volume name")

		return false
	}

	// Not mounted: Locate's own error, not a refusal.
	_, err := s.Locate("DUA5:[WORK]NOTES.TXT", false)
	if _, denied := IsDenied(err); err == nil || denied {
		t.Errorf("volume name: %v", err)
	}
}

// TestLocateOutputHostAccess: LocateOutput's refusal is marked as an
// output, so a command shows OPENOUT, and a volume name or an allowed
// host name is Locate's.
func TestLocateOutputHostAccess(t *testing.T) {
	s := NewSession(NewMountTable())
	s.HostAccess = func() bool { return false }

	if s.MayUseHost() {
		t.Error("MayUseHost with HostAccess refusing")
	}

	_, err := s.LocateOutput("/tmp/out.txt", false)

	var denied *DeniedError
	if !errors.As(err, &denied) || !denied.Host || !denied.Output {
		t.Errorf("LocateOutput: %v, want a host output DeniedError", err)
	}

	s.HostAccess = nil

	if !s.MayUseHost() {
		t.Error("MayUseHost refused with no HostAccess")
	}

	if loc, err := s.LocateOutput("/tmp/out.txt", false); err != nil || !loc.Host {
		t.Errorf("allowed: %+v, %v", loc, err)
	}
}

// TestLocateRelatedHostAccess: a bare name beside a host file is a host
// file too, refused like any other, where it was once returned unasked.
func TestLocateRelatedHostAccess(t *testing.T) {
	s := NewSession(NewMountTable())
	s.HostAccess = func() bool { return false }

	_, err := s.LocateRelated("B.OPT", false, FileLocation{Host: true, Name: "/tmp/src/a.obj"})
	if d, ok := IsDenied(err); !ok || d == nil || !d.Host || d.Name != "/tmp/src/B.OPT" {
		t.Errorf("LocateRelated: %v, want a host DeniedError for /tmp/src/B.OPT", err)
	}
}

// TestLocateLogicalToHost: a logical name that translates to a host path
// is refused by Locate as that path, and Side places it there unasked.
// (Before, the translation went through Locate, whose refusal made the
// name a bare name instead, placed by the default directory's side.)
func TestLocateLogicalToHost(t *testing.T) {
	s := newLocateSession(t)
	defineLogical(t, s, "NOTES", "/tmp/notes.txt")

	asked := 0
	s.HostAccess = func() bool { asked++; return false }

	if loc, err := s.Side("NOTES", false); err != nil || !loc.Host || loc.Name != "/tmp/notes.txt" || asked != 0 {
		t.Errorf("Side: %+v, %v (asked %d)", loc, err, asked)
	}

	_, err := s.Locate("NOTES", false)
	if d, ok := IsDenied(err); !ok || d == nil || d.Name != "/tmp/notes.txt" {
		t.Errorf("Locate: %v, want a refusal of /tmp/notes.txt", err)
	}
}
