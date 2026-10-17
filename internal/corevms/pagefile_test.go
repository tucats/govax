package corevms

import (
	"bytes"
	"errors"
	"path/filepath"
	"strconv"
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

// TestPageFileReport: the report says whether there's a page file and,
// once it's open, its blocks' use and thresholds; committed is every
// job's charges, counted once a job.
func TestPageFileReport(t *testing.T) {
	env, _ := fixture()
	if r := env.PageFileReport(); r.Status != PageFileNone {
		t.Errorf("paging off: status %d, want none", r.Status)
	}

	path := filepath.Join(t.TempDir(), "pagefile.sys")
	env, out := pagingFixture(`"`+path+`"`, 100)

	if r := env.PageFileReport(); r.Status != PageFileNotOpened || r.Name != `"`+path+`"` || r.Size != 0 {
		t.Errorf("before opening: %+v, want not opened, the configured name", r)
	}

	if env.PageFile() != nil {
		t.Fatal("the report opened the page file")
	}

	if env.OpenPageFile() == nil {
		t.Fatalf("OpenPageFile failed: %q", out)
	}

	for range 7 {
		if _, ok := env.AllocatePageFile(10); !ok {
			t.Fatal("allocation failed")
		}
	}

	env.Process.Job.PageFileCharged = 12

	r := env.PageFileReport()
	if r.Status != PageFileInUse || r.Name != path || r.Size != 100 || r.Used != 70 || r.Free != 30 ||
		r.Warned != paging.PageFrag || r.Full || r.Charged != 12 {
		t.Errorf("in use: %+v, want 70 of 100 blocks, PAGEFRAG, 12 committed", r)
	}

	env.notePageFileFull()

	if r := env.PageFileReport(); !r.Full {
		t.Error("the writer found it full, but the report doesn't say so")
	}

	if err := env.ClosePageFile(); err != nil {
		t.Fatal(err)
	}

	if r := env.PageFileReport(); r.Status != PageFileUnavailable {
		t.Errorf("closed: status %d, want unavailable", r.Status)
	}
}

// TestSYIPageFileItems: $GETSYI's page file items open the page file
// and report its size and free blocks; with none they're 0, as the swap
// file's always are.
func TestSYIPageFileItems(t *testing.T) {
	long := func(v uint32) string { return string([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}) }

	item := func(env *Environment, name string, want uint32) {
		t.Helper()

		if got, ok := env.SYIItem(name); !ok || got != long(want) {
			t.Errorf("%s = %q (%v), want %d", name, got, ok, want)
		}
	}

	env, _ := fixture()
	item(env, "SYI$_PAGEFILE_PAGE", 0)
	item(env, "SYI$_PAGEFILE_FREE", 0)
	item(env, "SYI$_MPW_WRTCLUSTER", DefaultMPWWriteCluster)
	item(env, "SYI$_PQL_MPGFLQUOTA", 16400)

	env, out := pagingFixture(`"`+filepath.Join(t.TempDir(), "pagefile.sys")+`"`, 200)
	item(env, "SYI$_PAGEFILE_PAGE", 200)

	if env.PageFile() == nil {
		t.Fatalf("PAGEFILE_PAGE didn't open the page file: %q", out)
	}

	if _, ok := env.AllocatePageFile(16); !ok {
		t.Fatal("allocation failed")
	}

	item(env, "SYI$_PAGEFILE_FREE", 184)
	item(env, "SYI$_SWAPFILE_PAGE", 0)
	item(env, "SYI$_SWAPFILE_FREE", 0)
}

// TestInstallPageFile: installing opens the page file only with paging
// on, a name, and no deferral; a file of another size is made the
// configured size, and the console is told, or told why it couldn't
// grow.
func TestInstallPageFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pagefile.sys")
	spec := `"` + path + `"`

	env, _ := fixture()
	env.Sysgen.PageFileName = spec

	if env.InstallPageFile() != nil {
		t.Error("installed with paging off")
	}

	env, _ = pagingFixture("", 0)
	if env.InstallPageFile() != nil {
		t.Error("installed with no name")
	}

	env, _ = pagingFixture(spec, 40)
	env.Sysgen.PageFileDefer = true

	if env.InstallPageFile() != nil || env.PageFile() != nil {
		t.Error("installed with vax.sysgen.paging.defer")
	}

	env, out := pagingFixture(spec, 40)
	if pf := env.InstallPageFile(); pf == nil || pf.Size() != 40 || !pf.Created() {
		t.Fatalf("not installed new at 40 blocks: %q", out)
	}

	if out.Len() != 0 {
		t.Errorf("console output %q for a new page file, want none", out)
	}

	if err := env.ClosePageFile(); err != nil {
		t.Fatal(err)
	}

	env, out = pagingFixture(spec, 60)
	if pf := env.InstallPageFile(); pf == nil || pf.Size() != 60 || pf.Created() {
		t.Fatalf("not resized to 60 blocks: %q", out)
	}

	if want := "%GOVAX-I-PAGEFILESIZE, page file " + path + " changed from 40 to 60 blocks\n"; out.String() != want {
		t.Errorf("console output %q, want %q", out, want)
	}

	if err := env.ClosePageFile(); err != nil {
		t.Fatal(err)
	}
}

// TestInstallPageFileCantGrow: a volume page file the volume hasn't room
// to grow to its configured size is installed at its own size, and the
// console is told why.
func TestInstallPageFileCantGrow(t *testing.T) {
	env, out := pagingFixture("DUA0:[000000]PAGEFILE.SYS", 64)
	newMountedVolumeFixture(t, env)

	pf := env.InstallPageFile()
	if pf == nil {
		t.Fatalf("InstallPageFile failed: %q", out)
	}

	size := pf.Size()
	mounts := env.Mounts

	if err := env.ClosePageFile(); err != nil {
		t.Fatal(err)
	}

	env, out = pagingFixture("DUA0:[000000]PAGEFILE.SYS", 10_000_000)
	env.Mounts = mounts

	if pf = env.InstallPageFile(); pf == nil || pf.Size() != size {
		t.Fatalf("not installed at its own %d blocks: %q", size, out)
	}

	want := "%GOVAX-W-PAGEFILESIZE, page file DUA0:[000000]PAGEFILE.SYS;1 kept at " +
		strconv.FormatUint(uint64(size), 10) + " blocks; it couldn't grow to 10000000\n-GOVAX-W-REASON, "
	if !strings.HasPrefix(out.String(), want) {
		t.Errorf("console output %q, want %q...", out, want)
	}

	if err := env.ClosePageFile(); err != nil {
		t.Fatal(err)
	}
}
