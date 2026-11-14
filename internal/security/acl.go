package security

import (
	"encoding/binary"
	"fmt"

	"github.com/tucats/govax/internal/vmsdef"
)

// ACE is one access control entry, as VMS stores it in a file header's
// ACL area and as $CHKPRO and $DISPLAY's XABPRO hand it over. The layout
// (A.4, "ACE layouts", with $ACEDEF's values):
//
//	byte 0       ACE$B_SIZE     the entry's length in bytes
//	byte 1       ACE$B_TYPE     its type: ACE$C_KEYID (an identifier ACE),
//	                            ACE$C_DIRDEF (default protection), ...
//	word 2       ACE$W_FLAGS    the options in the high byte (ACE$M_DEFAULT,
//	                            PROTECTED, HIDDEN, NOPROPAGATE); the low
//	                            byte is the type's own (an alarm's SUCCESS
//	                            and FAILURE)
//	longword 4   ACE$L_ACCESS   the access the entry gives (ARM$M_ bits)
//	8 ...                       by type: an identifier ACE's identifiers,
//	                            a longword each; a default protection ACE's
//	                            four categories' denied bits; ...
type ACE struct {
	Type        byte
	Flags       uint16
	Access      Access
	Identifiers []uint32 // an identifier ACE's identifiers

	// Data is the whole entry as stored. Decode fills it in for every
	// entry, so one this package doesn't read (an alarm, an audit, an
	// application's) can be written back unchanged.
	Data []byte
}

// The ACE types this package reads.
var (
	TypeIdentifier        = byte(vmsdef.Symbols["ACE$C_KEYID"])
	TypeDefaultProtection = byte(vmsdef.Symbols["ACE$C_DIRDEF"])
	TypeCreator           = byte(vmsdef.Symbols["ACE$C_NEW_OWNER"])
	TypeAudit             = byte(vmsdef.Symbols["ACE$C_AUDIT"])
	TypeAlarm             = byte(vmsdef.Symbols["ACE$C_ALARM"])
)

// The options, in ACE$W_FLAGS.
var (
	OptionDefault     = uint16(vmsdef.Symbols["ACE$M_DEFAULT"])
	OptionProtected   = uint16(vmsdef.Symbols["ACE$M_PROTECTED"])
	OptionHidden      = uint16(vmsdef.Symbols["ACE$M_HIDDEN"])
	OptionNoPropagate = uint16(vmsdef.Symbols["ACE$M_NOPROPAGATE"])
)

// The layout's offsets.
var (
	aceSize   = int(vmsdef.Symbols["ACE$B_SIZE"])
	aceType   = int(vmsdef.Symbols["ACE$B_TYPE"])
	aceFlags  = 2
	aceAccess = int(vmsdef.Symbols["ACE$L_ACCESS"])
	aceKey    = int(vmsdef.Symbols["ACE$L_KEY"])
	aceSysPro = int(vmsdef.Symbols["ACE$L_SYS_PROT"])
	aceHeader = int(vmsdef.Symbols["ACE$C_LENGTH"])
)

// Has reports whether every option in opts is set.
func (a ACE) Has(opts uint16) bool { return a.Flags&opts == opts }

// IsIdentifier reports whether a is an identifier ACE.
func (a ACE) IsIdentifier() bool { return a.Type == TypeIdentifier }

// controlsAccess reports whether a takes part in an access check: an
// identifier ACE without the DEFAULT option. "If an Identifier ACE holds
// the Default attribute, the ACE is ignored in access evaluations" (the
// Guide to System Security, 4.4.1).
func (a ACE) controlsAccess() bool {
	return a.IsIdentifier() && !a.Has(OptionDefault)
}

// DefaultProtection is a default protection ACE's protection code. VMS
// stores each category in a longword of the bits it denies, with
// control's denied too ((S:RWED) is %X10, A.4); the code is their low
// four bits.
func (a ACE) DefaultProtection() Protection {
	var p Protection

	for c := System; c <= World; c++ {
		off := aceSysPro + 4*int(c)
		if off+4 <= len(a.Data) {
			p |= Protection(binary.LittleEndian.Uint32(a.Data[off:])&0xF) << (4 * uint(c))
		}
	}

	return p
}

// NewIdentifierACE makes an identifier ACE giving access to the holder
// of every identifier in ids, with the options in opts.
func NewIdentifierACE(ids []uint32, access Access, opts uint16) ACE {
	data := make([]byte, aceHeader+4*len(ids))
	writeHeader(data, TypeIdentifier, opts, access)

	for i, id := range ids {
		binary.LittleEndian.PutUint32(data[aceKey+4*i:], id)
	}

	return ACE{Type: TypeIdentifier, Flags: opts, Access: access, Identifiers: append([]uint32(nil), ids...), Data: data}
}

// NewDefaultProtectionACE makes a default protection ACE giving new files
// in a directory the protection code p.
func NewDefaultProtectionACE(p Protection, opts uint16) ACE {
	data := make([]byte, aceSysPro+16)
	writeHeader(data, TypeDefaultProtection, opts, 0)

	for c := System; c <= World; c++ {
		denied := uint32(^p.Allows(c)&codeAccess) | uint32(Control)
		binary.LittleEndian.PutUint32(data[aceSysPro+4*int(c):], denied)
	}

	return ACE{Type: TypeDefaultProtection, Flags: opts, Data: data}
}

// writeHeader fills in an ACE's first eight bytes.
func writeHeader(data []byte, typ byte, flags uint16, access Access) {
	data[aceSize] = byte(len(data))
	data[aceType] = typ
	binary.LittleEndian.PutUint16(data[aceFlags:], flags)
	binary.LittleEndian.PutUint32(data[aceAccess:], uint32(access))
}

// DecodeACL reads an ACL, its entries one after another. An entry whose
// length byte is 0 ends the list, as the unused part of a file header's
// ACL area does.
func DecodeACL(raw []byte) ([]ACE, error) {
	var acl []ACE

	for off := 0; off < len(raw); {
		size := int(raw[off])
		if size == 0 {
			break
		}

		if size < aceAccess || off+size > len(raw) {
			return acl, fmt.Errorf("security: ACE at offset %d has a bad length, %d", off, size)
		}

		acl = append(acl, decodeACE(raw[off:off+size]))
		off += size
	}

	return acl, nil
}

// decodeACE reads one entry, whose length is already checked.
func decodeACE(data []byte) ACE {
	a := ACE{
		Type:  data[aceType],
		Flags: binary.LittleEndian.Uint16(data[aceFlags:]),
		Data:  append([]byte(nil), data...),
	}

	if len(data) >= aceHeader {
		a.Access = Access(binary.LittleEndian.Uint32(data[aceAccess:]))
	}

	if a.IsIdentifier() {
		for off := aceKey; off+4 <= len(data); off += 4 {
			a.Identifiers = append(a.Identifiers, binary.LittleEndian.Uint32(data[off:]))
		}
	}

	return a
}

// EncodeACL is the entries one after another, as DecodeACL reads them.
func EncodeACL(acl []ACE) []byte {
	var raw []byte

	for _, a := range acl {
		raw = append(raw, a.Data...)
	}

	return raw
}
