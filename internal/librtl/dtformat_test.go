package librtl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
)

// describe writes a parsed format back out, each mnemonic in brackets,
// so a test can see where the parser put each piece.
func describe(items []dtItem) string {
	var b strings.Builder

	for _, it := range items {
		if it.m != nil {
			b.WriteString("[" + it.m.code + "]")
		} else {
			b.WriteString(it.literal)
		}
	}

	return b.String()
}

// TestParseDateFormat: mnemonics are matched longest first, the rest is
// punctuation, and an unknown mnemonic fails (LIB$_UNRFORCOD).
func TestParseDateFormat(t *testing.T) {
	for _, c := range []struct {
		format, want string
		part         dtPart
	}{
		{"!DB-!MAAU-!Y4", "[DB]-[MAAU]-[Y4]", partDate},
		{"!MAU !DD, !Y4", "[MAU] [DD], [Y4]", partDate},
		{"!MN0/!D0/!Y2", "[MN0]/[D0]/[Y2]", partDate},
		{"/!Y2.!MN0.!D0", "/[Y2].[MN0].[D0]", partDate},
		{"!H04H!M0m", "[H04]H[M0]m", partTime},
		{"!HH4 h !MM min !SS s", "[HH4] h [MM] min [SS] s", partTime},
		{"kl !H04.!M0", "kl [H04].[M0]", partTime},
		{"!HH2:!M0 !MIU", "[HH2]:[M0] [MIU]", partTime},
		{"!WAC, !MNM", "[WAC], [MNM]", partDate},
		{"!H04:!M0:!S0.!C7", "[H04]:[M0]:[S0].[C7]", partTime},
		{"no fields", "no fields", partNone},
		{"!DB-!MAAU-!Y4 !H04:!M0", "[DB]-[MAAU]-[Y4] [H04]:[M0]", partMixed},
	} {
		items, ok := parseDateFormat(c.format)
		if !ok {
			t.Errorf("%q: didn't parse", c.format)

			continue
		}

		if got := describe(items); got != c.want {
			t.Errorf("%q: %s, want %s", c.format, got, c.want)
		}

		if got := partOf(items); got != c.part {
			t.Errorf("%q: part %d, want %d", c.format, got, c.part)
		}
	}

	for _, bad := range []string{"!X", "!D", "!Y5", "!C8", "date !"} {
		if _, ok := parseDateFormat(bad); ok {
			t.Errorf("%q parsed", bad)
		}
	}
}

// TestPredefinedFormatsParse: each of DIGITAL's predefined formats parses
// and gives one part, a date format the date and a time format the time.
func TestPredefinedFormatsParse(t *testing.T) {
	for i, f := range lnm.DateFormats {
		if items, ok := parseDateFormat(f); !ok || slotOf(items) != partDate || partOf(items) != partDate {
			t.Errorf("%s %q doesn't parse as a date", lnm.DateFormatName(i+1), f)
		}
	}

	for i, f := range lnm.TimeFormats {
		if items, ok := parseDateFormat(f); !ok || slotOf(items) != partTime || partOf(items) != partTime {
			t.Errorf("%s %q doesn't parse as a time", lnm.TimeFormatName(i+1), f)
		}
	}
}
