package corevms

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Information items as text (docs/PHASE-55, subtask 3). $GETJPI,
// $GETSYI, and $GETDVI return each item as bytes: a number, a mask, a
// time, a name. Two callers write those bytes as text: DCL's lexical
// functions (F$GETJPI, F$GETSYI, F$GETDVI) and LIBRTL's item routines
// (LIB$GETJPI, LIB$GETSYI, LIB$GETDVI). They mostly agree on how each
// item is written -- a PID as eight hexadecimal digits, a privilege mask
// as a list of names -- and this file is where they share it. Where they
// differ (DCL's numbers are signed, LIB$'s unsigned; F$GETDVI calls a
// device with no ACP ILLEGAL, LIB$GETDVI NONE), the Style says which.

// ItemForm is how an information item's value is written as text.
type ItemForm int

const (
	// FormNumber is a number, written in decimal: signed in DCL's
	// style, unsigned in LIB$'s.
	FormNumber ItemForm = iota

	// FormString is a string item: its bytes are the text.
	FormString

	// FormHex is a longword written as eight hexadecimal digits (a PID).
	FormHex

	// FormPrivileges is a privilege mask, written as the names of its
	// privileges, in bit order, separated by commas (PrivilegeList).
	FormPrivileges

	// FormTime is a quadword VMS time, written as $ASCTIM writes an
	// absolute time.
	FormTime

	// FormUIC is a UIC, written by identifier where the rights database
	// has one ("[SYSTEM]"), as numbers otherwise ("[1,4]").
	FormUIC

	// FormBoolean is TRUE or FALSE, by the value's low bit.
	FormBoolean

	// FormMode is JPI$_MODE's code written as its JPI$K_ name
	// (INTERACTIVE, BATCH, NETWORK, OTHER).
	FormMode

	// FormState is JPI$_STATE's scheduling state code written as SHOW
	// SYSTEM names it (CUR, LEF, HIB, ...).
	FormState

	// FormACP is DVI$_ACPTYPE's code written as its DVI$C_ACP_ name
	// (F11V2, MTA, ...).
	FormACP

	// FormProtection is a protection mask written as
	// "SYSTEM=RWLP,OWNER=RWLP,GROUP=RWLP,WORLD=RWLP" (DVI$_VPROT).
	FormProtection
)

// Style is which caller's conventions an item's text follows.
type Style int

const (
	// StyleDCL is the lexical functions' (the DCL Dictionary's F$GETJPI,
	// F$GETSYI, and F$GETDVI tables).
	StyleDCL Style = iota

	// StyleLIB is LIBRTL's item routines' (the LIB$ manual's Tables
	// LIB-4 and LIB-5).
	StyleLIB
)

// forms is an item's form in each style.
type forms struct{ dcl, lib ItemForm }

// both is an item written the same way in each style.
func both(f ItemForm) forms { return forms{f, f} }

// jpiForms are the $GETJPI items not written as FormNumber, by $JPIDEF
// name. LIB$GETJPI writes in hexadecimal only JPI$_PID (Table LIB-5's
// "all others" are unsigned decimal), where F$GETJPI writes the owner's
// and the master process's PIDs that way too.
var jpiForms = map[string]forms{
	"JPI$_ACCOUNT":    both(FormString),
	"JPI$_CLINAME":    both(FormString),
	"JPI$_PRCNAM":     both(FormString),
	"JPI$_TERMINAL":   both(FormString),
	"JPI$_USERNAME":   both(FormString),
	"JPI$_PID":        both(FormHex),
	"JPI$_MASTER_PID": {FormHex, FormNumber},
	"JPI$_OWNER":      {FormHex, FormNumber},
	"JPI$_AUTHPRIV":   both(FormPrivileges),
	"JPI$_CURPRIV":    both(FormPrivileges),
	"JPI$_IMAGPRIV":   both(FormPrivileges),
	"JPI$_PROCPRIV":   both(FormPrivileges),
	"JPI$_LOGINTIM":   both(FormTime),
	"JPI$_UIC":        both(FormUIC),
	"JPI$_MODE":       both(FormMode),
	"JPI$_STATE":      both(FormState),
}

// syiForms are the $GETSYI items not written as FormNumber, by $SYIDEF
// name. LIB$GETSYI's entry names no special forms: every numeric item is
// an unsigned decimal integer, the boot time's quadword among them
// (unconfirmed, the survey). F$GETSYI writes the boot time as a time,
// CLUSTER_MEMBER as a truth, and the CSID in hexadecimal.
var syiForms = map[string]forms{
	"SYI$_VERSION":        both(FormString),
	"SYI$_NODE_SWVERS":    both(FormString),
	"SYI$_NODE_SWTYPE":    both(FormString),
	"SYI$_NODENAME":       both(FormString),
	"SYI$_BOOTTIME":       {FormTime, FormNumber},
	"SYI$_CLUSTER_MEMBER": {FormBoolean, FormNumber},
	"SYI$_NODE_CSID":      {FormHex, FormNumber},
}

// dviForms are the $GETDVI items not written as FormNumber or
// FormBoolean (dviIsBoolean), by $DVIDEF name: Table LIB-4's, and the
// strings.
var dviForms = map[string]forms{
	"DVI$_DEVNAM":       both(FormString),
	"DVI$_FULLDEVNAM":   both(FormString),
	"DVI$_ALLDEVNAM":    both(FormString),
	"DVI$_ROOTDEVNAM":   both(FormString),
	"DVI$_MEDIA_NAME":   both(FormString),
	"DVI$_MEDIA_TYPE":   both(FormString),
	"DVI$_TT_PHYDEVNAM": both(FormString),
	"DVI$_VOLNAM":       both(FormString),
	"DVI$_PID":          both(FormHex),
	"DVI$_ACPPID":       both(FormHex),
	"DVI$_OWNUIC":       both(FormUIC),
	"DVI$_ACPTYPE":      both(FormACP),
	"DVI$_VPROT":        both(FormProtection),
}

// dviBooleans are the $GETDVI items that are truths but neither a
// DEVCHAR bit nor a terminal characteristic.
var dviBooleans = map[string]bool{
	"DVI$_REMOTE_DEVICE": true, "DVI$_SERVED_DEVICE": true, "DVI$_VOLSETMEM": true,
}

// dviIsBoolean reports whether the $GETDVI item called name is a truth:
// one of dviBooleans, a device characteristic bit ($DEVDEF's DEV$M_
// names: DVI$_MNT, DVI$_DIR, ...), or a terminal characteristic
// (DVI$_TT_ items but the page length and the physical device's name).
func dviIsBoolean(name string) bool {
	if dviBooleans[name] {
		return true
	}

	bit := strings.TrimPrefix(name, "DVI$_")
	if strings.HasPrefix(bit, "TT_") {
		return name != "DVI$_TT_PAGE" && name != "DVI$_TT_PHYDEVNAM"
	}

	_, ok := vmsdef.Symbols["DEV$M_"+bit]

	return ok
}

// pick is f's form in style s.
func (f forms) pick(s Style) ItemForm {
	if s == StyleLIB {
		return f.lib
	}

	return f.dcl
}

// JPIItemForm is how the $GETJPI item called name (its $JPIDEF name,
// "JPI$_PID") is written as text in style s.
func JPIItemForm(name string, s Style) ItemForm { return jpiForms[name].pick(s) }

// SYIItemForm is how the $GETSYI item called name is written in style s.
func SYIItemForm(name string, s Style) ItemForm { return syiForms[name].pick(s) }

// DVIItemForm is how the $GETDVI item called name is written in style s.
func DVIItemForm(name string, s Style) ItemForm {
	if f, ok := dviForms[name]; ok {
		return f.pick(s)
	}

	if dviIsBoolean(name) {
		return FormBoolean
	}

	return FormNumber
}

// ItemText writes an item's value, data (the bytes the service returns),
// in form, by style s's conventions.
func (env *Environment) ItemText(form ItemForm, data string, s Style) string {
	switch form {
	case FormString:
		return data
	case FormHex:
		return fmt.Sprintf("%08X", ItemLongword(data))
	case FormPrivileges:
		return PrivilegeList(ItemQuadword(data))
	case FormTime:
		text, _ := formatVMSTime(ItemQuadword(data), false)

		return text
	case FormUIC:
		return env.IdentifierText(ItemLongword(data))
	case FormBoolean:
		if ItemLongword(data)&1 != 0 {
			return "TRUE"
		}

		return "FALSE"
	case FormMode:
		return codeName("JPI$K_", ItemLongword(data), "INTERACTIVE", "BATCH", "NETWORK", "OTHER")
	case FormState:
		return sched.State(ItemLongword(data)).String()
	case FormACP:
		return acpName(ItemLongword(data), s)
	case FormProtection:
		return ProtectionText(ItemLongword(data))
	}

	if s == StyleLIB {
		return strconv.FormatUint(ItemQuadword(data), 10)
	}

	return strconv.FormatInt(int64(int32(ItemLongword(data))), 10)
}

// codeName is the name, among names, whose prefix+name symbol has the
// value code, or code in decimal if none does.
func codeName(prefix string, code uint32, names ...string) string {
	for _, name := range names {
		if v, ok := vmsdef.Symbols[prefix+name]; ok && v == code {
			return name
		}
	}

	return strconv.FormatUint(uint64(code), 10)
}

// acpNames are the ACP types Table LIB-4 of the 5.0 LIB$ manual names,
// by their DVI$C_ACP_ codes.
var acpNames = []string{"F11V1", "F11V2", "MTA", "NET", "REM", "JNL"}

// acpName is DVI$_ACPTYPE's text: the ACP type's name, and, for a
// device with no ACP (code 0), NONE in LIB$'s style, ILLEGAL in DCL's
// (the DCL Dictionary: "if the device is not mounted"). A code no name
// has is written in decimal.
func acpName(code uint32, s Style) string {
	if code == 0 {
		if s == StyleLIB {
			return "NONE"
		}

		return "ILLEGAL"
	}

	return codeName("DVI$C_ACP_", code, acpNames...)
}

// ProtectionText writes a device's protection mask as Table LIB-4 shows
// DVI$_VPROT: for System, Owner, Group, and World in turn (four bits
// each, from bit 0), the category's name, then "=" and the letters of
// the accesses it has (a clear bit grants one: Read, Write, Logical I/O,
// Physical I/O) if it has any, with commas between categories and no
// blanks: "SYSTEM=RWLP,OWNER=RWLP,GROUP=RWLP,WORLD".
func ProtectionText(mask uint32) string {
	categories := []string{"SYSTEM", "OWNER", "GROUP", "WORLD"}
	parts := make([]string, len(categories))

	for i, name := range categories {
		var letters strings.Builder

		for bit, letter := range "RWLP" {
			if mask>>(4*i+bit)&1 == 0 {
				letters.WriteRune(letter)
			}
		}

		parts[i] = name
		if letters.Len() > 0 {
			parts[i] += "=" + letters.String()
		}
	}

	return strings.Join(parts, ",")
}

// PrivilegeList writes a privilege mask as F$GETJPI and LIB$GETJPI do:
// the names of its privileges, in bit order, separated by commas.
func PrivilegeList(mask uint64) string {
	var names []string

	for bit, name := range PrivilegeNames {
		if name != "" && mask&(1<<bit) != 0 {
			names = append(names, name)
		}
	}

	return strings.Join(names, ",")
}

// ItemLongword and ItemQuadword read an item's value: the low four or
// eight bytes of data, little-endian, short data padded with zeros.
func ItemLongword(data string) uint32 { return uint32(ItemQuadword(data)) }

func ItemQuadword(data string) uint64 {
	var v uint64

	for i := min(len(data), 8) - 1; i >= 0; i-- {
		v = v<<8 | uint64(data[i])
	}

	return v
}
