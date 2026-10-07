package librtl

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// TestLibPutOutput writes two records, one empty, and checks each is a
// line of the output stream.
func TestLibPutOutput(t *testing.T) {
	out := &bytes.Buffer{}
	logicals := lnm.NewDatabase(corevms.NominalUIC)

	env := corevms.NewEnvironment(corevms.NewSystem(vax.New(), vm.NewMemory(1<<20), iodev.NewDeviceTable(),
		rms.NewMountTable()), logicals, bytes.NewReader(nil), out)
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

// TestLibPutOutput_badDescriptor returns SS$_ACCVIO for a descriptor
// outside memory, writing nothing.
func TestLibPutOutput_badDescriptor(t *testing.T) {
	env := fixture(t)

	if r0 := call(t, env, "LIB$PUT_OUTPUT", 0x7FFFFFF0); r0 != ssAccVio {
		t.Errorf("LIB$PUT_OUTPUT = %#x, want SS$_ACCVIO", r0)
	}
}
