package lbrrtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// TestTables checks the tables against LBRSHR.EXE's vector: every routine
// is one of its entries, once, and every entry has a stub (or, for a data
// cell, its longword), with codes in this package's range.
func TestTables(t *testing.T) {
	seen := map[string]bool{}

	for _, r := range Routines {
		if seen[r.Name] {
			t.Errorf("%s is listed twice", r.Name)
		}

		seen[r.Name] = true

		if sym, ok := vmsdef.ImageSymbols[r.Name]; !ok || sym.Image != Library {
			t.Errorf("%s isn't in %s's vector", r.Name, Library)
		}
	}

	links := Links()
	if len(links) != 29 {
		t.Errorf("%d links, want LBRSHR's 27 routines and 2 data cells", len(links))
	}

	codes := map[uint32]string{}

	for _, l := range links {
		if l.Data != nil {
			if !dataCells[l.Name] || len(l.Data) != 4 {
				t.Errorf("%s: data %v", l.Name, l.Data)
			}

			continue
		}

		if l.Code < codeBase || l.Code >= 10000 {
			t.Errorf("%s: code %d outside the range", l.Name, l.Code)
		}

		if other, dup := codes[l.Code]; dup {
			t.Errorf("%s and %s share code %d", l.Name, other, l.Code)
		}

		codes[l.Code] = l.Name
	}

	table := corevms.NewShimTable()
	Register(table)

	for _, r := range Routines {
		if table.CallsBack(codeOf(r.Name)) != r.CallsBack {
			t.Errorf("%s: registered with CallsBack %v", r.Name, !r.CallsBack)
		}
	}
}

// TestStatuses checks the statuses the routines return against LBRSHR's
// universal symbols of those names.
func TestStatuses(t *testing.T) {
	for name, v := range map[string]uint32{
		"LBR$_NORMAL": lbrNormal, "LBR$_NULIDX": lbrNulIdx, "LBR$_RECTRUNC": lbrRecTrunc,
		"LBR$_TYPMISMCH": lbrTypMismch, "LBR$_NOMTCHFOU": lbrNoMtchFou, "LBR$_ILLCTL": lbrIllCtl,
		"LBR$_ILLIDXNUM": lbrIllIdxNum, "LBR$_ILLFMT": lbrIllFmt, "LBR$_ILLFUNC": lbrIllFunc,
		"LBR$_INVNAM": lbrInvNam, "LBR$_INVRFA": lbrInvRFA, "LBR$_KEYNOTFND": lbrKeyNotFnd,
		"LBR$_LIBNOTOPN": lbrLibNotOpn, "LBR$_LKPNOTDON": lbrLkpNotDon, "LBR$_LIBOPN": lbrLibOpn,
		"LBR$_NOFILNAM": lbrNoFilNam, "LBR$_NOTHLPLIB": lbrNotHlpLib, "LBR$_TOOMNYLIB": lbrTooMnyLib,
		"LBR$_ILLOUTROU": lbrIllOutRou, "LBR$_ILLINROU": lbrIllInRou, "LBR$_TOOMNYARG": lbrTooMnyArg,
		"LBR$_USRINPERR": lbrUsrInpErr, "LBR$_INVKEY": lbrInvKey,
	} {
		if sym, ok := vmsdef.ImageSymbols[name]; !ok || sym.Value != v {
			t.Errorf("%s = %#x, LBRSHR's is %+v", name, v, sym)
		}
	}

	for name, v := range map[string]uint32{
		"HLP$M_PROMPT": hlpPrompt, "HLP$M_PROCESS": hlpProcess, "HLP$M_GROUP": hlpGroup,
		"HLP$M_SYSTEM": hlpSystem, "HLP$M_LIBLIST": hlpLibList, "HLP$M_HELP": hlpHelp,
		"LBR$C_CREATE": funcCreate, "LBR$C_READ": funcRead, "LBR$C_UPDATE": funcUpdate,
	} {
		if got, ok := vmsdef.Symbols[name]; !ok || got != v {
			t.Errorf("%s = %#x, vmsdef's is %#x", name, v, got)
		}
	}
}
