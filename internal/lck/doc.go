// Package lck is govax's lock manager: VMS's rules for locking named
// resources, as plain Go with no CPU or VMS dependency, so they can be
// tested on their own (Phase 47; docs/PHASE-47.md, and docs/PHASE-43.md's
// Part A for the program it belongs to).
//
// Processes that share something (a file, a global section, a record)
// need a way to take turns. VMS gives them one service-level mechanism
// for it, the lock manager: a process asks for a lock on a *resource*,
// which is nothing but a name (1 to 31 bytes) that the cooperating
// processes agree on, in one of six *modes* that say how much it wants to
// share. RMS and the file system use the same manager for their own
// interlocks, and programs reach it through $ENQ and $DEQ.
//
// The rules here are from DIGITAL's VMS System Services Reference Manual
// ($ENQ and $DEQ) and from *VAX/VMS Internals and Data Structures*
// (Kenah and Bate; chapter 13, "VAX/VMS Lock Manager"):
//
//   - The modes, from least to most restrictive, are NL (null: no access,
//     just a placeholder), CR (concurrent read), CW (concurrent write), PR
//     (protected read), PW (protected write), and EX (exclusive). Mode.
//     Compatible is the manual's table of which may be held together.
//   - Each resource has three queues of locks: *granted*, *conversion*
//     (granted locks waiting to change mode, which keep their old mode
//     meanwhile), and *waiting* (new locks not yet granted). The
//     conversion and waiting queues are first in, first out (section
//     13.1.2).
//   - A new lock is granted at once if its mode is compatible with every
//     lock on the resource (granted locks, and converting locks at their
//     granted modes) and nothing is queued before it; otherwise it waits
//     at the end of the waiting queue, or, with NOQUEUE, isn't queued at
//     all.
//   - A conversion is granted at once if the new mode is compatible with
//     every *other* lock on the resource (a lock never blocks its own
//     conversion: section 13.2.2's PW-to-EX example) and the conversion
//     queue is empty, or if the new mode is no more restrictive than the
//     old; otherwise it waits at the end of the conversion queue.
//   - When a lock goes away or its mode drops, the conversion queue is
//     granted from its head while its head can be, and then, once it's
//     empty, the waiting queue the same way (section 13.2.3).
//   - A lock may have a *parent* lock, held by the same process; its
//     resource is then a *sub-resource*, named within the parent's
//     resource, so the same name under two parents is two resources. RMS
//     puts its record locks under a file's lock this way. A lock with
//     sublocks can't be dequeued until they are.
//   - Resources are named within a UIC group unless the lock is a system
//     lock (group 0), and within an access mode.
//   - Each resource has a 16-byte *value block* the holders may read and
//     write: a lock granted, or converted up, with a value block gets the
//     resource's copy, and a PW or EX lock dequeued or converted down with
//     one stores it. It becomes invalid when a PW or EX holder is run down
//     abnormally or asks for it ($DEQ's INVVALBLK).
//   - A lock may ask for a *blocking* notice (a blocking AST, delivered by
//     the caller): it gets one when it's granted and a request queued
//     after it can't be granted because of it. A lock is told once per
//     grant.
//
// The Manager doesn't deliver anything itself. Each operation returns the
// Events it caused, for locks other than the one asked about as well:
// a waiting lock granted (its completion), a lock to notify (its blocking
// AST), a request aborted or a conversion canceled by $DEQ. The caller
// (internal/corevms) writes the lock status blocks, sets the event flags,
// and queues the ASTs, in each owner's process.
//
// What the Manager leaves out: deadlock detection (a deadlock waits
// forever, and Ctrl-C ends it), the ENQLM quota, and VAXcluster
// distribution. Owners are opaque process IDs.
package lck
