package securertl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// $PARSE_ACL and $FORMAT_ACL: an ACE between its text and its bytes, by
// internal/security's grammar (acetext.go), with the arguments, buffers,
// and statuses VMS 7.1's had (docs/planning/PHASE-93 - file
// protection.md, A.4's "Round 3").

// Statuses the routines return.
var (
	ssNormal     = vmsdef.Symbols["SS$_NORMAL"]
	ssAccVio     = vmsdef.Symbols["SS$_ACCVIO"]
	ssIvACL      = vmsdef.Symbols["SS$_IVACL"]
	ssNoSuchID   = vmsdef.Symbols["SS$_NOSUCHID"]
	ssIvIdent    = vmsdef.Symbols["SS$_IVIDENT"]
	ssBufferOvf  = vmsdef.Symbols["SS$_BUFFEROVF"]
	ssInsfArg    = vmsdef.Symbols["SS$_INSFARG"]
	ssBadParam   = vmsdef.Symbols["SS$_BADPARAM"]
	ssNoSuchObj  = vmsdef.Symbols["SS$_NOSUCHOBJECT"]
	ssNoPriv     = vmsdef.Symbols["SS$_NOPRIV"]
	ssNoEntry    = vmsdef.Symbols["SS$_NOENTRY"]
	ssNoMoreACE  = vmsdef.Symbols["SS$_NOMOREACE"]
	ssIvBufLen   = vmsdef.Symbols["SS$_IVBUFLEN"]
	ssACLFull    = vmsdef.Symbols["SS$_ACLFULL"]
	ssACLEmpty   = vmsdef.Symbols["SS$_ACLEMPTY"]
	aceHeaderLen = int(vmsdef.Symbols["ACE$C_LENGTH"])
)

// rights is a process's rights database as security.Names: the process's
// own identifier, the environmental ones, and the System's.
type rights struct{ env *corevms.Environment }

func (r rights) IdentifierValue(name string) (uint32, bool) {
	v, status := r.env.IdentifierValue(name)

	return v, status == 0
}

func (r rights) IdentifierName(id uint32) (string, bool) { return r.env.IdentifierName(id) }

// accessNames reads accnam, the address of 32 descriptors naming the
// access bits, bit 0's first; nil for none given.
func accessNames(env *corevms.Environment, addr uint32) (security.AccessNames, error) {
	if addr == 0 {
		return nil, nil
	}

	names := make(security.AccessNames, 32)

	for i := range names {
		s, err := librtl.ReadString(env, addr+8*uint32(i))
		if err != nil {
			return nil, err
		}

		names[i] = s
	}

	return names, nil
}

// readBytes reads n bytes at addr.
func readBytes(env *corevms.Environment, addr uint32, n int) ([]byte, error) {
	b := make([]byte, n)
	if n == 0 {
		return b, nil
	}

	return b, env.Memory().Load(env.CPU(), addr, b)
}

// writeBytes writes b at addr.
func writeBytes(env *corevms.Environment, addr uint32, b []byte) error {
	if len(b) == 0 {
		return nil
	}

	return env.Memory().Store(env.CPU(), addr, b)
}

// storeWord stores v at addr unless addr is 0.
func storeWord(env *corevms.Environment, addr uint32, v uint16) error {
	if addr == 0 {
		return nil
	}

	return env.Memory().StoreWord(env.CPU(), addr, v)
}

// loadWord is the word at addr, or def if addr is 0.
func loadWord(env *corevms.Environment, addr uint32, def int) (int, error) {
	if addr == 0 {
		return def, nil
	}

	w, err := env.Memory().LoadWord(env.CPU(), addr)

	return int(w), err
}

// parseACL is SYS$PARSE_ACL aclstr, aclent [,errpos] [,accnam] [,nullarg]:
// the ACE the text aclstr describes, written in the buffer aclent
// describes, and in the word errpos the offset in the text where reading
// stopped: its length on success, the item that failed otherwise.
//
// VMS 7.1 (A.4) wrote the ACE and cleared the rest of the buffer, by its
// descriptor's length; a buffer shorter than the ACE got what fitted and
// SS$_ACCVIO. A refused text is SS$_IVACL, SS$_NOSUCHID (a name the rights
// database lacks), or SS$_IVIDENT (a UIC out of range), with the buffer
// left alone (unconfirmed: the probe's buffers were empty already).
func parseACL(env *corevms.Environment, argv []uint32) (uint32, error) {
	if arg(argv, 0) == 0 || arg(argv, 1) == 0 {
		return ssAccVio, nil
	}

	text, err := librtl.ReadString(env, arg(argv, 0))
	if err != nil {
		return ssAccVio, nil
	}

	buf, size, err := librtl.Buffer(env, arg(argv, 1))
	if err != nil {
		return ssAccVio, nil
	}

	names, err := accessNames(env, arg(argv, 3))
	if err != nil {
		return ssAccVio, nil
	}

	ace, perr := security.ParseACE(text, rights{env}, names)

	errpos, status := len(text), ssNormal

	var bad *security.ACEError
	if errors.As(perr, &bad) {
		errpos = bad.Pos
		status = map[security.ACEStatus]uint32{
			security.BadACE: ssIvACL, security.NoSuchIdentifier: ssNoSuchID, security.BadIdentifier: ssIvIdent,
		}[bad.Status]
	}

	if storeWord(env, arg(argv, 2), uint16(errpos)) != nil {
		return ssAccVio, nil
	}

	if perr != nil {
		return status, nil
	}

	out := make([]byte, size)
	copy(out, ace)

	if writeBytes(env, buf, out) != nil {
		return ssAccVio, nil
	}

	if int(size) < len(ace) {
		return ssAccVio, nil
	}

	return ssNormal, nil
}

// formatACL is SYS$FORMAT_ACL aclent [,acllen] ,aclstr [,width] [,trmdsc]
// [,indent] [,accnam] [,nullarg] [,nullarg]: the first ACE in the buffer
// aclent describes, as text in the buffer aclstr describes, its length in
// the word acllen.
//
// As VMS 7.1's (A.4): the ACE's size is its first byte, or the
// descriptor's length when that is less than an ACE's header, and the
// descriptor's bytes past it must be whole ACEs, else SS$_IVACL with the
// output buffer filled with blanks and acllen not written; a descriptor
// shorter than the ACE reads zeros past it. With trmdsc and a width the
// text is laid out on lines (security.Wrap), indent blanks before each.
// Text longer than the buffer fills it and is SS$_BUFFEROVF, acllen the
// whole text's length; a buffer of length 0 takes it all; the rest of a
// longer buffer is cleared.
func formatACL(env *corevms.Environment, argv []uint32) (uint32, error) {
	if arg(argv, 0) == 0 || arg(argv, 2) == 0 {
		return ssAccVio, nil
	}

	aceAddr, aceLen, err := librtl.Buffer(env, arg(argv, 0))
	if err != nil {
		return ssAccVio, nil
	}

	raw, err := readBytes(env, aceAddr, int(aceLen))
	if err != nil {
		return ssAccVio, nil
	}

	out, outLen, err := librtl.Buffer(env, arg(argv, 2))
	if err != nil {
		return ssAccVio, nil
	}

	width, err := loadWord(env, arg(argv, 3), 0)
	if err != nil {
		return ssAccVio, nil
	}

	indent, err := loadWord(env, arg(argv, 5), 0)
	if err != nil {
		return ssAccVio, nil
	}

	terminator, wrap := "", arg(argv, 4) != 0
	if wrap {
		if terminator, err = librtl.ReadString(env, arg(argv, 4)); err != nil {
			return ssAccVio, nil
		}
	}

	names, err := accessNames(env, arg(argv, 6))
	if err != nil {
		return ssAccVio, nil
	}

	ace, ok := firstACE(raw)
	if !ok {
		blanks := make([]byte, outLen)
		for i := range blanks {
			blanks[i] = ' '
		}

		if writeBytes(env, out, blanks) != nil {
			return ssAccVio, nil
		}

		return ssIvACL, nil
	}

	text := security.Wrap(security.FormatPieces(ace, rights{env}, names), width, terminator, wrap, indent)

	status, written := ssNormal, []byte(text)

	switch {
	case outLen == 0:
	case len(text) > int(outLen):
		status, written = ssBufferOvf, written[:outLen]
	default:
		written = append(written, make([]byte, int(outLen)-len(text))...)
	}

	if writeBytes(env, out, written) != nil {
		return ssAccVio, nil
	}

	if storeWord(env, arg(argv, 1), uint16(len(text))) != nil {
		return ssAccVio, nil
	}

	return status, nil
}

// firstACE is the ACE $FORMAT_ACL formats from the descriptor's bytes raw,
// at its size with zeros past raw, and whether what follows it is whole
// ACEs.
func firstACE(raw []byte) ([]byte, bool) {
	size := 0
	if len(raw) > 0 {
		size = int(raw[0])
	}

	if size < aceHeaderLen {
		size = len(raw)
	}

	ace := make([]byte, size)
	copy(ace, raw)

	for off := size; off < len(raw); {
		n := int(raw[off])
		if n < aceHeaderLen || off+n > len(raw) {
			return nil, false
		}

		off += n
	}

	return ace, true
}
