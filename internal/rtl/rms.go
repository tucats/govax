package rtl

import (
	"fmt"
	"io"
	"os"

	"github.com/tucats/govax/internal/vax"
)

// Port of rms.c's rms_create/rms_connect/rms_put — the only three RMS
// operations the C source implements at all (no read/close/etc. exist
// there either; sequential organization only).
//
// FAB/RAB field byte offsets below are the real, standard VMS $FABDEF/
// $RABDEF layout (FAB_K_BLN/RAB_K_BLN — 80/68 bytes — and every field's
// natural C-struct offset in fab.h/rab.h have no compiler-inserted padding
// at any point a real VMS FAB/RAB compiled program would rely on), not a
// port of structure_mapping.c's map()/map_add() declarative field table:
// rms.c registers each field's *VAX-side* offset via map_add's own
// sequential auto-assignment (vax_offset == MAPNEXT), while its *native*
// offset comes from STROFF (real struct offsetof) — two different
// numbering schemes reused for one field, that only coincide here because
// fab.h/rab.h's field order happens to introduce zero padding at each
// transition. Since Go has no equivalent need for a separate native-struct
// offset (there's no local C struct to marshal into — VAX memory is read
// directly field-by-field below), porting the two-offset map() indirection
// itself would add a layer of machinery with nothing to abstract over;
// using the real, well-known field offsets directly is simpler and behaves
// identically for every field this phase touches.
const (
	fabIFI = 2  // FAB$W_IFI, word
	fabFAC = 22 // FAB$B_FAC, byte
	fabFNA = 44 // FAB$L_FNA, longword (VAX address of the filename string)
	fabFNS = 48 // FAB$B_FNS, byte
	fabSTS = 8  // FAB$L_STS, longword
	fabSTV = 12 // FAB$L_STV, longword

	rabFAB = 60 // RAB$L_FAB, longword (VAX address of the related FAB)
	rabISI = 2  // RAB$W_ISI, word
	rabRAC = 30 // RAB$B_RAC, byte
	rabRSZ = 34 // RAB$W_RSZ, word
	rabRBF = 40 // RAB$L_RBF, longword (VAX address of the record buffer)
	rabSTS = 8  // RAB$L_STS, longword
	rabSTV = 12 // RAB$L_STV, longword
)

// FAB$B_FAC values this phase implements (fab.h's FAB_M_PUT etc.) — only
// PUT access, matching rms_create's own single case.
const fabFACPut = 1

// RAB$B_RAC values (rab.h's RAB_C_SEQ etc.) — only sequential access,
// matching rms_put's own single case.
const rabRACSeq = 0

// allocIFI returns an unused RMS "internal file index" >= 4 (0-3 are the
// fixed invalid/stdout/stdin/stderr slots seeded at construction), matching
// get_free_ifi's own linear scan for a free slot.
func (env *Environment) allocIFI() uint16 {
	if env.nextIFI < 4 {
		env.nextIFI = 4
	}
	for {
		if _, used := env.ifiFiles[env.nextIFI]; !used {
			id := env.nextIFI
			env.nextIFI++
			return id
		}
		env.nextIFI++
	}
}

// ifiWriter resolves an RMS internal file index to where SYS$PUT should
// write, matching rmsinit's ifi[0]=invalid/ifi[1]=stdout/ifi[2]=stdin/
// ifi[3]=stderr seeding plus rms_create's dynamically fopen'd slots. Slots 2
// and 3 (stdin/stderr) are never written by any operation this phase
// implements, so they resolve to "not a writer" rather than a real stream.
func (env *Environment) ifiWriter(ifi uint16) (io.Writer, bool) {
	switch ifi {
	case 1:
		return env.consoleOut, env.consoleOut != nil
	case 0, 2, 3:
		return nil, false
	default:
		w, ok := env.ifiFiles[ifi]
		return w, ok
	}
}

// storeRMSStatus writes SS_NORMAL to both a block's STS and STV fields,
// matching every RMS handler's own "rc = SS_NORMAL; store_field(...STS...);
// store_field(...STV...)" tail.
func (env *Environment) storeRMSStatus(base uint32, stsOffset, stvOffset uint32) error {
	if err := env.mem.StoreLongword(env.cpu, base+stsOffset, ssNormal); err != nil {
		return err
	}
	return env.mem.StoreLongword(env.cpu, base+stvOffset, ssNormal)
}

// serviceSysCreate is SYS$CREATE: opens (or, for the console pseudo-device,
// binds) a file for output access, storing the resulting IFI back into the
// FAB.
func serviceSysCreate(env *Environment, argv []uint32) (uint32, error) {
	fabAddr := argv[0]

	fac, err := env.mem.LoadByte(env.cpu, fabAddr+fabFAC)
	if err != nil {
		return ssAccVio, nil
	}
	if fac != fabFACPut {
		return ssNoSuchFac, nil
	}

	fns, err := env.mem.LoadByte(env.cpu, fabAddr+fabFNS)
	if err != nil {
		return ssAccVio, nil
	}
	maxLen := int(fns)
	if maxLen == 0 {
		maxLen = 255
	}
	fna, err := env.mem.LoadLongword(env.cpu, fabAddr+fabFNA)
	if err != nil {
		return ssAccVio, nil
	}
	fn, err := loadString(env, fna, maxLen)
	if err != nil {
		return ssAccVio, nil
	}

	if ln, found := env.Logicals.Get("LNM$FILE_DEV", fn, 0); found {
		fn = ln.Value
	}

	var ifi uint16
	if fn == "TTA0:" {
		ifi = 1
	} else {
		w, err := openRMSFile(fn)
		if err != nil {
			return ssNoSuchFile, nil
		}
		ifi = env.allocIFI()
		env.ifiFiles[ifi] = w
	}

	if env.cpu.DebugEnabled(vax.DebugRMS) {
		fmt.Fprintf(env.cpu.DebugWriter(), "RMS: In SYS$CREATE function, FAB=%08X, FAC=%d, FN=%q, writing to IFI[%d]\n",
			fabAddr, fac, fn, ifi)
	}

	if err := env.mem.StoreWord(env.cpu, fabAddr+fabIFI, ifi); err != nil {
		return ssAccVio, nil
	}
	if err := env.storeRMSStatus(fabAddr, fabSTS, fabSTV); err != nil {
		return ssAccVio, nil
	}
	return ssNormal, nil
}

// serviceSysConnect is SYS$CONNECT: binds a RAB to its FAB's already-open
// IFI.
func serviceSysConnect(env *Environment, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	if env.cpu.DebugEnabled(vax.DebugRMS) {
		fmt.Fprintf(env.cpu.DebugWriter(), "RMS: In SYS$CONNECT function, RAB=%08X.\n", rabAddr)
	}

	fabAddr, err := env.mem.LoadLongword(env.cpu, rabAddr+rabFAB)
	if err != nil {
		return ssAccVio, nil
	}
	ifi, err := env.mem.LoadWord(env.cpu, fabAddr+fabIFI)
	if err != nil {
		return ssAccVio, nil
	}

	if err := env.mem.StoreWord(env.cpu, rabAddr+rabISI, ifi); err != nil {
		return ssAccVio, nil
	}
	if err := env.storeRMSStatus(rabAddr, rabSTS, rabSTV); err != nil {
		return ssAccVio, nil
	}
	return ssNormal, nil
}

// serviceSysPut is SYS$PUT: writes one sequential record.
//
// rms_put's own invalid-RAC-code branch has its "return SS_INVARG" nested
// inside an "if (debug flag)" block — a misplaced-brace bug (this is RTL
// tooling, not ISA behavior, so it's fixed directly per this project's
// established policy — see internal/io's own doc.go) that silently drops
// the error return whenever the debug flag is off, RMS's normal operating
// mode; fixed here to always report SS_INVARG for an unsupported RAC.
func serviceSysPut(env *Environment, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	fabAddr, err := env.mem.LoadLongword(env.cpu, rabAddr+rabFAB)
	if err != nil {
		return ssAccVio, nil
	}
	fabIfi, err := env.mem.LoadWord(env.cpu, fabAddr+fabIFI)
	if err != nil {
		return ssAccVio, nil
	}

	if env.cpu.DebugEnabled(vax.DebugRMS) {
		fmt.Fprintf(env.cpu.DebugWriter(), "RMS: In SYS$PUT function, RAB=%08X, FAB=%08X  IFI=%d\n",
			rabAddr, fabAddr, fabIfi)
	}

	w, ok := env.ifiWriter(fabIfi)
	if !ok {
		return ssInvArg, nil
	}

	rac, err := env.mem.LoadByte(env.cpu, rabAddr+rabRAC)
	if err != nil {
		return ssAccVio, nil
	}
	if rac != rabRACSeq {
		return ssInvArg, nil
	}

	rsz, err := env.mem.LoadWord(env.cpu, rabAddr+rabRSZ)
	if err != nil {
		return ssAccVio, nil
	}
	rbf, err := env.mem.LoadLongword(env.cpu, rabAddr+rabRBF)
	if err != nil {
		return ssAccVio, nil
	}

	buf := make([]byte, rsz)
	for i := range buf {
		b, err := env.mem.LoadByte(env.cpu, rbf+uint32(i))
		if err != nil {
			return ssAccVio, nil
		}
		buf[i] = b
	}
	if _, err := w.Write(buf); err != nil {
		return ssAccVio, nil
	}
	if fabIfi <= 3 {
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return ssAccVio, nil
		}
	}

	if err := env.storeRMSStatus(rabAddr, rabSTS, rabSTV); err != nil {
		return ssAccVio, nil
	}
	return ssNormal, nil
}

// openRMSFile opens fn for output, matching rms_create's own fopen(fn, "w")
// — truncate-or-create, write-only.
func openRMSFile(fn string) (io.Writer, error) {
	return os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
}

func registerRMSServices(t *ServiceTable) {
	t.Register("SYS$CREATE", serviceSysCreate)
	t.Register("SYS$CONNECT", serviceSysConnect)
	t.Register("SYS$PUT", serviceSysPut)
}
