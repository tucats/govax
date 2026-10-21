package console_test

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// tb70Dir holds Phase 70's probe of traceback (README.md): its sources,
// and in vax/ VMS V7.1's log and images.
const tb70Dir = "../../testdata/tb70"

// tb70Parts are the parts TB70.COM's RUNALL runs, in its order.
var tb70Parts = strings.Split("S,A,O,I,Z,R,J,P,L,K,W,E,H,T,X,D,G,V,C,M,N", ",")

// tb70Console is a console with a fresh volume as DUA1 holding VMS's
// TB70 images and TB70SHR, the shareable image they use, with TB70SHR
// defined as TB70.COM defines it.
func tb70Console(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	c, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "tb70.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 TB70`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, image := range []string{"TB70", "TB70NT", "TB70D", "TB70SHR", "TB70B", "TB70BNT"} {
		src, err := filepath.Abs(filepath.Join(tb70Dir, "vax", strings.ToLower(image)+".exe"))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST/BINARY DUA1:[000000]` + image + ".EXE")
	}

	dispatch("DEFINE TB70SHR DUA1:[000000]TB70SHR.EXE")

	return c, dispatch
}

// tb70VAXRuns returns VMS's output for each RUN/NODEBUG of image in
// TB70.COM's RUNALL, by part: the log's lines from the RUN to the SHOW
// SYMBOL $STATUS after it, without blank lines (which the console's
// dispatcher drops too), and the $STATUS line.
func tb70VAXRuns(t *testing.T, image string) map[string][]string {
	t.Helper()

	return tb70LogRuns(t, "tb70.log", "$   DEFINE/NOLOG TB70_PART ", "$   RUN/NODEBUG "+image)
}

// tb70BRuns is tb70VAXRuns for round 2's log, TB70B.COM's runs of image.
func tb70BRuns(t *testing.T, image string) map[string][]string {
	t.Helper()

	return tb70LogRuns(t, "tb70b.log", "$ DEFINE TB70B_PART ", "$ RUN "+image)
}

// tb70LogRuns returns the runs, by part, of the VMS log file in vax/
// whose part is defined by a line starting with define and whose run is
// the line run.
func tb70LogRuns(t *testing.T, file, define, run string) map[string][]string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(tb70Dir, "vax", file))
	if err != nil {
		t.Fatal(err)
	}

	runs := map[string][]string{}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")
	part := ""

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if p, ok := strings.CutPrefix(line, define); ok {
			part = p

			continue
		}

		if line != run || part == "" {
			continue
		}

		var out []string

		for i++; i < len(lines) && !strings.HasPrefix(lines[i], "$ "); i++ {
			if lines[i] != "" {
				out = append(out, lines[i])
			}
		}

		if i+1 < len(lines) {
			out = append(out, strings.TrimSpace(lines[i+1]))
		}

		runs[part] = out
		part = ""
	}

	return runs
}

// tb70Masked masks the dump lines of a run (tb70VAXRuns's form) whose
// PC is outside the main image, which ends below 1000: PCs in LIBRTL, in
// TB70SHR, and in the system's AST exit, all of which govax places
// elsewhere than VMS did (LIBRTL's routines are shims in S0, TB70SHR
// follows the image, the AST exit is the SYS$CLRAST stub in P1).
func tb70Masked(lines []string) []string {
	out := make([]string, len(lines))

	for i, line := range lines {
		out[i] = line

		if len(line) != 77 || line[67:69] != "  " {
			continue
		}

		if pc, err := strconv.ParseUint(line[69:], 16, 32); err == nil && (pc < 0x200 || pc >= 0x1000) {
			out[i] = "(a PC outside the image)"
		}
	}

	return out
}

// tb70DumpDifferences are the runs whose output differs from VMS's for a
// reason other than a PC's place, and why.
var tb70DumpDifferences = map[string]string{
	// LIB$MOVC3 is a shim: its access violation is reported at its
	// caller's PC, not in LIBRTL's code (VMS: PC=000130DE, its line
	// first).
	"L": "LIBRTL's routines are shims",
	// The access violation's message has TB70SHR's PC (VMS: 00024026).
	"M": "TB70SHR's place",
	// An exit handler's condition gets the "improperly handled" dump,
	// which TestTB70ImproperlyHandled compares.
	"X": "EXE$EXCMSG's dump",
}

// TestTB70Traceback runs each part of VMS's TB70 and TB70D (linked
// /DEBUG, run /NODEBUG, which gives the dump its line numbers) on govax,
// and matches VMS's output, dumps and all, but for the PCs outside the
// image (tb70Masked) and tb70DumpDifferences.
func TestTB70Traceback(t *testing.T) {
	_, dispatch := tb70Console(t)

	for _, image := range []string{"TB70", "TB70D"} {
		want := tb70VAXRuns(t, image)

		for _, part := range tb70Parts {
			if _, ok := tb70DumpDifferences[part]; ok {
				continue
			}

			dispatch("DEFINE/NOLOG TB70_PART " + part)

			got := strings.Split(dispatch("RUN/NODEBUG "+image), "|")
			got = append(got, strings.TrimSpace(dispatch("SHOW SYMBOL $STATUS")))

			g, w := strings.Join(tb70Masked(got), "\n"), strings.Join(tb70Masked(want[part]), "\n")
			if g != w {
				t.Errorf("%s part %s:\ngot:\n%s\nwant:\n%s", image, part, g, w)
			}
		}
	}
}

// TestTB70ImproperlyHandled runs the parts of VMS's TB70NT (linked
// /NOTRACEBACK) and of round 2's TB70BNT on govax, and part X of each
// round 1 image, and matches VMS's output and its "improperly handled
// condition" dumps (excmsgMatch's rules), but for the PCs outside the
// image (tb70Masked) and tb70DumpDifferences, X aside.
func TestTB70ImproperlyHandled(t *testing.T) {
	_, dispatch := tb70Console(t)

	type run struct {
		image, part, define string
		want                []string
	}

	var runs []run

	nt := tb70VAXRuns(t, "TB70NT")
	for _, part := range tb70Parts {
		if _, ok := tb70DumpDifferences[part]; !ok || part == "X" {
			runs = append(runs, run{"TB70NT", part, "TB70_PART", nt[part]})
		}
	}

	for _, image := range []string{"TB70", "TB70D"} {
		runs = append(runs, run{image, "X", "TB70_PART", tb70VAXRuns(t, image)["X"]})
	}

	bnt := tb70BRuns(t, "TB70BNT")
	for _, part := range []string{"O", "P", "Q", "S"} {
		runs = append(runs, run{"TB70BNT", part, "TB70B_PART", bnt[part]})
	}

	for _, r := range runs {
		if len(r.want) == 0 {
			t.Fatalf("%s part %s: not in VMS's log", r.image, r.part)
		}

		dispatch("DEFINE/NOLOG " + r.define + " " + r.part)

		got := strings.Split(dispatch("RUN/NODEBUG "+r.image), "|")
		got = append(got, strings.TrimSpace(dispatch("SHOW SYMBOL $STATUS")))

		for _, problem := range excmsgMatch(tb70Masked(got), tb70Masked(r.want), excmsgLoose[r.part] != "", excmsgUnset[r.image+" "+r.part]) {
			t.Errorf("%s part %s: %s\ngot:\n%s\nwant:\n%s", r.image, r.part, problem, strings.Join(got, "\n"), strings.Join(r.want, "\n"))
		}
	}
}

// excmsgLoose are the parts whose dumps' registers and stack aren't
// compared, and why: their layout, signal arrays, PCs, and PSLs are.
var excmsgLoose = map[string]string{
	// An access violation in a condition handler: SP is below the
	// dispatcher's frames and arrays, which aren't VMS's.
	"H": "the dispatcher's frames",
	// An access violation in an AST routine: SP is below the AST
	// delivery's frame, which isn't VMS's.
	"T": "the AST delivery's frame",
	// TB70SHR's frames hold its PCs, and govax places it elsewhere.
	"N": "TB70SHR's place",
	// The exit handler is called by image rundown's code, which isn't
	// VMS's: its frame is elsewhere on the stack, the stack below it
	// holds rundown's frames, and the registers are rundown's.
	"X": "image rundown's frames and registers",
}

// excmsgUnset are, by run, the registers and stack longwords ("stack
// n", n from 0) of the dumps that aren't compared, beside those
// excmsgMatch passes over itself: values the program never set.
var excmsgUnset = map[string][]string{
	// The program's R0 at a LIB$SIGNAL or LIB$STOP call, which VMS
	// shows, is gone by the time govax's shim runs: its stub has loaded
	// its own code into R0.
	"TB70NT C": {"R0"},
	// TB70B's main routine never sets R2, which VMS's image activator
	// left 4 and govax's leaves something else; part S's PART_S saves
	// it in its frame.
	"TB70BNT O": {"R0", "R2"},
	"TB70BNT Q": {"R0", "R2"},
	"TB70BNT S": {"R2", "stack 8"},
}

// excmsgHex is a longword in a dump.
var excmsgHex = regexp.MustCompile(`[0-9A-F]{8}`)

// excmsgRegister is a register's name and value in a register dump line.
var excmsgRegister = regexp.MustCompile(`(R[0-9]+|AP|FP|SP|PC|PSL) ?= ([0-9A-F]{8})`)

// govaxStackOffset is what moves an address in govax's user stack, in
// a run under RUN, to the same place in VMS's: VMS's is D53E8 bytes
// higher (testdata/tb70's runs). Above the main routine's argument list
// (govaxStartup, VMS's vmsStartup), it's D5400:
// that's the image startup's and DCL's part of the stack, laid out
// differently (VMS's image startup passes the main routine six
// arguments, govax's none).
const (
	govaxStackOffset = 0xD53E8
	govaxStartup     = 0x7FDFFFE4
	vmsStartup       = 0x7FED53CC
	startupOffset    = 0xD5400
)

// excmsgMatch compares a run's lines, got (govax's) and want (VMS's),
// returning what differs. Outside an "improperly handled condition"
// dump they must be the same. In one, the layout and the signal array,
// PC, and PSL must be the same; SP, FP, AP, and the stack's addresses
// the same once govax's are moved to VMS's stack (govaxStackOffset).
// A register or stack longword must be the same too, unless VMS's is an
// address in its own P1 or system space (7FF00000 and up), a value the
// program never set, left by the image activator or DCL, which govax
// doesn't have; the stack from the main routine's argument list up
// (vmsStartup) isn't compared; nor are unset's values, nor a loose
// part's registers and stack.
func excmsgMatch(got, want []string, loose bool, unset []string) []string {
	if len(got) != len(want) {
		return []string{fmt.Sprintf("%d lines, want %d", len(got), len(want))}
	}

	var problems []string

	skip := map[string]bool{}
	for _, name := range unset {
		skip[name] = true
	}

	// VMS's SP, where the stack column starts.
	var vmsSP uint64

	for _, w := range want {
		for _, m := range excmsgRegister.FindAllStringSubmatch(w, -1) {
			if m[1] == "SP" {
				vmsSP, _ = strconv.ParseUint(m[2], 16, 32)
			}
		}
	}

	inDump, inStack, row := false, false, 0

	for i := range got {
		g, w := got[i], want[i]

		switch {
		case strings.HasPrefix(w, "  Improperly handled condition"):
			inDump = true
		case strings.HasPrefix(w, "\tSignal arguments"):
			inStack, row = true, 0

			continue
		case strings.HasPrefix(w, "\tRegister dump"):
			inStack = false
		}

		if !inDump {
			if g != w {
				problems = append(problems, fmt.Sprintf("line %d: %q, want %q", i+1, g, w))
			}

			continue
		}

		if strings.HasPrefix(w, "\tPSL=") {
			inDump = false
		}

		if excmsgHex.ReplaceAllString(g, "#") != excmsgHex.ReplaceAllString(w, "#") {
			problems = append(problems, fmt.Sprintf("line %d's layout: %q, want %q", i+1, g, w))

			continue
		}

		gv, wv := excmsgHex.FindAllString(g, -1), excmsgHex.FindAllString(w, -1)
		names := make([]string, len(wv))

		if inStack && len(wv) > 0 {
			// The last longword is the stack's, any before it the
			// signal array's.
			names[len(names)-1] = fmt.Sprintf("stack %d", row)

			if vmsSP+uint64(4*row) >= vmsStartup {
				names[len(names)-1] = "startup"
			}

			row++
		} else {
			for j, m := range excmsgRegister.FindAllStringSubmatch(w, -1) {
				names[j] = m[1]
			}
		}

		for j := range wv {
			name := names[j]

			if skip[name] || name == "startup" || excmsgValueMatches(name, gv[j], wv[j], loose) {
				continue
			}

			problems = append(problems, fmt.Sprintf("line %d's %s: %s, want %s", i+1, cmp.Or(name, "signal array"), gv[j], wv[j]))
		}
	}

	return problems
}

// excmsgValueMatches is excmsgMatch's rule for one longword of a dump,
// named name ("stack n", a register's name, or "" for the signal
// array's).
func excmsgValueMatches(name, got, want string, loose bool) bool {
	g, _ := strconv.ParseUint(got, 16, 32)
	w, _ := strconv.ParseUint(want, 16, 32)

	switch {
	case g > govaxStartup && g < 0x7FE00000:
		g += startupOffset
	case g >= 0x7FC00000 && g <= govaxStartup:
		g += govaxStackOffset
	}

	switch name {
	case "", "PC", "PSL":
		return got == want

	case "SP", "FP", "AP":
		return loose || g == w
	}

	return loose || g == w || w >= 0x7FF00000
}

// TestTB70Output replays the parts of TB70.COM and TB70B.COM that run
// an image with SYS$OUTPUT and SYS$ERROR defined to different files
// (round 1's A, with and without traceback, and S; round 2's N and M),
// then TB70.COM's SPAWN of part A and its run with no part, and matches
// VMS's output line for line: where the messages and dumps went, as
// TYPE shows each file (excmsgMatch's rules for the "improperly
// handled" dump, tb70Masked's for PCs outside the image).
func TestTB70Output(t *testing.T) {
	for _, r := range []struct{ file, from, to, define string }{
		{"tb70.log", "$ DEFINE TB70_PART A", "$ DIRECTORY/SIZE=ALL/DATE", "TB70_PART"},
		{"tb70b.log", "$ DEFINE TB70B_PART N", "$ DIRECTORY/SIZE=ALL TB70B*.*", "TB70B_PART"},
	} {
		_, dispatch := tb70Console(t)

		// The procedure's earlier runs left the part's name defined.
		dispatch("DEFINE/NOLOG " + r.define + " S")

		for _, cmd := range tb70LogCommands(t, r.file, r.from, r.to) {
			if cmd.line == "" {
				continue
			}

			var got []string

			for _, line := range strings.Split(dispatch(cmd.line), "|") {
				if line != "" {
					got = append(got, line)
				}
			}

			for _, problem := range excmsgMatch(tb70Masked(got), tb70Masked(cmd.want), false, nil) {
				t.Errorf("%s: %s: %s\ngot:\n%s\nwant:\n%s", r.file, cmd.line, problem, strings.Join(got, "\n"), strings.Join(cmd.want, "\n"))
			}
		}
	}
}

// tb70Command is a command of a VMS log (tb70LogCommands) and its
// output, without blank lines.
type tb70Command struct {
	line string
	want []string
}

// tb70LogCommands returns the commands of the VMS log file in vax/ from
// the line from to the line to (not included), each with its output,
// leaving out comments.
func tb70LogCommands(t *testing.T, file, from, to string) []tb70Command {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(tb70Dir, "vax", file))
	if err != nil {
		t.Fatal(err)
	}

	var commands []tb70Command

	started := false

	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
		switch {
		case line == from:
			started = true
		case !started:
			continue
		case line == to:
			return commands
		}

		switch cmd, ok := strings.CutPrefix(line, "$ "); {
		case ok && strings.HasPrefix(cmd, "!"):
			commands = append(commands, tb70Command{}) // output after a comment is none
		case ok:
			commands = append(commands, tb70Command{line: cmd})
		case line != "" && len(commands) > 0:
			commands[len(commands)-1].want = append(commands[len(commands)-1].want, line)
		}
	}

	t.Fatalf("%s: no %q after %q", file, to, from)

	return nil
}

// TestTB70OutputSameFile runs part A twice with SYS$OUTPUT and SYS$ERROR
// both defined to one file: each run's message and dump are written to
// it once, and each image makes a version of its own, as its own open of
// the file would (the image's files are forgotten at rundown).
func TestTB70OutputSameFile(t *testing.T) {
	_, dispatch := tb70Console(t)

	dispatch("DEFINE/NOLOG TB70_PART A")

	for range 2 {
		dispatch("DEFINE/USER_MODE SYS$OUTPUT TB70SAME.LOG")
		dispatch("DEFINE/USER_MODE SYS$ERROR TB70SAME.LOG")

		if got := dispatch("RUN TB70"); got != "" {
			t.Errorf("RUN TB70 wrote %q to the terminal", got)
		}
	}

	for _, version := range []string{"1", "2"} {
		got := strings.Split(dispatch("TYPE TB70SAME.LOG;"+version), "|")
		if len(got) != 7 || !strings.HasPrefix(got[0], "%SYSTEM-F-ACCVIO,") || !strings.HasPrefix(got[1], "%TRACE-F-TRACEBACK,") {
			t.Errorf("TB70SAME.LOG;%s:\n%s", version, strings.Join(got, "\n"))
		}
	}
}
