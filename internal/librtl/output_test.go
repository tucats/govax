package librtl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
)

// TestLibPutOutput writes two records, one empty, and checks each is a
// line of the output stream.
func TestLibPutOutput(t *testing.T) {
	out := &bytes.Buffer{}
	logicals := lnm.NewDatabase(corevms.NominalUIC)

	env, err := corevms.NewEnvironment(corevms.NewSystem(registers.New(), vm.NewMemory(1<<20), iodev.NewDeviceTable(),
		rms.NewMountTable()), logicals, bytes.NewReader(nil), out)
	if err != nil {
		t.Fatal(err)
	}

	Register(env.Shims())

	a := newArena(t, env)

	for _, s := range []string{"Hello, world!", ""} {
		if r0 := call(t, env, "LIB$PUT_OUTPUT", a.desc(s)); r0 != ssNormal {
			t.Errorf("LIB$PUT_OUTPUT(%q) = %#x, want SS$_NORMAL", s, r0)
		}
	}

	if got, want := out.String(), "Hello, world!\n\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestLibPutOutput_badDescriptor signals an access violation for a
// descriptor outside memory, writing nothing. (This fixture has no
// SYS$SRCHANDLER stub to signal through, so the signal fails with an
// error; the console's TestLibrtlAccessViolation checks the signal.)
func TestLibPutOutput_badDescriptor(t *testing.T) {
	env := fixture(t)

	if _, err := libPutOutput(env, []uint32{0x7FFFFFF0}); err == nil || !strings.Contains(err.Error(), "signal") {
		t.Errorf("LIB$PUT_OUTPUT: err = %v, want a signal", err)
	}
}
