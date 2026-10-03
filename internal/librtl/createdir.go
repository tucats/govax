package librtl

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// LIB$CREATE_DIR (docs/PHASE-34.md, subtask 12), written from the VMS
// Run-Time Library Routines Volume, Library (LIB$) Manual (VMS 5.0,
// AA-LA76A-TE, LIB-35 to LIB-39):
//
//	LIB$CREATE_DIR device-directory-spec [,owner-UIC] [,protection-enable]
//	               [,protection-value] [,maximum-versions]
//	               [,relative-volume-number] [,initial-allocation]
//
// It creates the directory the specification names, and every missing
// directory above it, on a mounted volume. The directories themselves are
// made as CREATE/DIRECTORY makes them (rms.Session.CreateDirectory). The
// seventh argument isn't in the VMS 5.0 manual, but VMS 7.3 takes it: the
// probe's call with a longword 4 there made a 4-block directory.

// Statuses LIB$CREATE_DIR returns.
var (
	ssCreated    = vmsdef.Symbols["SS$_CREATED"]
	ssWritLck    = vmsdef.Symbols["SS$_WRITLCK"]
	ssNoSuchDev  = vmsdef.Symbols["SS$_NOSUCHDEV"]
	ssDevNotMnt  = vmsdef.Symbols["SS$_DEVNOTMOUNT"]
	rmsDir       = vmsdef.Symbols["RMS$_DIR"]
	rmsCre       = vmsdef.Symbols["RMS$_CRE"]
	libInvArg    = vmsdef.LibrarySymbols["LIB$_INVARG"]
	libInvFilSpe = vmsdef.LibrarySymbols["LIB$_INVFILSPE"]
)

// maxDirectorySpec is the longest device-directory-spec the manual allows.
const maxDirectorySpec = 255

// uicDirectory matches a directory in UIC format, "[123,321]" (with an
// optional device), which the manual treats specially.
var uicDirectory = regexp.MustCompile(`^([^\[<]*)[\[<]([0-7]+),([0-7]+)[\]>]$`)

// libCreateDir is LIB$CREATE_DIR. Following the manual:
//
//   - device-directory-spec (a descriptor, required) must name a directory
//     explicitly, and no node, file name, type, version, or wildcard, in at
//     most 255 characters. Missing or too long is LIB$_INVARG; anything
//     else wrong with it, or no mounted disk to put it on, LIB$_INVFILSPE.
//   - owner-UIC (a longword by reference): 0 or omitted is the parent
//     directory's owner, except that a directory in UIC format, [g,m], is
//     owned by that UIC (and named GGGMMM.DIR, the digits each padded to
//     three).
//   - protection-enable and protection-value (words by reference): enabled
//     bits come from the value, the rest from the parent's protection, less
//     delete. An enable of 0 or omitted means the parent's, less delete.
//   - maximum-versions (a word by reference): omitted is the parent's
//     default limit, 0 no limit.
//   - relative-volume-number (a word by reference): govax's volumes are
//     single, so it has no effect.
//   - initial-allocation (a longword by reference): the blocks to give each
//     new directory; omitted or 0, one.
//
// It returns SS$_CREATED when it made any directory and SS$_NORMAL when
// all of them existed. Otherwise the status says why, as VMS 7.3 returned
// it on the probe (testdata/credir/libcrd.mar): RMS$_DIR for a name too
// long, a path too deep, or one above the MFD; SS$_NOSUCHDEV for a device
// that doesn't exist (SS$_DEVNOTMOUNT for one that isn't mounted);
// SS$_WRITLCK for a volume mounted read-only. A directory named only
// through a logical name ("CRDLOG:", for "DUA1:[PLOG]") counts as named.
// An argument it can't read -- a 0 descriptor address included -- isn't a
// status: VMS signals an access violation, and so does govax.
func libCreateDir(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) == 0 {
		return libInvArg, nil
	}

	// VMS reads the descriptor without checking its address: a 0 address
	// is an access violation at 4, where its string pointer would be.
	descAddr := argv[0]
	if descAddr == 0 {
		return accessViolation(env, 4)
	}

	spec, ok, err := env.StringDescriptor(descAddr, maxDirectorySpec)

	switch {
	case err != nil:
		return accessViolation(env, descAddr)
	case !ok:
		return libInvArg, nil
	}

	opts := rms.CreateDirectoryOptions{VolumeOnly: true, RequireDirectory: true}

	if bad, unreadable := readCreateDirArgs(env, argv, &opts); unreadable {
		return accessViolation(env, bad)
	}

	spec, status := uicFormat(spec, &opts)
	if status != 0 {
		return status, nil
	}

	if strings.Contains(spec, "::") {
		return libInvFilSpe, nil
	}

	created, err := session(env).CreateDirectory(spec, opts)
	if err != nil {
		return createDirStatus(env, err), nil
	}

	for _, c := range created {
		if c.Created {
			return ssCreated, nil
		}
	}

	return ssNormal, nil
}

// accessViolation signals SS$_ACCVIO for a read at va (reason mask 0), as
// the hardware would have had LIB$CREATE_DIR touched it on VMS. The
// signal array is the hardware's: the condition, the reason, the address,
// then the PC and PSL.
func accessViolation(env *corevms.Environment, va uint32) (uint32, error) {
	return env.Signal([]uint32{ssAccVio, 0, va}, false)
}

// readCreateDirArgs reads LIB$CREATE_DIR's optional arguments into opts.
// If one can't be read, it returns that argument's address and true.
func readCreateDirArgs(env *corevms.Environment, argv []uint32, opts *rms.CreateDirectoryOptions) (uint32, bool) {
	mem, cpu := env.Memory(), env.CPU()

	word := func(i int) (uint16, bool, bool) {
		a := arg(argv, i)
		if a == 0 {
			return 0, false, false
		}

		v, err := mem.LoadWord(cpu, a)

		return v, true, err != nil
	}

	long := func(i int) (uint32, bool, bool) {
		a := arg(argv, i)
		if a == 0 {
			return 0, false, false
		}

		v, err := mem.LoadLongword(cpu, a)

		return v, true, err != nil
	}

	if uic, given, bad := long(1); bad {
		return arg(argv, 1), true
	} else if given && uic != 0 {
		opts.Owner = &ondisk.Uic{Group: uint16(uic >> 16), Member: uint16(uic)}
	}

	if enable, _, bad := word(2); bad {
		return arg(argv, 2), true
	} else {
		opts.ProtectionEnable = enable
	}

	if value, _, bad := word(3); bad {
		return arg(argv, 3), true
	} else {
		opts.ProtectionValue = value
	}

	if limit, given, bad := word(4); bad {
		return arg(argv, 4), true
	} else if given {
		opts.VersionLimit = &limit
	}

	if _, _, bad := word(5); bad { // relative-volume-number: no effect
		return arg(argv, 5), true
	}

	if blocks, _, bad := long(6); bad {
		return arg(argv, 6), true
	} else {
		opts.Allocation = blocks
	}

	return 0, false
}

// uicFormat rewrites a directory in UIC format, [g,m], as the directory it
// names, [GGGMMM], and makes that UIC its owner unless owner-UIC gave one.
// A UIC out of range is LIB$_INVFILSPE.
func uicFormat(spec string, opts *rms.CreateDirectoryOptions) (string, uint32) {
	m := uicDirectory.FindStringSubmatch(strings.TrimSpace(spec))
	if m == nil {
		return spec, 0
	}

	uic, err := ondisk.ParseUic("[" + m[2] + "," + m[3] + "]")
	if err != nil || uic.Group > 0o777 || uic.Member > 0o777 {
		return spec, libInvFilSpe
	}

	if opts.Owner == nil {
		opts.Owner = &uic
	}

	return fmt.Sprintf("%s[%03o%03o]", m[1], uic.Group, uic.Member), 0
}

// session is the RMS session a program's call resolves names in: the
// console's, so SET DEFAULT applies, or one of the environment's own.
func session(env *corevms.Environment) *rms.Session {
	if env.Session != nil {
		return env.Session
	}

	s := rms.NewSession(env.Mounts)
	s.Logicals = env.Logicals

	return s
}

// createDirStatus is the status for err, a directory that couldn't be made.
func createDirStatus(env *corevms.Environment, err error) uint32 {
	var notMounted *rms.NotMountedError

	switch {
	case errors.Is(err, volume.ErrDirectoryName):
		return rmsDir
	case errors.Is(err, rms.ErrNotDirectorySpec):
		return libInvFilSpe
	case errors.Is(err, rms.ErrACPWriteLocked):
		return ssWritLck
	case errors.As(err, &notMounted):
		if env.Devices != nil {
			if _, known := env.Devices.Find(notMounted.Device); known {
				return ssDevNotMnt
			}
		}

		return ssNoSuchDev
	}

	return rmsCre
}
