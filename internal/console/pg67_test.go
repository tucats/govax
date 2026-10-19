package console_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
)

// Phase 67's probe (testdata/pg67, README.md): the program's parts, run
// on govax before VMS's run comes back.

// pg67Dir is the probe's directory.
var pg67Dir = filepath.Join("..", "..", "testdata", "pg67")

// pg67Console is a console with paging on, the probe's program built
// (with govax's MACRO and LINK/NOTRACEBACK, as PG67.EXE, and with
// STACK=50 as PG67S.EXE) on an RD54 volume on DUA1, which holds the page
// file too and is the default directory.
func pg67Console(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.paging.file", "DUA1:[000000]PAGEFILE.SYS")
	setSetting(t, "vax.sysgen.paging.size", "20000")

	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "pg67.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 PG67`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)

	for _, f := range []string{"pg67.mar", "pg67s.opt"} {
		src, err := filepath.Abs(filepath.Join(pg67Dir, f))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST DUA1:[000000]` + strings.ToUpper(f))
	}

	dispatch("SET DEFAULT DUA1:[000000]")

	for _, cmd := range []string{
		"MACRO PG67", "LINK/NOTRACEBACK PG67",
		"LINK/NOTRACEBACK/EXECUTABLE=PG67S PG67,PG67S/OPTIONS",
	} {
		if out := dispatch(cmd); out != "" {
			t.Fatalf("%s: %s", cmd, out)
		}
	}

	return c, dispatch
}

// pg67Run runs part of the program from image, as PG67.COM does, and
// returns its lines.
func pg67Run(t *testing.T, dispatch func(string) string, part, image string) []string {
	t.Helper()

	dispatch("DEFINE PG67_PART " + part)
	lines := strings.Split(dispatch("RUN "+image), "|")
	t.Logf("%s from %s:\n%s", part, image, strings.Join(lines, "\n"))

	return lines
}

// pg67Expect fails the test for each prefix whose line lacks its value,
// or isn't there.
func pg67Expect(t *testing.T, id string, lines []string, want map[string]string) {
	t.Helper()

	for prefix, value := range want {
		found := false

		for _, line := range lines {
			if strings.HasPrefix(line, prefix) {
				found = true

				if !strings.Contains(line, value) {
					t.Errorf("%s: %q, want %q in it", id, line, value)
				}
			}
		}

		if !found {
			t.Errorf("%s: no line %q\n%s", id, prefix, strings.Join(lines, "\n"))
		}
	}
}

// pg67Harness is how many pages of process 1's paging file quota the
// test console's own pages hold (scheduledConsole's P0 and the user stack
// pages it pushed onto), which the first image's activation and rundown
// give back, as TestPG66Oracle's harness.
func pg67Harness(env *corevms.Environment) int64 {
	n := int64(0)

	for va := uint32(codeAddr); va < codeAddr+programExtent(brbSelf); va += 512 {
		if env.PageFileChargedAt(va) {
			n++
		}
	}

	for va := env.FreeP1VA() + 512; va < corevms.UserStackTop; va += 512 {
		if env.PageFileChargedAt(va) {
			n++
		}
	}

	return n
}

// TestPG67DryRun runs parts A, S, T (from both images), and C (part Q in
// a detached process) as PG67.COM runs them, before VMS's run comes
// back, and checks govax's own answers to the probe's questions (the
// README lists them): created space's items, the reason masks, stack
// expansion, the creations the table area can't hold, the detached
// image ended by an exception its stack can't report, and every charge
// given back at each image's exit.
func TestPG67DryRun(t *testing.T) {
	c, dispatch := pg67Console(t)
	env := c.ProcessOne()

	cnt := env.PageFileRemaining() + pg67Harness(env)

	a := pg67Run(t, dispatch, "A", "PG67")
	pg67Expect(t, "A", a, map[string]string{
		"A2 expreg 10 P0: 0":              "00000001 retadr frep0va+00000000 frep0va+000013FF",
		"A2 expreg 10 P0: p0=":            "p0=10 p1=0 free=-10 peak=10 charged=10",
		"A3 expreg 10 P1: 0":              "00000001 retadr frep1va+000001FF frep1va+FFFFEE00",
		"A3 expreg 10 P1: p0=":            "p0=0 p1=10 free=-10",
		"A4 deltva P1's lowest 3: p0=":    "p1=-3 free=3",
		"A4 then expreg 2 P1: 0":          "00000001 retadr frep1va+000007FF frep1va+00000400",
		"A5 deltva P0's last 3: p0=":      "p0=-3 p1=0 free=3",
		"A6 deltva a middle P0 page: p0=": "p0=0 p1=0 free=0",
		"A6 read of it:":                  "signal 0000000C mask=00000000 va=base+00000600",
		"A6 probe of it:":                 "read=0 write=0",
		"A7 cntreg 2 P0: p0=":             "p0=-2 p1=0 free=2",
		"A7 cntreg 2 P1: p0=":             "p0=0 p1=-2 free=2",
		"A8 cretva past P0's end: p0=":    "p0=6 p1=0 free=-6 peak=0 charged=1",
		"A8 read of a skipped page:":      "mask=00000000",
		"A8 read of the new page:":        "ok",
		"A9 read at P0's end:":            "mask=00000001 va=base+00000000",
		"A9 write at P0's end:":           "mask=00000005 va=base+00000000",
		"A9 probe at P0's end:":           "read=0 write=0",
		"A9 read far past P0's end:":      "mask=00000001 va=base+00100000",
		"A9 after them:":                  "p0=0 p1=0 free=0 peak=0 charged=0",
		"A10 after the probe:":            "p0=0 p1=0",
		"A10 read at frep1va:":            "ok",
		"A10 after the read:":             "p1=1 free=-1",
		"A10 read 50 pages below:":        "ok",
		"A10 after it:":                   "p1=51 free=-51",
		"A11 executive read at frep1va:":  "00000000 signal 0000000C mask=00000001 va=frep1va+00000000",
		"A11 after it:":                   "p0=0 p1=0",
		"A12 cretva freptecnt+100 in P0:": "00000244 made=0",
		"A12 cretva in P0:":               "p0=0 p1=0 free=0 peak=0 charged=0",
		"A12 cretva freptecnt+100 in P1:": "00000244 made=0",
		"A12 cretva in P1:":               "p0=0 p1=0 free=0 peak=0 charged=0",
		"A13 read beyond":                 "mask=00000001",
		"A13 after it:":                   "p0=0 p1=0 free=0 peak=0 charged=0",
	})

	if got := env.PageFileRemaining(); got != cnt {
		t.Errorf("part A: %d pages of quota left after the image, %d before", got, cnt)
	}

	s := pg67Run(t, dispatch, "S", "PG67")
	pg67Expect(t, "S", s, map[string]string{
		"S3 expreg 5 P1: 0":    "00000001 retadr deepest-frep1va+000001FF deepest-frep1va+FFFFF800",
		"S3 expreg 5 P1: p0=":  "p0=0 p1=5",
		"S2 after the return:": "p0=0",
	})

	// The stack pages the recursion made stay after it returns.
	if deepest, after := pg67Field(s, "S1 at the deepest:", "p1="), pg67Field(s, "S2 after the return:", "p1="); deepest == "" || deepest != after {
		t.Errorf("S: P1 grew by %q at the deepest, %q after the return; want the same", deepest, after)
	}

	if got := env.PageFileRemaining(); got != cnt {
		t.Errorf("part S: %d pages of quota left after the image, %d before", got, cnt)
	}

	// The user stack at the first instruction: STACK=50's 30 pages more
	// than LINK's default 20.
	t0 := pg67Run(t, dispatch, "T", "PG67")
	t50 := pg67Run(t, dispatch, "T", "PG67S")
	pg67Expect(t, "T", t0, map[string]string{"A1 first instruction:": "sp-frep1va=000029A8"})
	pg67Expect(t, "T50", t50, map[string]string{"A1 first instruction:": "sp-frep1va=000065A8"})

	// Part C: the image's and output file's names as PG67.COM gives
	// them, with the device's full name.
	dispatch(`DEV = F$GETDVI(F$PARSE("PG67.EXE",,,"DEVICE"),"FULLDEVNAM")`)
	dispatch(`DIR = F$PARSE("PG67.EXE",,,"DIRECTORY")`)
	dispatch("DEFINE PG67_IMAGE 'DEV''DIR'PG67.EXE")
	dispatch("DEFINE PG67_OUTQ 'DEV''DIR'PG67Q.OUT")

	// Part Q's recursion runs out of quota with no stack left to report
	// the access violation on: the handler is passed over and the image
	// ends with SS$_ACCVIO (the book's 16.2.1.3), its message on the
	// console, the creator's run going on.
	cl := pg67Run(t, dispatch, "C", "PG67")
	pg67Expect(t, "C", cl, map[string]string{
		"C1 crembx:":   "00000001",
		"C1 creprc:":   "00000001",
		"C2 ended;":    "00000001 iosb=00000001",
		"C3 type=":     "finalsts=1000000C",
		"%SYSTEM-F-AC": "reason mask=05",
	})

	q := strings.Split(dispatch("TYPE PG67Q.OUT"), "|")
	t.Logf("Q, in the detached process:\n%s", strings.Join(q, "\n"))
	pg67Expect(t, "Q", q, map[string]string{
		"Q1 expreg pagfilcnt-10 P0:": "00000001 pagfilcnt then 10",
	})

	for _, line := range q {
		if strings.HasPrefix(line, "Q2") || strings.HasPrefix(line, "Q3") {
			t.Errorf("Q: %q: the image went on after its stack ran out", line)
		}
	}
}

// pg67Field is the value of the field named by key ("p1=") in the line
// that starts with prefix, or "".
func pg67Field(lines []string, prefix, key string) string {
	for _, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			continue
		}

		if _, rest, ok := strings.Cut(line, key); ok {
			v, _, _ := strings.Cut(rest, " ")

			return v
		}
	}

	return ""
}
