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

*To be written.*

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
