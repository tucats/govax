package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// Every service serviceMinArgs names is a registered service that takes
// an argument list, so a misspelled name can't silently exempt one.
func TestServiceMinArgs_namesRegisteredServices(t *testing.T) {
	env, _ := fixture()

	for name, min := range serviceMinArgs {
		if _, ok := env.services.Lookup(name); !ok {
			t.Errorf("%s has a minimum argument count but is not a registered service", name)
		}

		if !env.services.ReadsArgs(name) || min < 1 {
			t.Errorf("%s: minimum %d, reads arguments %v", name, min, env.services.ReadsArgs(name))
		}
	}
}

// The counts VMS 7.1 was seen to demand: $GETDVIW 8, $CREMBX 7, $ASSIGN 4.
func TestServiceMinArgs_seenOnVMS(t *testing.T) {
	env, _ := fixture()

	for name, min := range map[string]uint32{"SYS$GETDVIW": 8, "SYS$CREMBX": 7, "SYS$ASSIGN": 4} {
		args := make([]uint32, min-1)
		putArgs(t, env, 0x2000, args)

		if r0, _, _ := env.SystemService(p1VectorAddr(name)); r0 != ssInsfArg {
			t.Errorf("%s with %d arguments: R0 %08X, want SS$_INSFARG", name, min-1, r0)
		}
	}
}

// An argument list the caller's mode can't read is SS$_ACCVIO in R0, before
// the service runs, as VMS's change mode dispatcher returns it (the Internals
// book, 9.3.1): the count longword's page, or any page the list runs onto.
// The count is the first longword's low byte.
func TestSystemService_argumentList(t *testing.T) {
	env := newMappedEnvironment(t)
	base, limit := s0TestRange()

	// Every list goes in S0 while the CPU is in kernel mode; the calls are
	// made from user mode, which may read S0 (URKW) but not P0, which has
	// no pages (P0LR 0).
	inS0 := base + 0x2000
	nearEnd := limit - 8 // the count and one argument fit; a second runs past SLR

	putArgs(t, env, inS0, []uint32{1, 2, 3})
	putLongword(t, env, inS0, 0x01000003) // a count of 3, in the low byte
	putArgs(t, env, nearEnd, []uint32{1})
	putLongword(t, env, nearEnd, 4) // ... but the count says four

	cases := []struct {
		name string
		ap   uint32
		want uint32
	}{
		{"a readable list of three ($ASSIGN takes 4)", inS0, ssInsfArg},
		{"a list in P0, past P0LR", 0x200, ssAccVio},
		{"a list running past SLR", nearEnd, ssAccVio},
	}

	psl := env.cpu.PSL()
	psl.SetCurMod(registers.User)
	env.cpu.SetPSL(psl)

	for _, c := range cases {
		env.cpu.SetGPR(registers.AP, c.ap)

		r0, handled, err := env.SystemService(p1VectorAddr("SYS$ASSIGN"))
		if err != nil || !handled || r0 != c.want {
			t.Errorf("%s: R0 %08X, handled %v, err %v; want %08X", c.name, r0, handled, err, c.want)
		}
	}
}

// readArgs takes the count from the first longword's low byte: the other
// three bytes are reserved, and don't add arguments.
func TestReadArgs_countIsLowByte(t *testing.T) {
	env, _ := fixture()
	putArgs(t, env, 0x2000, []uint32{0x11, 0x22})
	putLongword(t, env, 0x2000, 0x00010002)

	argv, err := readArgs(env.cpu, env.mem, 0x2000)
	if err != nil || len(argv) != 2 || argv[0] != 0x11 || argv[1] != 0x22 {
		t.Errorf("readArgs = %v, %v; want [11 22]", argv, err)
	}
}
