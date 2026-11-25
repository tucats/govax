package console

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/testproc"
)

// TestLinkOptionsByLogicalName links with an options file named by a logical
// name (Phase 113, subtask 5's A.7.1: LINK X/OPTIONS with X defined, as VMS's LINK
// takes it): first in the list, and after a host file, where the name was taken
// as a file beside that one.
func TestLinkOptionsByLogicalName(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	c := newBootableConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t))
	dir := t.TempDir()
	t.Chdir(dir) // where default outputs go

	writeHostFile(t, filepath.Join(dir, "main.mar"), "\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n\tCALLS\t#0,SUB\n\tRET\n\t.END\tSTART\n")
	writeHostFile(t, filepath.Join(dir, "sub.mar"), "\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSUB,^M<>\n\tMOVL\t#LIMIT,R0\n\tRET\n\t.END\n")
	writeHostFile(t, filepath.Join(dir, "real.opt"), "sub\nSYMBOL=LIMIT,7\n")

	for _, name := range []string{"main", "sub"} {
		if err := c.cmd().Macro(dclcmd.MacroOptions{Source: filepath.Join(dir, name+".mar"), Object: filepath.Join(dir, name+".obj")}); err != nil {
			t.Fatal(err)
		}
	}

	main := `"` + filepath.Join(dir, "main") + `"`
	exe := `"` + filepath.Join(dir, "main.exe") + `"`

	if err := d.Dispatch(`DEFINE POPT "` + filepath.Join(dir, "real.opt") + `"`); err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{
		`LINK/EXECUTABLE=` + exe + ` ` + main + `,POPT/OPTIONS`,
		`LINK/EXECUTABLE=` + exe + ` POPT/OPTIONS,` + main,
	} {
		_ = os.Remove(filepath.Join(dir, "main.exe"))

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		if _, err := os.Stat(filepath.Join(dir, "main.exe")); err != nil {
			t.Fatalf("%s: no image: %v", line, err)
		}
	}

	// The image runs SUB, which the options file named, with its SYMBOL=.
	if got := runImage(t, c, filepath.Join(dir, "main.exe")); got != 7 {
		t.Errorf("R0 = %d, want 7", got)
	}
}
