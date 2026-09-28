# Per-mode stacks after VMINIT

This note describes where VMINIT (`internal/console/vminit.go`) puts the
stack of each VAX access mode, and why. It was written on 2026-09-28,
when the executive and supervisor stacks got their own protected pages.
Revisit it if a program needs more stack, if the layout moves (to P1, as
VMS has it), or if the kernel stack's protection is tightened.

## Background

The VAX has a separate stack for each access mode (kernel, executive,
supervisor, user) and one for interrupts. The stack pointer of every mode
the CPU isn't running in is kept in a processor register: `KSP`, `ESP`,
`SSP`, `USP`, and `ISP`. When the CPU changes mode (`CHMx`, `REI`, an AST
delivered in another mode, `$CMKRNL`/`$CMEXEC`), it saves `SP` in the old
mode's register and loads the new mode's. So each mode's code needs a
stack that it may write under memory management. A page's protection code
says which modes may read and write it. `EW`, for instance, lets kernel
and executive mode write the page and no other mode touch it.

## The layout

VMINIT builds the page tables in S0 space, and then lays out the
privileged stacks in S0 right after them, in this order (addresses
increasing):

| Region | Size | Protection | Initial stack pointer |
| --- | --- | --- | --- |
| Kernel stack | `/KSP` pages (`vax.init` asks for 20; grammar default 4) | `URKW` (S0's default) | `KSP` = its last longword |
| Guard page | 1 page | `NA` (no access) | — |
| Executive stack | `/ESP` pages, default **8** | `EW` | `ESP` = its last longword |
| Guard page | 1 page | `NA` | — |
| Supervisor stack | `/SSP` pages, default **8** | `SW` | `SSP` = its last longword |
| Interrupt stack | `/ISP` pages (default 4) | S0's default, but addressed physically | `ISP` = its last longword (a physical address) |

The user stack is in P1 (`USP` = `0x7FE00000`), demand-paged, `UW`.

Stacks grow down, so each guard page sits just below its stack. A push
past the bottom of the executive or supervisor stack lands on the guard
page and takes an access-violation fault, instead of silently writing the
top of the stack below.

The default of 8 pages applies both to the VMINIT command (the `/ESP` and
`/SSP` qualifiers in `internal/bootdata/files/console.dcl` default to 8)
and to Go callers of `Console.VMInit` passing 0 pages (the constant
`defaultModeStackPages`).

## Why

Before this change, the executive and supervisor stacks were in S0 but
had S0's default protection, `URKW`: every mode could read them, but only
kernel mode could write them. So any executive- or supervisor-mode code
faulted on its first push once memory management was on. `$CMEXEC`'s
routine was the first to hit this (docs/PHASE-26.md subtask 26), and its
fixture had to set up a stack of its own. Also, a Go caller passing 0
pages for them (as most tests did) put `ESP` and `SSP` at the kernel
stack's own top, so the modes would have shared one stack.

## Deliberately unchanged

- **The kernel stack keeps `URKW`.** The console's CALL, RUN, and the
  acceptance tests build the frame that calls a program on the kernel
  stack. A program that drops to user mode (with `REI`) and then returns
  from its main routine executes that final `RET` in user mode, reading
  the frame, which needs user read access. `KW` would be the VMS
  protection; changing it needs the console to build that frame
  somewhere user mode can read, or to return differently.
- **No guard page below the kernel or interrupt stack.** The C source
  (`console_vminit.c`) installs `PTE$K_NONE` guard pages below each
  privileged stack through its `setpte` mini-parser; govax now does so
  for the executive and supervisor stacks only. A kernel stack overflow
  still runs into the page tables below it.
- **S0, not P1.** VMS keeps each process's inner-mode stacks in P1 space
  (per process). govax has one process and puts them in S0, which VMINIT
  maps eagerly, so they never page-fault.

## Where it's tested

- `internal/console/vminit_test.go`: `TestVMInit_modeStacks` (the default
  sizes, each page's protection, the guard pages, and which modes can
  write each stack) and `TestVMInit_modeStackSizes` (explicit sizes).
- `internal/console/dcl/parse_test.go`: the grammar's defaults.
- `testdata/asm/cmkrnl.asm` runs a `$CMEXEC` routine on VMINIT's own
  executive stack, with memory management on.
