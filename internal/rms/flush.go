package rms

// SysFlush implements SYS$FLUSH (RAB at argv[0]): what the stream has
// written goes to the disk, without closing the file. For a disk file
// the Writer's partly filled last block is written and the file's end of
// file moved past it (ods2's Writer.Flush), and the file's header is
// written back, so the end of file a XABFHC or another program reads is
// current: the RMS manual's XABFHC description says an unshared file's
// end of file is "the values at the time of the last Close or Flush".
// (A shared stream writes each record through already; its $FLUSH only
// writes the header.) A stream that only reads, the terminal, and a
// record device have nothing to flush.
func SysFlush(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	ifi, err := ctx.loadWord(rabAddr + rabISI)
	if err != nil {
		return 0, err
	}

	handle, ok := ctx.Files.Lookup(ifi)
	if !ok {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsInvalidIFI)
	}

	if handle.IsConsole() || handle.IsRecordDevice() || handle.Writer == nil {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
	}

	if err := handle.Writer.Flush(); err != nil {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsDeviceError)
	}

	if err := handle.File.WriteAttributes(); err != nil {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsDeviceError)
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}
