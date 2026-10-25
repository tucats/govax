package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTrace73Procedure runs testdata/trace73's TRACE73.COM on govax as it
// ran on VMS 7.1: on a fresh volume, building the images with govax's
// MACRO and LINK, each debugger session reading its commands from
// DBG$INPUT and logging itself with SET LOG and SET OUTPUT LOG,VERIFY
// (docs/PHASE-73 - trace trap.md, subtask 5). The session logs that VMS's
// match (trace73Identical) are held to them whole; the others differ only
// where internal/debugger's session oracle lists, and are checked for
// being logged at all. RUN/DEBUG's $STATUS is VMS's, and the volume
// dismounts afterward: no session leaves a file open.
func TestTrace73Procedure(t *testing.T) {
	sources := append([]string{"trace73.mar", "trace73.com"}, withType(trace73Sessions, ".dbg")...)
	out := runProbeProcedure(t, trace73Dir, sources, "TRACE73", trace73Sessions)

	compareSessions(t, out, trace73Dir, trace73Sessions, trace73Identical)

	// Every RUN/DEBUG's $STATUS: the image's, marked STS$M_INHIB_MSG,
	// or the DEBUG status of an EXIT with the image still running.
	got := runDebugStatuses(readLog(t, filepath.Join(out, "trace73.log")), "TRACE73D")
	want := runDebugStatuses(readLog(t, filepath.Join(trace73Dir, "vax", "trace73.log")), "TRACE73D")

	if len(want) != len(trace73Sessions) || strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("RUN/DEBUG statuses %v, want %v", got, want)
	}
}

// TestDbgcmdProcedure runs testdata/dbgcmd's DBGCMD.COM, Phase 42's
// probe of the debugger, on govax as it ran on VMS 7.3, each session
// reading DBG$INPUT and logging itself. The sessions with no difference
// from VMS's in internal/debugger's session oracle (dbgcmdIdentical) are
// held to VMS's logs whole; the others must have been logged.
func TestDbgcmdProcedure(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "dbgcmd")
	sessions := []string{"break", "brkcls", "step", "watch", "trace", "exam", "call", "except", "errors"}
	sources := append([]string{"dbgcmd.mar", "dbgcmd.com"}, withType(sessions, ".dbg")...)

	out := runProbeProcedure(t, dir, sources, "DBGCMD", sessions)
	compareSessions(t, out, dir, sessions, map[string]bool{"brkcls": true, "watch": true, "trace": true})
}

// runProbeProcedure copies a probe's sources from dir to a fresh volume,
// runs @proc/OUTPUT=proc.LOG there, copies each session's .DLG and the
// log back to a host directory, which it returns, and checks that the
// volume dismounts: nothing the procedure ran left a file open.
func runProbeProcedure(t *testing.T, dir string, sources []string, proc string, sessions []string) string {
	t.Helper()

	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "probe.dsk")
	out := t.TempDir()

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 PROBE`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, name := range sources {
		src, err := filepath.Abs(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST DUA1:[000000]` + strings.ToUpper(name))
	}

	dispatch("@" + proc + "/OUTPUT=" + proc + ".LOG")

	for _, name := range sessions {
		dispatch(`COPY ` + strings.ToUpper(name) + `.DLG "` + filepath.Join(out, name+".dlg") + `"/HOST/QUIET`)
	}

	dispatch(`COPY ` + proc + `.LOG "` + filepath.Join(out, strings.ToLower(proc)+".log") + `"/HOST/QUIET`)

	if got := dispatch("DISMOUNT DUA1"); strings.Contains(got, "DISM-W") {
		t.Errorf("DISMOUNT after the procedure: %s", got)
	}

	return out
}

// compareSessions holds each session's log in out to VMS's in dir's vax/:
// the identical ones line for line, the others for having been logged,
// from the line that ran their commands to the EXIT after them.
func compareSessions(t *testing.T, out, dir string, sessions []string, identical map[string]bool) {
	t.Helper()

	for _, name := range sessions {
		got := readLog(t, filepath.Join(out, name+".dlg"))
		want := readLog(t, filepath.Join(dir, "vax", name+".dlg"))

		if !identical[name] {
			if len(got) == 0 || got[0] != "@"+strings.ToUpper(name)+".DBG" || got[len(got)-1] != "EXIT" {
				t.Errorf("%s.dlg wasn't logged as VMS's was:\n%s", name, strings.Join(got, "\n"))
			}

			continue
		}

		if g, w := strings.Join(got, "\n"), strings.Join(want, "\n"); g != w {
			t.Errorf("%s.dlg:\ngot:\n%s\nwant:\n%s", name, g, w)
		}
	}
}

// withType is names, each with typ after it.
func withType(names []string, typ string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = n + typ
	}

	return out
}

// trace73Sessions are TRACE73.COM's debugger sessions, in its order.
var trace73Sessions = []string{"trstep", "trgo", "trexc", "trhand", "trcall", "trsvc", "trdep", "trdepgo", "trunh", "trpriv"}

// trace73Identical are the sessions whose logs are VMS's line for line.
// The others differ where govax pauses in the dispatcher at an exception
// break (TREXC, TRUNH), SHOW CALLS in a handler lacks the "above condition
// handler" lines (TREXC, TRHAND), and VMS's own debugger code was traced
// (TRSVC): internal/debugger's session oracle lists each.
var trace73Identical = map[string]bool{
	"trstep": true, "trgo": true, "trcall": true, "trdep": true, "trdepgo": true, "trpriv": true,
}

// readLog reads a log's lines, without carriage returns or trailing
// blanks.
func readLog(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var lines []string

	for _, l := range strings.Split(strings.TrimRight(strings.ReplaceAll(string(data), "\r", ""), "\n"), "\n") {
		lines = append(lines, strings.TrimRight(l, " "))
	}

	return lines
}

// runDebugStatuses are the $STATUS lines after each RUN/DEBUG of image in
// a procedure's log.
func runDebugStatuses(lines []string, image string) []string {
	var statuses []string

	for i, l := range lines {
		if strings.TrimSpace(l) != "$   RUN/DEBUG "+image {
			continue
		}

		for _, after := range lines[i+1:] {
			if s, ok := strings.CutPrefix(strings.TrimSpace(after), "$STATUS == "); ok {
				statuses = append(statuses, s)

				break
			}
		}
	}

	return statuses
}
