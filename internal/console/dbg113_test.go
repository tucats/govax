package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 113's debugger round (docs/planning/PHASE-113 - Fortran errata.md,
// subtask 12c): testdata/dbg113's DBG113.COM asks VMS's debugger what Phases
// 42, 85, and 89 left unconfirmed, in eleven sessions on one program. It goes
// to the VAX on testdata/err113's volume, in [DBG113]. TestDbg113Procedure
// rehearses it on govax, as the VAX will run it, so a mistake in a session
// shows first; once VMS's logs are in testdata/dbg113/vax, it logs each
// session's differences, for subtask 13 to settle.

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

		vax := filepath.Join(dir, "vax", name+".dlg")
		if _, err := os.Stat(vax); err != nil {
			t.Logf("%s.dlg (no VMS run yet):\n%s", name, strings.Join(got, "\n"))

			continue
		}

		if want := readLog(t, vax); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Logf("%s.dlg differs from VMS's:\ngovax:\n%s\nVMS:\n%s", name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}

	t.Logf("DBG113.LOG:\n%s", strings.Join(log, "\n"))
}
