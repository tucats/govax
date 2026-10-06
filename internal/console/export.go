package console

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/disasm"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is the surface the debugger (internal/debugger) reaches the
// console through. The debugger runs the program, but the console still
// owns what makes the machine a VMS one: image activation and rundown, the
// condition handling facility (CHF), the symbol tables and the loaded
// images' debug symbols, and the machine's lifetime. Like
// internal/coreos's export.go, the exported names the debugger needs and
// that have no other reason to be public are gathered here, so the
// boundary is visible in one place. (The engine, CPU, memory, output
// stream, and expression evaluator are exported fields and methods of
// Console already.)

// RequireInit reports the error a command gets when the machine hasn't
// been created yet (before INIT).
func (c *Console) RequireInit() error { return c.requireInit() }

// LocationText is pc as STEP's "Stepped to" and a breakpoint's "Break at"
// show it: a symbolic name where the program has one, else the address
// in 8 hex digits.
func (c *Console) LocationText(pc uint32) string { return c.locationText(pc) }

// TraceStep prints the instruction about to execute at pc when tracing is
// on (SET TRACE), or when force is true, and returns a function to call
// once the instruction has run, to print what it changed. See traceStep.
func (c *Console) TraceStep(pc uint32, force bool) func() { return c.traceStep(pc, force) }

// ExceptionName is the name of an exception (fault, trap, or interrupt)
// vector's code, as the console words it.
func (c *Console) ExceptionName(code cpu.Exception) string { return exceptionName(code) }

// EvalWhole evaluates text, which must be one whole expression: anything
// left over after the expression is an error. It is how a command's
// address or value parameter becomes a number.
func (c *Console) EvalWhole(text string) (uint32, error) {
	return c.EvalWholeIn(c.Radix, text)
}

// EvalWholeIn is EvalWhole with the radix a number without a prefix is read
// in: the debugger has a radix of its own (SET RADIX).
func (c *Console) EvalWholeIn(radix int, text string) (uint32, error) {
	return c.EvalWholeMode(radix, text, false)
}

// EvalWholeMode is EvalWholeIn with the evaluator's Value mode chosen: true
// gives an expression's value, as EVALUATE shows it (a data label is its
// contents), and false its location, as EXAMINE and DEPOSIT take it (a
// data label is its address).
func (c *Console) EvalWholeMode(radix int, text string, value bool) (uint32, error) {
	ev := c.Evaluator()
	ev.Radix = radix
	ev.Value = value

	v, rest, err := ev.Eval(text)
	if err != nil {
		return 0, debuggerSymbolError(err)
	}

	if extra := strings.TrimSpace(rest); extra != "" {
		return 0, vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, extra)
	}

	return v, nil
}

// debuggerSymbolError rewords an "undefined symbol" error of the
// expression evaluator the way the VMS debugger says it. The console's
// words are %CLI-E-UNDEFSYM, Undefined symbol "X"; the debugger's are
// %DEBUG-E-NOSYMBOL, symbol 'X' is not in the symbol table (the probe's
// sessions show it for a name, a path name, and a %LINE alike). Any other
// error is returned as it is.
func debuggerSymbolError(err error) error {
	var ve vmserrors.VMSError

	if errors.As(err, &ve) && ve.Status == vmserrors.CLI_UNDEFSYM && len(ve.Arguments) > 0 {
		return vmserrors.New(vmserrors.DBG_NOSYMBOL, ve.Arguments[0])
	}

	return err
}

// ParseCall evaluates the parameters of a CALL command: the routine's
// address, and the arguments to pass it. In CALL's syntax the argument
// list "(a,b,...)" follows the routine either directly ("F(1,2)", part of
// the routine expression, which the evaluator leaves unconsumed in its
// remainder) or after a blank ("F (1,2)", which the grammar delivers as a
// separate parameter, arguments).
func (c *Console) ParseCall(routine, arguments string) (addr uint32, args []uint32, err error) {
	addr, list, err := c.Evaluator().Eval(routine)
	if err != nil {
		return 0, nil, err
	}

	list = strings.TrimSpace(list + " " + arguments)
	if list == "" {
		return addr, nil, nil
	}

	if !strings.HasPrefix(list, "(") {
		return 0, nil, vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, list)
	}

	args, list, err = c.callArguments(list[1:])
	if err != nil {
		return 0, nil, err
	}

	// Nothing may follow the argument list.
	if extra := strings.TrimSpace(list); extra != "" {
		return 0, nil, vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, extra)
	}

	return addr, args, nil
}

// callArguments evaluates a CALL argument list, s being what follows its
// "(": expressions separated by commas, up to the closing ")". It returns
// the values and what follows the ")".
func (c *Console) callArguments(s string) ([]uint32, string, error) {
	var args []uint32

	ev := c.Evaluator()

	for {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, ")") {
			return args, s[1:], nil
		}

		if s == "" {
			return nil, "", vmserrors.New(vmserrors.CLI_INCOMPLETEARGS)
		}

		if len(args) > 0 {
			if !strings.HasPrefix(s, ",") {
				return nil, "", vmserrors.New(vmserrors.CLI_NEEDCOMMA)
			}

			s = s[1:]
		}

		v, rest, err := ev.Eval(s)
		if err != nil {
			return nil, "", err
		}

		args = append(args, v)
		s = rest
	}
}

// ImageActive reports whether an image started by RUN is running, from its
// start until it exits (or the machine is reset).
func (c *Console) ImageActive() bool { return c.imageActive }

// StatusText is a status value's message line, such as "%SYSTEM-S-NORMAL,
// normal successful completion".
func (c *Console) StatusText(status uint32) string {
	if c.RTL == nil {
		return ""
	}

	return c.RTL.StatusText(status)
}

// RoutineEntry reports whether addr is the entry point of a routine in a
// loaded image's debug symbol table, one that starts with a two-byte entry
// mask (a CALLS/CALLG routine, as opposed to a JSB subroutine). The
// debugger sets a breakpoint "at routine X" *after* that mask: the mask is
// data (which registers the routine saves), not an instruction, so a stop
// there would be somewhere no instruction starts.
func (c *Console) RoutineEntry(addr uint32) bool {
	prog := c.debugImageAt(addr)
	if prog == nil {
		return false
	}

	r, _, ok := prog.RoutineAt(addr)

	return ok && r.Address == addr && !r.NoCall
}

// RoutineExtent returns the first address and the size in bytes of the
// routine in a loaded image's debug symbol table that holds addr. ok is
// false where addr is in no routine the debugger knows. SET BREAK/RETURN
// uses it to know which RET instructions belong to a routine.
func (c *Console) RoutineExtent(addr uint32) (start, size uint32, ok bool) {
	prog := c.debugImageAt(addr)
	if prog == nil {
		return 0, 0, false
	}

	r, _, found := prog.RoutineAt(addr)
	if !found {
		return 0, 0, false
	}

	return r.Address, r.Size, true
}

// LineStart reports whether pc is the first byte of a source line's code in
// a loaded image's line-number table. SET BREAK/LINE stops at each such
// address: the first instruction of every line the program runs.
func (c *Console) LineStart(pc uint32) bool {
	prog := c.debugImageAt(pc)
	if prog == nil {
		return false
	}

	line, _, ok := prog.LineAt(pc)

	return ok && line.Address == pc
}

// DebugProgramAt returns the debug symbol table of the loaded image that
// holds addr, or nil when addr is in no image or its image has none. The
// debugger names data and registers' scopes from it.
func (c *Console) DebugProgramAt(addr uint32) *dbgsym.Program { return c.debugImageAt(addr) }

// DebugPrograms returns the debug symbol table of each loaded image that
// has one, main image first.
func (c *Console) DebugPrograms() []*dbgsym.Program {
	var out []*dbgsym.Program

	for _, icb := range c.ICBList {
		if icb.Debug != nil {
			out = append(out, icb.Debug)
		}
	}

	return out
}

// ReadBytes reads n bytes of the machine's memory starting at virtual
// address addr, as the console's EXAMINE does: through kernel-mode
// translation, whatever mode the program last ran in. A page that can't
// be read is the debugger's %DEBUG-E-NOACCESSR for the first address that
// failed.
func (c *Console) ReadBytes(addr, n uint32) ([]byte, error) {
	if err := c.requireInit(); err != nil {
		return nil, err
	}

	out := make([]byte, n)

	for i := range out {
		b, err := c.loadSized(addr+uint32(i), SizeByte)
		if err != nil {
			return nil, vmserrors.New(vmserrors.DBG_NOACCESSR, addr+uint32(i))
		}

		out[i] = byte(b)
	}

	return out, nil
}

// WriteBytes stores data into the machine's memory at virtual address
// addr, as DEPOSIT does (through kernel-mode translation, a byte at a
// time). It doesn't move the console's current deposit address.
func (c *Console) WriteBytes(addr uint32, data []byte) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	return c.withKernelMode(func() error {
		for i, b := range data {
			if err := c.Mem.StoreByte(c.CPU, addr+uint32(i), b); err != nil {
				return err
			}
		}

		return nil
	})
}

// FormatPTE is a page table entry as EXAMINE/PTE shows it: the entry's
// value and its fields (valid, protection, modified, owner, and the
// physical address of its page frame).
func (c *Console) FormatPTE(v uint32) string { return c.formatOne(0, SizePTE, v) }

// SetAccessMode switches the CPU to an access mode (kernel, executive,
// supervisor, or user), which also selects that mode's stack pointer; with
// interruptStack it switches to the interrupt stack instead, whatever the
// mode. It is SET MODE's access-mode half, which the debugger took over.
func (c *Console) SetAccessMode(mode vax.AccessMode, interruptStack bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Engine.SetModeStack(mode, interruptStack)

	return nil
}

// AccessModeName is the name of the CPU's current access mode (KERNEL,
// EXEC, SUPER, or USER), or "" before INIT has created the machine.
func (c *Console) AccessModeName() string {
	if c.requireInit() != nil {
		return ""
	}

	return modeNames[c.CPU.PSL().CurMod()]
}

// ImageInfo describes one loaded image for the debugger's SHOW IMAGE.
type ImageInfo struct {
	// Name is the image's file name without directory, type, or version,
	// in upper case (DBGDIS for DUA0:[DIR]DBGDIS.EXE;1), as SHOW IMAGE
	// lists it.
	Name string

	// Base and End are the first and last addresses the image occupies.
	Base, End uint32

	// Main is true for the image RUN started, which SHOW IMAGE marks with
	// an asterisk.
	Main bool

	// Program is the image's debug symbol table, nil for an image linked
	// without one. An image whose symbols the debugger can use is one
	// that has one.
	Program *dbgsym.Program
}

// Images returns the loaded images, main image first, as the debugger's
// SHOW IMAGE lists them.
func (c *Console) Images() []ImageInfo {
	var out []ImageInfo

	for _, icb := range c.ICBList {
		// The image's first address is its lowest section's: the load
		// offset alone would include the unused page 0 below it.
		base := icb.Base

		first := true

		for _, isd := range icb.ISDList {
			// The user stack and global sections aren't part of the
			// image's own pages (imageLoad skips them too).
			if isdType(isd.Flags) == isdUsrStack || isd.Flags&isdGBL != 0 || isd.Pages == 0 {
				continue
			}

			if addr := icb.Base + uint32(isd.VPN)<<9; first || addr < base {
				base, first = addr, false
			}
		}

		out = append(out, ImageInfo{
			Name:    imageDisplayName(icb.File),
			Base:    base,
			End:     icb.End,
			Main:    icb.Flags&icbMain != 0,
			Program: icb.Debug,
		})
	}

	return out
}

// imageDisplayName reduces an image file specification to its bare name:
// the part after the last "]", ":", or "/", up to the first ".", in upper
// case.
func imageDisplayName(file string) string {
	if at := strings.LastIndexAny(file, "]:/"); at >= 0 {
		file = file[at+1:]
	}

	name, _, _ := strings.Cut(file, ".")

	return strings.ToUpper(name)
}

// ScopeFrames returns the routine of each call frame, innermost first, as
// "MODULE\ROUTINE" (just "MODULE" for a routine named as its module, or a
// frame in no routine). It is what SHOW SCOPE lists: the same frames as
// SHOW CALLS. ok is false when the PC isn't in an image with debug
// symbols.
func (c *Console) ScopeFrames() (paths []string, ok bool, err error) {
	rows, ok, err := c.debugFrames(0)

	for _, row := range rows {
		path := row.module

		if row.hasRoutine && !strings.EqualFold(row.routine, row.module) {
			path += `\` + row.routine
		}

		paths = append(paths, path)
	}

	return paths, ok, err
}

// LanguageName is the name of the language a module's DST says it was
// written in, as the debugger shows it (MACRO).
func LanguageName(code uint32) string { return languageName(code) }

// HasSymbol reports whether the console's own symbol table (not the
// program's debug symbols) has a symbol matching name, which may contain
// the VMS wildcards.
func (c *Console) HasSymbol(name string) bool {
	if lnm.HasWildcards(name) {
		for _, s := range c.Symbols.All() {
			if lnm.Match(strings.ToUpper(name), strings.ToUpper(s.Name)) {
				return true
			}
		}

		return false
	}

	_, ok := c.Symbols.Find(name)

	return ok
}

// EntryMaskText describes the word at addr as a routine's entry mask, as
// the debugger's EXAMINE of a routine shows it ("entry mask ^M<R2,R3,R4>"),
// when addr is the start of a routine that is called (CALLS or CALLG), and
// so begins with one. A VAX routine starts with a 16-bit mask of the
// registers it saves; ok is false for any other address.
func (c *Console) EntryMaskText(addr uint32) (text string, ok bool) {
	name, isEntry := c.entryAt(addr)
	if !isEntry {
		return "", false
	}

	dec := disasm.EntryMask(memByteReader{c: c}, addr, name)

	return dec.Format(disasm.Options{Style: disasm.StyleDebugger}), true
}
