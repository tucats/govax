package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
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
	if !strings.Contains(out, "Directory DUA0:[]") || !strings.Contains(out, "Directory DUA1:[]") ||
		!strings.Contains(out, "Total of 3 file(s).") {
		t.Errorf("DIRECTORY BOTH:*.TXT =\n%s", out)
	}

	wantOutput(t, d, buf, "TYPE BOTH:SPEECH.TXT", "fourscore\n")
	wantOutput(t, d, buf, "TYPE BOTH:ONLY.TXT", "second\n")

	// A concealed device is shown by its logical name.
	buf.Reset()
	mustDispatch(t, d, "DIRECTORY DISK$WORKB:*.TXT")

	if !strings.Contains(buf.String(), "Directory DISK$WORKB:[]") {
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
