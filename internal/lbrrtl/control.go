package lbrrtl

import (
	"errors"
	"path"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/help"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/rms"
)

// The statuses the routines return. Each LBR$_ value is LBRSHR.EXE's
// universal symbol of that name (TestStatuses checks them against
// vmsdef.ImageSymbols).
const (
	ssNormal = 0x00000001 // SS$_NORMAL
	rmsEOF   = 0x0001827A // RMS$_EOF
	rmsFNF   = 0x00018292 // RMS$_FNF
	ssNoFile = 0x00000910 // SS$_NOSUCHFILE, RMS$_FNF's status value

	lbrNormal    = 0x00268001
	lbrNulIdx    = 0x00268810
	lbrRecTrunc  = 0x00268820
	lbrTypMismch = 0x00268830
	lbrNoMtchFou = 0x00268838
	lbrIllCtl    = 0x00269012
	lbrIllIdxNum = 0x00269022
	lbrIllFmt    = 0x0026902A
	lbrIllFunc   = 0x00269032
	lbrInvNam    = 0x00269052
	lbrInvRFA    = 0x0026905A
	lbrKeyNotFnd = 0x00269062
	lbrLibNotOpn = 0x0026906A
	lbrLkpNotDon = 0x00269072
	lbrLibOpn    = 0x0026907A
	lbrNoFilNam  = 0x00269082
	lbrNotHlpLib = 0x00269092
	lbrTooMnyLib = 0x002690B2
	lbrIllOutRou = 0x002690DA
	lbrIllInRou  = 0x002690EA
	lbrTooMnyArg = 0x002690F2
	lbrUsrInpErr = 0x002690FA
)

// The library functions LBR$INI_CONTROL takes (LBR$C_CREATE, LBR$C_READ,
// LBR$C_UPDATE).
const (
	funcCreate = 0
	funcRead   = 1
	funcUpdate = 2
)

// maxControls is how many control indexes a program may have at once (the
// manual's LBR$INI_CONTROL: "up to 16 libraries open simultaneously").
const maxControls = 16

// control is one control index's library: what LBR$INI_CONTROL said of it,
// and once LBR$OPEN has opened it, the library and the read context.
type control struct {
	function uint32
	typ      lbr.Type // LBR$C_TYP_UNK for any

	lib   *lbr.Library // nil until opened
	help  help.Library // the library as the help engine reads it, made on first need
	index int          // the current index, 1 to the library's count

	// locate is LBR$SET_LOCATE's mode: LBR$GET_RECORD returns a record in
	// buffer, a block of the image's heap, rather than copying it.
	locate bool
	buffer uint32

	// records are the module LBR$LOOKUP_KEY or LBR$FIND found, and next
	// the one LBR$GET_RECORD returns next; looked says one was found.
	records [][]byte
	next    int
	looked  bool
}

// state is one image's control indexes, and the RMS status value of its
// last LBR$OPEN (LBR$RET_RMSSTV). The routines' state lasts as long as the
// image, as VMS's lasts as long as the image's LBRSHR.
type state struct {
	controls map[uint32]*control
	stv      uint32
}

type stateKey struct{}

func stateOf(env *corevms.Environment) *state {
	return env.ImageData(stateKey{}, func() any {
		return &state{controls: map[uint32]*control{}}
	}).(*state)
}

// errStatus is a status a routine returns, from a helper that found it.
type errStatus uint32

func (e errStatus) Error() string { return "LBR status" }

// failure returns what a routine returns for err: its status, or for a
// memory error the access violation it signals.
func failure(env *corevms.Environment, err error) (uint32, error) {
	var s errStatus
	if errors.As(err, &s) {
		return uint32(s), nil
	}

	return librtl.StringFailure(env, err)
}

// controlAt reads the control index the longword at addr holds, and returns
// its control: LBR$_ILLCTL if it's none.
func controlAt(env *corevms.Environment, addr uint32) (*control, uint32, error) {
	n, err := env.Memory().LoadLongword(env.CPU(), addr)
	if err != nil {
		return nil, 0, err
	}

	c, ok := stateOf(env).controls[n]
	if !ok {
		return nil, n, errStatus(lbrIllCtl)
	}

	return c, n, nil
}

// openControl is controlAt for a routine that needs the library open:
// LBR$_LIBNOTOPN if it isn't.
func openControl(env *corevms.Environment, addr uint32) (*control, error) {
	c, _, err := controlAt(env, addr)
	if err != nil {
		return nil, err
	}

	if c.lib == nil {
		return nil, errStatus(lbrLibNotOpn)
	}

	return c, nil
}

// loadOptional reads the longword at addr, or returns def when addr is 0
// (an omitted argument passed by reference).
func loadOptional(env *corevms.Environment, addr, def uint32) (uint32, error) {
	if addr == 0 {
		return def, nil
	}

	return env.Memory().LoadLongword(env.CPU(), addr)
}

// iniControl is LBR$INI_CONTROL library_index, func [,type] [,namblk]: a
// new control index, the lowest free one from 1 (VMS 7.1 gave a program's
// first 1), stored at library_index. The NAM block isn't used: govax's
// LBR$OPEN finds the file by name each time.
func iniControl(env *corevms.Environment, argv []uint32) (uint32, error) {
	fn, err := loadOptional(env, arg(argv, 1), funcRead+100)
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	if fn > funcUpdate {
		return lbrIllFunc, nil
	}

	typ, err := loadOptional(env, arg(argv, 2), uint32(lbr.TypeUnknown))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	st := stateOf(env)

	for n := uint32(1); n <= maxControls; n++ {
		if _, used := st.controls[n]; used {
			continue
		}

		if err := env.Memory().StoreLongword(env.CPU(), arg(argv, 0), n); err != nil {
			return librtl.MemoryFault(env, librtl.Writing(err))
		}

		st.controls[n] = &control{function: fn, typ: lbr.Type(typ), index: 1}

		return lbrNormal, nil
	}

	return lbrTooMnyLib, nil
}

// open is LBR$OPEN library_index [,fns] [,create_options] [,dns] [,rlfna]
// [,rns] [,rnslen]: the library fns names, with dns's defaults, found as RMS
// finds a file (on a volume, or a host file), read whole. As VMS 7.1's did
// (testdata/help90's LBR90, round 1b):
//
//   - a file that isn't there is RMS's status (RMS$_FNF, its status value
//     SS$_NOSUCHFILE for LBR$RET_RMSSTV), and the control index is gone
//     with it: LBR$CLOSE then says LBR$_ILLCTL;
//   - a library of another type than LBR$INI_CONTROL's is opened, with
//     LBR$_TYPMISMCH, a warning; the status value is then the library's
//     type (VMS 7.1's was 3 for a help library opened as text,
//     unconfirmed why).
//
// Only reading is govax's yet: LBR$C_CREATE and LBR$C_UPDATE are the
// writing routines' (subtask 11), and signal NOTIMPL. The related name
// (rlfna) isn't used.
func open(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, n, err := controlAt(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	if c.lib != nil {
		return lbrLibOpn, nil
	}

	if c.function != funcRead {
		return librtl.NotImplemented("LBR$OPEN of a library to create or update")(env, argv)
	}

	if arg(argv, 1) == 0 {
		return lbrNoFilNam, nil
	}

	spec, err := librtl.ReadString(env, arg(argv, 1))
	if err != nil {
		return librtl.StringFailure(env, err)
	}

	var dns string

	if arg(argv, 3) != 0 {
		if dns, err = librtl.ReadString(env, arg(argv, 3)); err != nil {
			return librtl.StringFailure(env, err)
		}
	}

	st := stateOf(env)
	data, name, readErr := readLibraryFile(env.Session, spec, dns)

	if r, err := resultName(env, argv, name); err != nil || r != 0 {
		return r, err
	}

	if readErr != nil {
		delete(st.controls, n)
		st.stv = ssNoFile

		return rmsFNF, nil
	}

	lib, err := lbr.Open(data)
	if err != nil {
		delete(st.controls, n)
		st.stv = 0

		return lbrIllFmt, nil
	}

	c.lib, c.index, st.stv = lib, 1, 0

	if c.typ != lbr.TypeUnknown && c.typ != lib.Type {
		st.stv = uint32(lib.Type)

		return lbrTypMismch, nil
	}

	return lbrNormal, nil
}

// resultName writes the file's name to LBR$OPEN's rns and rnslen, when it
// passed them.
func resultName(env *corevms.Environment, argv []uint32, name string) (uint32, error) {
	if arg(argv, 5) != 0 {
		stored, _, err := librtl.WriteString(env, arg(argv, 5), name)
		if err != nil {
			return librtl.StringFailure(env, err)
		}

		if arg(argv, 6) != 0 {
			if err := env.Memory().StoreLongword(env.CPU(), arg(argv, 6), uint32(stored)); err != nil {
				return librtl.MemoryFault(env, librtl.Writing(err))
			}
		}
	}

	return 0, nil
}

// readLibraryFile reads the file spec names, with dns's defaults: on a
// volume through RMS's name processing (logical names, search lists), or a
// host file with dns's type if it has none. It returns the file's bytes and
// its name, or the name it tried with an error.
func readLibraryFile(s *rms.Session, spec, dns string) ([]byte, string, error) {
	if s == nil {
		return nil, spec, errors.New("lbrrtl: no files")
	}

	loc, err := s.Locate(spec, false)

	if err != nil || !loc.Host {
		found, tried, ferr := s.FindImageFile(spec, dns)
		if ferr != nil {
			return nil, tried, ferr
		}

		loc = found
	} else if typ := strings.TrimPrefix(path.Ext(dns), "."); typ != "" {
		loc = loc.WithDefaultType(typ)
	}

	data, found, err := s.ReadRawFile(loc)
	if err != nil {
		return nil, loc.Name, err
	}

	return data, found.Name, nil
}

// closeLibrary is LBR$CLOSE library_index: the library closed and its
// control index given back.
func closeLibrary(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, n, err := controlAt(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	if c.lib == nil {
		return lbrLibNotOpn, nil
	}

	delete(stateOf(env).controls, n)

	if c.buffer != 0 {
		env.FreeVM(c.buffer)
	}

	return lbrNormal, nil
}

// getHeader is LBR$GET_HEADER library_index, retary: the library's header
// as 128 longwords, the manual's 32 then zeros (lbr.Library.HeaderInfo).
func getHeader(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	info := c.lib.HeaderInfo()
	mem, cpu := env.Memory(), env.CPU()

	for i := range 128 {
		var v uint32
		if i < len(info) {
			v = info[i]
		}

		if err := mem.StoreLongword(cpu, arg(argv, 1)+uint32(4*i), v); err != nil {
			return librtl.MemoryFault(env, librtl.Writing(err))
		}
	}

	return lbrNormal, nil
}

// setIndex is LBR$SET_INDEX library_index, index_number: the index the
// key routines use. VMS 7.1 returned SS$_NORMAL for it (not LBR$_NORMAL),
// and LBR$_ILLIDXNUM for an index the library doesn't have.
func setIndex(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	n, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 1))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	if n < 1 || int(n) > len(c.lib.Indexes) {
		return lbrIllIdxNum, nil
	}

	c.index = int(n)

	return ssNormal, nil
}

// setLocate and setMove are LBR$SET_LOCATE and LBR$SET_MOVE library_index:
// LBR$GET_RECORD's two modes (move is the default).
func setLocate(env *corevms.Environment, argv []uint32) (uint32, error) {
	return setMode(env, argv, true)
}

func setMove(env *corevms.Environment, argv []uint32) (uint32, error) {
	return setMode(env, argv, false)
}

func setMode(env *corevms.Environment, argv []uint32, locate bool) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	c.locate = locate

	return lbrNormal, nil
}

// retRMSSTV is LBR$RET_RMSSTV: the RMS status value of the last LBR$OPEN.
func retRMSSTV(env *corevms.Environment, _ []uint32) (uint32, error) {
	return stateOf(env).stv, nil
}
