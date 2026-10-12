package librtl

import "github.com/tucats/govax/internal/corevms"

// maxSpawnArguments is how many arguments LIB$SPAWN takes.
const maxSpawnArguments = 13

// libSpawn is LIB$SPAWN (RTL Library manual, docs/PHASE-48.md):
//
//	LIB$SPAWN [command-string] [,input-file] [,output-file] [,flags]
//	          [,process-name] [,process-id] [,completion-status-address]
//	          [,byte-integer-event-flag-num] [,AST-address]
//	          [,varying-AST-argument] [,prompt-string] [,cli]
//	          [,table-name]
//
// It creates a subprocess running the command interpreter, which runs
// command-string (or reads SYS$INPUT) and logs out. Every argument is
// optional: the strings are by descriptor, flags and the event flag
// number by reference (a longword and a byte), process-id and
// completion-status-address are longwords it writes (the PID now, the
// subprocess's final status when it ends), AST-address is the AST
// routine and varying-AST-argument its argument, by value. The cli and
// table-name arguments are read for their access only; govax has one
// CLI (corevms.Spawn has the rest).
//
// Unless flags has CLI$M_NOWAIT, LIB$SPAWN returns only once the
// subprocess has ended: the call waits, and is made again when it can
// go on (corevms.AwaitSpawn).
//
// It returns SS$_NORMAL; LIB$_WRONUMARG for more than 13 arguments;
// LIB$_INVSTRDES for a string whose descriptor's class isn't a string's;
// or corevms.Spawn's failures (LIB$_NOCLI, LIB$_INVARG for a flag it
// doesn't define, $CREPRC's statuses). An argument it can't read or write
// is an access violation, signaled (memoryFault).
func libSpawn(env *corevms.Environment, argv []uint32) (uint32, error) {
	// A call made again while it waits for its subprocess.
	if waiting, err := env.AwaitSpawn(); waiting {
		if err != nil {
			return 0, err
		}

		return ssNormal, nil
	}

	if len(argv) > maxSpawnArguments {
		return libWroNumArg, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	var req corevms.SpawnRequest

	strs := []struct {
		index int
		to    *string
	}{
		{0, &req.Command}, {1, &req.Input}, {2, &req.Output}, {4, &req.Name}, {10, &req.Prompt},
		{11, nil}, {12, nil},
	}

	for _, s := range strs {
		addr := arg(argv, s.index)
		if addr == 0 {
			continue
		}

		text, err := readString(env, addr)
		if err != nil {
			return stringFailure(env, err)
		}

		if s.to != nil {
			*s.to = text
		}
	}

	if a := arg(argv, 3); a != 0 {
		flags, err := mem.LoadLongword(cpu, a)
		if err != nil {
			return memoryFault(env, err)
		}

		req.Flags = flags
	}

	if a := arg(argv, 7); a != 0 {
		efn, err := mem.LoadByte(cpu, a)
		if err != nil {
			return memoryFault(env, err)
		}

		req.EventFlag, req.HasEventFlag = uint32(efn), true
	}

	req.StatusAddress = arg(argv, 6)
	req.AST, req.ASTParameter = arg(argv, 8), arg(argv, 9)

	// The two output longwords must be writable before anything is made.
	pidAddr := arg(argv, 5)

	for _, a := range []uint32{pidAddr, req.StatusAddress} {
		if a == 0 {
			continue
		}

		v, err := mem.LoadLongword(cpu, a)
		if err != nil {
			return memoryFault(env, err)
		}

		if err := mem.StoreLongword(cpu, a, v); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	child, st := env.Spawn(req)
	if st != ssNormal {
		return st, nil
	}

	if pidAddr != 0 {
		_ = mem.StoreLongword(cpu, pidAddr, child.Process.PID)
	}

	// Unless CLI$M_NOWAIT, wait for the subprocess now.
	if _, err := env.AwaitSpawn(); err != nil {
		return 0, err
	}

	return ssNormal, nil
}
