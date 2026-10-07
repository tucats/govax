package console

import (
	"fmt"

	"github.com/tucats/govax/internal/corevms"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// RunDebug is RUN's /[NO]DEBUG choice: whether the image runs under the
// debugger.
type RunDebug int

const (
	// DebugDefault: no qualifier. The debugger starts if the image was
	// linked /DEBUG (its header's IHD$V_LNKDEBUG), as VMS starts it.
	DebugDefault RunDebug = iota

	// DebugOn: /DEBUG (also /STEP and /BREAK, govax's older spellings).
	// The debugger starts if the image has a debug symbol table, which
	// traceback alone provides.
	DebugOn

	// DebugOff: /NODEBUG. The image runs without the debugger.
	DebugOff
)

// RunOptions holds RUN's qualifiers: /[NO]INIT, /[NO]DEBUG (/STEP,
// /BREAK), /NOEXECUTE, and the file's /HOST. console_run read at most one,
// leading qualifier; the DCL grammar (docs/PHASE-37.md) reads any
// combination (commands.go's runOptions).
type RunOptions struct {
	RunInits  bool     // /INIT (run each dependency's LIB$INITIALIZE); /NOINIT is the same as the zero value
	Debug     RunDebug // /[NO]DEBUG: run under the debugger (see RunDebug)
	NoExecute bool // /NOEXECUTE: load and fix up, but don't transfer control
	// Host is an explicit /HOST after the file name: the image is a host
	// file whatever its name looks like (see readMainImage).
	Host bool

	// CommandLine is the text the image reads with LIB$GET_FOREIGN: a
	// foreign command's parameters (dclsym.go), or the text after the
	// file name on govax's own command line (RunCommandLine). RUN itself
	// takes no parameters, so it's empty for RUN.
	CommandLine string
}

// Run implements the RUN <filename> command: loads fn and its sharable-
// image dependencies (imageLoad), fixes them up (imageFixup), and
// transfers control to the main image's own entry point -- optionally
// calling each dependency's LIB$INITIALIZE entry point first -- via a small
// synthesized IMAGE$INIT driver procedure run through Console.Call.
func (c *Console) Run(fn string, opts RunOptions) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	savedMode := c.CPU.PSL().CurMod()
	c.Engine.SetModeStack(vax.Kernel, false)
	defer c.Engine.SetModeStack(savedMode, false) // safety net on an early-error return

	c.resetICBList()

	if err := c.ensureShims(); err != nil {
		return err
	}

	c.runHost = opts.Host
	c.RTL.CommandLine = opts.CommandLine

	main, err := c.activateImage(fn)
	if err != nil {
		return err
	}

	// Restores the caller's mode here -- before building and
	// running IMAGE$INIT, not after -- so the loaded program executes in
	// whatever mode RUN itself was invoked from.
	c.Engine.SetModeStack(savedMode, false)

	entry, _ := mainTransferAddress(main)

	driverAddr, ok, err := c.buildImageInitDriver(main, opts.RunInits)
	if err != nil {
		return err
	}

	if !ok {
		c.Printf("No transfer address!\n")

		return nil
	}

	if opts.NoExecute {
		return nil
	}

	c.imageActive = true

	if !c.runsUnderDebugger(main, opts.Debug) {
		return c.Call(driverAddr, false)
	}

	if c.Debugger == nil {
		return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
	}

	// The debugger stops the program at the main routine's first
	// instruction, which follows its two-byte entry mask.
	stopAt := entry + 2

	a := Activation{
		Kind:     ActivateImage,
		Addr:     &driverAddr,
		StopAt:   &stopAt,
		Language: "MACRO",
	}

	if m, ok := main.Debug.ModuleAt(entry); ok {
		a.Module = m.Name
		a.Language = languageName(m.Language)
	}

	return c.Debugger.Start(a)
}

// runsUnderDebugger decides whether RUN starts the debugger on the image
// main. An image with no debug symbol table (linked /NOTRACEBACK) runs
// without it, however RUN was asked, as the VMS debugger does for such an
// image (docs/PHASE-42.md, subtask 1's probe). Otherwise /DEBUG starts it,
// /NODEBUG doesn't, and with neither it starts for an image linked
// /DEBUG.
func (c *Console) runsUnderDebugger(main *ICB, choice RunDebug) bool {
	if main.Debug == nil {
		return false
	}

	switch choice {
	case DebugOn:
		return true
	case DebugOff:
		return false
	default:
		return main.LinkDebug && c.Debugger != nil
	}
}

// languageName is the name the debugger shows for a module's source
// language, from the language code in the debug symbol table's module
// record. Only MACRO (code 0) is confirmed, by every image the Phase 41
// and 42 probes ran; govax's tools write no other kind, so any other code
// is shown as UNKNOWN until a probe of a compiler's output settles it.
func languageName(code uint32) string {
	if code == 0 {
		return "MACRO"
	}

	return "UNKNOWN"
}

// activateImage loads fn and its sharable-image dependencies (imageLoad),
// fixes each up (imageFixup), and then gives every section's pages its
// protection (setImageProtection) -- only once every fixup is written,
// as VMS's image activator likewise changes a section's protection after
// fixing it up (the IAF's change-protection list). Returns the main
// image's ICB. Needs ensureShims to have run. The image goes into p's
// process, through its address space, whichever process is current.
func (p *imageProcess) activateImage(fn string) (*ICB, error) {
	c := p.c

	main, err := p.imageLoad(fn, icbMain)
	if err != nil {
		return nil, vmserrors.Wrap(vmserrors.CLI_ACTIVATE, err, fn)
	}

	if c.CPU.DebugEnabled(vax.DebugImages) {
		c.Printf("Main image is %s\n", main.Name)
	}

	for _, dep := range p.ICBList {
		if err := p.imageFixup(dep); err != nil {
			return nil, vmserrors.Wrap(vmserrors.CLI_FIXUP, err, dep.Name)
		}
	}

	for _, icb := range p.ICBList {
		p.setImageProtection(icb, true)
	}

	return main, nil
}

// Process 1's image activation. RUN, the debugger, and the console's
// commands that show images (SHOW IMAGES, the symbol lookups) all work
// on process 1, the console's own; these name its imageProcess's methods.

func (c *Console) activateImage(fn string) (*ICB, error) { return c.images().activateImage(fn) }

func (c *Console) imageLoad(fn string, flag uint32) (*ICB, error) {
	return c.images().imageLoad(fn, flag)
}

func (c *Console) imageFixup(icb *ICB) error { return c.images().imageFixup(icb) }

func (c *Console) resetICBList() { c.images().resetICBList() }

func (c *Console) findMainICB() *ICB { return c.images().findMainICB() }

func (c *Console) buildImageInitDriver(main *ICB, runInits bool) (uint32, bool, error) {
	return c.images().buildImageInitDriver(main, runInits)
}

// DefaultRunInits reports RUN's own default for whether to invoke each
// dependency's LIB$INITIALIZE before any /INIT or /NOINIT qualifier
// overrides it, DebugLibinit defaults on (vax.DebugDefault), so
// LIB$INITIALIZE runs by default, not only when /INIT is given explicitly.
// See commands.go's runOptions, which applies the qualifiers.
func (c *Console) DefaultRunInits() bool {
	return c.CPU != nil && c.CPU.DebugEnabled(vax.DebugLibinit)
}

// mainTransferAddress selects icb's own user-mode entry point, matching
// console_run's own transfer[0]-then-[1]-then-[2] fallback: the first
// transfer-vector entry that looks like a plausible P0/P1 user-space
// address (nonzero, below 0x3FFFFFFF) -- transfer[0] is often a debugger/
// LIB$INITIALIZE-style entry point outside that range, hence the fallback.
func mainTransferAddress(icb *ICB) (uint32, bool) {
	for _, addr := range icb.Transfer[:3] {
		if addr > 0 && addr < 0x3FFFFFFF {
			return addr, true
		}
	}

	return 0, false
}

// buildImageInitDriver writes a small procedure in p's driver page (for
// process 1, at CONSOLE$SCRATCH+8; see imageProcess.driver) (an
// empty entry mask, then one PUSHL/PUSHL/PUSHL/CALLS sequence per
// dependency's LIB$INITIALIZE entry point if runInits, then a final CALLS
// to main's own entry point, a call to $EXIT with main's status when the
// P1 vector is present, and a RET) and returns its address -- the Go
// equivalent of console_run's own assemble_direct-built IMAGE$INIT
// procedure. ok is false if main has no usable transfer address, matching
// console_run's own "No transfer address!" case: no driver is written at
// all, since a procedure with LIB$INITIALIZE calls but no final RET would
// have nothing safe to fall through to. p.ICBList[1:] is main's dependency
// list in load order (id-0 self-references live only in each ICB's own
// SHRList, never in ICBList itself, so no filtering is needed here).
func (p *imageProcess) buildImageInitDriver(main *ICB, runInits bool) (uint32, bool, error) {
	return p.buildDriver(main, runInits, false)
}

// buildDriver is buildImageInitDriver, with a call to $HIBER first when
// hibernate is set: for a process $CREPRC created with PRC$M_HIBER, which
// waits to be woken before its image runs (activateCreatedImage).
func (p *imageProcess) buildDriver(main *ICB, runInits, hibernate bool) (uint32, bool, error) {
	c := p.c

	addr, ok := mainTransferAddress(main)
	if !ok {
		return 0, false, nil
	}

	driverAddr, err := p.driverAddress()
	if err != nil {
		return 0, false, err
	}

	code := []byte{0x00, 0x00} // entry mask: no registers saved

	if hibernate {
		hiber, ok := p.p1Stub("SYS$HIBER")
		if !ok {
			return 0, false, fmt.Errorf("console: no SYS$HIBER stub for a process created to hibernate")
		}

		code = append(code, encodeCalls(0, hiber)...)
	}

	if runInits {
		for _, dep := range p.ICBList[1:] {
			if dep.Transfer[0] == 0 {
				continue
			}

			initAddr, ok := p.symbols[fmt.Sprintf("SHARE$%s_INITIALIZE", dep.Name)]
			if !ok {
				continue
			}

			if c.CPU.DebugEnabled(vax.DebugImages) {
				c.Printf("Preparing call to LIB$INITIALIZE entry %08X for image %s\n", initAddr, dep.Name)
			}
			// LIBRTL's LIB$INITIALIZE apparently wants 100 as a special
			// flag argument.
			flag := uint32(0)
			if dep.Name == "LIBRTL" {
				flag = 100
			}

			code = append(code, encodePushl(0)...)
			code = append(code, encodePushl(0)...)
			code = append(code, encodePushl(flag)...)
			code = append(code, encodeCalls(3, initAddr)...)
		}
	}

	code = append(code, encodeCalls(0, addr)...)

	// VMS's image activator calls $EXIT with whatever main returned, so
	// the image's exit handlers run (docs/PHASE-26.md subtask 19). $EXIT
	// doesn't return: it ends the image by returning from this driver's
	// own console call frame. It needs the P1 vector's SYS$EXIT stub in
	// memory; without one, the driver just returns main's status.
	if exit, ok := p.p1Stub("SYS$EXIT"); ok {
		code = append(code, 0xDD, 0x50) // PUSHL R0
		code = append(code, encodeCalls(1, exit)...)
	}

	code = append(code, 0x04) // RET

	if err := p.storeBytes(driverAddr, code); err != nil {
		return 0, false, err
	}

	return driverAddr, true, nil
}

// driverAddress is where p's IMAGE$INIT driver goes. Process 1's is
// CONSOLE$SCRATCH+8, where it has always been. Another process's is a
// pool page of its own, allocated the first time and freed with the
// process; S0, so kernel mode alone may write it, and every mode may read
// (and so run) it.
func (p *imageProcess) driverAddress() (uint32, error) {
	if p.env == nil {
		base, found := p.c.Symbols.Get("CONSOLE$SCRATCH")
		if !found {
			return 0, vmserrors.New(vmserrors.CLI_NOSCRATCH)
		}

		return base + 8, nil
	}

	if p.driver == 0 {
		page, err := p.env.AllocateS0(1, p.env.Process.PID, "IMAGE$INIT driver")
		if err != nil {
			return 0, err
		}

		p.driver = page
	}

	return p.driver, nil
}

// p1Stub returns the address of the P1-vector entry for service name,
// if its stub is in memory: a procedure entry mask, then the XFC
// instruction (opcode 0xFC, selector 0x7A, XFC$P1VECTOR) that calls the
// service. The stubs are there once .P1VECTOR has been assembled, as
// kernel.asm does. It looks in p's P1, where every process maps the
// vector.
func (p *imageProcess) p1Stub(name string) (uint32, bool) {
	for _, e := range vmsdef.P1VectorTable {
		if e.Name != name {
			continue
		}

		w, err := p.loadWord(e.Addr + 2)

		return e.Addr, err == nil && w == 0x7AFC
	}

	return 0, false
}

// encodePushl returns PUSHL #v's raw opcode bytes (0xDD, general-operand
// immediate-longword mode).
func encodePushl(v uint32) []byte {
	return []byte{0xDD, 0x8F, byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

// encodeCalls returns CALLS #argc,@#target's raw opcode bytes (0xFB, an
// immediate-longword argument count, and an absolute (@#) target address).
func encodeCalls(argc, target uint32) []byte {
	return []byte{
		0xFB,
		0x8F, byte(argc), byte(argc >> 8), byte(argc >> 16), byte(argc >> 24),
		0x9F, byte(target), byte(target >> 8), byte(target >> 16), byte(target >> 24),
	}
}

// activateCreatedImage is the System's ActivateImage hook (newRTL): the
// startup of a process $CREPRC created (docs/PHASE-45.md, subtask 5)
// activates the process's image into its own P0, as RUN activates one
// into process 1's, and builds its IMAGE$INIT driver, running
// LIB$INITIALIZE as RUN does by default. It returns the driver's address.
//
// The image is found as RUN finds a main image, by the console's default
// directory (readImage), which is the new process's own when, as always
// so far, process 1 created it. The name is never taken as a host file
// by /HOST: a $CREPRC image name is a file specification.
func (c *Console) activateCreatedImage(env *corevms.Environment, image string, hibernate bool) (uint32, error) {
	if err := c.ensureShims(); err != nil {
		return 0, err
	}

	savedHost := c.runHost
	c.runHost = false

	defer func() { c.runHost = savedHost }()

	p := c.imagesOf(env)
	p.resetICBList()

	main, err := p.activateImage(image)
	if err != nil {
		return 0, err
	}

	driver, ok, err := p.buildDriver(main, c.DefaultRunInits(), hibernate)
	if err != nil {
		return 0, err
	}

	if !ok {
		return 0, fmt.Errorf("console: image %s has no transfer address", image)
	}

	return driver, nil
}
