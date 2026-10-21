package lnm

import "strings"

// A process-permanent file is one RMS opened for a process (not for an
// image) and keeps open across image activations: SYS$INPUT, SYS$OUTPUT,
// SYS$ERROR, and SYS$COMMAND, a command procedure's input and /OUTPUT
// file, and the files DCL's OPEN opens (the Guide to OpenVMS File
// Applications, 6.6; docs/PHASE-71 - process-permanent files.md). The
// logical name that leads to one has an equivalence string of a special
// form: ESC, a zero byte, the file's internal file index (IFI) as a VAX
// word (low byte first), then the device's name:
//
//	1B 00 01 81 "_GOVAX$TTA0:"     IFI %X8101, the terminal
//
// The database stores the string as given, all of it: $TRNLNM returns it
// whole, and $CRELNM of a copy makes another name for the same file. What
// shows it to a person (SHOW LOGICAL, SHOW TRANSLATION, F$TRNLNM) shows
// only the device name after the four bytes, and file-specification
// translation stops at it (TranslateFileSpec), as RMS does.

// permanentPrefixLength is the length of a process-permanent file
// string's prefix: ESC, 0, and the IFI word.
const permanentPrefixLength = 4

// escape is the ASCII escape character a process-permanent file string
// begins with.
const escape = 0x1B

// PermanentFile is what a process-permanent file string says: which open
// file (its IFI) on which device.
type PermanentFile struct {
	// IFI is the internal file index of the open file, as RMS numbers
	// its process-permanent files.
	IFI uint16

	// Device is the text after the prefix: the device's physical name,
	// with or without its colon ("_GOVAX$TTA0:", "_GOVAX$DUA1").
	Device string
}

// PermanentFileString returns the equivalence string naming the
// process-permanent file ifi on device.
func PermanentFileString(ifi uint16, device string) string {
	return string([]byte{escape, 0, byte(ifi), byte(ifi >> 8)}) + device
}

// ParsePermanentFile reports whether value is a process-permanent file
// string and, if it is, what it names.
func ParsePermanentFile(value string) (PermanentFile, bool) {
	if len(value) < permanentPrefixLength || value[0] != escape || value[1] != 0 {
		return PermanentFile{}, false
	}

	return PermanentFile{
		IFI:    uint16(value[2]) | uint16(value[3])<<8,
		Device: value[permanentPrefixLength:],
	}, true
}

// DisplayValue returns an equivalence string as it is shown to a person:
// a process-permanent file string's device name, without the prefix, or
// any other string unchanged.
func DisplayValue(value string) string {
	if pf, ok := ParsePermanentFile(value); ok {
		return pf.Device
	}

	return value
}

// deviceSpec returns the device name of a process-permanent file as the
// device part of a file specification: with its colon, which the strings
// DCL's OPEN defines leave out.
func (pf PermanentFile) deviceSpec() string {
	if strings.HasSuffix(pf.Device, ":") {
		return pf.Device
	}

	return pf.Device + ":"
}
