package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// LIB$DELETE_FILE (docs/PHASE-58, subtask 5; the survey's "DELETE_FILE
// (106)"), written from the 5.0 LIB$ manual (LIB-106):
//
//	LIB$DELETE_FILE filespec [,default-filespec] [,related-filespec]
//	                [,user-success-procedure] [,user-error-procedure]
//	                [,user-confirm-procedure] [,user-specified-argument]
//	                [,resultant-name] [,file-scan-context]
//
// It deletes every file filespec matches, as DCL's DELETE does: a
// $PARSE and then $SEARCH until RMS$_NMF, as LIB$FILE_SCAN runs them
// (filescan.go), and for each file found
//
//  1. the confirm routine, when there is one, is called with the file's
//     name and the search's FAB, which describes the file (FAB$V_NAM is
//     set, so an $OPEN of it opens the file its NAM names); a failure
//     (bit 0 of R0 clear) passes the file over;
//  2. the file is deleted ($ERASE);
//  3. the success routine is called with the file's name.
//
// An error searching (error-source 0) or deleting (1) calls the error
// routine with the file's name, the RMS status and status value, and the
// error source, each of the three by reference. If it returns success
// the routine goes on with the next file; if failure, it stops at once.
// Each routine is given user-specified-argument last, the longword as
// the routine's own argument list has it, or 0.
//
// The names go to the callbacks by descriptor: resultant-name, the
// name written into it first, or else a class S descriptor in the
// stub's frame. The FAB and NAM the search runs on are in the stub's
// frame too (below SP, as the callbacks' argument lists are), and so is
// a second FAB, whose file name is the file found, which $ERASE deletes.
//
// file-scan-context is LIB$FILE_SCAN's context: the last file deleted
// is the related name of the next call ("need only be specified ... with
// multiple input files, as the DCL command DELETE does"), and
// LIB$FILE_SCAN_END frees it.
//
// What the manuals leave open is as VMS 7.1 does it (testdata/lib58,
// docs/PHASE-58's subtask 8), but where marked unconfirmed:
//
//   - No arguments, or more than nine, is LIB$_WRONUMARG (the VSI
//     manual's tenth argument, flags, isn't VMS 7.1's).
//   - With no default-filespec, a filespec with a wildcard is parsed
//     with ";*" as its default (the search FAB's DNA); one without
//     deletes the highest version (unconfirmed which rule VMS uses).
//   - The routine's status is SS$_NORMAL with no errors. After errors,
//     when there is an error routine, LIB$_ERRROUCAL, but the RMS status
//     made severe when the error routine stopped the deletion; with none,
//     the most severe error status met (the first of the most severe).
//   - The name the error routine gets is the NAM's resultant string,
//     else its expanded string, else filespec as given.
//   - A related-filespec wins over the context's last file, as the
//     program's own related NAM wins in LIB$FILE_SCAN (unconfirmed).
//   - A failed $SEARCH that can't go on (NAM$L_WCC unchanged) ends the
//     search, as in LIB$FILE_SCAN. A confirm routine that leaves the FAB
//     open makes the delete RMS$_IFI (error source 1), and then the
//     search (source 0), which ends it.
//   - The search NAM's related NAM holds the expanded name (what the
//     confirm routine sees of it).

// Statuses and fields LIB$DELETE_FILE uses beyond the search routines'.
var (
	libErrRouCal = vmsdef.LibrarySymbols["LIB$_ERRROUCAL"]
	rmsIFI       = vmsdef.Symbols["RMS$_IFI"]
	namESL       = vmsdef.Symbols["NAM$B_ESL"]
	fabIFI       = vmsdef.Symbols["FAB$W_IFI"]
	fabFOP       = vmsdef.Symbols["FAB$L_FOP"]
	fopNAM       = vmsdef.Symbols["FAB$M_NAM"]
)

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

// deleter is one LIB$DELETE_FILE call's state: the search's (fileWalk),
// and the routines and resultant-name only this routine has.
type deleter struct {
	fileWalk

	// success and confirm are the program's routines (0 for none),
	// result its resultant-name descriptor (0 for none).
	success, confirm uint32
	result           uint32
}

// libDeleteFile is LIB$DELETE_FILE.
func libDeleteFile(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) == 0 || len(argv) > 9 {
		return libWroNumArg, nil
	}

	d := &deleter{
		fileWalk: fileWalk{
			env:     env,
			failure: arg(argv, 4),
			user:    arg(argv, 6),
		},
		success: arg(argv, 3), confirm: arg(argv, 5),
		result: arg(argv, 7),
	}
	d.each = d.deleteOne
	d.searchError = func(name string, status, stv uint32) (bool, uint32, error) {
		return d.reportError(name, status, stv, sourceSearch)
	}

	if status, err := d.readNames(argv, 0); err != nil || status != 0 {
		return status, err
	}

	if d.defaultSpec == "" && (strings.ContainsAny(d.spec, "*%") || strings.Contains(d.spec, "...")) {
		d.defaultSpec = ";*"
	}

	return d.walk(arg(argv, 8))
}

// deleteOne asks the confirm routine about the file called name (the file
// block's), deletes it, and tells the success routine, or the error
// routine when the delete fails. deleted is true when the file went; stop
// is true, with the status to return, when the routine ends here (the
// error routine said so, or a string couldn't be written).
func (d *deleter) deleteOne(name string) (deleted, stop bool, status uint32, err error) {
	env := d.env

	desc, status, err := d.nameArgument(d.result, name)
	if err != nil || status != 0 {
		return false, true, status, err
	}

	fab := d.file + fileFAB

	if d.confirm != 0 {
		r0, err := d.confirming(d.confirm, d.withUser(desc, d.search+findFAB)...)
		if err != nil {
			status, err = memoryFault(env, err)

			return false, true, status, err
		}

		if r0&1 == 0 {
			return false, false, 0, nil
		}
	}

	open, err := d.searchOpen()
	if err != nil {
		status, err = memoryFault(env, err)

		return false, true, status, err
	}

	if open {
		stop, status, err := d.reportError(name, rmsIFI, 0, sourceDelete)

		return false, stop, status, err
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

// reportError notes an error, status (with its status value stv, from
// source), and calls the error
// routine, if there is one, about the file called name. stop is true
// when the routine must end with the status returned: the error routine
// returned failure, or a string couldn't be written.
func (d *deleter) reportError(name string, status, stv, source uint32) (stop bool, _ uint32, err error) {
	d.note(status)

	if d.failure == 0 {
		return false, 0, nil
	}

	env := d.env

	desc, st, err := d.nameArgument(d.result, name)
	if err != nil || st != 0 {
		return true, st, err
	}

	values, err := stackLongwords(env, status, stv, source)
	if err != nil {
		st, err = memoryFault(env, writing(err))

		return true, st, err
	}

	return d.callError(status, d.withUser(desc, values, values+4, values+8)...)
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
