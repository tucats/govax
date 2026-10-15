// Package paging holds the bookkeeping beneath govax's VMS paging (the
// paging program, Phases 62 to 67; docs/PHASE-62 - VMS paging.md): the
// PFN database and its page lists, as plain Go with no CPU or memory of
// its own, so the rules can be tested on their own.
//
// A VAX's physical memory is divided into 512-byte page frames, numbered
// from 0; a frame's number is its PFN (page frame number). A valid page
// table entry (PTE) holds the PFN of the frame that a virtual page lives
// in. VMS keeps one record per frame, the PFN database, so that it can
// answer the questions the PTEs can't: is the frame in use, and by which
// virtual page; where do its contents come from, and go back to (its
// backing store); has it been written since it was last saved; and how
// many reasons are there to keep it. *VAX/VMS Internals and Data
// Structures* (Kenah and Bate), section 14.2, describes the database, and
// chapter 15 the ways a frame moves through it:
//
//   - A frame that holds some process's valid page is *active*. Its
//     reference count says how many reasons there are to keep it there
//     (being in a working set is one; a global page's share count, the
//     number of processes mapping it, counts as one more).
//   - When the last reason goes (the page leaves the working set), the
//     frame keeps its contents and its tie to the virtual page, whose PTE
//     takes the *transition* form (the PFN, with the valid bit clear). The
//     frame goes to the tail of one of two lists, chosen by the *saved
//     modify bit*: the *free page list* if its contents are already safe
//     on backing store (or never need to be), or the *modified page list*
//     if they must be written first. Release does this.
//   - A fault on a transition page takes the frame off its list and makes
//     it active again, with no I/O: a *soft fault*. Reclaim does this.
//   - A new frame is always taken from the head of the free list, so the
//     frames that left their working sets longest ago are reused first.
//     If the frame is still tied to a virtual page, the tie is broken
//     first: that page's PTE goes back to its backing store form (a
//     section table index, a page file block, a global page table index),
//     so its next fault reads it back. Allocate does this, through the
//     TieBreaker the caller installs.
//   - A frame whose virtual page is deleted loses its tie and goes to the
//     *head* of the free list, to be used first: nothing will ever fault
//     it back. Delete does this.
//   - The modified page writer (Phase 64) writes frames from the modified
//     list to their backing store and moves them to the free list, where
//     they can still be faulted back softly. Cleaned does this.
//
// Like VMS, the database keeps the lists' links outside the frames (VMS
// in its FLINK and BLINK arrays, here in Go), because a frame on a list
// must keep its contents. Unlike VMS, it lives entirely in Go: nothing a
// VAX program can see depends on its layout (the phase doc's Decision 3).
//
// What it leaves out, and why:
//
//   - The read-in-progress, write-in-progress, and release-pending
//     states, and the collided-page bit: govax's page I/O completes
//     inside the fault (Decision 5), so no frame ever waits for I/O.
//   - The bad page list: govax's memory has no bad pages.
//   - The working set list index (the WSLX array): working sets are Phase
//     63's.
//
// The package doesn't know what a PTE looks like, or how to read a page:
// a frame's owner is an opaque handle (a PID, for a process page) and a
// virtual address, and its backing store a kind and two numbers. Breaking
// a tie is the caller's job, through TieBreaker. Check verifies the
// database's rules; the tests run it after every operation.
package paging
