package corevms

import (
	"fmt"
	"strings"

	iodev "github.com/tucats/govax/internal/io"
)

// $GETCHN and $GETDEV (docs/planning/PHASE-113 - Fortran errata.md, subtask
// 8): VMS V1's services for information about a device, which $GETDVI
// replaced in V3 (the Internals book's 18.7.1.1) and which VMS still has for
// old programs. $GETCHN finds the device by a channel and $GETDEV by a name;
// both then copy the same block, the device information buffer (DIB), into
// the caller's primary buffer, and into the secondary one the same block for
// the secondary device: a device's associated mailbox, a spooled device's
// intermediate one, or else the device itself. govax's devices have neither,
// so both buffers get the device's own block.
//
// No manual we may use describes the block: the 5.0 System Services manual
// lists the two services only as obsolete, and VSI's only names them. Its
// layout is $DIBDEF's, as VMS 7.1's MACRO assembled it (testdata/svc113's
// def_dib.obj), and what VMS wrote there is the probe's (Phase 113, subtask
// 13b):
//
//	  0  L  DEVCHAR    the device characteristics (DVI$_DEVCHAR)
//	  4  B  DEVCLASS   its class
//	  5  B  DEVTYPE    its type
//	  6  W  DEVBUFSIZ  its buffer size
//	  8  L  DEVDEPEND  its device-dependent characteristics (a disk's
//	                   SECTORS, TRACKS, and CYLINDERS at 8, 9, and 10)
//	 12  W  UNIT       its unit number
//	 14  W  DEVNAMOFF  the offset in the buffer of its controller's name
//	 16  L  PID        the owner process's PID
//	 20  L  OWNUIC     the owner's UIC
//	 24  W  VPROT      the volume's protection
//	 26  W  ERRCNT     its error count
//	 28  L  OPCNT      its operation count
//	 32  W  VOLNAMOFF  the offset of the volume's label, 0 with none mounted
//	 34  W  RECSIZ     its blocked record size
//	 36  T  DEVNAME    76 bytes for the names
//	112  L  MAXBLOCK   the volume's size in blocks
//	116                DIB$K_LENGTH
//
// The names are counted strings in DEVNAME's room: the controller's name
// (DEVNAMOFF 36), the device's without its unit ("DUA" for DUA1; LIB$ASN_WTH_MBX's
// manual page shows a program putting "MBcu" back together from it and UNIT),
// then the volume's label, blank-padded to 12, if one is mounted. VMS writes
// the fixed fields, the names, and MAXBLOCK, and leaves the rest of DEVNAME's
// room as the caller had it.

// The device information buffer's fields and size.
const (
	dibDevChar   = 0
	dibDevClass  = 4
	dibDevType   = 5
	dibDevBufSiz = 6
	dibDevDepend = 8
	dibUnit      = 12
	dibDevNamOff = 14
	dibPID       = 16
	dibOwnUIC    = 20
	dibVProt     = 24
	dibErrCnt    = 26
	dibOpCnt     = 28
	dibVolNamOff = 32
	dibRecSiz    = 34
	dibDevName   = 36
	dibMaxBlock  = 112
	dibLength    = 116
)

// deviceInfo is a device information buffer as VMS writes it: the whole
// block, and which of its bytes are written.
type deviceInfo struct {
	block   []byte
	written []bool
}

// deviceInformation is d's device information buffer.
func (env *Environment) deviceInformation(d *iodev.Device) deviceInfo {
	b := make([]byte, dibLength)
	w := make([]bool, dibLength)

	long := func(at int, v uint32) {
		b[at], b[at+1], b[at+2], b[at+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	word := func(at int, v uint32) { b[at], b[at+1] = byte(v), byte(v>>8) }

	owner := env.deviceOwner(d)
	volume := env.volumeInfo(d)

	long(dibDevChar, env.devChar(d))
	b[dibDevClass] = byte(d.DevClass)
	b[dibDevType] = byte(d.DevType)
	word(dibDevBufSiz, d.DevBufSize)
	long(dibDevDepend, env.devDepend(d))
	word(dibUnit, unitNumber(d))
	long(dibPID, owner.pid)
	long(dibOwnUIC, owner.uic)
	word(dibVProt, env.deviceProtection(d))
	word(dibErrCnt, d.ErrCnt)
	long(dibOpCnt, env.operationCount(d))
	word(dibRecSiz, d.RecSize)
	long(dibMaxBlock, volume.maxBlock)

	for i := range dibDevName {
		w[i] = true
	}

	for i := dibMaxBlock; i < dibLength; i++ {
		w[i] = true
	}

	at := dibDevName
	counted := func(s string) int {
		start := at

		s = s[:min(len(s), dibMaxBlock-at-1)]
		b[at] = byte(len(s))
		copy(b[at+1:], s)

		for i := at; i <= at+len(s); i++ {
			w[i] = true
		}

		at += 1 + len(s)

		return start
	}

	word(dibDevNamOff, uint32(counted(controllerName(d))))

	if volume.label != "" {
		word(dibVolNamOff, uint32(counted(fmt.Sprintf("%-12s", volume.label))))
	}

	return deviceInfo{block: b, written: w}
}

// controllerName is d's name without its unit number: "DUA" for DUA1.
func controllerName(d *iodev.Device) string {
	return strings.TrimRight(d.Name, "0123456789")
}

// storeDeviceInformation writes info into the buffer the descriptor at buf
// describes, as much as fits, and its length (the buffer's, at most the
// block's) to the word at length. A buffer address of 0 is no buffer, and its
// length isn't written. It returns SS$_BUFFEROVF if info was cut short,
// SS$_ACCVIO if either can't be written, else 0.
func (env *Environment) storeDeviceInformation(length, buf uint32, info deviceInfo) uint32 {
	if buf == 0 {
		return 0
	}

	size, err := env.mem.LoadWord(env.cpu, buf)
	if err != nil {
		return ssAccVio
	}

	text, err := env.mem.LoadLongword(env.cpu, buf+4)
	if err != nil {
		return ssAccVio
	}

	n := min(int(size), dibLength)
	for i := range n {
		if !info.written[i] {
			continue
		}

		if err := env.mem.StoreByte(env.cpu, text+uint32(i), info.block[i]); err != nil {
			return ssAccVio
		}
	}

	if length != 0 {
		if err := env.mem.StoreWord(env.cpu, length, uint16(n)); err != nil {
			return ssAccVio
		}
	}

	if n < dibLength {
		return ssBufferOvf
	}

	return 0
}

// deviceInformationService is $GETCHN's and $GETDEV's common part, once the
// device is found: both buffers, the primary's status first.
func (env *Environment) deviceInformationService(d *iodev.Device, argv []uint32) uint32 {
	info := env.deviceInformation(d)

	primary := env.storeDeviceInformation(optArg(argv, 1), optArg(argv, 2), info)
	if primary == ssAccVio {
		return primary
	}

	secondary := env.storeDeviceInformation(optArg(argv, 3), optArg(argv, 4), info)
	if secondary == ssAccVio {
		return secondary
	}

	if primary != 0 {
		return primary
	}

	if secondary != 0 {
		return secondary
	}

	return ssNormal
}

// serviceSysGetchn is SYS$GETCHN:
//
//	SYS$GETCHN chan ,[prilen] ,[pribuf] ,[scdlen] ,[scdbuf]
//
// The device information buffers (above) of the device channel chan (its
// low word) is assigned to: SS$_IVCHAN if it isn't assigned, 0 included,
// SS$_NOPRIV if it was assigned from a more privileged mode, as $GETDVI's
// channel is checked. Fewer than its five arguments is SS$_INSFARG, as the
// system service dispatcher checks a service's count (VMS 7.1's $GETDEV).
func serviceSysGetchn(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 5 {
		return ssInsfArg, nil
	}

	chanNum := optArg(argv, 0) & 0xFFFF
	if chanNum == 0 {
		return ssIvChan, nil
	}

	d, st := env.dviTarget(chanNum, 0)
	if st != 0 {
		return st, nil
	}

	return env.deviceInformationService(d, argv), nil
}

// serviceSysGetdev is SYS$GETDEV:
//
//	SYS$GETDEV devnam ,[prilen] ,[pribuf] ,[scdlen] ,[scdbuf]
//
// The device information buffers of the device devnam names, a device or
// a logical name for one, as $GETDVI reads it: SS$_IVDEVNAM for no name,
// SS$_IVLOGNAM for an empty or too long one, SS$_NOSUCHDEV for no such
// device; fewer than five arguments SS$_INSFARG.
func serviceSysGetdev(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 5 {
		return ssInsfArg, nil
	}

	d, st := env.dviTarget(0, optArg(argv, 0))
	if st != 0 {
		return st, nil
	}

	return env.deviceInformationService(d, argv), nil
}

func registerDeviceInformationServices(t *ServiceTable) {
	t.Register("SYS$GETCHN", serviceSysGetchn)
	t.Register("SYS$GETDEV", serviceSysGetdev)
}
