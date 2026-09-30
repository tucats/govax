package vmserrors

import (
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestSysCodes_matchSSDEF confirms each composite SS_* constant equals the
// real VMS 7.3 $SSDEF value (internal/vmsdef.Symbols, generated from
// reference/vms/ssdef.txt), since a VAX program compares a service's
// return value against those exact numbers.
func TestSysCodes_matchSSDEF(t *testing.T) {
	codes := map[string]uint32{
		"SS$_ACCVIO":      SS_ACCVIO,
		"SS$_BADPARAM":    SS_BADPARAM,
		"SS$_DEVMOUNT":    SS_DEVMOUNT,
		"SS$_DEVNOTMOUNT": SS_DEVNOTMOUNT,
		"SS$_NOSUCHFILE":  SS_NOSUCHFILE,
		"SS$_NOPRIV":      SS_NOPRIV,
		"SS$_DUPLNAM":     SS_DUPLNAM,
		"SS$_IVLOGNAM":    SS_IVLOGNAM,
		"SS$_IVLOGTAB":    SS_IVLOGTAB,
		"SS$_NOLOGNAM":    SS_NOLOGNAM,
		"SS$_TOOMANYLNAM": SS_TOOMANYLNAM,
		"SS$_SUPERSEDE":   SS_SUPERSEDE,
		"SS$_LNMCREATED":  SS_LNMCREATED,
		"SS$_PARENT_DEL":  SS_PARENT_DEL,
		"SS$_NOLOGTAB":    SS_NOLOGTAB,
	}

	for name, got := range codes {
		want, ok := vmsdef.Symbols[name]
		if !ok {
			t.Errorf("%s not in vmsdef.Symbols", name)

			continue
		}

		if got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

func TestSysCodes_logicalNameMessages(t *testing.T) {
	cases := map[uint32]string{
		SS_NOLOGNAM:    "SYSTEM-F-NOLOGNAM, no logical name match",
		SS_NOLOGTAB:    "SYSTEM-F-NOLOGTAB, no logical name table name match",
		SS_TOOMANYLNAM: "SYSTEM-F-TOOMANYLNAM, logical name translation exceeded allowed depth",
		SS_SUPERSEDE:   "SYSTEM-S-SUPERSEDE, logical name superseded",
	}

	for code, want := range cases {
		if got := New(code).Error(); got != want {
			t.Errorf("New(%d).Error() = %q, want %q", code, got, want)
		}
	}
}
