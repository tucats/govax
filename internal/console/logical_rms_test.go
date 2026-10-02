package console

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// TestLogicalNames_fileCommands is docs/PHASE-25.md subtask 7's console
// end-to-end test: two real containers mounted through DCL, and the file
// commands reaching them through logical names, search lists, the
// DISK$label names MOUNT defines, and SYS$DISK.
func TestLogicalNames_fileCommands(t *testing.T) {
	d, c, buf := logicalDispatcher(t)

	host := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(host, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		return p
	}

	// MOUNT defines DISK$label, concealed, in the system table.
	mustDispatch(t, d,
		fmt.Sprintf(`MOUNT DUA0 "%s"`, newTestContainer(t, "WORKA")),
		fmt.Sprintf(`MOUNT DUA1: "%s"`, newTestContainer(t, "WORKB")))

	wantOutput(t, d, buf, "SHOW LOGICAL/FULL DISK$WORKB",
		"  \"DISK$WORKB\" [exec] = \"_DUA1:\" [concealed,terminal] (LNM$SYSTEM_TABLE)\n")

	mustDispatch(t, d,
		fmt.Sprintf(`COPY/QUIET "%s"/HOST DISK$WORKA:SPEECH.TXT`, write("a.txt", "fourscore\n")),
		fmt.Sprintf(`COPY/QUIET "%s"/HOST DUA1:SPEECH.TXT`, write("b.txt", "and seven\n")),
		fmt.Sprintf(`COPY/QUIET "%s"/HOST DUA1:ONLY.TXT`, write("c.txt", "second\n")))

	// A search list: DIRECTORY lists every element, TYPE the first found.
	mustDispatch(t, d, "DEFINE BOTH DUA0:, DUA1:")

	buf.Reset()
	mustDispatch(t, d, "DIRECTORY BOTH:*.TXT")

	out := buf.String()
	if !strings.Contains(out, "Directory DUA0:[000000]") || !strings.Contains(out, "Directory DUA1:[000000]") ||
		!strings.Contains(out, "Total of 3 file(s).") {
		t.Errorf("DIRECTORY BOTH:*.TXT =\n%s", out)
	}

	wantOutput(t, d, buf, "TYPE BOTH:SPEECH.TXT", "fourscore\n")
	wantOutput(t, d, buf, "TYPE BOTH:ONLY.TXT", "second\n")

	// A concealed device is shown by its logical name.
	buf.Reset()
	mustDispatch(t, d, "DIRECTORY DISK$WORKB:*.TXT")

	if !strings.Contains(buf.String(), "Directory DISK$WORKB:[000000]") {
		t.Errorf("DIRECTORY DISK$WORKB:*.TXT =\n%s", buf.String())
	}

	// SET DEFAULT puts the device in SYS$DISK, keeping the concealed
	// name, and a spec with no device then uses it.
	mustDispatch(t, d, "SET DEFAULT DISK$WORKB:[000000]")
	wantOutput(t, d, buf, "SHOW DEFAULT", "  DISK$WORKB:[000000]\n")
	wantOutput(t, d, buf, "SHOW TRANSLATION SYS$DISK", "SYS$DISK = \"DISK$WORKB:\" (LNM$PROCESS_TABLE)\n")
	wantOutput(t, d, buf, "TYPE ONLY.TXT", "second\n")

	// SET DEFAULT to a search list (User's Manual §11.7.2).
	mustDispatch(t, d, "SET DEFAULT BOTH")
	wantOutput(t, d, buf, "SHOW DEFAULT", "  BOTH:[000000]\n=   DUA0:[000000]\n=   DUA1:[000000]\n")
	wantOutput(t, d, buf, "TYPE ONLY.TXT", "second\n")

	// DELETE works through every element.
	buf.Reset()
	mustDispatch(t, d, "DELETE SPEECH.TXT;*")

	if n := strings.Count(buf.String(), "%DELETE-S-DELETED"); n != 2 {
		t.Errorf("DELETE SPEECH.TXT;* deleted %d files:\n%s", n, buf.String())
	}

	// A circular definition is reported as such.
	mustDispatch(t, d, "DEFINE LOOP1 LOOP2:", "DEFINE LOOP2 LOOP1:")

	for _, line := range []string{"TYPE LOOP1:X.TXT", "DIRECTORY LOOP1:", "SET DEFAULT LOOP1:", "MOUNT LOOP1: X.DSK"} {
		wantDispatchStatus(t, d, line, vmserrors.SS_TOOMANYLNAM)
	}

	// DISMOUNT accepts a logical name, and removes DISK$label.
	mustDispatch(t, d, "DISMOUNT DISK$WORKA:")

	if _, ok := c.Mounts.Lookup("DUA0"); ok {
		t.Error("DUA0 still mounted after DISMOUNT DISK$WORKA:")
	}

	if _, err := c.Logicals.Translate(lnm.SystemTableName, "DISK$WORKA", lnm.User, 0); err == nil {
		t.Error("DISK$WORKA survived DISMOUNT")
	}

	// "_" suppresses translation, so _BOTH: is a (nonexistent) device.
	wantDispatchStatus(t, d, "DISMOUNT _BOTH:", vmserrors.SS_DEVNOTMOUNT)
}

// TestLNMRoundTrip_assembledProgram is docs/PHASE-25.md subtask 8's
// acceptance test: testdata/asm/lnm_roundtrip.asm, assembled and run as
// real VAX code, creates a logical name with $CRELNM, reads it back with
// $TRNLNM, writes a record through it with RMS (MYOUT -> SYS$OUTPUT ->
// the console), and deletes it with $DELLNM. The program checks each
// step itself and leaves 1 in R0 only if all of them worked.
func TestLNMRoundTrip_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	out := c.Out.(*bytes.Buffer)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "lnm_roundtrip.asm"))
	if err != nil {
		t.Fatalf("Assemble(lnm_roundtrip.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("lnm_roundtrip.asm has no entry address")
	}

	out.Reset()

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running lnm_roundtrip.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("lnm_roundtrip.asm didn't finish within 100,000 steps")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (every logical-name step worked)", got)
	}

	if !strings.Contains(out.String(), "LNM round trip OK") {
		t.Errorf("console output = %q, want the record written through MYOUT", out.String())
	}

	if _, err := c.Logicals.Translate("LNM$PROCESS", "MYOUT", lnm.User, 0); err == nil {
		t.Error("MYOUT survived the program's $DELLNM")
	}
}

// TestImageRundown: when RUN's image returns, its user-mode logical names
// in the process table are deleted, and others are kept (User's Manual
// §11.3.5). A CALL that isn't a RUN image leaves them alone.
func TestImageRundown(t *testing.T) {
	c := New(&bytes.Buffer{})

	define := func(name string, mode lnm.Mode) {
		t.Helper()

		if err := c.DefineLogicalName("LNM$PROCESS", name, []string{"X"}, mode, 0, false); err != nil {
			t.Fatal(err)
		}
	}

	exists := func(name string) bool {
		_, err := c.Logicals.Translate("LNM$PROCESS", name, lnm.User, 0)

		return err == nil
	}

	define("USERNAME", lnm.User)
	define("SUPERNAME", lnm.Supervisor)

	// Not a RUN image: nothing is run down.
	if err := c.reportStopReason(cpu.ErrConsoleCallReturned); err != nil {
		t.Fatal(err)
	}

	if !exists("USERNAME") {
		t.Fatal("a plain CALL's return ran down user-mode names")
	}

	c.imageActive = true

	if err := c.reportStopReason(cpu.ErrConsoleCallReturned); err != nil {
		t.Fatal(err)
	}

	if exists("USERNAME") || !exists("SUPERNAME") || !exists("SYS$OUTPUT") {
		t.Errorf("after rundown: USERNAME %v, SUPERNAME %v, SYS$OUTPUT %v",
			exists("USERNAME"), exists("SUPERNAME"), exists("SYS$OUTPUT"))
	}

	if c.imageActive {
		t.Error("imageActive still set after the image's exit")
	}
}

// TestImageRundown_run: the same, through a real RUN of simple.exe.
func TestImageRundown_run(t *testing.T) {
	c := newBootableConsole(t)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	runKernelInitialize(t, c)

	if err := c.DefineLogicalName("LNM$PROCESS", "USERNAME", []string{"X"}, lnm.User, 0, false); err != nil {
		t.Fatal(err)
	}

	if err := c.Run(exeFixturePath(t, "simple.exe"), RunOptions{}); err != nil {
		t.Fatalf("RUN simple.exe: %v", err)
	}

	if _, err := c.Logicals.Translate("LNM$PROCESS", "USERNAME", lnm.User, 0); err == nil {
		t.Error("USERNAME survived RUN's image exit")
	}
}
