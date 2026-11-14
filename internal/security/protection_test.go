package security

import (
	"errors"
	"testing"
)

// TestParseProtection replays the probe's SET PROTECTION/DEFAULT forms
// (testdata/prot93/vax/prot93.log, "DEFAULT PROTECTION"), each merged into
// the default the one before left, and what SHOW PROTECTION and
// F$ENVIRONMENT("PROTECTION") showed after it.
func TestParseProtection(t *testing.T) {
	steps := []struct {
		code      string
		show, env string
	}{
		{"(G:RWED,W:R)", "SYSTEM=RWED, OWNER=RWED, GROUP=RWED, WORLD=R", "SYSTEM=RWED, OWNER=RWED, GROUP=RWED, WORLD=R"},
		{"(S:R)", "SYSTEM=R, OWNER=RWED, GROUP=RWED, WORLD=R", "SYSTEM=R, OWNER=RWED, GROUP=RWED, WORLD=R"},
		{"(W)", "SYSTEM=R, OWNER=RWED, GROUP=RWED, WORLD=NO ACCESS", "SYSTEM=R, OWNER=RWED, GROUP=RWED, WORLD"},
		{"(SYSTEM:RWED,OWNER:RWED,GROUP:RE,WORLD)", "SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD=NO ACCESS", "SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD"},
		{"(s:rwe,o:rwe)", "SYSTEM=RWE, OWNER=RWE, GROUP=RE, WORLD=NO ACCESS", "SYSTEM=RWE, OWNER=RWE, GROUP=RE, WORLD"},
		{"(S=RWED,O=RWED,G=RE,W=RE)", "SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD=RE", "SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD=RE"},
		{"(SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD)", "SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD=NO ACCESS", "SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD"},
		{"S:RRR", "SYSTEM=R, OWNER=RWED, GROUP=RE, WORLD=NO ACCESS", "SYSTEM=R, OWNER=RWED, GROUP=RE, WORLD"},
	}

	p := Protection(0xFA00)

	for _, s := range steps {
		var err error
		if p, err = ParseProtection(s.code, p); err != nil {
			t.Fatalf("%s: %v", s.code, err)
		}

		if got := p.Text("NO ACCESS"); got != s.show {
			t.Errorf("%s: SHOW PROTECTION %q, want %q", s.code, got, s.show)
		}

		if got := p.Text(""); got != s.env {
			t.Errorf("%s: F$ENVIRONMENT %q, want %q", s.code, got, s.env)
		}
	}
}

// TestParseProtectionErrors is A.4's malformed codes, each the error its
// command's message is chosen by.
func TestParseProtectionErrors(t *testing.T) {
	var unknown ErrUnknownCategory

	var bad ErrBadAccess

	for _, c := range []struct {
		code  string
		check func(error) bool
	}{
		{"(S:RW,X:R)", func(err error) bool { return errors.As(err, &unknown) && unknown.Name == "X" }},
		{"(SYSTEMS:R)", func(err error) bool { return errors.As(err, &unknown) && unknown.Name == "SYSTEMS" }},
		{"()", func(err error) bool { return errors.Is(err, ErrNoCategories) }},
		{"S:RQ", func(err error) bool { return errors.As(err, &bad) && bad.Item == "S:RQ" }},
		{"S:C", func(err error) bool { return errors.As(err, &bad) }},
		{"(S:R,S:W)", func(err error) bool { return errors.Is(err, ErrRepeatedCategory) }},
	} {
		if _, err := ParseProtection(c.code, 0xFA00); !c.check(err) {
			t.Errorf("%s: %v", c.code, err)
		}
	}
}
