package corevms

import "github.com/tucats/govax/internal/vm"

// The null device (docs/PHASE-45.md, subtask 11): NLA0:, which NL: names.
// It is VMS's bit bucket. Every write succeeds and the data is thrown
// away; every read finds the end of the file at once. It is the
// natural SYS$INPUT, SYS$OUTPUT, and SYS$ERROR for a process that has
// nothing to talk to ($CREPRC's input, output, and error, and `NL:` in
// a DEFINE). Any number of processes may have it assigned at once.
//
// The device is defined by vax.init: VMS 7.1 reports NLA0: with class
// DC$_MAILBOX (160), type 3, and DEVCHAR 0C150001, so that is how it is
// defined, and qio.go's driverFor picks this driver by its type. This file
// is only the driver.

// nullDeviceName is the null device's name, which NL is short for.
const nullDeviceName = "NLA0"

// nullFunctions is the null driver's function table: reads, writes, and
// the functions that do nothing. Sense mode is not among them: VMS 7.1
// answered SS$_ILLIOFUNC (0xF4).
var nullFunctions = map[uint32]ioFunc{
	ioCode("IO$_READVBLK"):  nullRead,
	ioCode("IO$_READLBLK"):  nullRead,
	ioCode("IO$_READPBLK"):  nullRead,
	ioCode("IO$_WRITEVBLK"): nullWrite,
	ioCode("IO$_WRITELBLK"): nullWrite,
	ioCode("IO$_WRITEPBLK"): nullWrite,
	ioCode("IO$_WRITEOF"):   nullAccept,
	ioCode("IO$_SETMODE"):   nullAccept,
	ioCode("IO$_SETCHAR"):   nullAccept,
}

// nullRead is SS$_ENDOFFILE with nothing transferred. The caller's
// buffer isn't touched.
func nullRead(_ *Environment, _ *ioRequest) (ioStatus, uint32) {
	return ioStatus{status: ssEndOfFile}, 0
}

// nullWrite discards p2 bytes at p1. The IOSB's transfer count is 0 (VMS
// 7.1, testdata/mp/probe2). The buffer must be readable: a write from
// memory the process can't read is SS$_ACCVIO, as for any device.
func nullWrite(env *Environment, req *ioRequest) (ioStatus, uint32) {
	size := req.p[1] & 0xFFFF

	if size != 0 && !env.accessible(req.p[0], size, vm.AccessRead) {
		return ioStatus{}, ssAccVio
	}

	return ioStatus{status: ssNormal}, 0
}

// nullAccept is a function that does nothing and succeeds.
func nullAccept(_ *Environment, _ *ioRequest) (ioStatus, uint32) {
	return ioStatus{status: ssNormal}, 0
}
