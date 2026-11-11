package console

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/vmserrors"
)

// The page file on the system volume (docs/planning/PHASE-87 - system volume.md,
// subtask 7; Decision 8): with no vax.sysgen.paging.file, a writable
// system volume holds the page file, [SYSTEM.SYSEXE]PAGEFILE.SYS, made at
// the first start and used again by the next; a read-only one (another
// session holds it) holds none, nothing tries to make one, and SHOW
// MEMORY says why.

// systemPageFileSize is the page file's size in these tests: the test
// volumes are RD51s, too small for the default 32,768 blocks.
const systemPageFileSize = 500

// bootOnSystemVolume mounts the system volume at path on a new console,
// with paging on or off and vax.sysgen.paging.file pageFile ("" for
// none), runs its STARTUP.COM,
// and installs the page file, as cmd/govax's start does. It returns the
// console, its output, and a routine that dispatches a line and returns
// what it wrote.
func bootOnSystemVolume(t *testing.T, path string, paging bool, pageFile string) (*Console, *bytes.Buffer, func(string) string) {
	t.Helper()

	withSettings(t, map[string]string{
		machine.PagingSetting:     map[bool]string{true: "true", false: "false"}[paging],
		"vax.sysgen.paging.file":  pageFile,
		"vax.sysgen.paging.size":  "500",
		"vax.sysgen.paging.defer": "false",
	})

	c, buf, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Mounted {
		t.Fatalf("system volume not mounted: %+v\n%s", r, buf)
	}

	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t))
	installPlainDebugger(c)

	if err := c.RunStartup(d.Dispatch); err != nil {
		t.Fatalf("RunStartup: %v\n%s", err, buf)
	}

	c.StartPageFile()
	t.Cleanup(c.EndSession)

	return c, buf, func(line string) string {
		t.Helper()

		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		return buf.String()
	}
}

// TestSystemPageFile: with paging on, the page file is made on the system
// volume at the first start, and used again, not made again, by the next
// session; SHOW MEMORY and SHOW MEMORY/FILES name it.
func TestSystemPageFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	c, buf, dispatch := bootOnSystemVolume(t, path, true, "")

	pf := c.RTL.PageFile()
	if pf == nil {
		t.Fatalf("no page file on the system volume:\n%s", buf)
	}

	if pf.Name() != "DUA0:[SYSTEM.SYSEXE]PAGEFILE.SYS;1" || !pf.Created() || pf.Size() != systemPageFileSize {
		t.Errorf("page file %s, created %v, %d blocks", pf.Name(), pf.Created(), pf.Size())
	}

	if strings.Contains(buf.String(), "NOPAGEFILE") {
		t.Errorf("output:\n%s", buf)
	}

	if got := dispatch("SHOW MEMORY"); !strings.Contains(got, "    Page file: DUA0:[SYSTEM.SYSEXE]PAGEFILE.SYS;1\n") {
		t.Errorf("SHOW MEMORY:\n%s", got)
	}

	if got := dispatch("SHOW MEMORY/FILES"); !strings.Contains(got, "  DISK$GOVAXSYS:[SYSTEM.SYSEXE]PAGEFILE.SYS ") {
		t.Errorf("SHOW MEMORY/FILES:\n%s", got)
	}

	c.EndSession()

	if err := c.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	// The next session finds it there.
	c, buf, dispatch = bootOnSystemVolume(t, path, true, "")

	if pf = c.RTL.PageFile(); pf == nil || pf.Name() != "DUA0:[SYSTEM.SYSEXE]PAGEFILE.SYS;1" || pf.Created() {
		t.Errorf("second session: %+v\n%s", pf, buf)
	}

	if got := dispatch("DIRECTORY SYS$SYSTEM:PAGEFILE.SYS"); strings.Count(got, "PAGEFILE.SYS;") != 1 {
		t.Errorf("DIRECTORY:\n%s", got)
	}
}

// TestSystemPageFilePagingOff: with paging off, nothing is made.
func TestSystemPageFilePagingOff(t *testing.T) {
	c, buf, dispatch := bootOnSystemVolume(t, filepath.Join(t.TempDir(), "govax-system.dsk"), false, "")

	if c.RTL.PageFile() != nil || c.RTL.OpenPageFile() != nil {
		t.Errorf("a page file with paging off:\n%s", buf)
	}

	if got := dispatch("DIRECTORY SYS$SYSTEM:PAGEFILE.SYS"); !strings.Contains(got, "%DIRECT-W-NOFILES") {
		t.Errorf("DIRECTORY:\n%s", got)
	}
}

// TestSystemPageFileReadOnly: a system volume another session holds is
// mounted read-only, and has no page file: none installed, none opened at
// the pager's need, no message about it, and SHOW MEMORY says why.
func TestSystemPageFileReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")

	// The first session holds the volume.
	if _, _, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path}); !r.Writable {
		t.Fatalf("first session = %+v", r)
	}

	c, buf, dispatch := bootOnSystemVolume(t, path, true, "")
	if c.Mounts.Writable("DUA0") {
		t.Fatal("the second session's system volume is writable")
	}

	if c.RTL.PageFile() != nil || c.RTL.OpenPageFile() != nil {
		t.Error("a page file on a read-only system volume")
	}

	if strings.Contains(buf.String(), "PAGEFILE") {
		t.Errorf("output:\n%s", buf)
	}

	want := "    Page file: none\n        DUA0:[SYSTEM.SYSEXE]PAGEFILE.SYS not used: the system volume is read-only\n"
	if got := dispatch("SHOW MEMORY"); !strings.Contains(got, want) {
		t.Errorf("SHOW MEMORY lacks\n%s\n%s", want, got)
	}

	if got := dispatch("SHOW MEMORY/FILES"); strings.Contains(got, "PAGEFILE") {
		t.Errorf("SHOW MEMORY/FILES:\n%s", got)
	}

	if got := dispatch(`WRITE SYS$OUTPUT F$GETSYI("PAGEFILE_PAGE")`); got != "0\n" {
		t.Errorf("PAGEFILE_PAGE = %q", got)
	}
}

// TestSystemPageFileConfigured: a page file vax.sysgen.paging.file names
// is used in place of the system volume's.
func TestSystemPageFileConfigured(t *testing.T) {
	host := filepath.Join(t.TempDir(), "pagefile.sys")

	c, _, _ := bootOnSystemVolume(t, filepath.Join(t.TempDir(), "govax-system.dsk"), true, `"`+host+`"`)

	if pf := c.RTL.PageFile(); pf == nil || pf.Name() != host {
		t.Errorf("page file %+v, want %s", pf, host)
	}
}
