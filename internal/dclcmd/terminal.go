package dclcmd

import (
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// SET TERMINAL (docs/planning/PHASE-94 - privileges.md, subtask 6): a
// terminal's characteristics changed, the ones its qualifiers name, the
// rest left as they are. /PERMANENT changes the permanent characteristics,
// which needs LOG_IO or PHY_IO, as the terminal driver's IO$_SETCHAR does;
// without the privilege nothing changes, and SET says so as VMS 7.1 did
// (testdata/priv94/vax's TERMINAL sections):
//
//	%SET-W-NOTSET, error modifying OPA0:
//	-SYSTEM-F-NOPRIV, insufficient privilege or object protection violation
//
// govax's terminal is its console, whose characteristics only programs
// read (IO$_SENSEMODE, F$GETDVI, SHOW DEVICE/FULL): changing them changes
// nothing about the host's terminal.

// TerminalOptions are SET TERMINAL's parameter and qualifiers.
type TerminalOptions struct {
	// Device is the terminal, "" for SYS$COMMAND.
	Device string

	// Width and Page are /WIDTH and /PAGE, nil when not given.
	Width, Page *int

	// DeviceType is /DEVICE_TYPE's terminal type ("VT100"), "" for none.
	DeviceType string

	// Permanent is /PERMANENT.
	Permanent bool

	// Switches are the characteristics' qualifiers given, by qualifier
	// name, true to set the characteristic and false (NO) to clear it.
	Switches map[string]bool
}

// TerminalSwitch is one of SET TERMINAL's characteristics' qualifiers: the
// TT$M_ (or TT2$M_) bit it sets, and whether it sets the bit's opposite
// (/ECHO clears TT$M_NOECHO; /UPPERCASE clears TT$M_LOWER).
type TerminalSwitch struct {
	Qualifier string
	Bit       string
	Inverted  bool
}

// TerminalSwitches are SET TERMINAL's characteristics' qualifiers, each a
// bit of the terminal's characteristics, as the DCL Dictionary describes
// them. UPPERCASE and HARDCOPY are the opposites of LOWERCASE and SCOPE,
// and take no NO. The grammar's qualifiers are these (TestTerminalSwitches).
var TerminalSwitches = []TerminalSwitch{
	{"ADVANCED_VIDEO", "TT2$M_AVO", false},
	{"ALTYPEAHD", "TT2$M_ALTYPEAHD", false},
	{"ANSI_CRT", "TT2$M_ANSICRT", false},
	{"APPLICATION_KEYPAD", "TT2$M_APP_KEYPAD", false},
	{"BLOCK_MODE", "TT2$M_BLOCK", false},
	{"BRDCSTMBX", "TT2$M_BRDCSTMBX", false},
	{"BROADCAST", "TT$M_NOBRDCST", true},
	{"DEC_CRT", "TT2$M_DECCRT", false},
	{"DEC_CRT2", "TT2$M_DECCRT2", false},
	{"DISCONNECT", "TT2$M_DISCONNECT", false},
	{"DMA", "TT2$M_DMA", false},
	{"ECHO", "TT$M_NOECHO", true},
	{"EDIT_MODE", "TT2$M_EDIT", false},
	{"EIGHT_BIT", "TT$M_EIGHTBIT", false},
	{"ESCAPE", "TT$M_ESCAPE", false},
	{"FORM", "TT$M_MECHFORM", false},
	{"HANGUP", "TT2$M_HANGUP", false},
	{"HARDCOPY", "TT$M_SCOPE", true},
	{"HOSTSYNC", "TT$M_HOSTSYNC", false},
	{"INSERT", "TT2$M_INSERT", false},
	{"LINE_EDITING", "TT2$M_EDITING", false},
	{"LOCAL_ECHO", "TT2$M_LOCALECHO", false},
	{"LOWERCASE", "TT$M_LOWER", false},
	{"MODEM", "TT$M_MODEM", false},
	{"PASTHRU", "TT2$M_PASTHRU", false},
	{"PRINTER_PORT", "TT2$M_PRINTER", false},
	{"READSYNC", "TT$M_READSYNC", false},
	{"REGIS", "TT2$M_REGIS", false},
	{"SCOPE", "TT$M_SCOPE", false},
	{"SIXEL_GRAPHICS", "TT2$M_SIXEL", false},
	{"TAB", "TT$M_MECHTAB", false},
	{"TTSYNC", "TT$M_TTSYNC", false},
	{"TYPE_AHEAD", "TT$M_NOTYPEAHD", true},
	{"UPPERCASE", "TT$M_LOWER", true},
	{"WRAP", "TT$M_WRAP", false},
}

// setNotSet is SET's NOTSET condition, "error modifying !AS", a warning
// after SET TERMINAL (unconfirmed: the probe's logs show the message, not
// its value).
var setNotSet = setStatus("", 6, 0)

// SetTerminal is SET TERMINAL: the terminal's characteristics, as opts
// change them. A device that isn't a terminal, or a /PERMANENT change
// without LOG_IO or PHY_IO, is %SET-W-NOTSET with the status on the next
// line, and nothing is changed.
func (c *Context) SetTerminal(opts TerminalOptions) error {
	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	device := opts.Device
	if device == "" {
		device = "SYS$COMMAND"
	}

	s, st := c.RTL.TerminalSettings(device)
	if st != 0 {
		return notSet(strings.ToUpper(strings.TrimSuffix(device, ":"))+":", st)
	}

	if opts.Width != nil {
		if *opts.Width < 1 || *opts.Width > 511 {
			return vmserrors.NewSegment(vmserrors.CLI_IVVALU, "WIDTH")
		}

		s.Width = uint16(*opts.Width)
	}

	if opts.Page != nil {
		if *opts.Page < 0 || *opts.Page > 255 {
			return vmserrors.NewSegment(vmserrors.CLI_IVVALU, "PAGE")
		}

		s.Page = uint8(*opts.Page)
	}

	if opts.DeviceType != "" {
		code, ok := vmsdef.Symbols["TT$_"+strings.ToUpper(opts.DeviceType)]
		if !ok {
			return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, strings.ToUpper(opts.DeviceType))
		}

		s.Type = uint8(code)
	}

	for _, sw := range TerminalSwitches {
		on, given := opts.Switches[sw.Qualifier]
		if !given {
			continue
		}

		bits := &s.Characteristics
		if strings.HasPrefix(sw.Bit, "TT2$") {
			bits = &s.Extended
		}

		if on != sw.Inverted {
			*bits |= vmsdef.Symbols[sw.Bit]
		} else {
			*bits &^= vmsdef.Symbols[sw.Bit]
		}
	}

	if st := c.RTL.SetTerminalSettings(s, opts.Permanent); st != 0 {
		return notSet(s.Device, st)
	}

	return nil
}

// notSet is SET's NOTSET message for device, with status's on the line
// under it, shown as the command's failure.
func notSet(device string, status uint32) error {
	text := "SET-W-NOTSET, error modifying " + device + "\n" + ConditionLine(status)

	return vmserrors.ConditionError{Status: setNotSet, Text: text}
}

// init checks that every switch names a bit $TTDEF has.
func init() {
	for _, sw := range TerminalSwitches {
		if _, ok := vmsdef.Symbols[sw.Bit]; !ok {
			panic("dclcmd: no terminal characteristic " + sw.Bit)
		}
	}
}
