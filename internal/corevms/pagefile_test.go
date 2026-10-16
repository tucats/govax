package corevms

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/rms"
)

// pagingFixture is fixture with paging on and the page file named spec,
// size blocks.
func pagingFixture(spec string, size uint32) (*Environment, *bytes.Buffer) {
	env, out := fixture()
	env.Sysgen.Paging = true
	env.Sysgen.PageFileName = spec
	env.Sysgen.PageFileSize = size

	return env, out
}

// TestPageFileNotWanted: with paging off, or no page file named, there's
// none, and nothing is said.
func TestPageFileNotWanted(t *testing.T) {
	env, out := fixture()
	env.Sysgen.PageFileName = `"` + filepath.Join(t.TempDir(), "pagefile.sys") + `"`

	if env.OpenPageFile() != nil {
		t.Error("a page file opened with paging off")
	}

	env, out2 := pagingFixture("", 0)

	if env.OpenPageFile() != nil {
		t.Error("a page file opened with none named")
	}

	if out.Len()+out2.Len() != 0 {
		t.Errorf("console output %q %q, want none", out, out2)
	}
}

// TestPageFileOpenedOnceAtFirstNeed: the page file isn't open until it's
// asked for, is then created at its size, and the same one is returned
// after that.
func TestPageFileOpenedOnceAtFirstNeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pagefile.sys")
	env, out := pagingFixture(`"`+path+`"`, 0) // the default size

	if env.PageFile() != nil {
		t.Fatal("the page file is open before it's asked for")
	}

	pf := env.OpenPageFile()
	if pf == nil {
		t.Fatalf("OpenPageFile failed: %q", out)
	}

	if pf.Name() != path || !pf.Created() || pf.Size() != DefaultPageFileSize || pf.Map.Size() != DefaultPageFileSize {
		t.Errorf("page file %s, created %v, %d blocks (map %d); want a new one of %d",
			pf.Name(), pf.Created(), pf.Size(), pf.Map.Size(), DefaultPageFileSize)
	}

	if pf.Map.AllocationSize() != DefaultMPWWriteCluster {
		t.Errorf("allocation size %d, want MPW_WRTCLUSTER's %d", pf.Map.AllocationSize(), DefaultMPWWriteCluster)
	}

	if env.OpenPageFile() != pf || env.PageFile() != pf {
		t.Error("asked again, a different page file")
	}

	if err := env.ClosePageFile(); err != nil {
		t.Fatal(err)
	}

	if env.PageFile() != nil || env.OpenPageFile() != nil {
		t.Error("a closed System's page file came back")
	}

	if out.Len() != 0 {
		t.Errorf("console output %q, want none", out)
	}
}

// TestPageFileCantBeOpened: a page file that can't be opened is reported
// once, and paging goes on without one.
func TestPageFileCantBeOpened(t *testing.T) {
	env, out := pagingFixture("DUA0:[SYSEXE]PAGEFILE.SYS", 100) // nothing mounted

	for range 2 {
		if env.OpenPageFile() != nil {
			t.Fatal("a page file on an unmounted device opened")
		}
	}

	if got := strings.Count(out.String(), "%GOVAX-W-NOPAGEFILE, page file DUA0:[SYSEXE]PAGEFILE.SYS not opened"); got != 1 {
		t.Errorf("console output %q: want one NOPAGEFILE message", out)
	}

	if _, ok := env.AllocatePageFile(1); ok {
		t.Error("blocks allocated with no page file")
	}

	if err := env.FreePageFile(paging.Run{First: 1, Count: 1}); err == nil {
		t.Error("blocks freed with no page file")
	}
}

// TestPageFileWarnings: allocation passing 65% and 90% of the file tells
// the console PAGEFRAG and PAGECRIT, once each; blocks freed can be
// allocated again.
func TestPageFileWarnings(t *testing.T) {
	env, out := pagingFixture(`"`+filepath.Join(t.TempDir(), "pagefile.sys")+`"`, 100)
	if env.OpenPageFile() == nil {
		t.Fatalf("OpenPageFile failed: %q", out)
	}

	var runs []paging.Run

	for {
		run, ok := env.AllocatePageFile(16)
		if !ok {
			break
		}

		runs = append(runs, run)
	}

	for _, run := range runs {
		if err := env.FreePageFile(run); err != nil {
			t.Fatal(err)
		}
	}

	if _, ok := env.AllocatePageFile(100); !ok {
		t.Error("nothing could be allocated after everything was freed")
	}

	want := "%SYSTEM-W-PAGEFRAG, page file filling up; please create more space\n" +
		"%SYSTEM-W-PAGECRIT, page file nearly full; system trying to continue\n"
	if out.String() != want {
		t.Errorf("console output %q, want %q", out, want)
	}

	if err := env.PageFile().Map.Check(); err != nil {
		t.Error(err)
	}
}

// TestPageFileOnVolume: a volume page file is created on the volume, read
// and written there, held so that the volume can't be dismounted, and
// let go when the System closes it.
func TestPageFileOnVolume(t *testing.T) {
	env, out := pagingFixture("DUA0:[000000]PAGEFILE.SYS", 64)
	newMountedVolumeFixture(t, env)

	pf := env.OpenPageFile()
	if pf == nil {
		t.Fatalf("OpenPageFile failed: %q", out)
	}

	if pf.Name() != "DUA0:[000000]PAGEFILE.SYS;1" || pf.Size() < 64 {
		t.Errorf("page file %s of %d blocks, want DUA0:[000000]PAGEFILE.SYS;1 of at least 64", pf.Name(), pf.Size())
	}

	page := bytes.Repeat([]byte{0x5A}, 512)
	if err := pf.Write(7, page); err != nil {
		t.Fatal(err)
	}

	got := make([]byte, 512)
	if err := pf.Read(7, got); err != nil || !bytes.Equal(got, page) {
		t.Errorf("block 7 read back %v", err)
	}

	if err := pf.Write(pf.Size()+1, page); !errors.Is(err, rms.ErrACPEndOfFile) {
		t.Errorf("a write past the end = %v, want ErrACPEndOfFile", err)
	}

	if err := env.Mounts.Dismount("DUA0"); !errors.Is(err, rms.ErrFilesOpen) {
		t.Errorf("Dismount with the page file open = %v, want ErrFilesOpen", err)
	}

	if err := env.ClosePageFile(); err != nil {
		t.Fatal(err)
	}

	if err := env.Mounts.Dismount("DUA0"); err != nil {
		t.Errorf("Dismount after ClosePageFile = %v", err)
	}
}
