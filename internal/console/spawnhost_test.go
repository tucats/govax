package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spawnHostError runs line, which must fail, and returns its message.
func spawnHostError(t *testing.T, d *Dispatcher, line string) string {
	t.Helper()

	err := d.Dispatch(line)
	if err == nil {
		t.Fatalf("%s: no error", line)
	}

	return err.Error()
}

// TestSpawnHostOutput: the program runs with its words as they were typed,
// quotes keeping one together, and its output is DCL's.
func TestSpawnHostOutput(t *testing.T) {
	d, _, buf := shareFindConsole(t, false)

	shareFindRun(t, d, `SPAWN/HOST echo Hello "two  words"`)

	if got := buf.String(); !strings.Contains(got, "Hello two  words\n") {
		t.Errorf("output = %q", got)
	}
}

// TestSpawnHostEnvironment: /SYMBOLS and /LOGICAL_NAMES pass names with "$"
// made "_", and only when they're given.
func TestSpawnHostEnvironment(t *testing.T) {
	d, _, buf := shareFindConsole(t, false)

	shareFindRun(t, d, `DEFINE PARTS$DATA DUA1:[PARTS]`)
	shareFindRun(t, d, `DEFINE/USER_MODE PARTS$LIST ONE,TWO`)
	shareFindRun(t, d, `MY$SYMBOL == "a value"`)

	if got := spawnHostError(t, d, `SPAWN/HOST printenv GOVAX_LOG_PARTS_DATA`); !strings.Contains(got, "HOSTCMD") {
		t.Errorf("no /LOGICALS: %q", got)
	}

	if got := spawnHostError(t, d, `SPAWN/HOST/LOGICALS printenv GOVAX_SYM_MY_SYMBOL`); !strings.Contains(got, "HOSTCMD") {
		t.Errorf("/LOGICALS passed a symbol: %q", got)
	}

	buf.Reset()
	shareFindRun(t, d, `SPAWN/HOST/LOGICALS printenv GOVAX_LOG_PARTS_DATA`)
	shareFindRun(t, d, `SPAWN/HOST/LOG printenv GOVAX_LOG_PARTS_LIST`)
	shareFindRun(t, d, `SPAWN/HOST/SYMBOLS printenv GOVAX_SYM_MY_SYMBOL`)

	want := "DUA1:[PARTS]\nONE,TWO\na value\n"
	if got := buf.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestSpawnHostFiles: /INPUT and /OUTPUT are host files and volume files.
func TestSpawnHostFiles(t *testing.T) {
	d, c, _ := shareFindConsole(t, false)
	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")

	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.txt"), filepath.Join(dir, "out.txt")
	writeHostFile(t, in, "from the host\n")

	// Host to volume.
	shareFindRun(t, d, `SPAWN/HOST/INPUT="`+in+`"/OUTPUT=COPIED.TXT cat`)

	if got := strings.Join(editRecordTexts(t, c, "COPIED.TXT"), "|"); got != "from the host" {
		t.Errorf("volume file = %q", got)
	}

	// Volume to host.
	shareFindRun(t, d, `SPAWN/HOST/INPUT=COPIED.TXT/OUTPUT="`+out+`" tr a-z A-Z`)

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != "FROM THE HOST\n" {
		t.Errorf("host file = %q", got)
	}
}

// TestSpawnHostRefusals: what can't be asked of a host program.
func TestSpawnHostRefusals(t *testing.T) {
	d, _, _ := shareFindConsole(t, false)

	for _, line := range []string{`SPAWN/HOST/PROCESS=X true`, `SPAWN/HOST/PROMPT=X true`, `SPAWN/HOST`} {
		if got := spawnHostError(t, d, line); strings.Contains(got, "HOSTCMD") {
			t.Errorf("%s: %q", line, got)
		}
	}

	if got := spawnHostError(t, d, `SPAWN/HOST /no/such/program`); !strings.Contains(got, "NO") {
		t.Errorf("a path unquoted is a qualifier: %q", got)
	}

	if got := spawnHostError(t, d, `SPAWN/HOST "/no/such/program"`); !strings.Contains(got, "HOSTCMD") {
		t.Errorf("no such program: %q", got)
	}

	if got := spawnHostError(t, d, `SPAWN/HOST sh -c "exit 3"`); !strings.Contains(got, "exit status 3") {
		t.Errorf("exit status: %q", got)
	}
}

// TestSpawnHostNoWait: SPAWN returns at once, and the output and the volume
// file come at a later poll.
func TestSpawnHostNoWait(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)
	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	buf.Reset()

	shareFindRun(t, d, `SPAWN/HOST/NOWAIT sh -c "sleep 0.2; echo late; exit 2"`)
	shareFindRun(t, d, `SPAWN/HOST/NOWAIT/OUTPUT=LATER.TXT sh -c "sleep 0.2; echo saved"`)

	if got := buf.String(); strings.Contains(got, "late") {
		t.Fatalf("SPAWN waited: %q", got)
	}

	shareFindRun(t, d, "WAIT 00:00:01")

	got := buf.String()
	if !strings.Contains(got, "late\n") || !strings.Contains(got, "exit status 2") {
		t.Errorf("output = %q", got)
	}

	if got := strings.Join(editRecordTexts(t, c, "LATER.TXT"), "|"); got != "saved" {
		t.Errorf("volume file = %q", got)
	}
}

// TestSpawnHostNoWaitEnds: a program still running when govax ends is ended,
// and what it wrote to a volume file is saved.
func TestSpawnHostNoWaitEnds(t *testing.T) {
	d, c, _ := shareFindConsole(t, false)
	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")

	shareFindRun(t, d, `SPAWN/HOST/NOWAIT/OUTPUT=ENDED.TXT sh -c "echo first; exec sleep 30"`)
	shareFindRun(t, d, "WAIT 00:00:00.30")

	c.EndHostJobs()

	if got := strings.Join(editRecordTexts(t, c, "ENDED.TXT"), "|"); got != "first" {
		t.Errorf("volume file = %q", got)
	}
}

// editRecordTexts reads a text volume file's lines.
func editRecordTexts(t *testing.T, c *Console, spec string) []string {
	t.Helper()

	return editRecords(t, c, spec)
}
