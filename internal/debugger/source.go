package debugger

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmserrors"
)

// Source lines in the debugger's reports (docs/PHASE-42.md, subtask 8).
//
// After a breakpoint, a step, or an exception break, the VMS debugger shows
// the line of source code the program is stopped at, below its "break at"
// or "stepped to" message:
//
//	break at DBGCMD\FACT\BACK
//	    61: BACK:   MULL2   R2, R0                  ; each FACT call ...
//
// The number is right-aligned in six columns, then a colon and a blank,
// then the line with its tabs expanded to every eighth column. A line that
// is too long for the 80-column terminal continues on the next, marked
// with a dash in place of the number:
//
//	   110: JSBRTN: INCL    COUNT                   ; a JSB subroutine, ...
//	     -: e
//
// If the source file can't be found (see Console.SourceLine) nothing is
// shown, and the report is just the location line.

const (
	// sourceWidth is the terminal width the debugger wraps source lines at.
	sourceWidth = 80

	// sourcePrefix is the width of "   110: ": the number, colon, and
	// blank that start a source line, or "     -: " on a continuation.
	sourcePrefix = 8

	// tabStop is the distance between tab stops in the displayed text.
	tabStop = 8
)

// showSource prints the source line at pc, if there is one to show.
//
// While the program runs (runLoop), a source line is shown once for each
// instruction however many reports are made at it: when a tracepoint and a
// breakpoint (or two tracepoints) are reached at one pc, only the first
// report is followed by the line (the probe's trace.dlg).
func (d *Debugger) showSource(pc uint32) {
	if d.shown.active {
		if d.shown.ok && d.shown.pc == pc {
			return
		}

		d.shown.pc, d.shown.ok = pc, true
	}

	n, text, ok := d.Machine.SourceLine(pc)
	if !ok {
		return
	}

	d.showFortranLine(pc)
	d.Machine.Printf("%s", formatSource(n, text))
}

// formatSource lays out source line n, whose text is text, as the debugger
// displays it: the numbered line, and a continuation line for each further
// 72 columns of text. The result ends with a newline.
func formatSource(n int, text string) string {
	text = expandTabs(text)
	room := sourceWidth - sourcePrefix

	var b strings.Builder

	b.WriteString(fmt.Sprintf("%*d: ", sourcePrefix-2, n))

	for {
		chunk := text
		if len(chunk) > room {
			chunk = text[:room]
		}

		b.WriteString(chunk)
		b.WriteRune('\n')

		text = text[len(chunk):]
		if text == "" {
			return b.String()
		}

		b.WriteString(fmt.Sprintf("%*s: ", sourcePrefix-2, "-"))
	}
}

// expandTabs replaces each tab with the blanks that reach the next tab stop,
// counting columns from the start of the text.
func expandTabs(s string) string {
	if !strings.Contains(s, "\t") {
		return s
	}

	var b strings.Builder

	for _, r := range s {
		if r != '\t' {
			b.WriteRune(r)

			continue
		}

		b.WriteString(strings.Repeat(" ", tabStop-len([]rune(b.String()))%tabStop))
	}

	return b.String()
}

// SetSource implements SET SOURCE dir[,dir...]: the directories the
// debugger searches for a module's source file when the file named in the
// program's debug symbols isn't where it says. A directory is a host path
// or a VMS directory specification. As everywhere in the debugger's
// grammar, unquoted text is made upper case and a "/" starts a qualifier,
// so a host path, whose case counts, is written in quotes.
func (d *Debugger) SetSource(list string) error {
	// A table comes from a file the new list may find elsewhere.
	d.fortranTables = nil

	var dirs []string

	for _, dir := range splitTop(list, ',') {
		if dir = strings.Trim(strings.TrimSpace(dir), `"`); dir != "" {
			dirs = append(dirs, dir)
		}
	}

	if len(dirs) == 0 {
		return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "directories")
	}

	d.Machine.SetSourceDirs(dirs)

	return nil
}

// ShowSource implements SHOW SOURCE: the directory search list.
func (d *Debugger) ShowSource() error {
	dirs := d.Machine.SourceDirs()
	if len(dirs) == 0 {
		d.Machine.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_NOSOURCEDIR))

		return nil
	}

	d.Machine.Printf("source directory search list for all modules:\n")

	for _, dir := range dirs {
		d.Machine.Printf("    %s\n", dir)
	}

	return nil
}

// bindSource binds SET SOURCE, SHOW SOURCE, and CANCEL SOURCE.
// examineSource is EXAMINE/SOURCE of a location: the source line of the code there, under
// its module's name, as VMS's shows it (testdata/dbg85).
func (d *Debugger) examineSource(items []string) error {
	shown := ""

	for _, item := range items {
		pc, err := d.evalWhole(item)
		if err != nil {
			return err
		}

		prog := d.Machine.DebugProgramAt(pc)
		if prog == nil {
			return vmserrors.New(vmserrors.DBG_NOSYMBOL, item)
		}

		_, mod, found := prog.LineAt(pc)
		if !found {
			return vmserrors.New(vmserrors.DBG_NOSYMBOL, item)
		}

		n, text, ok := d.Machine.SourceLine(pc)
		if !ok {
			return vmserrors.New(vmserrors.DBG_NOSYMBOL, item)
		}

		if mod.Name != shown {
			d.Machine.Printf("module %s\n", mod.Name)
			shown = mod.Name
		}

		d.Machine.Printf("%s", formatSource(n, text))
	}

	return nil
}

// typeSource is TYPE [module\]line[:line][,...]: lines of a module's source, of the module
// the program is in unless one is named, as VMS's shows them (testdata/dbg85).
func (d *Debugger) typeSource(text string) error {
	for _, item := range strings.Split(text, ",") {
		item = strings.TrimSpace(item)

		mod := d.scopeModule()

		if name, rest, named := strings.Cut(item, `\`); named {
			if mod = d.moduleNamed(name); mod == nil {
				return vmserrors.New(vmserrors.DBG_NOSUCHMODULE, name)
			}

			item = rest
		}

		if mod == nil {
			return vmserrors.New(vmserrors.DBG_NOSUCHMODULE, item)
		}

		low, high, ranged := strings.Cut(item, ":")
		if !ranged {
			high = low
		}

		first, err := d.evalWhole(strings.TrimSpace(low))
		if err != nil {
			return err
		}

		last, err := d.evalWhole(strings.TrimSpace(high))
		if err != nil {
			return err
		}

		records := d.Machine.ModuleSource(mod)

		d.Machine.Printf("module %s\n", mod.Name)

		for n := int(first); n <= int(last) && n >= 1 && n <= len(records); n++ {
			d.Machine.Printf("%s", formatSource(n, records[n-1]))
		}
	}

	return nil
}

// scopeModule is the module the program is in, or the current image's first.
func (d *Debugger) scopeModule() *dbgsym.Module {
	pc := d.Machine.CPU.GPR(registers.PC)

	if prog := d.Machine.DebugProgramAt(pc); prog != nil {
		if m, ok := prog.ModuleAt(pc); ok {
			return m
		}
	}

	if modules := d.imageModules(); len(modules) > 0 {
		return modules[0]
	}

	return nil
}

func (d *Dispatcher) bindSource() {
	d.Grammar.Bind("TYPE", func(id int64, r *dcl.Result) error {
		if text := strings.TrimSpace(r.String("LINES")); text != "" {
			return d.Debugger.typeSource(text)
		}

		return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "lines")
	})

	d.Grammar.Bind("SET_SOURCE", func(id int64, r *dcl.Result) error {
		return d.Debugger.SetSource(r.String("DIRECTORIES"))
	})
	d.Grammar.Bind("SHOW_SOURCE", func(id int64, r *dcl.Result) error { return d.Debugger.ShowSource() })
	d.Grammar.Bind("CANCEL_SOURCE", func(id int64, r *dcl.Result) error {
		d.Debugger.Machine.SetSourceDirs(nil)

		return nil
	})
}
