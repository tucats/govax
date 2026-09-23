package rms

import "github.com/tucats/govax/internal/vmsdef"

// This file is rab.go's counterpart to fab.go: the real, standard-VMS byte
// offsets of the fields inside a RAB ("Record Access Block"). See fab.go's
// doc comment for the general background on why RMS control blocks have
// fixed byte layouts at all, for how these offsets were verified, and for
// why they're now read from internal/vmsdef.RABFields (fabOffset's RAB
// counterpart, rabOffset, below) instead of hand-carried literals.
//
// # RAB versus FAB, for a reader new to VMS RMS
//
// A FAB (fab.go) represents one open *file*. A RAB represents one open
// *stream of record-by-record access* to that file — SYS$CONNECT is what
// links a RAB to its FAB (see connect.go), and every subsequent
// SYS$GET/SYS$PUT call operates through the RAB, not the FAB directly.
// Real VMS allows more than one RAB to be connected to the same FAB at
// once (several independent read/write positions into the same open
// file); this package's handlers don't need to exploit that, but the RAB/
// FAB split still matters because it's what a real, unmodified VAX
// program's compiled code expects to poke values into and read status
// back from.
//
// A RAB is 68 bytes, always (RAB$K_BLN in the real $RABDEF definitions).
var (
	// rabFAB is RAB$L_FAB: a 4-byte field holding the VAX-memory address
	// of this RAB's related FAB — set by the calling program before
	// SYS$CONNECT, and how connect.go finds "which file does this stream
	// belong to".
	rabFAB = rabOffset("FAB")

	// rabISI is RAB$W_ISI: a 2-byte field holding the "internal stream
	// index" — RMS's name, at the RAB level, for the same IFI concept
	// fab.go's fabIFI holds at the FAB level (see ifi.go's FileTable).
	// SYS$CONNECT copies the FAB's IFI into this field.
	rabISI = rabOffset("ISI")

	// rabSTS/rabSTV are RAB$L_STS/RAB$L_STV: the RAB's own completion-
	// status pair, exactly analogous to fab.go's fabSTS/fabSTV — see
	// status.go's storeStatus, which writes both of a block's status
	// fields together regardless of whether the block is a FAB or a RAB.
	rabSTS = rabOffset("STS")
	rabSTV = rabOffset("STV")

	// rabRAC is RAB$B_RAC: a 1-byte field naming the record access mode
	// — sequential, keyed, by RFA (record file address), and so on (see
	// the rac* constants below). This package implements sequential
	// access only.
	rabRAC = rabOffset("RAC")

	// rabRSZ is RAB$W_RSZ: a 2-byte field. On SYS$PUT, the calling
	// program sets this to the length, in bytes, of the record it's
	// writing (found at rabRBF). On SYS$GET, RMS instead writes this
	// field itself, to tell the caller how long the record it just read
	// actually was.
	rabRSZ = rabOffset("RSZ")

	// rabRBF is RAB$L_RBF: a 4-byte field holding the VAX-memory address
	// of the record buffer — where SYS$PUT reads the outgoing record's
	// bytes from, and where SYS$GET writes the incoming record's bytes
	// to.
	rabRBF = rabOffset("RBF")

	// rabUBF/rabUSZ are RAB$L_UBF/RAB$W_USZ: an alternate "user buffer"
	// address/size pair a calling program may supply on SYS$GET instead
	// of (or in addition to) rabRBF/rabRSZ — real RMS uses UBF when it
	// wants a separate, caller-owned copy destination distinct from
	// whatever internal buffer it read the record into, most relevant
	// for RMS's read-ahead/locate-mode optimizations. This phase's
	// SYS$GET (get.go) always has real record bytes in hand already (no
	// separate internal buffer to avoid copying out of), so when the
	// caller has supplied a UBF/USZ pair get.go copies the record there
	// instead of RBF/RSZ, matching what a real, unmodified VAX program
	// expects to find populated.
	rabUBF = rabOffset("UBF")
	rabUSZ = rabOffset("USZ")
)

// RAB$B_RAC values (record access modes) this package recognizes. Real
// VMS defines several (sequential, keyed by an indexed file's key, direct
// by RFA, ...); this phase implements sequential organization only
// (fab.go's orgSeq), so racSeq is the only access mode SYS$PUT/SYS$GET
// accept — anything else is rejected with a real RMS$_ error rather than
// silently misbehaving.
var (
	racSeq = byte(vmsConst("RAB$C_SEQ")) // sequential access; the only mode this phase implements.
	racKey = byte(vmsConst("RAB$C_KEY")) // keyed access; not implemented (no indexed-file support yet).
	racRFA = byte(vmsConst("RAB$C_RFA")) // direct access by record file address; not implemented.
)

// rabOffset looks up keyword (e.g. "RAC") in internal/vmsdef.RABFields —
// fabOffset's RAB counterpart, same programming-error-guard reasoning.
func rabOffset(keyword string) uint32 {
	for _, f := range vmsdef.RABFields {
		if f.Keyword == keyword {
			return f.Offset
		}
	}

	panic("rms: no RAB field named " + keyword)
}
