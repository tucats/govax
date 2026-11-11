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

govax was built in phases, and each phase has a document in
`docs/planning/` (`PHASE-nn - topic.md`) recording what it set out to do, what it found,
what was decided, and a dated log of the work; `docs/planning/PLAN.md` indexes
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
and its default directory. Section 9.7 follows a `$GET` from a program's
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

RET reads it back, and after a CALLS removes the argument list too, by
the count longword's low byte: an argument list holds at most 255
arguments, and the procedure calling standard reserves the count's other
three bytes. The same `buildCallFrame` also delivers an AST
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
   admitted when its level is above the CPU's. Delivery goes through the
   SCB as an exception's does, on the stack its vector names (the
   microkernel's vectors all name the kernel stack). The interval
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
physically. Its bits 31:2 are the service routine's address, and its low
two bits say which stack: 0 the kernel stack (for a change-mode trap, the
stack of its mode, 4.6), unless the CPU is already on the interrupt stack,
where the event stays; 1 the interrupt stack, where an exception, though
not an interrupt, also raises IPL to 31.
Changing stacks never changes MAPEN: the interrupt stack is mapped like
the others (section 5.5). `HandleFault` clears T and TP in the new PSL,
switches stacks and modes, pushes the old PSL, the PC, and the
parameters, and jumps to the vector.

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

CHMK, CHME, CHMS, and CHMU are traps through their own SCB vectors, with
the instruction's operand (a code, conventionally the service's number) as
the parameter (`cpu/changemode.go`). The trap's mode is the instruction's
or the current one, whichever is more privileged, so a change-mode
instruction never moves outward: a CHMS from kernel mode stays in kernel
mode, on the kernel stack. One on the interrupt stack halts the processor. REI pops a PC and PSL and returns to
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
address, without faulting: the less privileged of the mode operand and
PSL's previous mode, so that code in an inner mode checks its caller's
access), and HALT.

### 4.7 Fidelity: how govax knows

govax's instructions answer to three authorities, in order: what a real
VAX running VMS does, when that has been measured; DIGITAL's *VAX
Architecture Reference Manual*; and, where neither speaks, a reasonable
choice, written down. The C emulator govax was first ported from was the
starting point, not an authority: its fidelity was good but not perfect,
and where it disagreed with the manual, the finding went into
`docs/DEVIATIONS.md` and the manual's behavior into govax. That log
(and `docs/planning/DEVIATIONS-RESOLVED.md`, the entries since settled) is the record of
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

This section is the VAX's memory as govax builds it: physical memory, the
page tables and how an address is translated through them, the caches
that make translation fast, the layout INITIALIZE/MEMORY gives the system
and the first process, and what happens when a page isn't there. VMS's
paging, which decides what happens to pages over time, is section 6. This
section is the mechanism it stands on, and, with paging off (the
default), the whole of memory management.

### 5.1 Physical memory

Physical memory is `vm.Memory`'s `ram`, a Go byte slice in which `ram[n]`
is physical address `n` (section 3.2). STARTUP.COM's `INIT/VAX` sizes it:
16384 pages, 8 MB, unless `vax.memory.size` says otherwise. A page of
physical memory is a *page frame*, or just a *frame*, named by its *page
frame number* (PFN): frame `n` is bytes `n*512` to `n*512+511`.

A `vm.FrameAllocator` hands frames out and takes them back
(`vm/frames.go`). With paging off it's a bitmap, one flag per frame,
handing out the lowest free frame; a frame given back is cleared at once,
so every frame handed out reads as zeros. With paging on it's VMS's PFN
database, which knows whose page each frame holds (section 6). Either way
frame 0 is never handed out: it's the first page of S0.

While MAPEN is 0, as it is from `INIT/VAX` until INITIALIZE/MEMORY turns
it on, there's no translation at all: every address is physical. Even with
mapping on, the processor reads a few structures by physical address, and
govax reads them the same way (`Memory.LoadPhysical` and
`StorePhysical`): the system page table (at SBR), the system control block
(at SCBB), and a process's hardware PCB (at PCBB).

Beside RAM are two more buffers, the console's ROM and NVRAM, each with a
physical base address beyond RAM's end. STARTUP.COM loads
`SYS$SYSTEM:DEFAULT.ROM` at `20040000`–`200BFFFF`, where a MicroVAX II's
boot ROM is, and no NVRAM. Every physical access goes through `phys`
(`vm/memory.go`), which looks in RAM, then the ROM, then the NVRAM, and
reports a `*vm.PhysicalAddressError` for anything else. Nothing in govax's
VMS maps them; they're the console's (LOAD/ROM, SHOW ROM).

### 5.2 Page table entries

A page table entry is a longword (`vm.PTE`, `vm/pte.go`):

```text
 31  30-27  26  25  24-23  22-21  20-0
  V  PROT   M   -   OWN    S      PFN
```

- **V**, valid: the page is in memory, in frame PFN. The rest of this
  list is about a valid PTE; an invalid one keeps PROT and OWN, and uses
  the other bits to say where the page is (section 5.7).
- **PROT**, the protection code: which access modes may read and write
  the page.
- **M**, modified: the hardware sets it on the first write to the page,
  so the operating system knows which pages must be written back.
- **OWN**, the owner: the access mode the page belongs to. The hardware
  ignores it; VMS's services check it, so that `$DELTVA` from user mode
  can't delete a page kernel mode made, say.
- **S**, two bits for the operating system's own use.
- **PFN**, the frame, 21 bits: up to a gigabyte of physical memory.

The protection code is four bits naming who may read and who may write.
Its names say it outright: `URKW` is "user read, kernel write". There are
fifteen:

| Code | Name | May read | May write |
| --- | --- | --- | --- |
| 0 | NA | no mode | no mode |
| 1 | — | reserved: the architecture leaves it unpredictable | |
| 2 | KW | kernel | kernel |
| 3 | KR | kernel | no mode |
| 4 | UW | every mode | every mode |
| 5 | EW | kernel, executive | kernel, executive |
| 6 | ERKW | kernel, executive | kernel |
| 7 | ER | kernel, executive | no mode |
| 8 | SW | kernel to supervisor | kernel to supervisor |
| 9 | SREW | kernel to supervisor | kernel, executive |
| 10 | SRKW | kernel to supervisor | kernel |
| 11 | SR | kernel to supervisor | no mode |
| 12 | URSW | every mode | kernel to supervisor |
| 13 | UREW | every mode | kernel, executive |
| 14 | URKW | every mode | kernel |
| 15 | UR | every mode | no mode |

Codes 2, 3, and 5 to 15 follow a formula: bits 3–2 are the least
privileged mode that may read, and bits 1–0, complemented, are how many
modes may write, counting out from kernel. (UW, code 4, is the exception,
in the place the formula would give a code that contradicted itself.) A
mode that may write may always read.
`Protection.allows` computes it, and the translation buffer keeps the
answer for every code and mode in a table (`protGrants`, `vm/tb.go`).

### 5.3 Translating an address

`Memory.Translate(cpu, addr, access)` (`vm/translate.go`) turns a virtual
address into a physical one for a read or a write in the CPU's current
mode, or fails with the fault the VAX would take. Here it is in order,
without the caches (section 5.4):

1. **Mapping off?** With MAPEN 0, the address is physical; return it.
2. **Split the address.** Bits 31–30 are the region, bits 29–9 the page
   number within it, bits 8–0 the byte within the page.
3. **Check the length.** Each region's length register bounds its page
   table. A P0 page must be below P0LR, and an S0 page below SLR. P1 grows
   down, so its table describes only its top pages, and P1LR is the
   lowest page it describes: a P1 page must be at or above P1LR. S1
   doesn't exist, and every S1 address fails this check. A failure is an
   access violation with the reason mask's *length* bit set.
4. **Find the PTE.** An S0 page's PTE is at `SBR + 4*page`, a physical
   address. A P0 page's is at `P0BR + 4*page`, but P0BR is an S0
   *virtual* address (the process page tables live in S0), so the PTE's
   own address must be translated first, through the system page table:
   the one recursion in the whole walk. P1 is the same, from P1BR, which
   is where P1's entry 0 *would* be; only the entries from P1LR up
   exist.
5. **Check the protection**, against the CPU's current mode, before the
   valid bit: an invalid PTE still has a protection, and when both checks
   would fail, the access violation takes precedence over the page fault.
6. **Valid?** If not, the page has to be brought in (section 5.7); if it
   can't be, the access is a *translation not valid* fault.
7. **Set M.** On a write to a page whose M is clear, set it in the PTE
   in memory.
8. **Answer:** `PFN*512 + offset`.

The recursion in step 4 is a read the processor makes on its own behalf,
and the VAX makes it as kernel mode, whatever mode the access is in (the
*VAX Architecture Reference Manual*: "fetch by system space translation
algorithm, including length and kernel mode access checks"). So a user
program's access to its own P0 goes through even when the page holding
its page table is one only kernel mode may read, as VMS's are; govax's
`translatePTE` makes the same kernel-mode read. If the PTE's own
reference fails (its S0 page is past SLR, no access even for kernel, or
not valid), the fault reports the original address, with the reason
mask's *PTE reference* bit set.

A walk through one, in process 1 after startup with paging off. A
user-mode program reads address `00000A10` for the first time (every
number hexadecimal):

```text
  address 00000A10: region 0 (P0), page 5, byte 010

  page 5 < P0LR (4000)                                     length is fine
  PTE address: P0BR + 4*5 = 80008C00 + 14 = 80008C14        an S0 address
    S0 page 46 < SLR (2000)
    its PTE, at SBR + 4*46 = 00000C00 + 118 = 00000D18:  F0000046
      valid, URKW, frame 46: kernel may read it           (step 5, as kernel)
    80008C14 is physical 00008C14
  the PTE at physical 00008C14:  21800000
    invalid, UW, owner user, frame 0: demand zero         (step 5: user may read)
  not valid: give it a frame (5.7), say frame n;
    the PTE becomes A1800000 + n
  physical address: n*512 + 010
```

A later write sets M, and the PTE becomes `A5800000 + n`.

A failed translation is a `*vm.TranslationFault`: its `Kind`
(`AccessViolation` for a length violation, `ProtectionViolation`,
`TranslationNotValid`), the address, and the reason mask (1 length
violation, 2 PTE reference, 4 a write). The CPU turns it into the VAX's
access violation (SCB offset `20`) or translation-not-valid fault (`24`),
with the mask and the address as its two parameters (section 4.5). govax's
VMS signals both to the program as `SS$_ACCVIO`, with the mask and the
address, as VMS does for a page that can't be made valid.

Not every reader of memory is an instruction, and `vm` has a door for
each kind:

| Method | For | How it differs |
| --- | --- | --- |
| `Translate`, `LoadLongword`, `StoreByte`, ... | instructions' operands, and Go code acting as the running process (a service reading its arguments) | the CPU's mode; the caches; an invalid page brought in |
| `ProbeTranslate` | Go code that mustn't grow the user stack: the debugger's reads, `$QIO`'s buffer checks | as `Translate`, without the stack expansion (section 6) |
| `ProbeAccess` | PROBER and PROBEW | reads the PTE's protection, and nothing more: an invalid page isn't brought in |
| `LookupPTE`, `StorePTE` | Go code that reads or changes a PTE: the services, the debugger's SHOW PAGE and SET PTE | the PTE itself, with no access check |
| `TranslateIn`, `LoadIn`, `StoreIn`, `LookupPTEIn`, `StorePTEIn` | Go code reaching a process that isn't running: a mailbox message copied into a waiting reader's buffer, an image put into a new process | another process's tables, named by a `vm.AddressSpace`; kernel mode; no caches |
| `LoadPhysical`, `StorePhysical` | the PCB, the SCB | no translation |

The console reads and writes memory in kernel mode, whatever mode the CPU
is in (`Machine.withKernelMode`, `machine/image.go`), which is how
EXAMINE, DEPOSIT, and the image loader reach pages a user program
couldn't.

An access that fits in one page is one translation. One that crosses a
page boundary, a longword at `000001FE` say, goes a byte at a time, each
byte translated on its own (`vm/storage.go`), so each page is checked.

### 5.4 Making it fast: the TB, the STC, and the fetch window

A walk is several memory reads, and nearly every instruction makes a
translation or three. Like the VAX, govax remembers recent answers. These
caches change only the speed: with them or without, every access gets
the same answer, as long as they're emptied whenever a PTE they might
hold changes.

**The translation buffer** (`vm/tb.go`) has 128 entries. A page's slot is
its region times 32 plus the low five bits of its page number, so each
region has 32 slots, and two pages with the same low bits take turns. An
entry holds the page number, the frame's address, the protection code,
the modify bit, and, worked out from the code, how much each of the four
modes may do. It hits when it holds the page and its grant for the
current mode lets the access through. One exception: a write to a page
whose M is still clear misses, so that the walk sets M in the PTE. The
entries are emptied by MTPR to TBIA (all of them) and TBIS (one), by
`StorePTE` for the page it changes, and at a process switch, which empties
only the P0 and P1 slots: S0 is the same in every process. A nonzero TBDR,
the TB disable register, stops lookups, though the walk still fills
entries.

**The sequential translation cache** (the STC) is a single slot in front
of the TB: the last page translated, and what the current mode may do
with it. Most accesses are to the page the last one used, and the STC
answers those with one comparison. It's emptied with the TB, at every
fault, and at every change of mode (`Memory.InvalidateProtection`, from
REI, CHMx, exception and interrupt delivery, and AST delivery). The
kernel-mode read of a process PTE (section 5.3) leaves it alone, since
what it holds is for the current mode.

**The instruction-fetch window** (`vm/fetch.go`) is a third, for the
instruction stream alone: one page of code, translated once, which the
decoder reads straight out of RAM until PC leaves the page. Data accesses
can't evict it. It's emptied with the STC, and before each instruction if
MAPEN or the mode isn't what it was when the window was filled
(`SyncFetchWindow`). It holds a physical address, not a copy, so code that
writes into itself is seen at once.

The debugger's SHOW TB shows the TB's entries and how often each cache
hit.

### 5.5 The layout INITIALIZE/MEMORY builds

`INITIALIZE/MEMORY` (still `VMInit` in the code, `machine/vminit.go`) makes
memory a VMS system's. STARTUP.COM runs it with paging off as

```text
$ initialize/memory /p0=16384 /p1=8192 /s0=8192 /ksp=20 /esp=8 /ssp=8 /isp=4
```

after `INIT/VAX` has made 8 MB of memory. It starts from scratch: a new
`vm.Memory` (so physical memory is all zeros), a new engine, a new
`corevms.System`, and the other processes ended. Then, with MAPEN still 0,
it builds the system page table, S0 page `n` mapping frame `n` for every
page of S0, each `URKW`. S0 is then the whole of physical memory, in
order, and anything the processor reads by physical address (the system
page table, the SCB) is at the same offset in S0. With paging off S0
therefore can't be bigger than physical memory. Then it lays out the rest
of S0 from its first page, sets the registers, and turns MAPEN on. The
result, from STARTUP.COM's sizes:

| S0 addresses | Pages | Protection | What |
| --- | --- | --- | --- |
| `80000000`–`80000BFF` | 6 | URKW | VMS V1's system service vector: a stub per service that jumps to its entry in the P1 vector, for images linked before VMS V4 (section 10) |
| `80000C00`–`80008BFF` | 64 | URKW | the system page table: SBR `00000C00`, SLR `2000` |
| `80008C00`–`80018BFF` | 128 | URKW | process 1's P0 page table: P0BR `80008C00`, P0LR `4000` |
| `80018C00`–`80020BFF` | 64 | URKW | process 1's P1 page table: P1BR `7F820C00`, P1LR `1FE000` |
| `80020C00`–`800233FF` | 20 | URKW | the kernel stack: KSP `800233FC` |
| `80023400`–`800235FF` | 1 | NA | a guard page |
| `80023600`–`800245FF` | 8 | EW | the executive stack: ESP `800245FC` |
| `80024600`–`800247FF` | 1 | NA | a guard page |
| `80024800`–`800257FF` | 8 | SW | the supervisor stack: SSP `800257FC` |
| `80025800`–`80025FFF` | 4 | URKW | the interrupt stack: ISP `80025FFC` |
| `80026000`–`800261FF` | 1 | URKW | `CONSOLE$SCRATCH`, where RUN builds the procedure that starts an image (section 11) |
| `80026200`–`800263FF` | 1 | URKW | the first page of shim stubs (section 10) |
| `80026400`–`800265FF` | 1 | URKW | the system control block: SCBB `00026400` |
| `80026600`–`800275FF` | 8 | UW | the string pool: string literals the console and debugger turn into descriptors |
| `80027600`–`800289FF` | 10 | URKW | the microkernel, which STARTUP.COM's ASM puts here |
| `80028A00`–`803FFFFF` | 7867 | URKW | the S0 pool |

A stack grows down, so each stack pointer starts at its region's last
longword, and a guard page below the executive and supervisor stacks makes
an overflow fault rather than write over what's below. ISP, like the
others, is an S0 virtual address: the interrupt stack is mapped as every
other stack is, and taking it doesn't change MAPEN.

The S0 pool (`corevms/s0pool.go`) is where every later process's
system-space structures come from: its page tables, its kernel,
executive, and supervisor stacks, and a page for its hardware PCB. An
allocation is a run of whole pages, first fit from the pool's bottom,
recorded with the process it belongs to and what it's for, so that SHOW
MEMORY can list them and a process's deletion can give them all back. A
run handed out is cleared, and its pages' protection set back to URKW, so
that a stack's guard page doesn't follow its page to the next owner. At these sizes a
process needs 231 pages (a 192-page table area, 38 stack pages, and the
PCB's), so 34 processes fit beside process 1. The microkernel's own pages
are claimed from the pool's bottom as ASM deposits them (`ClaimS0`).

With paging on, INITIALIZE/MEMORY's regions cost virtual space, not
frames, and it sizes them for processes (`P0` 19456, `P1` 5120, `S0`
16384 pages, unless the configuration says otherwise): only its own
layout's pages get frames, and an S0 pool page has none (its PTE all
zeros, no access) until the pool hands it out. Section 6 has the rest.

Where govax's layout differs from VMS's, it's for the sake of a machine
that is also a console:

- VMS keeps each process's kernel, executive, and supervisor stacks in its
  P1 space. govax keeps process 1's where INITIALIZE/MEMORY lays them out
  and every other process's in the S0 pool, mapped always, so a stack
  never faults.
- The kernel stack is URKW, where VMS's is kernel-only. When the console
  CALLs code in kernel mode, it builds the call frame on the kernel stack,
  and code that drops to user mode with REI, as many of govax's test
  programs do, reads that frame with its last RET. (RUN, after the
  microkernel's initialization, is in user mode, and builds its frame on
  the user stack; section 11.10.)
- There's no guard page below the kernel and interrupt stacks.
- S0 holds console structures VMS doesn't have: the scratch page, the
  shim pages, the string pool.

### 5.6 Process space: P0 and P1

Process 1's P0 and P1 page tables are the ones INITIALIZE/MEMORY built.
Every other process gets a *table area* from the S0 pool
(`System.BuildAddressSpace`, `corevms/addrspace.go`), laid out as VMS lays
out its own: P0's table at the area's bottom, growing up, and P1's at its
top, growing down, so that either region may grow into the room between
them (with paging on; with it off, each region keeps the size it was made
with).

Every page of a new process starts as `ProcessPTE` makes it: invalid,
with a frame of 0 (demand zero, section 5.7), UW, and owned by user mode.
P0's page 0 is the exception: no access, and owned by kernel mode, so that
a program following a null pointer faults rather than reading whatever is
at address 0.

P1, from STARTUP.COM's sizes, with paging off:

```text
  7FFFFFFF  ┐
            │  unused
  7FFEDE00  ├─ the P1 system service vector: a few pages, URKW, owned by
            │  kernel mode, shared by every process
            │  unused
  7FE00000  ├─ the user stack's top, every process's USP; the stack grows
            │  down, with 1024 pages kept for it
  7FD80000  ├─ P1 expansion ($EXPREG of region 1, SEC$M_EXPREG sections)
            │  grows down from here
  7FC00000  ┘  P1's low end: P1LR is 1FE000
```

The system service vector is the one piece of P1 every process shares.
The microkernel's `.P1VECTOR` directive writes, at each service's fixed
VMS address, a tiny procedure: an entry mask of 0, `XFC #^X7A`, and RET
(section 10). Those pages are deposited once, in process 1, and every
later process's P1 maps the same frames (`ShareP1`, `mapShared`), so a
service called from any process runs the same three instructions. In
every process they're URKW and owned by kernel mode, as VMS maps its P1
system service vectors (*VAX/VMS Internals and Data Structures*, Table
26-4b), so no user-mode program can change them or delete them. With
paging on they're system pages, never paged.

VMS's P1 holds much more: the inner-mode stacks, the image I/O segment,
the command language interpreter's data, the channel table, the
per-process common area. In govax those are in S0 (the stacks) or in Go
(channels, RMS's internal structures, DCL), and a program can't tell, so
long as it reaches them through the services, as a VMS program must.

### 5.7 When a page isn't valid: the pager hook

When a translation finds a valid bit clear (after the protection check,
which an invalid PTE still answers), the page has to be brought in. The
invalid PTE says where it is, in one of the forms *VAX/VMS Internals and
Data Structures* gives (14.1.1), told apart by two bits, bit 26 (TYP0,
the modify bit's place) and bit 22 (TYP1, one of the software bits):

| TYP1 | TYP0 | Bits 21–0 | The page is |
| --- | --- | --- | --- |
| 0 | 0 | frame 0 | demand zero: its first touch gives it a frame of zeros |
| 0 | 0 | a frame | in transition: out of the working set, but still in that frame (paging on only) |
| 0 | 1 | a process section table index | a page of a section of a file this process mapped: the index names the file and block |
| 1 | 0 | a global page table index | a page of a global section, shared with other processes |
| 1 | 1 | a page file block | in the page file, at that block (paging on only) |

`vm/pager.go` reads and makes each form (`SectionPTE`, `GlobalPTE`,
`PageFilePTE`, `TransitionPTE`, `ValidPTE`, and the tests `SectionIndex`,
`GlobalIndex`, ...).

The hardware's part ends at the fault; what a page fault *does* is the
operating system's. In govax it's the `vm.Pager` seam (section 2.5):

```go
type Pager interface {
    // PageIn returns the PTE the faulting page should now have, valid
    // and pointing at a frame holding the page's contents, or false if
    // it can't be brought in (the access then faults).
    PageIn(f PageFault) (PTE, bool)
}
```

`Memory.pageIn` hands the fault to the installed pager, and writes the
PTE it returns back to the page table before the translation goes on. So
the instruction that touched the page never sees a fault: the page is
brought in between the walk's step 6 and step 7, and the access completes
as if the page had always been there. A real VAX takes the fault, VMS's
pager runs, and the instruction runs again; govax does the same work
without the round trip, since its pager is Go code that needs no
instructions to run. (A page the pager can't bring in, a section page it
can't read, say, is a real translation-not-valid fault.)

The pager is `corevms.System`, installed by each new `System`
(`corevms/system.go`); its `PageIn` is in `corevms/filesec.go`. With
paging off it does three things, by the PTE's form:

- **Demand zero:** take a free frame (which reads as zeros) and make the
  PTE valid with it. With paging off any invalid PTE that isn't a
  section's or a global page's is taken as demand zero.
- **A private section page:** read the page's block of its file into a
  new frame. `$CRMPSC` of a file opened with RMS's user file open leaves
  each page's PTE in the section form, pointing at an entry of the
  process's section table (the file, its first block, the first page);
  the first touch reads the block. A modified page is written back when
  it's unmapped or by `$UPDSEC`.
- **A global page:** find the section's page in the global page table,
  reading it from the section's file the first time any process touches
  it (or giving it zeros, for a section in the page file); after that
  every process mapping the section shares the frame.

Once in, with paging off, a page stays until it's deleted: nothing takes
it away. With paging on the pager does a great deal more (working sets,
soft faults, the page file, read clustering), and section 6 is about that.
Without a pager at all, as in `vm`'s own tests, `validatePage` treats every
invalid page as demand zero.

One more hook sits beside the pager: the *stack expander*
(`SetStackExpander`). A user-mode access just below P1's low end, a
stack growing past its pages, is a length violation, unless the expander
lowers P1LR to cover it first. That's paging's created address space,
section 6.

### 5.8 What's VMS's and what isn't

The hardware here is the VAX's: the PTE, the protection codes, the
translation walk and its order of checks, the faults and their
parameters, the TB's invalidation by TBIA, TBIS, and process switch. The
system above it is shaped like VMS's but simpler, and these are the
differences a program could notice:

- **No S1 region.** No VAX used it either.
- **The inner-mode stacks are in S0**, mapped always, and the kernel
  stack is readable by user mode (section 5.5).
- **S0 holds the console's own pages**, and the old system service vector
  at its start.
- **A page fault completes the access** rather than restarting the
  instruction (section 5.7). A program can't see the difference, unless
  it counts faults: those are counted as VMS counts them, by the pager.
- **The caches are govax's sizes.** A real VAX's TB size varies by model;
  only a program timing itself could tell.

## 6. VMS paging

Section 5 ended with a page brought in on its first touch and kept until
it's deleted. That's all memory management needs while every process's
pages fit in physical memory, and it's how govax runs by default. VMS
does more: it gives each process a *working set*, a limited number of
pages it may hold in memory at once, and when a process touches a page
past that limit, VMS takes another of its pages away, keeping its
contents somewhere they can be fetched back from. So a program can use
more memory than its share of the machine, and many programs can share a
small one. This section is that machinery as govax builds it, from
*VAX/VMS Internals and Data Structures* (chapters 14 to 17, cited here as
"the book"), and checked against a VMS 7.1 VAX where a probe could show
the answer.

All of it is under one switch, and the subsections follow a page's life:
the record kept for each frame (6.2), the working set it lives in (6.3),
the fault that brings it in and the step that takes it out (6.4), the
writer that saves its contents and the file it saves them in (6.5), the
reclaimer that keeps frames free (6.6) and the quota that bounds the
file's use (6.7). Then the larger structures built on these: global
sections (6.8), reading several pages at once (6.9), images mapped rather
than copied (6.10), and address space that exists only where it's been
created (6.11). The last subsection lists what govax leaves out.

### 6.1 The switch

`vax.sysgen.paging.enable` turns paging on; it's off unless set.
INITIALIZE/MEMORY reads it (or its `/PAGING` and `/NOPAGING` override it),
and the choice lasts until the next INITIALIZE/MEMORY. With paging off
nothing in this section happens, and everything a program sees is section
5's. With paging on:

| | Paging off | Paging on |
| --- | --- | --- |
| Frames | a bitmap, lowest free first, cleared when freed | the PFN database: free and modified lists, each frame's owner and backing store (6.2) |
| A process's pages in memory | every valid page, until deleted | its working set, at most its limit (6.3) |
| Invalid PTE forms | demand zero, section, global | those, and transition and page file (5.7) |
| Modified pages | written back only when unmapped or by `$UPDSEC` | written by the modified page writer, to their files or the page file (6.5) |
| Running an image | copied into P0 | mapped: read page by page as it's touched (6.10) |
| P0LR and P1LR | the tables' full lengths, fixed | the pages that exist; services lengthen and shorten them (6.11) |
| INITIALIZE/MEMORY's regions | S0 fits in physical memory, every S0 page mapped | sized for processes; only its own layout's S0 pages have frames |

Paging's tuning comes from VMS's SYSGEN parameters. govax has no SYSGEN
command: each parameter is a configuration key named for it in lowercase
(`internal/machine/sysgen.go` reads them into a `corevms.Sysgen`, which
each new `System` gets with `SetSysgen`):

| Key `vax.sysgen.` | Default | What it governs |
| --- | --- | --- |
| `minwscnt` | 20 | the smallest working set limit (6.3) |
| `pql_dwsdefault`, `pql_dwsquota`, `pql_dwsextent` | 294, 588, 16400 | a created process's working set default limit, quota, and extent when its quota list names none; set, they're laid over process 1's account values too (512, 1024, 16400) |
| `pql_dpgflquota` | 16400 | the same for a job's paging file quota (process 1's account: 40960) (6.7) |
| `growlim` | 63 | past its quota, a working set grows only while more frames than this are free (6.3) |
| `tbskipwsl` | 8 | how many pages still in the TB replacement passes over (6.3) |
| `freelim`, `freegoal` | 32, 200 | the reclaimer wakes below the first and works up to the second (6.6) |
| `mpw_hilimit`, `mpw_lolimit` | 500, 32 | the modified page writer's thresholds (6.5) |
| `mpw_wrtcluster` | 120 | the most pages the writer writes at once (6.5) |
| `paging.file`, `paging.size`, `paging.defer` | the system volume's, 32768, false | the page file: its name, its size in blocks, and whether to wait for the first need to open it (6.5) |
| `gblsections`, `gblpages`, `gblpagfil` | 128, 4096, 4096 | the global section limits (6.8) |
| `pfcdefault` | 32 | how many pages a hard fault reads at once (6.9) |
| `virtualpagecnt` | P0 plus P1's sizes | a new process's P0 and P1 pages together (6.11) |

`paging.enable`, `paging.file`, `paging.size`, and `paging.defer` are
govax's own; VMS has no parameter for them. A `corevms.Sysgen`'s values
that start a process off (its working set sizes, its job's quota) are
copied into the process when it's made, so SET WORKING_SET changes one
process only.

One choice runs through all of it: govax's paging is **synchronous**. On
VMS a page read or write is an I/O request, and the faulting process
waits for it in the scheduler (the PFW state, "page fault wait"); the
modified page writer and the swapper are processes of their own. In govax
every page read and write completes inside the operation that needs it,
and the writer and the reclaimer are Go routines called at the right
moment. No process ever waits for paging, and nothing a program can see
depends on how long a disk takes.

### 6.2 The PFN database

VMS keeps a record for every frame of physical memory, indexed by its
frame number: the *PFN database* (the book's 14.2). govax's is
`paging.Database` (`internal/paging/database.go`), and with paging on it
is the frame allocator section 5.1 mentioned (`vm/frames.go`'s
`pagingFrames`). It's kept entirely in Go: VMS's lives in system memory,
but nothing a VAX program can do depends on its layout, so govax's is a
slice of records. Each record, a `paging.Entry`, says:

- **State**: `Active` (in use), `Free` (on the free page list), or
  `Modified` (on the modified page list).
- **Type**: a `ProcessPage` (a page of some process's P0 or P1), a
  `GlobalPage` (a global section's page, 6.8), or a `SystemPage` (one
  that's never paged).
- **Owner**, while the frame is *tied* to a virtual page: which page.
  For a process page, the process's PID and the page's address; for a
  global page, the section's serial number and the page's offset in it.
  No two frames of one type have the same owner.
- **Backing**: where the page's contents come from and go back to. Its
  `Kind` is `NoBacking`, `PageFile` (with a block, or 0 for "not
  written yet"), `SectionFile` (a process section table index and the
  file's block), or `GlobalSection` (a global page table index).
- **RefCount**, how many reasons there are to keep the frame active (a
  working set holding the page is one), and for a global page
  **ShareCount**, how many process PTEs are valid on it.
- **SavedModify**: the frame's contents differ from its backing store.
  The modify bit lives in each PTE, and a frame's PTE can come and go;
  the saved bit collects it, so the frame knows it must be written.

The two lists thread through the database (VMS's FLINK and BLINK arrays,
here two slices), so a frame can leave from the middle of a list in
constant time. A frame's life is five moves, each a method:

| Move | From | To | When |
| --- | --- | --- | --- |
| `Allocate` | the free list's head | active | a page needs a frame (only the free list gives frames: a modified one must be written first) |
| `Release` | active | the modified list's tail if its saved modify bit is set, else the free list's tail | the last reason to keep it goes, as when its page leaves a working set |
| `Reclaim` | either list | active | its page, in transition, is touched again: a soft fault, no I/O |
| `Cleaned` | the modified list | the free list's tail | the writer has written it |
| `Delete` | active, or either list | the free list's head | its page is deleted |

A frame `Release` sends to a list *keeps its contents and its tie*, and
a process page's PTE takes the transition form (5.7), naming the frame
(a global page's is different, 6.8). Only
`Delete` unties a frame, and puts it at the head, to be used first, since
nothing will ever fault it back.

So a frame released waits on the free list as long as it can, behind
every frame released before it, and the page it holds can be faulted back
cheaply all that time. When `Allocate` finally reuses it, the frame is
still tied to its old page, and that page's PTE must stop naming it. The
database can't do that itself (it knows no PTEs), so it calls a
`TieBreaker`, which is `corevms.System.BreakTie`
(`corevms/workingset.go`): it gives the old page's PTE the form that says
where the page's contents are now (`backingPTE`), a section table index,
a global page table index, or a page file block, so that the page's next
touch reads it back. A frame is cleared as it's handed out, unless the
caller fills it (`vm.FrameClaim.Fill`, for a page read from a file), since
on these lists frames keep their contents.

Every frame is claimed with a `vm.FrameClaim` saying whose page it is
(`corevms/frames.go`). Frames that are never paged are system pages, tied
to nothing: the S0 pages INITIALIZE/MEMORY lays out (`Reserve`, frame
`n` for S0 page `n`), the S0 pool's pages as the pool hands them out, the
pages of every process's page tables, and the frames of the P1 system
service vector, which every process maps (`MakeSystem` turns them into
system pages once process 1 has made them). So page tables never fault:
6.12 says how that differs from VMS.

`Database.Check` verifies the database's rules (each list's links agree
both ways, each frame is in exactly one place, no two tied frames share
an owner, the counts match), and the tests run it after every operation.

### 6.3 Working sets

A process's working set is the pages it holds valid in its page tables.
VMS keeps it as a list, the *working set list*, whose length is the
*working set limit*; govax's is `paging.WorkingSet`
(`internal/paging/workingset.go`), one per process, made at its first
use (`Environment.WorkingSet`, `corevms/workingset.go`). Each entry names
a page by its address and says whether it's a global page; the list
holds no frames or PTEs.

The list has two parts:

```text
  entries 0 .. dyn-1      locked pages ($LKWSET, $LCKPAG): never replaced
  entries dyn .. end      the dynamic part, a circular buffer
                          next = the entry filled most recently
                          next+1 (wrapping to dyn) = the oldest page
```

When a fault brings a page in, the pager calls `WorkingSet.Add` first.
If the list has an available entry the page may take, it takes it; if
not, the list *replaces* a page, which then leaves the working set. The
rules are the book's (15.4), "modified first-in, first-out", and `Add`
applies them in order, scanning from the oldest entry:

1. **Growth.** An available entry takes the page only if the set holds
   fewer pages than its limit and either it's below its *quota* or the
   free list holds more than GROWLIM frames. So a process grows past its
   quota toward its limit only while memory is plentiful.
2. **Pages the instruction is using are passed over** (below).
3. **Pages still in the translation buffer are passed over**, up to
   TBSKIPWSL of them: a page the TB still holds was used recently, and is
   worth keeping. Only for the process the CPU is running; the TB holds
   no one else's pages.
4. The first entry not passed over is reused, its page removed.

Three sizes bound the list (`paging.Bounds`): the **quota**, WSQUOTA,
above which growth needs free memory; the **extent**, WSEXTENT, the
largest the limit may be; and the **minimum**, MINWSCNT, the fewest
unlocked entries it keeps. The limit itself starts at WSDEFAULT.
`$ADJWSL` moves it within those bounds (`setWorkingSetLimit`,
which calls `WorkingSet.Resize`: a smaller limit removes the oldest pages
over it), image exit puts it back (`resetWorkingSet`, the book's
MMG$IMGRESET), and DCL's SET WORKING_SET sets the default, quota, and
extent within what the process is authorized (`setworkingset.go`).
`$LKWSET` and `$LCKPAG` move a page into the locked part (`Lock`, which
fails with `SS$_LKWSETFUL` if fewer than MINWSCNT unlocked entries would
be left), `$ULWSET` and `$ULKPAG` move it back, and `$PURGWS` removes a
range of pages (`RemoveRange`), all in `corevms/pageprot.go`.

**Instruction safety.** One VAX instruction can touch several pages: a
MOVC3 has a source and a destination, and a service written in Go may
translate an address, translate another, then copy between them. If the
second translation's fault took away the first page's frame, the copy
would go through a physical address that now holds another page. So `vm`
remembers which frames have been translated since the current
instruction began (`Memory.TranslatedThisInstruction`, a generation
number per frame that `BeginInstruction` moves on), and the working set
passes over such pages (`wsProtected`). If every page is in use that
way, as when the image activator touches a hundred pages in one call,
the list grows past its limit rather than replace one
(`WorkingSet.Over`), and comes back down at the next instruction
boundary: `enterWorkingSet` asks `Memory.AtNextInstruction` to call
`System.atInstructionBoundary`, which trims each such list
(`trimWorkingSets`).

### 6.4 A fault and an eviction

The pager is still `System.PageIn` (`corevms/filesec.go`), the `vm.Pager`
of section 5.7, and its order matters:

1. **Enter the working set** (`enterWorkingSet`). If the list is full, a
   page leaves it now (`evictPage`, below), before a frame is taken, as
   VMS's pager makes room first: a clean page leaving gives its frame to
   the free list, where the faulting page may take it.
2. **Resolve the fault** by the PTE's form (`resolveFault`):
   - **transition**: a *soft fault* (`softFault`). The frame is still
     tied to this page, on the free or modified list; `Reclaim` takes it
     back. No I/O.
   - **global page table index**: the section's page (6.8).
   - **process section table index**: read the block from the section's
     file into a new frame, its backing store that block. A
     copy-on-reference section's page instead becomes the process's own
     from this read: its backing store is the page file, and its saved
     modify bit is set, since its contents now exist only in memory.
   - **page file block** (not 0): a hard fault that reads the block back.
   - otherwise **demand zero**: a frame of zeros, backed by the page file
     and *effectively modified* (`effectivelyModified`), so that whenever
     it leaves the working set it goes to the modified list, to be saved.
3. **Count it** (`countFault`): in the process's `Faults`, by kind (free
   list, modified list, global valid, demand zero, read), and in
   `JPI$_PAGEFLTS`'s total, and in the system's `PageCounts`.
4. **On a hard fault, read the neighbors too** (`readCluster`, 6.9).

A fault the pager can't resolve (no frame free, a file that can't be
read) takes the page back out of the working set and is a real
translation-not-valid fault; the first such fault with no frame left
tells the console `%GOVAX-F-MEMEXHAUST`, with why the modified pages
couldn't make room (`noteFault`).

**Eviction** (`evictPage`) is a page leaving the working set while its
process keeps it. Its PTE becomes the transition form, keeping the frame
number, which also empties the page's TB entry; then
`Database.Release`, with the PTE's modify bit, sends the frame to a list.
A frame sent to the modified list may make three more things happen: a
page file block the page held goes back, since the block no longer
matches (`forgetPageFileBlock`); a read-only image page the debugger
wrote takes the page file as its backing store, so the change isn't lost
(`leaveReadOnlySection`); and the modified page writer may wake (6.5). A
global page leaves differently (6.8).

Here is one page's life, measured on govax with paging on and small
settings so that everything happens soon: a working set limit of 30,
MPW_HILIMIT 10, MPW_LOLIMIT 2, MPW_WRTCLUSTER 16, a 1000-block page file.
A program has an 80-page array in a demand-zero psect, at `00000200`; it
writes the first page, then the other 79, then reads the first page back
(every number hexadecimal):

```text
  before the touch    PTE 21800000   invalid, UW, owner user, frame 0:
                                     demand zero (made at activation)
  written             PTE A58000FD   valid, modified, frame FD
                      frame FD: active, process page of PID 301 at 200,
                      backing page file block 0, saved modify set
                      (a demand-zero fault)
  79 pages later      PTE 218000FD   transition, frame FD
                      frame FD: on the free list, backing page file
                      block 9, saved modify clear
                      (it left the working set for the modified list;
                      the writer, woken past MPW_HILIMIT, wrote it to
                      block 9 in a cluster and moved it to the free list)
  frame FD reused     PTE 25C00009   page file form, block 9
                      (BreakTie, as Allocate took frame FD for another page)
  read back           PTE A18000CB   valid, not modified, frame CB
                      frame CB: backing page file block 9, saved modify
                      clear (a hard fault: one read)
```

Had the program touched the page between "79 pages later" and the
frame's reuse, the fault would have been soft: frame FD back off the
free list, no read. And the read back is clean: if the page leaves the
working set unmodified, its frame goes to the free list, since block 9
still holds its contents.

### 6.5 The modified page writer and the page file

A frame on the modified list can't be reused until it's written. VMS's
*modified page writer* writes them (the book's 15.5.2); govax's is
`corevms/mpw.go`. It wakes when the list holds more than MPW_HILIMIT
frames (`wakeModifiedPageWriter`, called by `evictPage` and the global
page's equivalent) and writes, oldest first, down to a cluster below
MPW_HILIMIT, never below MPW_LOLIMIT (`mpwGoal`). That stopping point is
VMS 7.1's as a probe measured it (`testdata/pg64`): the book and the
SYSGEN manual say the writer writes down to MPW_LOLIMIT, but the VAX
stopped one cluster below where it woke. Writing for the reclaimer (6.6)
goes down to MPW_LOLIMIT. Each frame written moves to the free list
(`Cleaned`), still holding its page.

Where a frame goes depends on its backing store:

- a **writable private file section**'s page (`$CRMPSC` with `SEC$M_WRT`,
  not copy-on-reference): back to its block of the file
  (`writeSectionCluster`);
- a **writable global file section**'s page: back to its block of the
  section's file (`writeGlobalCluster`);
- **everything else**, a demand-zero page, a private copy of a
  copy-on-reference page, a page-file section's page: to the **page
  file**, to a block allocated as it's written (`writePageFileCluster`).
  The block becomes the frame's backing store, and the PTE's when the
  frame is reused.

**Clusters.** One write takes up to MPW_WRTCLUSTER pages (the book's
15.5.2.2): a frame from the list, with its neighbors in the process's page
table, first below it, then above, as long as each is in transition in a
frame waiting on the modified list bound for the same place (a
*minicluster*, `miniCluster`). For the page file the writer first
allocates a run of blocks and fills it with as many miniclusters as fit,
taking each next one from the next frame on the list; blocks left over go
back.

**The page file** (`corevms/pagefile.go`) is a file of
`vax.sysgen.paging.size` blocks, one page per block: the file
`vax.sysgen.paging.file` names, on a mounted volume or the host, or with
none named the system volume's `[SYSTEM.SYSEXE]PAGEFILE.SYS`, VMS's
`SYS$SYSTEM:PAGEFILE.SYS` (`pageFileName`). As on VMS it's installed when
the system starts (`InstallPageFile`, which the console calls once
STARTUP.COM has run): created if missing, made the configured size if it
isn't, and held so no one else may read or write it, which also keeps its
volume from being dismounted. With `paging.defer` it's opened instead at
the writer's first need. Either way it's opened only at an instruction
boundary (`openWantedPageFile`), never inside a fault: creating a file
changes a volume's bitmap and directory, which an RMS service the fault
interrupted may be changing too. Its blocks are read and written straight
through the volume (`rms.PageFileStore`), never through RMS's record
layer, and never extend the file, so that a page fault in the middle of
an RMS call can use it.

Which blocks are in use is `paging.PageFileMap`
(`internal/paging/pagefile.go`), a bitmap kept in Go and empty each
session, with the book's allocation rule (14.5): the writer asks for a run
of contiguous blocks, at most the current *allocation size* (MPW_WRTCLUSTER
to begin with); the map takes the first free run that long from the
file's start, and when there's none, drops the size by 16 and looks again;
at 16 it takes the first free run however short. Freeing blocks that
leave a run long enough grows the size back by 8. When the file first
passes 65% and 90% full the console is told, `%SYSTEM-W-PAGEFRAG` and
`%SYSTEM-W-PAGECRIT`, as VMS tells the operator.

A page's block goes back when the page is deleted (`releasePageFileBlock`,
from `releasePTE`) or when it's written again after being read back
(`forgetPageFileBlock`, 6.4).

A page bound for the page file when there is none, or when it's full,
stays on the modified list until it's faulted back or deleted. VMS's
writer would wait for space; govax's tells the console once
(`%GOVAX-W-PAGEFILEFULL`) and remembers how many frames it couldn't write
(`mpwStuck`), so that it wakes again only when the list is that much past
MPW_HILIMIT, rather than walking the list in vain at every eviction.

### 6.6 The reclaimer

Every new page takes a frame from the free list, and pages leaving
working sets give frames back only when their contents are safe. If
processes keep their pages, the list runs down. On VMS the swapper
watches it: when the pager leaves fewer than FREELIM frames free, the
swapper wakes and works to bring the list back to FREEGOAL (the book's
17.1.1). govax's *reclaimer* (`corevms/reclaim.go`) does two of the
swapper's four things:

1. write modified pages down to MPW_LOLIMIT, so their frames move to the
   free list (`reclaimWrite`);
2. trim working sets: each process whose set holds more pages than its
   quota, which it grew past while memory was plentiful, loses its oldest
   pages down to the quota (`trimToQuota`, `WorkingSet.Shrink`), in the
   order of the book's selection table: suspended processes first, then
   those waiting for an event flag or hibernating, the computable ones
   last, and the process the CPU is running last of all
   (`reclaimOrder`);

then writes again, stopping whenever the free list reaches FREEGOAL. The
other two, outswapping process headers and whole processes, need a
swapper govax doesn't have.

The frame allocator wakes it (`vm.Memory.SetFreeLimit`): an allocation
that leaves fewer than FREELIM frames free calls `wakeReclaimer`, as does
a soft fault that takes a frame off the free list. It doesn't run there,
in the middle of a fault, but at the next instruction boundary, through
`Memory.AtNextInstruction`, where nothing is half-translated. One routine
serves the boundary for everything the pager defers:
`atInstructionBoundary` trims working sets left over their limits, opens
a wanted page file, and runs the reclaimer if the free list is low.

### 6.7 The paging file quota

A page whose contents may one day have to go to the page file (a
demand-zero page, a private copy of a copy-on-reference page) is charged
against its job's PGFLQUOTA when it's *created*, not when it's touched
(the book's 16.2.1.1), and the charge is given back when it's deleted
(`corevms/pgflquota.go`). The quota is pooled: every process of a job
draws on one count, `$GETJPI`'s `JPI$_PAGFILCNT` being what's left. A
service that creates pages (`$CRETVA`, `$EXPREG`, the heap's growth of
P0) creates them one at a time, charging each, and stops with
`SS$_EXQUOTA` when the quota runs out, keeping the pages before it. A
copy-on-reference section's pages are charged when they're mapped, an
image's writable and demand-zero sections and its user stack when it's
activated (6.10). Since every page that can hold contents only memory
holds is made one of these ways, a fault never charges.

govax keeps each process's charged pages as a set by address, so no page
is charged twice, and the job's total (`Job.PageFileCharged`). Image
rundown and `$DELTVA` refund through `releasePTE`, a process's deletion
refunds the rest (`refundAllPageFile`), and the most a process has had
charged is its peak, LOGOUT/FULL's "Peak page file size". With paging
off, nothing is charged.

### 6.8 Global sections under the pager

A global section is pages several processes map together (section 7 has
the services). With paging on, a page of a global file section, or of a
page-file section, is in one of three states, which the section's entry
in govax's global page table records (`corevms/globalpages.go`; the
book's 14.3.3):

- **valid**, in one or more processes' working sets: its frame is
  active, its share count the number of process PTEs valid on it;
- **in transition**: no working set holds it, but its frame is on the
  free or modified list, tied to the *global* page;
- **out**: no frame. Its contents are in its block of the section's file,
  or for a page-file section in a page file block, or nowhere yet: a
  page-file section's page never written is a global demand-zero page.

A process PTE for a global page holds the global page table index (GPTX)
until the process faults it, and then the frame, valid. When the page
leaves that process's working set its PTE gets the GPTX back, never a
transition form, since the frame isn't the process's; the process's
share goes (`Unshare`), its modify bit gathered into the frame's saved
modify bit, and the last share's going sends the frame to a list
(`evictGlobalPage`). So every fault on a global page comes through its
GPTX (`pageInSharedGlobal`): one valid elsewhere is a soft *global valid*
fault that adds a share; one in transition is a soft fault off its list;
one out is a hard fault, or for a page-file section's unwritten page a
demand-zero one. A copy-on-reference global section's fault gives the
process a private copy instead (6.4).

When a global page's frame is reused, its tie is broken in the global
page table (`loseGlobalFrame`); no process PTE changes, since none names
the frame. The writer writes a global file section's modified frames to
the file, a cluster of the section's successive pages at a time, and a
page-file section's to the page file. `$UPDSEC` follows the global page
table too: with UPDFLG 0 it writes every page of the section that has a
frame, modified or not, as VMS 7.1 did, and with UPDFLG 1 only the
modified ones (`updateGlobalPage`).

A page-file section made with paging on has no frames when it's made;
its pages count against GBLPAGFIL rather than the creator's quota. Every
section uses GBLPAGES at a page per page plus 2, rounded up to an even
number, and GBLSECTIONS one slot, in both paging modes. A deleted
page-file or copy-on-reference section leaves the name table at once but
keeps its entries, frames, and blocks on a pending list until the next
global section is created or an image runs down
(`ReleasePendingSections`), as VMS 7.1 was seen to hold them
(`testdata/pg65`).

### 6.9 Reading more than one page

Most of a disk read's cost is starting it, and programs tend to touch the
pages next to the one they just touched. So after a hard fault the pager
reads some neighbors with it, each put in the working set, valid
(`corevms/cluster.go`; the book's 15.5.1):

- **The scan.** Up from the faulting page first, adding each page in the
  same backing store, until the cluster is full or a page doesn't match;
  only if that added nothing, down by the same rule.
- **What matches.** A private section's page: a PTE holding the same
  process section table index. A page file page: a PTE holding the next
  block (the one before, scanning down). A global page: the next GPTX of
  the same section, for a page that's out.
- **How many.** The section's own cluster size (`$CRMPSC`'s pfc
  argument, or for an image section LINK's `CLUSTER=` option), or when
  that's 0, PFCDEFAULT; for the page file and page-file sections,
  PFCDEFAULT. The count includes the faulting page, and is at most 127.
- **Where it stops early**: a neighbor needs an available working set
  entry it may take by the growth rules (`WorkingSet.AddAvailable`); the
  cluster never takes a page out to make room. Nor does it read with no
  free frame.

So clustering helps a process whose working set has room. In 6.4's
example, the read back of the first page read nothing more, though its
three neighbors' PTEs held blocks A, B, and C: the working set was full.

A cluster is one fault: `JPI$_PAGEFLTS` counts one, `PageCounts.Faults.Read`
one read, and `PageCounts.PagesRead` every page read.

### 6.10 Images mapped, not copied

With paging off, RUN copies an image's sections into P0. With paging on
the image activator maps it, as VMS's does (the book's 21.1.1.3;
`machine/imagemap.go`'s `imageMap`):

- The image file is opened as a *page source*
  (`corevms/pagesource.go`): a volume file accessed through RMS with
  sharing for readers only (`rms.ACPFile`), or a host file
  (`HostPageSource`). It's held while any of its pages is mapped, so a
  program that tries to write a running image's file gets `RMS$_FLK`.
- The header is decoded in Go (`vmsimage.ReadImage`) and never written to
  P0.
- Each process-private section descriptor (ISD) becomes a private
  section of the image file (`Environment.MapImageSection`,
  `corevms/imagesec.go`), its PTEs holding the section's index. A
  writable section is copy-on-reference, charged against the quota as
  it's mapped; a read-only one stays backed by the file. An image
  section never writes back to the image file, and a block past the
  file's end reads as zeros (`zeroPastEnd`).
- Each demand-zero ISD becomes demand-zero pages
  (`CreateImagePages`), charged as `$CRETVA`'s are.
- The user stack is created below `UserStackTop` from the image's stack
  ISD, LINK's `STACK=` option, 20 pages by default
  (`CreateUserStack`, 6.11).
- The fixups run next, faulting in the pages they read and write; then
  each section gets its own protection, whatever form its PTEs are in.

Nothing else is read: a page of the image comes in when the program
touches it. Shareable images the main image names are mapped the same
way, privately to the process, at P0's high-water mark.

Image rundown (`Environment.DeleteP0`, `corevms/vaspace.go`) deletes
every P0 page, each frame released as `$DELTVA` releases it, and the P1
pages the image added (`deleteImageP1`), including the user stack but for
its live part. The sections go with their last pages, and the image file
with its last section. The working set limit goes back to its default
(`resetWorkingSet`), and global sections the rundown left pending are
released.

### 6.11 Address space that exists only where it's made

With paging off, P0LR and P1LR are set once, to the page tables' whole
lengths, and every page in them exists. With paging on, as on VMS, they
describe the pages that *exist* (the book's 14.1.1 and 16.2;
`corevms/regions.go`): P0LR is the number of P0 pages from page 0 up, and
P1LR the lowest P1 page. A process starts with no P0 pages at all, and a
P1 whose length reaches from `UserStackTop` (`7FE00000`) to the top, of
which only the pages from the P1 vector up exist. Touching a page past either
region's end is a length violation, an access violation with the reason
mask's length bit set.

The services that create pages, `$CRETVA`, `$EXPREG`, `$CRMPSC`, the
image activator, the heap's page pool, lengthen a region to cover each
page before writing its PTE (`coverPage`, `setRegionLength`); those that
delete a region's end pages, `$DELTVA`, `$CNTREG`, image rundown, shorten
it after (`contractRegions`). `setRegionLength` keeps the length in three
places: the process's `ProcessSpace`, the CPU's register while it's
current, and its hardware PCB's (which the next LDPCTX loads) while it
isn't. A page inside a region that nothing created has the all-zero PTE,
no access. `$GETJPI` reports the results as VMS does: `JPI$_FREP0VA`, the
first address past P0; `JPI$_FREP1VA`, the page below P1's low end;
`JPI$_FREPTECNT`, how many more pages the tables can describe; and
`JPI$_VIRTPEAK`, the most pages the process has had.

**The table area.** Each process's page tables are one *table area* in S0
(section 5.6), VIRTUALPAGECNT entries: P0's table from its bottom up,
P1's from its top down, so either region can grow into the room between.
Past that room is `SS$_VASFULL`. Process 1's area, at INITIALIZE/MEMORY's
paging defaults (P0 19456 pages, P1 5120), is 192 pages; a small image
needs perhaps 33 of them. So an area page has a frame only while it holds
an entry of a page that exists (`corevms/tablearea.go`):
`setRegionLength` backs the area pages a growing region reaches *before*
the length register lets the hardware read them, and gives back those a
shrinking region leaves *after*. An area page without a frame has the
all-zero S0 PTE, so a stray reference to it is an access violation, not
a fault. The S0 pool works the same way: a pool page gets a frame when
the pool hands it out, and gives it back when it's freed.

**The user stack** is the image's, as on VMS (`corevms/userstack.go`).
Activation creates its pages; when a program pushes past them, the access
just below P1's low end is a length violation, which `vm` offers to the
*stack expander* first (section 5.7's `SetStackExpander`).
`System.expandUserStack` creates the pages from P1's low end down to the
address (`expandStack`, the book's EXE$EXPANDSTK), each charged against
the quota, and the access goes on. Only a user-mode access expands the
stack, and only a real one: PROBEs and Go code that mustn't
(`ProbeTranslate`) don't. Nothing else limits it but the table area's
room and the quota, as on VMS: a stray user-mode reference far below the
stack creates pages rather than failing. One check stops that from
wasting the room P0 needs: a P1 creation the table area can't hold
whole creates nothing (`spaceHolds`), as VMS 7.1 answered.
`$ADJSTK` of the user stack expands it the same way.

INITIALIZE/MEMORY with paging on makes process 1's tables this way too:
P0LR 0, P1LR `UserStackTop`'s page, and `/P1` must be at least 4116
pages (the 4096 from `UserStackTop` to the top, and a 20-page stack), or
it fails with `P1TOOSMALL`. The console's DEPOSIT and ASM create the
process 1 pages they write (`Environment.EnsurePages`).

### 6.12 What govax's paging leaves out

What's here is VMS's paging as the book describes it and a VMS 7.1 VAX
showed it, but not all of it. A program could notice these:

- **Paging is synchronous** (6.1). No process enters VMS's page fault
  wait (PFW), collided page (COLPG), free page wait (FPG), or the
  modified page writer's waits (RWMPE, RWMPB). A full page file or
  exhausted memory fails a fault rather than making the process wait.
- **No swapper.** Processes are never outswapped, nor their process
  headers; the reclaimer only writes and trims (6.6).
- **No permanent part of the working set.** VMS's working set list holds
  the process header's pages, the page tables, and the kernel stack;
  govax's tables and stacks are system pages, never in a working set, so
  its working sets hold fewer pages than VMS's, and MINWSCNT is the
  floor where VMS's is higher. The probes' expected differences are
  mostly this.
- **Page tables are never paged**, and a table page with no frame is no
  access, not a fault.
- **No automatic working set adjustment.** VMS grows and shrinks a
  process's limit by its fault rate (PFRATL, PFRATH, WSINC, WSDEC);
  govax's moves only by `$ADJWSL`, image exit, and SET WORKING_SET.
- **One page file**, and no swap file.
- **Shareable images aren't installed** as global sections shared between
  processes; each process maps its own copy (6.10).
- **The PFN database, the working set lists, and the page file's map are
  in Go**, not in VAX memory, so a privileged program reading VMS's
  structures directly would find nothing there.
- **A fault completes the access** (section 5.7), with paging on as with
  it off; the counts a program can see are VMS's.

## 7. Processes

govax's VAX has one CPU, and VMS runs many processes on one CPU by taking turns:
each has its own registers, its own P0 and P1, and its own stacks, and
between two instructions the scheduler may put one away and bring
another back. govax does the same with the same pieces: a hardware
process control block per process, the VAX's context-switch semantics,
and VMS's scheduling rules from *VAX/VMS Internals and Data Structures*
(chapter 10, cited as "the book", as in section 6). What's different is
who does the work: VMS's scheduler is VAX code run by a software
interrupt; govax's is Go, called by the engine between instructions, and
so are the services processes use to wait for, signal, and talk to one
another.

The subsections go from what a process is (7.1 to 7.3) to how the CPU
moves between processes (7.4 to 7.6), then what a process can wait for
and be interrupted by (7.7, 7.8), how processes talk (7.9), how they're
made and ended (7.10, 7.11), and the console's place among them: the
terminal they share, and the keys typed at it (7.12, 7.13).

### 7.1 Process 1 and the others

Every govax session has **process 1**, the console's. It's made with the
System (`corevms.NewEnvironment`), so it's the first process in the
table and has PID `00000301`. Its name, user name, and account are
`SYSTEM`; its base priority is 4, VMS's default for an interactive user;
its quotas are the SYSTEM account's (`corevms.SystemAccount`), with the
`vax.sysgen.pql_*` keys laid over them (section 6.1). Its address space
and stacks are the ones INITIALIZE/MEMORY laid out (section 5.5), and its
command interpreter is the console's DCL, which runs in Go between
machine runs. Process 1 is never deleted: an image it runs ends, and its
DCL goes on.

Every **other** process is made by a program or by DCL: `$CREPRC`,
LIB$SPAWN, or DCL's SPAWN (which calls LIB$SPAWN). Each gets an address
space, stacks, and a PCB from the S0 pool, runs an image or a command
interpreter, and is deleted when that ends (7.10).

All of this rests on one switch, `vax.process.scheduler`, on unless set
false. With it off, process 1 is the only process: `$CREPRC` and LIB$SPAWN
return `SS$_UNSUPPORTED`, and a service that must wait does so by running
its XFC instruction again and again (7.6). Two more keys tune it:

| Key | Default | What it governs |
| --- | --- | --- |
| `vax.process.scheduler` | true | whether processes other than process 1 exist |
| `vax.process.quantum` | 20000 | a quantum, in instructions (7.5) |
| `vax.process.preempt` | `all` | the modes in which a process may be preempted: `all` (VMS's rule), `user` (only user-mode code, so a service or supervisor-mode routine always finishes first), or `none` (processes switch only when one waits) |

They're read into a `corevms.ProcessSettings` (`corevms/procsettings.go`)
when the machine is initialized.

### 7.2 The process table and PIDs

VMS finds a process from its process ID through a table of pointers to
process control blocks, indexed by a small number, the *process index*.
The PID carries the index in its low bits and a *sequence number* above
it. Each slot has its own sequence number, which moves on each time a new
process takes the slot, so a deleted process's PID doesn't match the
slot's next occupant: a stale PID is told from a live one by comparing
the whole PID (the book's 20.1.3 and 20.1.4).

govax's table is `corevms/proctable.go`'s `processTable`, a fixed array of
`*Environment` by index. The PID's layout is govax's choice, in the shape
of VMS 7's *extended* PIDs, which pack both fields into 21 bits:

```text
   31          21 20                 8 7          0
  ┌──────────────┬────────────────────┬────────────┐
  │      0       │  sequence (13)     │ index (8)  │
  └──────────────┴────────────────────┴────────────┘
```

Index 0 is the *null process* (the idle loop) and never used, so there
are 255 slots. Every slot's sequence starts at 2, so a slot's first
process gets 3: process 1 is index 1, sequence 3, `00000301`. A new
process takes the next free index **after** the one the last new process
took, wrapping round, as VMS 7.3 did when a probe asked (the book's VMS
searched from the lowest index). Measured on govax, two subprocesses
spawned together, then a third after both had gone:

```text
00000301 SYSTEM          CUR      6 ...
00000302 KID             LEF      4 ...
00000303 KID2            LEF      4 ...
  (both log out)
00000301 SYSTEM          CUR      6 ...
00000304 KID3            LEF      4 ...
```

KID3 took index 4, not the freed 2. `System.FindProcess(pid)` looks the
index up and compares the whole PID; `FindProcessName` finds a process by
name, which is unique only within a UIC group.

**Jobs.** VMS groups processes into *jobs*: a detached process (one with
no owner, such as the one a user logs in to) and every subprocess under
it. A job shares one *job information block*, the JIB, which holds its
master process's PID and the *pooled* quotas all its processes draw on
together (the book's 20.1.1 and table 20-3). govax's is `corevms.Job`
(`corevms/job.go`): `MasterPID`, the subprocess limit and count (the PRCLM
quota: process 1's account allows 10, and an eleventh SPAWN fails with
`SS$_EXQUOTA`), the other pooled quotas, the page file charge of section
6.7, the job's logical name table, and which of its processes has the
terminal (7.12). Process 1 is the master of its own job, and so is every
detached process; a subprocess points at its creator's.

### 7.3 What a process owns

A process is a `corevms.Environment` (section 3.5), and what the CPU
needs to run it is three things, each built by `buildProcessMemory`
(`corevms/creprc.go`) from the S0 pool (`corevms/s0pool.go`), each run of
pages charged to the process's PID so its deletion can give it back:

1. **A table area** (`System.BuildAddressSpace`, section 5.6): its P0 and
   P1 page tables, with the P1 system service vector mapped. With paging
   off, the tables are the size of its creator's, and every entry is
   there; with paging on, the area holds `vax.sysgen.virtualpagecnt`
   entries, backed by frames only where the regions reach (section 6.11).
2. **Its privileged stacks** (`System.BuildStacks`, `corevms/stacks.go`),
   one run of 22 pages:

   ```text
     kernel stack       4 pages   URKW
     guard page         1 page    NA
     executive stack    8 pages   EW
     guard page         1 page    NA
     supervisor stack   8 pages   SW
   ```

   These are INITIALIZE/MEMORY's default sizes, whatever process 1's
   were given (STARTUP.COM gives process 1 a 20-page kernel stack). A
   push past the executive or supervisor stack's bottom lands on its
   guard page and faults. Each process needs its own kernel stack because
   one switched out in the middle of a service leaves its frames there;
   only the interrupt stack is shared, since interrupts belong to no
   process. The user stack is in the process's own P1, with its top at
   `7FE00000` in every process.
3. **A hardware PCB**, a pool page of its own, kernel-only (KW). VMS keeps
   the PCB in the process header beside the page tables; a page apart is
   simpler. The PCBB register holds the PCB's *physical* address, since
   the instructions that use it run with the old process's mapping.

Process 1 starts with no PCB: INITIALIZE/MEMORY's layout has no room for
one, and the pool's first pages are the microkernel's. It gets one the
first time it's switched out (`System.EnsurePCB`).

Everything else a process has is Go: its `Process` record (identity,
quotas, privileges, priorities, event flags, its AST queue, its timers),
its channels, its RMS files and session, its logical name tables, and,
for a process that runs DCL, its DCL state. None of it is in VAX memory,
so a program can reach it only through the services, as a VMS program
must.

### 7.4 Switching the CPU

The VAX defines what a process's registers look like at rest: the
**hardware process control block**, 96 bytes (the *VAX Architecture
Reference Manual*, chapter 6; `cpu/context.go`):

```text
  00  KSP      kernel stack pointer        40  AP
  04  ESP      executive                   44  FP
  08  SSP      supervisor                  48  PC
  0C  USP      user                        4C  PSL
  10  R0                                   50  P0BR
  ..  R1-R11                               54  P0LR (21:0), ASTLVL (26:24)
                                           58  P1BR
                                           5C  P1LR (21:0), PME (31)
```

and gives the scheduler two instructions: SVPCTX saves the running
process's general registers, stack pointers, PC, and PSL into the PCB
PCBB names, leaving the CPU on the interrupt stack; LDPCTX loads a PCB,
with its page table registers, empties the TB's process entries, and
pushes the PC and PSL for the REI that follows. govax has both
instructions, for VAX code that wants them. Its own scheduler, being Go,
uses their Go halves, `Engine.SaveContext` and `Engine.LoadContext`, which
do the same work and LoadContext also the REI's, checking the PSL it
restores by REI's rules.

`System.switchTo` (`corevms/schedule.go`) is the whole switch, between two
instructions:

1. Point PCBB at the outgoing process's PCB (giving process 1 its PCB if
   it has none), and write its page table registers into the PCB with
   `SaveMemoryContext`. SVPCTX doesn't save them, since VMS writes them
   into the PCB whenever it changes them; govax's services change them in
   the CPU's registers, so the switch copies them first.
2. `SaveContext`: registers, stack pointers, PC, PSL. The CPU is now in
   kernel mode on the interrupt stack, belonging to no process.
3. Point PCBB at the incoming process's PCB and `LoadContext`: its
   address space (with the TB's process entries and the instruction-fetch
   window emptied, section 5.4), its registers, and the PC and PSL it
   resumes at.
4. Make it the System's current process, the one `EngineHooks.Running`
   gives (section 3.4): from the next instruction on, an XFC is this
   process's service call.
5. If the outgoing process was deleted, free its memory now that the CPU
   has left its stacks and tables (7.10). If the incoming one is marked
   for deletion, delete it now, in its own context; if it has never run,
   start it (7.10).

Because the switch happens only between instructions, an instruction is
never split by one. That's what makes the VAX's interlocked instructions
(BBSSI, BBCCI, ADAWI, INSQHI, REMQHI, ...) work between processes sharing
a global section: each runs whole.

### 7.5 The scheduler

**The rules** are in `internal/sched`, plain Go with no CPU in it, so they
can be tested alone. A process (a `sched.Handle`, its PID) has a base
priority and a current priority, 0 to 31; 0 to 15 are *normal*, 16 to 31
*real-time*. It's in one **state**, by VMS's own numbers (`sched/state.go`,
the book's Table 10-1), which `$GETJPI` and SHOW SYSTEM report:

| State | Meaning |
| --- | --- |
| CUR | current: the CPU is running it |
| COM | computable: ready, waiting only for the CPU |
| LEF | waiting for one of its own event flags ($WAITFR, a $QIOW, a terminal read) |
| CEF | waiting for a flag in a common event flag cluster |
| HIB | hibernating, until a $WAKE |
| SUSP | suspended, until a $RESUME |
| MWAIT | waiting for a resource; SHOW SYSTEM names which (RWMBX for room in a mailbox, RWAST, ...) |

The book's other states (the outswapped COMO, LEFO, HIBO, and SUSPO, and
the paging waits PFW, COLPG, and FPG) are defined but never entered:
govax doesn't swap, and its paging doesn't wait (section 6.1).

The computable processes wait in 32 first-in, first-out **queues**, one
per current priority. `Reschedule` puts the current process, if any, at
the back of its queue, takes the head of the highest non-empty one, and,
for a normal process above its base, lowers its priority by one first, so
a process that keeps the CPU drifts back to its base (the book's 10.3.3).

When an event ends a wait, `Ready` makes the process computable with a
**boost** by the event's class (`sched/boost.go`, the book's Table 10-3):

| Class | Boost | The event |
| --- | --- | --- |
| PRI$_NULL | 0 | none (a retry, a quantum's end) |
| PRI$_IOCOM | 2 | disk, mailbox, or other non-terminal I/O completing |
| PRI$_RESAVL, PRI$_TIMER | 3 | a resource available, a $WAKE, a $RESUME, a timer |
| PRI$_TOCOM | 4 | terminal output completing |
| PRI$_TICOM | 6 | terminal input completing |

The boost is added to the base priority; the current priority is kept if
it's already higher; and if the result is above 15, the base priority is
used, so a real-time process is never boosted and a normal one never
enters the real-time range. A newly computable process whose priority is
higher than **or equal to** the current one's preempts it (the book's
10.2.3, step 4): the current process goes to the back of the same queue,
behind the newcomer.

A normal process runs until it waits, is preempted, or uses its
**quantum**, after which it goes to the back of its queue, so processes of
equal priority take turns. A real-time process has no quantum end. VMS's
quantum is time; govax's is `vax.process.quantum` instructions, so
scheduling, like the clock in quantum mode (section 4.4), is the same on
every run.

**The engine's hook** (`cpu/schedule.go`) gives the rules their chance.
The engine holds a `cpu.Scheduler`, the System, and a count of
instructions left before it must call it; `Step` counts down, so most
instructions pay a decrement and a compare. When the count runs out, or
something asks for a decision now (`Engine.RequestReschedule`: a process
started waiting, or one became computable that should run), the next
`Step` calls `System.Schedule` with the instructions run and whether the
process may be preempted at this boundary (`Engine.Preemptible`): IPL
below 3, where VMS's rescheduling interrupt is taken; not on the interrupt
stack; and in a mode `vax.process.preempt` allows. A process that waits
gives the CPU up whatever those say.

`System.Schedule` (`corevms/schedule.go`) then:

1. charges the instructions to the current process's quantum, and the
   system time since the last call to its CPU time (`accountTime`:
   `JPI$_CPUTIM` and SHOW SYSTEM's CPU column are system time, not
   instructions);
2. looks at every process (`pollEvents`): expires each one's due timers,
   runs the lock manager's deadlock search if one is due, completes
   terminal reads whose lines have come (7.12), and tests every waiting
   process (`wakeWaiters`, 7.6);
3. if no reschedule is due, or one is but the process can't be preempted
   and isn't waiting, returns at once;
4. otherwise asks `sched.Reschedule` for the next process, idling if
   there's none (7.6), and switches to it (7.4), choosing again if the
   one chosen was deleted as it got the CPU;
5. returns how many instructions may run before the next call: the rest
   of the quantum, but no further than the next timer due in any process,
   so a timer ends its process's wait on time while another runs.

Two things hold the scheduler still. `Engine.FreezeScheduling` keeps the
CPU in the process that has it: the debugger's STEP freezes it, so a step
is one instruction of the process being debugged, and so does a nested
run (a condition handler the console calls). And `Engine.SwitchIfDue`
lets a run loop give the scheduler its turn before looking at the next
instruction, so a breakpoint test or the trace sees the process that will
run it.

### 7.6 Waiting

A service that can't finish yet (`$HIBER` with no wakeup pending,
`$WAITFR` on a clear flag, `$QIOW` on an I/O still pending) returns
`corevms.ErrWait`. The XFC instruction that called it doesn't complete:
its handler puts PC back to the XFC (`cpu/xfc.go`, `ErrServiceWait`), so
the call is made again later, and finishes then if the wait is over.
Without the scheduler, "later" is the next instruction, and the process
spins on its XFC, with interrupts, ASTs, and timers between tries.

With the scheduler, the service first says what it's waiting for
(`Environment.waitOn`, `corevms/waits.go`): a wait state, the boost class
of the event that will end it, and a Go function that reports whether it
has ended. `enterWait` hands that to the scheduler, which marks the process
waiting and asks for a reschedule; another process runs. A wait ends in
one of two ways:

- **Reported.** Code that makes an event happen reports it for the
  process it concerns (`reportEvent`, the book's SCH$RSE): an I/O request
  completing, a timer expiring, a Ctrl/C or Ctrl/Y AST, a flag set in a
  common cluster, a lock granted, room in a mailbox. If the process's test now
  says its wait is over, it becomes computable at once, boosted by the
  event's class, and may preempt the process that caused it.
- **Noticed.** Before every choice, `wakeWaiters` runs every waiting
  process's test. This catches whatever isn't reported, at the cost of a
  call per waiter per choice, which with a handful of processes is
  nothing.

Either way, when the process next runs, its XFC runs again, and this time
the service completes. A waiting process that has an AST it could take
where it waits (by 7.7's rules, at the PSL saved in its PCB) is made
computable too; it takes the AST, whose routine returns to the XFC, and
the service runs again and waits again if it must, as on VMS (the book's
10.2.2.1).

The boosts govax gives the waits it can't trace to a reporter are its
choice: 3 for a wakeup or a resource, and 2, an I/O completion's, for an
event flag.

**Idling.** When no process is computable, VMS runs its null process
until an interrupt makes one so. In govax the only thing that can end a
wait without an instruction running is time, so `System.idle`
(`corevms/idle.go`) moves time to the next timer due in any process
(`Engine.IdleUntil`: a jump in quantum mode, a sleep in hardware-clock
mode), expires it, tests the waiters, and repeats, up to 64 timers. With
no timer pending, it waits a little in the host for a terminal line, or
for a wait on the host's time (DCL's WAIT, and process 1 lending the CPU:
7.12), testing the waits every 2 milliseconds for up to 50; after that,
every waiter is made to try again (`retryWaiters`), spinning on its XFC as
a lone process would, which keeps Ctrl/C and the run's limits working.
Idle time is charged to no process.

### 7.7 ASTs

An **AST**, an asynchronous system trap, is a procedure call VMS makes on
a program's behalf, between two of its instructions, when something it
asked about happens: a timer ($SETIMR's astadr), an I/O completing, a lock
granted, a subprocess ending, or a $DCLAST. govax keeps each process's
pending ASTs in Go (`Process.ast`, `corevms/ast.go`), with VMS's per-mode
state: whether ASTs are enabled in each mode ($SETAST, PCB$B_ASTEN) and
whether one is running (PCB$B_ASTACT).

**When one is delivered.** At every instruction boundary the engine asks
the running process's `NextAST`. VMS keeps a process's ASTs in one queue
ordered by mode, kernel's first and oldest first within a mode, and its
AST delivery interrupt looks only at the queue's first AST (the book's
7.2.2 and 7.3.1). That AST, of mode m, is delivered only if:

- the CPU is in mode m or a less privileged one (a kernel-mode AST
  interrupts user code, switching the CPU into kernel mode to run it; a
  user-mode AST never interrupts kernel code);
- IPL is below 2 (IPL$_ASTDEL) and the CPU isn't on the interrupt stack;
- ASTs are enabled in mode m; and
- no AST of mode m is running already, so an AST routine is never
  interrupted by another of its own mode and needn't be reentrant.

If the first AST can't be delivered, none is: the ASTs behind it wait for
it. govax's list isn't kept in mode order, so `deliverableAST` finds the
head as VMS's queue would have it.

**The frame.** `NextAST` switches mode if the AST's is more privileged
(saving the current SP in its mode's register and taking the AST mode's),
then pushes six longwords that are both the routine's argument list and
what's needed to come back:

```text
  SP+0   5        argument count
  SP+4   astprm   the request's parameter
  SP+8   R0       the interrupted program's R0 and R1
  SP+12  R1
  SP+16  PC       where it was
  SP+20  PSL      and its status
```

The engine calls the routine as `CALLG (SP), routine` would, with the
return address `EXE$ASTRET`, a stub the machine places in S0 with the shim
stubs (`ASTExitStub`), as VMS's routine returns to its executive's
EXE$ASTRET (the book's 4.4.7), so a traceback shows the same PC. The stub
jumps to the `SYS$CLRAST` entry of the system service vector, whose
service (`serviceSysClrast`) pops the six longwords, puts back R0, R1, PC,
and PSL, and switches back to the interrupted mode, never to one more
privileged than the AST's, whatever the frame says. The interrupted
instruction stream goes on.

The PCB's ASTLVL field, which on a VAX tells REI when to request the AST
delivery interrupt, is always 4 ("none pending") in govax: delivery is the
engine's question at every boundary, not an interrupt. Image rundown
discards the image's pending user-mode ASTs and enables user-mode ASTs
again (`flushUserASTs`).

### 7.8 Event flags, hibernation, timers, and suspension

**Event flags** (`corevms/eventflags.go`) are 128 bits per process, in
four clusters of 32. Clusters 0 and 1 (flags 0 to 63) are the process's
own (`Process.LocalEventFlags`). Clusters 2 and 3 (64 to 127) are
*common* clusters, named and shared by the processes of a UIC group: a
process reaches one only after `$ASCEFC` associates it with cluster 2 or
3. A common cluster is a `corevms.EventFlagCluster` in the System's table,
temporary (gone with its last association) or permanent (until
`$DLCEFC`). A flag set in one, by any process, is reported to every
process associated with it (`postFlag`), so a waiter in CEF wakes at once.
`$WAITFR`, `$WFLAND`, and `$WFLOR` wait with a test of the cluster's
flags; `$SYNCH` waits for a flag and an I/O status block together.

**Hibernation** (`corevms/hibernate.go`) is one flag per process, "wake
pending". `$WAKE` sets it, in any process; `$SCHDWK` sets it later, from
the target's timer queue; `$HIBER` waits in HIB until it's set, then clears
it. Being a flag, two `$WAKE`s before a `$HIBER` end only that one.

**Timers** (`corevms/timers.go`): each process has its own list of
pending `$SETIMR` and `$SCHDWK` requests, by expiry time. The scheduler
expires every process's at each call (7.5) and limits each process's
budget so that none comes due late; the AST check expires the running
process's at each boundary too. An expired timer sets its flag, wakes, or
queues its AST, and reports the event with a timer's boost.

**Suspension** (`corevms/suspend.go`): `$SUSPND` puts a process in SUSP,
where it ignores every event, until `$RESUME`.

### 7.9 How processes talk

VMS gives processes four ways to cooperate, and govax has them all.

**Mailboxes** (`corevms/mailbox.go`, `mbxdriver.go`) are software devices
that hold messages: `$CREMBX` makes one, named `MBAn`, in the System's
device table, and any process may assign a channel to it by that name or a
logical name, if its protection mask allows. A write when a read is waiting
hands its message straight to the reader; otherwise the message is queued,
and a plain write stays pending until it's read. A write to a full mailbox
(past its buffer quota) waits in MWAIT for the resource RWMBX, unless the
process turned resource waits off ($SETRWM) or the write said
`IO$M_NORSWAIT`; each read that makes room reports the resource to the
writers waiting. Section 8 has the driver's details: the IOSB, attention
ASTs, end-of-file messages.

**Global sections** (`corevms/gblsec.go`) are pages several processes map,
each process's PTEs pointing at the same frames, so a store by one is seen
by the others at once (section 5.7, and section 6.8 under paging). A
section has a group or system name and a UIC protection mask, and lasts
until its last mapping goes (or, if permanent, until `$DGBLSC` and then
its last mapping). Processes coordinate their use of it with the other
three mechanisms, or with the interlocked instructions, which, as 7.4
says, a switch never splits.

**The lock manager** (`internal/lck`, with the services in
`corevms/enq.go`) is VMS's: a process asks for a lock on a *resource*, a
name the cooperating processes agree on, in one of six modes (NL, CR, CW,
PR, PW, EX), and each resource keeps granted, conversion, and waiting
queues, first in, first out, with the book's compatibility table, parent
locks and sub-resources, value blocks, and NOQUEUE (the book's chapter
13). Like `internal/sched`, `lck` knows nothing of processes but PIDs. It
reports what happens to a lock (granted, blocking another) as an event,
and the request's record in corevms carries it out in the owner's
process: the lock status block is written through the owner's address
space, its event flag set, its completion or blocking AST queued, and, if
it waits, it's made computable (`reportEvent`). A request that has waited
`DEADLOCK_WAIT` starts the book's deadlock search (`lck/deadlock.go`),
whose victim gets `SS$_DEADLOCK`. RMS's record locks and file sharing go
through the same database (section 9).

**Event flags and wakeups** (7.8) carry no data, but are often all two
processes need: a common cluster's flag, or a `$WAKE` of a hibernating
server.

### 7.10 Making and ending a process

**$CREPRC** (`corevms/creprc.go`) only checks its arguments and builds the
process; everything else happens in the new process when it first runs,
which is VMS's order too. `CreateProcess`:

1. checks the request: the status flags, the name's and strings' lengths,
   the quota list's codes, the privileges some requests need (DETACH or
   CMKRNL for a detached process with another UIC, or a batch or network
   one; NETMBX, PSWAPM, NOACNT), and that no process of the new one's UIC
   group has its name;
2. works out its quotas (`resolveQuotas`, `corevms/quotas.go`): a
   subprocess's are held to its creator's, and the CPU time it gets is
   taken out of its creator's limit; a detached process created with the
   DETACH privilege gets what it asks for;
3. takes a slot in the table (7.2): a **subprocess**, without a UIC, joins
   its creator's job and counts against PRCLM; a **detached** process, with
   a UIC or `PRC$M_DETACH`, starts a job of its own, with a logical name
   table of its own;
4. builds its table area, stacks, and PCB (7.3), and writes the PCB the
   first switch loads: the stacks and page table registers, user mode, PC 0
   for now;
5. gives it its creator's default device and directory, its privileges
   (those asked for, limited to the creator's own without SETPRV), its base
   priority (no higher than the creator's without ALTPRI), and a
   `ProcessStartup` naming the image and its SYS$INPUT, SYS$OUTPUT, and
   SYS$ERROR.

`$CREPRC` returns with the process computable at its base priority, with
no boost (VMS's own runs gave a new process 2 or 3 above its base, which
fits no rule of the book's; govax adds none).

**Startup** (`corevms/startup.go`). The first time the scheduler switches
to the new process, `switchTo` calls `startProcess` instead of letting it
run its first instruction. In the new process's context (its address
space, its stacks), it opens the SYS$ names as process-permanent files
(section 8), has the machine activate the image in the process's P0 and
build its IMAGE$INIT driver (as RUN does; section 11), and calls the
driver in user mode on the process's user stack, on a frame whose return
address is the console's sentinel, `FFFFDEAF`. A process whose image
can't be activated (it doesn't exist, say) is deleted at once with that
failure's status, which its creator learns only from the termination
message, as on VMS: `$CREPRC` has already succeeded.

**Deletion** (`corevms/delete.go`). A process that runs an image ends
when the image does: its `$EXIT`, or its return to the sentinel. Then
`DeleteProcess` runs, in the process's own context, the steps of the
book's 22.2.1 in order: image rundown (its RMS files closed, user-mode
channels deassigned, user-mode ASTs discarded); process rundown (every
channel, every allocated device, its locks); the unused CPU time given
back to its owner; the **termination message**, if the creator named a
mailbox (an 84-byte accounting record, `MSG$_DELPROC` and the `ACC$`
fields: final status, PID, times, counts; `corevms/termmsg.go`); a
LIB$SPAWN parent told (7.11); the terminal given back (7.12); and the
process taken out of the table, the scheduler, and its job. What the CPU
is still standing on, its page tables, stacks, and PCB, is freed by the
next `switchTo`, once the CPU holds another process; the scheduler chooses
one at once, since the deleted process has left it.

A process with subprocesses can't go before them: it marks each for
deletion and waits (MWAIT), and each, deleted in its own context, leaves
its owner's count; when the last has gone, the owner's deletion goes on.
A tree goes from its leaves up. **`$DELPRC`** of another process only
marks it (`deletePending`); its deletion runs the next time the scheduler
gives it the CPU, before anything else it would run, as VMS's special
kernel-mode AST does. `$FORCEX` instead queues the target an AST that
calls `$EXIT` with the given status.

### 7.11 Processes that run DCL

A process LIB$SPAWN creates, or `$CREPRC` creates with the image
`LOGINOUT.EXE`, runs a command interpreter rather than one image
(`corevms/cliprocess.go`). On VMS that's DCL; in govax it's the console's
own DCL (`internal/dclshell/subcli.go`): the same grammar and commands,
with each process its own DCL state (symbols, command levels, `$STATUS`,
default directory, logical names, open files). Only the commands that
work the machine (DEBUG, ASM, INITIALIZE/MEMORY, ...) are process 1's.

**LIB$SPAWN** (`corevms/spawn.go`) makes the subprocess as `$CREPRC`
does, and gives it more of its parent: a name (by default the parent's
user name and `_n`), its parent's SYS$INPUT, SYS$OUTPUT, and SYS$ERROR
unless others are named, its base priority and current privileges, and
copies of its DCL symbols and user- and supervisor-mode process logical
names. Unless `CLI$M_NOWAIT`, LIB$SPAWN doesn't return until the
subprocess is gone: the parent waits in LEF. At the end, the
subprocess's final status, event flag, and AST reach the parent directly
from Go (`completeSpawn`), where VMS's LIB$SPAWN would read a termination
mailbox; the parent can't tell.

**The CLI runs in the process.** DCL is Go, but a process runs only VAX
code, so the startup calls a small procedure in S0, the *CLI stub*, as it
would an image's driver:

```text
stub:   .WORD   0                   ; entry mask
loop:   MOVL    #cliShimCode, R0
        XFC     #XFC$SHIM           ; EXE$CLI_COMMAND: the CLI, in Go
        PUSHL   R0                  ; the CLI's final status
        CALLS   #1, @#SYS$EXIT      ; logs out: deletes the process
        RET
args:   .LONG   0
ctrly:  .WORD   0                   ; the Ctrl/Y AST's entry mask
        MOVL    #cliShimCode, R0
        XFC     #XFC$SHIM           ; EXE$CLI_COMMAND, in the AST
        RET
```

The interpreter for each such process runs on a goroutine of its own,
which takes turns with the engine: the shim hands it the CPU and waits
until it hands it back, so only one of the two ever runs. It hands the
CPU back:

- **after each command**, yielding (`Environment.Yield`): the process
  stays computable and goes to the back of its queue, and its XFC runs
  again, with the next command, when the scheduler next chooses it. So a
  SPAWN/NOWAIT procedure's commands interleave with process 1's;
- **to wait**: for a command line from a terminal or mailbox not there
  yet, or for a WAIT's time. The shim returns `ErrWait`, and the process
  waits in the scheduler like any service;
- **to run an image**: RUN activates it in the process's P0, and the
  shim asks the engine to call its driver as an image's outermost call.
  When the image ends, the machine runs it down and puts the stub's
  registers back with PC at its MOVL, so the shim resumes the interpreter
  in the middle of the command (or procedure) that ran it;
- **to log out**, with the final status, which the stub passes to
  `$EXIT`.

LOGINOUT's CLI also echoes each command it reads from a file or a
mailbox (DCL's verify, on for a job that isn't interactive) and writes
LOGOUT's report when it ends, as VMS's did when a probe asked.

### 7.12 The terminal, and lending the CPU

Every process reads the one terminal (govax's standard input), and that
needs rules, because a process reading a line mustn't stop the others.

**One buffer, reads in turn** (`corevms/terminal.go`). Every read of the
terminal (a terminal `$QIO`, RMS's `$GET` of it, LIB$GET_INPUT) goes
through one shared buffer, so nothing one process's read takes ahead is
lost to another. A read goes ahead only when a whole line is there; until
then it waits in LEF, its test being "is a line there?", which the front
end can answer without blocking (cmd/govax's reader knows whether keys
it has read are waiting; `corevms.TerminalSource`). Reads are served in
the order they were made, as VMS's terminal driver queues them, and a
read's prompt is written when its turn comes. A `$QIO` read that must
wait doesn't make the `$QIO` wait: it's queued among the other reads and
the program goes on, and the scheduler's look at the terminal before each
choice (`serviceTerminal`) completes it when its line comes, with the
IOSB, event flag, and AST of any I/O.

**The terminal's holder** (`corevms/attach.go`). The processes of a job
share the terminal, but one *has* it: the one whose DCL reads what's typed,
and which the control keys go to (`Environment.Attached`). It's the job's
master process until a SPAWN that waits gives it to the subprocess (the
parent hibernating until it comes back), or an ATTACH (DCL's, or
LIB$ATTACH) gives it to another process of the job that's waiting for it.
When the process that has it is deleted, it goes back to the parent.

**Lending the CPU.** Process 1's DCL is Go that runs while the machine is
stopped, so without help a subprocess spawned with /NOWAIT would run only
during process 1's next RUN. On VMS the two take turns. So after each DCL
command, process 1 *lends* the CPU (`internal/dclshell/dcllend.go`,
`corevms/lend.go`): it yields in the scheduler (`LendCPU`), and the
console runs the machine until the scheduler chooses process 1 again, and
stops there, before process 1 runs an instruction (`ReclaimCPU` takes the
CPU back). A process of the same priority gets about a turn. Process 1's
own WAIT lends the CPU for its whole time, waiting in LEF on the host's
clock. At command level 0 the console lends it, waiting, until no other
process wants it or for 50 milliseconds of the host's time, so that a
SPAWN/NOWAIT DIRECTORY has finished by the next prompt; then, while the
prompt waits for a line, cmd/govax reads the terminal on a goroutine of
its own and the others go on running until the line comes
(`LendWhileReading`), the host waiting 10 milliseconds at a time when all
of them wait (`HostIdle`). Their output reaches the terminal through the
line editor a whole line at a time, so the line being typed is redrawn
below it.

Here's the trace SET DEBUG PROCESSES gives of that, measured on govax:
process 1 spawns `WAIT 00:00:00.20` with /NOWAIT, then runs its own
`WAIT 00:00:00.50`:

```text
DEBUG(PROCESS): SWITCH 00000301 -> 00000302, PC=80046409
DEBUG(PROCESS): SWITCH 00000302 -> 00000301, PC=FFFFDEAF
DEBUG(PROCESS): SWITCH 00000301 -> 00000302, PC=80046409
DEBUG(PROCESS): SWITCH 00000302 -> 00000301, PC=FFFFDEAF
DEBUG(PROCESS): SWITCH 00000301 -> 00000302, PC=80046409
DEBUG(PROCESS): 00000302's rundown closed 2 process-permanent file(s), error <nil>
DEBUG(PROCESS): 00000302 deleted, status 00000001
DEBUG(PROCESS): SWITCH 00000302 -> 00000301, PC=FFFFDEAF
```

The subprocess always resumes at `80046409`, the XFC in its CLI stub in
S0: it runs no VAX code of its own, only DCL, waiting in the scheduler for
its WAIT's time. Process 1 resumes at `FFFFDEAF`, the sentinel: it runs
nothing at all while it lends the CPU, and the console stops the machine
as soon as the scheduler chooses it. When the subprocess's WAIT is over
its CLI logs out; the stub's `$EXIT` deletes it, and the CPU goes back to
process 1. SHOW SYSTEM meanwhile shows process 1, base priority 4, at 6:
its lending wait ended with a timer's boost of 3, less the one it lost
when it was chosen.

### 7.13 Ctrl/C and Ctrl/Y

A key typed at the host goes a long way to reach a process. In cmd/govax
(`cmd/govax/attention.go`), one goroutine reads the terminal for govax's
whole life, so a key is seen even while nothing reads; while a program
runs, the terminal's driver turns Ctrl/C into SIGINT and Ctrl/Y into
SIGQUIT, which govax catches. Either way the key becomes the engine's
*attention* key (`Engine.Attention`), and the next `Step` offers it to the
machine before running anything (`cpu/attention.go`). The offer goes, in
order:

1. **To a process's AST.** A program can ask the terminal driver for an
   AST on Ctrl/C or Ctrl/Y (`IO$_SETMODE!IO$M_CTRLCAST` or `CTRLYAST`), a
   one-shot request. `System.AttentionAny` (`corevms/ctrlast.go`) gives
   the key to the running process if it has such a request, otherwise to
   the others in PID order; a Ctrl/C with no Ctrl/C AST is taken by a
   Ctrl/Y AST, as on VMS. A process waiting (for a line, say) is woken to
   run it.
2. **Nowhere,** with SET NOCONTROL=Y in effect: the key is echoed and
   dropped.
3. **To a subprocess's DCL**, if the terminal is attached to one. If it's
   running an image, its DCL interrupts the image as VMS's does, with a
   user-mode AST to the CLI stub's `ctrly` routine; when the process
   takes it, the interpreter acts on the Ctrl/Y in the middle of the
   command that ran the image, the image paused beneath the AST. CONTINUE
   returns from the AST; anything else ends the image.
4. **To process 1's DCL**, in a command procedure: its ON CONTROL_Y, or
   the Ctrl/Y level, with the image paused.
5. **Otherwise the machine stops** (`cpu.ErrAttention`). For Ctrl/C,
   process 1's DCL prompts again; if the CPU was in another process, it
   stays there until the next run, or the debugger's EXIT, gives it back
   to process 1. Ctrl/Y ends govax, dismounting its volumes first: govax's
   choice for a key nothing in the machine wanted, where VMS's DCL would
   interrupt to its prompt.

At govax's own `$ ` prompt, Ctrl/C cancels the line and Ctrl/Y ends govax.

### 7.14 What's VMS's and what isn't

- **The scheduler is Go**, called between instructions, not a software
  interrupt at IPL 3; ASTLVL and the AST delivery interrupt aren't used
  (7.7). A program sees the same choices, in instructions rather than time.
- **No swapping**, and no outswapped states; no paging waits (section 6).
- **No null process** to run: idle time is spent moving the clock to the
  next timer or waiting in the host (7.6).
- **No automatic priority or quantum adjustment** beyond the boosts and
  their decay; no IOTA charge for each wait.
- **The process header, the PCB, the JIB, and the AST queue are in Go**
  (but for the hardware PCB, in a page of its own), so a privileged
  program reading VMS's structures directly finds nothing.
- **A new process gets no boost**, and a LIB$SPAWN parent hears of its
  subprocess's end directly, not through a mailbox (7.10, 7.11).
- **Every process's DCL is the console's**, in Go, on a goroutine of its
  own; there is no CLI image in P1, and LOGINOUT is a name, not an image.
- **At most 255 processes**, and the PID's layout is govax's choice in
  VMS 7's form (7.2).

## 8. I/O

A VMS program never touches a device's registers. It asks VMS for a
*channel* to a device by name (`$ASSIGN`), then hands `$QIO`, "queue I/O
request", a request on that channel: a function (read, write, sense
mode, ...), its parameters, and where to report the outcome. VMS checks
the request, builds an *I/O request packet* (IRP), and queues it to the
device's *driver*, VAX code that starts the device and returns. When the
device interrupts, the driver finishes the request, and VMS's I/O
postprocessing writes the outcome to the program's *I/O status block*,
sets its event flag, and queues its AST (the book's chapter 18, and the
*I/O User's Reference Manual*).

govax gives a program the same interface with the same results, and
does the work differently. Its devices are records in a Go table, its
drivers Go functions, and there is no hardware to wait for: almost every
request is done by the time `$QIO` returns, and the few that must wait (a
terminal read with no line typed yet, a mailbox read with no message)
are kept in Go until their event comes. No device interrupts, no IRPs,
no fork processes.

Four paths lead from a program to a device, and this section is about
the first three:

1. **`$QIO`** and the services around it (`$ASSIGN`, `$DASSGN`,
   `$CANCEL`, `$QIOW`, `$GETDVI`), on four drivers: terminal, null
   device, mailbox, and disk (8.1 to 8.5).
2. **RMS**, which most programs use instead: it reaches the terminal,
   mailboxes, and the null device through the same Go state the drivers
   use, and disk files through the `ods2` module directly (section 9).
   The files every process starts with, SYS$INPUT and SYS$OUTPUT among
   them, are RMS's *process-permanent files* (8.7).
3. **The console's own instructions**, the XFCs and the console
   terminal's registers, which the microkernel uses and no VMS program
   does (8.6).
4. **Paging I/O**, a section's or the page file's pages read and
   written by the pager, which never goes through `$QIO` (sections 5.7
   and 6.5).

Under all of it is the host: one terminal, standard input and output,
and container files holding the volumes (8.8).

### 8.1 Devices

VMS describes each device unit with a *unit control block*, the UCB: its
name, class and type, characteristics, owner, counts, and the
device-specific fields its driver keeps. govax's stand-in is
`io.Device` (`io/device.go`), a Go record with the fields `$GETDVI` and
SHOW DEVICES report, named after the UCB's: `DevClass`, `DevType`,
`DevChar` and `DevChar2` (UCB$L_DEVCHAR, the `DEV$M_` bits), `DevDepend`
and `DevDepend2`, `DevBufSize`, `OwnUIC`, `PID` (the process that has it
allocated, or the last to assign it), `RefCnt` (channels assigned),
`OpCnt` (operations completed), and a disk's geometry. Every device is in
one `io.DeviceTable`, which the machine owns and every process's
`corevms.System` shares (`System.Devices`).

Three things make devices:

- **STARTUP.COM** defines the terminal and the null device with DCL's
  DEFINE/DEVICE, with the values VMS 7.1 reported for its own:

  ```text
  $ define/device tta0/devclass=term/devtype=vt100/devbufsiz=80/devchar=201588743/ownuic=65540
  $ define/device nla0/devclass=mailbox/devtype=null/devchar=202702849/ownuic=65537/devbufsiz=512
  ```

  `201588743` is `^X0C040007`, a terminal's characteristics
  (record-oriented, carriage control, terminal, available, input,
  output), and `65540` is `[1,4]`, SYSTEM's UIC. The null device really
  is of class mailbox (160) and type 3 on VMS 7.1, which is why it's
  defined so, and why `$QIO` picks its driver by its type (8.3).
- **MOUNT** defines the disk it mounts on (DUA0, DUA1, ...) if it isn't
  defined yet, of class disk, and the volume's label as the logical name
  `DISK$label`.
- **`$CREMBX`** defines a mailbox, `MBAn`, the next unit number free after
  the last one used, and removes it again when the mailbox is deleted
  (`corevms/mailbox.go`).

The class decides the driver: terminal (66), disk (1), or mailbox (160).
A device of any other class, which DEFINE/DEVICE can make, has no driver,
and every `$QIO` to it is `SS$_ILLIOFUNC`.

**Names.** A program names a device as VMS lets it: `TTA0:`, `TT:`,
`SYS$OUTPUT`, `_DUA1:`, `GOVAX$DUA1:`. `Environment.deviceName`
(`corevms/devices.go`) translates the name through the process's logical
names, as RMS does, unless it starts with an underscore; drops a leading
node name (`GOVAX$`, the node's own); turns `NL` into `NLA0`; and refuses a
name that isn't letters, digits, and dollar signs (`SS$_IVDEVNAM`). A
well-formed name no device has is `SS$_NOSUCHDEV`. `TT` is a process
logical name, defined at startup as its terminal's device name; the four
SYS$ names lead to process-permanent files (8.7), whose equivalence
strings end in the device's name, which is what `$ASSIGN` finds.

**`$GETDVI`** (`corevms/getdvi.go`) answers its item list from the record,
a table of items by `DVI$_` name, with two things added: a mounted disk's
characteristics get `DEV$M_MNT` (and `DEV$M_SWL` if the volume is
read-only), and its label, size, and free blocks come from the volume; a
mailbox's `DVI$_DEVDEPEND` holds its count of waiting messages. Like
`$GETJPI`, it completes during the call, so `$GETDVI` and `$GETDVIW` are
the same service.

**`$ALLOC` and `$DALLOC`** mark a device for one process's use: `DEV$M_ALL`
in its characteristics, its PID and the access mode of the allocation.
Another process's `$ASSIGN` is then `SS$_DEVALLOC`. A mounted disk or a
mailbox can't be allocated (`SS$_DEVMOUNT`). Image rundown deallocates
what the image allocated in user mode.

### 8.2 Channels

On VMS a channel is a *channel control block*, a CCB, in a table in the
process's P1 space, and the channel number is its offset in the table
(the book's 18.1.1). govax keeps a process's channels in Go, a list of
`corevms.channel` records on its `Environment`:

```go
type channel struct {
	Name    string             // the name it was assigned by
	Number  uint16             // 8, 16, 24, ...
	Class   iodev.DeviceClass
	Mode    uint32             // the access mode it was assigned from
	Device  *iodev.Device
	acp     *rms.ACPFile       // a disk file accessed on it, if any
	...
}
```

`$ASSIGN` (`serviceSysAssign`) finds the device, checks that another
process hasn't allocated it and that the process may use it (a mailbox's
protection; 7.9), and makes the channel: numbered 8 more than the last
one this process made, from the access mode given or the caller's,
whichever is less privileged, the device's reference count up by one.
The number goes back to the program in a word. A channel's mode guards
it: `$QIO`, `$CANCEL`, and `$DASSGN` from a more privileged mode than the
channel's are `SS$_NOPRIV`, so user-mode code can't use or release a
channel RMS assigned in executive mode.

`$DASSGN` (`releaseChannel`) undoes it all: the channel's pending
requests complete with `SS$_CANCEL`, its Ctrl/C and Ctrl/Y ASTs are
disarmed, a disk file accessed on it is closed, the device's count goes
down, and a temporary mailbox with no channels left is deleted. Image
rundown deassigns every channel assigned from user mode
(`deassignUserChannels`); process deletion, every channel.

### 8.3 `$QIO`

The call is

```text
SYS$QIO [efn] ,chan ,func [,iosb] [,astadr] [,astprm] [,p1] [,p2] [,p3] [,p4] [,p5] [,p6]
```

`func`'s low six bits are the *function code* (`IO$_READVBLK`,
`IO$_WRITEVBLK`, `IO$_SETMODE`, ...) and the bits above it *modifiers*
(`IO$M_NOECHO`, `IO$M_NOW`, ...); `p1` to `p6` mean what the function
says (for a read, `p1` is the buffer and `p2` its size). The *I/O status
block* is a quadword the outcome is written to:

```text
  word 0   the condition value: how the I/O went
  word 1   the transfer count
  long 1   the device's own information (a terminal read's terminator,
           a mailbox message's sender, ...)
```

`$QIO`'s own status, in R0, says only whether the request was *accepted*.
A read that meets the end of a file returns `SS$_NORMAL` in R0 and
`SS$_ENDOFFILE` in the IOSB.

`Environment.queueIO` (`corevms/qio.go`) is the body of both `$QIO` and
`$QIOW`, in VMS's order:

1. Clear the event flag (0 if none is named); a flag the process can't
   use is `SS$_ILLEFC` or `SS$_UNASEFC`.
2. Find the channel: `SS$_IVCHAN` for 0, `SS$_NOPRIV` for one that isn't
   assigned or belongs to a more privileged mode.
3. Clear the IOSB, if one is named: `SS$_ACCVIO` if it can't be written.
4. Look the function up in the device's driver: `SS$_ILLIOFUNC` if it has
   none.
5. Call the driver's function with an `ioRequest`: the channel, the
   function and modifiers, `p1` to `p6`, the efn, IOSB, and AST to
   complete with, the caller's access mode, the requesting process
   (`owner`), and the boost its completion carries (7.5).

A failure at steps 2 to 5 sets the event flag and returns the error,
with nothing written to the IOSB and no AST, as the *System Services
Reference Manual* says.

**The drivers** are a table, by device class, of tables by function code:

```go
var ioDrivers = map[iodev.DeviceClass]map[uint32]ioFunc{
	iodev.DeviceClassTT:      terminalFunctions,
	iodev.DeviceClassMailbox: mailboxFunctions,
	iodev.DeviceClassDisk:    diskFunctions,
}
```

with one exception, `driverFor`'s: a device of class mailbox and type 3 is
the null device, with its own table, `nullFunctions`. Adding a function is
adding a line to its driver's table.

A driver function returns one of four answers:

```go
type ioFunc func(env *Environment, req *ioRequest) (done ioStatus, reject uint32)
```

- **Done**: `reject` is 0 and `done` is the IOSB's contents. `queueIO`
  completes the request at once (8.4) and returns `SS$_NORMAL`.
- **Rejected**: `reject` is `$QIO`'s R0, usually `SS$_ACCVIO` for a
  buffer the caller can't read or write. Drivers check a buffer before
  doing anything (`Environment.accessible`, which probes every page it
  touches in the caller's mode), so a bad buffer consumes no input and
  writes no output.
- **Pending** (`ioPending`): the driver has kept the request, and will
  complete it later. It goes on the process's `pendingIO` list, and `$QIO`
  returns `SS$_NORMAL`.
- **Resource wait** (`ioResourceWait`): the request can't even start yet,
  a write to a full mailbox in resource wait mode. Nothing is queued;
  `$QIO` returns `corevms.ErrWait`, so its XFC runs again when the
  process next runs (7.6), and the request is made afresh.

None of VMS's I/O quotas (BIOLM, DIOLM, BYTLM, and the AST limit) is
charged, so `SS$_EXQUOTA` never comes from `$QIO`.

### 8.4 Completion

Whenever it happens, during the call or later, completing a request is
`Environment.completeIO`:

1. Count it: the device's `OpCnt` (UCB$L_OPCNT, SHOW DEVICE/FULL's
   "Operations completed"), and the requester's buffered or direct I/O
   count (`JPI$_BUFIO`, `JPI$_DIRIO`; a disk's and the null device's are
   direct, a terminal's and a mailbox's buffered, `corevms/iocount.go`).
2. Take it off the requester's pending list.
3. Write the IOSB: the status's low word, the count, the device's
   longword.
4. Set the event flag, reporting it to the scheduler with the request's
   boost.
5. Queue the AST, if one was named, in the mode the request was made
   from.
6. Report the event, so that a requester waiting for it becomes
   computable at once (7.6).

All of that belongs to the **requester**, which isn't always the process
the CPU is running. A write to a mailbox completes the read another
process has waiting; the scheduler's look at the terminal completes a
read whichever process is current. So the IOSB is written through the
requester's own page tables when it isn't current (`storeOwn`, with
`vm`'s `StoreIn`; section 5.3), and the event flag and the AST queue are
the requester's Go state.

**`$QIOW`** is `$QIO` and a wait in LEF until the request is done. A
request done during the call returns at once. One left pending waits as
every govax service waits (7.6): `ErrWait`, its XFC run again, until the
driver completes it. Since an AST routine can make a `$QIOW` of its own
while one is waiting beneath it, the waits are a stack on the
`Environment`, each entry the request and the frame pointer of the
`SYS$QIOW` call it belongs to; a call that comes round again finds its own
by its FP. `$SYNCH` waits for a flag and an IOSB together, for a program
that used `$QIO` and wants to wait later.

**`$CANCEL`** completes every request pending on a channel with
`SS$_CANCEL`, each with its IOSB, flag, and AST, and disarms the channel's
attention ASTs. A cancelled request is marked, and the driver holding it
drops it the next time it looks.

Here's all of it, measured on govax: a program that writes a line with
`$QIOW` and another with `$QIO` and an AST, reads a line with a prompt,
reads the null device, asks for a function no driver has (63), and passes
a message through a mailbox it creates. Each line of the program's own
report gives R0 and the IOSB's three fields:

```text
channel to TT: 8
Hello from $QIOW
$QIOW write: R0 00000001, IOSB status 00000001 count 16 info 00000000
Hello from $QIO
AST ran, astprm 00001234
$QIO returned 00000001
$QIO write: R0 00000001, IOSB status 00000001 count 15 info 00000000
Name? $QIOW read: R0 00000001, IOSB status 00000001 count 5 info 0001000D
read WORLD
NL: read: R0 00000001, IOSB status 00000870 count 0 info 00000000
bad function: R0 000000F4, IOSB status 00000000 count 0 info 00000000
MBX read: R0 00000001, IOSB status 00000001 count 14 info 00000301
```

The `$QIO` write was done during the call, so its user-mode AST was
already queued when the XFC finished, and the engine delivered it at the
next instruction boundary (7.7): "AST ran" comes before the line the
program printed after `$QIO` returned. The read's information longword is
its terminator, a carriage return (`0D`), and the terminator's size, 1.
`870` is `SS$_ENDOFFILE`, the null device's answer to every read. Function
63 was refused in R0 (`F4`, `SS$_ILLIOFUNC`), so the IOSB is as step 3
left it, cleared. The mailbox read's longword is the sender's PID,
process 1. (The reply `WORLD` came from a script, so nothing echoed it,
and the report follows the prompt on its line.)

### 8.5 The drivers

#### The terminal

Every terminal is the console: the terminal driver
(`corevms/ttdriver.go`) reads govax's standard input and writes its
standard output, through the buffer and queue all processes share
(7.12).

**Reads** (`IO$_READVBLK` and its logical, physical, and read-all forms;
`IO$_READPROMPT`, which writes `p5`/`p6`'s prompt first) collect
characters into `p1`'s buffer until a *terminator* comes (by default any
control character but backspace, tab, line feed, vertical tab, and form
feed: in practice Return or Ctrl/Z; `p4` may give a mask of its own), the
buffer is full, or the input ends (`SS$_ENDOFFILE`). The host delivers a
line ending in a newline where a terminal would send Return, so a newline
(or a `\r\n` pair) is read as one carriage return. The terminator isn't
counted in the transfer count, but is stored after the data when the
buffer has room, and reported in the IOSB's second longword: the
character in its low word, its size in its high word. `IO$M_CVTLOW`
raises lower case as it's stored, `IO$M_PURGE` throws away what was typed
ahead.

A read whose line isn't there yet doesn't wait in `$QIO`: it joins the
terminal's queue, pending, and the program goes on (7.12). The scheduler
completes it when its turn and its line have come (`serviceTerminal`),
whatever process is running then.

**Writes** (`IO$_WRITEVBLK` and its forms) send `p2` bytes from `p1`, with
`p4` the *carriage control*: either a FORTRAN carriage control character
in its low byte (`' '` a line, `'0'` a line after a blank one, `'1'` a new
page, `'+'` overprinting, `'$'` a prompt, the cursor left at its end), or a
*prefix* and *postfix* byte in its upper two (a count of line feeds, or a
control character), as an RMS print file's records have them. `p4` of 0
means the data has its own carriage returns and line feeds, and is sent as
it is.

VMS's convention for a line is a line feed before and a carriage return
after: the cursor rests at the start of the line just written. govax's
terminal is written a line at a time, each line followed by a newline,
the cursor resting at the start of the *next* line; that's how RMS's
records, DCL's output, and the host's own tools write. So the driver
doesn't send VMS's characters as they are: it translates the carriage
control into lines (`terminalText`, by RMS's rule for print files,
`rms.PrintTerminalText`). A prefix's first line feed is the newline
already written; a postfix's carriage return is the line's own newline;
a line with nothing after it is *open*, and the next line written starts
with a newline to end it. Whether the line is open is kept per process
(`rms.FileTable.LineOpen`), shared with RMS's own terminal output and
cleared by a read (the reply typed ends the line), so a `$QIO` line, a
LIB$PUT_OUTPUT line, a FORTRAN `$` prompt, and its reply sit on the screen
as they would on VMS. A program's own raw output leaves the line open
unless it ends in a line feed.

**Characteristics.** `IO$_SENSEMODE` returns the terminal's class, type,
page width, characteristics (`TT$M_` bits), and page length in an 8- or
12-byte buffer, and `IO$_SETMODE` sets them; they live in the device
record's `DevType`, `DevBufSize`, `DevDepend`, and `DevDepend2`, where
VMS keeps them in the UCB, so `$GETDVI` and SHOW DEVICE see a change. With
`IO$M_TYPEAHDCNT`, sense mode returns how many characters are typed ahead.
`IO$_SETMODE!IO$M_CTRLCAST` and `CTRLYAST` arm a one-shot Ctrl/C or Ctrl/Y
AST (7.13).

**What the host terminal can't do.** govax doesn't own the host's
terminal: in its ordinary mode the host has already edited and echoed a
line before govax sees it. So `IO$M_NOECHO` can't hide what's typed,
`IO$M_TIMED` with a limit of 0 seconds reads only what's typed ahead (the
usual "anything there?" poll) while a longer limit waits without one, and
the modifiers for escape sequences, line editing, Ctrl/O, broadcast, and
associated mailboxes succeed and do nothing.

#### The null device

`NLA0:` (`corevms/nulldriver.go`) is VMS's bit bucket. A read is
`SS$_ENDOFFILE` at once, the buffer untouched; a write needs a readable
buffer, discards it, and reports a transfer count of 0, as VMS 7.1's did;
set mode and end of file succeed and do nothing; sense mode is
`SS$_ILLIOFUNC`, again as on VMS 7.1. Its I/O counts as direct I/O. Any
number of processes may have it assigned, and it's the natural SYS$INPUT
and SYS$OUTPUT for a process with nothing to talk to.

#### Mailboxes

A mailbox (`corevms/mailbox.go`, its driver `mbxdriver.go`) holds whole
messages, not a stream of bytes, and pairs readers with writers:

- A **write** when a read is waiting hands its message straight to that
  read, and both complete. Otherwise the message is queued, and the write
  stays pending until someone reads it, so the writer learns its message
  arrived; with `IO$M_NOW` it completes at once.
- A **read** takes the oldest message, completing it and, if it waited,
  its write. With nothing queued, it stays pending until a write comes;
  with `IO$M_NOW` it completes at once with `SS$_ENDOFFILE`.

A read's IOSB has the message's length and the writer's PID; a write's
has the length and the reader's PID, or 0 for an `IO$M_NOW` write, which
finishes before anyone reads it (as VMS 7.3's did). `IO$_WRITEOF` sends an
end-of-file message, which a read gets as `SS$_ENDOFFILE`. A message longer
than the mailbox's largest is `SS$_MBTOOSML`; a read into a shorter buffer
gets `SS$_BUFFEROVF` and loses the rest. A write past the mailbox's buffer
quota waits for room in RWMBX (the resource wait of 8.3) unless the process
turned resource waits off or the write said `IO$M_NORSWAIT`, which get
`SS$_MBFULL`. Reads need read access by the mailbox's protection mask and
writes write access (`SS$_NOPRIV` otherwise); the system's own messages,
a termination message or OPCOM's reply, aren't checked.

A program that would rather be told than wait arms an *attention AST* with
`IO$_SETMODE`: `IO$M_READATTN` for a message no read is waiting for,
`IO$M_WRTATTN` for a read waiting on an empty mailbox, `IO$M_MB_ROOM_NOTIFY`
for room made. Each is delivered once, at once if its event has already
happened.

#### Disks

A program can read a file without RMS, block by block, through the disk
driver (`corevms/diskdriver.go`): `$ASSIGN` a channel to the disk, then
`IO$_ACCESS!IO$M_ACCESS` with a *file information block* (FIB) naming the
file by its file ID, or by a directory's ID and a name in `p2`; then
`IO$_READVBLK` and `IO$_WRITEVBLK` move the file's *virtual* blocks (block
1 is its first) into and out of `p1`'s buffer, `p3` the first block and
`p2` the byte count; then `IO$_DEACCESS`, or `$DASSGN`. `IO$_CREATE`,
`IO$_DELETE`, and `IO$_MODIFY` (to extend a file, or write its attributes)
take the same FIB, and every function may read or write an *attribute
list* in `p5` (the file's record attributes, dates, protection, ...,
`diskattr.go`). On VMS these are the functions of the disk's *ancillary
control process*, the file system that runs beside the driver; in govax
they're calls on `rms.MountTable`'s ACP routines, over `ods2` (section 9),
with ods2's errors turned into `SS$_` statuses by a table
(`acpStatuses`). The file accessed is kept on the channel (`channel.acp`),
shared with RMS's opens of it by the same sharing rules, so a program's
FIB$M_WRITE access conflicts with another's RMS `$OPEN` as it would on
VMS (`SS$_ACCONFLICT`).

`IO$_READLBLK` and `IO$_WRITELBLK` move the volume's *logical* blocks,
numbered from 0 across the whole disk with no files involved, and
`IO$_READPBLK` and `IO$_WRITEPBLK` its *physical* blocks, which on the
MSCP disks govax pretends to have are the same numbers
(`disklogical.go`). Going around the file system needs a privilege, which
`$QIO` checks before anything else: LOG_IO or PHY_IO for logical I/O,
PHY_IO for physical.

Every disk request completes during the call: the volume is a host file,
and reading it doesn't wait for anything govax could do something else
during. The disk must be mounted; govax has a disk's blocks only through
its mounted container.

### 8.6 The console's own way in

Beneath VMS is the console, and the microkernel (section 11) reaches it
three ways a VMS program never uses.

**XFCs.** XFC, opcode `FC`, which the VAX reserved for a customer's own
microcode, is govax's door from VAX code into Go, its selector byte
choosing what to do (`cpu/xfc.go`; section 4.1, and section 10 for the
rest). Four of the choices are the console's:

| Selector | Name | What it does |
| --- | --- | --- |
| 1 | `XFC$CONSOLE_WRITE` | write R0's low byte to the terminal |
| 2 | `XFC$CONSOLE_READ` | read one byte from the terminal into R0's low byte |
| 3 | `XFC$CONSOLE_CMD` | run the counted string R0 addresses as a console command line; R0 is 0 if it succeeded |
| 4 | `XFC$CONSOLE_PUT` | write the string R0's descriptor describes, in one write |

They go to the machine (`machine/services.go`), which writes to and reads
from the console's own streams, not through a device or a channel, and
they work in any mode, checked by the current mode's access to the string.
The microkernel's console output, `exe$put_one` in `kernel.asm`, is one
`XFC$CONSOLE_PUT` per string.

**The console terminal's registers.** The VAX architecture defines a
console terminal reached through four processor registers: RXCS and RXDB
to receive, TXCS and TXDB to transmit, each with an interrupt
(`EXC$CONREAD` and `EXC$CONWRITE`, at IPL 20). govax has them
(`cpu/procreg.go`): `MTPR` to TXDB writes the character to the console at
once, TXCS's ready bit is always set, and the transmit interrupt is
raised if TXCS enables it; `MFPR` of RXDB reads a byte from the console's
input then and there. The receive interrupt exists
(`Engine.DeliverConsoleByte`) but nothing raises it: input is read by the
processes from the shared terminal (7.12), never pushed at the CPU. The
microkernel still enables RXCS's interrupt and has a handler for it,
`exe$rx`, which never runs; its transmit handler only dismisses the
interrupt.

**CHMK.** The microkernel's own system calls, from before govax had VMS's
services, are `CHMK #n` with a table of eight routines (`exe$table`):
write a byte (an XFC), halt, read a byte (RXDB), the C library's read,
write, open, and close, and quit. No VMS program calls them.

### 8.7 SYS$INPUT, SYS$OUTPUT, and the process-permanent files

A VMS program that writes a line with LIB$PUT_OUTPUT doesn't name a
device: it writes to SYS$OUTPUT, whatever that leads to. On VMS, LOGINOUT
opens a process's SYS$INPUT, SYS$OUTPUT, SYS$ERROR, and SYS$COMMAND as
*process-permanent files*: files RMS opens for the process, not for an
image, which stay open from one image to the next until the process ends
(the *Guide to OpenVMS File Applications*, 6.6). DCL opens more of them: a
command procedure, its `/OUTPUT` file, and the files of its OPEN command.

govax does the same. Each process's RMS file table holds a table of
entries (`rms/ppf.go`, `rms.PermanentEntry`), each one open stream (the
terminal, a volume file, a mailbox, or the null device) with one position
and the access of the open that made it. A logical name leads to an entry
through an equivalence string of a special form: an escape, a zero byte,
the entry's *internal file index* as a word, then the device's name
(`lnm/ppf.go`):

```text
  1B 00 01 81 "_GOVAX$TTA0:"      IFI ^X8101: the terminal
```

`$TRNLNM` returns the string whole; SHOW LOGICAL shows only the device
name. Process startup (`corevms/ppfstart.go`) opens the entries: for
process 1, the terminal (`OpenTerminalFiles`), SYS$INPUT and SYS$COMMAND
sharing one entry and SYS$OUTPUT and SYS$ERROR another, the names defined
in executive mode as CONCEALED and TERMINAL; for a created process,
whatever its creator named, each a terminal, a mailbox or NL:, or a file,
each opened as an entry of the process's own, and TT defined as the
device its SYS$INPUT names.

An image's `$OPEN` of SYS$OUTPUT opens nothing: it's given an IFI marked
*indirect*, and every `$GET` and `$PUT` through it works on the entry's one
stream (`rms/ppfaccess.go`; section 9). Image rundown ends the indirect
accesses and closes none of the entries, so the next image finds them as
they were.

**Where a line goes.** LIB$PUT_OUTPUT is `Environment.PutOutput`
(`corevms/recdevice.go`), and what it does depends on what SYS$OUTPUT
translates to:

| SYS$OUTPUT leads to | The line goes |
| --- | --- |
| a process-permanent file (the usual case) | through the entry: the terminal, a procedure's `/OUTPUT` file, a subprocess's log, a mailbox (the write waits until it's read), or NL: |
| a mailbox or NL: the image's DCL named for it alone (`DEFINE/USER SYS$OUTPUT MBA3:`) | through a stream the process keeps open in executive mode, as RMS would |
| a file the image's DCL named for it alone (`DEFINE/USER SYS$OUTPUT X.LIS`) | into that file, made at the first line, held open by the image and closed at its rundown (`corevms/outfile.go`) |
| nothing that can be written | the terminal, as a line |

A condition's message goes the way `$PUTMSG` sends it, to SYS$ERROR and,
if different, SYS$OUTPUT (`corevms/msgout.go`). LIB$GET_INPUT reads
SYS$INPUT's entry the same way, which for a command procedure is the
procedure's own data lines.

### 8.8 The host side

Everything a govax session shows and reads passes through `cmd/govax`.

**Input.** One goroutine, the *pump* (`cmd/govax/attention.go`'s
`attentionStdin`), reads the host's terminal for govax's whole life and
hands each byte on through a channel, so that a key is seen even while
nothing is reading, a program computing, say. Everything that reads the
terminal reads the pump: the command line's editor at a prompt
(`github.com/chzyer/readline`, which puts the terminal in raw mode while
it reads a line), and, the rest of the time, the processes, with the
terminal in its ordinary mode set up for VMS's keys
(`cmd/govax/terminal_unix.go`'s `vmsTerminalMode`): Ctrl/C raises SIGINT
and Ctrl/Y SIGQUIT, which govax catches (7.13); Ctrl/Z is the end of file,
which the pump turns into VMS's Ctrl/Z character, echoing `*Exit*`; the
host's suspend keys are off. The pump is the corevms terminal's
`TerminalSource` (through the console's input): `Ready` says whether bytes
are waiting, which is how a process's read knows whether its line has come
without blocking the CPU (7.12). Input from a pipe or a file has no
terminal modes and no keys; govax reads it as it comes.

**Output.** Every process's terminal output (the drivers', RMS's,
LIB$PUT_OUTPUT's, DCL's) goes to one writer, the console's output. At a
terminal, that's `cmd/govax/promptout.go`'s `promptOutput`: it writes
straight to the terminal, except while a command line is being read with
the other processes running (7.12), when it passes whole lines through the
line editor's own writer, which takes the prompt and the half-typed line
off the screen, writes, and puts them back below.

**Volumes** are host files too, the *containers* MOUNT names, holding an
ODS-2 volume block for block (section 9). And to govax's own commands, a
file name in quotes, or one with no volume to go to, names a host file
directly, read and written as a VMS file of its own record format; a
program's RMS reaches only volumes (section 9.12).

### 8.9 What's VMS's and what isn't

- **No I/O hardware and no interrupts.** No device interrupts, IRPs, fork
  processes, or I/O postprocessing interrupt; drivers are Go functions,
  and most requests complete during `$QIO`.
- **The UCB, the CCB, and the IRP are Go records**, not in VAX memory, so
  a privileged program looking for VMS's I/O database finds none. Channel
  numbers are 8, 16, 24, ..., not offsets into a CCB table in P1.
- **Four drivers**: terminal, null, mailbox, disk. No tapes, printers,
  network devices, or associated mailboxes (`$ASSIGN`'s mailbox name is
  kept and not used).
- **No I/O quotas** charged: BIOLM, DIOLM, BYTLM, and ASTLM never refuse a
  request.
- **One terminal**, the host's, which edits and echoes lines itself:
  `IO$M_NOECHO` and timed reads longer than 0 seconds don't do what they
  ask, and terminal output is translated to the host's lines rather than
  sent as VMS's line feeds and carriage returns.
- **The console terminal's receive interrupt is never raised**; the
  registers work, polled.

## 9. RMS and ODS-2

On VMS, a program's files go through two layers. **RMS**, the Record
Management Services, is what the program calls: it names a file, opens
it, and reads and writes *records* (lines of text, fixed-size entries,
keyed rows), keeping the details of how they're laid out in blocks to
itself. Beneath RMS is the **file system**, Files-11 On-Disk Structure
Level 2 (**ODS-2**): the disk's directories, file headers, and free-space
maps, which the disk's *ancillary control process* (the ACP, later the XQP)
maintains when RMS asks it through `$QIO` (section 8.5). On VMS, RMS is
VAX code that runs in executive mode in the program's own process, with
its buffers and internal structures in P1 space (the *Guide to OpenVMS
File Applications* describes what a program sees of it; the *OpenVMS RMS
Reference Manual* each service).

govax keeps the same two layers and moves both into Go:

```text
   the program ............ FAB, RAB, NAM, XABs in its own memory
       |                    CALLS SYS$OPEN, SYS$GET, ...
       |                    (the system service vector: section 10)
   corevms/rms.go ......... the service table's entries for RMS
       |
   rms .................... name processing, open files, sharing, record
       |                    locks, sequential, relative, and indexed files
       |                    (isam: an indexed file's buckets and keys)
       |
   ods2 (its own module) .. volume: files, headers, directories, bitmaps
       |                    rms: records in a file's blocks
       |                    diskimage: 512-byte blocks of a container
       |
   the host ............... a container file, the disk's blocks in order
```

`internal/rms` is the only package that imports `ods2`. Everything else in
govax that touches a volume, DCL's DIRECTORY and COPY, LINK's libraries,
the image activator, the page file, goes through `rms` too, so there is one
idea of what a volume holds and who has a file open.

### 9.1 The `ods2` module

`github.com/tucats/ods2` is a separate Go module: a library for ODS-2
volumes held in host files, with a command-line tool of its own. It knows
nothing of the VAX, of FABs, or of govax. `go.mod` pins a tagged release;
a local `go.work` can put a sibling checkout in its place while both are
being changed.

**Containers.** A volume lives in a *container*, a host file whose bytes
are the disk's logical blocks, block 0 first (`diskimage`). A container can
also be a raw CD-ROM sector dump, which is read-only. Every read and write
of the volume is a whole 512-byte block of the container, numbered by
*logical block number* (LBN).

**Mounting** (`volume.Mount`) reads the *home block*, near the start of the
disk, which gives the volume's label, cluster size, and where its index
file is. The **index file**, INDEXF.SYS, holds a *file header* for every
file on the volume, one block each, by file number. Its own header sits at
a block the home block's fields locate directly, which breaks the circle:
from there the rest of INDEXF.SYS can be mapped like any file, and then any
other file's header found through it.

**A file** is named, at this level, by its *file ID* (FID): the file's
number in the index file, a sequence number bumped each time the slot is
reused (so a stale FID is caught), and a relative volume number. Its header
holds its attributes (the record attributes RMS needs, dates, owner,
protection, ACL) and its *map*: retrieval pointers, each a run of
contiguous logical blocks. `volume.File` has the header and the map
resolved into `Extents`, and its *virtual blocks* (VBNs, numbered from 1
within the file) are found by walking them:

```go
// ods2's volume/file.go, cut down
func resolveExtentLBN(extents []ExtentLocation, vbn uint32) (uint32, error) {
	remaining := vbn
	for _, e := range extents {
		if remaining <= e.Count {
			return e.StartLBN + (remaining - 1), nil
		}
		remaining -= e.Count
	}
	return 0, fmt.Errorf("virtual block %d is beyond the end of the file", vbn)
}
```

A block the file has been given but never written reads as zeros, as VMS
guarantees, rather than as whatever a deleted file left there.

**Directories** are files too, of a special format: a sorted list of
names, each with its versions in descending order and each version's FID.
`volume.Directory` looks a name and version up, lists, inserts, and
removes; the master file directory, `[000000]`, is always file (4,4,0).

**Free space** is two bitmaps: BITMAP.SYS, a bit per cluster of the disk,
and the index file's own bitmap of header slots. ods2 reads each into
memory the first time something allocates, and writes them back when the
volume is dismounted (`Volume.Dismount`), not after each operation. Headers,
directories, and data go to the container as they're written; the bitmaps
wait. So a govax that is killed rather than allowed to exit leaves a
writable volume's bitmaps as they were at its mount, and later allocations
can collide with blocks already in use; govax dismounts every volume at
the end of a session for this reason (`MountTable.DismountAll`).

**Records.** ods2's own `rms` package reads and writes records in a
sequential file's blocks by its record format (`odsrms.Reader`,
`odsrms.Writer`):

| Format | On disk |
| --- | --- |
| FIX, fixed length | each record its size in bytes, padded to a word |
| VAR, variable length | a word's length, the bytes, a pad byte to a word; a block's records end with `FFFF` |
| VFC, variable with fixed control | as VAR, the length counting the fixed control area (FAB$B_FSZ bytes) that leads the record |
| STM, STMLF, STMCR | bytes ending with CR LF, LF, or CR |
| UDF, undefined | bytes, with no records at all |

A *record's file address*, its RFA, is the VBN of the block holding its
first byte and that byte's offset in the block; govax keeps a sequential
record's position as its byte offset in the file, from which the RFA
follows.

Here is a small file govax made, two lines copied from the host into a VAR
file with carriage-return carriage control, as DCL's DUMP shows its one
block:

```text
Dump of file DUA1:[000000]PARTS.DAT;1 on 11-NOV-2026 12:37:58.69
File ID (11,1,0)   End of file block 1 / Allocated 1

Virtual block number 1 (00000001), 512 (0200) bytes

 00000000 FFFF3032 2D342F31 2054554E 000A0030 322D342F 3120544C 4F42000B ..BOLT 1/4-20...NUT 1/4-20...... 000000
 00000000 00000000 00000000 00000000 00000000 00000000 00000000 00000000 ................................ 000020
```

Read from the right: `000B`, a length of 11, then `BOLT 1/4-20` and a pad
byte; `000A`, 10, then `NUT 1/4-20`, even, so no pad; then `FFFF`, the end
of the block's records. The file is number 11 on the volume, its slot used
for the first time.

### 9.2 Mounted volumes

`rms.MountTable` (`rms/mount.go`) is the system's table of mounted
volumes: device name to an open `ods2` volume, with whether it was mounted
for writing. It's system state, shared by every process (`corevms.System`
holds it), and has no locking: one goroutine runs every process, switching
only between instructions (section 7), so no two operations on it ever
overlap.

DCL's MOUNT (`dclcmd`) opens the container and mounts it on a device name,
`DUA1` say, and defines the device in the device table if it isn't there
yet, with the logical name `DISK$label` (section 8.1). Device names are
compared after `normalizeDeviceName`, so `DUA1`, `dua1:`, `_DUA1:`, and
`_GOVAX$DUA1:` are one entry. DISMOUNT refuses a volume a process has a
file open on (`FilesOpenError`), and the system volume (section 11)
altogether; the page file's volume says so in its refusal.

A volume mounted read-only takes `$OPEN`s that only read; a `$CREATE`, or
an `$OPEN` asking to write, is `RMS$_PRV`.

### 9.3 From a service call to Go

A program calls an RMS service as it calls any system service,
`CALLS #1,G^SYS$OPEN` with a FAB's address in its argument list, and the
system service vector sends it into Go (section 10). corevms's service
table has one entry per RMS service (`corevms/rms.go`): `$CREATE`,
`$OPEN`, `$CONNECT`, `$DISCONNECT`, `$CLOSE`, `$GET`, `$PUT`, `$FIND`,
`$UPDATE`, `$DELETE`, `$TRUNCATE`, `$REWIND`, `$FLUSH`, `$FREE`,
`$RELEASE`, `$WAIT`, `$READ`, `$WRITE`, `$PARSE`, `$SEARCH`, `$DISPLAY`,
`$ERASE`, `$RENAME`, and `$EXTEND`. Each entry is a small wrapper around
`rms`'s handler, which has the same shape as a service with a different
first argument:

```go
// corevms/rms.go, cut down
t.Register(name, func(env *Environment, argv []uint32) (uint32, error) {
	if sts := env.rmsBlockFault(name, argv); sts != 0 {
		return sts, nil // an ill-formed call: R0 only
	}
	ctx := env.rmsContext()
	r0, err := fn(ctx, argv) // rms.SysOpen, rms.SysGet, ...
	env.noteRMSBlockUse(name, argv[0], r0, err)
	if err == nil {
		env.rmsCompletion(name, argv, r0)
	}
	return r0, err
})
```

`rms` can't take a `corevms.Environment`: corevms imports `rms`, and Go
forbids an import cycle. So each call builds an `rms.Context`
(`rms/context.go`, filled by `Environment.rmsContext`): the process's
memory and CPU, its open files (`rms.FileTable`), its logical names, its
`rms.Session` (default directory), the mount table, and, as function
fields, what RMS needs of the rest of VMS: the shared terminal's queue
(`AwaitTerminal`), mailboxes and NL: (`Devices`), the lock database and the
process's PID, a way to wait for a record lock (`AwaitLock`), the clock, the
I/O counts, and a channel for a user file open (`AssignFileChannel`).

A handler reads the FAB or RAB, does its work, stores the status in the
block's STS (and STV), and returns the same status as R0: `storeStatus`
(`rms/status.go`) does both at once, so a program may check either. A Go
`error` is something else: a field the program's own memory can't supply
(a FAB on a page it can't read), which goes back through the service's XFC
instruction as that instruction's failure, an access violation as any
instruction would raise (section 10).

**Ill-formed calls.** "Every RMS service call requires an interface data
structure (FAB or RAB) which is writable in caller's mode and not currently
in use for another operation" (the manual's 2.4.1). Before a service runs,
`rmsBlockFault` checks its block (both FABs, for `$RENAME`): that the
caller's mode may write it for its whole length, by the page table entries'
protection as PROBEW tests it, so no page is brought in for the test;
that its identification byte, FAB$B_BID or RAB$B_BID, says it's the kind
the service takes (`$WAIT` takes either); and that its length byte isn't
less than the block's, 80 bytes for a FAB, 68 for a RAB. A block that fails
is `RMS$_FAB` or `RMS$_RAB`, or `RMS$_BLN` for the length. A block is *in
use* while a service on it waits (a terminal `$GET` with no line typed yet,
a record lock, a full mailbox), during which the process may take an AST,
and while an operation on it is `RMS$_PENDING`, until the `$WAIT` that
finishes it; another call on it is `RMS$_BUSY`, but for the waiting call
itself, run again from its own frame, and the `$WAIT`. RMS can't trust an
ill-formed call's block, so the status goes only in R0: nothing is stored
in the block, and no completion routine is called.

**Completion routines.** After the FAB or RAB, an RMS call may name an
*error* and a *success completion routine*, which RMS calls as an AST, with
the block's address as the AST parameter. For an operation that completes
during the call, the manual says the routine "executes prior to returning
control from the service call" (2.4.3). `rmsCompletion` queues the AST for
the routine the status chooses, in the caller's mode, as the service
finishes; the engine delivers it at the next instruction boundary, which is
still inside the service's vector entry, before its RET (section 7.7). The
service's R0 survives the AST.

**Asynchronous operations.** With FAB$V_ASY or RAB$V_ASY a program asks for
a service to return before its I/O is done, and `$WAIT`s for it later.
govax's RMS finishes every operation during the call, ASY or not, but one:
a `$PUT` to a mailbox, which on VMS too isn't done until the message has
been read. With ASY it returns `RMS$_PENDING`; `$WAIT` waits for the reader
and stores the real status, and the `$PUT`'s completion routine, remembered
until then, is called at the `$WAIT`.

**SET DEBUG RMS** prints each RMS call's control block as the service left
it (`rms/trace.go`). Here is a program reading PARTS.DAT (9.1) with
`$OPEN`, `$CONNECT`, `$GET` until it fails, and `$CLOSE`, printing each
record with LIB$PUT_OUTPUT:

```text
DEBUG(RMS): SYS$OPEN 00000200 name "PARTS.DAT" FAC 02 SHR 00 ORG 00 RFM 02 RAT 02 MRS 0 FOP 00000000 ALQ 1 IFI 0004, RMS$_NORMAL/S (00010001)
DEBUG(RMS): SYS$CONNECT 00000250 ISI 0004 (file, access 02) RAC 0 ROP 00000000 RSZ 0, RMS$_NORMAL/S (00010001)
DEBUG(RMS): SYS$GET 00000250 ISI 0004 (file, access 02) RAC 0 ROP 00000000 RSZ 11 "BOLT 1/4-20", RMS$_NORMAL/S (00010001)
BOLT 1/4-20
DEBUG(RMS): SYS$GET 00000250 ISI 0004 (file, access 02) RAC 0 ROP 00000000 RSZ 10 "NUT 1/4-20", RMS$_NORMAL/S (00010001)
NUT 1/4-20
DEBUG(RMS): SYS$GET 00000250 ISI 0004 (file, access 02) RAC 0 ROP 00000000 RSZ 0, RMS$_EOF/E (0001827A)
DEBUG(RMS): SYS$CLOSE 00000200 name "PARTS.DAT" FAC 02 SHR 00 ORG 00 RFM 02 RAT 02 MRS 0 FOP 00000000 ALQ 1 IFI 0000, RMS$_NORMAL/S (00010001)
```

The `$OPEN` filled in the file's organization (sequential, 0), record
format (VAR, 2), attributes (CR, 2), and allocation from its header, and
its IFI, 4. `$CONNECT` copied the IFI to the RAB's ISI. The third `$GET`
met the end of the file. `$CLOSE` cleared the IFI. The rest of this section
follows those calls through `rms`.

### 9.4 The control blocks

A program describes what it wants in blocks in its own memory, and RMS
reads and writes their fields there, by their VMS offsets:

| Block | What it is | govax's offsets |
| --- | --- | --- |
| FAB, file access block | one file: its name (FNA/FNS) and default name (DNA/DNS), the access wanted (FAC) and the sharing allowed (SHR), organization (ORG), record format (RFM), attributes (RAT), sizes (MRS, ALQ, DEQ, BKS, FSZ), options (FOP); RMS returns the IFI, STS, STV, and the device's characteristics (DEV, SDC) | `rms/fab.go`, from `vmsdef.FABFields` |
| RAB, record access block | one record stream on a FAB's file: the access mode (RAC: sequential, keyed, by RFA), options (ROP), the user buffer (UBF, USZ), the record buffer (RBF, RSZ), the key (KBF, KSZ, KRF), the RFA, a prompt; RMS returns the ISI, STS, STV, RFA, and RSZ | `rms/rab.go`, from `vmsdef.RABFields` |
| NAM, name block | name processing's results: the expanded and resultant strings and their buffers, the FID, the directory's ID (DID), the device's name (DVI), the FNB bits, the wildcard context (WCC), the related file (RLF) | `rms/nam.go` |
| XABs, extended attribute blocks | a chain from FAB$L_XAB: dates (XABDAT, XABRDT), protection (XABPRO), allocation (XABALL), the file header's characteristics (XABFHC), an indexed file's keys and summary (XABKEY, XABSUM) | `rms/xab.go`, `rms/indexedxab.go` |

Every field is read when a service needs it, through the process's page
tables in the current mode (`Context.loadByte`, `storeLongword`, ...), and
written back the same way; nothing about a FAB or RAB is kept in Go but
what its IFI or ISI names. A string RMS reads, a file name, is exactly
FNS bytes from FNA, never NUL-terminated.

The FAB and RAB's offsets were checked against VMS's own `$FABDEF` and
`$RABDEF` and are kept in `vmsdef`, where the console assembler's `.FAB`
and `.RAB` read them too, so the two can't disagree.

### 9.5 Names

A file name a program gives RMS is rarely complete. `PARTS.DAT` has no
device or directory; `SYS$LOGIN:NOTES` has a logical name for its device
and directory and no type; `[-]` is relative to wherever the program is.
*Name processing* turns it into the *expanded specification*, the
`DUA1:[000000]PARTS.DAT;` that names one place to look. govax has one name
processor (`rms/name.go`'s `expandNames`) for every service and for DCL's
utilities, which reach it through `logicals.go`'s `expandSpec`, so the same
name means the same file wherever it's typed.

A name is filled in from four sources, each supplying only the fields the
ones before it left out (the RMS manual's FAB$L_DNA and NAM$L_RLF):

1. the primary name, FAB$L_FNA, after its logical name is translated;
2. the default name, FAB$L_DNA, translated the same way;
3. the *related file*'s resultant string (NAM$L_RLF), for the device,
   directory, name, and type, or with FAB$V_OFP only the name and type;
4. the process's defaults: SYS$DISK's device and the default directory.

A relative directory (`[.SUB]`, `[-]`) applies to the directory the later
sources give. What no source gives stays empty: no name, a type of `.`, a
version of `;`. A field is checked as it's scanned (`scanFields`): letters,
digits, `$`, `_`, `-`, and the wildcards, names and types up to 39
characters, eight directory levels, each fault its own status (`RMS$_DEV`,
`RMS$_DIR`, `RMS$_FNM`, `RMS$_TYP`, `RMS$_VER`, `RMS$_SYN`).

**Logical names.** Only the leftmost part of a name is a candidate, the text
before its first colon, or the whole name when it has none, and only if
it's a name's characters. It's looked up through LNM$FILE_DEV in the
process's logical name tables (`lnm.Database.TranslateFileSpec`), and its
equivalence string takes its place; the result is looked at again from the
start, up to `LNM$C_MAXDEPTH` times, until nothing translates or the
equivalence was terminal. A field that both the program and the
translation give is an error, as VMS 7.1 has it. A name beginning with `_`
is physical and isn't translated.

- A **search list**, a name with several equivalence strings, makes several
  expanded names, one per element in order; `$OPEN` tries each until a
  file is found, and `$SEARCH` searches each in turn.
- A **concealed** name (`DEFINE/TRANSLATION=CONCEALED`) stays in what RMS
  shows: the expanded and resultant strings say `SYS$SYSDEVICE:` rather
  than `_DUA0:`, while the volume is found by the device behind it.
- A **rooted directory**, a concealed name translating to a device and a
  directory ending in a period, `_DUA0:[SYSTEM.]`, makes that directory
  look like a volume's master file directory: `SYS$SYSROOT:[SYSEXE]` is
  `[SYSTEM.SYSEXE]` on the disk, and `[-]` can't climb out of it. A name
  is kept as a device, a root, and a directory relative to the root
  (`parsedName`), joined only when the volume is searched (`volumeDir`).
- A **process-permanent file**'s name, SYS$INPUT and its kin (section 8.7),
  ends name processing: its equivalence string is an escape and an IFI, not
  a name, and the open goes to that file (`parsedName.Permanent`).

**The default directory** is the process's `rms.Session` (`rms/session.go`):
`Default`, the directory SET DEFAULT last set, and the logical name
SYS$DISK, which holds the default device, as on VMS. A new process gets a
copy of its creator's (`Session.ForProcess`).

**`$PARSE` and `$SEARCH`** (`rms/parse.go`, `search.go`) are name
processing on its own, with its results in a NAM, and the search for the
files that match. VMS searches two ways, and govax does both as VMS 7.3's
probe showed them: a name with no wildcard directory and no search list
keeps no context, NAM$L_WCC holding the position of the directory entry
last returned, so each `$SEARCH` starts again from the expanded string; a
wildcard directory (`[*]`, `[TEST...]`) or a search list keeps a context,
which `$PARSE` makes and the FileTable holds (`searchState`), WCC holding
its number. After the last match the status is `RMS$_NMF`, or `RMS$_FNF` if
nothing matched at all.

### 9.6 Opening and closing

**`$OPEN`** (`rms/open.go`'s `SysOpen`) goes in VMS's order:

1. Read FAC and SHR. A FAB asking for no access is opened for GET.
2. Check the NAM and the XAB chain, and FOP's options that refuse at once:
   FAB$V_PPF (for executive mode, which no program's call is), a user file
   open whose sharing lets others write without FAB$V_UPI, block I/O that
   lets others write.
3. With FAB$V_NAM, open by the NAM's FID, or its DID and name, skipping
   name processing.
4. Run name processing. For each expanded name in turn (more than one for a
   search list), stopping at the first that opens or fails for a reason
   other than not finding the file:
   - a process-permanent file: an *indirect* open (section 8.7), no file
     opened, the FAB given an IFI that names the permanent entry;
   - a mailbox or NL: (a `RecordDevice` stream, `rms/recdevice.go`);
   - the terminal, TTA0:;
   - otherwise a volume file: find the mounted volume (`RMS$_DNR` if the
     device holds none, `RMS$_DEV` if there's no such device), resolve the
     directory (`RMS$_DNF`), look the name and version up (`RMS$_FNF`; no
     version or `;0` is the highest, `;-1` the one below it), and open the
     file by its FID (`openFID`).
5. Fill in the FAB from the file's header (ORG, RFM, RAT, MRS, ALQ, ...),
   the NAM (expanded and resultant strings, FID, DID, DVI), and the XABs;
   store the IFI.

`openFID` is where the file is opened twice over, once for each layer of
sharing (9.9): `MountTable.claimOpen` adds the open to the file's list of
RMS openers if every opener's access and sharing allows it, and
`Volume.Access` adds it to the file system's accessors. Either can refuse,
`RMS$_FLK`, "file currently locked by another user". The result is a
`FileHandle` (`rms/ifi.go`) in the process's `FileTable`, under a new IFI.

**`$CREATE`** (`rms/create.go`) checks the FAB's organization, record
format, and sizes, and an indexed file's XABKEYs and XABALLs, in VMS's
order, then makes the file: the next version above the directory's highest
(or the one asked for; with FAB$V_MXV, whichever is higher), a header with
the record attributes, an entry in the directory (`Volume.CreateFileVersion`).
FAB$V_CIF opens a file that's already there instead; FAB$V_TMP and TMD make
one with no directory entry, deleted at its close with TMD.

**The file table.** A `FileHandle` is one open file: the shared
`volume.File` and the open's `volume.Access`, the FAC and SHR it was opened
with, its place among the RMS openers, its stream's state (9.7), its record
locks, and, for a relative or an indexed file, that organization's state.
`FileTable.Alloc` gives the lowest IFI not in use from 4 up (which IFIs
VMS's RMS hands an image's files no probe has recorded); a FAB whose IFI no
open gave out, never opened or already closed, is `RMS$_IFI`.

**`$CLOSE`** writes what the stream still holds to the volume (a `Writer`'s
last partial block) and the header's end of file and longest record; for a
file opened to write, it sets the revision date and count (from an XABRDT
the FAB names, or now and one more) and applies an XABPRO; then it ends the
file system access and the RMS claim, and clears the IFI. The header goes back to the disk with the last accessor's
deaccess; a file deleted while open is deleted then. At image rundown
`FileTable.Rundown` closes every file the image left open, its records
written, as VMS's RMS rundown does; the process-permanent files stay.

### 9.7 Records

**`$CONNECT`** (`rms/connect.go`) starts a record stream: it copies the
FAB's IFI to the RAB's ISI and *arms* the file for the access FAC asked for,
an `odsrms.Reader` for GET and an `odsrms.Writer` for PUT (both for both).
A relative or indexed file has no Reader or Writer; its stream is its own
organization's (9.8). govax keeps one stream per open file, so a second RAB
connected to the same FAB continues the first's stream rather than starting
another.

**The stream's context** (`rms/stream.go`) is what the *Guide to OpenVMS
File Applications* (8.6, Table 8-3) says a stream remembers between
operations, as two positions, byte offsets in the file:

- the **current record**, the one the last successful `$GET` or `$FIND`
  reached, which `$UPDATE`, `$DELETE`, and `$TRUNCATE` act on; any other
  operation, and any failure, leaves none;
- the **next record**, where a sequential `$GET` or `$FIND` reads: the
  first record after `$CONNECT` (the end of the file with RAB$V_EOF), the
  one after the current record after a `$GET`, the end of the file after a
  `$PUT`. A `$GET` right after a `$FIND` reads the record the `$FIND` found.

**`$GET`** (`rms/get.go`) takes the record RAB$B_RAC chooses (the next one;
with RAB$C_RFA the one at RAB$W_RFA) and copies it into the *user buffer*,
RAB$L_UBF, of RAB$W_USZ bytes, which `$GET` requires: without one it's
`RMS$_UBF` or `RMS$_USZ`, before any record is read. A record longer than
the buffer is cut to it with `RMS$_RTB`, a warning whose STV is the
record's whole size. RAB$W_RSZ gets the length stored, RAB$L_RBF the
buffer's address, and RAB$W_RFA the record's RFA.

So a `$GET` from the program in 9.3 goes like this:

1. The wrapper builds a Context and calls `rms.SysGet`, which reads the
   RAB's ISI, 4, and finds the FileHandle in the FileTable.
2. It isn't a terminal, a mailbox, or a relative or indexed file, so
   `locate` (`stream.go`) picks the next record's offset, takes a record
   lock if the stream locks records (9.9), and asks the handle's Reader for
   the record at that offset.
3. The Reader (ods2's `rms/reader.go`) needs block 1 of the file, and asks
   the `volume.File` for it: `ReadBlock` walks the extents to an LBN and
   reads that block of the container, one host `ReadAt` of 512 bytes.
4. The Reader takes the length word, 11, and the eleven bytes after it.
5. `storeRecordStatus` copies them into the program's buffer through its
   page tables, stores RSZ, RBF, and STS, and returns `RMS$_NORMAL` as R0.

**`$PUT`** (`rms/put.go`) writes RAB$W_RSZ bytes from RAB$L_RBF. On a
sequential file it goes at the end only, when the next record is the end
of the file; anywhere else it's `RMS$_NEF`, unless RAB$V_TPT cuts the file
off there first. The Writer frames the record by the format, keeps a
partial block in memory and writes each block as it fills; a file others
also have open is written through, each record on the disk with the end of
file moved past it before `$PUT` returns, so their streams see it.

The rest work on the current record: **`$FIND`** locates without copying;
**`$UPDATE`** rewrites the current record in place (the same length on a
sequential file; a stream file's record may change length, as on VMS);
**`$DELETE`** is `RMS$_IOP` on a sequential file, as on VMS; **`$TRUNCATE`**
ends the file at the current record; **`$REWIND`** starts the stream again;
**`$FLUSH`** writes what the stream holds. On a fixed-length sequential file
a program may also reach a record by its number (RAB$C_KEY, the number at
RAB$L_KBF; `rms/keyed.go`, `keyedput.go`).

### 9.8 Relative and indexed files

A **relative file** (`rms/relative.go`) is a row of numbered cells, one
record a cell, record 1 first. A program reaches a record by its number, or
in sequence, which passes over cells that hold none. Block 1 is the
prologue (the first data block, the maximum record number, the end of
file, a checksum); the cells follow in buckets of FAB$B_BKS blocks, each
cell a control byte (never used, a record, a deleted record) and room for
the largest record. No manual gives the layout; it was read from files VMS
7.1 made, and govax's are the same byte for byte. RMS reads and writes
the cells' blocks itself, through the `volume.File`, with no ods2 Reader.

An **indexed file** keeps its records in order of one or more keys, so a
program can ask for "the record whose part number is 1234". Its structure
is `internal/isam`'s, a package that knows nothing of RMS or of VAX memory:

- The file is divided into *buckets*, 1 to 63 blocks each, RMS's unit of
  I/O. The first blocks are the *prolog*: a *key descriptor* per key (where
  the key is in a record, its type, its root bucket) and an *area
  descriptor* per area (a set of buckets of one size).
- Each key has a tree of buckets: level 0 is the data level, and each level
  above it an index whose entries are "the highest key in that bucket
  below". Every level's buckets are also chained in key order, so reading
  in sequence never climbs the tree.
- Key 0, the primary key, keeps the records themselves in its level 0. Each
  alternate key keeps *SIDRs* there, a key value and pointers to the
  records having it.
- A record's RFA is the bucket it was first written into and an ID unique
  in that bucket, and never changes: when a split moves a record, the old
  bucket keeps an *RRV*, a record reference vector, saying where it went.
  That's why an alternate key's pointers, and an RFA a program kept, still
  work after the file grows.

`isam` finds, reads in order, adds (splitting full buckets, adding index
levels), updates, and deletes, on a `Blocks` interface. `rms/indexed.go` is
what RMS puts around it: a `$CREATE`'s FAB and XABKEYs and XABALLs checked in
VMS's order and turned into an `isam.Spec`, and the blocks isam reads and
writes, which are the volume file's. `indexedrecords.go` is the record
services on one: `$GET`, `$FIND`, `$PUT`, `$UPDATE`, `$DELETE`, and
`$REWIND` by key (RAB$B_KRF the key, KBF and KSZ its value, ROP's KGE and
KGT for "equal or greater" and "greater"), in sequence on the key of
reference, and by RFA. Only Prolog 3 files are read, by the author's
choice; an older prolog is `RMS$_PLV`.

### 9.9 Sharing a file

Several processes may have one file open at once, and RMS decides who may
do what with it in two layers, as VMS's does.

**RMS's rule** (`rms/sharing.go`) is the *Guide to OpenVMS File
Applications*' (7.1.2, Tables 7-3 and 7-4), by record operation: GET, PUT,
UPDATE, DELETE. An open *asks* for the operations its FAB$B_FAC names (GET
whenever it asks for any) and *allows* the others its FAB$B_SHR names (none
with FAB$V_NIL; with SHR 0, reading if it only reads and nothing if it
writes). A new opener may join if every operation it asks for is one each
current opener allows, and every operation each current opener has is one
it allows:

```go
// rms/sharing.go
func (a opener) compatible(b opener) bool {
	return a.access&^b.share == 0 && b.access&^a.share == 0
}
```

Each volume's entry in the mount table keeps every open file's list of
openers by FID (`claimOpen`).

**The file system's rule** (ods2's `volume.Access`) is coarser, as the ACP's
is: an accessor writes or doesn't, and may deny others reading or writing.
Every RMS open is also such an access, derived from its RMS access and
sharing (`accessMode`), and so is a program's own `IO$_ACCESS` through the
disk driver (section 8.5); this is the layer that arbitrates between the
two, `SS$_ACCONFLICT` for the `$QIO`.

While a file is accessed, every accessor shares one `volume.File`: one
header, one map, one end of file, as VMS's accessors share one *file
control block*. One process's appends and extensions are what the others
read, and a file deleted while open loses its directory entry at once but
keeps its blocks until its last accessor deaccesses it.

**Record locks** (`rms/recordlock.go`). In a file others may write (an SHR
with PUT, UPD, or DEL, and not FAB$V_UPI, which turns RMS's locking off),
RMS locks each record a stream reads, so another stream can't take it from
under it. The locks are real locks in the system's lock database (section
7.9's `lck`), as VMS's RMS takes them in VMS's lock manager: each stream's
record locks are sublocks of a null-mode lock on the file, named by its
volume and FID, the record's named by its RFA, taken in executive mode so
image rundown leaves them to the file's close. RAB$L_ROP says how:

| Option | Effect |
| --- | --- |
| (none) | exclusive: no other stream may lock the record |
| RAB$V_RLK | a write lock: others may still read it, without a lock |
| RAB$V_REA | a read lock: others may read-lock it too |
| RAB$V_NLK | no lock, only a query: `RMS$_RLK` if someone holds it exclusively |
| RAB$V_RRL | a locked record is read anyway, without a lock |
| RAB$V_WAT | wait for a locked record (`RMS$_OK_WAT`); with RAB$V_TMO, for at most RAB$B_TMO seconds (`RMS$_TMO`) |
| RAB$V_ULK | locks stay until `$FREE`, `$RELEASE`, or the close |

A refused lock is `RMS$_RLK`, and the record isn't consumed: the next `$GET`
tries it again. A waiting `$GET` waits as any service waits (section 7.6):
it returns `ErrWait`, its process waits in the scheduler, and the lock's
grant, delivered to the stream's notifier, makes it computable; the `$GET`
then runs again and finds the lock its own.

### 9.10 Blocks rather than records

A program that wants a file's blocks has three ways to them.

- **Block I/O** (`rms/blockio.go`): with FAB$V_BIO (or FAB$V_BRO and
  RAB$V_BIO), `$READ` and `$WRITE` move RAB$W_USZ or RAB$W_RSZ bytes of the
  file's virtual blocks from RAB$L_BKT, 0 meaning the block after the last.
  A record operation on such a stream is `RMS$_IOP`.
- **User file open** (`rms/ufo.go`): with FAB$V_UFO, `$OPEN` or `$CREATE`
  finds the file, checks access and sharing, and fills in the NAM and XABs,
  then steps aside: the file is left accessed on a new I/O channel, its
  number in FAB$L_STV, and the program reads and writes it with `$QIO`, or
  maps it with `$CRMPSC`. In govax the open's access moves from the file
  table to an `rms.ACPFile` (`rms/acp.go`), the same kind a disk
  `IO$_ACCESS` makes, which corevms puts on the channel.
- **The disk's `$QIO`** functions (section 8.5), on `rms/acp*.go`'s ACP
  routines: access by FID or by directory and name, read and write virtual
  blocks, create, delete, extend, and attributes, each over ods2, its errors
  turned into `SS$_` statuses.

### 9.11 Terminals, mailboxes, and process-permanent files

RMS reaches three things that aren't files:

- **The terminal** (`rms/terminal.go`), named TT:, TTA0:, or through a
  logical name: a `$GET` writes the prompt, RAB$L_PBF's RAB$B_PSZ bytes, if
  RAB$V_PMT asks for one, then reads up to a carriage return or line feed,
  which isn't part of the record, or until the user buffer is full; a
  Ctrl/Z with nothing before it is `RMS$_EOF`. It reads through the shared
  terminal's queue (section 7.12), so a process waits its turn. A `$PUT`
  writes a line, by the file's carriage control.
- **Mailboxes and NL:** (`rms/recdevice.go`), streams on the same Go state
  the mailbox and null drivers use (section 8.5): a `$PUT` is a message,
  done when it's read; a `$GET` takes one, `RMS$_EOF` at an end-of-file
  message, and NL: is always at its end.
- **Process-permanent files**, the files a process keeps from one image to
  the next (section 8.7). An image's `$OPEN` of SYS$OUTPUT opens nothing:
  its IFI is *indirect*, with the PPF bit set, and every `$GET` and `$PUT`
  through it works on the permanent entry's one stream
  (`rms/ppfaccess.go`, the guide's 6.6). `$DISPLAY` of one shows the file
  behind it; `$REWIND` is `RMS$_IOP`, `$ERASE` `RMS$_IFI`, and a `$GET` or
  `$FIND` of SYS$OUTPUT `RMS$_FAC`, as VMS 7.1's answers were.

### 9.12 Host files

A program's RMS reaches only volumes, the terminal, mailboxes, and NL:. A
name whose device has no volume mounted is `RMS$_DNR`, whatever files the
host has; a VMS program can't reach the host's file system by RMS.

govax's own commands can. DCL's utilities, MACRO, LINK, LIBRARY, and the
rest take file names a user types, which may mean the host
(`rms/location.go`'s `Session.Locate`, by the name's syntax):

1. a name given with `/HOST`, or with a `/` or `\` in it (which, typed,
   needs quotes), or a one-letter drive prefix (`C:`), is a host path;
2. a name in VMS's syntax, with a bracketed directory or a device or logical
   name of two or more characters before a colon, is a volume file (a
   logical name that leads to no mounted volume is an error, not a host
   file);
3. a bare name, `HELLO.MAR`, is a volume file when a SET DEFAULT onto a
   volume is in effect, and a host file otherwise.

`rms/recordfile.go` reads and writes records on either side
(`ReadRecordFile`, `CreateRecordFile`): a host file of text is lines; one of
an object module's records is ODS-2's own VAR layout, so its bytes are what
a copy of the volume file's blocks would be; an image is its blocks. COPY
gives a host file the record format the user names (`/RECORD_FORMAT` and its
companions; a host file can't say what records it holds).

Two things a program runs do reach the host: the C library's `open` and its
kin through the console's `EXE$OPEN` shim (arbitrated between processes by
RMS's sharing rule, by host file identity: `rms/hostshare.go`), and RUN of
an image, which may be a host file.

### 9.13 What's VMS's and what isn't

- **RMS is Go, not VAX code in executive mode.** There are no RMS
  structures in P1 space (no IFABs or IRABs, no I/O segment), no RMS
  buffers or global buffers, and no multiblock or multibuffer counts; RMS's
  share of the I/O counts (JPI$_BUFIO, JPI$_DIRIO) is a model fitted to one
  VMS run (`rms/iocount.go`).
- **Operations complete during the call**, with or without ASY, but for a
  `$PUT` to a mailbox. A pending `$PUT`'s status, and its completion
  routine, come at the `$WAIT`, not when the message is read. FAB$V_SYNCSTS
  and RAB$V_SYNCSTS (`RMS$_SYNCH`), from a later OpenVMS, aren't there.
- **Which status a second operation on a RAB with one pending gets** is
  unconfirmed: govax's is `RMS$_BUSY`, where VMS may give `RMS$_RSA`,
  "record stream currently active".
- **One stream per open file**: a second RAB on a FAB continues the first's
  stream, where VMS can keep several (FAB$V_MSE).
- **No file protection is checked**: a file's owner and protection mask,
  and its ACL, are kept and shown, and any process may open any file. A
  mailbox's protection is checked (section 7.9).
- **No network files** (a node name is `RMS$_SYN`), no journaling, no
  magnetic tape.
- **Indexed files are Prolog 3 only**, and the file system is a single
  volume: ods2 can mount a volume set, govax doesn't.
- **The bitmaps are written at DISMOUNT**, not as each allocation is made
  (9.1).
- **A program's RMS never reaches host files**; govax's commands do (9.12).

## 10. From VAX code to Go and back

A program running on govax is VAX code, and almost everything it asks of
VMS is answered by Go: the system services, RMS, and most of the run-time
library routines. This section is about the crossing. A program never
knows it happens: it calls `SYS$ASSIGN` or `LIB$GET_EF` at the address a
VMS program would, with the argument list a VMS program would build, and
gets back the status and results VMS would give. In between, the engine
stops running instructions, a Go function reads the argument list out of
VAX memory, does the work, and writes its answers back.

The crossing goes the other way too. A Go service sometimes has to run
VAX code before it can finish: `$EXIT` calls the program's exit handlers,
the condition dispatcher calls its condition handlers, `LIB$FILE_SCAN`
calls its success routine for each file. And Go can end the program
altogether. This section takes each in turn: the instruction that
crosses (10.1), the system service vector (10.2), a service in Go (10.3),
waiting and calling back (10.4), the run-time libraries (10.5) and their
callbacks (10.6), conditions (10.7), and the image's end (10.8).

### 10.1 XFC

The VAX architecture reserved opcode `FC`, XFC, "Extended Function
Call", for a customer's own microcode: on a VAX without any, it takes the
*opcode-reserved-to-customers* fault through SCB vector `^X14`. govax
gives it a meaning. Its second byte, an implicit operand (section 4.1), is
a *selector*, and the engine's XFC handler (`cpu/xfc.go`) does what the
selector says:

| Selector | Name | What it does |
| --- | --- | --- |
| `01`–`04` | `XFC$CONSOLE_WRITE`, `_READ`, `_CMD`, `_PUT` | the console's I/O (section 8.6) |
| `05` | `XFC$JSBSHIM` | a run-time library routine reached by JSB (10.5) |
| `78` | `XFC$QUIT_EMULATION` | end govax (kernel mode only) |
| `7A` | `XFC$P1VECTOR` | a system service (10.2) |
| `7B`, `7C` | `XFC$HALT_SILENT`, `XFC$HALT` | halt the CPU, quietly or with a message |
| `7D` | `XFC$SHIM` | a run-time library routine reached by CALLS or CALLG (10.5) |
| `7E`, `7F` | `XFC$VMW`, `XFC$VMR` | translate the address in R0 for writing or reading, into R0, PSL<V> set if it can't be |

Any other selector is the customer-reserved fault, as the architecture
suggests for "an unrecognized second byte of the instruction"; the
program's handlers see it as `SS$_OPCCUS` (10.7).

The engine itself knows nothing of VMS. Everything but the halts and the
translation goes through the `cpu.SystemServices` interface
(`cpu/services.go`), which `machine.Machine` implements: it answers the
console's selectors itself, and passes the services and the run-time
library routines, through its embedded `corevms.EngineHooks`, to whichever
process the CPU is running (section 3.4). So an XFC is a Go call, and
when it returns, the XFC instruction has one of five outcomes:

- **Done.** R0 is set to the status the Go side returned, and execution
  goes on at the next instruction. This is nearly every call.
- **Wait.** The Go side can't finish yet (`cpu.ErrServiceWait`). The
  handler puts PC back to the XFC, so the next `Step` runs it again;
  between tries, time moves, interrupts and ASTs are delivered, and with
  the scheduler the process waits in it (section 7.6).
- **Call.** The Go side needs a VAX procedure run first (a
  `cpu.ServiceCall`). The engine builds the procedure's call frame, with
  the XFC as its return address, and jumps to it; when the procedure's
  RET lands on the XFC, the Go side is called again (10.4).
- **Exit.** The image has ended (`cpu.ErrImageExit`): R0 is its status,
  and the engine unwinds to the frame that started it (10.8).
- **Fault.** The Go side met memory it couldn't reach: a `vm` translation
  fault, which `raise` turns into the access violation the XFC
  instruction itself takes, PC back at the XFC, as any instruction's
  would be (section 4.5). Any other Go error stops the run, and the
  console shows it: a defect in govax, not something a program can cause.

No mode changes along the way. A VMS program's service call changes mode
twice, into kernel or executive mode by CHMK or CHME and back by REI; in
govax the CPU stays in the caller's mode while the Go code runs, and the
service's reads and writes of the caller's memory are checked against
that mode's access, as VMS's services check them with PROBE.

### 10.2 The system service vector

VMS puts its services at fixed addresses in P1, eight bytes apart, from
`^X7FFEDE00` up: `SYS$ASSIGN` is `^X7FFEDE50` on every VMS since V4, and
a program linked on one version runs on the next because the address
never changes. `vmsdef.P1VectorTable` (`vmsdef/p1vector.go`) lists 312
entries, VMS's whole vector. The microkernel's `.P1VECTOR` directive
(section 5.6) writes the same procedure at each:

```text
SYS$ASSIGN:     entry mask ^M<>        ; 00 00
7FFEDE52:       XFC      #7A           ; FC 7A
7FFEDE54:       RET                    ; 04
```

(that is the debugger's `EXAMINE/INSTRUCTION` of the vector on govax).
A program's `CALLS #4,G^SYS$ASSIGN` builds its frame and lands on the XFC,
two bytes in. Where VMS's vector has a `CHMK #code`, govax's has the XFC,
and that's the whole of the difference a program could see.

The XFC's handler passes the vector entry's address, PC less 4, and
corevms finds the service's name by it (`lookupP1Vector`). Three entries
are different. `SYS$SRCHANDLER`, where the condition dispatcher sends a
program (10.7), is reached by a jump, not a call: its XFC is at the
entry's own address, with no entry mask before it. And `SYS$GL_ASTRET`
and `SYS$GL_COMMON` are data cells, longwords that share the vector to get
an address, inside their neighbors' procedures: `.P1VECTOR` defines their
symbols and writes nothing for them.

Images linked before VMS V4 call their services through an older vector,
in S0 from `^X80000000`, at the same offsets. govax keeps it too
(`machine/legacyvector.go`): at each offset an empty entry mask and a
`JMP @#` to the P1 entry's XFC, so the call is dispatched by the P1
address as any other (section 5.5's first row).

### 10.3 A service in Go

A service is a Go function of one shape (`corevms/service.go`):

```go
type ServiceFunc func(env *Environment, argv []uint32) (uint32, error)
```

`env` is the calling process (section 3.5), `argv` its argument list
already read out of memory, and the `uint32` the status for R0. Each
service is registered by name in the System's `ServiceTable`, by a
`register...Services` function in the file that implements it (122
services, from `registerCoreServices` to `registerRMSServices`). Here is
`$GETTIM`, whole (`corevms/vmstime.go`):

```go
func serviceSysGettim(env *Environment, argv []uint32) (uint32, error) {
	timadr := optArg(argv, 0)
	if timadr == 0 { // page 0 is never accessible on VMS
		return ssAccVio, nil
	}

	if !env.storeQuad(timadr, env.Clock()) {
		return ssAccVio, nil
	}

	return ssNormal, nil
}
```

`optArg` gives 0 for an argument the list doesn't have, which is how VMS
reads an omitted optional argument. The time is written with the caller's
access, and an address the caller can't write is `SS$_ACCVIO`, returned:
a VMS service probes what it's given and returns that status, rather than
faulting. Statuses are VMS's own values, from `vmsdef.Symbols`
(`SS$_ACCVIO` is 12).

**The dispatch.** `Environment.SystemService` (`corevms/environment.go`)
does what VMS's change mode dispatcher does before a service runs (*VAX/VMS
Internals and Data Structures*, 9.3.1), then calls it:

1. It finds the entry by its address, and the service by the entry's name.
   An entry with no service behind it signals `SS$_UNSUPPORTED`, followed
   by `GOVAX-E-NOTIMPL` naming the service (`corevms/missingservice.go`):
   an error, not a severe one, so with no handler to take it the program
   goes on, the call returning `SS$_UNSUPPORTED`.
2. It checks that the caller's mode can read the whole argument list: the
   count longword, then as many longwords as its low byte says (the
   calling standard reserves the other three bytes). Like VMS's PROBER, it
   looks only at the pages' protection (`argListReadable`). A list that
   can't be read is `SS$_ACCVIO`, and the service isn't called.
3. It reads the list (`readArgs`), and for the services VMS was seen to
   count, checks there are enough (`serviceMinArgs`, `corevms/argcount.go`):
   `$ASSIGN` with three arguments is `SS$_INSFARG`, as on VMS 7.1, though
   its fourth is optional.
4. It calls the service through `callHandler`, which turns a Go panic into
   an error, so a service that indexes past a short `argv` stops the run
   with a message instead of taking govax down.
5. A service that waits is noted for the scheduler (`enterWait`; section
   7.6), and with `SET DEBUG SERVICES` the call is traced, once, however
   many times a wait makes it run.
6. A service that failed, in a mode that asked for system service failure
   exceptions, signals `SS$_SSFAIL` instead of simply returning (10.7).

One service, `SYS$CLRAST`, isn't called with an argument list at all
(`RegisterNoArgs`): an AST routine's RET reaches it (section 7.7), as
`SYS$SRCHANDLER` is reached by a jump. Neither reads AP.

**A run, traced.** Here is a program that calls `$GETTIM`, `LIB$GET_EF`,
and `$DCLEXH`; signals a warning with LIB$SIGNAL, which its handler
continues; makes an access violation, which the handler continues past;
calls `$GETTIM` with an argument list on page 0, which a program can't
read; and calls `$EXIT`, whose exit handler prints a line. It was run
with `SET DEBUG SERVICES` and `SET DEBUG EXCEPTIONS`:

```text
DEBUG(SERVICES): SYS$GETTIM( 00000200 ), returns 00000001
DEBUG(SERVICES): SYS$DCLEXH( 00000210 ), returns 00000001
DEBUG(EXCEPTION): LIB$SIGNAL, CONDITION=00000028  PC=00000439  SIGNAL=7FDFFFA0
DEBUG(EXCEPTION): condition handler 00000454, frame 7FDFFFD0, depth 0
DEBUG(SERVICES): SYS$SRCHANDLER(  ), calls 00000454
handler called
DEBUG(SERVICES): SYS$SRCHANDLER(  ), returns 00000001
DEBUG(EXCEPTION): SET, CODE=20  PC=00000439  PSL=03C00000  ARGC=2
DEBUG(EXCEPTION): ARGS = 00000004, 00000000
DEBUG(EXCEPTION): DISPATCH, CONDITION=0000000C  PC=00000439  SIGNAL=7FDFFFB8
DEBUG(EXCEPTION): condition handler 00000454, frame 7FDFFFD0, depth 0
DEBUG(SERVICES): SYS$SRCHANDLER(  ), calls 00000454
handler called
DEBUG(SERVICES): SYS$SRCHANDLER(  ), returns 00000001
DEBUG(SERVICES): SYS$GETTIM, argument list at 00000004 can't be read, returns 0000000C
DEBUG(SERVICES): SYS$EXIT( 00000001 ), calls 0000047C
in the exit handler
DEBUG(SERVICES): SYS$EXIT( 00000001 ), exits with status 00000001
```

`LIB$GET_EF` isn't in the trace: it isn't a system service but a shim
(10.5). The rest of this section explains the other lines: the handler
called from `SYS$SRCHANDLER` and its "returns 1", the continue (10.7), and
`$EXIT`'s two calls, one to run the exit handler and one to end (10.8).

### 10.4 Waiting, and calling VAX code

A service that must wait returns `corevms.ErrWait`, which `EngineError`
(`corevms/enginehooks.go`) turns into the engine's `cpu.ErrServiceWait`,
and the XFC runs again until the service no longer waits. Section 7.6 has
the rest: what the scheduler does with the process meanwhile, and how the
wait ends.

A service that needs VAX code run returns a `*corevms.CallRequest`
(`corevms/exit.go`): a procedure's entry mask address and an argument
list's, as for CALLG. `EngineError` makes it a `cpu.ServiceCall`, and the
engine's `callForService` (`cpu/exit.go`) builds the procedure's frame as
CALLG would, the saved PC the XFC's own address. When the procedure
returns, its RET puts PC on the XFC, and the next `Step` calls the service
again. R0 is left as the procedure set it.

That second call has to know it's a second call, and where it left off.
Each service that calls out keeps that in Go, and recognizes its return
by the registers the RET restored:

| Service | What it calls | How it knows the call has returned |
| --- | --- | --- |
| `$EXIT` | each exit handler, newest first | the process is exiting, and SP is where the first `$EXIT` had it |
| `SYS$SRCHANDLER` | each condition handler, `$UNWIND`'s cleanup calls | SP is the dispatch's arrays' (10.7) |
| `$CMKRNL`, `$CMEXEC` | the program's routine, in kernel or executive mode | FP is the service's stub's frame, and SP where the routine's frame was built |
| `$PUTMSG` | the program's action routine, for each message line | the same |
| the shims that call back | a routine the program passed (10.6) | FP, SP, PC, and mode the same as when the callback was made |

`$CMKRNL` and `$CMEXEC` are the one place a service changes mode: they
switch the CPU into the target mode and its stack before asking for the
call, and back when the routine returns (`corevms/cmode.go`), as AST
delivery does.

Two variants. A `Tail` call is the service's last act: the frame's saved
PC is the instruction after the XFC, so the procedure returns to the
service's caller with its own R0 as the result. `LIB$SHOW_TIMER` calls its
action routine this way, and `Environment.ExitFromShim` uses it to call
the `SYS$EXIT` entry from a shim. An `Image` call starts an image: its
frame's saved PC and FP are the console's sentinel, `^XFFFFDEAF`, so the
image's `$EXIT` unwinds to it and no further (10.8). The subprocess CLI
runs each command's image this way (section 7.11).

### 10.5 The run-time libraries

VMS's run-time libraries are shareable images of VAX code: LIBRTL.EXE
(the LIB$ and STR$ routines), MTHRTL.EXE (mathematics), LBRSHR.EXE (the
librarian's routines), the C library, and the rest. A program linked
against one calls its routines through the image's *transfer vector*, a
table of entry points at fixed offsets from the image's start.

govax writes most of them in Go instead. Each routine is a *shim*: a Go
function of the service's shape (`corevms.ShimFunc`), registered under a
number, its *code*, and reached through a stub, a twelve-byte procedure in
system space (`corevms/shimlink.go`):

```text
LIB$GET_EF:     entry mask ^M<>        ; 00 00
80028BFA:       MOVL     I^#00000072,R0  ; D0 8F 72 00 00 00 50
80028C01:       XFC      #7D           ; FC 7D
80028C03:       RET                    ; 04
```

The stub puts its code in R0, and `XFC$SHIM` runs the shim registered
under it, with the caller's argument list at AP; the shim's status comes
back in R0, and RET returns to the caller. The dispatch,
`Environment.Shim`, is `SystemService`'s with a code in place of an
address, and a shim can wait, call, or exit as a service can.

**Where a stub is.** `ShimTable` (`machine/shim.go`) is every routine
govax provides, as `corevms.ShimLink`s: a name, the library, the offset
in its transfer vector, and the code. At the first RUN, `EnsureShims`
writes a stub for each: the first 42 into the page INITIALIZE/MEMORY set
aside for them (section 5.5), the rest into pages from the S0 pool, so
every process reaches a stub at the same address. Each stub's address is
also a symbol, `SHIM$<library>_<offset>` (`LIB$GET_EF`'s is
`SHIM$LIBRTL_000003B0`).

**How a program finds it.** LINK turned the program's `CALLS
#1,G^LIB$GET_EF` into a call through a longword in the image's fixup
section, holding the routine's offset in LIBRTL's transfer vector. When
the image is activated, the image activator (`imageFixup`,
`machine/image.go`) writes the routine's address into that longword: the
loaded image's base plus the offset, if a real LIBRTL.EXE was loaded, or
else the `SHIM$LIBRTL_<offset>` stub. The program's code never changes.
An image linked against a *based* library, one LINK placed at a fixed
address, gets demand-zero pages there holding a transfer vector of its
own, each entry an entry mask and a jump to the routine's stub
(`placeEmulated`, `machine/activate.go`).

**Codes.** The codes say whose a routine is:

| Codes | Routines |
| --- | --- |
| 1 to about 150 | LIBRTL's and LIBRTL2's routines (`librtl.Routines`), the C library's and govax's own (`corevms.CRTLLinks`); 64 is DCL's `EXE$CLI_COMMAND`, the subprocess CLI's stub (section 7.11) |
| 1000, 2000 on | every LIBRTL entry govax lacks, CALL and JSB: a stub that signals `SS$_UNSUPPORTED` and `GOVAX-E-NOTIMPL` with its name (`librtl/missing.go`) |
| 3000 | the traceback handler, `EXE$TRACEBACK` (10.7) |
| 4000 on | MTHRTL's, 4000 plus the entry's offset over 8 (`internal/mthrtl`) |
| 5000 on | LBRSHR's, 5000 plus the entry's offset (`internal/lbrrtl`) |
| 10000 on | a routine of a library govax stands for that no table has, made on first use when an image's fixup names it, which signals NOTIMPL with the library and offset (`machine/unresolved.go`) |

**Three other kinds of stub.** A routine reached by JSB has its arguments
in R0, R1, ..., so R0 can't carry its code: its stub pushes the code,
`XFC$JSBSHIM` pops it, and the shim (a `corevms.JSBShimFunc`) works on R0
to R7 in place, returning by RSB. LIB$SIGNAL and LIB$STOP need their
caller's R0, which a handler sees in the mechanism array: their stub
pushes R0 before it loads the code (`CallerR0Stub`). And a handful of
routines whose whole job is a transfer of control are VAX code in the
microkernel (`LIB$CALLG`, `DECC$STRCPY`, and a few more): code 0, no stub,
the `SHIM$` symbol the routine's own address. EXE$ASTRET, the AST exit
(section 7.7), takes a stub's slot after the table's last.

**Not every library is Go.** The image locator (section 11) finds govax's
own libraries unless a logical name of the library's name is defined, so a
real VMS LIBRTL.EXE or FORRTL.EXE can be run in their place; such a
library's own calls to LIBRTL land on the shims all the same. FTNRTL.EXE,
the FORTRAN run-time library, is VAX code govax carries: MACRO-32 sources
assembled and linked by govax's own MACRO and LINK (`internal/ftnrtl`),
found by the locator last, and calling LIBRTL's and MTHRTL's shims and
the services as any program does.

**Access violations.** A LIB$ routine on VMS is VAX code, so an argument it
can't reach faults in its own instruction, and the program's handlers see
the hardware's `SS$_ACCVIO`. A shim does the same through
`librtl.memoryFault`: the access violation is signaled with the translation
fault's own reason mask and address, and if a handler continues, the
routine returns `SS$_ACCVIO`. A service, as 10.3 said, returns it instead.
An argument list a shim can't read is an access violation at the stub's
XFC.

### 10.6 When a library routine calls the program

Some LIB$ routines call procedures the program passes them:
`LIB$FILE_SCAN` its success routine for each file, `LIB$DELETE_FILE` its
confirm routine before each deletion, `LIB$TPARSE` its action routines,
`LIB$GET_VM` a zone's own get-page routine. Each callback is a
`CallRequest`, as in 10.4. But a routine like `LIB$DELETE_FILE` calls out
from the middle of a loop, from several places, and writing it as a state
machine re-entered after each callback would be hard to follow.

So a shim registered with `ShimTable.RegisterCallback` runs on a goroutine
of its own (`corevms/callback.go`), and a callback is a plain Go call,
`Environment.Callback(routine, args...)`, which returns the procedure's R0.
Behind it, the goroutine and the engine take turns over two channels:
`Callback` hands the dispatch its request and waits; the dispatch returns
the `CallRequest` to the engine; when the procedure's RET runs the stub's
XFC again, the dispatch hands the goroutine the R0 and waits for its next
request or its result. Only one of the two runs at a time, so the machine
stays deterministic and nothing needs a lock. The callback's argument
list goes on the stack below SP, in the stub's frame.

A shim's XFC chooses its routine by R0, which the callback has just set to
its own result, so the dispatch first looks for a routine waiting for a
callback whose stub frame this is: FP, SP, PC, and mode the same as when
the callback was made. Nothing else can execute that XFC with that frame;
a callback that calls the same routine again does so through a new frame.

A callback may never return: a condition handler may unwind past the
stub's frame, or the image may exit. The routine is then *abandoned*: its
pending `Callback` panics with a sentinel its goroutine recovers, so the
routine's own code needs no check, and the goroutine ends. `$UNWIND`
abandons the calls whose frames it removes, image rundown all of them.

### 10.7 Conditions

A condition reaches a program's handlers from three places: a hardware
exception, a routine that signals (LIB$SIGNAL and LIB$STOP, or a shim
signaling its own failure), and govax itself (a missing service or
routine). All three end in one mechanism, which runs in the program's own
mode and stack, as VMS's does (`corevms/condition.go`).

**An exception.** The microkernel sets the SCB vectors of the exceptions a
program can handle to `console$handler`, `FFFFFFFF`: the privileged and
customer-reserved instructions, reserved operand and addressing mode,
access violation, translation not valid, trace, breakpoint, arithmetic,
and CHMS and CHMU. `HandleFault` offers such an exception to the running
process's `DispatchException` (section 4.5), which turns it into a
condition by *VAX/VMS Internals and Data Structures*' Table 4-2:

| SCB vector | Exception | Condition | Detail in the signal array |
| --- | --- | --- | --- |
| `10` | privileged or reserved instruction | `SS$_OPCDEC` | |
| `14` | customer-reserved instruction (an unknown XFC) | `SS$_OPCCUS` | |
| `18` | reserved operand | `SS$_ROPRAND` | |
| `1C` | reserved addressing mode | `SS$_RADRMOD` | |
| `20`, `24` | access violation; a page that can't be made valid | `SS$_ACCVIO` | the reason mask and the address |
| `28` | trace | `SS$_TBIT` | |
| `2C` | BPT | `SS$_BREAK` | |
| `34` | arithmetic | `SS$_INTOVF`, `SS$_FLTDIV_F`, ... by its type code | |
| `48` | CHMS, with no change mode handler | `SS$_CMODSUPR` | the change mode code |
| `4C` | CHMU, with no change mode handler | `SS$_CMODUSER` | the change mode code |

**The dispatch.** `startDispatch` lays three blocks out on the current
stack, just as VMS's exception code leaves them:

```text
  SP →  argList:  2, sig, mech            the handlers' argument list
        mech:     4, frame, depth, R0, R1 the mechanism array
        sig:      n, condition, detail..., PC, PSL
```

records the dispatch on the process's stack of dispatches in progress, and
sets PC to `SYS$SRCHANDLER`. The program "returns" there, and that
entry's XFC runs `serviceSysSrchandler`, the search. It goes through the
mode's primary and secondary exception vectors (`$SETEXV`), then the call
frames from the condition's outward, each frame's handler at `0(FP)`, its
caller at `12(FP)`; each handler found is a `CallRequest` with the
mechanism array's frame and depth filled in. The handler's RET lands on
`SYS$SRCHANDLER`'s XFC again, with SP at the arrays, which is how the
search knows which dispatch it is continuing, and it acts on the
handler's R0:

- **Low bit set (`SS$_CONTINUE`).** The program goes on: for an exception,
  at the signal array's PC (which a handler may change, as the traced
  program's did to skip its faulting MOVL), its condition codes from the
  array's PSL; for LIB$SIGNAL, by returning from LIB$SIGNAL; R0 and R1 from
  the mechanism array. The trace's "returns 00000001" is this.
- **Low bit clear (`SS$_RESIGNAL`).** The search goes on to the next frame.
- **A `$UNWIND` was called.** Each frame to be removed has its handler
  called once more with `SS$_UNWIND`, then each removed frame's saved PC
  is pointed at a RET, so the frames go one by one as their own RETs would
  take them (`corevms/unwind.go`).

A handler that signals or faults starts a dispatch of its own on top of
the first, whose search passes over the frames the first already searched
(the book's 4.3.3).

**Change mode handlers.** VMS uses CHMK and CHME for its services and
leaves CHMS and CHMU to programs: a program, or a command interpreter in
supervisor mode, declares a *change mode handler* for the mode it calls
`$DCLCMH` from (`corevms/changemode.go`), and a CHMS then goes to the
supervisor-mode handler, a CHMU to the user-mode one. The engine asks the
running process for one before it dispatches the condition
(`cpu.ChangeModeDispatcher`), and delivers the trap to it as the hardware
delivers any: in the trap's mode, on its stack, the code on top, then the
PC of the next instruction and the PSL. The handler pops the code and
returns with REI. With no handler, the trap is the condition in the table
above, reported to the mode that executed the CHMx. Image rundown forgets
the user-mode handler; a supervisor-mode one lasts as long as the process.

**System service failure exceptions.** A program that would rather not
test every call's status can ask, with `$SETSFM`, for a failing service to
signal `SS$_SSFAIL` instead. VMS keeps a bit per access mode; `$SETSFM`
sets the caller's, `$CREPRC` with `PRC$M_SSFEXCU` starts a process with the
user-mode one set, and image rundown clears it. VMS raises the exception
in the exit path of the services that change mode, which RMS's reach too
when they fail: an error or severe status (a warning doesn't count) from a
mode whose bit is set (the book's 9.3.5 and 9.3.6.2). govax raises it in
`SystemService` after the service returns (`failureException`), except for
the services VMS runs in the caller's mode (`$FAO`, `$GETMSG`, `$PUTMSG`,
`$UNWIND`, and a few more), which never pass through that path. The signal
array is `SS$_SSFAIL`, the status, and the PC of the vector entry's RET and
the PSL; a handler that continues it has the service return, with R0 from
the mechanism array, the status unless the handler changed it. Unhandled,
`SS$_SSFAIL` is severe, and the catch-all ends the image.

**The end of the search.** When the frames run out (the frame whose saved
FP and PC are the console's sentinel), the *catch-all* acts: it writes the
condition's message where `$PUTMSG` writes it, and for a severe condition
(or any LIB$STOP) ends the image by calling the `SYS$EXIT` entry with the
condition, marked `STS$M_INHIB_MSG` so it isn't shown twice, as the
status; for any other condition the program continues. An image linked
with traceback has one frame more: RUN's driver puts the traceback handler,
`EXE$TRACEBACK` (shim code 3000, `corevms/traceback.go`), in its own frame,
as VMS's image startup does: for every warning, error, and severe
condition the program's handlers pass on, it writes the message with a
symbolic dump of the stack, then continues the program, or for a severe
condition ends the image. A frame that can't be read ends the search at the
last-chance vector instead, and an exception the user stack has no room to
report ends the image with no handler called (the book's 16.2.1.3).

Only if `DispatchException` can't take an exception at all (no condition
for it, the interrupt stack, no `SYS$SRCHANDLER` stub) does it go back to
the console, which reports it and stops the run.

### 10.8 How an image ends

RUN doesn't call an image's main routine directly. It writes a small
procedure, the *IMAGE$INIT driver*, and calls that from a frame whose saved
PC and FP are the sentinel (`buildDriver`, `machine/run.go`; section 11
has the rest):

```text
        .WORD   ^M<>
        MOVL    #EXE$TRACEBACK,(FP)     ; if the image was linked with traceback
        CALLS   #0,main                 ; the image's transfer address
        PUSHL   R0                      ; what main returned
        CALLS   #1,SYS$EXIT             ; is the image's status
        RET
```

So an image that returns from its main routine ends by `$EXIT` too, as on
VMS, whose image activator calls `$EXIT` with what main returned.
`$EXIT` (`serviceSysExit`) first calls the caller's mode's exit handlers,
one `CallRequest` each, the block removed from the list before its call;
when none are left it returns `ErrExit` with the status. The engine's
`exitImage` then follows the frames from FP to the sentinel frame and
executes its RET, which restores the registers RUN saved and reports
`cpu.ErrConsoleCallReturned`, as if the outermost procedure had returned.
R0 is the exit status.

The machine's run loop takes it from there (`ReportStop`,
`machine/execute.go`): the status becomes RUN's, and `$STATUS`, and the
machine runs the image down. `Environment.ImageRundown`
(`corevms/process.go`) does what VMS does at image exit for the state
corevms owns: it closes the files the image left open, deassigns its
user-mode channels and deallocates its devices, disassociates its event
flag clusters, cancels its timers and wakeups, dequeues its user-mode
locks, discards its pending user-mode ASTs, forgets its exit handlers, the
`$PUTMSG`, `$CMKRNL`, and library calls left waiting for a routine that
never returned, its condition dispatches and user-mode exception vectors,
and its page locks, deletes P0 (unless a debugger session holds the image)
and the image's P1 pages, resets its privileges and working set, and frees
its heap and its run-time libraries' per-image data. The machine deletes
the user-mode logical names (`DEFINE/USER_MODE`'s) too.

An image another process runs ends the same way, but `StepMachine` then
deletes the process, or, if its CLI ran the image, the CLI goes on
(section 7.11). An image that never exits (a HALT, Ctrl/C, Ctrl/Y, a
limit) is run down when the next image starts, as VMS runs it down when
the process next runs one (`EndAbandonedImage`).

### 10.9 What's VMS's and what isn't

- **Services run in Go, in the caller's mode.** There's no change mode
  dispatcher, no kernel or executive stack in use during a service, and no
  inner-mode code a program could trace: a program that traces itself
  through a service sees the vector's XFC and RET. `$CMKRNL` and `$CMEXEC`
  do change mode, to run the program's routine.
- **Which services raise `SS$_SSFAIL`** is the Internals book's list of
  the services that change mode, with RMS's; services VMS added later are
  assumed to change mode too. Not checked on VMS.
- **A CHME or CHMK from a program** reaches no VMS dispatcher: a CHME's
  SCB vector isn't set, which stops the console, where VMS would return
  `SS$_ILLSER` for a code it doesn't know; a CHMK reaches the microkernel's
  own dispatcher (section 8.6).
- **No compatibility mode.** `$DCLCMH` keeps a compatibility mode handler,
  which nothing calls; and a call from kernel or executive mode is
  `SS$_IVSSRQ`, govax's choice where the manual says only that the service
  is for user and supervisor modes.
- **Most run-time library routines are Go**, so a program can't step into
  them, and a signal from one is raised at its stub's PC, in its caller's
  frame for a JSB routine. A real library can be named instead (10.5).
- **Only the caller's mode's exit handlers run** at `$EXIT`: govax's
  images have no supervisor- or executive-mode ones.
- **A missing service or routine signals** `SS$_UNSUPPORTED` with
  `GOVAX-E-NOTIMPL`, where VMS would have the routine; the program runs up
  to the call.

## 11. Starting the system

A real VAX starts VMS from its console. The console program in the boot
ROM initializes the processor, finds a boot device, and loads VMB, the
primary bootstrap, which sizes memory and loads SYSBOOT; SYSBOOT reads the
SYSGEN parameters and loads the executive; the executive initializes
itself and creates the swapper and then a STARTUP process, which runs
`SYS$SYSTEM:STARTUP.COM` and, last, the site's `SYSTARTUP_VMS.COM`; only
then can anyone log in.

govax has the same pieces in a different order, because the console *is*
the program the user runs. The Go program starts first, with a console and
no machine; it mounts the system volume and reads its grammars; and then
process 1, the console's own process, runs `STARTUP.COM` as its login
command procedure. That procedure builds the VAX with console commands:
memory, the address space, the microkernel, the devices. After it come the
page file, the user's own startup, and the user's commands. Each step
measured below is from a session started on a fresh system volume.

| Step | Where | What |
| --- | --- | --- |
| 1 | `cmd/govax` | the command line, the configuration profile, the terminal |
| 2 | `console.New`, `Console.Init` | a console, a DCL interpreter, and a 1 MB machine with no address space |
| 3 | `dclcmd.MountSystemVolume` | the system volume made or refreshed, mounted as `DUA0:`, and its logical names |
| 4 | `cmd/govax/grammarfiles.go` | the console's and the debugger's grammars |
| 5 | `cmd/govax` | the `--mount` volumes and the default volume |
| 6 | `STARTUP.COM` | the system names, `INIT/VAX`, `INITIALIZE/MEMORY`, the ROM, the microkernel and its initialization, the devices, `SYSTARTUP_VMS.COM` |
| 7 | `Machine.StartPageFile` | the page file, with paging on |
| 8 | `vax.startup.procedure` | the user's own procedure, if one is configured |
| 9 | `Console.RunCommandLine`, the prompt | the one-shot command, or the user's commands |
| 10 | `Console.EndSession` | process 1's files closed, every volume dismounted |

A start on an existing system volume takes 20 to 30 ms on the development
machine; making the volume the first time adds about 190 ms.

### 11.1 The program and its console

`main` (`cmd/govax/main.go`) hands the command line to `tucats/gopackages`'s
`app-cli` package, which reads the options (`cmd/govax/grammar.go`), the
subcommand (`console` unless another is named), and the configuration
profile, a set of `vax.*` settings in `~/.govax/govax.json` (`--profile`
picks one). Every setting govax reads is listed in `validConfigs`, and
`auditConfig` warns of a setting in the profile that isn't, which is
usually a misspelling. A subcommand other than `console` becomes a console
command: `govax run hello.exe` is the console's `RUN "hello.exe"`, left as
the *one-shot command* for step 9.

`run` then makes the console (`console.New`), which is two things over one
machine: a `machine.Machine` (`NewMachine`, `machine/machine.go`) and DCL's
`dclshell.Interpreter`. The machine is made with its device table, its
mount table, its RMS session, and process 1's logical name database, with
its four process-permanent names already defined (SYS$INPUT, SYS$OUTPUT,
SYS$ERROR, SYS$COMMAND, each `_GOVAX$TTA0:`; section 8.7). These outlive
every machine `INIT` makes: a volume mounted, a logical name defined, or a
DCL symbol set before `INIT` is still there after it. The interpreter makes
process 1's DCL, its `Shell`, and puts DCL's hooks in the machine (where a
command's output goes, the default directory, `$STATUS`, Ctrl/Y). At a
terminal, `run` also puts the terminal in VMS's mode, so that Ctrl/C,
Ctrl/Y, and Ctrl/Z mean what they mean on VMS (`cmd/govax/attention.go`;
section 7.13).

Then it makes a small machine: `Console.Init(minimumVAXMemory)`, 2048
pages, 1 MB, with memory mapping off. It exists only so that commands that
need a machine work before `STARTUP.COM` makes the real one; the system
volume's mount, which comes next, is Go code that needs none.

### 11.2 The system volume

The system volume is govax's system disk: an ODS-2 volume in a container
file, by default `~/.govax/govax-system.dsk`, mounted on `DUA0:` with the
label `GOVAXSYS` every time govax starts. It holds what VMS's system disk
holds for the same purposes, in the same directories, as files a user can
read, edit, and rebuild with DCL's own commands
(`internal/sysvol/files.go`'s table, written out by `Files()`):

| Directory | Logical name | What |
| --- | --- | --- |
| `[SYSTEM.SYSEXE]` | SYS$SYSTEM | `DEFAULT.ROM`, the console ROM; `PAGEFILE.SYS`, the default page file (made by the pager, not supplied) |
| `[SYSTEM.SYSMGR]` | SYS$MANAGER | `STARTUP.COM`, `SYSTARTUP_VMS.COM`, the grammars `CONSOLE.DCL` and `DEBUG.DCL` |
| `[SYSTEM.SYSHLP]` | SYS$HELP | `HELPLIB.HLB`, `DBG$HELP.HLB`, and their `.HLP` sources below it |
| `[SYSTEM.SYSLIB]` | SYS$LIBRARY, SYS$SHARE | `STARLET.MLB`, `FTNRTL.EXE`, `FORSYSDEF.TLB` |
| `[SYSTEM.SOURCES]` | SYS$SOURCES | `KERNEL.ASM` and `SSDEF.ASM`, FTNRTL's and STARLET's sources, and the procedures that rebuild them |

On the fresh volume of the measurement, that was 124 files in 8
directories. Every file comes from govax itself: `internal/bootdata`'s
embedded files, `internal/ftnrtl`'s image and sources, and the FORTRAN
definition library `internal/fortran/sysdef` builds.

**Mounting it** is `dclcmd.Context.MountSystemVolume`
(`dclcmd/sysvolume.go`), which `run` calls before anything else, since it
needs no grammar: it's RMS's and `ods2`'s work in Go. In order:

1. It finds the container (`--system-file`, or `vax.default.system.file`,
   or the default), or makes one, an RD54 initialized with the label
   `GOVAXSYS` (`InitializeContainer`, as `INITIALIZE/CONTAINER` makes one).
   `--system-initialize` makes it again, keeping the old one as `FILE.old`.
   `--system-file none` means no system volume.
2. It takes a host lock on the container (`internal/hostlock`). A second
   govax session finds the lock held and mounts the volume read-only
   (`%MOUNT-I-SYSREADONLY`), so two sessions never write one volume.
3. It mounts the volume on its device (`--system-device`, `DUA0` by
   default; a controller alone, `DUA`, is its next free unit), checks the
   label, and marks it the system volume in the mount table
   (`MountTable.SetSystem`), which `DISMOUNT` then refuses.
4. It defines the system's names for it (`dclcmd/sysnames.go`), in the
   system table at executive mode, as VMS's boot defines its own:
   `SYS$SYSDEVICE` as `_DUA0:`, and `SYS$SYSROOT`, `SYS$COMMON`, and
   `SYS$SPECIFIC`, all three as the root `_DUA0:[SYSTEM.]`. Each is
   concealed and terminal, so a file reached through one is shown by the
   logical name, and `SYS$SYSROOT:[SYSEXE]` is the directory
   `[SYSTEM.SYSEXE]` (section 9.5 has rooted directories). VMS has a root
   per system, `[SYS0.]`; govax has one system and one root, and VMS's two
   halves of it are the same root.
5. On a volume mounted for writing, it brings the files up to date
   (`sysvol.Refresh`, `sysvol/populate.go`).

**The refresh** is how a newer govax gets its files onto an older volume
without losing the user's changes. Each file is either *supplied*, govax's
to keep current, or *seeded*, written once as a starting point and the
user's from then on (`SYSTARTUP_VMS.COM` is the one seeded file). The
volume keeps a manifest, `[SYSTEM]GOVAX$FILES.DAT`, with a SHA-256 digest of
each supplied file as govax last wrote it, and the refresh decides by
comparing three things, the file on the volume, the manifest's digest, and
this govax's copy:

- a supplied file that's missing is written;
- one that is already this govax's copy is left alone;
- one that matches the manifest's digest is govax's older copy, which the
  user hasn't touched, so the new copy is written as a new version, the
  old one kept below it;
- any other was changed by the user, and is never written: govax says so
  once per new copy, `%MOUNT-I-SYSKEPT`, and goes on with the user's;
- a seeded file is written only if it's missing and the manifest doesn't
  say it was made before, so one the user deleted stays deleted.

A help library is compared module by module, so the user's own module in
`HELPLIB.HLB` doesn't stop the rest of it from being refreshed. What a
refresh writes is logged in `[SYSTEM]GOVAX$UPDATE.LOG`.

Nothing about the system volume can stop govax from starting. A container
that can't be made, opened, or mounted, or has the wrong label, is
`%MOUNT-W-NOSYSVOL`, and govax goes on with no system volume, using its
built-in files: the startup procedure, kernel, grammars, and libraries
embedded in the program.

### 11.3 The grammars, and the files govax reads as it starts

Every console and debugger command is parsed by a grammar
(`internal/dcl`): `CONSOLE.DCL` for the console, whose verbs DCL and the
machine bind, and `DEBUG.DCL` for the debugger. govax reads them through
its *resolver* (`internal/respath`), which looks for a file by name in
four places: as the name is given, in the `--path` directories, on the
system volume (the names in `sysvol.Resolved`, opened through RMS by
`dclcmd.SystemFS`), and last in govax's embedded files. A host copy comes
first so a developer can try a changed grammar without touching the
volume. The volume's copy is the user's, and may be old or broken, so one
that doesn't parse, or lacks a command this govax binds, is
`%VAX-W-SYSGRAMMAR` and govax uses its own copy
(`cmd/govax/grammarfiles.go`).

With the grammars in place, `run` mounts the `--mount` and
`--mount-write` containers, then the configured default volume
(`vax.default.volume.*`), which it also makes the default device and
directory (`dclcmd/defvolume.go`).

### 11.4 STARTUP.COM

`Console.RunStartup` (`console/startup.go`) runs the system volume's
`STARTUP.COM` at command level 1, as a login command procedure runs before
a user's first command. It names the file by device and directory,
`DUA0:[SYSTEM.SYSMGR]STARTUP.COM`, so as not to depend on a logical name
it hasn't defined yet; with no system volume it runs govax's built-in copy.
It's an ordinary DCL procedure: a command that fails ends it, as DCL's
default `ON ERROR THEN EXIT` says, and its lines without a `$` are data for
whatever reads its input. What it does, in order (`SET VERIFY` and a second
run of it showed these lines):

```text
$ define/system/executive_mode sys$system  sys$sysroot:[sysexe]
   ...                         sys$manager, sys$help, sys$library,
                               sys$share, sys$sources
$ init/vax ^d16384
$ initialize/memory /p0=16384 /p1=8192 /s0=8192 -
     /ksp=20 /esp=8 /ssp=8 /isp=4
$ load/rom/noerror sys$system:default.rom
$ load/nvram/noerror
$ asm sys$sources:kernel.asm
$ set quantum 1
$ debug
go exe$initialize
%SYSTEM-S-INIT, Kernel initialized
exit
$ set debug nouserhalt
$ set radix dec
$ print " "
$ define/device tta0/devclass=term/devtype=vt100/devbufsiz=80/devchar=201588743/ownuic=65540
$ define/device nla0/devclass=mailbox/devtype=null/devchar=202702849/ownuic=65537/devbufsiz=512
$ if f$search("SYS$MANAGER:SYSTARTUP_VMS.COM") .nes. "" then -
        @sys$manager:systartup_vms
```

The rest of the file is comments and the DCL that chooses the values. The
system names are defined only when `SYS$SYSDEVICE` says there's a system
volume. The kernel and ROM are the volume's files, so an edited
`KERNEL.ASM` is the one that boots, or govax's built-in `kernel.asm` and
`default.rom` without a volume. The memory's sizes are the procedure's
defaults unless the configuration's `vax.memory.*` settings name others
(read with govax's own lexical function `F$CONFIG`), and with
`vax.sysgen.paging.enable` set, P0, P1, and S0 are 0, which
`INITIALIZE/MEMORY` takes as its paging defaults. The sections below take
the steps in turn.

### 11.5 INIT/VAX: a new machine

`INIT/VAX ^d16384` is `Machine.Init` (`machine/init_vax.go`). It ends every
other process, then makes a new machine of the size asked for, rounded to
a whole number of pages: a new `registers.CPU`, with every register and the
PSL zero, a new `vm.Memory`, a new `cpu.Engine` (which reads the engine's
settings, the clock among them, as it's made; section 4.4), and a new VMS.

The new VMS is `newRTL` (`machine/machine.go`), and it's the same whenever
the machine is remade, by `INIT`, `INITIALIZE/MEMORY`, or `ZERO`. It closes
what the old process 1 left open, and the old page file, then makes a
`corevms.System` on the new CPU and memory, sharing the machine's devices
and mounts (section 3.5), and:

- registers the run-time libraries' routines in its shim table: LIBRTL's,
  MTHRTL's, and LBRSHR's (`librtl.Register`, `mthrtl.Register`,
  `lbrrtl.Register`; section 10.5);
- gives it the process settings (`vax.process.*`) and the SYSGEN parameters
  (`vax.sysgen.*`; section 6.1), and its hooks into the machine: the image
  activator for `$CREPRC`'s processes, the traceback's names, the AST exit
  stub;
- sets its clock to the engine's system time, and its boot time
  (`SYI$_BOOTTIME`) to now on that clock, and makes it the engine's
  scheduler if `vax.process.scheduler` is on (section 7.5);
- makes process 1's `corevms.Environment`, on the machine's logical name
  database, and opens the console terminal as its SYS$INPUT, SYS$OUTPUT,
  SYS$ERROR, and SYS$COMMAND, process-permanent files (section 8.7), as
  VMS's LOGINOUT does for a terminal's process.

`Zero` then clears the symbols and the memory-management registers, MAPEN
among them, and sets every stack pointer to the last longword of physical
memory. The machine is now a VAX with memory, a processor in kernel mode
at IPL 0, and no address space.

### 11.6 INITIALIZE/MEMORY: the address space

`INITIALIZE/MEMORY` (`Machine.VMInit`, `machine/vminit.go`) lays out the
address space section 5.5 describes. In order, it:

1. works out the regions' sizes (with paging off, regions given 0 share
   what physical memory is left; with paging on, they take the paging
   defaults) and refuses sizes that can't work: `P1TOOSMALL`,
   `VMTOOLARGE`, `S0TOOSMALL`;
2. ends the other processes and makes memory, the engine, and the
   `System` new again, so physical memory is all zeros, with `vm`'s frame
   allocator chosen by the paging mode (section 6.1);
3. with MAPEN still 0, writes VMS V1's system service vector at physical
   0 (section 10.2), then the system page table after it, S0 page `n`
   mapping frame `n`;
4. writes process 1's P0 and P1 page tables, every entry an invalid,
   user-owned page (with paging on, only P1's existing part; section 6.11);
5. lays out the stacks, the guard pages, the scratch page
   (`CONSOLE$SCRATCH`), the first shim page, the SCB (SCBB), and the
   string pool, and makes the rest of S0 the S0 pool;
6. sets the registers: SBR and SLR, P0BR to P1LR, the five stack pointers
   (KSP, ESP, SSP, and ISP to their stacks' tops, USP to
   `corevms.UserStackTop`, `7FE00000`), SP to KSP, and the PSL's current
   mode to kernel and its previous mode to user;
7. adopts those tables and stacks as process 1's (`AdoptAddressSpace`,
   `AdoptStacks`), and sets VIRTUALPAGECNT, every new process's table
   area, to process 1's unless the configuration names one;
8. turns MAPEN on, and empties the TB.

It also forgets the interactive assembler's session and that a microkernel
was placed: the memory they described is gone.

`LOAD/ROM/NOERROR SYS$SYSTEM:DEFAULT.ROM` then reads the console ROM
(`Machine.LoadROM`, `machine/rom.go`): a file that starts `;ROMIMG` and a
carriage return, then the ROM's physical base and end addresses, then its
non-zero pages, each with its offset. govax's is at `20040000`, as a
MicroVAX II's boot ROM is (section 5.1). Nothing in govax's VMS uses it.
`LOAD/NVRAM/NOERROR` reads `DEFAULT.NVRAM` the same way if one is found
through the search path; govax supplies none, so there is no NVRAM.

### 11.7 The microkernel

VMS's executive is VAX code: the scheduler, the pager, the system
services, the drivers. govax writes all of those in Go, and what's left
that must be VAX code is small enough to be assembled at every start from
its source, `SYS$SOURCES:KERNEL.ASM`: govax calls it the *microkernel*.
It has

- the exception and interrupt routines the SCB names, and the dispatcher
  for its own `CHMK` calls (section 8.6);
- `EXE$INITIALIZE`, below;
- a page of user-mode routines: the console output routines the
  microkernel's own messages use, a few C library routines, and LIB$CALLG,
  whose whole job is a CALLG (section 10.5);
- a table of `.SHIM` declarations, the shims' names and the shareable
  image offsets they stand for;
- writable data: the AST queue's head, the register save areas, the
  message texts, the C library's character table.

**Assembling into live memory.** The console's `ASM` command
(`Machine.Assemble`, `machine/asm.go`) assembles a file with
`internal/asm` in its console dialect, absolute rather than relocatable,
and then deposits the result straight into the machine's memory. One
assembler session lasts from one `INITIALIZE/MEMORY` to the next, so a file
assembled later can name an earlier one's labels. The session's P0 code
starts at `200`, its S0 code at the first page past `INITIALIZE/MEMORY`'s
layout (`s0Free`), and its SCB at SCBB. `kernel.asm` starts with
`.MICROKERNEL`, which enables the directives only a microkernel uses:

| Directive | What it does |
| --- | --- |
| `.MICROKERNEL` | marks the assembly as the microkernel's, enabling the rest |
| `.P1VECTOR` | writes the system service vector into P1 (`vmsdef.P1VectorTable`; section 10.2) and defines each service's name |
| `.REGION SYSTEM` / `P0` | switches between the session's S0 and P0 locations |
| `.SCB code, address` | stores a vector at SCBB + `code` (a stack can be named: `ISP` sets the vector's low bit) |
| `.SHIM name, code, library, offset` | writes a shim stub (or names a routine of code 0) and defines `SHIM$library_offset` (section 10.5) |
| `.CONSOLE command` | names a console command to run while assembling; govax's assembler accepts it and runs nothing |

`depositAsmImage` copies four parts of the assembly into memory: the P0
code; the S0 code, whose pages it first claims from the S0 pool's bottom
(`Environment.ClaimS0`); the SCB page; and the P1 vector, whose pages every
process then maps as its own (`Environment.ShareP1`, section 5.6). The
assembly's symbols join the console's symbol table, so `EXE$INITIALIZE`
and the rest can be named in commands; and once `EXE$INITIALIZE` is
defined the microkernel counts as placed (`Machine.KernelPlaced`), and
`ASM` is no longer a console command, but the debugger's.

The microkernel's last lines set its SCB:

```text
                .scb            exc$chmk,      exe$chmk
                .scb            exc$priv,      console$handler
                .scb            exc$accvio,    console$handler
                .scb            exc$resop,     console$handler
                .scb            exc$resaddr,   console$handler
                .scb            exc$tnv,       console$handler
                .scb            exc$tp,        console$handler
                .scb            exc$customer,  console$handler
                .scb            exc$bpt,       console$handler
                .scb            exc$chms,      console$handler
                .scb            exc$chmu,      console$handler
                .scb            exc$arith,     console$handler
                .scb            exc$interval,  exe$interval
                .scb            exc$software2, exe$deliver_ast
                .scb            exc$conwrite,  exe$tx
                .scb            exc$conread,   exe$rx
```

`console$handler` is the value `FFFFFFFF`, which no routine can have: an
exception whose vector holds it goes to Go, where VMS's condition handling
takes it (section 10.7), and a stop at the console if nothing does (section
4.5). Every page of the microkernel keeps S0's protection, URKW: every
mode can read it, and so run its code, and only kernel mode can write it.
Its code, its user-mode routines, its data, and a spare buffer each start a
page of their own, so that each could have a protection of its own, as the
source once asked for with `.CONSOLE SET PAGE`; govax's assembler never ran
those lines, and they're gone.

**Initialization.** The microkernel's one run of its own is
`EXE$INITIALIZE`. `GO` is a debugger command, so `STARTUP.COM` starts the
debugger with `DEBUG` and gives it, as the procedure's data lines, `GO
EXE$INITIALIZE` and `EXIT`. The routine runs in kernel mode, as
`INITIALIZE/MEMORY` left the PSL:

```text
exe$initialize: tstl            @#exe$init_done ; If we've already done
                bneq            exe$init_exit   ; this, just return
                mtpr            #^X40,#VAX$PR_RXCS
                movl            #1, @#exe$init_done
                pushl           #^X03C00000       ; desired PSL
                pushal          @#exe$init_exit ; desired PC
                rei                             ; do it

exe$init_exit:  tstl            @#exe$verbose
                bneq            exe$printinitmsg
                brw             exe$silenthalt

exe$printinitmsg:
                pushal          @#exe$init_msg
                calls           #1, @#exe$put_output
                brw             exe$silenthalt
```

It enables the console receiver's interrupt (which nothing raises; section
8.6), notes that it has run, and leaves kernel mode the only way a VAX
can: it pushes a PSL and a PC and executes REI, which pops them. The PSL,
`03C00000`, is user mode with a previous mode of user and IPL 0, so
interrupts and ASTs can be taken from now on; REI saves SP as KSP and
loads SP from USP, the user stack's top. In user mode it writes its banner,
`%SYSTEM-S-INIT, Kernel initialized`, through `exe$put_output`, one
`XFC$CONSOLE_PUT` per string, unless the console is quiet (a one-shot
command), and then `exe$silenthalt` executes `XFC #XFC$HALT_SILENT`, which
stops the run without a message. The debugger's `EXIT` ends its session,
and `STARTUP.COM` goes on, with the processor in user mode, where it stays:
everything the user runs from here on starts in user mode.

The microkernel doesn't start the interval clock: govax keeps time itself
(section 4.4), and nothing needs the clock's interrupt.

Three console settings follow. `SET QUANTUM 1` sets the instructions per
emulated millisecond (section 4.4). `SET DEBUG NOUSERHALT` makes HALT
outside kernel mode a privileged instruction fault, as the architecture
has it: until then, so that a test program could stop the machine, a HALT
in any mode halted. `SET RADIX DEC` makes the console's numbers decimal.

### 11.8 Devices

The devices `STARTUP.COM` defines are the two VMS always has: `TTA0:`, the
console terminal, and `NLA0:`, the null device, each with the device
characteristics, buffer size, and owner VMS 7.1 showed for its own
(`DEFINE/DEVICE`, section 8.1). The disks are defined as they're mounted:
the system volume's `DUA0:` before `STARTUP.COM` ran, and any others by
`--mount` or `MOUNT`. Process 1's terminal names are already there, opened
by `newRTL`. A site that wants more devices defines them in
`SYSTARTUP_VMS.COM`, which `STARTUP.COM` runs last.

### 11.9 After STARTUP.COM

With the procedure done, `run` goes on in Go:

1. **The page file.** `Machine.StartPageFile` installs it, with paging on:
   the file `vax.sysgen.paging.file` names, or the system volume's
   `SYS$SYSTEM:PAGEFILE.SYS` if it's mounted for writing, made or resized
   to `vax.sysgen.paging.size` blocks, unless `vax.sysgen.paging.defer`
   leaves it for the pager's first need (section 6.5). From here on, each
   new `System` installs its own as it's made.
2. **The user's procedure.** If the configuration names one in
   `vax.startup.procedure`, it's run, a volume file or a host file by the
   name's form; one that isn't there is passed over without a message.
3. **The one-shot command.** A command left on govax's command line
   (`Console.RunCommandLine`) is level 0's input in place of the terminal,
   and the session ends after it. The console is quiet for it: no banner,
   no kernel message.
4. **The prompt.** Otherwise govax reads the terminal, one line at a time,
   with the prompt of whichever reader has it: DCL's `$ `, the debugger's
   `DBG> `, or the assembler's `ASM> `. While a line is typed, the other
   processes run (section 7.12).

The `--instruction-limit` and `--time-limit` options apply only from step
3, so a limit meant for a runaway program doesn't stop the system's own
start. When the session ends, `Console.EndSession` closes what process 1
left open and the mount table dismounts every volume, so that each
container has all its blocks, its bitmaps included.

### 11.10 Running an image

`RUN` is where everything before it comes together. `Machine.Run`
(`machine/run.go`) does, for process 1:

1. **Ends what came before.** An image that was stopped and never exited
   (a breakpoint left, a Ctrl/C, a limit) is run down now
   (`EndAbandonedImage`), the CPU comes back to process 1 if a run left it
   in another, and the debugger forgets the old image's breakpoints.
2. **Activates in kernel mode.** The activation writes the image into
   pages a user-mode program could not write (the header, before the
   sections' protection is set), so the machine switches to kernel mode and
   its stack for it, and back afterward.
3. **Writes the shim stubs**, the first time (`EnsureShims`; section 10.5).
4. **Activates the image** (`ImageProcess.ActivateImage`), below.
5. **Builds the IMAGE$INIT driver** (`buildDriver`), below.
6. **Calls the driver**, under the debugger if the image was linked
   `/DEBUG` or RUN said `/DEBUG`, and otherwise directly.

**Finding the image.** The main image is found by the rules MACRO and LINK
use (`rms.Session.Locate`): a VMS file specification, or a bare name while
the default directory is on a volume, is a volume file with the default
type `.EXE`; anything else, or a name with `/HOST`, is a host file, looked
for through the resolver. A shareable image is found by the image
activator's rules (`machine.ImageLocator` over `rms.ImageLocator`,
`rms/shareable.go`), in this order:

1. If govax stands for the image (LIBRTL, MTHRTL, LBRSHR, DECC$SHR, or any
   other of VMS's run-time libraries: `machine.Emulated`), it has no file,
   and its routines are the shims, unless a logical name of the image's
   name is defined.
2. `$PARSE`'s rules on the name with the default `SYS$SHARE:.EXE`, each
   element of a search list in turn.
3. A host file: after `SET SHARE_PREFIX`'s prefix, in the host library
   directory (`vax.library`), then as given.
4. An image built into govax: `FTNRTL.EXE`.

So `DEFINE LIBRTL dua1:[mylib]librtl.exe` runs a program on a real
LIBRTL, and a user's own `FTNRTL.EXE` on SYS$SHARE comes before the built-in
one. An image found nowhere is `ACTIMAGE` and `IMAGEFNF`; a run-time library
of VMS's that no shim stands for doesn't stop the activation, its routines
each a stub that signals NOTIMPL (section 10.5).

**Loading it.** With paging off, the *copying loader* (`loadImage`,
`machine/image.go`) reads the whole file, copies its header into P0 just
above the high-water mark to read it, and then copies each section to its
place (`loadSection`), demand-zero sections as pages of zeros, moving P0's
high-water mark past them. With paging on, the *mapping loader* decodes the
header in Go and maps each section (section 6.10). Either way the image
then gets an *image control block*, an `ICB`, with its base, its end, its
sections, and its transfer addresses: up to three, of which the first is
`SYS$IMGSTA` in P1 for an image linked with traceback, and the main
routine is the first that's in P0 (`mainTransferAddress`). The main image's
is defined as the console symbol `MAIN`. `finishLoad` reads the image's
ident and GSMATCH, its debug symbol table (for the debugger and the
traceback), and, if the image has a fixup section, its list of shareable
images, each of which is loaded the same way, from the list's end.

A shareable image the link placed at a fixed address goes there; others
go at P0's high-water mark (`imageBase`, `machine/activate.go`). Each is
checked against the GSMATCH its caller was linked with (`identMatches`), a
mismatch being `SHRIDMISMAT`.

**Fixing it up.** Once every image is loaded, `imageFixup` resolves each
image's fixups: each G^ cell, which the linker left holding an offset into
a shareable image, gets an address, the image's base plus the offset if
it's loaded, the shim stub `SHIM$image_offset` if govax stands for it, or a
NOTIMPL stub; and each `.ADDRESS` cell is rebased by its image's base. Only
then does each section get its protection (`setImageProtection`), as VMS's
image activator changes a section's protection after its fixups, and the
process's `JPI$_IMAGNAME` become the image's file.

**The driver.** RUN doesn't call the main routine itself, but a small
procedure it writes at `CONSOLE$SCRATCH+8`, the IMAGE$INIT driver (section
10.8). For `HELLO.EXE`, a MACRO program linked with traceback,
`EXAMINE/INSTRUCTION` showed:

```text
80026008:       0000
8002600A:       MOVL     I^#80029714,(FP)
80026011:       CALLS    I^#00000000,@#HELLO
8002601C:       PUSHL    R0
8002601E:       CALLS    I^#00000001,@#SYS$EXIT
80026029:       RET
```

The first word is the entry mask, which saves no registers. The `MOVL`
puts the traceback handler, `EXE$TRACEBACK`, a shim stub, in the driver's
frame, where VMS's `SYS$IMGSTA` puts it for an image linked with traceback
(section 10.7). With `/INIT` in effect, as it is unless `SET DEBUG
NOLIBINIT` or RUN's `/NOINIT` turns it off, a `CALLS` of each shareable
image's initialization routine, its first transfer address, would come
next, with three arguments. Then the main routine, and `$EXIT` with what it
returned. An image with no transfer address, a shareable image, isn't run:
DCL's `NOTFR`, and what its activation made goes. `RUN/NOEXECUTE` stops
before the call.

**The call.** `Machine.Call` has the engine build a CALLS frame for the
driver on the current stack (`Engine.CallEntry`, `cpu/call.go`), with a
saved PC and FP of `FFFFDEAF`, the console's sentinel, so that the
driver's last RET, or `$EXIT`'s unwinding (section 10.8), ends the run
rather than returning anywhere. Since `EXE$INITIALIZE`, the processor is in
user mode, so the current stack is the user stack. At `HELLO`'s first
instruction, under the debugger, the stack held the two frames:

```text
7FDFFFD0:       00000000    HELLO's frame: no handler
7FDFFFD4:       20000000    the entry mask and PSW; bit 29 says CALLS
7FDFFFD8:       7FDFFFFC    saved AP
7FDFFFDC:       7FDFFFE8    saved FP: the driver's frame
7FDFFFE0:       8002601C    saved PC: in the driver
7FDFFFE4:       00000000    HELLO's argument count
7FDFFFE8:       80029714    the driver's frame: EXE$TRACEBACK
7FDFFFEC:       20000000
7FDFFFF0:       00000000    saved AP
7FDFFFF4:       FFFFDEAF    saved FP: the sentinel
7FDFFFF8:       FFFFDEAF    saved PC: the sentinel
7FDFFFFC:       00000000    the driver's argument count
```

The user stack starts at `UserStackTop`, `7FE00000`, so the driver's
argument count is its first longword. The run loop then runs until the
image ends, and `$STATUS` becomes its exit status.

**Other processes.** A process `$CREPRC` makes activates its image the same
way when it first runs (`ActivateCreatedImage`), into its own P0, with the
driver in an S0 pool page of its own rather than the console's scratch page,
and, for a process created to hibernate, a call of `$HIBER` before the main
routine (section 7.10).

### 11.11 What's VMS's and what isn't

- **No boot path.** There's no boot ROM program, VMB, SYSBOOT, SYSINIT, or
  STARTUP process: a Go program builds the machine, and process 1 runs
  `STARTUP.COM` as its login procedure, with console commands that a VMS
  startup procedure doesn't have (`INIT/VAX`, `INITIALIZE/MEMORY`, `ASM`).
- **A microkernel, not an executive.** What VMS's executive does is Go;
  the microkernel is the SCB, a few handlers, and a few routines, assembled
  from source at every start.
- **The processor's initial state.** `INIT/VAX` leaves the PSL zero:
  kernel mode, IPL 0, not on the interrupt stack. The architecture's
  processor initialization sets `041F0000`, kernel mode on the interrupt
  stack at IPL 31. The microkernel's initialization doesn't depend on the
  difference, but the console's `GO` and `CALL` of code before it does.
- **The microkernel's pages are all URKW.** Its code is writable by kernel
  mode, where VMS's executive code is read-only.
- **SYSGEN parameters come from the configuration** (`vax.sysgen.*`),
  read when a `System` is made; there's no SYSGEN utility or parameter
  file.
- **No LOGINOUT or SYSUAF for process 1.** Its UIC, privileges, and quotas
  are govax's.
- **One system root.** `SYS$SYSROOT`, `SYS$COMMON`, and `SYS$SPECIFIC` are
  the same root, `[SYSTEM.]`.
- **No installed images.** A shareable image is mapped privately into each
  process that uses it, never shared as global sections, and VMS's run-time
  libraries are govax's shims unless a logical name names a real one.
- **The system volume's refresh**, which keeps a user's changes while
  bringing govax's files up to date, is govax's own.
- **The copying loader** (paging off) writes an image's header into P0
  while it reads it; VMS reads it into a buffer of its own.
- **Initialization routines.** RUN's driver calls each shareable image's
  initialization routine, with three arguments (100 as the first for
  LIBRTL's, unconfirmed). govax's LINK builds no `LIB$INITIALIZE` into an
  image, so a main image's own initialization routines are never called.

## 12. A map of the source

Section 2 listed the packages a layer at a time. This section is the
other way in: for each subject of sections 4 to 11, the files to open
first, in the order they're best read; then the tests that hold each
subject to the VAX or to VMS; then a few ways to watch the machine work.
Paths are under `internal/` unless they say otherwise, and `ods2/...` is
the separate `github.com/tucats/ods2` module.

Every package has a `doc.go` that says what the package is for and how
its files divide the work. It's the place to start in a package this map
doesn't cover.

### 12.1 By subject

**The instruction set** (section 4).

| File | What to read it for |
| --- | --- |
| `cpu/gen/operands.go`, `cpu/instructions_table.go` | the table: each opcode's name, operands, access kinds, and data types (generated; don't edit the table by hand) |
| `cpu/engine.go` | `Engine`, `NewEngine`, and `Step`: what happens between two instructions and in one |
| `cpu/decode.go`, `cpu/operand.go`, `cpu/operandaccess.go` | operand specifiers, and the `Operand` an instruction reads and writes through |
| `cpu/dispatch.go` | `Handler`, and how each file registers its instructions' handlers |
| `cpu/condcodes.go` | N, Z, V, and C |
| `cpu/mov.go`, `cpu/integermath.go`, `cpu/branch.go`, `cpu/call.go`, ... | the instructions, a family to a file; `call.go` is CALLS, CALLG, RET, and REI |
| `cpu/fpu.go`, `vaxfloat/` | the floating instructions and the formats beneath them |
| `cpu/decimal.go`, `cpu/decimalmath.go`, `cpu/editpc.go` | packed decimal |
| `registers/operandlog.go` | the operand log, which puts registers back when an instruction faults |
| `cpu/exception.go`, `cpu/handlefault.go` | faults, traps, and aborts, and the SCB sequence that takes them |
| `cpu/inttrap.go`, `cpu/trace.go` | the arithmetic traps and the trace trap |
| `cpu/interrupt.go`, `cpu/clock.go`, `cpu/systime.go` | interrupts, the interval clock, and the time of day |
| `cpu/changemode.go`, `cpu/procreg.go` | CHMK to CHMU, and MTPR and MFPR |
| `cpu/context.go` | the hardware PCB, LDPCTX, and SVPCTX |

**Memory** (section 5).

| File | What to read it for |
| --- | --- |
| `vm/memory.go` | `Memory`: RAM, the ROM and NVRAM, physical loads and stores |
| `vm/pte.go` | the page table entry and its forms, valid and not |
| `vm/translate.go` | a virtual address made physical, step by step, and the faults on the way |
| `vm/tb.go`, `vm/fetch.go` | the translation buffer and the instruction-fetch window |
| `vm/space.go` | another process's address space, reached without a context switch |
| `vm/pager.go`, `vm/frames.go` | the pager hook and the frame allocator: the two seams to VMS |
| `machine/vminit.go` | INITIALIZE/MEMORY's layout of S0, P0, and P1 |
| `corevms/addrspace.go`, `corevms/stacks.go`, `corevms/p1vector.go` | a process's address space, its mode stacks, and the P1 vector shared by all |
| `corevms/filesec.go` | file sections, and `PageIn`, the pager every fault reaches |

**VMS paging** (section 6).

| File | What to read it for |
| --- | --- |
| `paging/doc.go`, `paging/database.go`, `paging/list.go` | the PFN database and its free and modified lists |
| `paging/workingset.go` | one process's working set list and its replacement |
| `corevms/frames.go`, `corevms/workingset.go` | frames claimed with their owners; a fault, an eviction, a soft fault |
| `corevms/mpw.go`, `corevms/reclaim.go` | the modified page writer and the reclaimer |
| `paging/pagefile.go`, `corevms/pagefile.go`, `rms/pagefile.go` | the page file's block map, its use, and the file itself |
| `corevms/pgflquota.go` | PGFLQUOTA |
| `corevms/globalpages.go`, `corevms/gblsec.go` | global sections and their pages |
| `corevms/cluster.go` | read clustering |
| `machine/imagemap.go`, `corevms/imagesec.go`, `corevms/pagesource.go` | an image activated by mapping |
| `corevms/regions.go`, `corevms/tablearea.go`, `corevms/userstack.go` | created address space, the table area, and the user stack |
| `machine/sysgen.go`, `corevms/sysgen.go` | the `vax.sysgen.*` settings |

**Processes** (section 7).

| File | What to read it for |
| --- | --- |
| `corevms/system.go`, `corevms/environment.go`, `corevms/process.go`, `corevms/job.go` | what's shared, what's one process's, and the job |
| `corevms/proctable.go` | the process table and PIDs |
| `sched/` | VMS's scheduling rules in plain Go |
| `cpu/schedule.go`, `corevms/schedule.go` | the engine's hook and `switchTo`, where the CPU changes process |
| `corevms/waits.go`, `corevms/idle.go` | waiting, event reporting, and what happens with nothing to run |
| `cpu/ast.go`, `corevms/ast.go` | ASTs, the engine's half and VMS's |
| `corevms/eventflags.go`, `corevms/hibernate.go`, `corevms/suspend.go`, `corevms/timers.go` | event flags, hibernation, suspension, and timers |
| `corevms/mailbox.go`, `lck/`, `corevms/enq.go` | mailboxes and the lock manager |
| `corevms/creprc.go`, `corevms/startup.go`, `corevms/delete.go`, `corevms/termmsg.go` | a process made, started, and deleted |
| `corevms/spawn.go`, `corevms/cliprocess.go` | subprocesses, and processes that run DCL |
| `corevms/terminal.go`, `corevms/lend.go` | the shared terminal, and lending the CPU while DCL waits |
| `cpu/attention.go`, `corevms/ctrlast.go` | Ctrl/C and Ctrl/Y |

**I/O** (section 8).

| File | What to read it for |
| --- | --- |
| `io/device.go`, `corevms/devices.go` | the device table, and channels |
| `corevms/qio.go` | `$QIO`: the request, its driver, and its completion |
| `corevms/ttdriver.go`, `corevms/mbxdriver.go`, `corevms/nulldriver.go`, `corevms/diskdriver.go` | the drivers |
| `corevms/iocount.go` | the I/O counts |
| `cpu/xfc.go`, `machine/services.go` | the console's own XFCs |
| `lnm/ppf.go`, `rms/ppf.go`, `rms/ppfaccess.go`, `corevms/ppfstart.go` | process-permanent files |

**RMS and ODS-2** (section 9).

| File | What to read it for |
| --- | --- |
| `ods2/volume/volume.go`, `ods2/ondisk/` | a mounted volume, and the structures on the disk |
| `ods2/volume/access.go`, `ods2/volume/file.go` | an open file and its sharing counts |
| `rms/mount.go`, `rms/session.go`, `rms/context.go` | the mount table, a session, and what a service is given |
| `corevms/rms.go` | the services' connection from the vector to `rms` |
| `rms/fab.go`, `rms/rab.go`, `rms/nam.go`, `rms/xab.go` | the control blocks in VAX memory |
| `rms/name.go`, `rms/parse.go`, `rms/search.go`, `lnm/filespec.go` | names, logical names, `$PARSE`, and `$SEARCH` |
| `rms/open.go`, `rms/create.go`, `rms/close.go`, `rms/ifi.go` | opening and closing, and the open-file table |
| `rms/stream.go`, `rms/get.go`, `rms/put.go`, `rms/recordops.go` | records |
| `rms/relative.go`, `rms/indexed.go`, `rms/indexedrecords.go`, `isam/` | relative and indexed files |
| `rms/sharing.go`, `rms/recordlock.go` | sharing a file, and record locks |
| `rms/blockio.go`, `rms/ufo.go`, `rms/acp.go` | block I/O, user file open, and the ACP's file operations |
| `rms/location.go`, `rms/hostfile.go` | where a name points, host or volume |

**From VAX code to Go** (section 10).

| File | What to read it for |
| --- | --- |
| `cpu/xfc.go`, `cpu/services.go` | XFC, its selectors, and the interface it calls |
| `vmsdef/p1vector.go` | the system service vector |
| `machine/services.go`, `corevms/enginehooks.go` | the machine's and the process's halves of the engine's hooks |
| `corevms/service.go` | `ServiceTable`, `SystemService`, and the dispatch's steps |
| `corevms/vmstime.go` | `$GETTIM`, a short service to read whole |
| `corevms/changemode.go` | `$DCLCMH` and failure exceptions |
| `corevms/callback.go` | calling VAX code from Go |
| `corevms/shim.go`, `corevms/shimlink.go`, `machine/shim.go` | the shims and their stubs |
| `librtl/routines.go`, `mthrtl/routines.go`, `lbrrtl/` | the run-time libraries' tables |
| `ftnrtl/mar/` | FTNRTL, a library in VAX code |
| `corevms/condition.go`, `corevms/signal.go`, `corevms/unwind.go` | conditions: the search, signals, unwinds |
| `corevms/traceback.go`, `corevms/excmsg.go`, `corevms/msgout.go` | traceback, the catch-all's dump, and where messages go |
| `corevms/exit.go` | `$EXIT`, exit handlers, and rundown |

**Starting the system** (section 11).

| File | What to read it for |
| --- | --- |
| `cmd/govax/main.go`, `cmd/govax/grammar.go` | the program, its options, and its subcommands |
| `console/console.go`, `console/startup.go` | the console made, and STARTUP.COM run |
| `sysvol/`, `dclcmd/sysvolume.go` | the system volume: what it holds, how it's mounted and refreshed |
| `bootdata/files/startup.com` | STARTUP.COM |
| `machine/init_vax.go`, `machine/vminit.go` | `INIT/VAX` and `INITIALIZE/MEMORY` |
| `bootdata/files/kernel.asm`, `machine/asm.go` | the microkernel, and ASM into live memory |
| `machine/rom.go` | `LOAD/ROM` and `LOAD/NVRAM` |
| `machine/run.go`, `machine/activate.go`, `machine/shareable.go`, `machine/image.go` | RUN, the image activator, and the copying loader |

### 12.2 The tests that hold it to the VAX and to VMS

Each package's unit tests sit beside its files. The tests that compare
govax with a real VAX running VMS are mostly in `console`'s tests, since
they need the whole machine; each reads a program and VMS's own output
from a directory under `testdata/`, whose `README.md` says how the run
on VMS was made.

| Subject | Test | `testdata/` |
| --- | --- | --- |
| every opcode has a handler | `TestEveryInstructionImplemented` (`cpu`) | |
| the instructions' results | `TestInsn35Oracle` | `insn35` |
| LIBRTL's instruction routines | `TestLib53Oracle` | `lib53` |
| processes, mailboxes, and the scheduler | `TestMilestone` and its variants | `mp` |
| working sets, the page file, sections, activation, address space | `TestPG63Oracle` to `TestPG67Oracle` | `pg63` to `pg67` |
| RMS's names and `$SEARCH` | `TestRMS3Oracle` | `mar/rms3` |
| relative files | `TestFor81RMSOracle` | `for81` |
| indexed files | `TestIdx88Oracle` | `idx88` |
| concealed and rooted names | `TestLnm91ServicesOracle` | `lnm91` |
| process-permanent files | `TestPPF71Oracle` | `ppf71` |
| conditions and traceback | `TestTB70Oracle` | `tb70` |
| the trace trap | `TestTrace73Oracle` | `trace73` |
| the system names | `TestSystemNames` | |
| a service's argument list out of reach | `TestServiceArgumentListOutOfReach` | |
| ill-formed RMS calls | `TestRMSIllFormedCalls` (`corevms`) | |

A difference from VMS that a test has found and govax keeps on purpose
is listed in the test, beside the comparison, with the reason; the
departures this document notes in each section's last part are the same
ones.

### 12.3 Watching it run

Reading the source goes faster beside a running machine. A few ways to
watch one:

- **`SET DEBUG SERVICES`** at the console traces each system service a
  program calls, with its arguments and status (not the shims), and
  **`SET DEBUG EXCEPTIONS`** each exception and signal and the handlers
  the condition search calls (section 10.3).
- **`SET DEBUG RMS`** traces RMS's services on a file.
- **`RUN/DEBUG`** stops a program at its first instruction under the
  debugger, where `EXAMINE/INSTRUCTION`, `STEP`, `SHOW CALLS`, and
  `SHOW REGISTERS` show the VAX's side of everything here.
- **`SHOW MEMORY`**, **`SHOW SYSTEM`**, and **`SHOW PROCESS`** report the
  frames, the page file, and the processes (sections 6 and 7).
- **`govax --stats`** prints the instruction, interrupt, and memory
  counts and, with paging on, the paging statistics when it ends; **`govax --cpu-profile FILE`**
  writes a Go CPU profile of a run.
- **A test** is often the shortest way into a mechanism: each `doc.go`
  and most files name the tests that exercise them, and `go test -run`
  with `-v` runs one alone.
