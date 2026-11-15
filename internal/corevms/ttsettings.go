package corevms

import (
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/registers"
)

// A terminal's characteristics as DCL's SET TERMINAL reads and changes them
// (docs/planning/PHASE-94 - privileges.md, subtask 6). VMS's SET TERMINAL
// is a program that senses the terminal's characteristics, changes the
// ones its qualifiers name, and sets them again: with IO$_SETMODE, which
// needs no privilege, or with /PERMANENT IO$_SETCHAR, which needs LOG_IO
// or PHY_IO. DCL's SET TERMINAL is Go code, so it comes here, where the
// same rule as the terminal driver's IO$_SETCHAR (ttdriver.go) is asked,
// from supervisor mode, DCL's.

// TerminalSettings are a terminal's characteristics, as IO$_SENSEMODE
// returns them (ttdriver.go's opening comment has the layout).
type TerminalSettings struct {
	// Device is the terminal's name, as SET TERMINAL's messages show it:
	// "TTA0:".
	Device string

	// Type is the terminal type, a DT$_ or TT$_ code (TT$_VT100).
	Type uint8

	// Width is the page width, Page the page length.
	Width uint16
	Page  uint8

	// Characteristics are the TT$M_ bits (DEVDEPEND's low three bytes),
	// Extended the TT2$M_ bits (DEVDEPND2).
	Characteristics uint32
	Extended        uint32
}

// TerminalSettings returns the characteristics of the terminal device
// names (a device or a logical name for one, as $ASSIGN takes it). The
// status is 0, the name's translation status, SS$_NOSUCHDEV for no such
// device, or SS$_IVDEVNAM for a device that isn't a terminal
// (unconfirmed: what VMS's SET TERMINAL says of a disk).
func (env *Environment) TerminalSettings(device string) (TerminalSettings, uint32) {
	d, st := env.terminalDevice(device)
	if st != 0 {
		return TerminalSettings{}, st
	}

	return TerminalSettings{
		Device:          d.Name + ":",
		Type:            uint8(d.DevType),
		Width:           uint16(d.DevBufSize),
		Page:            uint8(d.DevDepend >> 24),
		Characteristics: d.DevDepend & 0xFFFFFF,
		Extended:        d.DevDepend2,
	}, 0
}

// SetTerminalSettings sets the characteristics of the terminal s.Device
// names to s's, as IO$_SETMODE would, or with permanent as IO$_SETCHAR
// would: that needs LOG_IO or PHY_IO (SS$_NOPRIV, nothing changed). The
// status is 0, TerminalSettings' for the name, or the privilege's.
//
// VMS keeps a terminal's permanent characteristics apart from its current
// ones; govax keeps one set, so a permanent change is a current one too.
// Whether VMS 7.1's SET TERMINAL/PERMANENT changed the current
// characteristics before IO$_SETCHAR refused it is unconfirmed: govax
// changes nothing.
func (env *Environment) SetTerminalSettings(s TerminalSettings, permanent bool) uint32 {
	d, st := env.terminalDevice(s.Device)
	if st != 0 {
		return st
	}

	if permanent {
		if st := env.requirePrivilegeAt(ruleTerminalSetChar, registers.Supervisor); st != 0 {
			return st
		}
	}

	depend := s.Characteristics&0xFFFFFF | uint32(s.Page)<<24

	setTerminalCharacteristics(d, []byte{
		byte(d.DevClass), s.Type, byte(s.Width), byte(s.Width >> 8),
		byte(depend), byte(depend >> 8), byte(depend >> 16), byte(depend >> 24),
		byte(s.Extended), byte(s.Extended >> 8), byte(s.Extended >> 16), byte(s.Extended >> 24),
	})

	return 0
}

// terminalDevice is the terminal device names, or the status that says
// why there's none: TerminalSettings has them.
func (env *Environment) terminalDevice(device string) (*iodev.Device, uint32) {
	physical, st := env.deviceName(device)
	if st != 0 {
		return nil, st
	}

	d, found := env.Devices.Find(physical)
	if !found {
		return nil, ssNoSuchDev
	}

	if d.DevClass != iodev.DeviceClassTT {
		return nil, ssIvDevNam
	}

	return d, 0
}
