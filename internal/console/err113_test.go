package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 113's probe round (docs/planning/PHASE-113 - Fortran errata.md, subtask
// 12; testdata/err113's README): ERR113.COM asks VMS what the phase's earlier
// subtasks left unconfirmed (STR$'s arithmetic, the UTC and device information
// services, LIB$INITIALIZE, MACRO's questions, DCL's messages, TYPE without
// carriage control, a full volume, and the index file past its header).
// TestErr113Procedure rehearses it on govax, so a mistake in the procedure shows
// here before the VAX's run; with VMS's log in testdata/err113/vax, the test shows
// it beside govax's, for subtask 13 to hold govax to.

// runErr113 makes the round's volume as exchange.cmd does, and a scratch volume as
// INITIALIZE/HEADERS=16 makes it on the VAX, and runs ERR113.COM with the volume on
// DUA1 and the scratch volume on DUA3. It returns the log and STR113.TXT.
func runErr113(t *testing.T) (string, string) {
	t.Helper()

	d, c, buf := shareFindConsole(t, false)
	dir := t.TempDir()
	exchange := filepath.Join(dir, "err113.dsk")
	scratch := filepath.Join(dir, "scratch.dsk")

	shareFindRun(t, d, `INITIALIZE/CONTAINER "`+exchange+`" /DEVICE=RD54 ERR113`)
	shareFindRun(t, d, `INITIALIZE/CONTAINER "`+scratch+`" /DEVICE=RD51 /HEADERS=16 SCR113`)

	if err := c.cmd().Mount("DUA5:", exchange, true); err != nil {
		t.Fatal(err)
	}

	t.Chdir(filepath.Join("..", ".."))

	script, err := os.ReadFile(filepath.Join("testdata", "err113", "exchange.cmd"))
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	for line := range strings.SplitSeq(string(script), "\n") {
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "INITIALIZE/") ||
			strings.HasPrefix(line, "MOUNT/") || strings.HasPrefix(line, "DISMOUNT") {
			continue
		}

		err := d.Dispatch(line)
		if out := buf.String(); err != nil || out != "" {
			t.Fatalf("%s returned %v and said:\n%.600s", line, err, out)
		}
	}

	// On the VAX the volume is DUA1 and the scratch disk DUA3.
	shareFindRun(t, d, "SET DEFAULT DUA2:[000000]")
	shareFindRun(t, d, "DISMOUNT DUA5")
	shareFindRun(t, d, "DISMOUNT DUA1")

	if err := c.cmd().Mount("DUA1:", exchange, true); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA3:", scratch, true); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	shareFindRun(t, d, "@ERR113/OUTPUT=ERR113.LOG")

	read := func(spec, name string) string {
		host := filepath.Join(t.TempDir(), name)
		shareFindRun(t, d, `COPY/QUIET `+spec+` "`+host+`"/HOST`)

		data, err := os.ReadFile(host)
		if err != nil {
			t.Fatal(err)
		}

		return strings.ReplaceAll(string(data), "\r", "")
	}

	return read("DUA1:[000000]ERR113.LOG", "err113.log"), read("DUA1:[ERR113]STR113.TXT", "str113.txt")
}

func TestErr113Procedure(t *testing.T) {
	if testing.Short() {
		t.Skip("the round's procedure makes some 3,000 files")
	}

	log, str113 := runErr113(t)
	sections := vms93Sections(log)

	for _, name := range []string{"STR113", "SVC113", "INI113", "INI113NT", "MAC113A", "MAC113B", "MAC113C",
		"MAC113D", "DCL", "NOCC", "FULL", "INDEX", "END"} {
		if _, ok := sections[name]; !ok {
			t.Errorf("no section %s in the log:\n%s", name, log)
		}
	}

	// Each section's lines govax's run must have, in this order.
	want := map[string][]string{
		"STR113": {`$STATUS == "%X00000001"`},
		// The shareable image's routine first, then the main image's two, then
		// the main program, run from INITB's call of the init-coroutine.
		"INI113": {
			`$STATUS == "%X10000001"`, `$STATUS == "%X10000001"`,
			"SINIT: 3 arguments", "INITA: 3 arguments", "INITB: 3 arguments", "MAIN: 3 arguments",
			"INI113S_HELLO", "MAIN: returning 3", "INITB: the init-coroutine returned 00000003",
			`$STATUS == "%X00000003"`,
		},
		"INI113NT": {"SINIT: 3 arguments", "INITA: 3 arguments", "MAIN: returning 3", `$STATUS == "%X00000003"`},
		"MAC113A":  {`$STATUS ==`},
		"MAC113B":  {"%DCL-W-ASMWARNING", "%DCL-E-ASSEMBLING", `$STATUS ==`},
		"MAC113C":  {`$STATUS == "%X10000001"`},
		"MAC113D":  {`$STATUS == "%X10000001"`},
		"DCL": {
			"%DELETE-E-DELVER, explicit version number or wild card required",
			"%PURGE-W-SEARCHFAIL", "-RMS-F-SYN",
			"%COPY-E-OPENOUT", "-RMS-F-SYN",
			"%DCL-W-IVVALU", `\0\`,
			"%DCL-W-INSFPRM", "%DCL-W-INSFPRM",
		},
		"NOCC": {"BEFORE", "ONE", "TWO", "AFTER"},
		"FULL": {
			"BIG.DAT: 4064 blocks", `S = "%X00030001"`,
			"%COPY-E-WRITEERR", "-RMS-F-FUL", "-SYSTEM-W-DEVICEFULL", `S = "%X00018544"`,
			"%DCL-E-OPENOUT", "-RMS-F-FUL",
			"%CREATE-E-OPENOUT", "-RMS-F-FUL",
			"%DCL-E-OBJWRITE", "-RMS-F-FUL",
		},
		// govax has no index file limit (its header takes extension headers),
		// DUMP/HEADER, or ANALYZE/DISK_STRUCTURE.
		"INDEX": {"FILES 3000 STATUS %X00000001", "INDEXF.SYS;1"},
	}

	for name, lines := range want {
		text := sections[name]
		at := 0

		for _, line := range lines {
			i := strings.Index(text[at:], line)
			if i < 0 {
				t.Errorf("%s: no %q after line %d:\n%s", name, line, strings.Count(text[:at], "\n")+1, text)

				break
			}

			at += i + len(line)
		}
	}

	// STR113's output went to its own file, with the MATCH_WILD cases.
	if n := strings.Count(str113, "\nWILD "); n != 12 {
		t.Errorf("STR113.TXT has %d WILD cases, want 12:\n%s", n, str113)
	}

	vms, err := os.ReadFile(filepath.Join("testdata", "err113", "vax", "err113.log"))
	if os.IsNotExist(err) {
		return
	} else if err != nil {
		t.Fatal(err)
	}

	t.Logf("VMS's log, for subtask 13:\n%s\ngovax's:\n%s", vms, log)
}
