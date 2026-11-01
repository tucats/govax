package rms

import (
	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
)

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
// VMS 7.1 confirmed it (testdata/for81, FOR81R; keyedput.go is the $PUT): RMS$_KEY for a record
// number of 0, RMS$_EOF past the end of the file, RMS$_RAC for a file of
// another format, and an odd record size padded to a word.

var (
	// rmsInvalidKey is RMS$_KEY: a record number that names no record
	// (0); rmsInvalidKeySize RMS$_KSZ, a RAB$B_KSZ other than 0 or 4.
	rmsInvalidKey     = vmsConst("RMS$_KEY")
	rmsInvalidKeySize = vmsConst("RMS$_KSZ")
)

// keyedNumber returns the record number RAB$L_KBF names, for a file with
// attributes attr, or the status for why it can't be used; err is a failed
// read of the RAB or the key buffer, as for its other fields.
func (ctx *Context) keyedNumber(rabAddr uint32, attr ondisk.RecAttr) (uint32, uint32, error) {
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

	// A number past 2,147,483,647 (FORTRAN's -1) is RMS$_KEY too, govax's
	// choice: FORRTL's direct READ and WRITE of record -1 failed with a status
	// from RMS (testdata/for81's #553), which VMS's run didn't show, and a
	// relative file's records are numbered no higher.
	if number == 0 || number > 0x7FFFFFFF {
		return 0, rmsInvalidKey, nil
	}

	return number, 0, nil
}

// keyedRecord returns the byte offset of the record RAB$L_KBF's record
// number names in h's file, or the status for why there is none; err is
// a failed read of the RAB or the key buffer.
func (ctx *Context) keyedRecord(rabAddr uint32, h *FileHandle) (int64, uint32, error) {
	attr := h.File.Header.RecordAttributes

	number, sts, err := ctx.keyedNumber(rabAddr, attr)
	if err != nil || sts != 0 {
		return 0, sts, err
	}

	off := odsrms.FixedRecordOffset(attr, number)
	if off >= h.endOfFile() {
		return 0, rmsEOF, nil
	}

	return off, 0, nil
}
