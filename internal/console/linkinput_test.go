package console

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestLinkOptionsInput runs LINKs whose options come from SYS$INPUT, a
// procedure's data lines, and from a file, with verification on
// (docs/PHASE-68's subtask 3). As in VMS's log (testdata/link68), each
// options line shows as LINK reads it, and a line in error stops LINK
// with OPTSYNERR and the part in error quoted. The data lines LINK didn't
// read are DCL's to skip (SKPDAT), not commands. An option govax's LINK
// doesn't act on yet is refused.
func TestLinkOptionsInput(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()
	obj := assembleFixture(t, c, "entry", dir)
	exe := filepath.Join(dir, "a.exe")
	opt := filepath.Join(dir, "gsm.opt")

	writeHostFile(t, opt, "! GSMATCH= alone\nGSMATCH=NEVER,1,1000\n")

	link := `$ LINK/NOMAP/EXECUTABLE="` + exe + `" "` + obj + `",`
	proc := writeProcedure(t, dir, "link.com",
		"$ SET NOON",
		"$ SET VERIFY",
		link+"SYS$INPUT/OPTIONS",
		"IDENT=V7",
		"GSMATCH=SOMETIMES,1,2",
		"UNIVERSAL=X",
		"$ SHOW SYMBOL $STATUS",
		link+`"`+opt+`"/OPTIONS`,
		"$ SHOW SYMBOL $STATUS",
		link+"SYS$INPUT/OPTIONS",
		"GSMATCH=LEQUAL,1,1000",
		"UNIVERSAL=X",
		"$ LINK_STATUS == $STATUS",
		link+"SYS$INPUT/OPTIONS",
		"BASE=%X10000",
		"$ SET NOVERIFY",
	)

	if err := d.Dispatch("@" + proc); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@link: %v", err)
	}

	link = strings.ReplaceAll(link, dir+"/", "")

	want := strings.Join([]string{
		link + "SYS$INPUT/OPTIONS",
		"IDENT=V7",
		"GSMATCH=SOMETIMES,1,2",
		"%LINK-F-OPTSYNERR, syntax error in options file SYS$INPUT:.;",
		"-LINK-E-OPTLIN, options line in error",
		"\tGSMATCH='SOMETIMES',1,2",
		skpdatLine[:len(skpdatLine)-1],
		"$ SHOW SYMBOL $STATUS",
		`  $STATUS == "%X00038120"`, // SKPDAT's
		link + `"gsm.opt"/OPTIONS`,
		"! GSMATCH= alone",
		"GSMATCH=NEVER,1,1000",
		"%LINK-F-OPTSYNERR, syntax error in options file gsm.opt",
		"-LINK-E-OPTLIN, options line in error",
		"\tGSMATCH='NEVER',1,1000",
		"$ SHOW SYMBOL $STATUS",
		`  $STATUS == "%X106483B4"`,
		link + "SYS$INPUT/OPTIONS",
		"GSMATCH=LEQUAL,1,1000",
		"UNIVERSAL=X",
		"$ LINK_STATUS == $STATUS",
		link + "SYS$INPUT/OPTIONS",
		"BASE=%X10000",
		"%DCL-E-LINKING, Linking entry.obj: the option BASE= isn't supported yet",
		"$ SET NOVERIFY",
		"",
	}, "\n")

	if got := strings.ReplaceAll(buf.String(), dir+"/", ""); got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}

	// The good options linked an image: GSMATCH= and UNIVERSAL= are
	// ignored in an executable, as VMS's LINK ignored UNIVERSAL= (E8).
	v, _ := c.Symbol("LINK_STATUS")
	if n, err := strconv.ParseUint(strings.TrimPrefix(v, "%X"), 16, 32); err != nil || n&1 == 0 {
		t.Errorf("LINK_STATUS %s", v)
	}

	if _, err := os.Stat(exe); err != nil {
		t.Error(err)
	}
}
