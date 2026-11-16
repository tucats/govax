package console

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/sysvol"
	"github.com/tucats/govax/internal/uaf"
)

// The system volume's protection (docs/planning/PHASE-95 -
// authentication.md, Decision 9 and subtask 7).

// volumeProfile returns the owner and protection of the highest version of
// spec on the console's volumes.
func volumeProfile(t *testing.T, c *Console, spec string) sysvol.Profile {
	t.Helper()

	objects, err := c.ContainerSession.SecurityObjects(spec)
	if err != nil || len(objects) != 1 {
		t.Fatalf("%s: %d objects, %v", spec, len(objects), err)
	}

	p := objects[0].Profile()

	return sysvol.Profile{Owner: p.Owner, Protection: p.Protection}
}

// checkSystemProfiles checks that every directory and file of the system
// volume on DUA0, and its manifest, have Decision 9's profiles.
func checkSystemProfiles(t *testing.T, c *Console) {
	t.Helper()

	for _, dir := range sysvol.Directories {
		if got := volumeProfile(t, c, "DUA0:"+sysvol.DirectoryFile(dir)); got != sysvol.DirectoryProfile {
			t.Errorf("%s: %v, want %v", dir, got, sysvol.DirectoryProfile)
		}
	}

	for _, f := range append(sysvol.Files(), sysvol.File{Directory: sysvol.Root, Name: sysvol.ManifestName}) {
		if got := volumeProfile(t, c, "DUA0:"+f.Spec()); got != sysvol.FileProfile {
			t.Errorf("%s: %v, want %v", f.Spec(), got, sysvol.FileProfile)
		}
	}
}

// TestSystemVolumeProtection checks a new system volume's profiles, and
// that DIRECTORY/SECURITY shows them as VMS words them.
func TestSystemVolumeProtection(t *testing.T) {
	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(t.TempDir(), "sys.dsk")})
	if !r.Mounted || !r.Created {
		t.Fatalf("result = %+v; output:\n%s", r, buf)
	}

	checkSystemProfiles(t, c)

	buf.Reset()

	if err := c.cmd().Directory("DUA0:[SYSTEM.SYSMGR]SYLOGIN.COM", rms.DirectoryOptions{Security: true}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "[SYSTEM]") || !strings.Contains(buf.String(), "(RWED,RWED,RE,RE)") {
		t.Errorf("DIRECTORY/SECURITY:\n%s", buf)
	}
}

// TestSystemVolumeProtectionCorrected checks an existing volume an older
// govax made, with no profiles in its manifest and its files' protection
// the creator's default: corrected once at the next start; a protection
// the user then changes is kept at the start after.
func TestSystemVolumeProtectionCorrected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sys.dsk")

	first, _, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})

	// An older govax's volume: its files (S:RWED,O:RWED,G:RE,W), its
	// manifest without profiles.
	old := sysvol.Profile{Owner: sysvol.SystemOwner, Protection: sysvol.FileProfile.Protection.With(security.World, 0)}
	setVolumeProfile(t, first, "DUA0:"+sysvol.SysMgr+"STARTUP.COM;*", old)
	setVolumeProfile(t, first, "DUA0:"+sysvol.SysLib+"FTNRTL.EXE;*", old)
	setVolumeProfile(t, first, "DUA0:"+sysvol.DirectoryFile(sysvol.SysLib), old)

	loc := rms.FileLocation{Name: "DUA0:" + sysvol.ManifestSpec}

	records, _, err := first.ContainerSession.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	profileWords := regexp.MustCompile(` OWNER=\S+ PROTECTION=\S+$`)

	var stripped [][]byte

	for _, r := range records {
		line := profileWords.ReplaceAllString(string(r), "")
		if strings.Contains(line, " ") || strings.HasPrefix(line, "!") { // not one with nothing but its profile
			stripped = append(stripped, []byte(line))
		}
	}

	if _, err := first.ContainerSession.CreateRecordFile(loc, rms.TextRecords, stripped); err != nil {
		t.Fatal(err)
	}

	if err := first.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	second, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Mounted || r.Created || strings.Contains(buf.String(), "-W-") {
		t.Fatalf("second start = %+v:\n%s", r, buf)
	}

	checkSystemProfiles(t, second)

	// The user takes WORLD's access from STARTUP.COM: kept from now on.
	setVolumeProfile(t, second, "DUA0:"+sysvol.SysMgr+"STARTUP.COM;*", old)

	if err := second.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	third, _, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})

	if got := volumeProfile(t, third, "DUA0:"+sysvol.SysMgr+"STARTUP.COM"); got != old {
		t.Errorf("the user's protection on STARTUP.COM became %v", got)
	}

	if got := volumeProfile(t, third, "DUA0:"+sysvol.SysLib+"FTNRTL.EXE"); got != sysvol.FileProfile {
		t.Errorf("FTNRTL.EXE: %v", got)
	}
}

// setVolumeProfile gives the files spec names a profile, as SET SECURITY
// would.
func setVolumeProfile(t *testing.T, c *Console, spec string, p sysvol.Profile) {
	t.Helper()

	objects, err := c.ContainerSession.SecurityObjects(spec)
	if err != nil {
		t.Fatalf("%s: %v", spec, err)
	}

	for _, o := range objects {
		if err := o.SetOwnerProtection(p.Owner, p.Protection); err != nil {
			t.Fatalf("%s: %v", o.Name, err)
		}
	}
}

// TestSystemVolumeAsUser checks the system volume as an account without
// privilege sees it: it can read the system's files (SYLOGIN.COM, which
// it must to run it at login) but not delete, change, or add to them.
func TestSystemVolumeAsUser(t *testing.T) {
	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: filepath.Join(t.TempDir(), "sys.dsk")})
	if !r.Mounted {
		t.Fatalf("result = %+v; output:\n%s", r, buf)
	}

	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t))
	installPlainDebugger(c)

	if err := c.RunHostProcedure("startup.com", d.Dispatch); err != nil {
		t.Fatalf("STARTUP.COM: %v", err)
	}

	err := c.RTL.System.UpdateAuthorization(func(f *uaf.File) error {
		rec := uaf.DefaultRecord()
		rec.Username, rec.Flags, rec.UIC = "SMITH", 0, uaf.NewUIC(0o300, 1)
		f.Put(rec)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.LogIn("SMITH", ""); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	mustDispatch(t, d, "SHOW LOGICAL SYS$SYLOGIN", "TYPE SYS$MANAGER:SYLOGIN.COM")

	out := buf.String()
	if !strings.Contains(out, `"SYS$SYLOGIN" = "SYS$MANAGER:SYLOGIN" (LNM$SYSTEM_TABLE)`) ||
		!strings.Contains(out, "SYLOGIN.COM - the system's login procedure") {
		t.Errorf("SYS$SYLOGIN as SMITH:\n%s", out)
	}

	for _, line := range []string{
		"DELETE SYS$SYSTEM:DEFAULT.ROM;*",
		"CREATE SYS$MANAGER:MINE.COM",
		"SET SECURITY/PROTECTION=(W:RWED) SYS$MANAGER:STARTUP.COM",
		"COPY SYS$MANAGER:STARTUP.COM SYS$MANAGER:STARTUP.COM",
	} {
		buf.Reset()

		said := ""
		if err := d.Dispatch(line); err != nil {
			said = err.Error() + "\n"
		}

		said += buf.String()
		t.Logf("%s:\n%s", line, said)

		if !strings.Contains(said, "-NOPRIV") && !strings.Contains(said, "-PRV") {
			t.Errorf("%s as SMITH:\n%s", line, said)
		}
	}

	// Nothing changed.
	if got := volumeProfile(t, c, "DUA0:"+sysvol.SysMgr+"STARTUP.COM"); got != sysvol.FileProfile {
		t.Errorf("STARTUP.COM: %v", got)
	}

	if names, err := c.ContainerSession.Search("DUA0:" + sysvol.SysExe + "DEFAULT.ROM;*"); err != nil || len(names) != 1 {
		t.Errorf("DEFAULT.ROM: %v, %v", names, err)
	}
}
