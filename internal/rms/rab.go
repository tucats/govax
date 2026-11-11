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

	// rabRAC is RAB$B_RAC: a 1-byte field naming the record access mode:
	// sequential, keyed, or by RFA (record file address); see the rac*
	// constants below.
	rabRAC = rabOffset("RAC")

	// rabRSZ is RAB$W_RSZ: a 2-byte field. On SYS$PUT, the calling
	// program sets this to the length, in bytes, of the record it's
	// writing (found at rabRBF). On SYS$GET, RMS instead writes this
	// field itself, to tell the caller how long the record it just read
	// actually was.
	rabRSZ = rabOffset("RSZ")

	// rabRBF is RAB$L_RBF: a 4-byte field holding the VAX-memory address
	// of the record buffer. SYS$PUT reads the outgoing record from it;
	// SYS$GET sets it to where the record it read is (the user buffer,
	// in move mode).
	rabRBF = rabOffset("RBF")

	// rabUBF/rabUSZ are RAB$L_UBF/RAB$W_USZ: the user buffer's address and
	// size, where SYS$GET (get.go) and SYS$READ put what they read. $GET
	// requires one (RMS$_UBF, RMS$_USZ without it), and cuts a record
	// longer than it with RMS$_RTB.
	rabUBF = rabOffset("UBF")
	rabUSZ = rabOffset("USZ")

	// rabROP is RAB$L_ROP, the record-processing options: bits that
	// change how one $GET or $PUT behaves. SYS$GET reads RAB$V_PMT from
	// it (ropPMT): prompt the terminal before reading.
	rabROP = rabOffset("ROP")

	// rabRFA is RAB$W_RFA, the record's file address: six bytes, the
	// virtual block (a longword) and the byte offset in it (a word) of
	// the record $GET or $PUT last reached (recordlock.go).
	rabRFA = rabOffset("RFA")

	// rabPBF/rabPSZ are RAB$L_PBF/RAB$B_PSZ, the prompt buffer's address
	// and size: the text a terminal $GET writes before reading, when
	// RAB$V_PMT is set.
	rabPBF = rabOffset("PBF")
	rabPSZ = rabOffset("PSZ")

	// rabKBF/rabKSZ are RAB$L_KBF/RAB$B_KSZ, the key buffer's address
	// and size for random access by key (RAB$C_KEY). They share their
	// offsets with PBF and PSZ: a terminal read never uses a key. For a
	// sequential file of fixed-length records the key is a longword
	// relative record number (keyed.go).
	rabKBF = rabOffset("KBF")
	rabKSZ = rabOffset("KSZ")
)

// ropPMT is RAB$M_PMT, the ROP bit asking a terminal $GET to prompt;
// ropASY is RAB$M_ASY, asking for an operation that would wait to return
// RMS$_PENDING instead, and finish at $WAIT (only a record device's $PUT
// can wait so).
var (
	ropPMT = vmsConst("RAB$M_PMT")
	ropASY = vmsConst("RAB$M_ASY")
)

// ropEOF is RAB$M_EOF: $CONNECT positions the stream at the end of the
// file, so $PUT appends.
var ropEOF = vmsConst("RAB$M_EOF")

// RAB$B_RAC values, the record access modes: in sequence; by key (a
// relative file's record number, an indexed file's key, or a record
// number on a fixed-length sequential file, keyed.go); and by record file
// address. Any other value is RMS$_RAC.
var (
	racSeq = byte(vmsConst("RAB$C_SEQ"))
	racKey = byte(vmsConst("RAB$C_KEY"))
	racRFA = byte(vmsConst("RAB$C_RFA"))
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
