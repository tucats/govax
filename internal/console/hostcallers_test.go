package console_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/testproc"
	"github.com/tucats/govax/internal/vmserrors"
)

// hostCommand is one command of TestHostFileCommands: its line, with H/
// for the host directory, and the lines it shows when the host file is
// refused, in the same form.
type hostCommand struct {
	line    string
	refused []string
}

// hostPRV is the RMS$_PRV line every refusal ends with.
const hostPRV = "-RMS-E-PRV, insufficient privilege or file protection violation"

// hostCommands are the commands that reach a host file through
// rms.Session.Locate (docs/planning/PHASE-97 - host file access.md, A.2),
// each refused as it refuses a volume file the process's protection
// keeps it from (Decision 5).
var hostCommands = []hostCommand{
	{`TYPE "H/notes.txt"`, []string{"%TYPE-W-OPENIN, error opening H/notes.txt as input", hostPRV}},
	{`TYPE/HOST "H/notes.txt"`, []string{"%TYPE-W-OPENIN, error opening H/notes.txt as input", hostPRV}},
	{`COPY "H/notes.txt" Y.TXT`, []string{"%COPY-E-OPENIN, error opening H/notes.txt as input", hostPRV}},
	{`COPY X.TXT "H/out.txt"`, []string{"%COPY-E-OPENOUT, error opening H/out.txt as output", hostPRV,
		"%COPY-W-NOTCOPIED, DUA1:[PRIV]X.TXT; not copied"}},
	{`DIRECTORY/OUTPUT="H/dir.lis"`, []string{"%" + strings.TrimPrefix(hostPRV, "-")}},
	{`CREATE/FDL="H/x.fdl" Y.DAT`, []string{"%" + strings.TrimPrefix(hostPRV, "-")}},
	{`CONVERT "H/notes.txt" Z.TXT`, []string{"%CONV-F-OPENIN, error opening H/notes.txt as input", hostPRV}},
	{`BACKUP X.TXT "H/s.bck"/SAVE_SET`, []string{"%BACKUP-E-OPENOUT, error opening H/s.bck as output", hostPRV}},
	{`BACKUP/LIST "H/hello.bck"/SAVE_SET`, []string{"Listing of save set(s)", "",
		"%BACKUP-F-OPENIN, error opening H/hello.bck as input", hostPRV}},
	{`BACKUP S.BCK/SAVE_SET "H/*.*"`, []string{"%BACKUP-E-OPENOUT, error opening H/*.* as output", hostPRV}},
	{`MACRO "H/hello.mar"`, []string{"%MACRO-E-OPENIN, error opening H/hello.mar as input", hostPRV}},
	{`MACRO X.MAR/OBJECT="H/hello2.obj"`, []string{"%MACRO-E-OPENOUT, error opening H/hello2.obj as output", hostPRV}},
	{`MACRO X.MAR/LIST="H/hello2.lis"`, []string{"%MACRO-E-OPENOUT, error opening H/hello2.lis as output", hostPRV}},
	{`FORTRAN "H/hello.for"`, []string{"%FORT-F-OPENIN, error opening H/hello.for as input", hostPRV}},
	{`LINK "H/hello.obj"`, []string{"%LINK-F-OPENIN, error opening H/hello.obj as input", hostPRV}},
	{`LIBRARY/CREATE "H/new.olb"`, []string{"%LIBRAR-F-OPENOUT, error opening H/new.olb as output", hostPRV}},
	{`LIBRARY/LIST "H/hello.olb"`, []string{"%LIBRAR-F-OPENIN, error opening H/hello.olb as input", hostPRV}},
	{`ANALYZE/OBJECT "H/hello.obj"`, []string{"%ANALYZE-E-OPENIN, error opening H/hello.obj as input", hostPRV}},
	{`ANALYZE/RMS_FILE "H/notes.txt"`, []string{"%ANALYZE-E-OPENIN, error opening H/notes.txt as input", hostPRV}},
	{`HELP/LIBRARY="H/hello.hlb" FOO`, []string{"%HELP-E-OPENIN, error opening H/hello.hlb as input", hostPRV}},
	{`HELP/OUTPUT="H/help.txt" HELP`, []string{"%" + strings.TrimPrefix(hostPRV, "-")}},
	{`@"H/hello.com"`, []string{"%DCL-E-OPENIN, error opening H/hello.com as input", hostPRV}},
	{`@X.COM/OUTPUT="H/out.lis"`, []string{"%DCL-E-OPENOUT, error opening H/out.lis as output", hostPRV}},
	{`OPEN/READ F "H/notes.txt"`, []string{"%DCL-E-OPENIN, error opening H/notes.txt as input", hostPRV}},
	{`OPEN/WRITE G "H/new.txt"`, []string{"%DCL-E-OPENOUT, error opening H/new.txt as output", hostPRV}},
	{`WRITE SYS$OUTPUT "[" + F$SEARCH("H/*.txt") + "]"`, []string{"[]"}},
	{`WRITE SYS$OUTPUT F$FILE_ATTRIBUTES("H/notes.txt","EOF")`, []string{
		"%SYSTEM-F-NOPRIV, insufficient privilege or object protection violation", ` \H/NOTES.TXT\`}},
	{`RUN "H/hello.exe"`, []string{"%DCL-W-ACTIMAGE, error activating image H/HELLO.EXE",
		"-CLI-E-IMGNAME, image file H/HELLO.EXE;", hostPRV}},
}

// TestHostFileCommands runs each of hostCommands three ways (the phase
// doc's subtask 3): with vax.host.sysprv on, as PRIV94, which doesn't
// hold SYSPRV, refused with the message given and nothing made on the
// host; again after SET PROCESS/PRIVILEGES=SYSPRV, refused by nothing;
// and with the key off, without SYSPRV, as before.
func TestHostFileCommands(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	for _, pass := range []struct {
		name         string
		key, sysprv  bool
		wantRefusals bool
	}{
		{"on", true, false, true},
		{"on-sysprv", true, true, false},
		{"off", false, false, false},
	} {
		t.Run(pass.name, func(t *testing.T) {
			host, dispatch := hostCommandConsole(t, pass.key, pass.sysprv)
			show := func(text string) string {
				text = strings.ReplaceAll(text, host+"/", "H/")

				return strings.ReplaceAll(text, strings.ToUpper(host)+"/", "H/")
			}

			before := hostFiles(t, host)

			for _, cmd := range hostCommands {
				line := strings.ReplaceAll(cmd.line, "H/", host+"/")
				got := show(dispatch(line))

				if pass.wantRefusals {
					if want := strings.Join(cmd.refused, "\n"); got != want {
						t.Errorf("%s:\n got %q\nwant %q", cmd.line, got, want)
					}

					continue
				}

				if strings.Contains(got, "PRV") || got == "[]" {
					t.Errorf("%s: refused:\n%s", cmd.line, got)
				}
			}

			if after := hostFiles(t, host); pass.wantRefusals && !slices.Equal(before, after) {
				t.Errorf("host files made while refused: %v, were %v", after, before)
			}
		})
	}
}

// hostFiles is the names in the host directory dir.
func hostFiles(t *testing.T, dir string) []string {
	t.Helper()

	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, len(ents))
	for i, e := range ents {
		names[i] = e.Name()
	}

	return names
}

// hostCommandConsole is a console with the key on or off, a host
// directory of files for hostCommands (made as SYSTEM), and a volume on
// DUA1 whose [PRIV] is PRIV94's and the default directory, holding X.TXT,
// X.MAR, X.COM, and the save set S.BCK; process 1 is PRIV94, with SYSPRV
// turned on if sysprv. It returns the host directory and a dispatcher of
// lines that returns what each showed.
func hostCommandConsole(t *testing.T, key, sysprv bool) (string, func(string) string) {
	t.Helper()

	host := t.TempDir()
	h := func(name string) string { return `"` + filepath.Join(host, name) + `"` }

	for name, text := range map[string]string{
		"notes.txt": "host text\n",
		"hello.mar": "\t.TITLE HELLO\n\t.ENTRY START,0\n\tRET\n\t.END START\n",
		"hello.for": "      PRINT *, 'HI'\n      END\n",
		"hello.com": "$ WRITE SYS$OUTPUT \"hi from host\"\n",
		"hello.hlb": "",
		"x.fdl":     "FILE\n\tORGANIZATION sequential\n",
	} {
		if err := os.WriteFile(filepath.Join(host, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	setSetting(t, corevms.HostSYSPRVSetting, map[bool]string{true: "true", false: "false"}[key])

	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
	dispatch := func(line string) string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		return strings.TrimRight(strings.ReplaceAll(buf.String(), "\r", ""), "\n")
	}

	dsk := filepath.Join(t.TempDir(), "work.dsk")
	for _, line := range []string{
		`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD53 WORK97`,
		`MOUNT/WRITE DUA1 "` + dsk + `"`,
		"CREATE/DIRECTORY DUA1:[PRIV]/OWNER=[200,3]",
		"SET DEFAULT DUA1:[PRIV]",
		"COPY " + h("notes.txt") + " X.TXT",
		"COPY " + h("hello.mar") + " X.MAR",
		"COPY " + h("hello.com") + " X.COM",
		"BACKUP X.TXT S.BCK/SAVE_SET",
		"BACKUP X.TXT " + h("hello.bck") + "/SAVE_SET",
		"MACRO " + h("hello.mar") + "/OBJECT=" + h("hello.obj"),
		"LINK " + h("hello.obj") + "/EXECUTABLE=" + h("hello.exe"),
		"LIBRARY/CREATE/OBJECT " + h("hello.olb") + " " + h("hello.obj"),
		"SET SECURITY/PROTECTION=(W:RWED) X.*",
		"SET SECURITY/PROTECTION=(W:RWED) S.BCK",
	} {
		if out := dispatch(line); strings.Contains(out, "-E-") || strings.Contains(out, "-F-") {
			t.Fatalf("setup: %s:\n%s", line, out)
		}
	}

	priv94Account.become(c)

	if sysprv {
		if out := dispatch("SET PROCESS/PRIVILEGES=SYSPRV"); out != "" {
			t.Fatalf("SET PROCESS/PRIVILEGES=SYSPRV: %s", out)
		}
	}

	// A host file command with no output named writes its output to the
	// process's current directory (FORTRAN "H/hello.for" makes HELLO.MAR
	// there, MACRO an object, LINK an image), which under go test is this
	// package's source directory. The test runs in a scratch directory of its
	// own instead, removed when it ends, so nothing is left in the tree; and a
	// directory apart from host, so that what the commands make there doesn't
	// count among host's files.
	t.Chdir(t.TempDir())

	return host, dispatch
}
