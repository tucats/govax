package machine

import (
	"fmt"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"

	"github.com/tucats/govax/internal/registers"
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
	NoExecute bool     // /NOEXECUTE: load and fix up, but don't transfer control
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
func (m *Machine) Run(fn string, opts RunOptions) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	// A previous image that was stopped and never exited ends now.
	m.EndAbandonedImage()

	savedMode := m.CPU.PSL().CurMod()
	m.Engine.SetModeStack(registers.Kernel, false)
	defer m.Engine.SetModeStack(savedMode, false) // safety net on an early-error return

	m.ResetICBList()

	if err := m.EnsureShims(); err != nil {
		return err
	}

	m.runHost = opts.Host
	m.processOne.CommandLine = opts.CommandLine

	main, err := m.ActivateImage(fn)
	if err != nil {
		return err
	}

	// Restores the caller's mode here -- before building and
	// running IMAGE$INIT, not after -- so the loaded program executes in
	// whatever mode RUN itself was invoked from.
	m.Engine.SetModeStack(savedMode, false)

	entry, _ := mainTransferAddress(main)

	driverAddr, ok, err := m.BuildImageInitDriver(main, opts.RunInits)
	if err != nil {
		return err
	}

	// An image with no transfer address (a shareable image) doesn't run:
	// DCL's NOTFR. What its activation made goes.
	if !ok {
		m.ResetICBList()

		return noTransfer(main, fn)
	}

	if opts.NoExecute {
		return nil
	}

	m.imageActive = true

	if !m.runsUnderDebugger(main, opts.Debug) {
		m.imageEnded = false
		if m.processOne != nil {
			m.processOne.Process.ExitStatus = ssNormal
		}

		if err := m.Call(driverAddr, false); err != nil {
			return err
		}

		return m.imageCompletion()
	}

	if m.Debugger == nil {
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

	return m.Debugger.Start(a)
}

// runsUnderDebugger decides whether RUN starts the debugger on the image
// main. An image with no debug symbol table (linked /NOTRACEBACK) runs
// without it, however RUN was asked, as the VMS debugger does for such an
// image (docs/PHASE-42.md, subtask 1's probe). Otherwise /DEBUG starts it,
// /NODEBUG doesn't, and with neither it starts for an image linked
// /DEBUG.
func (m *Machine) runsUnderDebugger(main *ICB, choice RunDebug) bool {
	if main.Debug == nil {
		return false
	}

	switch choice {
	case DebugOn:
		return true
	case DebugOff:
		return false
	default:
		return main.LinkDebug && m.Debugger != nil
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

// ActivateImage loads fn and its sharable-image dependencies (imageLoad),
// fixes each up (imageFixup), and then gives every section's pages its
// protection (setImageProtection) -- only once every fixup is written,
// as VMS's image activator likewise changes a section's protection after
// fixing it up (the IAF's change-protection list). Returns the main
// image's ICB. Needs ensureShims to have run. The image goes into p's
// process, through its address space, whichever process is current. An
// activation that fails takes down what it made (ResetICBList), so a
// mapped image file it opened isn't left held (a DISMOUNT after
// testdata/link68's R3 found one).
func (p *ImageProcess) ActivateImage(fn string) (*ICB, error) {
	c := p.m

	main, err := p.imageLoad(fn, icbMain)
	if err != nil {
		p.ResetICBList()

		return nil, vmserrors.Wrap(vmserrors.CLI_ACTIVATE, err, fn)
	}

	if c.CPU.DebugEnabled(registers.DebugImages) {
		c.Printf("Main image is %s\n", main.Name)
	}

	for _, dep := range p.ICBList {
		if err := p.imageFixup(dep); err != nil {
			p.ResetICBList()

			return nil, vmserrors.Wrap(vmserrors.CLI_FIXUP, err, dep.Name)
		}
	}

	for _, icb := range p.ICBList {
		p.setImageProtection(icb, true)
	}

	if env := p.process(); env != nil {
		env.SetImageName(main.Spec.Name, main.Spec.Host)
	}

	return main, nil
}

// Process 1's image activation. RUN, the debugger, and the console's
// commands that show images (SHOW IMAGES, the symbol lookups) all work
// on process 1, the console's own; these name its ImageProcess's methods.

func (m *Machine) ActivateImage(fn string) (*ICB, error) { return m.images().ActivateImage(fn) }

func (m *Machine) imageLoad(fn string, flag uint32) (*ICB, error) {
	return m.images().imageLoad(fn, flag)
}

func (m *Machine) imageFixup(icb *ICB) error { return m.images().imageFixup(icb) }

func (m *Machine) ResetICBList() { m.images().ResetICBList() }

func (m *Machine) FindMainICB() *ICB { return m.images().FindMainICB() }

func (m *Machine) BuildImageInitDriver(main *ICB, runInits bool) (uint32, bool, error) {
	return m.images().BuildImageInitDriver(main, runInits)
}

// DefaultRunInits reports RUN's own default for whether to invoke each
// dependency's LIB$INITIALIZE before any /INIT or /NOINIT qualifier
// overrides it, DebugLibinit defaults on (registers.DebugDefault), so
// LIB$INITIALIZE runs by default, not only when /INIT is given explicitly.
// See commands.go's runOptions, which applies the qualifiers.
func (m *Machine) DefaultRunInits() bool {
	return m.CPU != nil && m.CPU.DebugEnabled(registers.DebugLibinit)
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

// BuildImageInitDriver writes a small procedure in p's driver page (for
// process 1, at CONSOLE$SCRATCH+8; see ImageProcess.driver) (an
// empty entry mask, then one PUSHL/PUSHL/PUSHL/CALLS sequence per
// dependency's LIB$INITIALIZE entry point if runInits, then a final CALLS
// to main's own entry point, a call to $EXIT with main's status when the
// P1 vector is present, and a RET) and returns its address -- the Go
// equivalent of console_run's own assemble_direct-built IMAGE$INIT
// procedure. ok is false if main has no usable transfer address (RUN's
// ErrNoTransfer): no driver is written at
// all, since a procedure with LIB$INITIALIZE calls but no final RET would
// have nothing safe to fall through to. p.ICBList[1:] is main's dependency
// list in load order (id-0 self-references live only in each ICB's own
// SHRList, never in ICBList itself, so no filtering is needed here).
func (p *ImageProcess) BuildImageInitDriver(main *ICB, runInits bool) (uint32, bool, error) {
	return p.buildDriver(main, runInits, false)
}

// buildDriver is BuildImageInitDriver, with a call to $HIBER first when
// hibernate is set: for a process $CREPRC created with PRC$M_HIBER, which
// waits to be woken before its image runs (activateCreatedImage).
func (p *ImageProcess) buildDriver(main *ICB, runInits, hibernate bool) (uint32, bool, error) {
	c := p.m

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

		code = append(code, EncodeCalls(0, hiber)...)
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

			if c.CPU.DebugEnabled(registers.DebugImages) {
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
			code = append(code, EncodeCalls(3, initAddr)...)
		}
	}

	code = append(code, EncodeCalls(0, addr)...)

	// VMS's image activator calls $EXIT with whatever main returned, so
	// the image's exit handlers run (docs/PHASE-26.md subtask 19). $EXIT
	// doesn't return: it ends the image by returning from this driver's
	// own console call frame. It needs the P1 vector's SYS$EXIT stub in
	// memory; without one, the driver just returns main's status.
	if exit, ok := p.p1Stub("SYS$EXIT"); ok {
		code = append(code, 0xDD, 0x50) // PUSHL R0
		code = append(code, EncodeCalls(1, exit)...)
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
func (p *ImageProcess) driverAddress() (uint32, error) {
	if p.env == nil {
		base, found := p.m.Symbols.Get("CONSOLE$SCRATCH")
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
func (p *ImageProcess) p1Stub(name string) (uint32, bool) {
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

// EncodeCalls returns CALLS #argc,@#target's raw opcode bytes (0xFB, an
// immediate-longword argument count, and an absolute (@#) target address).
func EncodeCalls(argc, target uint32) []byte {
	return []byte{
		0xFB,
		0x8F, byte(argc), byte(argc >> 8), byte(argc >> 16), byte(argc >> 24),
		0x9F, byte(target), byte(target >> 8), byte(target >> 16), byte(target >> 24),
	}
}

// ActivateCreatedImage is the System's ActivateImage hook (newRTL): the
// startup of a process $CREPRC created (docs/PHASE-45.md, subtask 5)
// activates the process's image into its own P0, as RUN activates one
// into process 1's, and builds its IMAGE$INIT driver, running
// LIB$INITIALIZE as RUN does by default. It returns the driver's address.
//
// The image is found as RUN finds a main image, by the console's default
// directory (ReadImage), which is the new process's own when, as always
// so far, process 1 created it. The name is never taken as a host file
// by /HOST: a $CREPRC image name is a file specification.
func (m *Machine) ActivateCreatedImage(env *corevms.Environment, image string, hibernate bool) (uint32, error) {
	if err := m.EnsureShims(); err != nil {
		return 0, err
	}

	savedHost := m.runHost
	m.runHost = false

	defer func() { m.runHost = savedHost }()

	p := m.ImageState(env)
	p.ResetICBList()

	main, err := p.ActivateImage(image)
	if err != nil {
		return 0, err
	}

	driver, ok, err := p.buildDriver(main, m.DefaultRunInits(), hibernate)
	if err != nil {
		return 0, err
	}

	if !ok {
		p.ResetICBList()

		return 0, noTransfer(main, image)
	}

	return driver, nil
}

// imageCompletion is what RUN reports when its image has run to its end
// ($EXIT): the image's status is the command's, as DCL makes it $STATUS,
// and a failure is returned, its message shown unless the image's status
// says it has been (STS$M_INHIB_MSG, as an unhandled condition's has).
// When the run stopped some other way (a HALT, a limit, CTRL/C), nothing
// is reported.
func (m *Machine) imageCompletion() error {
	if !m.imageEnded {
		return nil
	}

	m.imageEnded = false

	return m.Hooks.CommandStatus(m.imageStatus)
}

// ImageRundown does what VMS does at process 1's image exit. For logical names
// (User's Manual §11.3.5, §11.4) it deletes the user-mode names in the
// process table, such as those DEFINE/USER_MODE made for the image that
// just ended. The RTL's own per-image state (the files the image left
// open, user-mode device allocations, docs/PHASE-26.md) is cleaned up by
// Environment.ImageRundown.
func (m *Machine) ImageRundown() {
	if n, err := m.processOneLogicals.Delete(lnm.ProcessTableName, "", lnm.User); err == nil && n > 0 {
		m.traceLogicals("image rundown deleted %d user-mode name(s)", n)
	}

	if m.processOne != nil {
		// A debugger session keeps the image's memory to examine after
		// its exit; the next image's activation deletes it (ResetICBList).
		m.processOne.KeepP0 = m.InDebugger()
		m.processOne.ImageRundown()
		m.processOne.KeepP0 = false
	}
}

// EndAbandonedImage runs down an image RUN started that never
// exited: one a HALT, CTRL/C, CTRL/Y, or an instruction or time limit
// stopped, and that nothing resumed. On VMS such an image is run down
// when the next one starts (or the process logs out), closing the files
// it left open; RUN and EndSession do the same here.
func (m *Machine) EndAbandonedImage() {
	if m.imageActive {
		m.imageActive = false
		m.ImageRundown()
	}
}

// traceLogicals is the logical-name database's Trace hook: it writes
// each line to the CPU's debug writer while SET DEBUG LOGICALS is on.
func (m *Machine) traceLogicals(format string, args ...any) {
	if m.CPU != nil && m.CPU.DebugEnabled(registers.DebugLogicals) {
		fmt.Fprintf(m.CPU.DebugWriter(), "DEBUG: LNM: "+format+"\n", args...)
	}
}
