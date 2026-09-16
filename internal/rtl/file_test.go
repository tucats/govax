package rtl

import (
	"os"
	"testing"
)

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
	putString(t, env, dataAddr, "hello")

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

	if string(got) != "hello" {
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

	if readBack != "hello" {
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
