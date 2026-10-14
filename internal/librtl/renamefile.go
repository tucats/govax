package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
)

// LIB$RENAME_FILE (docs/PHASE-58, subtask 6; the survey's "RENAME_FILE
// (319)"), written from the 5.0 LIB$ manual (LIB-319):
//
//	LIB$RENAME_FILE old-filespec, new-filespec [,default-filespec]
//	                [,related-filespec] [,flags]
//	                [,user-success-procedure] [,user-error-procedure]
//	                [,user-confirm-procedure] [,user-specified-argument]
//	                [,old-resultant-name] [,new-resultant-name]
//	                [,file-scan-context]
//
// It renames every file old-filespec matches, as DCL's RENAME does. The
// search is LIB$DELETE_FILE's (fileWalk, filewalk.go): default-filespec
// and related-filespec apply to old-filespec, with input file parsing,
// and file-scan-context carries the last file renamed to the next call.
// For each file found
//
//  1. its new name is worked out: new-filespec, its missing fields and
//     its "*" fields the old file's, and the version by DCL RENAME's
//     rules (rms.Session.RenameTarget, the code DCL's RENAME uses), and
//     its directory must exist; a failure (a syntax error, a wildcard the
//     new name can't take, RMS$_DNF) is error source 1;
//  2. the confirm routine, when there is one, is called with the old and
//     new names and the search's FAB, which describes the old file; a
//     failure (bit 0 of R0 clear) passes the file over;
//  3. the file is renamed with $RENAME's code (rms.Session.RenameFile); a
//     failure is error source 2;
//  4. the success routine is called with the old name and the new one,
//     the version the file got included.
//
// An error searching for old-filespec is error source 0. The error
// routine gets the old and new names, the RMS status and status value
// and the error source by reference, and the user-specified-argument.
// Every routine gets that last, 0 when the program gave none. The error
// routine's success goes on, its failure stops the routine at once with
// the RMS status made severe.
//
// flags bit 0, LIB$M_FIL_CUR_VER, keeps a file's version when
// new-filespec gives none, as /NONEW_VERSION does. Bit 1 (INH_SECUR)
// changes nothing on the VAX; bit 2 (the VSI manual's LONG_NAMES) and the
// rest are LIB$_INVARG, as on VMS 7.1.
//
// What the manuals leave open is as VMS 7.1 does it (testdata/lib58,
// docs/PHASE-58's subtask 8), but where marked unconfirmed:
//
//   - Fewer than two arguments, or more than twelve, is LIB$_WRONUMARG; a
//     new-filespec over 255 characters is LIB$_INVFILSPE, as old-filespec
//     is.
//   - The new name the error routine gets at source 0 is new-filespec as
//     given; at source 1, the new name as far as it was worked out (else,
//     for a syntax error, new-filespec as given).
//   - An input that named a version doesn't keep it: the file gets the
//     next version of its new name (1 when there's none).
//   - A new name that exists already, with CUR_VER, is $RENAME's failure,
//     RMS$_ENT with SS$_DUPFILENAME.
//   - The statuses are LIB$DELETE_FILE's (SS$_NORMAL; LIB$_ERRROUCAL; the
//     RMS status made severe when the error routine stops; the most severe
//     error with no error routine).

// renameFlags are the flag bits LIB$RENAME_FILE accepts, and renameCurVer
// LIB$M_FIL_CUR_VER.
const (
	renameCurVer = 1 << 0
	renameFlags  = renameCurVer | 1<<1
)

// The error source for a new name that can't be worked out; errors
// searching are sourceSearch (0), and $RENAME's failures sourceRename.
const (
	sourceParse  = 1
	sourceRename = 2
)

// renamer is one LIB$RENAME_FILE call's state: the search's (fileWalk),
// and what only this routine has.
type renamer struct {
	fileWalk

	// newSpec is new-filespec, and curVer flags bit 0.
	newSpec string
	curVer  bool

	// success and confirm are the program's routines (0 for none),
	// oldResult and newResult its resultant-name descriptors (0 for
	// none).
	success, confirm     uint32
	oldResult, newResult uint32
}

// libRenameFile is LIB$RENAME_FILE.
func libRenameFile(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 || len(argv) > 12 {
		return libWroNumArg, nil
	}

	r := &renamer{
		fileWalk: fileWalk{
			env:     env,
			failure: arg(argv, 6),
			user:    arg(argv, 8),
		},
		success: arg(argv, 5), confirm: arg(argv, 7),
		oldResult: arg(argv, 9), newResult: arg(argv, 10),
	}
	r.each = r.renameOne
	r.searchError = func(name string, status, stv uint32) (bool, uint32, error) {
		return r.reportError(name, r.newSpec, status, stv, sourceSearch)
	}

	// old-filespec, then new-filespec, then the default and related
	// names (readNames's three, from argument 2 on, with old-filespec's
	// place taken first).
	var err error
	if r.newSpec, err = readString(env, arg(argv, 1)); err != nil {
		return stringFailure(env, err)
	}

	names := append([]uint32{argv[0]}, argv[2:min(len(argv), 4)]...)
	if status, err := r.readNames(names, 0); err != nil || status != 0 {
		return status, err
	}

	if len(r.newSpec) > maxFileSpec {
		return libInvFilSpe, nil
	}

	flags, err := optionalLongword(env, arg(argv, 4), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	if flags&^renameFlags != 0 {
		return libInvArg, nil
	}

	r.curVer = flags&renameCurVer != 0

	return r.walk(arg(argv, 11))
}

// renameOne works out the new name of the file called name (the file
// block's), asks the confirm routine, renames the file, and tells the
// success routine, or the error routine when a step fails. renamed is
// true when the file was; stop is true, with the status to return, when
// the routine ends here.
func (r *renamer) renameOne(name string) (renamed, stop bool, status uint32, err error) {
	env := r.env
	s := session(env)

	target, status, stv := s.RenameTarget(name, r.newSpec, inputVersion(r.expanded), !r.curVer)
	if status != 0 {
		if target == "" {
			target = r.newSpec
		}

		stop, status, err := r.reportError(name, target, status, stv, sourceParse)

		return false, stop, status, err
	}

	oldDesc, newDesc, status, err := r.nameArguments(name, target)
	if err != nil || status != 0 {
		return false, true, status, err
	}

	if r.confirm != 0 {
		r0, err := r.confirming(r.confirm, r.withUser(oldDesc, newDesc, r.search+findFAB)...)
		if err != nil {
			status, err = memoryFault(env, err)

			return false, true, status, err
		}

		if r0&1 == 0 {
			return false, false, 0, nil
		}
	}

	open, err := r.searchOpen()
	if err != nil {
		status, err = memoryFault(env, err)

		return false, true, status, err
	}

	if open {
		stop, status, err := r.reportError(name, target, rmsIFI, 0, sourceRename)

		return false, stop, status, err
	}

	newName, status, stv := s.RenameFile(name, target)
	if status&1 == 0 {
		stop, status, err := r.reportError(name, newName, status, stv, sourceRename)

		return false, stop, status, err
	}

	if r.success != 0 {
		// The new name now has the version the file got.
		if oldDesc, newDesc, status, err = r.nameArguments(name, newName); err != nil || status != 0 {
			return true, true, status, err
		}

		if _, err := env.Callback(r.success, r.withUser(oldDesc, newDesc)...); err != nil {
			status, err = memoryFault(env, err)

			return true, true, status, err
		}
	}

	return true, false, 0, nil
}

// reportError notes an error, status (with its status value stv, from
// source), and calls the error routine, if there is one, about the file
// called oldName, being renamed to newName.
func (r *renamer) reportError(oldName, newName string, status, stv, source uint32) (stop bool, _ uint32, err error) {
	r.note(status)

	if r.failure == 0 {
		return false, 0, nil
	}

	env := r.env

	oldDesc, newDesc, st, err := r.nameArguments(oldName, newName)
	if err != nil || st != 0 {
		return true, st, err
	}

	values, err := stackLongwords(env, status, stv, source)
	if err != nil {
		st, err = memoryFault(env, writing(err))

		return true, st, err
	}

	return r.callError(status, oldDesc, newDesc, values, values+4, values+8, r.user)
}

// nameArguments returns the descriptors of the old and new names for a
// callback: the program's resultant-name descriptors, the names written
// into them, or class S descriptors below SP.
func (r *renamer) nameArguments(oldName, newName string) (oldDesc, newDesc, status uint32, err error) {
	if oldDesc, status, err = r.nameArgument(r.oldResult, oldName); err != nil || status != 0 {
		return 0, 0, status, err
	}

	newDesc, status, err = r.nameArgument(r.newResult, newName)

	return oldDesc, newDesc, status, err
}

// inputVersion is the version field of an expanded name: what follows
// its last ";" ("" when there's none).
func inputVersion(expanded string) string {
	if i := strings.LastIndexByte(expanded, ';'); i >= 0 {
		return expanded[i+1:]
	}

	return ""
}
