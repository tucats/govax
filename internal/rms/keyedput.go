package rms

import (
	"github.com/tucats/ods2/ondisk"
	odsrms "github.com/tucats/ods2/rms"
)

// putKeyed is $PUT with RAB$B_RAC RAB$C_KEY on a sequential file of fixed-length
// records: the record goes where its number says (FORTRAN's direct access WRITE).
// As VMS 7.1's did (testdata/for81, FOR81R's FIX cases):
//
//   - another format is RMS$_RAC, a record number of 0 RMS$_KEY, a size other
//     than the file's RMS$_RSZ;
//   - over a record that's there, RMS$_NEF without RAB$V_UIF, and with it the
//     record is replaced;
//   - past the end the file grows, the records between zeros, which read as
//     records.
//
// What VMS's run shows of a sequential $PUT after one is odder: the stream's
// place doesn't move, so on a stream at the end of the file (a new file, say) a
// keyed $PUT that grows it leaves the next sequential $PUT going where the end
// was, the file then ending after that record (case "FIX 16, keyed then
// sequential": record 4, then a sequential $PUT, left one record). streamContext's
// heldEnd is that. A sequential $GET in that state is RMS$_EOF (unconfirmed:
// the case had a sequential $PUT in between).
func (ctx *Context) putKeyed(rabAddr uint32, h *FileHandle, record []byte) (uint32, error) {
	fail := func(sts uint32) (uint32, error) {
		h.stream.lost()

		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, sts)
	}

	attr := h.File.Header.RecordAttributes

	number, sts, err := ctx.keyedNumber(rabAddr, attr)
	if err != nil {
		return 0, err
	}

	if sts != 0 {
		return fail(sts)
	}

	if len(record) != int(attr.MaxRecordSize) {
		return fail(rmsRecordTooBig)
	}

	rop, err := ctx.loadLongword(rabAddr + rabROP)
	if err != nil {
		return 0, err
	}

	if err := h.flushWriter(); err != nil {
		return fail(rmsDeviceError)
	}

	at := odsrms.FixedRecordOffset(attr, number)
	end := h.endOfFile()

	if at < end && rop&ropUIF == 0 {
		return fail(rmsNotAtEOF)
	}

	data := make([]byte, 0, max(at-end, 0)+int64(len(record))+1)
	start := at

	// The gap's bytes are written as zeros: what a block held past an old end
	// of file isn't the file's.
	if at > end {
		data = append(data, make([]byte, at-end)...)
		start = end
	}

	data = append(data, record...)
	if len(record)%2 != 0 {
		data = append(data, 0)
	}

	h.wrote(ctx, at)

	if err := writeAt(h.File, start, data); err != nil {
		return fail(rmsDeviceError)
	}

	if last := start + int64(len(data)); last > end {
		s := &h.stream
		if h.atEndOfFile() {
			s.next, s.atEnd, s.heldEnd = end, false, true
		}

		h.File.SetEndOfFile(uint32(last/ondisk.BlockSize)+1, uint16(last%ondisk.BlockSize))
	}

	if err := ctx.storeRFA(rabAddr, rfaAt(at)); err != nil {
		return 0, err
	}

	h.stream.lost()

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}
