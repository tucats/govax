package console

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/rms"
)

// editConsole is a console with a fresh volume as the default device, and a
// scratch directory of its own. Each test's "editor" is a shell script.
func editConsole(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer, string) {
	t.Helper()

	scratch := t.TempDir()

	withSettings(t, map[string]string{
		dclcmd.EditorScratchSetting: scratch,
		dclcmd.EditorCommandSetting: "sh -c true",
	})

	d, c, buf := shareFindConsole(t, false)
	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	buf.Reset()

	return d, c, buf, scratch
}

// useEditor makes script, a shell script with the file as $1, the editor.
func useEditor(t *testing.T, script string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "editor.sh")
	writeHostFile(t, path, script+"\n")
	settings.Set(dclcmd.EditorCommandSetting, "sh "+path+" %file")
}

// editRecords reads a volume file's records, control bytes and all.
func editRecords(t *testing.T, c *Console, spec string) []string {
	t.Helper()

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: spec}, rms.VariableRecords)
	if err != nil {
		t.Fatalf("reading %s: %v", spec, err)
	}

	out := make([]string, len(records))
	for i, r := range records {
		out[i] = string(r)
	}

	return out
}

// editVersions lists the versions of a volume file that exist.
func editVersions(t *testing.T, c *Console, spec string) int {
	t.Helper()

	names, err := c.ContainerSession.Search(spec + ";*")
	if err != nil {
		return 0
	}

	return len(names)
}

// editLoad copies text in as a volume file with the COPY qualifiers given.
func editLoad(t *testing.T, d *Dispatcher, name, text, qualifiers string) {
	t.Helper()

	host := filepath.Join(t.TempDir(), "in.txt")
	writeHostFile(t, host, text)
	shareFindRun(t, d, `COPY/QUIET "`+host+`"/HOST `+name+qualifiers)
}

// TestEditKeepsRecordLayout has an editor append a line to a file of each kind,
// and checks the new version's records and that its layout is the old one's.
func TestEditKeepsRecordLayout(t *testing.T) {
	d, c, buf, _ := editConsole(t)

	useEditor(t, `printf 'SHORT\n' >> "$1"`)

	blank := func(s string, n int) string { return s + strings.Repeat(" ", n-len(s)) }

	cases := []struct {
		name, qualifiers string
		format, attrs    string
		want             []string
	}{
		{
			"fixed", "/RECORD_FORMAT=FIXED/RECORD_SIZE=9", "Fixed length 9 byte records", "Carriage return",
			[]string{blank("ONE", 9), blank("TWO TWO", 9), blank("SHORT", 9)},
		},
		{
			"odd fixed none", "/RECORD_FORMAT=FIXED/RECORD_SIZE=9/CARRIAGE_CONTROL=NONE", "Fixed length 9 byte records", "None",
			[]string{blank("ONE", 9), blank("TWO TWO", 9), blank("SHORT", 9)},
		},
		{
			"variable", "/RECORD_FORMAT=VARIABLE", "Variable length", "Carriage return",
			[]string{"ONE", "TWO TWO", "SHORT"},
		},
		{
			"fortran", "/RECORD_FORMAT=VARIABLE/CARRIAGE_CONTROL=FORTRAN", "Variable length", "Fortran",
			[]string{"ONE", "TWO TWO", "SHORT"},
		},
		{
			"vfc", "/RECORD_FORMAT=VFC", "VFC", "Print",
			[]string{"\x01\x8dONE", "\x01\x8dTWO TWO", "\x01\x8dSHORT"},
		},
		{
			"stream", "/RECORD_FORMAT=STREAM", "Stream", "Carriage return",
			[]string{"ONE", "TWO TWO", "SHORT"},
		},
		{
			"stream LF", "/RECORD_FORMAT=STREAM_LF", "Stream_LF", "Carriage return",
			[]string{"ONE", "TWO TWO", "SHORT"},
		},
		{
			"stream CR", "/RECORD_FORMAT=STREAM_CR", "Stream_CR", "Carriage return",
			[]string{"ONE", "TWO TWO", "SHORT"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			editLoad(t, d, "OUT.DAT", "ONE\nTWO TWO\n", tc.qualifiers)

			buf.Reset()
			shareFindRun(t, d, "EDIT OUT.DAT")

			if out := buf.String(); !strings.Contains(out, "EDIT-S-CREATED, DUA1:[000000]OUT.DAT;2 created, 3 lines") {
				t.Errorf("EDIT said %q", out)
			}

			if got := editRecords(t, c, "OUT.DAT;2"); strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("records %q, want %q", got, tc.want)
			}

			buf.Reset()
			shareFindRun(t, d, "DIRECTORY/FULL OUT.DAT;2")

			out := buf.String()
			if !strings.Contains(out, "Record format:      "+tc.format) {
				t.Errorf("not %q:\n%s", tc.format, out)
			}

			if !strings.Contains(out, "Record attributes:  "+tc.attrs) {
				t.Errorf("not %q:\n%s", tc.attrs, out)
			}

			shareFindRun(t, d, "DELETE OUT.DAT;*")
		})
	}
}

// TestEditFixedLines is the rules for a line too long for a fixed record: blanks
// at its end may go, and anything else means no new version, with the user's text
// kept where the message says.
func TestEditFixedLines(t *testing.T) {
	d, c, buf, scratch := editConsole(t)

	editLoad(t, d, "F.FOR", "      PROG\nA\n", "/RECORD_FORMAT=FIXED/RECORD_SIZE=10")

	t.Run("the editor doesn't see the padding", func(t *testing.T) {
		useEditor(t, `grep -q ' $' "$1" && exit 7; exit 0`)

		if err := d.Dispatch("EDIT F.FOR"); err != nil {
			t.Errorf("EDIT: %v", err)
		}
	})

	t.Run("blanks past the record go", func(t *testing.T) {
		useEditor(t, `printf '0123456789     \n' > "$1"`)
		shareFindRun(t, d, "EDIT F.FOR")

		if got := editRecords(t, c, "F.FOR"); len(got) != 1 || got[0] != "0123456789" {
			t.Errorf("records %q", got)
		}
	})

	versions := editVersions(t, c, "F.FOR")

	t.Run("text past the record is an error", func(t *testing.T) {
		useEditor(t, `printf 'FINE\n0123456789X\n' > "$1"`)

		err := d.Dispatch("EDIT F.FOR")
		if err == nil {
			t.Fatal("EDIT succeeded")
		}

		for _, want := range []string{"EDIT-E-TOOLONG, line 2 is 11 characters long", "whose records hold 10", "-EDIT-I-KEPT"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("no %q in %q", want, err)
			}
		}

		if got := editVersions(t, c, "F.FOR"); got != versions {
			t.Errorf("%d versions, was %d", got, versions)
		}

		kept := err.Error()[strings.Index(err.Error(), "is in ")+len("is in "):]
		if !strings.HasPrefix(kept, scratch) {
			t.Fatalf("kept in %q, not %q", kept, scratch)
		}

		if data, err := os.ReadFile(kept); err != nil || string(data) != "FINE\n0123456789X\n" {
			t.Errorf("the kept text is %q, %v", data, err)
		}
	})

	buf.Reset()
}

// TestEditLeavesFileAlone is every way out of the editor that makes no new version.
func TestEditLeavesFileAlone(t *testing.T) {
	d, c, buf, scratch := editConsole(t)

	editLoad(t, d, "F.TXT", "ONE\n", "/RECORD_FORMAT=VARIABLE")

	cases := []struct {
		name, script string
		fails        string // what the error says, if EDIT fails
	}{
		{"quitting without writing", `exit 0`, ""},
		{"removing the file", `rm "$1"`, ""},
		{"failing without writing", `exit 3`, "EDIT-E-EDITOR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useEditor(t, tc.script)
			buf.Reset()

			err := d.Dispatch("EDIT F.TXT")

			if tc.fails == "" {
				if err != nil {
					t.Fatalf("EDIT: %v", err)
				}

				if !strings.Contains(buf.String(), "EDIT-I-NOCHANGE") {
					t.Errorf("EDIT said %q", buf.String())
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.fails) {
				t.Errorf("EDIT: %v, want %s", err, tc.fails)
			}

			if got := editVersions(t, c, "F.TXT"); got != 1 {
				t.Errorf("%d versions", got)
			}
		})
	}

	// A failure with nothing written leaves nothing behind.
	if left, _ := filepath.Glob(filepath.Join(scratch, "*")); len(left) != 0 {
		t.Errorf("the scratch directory holds %q", left)
	}

	t.Run("an editor that isn't there", func(t *testing.T) {
		settings.Set(dclcmd.EditorCommandSetting, "no-such-editor-at-all")

		err := d.Dispatch("EDIT F.TXT")
		if err == nil || !strings.Contains(err.Error(), "EDIT-E-EDITOR") {
			t.Errorf("EDIT: %v", err)
		}

		if got := editVersions(t, c, "F.TXT"); got != 1 {
			t.Errorf("%d versions", got)
		}
	})

	t.Run("writing and then failing keeps the text", func(t *testing.T) {
		useEditor(t, `echo CHANGED > "$1"; exit 1`)

		err := d.Dispatch("EDIT F.TXT")
		if err == nil || !strings.Contains(err.Error(), "-EDIT-I-KEPT") {
			t.Fatalf("EDIT: %v", err)
		}

		if got := editVersions(t, c, "F.TXT"); got != 1 {
			t.Errorf("%d versions", got)
		}
	})

	t.Run("writing the same text is writing", func(t *testing.T) {
		useEditor(t, `sleep 0.01; cp "$1" "$1.new"; mv "$1.new" "$1"`)
		shareFindRun(t, d, "EDIT F.TXT")

		if got := editVersions(t, c, "F.TXT"); got != 2 {
			t.Errorf("%d versions", got)
		}
	})
}

// TestEditNewFile is a file that doesn't exist: made as a text file if the editor
// writes it, and not otherwise.
func TestEditNewFile(t *testing.T) {
	d, c, buf, _ := editConsole(t)

	// The editor has no file to start with, as vi would show for a new one.
	useEditor(t, `test -e "$1" && exit 5; exit 0`)
	shareFindRun(t, d, "EDIT NEW.TXT")

	if !strings.Contains(buf.String(), "DUA1:[000000]NEW.TXT not created") {
		t.Errorf("EDIT said %q", buf.String())
	}

	if got := editVersions(t, c, "NEW.TXT"); got != 0 {
		t.Errorf("%d versions", got)
	}

	useEditor(t, `echo HELLO > "$1"`)
	buf.Reset()
	shareFindRun(t, d, "EDIT NEW.TXT")

	if !strings.Contains(buf.String(), "NEW.TXT;1 created, 1 line\n") {
		t.Errorf("EDIT said %q", buf.String())
	}

	buf.Reset()
	shareFindRun(t, d, "DIRECTORY/FULL NEW.TXT")

	for _, want := range []string{"Variable length", "Carriage return"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("no %q in\n%s", want, buf.String())
		}
	}
}

// TestEditScratchName is the file the editor is given: in a directory of its own
// under the scratch area, with the name in lowercase so an editor knows its type,
// and %file wherever the command says.
func TestEditScratchName(t *testing.T) {
	d, _, buf, scratch := editConsole(t)

	editLoad(t, d, "LOGIN.COM", "$ EXIT\n", "/RECORD_FORMAT=VARIABLE")

	script := filepath.Join(t.TempDir(), "editor.sh")
	writeHostFile(t, script, `echo "ARGS $*"`+"\n")

	settings.Set(dclcmd.EditorCommandSetting, `sh `+script+` --before "two words" +%file+ --after`)
	shareFindRun(t, d, "EDIT LOGIN.COM")

	out := buf.String()
	if !strings.Contains(out, "ARGS --before two words +"+scratch+"/govax-edit-") ||
		!strings.Contains(out, "/login.com+ --after") {
		t.Errorf("EDIT said %q", out)
	}

	// Without %file, the file goes last.
	buf.Reset()
	settings.Set(dclcmd.EditorCommandSetting, "sh "+script+" --first")
	shareFindRun(t, d, "EDIT LOGIN.COM")

	if !strings.Contains(buf.String(), "ARGS --first "+scratch) {
		t.Errorf("EDIT said %q", buf.String())
	}
}

// TestEditRefused is what never gets as far as an editor.
func TestEditRefused(t *testing.T) {
	d, c, _, _ := editConsole(t)

	// An editor that writes would make a version if EDIT got that far.
	useEditor(t, `echo WRONG > "$1"`)

	editLoad(t, d, "A.TXT", "ONE\n", "/RECORD_FORMAT=VARIABLE")
	editLoad(t, d, "B.TXT", "ONE\n", "/RECORD_FORMAT=VARIABLE")
	editLoad(t, d, "UNDEF.DAT", "ABCD", "/BINARY/RECORD_FORMAT=UNDEFINED")
	editLoad(t, d, "BIN.DAT", "AB\x00D", "/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=4")
	editLoad(t, d, "BIN2.DAT", "AB\nD", "/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=4")
	shareFindRun(t, d, "CREATE/DIRECTORY [SUB]")

	cases := []struct{ name, command, says string }{
		{"a wildcard", "EDIT *.TXT", "AMBIGUOUS"},
		{"a wildcard matching nothing", "EDIT *.NONE", "not found"},
		{"a version that isn't there", "EDIT A.TXT;9", "not found"},
		{"a directory that isn't there", "EDIT [NOPE]A.TXT", ""},
		{"a directory", "EDIT SUB.DIR", "it is a directory"},
		{"undefined records", "EDIT UNDEF.DAT", "its records are undefined"},
		{"a null byte", "EDIT BIN.DAT", "record 1 holds a null byte"},
		{"a line feed", "EDIT BIN2.DAT", "record 1 holds a line feed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := d.Dispatch(tc.command)
			if err == nil {
				t.Fatal("EDIT succeeded")
			}

			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.says)) {
				t.Errorf("%q, want %q", err, tc.says)
			}
		})
	}

	if got := editVersions(t, c, "A.TXT"); got != 1 {
		t.Errorf("%d versions of A.TXT", got)
	}
}

// TestEditReadOnlyVolume finds out before anyone types anything.
func TestEditReadOnlyVolume(t *testing.T) {
	d, c, _, _ := editConsole(t)

	useEditor(t, `exit 9`)

	// A volume with a file on it, mounted again read-only.
	path := filepath.Join(t.TempDir(), "ro.dsk")
	if err := c.cmd().InitializeContainer(path, 400, "ROVOL", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA3:", path, true); err != nil {
		t.Fatal(err)
	}

	editLoad(t, d, "DUA3:[000000]A.TXT", "ONE\n", "/RECORD_FORMAT=VARIABLE")
	shareFindRun(t, d, "DISMOUNT DUA3:")

	if err := c.cmd().Mount("DUA3:", path, false); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"DUA3:[000000]A.TXT", "DUA3:[000000]NEW.TXT"} {
		err := d.Dispatch("EDIT " + name)
		if err == nil || !strings.Contains(err.Error(), "EDIT-E-READONLY") {
			t.Errorf("EDIT %s: %v", name, err)
		}
	}
}

// TestEditKeepsProtection is the old version's protection on the new one.
func TestEditKeepsProtection(t *testing.T) {
	d, _, buf, _ := editConsole(t)

	editLoad(t, d, "A.TXT", "ONE\n", "/RECORD_FORMAT=VARIABLE")
	shareFindRun(t, d, "DIRECTORY/FULL A.TXT")

	protection := func() string {
		for line := range strings.SplitSeq(buf.String(), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "File protection:") {
				return line
			}
		}

		return ""
	}

	before := protection()
	if before == "" {
		t.Fatalf("no protection in\n%s", buf.String())
	}

	useEditor(t, `echo TWO >> "$1"`)
	shareFindRun(t, d, "EDIT A.TXT")
	buf.Reset()
	shareFindRun(t, d, "DIRECTORY/FULL A.TXT;2")

	if after := protection(); after != before {
		t.Errorf("%q, was %q", after, before)
	}
}

// TestEditHostFile is a name on the host's side: the file is replaced, with the
// same rules for when.
func TestEditHostFile(t *testing.T) {
	d, _, buf, _ := editConsole(t)

	host := filepath.Join(t.TempDir(), "notes.txt")
	writeHostFile(t, host, "ONE\n")

	useEditor(t, `exit 0`)
	shareFindRun(t, d, `EDIT "`+host+`"/HOST`)

	if data, _ := os.ReadFile(host); string(data) != "ONE\n" {
		t.Errorf("the file is %q", data)
	}

	useEditor(t, `echo TWO >> "$1"`)
	buf.Reset()
	shareFindRun(t, d, `EDIT "`+host+`"/HOST`)

	if data, _ := os.ReadFile(host); string(data) != "ONE\nTWO\n" {
		t.Errorf("the file is %q", data)
	}

	if !strings.Contains(buf.String(), "created, 2 lines") {
		t.Errorf("EDIT said %q", buf.String())
	}
}

// TestSetEditor is SET EDITOR and SHOW EDITOR.
func TestSetEditor(t *testing.T) {
	d, _, buf, scratch := editConsole(t)

	other := t.TempDir()

	shareFindRun(t, d, `SET EDITOR/COMMAND="vim -n %file"/SCRATCH="`+other+`"`)

	if got := settings.Get(dclcmd.EditorCommandSetting); got != "vim -n %file" {
		t.Errorf("the command is %q", got)
	}

	if got := settings.Get(dclcmd.EditorScratchSetting); got != other {
		t.Errorf("the scratch directory is %q", got)
	}

	shareFindRun(t, d, "SHOW EDITOR")

	if out := buf.String(); !strings.Contains(out, "vim -n %file") || !strings.Contains(out, other) {
		t.Errorf("SHOW EDITOR said %q", out)
	}

	// One qualifier leaves the other value alone, and what can't be used changes
	// neither.
	shareFindRun(t, d, `SET EDITOR/SCRATCH="`+scratch+`"`)

	if got := settings.Get(dclcmd.EditorCommandSetting); got != "vim -n %file" {
		t.Errorf("the command is %q", got)
	}

	if err := d.Dispatch(`SET EDITOR/SCRATCH="` + filepath.Join(other, "nope") + `"/COMMAND="ed"`); err == nil {
		t.Error("a missing scratch directory was taken")
	}

	if got := settings.Get(dclcmd.EditorCommandSetting); got != "vim -n %file" {
		t.Errorf("the command is %q", got)
	}

	if err := d.Dispatch(`SET EDITOR/COMMAND="vi 'x"`); err == nil {
		t.Error("an unclosed quote was taken")
	}

	// An empty value goes back to the default.
	buf.Reset()
	shareFindRun(t, d, `SET EDITOR/COMMAND=""/SCRATCH=""`)
	shareFindRun(t, d, "SHOW EDITOR")

	if out := buf.String(); !strings.Contains(out, "vi %file (the default)") || !strings.Contains(out, "(the default)\n") {
		t.Errorf("SHOW EDITOR said %q", out)
	}
}

// TestEditInSubprocess is EDIT from a subprocess's DCL, which holds the machine as
// process 1's does.
func TestEditInSubprocess(t *testing.T) {
	d, c, _, _ := editConsole(t)

	editLoad(t, d, "A.TXT", "ONE\n", "/RECORD_FORMAT=VARIABLE")
	useEditor(t, `echo TWO >> "$1"`)
	shareFindRun(t, d, "SPAWN EDIT A.TXT")

	if got := editRecords(t, c, "A.TXT"); strings.Join(got, "|") != "ONE|TWO" {
		t.Errorf("records %q", got)
	}
}

// TestEditMessagesSayItOnce is a failure's message, which has its cause once.
func TestEditMessagesSayItOnce(t *testing.T) {
	d, _, _, _ := editConsole(t)

	editLoad(t, d, "A.TXT", "ONE\n", "/RECORD_FORMAT=VARIABLE")
	useEditor(t, `exit 4`)

	err := d.Dispatch("EDIT A.TXT")
	if err == nil || strings.Count(err.Error(), "exit status 4") != 1 {
		t.Errorf("EDIT: %v", err)
	}
}
