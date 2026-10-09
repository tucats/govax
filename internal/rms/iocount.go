package rms

// The process's I/O counts (docs/PHASE-49.md, subtask 11; corevms's
// iocount.go keeps them): what RMS's own I/O adds to JPI$_BUFIO and
// JPI$_DIRIO. VMS's RMS reaches the disk through the file system's
// calls, which are buffered I/O, and moves records through multiblock
// buffers, one direct I/O per buffer read or written; the file system's
// own disk transfers (headers, directories, bitmaps) are counted to the
// process too. govax has neither RMS's buffers nor the file system's
// I/O, so it counts by a model fitted to VMS 7.3's run
// (testdata/probe49, step 8):
//
//   - $CREATE of a file: 2 buffered and 5 direct (VMS: $CREATE, ten
//     80-byte $PUTs, and $CLOSE were 3 buffered and 7 direct);
//   - $OPEN: 1 buffered; $ERASE and $RENAME: 1 buffered;
//   - $CLOSE: 1 buffered, and 1 direct if the stream wrote anything
//     (VMS: $OPEN, $GETs, $CLOSE of the same file were 2 and 1);
//   - each multiblock buffer's worth of the file (ioBufferBlocks
//     blocks) a stream reads, or writes, for the first time in a row:
//     1 direct;
//   - a record read from or written to the terminal: 1 buffered.
//
// *Unconfirmed*: one run settles only those two totals; how VMS's
// counts grow with file size, the buffer count, and other services
// isn't known.

// ioBufferBlocks is the size of the multiblock buffer the model counts
// a direct I/O for: RMS's default multiblock count for a sequential file
// on disk (SYSGEN's RMS_DFMBC; unconfirmed that it's 16 on VMS 7.3).
const ioBufferBlocks = 16

// The counts the model gives the file system's calls.
const (
	ioCreateBuffered, ioCreateDirect = 2, 5
	ioOpenBuffered                   = 1
	ioCloseBuffered                  = 1
)

// countIO counts buffered and direct I/O operations for the caller,
// if it asked for counts (Context.CountIO).
func (ctx *Context) countIO(buffered, direct uint32) {
	if ctx.CountIO != nil && buffered+direct > 0 {
		ctx.CountIO(buffered, direct)
	}
}

// ioBuffer is a stream's last buffer read or written: its number plus
// one (0: none yet).
type ioBuffer int64

// touch counts a direct I/O when byte offset off is in another buffer
// than the last one b saw, and makes it the last.
func (b *ioBuffer) touch(ctx *Context, off int64) {
	n := ioBuffer(off/(ioBufferBlocks*512) + 1)
	if *b != n {
		*b = n
		ctx.countIO(0, 1)
	}
}
