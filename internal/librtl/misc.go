package librtl

import (
	"errors"
	"math"
	"math/big"
	"os"
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// The miscellaneous interface routines (docs/PHASE-55, the survey's
// "Miscellaneous"; the 5.0 manual's section 2.7): LIB$AST_IN_PROG,
// LIB$ASN_WTH_MBX, LIB$WAIT, and LIB$GET_USERS_LANGUAGE. Each is a small
// layer over system services or the logical names.

var (
	libEnglUsed = vmsdef.LibrarySymbols["LIB$_ENGLUSED"]

	jpiAstAct = vmsdef.Symbols["JPI$_ASTACT"]
	dviDevNam = vmsdef.Symbols["DVI$_DEVNAM"]
)

// libAstInProg is LIB$AST_IN_PROG:
//
//	LIB$AST_IN_PROG
//
// It returns 1 if an AST routine is running in the caller's access mode
// (the PSL's current mode at the call; unconfirmed which mode VMS asks
// about), else 0: $GETJPI's JPI$_ASTACT, as the manual's condition
// values say it asks. A value, not a status. govax's can't fail.
func libAstInProg(env *corevms.Environment, _ []uint32) (uint32, error) {
	item, st := env.ProcessItem(jpiAstAct, 0, "", false)
	if st != 0 {
		return 0, nil
	}

	mode := uint32(env.CPU().PSL().CurMod())

	return corevms.ItemLongword(item.Data) >> mode & 1, nil
}

// libAsnWthMbx is LIB$ASN_WTH_MBX:
//
//	LIB$ASN_WTH_MBX device-name.rt.dx ,[maximum-message-size.rl.r]
//	                ,[buffer-quota.rl.r] ,device-channel.wwu.r
//	                ,mailbox-channel.wwu.r
//
// It creates a temporary mailbox ($CREMBX, no logical name; the size and
// quota by reference, omitted or 0 the service's defaults) on
// mailbox-channel, then assigns device-name to device-channel ($ASSIGN)
// with that mailbox, by its device name ($GETDVI's DVI$_DEVNAM), as the
// device's associated mailbox. $ASSIGN translates a logical name. If
// $ASSIGN fails, the mailbox channel is deassigned, which deletes the
// temporary mailbox (unconfirmed), and $ASSIGN's status is returned. The
// manual has the routine take an event flag for its $GETDVIW; govax's
// asks the device's name of Go and takes none (the survey's "No event
// flag of their own").
func libAsnWthMbx(env *corevms.Environment, argv []uint32) (uint32, error) {
	device, err := readString(env, arg(argv, 0))
	if err != nil {
		return stringFailure(env, err)
	}

	maxmsg, err := optionalLongword(env, arg(argv, 1), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	bufquo, err := optionalLongword(env, arg(argv, 2), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	devChan, mbxChanAdr := arg(argv, 3), arg(argv, 4)

	status, err := env.CallService("SYS$CREMBX", []uint32{0, mbxChanAdr, maxmsg, bufquo, 0, 0, 0})
	if err != nil || status&1 == 0 {
		return status, err
	}

	mbxChan, err := env.Memory().LoadWord(env.CPU(), mbxChanAdr)
	if err != nil {
		return memoryFault(env, err)
	}

	item, st := env.DeviceItem(dviDevNam, uint32(mbxChan), "", false)
	if st != 0 {
		return st, nil
	}

	// $ASSIGN's names go by class S descriptors in this routine's frame,
	// whatever class the caller's device-name was.
	devDesc, err := stackString(env, device)
	if err != nil {
		return memoryFault(env, writing(err))
	}

	mbxDesc, err := stackString(env, strings.TrimRight(item.Data, "\x00 "))
	if err != nil {
		return memoryFault(env, writing(err))
	}

	status, err = env.CallService("SYS$ASSIGN", []uint32{devDesc, devChan, 0, mbxDesc})
	if err != nil {
		return status, err
	}

	if status&1 == 0 {
		_, _ = env.CallService("SYS$DASSGN", []uint32{uint32(mbxChan)})
	}

	return status, nil
}

// LIB$WAIT's flags bit and float types (the VSI manual's LIB$K_
// values; none is in vmsdef).
const (
	waitNoWake = 1 // LIB$K_NOWAKE: wait with $SETIMR, not $SCHDWK/$HIBER

	floatVAXF  = 0 // LIB$K_VAX_F, the default
	floatVAXD  = 1 // LIB$K_VAX_D
	floatVAXG  = 2 // LIB$K_VAX_G
	floatVAXH  = 3 // LIB$K_VAX_H
	floatIEEES = 4 // LIB$K_IEEE_S, IEEE single precision
	floatIEEET = 5 // LIB$K_IEEE_T, IEEE double precision
)

// maxWaitSeconds is the longest wait LIB$WAIT takes.
const maxWaitSeconds = 100_000

// hundredthTicks is a hundredth of a second in VMS time units (100ns).
const hundredthTicks = vmsdef.TicksPerSecond / 100

// vaxFloatTypes are the float-type codes of the VAX formats.
var vaxFloatTypes = map[uint32]vaxfloat.Format{
	floatVAXF: vaxfloat.F, floatVAXD: vaxfloat.D, floatVAXG: vaxfloat.G, floatVAXH: vaxfloat.H,
}

// pendingWait is a LIB$WAIT call that is waiting: its $HIBER, or, with
// NOWAKE, its $WAITFR on the event flag efn, which it allocated from
// LIB$GET_EF's pool.
type pendingWait struct {
	noWake bool
	efn    int32
}

// waitsKey is the corevms.Environment.ImageData key of the image's
// pending waits.
type waitsKey struct{}

// waits returns the running image's pending LIB$WAIT calls, by the frame
// pointer of the routine's stub (see libWait).
func waits(env *corevms.Environment) map[uint32]*pendingWait {
	return env.ImageData(waitsKey{}, func() any { return map[uint32]*pendingWait{} }).(map[uint32]*pendingWait)
}

// libWait is LIB$WAIT:
//
//	LIB$WAIT seconds.rf.r [,flags.rlu.r] [,float-type.rlu.r]
//
// It waits seconds (0.0 to 100,000.0, rounded to the nearest hundredth;
// else LIB$_INVARG), by scheduling a wakeup that far off ($SCHDWK) and
// hibernating ($HIBER) in the caller's mode, so ASTs are delivered and a
// $WAKE ends the wait early. A wake already pending ends it at once, and
// the scheduled one stays pending (unconfirmed; the 5.0 entry describes
// no loop). With flags bit 0, LIB$K_NOWAKE (the VSI manual's), it
// waits with $SETIMR on an event flag from LIB$GET_EF's pool instead,
// which no $WAKE ends (LIB$_INSEF with none free). float-type (also the
// VSI manual's) names seconds' format: VAX F (the default), D, G, or H,
// or IEEE S or T; another is LIB$_INVARG (unconfirmed), as is a reserved
// operand, an IEEE NaN or infinity. LIB$_WRONUMARG for no arguments or
// more than three.
//
// A wait is the service's: it returns corevms.ErrWait, and the XFC runs
// again later, so this routine is called again with the same stub frame.
// It remembers the call by that frame's FP (an AST's own LIB$WAIT has a
// frame below it), so a call already waiting only waits again.
func libWait(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) == 0 || len(argv) > 3 {
		return libWroNumArg, nil
	}

	pending := waits(env)
	fp := env.CPU().GPR(registers.FP)

	if p, ok := pending[fp]; ok {
		return awaitWait(env, pending, fp, p)
	}

	flags, err := optionalLongword(env, arg(argv, 1), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	floatType, err := optionalLongword(env, arg(argv, 2), floatVAXF)
	if err != nil {
		return memoryFault(env, err)
	}

	hundredths, ok, err := waitHundredths(env, arg(argv, 0), floatType)
	if err != nil {
		return memoryFault(env, err)
	}

	if !ok {
		return libInvArg, nil
	}

	delta := -int64(hundredths * hundredthTicks)
	p := &pendingWait{noWake: flags&waitNoWake != 0}

	if p.noWake {
		pl := pools(env)

		efn, ok := nextEF(&pl.flags)
		if !ok {
			return libInsEF, nil
		}

		p.efn = efn

		status, err := callWithTime(env, "SYS$SETIMR", delta, func(q uint32) []uint32 { return []uint32{uint32(efn), q, 0, 0} })
		if err != nil || status&1 == 0 {
			pl.flags &^= 1 << efn

			return status, err
		}
	} else {
		status, err := callWithTime(env, "SYS$SCHDWK", delta, func(q uint32) []uint32 { return []uint32{0, 0, q, 0} })
		if err != nil || status&1 == 0 {
			return status, err
		}
	}

	return awaitWait(env, pending, fp, p)
}

// awaitWait makes p's waiting call: $HIBER, or $WAITFR on its flag. While
// it waits, p is remembered under fp; once it's done, p is forgotten and
// its flag, if any, goes back to the pool.
func awaitWait(env *corevms.Environment, pending map[uint32]*pendingWait, fp uint32, p *pendingWait) (uint32, error) {
	var (
		status uint32
		err    error
	)

	if p.noWake {
		status, err = env.CallService("SYS$WAITFR", []uint32{uint32(p.efn)})
	} else {
		status, err = env.CallService("SYS$HIBER", nil)
	}

	if errors.Is(err, corevms.ErrWait) {
		pending[fp] = p

		return 0, err
	}

	delete(pending, fp)

	if p.noWake {
		pools(env).flags &^= 1 << p.efn
	}

	return status, err
}

// callWithTime calls service with the arguments args gives for the
// address of a quadword holding t (a VMS time), which it puts below SP
// for the call and takes off again after.
func callWithTime(env *corevms.Environment, service string, t int64, args func(uint32) []uint32) (uint32, error) {
	cpu := env.CPU()
	sp := cpu.GPR(registers.SP)
	q := sp - 8

	if err := env.Memory().StoreQuadword(cpu, q, uint64(t)); err != nil {
		return memoryFault(env, writing(err))
	}

	cpu.SetGPR(registers.SP, q)
	status, err := env.CallService(service, args(q))
	cpu.SetGPR(registers.SP, sp)

	return status, err
}

// waitHundredths reads LIB$WAIT's seconds at addr, in the format
// floatType names, and returns it in hundredths of a second, rounded to
// the nearest (half up). ok is false, for LIB$_INVARG, for an unknown
// float type, a value that isn't a number (a VAX reserved operand, an
// IEEE NaN or infinity), or one outside 0 to 100,000 seconds. err is a
// memory error reading it.
func waitHundredths(env *corevms.Environment, addr, floatType uint32) (uint64, bool, error) {
	var seconds *big.Rat

	switch floatType {
	case floatVAXF, floatVAXD, floatVAXG, floatVAXH:
		v, err := loadFloat(env, vaxFloatTypes[floatType], addr)
		if errors.Is(err, vaxfloat.ErrReserved) {
			return 0, false, nil
		} else if err != nil {
			return 0, false, err
		}

		seconds = v.Rat()

	case floatIEEES, floatIEEET:
		var f float64

		if floatType == floatIEEES {
			l, err := env.Memory().LoadLongword(env.CPU(), addr)
			if err != nil {
				return 0, false, err
			}

			f = float64(math.Float32frombits(l))
		} else {
			q, err := env.Memory().LoadQuadword(env.CPU(), addr)
			if err != nil {
				return 0, false, err
			}

			f = math.Float64frombits(q)
		}

		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false, nil
		}

		seconds = new(big.Rat).SetFloat64(f)

	default:
		return 0, false, nil
	}

	if seconds.Sign() < 0 || seconds.Cmp(big.NewRat(maxWaitSeconds, 1)) > 0 {
		return 0, false, nil
	}

	// Round seconds*100 to the nearest integer: floor(x + 1/2).
	x := new(big.Rat).Mul(seconds, big.NewRat(100, 1))
	x.Add(x, big.NewRat(1, 2))

	return new(big.Int).Quo(x.Num(), x.Denom()).Uint64(), true, nil
}

// languageSetting is the configuration key that names the user's
// language when SYS$LANGUAGE doesn't (govax's own; HELP CONFIG KEYS).
const languageSetting = "vax.language"

// hostLanguages are the VMS language names of the host's ISO 639
// language codes, for a locale such as fr_FR.UTF-8.
var hostLanguages = map[string]string{
	"da": "DANISH", "de": "GERMAN", "en": "ENGLISH", "es": "SPANISH",
	"fi": "FINNISH", "fr": "FRENCH", "it": "ITALIAN", "ja": "JAPANESE",
	"ko": "KOREAN", "nb": "NORWEGIAN", "nl": "DUTCH", "nn": "NORWEGIAN",
	"no": "NORWEGIAN", "pl": "POLISH", "pt": "PORTUGUESE", "ru": "RUSSIAN",
	"sv": "SWEDISH", "zh": "CHINESE",
}

// hostLanguage is the language of the host's locale, as the C library
// would choose it (Go has no locale API of its own): the first of
// LC_ALL, LC_MESSAGES, and LANG that is set, its language code
// ("fr" of "fr_FR.UTF-8") named as hostLanguages names it. ok is false
// when none is set, it's the C or POSIX locale, or its language isn't
// one hostLanguages knows.
func hostLanguage() (string, bool) {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := os.Getenv(name)
		if v == "" {
			continue
		}

		code, _, _ := strings.Cut(v, "_")
		code, _, _ = strings.Cut(code, ".")
		lang, ok := hostLanguages[strings.ToLower(code)]

		return lang, ok
	}

	return "", false
}

// usersLanguage is the user's language: SYS$LANGUAGE's first
// equivalence string (translated through LNM$FILE_DEV, as RMS translates
// a name; unconfirmed which tables VMS searches), else govax's
// vax.language setting, else the host's locale's language. ok is false
// when none of them gives one.
func usersLanguage(env *corevms.Environment) (string, bool) {
	if db := env.Logicals; db != nil {
		e, err := db.Translate(lnm.FileDevName, "SYS$LANGUAGE", lnm.User, 0)
		if err == nil && len(e.Equivalences) > 0 {
			return e.Equivalences[0].Value, true
		}
	}

	if v := strings.TrimSpace(settings.Get(languageSetting)); v != "" {
		return strings.ToUpper(v), true
	}

	return hostLanguage()
}

// libGetUsersLanguage is LIB$GET_USERS_LANGUAGE:
//
//	LIB$GET_USERS_LANGUAGE language.wt.dx
//
// It writes the user's language (usersLanguage) to language by its
// class, or "ENGLISH" with LIB$_ENGLUSED, a warning, when nothing names
// one. A string failure or LIB$_STRTRU overrides either status, as
// LIB$SCOPY_R_DX's would. Where VMS has only the logical name, govax
// also asks its configuration and the host (the author's choice,
// 2026-10-13).
func libGetUsersLanguage(env *corevms.Environment, argv []uint32) (uint32, error) {
	status := ssNormal

	lang, ok := usersLanguage(env)
	if !ok {
		lang, status = "ENGLISH", libEnglUsed
	}

	_, truncated, err := writeString(env, arg(argv, 0), lang)
	if err != nil {
		return stringFailure(env, err)
	}

	if truncated {
		return libStrTru, nil
	}

	return status, nil
}
