package console

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/sysvol"
)

// The help libraries on the system volume (docs/PHASE-90 - VMS help.md,
// subtask 7): SYS$HELP:HELPLIB.HLB is DCL's HELP and SYS$HELP:DBG$HELP.HLB
// the debugger's, BUILDHELP.COM rebuilds them from their sources in
// SYS$HELP:[.SOURCES], and a newer govax refreshes them module by module.

// TestSystemHelpLibraries: BUILDHELP.COM builds both libraries as new
// versions in SYS$HELP, each byte for byte govax's built at LIBRARY's time;
// with a topic edited in its source, the library it builds is the one HELP
// reads.
func TestSystemHelpLibraries(t *testing.T) {
	c, run := libraryConsole(t, false)

	const built = "HELPLIB.HLB and DBG$HELP.HLB built in SYS$HELP\n"

	if out := run("@SYS$SOURCES:BUILDHELP"); !strings.HasSuffix(out, built) {
		t.Fatalf("BUILDHELP:\n%s", out)
	}

	for name, dir := range bootdata.HelpLibraries {
		spec := "SYS$HELP:" + bootdata.HelpLibraryName(name) + ";2"
		data := readVolume(t, c, spec)

		lib, err := lbr.Open(data)
		if err != nil {
			t.Fatalf("%s: %v", spec, err)
		}

		want, err := bootdata.BuildHelpAt(bootdata.FS, dir, lib.Created)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(data, want) {
			t.Errorf("%s (%d bytes) isn't govax's library built at its time (%d bytes)", spec, len(data), len(want))
		}
	}

	if out := run("HELP/NOPROMPT MOUNT"); !strings.Contains(out, "Mounts a container, a host file holding") {
		t.Errorf("HELP MOUNT:\n%s", out)
	}

	writeVolumeFile(t, c, sysvol.HelpSources+"MOUNT.HLP", sysvol.Canonical([]byte(
		"1 MOUNT\n  This system's own MOUNT help.\n"), sysvol.Text), sysvol.Text)

	if out := run("@SYS$SOURCES:BUILDHELP"); !strings.HasSuffix(out, built) {
		t.Fatalf("BUILDHELP of the edited source:\n%s", out)
	}

	if out := run("HELP/NOPROMPT MOUNT"); !strings.Contains(out, "This system's own MOUNT help.") {
		t.Errorf("HELP MOUNT after BUILDHELP:\n%s", out)
	}
}

// TestSystemHelpRefresh: a module the user inserted into HELPLIB.HLB, and
// one of govax's they replaced, survive a newer govax's refresh, which
// gives the library its own changed modules; the changed one is reported
// with SYSKEPT naming it, once.
func TestSystemHelpRefresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	c, _, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Created || !r.Writable {
		t.Fatalf("first start = %+v", r)
	}

	spec := sysvol.SysHlp + "HELPLIB.HLB"
	f, _ := sysvol.Lookup(spec)

	ours, err := f.Contents()
	if err != nil {
		t.Fatal(err)
	}

	// The user's library: a module of their own, and their own COPY. Then
	// an older govax's TYPE, which the manifest records as govax's.
	l, err := lbr.Open(ours)
	if err != nil {
		t.Fatal(err)
	}

	b, err := lbr.Edit(l)
	if err != nil {
		t.Fatal(err)
	}

	for _, text := range []string{"1 SITE\n  This site's help.", "1 COPY\n  My COPY.", "1 TYPE\n  An older TYPE."} {
		var lines [][]byte
		for _, line := range strings.Split(text, "\n") {
			lines = append(lines, []byte(line))
		}

		entries, bad := b.HelpModules(lines)
		if bad != nil {
			t.Fatal(bad)
		}

		if _, err := b.Replace(entries[0]); err != nil {
			t.Fatal(err)
		}
	}

	writeVolumeFile(t, c, spec, sysvol.Canonical(b.Bytes(), sysvol.Binary), sysvol.Binary)

	records := volumeText(t, c, sysvol.ManifestSpec, sysvol.Text)

	m, err := sysvol.ParseManifest(bytes.Split(records, []byte{'\n'}))
	if err != nil {
		t.Fatal(err)
	}

	m.Set(spec+"(TYPE)", moduleDigest("1 TYPE", "  An older TYPE."))
	writeVolumeFile(t, c, sysvol.ManifestSpec, bytes.Join(append(m.Records(), nil), []byte{'\n'}), sysvol.Text)

	if err := c.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	// The next start: govax's TYPE is back, the user's COPY and SITE stay,
	// and COPY isn't said (govax's COPY hasn't changed since it wrote it).
	c, buf, _ := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if strings.Contains(buf.String(), "SYSKEPT") {
		t.Errorf("COPY, unchanged by govax, was reported:\n%s", buf)
	}

	got := helpModules(t, volumeText(t, c, spec, sysvol.Binary))
	if !strings.Contains(got["SITE"], "This site's help.") || !strings.Contains(got["COPY"], "My COPY.") {
		t.Errorf("the user's modules are gone: SITE %q, COPY %q", got["SITE"], got["COPY"])
	}

	if strings.Contains(got["TYPE"], "An older TYPE.") || !strings.Contains(got["TYPE"], "Writes a file to the terminal") {
		t.Errorf("TYPE wasn't refreshed: %q", got["TYPE"])
	}

	// A govax whose COPY is newer than the one it wrote: the user's COPY is
	// kept, and said once.
	records = volumeText(t, c, sysvol.ManifestSpec, sysvol.Text)

	m, err = sysvol.ParseManifest(bytes.Split(records, []byte{'\n'}))
	if err != nil {
		t.Fatal(err)
	}

	m.Set(spec+"(COPY)", moduleDigest("1 COPY", "  An older govax's COPY."))
	writeVolumeFile(t, c, sysvol.ManifestSpec, bytes.Join(append(m.Records(), nil), []byte{'\n'}), sysvol.Text)

	if err := c.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	_, buf, _ = mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})

	want := "%MOUNT-I-SYSKEPT, DUA0:" + spec + "(COPY) was changed, govax's new version not installed\n"
	if !strings.Contains(buf.String(), want) || strings.Count(buf.String(), "SYSKEPT") != 1 {
		t.Errorf("output lacks %q once:\n%s", want, buf)
	}

	_, buf, _ = mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if strings.Contains(buf.String(), "SYSKEPT") {
		t.Errorf("COPY was reported again:\n%s", buf)
	}
}

// moduleDigest is the manifest's digest of a module of these lines.
func moduleDigest(lines ...string) sysvol.Sum {
	return sysvol.SumOf([]byte(strings.Join(lines, "\n") + "\n"))
}

// helpModules is each module of a help library, its records joined.
func helpModules(t *testing.T, data []byte) map[string]string {
	t.Helper()

	l, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	out := map[string]string{}

	for _, k := range l.Indexes[0].Keys {
		m, err := l.Module(k.RFA)
		if err != nil {
			t.Fatal(err)
		}

		out[k.Name] = string(bytes.Join(m.Records, []byte{'\n'}))
	}

	return out
}
