package rms

import "github.com/tucats/ods2/ondisk"

// Random access by relative record number (Phase 72).
//
// A sequential file of fixed-length records on disk can be read at
// random by record number: RAB$B_RAC is RAB$C_KEY, RAB$L_KBF points to
// a longword holding the number (the first record is 1), and RAB$B_KSZ
// is 4, or 0 for the default of 4 (the RMS Reference Manual, RAB$B_KSZ;
// the Guide to OpenVMS File Applications, 8.4.1.2). FORTRAN's direct
// access READ (READ (u, REC=n)) is this; DUNGEON reads its message text
// that way.
//
// Unconfirmed (no probe yet): the statuses for a record number of 0
// (RMS$_KEY here) and for one past the end of the file (RMS$_EOF here),
// RMS$_RAC for a file of another format, and whether an odd record size
// is padded to a word: govax lays fixed-length records end to end, as
// its sequential reads do.

var (
	// rmsInvalidKey is RMS$_KEY: a record number that names no record
	// (0); rmsInvalidKeySize RMS$_KSZ, a RAB$B_KSZ other than 0 or 4.
	rmsInvalidKey     = vmsConst("RMS$_KEY")
	rmsInvalidKeySize = vmsConst("RMS$_KSZ")
)

// keyedRecord returns the byte offset of the record RAB$L_KBF's record
// number names in h's file, or the status for why there is none; err is
// a failed read of the RAB or the key buffer, as for its other fields.
func (ctx *Context) keyedRecord(rabAddr uint32, h *FileHandle) (int64, uint32, error) {
	attr := h.File.Header.RecordAttributes
	if attr.Format != ondisk.RecordFormatFixed || attr.MaxRecordSize == 0 {
		return 0, rmsInvalidRAC, nil
	}

	ksz, err := ctx.loadByte(rabAddr + rabKSZ)
	if err != nil {
		return 0, 0, err
	}

	if ksz != 0 && ksz != 4 {
		return 0, rmsInvalidKeySize, nil
	}

	kbf, err := ctx.loadLongword(rabAddr + rabKBF)
	if err != nil {
		return 0, 0, err
	}

	number, err := ctx.loadLongword(kbf)
	if err != nil {
		return 0, 0, err
	}

	if number == 0 {
		return 0, rmsInvalidKey, nil
	}

	off := int64(number-1) * int64(attr.MaxRecordSize)
	if off >= h.endOfFile() {
		return 0, rmsEOF, nil
	}

	return off, 0, nil
}
