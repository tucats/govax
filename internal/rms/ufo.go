package rms

// User file open (FAB$V_UFO; docs/PHASE-49 - record updates and locks.md,
// subtask 9).
//
// A program that wants a file's blocks rather than its records asks RMS
// to open (or create) the file and then step aside: with FAB$V_UFO in
// FAB$L_FOP, $OPEN and $CREATE find the file, check access and sharing,
// and fill in the NAM and XABs as usual, but leave no RMS stream on it.
// Instead the file is left accessed on an I/O channel, whose number RMS
// returns in FAB$L_STV, and FAB$W_IFI stays 0 ($CONNECT and $CLOSE don't
// apply). The program reads and writes the file with $QIO on that channel
// (IO$_READVBLK, IO$_WRITEVBLK), or maps it into memory ($CRMPSC takes the
// channel), and closes it with $DASSGN. That is the RMS manual's
// description of UFO, and the System Services manual's for $CRMPSC: "The
// file must have been accessed with a VMS RMS $OPEN macro; the file
// options parameter (FOP) in the FAB must indicate a user file open".
//
// # How govax does it
//
// The open is an ordinary one up to the end (sharing.go's arbitration
// included); then the file's access moves from the IFI table to an
// ACPFile, the same kind of access a disk IO$_ACCESS makes, which keeps
// the open's place among the file's RMS openers until it ends, and
// Context.AssignFileChannel (internal/corevms) puts it on a new channel.
// A UFO open of a device that holds no files (the terminal, a mailbox,
// NL:) gives a channel to the device itself, with nothing accessed on
// it, as VMS 7.3's did (testdata/probe49, step 7f: $OPEN of NLA0: with
// UFO, RMS$_NORMAL and a channel in FAB$L_STV).

var (
	fopUFO     = vmsConst("FAB$M_UFO")
	rmsSupport = vmsConst("RMS$_SUPPORT")
	rmsChannel = vmsConst("RMS$_CHN")
	rmsSharing = vmsConst("RMS$_SHR")
)

// ufoSharing checks a user file open's FAB$B_SHR (fop its FAB$L_FOP):
// the RMS manual (FAB$V_UFO, FAB$V_UPI) requires FAB$V_UPI when the file
// is write shared (SHRPUT, SHRUPD, or SHRDEL), and VMS 7.3 refused such
// an open without it with RMS$_SHR (testdata/probe49, round 2, step 7).
// It returns that status, or 0.
func ufoSharing(fop uint32, shr byte) uint32 {
	if fop&fopUFO != 0 && shr&(shrPut|shrUpd|shrDel) != 0 && shr&shrUPI == 0 {
		return rmsSharing
	}

	return 0
}

// ufoEndOfFile is a user file open's $CREATE's end of file: the RMS
// manual (FAB$V_UFO) sets it "to the end of the block specified in the
// FAB$L_ALQ field on input" (alq, as the FAB had it before $CREATE
// wrote back the allocation rounded to whole clusters), so a section of
// the new file maps every block asked for. VMS 7.3 so: ALQ=10 on a
// volume of 3-block clusters, 12 blocks allocated, the end of file at
// block 10's end (testdata/probe49, round 3, step 7c).
func (ctx *Context) ufoEndOfFile(ifi uint16, alq uint32) {
	if h, ok := ctx.Files.Lookup(ifi); ok && h.File != nil && alq != 0 {
		h.File.SetEndOfFile(alq+1, 0)
	}
}

// userFileOpen finishes a $OPEN or $CREATE with FAB$V_UFO: the file just
// opened as IFI ifi, on device, is handed to a channel, whose number goes
// to FAB$L_STV, FAB$W_IFI is cleared, and FAB$L_STS gets status (the
// open's success status). A record device or the terminal is closed
// again and the channel is to the device. A process with no way to
// assign a channel is RMS$_SUPPORT; a channel that can't be assigned is
// RMS$_CHN, with the system service status in FAB$L_STV. On failure the
// file is closed again.
func (ctx *Context) userFileOpen(fabAddr uint32, ifi uint16, device string, status uint32) (uint32, error) {
	h, ok := ctx.Files.Lookup(ifi)
	if !ok {
		return fabStatus(ctx, fabAddr, rmsInvalidIFI, 0)
	}

	if ctx.AssignFileChannel == nil {
		ctx.closeHandle(ifi, h)

		return fabStatus(ctx, fabAddr, rmsSupport, 0)
	}

	if h.File == nil || h.Accessor == nil {
		ctx.closeHandle(ifi, h)

		channel, st := ctx.AssignFileChannel(device, nil)
		if st&1 == 0 {
			return fabStatus(ctx, fabAddr, rmsChannel, st)
		}

		if err := ctx.storeWord(fabAddr+fabIFI, 0); err != nil {
			return 0, err
		}

		return fabStatus(ctx, fabAddr, status, uint32(channel))
	}

	m := ctx.Mounts.mounts[normalizeDeviceName(device)]
	a := &ACPFile{
		file: h.File, fid: fileIDFrom(h.File.Header.Fid), writable: h.Writable,
		access: h.Accessor, usedAtAccess: h.File.UsedBlocks(), mount: m, claim: h.claim,
	}

	// The access is the ACPFile's now: releasing the IFI mustn't end it.
	ctx.Files.Release(ifi)

	channel, st := ctx.AssignFileChannel(device, a)
	if st&1 == 0 {
		_ = a.Deaccess()

		return fabStatus(ctx, fabAddr, rmsChannel, st)
	}

	if err := ctx.storeWord(fabAddr+fabIFI, 0); err != nil {
		return 0, err
	}

	return fabStatus(ctx, fabAddr, status, uint32(channel))
}

// closeHandle closes the file open as IFI ifi and frees the slot.
func (ctx *Context) closeHandle(ifi uint16, h *FileHandle) {
	switch {
	case h.IsRecordDevice():
		h.Device.Close()
	case !h.IsConsole():
		_ = closeVolumeFile(h, nil)
	}

	ctx.Files.Release(ifi)
}
