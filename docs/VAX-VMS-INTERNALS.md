# Inside govax's VAX and VMS

## 0. About this document

govax is a VAX computer and enough of the VAX/VMS operating system to run
real VMS programs, written in Go. This document explains how it works
underneath: how a VAX instruction is decoded and executed, how a virtual
address becomes a physical one, how VMS's paging, processes, I/O, and
files are made, and how a program running on the emulated VAX reaches
VMS's services, which govax writes in Go rather than in VAX code.

It's meant to be read in order, by someone who may know little of the VAX
or of VMS. Section 1 is a primer on both, enough for the rest. Section 2
lays out govax's packages, and section 3 the handful of structures at the
center of the machine. The sections after that take one subject each,
from the bottom up.

What it leaves out is the user's side of the system: DCL's commands,
command procedures, the FORTRAN compiler, MACRO, LINK, LIBRARY, BACKUP,
and the other utilities. Those are the user's manual's (`docs/MANUAL.md`),
and the compiler has its own tour (`docs/FORTRAN-INTERNALS.md`). This
document is about what a user-mode program stands on: the VAX it runs on
and the VMS it calls.

govax was built in phases, and each phase has a document in `docs/`
(`PHASE-nn - topic.md`) recording what it set out to do, what it found,
what was decided, and a dated log of the work; `docs/PLAN.md` indexes
them and summarizes the larger programs. Those are the history: where
this document says how something *is*, they say how it came to be and
what else was considered. Nothing here depends on them.

A few conventions. Numbers written `^X1F` or `0x1F` are hexadecimal, as
VMS and Go write them. A *longword* is 32 bits, a *word* 16, a *quadword*
64, and an *octaword* 128, the VAX's names. Source files are named by
their path under `internal/`, so `cpu/engine.go` is
`internal/cpu/engine.go`. Code excerpts are cut down from the source;
the file they come from has the whole of it, with comments that often go
further than this document does.

## 1. A VAX and VMS in brief

### 1.1 The VAX

The VAX ("Virtual Address eXtension") is the 32-bit computer DIGITAL
made from 1977, and VMS the operating system DIGITAL wrote for it. The
VAX's architecture is famously regular and famously rich: some 300
instructions, nearly all of whose operands can use any of a dozen
addressing modes.

**Registers.** There are sixteen 32-bit general registers, R0 to R15.
The last four have jobs: R12 is AP, the *argument pointer*, which
addresses a procedure's argument list; R13 is FP, the *frame pointer*,
which addresses its call frame; R14 is SP, the *stack pointer*; and R15
is PC, the *program counter*. Because PC is an ordinary register, the
addressing modes that work on registers work on it too, which is how
the VAX gets immediate operands and position-independent code.

Beside them are the *privileged registers*, reached only with the MTPR
and MFPR instructions (move to and from processor register) in kernel
mode: the stack pointer of each access mode, the page table registers,
the base of the system control block, and others, each with a number.

**The PSL.** The *processor status longword* is the CPU's state word. Its
low 16 bits are the PSW, which programs may change; its high 16 bits are
privileged:

```text
 31 30  29-28 27  26  25-24  23-22  21  20-16  15-8   7   6   5   4  3 2 1 0
 CM TP  MBZ  FPD  IS  CUR    PRV    MBZ  IPL   MBZ   DV  FU  IV  T  N Z V C
```

N, Z, V, and C are the *condition codes* (negative, zero, overflow,
carry), set by most instructions and tested by the conditional branches.
T is the trace bit, and DV, FU, and IV enable traps for decimal
overflow, floating underflow, and integer overflow. IPL is the
*interrupt priority level*, 0 to 31: an interrupt is taken only if its
level is above the CPU's. IS says the CPU is running on the interrupt
stack. CUR and PRV are the current and previous *access modes*.

**Access modes.** The VAX has four levels of privilege, from most to
least: kernel (0), executive (1), supervisor (2), and user (3). VMS runs
its innermost code in kernel mode, RMS (the file system's record layer)
in executive, the command language interpreter (DCL) in supervisor, and
programs in user mode. Each mode has its own stack, and memory
protection is by mode: a page can be readable by user mode and writable
only by kernel mode, say. A program moves inward only through the
*change mode* instructions (CHMK, CHME, CHMS, CHMU), which trap to a
handler the operating system chose, and moves outward with REI (return
from exception or interrupt), which refuses to move inward.

**The address space.** A virtual address is 32 bits, and its top two bits
choose one of four regions of a gigabyte each:

| Bits 31–30 | Region | Addresses | Use |
| --- | --- | --- | --- |
| 00 | P0 | `00000000`–`3FFFFFFF` | the *program region*: a process's program and data, growing up |
| 01 | P1 | `40000000`–`7FFFFFFF` | the *control region*: a process's stacks and VMS's per-process data, growing down |
| 10 | S0 | `80000000`–`BFFFFFFF` | the *system region*: the operating system, the same in every process |
| 11 | S1 | `C0000000`–`FFFFFFFF` | reserved; no VAX ever used it |

P0 and P1 together are the *process space*, private to each process; S0
is shared by all. Memory is divided into *pages* of 512 bytes, and each
region has a *page table*, an array of one *page table entry* (PTE) per
virtual page, saying whether the page is in memory (the valid bit), which
physical page holds it (the *page frame number*, PFN), and who may read
and write it (a four-bit protection code). The page table registers say
where each table is and how long: SBR and SLR for S0 (the system table
is addressed physically), P0BR and P0LR, P1BR and P1LR for the process
regions (whose tables are themselves in S0, addressed virtually). An
access to a page whose PTE isn't valid takes a *translation not valid*
fault, which is how the operating system pages: the fault's handler
brings the page in and lets the instruction run again. Mapping is on
only when the MAPEN register is 1; before that, addresses are physical.

**Instructions.** An instruction is an opcode of one byte (or two, for
the few with an `FD` prefix) followed by its *operand specifiers*, one
per operand, each a byte saying the addressing mode and register, and
sometimes a displacement or a constant after it:

```text
    ADDL3   #5, 8(R2), R3      ; R3 <- 5 + the longword at R2+8

    C1          ADDL3
    05          short literal 5 (mode 0)
    A2 08       byte displacement, R2, displacement 8 (mode A)
    53          register R3 (mode 5)
```

The instruction's operand types come from the opcode (ADDL3 has three
longword operands, read, read, and written), and the specifier says only
where each operand is. That separation is why one table can describe
the whole instruction set, as section 4 shows.

**Exceptions and interrupts.** When an instruction can't finish (a
memory fault, a reserved operand, a privileged instruction in user mode)
or an arithmetic condition must be reported, the CPU takes an
*exception*: it pushes the PSL and PC on the kernel (or interrupt)
stack, sometimes with parameters, and jumps to a handler whose address
it reads from the *system control block* (SCB), a page of vectors one
per kind of event, which the SCBB register locates. A *fault* backs PC up
so the instruction runs again once the cause is fixed; a *trap* lets PC
point past it. Interrupts from devices and clocks arrive the same way,
between instructions, when their IPL is above the CPU's.

**Procedure calls.** The VAX has a calling standard that its
instructions enforce. CALLS (or CALLG) takes an argument list and a
procedure's address; the procedure begins with a 16-bit *entry mask*
naming the registers it uses, which CALLS saves on the stack with the
caller's AP, FP, PC, and PSW, building a *call frame*; RET undoes it.
AP points at the argument list, a longword count followed by the
arguments, each usually an address. JSB and RSB are the light-weight
call, saving only PC.

```text
    PUSHAL  BUFFER          ; argument 2: the address of BUFFER
    PUSHL   #10             ; argument 1: the value 10
    CALLS   #2, G^SUB       ; call SUB with 2 arguments
    ...
    .ENTRY  SUB, ^M<R2,R3>  ; SUB saves R2 and R3
    MOVL    4(AP), R2       ; the first argument
    MOVL    8(AP), R3       ; the second
    RET
```

### 1.2 VMS

**Processes and images.** VMS runs *processes*, each with its own P0 and
P1 space, its own stacks, quotas, privileges, logical names, and open
files. A process runs one *image* at a time: a program, read from an
`.EXE` file by the *image activator*, which maps its sections into P0,
links it to the *shareable images* it uses (the run-time libraries,
LIBRTL.EXE and the rest, which are themselves images), and calls its
transfer address. When the image ends, *image rundown* takes it away,
and the process goes on, usually to its command interpreter, DCL, which
lives in the process's P1 space and reads the next command.

**System services.** A program asks VMS for anything through *system
services*: `SYS$QIO` (queue an I/O request), `SYS$ASSIGN` (get a
channel to a device), `SYS$CRMPSC` (map a section), `SYS$EXIT`, and some
two hundred more. Each is reached by CALLS at a fixed address in the
*system service vector*, a page near the top of P1 that every process
has at the same address, so that a program linked on one version of VMS
runs on the next. A service returns a *status* in R0: a longword whose
low bit is set for success, whose low three bits are a *severity*
(warning, success, error, information, severe), and whose upper bits
name the facility and the message (`SS$_NORMAL` is 1, `SS$_ACCVIO` 12).

**Asynchronous events.** A service that starts something that finishes
later (an I/O, a timer) reports its end by an *event flag*, a bit the
program can wait on, by an *I/O status block* it fills in, or by an
*AST*, an asynchronous system trap: a call of a routine the program
named, made between two of its instructions whenever the event happens.

**Conditions.** VMS's errors and exceptions are *conditions*: a hardware
exception, or a status a program *signals* with LIB$SIGNAL, goes to the
*condition handlers* the program's procedures established in their call
frames, searched from the innermost frame out. A handler can continue the
program, *unwind* the stack to an outer frame, or *resignal* to the next.
If none does, the last chance is a handler VMS put there, which prints
the message (`%SYSTEM-F-ACCVIO, access violation, ...`) and, for an
error or severe condition, ends the image.

**Files.** VMS's disks hold the *ODS-2* file structure (*On-Disk
Structure* level 2, also called Files-11): a volume of 512-byte blocks
with an index file of file headers, a bitmap of free clusters, and
directories that are themselves files. Programs reach files through
*RMS*, the Record Management Services: a set of system services
(`SYS$OPEN`, `SYS$GET`, `SYS$PUT`, ...) that take control blocks in the
program's memory (the FAB, a file access block, and the RAB, a record
access block, with NAM and XAB blocks for names and attributes) and
read and write *records*: lines of text, fixed-length records, the
records of relative and indexed files. A file is named
`DUA1:[WORK.SUB]FILE.TYP;3`: a device, a directory path, a name, a type,
and a version.

**Logical names.** A *logical name* is a name defined to stand for
another, looked up when a file is named: `SYS$SYSTEM` stands for the
directory of system files, `SYS$OUTPUT` for wherever the process's
output goes. They live in tables (the process's, its job's, its
group's, the system's), searched in order, so a process can redirect
a name for itself alone.

That's the whole of what the rest of this document assumes.

## 2. The shape of govax

govax is a Go program, `cmd/govax`, and its work is in some forty
packages under `internal/`. Seen from the bottom, they make four layers:
the VAX itself; the machine as an operator sees it, with its console;
VMS's executive and file system, written in Go; and VMS's run-time
libraries, also in Go. Above those sit DCL and the user's tools, which
this document leaves out.

```text
   cmd/govax ............... the host program: terminal, options, startup
       |
   console ................. the front end: routes each line to DCL, the
       |                     debugger, or the interactive assembler
   dclshell, dclcmd, dcl ... DCL and its commands (not in this document)
       |
   machine ................. the machine as the operator sees it: INIT,
       |                     INITIALIZE/MEMORY, RUN, the image activator,
       |                     the run loop
   librtl, mthrtl, lbrrtl .. the run-time libraries, in Go (shims)
   corevms ................. VMS's executive in Go: services, processes,
       |                     paging, I/O, conditions
   rms ..................... RMS, on ...
   ods2 (separate module) .. ODS-2 volumes in container files
       |
   cpu ..................... the instruction engine
   vm ...................... virtual memory
   registers ............... the CPU's registers and PSL
```

The rule between the layers is that a package imports only those below
it. The CPU knows nothing of VMS: when VAX code asks for a VMS service,
the CPU calls through an interface (`cpu.SystemServices`) that a higher
layer implements. Section 3 shows how those connections are made, and
section 10 how a call crosses them.

### 2.1 The VAX

| Package | What it is |
| --- | --- |
| `registers` | The CPU's state: the sixteen general registers, the privileged registers, and the PSL with its fields (`registers.go`, `psl.go`). Plain data, no behavior beyond reading and writing it. |
| `vm` | Physical memory and virtual memory: RAM as a byte slice, the ROM and NVRAM, page table entries (`pte.go`), translation (`translate.go`), the translation buffer (`tb.go`), the instruction-fetch window (`fetch.go`), another process's address space (`space.go`), the hook through which the operating system resolves page faults (`pager.go`), and the allocator of physical frames (`frames.go`). |
| `cpu` | The instruction engine (`Engine`): the instruction table (`instructions_table.go`, generated, with a handler for each opcode registered by the file that implements it), decoding (`decode.go`, `operand.go`), the step loop (`engine.go`), exceptions (`exception.go`, `handlefault.go`), interrupts and the clocks (`interrupt.go`, `clock.go`), ASTs (`ast.go`), the trace trap (`trace.go`), process context (`context.go`), the scheduling hook (`schedule.go`), and XFC, the opcode through which VAX code reaches Go (`xfc.go`). |
| `vaxfloat` | The VAX's four floating formats (F, D, G, H), with exact arithmetic that rounds as the hardware does. |
| `disasm` | The disassembler, which reads instructions through `cpu`'s table. |
| `io` | The device table: each device's name, class, type, and characteristics, as `$GETDVI` reports them. |

### 2.2 VMS's executive and file system

| Package | What it is |
| --- | --- |
| `corevms` | VMS in Go. A `System` is what every process shares (the service and shim tables, devices, mounts, mailboxes, global sections, the lock database, the scheduler, the page file), and an `Environment` is one process. Here are the system services (a file or two each: `qio.go`, `creprc.go`, `gblsec.go`, `enq.go`, ...), condition dispatch (`condition.go`), ASTs, event flags, timers, the process table and scheduler glue (`proctable.go`, `schedule.go`), the device drivers (`ttdriver.go`, `mbxdriver.go`, `nulldriver.go`, `diskdriver.go`), and VMS paging (`workingset.go`, `mpw.go`, `pagefile.go`, `globalpages.go`, ...). |
| `sched` | VMS's scheduling rules, as plain Go: process states, priorities and boosts, the queues, the choice of the next process. |
| `paging` | The page frame number database (one entry per physical page, with the free and modified lists), working set lists, and the page file's block map. |
| `lck` | The lock manager: resources and locks, the six lock modes, conversions, deadlock detection. |
| `lnm` | The logical name database: tables, directories, access modes, translation. |
| `rms` | RMS: the FAB, RAB, NAM, and XAB in VAX memory, name processing, sharing and record locks, and sequential, relative, and indexed files, over ODS-2 volumes (and host files). The only package that imports `ods2`. |
| `isam` | The structure of indexed files on disk: buckets, keys, splits. |
| `ods2` | A separate Go module, `github.com/tucats/ods2`: ODS-2 volumes in host files ("containers"), with real index files, bitmaps, directories, and file headers. |
| `vmsdef` | VMS's own constants and tables: the symbols (`SS$_`, `RMS$_`, `IO$_`, ...), message texts, control block layouts, the system service vector's addresses, the run-time libraries' transfer vectors. |
| `vmsimage` | Reads an image file's header and sections. |
| `dbgsym` | Reads an image's debug symbol table (used by traceback and the debugger). |

### 2.3 The run-time libraries

VMS's run-time libraries are shareable images of VAX code. govax writes
most of them in Go instead: a program linked against LIBRTL.EXE calls a
routine through LIBRTL's transfer vector as it would on VMS, and finds
there a twelve-byte stub that hands the call to Go (section 10).

| Package | What it is |
| --- | --- |
| `librtl` | LIBRTL.EXE's LIB$ and STR$ routines, and the C library routines a program may call. |
| `mthrtl` | MTHRTL.EXE, the mathematics library. |
| `lbrrtl` | LBRSHR.EXE, the librarian's routines. |
| `vmheap` | The heap's mechanisms beneath LIB$GET_VM and its zones. |
| `ftnrtl` | FTNRTL.EXE, the FORTRAN run-time library: unlike the others, real VAX code, written in MACRO-32, assembled and linked by govax, and carried inside it. |

### 2.4 The machine and the console

| Package | What it is |
| --- | --- |
| `machine` | The machine as its operator sees it: the engine, CPU, and memory together, INIT and INITIALIZE/MEMORY (which lays out the address space; the code still calls it VMINIT, its old name), the ROM, the shim stubs, the image activator (`activate.go`, `image.go`, `imagemap.go`), RUN, the run loop, the console's symbol table and expression evaluator, the interactive assembler's session. |
| `console` | The front end: a `Console` holds a `machine.Machine` and DCL's interpreter, and routes each line typed to DCL, the debugger, or the assembler. |
| `debugger` | The machine debugger, modeled on VMS's. |
| `asm` | The assembler, both the console's ASM (which assembles the microkernel into memory) and MACRO-32. |
| `bootdata` | Files built into govax: the microkernel's source (`files/kernel.asm`), the default ROM, STARTUP.COM, the grammars, govax's system macros, the help libraries. |
| `sysvol` | What the system volume holds, and how it's made and refreshed. |

The other packages (`dcl`, `dclshell`, `dclcmd`, `fortran`, `link`,
`obj`, `lbr`, `anl`, `backup`, `fdl`, `help`, `symtab`, and the rest)
are DCL, the tools, and their file formats.

### 2.5 The seams

Where a lower package needs something only a higher one can give, it
declares an interface, and the higher one implements it. These are the
joints of the whole system, and most of this document is about what
happens at one of them:

| Interface | Declared in | Implemented by | What crosses it |
| --- | --- | --- | --- |
| `cpu.SystemServices` | `cpu/services.go` | `machine.Machine` (with `corevms.EngineHooks`) | XFC: the console's I/O, system services, shims |
| `cpu.ASTSource` | `cpu/ast.go` | the same | an AST to deliver between instructions |
| `cpu.ExceptionDispatcher` | `cpu/handlefault.go` | the same | an exception for a program's condition handlers |
| `cpu.Scheduler` | `cpu/schedule.go` | `corevms.System` | the end of a process's time slice |
| `cpu.AttentionHandler` | `cpu/attention.go` | the machine | a Ctrl/C or Ctrl/Y typed at the terminal |
| `vm.Pager` | `vm/pager.go` | `corevms.System` | a fault on a page that isn't valid |
| `vm.FrameAllocator` | `vm/frames.go` | a bitmap, or `paging`'s database | a physical page wanted or given back |
| `corevms.CommandInterpreter` | `corevms/cliprocess.go` | DCL (`dclshell`) | a process that runs a command interpreter |

## 3. The central structures

Six structures hold nearly everything, and their relation is worth
learning before anything else:

```text
 console.Console
   ├── *machine.Machine ──────────────┐
   │     ├── CPU    *registers.CPU    │ the hardware
   │     ├── Mem    *vm.Memory        │
   │     ├── Engine *cpu.Engine ──────┘ (holds CPU, Mem, and the table;
   │     │                               its hooks point back up at the
   │     │                               Machine and the System)
   │     └── EngineHooks ─────────────> the running corevms.Environment
   └── *dclshell.Interpreter (DCL)

 corevms.System  (one per INIT or VMINIT; what all processes share)
   ├── mem, cpu  (the same Memory and CPU)
   ├── services, shims  (the tables XFC dispatches through)
   ├── Devices, Mounts, Mailboxes, Sections, Locks, EventFlagClusters
   ├── sched, the process table, the page file, the PFN database's users
   └── Environments, one per process:
         corevms.Environment
           ├── *System (embedded)
           ├── Process   (PID, name, quotas, privileges, event flags, ...)
           ├── Space     (its P0 and P1 page tables)
           ├── Stacks    (its kernel, executive, supervisor stacks, its PCB)
           ├── Logicals  (its view of the logical name database)
           ├── Session   (its RMS session: default directory, mounts)
           └── files, channels, output, ...
```

### 3.1 `registers.CPU`: the state

`registers.CPU` (`registers/registers.go`) is the processor's state and
nothing else:

```go
type CPU struct {
    gpr [MaxReg + 1]uint32     // R0..R15
    pr  [MaxPrivReg + 1]uint32 // the privileged registers, by number
    psl PSL
    ...
}
```

Its methods read and write registers (`GPR`, `SetGPR`, `PR`, `SetPR`)
and the PSL (`PSL`, `SetPSL`), and `PSL` has methods for each field
(`CurMod`, `IPL`, `WithConditionCodes`, ...). The privileged registers
are named by their architectural numbers (`KSP` is 0, `PCBB` 16, `SCBB`
17, `ASTLVL` 19, `MAPEN` 56), so that MTPR and MFPR need no table. It
also keeps the *operand log*, the registers an instruction's operand
specifiers changed (an autoincrement's, say), so that a fault can put
them back before the instruction runs again (section 4).

### 3.2 `vm.Memory`: physical memory and translation

`vm.Memory` (`vm/memory.go`) owns the machine's RAM, a Go byte slice in
which `ram[n]` is physical address `n`, and the ROM and NVRAM beside it.
Everything that reads or writes emulated memory goes through its
methods: the decoder, the instruction handlers, the console's EXAMINE
and DEPOSIT, and every Go system service that reads an argument.

Its methods take the CPU as their first argument:

```go
func (m *Memory) LoadLongword(cpu *registers.CPU, addr uint32) (uint32, error)
func (m *Memory) StoreLongword(cpu *registers.CPU, addr uint32, v uint32) error
func (m *Memory) Translate(cpu *registers.CPU, addr uint32, access AccessType) (uint32, error)
```

because translation depends on the CPU's state: whether MAPEN is on,
where the page tables are (the page table registers), and the current
access mode (from the PSL), against which a page's protection is
checked. The memory doesn't keep a pointer to the CPU, and so one memory
could be read as any process's: `space.go`'s `LoadIn` and `StoreIn` take
a `vm.AddressSpace` (a process's four page table registers) in place of
the CPU's, which is how Go code reaches a process that isn't running.

The memory also keeps the *translation buffer*, a cache of recent
translations (`tb.go`), the *instruction-fetch window*, one page of
instruction stream translated once and then read directly (`fetch.go`),
the pager hook, and the frame allocator. A failed translation is a Go
error, `*vm.TranslationFault`, which the CPU turns into the VAX's
access-violation or translation-not-valid fault.

### 3.3 `cpu.Engine`: the processor in motion

`cpu.Engine` (`cpu/engine.go`) puts the two together with the
instruction table and runs instructions. Why three types and not one?
Because each is useful alone: `registers.CPU` is state that the console,
the debugger, and a process switch save and restore as a whole;
`vm.Memory` is read by Go code that runs no instructions at all; and the
engine's state (the instruction being executed, the interrupt queue, the
clocks, the hooks) belongs to neither. Its fields, cut down:

```go
type Engine struct {
    cpu   *registers.CPU
    mem   *vm.Memory
    table *Table

    instructionPC uint32 // where the current instruction began
    halted        bool

    services   SystemServices // XFC's way out to Go
    astSource  ASTSource      // ASTs waiting to be delivered
    scheduler  Scheduler      // the end of a time slice
    schedLeft  int            // instructions left in it

    iqueue           []*queuedInterrupt // interrupts requested
    interruptPending bool
    hardwareClock    bool   // the host's clock, or a count of instructions
    clockTicks       uint64 // emulated milliseconds, in the latter mode
    ...
}
```

Its heart is `Step`, which runs one instruction. In outline:

```go
func (e *Engine) Step() error {
    // Instruction and time limits, and a Ctrl/C typed at the host.
    if err := e.checkLimits(); err != nil { return err }
    if err := e.checkAttention(); err != nil { return err }

    e.mem.BeginInstruction() // the pager may take last instruction's pages

    e.tickQuantum() // or poll the host's clock: time moves on

    if e.interruptPending { e.deliverPendingInterrupt() }

    if e.scheduler != nil && e.schedLeft <= 0 { e.schedule() } // maybe switch process
    e.schedLeft--

    if e.astSource != nil { e.deliverAST() } // an AST, between instructions

    e.instructionPC = e.cpu.GPR(registers.PC)
    if f := e.beginTrace(); f != nil { return e.raise(f) } // the trace trap

    dec := ...                                   // a reused buffer
    if err := decodeInstruction(e.cpu, e.mem, e.table, dec); err != nil {
        e.undoOperands(err)
        return e.raise(err)                      // a fault while decoding
    }
    e.cpu.SetGPR(registers.PC, dec.NextPC)

    handler := e.table.HandlerFor(dec.Instruction)
    if err := handler(e, dec); err != nil {
        e.undoOperands(err)
        return e.raise(err)                      // a fault while executing
    }
    return e.integerOverflowTrap(dec.Instruction)
}
```

Everything between instructions happens at the top: time, interrupts,
the scheduler, ASTs. Then the instruction is decoded into a `Decoded`
(the `Instruction` from the table and up to six resolved `Operand`s), PC
is moved past it, and its handler runs. A handler is an ordinary Go
function, `func(e *Engine, d *Decoded) error`, registered for its
opcodes by the file that implements it. Here is ADD, all six forms of it
(ADDB2, ADDB3, ADDW2, ..., ADDL3), from `cpu/integermath.go`:

```go
func emulAdd(e *Engine, d *Decoded) error {
    a, b, size, err := loadPair(e, d) // operands 0 and 1, at the operand size
    if err != nil {
        return err
    }
    result, v, c := addResult(a, b, size)
    setArithPSL(e.cpu, result, v, c, size) // N, Z, V, C
    return storeResult(e, d, result)       // the last operand
}
```

A handler that can't finish returns a `*cpu.Fault` (an exception code
and its parameters), and `raise` puts PC back to the instruction's start
and delivers the exception through the SCB, as the VAX does (section 4).

Who calls `Step`? The machine's run loop (`machine/execute.go`), which
steps until the program it was asked to run finishes, a breakpoint or a
limit stops it, or the CPU halts.

### 3.4 `machine.Machine`: the machine as its operator sees it

`machine.Machine` (`machine/machine.go`) holds the engine, CPU, and
memory, and what the console keeps around them: the symbol table, the
radix, the state INITIALIZE/MEMORY computed, the image activator's state
for each process, and the debugger if one is installed. It is the
engine's `cpu.SystemServices`: XFC's selectors that belong to the console
(write a string to the terminal, read a character, run a console command)
it answers itself, through `Hooks`, what it asks of the console that runs
it; those that belong to a process (system services, shims, ASTs,
exceptions) it passes on through its embedded `corevms.EngineHooks` to
whichever process the CPU is running:

```go
func (h EngineHooks) Running() *Environment {
    home := h.Home()              // process 1
    if cur := home.Current(); cur != nil {
        return cur                // the process whose context the CPU holds
    }
    return home
}

func (h EngineHooks) SystemService(pc uint32) (uint32, bool, error) {
    r0, handled, err := h.Running().SystemService(pc)
    return r0, handled, EngineError(err)
}
```

This one indirection is how a single engine serves many processes: the
CPU always runs the current process's code, and the services it calls
are always that process's.

### 3.5 `corevms.System` and `corevms.Environment`: VMS

A `corevms.System` (`corevms/system.go`) is VMS's system-wide state: the
tables of services and shims, the device table, the mount table, the
mailboxes, global sections, common event flag clusters, the lock
database, OPCOM, the clock, the scheduler and the process table, the S0
pool, and, with VMS paging on, the page file and the paging counts. One
is made when the machine is initialized (INIT, and again by
INITIALIZE/MEMORY), so a fresh machine has a fresh VMS. The device and
mount tables are the exception: they belong to the console and are
handed to each new System, because a mounted volume should outlast a
re-initialization of memory.

A `corevms.Environment` (`corevms/environment.go`) is one process, and
embeds its System, so that `env.Mailboxes` or `env.Locks` reads the
shared table directly (Go *promotes* an embedded struct's fields). What's
the process's own:

- `Process`: the process's identity and accounting, much of VMS's
  process control block and job information block: PID, name, user
  name, owner, quotas, privileges, its local event flags, its image's
  name (`corevms/process.go`).
- `Space`: its address space, a `ProcessSpace` that holds the four page
  table registers (`vm.AddressSpace`) and where its page tables are.
- `Stacks`: its kernel, executive, and supervisor stacks and its
  hardware PCB, the 96-byte block of registers LDPCTX loads.
- `Logicals`: its view of the logical name database (its own process
  table, and the shared job, group, and system tables).
- `Session`: its RMS session (its default directory and the mount
  table), and `files`, its open files.
- `channels`: the channels `$ASSIGN` gave it.

Process 1 is special: it's the console's process, whose address space and
stacks are the ones INITIALIZE/MEMORY lays out, and whose command
interpreter is the console's DCL. The others are made by `$CREPRC`,
LIB$SPAWN, and DCL's SPAWN, each with an address space and stacks taken
from the S0 pool (section 7).

### 3.6 RMS's session and the mount table

`rms.MountTable` (`rms/mount.go`) is the table of mounted volumes,
device name to an `ods2` volume opened on a container file, shared by
the console and every process. `rms.Session` (`rms/session.go`) is what
one process's file operations need: the mount table, its logical names,
and its default directory. Section 9 follows a `$GET` from a program's
RAB to a block of a container.

### 3.7 One instruction's trip

To see the structures work together, follow a program's call of a system
service, `$ASSIGN`, from VAX code to Go and back:

1. The program executes `CALLS #4, @#SYS$ASSIGN`. SYS$ASSIGN is an
   address in the system service vector near the top of P1. The engine's
   CALLS handler builds a call frame and jumps there.
2. At that address the microkernel's assembly put a tiny procedure: an
   entry mask of 0 (save nothing), then `XFC #^X7A`, then RET. The engine
   decodes XFC, opcode `FC`, whose handler (`cpu/xfc.go`) reads the
   selector, `7A`, XFC$P1VECTOR.
3. That handler calls `e.services.SystemService(pc)` with the address of
   the vector entry. `services` is the `machine.Machine`, whose
   `EngineHooks` pass the call to the running `corevms.Environment`.
4. `Environment.SystemService` (`corevms/environment.go`) looks the
   address up in the vector's table (`vmsdef/p1vector.go`), finds the name
   `SYS$ASSIGN`, finds the Go function registered under it, reads the
   argument list from VAX memory at AP, and calls it.
5. The service, in Go, reads the device name through the argument's
   descriptor, finds the device in the System's device table, makes a
   channel in the Environment, writes the channel number to the address
   the program gave, and returns `SS$_NORMAL`.
6. Back in the XFC handler, R0 is set to the status, and the engine goes
   on to the next instruction, the vector's RET, which returns to the
   program.

Every one of govax's VMS services is reached this way, and section 10
takes the path apart, with what happens when a service must wait, call
back into VAX code, or end the image.

## 4. The instruction set

govax implements every instruction of the VAX architecture: 306 of them,
from HALT to the H_floating polynomial, 56 of which are two-byte opcodes
behind the `FD` prefix. This section follows an instruction from the
table that describes it, through decoding and execution, to what
happens when it can't finish, and ends with how govax knows it has the
instructions right.

### 4.1 The table

Everything the CPU knows about an opcode before it sees any operands is
one `cpu.Instruction` (`cpu/instruction.go`):

```go
type Instruction struct {
    Name         string
    Opcode       Opcode       // the FD prefix (or 0) and the opcode byte
    OperandCount int
    Scale        [6]int        // each operand's size in bytes
    Access       [6]AccessKind // read, write, modify, address, field, branch, ...
    DataType     [6]DataType   // byte, word, ..., F, D, G, H floating
    TrapsOnOverflow bool       // V means integer overflow (section 4.5)
}
```

That's the VAX manual's *format line*, the line each instruction's page
begins with, in a struct. The manual writes ADDL3 as

```text
ADDL3 add1.rl, add2.rl, sum.wl
```

where each operand's two letters are its *access* (`r` read, `w` write,
`m` modify, `a` address, `v` bit-field base, `b` branch displacement) and
its *data type* (`b`, `w`, `l`, `q`, `o` for the integers of 1 to 16
bytes; `f`, `d`, `g`, `h` for the floating formats). The table has the
same: ADDL3 is three longword operands, read, read, and written.

The table, `cpu/instructions_table.go`, is generated, not written by
hand. Its generator (`cpu/gen`) read the C emulator govax was first
ported from and checked it against a second source, `cpu/gen/operands.go`,
every instruction's operands transcribed from the manual's format lines;
where they disagreed the manual won. `TestInstructionTableMatchesManual`
holds the table to that transcription still.

Two kinds of entry aren't in the manual. Eight placeholders (`RSVD_...`
and `EXT_...`) fill reserved opcodes, so that decoding one gives the
architecture's reserved-instruction fault. And XFC, opcode `FC`, which
the VAX reserved for a customer's own microcode, is govax's way out of
the emulated machine into Go: it takes one *implicit* byte, a selector,
which the manual has no letter for, so govax added `i`. Section 10 is
about XFC.

The table holds data only. Each instruction's behavior is a *handler*, a
Go function registered for its opcodes by the file that implements it,
in that file's `init` function (which Go runs once, when the program
starts):

```go
func init() {
    reg := func(fn byte, h Handler) {
        instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
    }
    for _, fn := range []byte{0x80, 0x81, 0xA0, 0xA1, 0xC0, 0xC1} { // ADDB2 ... ADDL3
        reg(fn, emulAdd)
    }
    ...
}
```

So `integermath.go` has ADD, SUB, MUL, DIV, BIS, BIC, XOR; `movc.go` the
character moves; `fpu.go`, `floatmath.go`, `cvtfloat.go` the floating
instructions; `decimal*.go` and `editpc.go` packed decimal; `call.go`
CALLS, CALLG, RET, and REI; and so on, a file per family. An opcode with
no handler gets the reserved-instruction fault, and
`TestEveryInstructionImplemented` checks that only the placeholders are
left that way. The assembler and the disassembler read the same table,
so the three can't disagree about an instruction's operands.

### 4.2 Decoding: operand specifiers

`decodeInstruction` (`cpu/decode.go`) reads the opcode (a second byte if
the first is `FD` or above), looks it up, and then decodes one operand
per entry of the format, with `decodeOperand` (`cpu/operand.go`). Two
kinds of operand have no specifier byte: a branch displacement, a signed
1, 2, or 4 bytes added to the PC, and govax's implicit immediate. For
the rest, the specifier's high four bits are the *mode* and its low four
a register:

| Mode | Assembler | Meaning | With PC as the register |
| --- | --- | --- | --- |
| 0–3 | `S^#n` | short literal: the six bits are the value (0 to 63, or a floating value from 0.5 to 120) | |
| 4 | `base[Rx]` | indexed: another specifier follows, and Rx times the operand's size is added to its address | |
| 5 | `Rn` | register | |
| 6 | `(Rn)` | register deferred: the operand is at the address in Rn | |
| 7 | `-(Rn)` | autodecrement: Rn is decreased by the size, then used | |
| 8 | `(Rn)+` | autoincrement: Rn is used, then increased | `I^#n`, immediate: the value follows |
| 9 | `@(Rn)+` | autoincrement deferred: the longword at Rn is the address | `@#addr`, absolute |
| A, C, E | `B^d(Rn)`, `W^`, `L^` | displacement: Rn plus a byte, word, or longword that follows | relative to the PC: position-independent |
| B, D, F | `@B^d(Rn)`, ... | displacement deferred: the longword there is the address | relative deferred |

Immediate and absolute modes are just autoincrement on the PC: the PC
points at the bytes after the specifier, so `(PC)+` reads them and steps
past. That's the VAX's regularity at work, and the decoder keeps it:
mode 8 on the PC is decoded as an immediate, but it's the same idea.

Decoding resolves each operand to a `cpu.Operand`, which says where the
value is, not yet what it is:

```go
type Operand struct {
    Access AccessKind
    Kind   OperandKind   // OperandRegister, OperandMemory, or OperandImmediate
    Reg    registers.Reg // for a register operand
    Size   uint8         // 1, 2, 4, 8, or 16 bytes
    Addr   uint32        // for a memory operand: its virtual address
    Value  uint64        // for an immediate: the value (low 64 bits)
    High   uint64        // the high 64 bits of a 16-byte immediate
}
```

Here is `ADDL3 #5, 8(R2), R3` decoded, with R2 holding `00001000`:

```text
  bytes   C1  05  A2 08  53
          |   |   |      +-- operand 2: mode 5, R3        -> Register R3
          |   |   +--------- operand 1: mode A, R2, disp 8 -> Memory at 00001008
          |   +------------- operand 0: mode 0, literal 5  -> Immediate 5
          +----------------- opcode ADDL3: rl, rl, wl
```

A register, an address, or a value: that's all a handler needs.
`Operand.Load` and `Operand.Store` (`cpu/operandaccess.go`) read and
write each kind at its size, so `emulAdd` (section 3.3) never asks which
mode an operand used.

Decoding has side effects: autoincrement and autodecrement change their
registers as they're decoded, and a deferred mode reads memory, which
may fault. The decoder enforces the architecture's rules as it goes: a
short literal or an immediate can't be written (a *reserved addressing
mode* fault), indexed mode can't index itself, and an octaword in
registers, which spans four of them, can't start past R11. A register
operand where the instruction wants an address is allowed through to
the handler, which decides: CALLG uses the register's value as the
argument list's address, and govax's microkernel depends on that.

A note on speed. govax runs some 10 to 20 million VAX instructions a
second, and the decode path is most of where the time goes. A few
choices there are for speed alone: the `Decoded` is two buffers kept in
the engine and reused (so no instruction allocates memory), an `Operand`
is packed into 24 bytes, and the instruction stream is read through the
memory's *instruction-fetch window*, one page of code translated once
and then read straight from the RAM slice (section 5). None of this
changes what an instruction does.

### 4.3 Executing

Once decoded, the engine moves PC past the instruction and calls its
handler. Most handlers are short: load the operands, compute, set the
condition codes, store. The condition codes are where much of the care
goes; `cpu/condcodes.go` computes N, Z, V, and C for every size from a
result worked out in 64 bits, so that "did this byte add overflow?" is
asked the same way for every width.

The call instructions are the richest of the ordinary ones, and show the
style. CALLS's handler (`emulCall` and `buildCallFrame`, `cpu/call.go`)
does what the manual's description says, step by step:

```go
// CALLS: push the argument count; it and the arguments are the list.
count, _ := d.Operands[0].Load(e.cpu, e.mem)
sp := e.cpu.GPR(registers.SP) - 4
e.mem.StoreLongword(e.cpu, sp, uint32(count))
newAP := sp
...
// The procedure's entry mask: which of R0-R11 to save.
mask, _ := e.mem.LoadWord(e.cpu, newPC)
if mask&0x3000 != 0 {
    return &Fault{Code: ExcReservedOp} // bits 13:12 must be zero
}
for n := 11; n >= 0; n-- { if mask&(1<<n) != 0 { push R[n] } }
push(PC); push(FP); push(AP)
// The PSW with T clear, the mask, and how SP was aligned, in one longword;
// then the trap enables the mask asks for take effect (IV, DV).
push(maskWord); push(0) // the condition handler slot
FP = SP; AP = newAP; PC = newPC + 2
```

which leaves the frame VMS's condition handling and its debugger walk:

```text
            :                 :
   FP ->  | 0 (handler)       |   0(FP)  a condition handler, if one is set
          | mask | PSW        |   4(FP)
          | saved AP          |   8(FP)
          | saved FP          |  12(FP)  the caller's frame: a chain
          | saved PC          |  16(FP)  the return address
          | saved R2 ...      |  the registers the mask named
          | argument count    |  <- AP, for CALLS
          | arguments         |
```

RET reads it back. The same `buildCallFrame` also delivers an AST
(section 7), which on VMS is a CALLG made between two instructions.

Beyond the integers, the instruction set has three families of its own,
each with a package or file that does the arithmetic:

- **Floating point.** The VAX has four formats: F (32 bits), D and G (64
  bits, D with more fraction, G with more exponent), and H (128 bits). They
  aren't IEEE formats, and their rounding (half away from zero) isn't
  IEEE's, so govax doesn't use Go's `float64` for them. `internal/vaxfloat`
  unpacks a value into an exact number (`math/big`), does the arithmetic
  exactly, and rounds once to the destination's format, reporting overflow,
  underflow, divide by zero, and the *reserved operand* (a value with the
  sign set and the exponent zero, which faults when used). `cpu/fpu.go` is
  the CPU's side, which moves bits between operands and that package. An
  operand's format comes from its data type in the table, never its size,
  since D and G are both 8 bytes.
- **Packed decimal.** Strings of up to 31 decimal digits, two to a byte,
  with a sign nibble (`-123` is the bytes `12 3D`), for COBOL and PL/I:
  move, compare, add, subtract, multiply, divide, shift, convert, and
  EDITPC's picture editing (`cpu/decimal.go`, `decimalmath.go`,
  `decimalconvert.go`, `editpc.go`).
- **Strings, bit fields, queues.** MOVC3 and MOVC5, CMPC, LOCC, SKPC,
  SCANC, MATCHC, MOVTC, CRC; variable-length bit fields (EXTV, INSV, FFS,
  ...), which may lie in registers; and the queue instructions, INSQUE and
  REMQUE and their interlocked forms. govax emulates one processor, so the
  interlocked ones are the plain ones done at once.

The long instructions don't model `PSL<FPD>`, the *first part done* bit
with which a real VAX lets a string move be interrupted and resumed
part way. Each reads all its sources, computes its result, and only then
writes, so if it faults it can run again from the start and get the same
answer.

### 4.4 Between instructions

The top of `Step` (section 3.3) is where the VAX's asynchronous events
are taken, in the architecture's order, before the next instruction is
decoded:

1. **Time.** In the default *quantum* mode, the engine counts
   instructions, and every `quantum` of them is one emulated millisecond
   of system time (STARTUP.COM's `SET QUANTUM 1` makes each instruction
   a millisecond). Time is then a function of the
   instructions executed, and a program's timers expire at the same
   instruction every run, which makes govax deterministic and its tests
   repeatable. With `vax.hardware.clock` set, system time is the host's,
   read every 1,024 instructions (`cpu/clock.go`, `cpu/systime.go`). The
   time-of-year register, TODR, is computed from the system time whenever
   it's read, so the two always agree.
2. **Interrupts.** A device or software interrupt request
   (`Engine.Interrupt`, `cpu/interrupt.go`) is queued with its IPL, and
   admitted when its level is above the CPU's. Delivery is an exception
   through the SCB like any other, on the interrupt stack. The interval
   clock, the software interrupt requests of MTPR to SIRR, and the console
   terminal's registers make interrupts. govax's own I/O doesn't: its
   device drivers are Go code that completes an I/O and reports it to the
   waiting process directly (section 8), so in practice a running govax
   takes almost no interrupts. The microkernel leaves the interval clock
   stopped.
3. **The scheduler.** When the current process's slice of instructions
   is spent, the scheduler may switch to another process (section 7).
4. **ASTs.** If an AST is due for the process in a mode it may be
   delivered to, it's delivered: the Go side pushes its argument list and
   the engine builds a CALLG frame for its routine (`cpu/ast.go`;
   section 7).
5. **The trace trap.** If the last instruction was traced, its trace
   fault comes now (`cpu/trace.go`). Tracing takes two PSL bits, T (trace
   enable) and TP (trace pending): at each instruction's start TP is
   copied from T, and a set TP makes the fault at the start of the *next*
   one. The two-step is what gives exactly one trace per instruction when
   the instruction faults and runs again, takes an arithmetic trap, or is
   followed by an interrupt, and govax follows the manual's rules for each.
   VMS signals the trace to the program as `SS$_TBIT`, so a program can
   trace itself.

### 4.5 When an instruction can't finish

A handler that meets an exception returns a `*cpu.Fault`:

```go
type Fault struct {
    Code Exception // its offset in the SCB: ExcAccessViol is 0x20, ...
    Args []uint32  // its parameters: an access violation's address and reason
    Trap bool      // taken after the instruction finished
}
```

and `Engine.raise` delivers it. The difference between a fault and a trap
is the whole of the VAX's restart model:

- A **fault** (an access violation, a reserved operand, a page not
  valid) means the instruction hasn't happened. The saved PC is the
  instruction's own, so that when the handler returns, it runs again. For
  that to work, its autoincrements and autodecrements must be undone: the
  CPU's *operand log* (`registers/operandlog.go`) recorded each register
  before a specifier changed it, and `undoOperands` puts them back, last
  first. So a program's handler can fix a reserved floating operand, or
  create the page that was missing, and continue.
- A **trap** (integer overflow, decimal overflow, divide by zero, a
  change-mode instruction) means it has: results stored, PC past it.
  Integer overflow traps only if `PSL<IV>` is set (a procedure's entry mask
  sets it with bit 14), and only for the instructions whose V means
  overflow, which the table marks (`TrapsOnOverflow`; `cpu/inttrap.go`).
  An integer divide by zero always traps.

`Engine.HandleFault` (`cpu/handlefault.go`) then does what the processor
does. It reads the exception's vector from the SCB at `SCBB + code`,
physically. The vector's low two bits say which stack: the current
mode's (or the target mode's, for a change-mode trap), or the interrupt
stack. It clears T and TP in the new PSL, switches stacks and modes,
pushes the old PSL, the PC, and the parameters, and jumps to the vector.

One vector value is govax's own. The microkernel fills most of the SCB's
exception vectors with `console$handler`, which is `FFFFFFFF`: "not
handled by VAX code". When `HandleFault` finds it, it offers the
exception to the program's VMS condition handlers through the
`cpu.ExceptionDispatcher` seam, which the Go side of VMS implements: it
builds the signal and mechanism arrays on the program's stack and starts
the search of its call frames, exactly where VMS's exception dispatcher
would (section 10). Only if the dispatcher can't take it (no program is
running, say) does the exception go back to the console, which reports it
and stops.

### 4.6 Changing mode

CHMK, CHME, CHMS, and CHMU are traps to the target mode's stack, with the
instruction's operand (a code, conventionally the service's number) as
the parameter (`cpu/changemode.go`). REI pops a PC and PSL and returns to
them, saving the old mode's stack pointer in its register and loading the
new mode's (`emulRei`, `cpu/call.go`). The previous-mode field of the PSL
is set as the architecture requires.

Since any code may execute REI, user mode included, REI is also a guard.
Before it changes anything it checks the PSL it popped against the one
the CPU has (`reiAllows`), and takes a reserved operand fault, with SP
and everything else as they were, if the new PSL would:

- move to a more privileged mode (REI only goes outward; inward takes
  CHMx, which goes only where the SCB's vectors say);
- raise IPL, or have an IPL above 0 outside kernel mode;
- set IS, the interrupt stack bit, when the CPU isn't on the interrupt
  stack now, or with IPL 0;
- record a previous mode more privileged than its current one;
- set a must-be-zero bit, or CM, compatibility mode, which govax doesn't
  implement.

So a user-mode program can't push a kernel-mode PSL and REI into kernel
mode. The microkernel's own REI into user mode at boot pushes a PSL with
both modes user, `^X03C00000`, for the same reason. `Engine.LoadContext`,
the Go scheduler's LDPCTX-and-REI (section 7), applies the same check to
the PSL in the process's PCB. Every change of mode empties the
translation buffer's notion of what the current mode may access
(`Memory.InvalidateProtection`), since the same page may be writable in
one mode and not in another.

VMS reaches its system services by CHMK and CHME. govax's don't: its
system service vector holds an XFC instead of a CHMK (section 10), and
the service runs in Go without the CPU changing mode at all. The
microkernel still installs a CHMK dispatcher for VAX code that uses
one. `$CMKRNL` and `$CMEXEC`, which call a program's own routine in an
inner mode, do change mode, but from Go: the service switches the CPU's
mode and stack itself, as REI would, and has the engine call the routine
there (`corevms/cmode.go`).

The other privileged instructions are here too: MTPR and MFPR
(`cpu/procreg.go`, with a table of which registers may be read and
written, and the side effects of writing one, such as TBIA flushing the
translation buffer or SIRR requesting a software interrupt), LDPCTX and
SVPCTX (`cpu/context.go`, a process's hardware context; section 7),
PROBER and PROBEW (which ask whether a mode could read or write an
address, without faulting), and HALT.

### 4.7 Fidelity: how govax knows

govax's instructions answer to three authorities, in order: what a real
VAX running VMS does, when that has been measured; DIGITAL's *VAX
Architecture Reference Manual*; and, where neither speaks, a reasonable
choice, written down. The C emulator govax was first ported from was the
starting point, not an authority: its fidelity was good but not perfect,
and where it disagreed with the manual, the finding went into
`docs/DEVIATIONS.md` and the manual's behavior into govax. That log
(and `DEVIATIONS-RESOLVED.md`, the entries since settled) is the record of
every place govax was found to differ from VAX/VMS, with the reason and
what was done.

The measurements come from *oracle tests*: a program run on a real VMS
system (OpenVMS V7.1 on the simh simulator's VAX 8600, a model with the
whole instruction set in its microcode), its output saved in the
repository, and a Go test that runs the same program on govax and
compares. For the instruction set:

- `testdata/insn35` is 567 cases of the floating, octaword, and packed
  decimal instructions, each one instruction run under a condition
  handler, recording the PSL, R0 to R11, the signal array if any, and the
  destination's bytes. `TestInsn35Oracle` (`internal/console`) holds every
  case to VMS's, field for field, but for the few fields it masks, each
  one of the differences listed below. The cases were chosen for the edges:
  rounding ties, the low bits of D_floating, reserved operands, overflow
  and underflow with and without their trap enables, invalid digits and
  signs, lengths 0 and 31.
- `testdata/lib53` runs 132 cases of LIBRTL's routines that are
  instructions (LIB$EMUL, LIB$INSQHI, LIB$MOVTC, ...), which govax writes
  with the CPU's own code, so they check it too.
- `testdata/trace73` checks the trace trap, and `testdata/tb70` the
  exceptions and traps as VMS reports them to a program.
- Beyond the probes, every program in the FORTRAN corpus and govax's run
  time library FTNRTL has been built by VMS's own MACRO and LINK and run
  there, giving govax's output (but for differences the tests list), and
  DUNGEON, a large FORTRAN game, plays a scripted game the same on both to
  the byte.

Below them are the `cpu` package's own unit tests, some seventy files,
which check instructions against the manual's descriptions, and the
table against its format lines.

What govax knowingly does differently:

- **`PSL<FPD>` isn't modeled.** The long instructions run whole or restart
  (section 4.3). EDITPC's aborts part way, which VMS reports with FPD set
  and its registers part way, report with neither.
- **CHMx skips two checks:** that it isn't executed on the interrupt
  stack, and that the new stack has room for the exception's frame.
- **No PDP-11 compatibility mode,** and no machine checks or power
  failures: govax's hardware doesn't fail.
- **One processor.** The interlocked instructions interlock nothing.
- **Choices where the manual says UNPREDICTABLE:** an octaword in R12 or
  above is a reserved addressing mode fault; DIVP's result with an
  invalid digit differs from the VAX 8600's.
- **The services aren't VAX code.** A program that traces itself through
  a system service or a LIBRTL routine sees govax's few stub
  instructions where VMS would show its own code (section 10).

## 5. Memory: layout, translation, and faults

*To be written.*

## 6. VMS paging

*To be written.*

## 7. Processes

*To be written.*

## 8. I/O

*To be written.*

## 9. RMS and ODS-2

*To be written.*

## 10. From VAX code to Go and back

*To be written.*

## 11. Starting the system

*To be written.*

## 12. A map of the source

*To be written.*
