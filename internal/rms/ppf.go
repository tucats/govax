package rms

import (
	"errors"

	"github.com/tucats/govax/internal/lnm"
	odsrms "github.com/tucats/ods2/rms"
)

// Process-permanent files (docs/PHASE-71 - process-permanent files.md):
// the files RMS opens for a process, not for an image, and keeps open
// across image activations, until the process closes them or ends.
// LOGINOUT opens SYS$INPUT, SYS$OUTPUT, SYS$ERROR, and SYS$COMMAND this
// way; DCL opens a command procedure, its /OUTPUT file, and the files of
// its OPEN command this way (the Guide to OpenVMS File Applications,
// 6.6).
//
// Each process's FileTable keeps them as a table of PermanentEntry, in Go,
// as govax keeps its other per-process structures. An entry is one open
// record stream (a terminal, a volume file, a mailbox or NL:) with one
// position, and the access, sharing, and record attributes of the open
// that made it. A logical name leads to an entry through its equivalence
// string (lnm's ppf.go): ESC, 0, the entry's IFI word, the device.
//
// An image reaches an entry only indirectly: its $OPEN or $CREATE of the
// logical name opens nothing, but returns an IFI marked indirect, and
// every operation through it works on the entry's one stream. The
// guide's rules for the indirect accessor, and what VMS 7.1 showed of
// them (testdata/ppf71):
//
//   - $CREATE is $OPEN; neither does I/O; any number of them may be open.
//   - The FAB gets the file's attributes, the device characteristics,
//     the NAM's strings are the logical name ("SYS$OUTPUT:.;"), and
//     FAB$B_FAC is ignored: operations are checked against the original
//     open's access.
//   - $REWIND is RMS$_IOP; $ERASE of an open FAB is RMS$_IFI, as for any
//     open FAB; FOP's PPF is RMS$_PRV, its UFO and NFS RMS$_FOP.
//   - $CLOSE ends the indirect access, not the file, and image rundown
//     ends them all and closes nothing.
//
// # The numbers
//
// An IFI or ISI word of a process-permanent file has FAB$V_PPIFI (bit 15)
// set, record attributes in FAB$V_PPF_RAT (bits 6 to 13), and the entry's
// index in bits 0 to 5. The word in the equivalence string carries the
// RAT of the open that made the file (PRN for a print file, %X0100); an
// indirect accessor's FAB$W_IFI and RAB$W_ISI also have FAB$V_PPF_IND
// (bit 14), and the RAT of the accessor's own FAB as it was at $OPEN,
// which RMS saves there for $PUT's carriage control. The ISI's index is
// the entry's stream's, numbered apart from the files (VMS 7.1: SYS$INPUT
// file 2, stream 3). The numbers VMS gives depend on its process I/O
// segment; govax numbers entries and streams each from 1, lowest free
// first, in VMS's form.

// The IFI and ISI word's fields.
var (
	ppfIFI      = uint16(vmsConst("FAB$M_PPIFI"))
	ppfIndirect = uint16(vmsConst("FAB$M_PPF_IND"))
	ppfRATMask  = uint16(vmsConst("FAB$M_PPF_RAT"))
	ppfRATShift = vmsConst("FAB$V_PPF_RAT")
)

// ppfIndexMask is the entry's index in an IFI or ISI word.
const ppfIndexMask = 0x3F

// maxPermanent is how many process-permanent files a process may have
// open: the index field's largest value.
const maxPermanent = ppfIndexMask

// The FAB$L_FOP bits an indirect open refuses.
var (
	fopPPF = vmsConst("FAB$M_PPF")
	fopNFS = vmsConst("FAB$M_NFS")

	rmsFileOptionError = vmsConst("RMS$_FOP")
)

// fnbPPF is NAM$M_PPF: the name led to a process-permanent file.
var fnbPPF = vmsConst("NAM$M_PPF")

// Device characteristics (DEV$M_) an indirect open reports.
var (
	devREC = vmsConst("DEV$M_REC")
	devCCL = vmsConst("DEV$M_CCL")
	devTRM = vmsConst("DEV$M_TRM")
	devAVL = vmsConst("DEV$M_AVL")
	devIDV = vmsConst("DEV$M_IDV")
	devODV = vmsConst("DEV$M_ODV")
	devRND = vmsConst("DEV$M_RND")
	devFOD = vmsConst("DEV$M_FOD")
	devDIR = vmsConst("DEV$M_DIR")
)

// terminalDevChar is a terminal's characteristics, as VMS 7.1 reported
// its console terminal's (^X0C040007): record-oriented, carriage
// control, a terminal, available, input and output.
var terminalDevChar = devREC | devCCL | devTRM | devAVL | devIDV | devODV

// recordDevChar is what govax reports for a mailbox or NL: entry
// (unconfirmed): record-oriented, available, input and output.
var recordDevChar = devREC | devAVL | devIDV | devODV

// RMS record formats and attributes a stream with no file reports.
const (
	rfmVAR = 2
	rfmVFC = 3
	ratCR  = 2
	ratPRN = 4
)

// ErrPermanentTableFull is AddPermanent's error when the process has
// as many process-permanent files open as it may.
var ErrPermanentTableFull = errors.New("rms: process-permanent file table full")

// PermanentOptions are what AddPermanent needs of the open that made a
// process-permanent file, beyond its stream.
type PermanentOptions struct {
	// Device is the device's name as the equivalence string carries it
	// after its prefix: "_GOVAX$TTA0:", or, for DCL's OPEN, without the
	// colon ("_GOVAX$DUA1").
	Device string

	// RAT is the FAB$B_RAT of the open that made it, which the IFI word
	// carries: PRN for LOGINOUT's files, DCL's procedures and /OUTPUT
	// files, and OPEN/WRITE's; none for OPEN/READ's (VMS 7.1).
	RAT byte

	// SQO is FAB$V_SQO: the file is sequential only, as LOGINOUT's are.
	SQO bool

	// Characteristics, when not 0, are the device characteristics the
	// entry reports in place of its stream's kind's: a command
	// procedure's data records are a disk file's on VMS.
	Characteristics uint32
}

// PermanentEntry is one process-permanent file: its stream and what the
// open that made it asked for.
type PermanentEntry struct {
	// index is the entry's number, from 1, and stream its stream's.
	index, stream uint16

	// handle is the open stream, with one position however many
	// indirect accessors use it.
	handle *FileHandle

	options PermanentOptions

	// borrowed says the stream is another process's entry's (Share):
	// closing this entry leaves it open.
	borrowed bool

	// characteristics are the device's (FAB$L_SDC of an indirect open):
	// a volume file's without input (DEV$M_IDV) if the open doesn't
	// read, without output (DEV$M_ODV) if it doesn't write.
	characteristics uint32
}

// IFI is the entry's word as its equivalence string carries it.
func (e *PermanentEntry) IFI() uint16 {
	return ppfWord(e.index, e.options.RAT)
}

// EquivalenceString is the string a logical name naming the entry is
// defined as.
func (e *PermanentEntry) EquivalenceString() string {
	return lnm.PermanentFileString(e.IFI(), e.options.Device)
}

// Access is the FAB$B_FAC of the open that made the entry.
func (e *PermanentEntry) Access() byte { return e.handle.Access }

// Handle is the entry's stream.
func (e *PermanentEntry) Handle() *FileHandle { return e.handle }

// ppfWord is an IFI or ISI word for index with record attributes rat.
func ppfWord(index uint16, rat byte) uint16 {
	return ppfIFI | uint16(rat)<<ppfRATShift&ppfRATMask | index&ppfIndexMask
}

// indirectWord is an indirect accessor's IFI or ISI word.
func indirectWord(index uint16, rat byte) uint16 {
	return ppfWord(index, rat) | ppfIndirect
}

// wordRAT is the record attributes an IFI or ISI word carries.
func wordRAT(w uint16) byte {
	return byte((w & ppfRATMask) >> ppfRATShift)
}

// isIndirect reports whether w is an indirect accessor's IFI or ISI.
func isIndirect(w uint16) bool {
	return w&(ppfIFI|ppfIndirect) == ppfIFI|ppfIndirect
}

// AddPermanent makes h, an open stream, a process-permanent file of t's,
// armed for the access it was opened with (h.Access) if it isn't yet.
// It returns ErrPermanentTableFull when t has no room for another.
func (t *FileTable) AddPermanent(h *FileHandle, opts PermanentOptions) (*PermanentEntry, error) {
	index, stream := t.freePermanent()
	if index == 0 {
		return nil, ErrPermanentTableFull
	}

	e := &PermanentEntry{index: index, stream: stream, handle: h, options: opts}

	switch {
	case h.IsConsole():
		e.characteristics = terminalDevChar
	case h.IsRecordDevice():
		e.characteristics = recordDevChar
	default:
		e.characteristics = fileDevChar(h.Access)

		// A stream that only writes can only add to the file: it starts
		// at the end.
		var rop uint32
		if h.Access&facReads == 0 {
			rop = ropEOF
		}

		if h.Reader == nil && h.Writer == nil {
			if sts := h.arm(h.Access, rop); sts != 0 {
				return nil, errors.New("rms: process-permanent file can't be armed")
			}
		}
	}

	if opts.Characteristics != 0 {
		e.characteristics = opts.Characteristics
	}

	if t.permanent == nil {
		t.permanent = map[uint16]*PermanentEntry{}
	}

	t.permanent[index] = e

	return e, nil
}

// FileCharacteristics is fileDevChar for DCL, whose command procedure
// input entry reports a disk file's characteristics.
func FileCharacteristics(fac byte) uint32 { return fileDevChar(fac) }

// fileDevChar is a volume file's device characteristics as an entry
// reports them for an open with access fac: without input (DEV$M_IDV) if
// it doesn't read, without output (DEV$M_ODV) if it doesn't write.
func fileDevChar(fac byte) uint32 {
	c := diskDevChar

	if fac&facReads == 0 {
		c &^= devIDV
	}

	if fac&facPut == 0 {
		c &^= devODV
	}

	return c
}

// MakePermanent makes the file t has open as ifi (by $OPEN or $CREATE) a
// process-permanent file: it leaves the image's files, so rundown doesn't
// close it.
func (t *FileTable) MakePermanent(ifi uint16, opts PermanentOptions) (*PermanentEntry, error) {
	h, ok := t.handles[ifi]
	if !ok {
		return nil, errors.New("rms: no such file open")
	}

	e, err := t.AddPermanent(h, opts)
	if err != nil {
		return nil, err
	}

	delete(t.handles, ifi)

	return e, nil
}

// Adopt makes e, an entry of another table's (an earlier process 1's,
// which ZERO, INIT, or VMINIT replaced), an entry of t's on the same open
// stream, with the same options.
func (t *FileTable) Adopt(e *PermanentEntry) (*PermanentEntry, error) {
	return t.AddPermanent(e.handle, e.options)
}

// Share makes e, another process's entry, an entry of t's on the same
// stream, which closing this one leaves open: a subprocess writing to its
// creator's /OUTPUT file.
func (t *FileTable) Share(e *PermanentEntry) (*PermanentEntry, error) {
	shared, err := t.AddPermanent(e.handle, e.options)
	if err != nil {
		return nil, err
	}

	shared.borrowed = true

	return shared, nil
}

// freePermanent returns the lowest free entry and stream numbers, 0 when
// the table is full.
func (t *FileTable) freePermanent() (index, stream uint16) {
	usedStreams := map[uint16]bool{}
	for _, e := range t.permanent {
		usedStreams[e.stream] = true
	}

	for i := uint16(1); i <= maxPermanent; i++ {
		if _, used := t.permanent[i]; !used && index == 0 {
			index = i
		}

		if !usedStreams[i] && stream == 0 {
			stream = i
		}
	}

	if stream == 0 {
		index = 0
	}

	return index, stream
}

// Permanent returns the process-permanent file an IFI word names (the
// equivalence string's, or an indirect accessor's), and whether there is
// one.
func (t *FileTable) Permanent(ifi uint16) (*PermanentEntry, bool) {
	if ifi&ppfIFI == 0 {
		return nil, false
	}

	e, ok := t.permanent[ifi&ppfIndexMask]

	return e, ok
}

// indirectFile returns the entry an indirect FAB$W_IFI names.
func (t *FileTable) indirectFile(ifi uint16) (*PermanentEntry, bool) {
	if !isIndirect(ifi) {
		return nil, false
	}

	return t.Permanent(ifi)
}

// indirectStream returns the entry an indirect RAB$W_ISI names, by its
// stream's number.
func (t *FileTable) indirectStream(isi uint16) (*PermanentEntry, bool) {
	if !isIndirect(isi) {
		return nil, false
	}

	for _, e := range t.permanent {
		if e.stream == isi&ppfIndexMask {
			return e, true
		}
	}

	return nil, false
}

// LookupStream returns the stream a RAB$W_ISI names: an open file's (its
// ISI is its IFI), or an indirect accessor's process-permanent file's.
func (t *FileTable) LookupStream(isi uint16) (*FileHandle, bool) {
	if e, ok := t.indirectStream(isi); ok {
		return e.handle, true
	}

	return t.Lookup(isi)
}

// ClosePermanent closes the process-permanent file ifi names, as DCL's
// CLOSE and the end of a command procedure do. It returns false if there
// is none.
func (t *FileTable) ClosePermanent(ifi uint16) (bool, error) {
	e, ok := t.Permanent(ifi)
	if !ok {
		return false, nil
	}

	delete(t.permanent, e.index)

	if e.borrowed {
		return true, nil
	}

	return true, closeHandle(e.handle)
}

// ClosePermanentFiles closes every process-permanent file, as RMS's
// rundown does when the process is deleted. It returns how many it
// closed and the first error a close met.
func (t *FileTable) ClosePermanentFiles() (int, error) {
	var (
		closed   int
		firstErr error
	)

	for index, e := range t.permanent {
		if e.borrowed {
			delete(t.permanent, index)

			continue
		}

		if err := closeHandle(e.handle); err != nil && firstErr == nil {
			firstErr = err
		}

		closed++

		delete(t.permanent, index)
	}

	return closed, firstErr
}

// closeHandle ends an open stream with no FAB at hand: its record locks,
// a record device's stream, a volume file's records written. The
// terminal stays open.
func closeHandle(h *FileHandle) error {
	if h.IsConsole() {
		return nil
	}

	h.locks.release()

	if h.IsRecordDevice() {
		h.Device.Close()

		return nil
	}

	return closeVolumeFile(h, nil)
}

// arm connects h's record stream for access fac, as $CONNECT does: at
// the first record, or with RAB$V_EOF in rop at the end of the file. A
// direction already armed keeps its Reader or Writer, so a second
// stream on the file continues where the first is. It returns RMS$_DEV
// if the file can't be read or written.
func (h *FileHandle) arm(fac byte, rop uint32) uint32 {
	h.startStream(fac, rop)

	if fac&facPut != 0 && h.Writer == nil {
		w, err := odsrms.NewAppender(h.File)
		if err != nil {
			return rmsDeviceError
		}

		// A file others may read or write, or that this stream also
		// reads or changes in place, gets each record on the disk as
		// it's put, at the end of file as it is then.
		w.SetShared(h.Accessor != nil && h.stream.fresh)
		h.Writer = w
	}

	if fac&facReads != 0 && h.Reader == nil {
		r, err := odsrms.NewReader(h.File)
		if err != nil {
			return rmsDeviceError
		}

		h.Reader = r
	}

	return 0
}
