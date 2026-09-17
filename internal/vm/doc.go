// Package vm implements VAX virtual memory: address translation, page
// tables, and the load/store primitive layer. See docs/PHASE-02.md.
//
// # What this package is for
//
// Real VAX/VMS software (and this emulator's own CPU engine) almost never
// talks about memory in terms of "byte 12345 of physical RAM." Instead, a
// running program uses virtual addresses: 32-bit numbers that look like
// ordinary addresses but must be translated into real physical RAM
// addresses before any byte can actually be read or written. This
// indirection is what lets the operating system give every process its own
// private address space, protect the kernel from user programs, and swap
// pages of memory in and out without programs noticing.
//
// This package is where that translation happens. It is deliberately kept
// separate from internal/vax (which only models CPU registers and the
// processor status word) and from internal/cpu (which decodes and executes
// instructions): vm.Memory owns the raw bytes of RAM and the logic for
// turning a virtual address into a physical one, while everything that
// needs to actually read or write memory — instruction decode, the console
// EXAMINE/DEPOSIT commands, RTL system services — calls through the typed
// Load*/Store* methods here rather than touching a byte slice directly.
//
// # A quick tour of VAX virtual memory, for readers new to the architecture
//
// A VAX virtual address is a 32-bit number. Its top two bits select one of
// four "regions" of the address space:
//
//	00 -- P0, the process program region (where a program's own code and
//	      heap normally live)
//	01 -- P1, the process control region (stack space, growing downward)
//	10 -- S0, the system region (the operating system kernel, shared by
//	      every process)
//	11 -- S1, a second system region reserved for future expansion; no
//	      real VAX operating system used it, and this emulator (like the
//	      C program it's ported from) always treats it as inaccessible.
//
// Memory is managed in fixed-size chunks called pages, each 512 bytes
// (see the pageSize constant in translate.go). Every virtual page in a
// region maps to a physical page of RAM through a page table: an array of
// 32-bit page table entries (PTEs, see pte.go), one per virtual page in
// that region. A PTE records which physical page the virtual page maps to,
// whether the mapping is currently valid, and a protection code saying
// which access modes (kernel/executive/supervisor/user — the same
// privilege levels used elsewhere in the VAX architecture) may read or
// write that page.
//
// Translating an address, in outline, means: figure out which region and
// which page number the address falls in, find that region's page table
// (located via one of the processor's base-and-length registers —
// P0BR/P0LR, P1BR/P1LR, SBR/SLR), fetch the PTE for that page number, check
// its protection code against the current privilege mode, and then combine
// the PTE's physical page number with the low-order bits of the original
// address (the offset within the page) to get the final physical address.
// translate.go's Translate method is a direct, step-by-step implementation
// of exactly this process, and is the best starting point for
// understanding how the rest of the package fits together.
//
// Because this translation is done for essentially every memory access,
// real VAX hardware (and the C emulator this package is ported from) caches
// recent results rather than walking the page table every time. tb.go
// implements that cache; see its own doc comment for details. The cache is
// purely a performance optimization — disabling it would produce identical
// results, just slower — so if you are trying to understand *what* the
// system computes rather than *how fast*, translate.go and pte.go are the
// files that matter; tb.go only matters once you care about speed or about
// exactly reproducing the reference emulator's cache statistics.
package vm
