package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
)

// defineName defines name as value in table, at mode.
func defineName(t *testing.T, env *corevms.Environment, table, name, value string, mode lnm.Mode) {
	t.Helper()

	if _, err := env.Logicals.Define(table, name, mode, 0, []lnm.Equivalence{{Value: value}}); err != nil {
		t.Fatal(err)
	}
}

// TestLocaleDefaults: with no logical names, the United States symbols,
// and resultant-length the characters written.
func TestLocaleDefaults(t *testing.T) {
	env := fixture(t)

	for _, c := range []struct{ routine, want string }{
		{"LIB$DIGIT_SEP", ","}, {"LIB$RADIX_POINT", "."}, {"LIB$CURRENCY", "$"},
	} {
		putDesc(t, env, 0x1000, 3, dscClassS, 0x3000)
		putLongword(t, env, 0x2000, 0xFFFFFFFF)

		if got := call(t, env, c.routine, 0x1000, 0x2000); got != ssNormal || bytesAt(t, env, 0x3000, 3) != c.want+"  " || longword(t, env, 0x2000) != 0xFFFF0001 {
			t.Errorf("%s: %#x, %q, length longword %#x", c.routine, got, bytesAt(t, env, 0x3000, 3), longword(t, env, 0x2000))
		}
	}
}

// TestLocaleLogicalNames: the process table's name wins over the system
// table's; the translation is returned whole, and cut to a fixed
// destination (LIB$_STRTRU, resultant-length the part written).
func TestLocaleLogicalNames(t *testing.T) {
	env := fixture(t)
	defineName(t, env, lnm.SystemTableName, "SYS$CURRENCY", "EUR", lnm.Executive)

	putDesc(t, env, dynDesc, 0, dscClassD, 0)
	call(t, env, "LIB$CURRENCY", dynDesc)

	if length, _, ptr := descriptorAt(t, env, dynDesc); length != 3 || bytesAt(t, env, ptr, 3) != "EUR" {
		t.Errorf("the system's: length %d, %q", length, bytesAt(t, env, ptr, int(length)))
	}

	defineName(t, env, lnm.ProcessTableName, "SYS$CURRENCY", "#", lnm.Supervisor)
	call(t, env, "LIB$CURRENCY", dynDesc)

	if length, _, ptr := descriptorAt(t, env, dynDesc); length != 1 || bytesAt(t, env, ptr, 1) != "#" {
		t.Errorf("the process's: length %d, %q", length, bytesAt(t, env, ptr, int(length)))
	}

	defineName(t, env, lnm.ProcessTableName, "SYS$RADIX_POINT", "DOT", lnm.Supervisor)
	putDesc(t, env, 0x1000, 2, dscClassS, 0x3000)

	if got := call(t, env, "LIB$RADIX_POINT", 0x1000, 0x2000); got != libStrTru || bytesAt(t, env, 0x3000, 2) != "DO" || longword(t, env, 0x2000)&0xFFFF != 2 {
		t.Errorf("cut: %#x, %q, length %d", got, bytesAt(t, env, 0x3000, 2), longword(t, env, 0x2000)&0xFFFF)
	}
}

// TestLpLines: SYS$LP_LINES in [30, 255], else 66.
func TestLpLines(t *testing.T) {
	env := fixture(t)

	if got := call(t, env, "LIB$LP_LINES"); got != 66 {
		t.Errorf("no name: %d, want 66", got)
	}

	for _, c := range []struct {
		value string
		want  uint32
	}{{"60", 60}, {"30", 30}, {"255", 255}, {"29", 66}, {"256", 66}, {"6O", 66}, {"-60", 66}} {
		defineName(t, env, lnm.ProcessTableName, "SYS$LP_LINES", c.value, lnm.Supervisor)

		if got := call(t, env, "LIB$LP_LINES"); got != c.want {
			t.Errorf("SYS$LP_LINES %q: %d, want %d", c.value, got, c.want)
		}
	}
}
