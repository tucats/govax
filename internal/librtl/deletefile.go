package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// LIB$DELETE_FILE (docs/PHASE-58, subtask 5; the survey's "DELETE_FILE
// (106)"), written from the 5.0 LIB$ manual (LIB-106), with the VSI
// manual's tenth argument:
//
//	LIB$DELETE_FILE filespec [,default-filespec] [,related-filespec]
//	                [,user-success-procedure] [,user-error-procedure]
//	                [,user-confirm-procedure] [,user-specified-argument]
//	                [,resultant-name] [,file-scan-context] [,flags]
//
// It deletes every file filespec matches, as DCL's DELETE does: a
// $PARSE and then $SEARCH until RMS$_NMF, as LIB$FILE_SCAN runs them
// (filescan.go), and for each file found
//
//  1. the confirm routine, when there is one, is called with the file's
//     name and a FAB that describes the file; a failure (bit 0 of R0
//     clear) passes the file over;
//  2. the file is deleted ($ERASE of that FAB);
//  3. the success routine is called with the file's name.
//
// An error searching (error-source 0) or deleting (1) calls the error
// routine with the file's name, the RMS status and status value, and the
// error source, each of the three by reference. If it returns success
// the routine goes on with the next file; if failure, it stops at once.
// Each routine is given user-specified-argument last, the longword as
// the routine's own argument list has it, when the program passed one.
//
// The names go to the callbacks by descriptor: resultant-name, the
// name written into it first, or else a class S descriptor in the
// stub's frame. The FAB and NAM the search runs on are in the stub's
// frame too (below SP, as the callbacks' argument lists are), and so is
// a second FAB and NAM, the file's: its file name is the file found,
// and it is the FAB the confirm routine gets, to $OPEN (and $CLOSE) if
// it wants the file's attributes, and the one $ERASE deletes. A FAB of
// its own keeps a confirm routine's $OPEN from disturbing the search's
// NAM (whose wildcard context an $OPEN would clear).
//
// file-scan-context is LIB$FILE_SCAN's context: the last file deleted
// is the related name of the next call ("need only be specified ... with
// multiple input files, as the DCL command DELETE does"), and
// LIB$FILE_SCAN_END frees it.
//
// The choices the manuals leave open are the survey's probe cases, each
// unconfirmed until the probe (subtask 8) runs on VMS:
//
//   - No arguments, or more than ten, is LIB$_WRONUMARG. A flag bit
//     other than the VSI manual's three (two reserved, LONG_NAMES) is
//     LIB$_INVARG; those three change nothing on the VAX.
//   - A name without a version deletes the highest version, $SEARCH's
//     and $ERASE's rule (the manual's Pascal example passes "*;" as the
//     default name for DCL's rule).
//   - The routine's status is SS$_NORMAL with no errors. After errors,
//     when there is an error routine, LIB$_ERRROUCAL, but the RMS status
//     itself when the error routine stopped the deletion; with none, the
//     most severe error status met (the first of the most severe).
//   - The name the error routine gets is the NAM's resultant string,
//     else its expanded string, else filespec as given.
//   - A related-filespec wins over the context's last file, as the
//     program's own related NAM wins in LIB$FILE_SCAN.
//   - A failed $SEARCH that can't go on (NAM$L_WCC unchanged) ends the
//     search, as in LIB$FILE_SCAN.
//   - What the success and confirm routines' argument lists hold when the
//     program gave no user-specified-argument: just the name (and the
//     FAB).

// Statuses and fields LIB$DELETE_FILE uses beyond the search routines'.
var (
	libErrRouCal = vmsdef.LibrarySymbols["LIB$_ERRROUCAL"]
	namESL       = vmsdef.Symbols["NAM$B_ESL"]
)

// deleteFlags are the flag bits LIB$DELETE_FILE accepts: the VSI
// manual's two reserved bits and LIB$M_FIL_LONG_NAMES.
const deleteFlags = 1<<0 | 1<<1 | 1<<2

// The error sources the error routine is given.
const (
	sourceSearch = 0 // an error searching for the file
	sourceDelete = 1 // an error deleting it
)

// A file block's layout, in the stub's frame: the FAB and NAM of the
// file found, its expanded and resultant buffers, and its name.
const (
	fileFAB        = 0
	fileNAM        = fileFAB + 80
	fileESA        = fileNAM + 96
	fileRSA        = fileESA + fileBuffer
	fileName       = fileRSA + fileBuffer
	fileBlockBytes = fileName + fileBuffer
)

// deleter is one LIB$DELETE_FILE call's state.
type deleter struct {
	env *corevms.Environment

	spec, defaultSpec, related string

	// success, failure, and confirm are the program's routines (0 for
	// none), user its argument (when hasUser), result its resultant-name
	// descriptor (0 for none).
	success, failure, confirm uint32
	user                      uint32
	hasUser                   bool
	result                    uint32

	// search is the search block's address, file the file block's.
	search, file uint32

	// erred is true once any error has been met, and worst is the most
	// severe status among them.
	erred bool
	worst uint32
}

// libDeleteFile is LIB$DELETE_FILE.
func libDeleteFile(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) == 0 || len(argv) > 10 {
		return libWroNumArg, nil
	}

	d := &deleter{
		env:     env,
		success: arg(argv, 3), failure: arg(argv, 4), confirm: arg(argv, 5),
		user: arg(argv, 6), hasUser: len(argv) > 6,
		result: arg(argv, 7),
	}

	if status, err := d.readNames(argv); err != nil || status != 0 {
		return status, err
	}

	flags, err := optionalLongword(env, arg(argv, 9), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	if flags&^deleteFlags != 0 {
		return libInvArg, nil
	}

	// The context, made on the first call that passes one.
	var context uint32

	if cell := arg(argv, 8); cell != 0 {
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

		if d.related == "" {
			if d.related, err = resultantName(env, context+scanNAM); err != nil {
				return memoryFault(env, err)
			}
		}
	}

	// SP goes back where it was when the deletion is over; the stub's RET
	// would put it back anyway.
	cpu := env.CPU()
	sp := cpu.GPR(registers.SP)

	status, last, err := d.run()

	cpu.SetGPR(registers.SP, sp)

	if err != nil || last == "" || context == 0 {
		return status, err
	}

	if err := setScanName(env, context, last); err != nil {
		return memoryFault(env, writing(err))
	}

	return status, nil
}

// readNames reads filespec, default-filespec, and related-filespec. A
// name longer than a FAB can hold is LIB$_INVFILSPE; err is a memory
// error, already signaled.
func (d *deleter) readNames(argv []uint32) (uint32, error) {
	for _, s := range []struct {
		i   int
		out *string
	}{{0, &d.spec}, {1, &d.defaultSpec}, {2, &d.related}} {
		addr := arg(argv, s.i)
		if addr == 0 && s.i > 0 {
			continue
		}

		var err error
		if *s.out, err = readString(d.env, addr); err != nil {
			return stringFailure(d.env, err)
		}
	}

	if len(d.spec) > maxFileSpec || len(d.defaultSpec) > maxFileSpec {
		return libInvFilSpe, nil
	}

	return 0, nil
}

// run is the deletion: the blocks laid out below SP, the search, and the
// loop over the files found. It returns the routine's status and the
// last file deleted ("" for none).
func (d *deleter) run() (status uint32, last string, err error) {
	env := d.env
	cpu := env.CPU()

	d.file = cpu.GPR(registers.SP) - fileBlockBytes
	d.search = d.file - findBlockBytes
	cpu.SetGPR(registers.SP, d.search)

	if err := layoutSearchBlock(env, d.search); err != nil {
		status, err = memoryFault(env, writing(err))

		return status, "", err
	}

	status, err = startSearch(env, d.search, findArgs{spec: d.spec, defaultSpec: d.defaultSpec}, d.related)
	if err != nil {
		status, err = memoryFault(env, err)

		return status, "", err
	}

	if status&1 == 0 {
		stop, status, err := d.searchFailed(status)
		if err != nil || stop {
			return status, "", err
		}

		return d.status(), "", nil
	}

	// Each file's strings and argument lists go below base, and SP goes
	// back to it for the next file.
	base := cpu.GPR(registers.SP)
	nam := d.search + findNAM

	for {
		cpu.SetGPR(registers.SP, base)

		wcc, err := searchPlace(env, nam)
		if err != nil {
			status, err = memoryFault(env, err)

			return status, last, err
		}

		status, err := env.CallService("SYS$SEARCH", []uint32{d.search + findFAB})
		if err != nil {
			return status, last, err
		}

		if status == rmsNMF {
			break
		}

		if status&1 == 0 {
			stop, status, err := d.searchFailed(status)
			if err != nil || stop {
				return status, last, err
			}

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

		deleted, stop, status, err := d.deleteOne(name)
		if err != nil || stop {
			return status, last, err
		}

		if deleted {
			last = name
		}
	}

	return d.status(), last, nil
}

// deleteOne asks the confirm routine about the file called name, deletes
// it, and tells the success routine, or the error routine when the
// delete fails. deleted is true when the file went; stop is true, with
// the status to return, when the routine ends here (the error routine
// said so, or a string couldn't be written).
func (d *deleter) deleteOne(name string) (deleted, stop bool, status uint32, err error) {
	env := d.env

	if err := setFileBlock(env, d.file, name); err != nil {
		status, err = memoryFault(env, writing(err))

		return false, true, status, err
	}

	desc, status, err := d.nameArgument(name)
	if err != nil || status != 0 {
		return false, true, status, err
	}

	fab := d.file + fileFAB

	if d.confirm != 0 {
		r0, err := env.Callback(d.confirm, d.withUser(desc, fab)...)
		if err != nil {
			status, err = memoryFault(env, err)

			return false, true, status, err
		}

		if r0&1 == 0 {
			return false, false, 0, nil
		}
	}

	status, err = env.CallService("SYS$ERASE", []uint32{fab})
	if err != nil {
		return false, true, status, err
	}

	if status&1 == 0 {
		stv, err := env.Memory().LoadLongword(env.CPU(), fab+fabSTV)
		if err != nil {
			status, err = memoryFault(env, err)

			return false, true, status, err
		}

		stop, status, err := d.reportError(name, status, stv, sourceDelete)

		return false, stop, status, err
	}

	if d.success != 0 {
		if _, err := env.Callback(d.success, d.withUser(desc)...); err != nil {
			status, err = memoryFault(env, err)

			return true, true, status, err
		}
	}

	return true, false, 0, nil
}

// searchFailed reports a failed $PARSE or $SEARCH, status, to the error
// routine, with the search FAB's status value and the NAM's name.
func (d *deleter) searchFailed(status uint32) (stop bool, _ uint32, err error) {
	env := d.env
	mem, cpu := env.Memory(), env.CPU()

	stv, err := mem.LoadLongword(cpu, d.search+findFAB+fabSTV)
	if err != nil {
		status, err = memoryFault(env, err)

		return true, status, err
	}

	name, err := d.searchName()
	if err != nil {
		status, err = memoryFault(env, err)

		return true, status, err
	}

	return d.reportError(name, status, stv, sourceSearch)
}

// searchName is the name an error searching is reported under: the
// search NAM's resultant string, else its expanded string, else filespec
// as given.
func (d *deleter) searchName() (string, error) {
	env := d.env
	nam := d.search + findNAM

	name, err := resultantName(env, nam)
	if err != nil || name != "" {
		return name, err
	}

	esl, err := env.Memory().LoadByte(env.CPU(), nam+namESL)
	if err != nil {
		return "", err
	}

	if esl != 0 {
		return readText(env, d.search+findESA, uint32(esl))
	}

	return d.spec, nil
}

// reportError notes an error, status (with its status value stv, from
// source), and calls the error routine, if there is one, about the file
// called name. stop is true when the routine must end with the status
// returned: the error routine returned failure, or a string couldn't be
// written.
func (d *deleter) reportError(name string, status, stv, source uint32) (stop bool, _ uint32, err error) {
	if !d.erred || severityRank(status) > severityRank(d.worst) {
		d.worst = status
	}

	d.erred = true

	if d.failure == 0 {
		return false, 0, nil
	}

	env := d.env

	desc, st, err := d.nameArgument(name)
	if err != nil || st != 0 {
		return true, st, err
	}

	values, err := stackLongwords(env, status, stv, source)
	if err != nil {
		st, err = memoryFault(env, writing(err))

		return true, st, err
	}

	r0, err := env.Callback(d.failure, d.withUser(desc, values, values+4, values+8)...)
	if err != nil {
		st, err = memoryFault(env, err)

		return true, st, err
	}

	if r0&1 == 0 {
		return true, status, nil
	}

	return false, 0, nil
}

// status is the routine's status when it ran to the end: SS$_NORMAL
// with no errors; after errors, LIB$_ERRROUCAL when there is an error
// routine, else the most severe error.
func (d *deleter) status() uint32 {
	switch {
	case !d.erred:
		return ssNormal
	case d.failure != 0:
		return libErrRouCal
	default:
		return d.worst
	}
}

// nameArgument returns a descriptor of name for a callback: the
// program's resultant-name, with name written into it, or else a class S
// descriptor below SP. A string the routine can't write ends it with
// status (string copying errors don't go to the error routine).
func (d *deleter) nameArgument(name string) (desc, status uint32, err error) {
	env := d.env

	if d.result == 0 {
		if desc, err = stackString(env, name); err != nil {
			status, err = memoryFault(env, writing(err))
		}

		return desc, status, err
	}

	if _, _, err := writeString(env, d.result, name); err != nil {
		status, err = stringFailure(env, err)

		return 0, status, err
	}

	return d.result, 0, nil
}

// withUser returns args with the user-specified-argument after them,
// when the program passed one.
func (d *deleter) withUser(args ...uint32) []uint32 {
	if d.hasUser {
		return append(args, d.user)
	}

	return args
}

// setFileBlock clears a file block and sets its FAB and NAM up as the
// $FAB and $NAM macros would, the FAB's file name name (at most a file
// specification's length).
func setFileBlock(env *corevms.Environment, block uint32, name string) error {
	mem, cpu := env.Memory(), env.CPU()

	if len(name) > maxFileSpec {
		name = name[:maxFileSpec]
	}

	for off := uint32(0); off < fileBlockBytes; off += 4 {
		if err := mem.StoreLongword(cpu, block+off, 0); err != nil {
			return err
		}
	}

	for _, b := range []struct{ addr, v uint32 }{
		{block + fileFAB + fabBID, fabBIDValue},
		{block + fileFAB + fabBLN, fabBLNValue},
		{block + fileFAB + fabFNS, uint32(len(name))},
		{block + fileNAM + namBID, namBIDValue},
		{block + fileNAM + namBLN, namBLNValue},
		{block + fileNAM + namESS, maxFileSpec},
		{block + fileNAM + namRSS, maxFileSpec},
	} {
		if err := mem.StoreByte(cpu, b.addr, byte(b.v)); err != nil {
			return err
		}
	}

	for _, l := range []struct{ addr, v uint32 }{
		{block + fileFAB + fabNAM, block + fileNAM},
		{block + fileFAB + fabFNA, block + fileName},
		{block + fileNAM + namESA, block + fileESA},
		{block + fileNAM + namRSA, block + fileRSA},
	} {
		if err := mem.StoreLongword(cpu, l.addr, l.v); err != nil {
			return err
		}
	}

	for i := range len(name) {
		if err := mem.StoreByte(cpu, block+fileName+uint32(i), name[i]); err != nil {
			return err
		}
	}

	return nil
}

// stackLongwords puts values on the stack below SP, moves SP below them,
// and returns the first one's address: arguments passed by reference to
// a callback. As with stackString, the space is the stub's frame's.
func stackLongwords(env *corevms.Environment, values ...uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	addr := cpu.GPR(registers.SP) - 4*uint32(len(values))

	for i, v := range values {
		if err := mem.StoreLongword(cpu, addr+4*uint32(i), v); err != nil {
			return 0, err
		}
	}

	cpu.SetGPR(registers.SP, addr)

	return addr, nil
}

// severityRank orders condition values by severity, for "the most severe
// error status": severe, then error, then warning, then the successes.
func severityRank(status uint32) int {
	switch status & 7 {
	case 4: // severe
		return 3
	case 2: // error
		return 2
	case 0: // warning
		return 1
	default: // success, informational
		return 0
	}
}
