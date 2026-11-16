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
