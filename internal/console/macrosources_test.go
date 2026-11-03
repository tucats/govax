package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// macroModule is a module named name with one global symbol of its own, and no
// .END if it is to have something joined after it. Its name is its .TITLE, so an
// object says which module it came from.
func macroModule(name string, end bool) string {
	text := "\t.TITLE\t" + name + "\n" +
		"\t.PSECT\tDATA,NOEXE,WRT\n" +
		name + "_V::\t.LONG\t1\n"
	if end {
		text += "\t.END\n"
	}

	return text
}

// macroObjectText is an object file's module name and the global symbols it
// defines, as text to look through.
func macroObjectText(t *testing.T, c *Console, loc rms.FileLocation) string {
	t.Helper()

	var text strings.Builder
	if err := obj.Dump(&text, readObject(t, c, loc)); err != nil {
		t.Fatal(err)
	}

	return text.String()
}

// TestMacroListVolume assembles a list on a volume: each comma's file to an object
// of its own, a + joining files into one, a later name taking its defaults from the
// name before it, and a file's error not stopping the next.
func TestMacroListVolume(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)

	write := func(name, text string) {
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name},
			rms.TextRecords, splitSource(text)); err != nil {
			t.Fatal(err)
		}
	}

	exists := func(name string) bool {
		_, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.VariableRecords)

		return err == nil
	}

	write("AA.MAR", macroModule("AA", true))
	write("BB.MAR", macroModule("BB", true))
	write("JA.MAR", macroModule("JA", false))
	write("JB.MAR", macroModule("JB", true))
	write("BAD.MAR", "\t.TITLE\tBAD\n\tNOTANOP\tR0\n\t.END\n")

	t.Run("commas", func(t *testing.T) {
		shareFindRun(t, d, "MACRO AA,BB")

		if out := buf.String(); out != "" {
			t.Fatalf("MACRO said %s", out)
		}

		for _, name := range []string{"AA", "BB"} {
			if got := macroObjectText(t, c, rms.FileLocation{Name: "DUA1:[000000]" + name + ".OBJ"}); !strings.Contains(got, name+"_V") {
				t.Errorf("%s.OBJ lacks %s_V:\n%s", name, name, got)
			}
		}
	})

	t.Run("plus", func(t *testing.T) {
		shareFindRun(t, d, "MACRO JA+JB")

		if out := buf.String(); out != "" {
			t.Fatalf("MACRO said %s", out)
		}

		got := macroObjectText(t, c, rms.FileLocation{Name: "DUA1:[000000]JA.OBJ"})
		if !strings.Contains(got, "JA_V") || !strings.Contains(got, "JB_V") {
			t.Errorf("JA.OBJ lacks a file's symbol:\n%s", got)
		}

		if exists("JB.OBJ") {
			t.Error("A+B wrote a second object")
		}
	})

	t.Run("both", func(t *testing.T) {
		shareFindRun(t, d, "DELETE AA.OBJ;*,JA.OBJ;*")
		buf.Reset()
		shareFindRun(t, d, "MACRO JA+JB,AA")

		if out := buf.String(); out != "" {
			t.Fatalf("MACRO said %s", out)
		}

		if !exists("JA.OBJ") || !exists("AA.OBJ") || exists("JB.OBJ") {
			t.Errorf("JA.OBJ %v, AA.OBJ %v, JB.OBJ %v", exists("JA.OBJ"), exists("AA.OBJ"), exists("JB.OBJ"))
		}
	})

	t.Run("an error goes on to the next", func(t *testing.T) {
		shareFindRun(t, d, "DELETE AA.OBJ;*,BB.OBJ;*")
		buf.Reset()
		shareFindRun(t, d, "MACRO AA,BAD,BB")

		if out := buf.String(); !strings.Contains(out, "ASMERRORS") {
			t.Errorf("no ASMERRORS for BAD:\n%s", out)
		}

		if !exists("AA.OBJ") || !exists("BB.OBJ") || exists("BAD.OBJ") {
			t.Errorf("AA.OBJ %v, BB.OBJ %v, BAD.OBJ %v", exists("AA.OBJ"), exists("BB.OBJ"), exists("BAD.OBJ"))
		}

		// The last failure's status, though a good file came after it.
		buf.Reset()
		shareFindRun(t, d, "SHOW SYMBOL $STATUS")

		want := fmt.Sprintf("%%X%08X", uint32(vmserrors.CLI_ASMERRORS)|0x10000000)
		if out := buf.String(); !strings.Contains(out, want) {
			t.Errorf("$STATUS is %s, want %s", out, want)
		}
	})

	t.Run("a missing joined file", func(t *testing.T) {
		shareFindRun(t, d, "DELETE JA.OBJ;*")
		buf.Reset()

		if err := d.Dispatch("MACRO JA+NOSUCH"); err == nil {
			t.Error("no failure")
		}

		if exists("JA.OBJ") {
			t.Error("an object from half the input")
		}
	})

	t.Run("wildcards", func(t *testing.T) {
		buf.Reset()

		if err := d.Dispatch("MACRO *.MAR"); err == nil {
			t.Error("a wildcard assembled")
		}
	})
}

// TestMacroListHost: the same on host files, each name quoted, where a comma inside
// the quotes is part of the name.
func TestMacroListHost(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)
	dir := t.TempDir()

	path := func(name string) string { return filepath.Join(dir, name) }

	writeHostFile(t, path("aa.mar"), macroModule("AA", true))
	writeHostFile(t, path("x,y.mar"), macroModule("XY", true))
	writeHostFile(t, path("ja.mar"), macroModule("JA", false))
	writeHostFile(t, path("jb.mar"), macroModule("JB", true))

	shareFindRun(t, d, fmt.Sprintf(`MACRO %q,%q,%q+"jb"`, path("aa.mar"), path("x,y.mar"), path("ja.mar")))

	if out := buf.String(); out != "" {
		t.Fatalf("MACRO said %s", out)
	}

	for _, name := range []string{"aa.obj", "x,y.obj", "ja.obj"} {
		macroObjectText(t, c, rms.FileLocation{Host: true, Name: path(name)})
	}

	if _, err := os.Stat(path("jb.obj")); err == nil {
		t.Error("A+B wrote a second object")
	}

	if got := macroObjectText(t, c, rms.FileLocation{Host: true, Name: path("ja.obj")}); !strings.Contains(got, "JB_V") {
		t.Errorf("ja.obj lacks jb.mar's symbol, its directory the name before it:\n%s", got)
	}
}
