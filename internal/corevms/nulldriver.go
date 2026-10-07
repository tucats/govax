package corevms

import "github.com/tucats/govax/internal/vm"

// The null device (docs/PHASE-45.md, subtask 11): NLA0:, which NL: names.
// It is VMS's bit bucket. Every write succeeds and the data is thrown
// away; every read finds the end of the file at once. It is the
// natural SYS$INPUT, SYS$OUTPUT, and SYS$ERROR for a process that has
// nothing to talk to ($CREPRC's input, output, and error, and `NL:` in
// a DEFINE). Any number of processes may have it assigned at once.
//
// The device is defined by vax.init (`define/device nla0/devclass=misc`);
// this file is only its driver.

// nullDeviceName is the null device's name, which NL is short for.
const nullDeviceName = "NLA0"

// nullFunctions is the null driver's function table: the same read,
// write, and mode functions the terminal and mailbox drivers have.
var nullFunctions = map[uint32]ioFunc{
	ioCode("IO$_READVBLK"):  nullRead,
	ioCode("IO$_READLBLK"):  nullRead,
	ioCode("IO$_READPBLK"):  nullRead,
	ioCode("IO$_WRITEVBLK"): nullWrite,
	ioCode("IO$_WRITELBLK"): nullWrite,
	ioCode("IO$_WRITEPBLK"): nullWrite,
	ioCode("IO$_WRITEOF"):   nullAccept,
	ioCode("IO$_SENSEMODE"): nullSense,
	ioCode("IO$_SENSECHAR"): nullSense,
	ioCode("IO$_SETMODE"):   nullAccept,
	ioCode("IO$_SETCHAR"):   nullAccept,
}

// nullRead is SS$_ENDOFFILE with nothing transferred. The caller's
// buffer isn't touched.
func nullRead(_ *Environment, _ *ioRequest) (ioStatus, uint32) {
	return ioStatus{status: ssEndOfFile}, 0
}

// nullWrite discards p2 bytes at p1, reporting them all transferred. The
// buffer must be readable: a write from memory the process can't read is
// SS$_ACCVIO, as for any device.
func nullWrite(env *Environment, req *ioRequest) (ioStatus, uint32) {
	size := req.p[1] & 0xFFFF

	if size != 0 && !env.accessible(req.p[0], size, vm.AccessRead) {
		return ioStatus{}, ssAccVio
	}

	return ioStatus{status: ssNormal, count: uint16(size)}, 0
}

// nullAccept is a function that does nothing and succeeds.
func nullAccept(_ *Environment, _ *ioRequest) (ioStatus, uint32) {
	return ioStatus{status: ssNormal}, 0
}

// nullSense returns the device's class and type in the first two bytes
// of the buffer at p1 (of p2 bytes, at most 8), the rest zero, as the
// other drivers' sense-mode functions lay it out.
func nullSense(env *Environment, req *ioRequest) (ioStatus, uint32) {
	size := min(req.p[1], 8)
	d := req.channel.Device

	data := []byte{byte(d.DevClass), byte(d.DevType), 0, 0, 0, 0, 0, 0}[:size]

	if size != 0 {
		if !env.accessible(req.p[0], size, vm.AccessWrite) {
			return ioStatus{}, ssAccVio
		}

		if err := env.mem.Store(env.cpu, req.p[0], data); err != nil {
			return ioStatus{}, ssAccVio
		}
	}

	return ioStatus{status: ssNormal}, 0
}
