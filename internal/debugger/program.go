package debugger

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/symtab"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is what the debugger knows about the program it is
// debugging (docs/PHASE-42.md, subtask 12): SHOW IMAGE, SHOW MODULE,
// SHOW SYMBOL, SHOW SCOPE, SHOW LANGUAGE, and SET MODULE.
//
// Some background for a reader new to VMS. A program is built from one or
// more *modules*: each source file the assembler (or a compiler) turns
// into an object module. The linker joins the modules into an *image*, the
// executable file (.EXE). An image linked with /DEBUG (or with traceback,
// the default) carries a *debug symbol table* (DST) that describes each
// module: its routines, labels, and data, with their addresses and types,
// and which source line each instruction came from.
//
// The VMS debugger loads the symbols of one module at first (the module
// holding the program's main routine) and the others only when asked
// (SET MODULE). govax reads every module's symbols when it loads the
// image (internal/dbgsym), so loading is not a real step here. SET
// MODULE only changes what SHOW MODULE reports in its "symbols" column,
// and which modules SHOW SYMBOL searches when it isn't told which.
//
// A shareable image (a library such as LIBRTL) is also an image the
// debugger lists. In a real VMS session SHOW IMAGE also lists the
// debugger's own images; govax lists only the images it has loaded.

// bindProgram binds the commands of this file.
func (d *Dispatcher) bindProgram() {
	g := d.Grammar
	dbg := d.Debugger

	g.Bind("SHOW_IMAGE", func(id int64, r *dcl.Result) error { return dbg.showImage(r.Present("FULL")) })
	g.Bind("SHOW_MODULE", func(id int64, r *dcl.Result) error { return dbg.showModule() })
	g.Bind("SHOW_LANGUAGE", func(id int64, r *dcl.Result) error { return dbg.showLanguage() })
	g.Bind("SHOW_SCOPE", func(id int64, r *dcl.Result) error { return dbg.showScope() })
	g.Bind("SHOW_SYMBOL", func(id int64, r *dcl.Result) error {
		return dbg.showSymbol(r.String("PATTERN"), r.Present("ADDRESS"), r.Present("TYPE"))
	})
	g.Bind("SET_MODULE", func(id int64, r *dcl.Result) error {
		return dbg.setModule(r.String("MODULES"), r.Present("ALL"))
	})
}

// allModules returns the modules of every loaded image that has debug
// symbols, in the order the images were loaded and the DST lists them.
func (d *Debugger) allModules() []*dbgsym.Module {
	var out []*dbgsym.Module

	for _, p := range d.Console.DebugPrograms() {
		out = append(out, p.Modules...)
	}

	return out
}

// seedModules makes the main routine's module the one module whose
// symbols are set, as the VMS debugger starts: the module that holds the
// program counter now. With the PC in no module (no image), the first
// module is used. startImage calls it when the program stops at its first
// instruction; a session that never ran an image gets it on first use.
func (d *Debugger) seedModules() {
	d.symbolsSet = map[*dbgsym.Module]bool{}

	if p := d.Console.DebugProgramAt(d.Console.CPU.GPR(vax.PC)); p != nil {
		if m, ok := p.ModuleAt(d.Console.CPU.GPR(vax.PC)); ok {
			d.symbolsSet[m] = true

			return
		}
	}

	if all := d.allModules(); len(all) > 0 {
		d.symbolsSet[all[0]] = true
	}
}

// moduleSet reports whether module m's symbols are "set" (loaded, in the
// VMS debugger's words).
func (d *Debugger) moduleSet(m *dbgsym.Module) bool {
	if d.symbolsSet == nil {
		d.seedModules()
	}

	return d.symbolsSet[m]
}

// yesNo is how SHOW IMAGE and SHOW MODULE print a flag.
func yesNo(b bool) string {
	if b {
		return "yes"
	}

	return "no"
}

// showImage runs SHOW IMAGE: a table of the loaded images by name, with a
// "*" before the main one (the image RUN started) and whether the
// debugger has its symbols. The layout is VMS's, from the probe's
// sessions:
//
//	 image name                      set    base address   end address
//
//	*DBGDIS                          yes    00000200       000007FF
//
//	 total images: 1                 bytes allocated: 1536
//
// "bytes allocated" is the size of VMS's debugger's own tables, which
// govax has no counterpart of; it shows the total size of the images
// instead (unconfirmed, no probe could say what VMS counts).
//
// /FULL is govax's own: it adds, below each image with symbols, how many
// modules and routines they describe.
func (d *Debugger) showImage(full bool) error {
	c := d.Console

	if err := c.RequireInit(); err != nil {
		return err
	}

	images := c.Images()
	sort.SliceStable(images, func(i, j int) bool { return images[i].Name < images[j].Name })

	c.Printf(" image name                      set    base address   end address\n\n")

	total := uint32(0)

	for _, img := range images {
		mark := ' '
		if img.Main {
			mark = '*'
		}

		c.Printf("%c%-32s%-7s%-15s%08X\n", mark, img.Name, yesNo(img.Program != nil),
			fmt.Sprintf("%08X", img.Base), img.End)

		total += img.End - img.Base + 1

		if full && img.Program != nil {
			routines := 0
			for _, m := range img.Program.Modules {
				routines += len(m.Routines)
			}

			c.Printf("    %d modules, %d routines\n", len(img.Program.Modules), routines)
		}
	}

	c.Printf("\n %-32sbytes allocated: %d\n", fmt.Sprintf("total images: %d", len(images)), total)

	return nil
}

// showModule runs SHOW MODULE: a table of every module of the loaded
// images, by name, with whether its symbols are set and the size of its
// debug records. VMS's layout:
//
//	module name                     symbols    size
//
//	DBGDIS                          yes        1804
//	DBGSUB                          no          644
//
//	total MACRO modules: 2.         bytes allocated: 67120.
//
// As in SHOW IMAGE, "bytes allocated" is govax's figure (the size of the
// set modules' records), since VMS's counts its own memory.
func (d *Debugger) showModule() error {
	c := d.Console

	if err := c.RequireInit(); err != nil {
		return err
	}

	modules := d.allModules()
	sort.SliceStable(modules, func(i, j int) bool { return modules[i].Name < modules[j].Name })

	c.Printf("module name                     symbols    size\n\n")

	var (
		total     uint32
		languages []string
	)

	for _, m := range modules {
		set := d.moduleSet(m)
		if set {
			total += m.DSTSize
		}

		c.Printf("%-32s%-3s%12d\n", m.Name, yesNo(set), m.DSTSize)

		// The footer names the language the modules share ("MACRO").
		lang := console.LanguageName(m.Language)
		if len(languages) == 0 || languages[len(languages)-1] != lang {
			languages = append(languages, lang)
		}
	}

	kind := "total"
	if len(languages) == 1 {
		kind += " " + languages[0]
	}

	c.Printf("\n%-32sbytes allocated: %d.\n", fmt.Sprintf("%s modules: %d.", kind, len(modules)), total)

	return nil
}

// setModule runs SET MODULE name[,name...] and SET MODULE/ALL: marks
// modules as having their symbols set. A name no module has is
// %DEBUG-E-NOSUCHMODULE (govax's wording).
func (d *Debugger) setModule(list string, all bool) error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	if d.symbolsSet == nil {
		d.seedModules()
	}

	if all {
		for _, m := range d.allModules() {
			d.symbolsSet[m] = true
		}

		return nil
	}

	names := splitTop(list, ',')
	if len(strings.TrimSpace(list)) == 0 {
		return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "module")
	}

	// Check every name before setting any, so a bad one changes nothing.
	var found []*dbgsym.Module

	for _, name := range names {
		name = strings.TrimSpace(name)

		m := d.moduleNamed(name)
		if m == nil {
			return vmserrors.New(vmserrors.DBG_NOSUCHMODULE, name)
		}

		found = append(found, m)
	}

	for _, m := range found {
		d.symbolsSet[m] = true
	}

	return nil
}

// moduleNamed finds a module of any loaded image by name (ignoring case).
func (d *Debugger) moduleNamed(name string) *dbgsym.Module {
	for _, m := range d.allModules() {
		if strings.EqualFold(m.Name, name) {
			return m
		}
	}

	return nil
}

// showLanguage runs SHOW LANGUAGE: the language of the module the program
// counter is in ("language: MACRO"). With the PC in no module the answer
// is UNKNOWN (govax's choice; the probe had no such case).
func (d *Debugger) showLanguage() error {
	c := d.Console

	if err := c.RequireInit(); err != nil {
		return err
	}

	pc := c.CPU.GPR(vax.PC)
	language := "UNKNOWN"

	if p := c.DebugProgramAt(pc); p != nil {
		if m, ok := p.ModuleAt(pc); ok {
			language = console.LanguageName(m.Language)
		}
	}

	c.Printf("language: %s\n", language)

	return nil
}

// showScope runs SHOW SCOPE: the call levels, innermost first, that the
// debugger looks names up in. The innermost, level 0, is the routine the
// program is in and is marked "*" (the VMS debugger's SET SCOPE can pick
// another; govax doesn't have it yet, so the current scope is always
// level 0). From the probe:
//
//	scope:
//	 *  0 [ = DBGCMD\FACT ],
//	    1 [ = DBGCMD\FACT 1 ],
//	    2 [ = DBGCMD\START ]
//
// A routine that is already in a more inner level is written again with
// a number after it, how many levels of the same routine are inside it
// (the recursive FACT's second activation is "FACT 1"). Only one and
// two activations are confirmed by the probe; the numbering past that
// is unconfirmed.
func (d *Debugger) showScope() error {
	c := d.Console

	if err := c.RequireInit(); err != nil {
		return err
	}

	paths, ok, err := c.ScopeFrames()
	if !ok || d.imageExited || len(paths) == 0 {
		return vmserrors.New(vmserrors.DBG_NOCALLS)
	}

	c.Printf("scope: \n")

	seen := map[string]int{}

	for level, path := range paths {
		name := path
		if n := seen[path]; n > 0 {
			name = fmt.Sprintf("%s %d", path, n)
		}

		seen[path]++

		mark := ' '
		if level == 0 {
			mark = '*'
		}

		// Every level but the last ends with a comma and a blank, as the
		// VMS debugger prints it.
		tail := ""
		if level < len(paths)-1 {
			tail = ", "
		}

		c.Printf(" %c %2d [ = %s ]%s\n", mark, level, name, tail)
	}

	return err
}

// showSymbol runs SHOW SYMBOL [/ADDRESS] [/TYPE] pattern [IN module[,...]]:
// lists the program's symbols whose names match pattern, a name with the
// VMS wildcards "*" (any run of characters) and "%" (one character). With
// IN the search is those modules; without it, the modules whose symbols
// are set (SET MODULE), and, when none of them has a match, the console's
// own symbol table (the symbols DEFINE and SET SYMBOL make).
//
// Each symbol is a line "kind PATH" and, below it, what the qualifier
// asks for: /ADDRESS (the default) gives its address, size, or constant
// value; /TYPE gives a datum's type; both give both. The kinds are
// "routine", "data", and "label", where a program section (psect) counts
// as a label. Everything here is VMS's layout from the probe (the
// default of neither qualifier is govax's choice, the same as /ADDRESS).
//
// The order is VMS's: each module's routines by name, then its data and
// labels by name, then its program sections in address order.
func (d *Debugger) showSymbol(text string, address, typed bool) error {
	c := d.Console

	if err := c.RequireInit(); err != nil {
		return err
	}

	pattern, modules := splitIn(text)
	if pattern == "" {
		return vmserrors.New(vmserrors.CLI_MISSINGPARAMETER, "symbol")
	}

	if !address && !typed {
		address = true
	}

	var searched []*dbgsym.Module

	if modules == nil {
		for _, m := range d.allModules() {
			if d.moduleSet(m) {
				searched = append(searched, m)
			}
		}
	}

	for _, name := range modules {
		m := d.moduleNamed(name)
		if m == nil {
			return vmserrors.New(vmserrors.DBG_NOSUCHMODULE, name)
		}

		searched = append(searched, m)
	}

	matched := 0

	for _, m := range searched {
		for _, s := range orderedSymbols(m) {
			if !symbolMatches(pattern, m, s) {
				continue
			}

			matched++

			d.printSymbol(m, s, address, typed)
		}
	}

	if matched > 0 {
		return nil
	}

	// Nothing in the program's symbols: for a search of the whole
	// program, try the console's table before giving up.
	if modules == nil && c.HasSymbol(pattern) {
		return c.ShowSymbol(pattern)
	}

	return vmserrors.New(vmserrors.DBG_NOSYMBOL, pattern)
}

// splitIn separates SHOW SYMBOL's parameter "pattern IN module[,module]"
// into the pattern and the module names (nil when there is no IN). The
// word IN is found ignoring case, as a word of its own.
func splitIn(text string) (pattern string, modules []string) {
	text = strings.TrimSpace(text)
	upper := strings.ToUpper(text)

	at := strings.Index(upper, " IN ")
	if at < 0 {
		return text, nil
	}

	for _, name := range strings.Split(text[at+4:], ",") {
		if name = strings.TrimSpace(name); name != "" {
			modules = append(modules, name)
		}
	}

	return strings.TrimSpace(text[:at]), modules
}

// orderedSymbols returns a module's symbols in the order SHOW SYMBOL lists
// them: routines by name, then data and labels by name, then program
// sections by address.
func orderedSymbols(m *dbgsym.Module) []*symtab.Symbol {
	var routines, others, psects []*symtab.Symbol

	for _, s := range m.Symbols.All() {
		switch {
		case s.IsEntry():
			routines = append(routines, s)
		case s.Has(symtab.Psect):
			psects = append(psects, s)
		default:
			others = append(others, s)
		}
	}

	byName := func(list []*symtab.Symbol) {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	}

	byName(routines)
	byName(others)
	sort.SliceStable(psects, func(i, j int) bool { return psects[i].Value < psects[j].Value })

	return append(append(routines, others...), psects...)
}

// symbolMatches reports whether symbol s of module m matches SHOW
// SYMBOL's pattern. A pattern with no backslash is matched against the
// symbol's own name; one with a backslash against its whole path name
// (DBGDIS\START\LOOP).
func symbolMatches(pattern string, m *dbgsym.Module, s *symtab.Symbol) bool {
	if strings.Contains(pattern, `\`) {
		return lnm.Match(strings.ToUpper(pattern), strings.ToUpper(m.Path(s)))
	}

	return lnm.Match(strings.ToUpper(pattern), strings.ToUpper(s.Name))
}

// printSymbol prints one symbol of SHOW SYMBOL's listing.
func (d *Debugger) printSymbol(m *dbgsym.Module, s *symtab.Symbol, address, typed bool) {
	c := d.Console
	datum := m.DatumNamed(s.Name)

	var kind string

	switch {
	case s.IsEntry():
		kind = "routine"
	case s.Has(symtab.Psect), s.IsLabel():
		kind = "label"
	default:
		kind = "data"
	}

	c.Printf("%s %s\n", kind, m.Path(s))

	if address {
		switch {
		case s.IsEntry(), s.Has(symtab.Psect):
			// A routine's or psect's extent is shown with its address.
			c.Printf("    address: %08X, size: %08X bytes\n", s.Value, s.Size)
		case s.IsLabel():
			c.Printf("    address: %08X\n", s.Value)
		case s.Has(symtab.Literal):
			c.Printf("    constant: %08X\n", s.Value)
		case datum != nil && (datum.Kind == dbgsym.DescriptorAddress || datum.Descriptor != nil):
			// A string or array described by a descriptor: VMS shows the
			// address of the descriptor, which for its debugger is in
			// the debugger's own memory. govax shows the datum's address
			// (or, for .ASCID's label, the descriptor's: the label's).
			c.Printf("    descriptor address: %08X\n", s.Value)
		default:
			c.Printf("    address: %08X\n", s.Value)
		}
	}

	// Only data have a type; routines and labels show nothing more.
	if typed && kind == "data" && datum != nil {
		d.printType(datum, s.Size)
	}
}

// printType prints the type of a datum as SHOW SYMBOL/TYPE does: an
// atomic type (one number), a string descriptor, or an array descriptor
// and its cell type on a second line.
func (d *Debugger) printType(datum *dbgsym.Datum, symbolSize uint32) {
	c := d.Console

	switch {
	case datum.IsText():
		size := symbolSize
		if datum.Descriptor != nil {
			size = uint32(datum.Descriptor.Length)
		} else if datum.Kind == dbgsym.DescriptorAddress {
			// .ASCID's label names a descriptor in the program's own
			// memory, whose first word is the string's length.
			if b, err := c.ReadBytes(datum.Value, 2); err == nil {
				size = uint32(b[0]) | uint32(b[1])<<8
			}
		}

		c.Printf("    string descriptor type, character-coded text, size: %s\n", byteCount(size))

	case datum.IsArray():
		desc := datum.Descriptor

		// The size is every cell: the product of each dimension's
		// extent, times the cell's length.
		cells, bounds := uint32(1), make([]string, 0, len(desc.Bounds))
		for _, b := range desc.Bounds {
			cells *= uint32(b.Upper - b.Lower + 1)
			bounds = append(bounds, fmt.Sprintf("%d:%d", b.Lower, b.Upper))
		}

		plural := "s"
		if len(desc.Bounds) == 1 {
			plural = ""
		}

		c.Printf("    array descriptor type, %d dimension%s, bounds: [%s], size: %s\n",
			len(desc.Bounds), plural, strings.Join(bounds, ","), byteCount(cells*uint32(desc.Length)))
		c.Printf("        cell type: atomic type, %s, size: %s\n", dataTypeName(desc.Type), byteCount(uint32(desc.Length)))

	default:
		size := dbgsym.TypeSize(datum.Type)
		if size == 0 {
			size = symbolSize
		}

		c.Printf("    atomic type, %s, size: %s\n", dataTypeName(datum.Type), byteCount(size))
	}
}

// byteCount is a size as the debugger writes it: "1 byte", "4 bytes".
func byteCount(n uint32) string {
	if n == 1 {
		return "1 byte"
	}

	return fmt.Sprintf("%d bytes", n)
}

// dataTypeName is the name of a VMS data type code (the DSC$K_DTYPE_*
// values the debug symbols use) as the debugger shows it: a "logical"
// type is an unsigned integer and an "integer" type a signed one.
func dataTypeName(code byte) string {
	switch code {
	case 2:
		return "byte logical"
	case 3:
		return "word logical"
	case 4:
		return "longword logical"
	case 5:
		return "quadword logical"
	case 6:
		return "byte integer"
	case 7:
		return "word integer"
	case 8:
		return "longword integer"
	case 9:
		return "quadword integer"
	case 10:
		return "F_floating"
	case 11:
		return "D_floating"
	case 14:
		return "character-coded text"
	case 25:
		return "octaword logical"
	case 26:
		return "octaword integer"
	case 27:
		return "G_floating"
	case 28:
		return "H_floating"
	}

	return fmt.Sprintf("type %d", code)
}
