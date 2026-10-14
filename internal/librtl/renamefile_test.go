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
	rmsFNMSt = vmsdef.Symbols["RMS$_FNM"]
	rmsENT   = vmsdef.Symbols["RMS$_ENT"]
	rmsFNFSt = vmsdef.Symbols["RMS$_FNF"]
)

// renameLog records LIB$RENAME_FILE's callbacks: "?old>new" for each
// confirm, "+old>new" for each success, and "-old>new STS source" for
// each error, each followed by " u=%X" when the call had the user
// argument.
type renameLog struct {
	t   *testing.T
	env *corevms.Environment

	calls []string

	// refuse names the old files the confirm routine refuses; errorStatus
	// is what the error routine returns.
	refuse      map[string]bool
	errorStatus uint32
}

func newRenameLog(t *testing.T, env *corevms.Environment) *renameLog {
	return &renameLog{t: t, env: env, refuse: map[string]bool{}, errorStatus: 1}
}

// procedure answers the callbacks: the names are the first two
// arguments' descriptors' text, trailing blanks removed.
func (l *renameLog) procedure(routine uint32, args []uint32) uint32 {
	t, env := l.t, l.env
	t.Helper()

	text := func(i int) string { return strings.TrimRight(descText(t, env, args[i]), " ") }
	names := text(0) + ">" + text(1)

	user := func(n int) string {
		if len(args) > n {
			return fmt.Sprintf(" u=%X", args[n])
		}

		return ""
	}

	switch routine {
	case confirmRoutine:
		fab := args[2]
		if fns, fna := byteAt(t, env, fab+fabFNS), longword(t, env, fab+fabFNA); fns == 0 || fna == 0 {
			t.Errorf("the confirm routine's FAB names no file")
		}

		l.calls = append(l.calls, "?"+names+user(3))

		if l.refuse[text(0)] {
			return 0
		}

		return 1
	case successRoutine:
		l.calls = append(l.calls, "+"+names+user(2))

		return 0 // ignored
	case errorRoutine:
		if len(args) != 6 {
			t.Errorf("the error routine got %d arguments, want 6", len(args))
		}

		sts, source := longword(t, env, args[2]), longword(t, env, args[4])
		l.calls = append(l.calls, fmt.Sprintf("-%s %s %d%s", names, hexStatus(sts), source, user(5)))

		return l.errorStatus
	}

	t.Errorf("callback of %#x", routine)

	return 0
}

// rename runs LIB$RENAME_FILE with argv, its strings already in a.
func (l *renameLog) rename(a *arena, argv ...uint32) uint32 {
	l.t.Helper()

	return callingBack(l.t, l.env, a, "LIB$RENAME_FILE", l.procedure, argv...)
}

// renameCall runs LIB$RENAME_FILE with argv in a stub frame, with no
// callbacks expected.
func renameCall(t *testing.T, env *corevms.Environment, a *arena, argv ...uint32) uint32 {
	t.Helper()

	return callingBack(t, env, a, "LIB$RENAME_FILE", func(routine uint32, _ []uint32) uint32 {
		t.Errorf("unexpected callback of %#x", routine)

		return 0
	}, argv...)
}

// TestRenameFileWildcard: each file a wildcard name matches gets its "*"
// fields from the old name; the confirm routine sees the new name before
// its version is known, the success routine after; a refused file stays;
// the user argument goes to each routine.
func TestRenameFileWildcard(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	l := newRenameLog(t, env)
	l.refuse["DUA0:[000000]B.TXT;1"] = true

	st := l.rename(a, a.desc("DUA0:[000000]*.TXT"), a.desc("*.OLD"), 0, 0, 0,
		successRoutine, errorRoutine, confirmRoutine, 0x55)

	want := []string{
		"?DUA0:[000000]A.TXT;1>DUA0:[000000]A.OLD; u=55",
		"+DUA0:[000000]A.TXT;1>DUA0:[000000]A.OLD;1 u=55",
		"?DUA0:[000000]B.TXT;1>DUA0:[000000]B.OLD; u=55",
	}
	if st != ssNormal || !slices.Equal(l.calls, want) {
		t.Errorf("RENAME_FILE = %#x %q, want SS$_NORMAL %q", st, l.calls, want)
	}

	if exists(t, env, "DUA0:[000000]A.TXT") || !exists(t, env, "DUA0:[000000]A.OLD") ||
		!exists(t, env, "DUA0:[000000]B.TXT") {
		t.Error("A.TXT should now be A.OLD, and B.TXT kept")
	}
}

// TestRenameFileDirectory: a new name that gives only a directory moves
// the file there under its own name; the default name fills in the old
// name's type.
func TestRenameFileDirectory(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	l := newRenameLog(t, env)

	st := l.rename(a, a.desc("C"), a.desc("DUA0:[SUB]"), a.desc("DUA0:[000000].DAT"), 0, 0, successRoutine)

	want := []string{"+DUA0:[000000]C.DAT;1>DUA0:[SUB]C.DAT;1 u=0"}
	if st != ssNormal || !slices.Equal(l.calls, want) {
		t.Errorf("RENAME_FILE = %#x %q, want SS$_NORMAL %q", st, l.calls, want)
	}
}

// TestRenameFileVersions: with no version on the new name the file gets
// the new name's next version, even when the old name gave one (VMS
// 7.1); with CUR_VER it keeps its own, which fails when that name exists
// (error source 2).
func TestRenameFileVersions(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)

	l := newRenameLog(t, env)
	if st := l.rename(a, a.desc("DUA0:[000000]A.TXT"), a.desc("B"), 0, 0, 0, successRoutine); st != ssNormal ||
		!slices.Equal(l.calls, []string{"+DUA0:[000000]A.TXT;1>DUA0:[000000]B.TXT;2 u=0"}) {
		t.Errorf("a new version = %#x %q", st, l.calls)
	}

	l = newRenameLog(t, env)
	st := l.rename(a, a.desc("DUA0:[000000]B.TXT;2"), a.desc("D.TXT"), 0, 0, 0, successRoutine)

	if st != ssNormal || !slices.Equal(l.calls, []string{"+DUA0:[000000]B.TXT;2>DUA0:[000000]D.TXT;1 u=0"}) {
		t.Errorf("an explicit version = %#x %q", st, l.calls)
	}

	l = newRenameLog(t, env)
	st = l.rename(a, a.desc("DUA0:[000000]C.DAT"), a.desc("B.TXT"), 0, 0, a.long(renameCurVer), successRoutine, errorRoutine)

	want := []string{"-DUA0:[000000]C.DAT;1>DUA0:[000000]B.TXT;1 " + hexStatus(rmsENT) + " 2 u=0"}
	if st != libErrRouCal || !slices.Equal(l.calls, want) {
		t.Errorf("CUR_VER onto an existing name = %#x %q, want LIB$_ERRROUCAL %q", st, l.calls, want)
	}

	l = newRenameLog(t, env)
	st = l.rename(a, a.desc("DUA0:[000000]D.TXT"), a.desc("E"), 0, 0, a.long(renameCurVer), successRoutine)

	if st != ssNormal || !slices.Equal(l.calls, []string{"+DUA0:[000000]D.TXT;1>DUA0:[000000]E.TXT;1 u=0"}) {
		t.Errorf("CUR_VER = %#x %q", st, l.calls)
	}
}

// TestRenameFileErrors: a missing file is error source 0, with the new
// name as given; a wildcard among a name's characters is RMS$_FNM, at
// source 1; with no error routine the status is the error, and an error
// routine that returns failure stops with the RMS status made severe
// (RMS$_FNM's severity is already).
func TestRenameFileErrors(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)

	l := newRenameLog(t, env)
	st := l.rename(a, a.desc("DUA0:[000000]NONE.TXT"), a.desc("X.TXT"), 0, 0, 0, 0, errorRoutine, 0, 9)

	want := []string{"-DUA0:[000000]NONE.TXT;>X.TXT " + hexStatus(rmsFNFSt) + " 0 u=9"}
	if st != libErrRouCal || !slices.Equal(l.calls, want) {
		t.Errorf("a missing file = %#x %q, want LIB$_ERRROUCAL %q", st, l.calls, want)
	}

	l = newRenameLog(t, env)
	st = l.rename(a, a.desc("DUA0:[000000]*.TXT"), a.desc("X%.TXT"), 0, 0, 0, 0, errorRoutine)

	want = []string{
		"-DUA0:[000000]A.TXT;1>DUA0:[000000]X%.TXT; " + hexStatus(rmsFNMSt) + " 1 u=0",
		"-DUA0:[000000]B.TXT;1>DUA0:[000000]X%.TXT; " + hexStatus(rmsFNMSt) + " 1 u=0",
	}
	if st != libErrRouCal || !slices.Equal(l.calls, want) {
		t.Errorf("a wildcard new name = %#x %q, want LIB$_ERRROUCAL %q", st, l.calls, want)
	}

	l = newRenameLog(t, env)
	l.errorStatus = 0

	if st := l.rename(a, a.desc("DUA0:[000000]*.TXT"), a.desc("X%.TXT"), 0, 0, 0, 0, errorRoutine); st != rmsFNMSt || len(l.calls) != 1 {
		t.Errorf("an error routine that stops = %#x %q, want RMS$_FNM", st, l.calls)
	}

	if st := renameCall(t, env, a, a.desc("DUA0:[000000]NONE.TXT"), a.desc("X")); st != rmsFNFSt {
		t.Errorf("no error routine = %#x, want RMS$_FNF", st)
	}
}

// TestRenameFileResultant: the resultant-name descriptors get the names
// and are the ones the callbacks are given.
func TestRenameFileResultant(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	l := newRenameLog(t, env)
	oldResult, newResult := a.desc(strings.Repeat(" ", 40)), a.desc(strings.Repeat(" ", 40))

	var given [][2]uint32

	proc := func(routine uint32, args []uint32) uint32 {
		given = append(given, [2]uint32{args[0], args[1]})

		return l.procedure(routine, args)
	}

	st := callingBack(t, env, a, "LIB$RENAME_FILE", proc, a.desc("DUA0:[000000]A.TXT"), a.desc("Z"),
		0, 0, 0, successRoutine, 0, confirmRoutine, 0, oldResult, newResult)

	pair := [2]uint32{oldResult, newResult}
	if st != ssNormal || !slices.Equal(given, [][2]uint32{pair, pair}) {
		t.Errorf("RENAME_FILE = %#x, descriptors %#x, want the resultant names %#x", st, given, pair)
	}

	if s := strings.TrimRight(descText(t, env, newResult), " "); s != "DUA0:[000000]Z.TXT;1" {
		t.Errorf("new-resultant-name = %q", s)
	}
}

// TestRenameFileContext: with a file-scan context the last file renamed
// is the next call's related name.
func TestRenameFileContext(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	cell := a.long(0)

	if st := renameCall(t, env, a, a.desc("DUA0:[SUB]D.TXT"), a.desc("D2"), 0, 0, 0, 0, 0, 0, 0, 0, 0, cell); st != ssNormal {
		t.Fatalf("first RENAME_FILE = %#x", st)
	}

	// The context's name is D.TXT's, in [SUB]: no D.TXT is left there,
	// so "D2" names [SUB]D2.TXT.
	if st := renameCall(t, env, a, a.desc("D2"), a.desc("D3"), 0, 0, 0, 0, 0, 0, 0, 0, 0, cell); st != ssNormal {
		t.Errorf("second RENAME_FILE = %#x", st)
	}

	if !exists(t, env, "DUA0:[SUB]D3.TXT") {
		t.Error("[SUB]D3.TXT isn't there: the context gave no related name")
	}

	if st := call(t, env, "LIB$FILE_SCAN_END", 0, cell); st != ssNormal || longword(t, env, cell) != 0 {
		t.Errorf("FILE_SCAN_END = %#x", st)
	}
}

// TestRenameFileArguments: fewer than two arguments or more than twelve
// are LIB$_WRONUMARG, a flag bit past INH_SECUR LIB$_INVARG (the VSI
// manual's LONG_NAMES too, as on VMS 7.1), and a name over 255
// characters LIB$_INVFILSPE; INH_SECUR is taken.
func TestRenameFileArguments(t *testing.T) {
	env := findFixture(t)
	a := newArena(t, env)
	old, newName := a.desc("DUA0:[000000]A.TXT"), a.desc("Q")
	long := a.desc(strings.Repeat("A", 256))

	cases := []struct {
		name string
		argv []uint32
		want uint32
	}{
		{"one argument", []uint32{old}, libWroNumArg},
		{"thirteen arguments", make([]uint32, 13), libWroNumArg},
		{"flag bit 3", []uint32{old, newName, 0, 0, a.long(8)}, libInvArg},
		{"LONG_NAMES", []uint32{old, newName, 0, 0, a.long(4)}, libInvArg},
		{"a long old name", []uint32{long, newName}, libInvFilSpe},
		{"a long new name", []uint32{old, long}, libInvFilSpe},
	}

	for _, c := range cases {
		if st := renameCall(t, env, a, c.argv...); st != c.want {
			t.Errorf("%s: %#x, want %#x", c.name, st, c.want)
		}
	}

	if !exists(t, env, "DUA0:[000000]A.TXT") {
		t.Error("a refused call renamed A.TXT")
	}

	if st := renameCall(t, env, a, old, newName, 0, 0, a.long(2)); st != ssNormal || !exists(t, env, "DUA0:[000000]Q.TXT") {
		t.Errorf("INH_SECUR = %#x, want SS$_NORMAL", st)
	}
}

// TestRenameFileSameName: renaming files to their own names doesn't find
// them again, with a search that has a wildcard context and one that
// hasn't. Each keeps version 1: $RENAME removes the old entry before it
// works out the new name's next version (unconfirmed; a probe case).
func TestRenameFileSameName(t *testing.T) {
	for _, spec := range []string{"DUA0:[000000]*.TXT", "DUA0:[000000...]*.TXT"} {
		env := findFixture(t)
		a := newArena(t, env)
		l := newRenameLog(t, env)

		st := l.rename(a, a.desc(spec), a.desc("*.*"), 0, 0, 0, successRoutine)

		want := []string{
			"+DUA0:[000000]A.TXT;1>DUA0:[000000]A.TXT;1 u=0",
			"+DUA0:[000000]B.TXT;1>DUA0:[000000]B.TXT;1 u=0",
		}
		if strings.Contains(spec, "...") {
			want = append(want, "+DUA0:[SUB]D.TXT;1>DUA0:[SUB]D.TXT;1 u=0")
		}

		if st != ssNormal || !slices.Equal(l.calls, want) {
			t.Errorf("%s: RENAME_FILE = %#x %q, want SS$_NORMAL %q", spec, st, l.calls, want)
		}
	}
}
