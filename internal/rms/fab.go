package rms

// This file defines the real, standard-VMS byte offsets of the fields
// inside a FAB ("File Access Block") that this package's service handlers
// (create.go, open.go, close.go, get.go, put.go, connect.go) need to read
// or write directly out of emulated VAX memory.
//
// # Background for a reader new to VMS RMS
//
// A VAX program that wants to open a file doesn't pass a filename string
// straight to SYS$OPEN the way a C program passes a path to open(2).
// Instead the calling program builds a small, fixed-layout block of
// memory called a FAB — 80 bytes, always — pokes values into its
// individual fields (the file name's address and length, what kind of
// access it wants, and so on), and passes SYS$OPEN the FAB's own VAX
// memory address. RMS then reads whatever fields it needs directly out of
// that block by byte offset, and writes some fields back itself (a
// completion status code, an "internal file index" identifying the now-
// open file, ...). Emulating this faithfully means this Go code has to
// know the exact byte offset of each field within the FAB — precisely
// the same kind of fixed, must-match-real-hardware fact as a VAX
// instruction's own opcode encoding, which is why these are plain
// constants rather than something this package gets to choose freely.
//
// # Where these numbers come from
//
// Every offset below was cross-checked two independent ways during Phase
// 22's implementation (see docs/PHASE-22.md):
//
//  1. Directly against a real VAX/VMS 7.3 system's own $FABDEF structure
//     definitions (not against reference/eVAX/eVAX/Headers/fab.h — see
//     docs/PHASE-22.md's "Why this phase looks different" section for why
//     the C reference project isn't this phase's correctness reference).
//  2. Independently, by hand-walking fab.h's own C struct declaration
//     field by field — respecting each field's natural alignment the way
//     a C compiler would lay it out — and confirming the running total
//     lands exactly on FAB_K_BLN, the struct's documented overall size
//     (80 bytes).
//
// Both methods agree on every field below. Where they matter, offsets are
// also cross-checked against every offset the project's earlier Phase 10
// RMS stopgap (deleted; see git history for internal/rtl/rms.go) already
// used successfully — except one: see fabFNS's own comment for a bug that
// comparison caught.
const (
	// fabIFI is FAB$W_IFI: a 2-byte ("word") field holding the "internal
	// file index" — a small integer identifying an open file, RMS's
	// rough equivalent of a Unix file descriptor. RMS itself fills this
	// field in (SYS$CREATE/SYS$OPEN write it); it isn't something a
	// calling program sets before the call. See ifi.go's FileTable for
	// this package's own IFI bookkeeping.
	fabIFI = 2

	// fabSTS is FAB$L_STS: a 4-byte ("longword") field. After any RMS
	// service call touching this FAB, RMS writes a completion status
	// code here — success, or a specific error such as "file not
	// found". See status.go for the real RMS$_ values this package
	// writes.
	fabSTS = 8

	// fabSTV is FAB$L_STV: a secondary status value accompanying fabSTS,
	// filled in for certain errors (for instance, an underlying device
	// error code). This package always writes a matching (fabSTS,
	// fabSTV) pair together — see status.go's storeStatus.
	fabSTV = 12

	// fabFAC is FAB$B_FAC: a 1-byte field naming what kind of access the
	// calling program is asking for (write, read, and so on — see the
	// fac* constants below). A calling program sets this before
	// SYS$CREATE/SYS$OPEN; RMS reads it to decide whether the requested
	// operation is even allowed.
	fabFAC = 22

	// fabORG is FAB$B_ORG: a 1-byte field naming the file's organization
	// — sequential, relative, indexed, or hashed (see the org* constants
	// below). This package implements sequential organization only
	// (docs/PHASE-22.md's scope); any other value is a real error to
	// report back to the caller, not something to silently misinterpret.
	fabORG = 29

	// fabRAT is FAB$B_RAT: a 1-byte field of "record attributes" flags
	// describing how a text file's carriage control should be
	// interpreted (Fortran-style control characters, an implied
	// carriage return before each record, print-file form feeds, and so
	// on). This mirrors ondisk.RecAttr.Attributes on the ods2 side —
	// see that type's own doc comment in the sibling ods2 module for
	// what each bit means.
	fabRAT = 30

	// fabRFM is FAB$B_RFM: a 1-byte field naming the file's record
	// format — fixed-length, variable-length, "VFC" (variable with
	// fixed control), or one of the stream text formats. SYS$CREATE
	// reads this to decide which ondisk.RecordFormat to give the new
	// file (see create.go). Conveniently, VMS's own FAB$C_* record-
	// format values and ods2's ondisk.RecordFormat constants use
	// identical numbers (both ultimately trace back to the same VMS
	// convention), so this package can convert the raw byte read from
	// this field straight into an ondisk.RecordFormat with a plain type
	// conversion — no separate translation table needed.
	fabRFM = 31

	// fabFNA is FAB$L_FNA: a 4-byte field holding the VAX-memory address
	// of the file specification string (for example
	// "DUA0:[FOO]BAR.DAT"). The string's bytes live elsewhere in VAX
	// memory; the FAB just points at them, the same way a C `char *`
	// points at string data stored somewhere else.
	fabFNA = 44

	// fabFNS is FAB$B_FNS: a 1-byte field giving the length, in bytes,
	// of the string fabFNA points at (VAX RMS file-spec strings are not
	// NUL-terminated — the caller must say how long the string is).
	//
	// The real offset is 52, not 48. The project's now-deleted Phase 10
	// stopgap (internal/rtl/rms.go) used 48 here, which was simply
	// wrong: real VMS's $FABDEF places a whole extra 4-byte field,
	// FAB$L_DNA (the "default file name" address — unused by this
	// package, but still present and still occupying space in the
	// struct), between FAB$L_FNA and FAB$B_FNS. The old code's tests
	// never caught this because its own hand-built fixtures used the
	// same (wrong) offset consistently on both the write and read side —
	// internally self-consistent, but not what a real, unmodified VAX
	// program's compiled FAB layout actually looks like.
	fabFNS = 52

	// fabMRS is FAB$W_MRS: a 2-byte field giving a file's maximum record
	// size in bytes. SYS$CREATE reads this for fixed- and VFC-format
	// files, where every record must fit within a single declared size.
	fabMRS = 54
)

// FAB$B_FAC values (file-access-request flags) this package recognizes.
// Real VMS defines FAB$B_FAC as a bitmask — a program can ask for PUT and
// GET access together, for instance — but this package's SYS$CREATE/
// SYS$OPEN only ever need to tell "the caller wants to write" apart from
// "the caller wants to read" for the sequential files this phase
// supports, so only those two single-bit values are named here; the full
// FAB$M_* bit set (delete access, truncate access, block I/O, ...) is
// simply never checked.
const (
	facPut = 0x01 // FAB$V_PUT (bit 0): write access — every SYS$CREATE implies this.
	facGet = 0x02 // FAB$V_GET (bit 1): read access — what SYS$OPEN normally asks for.
)

// FAB$C_ORG values (file organizations). orgSeq is the only one this
// package's handlers actually accept; the rest are named purely so a
// handler rejecting an unsupported organization can say precisely which
// one it saw in an error message, rather than just "some other value".
const (
	orgSeq = 0  // FAB$C_SEQ — sequential; the only organization this phase implements.
	orgRel = 16 // FAB$C_REL — relative; not implemented.
	orgIdx = 32 // FAB$C_IDX — indexed; not implemented.
	orgHsh = 48 // FAB$C_HSH — hashed; not implemented.
)
