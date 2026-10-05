package debugger_test

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// rawCommand is one command of a VMS debugger session log and the lines
// it printed, with the log's leading "!" removed and nothing else changed.
type rawCommand struct {
	command string
	output  []string
}

// sessionVerbs are the first words of the commands the probe's sessions
// give. A log line that starts with one of them after the single space the
// logger puts before every command is a command; any other line is output.
// (SHOW IMAGE's and SHOW MODULE's own output starts with a space too, so
// the space alone can't tell them apart.)
var sessionVerbs = map[string]bool{
	"SHOW": true, "SET": true, "EXAMINE": true, "SYMBOLIZE": true, "EVALUATE": true,
	"GO": true, "STEP": true, "CALL": true, "CANCEL": true, "DEPOSIT": true, "EXIT": true,
}

// readRawSession reads the session log testdata/dbg/vax/name.dlg.
func readRawSession(t *testing.T, name string) []rawCommand {
	t.Helper()

	data, err := os.ReadFile(consoletest.RepoPath(t, "testdata", "dbg", "vax", name))
	if err != nil {
		t.Fatal(err)
	}

	var cmds []rawCommand

	for _, l := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		line, ok := strings.CutPrefix(l, "!")
		if !ok {
			continue
		}

		verb, _, _ := strings.Cut(strings.TrimSpace(line), " ")
		verb, _, _ = strings.Cut(verb, "/")

		switch {
		case strings.HasPrefix(line, " !"):
			// A comment of the command file, echoed: it ends the output
			// before it.
			cmds = append(cmds, rawCommand{command: "!"})
		case strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "  ") && sessionVerbs[verb]:
			cmds = append(cmds, rawCommand{command: strings.TrimSpace(line)})
		case line != "" && !strings.HasPrefix(line, "%") && len(cmds) > 0:
			last := &cmds[len(cmds)-1]
			last.output = append(last.output, line)
		}
	}

	return cmds
}

// descriptorAddress matches the one value VMS's listing has that govax's
// can't equal: the address of a descriptor the debugger built in its own
// memory.
var descriptorAddress = regexp.MustCompile(`descriptor address: [0-9A-F]{8}`)

// dropBlank removes empty lines, which the log reader leaves out.
func dropBlank(lines []string) []string {
	var kept []string

	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			kept = append(kept, l)
		}
	}

	return kept
}

// TestShowSymbolOracle runs every SHOW SYMBOL, SHOW MODULE, SHOW LANGUAGE,
// and SHOW SCOPE of the probe's sessions of DBGDIS (as VMS linked it, and
// as a traceback-only link, and as govax linked it) and FORTH, and
// compares what the debugger printed: the symbols in VMS's order with
// their addresses and types, and the module table.
//
// What can't match is left out: the sessions of TRACE, where VMS listed
// only the routine named as the module (docs/PHASE-41.md's notes), and the
// "bytes allocated" figures, which are the size of VMS's own tables.
func TestShowSymbolOracle(t *testing.T) {
	compared := 0

	for _, tc := range []struct{ session, image string }{
		{"dbgdis.dlg", "dbgdis.exe"},
		{"dbgtrc.dlg", "dbgtrc.exe"},
		{"gvdbgdis.dlg", "gvdbgdis.exe"},
		{"forth.dlg", "forth.exe"},
	} {
		t.Run(tc.session, func(t *testing.T) {
			c, _ := runImage(t, dbgImagePath(t, tc.image), console.RunOptions{Debug: console.DebugOn})

			for _, e := range readRawSession(t, tc.session) {
				switch {
				case strings.HasPrefix(e.command, "SHOW SYMBOL"),
					e.command == "SHOW MODULE", e.command == "SHOW LANGUAGE", e.command == "SHOW SCOPE",
					strings.HasPrefix(e.command, "SET MODULE"):
				default:
					continue
				}

				got, err := sayErr(c, e.command)
				if err != nil {
					t.Errorf("%s: %v", e.command, err)

					continue
				}

				want := dropBlank(e.output)
				have := dropBlank(strings.Split(strings.TrimSuffix(got, "\n"), "\n"))

				// VMS's listing of FORTH's F_A* omits the dictionary
				// headers (F_ABORT_H, ...: labels DEFWORD puts in the
				// FORTH_WORDS psect), which govax lists as data. No
				// probe explains why (docs/PHASE-42.md, subtask 12).
				if tc.image == "forth.exe" {
					have = withoutHeaders(have)
				}

				if len(want) == 0 && len(have) == 1 && have[0] == "" {
					have = nil
				}

				if len(have) != len(want) {
					t.Errorf("%s: %d lines, want %d:\n%s", e.command, len(have), len(want), got)

					continue
				}

				for i := range want {
					w, g := want[i], have[i]

					if strings.HasPrefix(w, "total MACRO modules") {
						// Up to VMS's own byte count.
						w, g = strings.SplitN(w, "bytes", 2)[0], strings.SplitN(g, "bytes", 2)[0]
					}

					if strings.HasPrefix(e.command, "SHOW MODULE") && i >= 1 && !strings.HasPrefix(w, "total") {
						// A module's size is VMS's in-memory figure for
						// its symbols; govax shows its DST records' size.
						w, g = w[:35], g[:35]
					}

					w = descriptorAddress.ReplaceAllString(w, "descriptor address: X")
					g = descriptorAddress.ReplaceAllString(g, "descriptor address: X")

					if w != g {
						t.Errorf("%s line %d:\n got %q\nwant %q", e.command, i+1, g, w)
					}
				}

				compared += len(want)
			}
		})
	}

	if compared < 250 {
		t.Errorf("only %d lines compared; is the log being read?", compared)
	}
}

// TestShowImage: the main image is marked, listed by its file's name with
// the base and end addresses VMS showed for DBGDIS (the first row of
// dbgdis.dlg's table), and the totals count the images.
func TestShowImage(t *testing.T) {
	c, _ := runImage(t, dbgImagePath(t, "dbgdis.exe"), console.RunOptions{Debug: console.DebugOn})

	out := say(t, c, "SHOW IMAGE")
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")

	if lines[0] != " image name                      set    base address   end address" || lines[1] != "" {
		t.Errorf("heading:\n%s", out)
	}

	found := false

	for _, l := range lines {
		if l == "*DBGDIS                          yes    00000200       000007FF" {
			found = true
		}
	}

	if !found {
		t.Errorf("no row for the main image:\n%s", out)
	}

	if !regexp.MustCompile(` total images: \d+ +bytes allocated: \d+$`).MatchString(lines[len(lines)-1]) {
		t.Errorf("total line: %q", lines[len(lines)-1])
	}

	if full := say(t, c, "SHOW IMAGE/FULL"); !strings.Contains(full, "    2 modules, 4 routines\n") {
		t.Errorf("SHOW IMAGE/FULL:\n%s", full)
	}
}

// TestSetModule: SET MODULE sets one module's symbols, names are checked
// before any is set, and the footer counts the set modules' sizes.
func TestSetModule(t *testing.T) {
	c, _ := runImage(t, dbgImagePath(t, "dbgdis.exe"), console.RunOptions{Debug: console.DebugOn})

	if out := say(t, c, "SHOW MODULE"); !strings.Contains(out, "DBGDIS                          yes         582\n") ||
		!strings.Contains(out, "DBGSUB                          no          209\n") {
		t.Errorf("SHOW MODULE at the start:\n%s", out)
	}

	// Only DBGDIS is searched until DBGSUB is set.
	if _, err := sayErr(c, "SHOW SYMBOL SUB1"); !errors.Is(err, vmserrors.New(vmserrors.DBG_NOSYMBOL, "SUB1")) {
		t.Errorf("SHOW SYMBOL SUB1 before SET MODULE: %v", err)
	}

	if _, err := sayErr(c, "SET MODULE DBGSUB,NOPE"); !errors.Is(err, vmserrors.New(vmserrors.DBG_NOSUCHMODULE, "NOPE")) {
		t.Errorf("SET MODULE with a bad name: %v", err)
	}

	if out := say(t, c, "SHOW MODULE"); !strings.Contains(out, "DBGSUB                          no ") {
		t.Errorf("a failed SET MODULE set DBGSUB:\n%s", out)
	}

	say(t, c, "SET MODULE dbgsub")

	expect(t, "SHOW SYMBOL SUB1", say(t, c, "SHOW SYMBOL SUB1"), "routine DBGSUB\\SUB1\n    address: 00000544, size: 00000014 bytes\n")

	if _, err := sayErr(c, "SET MODULE"); err == nil {
		t.Error("SET MODULE with nothing didn't fail")
	}
}

// TestShowSymbolForms: IN, the qualifiers, wildcards, and the
// path-name pattern.
func TestShowSymbolForms(t *testing.T) {
	c, _ := runImage(t, dbgImagePath(t, "dbgdis.exe"), console.RunOptions{Debug: console.DebugOn})

	expect(t, "SHOW SYMBOL/TYPE", say(t, c, "SHOW SYMBOL/TYPE BYTES IN DBGDIS"),
		"data DBGDIS\\BYTES\n"+
			"    array descriptor type, 1 dimension, bounds: [0:7], size: 8 bytes\n"+
			"        cell type: atomic type, byte integer, size: 1 byte\n")

	// Both qualifiers: the address, then the type.
	expect(t, "SHOW SYMBOL/ADDRESS/TYPE", say(t, c, "SHOW SYMBOL/ADDRESS/TYPE COUNT IN DBGDIS"),
		"data DBGDIS\\COUNT\n    address: 00000200\n    atomic type, longword integer, size: 4 bytes\n")

	// % is one character; a path pattern matches the whole name.
	expect(t, "SHOW SYMBOL LOO%", say(t, c, "SHOW SYMBOL LOO% IN DBGDIS"),
		"label DBGDIS\\START\\LOOP\n    address: 000004DD\n")
	expect(t, "SHOW SYMBOL path", say(t, c, `SHOW SYMBOL DBGDIS\START\L*`),
		"label DBGDIS\\START\\LAST\n    address: 00000531\n"+
			"label DBGDIS\\START\\LOOP\n    address: 000004DD\n")

	if _, err := sayErr(c, "SHOW SYMBOL COUNT IN NOPE"); !errors.Is(err, vmserrors.New(vmserrors.DBG_NOSUCHMODULE, "NOPE")) {
		t.Errorf("IN a missing module: %v", err)
	}

	if _, err := sayErr(c, "SHOW SYMBOL ZZZ*"); !errors.Is(err, vmserrors.New(vmserrors.DBG_NOSYMBOL, "ZZZ*")) {
		t.Errorf("a pattern that matches nothing: %v", err)
	}
}

// TestShowScopeRecursion: the scope lists each call level, and a routine
// that is already inside is numbered, as exam.dlg's SHOW SCOPE at the
// fourth FACT call (FACT, FACT 1, START; BACK is a JSB routine, which
// builds no frame).
func TestShowScopeRecursion(t *testing.T) {
	c := stepSession(t)

	say(t, c, "SET BREAK/AFTER:3 BACK")
	say(t, c, "GO")

	expect(t, "SHOW SCOPE", say(t, c, "SHOW SCOPE"),
		"scope: \n *  0 [ = DBGCMD\\FACT ], \n    1 [ = DBGCMD\\FACT 1 ], \n    2 [ = DBGCMD\\START ]\n")

	expect(t, "SHOW LANGUAGE", say(t, c, "SHOW LANGUAGE"), "language: MACRO\n")
}

// withoutHeaders removes FORTH's dictionary header symbols (data whose
// names end in _H) and their address lines from a SHOW SYMBOL listing.
func withoutHeaders(lines []string) []string {
	var kept []string

	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], `data FORTH\`) && strings.HasSuffix(lines[i], "_H") {
			i++ // its address line

			continue
		}

		kept = append(kept, lines[i])
	}

	return kept
}
