package security

import (
	"errors"
	"testing"
)

// TestParseUIC: every form a command's UIC may take, and the faults VMS
// 7.1's SET and CREATE/DIRECTORY refused (Phase 93's probe, "MALFORMED AND
// ODD VALUES"), with a rights database of SYSTEM, PROT93A, and a general
// identifier.
func TestParseUIC(t *testing.T) {
	db := map[string]uint32{"SYSTEM": 0o1<<16 | 0o4, "PROT93A": 0o200<<16 | 0o1, "PROT93_TEAM": 0x80010001}
	lookup := func(name string) (uint32, bool) {
		v, ok := db[name]

		return v, ok
	}

	good := map[string]UIC{
		"[1,4]":              NewUIC(1, 4),
		"<200,1>":            NewUIC(0o200, 1),
		"[ 37776 , 177776 ]": NewUIC(MaxGroup, MaxMember),
		"[SYSTEM]":           NewUIC(1, 4),
		"[system]":           NewUIC(1, 4),
		"SYSTEM":             NewUIC(1, 4),
		"PROT93_TEAM":        0x80010001,
		"[PROT93A,PROT93A]":  NewUIC(0o200, 1),
		"[200,PROT93A]":      NewUIC(0o200, 1),
	}

	for text, want := range good {
		if got, err := ParseUIC(text, lookup); err != nil || got != want {
			t.Errorf("%s: %v, %v; want %v", text, got, err, want)
		}
	}

	bad := map[string]error{
		"[1,99999]":     ErrBadUIC,
		"[400000,1]":    ErrBadUIC,
		"[1,9]":         ErrBadUIC,
		"[1]":           ErrBadUIC,
		"[1,*]":         ErrBadUIC,
		"[PROT93_TEAM]": ErrBadUIC,
		"[NOSUCH]":      ErrNoSuchIdentifier{Name: "NOSUCH"},
		"NOSUCH":        ErrNoSuchIdentifier{Name: "NOSUCH"},
		"[PROT93G2,*]":  ErrNoSuchIdentifier{Name: "PROT93G2"},
		"A-B":           ErrBadUIC,
	}

	for text, want := range bad {
		if _, err := ParseUIC(text, lookup); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", text, err, want)
		}
	}

	if _, err := ParseUIC("[SYSTEM]", nil); !errors.Is(err, ErrNoSuchIdentifier{Name: "SYSTEM"}) {
		t.Errorf("no database: %v", err)
	}
}

// TestParseUICPattern: "*" for a group or member, a wildcard.
func TestParseUICPattern(t *testing.T) {
	for text, want := range map[string]UIC{
		"[200,*]": NewUIC(0o200, WildMember),
		"[*,4]":   NewUIC(WildGroup, 4),
		"[*,*]":   NewUIC(WildGroup, WildMember),
		"[1,4]":   NewUIC(1, 4),
	} {
		if got, err := ParseUICPattern(text, nil); err != nil || got != want {
			t.Errorf("%s: %v, %v; want %v", text, got, err, want)
		}
	}
}
