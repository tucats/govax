package sysvol

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/security"
)

// TestProfiles checks Decision 9's profiles as text: files readable by
// everyone and writable by SYSTEM and their owner, directories the same
// without delete, and the owner [1,4].
func TestProfiles(t *testing.T) {
	for _, c := range []struct {
		p    Profile
		want string
	}{
		{FileProfile, "OWNER=[1,4] PROTECTION=(S:RWED,O:RWED,G:RE,W:RE)"},
		{DirectoryProfile, "OWNER=[1,4] PROTECTION=(S:RWE,O:RWE,G:RE,W:RE)"},
		{Profile{Owner: security.NewUIC(0o200, 1), Protection: security.NoAccess}, "OWNER=[200,1] PROTECTION=(S,O,G,W)"},
	} {
		if got := c.p.String(); got != c.want {
			t.Errorf("String() = %s, want %s", got, c.want)
		}
	}

	for dir, want := range map[string]string{
		Root:             "[000000]SYSTEM.DIR",
		SysLib:           "[SYSTEM]SYSLIB.DIR",
		DebugHelpSources: "[SYSTEM.SYSHLP.SOURCES]DEBUG.DIR",
	} {
		if got := DirectoryFile(dir); got != want {
			t.Errorf("DirectoryFile(%s) = %s, want %s", dir, got, want)
		}
	}
}

// TestManifestProfiles checks that a profile survives the manifest's round
// trip beside each other note, alone (a directory's record), and that a
// damaged profile is refused.
func TestManifestProfiles(t *testing.T) {
	sum := SumOf([]byte("x"))
	other := Profile{Owner: security.NewUIC(0o200, 1), Protection: FileProfile.Protection.With(security.World, 0)}

	m := NewManifest()
	m.Set(SysLib+"FTNRTL.EXE", sum)
	m.SetProfile(SysLib+"FTNRTL.EXE", FileProfile)
	m.SetSeeded(SysMgr + "SYLOGIN.COM")
	m.SetProfile(SysMgr+"SYLOGIN.COM", other)
	m.Set(SysMgr+"CONSOLE.DCL", sum)
	m.SetKept(SysMgr+"CONSOLE.DCL", sum)
	m.SetProfile(SysMgr+"CONSOLE.DCL", FileProfile)
	m.SetProfile(SysLib, DirectoryProfile)

	back, err := ParseManifest(m.Records())
	if err != nil {
		t.Fatal(err)
	}

	for spec, want := range map[string]Profile{
		SysLib + "FTNRTL.EXE":  FileProfile,
		SysMgr + "SYLOGIN.COM": other,
		SysMgr + "CONSOLE.DCL": FileProfile,
		SysLib:                 DirectoryProfile,
	} {
		if got, ok := back.Profile(spec); !ok || got != want {
			t.Errorf("%s: profile %v, %v after the round trip, want %v", spec, got, ok, want)
		}
	}

	if !back.Seeded(SysMgr+"SYLOGIN.COM") || back.Seeded(SysLib) {
		t.Error("SEEDED isn't where it was put")
	}

	if _, ok := back.Kept(SysMgr + "CONSOLE.DCL"); !ok {
		t.Error("the KEPT note was lost beside a profile")
	}

	if _, ok := back.Sum(SysLib); ok {
		t.Error("a directory's record has a digest")
	}

	good := sum.String()

	for _, record := range []string{
		SysLib + "X.EXE " + good + " OWNER=[1,4]",
		SysLib + "X.EXE " + good + " PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) OWNER=[1,4]",
		SysLib + "X.EXE " + good + " OWNER=[1,4 PROTECTION=(S:RWED,O:RWED,G:RE,W:RE)",
		SysLib + "X.EXE " + good + " OWNER=[1,4] PROTECTION=(S:RWXD)",
		SysLib + "X.EXE OWNER=[1,4] PROTECTION=(S:RWED) " + good,
	} {
		if _, err := ParseManifest([][]byte{[]byte(record)}); err == nil {
			t.Errorf("ParseManifest(%q) took it", record)
		}
	}
}

// TestPopulateProfiles checks that a new volume's every directory and file,
// and its manifest, have Decision 9's profiles, and that the manifest
// records them.
func TestPopulateProfiles(t *testing.T) {
	v := newMapVolume()

	m, err := Populate(v)
	if err != nil {
		t.Fatal(err)
	}

	for _, dir := range Directories {
		if got := v.dirProfiles[DirectoryFile(dir)]; got != DirectoryProfile {
			t.Errorf("%s: profile %v", dir, got)
		}

		if got, ok := m.Profile(dir); !ok || got != DirectoryProfile {
			t.Errorf("%s: the manifest records %v, %v", dir, got, ok)
		}
	}

	for _, f := range append(Files(), File{Directory: Root, Name: ManifestName}) {
		if got := v.profile(f.Spec()); got != FileProfile {
			t.Errorf("%s: profile %v", f.Spec(), got)
		}

		if got, ok := m.Profile(f.Spec()); !ok || got != FileProfile {
			t.Errorf("%s: the manifest records %v, %v", f.Spec(), got, ok)
		}
	}

	// A second refresh, with nothing changed, sets nothing and writes no
	// manifest.
	v.profileSets = 0

	report, err := Refresh(v)
	if err != nil {
		t.Fatal(err)
	}

	if v.profileSets != 0 || report.ManifestWritten {
		t.Errorf("a refresh of a current volume set %d profiles; manifest written %v", v.profileSets, report.ManifestWritten)
	}
}

// TestRefreshProfiles walks a volume made before profiles through the
// refreshes Decision 9 describes: corrected once, the user's SET SECURITY
// kept afterwards (and by a new version), a file made again given govax's
// profile, a deleted seeded file left alone, and the update log corrected
// when it is there.
func TestRefreshProfiles(t *testing.T) {
	r := newRefreshTest(t)
	r.expect("A.COM=written B.EXE=written SITE.COM=written", true)

	// A volume an older govax made: no profiles on the volume or in the
	// manifest.
	m, _, _ := ReadManifest(r.v)
	stripped := NewManifest()

	for _, spec := range m.Specs() {
		if sum, ok := m.Sum(spec); ok {
			stripped.Set(spec, sum)
		}

		if m.Seeded(spec) {
			stripped.SetSeeded(spec)
		}
	}

	for spec, versions := range r.v.files {
		for i := range versions {
			versions[i].profile = Profile{}
		}

		r.v.files[spec] = versions
	}

	r.v.dirProfiles = map[string]Profile{}
	_ = WriteManifest(r.v, stripped)
	_ = r.v.WriteFile(UpdateLogSpec, [][]byte{[]byte("a line")}, Text)

	r.expect("A.COM=current B.EXE=current SITE.COM=left", true)

	for _, spec := range []string{SysMgr + "A.COM", SysLib + "B.EXE", SysMgr + "SITE.COM", ManifestSpec, UpdateLogSpec} {
		if got := r.v.profile(spec); got != FileProfile {
			t.Errorf("after the correction, %s has %v", spec, got)
		}
	}

	if got := r.v.dirProfiles[DirectoryFile(SysMgr)]; got != DirectoryProfile {
		t.Errorf("after the correction, %s has %v", SysMgr, got)
	}

	// The user changes A.COM's protection (SET SECURITY): the next
	// refresh leaves it, and so does a newer govax copy, written as a new
	// version that takes the old one's.
	mine := Profile{Owner: SystemOwner, Protection: FileProfile.Protection.With(security.World, 0)}
	_ = r.v.SetProfile(SysMgr+"A.COM", mine)
	r.v.profileSets = 0

	r.expect("A.COM=current B.EXE=current SITE.COM=left", false)

	if r.v.profileSets != 0 {
		t.Errorf("a refresh set %d profiles on a corrected volume", r.v.profileSets)
	}

	r.copies["A.COM"] = "$ write sys$output \"a, again\"\n"
	r.expect("A.COM=updated B.EXE=current SITE.COM=left", true)

	if got := r.v.profile(SysMgr + "A.COM"); got != mine || r.versions(SysMgr, "A.COM") != 2 {
		t.Errorf("A.COM's new version has %v, want the user's %v", got, mine)
	}

	// B.EXE deleted and made again has govax's profile, not the zero one
	// a new file would have here; SITE.COM, the user's, deleted, stays
	// deleted and needs none.
	delete(r.v.files, SysLib+"B.EXE")
	delete(r.v.files, SysMgr+"SITE.COM")

	r.expect("A.COM=current B.EXE=written SITE.COM=left", false)

	if got := r.v.profile(SysLib + "B.EXE"); got != FileProfile {
		t.Errorf("B.EXE made again has %v", got)
	}

	// A later govax that wants another profile sets it once more, over
	// the user's.
	recorded, _, _ := ReadManifest(r.v)
	if got, _ := recorded.Profile(SysMgr + "A.COM"); got != FileProfile {
		t.Fatalf("the manifest records %v for A.COM", got)
	}

	changed := recorded
	changed.SetProfile(SysMgr+"A.COM", Profile{Owner: SystemOwner, Protection: security.NoAccess})
	_ = WriteManifest(r.v, changed)

	r.expect("A.COM=current B.EXE=current SITE.COM=left", true)

	if got := r.v.profile(SysMgr + "A.COM"); got != FileProfile {
		t.Errorf("A.COM after govax wanted another profile: %v", got)
	}

	if !strings.Contains(r.text(Root, ManifestName), SysMgr+"A.COM ") {
		t.Error("the manifest lost A.COM")
	}
}
