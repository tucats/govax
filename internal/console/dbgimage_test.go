package console

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// listImagePath is a path to one of testdata/mar/list/vax's images: real
// LINK's links of the Phase 29 listing probes, some with debug data.
func listImagePath(t *testing.T, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "mar", "list", "vax", name)
}

// runStepped loads the image at path as RUN/STEP does, leaving it stopped
// at its first instruction, and clears the output so far.
func runStepped(t *testing.T, path string) (*Console, *bytes.Buffer) {
	t.Helper()

	c := newRunnableConsole(t)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.Run(path, RunOptions{Step: true}); err != nil {
		t.Fatalf("RUN/STEP %s: %v", filepath.Base(path), err)
	}

	buf := c.Out.(*bytes.Buffer)
	buf.Reset()

	return c, buf
}

// TestImageDebugSymbols: each image's debug symbol table is read as it's
// loaded, and SHOW IMAGES says which kind of debug data it has.
func TestImageDebugSymbols(t *testing.T) {
	cases := []struct {
		image string
		kind  string // SHOW IMAGES' note
	}{
		{"trlnkdbg.exe", "DEBUG"},
		{"trdbgtrc.exe", "TRACEBACK"},
		{"trnotb.exe", ""},
	}

	for _, tc := range cases {
		c, buf := runStepped(t, listImagePath(t, tc.image))

		main := c.findMainICB()
		if main == nil {
			t.Fatalf("%s: no main image loaded", tc.image)
		}

		if main.DebugErr != nil {
			t.Errorf("%s: reading its debug symbols: %v", tc.image, main.DebugErr)
		}

		if (main.Debug != nil) != (tc.kind != "") {
			t.Errorf("%s: Debug = %v, want debug symbols %v", tc.image, main.Debug, tc.kind != "")
		}

		if err := c.ShowImages(false); err != nil {
			t.Fatalf("%s: SHOW IMAGES: %v", tc.image, err)
		}

		var line string

		for _, l := range strings.Split(buf.String(), "\n") {
			if strings.Contains(l, "<MAIN>") {
				line = strings.TrimRight(l, " ")
			}
		}

		got := ""
		if f := strings.Fields(line); len(f) == 4 {
			got = f[3]
		}

		if got != tc.kind {
			t.Errorf("%s: SHOW IMAGES line %q, want the note %q", tc.image, line, tc.kind)
		}
	}
}

// TestDisassembleImageEntryMasks: in a real image RUN loaded, the word at
// each .ENTRY is shown as its register-save mask (the DST names the
// routines; the console's symbol table has none of them). TRACE's
// routines are TRACE (0x600, ^M<R2>), FIRST
// (0x61E, ^M<>), and SECOND (0x62D, ^M<R2>); the traceback-only link
// names them too.
func TestDisassembleImageEntryMasks(t *testing.T) {
	for _, image := range []string{"trlnkdbg.exe", "trdbgtrc.exe"} {
		c, buf := runStepped(t, listImagePath(t, image))

		if err := c.Disassemble(0x600, 0x641); err != nil {
			t.Fatalf("%s: DISASSEMBLE: %v", image, err)
		}

		out := buf.String()

		for _, want := range []string{
			"00000600: .ENTRY TRACE,^M<R2>\n",
			"0000061E: .ENTRY FIRST,^M<>\n",
			"0000062D: .ENTRY SECOND,^M<R2>\n",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: DISASSEMBLE output lacks %q:\n%s", image, want, out)
			}
		}
	}
}
