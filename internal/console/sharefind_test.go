package console

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// docs/PHASE-68 - link shareable images.md, subtask 10: RUN finds a
// program's shareable images as VMS's image activator does (Decision 3),
// checked against testdata/link68's runs R1 to R4. VMS's MAIN1 uses SHR1
// and LIBRTL; RUN/NOEXECUTE loads and fixes them up.

// shareFindConsole boots a console as cmd/govax does, paging on or off,
// with fresh volumes on DUA1 and DUA2, and VMS's MAIN1.EXE and SHR1.EXE in
// DUA1:[000000], the default directory.
func shareFindConsole(t *testing.T, paging bool) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	if paging {
		withSettings(t, map[string]string{machine.PagingSetting: "true"})
	}

	c, buf := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v", err)
	}

	if (c.Mem.Paging() != nil) != paging {
		t.Fatalf("paging is %v, want %v", c.Mem.Paging() != nil, paging)
	}

	mountFreshContainer(t, c, "DUA1:")
	mountFreshContainer(t, c, "DUA2:")

	for _, name := range []string{"main1", "shr1"} {
		shareFindCopy(t, d, filepath.Join(link68Vax, name+".exe"), "DUA1:[000000]"+strings.ToUpper(name)+".EXE")
	}

	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	buf.Reset()

	return d, c, buf
}

// shareFindCopy copies the host file host to the volume file spec, and
// checks that its bytes came through.
func shareFindCopy(t *testing.T, d *Dispatcher, host, spec string) {
	t.Helper()

	shareFindRun(t, d, `COPY "`+host+`"/HOST `+spec)

	want, err := os.ReadFile(host)
	if err != nil {
		t.Fatal(err)
	}

	got, _, err := d.Console.ContainerSession.ReadRawFile(rms.FileLocation{Name: spec})
	if err != nil || !bytes.Equal(got[:len(want)], want) {
		t.Fatalf("%s isn't %s's copy (%v)", spec, host, err)
	}
}

// shareFindRun dispatches line, which should succeed.
func shareFindRun(t *testing.T, d *Dispatcher, line string) {
	t.Helper()

	if err := d.Dispatch(line); err != nil {
		t.Fatalf("%s: %v", line, err)
	}
}

// shareFindRunMain runs MAIN1 with RUN/NOEXECUTE, and returns the file
// its SHR1 came from (SHOW IMAGE/FULL's), or the messages and $STATUS of
// a run that failed.
func shareFindRunMain(t *testing.T, d *Dispatcher, c *Console, buf *bytes.Buffer) string {
	t.Helper()

	buf.Reset()

	err := d.Dispatch("RUN/NOEXECUTE MAIN1")
	if err != nil {
		if !vmserrors.MessageInhibited(err) {
			t.Fatalf("RUN: %v", err)
		}

		out := buf.String()
		buf.Reset()
		shareFindRun(t, d, "SHOW SYMBOL $STATUS")

		return out + buf.String()
	}

	var file string

	for _, icb := range c.ImageState(nil).ICBList {
		if icb.Name == "SHR1" {
			file = icb.Spec.Name
		}

		if icb.Name == "LIBRTL" {
			t.Error("LIBRTL was loaded from a file; its shims stand for it")
		}
	}

	// The debugger's SHOW IMAGE/FULL shows the file.
	shown := false

	for _, img := range c.Images() {
		shown = shown || img.Name == "SHR1" && img.File == file
	}

	if file == "" || !shown {
		t.Errorf("SHR1's file %q isn't SHOW IMAGE's", file)
	}

	c.ImageRundown()
	c.ResetICBList()

	return file
}

// TestFindShareableImageAtRun runs MAIN1 with SHR1 found in each of the
// ways VMS 7.1 found it, and not found, in both paging modes (the
// copying and the mapping loader).
func TestFindShareableImageAtRun(t *testing.T) {
	for _, paging := range []bool{false, true} {
		t.Run(map[bool]string{false: "copying", true: "mapping"}[paging], func(t *testing.T) {
			d, c, buf := shareFindConsole(t, paging)

			// R1: a logical name of the image's name, the whole file.
			shareFindRun(t, d, "DEFINE SHR1 DUA1:[000000]SHR1.EXE")

			if got, want := shareFindRunMain(t, d, c, buf), "DUA1:[000000]SHR1.EXE;1"; got != want {
				t.Errorf("R1: SHR1 from %q, want %q", got, want)
			}

			// R2: a logical name with no device or type takes them from
			// SYS$SHARE's translation; DUA2:[000000] has no SHR1.EXE.
			shareFindRun(t, d, "DEFINE SYS$SHARE DUA2:[SYSLIB]")
			shareFindRun(t, d, "DEFINE SHR1 [000000]SHR1")

			const missing = "%DCL-W-ACTIMAGE, error activating image SHR1\n" +
				"-CLI-E-IMAGEFNF, image file not found DUA2:%sSHR1.EXE;\n" +
				"  $STATUS == \"%X100388B2\"\n"

			if got, want := shareFindRunMain(t, d, c, buf), strings.ReplaceAll(missing, "%s", "[000000]"); got != want {
				t.Errorf("R2: %q, want %q", got, want)
			}

			// R3: no logical name, and no SYS$SHARE:SHR1.EXE.
			shareFindRun(t, d, "DEASSIGN SHR1")

			if got, want := shareFindRunMain(t, d, c, buf), strings.ReplaceAll(missing, "%s", "[SYSLIB]"); got != want {
				t.Errorf("R3: %q, want %q", got, want)
			}

			// R4: SYS$SHARE a search list: SHR1.EXE in its first place,
			// then only in its second.
			shareFindRun(t, d, "CREATE/DIRECTORY DUA1:[SHRDIR]")
			shareFindRun(t, d, "CREATE/DIRECTORY DUA2:[SYSLIB]")
			shareFindCopy(t, d, filepath.Join(link68Vax, "shr1.exe"), "DUA1:[SHRDIR]SHR1.EXE")
			shareFindCopy(t, d, filepath.Join(link68Vax, "shr1.exe"), "DUA2:[SYSLIB]SHR1.EXE")
			shareFindRun(t, d, "DEFINE SYS$SHARE DUA1:[SHRDIR],DUA2:[SYSLIB]")

			if got, want := shareFindRunMain(t, d, c, buf), "DUA1:[SHRDIR]SHR1.EXE;1"; got != want {
				t.Errorf("R4: SHR1 from %q, want %q", got, want)
			}

			shareFindRun(t, d, "DEFINE SYS$SHARE DUA1:[NOSUCH],DUA2:[SYSLIB]")

			if got, want := shareFindRunMain(t, d, c, buf), "DUA2:[SYSLIB]SHR1.EXE;1"; got != want {
				t.Errorf("R4, its second place: SHR1 from %q, want %q", got, want)
			}

			// The host, when the volumes have none: the host library
			// directory, then SET SHARE_PREFIX's prefix before it.
			shareFindRun(t, d, "DEASSIGN SYS$SHARE")

			lib, prefix := t.TempDir(), t.TempDir()
			for _, dir := range []string{lib, prefix} {
				writeHostFile(t, filepath.Join(dir, "shr1.exe"), string(readFile(t, filepath.Join(link68Vax, "shr1.exe"))))
			}

			c.HostLibrary = lib

			// (A host file's name is matched as written, then in lower
			// case, so a file system that ignores case finds SHR1.exe.)
			if got, want := shareFindRunMain(t, d, c, buf), absPath(t, filepath.Join(lib, "shr1.exe")); !strings.EqualFold(got, want) {
				t.Errorf("host library: SHR1 from %q, want %q", got, want)
			}

			c.SharePrefix = prefix + string(filepath.Separator)

			if got, want := shareFindRunMain(t, d, c, buf), absPath(t, filepath.Join(prefix, "shr1.exe")); !strings.EqualFold(got, want) {
				t.Errorf("SET SHARE_PREFIX: SHR1 from %q, want %q", got, want)
			}
		})
	}
}

// TestFindShareableImageEmulated: govax's shims stand for LIBRTL (and
// for VMS's other run-time libraries) unless a logical name of its name
// is defined (question 1's answer); a user's image that is nowhere is
// an ImageNotFoundError with the specification RMS tried.
func TestFindShareableImageEmulated(t *testing.T) {
	d, c, _ := shareFindConsole(t, false)

	for _, name := range []string{"LIBRTL", "librtl", "MTHRTL", "DECC$SHR"} {
		if found, err := c.FindShareableImage(name); err != nil || !found.Emulated {
			t.Errorf("%s: %+v, %v; want govax's shims", name, found, err)
		}
	}

	shareFindRun(t, d, "DEFINE LIBRTL DUA1:[000000]SHR1.EXE")

	found, err := c.FindShareableImage("LIBRTL")
	if err != nil || found.Emulated || found.Location.Name != "DUA1:[000000]SHR1.EXE;1" {
		t.Errorf("LIBRTL with its logical name: %+v, %v", found, err)
	}

	_, err = c.FindShareableImage("NOSUCH")

	var notFound *rms.ImageNotFoundError
	if !errors.As(err, &notFound) || notFound.Image != "NOSUCH" || notFound.Spec != "SYS$SHARE:[000000]NOSUCH.EXE;" {
		t.Errorf("NOSUCH: %v", err)
	}

	if !errors.Is(err, vmserrors.New(vmserrors.RMS_IMAGENOTFOUND)) {
		t.Errorf("NOSUCH's error %v isn't RMS_IMAGENOTFOUND", err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func absPath(t *testing.T, path string) string {
	t.Helper()

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}

	return abs
}

// TestLinkImageLocator: LINK finds a shareable image as RUN does, but
// reads a file even for an image govax's shims stand for, and is told
// when there's none (openSharedImage then uses govax's own tables).
func TestLinkImageLocator(t *testing.T) {
	d, c, _ := shareFindConsole(t, false)

	shareFindRun(t, d, "CREATE/DIRECTORY DUA2:[SYSLIB]")
	shareFindCopy(t, d, filepath.Join(link68Vax, "shr1.exe"), "DUA2:[SYSLIB]SHR1.EXE")
	shareFindRun(t, d, "DEFINE SYS$SHARE DUA1:[NOSUCH],DUA2:[SYSLIB]")

	locator := c.cmd().ImageLocator()

	if found, err := locator.Find("SHR1"); err != nil || found.Location.Name != "DUA2:[SYSLIB]SHR1.EXE;1" {
		t.Errorf("SHR1: %+v, %v", found, err)
	}

	var notFound *rms.ImageNotFoundError
	if _, err := locator.Find("LIBRTL"); !errors.As(err, &notFound) || notFound.Spec != "DUA2:[SYSLIB]LIBRTL.EXE;" {
		t.Errorf("LIBRTL: %v, want its file not found", err)
	}

	shareFindRun(t, d, "DEFINE LIBRTL DUA2:[SYSLIB]SHR1.EXE")

	if found, err := locator.Find("LIBRTL"); err != nil || found.Location.Name != "DUA2:[SYSLIB]SHR1.EXE;1" {
		t.Errorf("LIBRTL through its logical name: %+v, %v", found, err)
	}
}
