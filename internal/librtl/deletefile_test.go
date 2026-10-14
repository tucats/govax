package librtl

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

var (
	rmsIFI = vmsdef.Symbols["RMS$_IFI"]
	fabFAC = vmsdef.Symbols["FAB$B_FAC"]
	fabPUT = vmsdef.Symbols["FAB$M_PUT"]
)

// confirmRoutine is the confirm routine's address in the tests, beside
// successRoutine and errorRoutine.
const confirmRoutine = 0x3200

// deleteLog records LIB$DELETE_FILE's callbacks: "?name" for each
// confirm, "+name" for each success, and "-name STS source" for each
// error, each followed by " u=%X" when the call had the user argument.
type deleteLog struct {
	t   *testing.T
	env *corevms.Environment

	calls []string

	// refuse names the files the confirm routine refuses; errorStatus is
	// what the error routine returns; onConfirm, when set, runs in the
	// confirm routine with the FAB it was given.
	refuse      map[string]bool
	errorStatus uint32
	onConfirm   func(fab uint32)
}

func newDeleteLog(t *testing.T, env *corevms.Environment) *deleteLog {
	return &deleteLog{t: t, env: env, refuse: map[string]bool{}, errorStatus: 1}
}

// procedure answers the callbacks: the name is the first argument's
// descriptor's text, trailing blanks removed.
func (l *deleteLog) procedure(routine uint32, args []uint32) uint32 {
	t, env := l.t, l.env
	t.Helper()

	name := strings.TrimRight(descText(t, env, args[0]), " ")

	user := func(n int) string {
		if len(args) > n {
			return fmt.Sprintf(" u=%X", args[n])
		}

		return ""
	}

	switch routine {
	case confirmRoutine:
		fab := args[1]
		if fns, fna := byteAt(t, env, fab+fabFNS), longword(t, env, fab+fabFNA); fns == 0 || fna == 0 {
			t.Errorf("the confirm routine's FAB names no file")
		}

		l.calls = append(l.calls, "?"+name+user(2))

		if l.onConfirm != nil {
			l.onConfirm(fab)
		}

		if l.refuse[name] {
			return 0
		}

		return 1
	case successRoutine:
		l.calls = append(l.calls, "+"+name+user(1))

		return 0 // ignored
	case errorRoutine:
		sts, source := longword(t, env, args[1]), longword(t, env, args[3])
		l.calls = append(l.calls, fmt.Sprintf("-%s %s %d%s", name, hexStatus(sts), source, user(4)))

		return l.errorStatus
	}

	t.Errorf("callback of %#x", routine)

	return 0
}

// deleteFile runs LIB$DELETE_FILE with argv, its strings already in a.
func (l *deleteLog) deleteFile(a *arena, argv ...uint32) uint32 {
	l.t.Helper()

	return callingBack(l.t, l.env, a, "LIB$DELETE_FILE", l.procedure, argv...)
}

// deleteCall runs LIB$DELETE_FILE with argv in a stub frame, with no
// callbacks expected.
func deleteCall(t *testing.T, env *corevms.Environment, a *arena, argv ...uint32) uint32 {
	t.Helper()

	return callingBack(t, env, a, "LIB$DELETE_FILE", func(routine uint32, _ []uint32) uint32 {
		t.Errorf("unexpected callback of %#x", routine)

		return 0
	}, argv...)
}

// exists says whether name is a file.
func exists(t *testing.T, env *corevms.Environment, name string) bool {
	t.Helper()

	// An arena of its own, clear of the test's.
	a := &arena{t: t, env: env, next: 0x80000}
	f := &finder{t: t, env: env, a: a, cell: a.long(0), stv: a.long(0)}
	f.result = a.desc(strings.Repeat(" ", 80))

	st, _ := f.find(name, "", "", findNoWild)
	f.end()

	return st&1 != 0
}

// TestDeleteFileWildcard: each file a wildcard name matches is offered to
// the confirm routine, then deleted and reported to the success routine;
// a file the confirm routine refuses stays; every routine gets the user
// argument.
func TestDeleteFileWildcard(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	l := newDeleteLog(t, env)
	l.refuse["DUA0:[000000]B.TXT;1"] = true

	st := l.deleteFile(a, a.desc("DUA0:[000000]*.TXT"), 0, 0, successRoutine, errorRoutine, confirmRoutine, 0x55)

	want := []string{
		"?DUA0:[000000]A.TXT;1 u=55", "+DUA0:[000000]A.TXT;1 u=55",
		"?DUA0:[000000]B.TXT;1 u=55",
	}
	if st != ssNormal || !slices.Equal(l.calls, want) {
		t.Errorf("DELETE_FILE = %#x %q, want SS$_NORMAL %q", st, l.calls, want)
	}

	if exists(t, env, "DUA0:[000000]A.TXT") || !exists(t, env, "DUA0:[000000]B.TXT") {
		t.Error("A.TXT should be gone and B.TXT kept")
	}
}

// TestDeleteFileOne: a single file, with no routines, the default name
// filling in what filespec leaves out.
func TestDeleteFileOne(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)

	if st := deleteCall(t, env, a, a.desc("C"), a.desc("DUA0:[000000].DAT")); st != ssNormal {
		t.Errorf("DELETE_FILE = %#x", st)
	}

	if exists(t, env, "DUA0:[000000]C.DAT") {
		t.Error("C.DAT is still there")
	}
}

// TestDeleteFileErrors: a missing file is an error searching (source 0);
// with an error routine that returns success the status is
// LIB$_ERRROUCAL, with one that returns failure the RMS status, and with
// none the error itself.
func TestDeleteFileErrors(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	missing := a.desc("DUA0:[000000]NONE.TXT")

	l := newDeleteLog(t, env)
	st := l.deleteFile(a, missing, 0, 0, successRoutine, errorRoutine, 0, 7)

	want := []string{"-DUA0:[000000]NONE.TXT; " + hexStatus(rmsFNF) + " 0 u=7"}
	if st != libErrRouCal || !slices.Equal(l.calls, want) {
		t.Errorf("a missing file = %#x %q, want LIB$_ERRROUCAL %q", st, l.calls, want)
	}

	l = newDeleteLog(t, env)
	l.errorStatus = 0

	if st := l.deleteFile(a, missing, 0, 0, 0, errorRoutine); st != rmsFNF || len(l.calls) != 1 {
		t.Errorf("an error routine that stops = %#x %q, want RMS$_FNF", st, l.calls)
	}

	if st := deleteCall(t, env, a, missing); st != rmsFNF {
		t.Errorf("no error routine = %#x, want RMS$_FNF", st)
	}

	if st := deleteCall(t, env, a, a.desc("DUA0:[NODIR]X.TXT")); st != rmsDNF {
		t.Errorf("a missing directory = %#x, want RMS$_DNF", st)
	}
}

// TestDeleteFileDeleteError: an $ERASE that fails (here the confirm
// routine leaves the FAB open) is error source 1, and the search goes on.
func TestDeleteFileDeleteError(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	l := newDeleteLog(t, env)

	l.onConfirm = func(fab uint32) {
		if err := env.Memory().StoreByte(env.CPU(), fab+fabFAC, byte(fabPUT)); err != nil {
			t.Fatal(err)
		}

		if st, err := env.CallService("SYS$OPEN", []uint32{fab}); err != nil || st&1 == 0 {
			t.Fatalf("$OPEN of the confirm routine's FAB = %#x, %v", st, err)
		}
	}

	st := l.deleteFile(a, a.desc("DUA0:[000000]*.TXT"), 0, 0, successRoutine, errorRoutine, confirmRoutine)

	want := []string{
		"?DUA0:[000000]A.TXT;1", "-DUA0:[000000]A.TXT;1 " + hexStatus(rmsIFI) + " 1",
		"?DUA0:[000000]B.TXT;1", "-DUA0:[000000]B.TXT;1 " + hexStatus(rmsIFI) + " 1",
	}
	if st != libErrRouCal || !slices.Equal(l.calls, want) {
		t.Errorf("DELETE_FILE = %#x %q, want LIB$_ERRROUCAL %q", st, l.calls, want)
	}
}

// TestDeleteFileConfirmOpen: the confirm routine may $OPEN and $CLOSE
// its FAB, and the search goes on.
func TestDeleteFileConfirmOpen(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	l := newDeleteLog(t, env)

	l.onConfirm = func(fab uint32) {
		for _, service := range []string{"SYS$OPEN", "SYS$CLOSE"} {
			if st, err := env.CallService(service, []uint32{fab}); err != nil || st&1 == 0 {
				t.Fatalf("%s of the confirm routine's FAB = %#x, %v", service, st, err)
			}
		}
	}

	st := l.deleteFile(a, a.desc("DUA0:[000000...]*.TXT"), 0, 0, successRoutine, 0, confirmRoutine)

	want := []string{
		"?DUA0:[000000]A.TXT;1", "+DUA0:[000000]A.TXT;1",
		"?DUA0:[000000]B.TXT;1", "+DUA0:[000000]B.TXT;1",
		"?DUA0:[SUB]D.TXT;1", "+DUA0:[SUB]D.TXT;1",
	}
	if st != ssNormal || !slices.Equal(l.calls, want) {
		t.Errorf("DELETE_FILE = %#x %q, want SS$_NORMAL %q", st, l.calls, want)
	}
}

// TestDeleteFileResultant: a resultant-name descriptor gets each file's
// name and is the one the callbacks are given.
func TestDeleteFileResultant(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	l := newDeleteLog(t, env)
	result := a.desc(strings.Repeat(" ", 40))

	var given []uint32

	proc := func(routine uint32, args []uint32) uint32 {
		given = append(given, args[0])

		return l.procedure(routine, args)
	}

	st := callingBack(t, env, a, "LIB$DELETE_FILE", proc,
		a.desc("DUA0:[000000]A.TXT"), 0, 0, successRoutine, 0, confirmRoutine, 0, result)
	if st != ssNormal || !slices.Equal(given, []uint32{result, result}) {
		t.Errorf("DELETE_FILE = %#x, descriptors %#x, want the resultant-name %#x", st, given, result)
	}

	if s := strings.TrimRight(descText(t, env, result), " "); s != "DUA0:[000000]A.TXT;1" {
		t.Errorf("resultant-name = %q", s)
	}
}

// TestDeleteFileContext: with a file-scan context the last file deleted
// is the next call's related name, and FILE_SCAN_END frees the context.
func TestDeleteFileContext(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	cell := a.long(0)

	if st := deleteCall(t, env, a, a.desc("DUA0:[000000]A.TXT"), 0, 0, 0, 0, 0, 0, 0, cell); st != ssNormal {
		t.Fatalf("first DELETE_FILE = %#x", st)
	}

	if st := deleteCall(t, env, a, a.desc("B"), 0, 0, 0, 0, 0, 0, 0, cell); st != ssNormal {
		t.Errorf("second DELETE_FILE = %#x", st)
	}

	if exists(t, env, "DUA0:[000000]B.TXT") {
		t.Error("B.TXT is still there: the context gave no related name")
	}

	if st := call(t, env, "LIB$FILE_SCAN_END", 0, cell); st != ssNormal || longword(t, env, cell) != 0 {
		t.Errorf("FILE_SCAN_END = %#x", st)
	}
}

// TestDeleteFileArguments: no arguments or more than ten are
// LIB$_WRONUMARG, an undefined flag bit LIB$_INVARG, a name over 255
// characters LIB$_INVFILSPE, and a context longword that names none
// LIB$_INVARG.
func TestDeleteFileArguments(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	spec := a.desc("DUA0:[000000]A.TXT")

	cases := []struct {
		name string
		argv []uint32
		want uint32
	}{
		{"no arguments", nil, libWroNumArg},
		{"eleven arguments", make([]uint32, 11), libWroNumArg},
		{"flag bit 3", []uint32{spec, 0, 0, 0, 0, 0, 0, 0, 0, a.long(8)}, libInvArg},
		{"a long name", []uint32{a.desc(strings.Repeat("A", 256))}, libInvFilSpe},
		{"a bad context", []uint32{spec, 0, 0, 0, 0, 0, 0, 0, a.long(0x12340)}, libInvArg},
	}

	for _, c := range cases {
		if st := deleteCall(t, env, a, c.argv...); st != c.want {
			t.Errorf("%s: %#x, want %#x", c.name, st, c.want)
		}
	}

	if !exists(t, env, "DUA0:[000000]A.TXT") {
		t.Error("a refused call deleted A.TXT")
	}

	if st := deleteCall(t, env, a, spec, 0, 0, 0, 0, 0, 0, 0, 0, a.long(4)); st != ssNormal {
		t.Errorf("LONG_NAMES = %#x, want SS$_NORMAL", st)
	}
}

// TestSeverityRank: severe, then error, then warning, then successes.
func TestSeverityRank(t *testing.T) {
	order := []uint32{4, 2, 0, 1}
	for i := 1; i < len(order); i++ {
		if severityRank(order[i-1]) <= severityRank(order[i]) {
			t.Errorf("severity %d should outrank %d", order[i-1], order[i])
		}
	}

	if severityRank(3) != severityRank(1) {
		t.Error("informational and success should rank alike")
	}
}
