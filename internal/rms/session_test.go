package rms

import "testing"

// TestSession_setDefaultAndShow regresses the plain SET DEFAULT/SHOW
// DEFAULT round trip: parsing a fully-qualified spec and rendering it back
// out should reproduce the same text.
func TestSession_setDefaultAndShow(t *testing.T) {
	s := NewSession(NewMountTable())

	if err := s.SetDefault("DUA0:[MYDIR]"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	if got, want := s.DefaultString(), "DUA0:[MYDIR]"; got != want {
		t.Errorf("DefaultString() = %q, want %q", got, want)
	}
}

// TestSession_setDefaultInheritsPreviousComponents checks filespec.Parse's
// own documented "unspecified components carry forward" rule, applied
// across two separate SetDefault calls the way an operator would type two
// separate SET DEFAULT commands in a row: SET DEFAULT DUA0: alone must not
// clear a directory a previous SET DEFAULT already established.
func TestSession_setDefaultInheritsPreviousComponents(t *testing.T) {
	s := NewSession(NewMountTable())

	if err := s.SetDefault("DUA0:[MYDIR]"); err != nil {
		t.Fatalf("first SetDefault: %v", err)
	}

	if err := s.SetDefault("DUB0:"); err != nil {
		t.Fatalf("second SetDefault: %v", err)
	}

	if got, want := s.DefaultString(), "DUB0:[MYDIR]"; got != want {
		t.Errorf("DefaultString() = %q, want %q (directory should carry forward)", got, want)
	}
}

// TestSession_setDefaultRelativeDirectory checks that a relative directory
// spec ("[.SUBDIR]") — one of filespec.Parse's documented relative forms —
// works the same way through Session as it does through filespec.Parse
// directly, since a real operator might well type "SET DEFAULT [.SUBDIR]"
// to descend into a subdirectory of wherever they currently are.
func TestSession_setDefaultRelativeDirectory(t *testing.T) {
	s := NewSession(NewMountTable())

	if err := s.SetDefault("DUA0:[MYDIR]"); err != nil {
		t.Fatalf("first SetDefault: %v", err)
	}

	if err := s.SetDefault("[.SUBDIR]"); err != nil {
		t.Fatalf("second SetDefault: %v", err)
	}

	if got, want := s.DefaultString(), "DUA0:[MYDIR.SUBDIR]"; got != want {
		t.Errorf("DefaultString() = %q, want %q", got, want)
	}
}

// TestSession_setDefaultBadSyntax checks that a malformed file
// specification (an unterminated directory bracket) is reported as an
// error rather than silently accepted, and that a failed SetDefault leaves
// the previous Default untouched — matching real SET DEFAULT, where a
// rejected command doesn't half-apply.
func TestSession_setDefaultBadSyntax(t *testing.T) {
	s := NewSession(NewMountTable())

	if err := s.SetDefault("DUA0:[MYDIR]"); err != nil {
		t.Fatalf("first SetDefault: %v", err)
	}

	if err := s.SetDefault("DUA0:[UNTERMINATED"); err == nil {
		t.Fatal("expected an error for an unterminated directory bracket")
	}

	if got, want := s.DefaultString(), "DUA0:[MYDIR]"; got != want {
		t.Errorf("DefaultString() = %q after a failed SetDefault, want unchanged %q", got, want)
	}
}

// TestSession_defaultStringInitialState checks that a brand-new Session
// (no SET DEFAULT run yet) still renders something displayable for SHOW
// DEFAULT, rather than panicking or returning an empty string that would
// look like a bug rather than genuine "nothing set" state.
func TestSession_defaultStringInitialState(t *testing.T) {
	s := NewSession(NewMountTable())

	if got, want := s.DefaultString(), "[000000]"; got != want {
		t.Errorf("DefaultString() on a fresh Session = %q, want %q", got, want)
	}
}

// TestSession_resolveVolume regresses the shared lookup helper every later
// docs/PHASE-23.md subtask's command (DIRECTORY, DELETE, PURGE, COPY, TYPE)
// will call: parsing a partial spec against Default and finding its
// device's mounted volume.
func TestSession_resolveVolume(t *testing.T) {
	mounts := NewMountTable()
	path := newTestVolumeFile(t, "MYVOL")

	if err := mounts.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	s := NewSession(mounts)
	if err := s.SetDefault("DUA0:[MYDIR]"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, spec, err := s.resolveVolume("FOO.TXT")
	if err != nil {
		t.Fatalf("resolveVolume: %v", err)
	}

	if vol == nil {
		t.Fatal("resolveVolume returned a nil *volume.Volume")
	}

	if spec.Device != "DUA0" || spec.Name != "FOO" || spec.Type != "TXT" {
		t.Errorf("resolveVolume spec = %+v, want Device=DUA0 Name=FOO Type=TXT", spec)
	}

	if len(spec.Dirs) != 1 || spec.Dirs[0] != "MYDIR" {
		t.Errorf("resolveVolume spec.Dirs = %v, want [MYDIR] (inherited from Default)", spec.Dirs)
	}
}

// TestSession_resolveVolumeNotMounted checks that resolveVolume reports a
// clear error (rather than a nil-pointer panic further down the line) when
// the resolved device has nothing mounted on it.
func TestSession_resolveVolumeNotMounted(t *testing.T) {
	s := NewSession(NewMountTable())

	if _, _, err := s.resolveVolume("DUA0:FOO.TXT"); err == nil {
		t.Fatal("expected an error for an unmounted device")
	}
}

// TestSession_resolveVolumeBadSyntax checks that a malformed spec is
// reported as an error rather than passed through to the Mounts lookup
// with garbage data.
func TestSession_resolveVolumeBadSyntax(t *testing.T) {
	s := NewSession(NewMountTable())

	if _, _, err := s.resolveVolume("DUA0:[UNTERMINATED"); err == nil {
		t.Fatal("expected an error for an unterminated directory bracket")
	}
}
