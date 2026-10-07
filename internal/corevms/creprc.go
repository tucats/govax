package corevms

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// $CREPRC, creating a process (docs/PHASE-45.md, subtask 4).
//
// A VMS program starts another process with the Create Process service:
//
//	SYS$CREPRC [pidadr] ,[image] ,[input] ,[output] ,[error] ,[prvadr]
//	           ,[quota] ,[prcnam] ,[baspri] ,[uic] ,[mbxunt] ,[stsflg]
//	           ,[itemlst] ,[node]
//
// The new process runs the image named by image, with SYS$INPUT,
// SYS$OUTPUT, and SYS$ERROR translating to input, output, and error.
// Without uic it is a *subprocess*: it belongs to its creator's job, has
// its creator's UIC and user name, counts against the job's subprocess
// quota (PRCLM), and is deleted with its creator. With a uic (or the
// PRC$M_DETACH flag) it is a *detached* process, the master process of a
// job of its own (job.go).
//
// $CREPRC itself only checks its arguments and builds the process: its
// slot in the process table, its address space, stacks, and hardware
// PCB, its quotas, privileges, and priority. It returns as soon as the
// process exists, computable. Everything else happens in the new process
// the first time the scheduler runs it (Environment.Startup): defining
// its SYS$ names, finding the image, and activating it. That is VMS's
// order too, which is why an image that doesn't exist isn't $CREPRC's
// error: the new process starts, fails to activate it, and is deleted,
// its termination message giving the status.
//
// Everything here follows the VMS 5.0 System Services Reference Manual's
// $CREPRC entry, except where a comment says otherwise. Two later
// arguments, itemlst and node (an item list of process attributes, and
// another cluster node to create the process on), aren't in that manual;
// govax accepts and ignores them.
//
// Like every other part of multiprocessing, $CREPRC works only with the
// vax.process.scheduler setting on (docs/PHASE-43.md, Part A, rule 2);
// with it off, it returns SS$_UNSUPPORTED.

// Status codes $CREPRC returns, besides the ones other services share.
var (
	ssUnsupported = vmsdef.Symbols["SS$_UNSUPPORTED"]
	ssIvQuotaL    = vmsdef.Symbols["SS$_IVQUOTAL"]
)

// maxEquivalenceLength is the longest image, input, output, or error
// string: longer is SS$_IVLOGNAM. (The manual's description of image
// says 63 characters, but its list of statuses says 255 for all four;
// govax takes the list's.)
const maxEquivalenceLength = 255

// The status flags (stsflg, the PRC$M_ bits) $CREPRC looks at.
var (
	prcSSRWAIT = vmsdef.Symbols["PRC$M_SSRWAIT"]
	prcPSWAPM  = vmsdef.Symbols["PRC$M_PSWAPM"]
	prcNOACNT  = vmsdef.Symbols["PRC$M_NOACNT"]
	prcBATCH   = vmsdef.Symbols["PRC$M_BATCH"]
	prcNETWRK  = vmsdef.Symbols["PRC$M_NETWRK"]
	prcDETACH  = vmsdef.Symbols["PRC$M_DETACH"]
)

// validCreateFlags are the stsflg bits $PRCDEF defines: bits 0 through
// PRC$V_TCB, VMS 7.3's highest. Any other bit is reserved, and setting
// one is SS$_IVSTSFLG. (The 5.0 manual stops at bit 10; the later bits
// are VMS 7.3's definitions, which govax accepts.)
var validCreateFlags = uint32(1)<<(vmsdef.Symbols["PRC$V_TCB"]+1) - 1

// A new process's privileged stacks, in pages: VMINIT's defaults for
// process 1's (docs/MODE-STACKS.md). Its page tables are its creator's
// size (createSpace).
const (
	newKernelStackPages = 4
	newModeStackPages   = 8
)

// ProcessStartup is what a created process still has to do before it
// runs its image, as $CREPRC asked: the image to activate, and the
// equivalence strings of its SYS$INPUT, SYS$OUTPUT, and SYS$ERROR
// logical names (empty when not given).
type ProcessStartup struct {
	Image, Input, Output, Error string
}

// QuotaItem is one entry of $CREPRC's quota list: a PQL$_ code and the
// value asked for.
type QuotaItem struct {
	Code, Value uint32
}

// CreateRequest is a $CREPRC call's arguments, read from the caller's
// memory: what CreateProcess needs to create the process.
type CreateRequest struct {
	// Image, Input, Output, and Error are the image and the SYS$ names'
	// equivalence strings (ProcessStartup).
	Image, Input, Output, Error string

	// Name is the process name, 1 to 15 characters; empty for none.
	Name string

	// Privileges is the privilege mask asked for, if HasPrivileges.
	// Without it the process gets its creator's current privileges.
	Privileges    uint64
	HasPrivileges bool

	// Quotas are the quota list's entries, in order.
	Quotas []QuotaItem

	// BasePriority is baspri; UIC is uic (0: a subprocess, unless Flags
	// has PRC$M_DETACH); TerminationMailbox is mbxunt, a mailbox unit
	// number (0: none); Flags is stsflg.
	BasePriority, UIC, TerminationMailbox, Flags uint32
}

// serviceSysCreprc is SYS$CREPRC. It reads the arguments (SS$_ACCVIO for
// a string, descriptor, privilege mask, or quota list it can't read;
// SS$_IVLOGNAM for a process name of 0 or more than 15 characters, or a
// string longer than 255; SS$_IVQUOTAL for a quota list with a code
// $PQLDEF doesn't define), checks that it can write the PID at pidadr
// (SS$_ACCVIO), and leaves the rest to CreateProcess. On success the new
// process's PID is written to pidadr.
func serviceSysCreprc(env *Environment, argv []uint32) (uint32, error) {
	if !env.ProcessSettings.Scheduler {
		return ssUnsupported, nil
	}

	req := CreateRequest{
		BasePriority:       optArg(argv, 8),
		UIC:                optArg(argv, 9),
		TerminationMailbox: optArg(argv, 10) & 0xFFFF, // a word, by value
		Flags:              optArg(argv, 11),
	}

	// The four strings: an omitted one is empty.
	for i, s := range []*string{&req.Image, &req.Input, &req.Output, &req.Error} {
		addr := optArg(argv, 1+i)
		if addr == 0 {
			continue
		}

		text, ok, err := strGet(env, addr, maxEquivalenceLength)

		switch {
		case err != nil:
			return ssAccVio, nil
		case !ok:
			return ssIvLogNam, nil
		}

		*s = text
	}

	if prcnam := optArg(argv, 7); prcnam != 0 {
		name, ok, err := strGet(env, prcnam, maxProcessNameLength)

		switch {
		case err != nil:
			return ssAccVio, nil
		case !ok || name == "":
			return ssIvLogNam, nil
		}

		req.Name = name
	}

	if prvadr := optArg(argv, 5); prvadr != 0 {
		mask, err := env.mem.LoadQuadword(env.cpu, prvadr)
		if err != nil {
			return ssAccVio, nil
		}

		req.Privileges, req.HasPrivileges = mask, true
	}

	if quota := optArg(argv, 6); quota != 0 {
		items, st := env.readQuotaList(quota)
		if st != 0 {
			return st, nil
		}

		req.Quotas = items
	}

	pidadr := optArg(argv, 0)
	if pidadr != 0 && !env.canWriteLongword(pidadr) {
		return ssAccVio, nil
	}

	child, st := env.CreateProcess(req)
	if st != ssNormal {
		return st, nil
	}

	if pidadr != 0 {
		// canWriteLongword said this works; if it fails after all, the
		// process exists anyway, as VMS's would.
		_ = env.mem.StoreLongword(env.cpu, pidadr, child.Process.PID)
	}

	return ssNormal, nil
}

// maxQuotaItems bounds how many entries readQuotaList reads, so a list
// with no PQL$_LISTEND in memory full of valid codes still ends: more
// than this is SS$_IVQUOTAL. A real list names each of the 14 quotas
// at most a few times.
const maxQuotaItems = 1024

// readQuotaList reads the quota list at addr: entries of a one-byte
// PQL$_ code followed by a longword value, ended by a PQL$_LISTEND byte
// (0). SS$_ACCVIO if it can't be read, SS$_IVQUOTAL for a code $PQLDEF
// doesn't define.
func (env *Environment) readQuotaList(addr uint32) ([]QuotaItem, uint32) {
	var items []QuotaItem

	for range maxQuotaItems {
		code, err := env.mem.LoadByte(env.cpu, addr)
		if err != nil {
			return nil, ssAccVio
		}

		if uint32(code) == pqlListEnd {
			return items, 0
		}

		if _, ok := quotasByCode[uint32(code)]; !ok {
			return nil, ssIvQuotaL
		}

		value, err := env.mem.LoadLongword(env.cpu, addr+1)
		if err != nil {
			return nil, ssAccVio
		}

		items = append(items, QuotaItem{Code: uint32(code), Value: value})
		addr += 5
	}

	return nil, ssIvQuotaL
}

// canWriteLongword reports whether the caller can write the longword at
// addr. It reads the longword and writes the same value back, so memory
// doesn't change: VMS probes an output argument before it does anything
// it would have to undo.
func (env *Environment) canWriteLongword(addr uint32) bool {
	v, err := env.mem.LoadLongword(env.cpu, addr)
	if err != nil {
		return false
	}

	return env.mem.StoreLongword(env.cpu, addr, v) == nil
}

// CreateProcess creates the process req describes, as env's process
// asked: $CREPRC's checks and creation, without reading the caller's
// memory. It returns the new process and SS$_NORMAL, or nil and the
// status of the first check that failed, with nothing created:
//
//   - SS$_UNSUPPORTED: the vax.process.scheduler setting is off.
//   - SS$_IVSTSFLG: Flags has a reserved bit set.
//   - SS$_IVLOGNAM: Name is longer than 15 characters, or a string
//     longer than 255.
//   - SS$_IVQUOTAL: a quota code $PQLDEF doesn't define.
//   - SS$_NOPRIV: the creator lacks a privilege the request needs
//     (checkCreatePrivileges).
//   - SS$_DUPLNAM: another process in the new process's UIC group has
//     Name.
//   - SS$_EXQUOTA: the job already has as many subprocesses as its
//     PRCLM quota allows, or the CPU time a subprocess takes out of its
//     creator's limit would leave the creator none (resolveQuotas).
//   - SS$_NOSLOT: the process table is full, or there is no room in the
//     S0 pool for the process's page tables, stacks, and PCB (or no pool
//     at all: VMINIT hasn't run). The manual's SS$_INSFMEM is for
//     dynamic memory, which Go provides.
//
// The new process is computable, with its Startup pending. It has its
// own address space (page tables the size of its creator's, with the P1
// vector mapped), its own privileged stacks and hardware PCB (whose PC
// the startup sets), the quotas resolveQuotas works out, the privileges
// asked for (limited to its creator's unless the creator holds SETPRV),
// and the base priority asked for (no higher than its creator's unless
// the creator holds ALTPRI). It shares its creator's terminal.
func (env *Environment) CreateProcess(req CreateRequest) (*Environment, uint32) {
	if !env.ProcessSettings.Scheduler {
		return nil, ssUnsupported
	}

	if req.Flags&^validCreateFlags != 0 {
		return nil, ssIvStsFlg
	}

	if len(req.Name) > maxProcessNameLength {
		return nil, ssIvLogNam
	}

	for _, s := range []string{req.Image, req.Input, req.Output, req.Error} {
		if len(s) > maxEquivalenceLength {
			return nil, ssIvLogNam
		}
	}

	for _, item := range req.Quotas {
		if _, ok := quotasByCode[item.Code]; !ok {
			return nil, ssIvQuotaL
		}
	}

	creator := env.Process
	detached := req.UIC != 0 || req.Flags&prcDETACH != 0

	uic := creator.UIC
	if req.UIC != 0 {
		uic = req.UIC
	}

	if st := creator.checkCreatePrivileges(req, uic, detached); st != 0 {
		return nil, st
	}

	// A subprocess's name is qualified by its creator's group, a
	// detached process's by the group of its own UIC: either way, the
	// new process's group.
	if _, taken := env.FindProcessName(uic>>16, req.Name); taken {
		return nil, ssDuplNam
	}

	// With DETACH, a detached process's quotas are what it asks for;
	// anything else is held to its creator's.
	independent := detached && creator.hasPrivilege(privDETACH)

	quotas, cpuDeducted, st := resolveQuotas(creator, req.Quotas, detached, independent)
	if st != 0 {
		return nil, st
	}

	// The process takes its slot (and, as a subprocess, its place in
	// the job), then gets its memory; if that fails, it's taken out
	// again.
	var (
		child *Environment
		err   error
	)

	if detached {
		child, err = env.newDetachedProcess()
	} else {
		child, err = NewSubprocess(env, env.consoleIn, env.consoleOut)
	}

	if err != nil {
		var ve vmserrors.VMSError
		if errors.As(err, &ve) {
			return nil, ve.Status
		}

		return nil, ssNoSlot
	}

	if !env.buildProcessMemory(child) {
		env.RemoveProcess(child)

		return nil, ssNoSlot
	}

	// A detached process's job gets its logical-name table now that the
	// process is sure to exist (a table, once made, is never deleted).
	if detached {
		child.Logicals = env.Logicals.NewProcessView(uic, env.Logicals.NewJobTable())
	}

	p := child.Process
	p.UIC = uic
	p.Name = req.Name
	p.CreateFlags = req.Flags
	p.TerminationMailbox = req.TerminationMailbox
	p.ResourceWaitDisabled = req.Flags&prcSSRWAIT != 0

	applyQuotas(p, quotas, detached)
	creator.CPULimit -= cpuDeducted
	p.cpuDeducted = cpuDeducted

	// Privileges: those asked for (or, if none were, the creator's
	// current ones), but only the creator's own unless it may give any
	// (SETPRV). They are the new process's permanent and current
	// privileges; it is authorized for them and for whatever its creator
	// is authorized for. (Unconfirmed: what VMS gives an omitted prvadr,
	// and the new process's authorized mask.)
	privileges := creator.CurrentPrivileges
	if req.HasPrivileges {
		privileges = req.Privileges
	}

	if !creator.hasPrivilege(privSETPRV) {
		privileges &= creator.CurrentPrivileges
	}

	privileges &= allPrivileges
	p.AuthorizedPrivileges = creator.AuthorizedPrivileges | privileges
	p.ProcessPrivileges, p.CurrentPrivileges = privileges, privileges

	// The base priority: baspri's low five bits, as $SETPRI takes them,
	// no higher than the creator's own without ALTPRI. (An omitted
	// baspri is 0, the lowest.)
	pri := req.BasePriority & maxPriority
	if !creator.hasPrivilege(privALTPRI) {
		pri = min(pri, creator.BasePriority)
	}

	p.BasePriority, p.Priority, p.AuthorizedPriority = pri, pri, creator.AuthorizedPriority

	if err := env.sched.SetBasePriority(handle(child), int(pri)); err == nil {
		env.requestReschedule()
	}

	child.Startup = &ProcessStartup{Image: req.Image, Input: req.Input, Output: req.Output, Error: req.Error}

	return child, ssNormal
}

// checkCreatePrivileges returns SS$_NOPRIV if p may not create the
// process req asks for, with UIC uic, detached or not; 0 if it may. The
// manual's rules:
//
//   - DETACH or CMKRNL to create a detached process with a UIC other
//     than p's own, a batch process (PRC$M_BATCH), or a network process
//     (PRC$M_NETWRK);
//   - NETMBX too, for a network process;
//   - PSWAPM to disable swapping (PRC$M_PSWAPM), and NOACNT to disable
//     accounting (PRC$M_NOACNT).
//
// The rules for privileges and priority beyond p's own (SETPRV, ALTPRI)
// don't refuse: what's asked for is reduced instead.
func (p *Process) checkCreatePrivileges(req CreateRequest, uic uint32, detached bool) uint32 {
	detachOrKernel := p.hasAnyPrivilege(privDETACH | privCMKRNL)

	switch {
	case detached && uic != p.UIC && !detachOrKernel,
		req.Flags&(prcBATCH|prcNETWRK) != 0 && !detachOrKernel,
		req.Flags&prcNETWRK != 0 && !p.hasPrivilege(privNETMBX),
		req.Flags&prcPSWAPM != 0 && !p.hasPrivilege(privPSWAPM),
		req.Flags&prcNOACNT != 0 && !p.hasPrivilege(privNOACNT):
		return ssNoPriv
	}

	return 0
}

// newDetachedProcess adds a new detached process to the process table:
// the master process of a new job, with no owner, and env's user name
// and account (it has no logical names yet; CreateProcess gives it its
// view once its memory is built). SS$_NOSLOT if the table is full.
func (env *Environment) newDetachedProcess() (*Environment, error) {
	child := newEnvironment(env.System, nil, env.consoleIn, env.consoleOut)
	if err := env.addProcess(child); err != nil {
		return nil, err
	}

	p := child.Process
	p.Job = newJob(p.PID)
	p.Username, p.Account = env.Process.Username, env.Process.Account

	return child, nil
}

// buildProcessMemory gives child its address space, privileged stacks,
// and hardware PCB, from the S0 pool, and writes the PCB a first
// dispatch loads: the stacks' and address space's registers, user mode,
// and PC 0 for now (the process's startup decides where the image
// starts). Its page tables are the size of env's (on VMS, every
// process's are sized by one system parameter; govax's stand-in is
// process 1's, from VMINIT). It reports whether it could; if it couldn't,
// whatever it allocated is the process's (charged to its PID), for
// RemoveProcess to free.
func (env *Environment) buildProcessMemory(child *Environment) bool {
	if env.s0 == nil || env.Space == nil {
		return false
	}

	pid := child.Process.PID

	space, err := env.BuildAddressSpace(pid, env.Space.P0Pages, env.Space.P1Pages)
	if err != nil {
		return false
	}

	child.Space = space

	stacks, err := env.BuildStacks(pid, newKernelStackPages, newModeStackPages, newModeStackPages)
	if err != nil {
		return false
	}

	child.Stacks = stacks

	var psl vax.PSL

	psl.SetCurMod(vax.User)
	psl.SetPrvMod(vax.User)

	start := InitialPCB(space, stacks, 0, psl)

	return cpu.WritePCB(env.mem, stacks.PCBB, &start) == nil
}

// startProcess runs the startup of env, a process $CREPRC created, the
// first time the scheduler switches to it. Process startup is
// docs/PHASE-45.md's subtask 5; until it's written, a created process
// can't run, and switching to one stops the run with this error.
func (sys *System) startProcess(env *Environment) error {
	return fmt.Errorf("corevms: process %08X can't start: process startup isn't written yet", env.Process.PID)
}
