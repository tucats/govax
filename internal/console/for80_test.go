package console_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// The Phase 80 probe (docs/PHASE-80 - FORTRAN formatted IO.md, subtask 2):
// testdata/for80's for80.mar calls FORRTL's statement routines, case by case.
// VMS's run of it is FTNRTL's requirement.

var for80Dir = filepath.Join("..", "..", "testdata", "for80")

// for80Programs are the probe's programs that use FORRTL.
var for80Programs = []string{"FOR80", "FOR80E", "FOR80T"}

// copyFor80 copies the probe's sources to the default directory.
func copyFor80(t *testing.T, dispatch func(string) string) {
	t.Helper()

	for _, name := range []string{"for80.mar", "for80d.mar", "for80e.mar", "for80t.mar", "for80r.com", "for80b.mar", "for80c.mar"} {
		src, err := filepath.Abs(filepath.Join(for80Dir, name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST ` + strings.ToUpper(name))
	}
}

// TestFor80Assembles: every program assembles without a message, so the volume
// the VAX runs them from needs no second try.
func TestFor80Assembles(t *testing.T) {
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for80.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 FOR80`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	copyFor80(t, dispatch)

	for _, name := range append([]string{"FOR80D", "FOR80B", "FOR80C"}, for80Programs...) {
		if out := dispatch("MACRO/NOLIST " + name); out != "" {
			t.Errorf("MACRO %s: %s", name, out)
		}
	}
}

// TestFor80RunsOnFORRTL links the programs against VMS's FORRTL.EXE, runs the
// cases, and checks that they reach their end with no driver line lost. FORRTL
// runs over govax's LIBRTL here, so this says nothing of what's right: VMS's run
// does.
func TestFor80RunsOnFORRTL(t *testing.T) {
	disk, file := findFORRTL(t)
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for80.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 FOR80`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)
	copyFor80(t, dispatch)

	opt := filepath.Join(t.TempDir(), "for80.opt")
	if err := os.WriteFile(opt, []byte("FORRTL/SHAREABLE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + opt + `"/HOST FOR80.OPT`)

	for _, name := range for80Programs {
		if out := dispatch("MACRO/NOLIST " + name); out != "" {
			t.Fatalf("MACRO %s: %s", name, out)
		}

		if out := dispatch("LINK/NOMAP " + name + ",FOR80/OPTIONS"); out != "" {
			t.Fatalf("LINK %s: %s", name, out)
		}
	}

	dispatch("MACRO/NOLIST FOR80D")
	dispatch("LINK/NOMAP FOR80D")
	dispatch("DEFINE/NOLOG FOR006 FOR80.DAT")

	if out := dispatch("@FOR80R"); strings.Contains(out, "was lost") {
		t.Errorf("a driver line was lost: %s", out)
	}

	dispatch("DEASSIGN FOR006")

	got := dispatch("RUN FOR80D")
	for _, want := range []string{"|##### the end of the cases|", "|M127 001883FA"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in the output", want)
		}
	}

	// A case's values, the manual's own list-directed example (7-19).
	if !strings.Contains(got, "| v7=0000000E|") {
		t.Errorf("no K=14 from the manual's list-directed READ")
	}

	checkFor80Namelist(t, got)
}

// for80Address masks what depends on where the images are: a case's PC.
var for80Address = regexp.MustCompile(`^(\|=\d+ r=\w+ e=\d+ \w+ \w+ \d+ )\w+|^\|=c \w+`)

// checkFor80Namelist holds the namelist cases, run on VMS's FORRTL over govax's
// LIB$TPARSE (Phase 80's subtask 10), to VMS's output, up to where VMS's run
// ended. FORRTL's state tables are VMS's macros' own, so this checks that the
// parser reads the real tables' layout and follows their grammar.
func checkFor80Namelist(t *testing.T, got string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(for80Dir, "vax", "for80-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	mask := func(line string) string { return for80Address.ReplaceAllString(line, "$1") }

	var want []string

	for line := range strings.SplitSeq(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		if line != "" {
			want = append(want, line)
		}
	}

	section := func(lines []string) []string {
		return lines[slices.IndexFunc(lines, func(s string) bool { return strings.Contains(s, "##### Namelist") }):]
	}

	// The output's lines each have bars around them, and dispatch joins them with
	// another.
	var lines []string
	for line := range strings.SplitSeq(strings.Trim(got, "|"), "|||") {
		lines = append(lines, "|"+line+"|")
	}

	have := section(lines)
	want = section(want)

	if len(have) < len(want) {
		t.Fatalf("%d lines of namelist cases, want %d", len(have), len(want))
	}

	for i, w := range want {
		if mask(have[i]) != mask(w) {
			t.Errorf("namelist line %d:\n vms   %q\n govax %q", i+1, w, have[i])
		}
	}
}

// for80MessageLine says whether a line of run 3 is one the test compares: the
// conditions' messages, FOR80E's own line, the field before OUTCONERR's
// message, and the statuses. The tracebacks' addresses are VMS's images'.
func for80MessageLine(line string) bool {
	for _, prefix := range []string{"%FOR-", "-FOR-", "-RMS-", "  unit ", "  user PC ", "FOR80E:", "**", "  $STATUS"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}

	return false
}

// TestFor80Messages runs FOR80.COM's run 3, each FOR condition with no handler,
// against VMS's FORRTL.EXE, and holds the messages and statuses to VMS's log:
// the FOR texts are vmsdef's (forMessages), and $PUTMSG's walk of FORRTL's
// signal arrays is govax's.
func TestFor80Messages(t *testing.T) {
	disk, file := findFORRTL(t)
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for80.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 FOR80`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)
	copyFor80(t, dispatch)

	log, err := os.ReadFile(filepath.Join(for80Dir, "vax", "for80.log"))
	if err != nil {
		t.Fatal(err)
	}

	// Run 3 is the log from FOR80E's symbol to the DIRECTORY after it. VMS
	// writes a message of several lines as one record, CR LF between them.
	text := strings.ReplaceAll(string(log), "\r", "")
	start := strings.Index(text, "$ FOR80E := ")
	end := strings.Index(text, "$ DIRECTORY/SIZE FOR*.*")

	if start < 0 || end < start {
		t.Fatal("no run 3 in VMS's log")
	}

	var proc strings.Builder

	var want []string

	proc.WriteString("$ SET NOON\n$ FOR80E := $DUA1:[000000]FOR80E\n")

	for _, line := range strings.Split(text[start:end], "\n")[1:] {
		switch {
		case strings.HasPrefix(line, "$ FOR80E ") || strings.HasPrefix(line, "$ SHOW SYMBOL"):
			proc.WriteString(line + "\n")
		case line == "ABC":
			proc.WriteString(line + "\n") // part F's record
		case for80MessageLine(line):
			want = append(want, line)
		}
	}

	host := filepath.Join(t.TempDir(), "run3.com")
	opt := filepath.Join(t.TempDir(), "for80.opt")

	for name, body := range map[string]string{host: proc.String(), opt: "FORRTL/SHAREABLE\n"} {
		if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	dispatch(`COPY "` + host + `"/HOST RUN3.COM`)
	dispatch(`COPY "` + opt + `"/HOST FOR80.OPT`)

	if out := dispatch("MACRO/NOLIST FOR80E"); out != "" {
		t.Fatalf("MACRO: %s", out)
	}

	if out := dispatch("LINK/NOMAP/TRACEBACK FOR80E,FOR80/OPTIONS"); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	var got []string

	for _, line := range strings.Split(dispatch("@RUN3"), "|") {
		if for80MessageLine(line) {
			got = append(got, line)
		}
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("run 3:\ngovax:\n%s\nVMS:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestFor80Round3 runs round 3 of the probe, what rounds 1 and 2 left open of
// internal files, ENCODE, and DECODE, on VMS's FORRTL.EXE over govax, and holds
// it to vax/for80c-forrtl.txt, that run's output, which TestFor80InternalOracle
// holds FTNRTL to. (FORRTL gave VMS's output over govax in every case of round
// 1's editing sections.) With FOR80C_WRITE set it rewrites the file.
func TestFor80Round3(t *testing.T) {
	disk, file := findFORRTL(t)
	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for80.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 FOR80`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)

	for _, name := range []string{"for80c.mar", "for80d.mar"} {
		path, err := filepath.Abs(filepath.Join(for80Dir, name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + path + `"/HOST ` + strings.ToUpper(name))
	}

	opt := filepath.Join(t.TempDir(), "for80.opt")
	if err := os.WriteFile(opt, []byte("FORRTL/SHAREABLE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + opt + `"/HOST FOR80.OPT`)

	if out := dispatch("MACRO/NOLIST FOR80C"); out != "" {
		t.Fatalf("MACRO FOR80C: %s", out)
	}

	if out := dispatch("LINK/NOMAP FOR80C,FOR80/OPTIONS"); out != "" {
		t.Fatalf("LINK FOR80C: %s", out)
	}

	dispatch("MACRO/NOLIST FOR80D")
	dispatch("LINK/NOMAP FOR80D")
	dispatch("DEFINE/NOLOG FOR006 FOR80.DAT")

	if out := dispatch("RUN FOR80C"); out != "" {
		t.Errorf("RUN FOR80C said: %s", out)
	}

	dispatch("DEASSIGN FOR006")
	dispatch("DEFINE/USER_MODE SYS$OUTPUT FOR80C.TXT")

	if out := dispatch("RUN FOR80D"); out != "" {
		t.Errorf("RUN FOR80D said: %s", out)
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]FOR80C.TXT"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	var run strings.Builder

	for _, record := range records {
		run.Write(record)
		run.WriteByte('\n')
	}

	got := run.String()

	path := filepath.Join(for80Dir, "vax", "for80c-forrtl.txt")
	if os.Getenv("FOR80C_WRITE") != "" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if got != string(want) {
		t.Errorf("FORRTL's run differs from %s:\n%s", path, got)
	}
}
