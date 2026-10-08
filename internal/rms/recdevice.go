package rms

// RMS on record devices (docs/PHASE-46.md, subtask 6): a file
// specification that names a mailbox or the null device (NL:) opens a
// record stream on the device, not a file on a volume.
//
// A record-oriented device has no files, no directories, and no
// attributes; RMS reads and writes it a record at a time, each record
// one device transfer. On a mailbox, $PUT writes one message, and
// finishes when the message has been read (VMS 7.3, testdata/mp/probe3):
// the process waits for that, or, with RAB$V_ASY, the $PUT returns
// RMS$_PENDING and $WAIT waits. $GET reads one: the oldest waiting, or,
// with none, the next to arrive (the process waits for it); an
// end-of-file message is RMS$_EOF (RAB$L_STV 0). On NL:, $PUT throws the
// record away and $GET is always RMS$_EOF. $CLOSE gives back the channel
// $CREATE or $OPEN assigned.
//
// That's what lets a process's SYS$OUTPUT be a mailbox: a subprocess
// created with its output going to a mailbox writes its lines there, and
// its creator reads them.
//
// The devices themselves (their message queues, waiting, protection) are
// internal/corevms's, which this package can't import; Context.Devices
// is how it reaches them.

// RecordDevice is a record-oriented device RMS has opened: one channel
// to a mailbox or NL:. Statuses are system service (SS$_) statuses,
// which RMS turns into its own.
type RecordDevice interface {
	// Characteristics is the device's DEVCHAR, which FAB$L_DEV and
	// FAB$L_SDC report.
	Characteristics() uint32

	// Put writes record, and finishes when the record has been read. If
	// it can't finish yet, an asynchronous Put (async) reports pending,
	// and WaitPut finishes it later; a synchronous one returns the
	// process's wait as its error. A non-nil error is the process
	// waiting (the service is called again later, and Put carries on
	// with the same record) or a failure of the machine, not an RMS
	// condition.
	Put(record []byte, async bool) (status uint32, pending bool, err error)

	// WaitPut waits for a pending Put to finish, returning its status
	// (SS$_NORMAL if none is pending). A non-nil error is as for Put.
	WaitPut() (status uint32, err error)

	// Get reads a record: SS$_ENDOFFILE at the end of the data, or the
	// record and SS$_NORMAL. A non-nil error is as for Put.
	Get() (record []byte, status uint32, err error)

	// Close gives back the device's channel.
	Close()
}

// DeviceOpener opens record devices for RMS: Context.Devices.
type DeviceOpener interface {
	// OpenRecordDevice opens device (a physical device name, without its
	// underscore and colon) for the access fac asks for (FAB$V_GET,
	// FAB$V_PUT). found is false if device isn't a record device RMS
	// opens this way; status is SS$_NORMAL, or why it can't be opened.
	OpenRecordDevice(device string, fac byte) (dev RecordDevice, found bool, status uint32)
}

// The RMS statuses of a record device's failures: the operation's
// status, its STV the system service status.
var (
	rmsPending    = vmsConst("RMS$_PENDING")
	rmsReadError  = vmsConst("RMS$_RER")
	rmsWriteError = vmsConst("RMS$_WER")
	ssEndOfFile   = vmsConst("SS$_ENDOFFILE")
	ssNoPriv      = vmsConst("SS$_NOPRIV")
	ssMbTooSml    = vmsConst("SS$_MBTOOSML")
)

// IsRecordDevice reports whether h is a record device's stream.
func (h *FileHandle) IsRecordDevice() bool {
	return h.Device != nil
}

// openRecordDevice is $CREATE's and $OPEN's step for a name whose
// device lookup is: if it's a record device, it opens a stream on it,
// allocates the IFI, and reports the device in the FAB (FAB$L_DEV and
// FAB$L_SDC, its characteristics), and stores 0 in FAB$W_MRS: VMS 7.3
// did, over 1234 (testdata/mp/probe5/vax, step 6; probe3 had seen 0
// over 0). found
// is false if lookup isn't a record device. A device that can't be
// opened is RMS$_PRV for SS$_NOPRIV, and RMS$_DNR otherwise, with the
// system service status in FAB$L_STV.
func (ctx *Context) openRecordDevice(fabAddr uint32, lookup string, fac byte) (ifi uint16, found bool, failStatus, stv uint32, err error) {
	if ctx.Devices == nil {
		return 0, false, 0, 0, nil
	}

	dev, found, status := ctx.Devices.OpenRecordDevice(normalizeDeviceName(lookup), fac)
	if !found {
		return 0, false, 0, 0, nil
	}

	switch {
	case status == ssNoPriv:
		return 0, true, rmsPrivilegeViolation, status, nil
	case status != ssNormal:
		return 0, true, rmsDeviceNotReady, status, nil
	}

	char := dev.Characteristics()

	for _, s := range []struct{ off, v uint32 }{{fabDEV, char}, {fabSDC, char}} {
		if err := ctx.storeLongword(fabAddr+s.off, s.v); err != nil {
			dev.Close()

			return 0, true, 0, 0, err
		}
	}

	if err := ctx.storeWord(fabAddr+fabMRS, 0); err != nil {
		dev.Close()

		return 0, true, 0, 0, err
	}

	return ctx.Files.Alloc(&FileHandle{Device: dev, Access: fac}), true, 0, 0, nil
}

// putRecordDevice is $PUT's step for a record device: the record goes
// to the device, and the $PUT finishes when it has been read: until then
// the process waits, or, with RAB$V_ASY, the $PUT is RMS$_PENDING and
// $WAIT finishes it (waitRecordDevice). A record longer than the device
// takes is RMS$_RSZ; any other failure RMS$_WER, with the system service
// status in RAB$L_STV.
func putRecordDevice(ctx *Context, rabAddr uint32, h *FileHandle, record []byte) (uint32, error) {
	if h.Access&facPut == 0 {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsPrivilegeViolation)
	}

	rop, err := ctx.loadLongword(rabAddr + rabROP)
	if err != nil {
		return 0, err
	}

	status, pending, err := h.Device.Put(record, rop&ropASY != 0)
	if err != nil {
		return 0, err
	}

	if pending {
		h.PutPending = true

		return rabStatus(ctx, rabAddr, rmsPending, 0)
	}

	return putResult(ctx, rabAddr, status)
}

// waitRecordDevice is $WAIT's step for a record device whose $PUT is
// pending: it waits for the record to be read, then stores the $PUT's
// status as putRecordDevice would have.
func waitRecordDevice(ctx *Context, rabAddr uint32, h *FileHandle) (uint32, error) {
	status, err := h.Device.WaitPut()
	if err != nil {
		return 0, err
	}

	h.PutPending = false

	return putResult(ctx, rabAddr, status)
}

// putResult stores a record device $PUT's RMS status for the system
// service status of its write.
func putResult(ctx *Context, rabAddr, status uint32) (uint32, error) {
	switch status {
	case ssNormal:
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
	case ssMbTooSml:
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsRecordTooBig)
	}

	return rabStatus(ctx, rabAddr, rmsWriteError, status)
}

// getRecordDevice is $GET's step for a record device: the next record,
// stored as storeRecord stores a file's; RMS$_EOF at the end of the
// data; RMS$_RER, with the system service status in RAB$L_STV, for any
// other failure.
func getRecordDevice(ctx *Context, rabAddr uint32, h *FileHandle) (uint32, error) {
	if h.Access&facGet == 0 {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsPrivilegeViolation)
	}

	record, status, err := h.Device.Get()
	if err != nil {
		return 0, err
	}

	switch status {
	case ssNormal:
		return storeRecord(ctx, rabAddr, record)
	case ssEndOfFile:
		// RAB$L_STV is 0 (VMS 7.3, testdata/mp/probe3).
		return rabStatus(ctx, rabAddr, rmsEOF, 0)
	}

	return rabStatus(ctx, rabAddr, rmsReadError, status)
}

// rabStatus stores sts and stv in a RAB's RAB$L_STS and RAB$L_STV,
// returning sts.
func rabStatus(ctx *Context, rab, sts, stv uint32) (uint32, error) {
	if err := ctx.storeLongword(rab+rabSTS, sts); err != nil {
		return 0, err
	}

	if err := ctx.storeLongword(rab+rabSTV, stv); err != nil {
		return 0, err
	}

	return sts, nil
}
