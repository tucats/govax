package corevms

import (
	"os"
	"testing"
)

const testGreeting = "hello"

func TestShimExeOpenWriteReadClose(t *testing.T) {
	env, _ := fixture()
	dir := t.TempDir()
	fn := dir + "/exe_file.txt"

	nameAddr := uint32(0x1000)
	putString(t, env, nameAddr, fn)

	fid, err := shimExeOpen(env, []uint32{nameAddr, posixOWronly | posixOCreat | posixOTrunc})
	if err != nil {
		t.Fatal(err)
	}

	if fid == 0xFFFFFFFF {
		t.Fatal("exe_open failed")
	}

	dataAddr := uint32(0x2000)
	putString(t, env, dataAddr, testGreeting)

	n, err := shimExeWrite(env, []uint32{fid, dataAddr, 5})
	if err != nil {
		t.Fatal(err)
	}

	if n != 5 {
		t.Errorf("wrote %d bytes, want 5", n)
	}

	if _, err := shimExeClose(env, []uint32{fid}); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(fn)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != testGreeting {
		t.Errorf("file contents = %q, want \"hello\"", got)
	}

	// Re-open for reading.
	fid2, err := shimExeOpen(env, []uint32{nameAddr, 0 /* O_RDONLY */})
	if err != nil {
		t.Fatal(err)
	}

	readAddr := uint32(0x3000)

	n, err = shimExeRead(env, []uint32{fid2, readAddr, 10})
	if err != nil {
		t.Fatal(err)
	}

	if n != 5 {
		t.Errorf("read %d bytes, want 5", n)
	}

	readBack, err := loadString(env, readAddr, 10)
	if err != nil {
		t.Fatal(err)
	}

	if readBack != testGreeting {
		t.Errorf("read back = %q, want \"hello\"", readBack)
	}
}

func TestShimExeOpenFailure(t *testing.T) {
	env, _ := fixture()
	nameAddr := uint32(0x1000)
	putString(t, env, nameAddr, "/nonexistent/dir/file.txt")

	fid, err := shimExeOpen(env, []uint32{nameAddr, 0})
	if err != nil {
		t.Fatal(err)
	}

	if fid != 0xFFFFFFFF {
		t.Errorf("fid = %#x, want -1 (open should fail)", fid)
	}
}

func TestShimExeWriteToConsole(t *testing.T) {
	env, out := fixture()
	dataAddr := uint32(0x1000)
	putString(t, env, dataAddr, "console text")

	n, err := shimExeWrite(env, []uint32{1, dataAddr, 12})
	if err != nil {
		t.Fatal(err)
	}

	if n != 12 {
		t.Errorf("n = %d, want 12", n)
	}

	if out.String() != "console text" {
		t.Errorf("console output = %q, want \"console text\"", out.String())
	}
}

// TestShimExeOpenSharing: host files opened by EXE$OPEN are shared by
// RMS's rule (docs/PHASE-49.md, subtask 11): two processes may read a
// file together, but a writer and a reader may not have it open at once
// either way round; a refused truncating open leaves the file alone; a
// close, or the process's rundown, lets the next open in.
func TestShimExeOpenSharing(t *testing.T) {
	env, _ := fixture()
	other := newProcess(t, env)

	fn := t.TempDir() + "/shared.txt"
	if err := os.WriteFile(fn, []byte(testGreeting), 0o644); err != nil {
		t.Fatal(err)
	}

	const nameAddr = uint32(0x1000)

	putString(t, env, nameAddr, fn)

	open := func(p *Environment, flags uint32) uint32 {
		t.Helper()

		fid, err := shimExeOpen(p, []uint32{nameAddr, flags})
		if err != nil {
			t.Fatal(err)
		}

		return fid
	}

	const failed = 0xFFFFFFFF

	// Two readers share.
	r1 := open(env, 0)
	r2 := open(other, 0)

	if r1 == failed || r2 == failed {
		t.Fatalf("two readers: %#x, %#x; want both open", r1, r2)
	}

	// A writer can't join them, and its truncation doesn't happen.
	if fid := open(other, posixOWronly|posixOTrunc); fid != failed {
		t.Errorf("a writer joined two readers: %#x", fid)
	}

	if got, _ := os.ReadFile(fn); string(got) != testGreeting {
		t.Errorf("after the refused open, the file holds %q", got)
	}

	// With the readers gone, the writer gets it, and then a reader can't.
	_, _ = shimExeClose(env, []uint32{r1})
	_, _ = shimExeClose(other, []uint32{r2})

	w := open(other, posixOWronly|posixOAppend)
	if w == failed {
		t.Fatal("the writer was refused with no other opener")
	}

	if fid := open(env, 0); fid != failed {
		t.Errorf("a reader joined a writer: %#x", fid)
	}

	// The writer's image rundown (which closes its files) releases it.
	other.ImageRundown()

	if fid := open(env, 0); fid == failed || env.HostOpeners.Count() != 1 {
		t.Errorf("after the writer's rundown: %#x, %d files; want the reader open", fid, env.HostOpeners.Count())
	}
}
