package dclshell

import (
	"testing"

	"github.com/tucats/govax/internal/librtl"
)

// TestCLIShimCodeFree: EXE$CLI_COMMAND's XFC$SHIM code is no LIBRTL
// routine's, so registering it doesn't replace one (both share the
// system's shim table).
func TestCLIShimCodeFree(t *testing.T) {
	for _, r := range append(librtl.Routines, librtl.Missing()...) {
		if r.Code == cliShimCode {
			t.Errorf("%s has code %d, EXE$CLI_COMMAND's", r.Name, r.Code)
		}
	}
}
