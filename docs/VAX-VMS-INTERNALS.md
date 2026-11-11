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
physically. The vector's low two bits say which stack: 0 the kernel
stack (the target mode's, for a change-mode trap), unless the CPU is
already on the interrupt stack, where the event stays; 1 the interrupt
stack, where an exception, though not an interrupt, also raises IPL to 31.
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
- The kernel stack is URKW, where VMS's is kernel-only: the console builds
  the call frame that runs a program on it, and a user-mode program's last
  RET reads that frame.
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

*To be written.*

## 9. RMS and ODS-2

*To be written.*

## 10. From VAX code to Go and back

*To be written.*

## 11. Starting the system

*To be written.*

## 12. A map of the source

*To be written.*
