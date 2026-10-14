package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
)

// fileWalk is what LIB$DELETE_FILE and LIB$RENAME_FILE share
// (docs/PHASE-58, subtasks 5 and 6): reading the file specification and
// its default and related names, the file-scan context, the search, and
// the status rule. Each routine supplies what it does with a file found
// (each) and how it reports an error searching (searchError).
//
// The search runs on a search block (FIND_FILE's layout,
// layoutSearchBlock) below SP in the stub's frame: a $PARSE, then $SEARCH
// until RMS$_NMF, as LIB$FILE_SCAN runs them (filescan.go). Below it is a
// file block, the FAB and NAM of the file found (setFileBlock): the FAB a
// confirm routine gets, which it may $OPEN and $CLOSE without disturbing
// the search NAM's wildcard context. Each file's strings and argument
// lists go below both, and SP goes back above them for the next file.
//
// file-scan-context is LIB$FILE_SCAN's context: the last file the routine
// acted on is the related name of the next call ("need only be specified
// ... with multiple input files"), and LIB$FILE_SCAN_END frees it. A
// related-filespec wins over the context's name, as the program's own
// related NAM wins in LIB$FILE_SCAN (unconfirmed).
type fileWalk struct {
	env *corevms.Environment

	spec, defaultSpec, related string

	// failure is the program's error routine (0 for none), user its
	// user-specified-argument (when hasUser).
	failure uint32
	user    uint32
	hasUser bool

	// search is the search block's address, file the file block's.
	search, file uint32

	// expanded is the search NAM's expanded string after the $PARSE.
	expanded string

	// erred is true once any error has been met, and worst is the most
	// severe status among them.
	erred bool
	worst uint32

	// each acts on the file called name. done is true when it did (the
	// name becomes the context's); stop is true, with the status to
	// return, when the routine ends here.
	each func(name string) (done, stop bool, status uint32, err error)

	// searchError reports a failed $PARSE or $SEARCH, status with its
	// status value stv, about the file called name; stop as for each.
	searchError func(name string, status, stv uint32) (stop bool, _ uint32, err error)
}

// readNames reads the file specification and the default and related
// names from argv's elements i, i+1, and i+2. A name longer than a FAB
// can hold is LIB$_INVFILSPE; err is a memory error, already signaled.
func (w *fileWalk) readNames(argv []uint32, i int) (uint32, error) {
	for _, s := range []struct {
		i   int
		out *string
	}{{i, &w.spec}, {i + 1, &w.defaultSpec}, {i + 2, &w.related}} {
		addr := arg(argv, s.i)
		if addr == 0 && s.i > i {
			continue
		}

		var err error
		if *s.out, err = readString(w.env, addr); err != nil {
			return stringFailure(w.env, err)
		}
	}

	if len(w.spec) > maxFileSpec || len(w.defaultSpec) > maxFileSpec {
		return libInvFilSpe, nil
	}

	return 0, nil
}

// walk runs the search with the file-scan context whose longword is at
// cell (0 for none), making the context on the first call that passes
// one, and returns the routine's status. A context longword that names
// no context is LIB$_INVARG.
func (w *fileWalk) walk(cell uint32) (uint32, error) {
	env := w.env

	var (
		context uint32
		err     error
	)

	if cell != 0 {
		if context, err = env.Memory().LoadLongword(env.CPU(), cell); err != nil {
			return memoryFault(env, err)
		}

		switch {
		case context == 0:
			if status, err := newScanBlock(env, cell, &context); err != nil || status != 0 {
				return status, err
			}
		case !scanContexts(env)[context]:
			return libInvArg, nil
		}

		if w.related == "" {
			if w.related, err = resultantName(env, context+scanNAM); err != nil {
				return memoryFault(env, err)
			}
		}
	}

	// SP goes back where it was when the walk is over; the stub's RET
	// would put it back anyway.
	cpu := env.CPU()
	sp := cpu.GPR(registers.SP)

	status, last, err := w.run()

	cpu.SetGPR(registers.SP, sp)

	if err != nil || last == "" || context == 0 {
		return status, err
	}

	if err := setScanName(env, context, last); err != nil {
		return memoryFault(env, writing(err))
	}

	return status, nil
}

// run is the search: the blocks laid out below SP, the $PARSE, and the
// loop over the files found. It returns the routine's status and the
// last file acted on ("" for none).
func (w *fileWalk) run() (status uint32, last string, err error) {
	env := w.env
	cpu := env.CPU()

	w.file = cpu.GPR(registers.SP) - fileBlockBytes
	w.search = w.file - findBlockBytes
	cpu.SetGPR(registers.SP, w.search)

	if err := layoutSearchBlock(env, w.search); err != nil {
		status, err = memoryFault(env, writing(err))

		return status, "", err
	}

	status, err = startSearch(env, w.search, findArgs{spec: w.spec, defaultSpec: w.defaultSpec}, w.related)
	if err != nil {
		status, err = memoryFault(env, err)

		return status, "", err
	}

	if status&1 == 0 {
		stop, status, err := w.searchFailed(status)
		if err != nil || stop {
			return status, "", err
		}

		return w.status(), "", nil
	}

	if w.expanded, err = w.expandedName(); err != nil {
		status, err = memoryFault(env, err)

		return status, "", err
	}

	base := cpu.GPR(registers.SP)
	nam := w.search + findNAM

	for {
		cpu.SetGPR(registers.SP, base)

		wcc, err := searchPlace(env, nam)
		if err != nil {
			status, err = memoryFault(env, err)

			return status, last, err
		}

		status, err := env.CallService("SYS$SEARCH", []uint32{w.search + findFAB})
		if err != nil {
			return status, last, err
		}

		if status == rmsNMF {
			break
		}

		if status&1 == 0 {
			stop, status, err := w.searchFailed(status)
			if err != nil || stop {
				return status, last, err
			}

			// A failed $SEARCH that can't go on (NAM$L_WCC unchanged)
			// ends the search, as in LIB$FILE_SCAN.
			after, err := searchPlace(env, nam)
			if err != nil {
				status, err = memoryFault(env, err)

				return status, last, err
			}

			if after == wcc {
				break
			}

			continue
		}

		name, err := resultantName(env, nam)
		if err != nil {
			status, err = memoryFault(env, err)

			return status, last, err
		}

		if err := setFileBlock(env, w.file, name); err != nil {
			status, err = memoryFault(env, writing(err))

			return status, last, err
		}

		done, stop, status, err := w.each(name)
		if err != nil || stop {
			return status, last, err
		}

		if done {
			last = name
		}
	}

	return w.status(), last, nil
}

// expandedName is the search NAM's expanded string.
func (w *fileWalk) expandedName() (string, error) {
	env := w.env

	esl, err := env.Memory().LoadByte(env.CPU(), w.search+findNAM+namESL)
	if err != nil {
		return "", err
	}

	return readText(env, w.search+findESA, uint32(esl))
}

// searchFailed reports a failed $PARSE or $SEARCH, status, through
// searchError, with the search FAB's status value and the NAM's name.
func (w *fileWalk) searchFailed(status uint32) (stop bool, _ uint32, err error) {
	env := w.env

	stv, err := env.Memory().LoadLongword(env.CPU(), w.search+findFAB+fabSTV)
	if err != nil {
		status, err = memoryFault(env, err)

		return true, status, err
	}

	name, err := w.searchName()
	if err != nil {
		status, err = memoryFault(env, err)

		return true, status, err
	}

	return w.searchError(name, status, stv)
}

// searchName is the name an error searching is reported under: the
// search NAM's resultant string, else its expanded string, else the file
// specification as given (unconfirmed).
func (w *fileWalk) searchName() (string, error) {
	name, err := resultantName(w.env, w.search+findNAM)
	if err != nil || name != "" {
		return name, err
	}

	if name, err = w.expandedName(); err != nil || name != "" {
		return name, err
	}

	return w.spec, nil
}

// note records an error status, for the routine's final status.
func (w *fileWalk) note(status uint32) {
	if !w.erred || severityRank(status) > severityRank(w.worst) {
		w.worst = status
	}

	w.erred = true
}

// status is the routine's status when it ran to the end: SS$_NORMAL
// with no errors; after errors, LIB$_ERRROUCAL when there is an error
// routine, else the most severe error met (the first of that severity).
func (w *fileWalk) status() uint32 {
	switch {
	case !w.erred:
		return ssNormal
	case w.failure != 0:
		return libErrRouCal
	default:
		return w.worst
	}
}

// nameArgument returns a descriptor of name for a callback: result, the
// program's resultant-name descriptor, with name written into it, or
// else (result 0) a class S descriptor below SP. A string the routine
// can't write ends it with status (string copying errors don't go to the
// error routine).
func (w *fileWalk) nameArgument(result uint32, name string) (desc, status uint32, err error) {
	env := w.env

	if result == 0 {
		if desc, err = stackString(env, name); err != nil {
			status, err = memoryFault(env, writing(err))
		}

		return desc, status, err
	}

	if _, _, err := writeString(env, result, name); err != nil {
		status, err = stringFailure(env, err)

		return 0, status, err
	}

	return result, 0, nil
}

// withUser returns args with the user-specified-argument after them,
// when the program passed one.
func (w *fileWalk) withUser(args ...uint32) []uint32 {
	if w.hasUser {
		return append(args, w.user)
	}

	return args
}

// callError calls the error routine with args (the status values by
// reference, below SP, are the routine's to place). stop is true when
// the routine must end with status: the error routine returned failure.
func (w *fileWalk) callError(status uint32, args ...uint32) (stop bool, _ uint32, err error) {
	r0, err := w.env.Callback(w.failure, args...)
	if err != nil {
		status, err = memoryFault(w.env, err)

		return true, status, err
	}

	if r0&1 == 0 {
		return true, status, nil
	}

	return false, 0, nil
}
