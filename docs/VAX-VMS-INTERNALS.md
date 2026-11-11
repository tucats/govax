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
