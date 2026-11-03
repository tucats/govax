package console

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
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

	d, c, buf := dungeonConsole(t)

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

		for host, name := range sources {
			data, err := os.ReadFile(host)
			if err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
				t.Fatal(err)
			}

			// The source's INCLUDE names it in lower case, which a case-sensitive
			// host file system tells apart.
			if name == "DPARAM.FOR" {
				if err := os.WriteFile(filepath.Join(dir, "dparam.for"), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}

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
