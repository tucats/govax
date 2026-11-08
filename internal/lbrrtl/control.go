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

	rmsFLK   = 0x0001828A // RMS$_FLK

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

	// The writing routines' statuses (round 3's definitions).
	lbrDupKey     = 0x0026900A
	lbrIllCreOpt  = 0x0026901A
	lbrIllOp      = 0x0026903A
	lbrInvKey     = 0x0026904A
	lbrRecLng     = 0x0026909A
	lbrRFAPastEOF = 0x002690AA
	lbrUpdurTrav  = 0x002690BA
	lbrWriteErr   = 0x002690D2
	lbrStillKeys  = 0x00268828
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

	// lastModule is the records of the module last read, which a lookup of
	// a key whose module can't be read gives again (readModule).
	lastModule [][]byte

	// upd is the library being created or changed (LBR$C_CREATE or
	// LBR$C_UPDATE), written back to file by LBR$CLOSE; nil when it's
	// only read. changed says a routine has changed it.
	upd     *lbr.Updater
	file    rms.FileLocation
	changed bool

	// written says LBR$FLUSH has written a new library's file, which
	// LBR$CLOSE then writes over rather than making another.
	written bool

	// unwritable says a routine took a library opened only to read for
	// changed (LBR$PUT_HISTORY does), so that LBR$FLUSH and LBR$CLOSE fail
	// to write it, as VMS 7.1's did.
	unwritable bool

	// traversing is set while LBR$GET_INDEX calls its routine, which may
	// not change the index (LBR$_UPDURTRAV).
	traversing bool
}

// refresh reads the library again after a routine changed it, so that the
// reading routines see the change.
func (c *control) refresh() {
	c.changed = true

	if lib, err := c.upd.Library(); err == nil {
		c.lib, c.help = lib, nil
	}
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
// A library opened to create is made in memory from the create options
// (createOptions) and written as a new file by LBR$CLOSE; one opened to
// update is read whole and written back over itself (subtask 11, round
// 3). The related name (rlfna) isn't used.
func open(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, n, err := controlAt(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	if c.lib != nil {
		return lbrLibOpn, nil
	}

	st := stateOf(env)

	if c.function == funcCreate {
		return openCreate(env, argv, c, n)
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

	data, found, readErr := readLibraryFile(env.Session, spec, dns)

	if r, err := resultName(env, argv, found.Name); err != nil || r != 0 {
		return r, err
	}

	if readErr != nil {
		delete(st.controls, n)
		st.stv = ssNoFile

		return rmsFNF, nil
	}

	// A library another control index has open to update is locked
	// (VMS 7.1 waited for it, past the probe's two minutes).
	if c.function == funcUpdate {
		for _, o := range st.controls {
			if o.upd != nil && strings.EqualFold(o.file.Name, found.Name) {
				delete(st.controls, n)

				return rmsFLK, nil
			}
		}
	}

	lib, err := lbr.Open(data)
	if err != nil {
		delete(st.controls, n)
		st.stv = 0

		return lbrIllFmt, nil
	}

	c.lib, c.index, st.stv = lib, 1, 0

	if c.function == funcUpdate {
		if c.upd, err = lbr.OpenUpdater(data); err != nil {
			delete(st.controls, n)

			return lbrIllFmt, nil
		}

		c.upd.Now, c.file = env.Clock, found
	}

	if c.typ != lbr.TypeUnknown && c.typ != lib.Type {
		st.stv = uint32(lib.Type)

		return lbrTypMismch, nil
	}

	return lbrNormal, nil
}

// The create options' longwords (the manual's CRE$ block, round 3's
// definitions).
const (
	creType   = 0x00
	creKeyLen = 0x04
	creIdxMax = 0x0C
	creUhdMax = 0x10
	creEntAll = 0x14
	creLuhMax = 0x18
	creVerTyp = 0x1C
	creIdxOpt = 0x20

	creVMSV3 = 3 // CRE$C_VMSV3
)

// openCreate is LBR$OPEN of a library to create: the create options read
// (LBR$_ILLCREOPT without them, or with values out of range, the control
// index then gone as it is for a missing file), the new library made in
// memory, and its file's name, fns with dns's defaults, kept for
// LBR$CLOSE. The type is the options', whatever LBR$INI_CONTROL said
// (round 3's WE2). Only the V3 format is govax's: CRE$C_VMSV2 signals
// NOTIMPL.
func openCreate(env *corevms.Environment, argv []uint32, c *control, n uint32) (uint32, error) {
	st := stateOf(env)

	if arg(argv, 2) == 0 {
		delete(st.controls, n)

		return lbrIllCreOpt, nil
	}

	var cre [9]uint32

	for i := range cre {
		v, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 2)+uint32(4*i))
		if err != nil {
			return librtl.MemoryFault(env, err)
		}

		cre[i] = v
	}

	if cre[creVerTyp/4] != creVMSV3 {
		return librtl.NotImplemented("LBR$OPEN of a library in the VMS V2 format")(env, argv)
	}

	if arg(argv, 1) == 0 {
		delete(st.controls, n)

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

	loc, err := env.Session.Locate(spec, false)
	if err != nil {
		delete(st.controls, n)

		return lbrNoFilNam, nil
	}

	if typ := strings.TrimPrefix(path.Ext(dns), "."); typ != "" {
		loc = loc.WithDefaultType(strings.ToUpper(typ))
	}

	now := env.Clock()

	u, err := lbr.NewUpdater(lbr.UpdateOptions{
		Type:      lbr.Type(cre[creType/4]),
		KeyLength: int(cre[creKeyLen/4]),
		Indexes:   int(cre[creIdxMax/4]),
		UserData:  int(cre[creUhdMax/4]),
		Entries:   int(cre[creEntAll/4]),
		History:   int(cre[creLuhMax/4]),
		IndexOpts: int(cre[creIdxOpt/4]),
		Librarian: lbr.LibrarianName,
		Created:   now,
	})
	if err != nil {
		delete(st.controls, n)

		return lbrIllCreOpt, nil
	}

	lib, err := u.Library()
	if err != nil {
		delete(st.controls, n)

		return lbrIllCreOpt, nil
	}

	if r, err := resultName(env, argv, loc.Name); err != nil || r != 0 {
		return r, err
	}

	u.Now = env.Clock
	c.lib, c.upd, c.file, c.index, c.changed, st.stv = lib, u, loc, 1, true, 0

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
// its place, or the name it tried with an error.
func readLibraryFile(s *rms.Session, spec, dns string) ([]byte, rms.FileLocation, error) {
	if s == nil {
		return nil, rms.FileLocation{Name: spec}, errors.New("lbrrtl: no files")
	}

	loc, err := s.Locate(spec, false)

	if err != nil || !loc.Host {
		found, tried, ferr := s.FindImageFile(spec, dns)
		if ferr != nil {
			return nil, rms.FileLocation{Name: tried}, ferr
		}

		loc = found
	} else if typ := strings.TrimPrefix(path.Ext(dns), "."); typ != "" {
		loc = loc.WithDefaultType(typ)
	}

	data, found, err := s.ReadRawFile(loc)
	if err != nil {
		return nil, loc, err
	}

	return data, found, nil
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

	if c.unwritable {
		return rmsFAC, nil
	}

	if c.upd != nil && c.changed {
		if err := writeLibrary(env, c); err != nil {
			return lbrWriteErr, nil
		}
	}

	return lbrNormal, nil
}

// writeLibrary writes a library the routines created or changed: a new
// one as a new file (a new version, on a volume), a changed one, or a new
// one LBR$FLUSH has written before, over the old, keeping its version, as
// LIBRARY writes them.
func writeLibrary(env *corevms.Environment, c *control) error {
	if c.function == funcUpdate {
		c.upd.SetUpdated(env.Clock())
	}

	data := c.upd.Bytes()

	blocks := make([][]byte, 0, len(data)/512)
	for i := 0; i < len(data); i += 512 {
		blocks = append(blocks, data[i:i+512])
	}

	write := env.Session.RewriteRecordFile
	if c.function == funcCreate && !c.written {
		write = env.Session.CreateRecordFile
	}

	written, err := write(c.file, rms.ImageBlocks, blocks)
	if err == nil {
		c.file = written // with its version, for a later rewrite
	}

	return err
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
