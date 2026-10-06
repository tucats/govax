package link

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// realMap reads a map real LINK wrote, as lines, and the names it records:
// the object file, the image file, and the map file. The lines stop before
// its last section, the run statistics, which govax doesn't write.
func realMap(t *testing.T, path string) (lines []string, object, image, mapFile string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	all := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")

	for i, line := range all {
		switch {
		case strings.Contains(line, "! Link Run Statistics !"):
			return lines[:len(lines)-1], object, image, mapFile
		case strings.HasPrefix(line, "Map format:"):
			mapFile = line[strings.Index(line, " in file ")+len(" in file "):]
		case i > 0 && all[i-1] == "\f":
			// The image synopsis page's heading has the image's full
			// name.
			image = strings.TrimSpace(line[:64])
		case object == "" && len(line) > 79 && strings.HasSuffix(strings.TrimSpace(line[:41]), strings.TrimSpace(line[31:41])) && strings.Contains(line, ".OBJ"):
			object = strings.TrimSpace(line[42:79])
		}

		lines = append(lines, line)
	}

	t.Fatalf("%s has no run statistics", path)

	return nil, "", "", ""
}

// TestMapMatchesRealLINK links entry, hello, and psects from real MACRO's
// objects with real LINK's libraries, and checks each map line for line
// against the map real LINK wrote (testdata/mar/vax/*.map), up to its run
// statistics: the object module synopsis, the program sections and their
// contributions, the symbols, and the image synopsis's layout and counts.
func TestMapMatchesRealLINK(t *testing.T) {
	for _, name := range []string{"entry", "hello", "psects", "forth", "pi"} {
		t.Run(name, func(t *testing.T) {
			var err error

			want, object, image, mapFile := realMap(t, filepath.Join(fixtureDir, "vax", name+".map"))
			_, opts := realImage(t, filepath.Join(fixtureDir, "vax", name+".exe"))
			opts.Sources = vmsSources(t)

			// The images were linked again after the maps were written.
			if opts.Time, err = time.ParseInLocation("_2-Jan-2006 15:04", want[1][64:81], time.UTC); err != nil {
				t.Fatal(err)
			}

			img, err := Link([]Input{{File: object, Module: realObject(t, name)}}, opts)
			if err != nil {
				t.Fatal(err)
			}

			got := img.Map(MapOptions{ImageFile: image, MapFile: mapFile})

			for i := range max(len(got), len(want)) {
				var g, w string
				if i < len(got) {
					g = got[i]
				}

				if i < len(want) {
					w = want[i]
				}

				if g != w {
					t.Errorf("line %d:\n got %q\nwant %q", i+1, g, w)
				}
			}
		})
	}
}

// mapOf links MACRO sources and returns the image's map.
func mapOf(t *testing.T, opts MapOptions, srcs ...string) []string {
	t.Helper()

	img, err := linkSources(t, srcs...)
	if err != nil {
		t.Fatal(err)
	}

	return img.Map(opts)
}

// TestMapSymbolColumns lists 300 symbols: the cross reference facility's
// four 28-character columns, 6 blanks apart, filled down the rest of the
// first page, then down each new page, which repeats the column headings.
func TestMapSymbolColumns(t *testing.T) {
	var src strings.Builder

	src.WriteString(".PSECT CODE,NOWRT,EXE\n.ENTRY START,^M<>\nRET\n")

	for i := range 300 {
		fmt.Fprintf(&src, "S%03d == %d\n", i, i)
	}

	src.WriteString(".END START\n")

	lines := mapOf(t, MapOptions{ImageFile: "T.EXE", MapFile: "T.MAP"}, src.String())

	for i, line := range lines {
		if len(line) > mapLineWidth {
			t.Errorf("line %d is %d characters", i+1, len(line))
		}
	}

	// Each page's symbol rows: the lines after the column headings' dashes,
	// up to the page's end or a blank line.
	var pages [][]string

	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "------          -----") {
			continue
		}

		var rows []string
		for i++; i < len(lines) && lines[i] != "" && lines[i] != "\f"; i++ {
			rows = append(rows, lines[i])
		}

		pages = append(pages, rows)
	}

	// The symbols are S000 to S299, then START.
	entry := func(n int) string {
		if n == 300 {
			return "START           00000200-R  "
		}

		return fmt.Sprintf("S%03d            %08X    ", n, n)
	}

	next := 0

	for p, rows := range pages {
		per := len(rows)
		if p < len(pages)-1 && per != mapLinesPerPage-6 && p > 0 {
			t.Errorf("page %d has %d rows", p+1, per)
		}

		for r, row := range rows {
			for col := 0; col < 4 && next+col*per+r <= 300; col++ {
				at := col * 34
				want := entry(next + col*per + r)

				if len(row) < at+len(want) || row[at:at+len(want)] != want {
					t.Fatalf("page %d row %d column %d: %q, want %q", p+1, r+1, col+1, row, want)
				}
			}
		}

		next += 4 * per
	}

	if next < 301 || len(pages) < 2 {
		t.Errorf("%d pages list %d symbols", len(pages), next)
	}
}

// TestMapLongNames lists a module, psect, and symbol whose names are
// longer than 15 characters: the module's and psect's names get lines or
// columns of their own, and the symbol table has two 44-character columns.
func TestMapLongNames(t *testing.T) {
	lines := mapOf(t, MapOptions{ImageFile: "T.EXE", MapFile: "T.MAP"},
		".TITLE A_LONG_MODULE_NAME\n.PSECT A_LONG_PSECT_NAME,NOWRT,EXE\n.ENTRY A_LONG_SYMBOL_NAME,^M<>\nRET\n.END A_LONG_SYMBOL_NAME\n")

	text := strings.Join(lines, "\n")

	// Real LINK's formats: a module line after the name's own line is
	// !16AC!15AC... with a blank name, a long psect name is !31AC, and a
	// contribution from a long-named module is !16AC!39AC and then !31AC
	// with a blank name. An entry that isn't in the last column is
	// followed by the blanks between columns, 44 of them here.
	for _, want := range []string{
		"\nA_LONG_MODULE_NAME\n                0                       3 m0.obj",
		"\nA_LONG_PSECT_NAME               00000200 00000202 00000003 (          3.) BYTE  0 NOPIC,USR,CON,REL,LCL,NOSHR,  EXE,  RD,NOWRT,NOVEC\n",
		"\n                A_LONG_MODULE_NAME                     \n                                00000200 00000202 00000003 (          3.) BYTE  0\n",
		"\nSymbol                          Value       Symbol                          Value       Symbol                          Value\n",
		"\nA_LONG_SYMBOL_NAME              00000200-R  " + strings.Repeat(" ", 44) + "\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
}

// TestMapBriefAndNoImage writes a brief map of a link with no image: it
// has no psects or symbols, and every page's heading says there's no
// image.
func TestMapBriefAndNoImage(t *testing.T) {
	lines := mapOf(t, MapOptions{MapFile: "T.MAP", Brief: true}, ".PSECT CODE,NOWRT,EXE\n.ENTRY START,^M<>\nRET\n.END START\n")
	text := strings.Join(lines, "\n")

	for _, unwanted := range []string{"Program Section Synopsis", "Symbols By Name"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("a brief map has %s", unwanted)
		}
	}

	for i, line := range lines {
		if line == "\f" && !strings.HasPrefix(lines[i+1], mapNoImage+" ") {
			t.Errorf("heading %q", lines[i+1])
		}
	}

	if !strings.Contains(text, "Map format:                                       BRIEF in file T.MAP\n") {
		t.Errorf("map format line missing:\n%s", text)
	}

	// The estimate for a brief map counts only the modules: 7 + 2/4. The
	// modules are the one linked and SYS$IMGSTA's, which traceback adds.
	if !strings.Contains(text, "Estimated map length:                             7. blocks") {
		t.Errorf("estimate:\n%s", text)
	}
}
