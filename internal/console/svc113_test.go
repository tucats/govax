package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
)

// The Phase 113 probe of the services subtask 8 added (docs/planning/PHASE-113,
// subtask 8): testdata/svc113's svc113.mar calls $GETUTC, $ASCUTC, $BINUTC,
// $NUMUTC, $TIMCON, $GETCHN, and $GETDEV case by case. govax's run is held to
// VMS 7.1's, vax/svc113-vax.txt (subtask 12d), line for line, but for
// expectedSvc113Differences and the device's operation count (svc113Masked).

// svc113Compared turns on the comparison with VMS's run, as str113Compared
// does; subtask 13b made govax's services VMS's.
const svc113Compared = true

// svc113NoDevice is why the terminal's and the default disk's cases differ:
// the test console has neither, and VMS's describe its own OPA0: and the
// round's volume.
const svc113NoDevice = "the test console has no terminal or disk; VMS's lines are its own OPA0: and volume"

// expectedSvc113Differences are the lines of VMS's output govax doesn't
// reproduce, by the case's name and the line's first word, each with why.
var expectedSvc113Differences = map[string]string{
	"GETDEV TT/GETDEV":       svc113NoDevice,
	"GETDEV TT/PL:":          svc113NoDevice,
	"GETDEV TT/PB:":          svc113NoDevice,
	"GETDEV TT/PB+32:":       svc113NoDevice,
	"GETDEV TT/PB+96:":       svc113NoDevice,
	"GETDEV TT/SB:":          svc113NoDevice,
	"GETDEV TT/SB+32:":       svc113NoDevice,
	"GETDEV TT/SB+96:":       svc113NoDevice,
	"GETDEV SYS$DISK/GETDEV": svc113NoDevice,
	"GETDEV SYS$DISK/PL:":    svc113NoDevice,
	"GETDEV SYS$DISK/PB:":    svc113NoDevice,
	"GETDEV SYS$DISK/PB+32:": svc113NoDevice,
	"GETDEV SYS$DISK/PB+96:": svc113NoDevice,
	"GETDEV SYS$DISK/SB:":    svc113NoDevice,
	"GETDEV SYS$DISK/SB+32:": svc113NoDevice,
	"GETDEV SYS$DISK/SB+96:": svc113NoDevice,
}

// svc113Masked is a line as both sides are compared: a buffer's first line
// without its eighth longword, DIB$L_OPCNT, the operations the device has
// done, which are each system's own (NLA0:'s were 13 on the VAX).
func svc113Masked(line string) string {
	f := strings.Fields(line)
	if len(f) >= 9 && (f[0] == "PB:" || f[0] == "SB:") {
		f[8] = "(OPCNT)"

		return "  " + strings.Join(f, " ")
	}

	return line
}

// svc113Manual is what the manuals settle: each case's lines from its first,
// the case's name and R0, on.
var svc113Manual = map[string][]string{
	// $GETUTC's time is $GETTIM's, in UTC: through $TIMCON, within a second.
	"GETUTC vs GETTIM": {"GETUTC vs GETTIM R0=00000001", "  Z: 00000001"},
	// $ASCUTC's text is $ASCTIM's of the local time, in the octaword's zone;
	// cvtflg 1 the time alone; 12 bytes the date.
	"ASCUTC eastern":      {"ASCUTC eastern R0=00000001", "  L: 5A5A0017", `  D: 23 " 4-MAR-1999 13:30:15.25"`},
	"ASCUTC eastern time": {"ASCUTC eastern time R0=00000001", "  L: 5A5A000B", `  D: 11 "13:30:15.25"`},
	"ASCUTC plus hour":    {"ASCUTC plus hour R0=00000001", "  L: 5A5A0017", `  D: 23 " 4-MAR-1999 13:30:15.25"`},
	"ASCUTC short 12":     {"ASCUTC short 12 R0=00000001", "  L: 5A5A000C", `  D: 12 " 4-MAR-1999 "`},
	// $TIMCON 0 of an octaword made from the local time: that time.
	"TIMCON 0 eastern":   {"TIMCON 0 eastern R0=00000001", "  S: 99E23320 009D49CB"},
	"TIMCON 0 plus hour": {"TIMCON 0 plus hour R0=00000001", "  S: 99E23320 009D49CB"},
	// A delta time has no UTC (SS$_IVTIME).
	"BINUTC delta":   {"BINUTC delta R0=00000184"},
	"TIMCON 1 delta": {"TIMCON 1 delta R0=00000184"},
	// A channel that isn't assigned.
	"GETCHN 0":   {"GETCHN 0 R0=0000013C"},
	"GETCHN 999": {"GETCHN 999 R0=0000013C"},
	// No such device.
	"GETDEV nosuch": {"GETDEV nosuch R0=00000908"},
}

func TestSvc113Probe(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "svc113")

	src, err := os.ReadFile(filepath.Join(dir, "svc113.mar"))
	if err != nil {
		t.Fatal(err)
	}

	r0, out := runSvc113(t, string(src))
	if r0 != 1 {
		t.Errorf("the program's status = %#x, want 1", r0)
	}

	gotLines := strings.Split(strings.ReplaceAll(out, "\r", ""), "\n")
	if path := os.Getenv("SVC113_WRITE"); path != "" {
		if err := os.WriteFile(path, []byte(strings.Join(gotLines, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(filepath.Join(dir, "vax", "svc113-vax.txt"))
	if os.IsNotExist(err) {
		holdSvc113ToManual(t, gotLines)

		return
	} else if err != nil {
		t.Fatal(err)
	}

	if !svc113Compared {
		holdSvc113ToManual(t, gotLines)
		t.Logf("%d lines differ from VMS's run (svc113Compared is off)",
			countDifferentLines(gotLines, strings.Split(string(want), "\n")))

		return
	}

	compareSvc113(t, gotLines, strings.Split(string(want), "\n"))
}

// runSvc113 is runMacroProgram with the null device defined as STARTUP.COM
// defines it, for the device information cases.
func runSvc113(t *testing.T, source string) (uint32, string) {
	t.Helper()

	c := newBootableConsole(t)
	out := &bytes.Buffer{}
	c.Out = out
	path := filepath.Join(t.TempDir(), "svc113")

	d := NewDispatcher(c, loadEvaxGrammar(t))
	if err := d.Dispatch("DEFINE/DEVICE NLA0/DEVCLASS=MAILBOX/DEVTYPE=NULL/DEVCHAR=202702849/OWNUIC=65537/DEVBUFSIZ=512"); err != nil {
		t.Fatal(err)
	}

	writeHostFile(t, path+".mar", source)

	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: path + ".mar", Object: path + ".obj"}); err != nil {
		t.Fatalf("MACRO: %v\n%s", err, out.String())
	}

	if err := c.cmd().Link(dclcmd.LinkOptions{Objects: []string{path}, Executable: path + ".exe"}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	r0 := runImage(t, c, path+".exe")
	_, after, _ := strings.Cut(out.String(), "Kernel initialized\n\r")

	return r0, after
}

// holdSvc113ToManual checks svc113Manual's cases among govax's lines.
func holdSvc113ToManual(t *testing.T, lines []string) {
	t.Helper()

	found := 0

	for i, line := range lines {
		name, _, ok := strings.Cut(line, " R0=")
		if !ok || strings.HasPrefix(line, " ") {
			continue
		}

		want, ok := svc113Manual[name]
		if !ok {
			continue
		}

		found++

		for k, w := range want {
			if i+k >= len(lines) || lines[i+k] != w {
				t.Errorf("%s: line %d is %q, want %q", name, k+1, at(lines, i+k), w)
			}
		}
	}

	if found != len(svc113Manual) {
		t.Errorf("found %d of the manuals' %d cases", found, len(svc113Manual))
	}
}

// compareSvc113 holds govax's lines to VMS's, as compareStr113 does.
func compareSvc113(t *testing.T, gotLines, wantLines []string) {
	t.Helper()

	caseName := ""
	seen := map[string]bool{}

	for i := range max(len(gotLines), len(wantLines)) {
		g, w := svc113Masked(at(gotLines, i)), svc113Masked(at(wantLines, i))

		if name, _, ok := strings.Cut(w, " R0="); ok && !strings.HasPrefix(w, " ") {
			caseName = name
		}

		if g == w {
			continue
		}

		key := caseName + "/" + strings.Fields(w + " ?")[0]
		if why, ok := expectedSvc113Differences[key]; ok {
			seen[key] = true

			t.Logf("line %d differs as expected (%s):\n\tgot  %q\n\tvax  %q", i+1, why, g, w)

			continue
		}

		t.Errorf("line %d:\n\tgot  %q\n\twant %q", i+1, g, w)
	}

	for key := range expectedSvc113Differences {
		if !seen[key] {
			t.Errorf("%s: expected to differ from VMS, but matches; remove it from expectedSvc113Differences", key)
		}
	}
}
