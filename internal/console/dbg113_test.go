package console_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Phase 113's debugger round (docs/planning/PHASE-113 - Fortran errata.md,
// subtask 12c): testdata/dbg113's DBG113.COM asks VMS's debugger what Phases
// 42, 85, and 89 left unconfirmed, in eleven sessions on one program. It goes
// to the VAX on testdata/err113's volume, in [DBG113]. TestDbg113Procedure
// rehearses it on govax, as the VAX ran it, and holds each session's log to
// VMS 7.1's in testdata/dbg113/vax (subtask 13f), stack addresses masked,
// but for dbg113Differences.

// dbg113Sessions are DBG113.COM's sessions, in its order; the last runs
// DBG113S on GVDBG113, govax's own image.
var dbg113Sessions = []string{
	"dbg113s", "dbg113a", "dbg113i", "dbg113t", "dbg113r", "dbg113n",
	"dbg113o", "dbg113l", "dbg113x", "dbg113m", "dbg113q", "gvdbg113s",
}

func TestDbg113Procedure(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "dbg113")
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "dbg113.dsk")
	out := t.TempDir()

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 DBG113`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		name := e.Name()
		if ext := filepath.Ext(name); ext != ".mar" && ext != ".dbg" && ext != ".com" {
			continue
		}

		src, err := filepath.Abs(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST DUA1:[000000]` + strings.ToUpper(name))
	}

	// GVDBG113 is govax's image, which EXCHANGE.CMD builds for VMS's debugger
	// to read; here it's the same as the procedure's own.
	dispatch("MACRO/DEBUG/OBJECT=GVDBG113.OBJ DBG113.MAR")
	dispatch("LINK/DEBUG/NOMAP/EXECUTABLE=GVDBG113.EXE GVDBG113.OBJ")

	dispatch("@DBG113/OUTPUT=DBG113.LOG")

	for _, name := range dbg113Sessions {
		dispatch(`COPY ` + strings.ToUpper(name) + `.DLG "` + filepath.Join(out, name+".dlg") + `"/HOST/QUIET`)
	}

	dispatch(`COPY DBG113.LOG "` + filepath.Join(out, "dbg113.log") + `"/HOST/QUIET`)

	if got := dispatch("DISMOUNT DUA1"); strings.Contains(got, "DISM-W") {
		t.Errorf("DISMOUNT after the procedure: %s", got)
	}

	log := readLog(t, filepath.Join(out, "dbg113.log"))

	// The image built without a message, and every session ran.
	for _, l := range log {
		if strings.HasPrefix(l, "%MACRO-") || strings.HasPrefix(l, "%LINK-") {
			t.Errorf("the build: %s", l)
		}
	}

	if got := len(runDebugStatuses(log, "DBG113")); got != len(dbg113Sessions)-1 {
		t.Errorf("%d RUN/DEBUG DBG113 statuses in the log, want %d", got, len(dbg113Sessions)-1)
	}

	for _, name := range dbg113Sessions {
		got := readLog(t, filepath.Join(out, name+".dlg"))
		script := strings.ToUpper(strings.TrimPrefix(name, "gv"))
		// DBG113Q ends with its own QUIT, which may not be logged (the round
		// asks); the others with the procedure's EXIT.
		ended := len(got) > 0 && (name == "dbg113q" || got[len(got)-1] == "EXIT")

		if len(got) == 0 || got[0] != "@"+script+".DBG" || !ended {
			t.Errorf("%s.dlg wasn't logged whole:\n%s", name, strings.Join(got, "\n"))
		}

		// A command govax's debugger doesn't read is a mistake in the session
		// unless the round asks about it.
		for _, l := range got {
			if strings.Contains(l, "%DEBUG-E-SYNTAX") || strings.Contains(l, "%DEBUG-W-IVVERB") {
				t.Logf("%s: %s", name, l)
			}
		}

		want := readLog(t, filepath.Join(dir, "vax", name+".dlg"))

		for _, h := range logHunks(maskStack(want), maskStack(got)) {
			if !dbg113Explained(name, h) {
				t.Errorf("%s.dlg differs from VMS's:\nVMS:\n%s\ngovax:\n%s",
					name, strings.Join(h.vms, "\n"), strings.Join(h.govax, "\n"))
			}
		}
	}

	// EXIT at a break ran the exit handler, as on VMS, after every session
	// but DBG113X's, whose program ended by itself.
	handlers := 0

	for _, l := range log {
		if l == "DBG113: the exit handler ran" {
			handlers++
		}
	}

	if handlers != len(dbg113Sessions) {
		t.Errorf("the exit handler ran %d times, want %d", handlers, len(dbg113Sessions))
	}

}

// dbg113Differences are what may differ from VMS's logs, by session: a
// hunk is explained when one of its lines, either side, holds one of the
// texts.
var dbg113Differences = map[string][]string{
	// R11 as VMS's image activator left it, and the bytes past R11, AP's
	// and FP's, which hold stack addresses. DBG113S's DEPOSIT PC = DEEP
	// made LEAF return to 005001D0: VMS's P0 ends below it, an access
	// violation; govax's is larger, so the zeros there are a reserved
	// instruction (OPCDEC), and at the break govax is in its condition
	// dispatcher, whose PSL (IV clear) is shown.
	"dbg113s":   {"%R11:", "005001D0", "USER   USER    0  0  0  1 0 0 0 0 0"},
	"gvdbg113s": {"%R11:", "005001D0", "USER   USER    0  0  0  1 0 0 0 0 0"},
	"dbg113i":   {"%R11:"},
	// SYS$LOGIN's translation is the account's directory.
	"dbg113m": {"= SYS$SYSROOT:[SYSMGR]"},
	// SHOW CALLS at a break on an exception: govax is paused in the
	// condition dispatcher (docs/planning/PHASE-42.md, subtask 5).
	"dbg113x": {"Saved PC", "module name"},
}

// dbg113Explained reports whether hunk h of session name's log is one
// dbg113Differences explains.
func dbg113Explained(name string, h logHunk) bool {
	for _, text := range dbg113Differences[name] {
		for _, l := range append(append([]string{}, h.vms...), h.govax...) {
			if strings.Contains(l, text) {
				return true
			}
		}
	}

	return false
}

// stackAddress matches a user stack address, VMS's (7FExxxxx) or govax's
// (7FDxxxxx), which differ for reasons not the debugger's.
var stackAddress = regexp.MustCompile(`\b7F[DE][0-9A-F]{5}\b`)

// maskStack replaces stack addresses with <stack>.
func maskStack(lines []string) []string {
	out := make([]string, len(lines))

	for i, l := range lines {
		out[i] = stackAddress.ReplaceAllString(l, "<stack>")
	}

	return out
}

// logHunk is one place two logs differ: VMS's lines and govax's there.
type logHunk struct {
	vms, govax []string
}

// logHunks compares two logs line by line (a longest common subsequence)
// and returns where they differ.
func logHunks(vms, govax []string) []logHunk {
	n, m := len(vms), len(govax)
	common := make([][]int, n+1)

	for i := range common {
		common[i] = make([]int, m+1)
	}

	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if vms[i] == govax[j] {
				common[i][j] = common[i+1][j+1] + 1
			} else {
				common[i][j] = max(common[i+1][j], common[i][j+1])
			}
		}
	}

	var (
		hunks []logHunk
		cur   logHunk
	)

	flush := func() {
		if len(cur.vms)+len(cur.govax) > 0 {
			hunks = append(hunks, cur)
			cur = logHunk{}
		}
	}

	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && vms[i] == govax[j]:
			flush()
			i++
			j++
		case j < m && (i == n || common[i][j+1] >= common[i+1][j]):
			cur.govax = append(cur.govax, govax[j])
			j++
		default:
			cur.vms = append(cur.vms, vms[i])
			i++
		}
	}

	flush()

	return hunks
}
