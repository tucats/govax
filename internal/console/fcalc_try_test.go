package console

import (
	"os"
	"strings"
	"testing"
)

// A scratch test (not committed): FCALC_TRY names scripts, FCALC_QUALS the
// FORTRAN qualifiers, FCALC_PAGING paging; each script's output is written
// beside FCALC_OUT.
func TestFcalcTry(t *testing.T) {
	scripts := os.Getenv("FCALC_TRY")
	if scripts == "" {
		t.Skip()
	}

	d, c, buf := fcalcConsole(t, os.Getenv("FCALC_PAGING") != "")
	buildFcalc(t, d, c, buf, os.Getenv("FCALC_QUALS"))

	for _, script := range strings.Fields(scripts) {
		input, err := os.ReadFile(script)
		if err != nil {
			t.Fatal(err)
		}

		c.In = strings.NewReader(string(input))
		got := fcalcTime.ReplaceAllString(shareRun(t, d, buf, "FCALC"), fcalcTimeMask)
		name := os.Getenv("FCALC_OUT") + "/" + strings.TrimSuffix(script[strings.LastIndex(script, "/")+1:], ".in") + ".govax"

		if err := os.WriteFile(name, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	checkFcalcFiles(t, c)
	runFcalcOneShot(t, d, c, buf)
}
