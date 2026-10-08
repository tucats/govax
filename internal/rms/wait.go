package rms

// SysWait implements SYS$WAIT: given a RAB's VAX address (argv[0]), wait
// for the operation the RAB last started, and return its completion
// status.
//
// A program that sets RAB$V_ASY in RAB$L_ROP asks RMS to start each
// record operation ($GET, $PUT, $CONNECT, ...) and return at once, and
// calls $WAIT on the RAB before it looks at the operation's results.
// govax's RMS finishes every operation before its service returns, with
// or without RAB$V_ASY, so by the time $WAIT is called there is nothing
// left to wait for: it returns RAB$L_STS, which the operation stored,
// exactly as RMS's $WAIT returns the status of the operation it waited
// for.
func SysWait(ctx *Context, argv []uint32) (uint32, error) {
	return ctx.loadLongword(argv[0] + rabSTS)
}
