package corevms

import "github.com/tucats/govax/internal/vmserrors"


const cliGetSymbol = 0x1305

// cliUndefinedSymbol is CLI$_UNDSYM.
const cliUndefinedSymbol = 0x38140

// ErrHalt is returned by a ServiceFunc/ShimFunc to request that the machine
// halt. Kept as a package-local sentinel rather than internal/cpu's
// ErrHalted so this package doesn't need to import internal/cpu — whatever
// wires an Environment into internal/cpu.SystemServices (internal/console)
// translates this into cpu.ErrHalted.
var ErrHalt = vmserrors.New(vmserrors.LIB_HALT)

// serviceSysCli is SYS$CLI: dispatches one fixed-format callback request.
// Every request other than "get symbol" is unimplemented in the C source
// itself (a single default case lists over a dozen request codes as
// comments, none actually handled), replicated as-is: an unrecognized
// request halts the machine.
func serviceSysCli(env *Environment, argv []uint32) (uint32, error) {
	reqAddr := argv[0]

	request, err := env.mem.LoadByte(env.cpu, reqAddr)
	if err != nil {
		return ssAccVio, nil
	}

	subrequest, err := env.mem.LoadByte(env.cpu, reqAddr+1)
	if err != nil {
		return ssAccVio, nil
	}

	length, err := env.mem.LoadLongword(env.cpu, reqAddr+4)
	if err != nil {
		return ssAccVio, nil
	}

	ptr, err := env.mem.LoadLongword(env.cpu, reqAddr+8)
	if err != nil {
		return ssAccVio, nil
	}

	reqword := uint32(subrequest)<<8 | uint32(request)

	switch reqword {
	case cliGetSymbol:
		if _, err := loadString(env, ptr, int(length)); err != nil {
			return ssAccVio, nil
		}

		return cliUndefinedSymbol, nil

	default:
		return ssInvArg, ErrHalt
	}
}

func registerCLIService(t *ServiceTable) {
	t.Register("SYS$CLI", serviceSysCli)
}
