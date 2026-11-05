package console

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
)

// This file is the FORTRAN milestone's test (docs/PHASE-86 - FORTRAN milestone.md).
// DUNGEON's source stays out of the repository, so it skips without it, and nothing
// here quotes the game.

// dungeonCompile is DUNGEON's own build, as its documentation types it, with
// qualifiers added: its sources by the kit's names, in two lists. Then MACRO on all
// of them, which VAX FORTRAN doesn't need, and ours does.
var (
	dungeonCompile = []string{
		"DUNGEON.F,GAME.F,GDT.F,PARSER.F,SUBR.F",
		"ROOMS.F,VERBS.F,OBJECTS.F,TIMEFNC.F",
	}
	dungeonAssemble = "MACRO DUNGEON,GAME,GDT,PARSER,SUBR,ROOMS,VERBS,OBJECTS,TIMEFNC"
)

// buildDungeon compiles and assembles DUNGEON in the console's default directory, with
// FORT's qualifier.
func buildDungeon(t *testing.T, d *Dispatcher, qualifier string) {
	t.Helper()

	for _, list := range dungeonCompile {
		shareFindRun(t, d, "FORT/NOCHECK/CONT=40"+qualifier+" "+list)
	}

	shareFindRun(t, d, dungeonAssemble)
}

// afterMacroHeader is a MACRO file's lines after its header, but for the comments
// around an INCLUDE file's lines: those and the header name a file where it was
// found, so a volume's and a host's differ.
func afterMacroHeader(lines []string) []string {
	start := slices.IndexFunc(lines, func(line string) bool {
		return strings.HasPrefix(line, "; Each line of the FORTRAN source")
	})

	return slices.DeleteFunc(slices.Clone(lines[max(start, 0):]), func(line string) bool {
		return strings.HasPrefix(line, "; INCLUDE file ") || strings.HasPrefix(line, "; end of ")
	})
}

// dungeonSources is the kit's files that a compilation reads, each with the name to
// keep it under.
func dungeonSources(t *testing.T) map[string]string {
	t.Helper()

	dir := dungeonSource()
	if _, err := os.Stat(filepath.Join(dir, "dungeon.f")); err != nil {
		t.Skipf("no DUNGEON source: %v", err)
	}

	sources := map[string]string{filepath.Join(dir, "dparam.for"): "DPARAM.FOR"}

	for _, module := range dungeonModules {
		sources[filepath.Join(dir, module+".f")] = strings.ToUpper(module) + ".F"
	}

	return sources
}

// TestDungeonCompiles is the milestone's subtask 1: DUNGEON's own commands, on a
// volume and on host files, say nothing and make every object, and the two sides
// make the same MACRO files. Each compiler setting a program might use must also get
// through FORTRAN and MACRO without an error.
func TestDungeonCompiles(t *testing.T) {
	sources := dungeonSources(t)

	d, c, buf := dungeonConsole(t, false)

	for host, name := range sources {
		data, err := os.ReadFile(host)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA3:[000000]" + name},
			rms.TextRecords, splitSource(string(data))); err != nil {
			t.Fatal(name, err)
		}
	}

	buf.Reset()

	settings := []string{
		"", "/NOOPTIMIZE", "/CHECK=ALL", "/CHECK=ALL/NOOPTIMIZE", "/DEBUG/NOOPTIMIZE",
		"/LIST/CROSS_REFERENCE/SHOW=ALL", "/NOF77", "/G_FLOATING", "/ALIGN=ALL", "/D_LINES", "/EXTEND_SOURCE",
	}

	for _, qualifier := range settings {
		t.Run("volume"+qualifier, func(t *testing.T) {
			buildDungeon(t, d, qualifier)

			if out := buf.String(); out != "" {
				t.Errorf("it said %d bytes, first: %.200s", len(out), out)
			}

			buf.Reset()

			for _, module := range dungeonModules {
				readObject(t, c, rms.FileLocation{Name: "DUA3:[000000]" + strings.ToUpper(module) + ".OBJ"})
			}

			// Each build's files would soon fill the volume.
			shareFindRun(t, d, "DELETE *.MAR;*,*.OBJ;*")

			if strings.Contains(qualifier, "LIST") {
				shareFindRun(t, d, "DELETE *.LIS;*")
			}

			buf.Reset()
		})
	}

	// What the default setting wrote, for the host's to match.
	buildDungeon(t, d, "")
	buf.Reset()

	onVolume := map[string][]string{}
	for _, module := range dungeonModules {
		name := strings.ToUpper(module) + ".MAR"
		onVolume[name] = macroLines(t, c, rms.FileLocation{Name: "DUA3:[000000]" + name})
	}

	// /WARNINGS=UNUSED has a finding for each of the names and labels the source
	// doesn't use, read and checked by hand once (the phase's log), and none for
	// what it does: RAN is declared, and called.
	t.Run("warnings", func(t *testing.T) {
		for _, list := range dungeonCompile {
			shareFindRun(t, d, "FORTRAN/CONT=40/NOMACRO/WARNINGS=(NODECLARATIONS,UNUSED) "+list)
		}

		out := buf.String()
		buf.Reset()

		if got := strings.Count(out, "%FORT-W-UNUSED"); got != 13 {
			t.Errorf("%d findings, want 13", got)
		}

		if strings.Contains(out, " RAN ") {
			t.Error("RAN is said to be unused")
		}
	})

	// /CONTINUATIONS=40 is needed: the default limit stops some statements.
	t.Run("continuations", func(t *testing.T) {
		for _, list := range dungeonCompile {
			shareFindRun(t, d, "FORTRAN/NOMACRO "+list)
		}

		out := buf.String()
		buf.Reset()

		if !strings.Contains(out, "TOOMANYCONT") {
			t.Errorf("no TOOMANYCONT with the default limit:\n%.300s", out)
		}
	})

	t.Run("host", func(t *testing.T) {
		dir := t.TempDir()
		copyDungeonKit(t, sources, dir)

		c, buf := newTestConsole(t)
		c.Paths = respath.New(nil, bootdata.FS)
		d := NewDispatcher(c, loadEvaxGrammar(t), nil)

		// With no SET DEFAULT onto a volume, a bare name is a host file.
		t.Chdir(dir)
		buildDungeon(t, d, "")

		if out := buf.String(); out != "" {
			t.Errorf("it said %d bytes, first: %.200s", len(out), out)
		}

		for _, module := range dungeonModules {
			upper := strings.ToUpper(module)
			readObject(t, c, rms.FileLocation{Host: true, Name: filepath.Join(dir, upper+".OBJ")})

			got := macroLines(t, c, rms.FileLocation{Host: true, Name: filepath.Join(dir, upper+".MAR")})
			if want := onVolume[upper+".MAR"]; !slices.Equal(afterMacroHeader(got), afterMacroHeader(want)) {
				t.Errorf("%s.MAR differs between the volume and the host", upper)
			}
		}
	})
}

// A game saved, and one restoring it.
const (
	dungeonSaveGame    = "open mailbox\nsave\nquit\ny\n"
	dungeonRestoreGame = "restore\nquit\ny\n"
)

// lastLines is text's last n lines.
func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")

	return strings.Join(lines[max(len(lines)-n, 0):], "\n")
}

// checkDungeonRun holds what a run printed to its having started and taken every
// move: the game prompts for each with a line starting ">", which a failed start
// never gets to, and no system message. What the game says isn't quoted here.
func checkDungeonRun(t *testing.T, out string, moves int) {
	t.Helper()

	if got := strings.Count("\n"+out, "\n>"); got < moves {
		t.Errorf("%d prompts, want at least %d, in:\n%.300s", got, moves, out)
	}

	if strings.Contains("\n"+out, "\n%") {
		t.Errorf("a system message in:\n%s", out)
	}
}

// TestDungeonRuns is the milestone's subtask 2: DUNGEON built by its documentation's
// commands, with its ITIME and data files, and played. On a volume a game is saved
// and restored by another run. From the host, the program is on the host and its
// data on a volume, by the logical names the kit gives; a program's RMS reaches only
// volume files.
func TestDungeonRuns(t *testing.T) {
	sources := dungeonSources(t)
	dir := dungeonSource()

	vmssubr, err := filepath.Abs(filepath.Join("..", "..", "testdata", "dungeon86", "vmssubr.for"))
	if err != nil {
		t.Fatal(err)
	}

	d, c, buf := dungeonConsole(t, false)

	run := func(t *testing.T, command string) {
		t.Helper()
		shareFindRun(t, d, command)

		if out := buf.String(); out != "" {
			t.Fatalf("%s said: %.300s", command, out)
		}
	}

	bringData := func(t *testing.T) {
		t.Helper()
		run(t, `COPY/QUIET "`+filepath.Join(dir, "dindx")+`"/HOST DUA3:[000000]DINDX.DAT`)
		run(t, `COPY/QUIET "`+filepath.Join(dir, "dtext")+`"/HOST DUA3:[000000]DTEXT.DAT/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=80`)
	}

	link := func(t *testing.T) {
		t.Helper()
		run(t, "FORT/NOCHECK/CONT=40 "+dungeonCompile[0])
		run(t, "FORT/NOCHECK/CONT=40 "+dungeonCompile[1]+",VMSSUBR.FOR")
		run(t, dungeonAssemble+",VMSSUBR")

		// The kit's LINK, as it's written: on two lines.
		run(t, "LINK DUNGEON,GAME,GDT,PARSER,SUBR,-")

		if !d.Continuing() {
			t.Fatal("LINK's first line isn't held for the next")
		}

		run(t, "ROOMS,VERBS,OBJECTS,TIMEFNC,VMSSUBR")
	}

	play := func(t *testing.T, game string, moves int) string {
		t.Helper()

		c.In = strings.NewReader(game)
		out := shareRun(t, d, buf, "DUNGEON")
		checkDungeonRun(t, out, moves)

		return out
	}

	t.Run("volume", func(t *testing.T) {
		for host, name := range sources {
			run(t, `COPY/QUIET "`+host+`"/HOST `+name)
		}

		run(t, `COPY/QUIET "`+vmssubr+`"/HOST VMSSUBR.FOR`)
		bringData(t)
		link(t)

		shareFindRun(t, d, "DIRECTORY/FULL DTEXT.DAT")

		if out := buf.String(); !strings.Contains(out, "Fixed length 80 byte records") {
			t.Errorf("DTEXT.DAT is:\n%s", out)
		}

		buf.Reset()

		saved := play(t, dungeonSaveGame, 3)
		if _, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DSAVE.DAT"}, rms.VariableRecords); err != nil {
			t.Errorf("no save file: %v", err)
		}

		// The score and moves at the end of a game are the saved game's, the
		// restore not being a move.
		restored := play(t, dungeonRestoreGame, 2)
		if got, want := lastLines(restored, 4), lastLines(saved, 4); got != want {
			t.Errorf("the restored game ends:\n%s\nand the saved one:\n%s", got, want)
		}
	})

	t.Run("host", func(t *testing.T) {
		work := t.TempDir()
		copyDungeonKit(t, sources, work)

		data, err := os.ReadFile(vmssubr)
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(work, "VMSSUBR.FOR"), data, 0o644); err != nil {
			t.Fatal(err)
		}

		hd, hc, hbuf := dungeonHostConsole(t, false)

		d, c, buf = hd, hc, hbuf
		buf.Reset()

		bringData(t)
		t.Chdir(work)
		link(t)

		run(t, "DEFINE DINDX DUA3:[000000]DINDX.DAT")
		run(t, "DEFINE DTEXT DUA3:[000000]DTEXT.DAT")

		play(t, "open mailbox\nquit\ny\n", 2)
	})
}

// copyDungeonKit copies what a compilation reads into the host directory dir. The
// source's INCLUDE names its file in lower case, which a case-sensitive host file
// system tells apart.
func copyDungeonKit(t *testing.T, sources map[string]string, dir string) {
	t.Helper()

	for host, name := range sources {
		data, err := os.ReadFile(host)
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}

		if name == "DPARAM.FOR" {
			if err := os.WriteFile(filepath.Join(dir, "dparam.for"), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// dungeonHostConsole is a console with no default directory on a volume, whose bare
// names are host files, and with a volume large enough for the game's data on DUA3:,
// reached only by name.
func dungeonHostConsole(t *testing.T, paging bool) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	if paging {
		withSettings(t, map[string]string{machine.PagingSetting: "true"})
	}

	c, buf := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.RunHostProcedure("startup.com", d.Dispatch); err != nil {
		t.Fatal(err)
	}

	if (c.Mem.Paging() != nil) != paging {
		t.Fatalf("paging is %v, want %v", c.Mem.Paging() != nil, paging)
	}

	mountFreshContainerSized(t, c, "DUA3:", 60000)
	buf.Reset()

	return d, c, buf
}

// mountFreshContainerSized mounts a new volume of the given blocks on device.
func mountFreshContainerSized(t *testing.T, c *Console, device string, blocks uint32) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sized.dsk")
	if err := c.cmd().InitializeContainer(path, blocks, "SIZED", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount(device, path, true); err != nil {
		t.Fatal(err)
	}
}

// dungeonGameSHA is the SHA-256 of what DUNGEON prints playing testdata/dungeon86's
// game.in. The game's text can't be kept here, so its digest stands for it. Before it
// was recorded, the same game on gfortran's build of the same source printed the same
// (testdata/dungeon86/gfortran/compare.py; its --sha option prints this). A change here
// means the program's behavior changed: run compare.py to see whether it's still right.
const dungeonGameSHA = "a4e1c720a3c32d67ad40110986492a42192101fb6f68106d3d8ca1a2a23c0f26"

// TestDungeonMilestone is the FORTRAN program's milestone, its subtask 3: DUNGEON built by
// its own commands and playing a scripted game, on volume files and from host files, in
// both paging modes, with and without the optimizer, and /RECURSIVE (Phase 89), printing
// the same every time. On a volume a game is also saved, and restored by another run.
// fixtime.for fixes the seed.
func TestDungeonMilestone(t *testing.T) {
	sources := dungeonSources(t)
	kit := filepath.Join("..", "..", "testdata", "dungeon86")

	dir := dungeonSource()

	game, err := filepath.Abs(filepath.Join(kit, "game.in"))
	if err != nil {
		t.Fatal(err)
	}

	fixtime, err := filepath.Abs(filepath.Join(kit, "fixtime.for"))
	if err != nil {
		t.Fatal(err)
	}

	for _, paging := range []bool{false, true} {
		for _, mode := range append(optimizerModes, recursiveModes...) {
			for _, host := range []bool{false, true} {
				name := map[bool]string{false: "volume", true: "host"}[host] + "/" +
					map[bool]string{false: "nopaging", true: "paging"}[paging] + "/" + mode.name

				t.Run(name, func(t *testing.T) {
					var (
						d   *Dispatcher
						c   *Console
						buf *bytes.Buffer
					)

					run := func(command string) {
						t.Helper()
						shareFindRun(t, d, command)

						if out := buf.String(); out != "" {
							t.Fatalf("%s said: %.300s", command, out)
						}
					}

					if host {
						work := t.TempDir()
						copyDungeonKit(t, sources, work)

						data, err := os.ReadFile(fixtime)
						if err != nil {
							t.Fatal(err)
						}

						if err := os.WriteFile(filepath.Join(work, "FIXTIME.FOR"), data, 0o644); err != nil {
							t.Fatal(err)
						}

						d, c, buf = dungeonHostConsole(t, paging)
						t.Chdir(work)
					} else {
						d, c, buf = dungeonConsole(t, paging)

						for host, name := range sources {
							run(`COPY/QUIET "` + host + `"/HOST ` + name)
						}

						run(`COPY/QUIET "` + fixtime + `"/HOST FIXTIME.FOR`)
					}

					run(`COPY/QUIET "` + filepath.Join(dir, "dindx") + `"/HOST DUA3:[000000]DINDX.DAT`)
					run(`COPY/QUIET "` + filepath.Join(dir, "dtext") + `"/HOST DUA3:[000000]DTEXT.DAT/BINARY/RECORD_FORMAT=FIXED/RECORD_SIZE=80`)
					run(`COPY/QUIET "` + game + `"/HOST DUA3:[000000]GAME.IN`)

					// As in dungeonCompile, but for GAME.F's having FIXTIME's IDATE.
					qualifiers := "FORT/NOCHECK/CONT=40" + mode.qualifier + " "
					run(qualifiers + "DUNGEON.F,GDT.F,PARSER.F,SUBR.F,GAME.F+FIXTIME.FOR")
					run(qualifiers + "ROOMS.F,VERBS.F,OBJECTS.F,TIMEFNC.F")
					run(dungeonAssemble)
					run("LINK DUNGEON,GAME,GDT,PARSER,SUBR,ROOMS,VERBS,OBJECTS,TIMEFNC")

					if host {
						run("DEFINE/NOLOG DINDX DUA3:[000000]DINDX.DAT")
						run("DEFINE/NOLOG DTEXT DUA3:[000000]DTEXT.DAT")
					}

					// The moves come from a file, so nothing is left to reach DCL.
					run("DEFINE/USER_MODE/NOLOG SYS$INPUT DUA3:[000000]GAME.IN")

					shareFindRun(t, d, "RUN DUNGEON")

					out := buf.String()
					buf.Reset()

					sum := sha256.Sum256([]byte(out))

					if got := hex.EncodeToString(sum[:]); got != dungeonGameSHA {
						t.Errorf("the game printed %d bytes, SHA-256 %s, want %s", len(out), got, dungeonGameSHA)
					}

					if host {
						return
					}

					// Saved here and restored by another run. Each ends by showing the
					// score and moves, the restore not being a move.
					c.In = strings.NewReader(dungeonSaveGame)
					saved := shareRun(t, d, buf, "DUNGEON")
					checkDungeonRun(t, saved, 3)

					c.In = strings.NewReader(dungeonRestoreGame)
					restored := shareRun(t, d, buf, "DUNGEON")
					checkDungeonRun(t, restored, 2)

					if got, want := lastLines(restored, 4), lastLines(saved, 4); got != want {
						t.Errorf("the restored game ends:\n%s\nand the saved one:\n%s", got, want)
					}
				})
			}
		}
	}
}
