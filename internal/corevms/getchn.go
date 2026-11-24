package corevms

import (
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
// layout here is govax's reconstruction, unconfirmed until Phase 113's probe
// round (testdata/svc113) runs $DIBDEF and both services on VMS 7.1:
//
//	 0  L  DEVCHAR    the device characteristics (DVI$_DEVCHAR)
//	 4  B  DEVCLASS   its class
//	 5  B  DEVTYPE    its type
//	 6  W  DEVBUFSIZ  its buffer size
//	 8  L  DEVDEPEND  its device-dependent characteristics
//	12  W  UNIT       its unit number
//	14  W  DEVNAMOFF  the offset in the buffer of its controller's name
//	16  L  PID        the owner process's PID
//	20  L  OWNUIC     the owner's UIC
//	24  W  VPROT      the volume's protection
//	26  W  ERRCNT     its error count
//	28  L  OPCNT      its operation count
//	32  W  VOLNAMOFF  the offset of the volume's label, 0 with none mounted
//	34  W  RECSIZ     its blocked record size
//	36  L  MAXBLOCK   the volume's size in blocks
//	40  L  DEVDEPND2  more device-dependent characteristics
//	44     reserved, zero, to the block's end at 116 (DIB$K_LENGTH)
//
// after which come the names, each a counted string: the controller's name,
// the device's without its unit ("DUA" for DUA1; LIB$ASN_WTH_MBX's manual
// page shows a program putting "MBcu" back together from it and UNIT), then
// the volume's label if one is mounted.

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
	dibMaxBlock  = 36
	dibDevDepnd2 = 40
	dibLength    = 116
)

// deviceInformation is d's device information buffer: the block and the
// names after it.
func (env *Environment) deviceInformation(d *iodev.Device) []byte {
	b := make([]byte, dibLength)

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
	long(dibDevDepnd2, d.DevDepend2)

	counted := func(s string) {
		if len(s) > 255 {
			s = s[:255]
		}

		b = append(b, byte(len(s)))
		b = append(b, s...)
	}

	word(dibDevNamOff, uint32(len(b)))
	counted(controllerName(d))

	if volume.label != "" {
		word(dibVolNamOff, uint32(len(b)))
		counted(volume.label)
	}

	return b
}

// controllerName is d's name without its unit number: "DUA" for DUA1.
func controllerName(d *iodev.Device) string {
	return strings.TrimRight(d.Name, "0123456789")
}

// storeDeviceInformation writes info into the buffer the descriptor at buf
// describes, as much as fits, and its length to the word at length; either
// address may be 0, for no buffer. It returns the status for the buffer:
// SS$_BUFFEROVF if info was cut short (unconfirmed: VMS's status for a short
// buffer), SS$_ACCVIO if either can't be written, else 0.
func (env *Environment) storeDeviceInformation(length, buf uint32, info []byte) uint32 {
	if buf == 0 {
		return 0
	}

	n, truncated, err := storeDescriptor(env, buf, string(info))
	if err != nil {
		return ssAccVio
	}

	if length != 0 {
		if err := env.mem.StoreWord(env.cpu, length, n); err != nil {
			return ssAccVio
		}
	}

	if truncated {
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
// channel is checked.
func serviceSysGetchn(env *Environment, argv []uint32) (uint32, error) {
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
// device.
func serviceSysGetdev(env *Environment, argv []uint32) (uint32, error) {
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
