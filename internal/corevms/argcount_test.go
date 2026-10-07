package corevms

import "testing"

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
