package librtl

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/rtl"
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
//	               [,relative-volume-number]
//
// It creates the directory the specification names, and every missing
// directory above it, on a mounted volume. The directories themselves are
// made as CREATE/DIRECTORY makes them (rms.Session.CreateDirectory).

// Statuses LIB$CREATE_DIR returns.
var (
	ssCreated    = vmsdef.Symbols["SS$_CREATED"]
	ssWritLck    = vmsdef.Symbols["SS$_WRITLCK"]
	rmsDir       = vmsdef.Symbols["RMS$_DIR"]
	rmsDev       = vmsdef.Symbols["RMS$_DEV"]
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
//
// It returns SS$_CREATED when it made any directory and SS$_NORMAL when
// all of them existed. Otherwise the status says why, as $PARSE or the
// file system would: RMS$_DIR for a name too long or a path too deep,
// RMS$_DEV for a device that isn't mounted, SS$_WRITLCK for a volume
// mounted read-only, SS$_ACCVIO for an argument it can't read.
func libCreateDir(env *rtl.Environment, argv []uint32) (uint32, error) {
	descAddr := arg(argv, 0)
	if descAddr == 0 {
		return libInvArg, nil
	}

	spec, ok, err := env.StringDescriptor(descAddr, maxDirectorySpec)

	switch {
	case err != nil:
		return ssAccVio, nil
	case !ok:
		return libInvArg, nil
	}

	opts := rms.CreateDirectoryOptions{VolumeOnly: true}

	if status := readCreateDirArgs(env, argv, &opts); status != 0 {
		return status, nil
	}

	spec, status := uicFormat(spec, &opts)
	if status != 0 {
		return status, nil
	}

	// The specification must name a directory itself: without one, RMS
	// would fill in the default, which the manual doesn't allow.
	if !strings.ContainsAny(spec, "[<") || strings.Contains(spec, "::") {
		return libInvFilSpe, nil
	}

	created, err := session(env).CreateDirectory(spec, opts)
	if err != nil {
		return createDirStatus(err), nil
	}

	for _, c := range created {
		if c.Created {
			return ssCreated, nil
		}
	}

	return ssNormal, nil
}

// readCreateDirArgs reads LIB$CREATE_DIR's optional arguments into opts,
// returning SS$_ACCVIO for one it can't read and 0 otherwise.
func readCreateDirArgs(env *rtl.Environment, argv []uint32, opts *rms.CreateDirectoryOptions) uint32 {
	mem, cpu := env.Memory(), env.CPU()

	if a := arg(argv, 1); a != 0 {
		uic, err := mem.LoadLongword(cpu, a)
		if err != nil {
			return ssAccVio
		}

		if uic != 0 {
			opts.Owner = &ondisk.Uic{Group: uint16(uic >> 16), Member: uint16(uic)}
		}
	}

	if a := arg(argv, 2); a != 0 {
		enable, err := mem.LoadWord(cpu, a)
		if err != nil {
			return ssAccVio
		}

		opts.ProtectionEnable = enable
	}

	if a := arg(argv, 3); a != 0 {
		value, err := mem.LoadWord(cpu, a)
		if err != nil {
			return ssAccVio
		}

		opts.ProtectionValue = value
	}

	if a := arg(argv, 4); a != 0 {
		limit, err := mem.LoadWord(cpu, a)
		if err != nil {
			return ssAccVio
		}

		opts.VersionLimit = &limit
	}

	if a := arg(argv, 5); a != 0 {
		if _, err := mem.LoadWord(cpu, a); err != nil {
			return ssAccVio
		}
	}

	return 0
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
func session(env *rtl.Environment) *rms.Session {
	if env.Session != nil {
		return env.Session
	}

	s := rms.NewSession(env.Mounts)
	s.Logicals = env.Logicals

	return s
}

// createDirStatus is the status for err, a directory that couldn't be made.
func createDirStatus(err error) uint32 {
	var notMounted *rms.NotMountedError

	switch {
	case errors.Is(err, volume.ErrDirectoryName):
		return rmsDir
	case errors.Is(err, rms.ErrNotDirectorySpec):
		return libInvFilSpe
	case errors.Is(err, rms.ErrACPWriteLocked):
		return ssWritLck
	case errors.As(err, &notMounted):
		return rmsDev
	}

	return rmsCre
}
