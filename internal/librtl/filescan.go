package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// LIB$FILE_SCAN and LIB$FILE_SCAN_END (docs/PHASE-58, subtask 4; the
// survey's "FILE_SCAN (151) and FILE_SCAN_END (153)"), written from the
// 5.0 LIB$ manual (LIB-151, LIB-153, and section 2.7.4, "File Searching
// Routines"):
//
//	LIB$FILE_SCAN fab ,user-success-procedure ,user-error-procedure
//	              [,context]
//	LIB$FILE_SCAN_END [fab] [,context]
//
// LIB$FILE_SCAN runs a whole wildcard search in one call, on the
// program's own FAB and NAM: a $PARSE, then $SEARCH until RMS$_NMF,
// calling the success routine for each file found and the error routine
// for each failure. The routines read what they need from the FAB and its
// NAM (the resultant name, FAB$L_STS and FAB$L_STV, the program's own
// FAB$L_CTX). It is the first routine that calls back into the program,
// so it runs on a goroutine of its own (corevms.Environment.Callback;
// corevms's callback.go), and reads as the loop the manual describes.
//
// The context, "the address of a longword" in which the routine keeps
// the "name blocks and file specification strings" it allocates for
// temporary defaulting across "multiple input files" (DCL's
// COPY [SMITH]A,B,C *), is a heap block holding a NAM and a buffer: the
// last call's file specification, the related name of the next call's
// $PARSE. What's kept in Go is only which blocks are this image's
// contexts, as for LIB$FIND_FILE's (findfile.go).
//
// What the manuals leave open is as VMS 7.1 does it (testdata/lib58,
// docs/PHASE-58's subtask 8):
//
//   - The routines are called with LIB$FILE_SCAN's own argument list, the
//     FAB's address first (a CALLG with its AP). A routine of 0 isn't
//     called, and what the routines return is ignored.
//   - The first thing the routine does is write the NAM (its WCC's high
//     byte), without checking the FAB: a FAB with no NAM, or a block
//     that isn't a FAB, is an access violation at NAM$L_WCC+3.
//   - A $PARSE that fails leaves the file specification in the NAM's
//     resultant string, and the error routine is called.
//   - A specification with no wildcard (NAM$V_WILDCARD clear, a search
//     list included) gets one $SEARCH. Otherwise $SEARCH runs until
//     RMS$_NMF, or until it fails with RMS$_FNF or RMS$_DNF, or in a way
//     that leaves NAM$L_WCC as it was (it would fail the same way again).
//     The routine's status is the last service's.
//   - With a context, the last call's file specification (FNA, as the
//     program gave it) is the related name: the NAM's RLF points at the
//     context's NAM, and stays so. The context is made, and its longword
//     written, when the call ends; a call abandoned in a callback leaves
//     the longword 0. A longword that holds an address that isn't one of
//     this image's contexts is taken as one anyway, unchecked, as VMS
//     does (its related NAM then fails $PARSE with RMS$_RLF). Without a
//     context, the routine does no defaulting of its own.
//   - LIB$FILE_SCAN_END's fab is parsed with FNS and DNS 0 (the null
//     string) and its NAM's ESS 0, which frees RMS's wildcard context,
//     and they stay 0 (unconfirmed which of them VMS sets); its
//     status is SS$_NORMAL, or RMS$_FAB for a block that isn't a FAB. Its
//     context is freed (when it's this image's) and the longword cleared.

// Statuses and fields LIB$FILE_SCAN uses beyond LIB$FIND_FILE's.
var (
	rmsNMF = vmsdef.Symbols["RMS$_NMF"]
	rmsFNF = vmsdef.Symbols["RMS$_FNF"]
	rmsDNF = vmsdef.Symbols["RMS$_DNF"]
	namWCC = vmsdef.Symbols["NAM$L_WCC"]
)

// A scan context's layout: a chain of name blocks, each a NAM and its
// resultant string's buffer, the first the newest file specification.
// Each NAM's RLF points at the next, so a specification is defaulted from
// the one before it, which was from the one before that ("the addresses
// of name blocks and file specification strings" the manual says the
// context keeps).
const (
	scanNAM        = 0
	scanRSA        = scanNAM + 96
	scanSlot       = scanRSA + fileBuffer
	scanSlots      = 8
	scanBlockBytes = scanSlot * scanSlots
)

// scanContext is what LIB$FILE_SCAN keeps in Go for a context: the
// specifications in its chain, newest first, and the one the next call
// adds to it (the last call's, as VMS 7.1 adds it: at the start of the
// call after).
type scanContext struct {
	names   []string
	pending string
}

// scanContextsKey is the corevms.Environment.ImageData key of the image's
// scan contexts, by their blocks' addresses.
type scanContextsKey struct{}

// scanContexts returns the running image's scan contexts.
func scanContexts(env *corevms.Environment) map[uint32]*scanContext {
	return env.ImageData(scanContextsKey{}, func() any {
		return map[uint32]*scanContext{}
	}).(map[uint32]*scanContext)
}

// libFileScan is LIB$FILE_SCAN.
func libFileScan(env *corevms.Environment, argv []uint32) (uint32, error) {
	fab, success, failure, cell := arg(argv, 0), arg(argv, 1), arg(argv, 2), arg(argv, 3)
	mem, cpu := env.Memory(), env.CPU()

	nam, err := mem.LoadLongword(cpu, fab+fabNAM)
	if err != nil {
		return memoryFault(env, err)
	}

	// The NAM's WCC is the routine's first write (see the opening comment).
	if err := touchByte(env, nam+namWCC+3); err != nil {
		return memoryFault(env, writing(err))
	}

	var block uint32

	if cell != 0 {
		if block, err = mem.LoadLongword(cpu, cell); err != nil {
			return memoryFault(env, err)
		}
	}

	// call calls routine, when there is one, with the routine's own
	// argument list.
	call := func(routine uint32) error {
		if routine == 0 {
			return nil
		}

		_, err := env.Callback(routine, argv...)

		return err
	}

	if block != 0 {
		related, err := scanChain(env, block)
		if err != nil {
			return memoryFault(env, writing(err))
		}

		if related {
			if err := mem.StoreLongword(cpu, nam+namRLF, block+scanNAM); err != nil {
				return memoryFault(env, writing(err))
			}
		}
	}

	status, err := env.CallService("SYS$PARSE", []uint32{fab})
	if err != nil {
		return status, err
	}

	if status&1 == 0 {
		if err := resultantIsSpec(env, fab, nam); err != nil {
			return memoryFault(env, err)
		}

		if err := call(failure); err != nil {
			return memoryFault(env, err)
		}

		return status, nil
	}

	// The default name has been applied: VMS 7.1 leaves FAB$B_DNS 0 from
	// here on (testdata/lib58's FS default name).
	if err := mem.StoreByte(cpu, fab+fabDNS, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	fnb, err := mem.LoadLongword(cpu, nam+namFNB)
	if err != nil {
		return memoryFault(env, err)
	}

	wild := fnb&fnbWildcard != 0

	for {
		wcc, err := searchPlace(env, nam)
		if err != nil {
			return memoryFault(env, err)
		}

		if status, err = env.CallService("SYS$SEARCH", []uint32{fab}); err != nil {
			return status, err
		}

		if status == rmsNMF {
			break
		}

		routine := success
		if status&1 == 0 {
			routine = failure
		}

		if err := call(routine); err != nil {
			return memoryFault(env, err)
		}

		if !wild {
			break
		}

		if status&1 == 0 {
			after, err := searchPlace(env, nam)
			if err != nil {
				return memoryFault(env, err)
			}

			if after == wcc || status == rmsFNF || status == rmsDNF {
				break
			}
		}
	}

	if cell != 0 {
		if err := keepScanSpec(env, fab, cell, block); err != nil {
			return memoryFault(env, err)
		}
	}

	return status, nil
}

// touchByte writes the byte at addr with its own value: what an access
// to it would do, without changing it.
func touchByte(env *corevms.Environment, addr uint32) error {
	mem, cpu := env.Memory(), env.CPU()

	b, err := mem.LoadByte(cpu, addr)
	if err != nil {
		return err
	}

	return mem.StoreByte(cpu, addr, b)
}

// resultantIsSpec writes the NAM's expanded string, or, when $PARSE got
// no expanded string, the FAB's file specification (FNA, FNS), as its
// resultant string, as far as RSS allows: what LIB$FILE_SCAN leaves after
// a $PARSE that failed.
func resultantIsSpec(env *corevms.Environment, fab, nam uint32) error {
	mem, cpu := env.Memory(), env.CPU()

	spec, err := expandedString(env, nam)
	if err == nil && spec == "" {
		spec, err = fabFileName(env, fab)
	}

	if err != nil {
		return err
	}

	rss, err := mem.LoadByte(cpu, nam+namRSS)
	if err != nil {
		return err
	}

	rsa, err := mem.LoadLongword(cpu, nam+namRSA)
	if err != nil {
		return err
	}

	if rsa == 0 {
		return nil
	}

	spec = spec[:min(len(spec), int(rss))]

	for i := range len(spec) {
		if err := mem.StoreByte(cpu, rsa+uint32(i), spec[i]); err != nil {
			return writing(err)
		}
	}

	return writing(mem.StoreByte(cpu, nam+namRSL, byte(len(spec))))
}

// fabFileName reads the FAB's file specification, FNA for FNS bytes.
func fabFileName(env *corevms.Environment, fab uint32) (string, error) {
	mem, cpu := env.Memory(), env.CPU()

	fns, err := mem.LoadByte(cpu, fab+fabFNS)
	if err != nil {
		return "", err
	}

	fna, err := mem.LoadLongword(cpu, fab+fabFNA)
	if err != nil {
		return "", err
	}

	return readText(env, fna, uint32(fns))
}

// keepScanSpec keeps the FAB's file specification for the scan context
// to add to its chain at the next call, making the context first when the
// longword at cell is 0 (block). err is a memory error, for the caller to
// signal.
func keepScanSpec(env *corevms.Environment, fab, cell, block uint32) error {
	block, err := scanBlockAt(env, cell, block)
	if err != nil || block == 0 {
		return err
	}

	st := scanContexts(env)[block]
	if st == nil {
		// A block this image didn't make: nothing is kept in it.
		return nil
	}

	st.pending, err = fabFileName(env, fab)

	return err
}

// scanChain adds the scan context's pending specification to its chain
// and lays the chain out, reporting whether the block now has a related
// name to give (a block that isn't one of this image's contexts is taken
// as having one, unchecked). err is a memory error.
func scanChain(env *corevms.Environment, block uint32) (bool, error) {
	st := scanContexts(env)[block]
	if st == nil {
		return true, nil
	}

	if st.pending != "" {
		st.names = append([]string{st.pending}, st.names...)
		st.names = st.names[:min(len(st.names), scanSlots)]
		st.pending = ""
	}

	if err := layoutScanChain(env, block, st.names); err != nil {
		return false, err
	}

	return len(st.names) > 0, nil
}

// searchPlace returns the NAM's NAM$L_WCC, where its search is: 0 when
// there is no NAM (whose $SEARCH fails the same way every time).
func searchPlace(env *corevms.Environment, nam uint32) (uint32, error) {
	if nam == 0 {
		return 0, nil
	}

	return env.Memory().LoadLongword(env.CPU(), nam+namWCC)
}

// initScanBlock clears a scan context and lays out an empty chain, then
// writes the block's address to the longword at cell.
func initScanBlock(env *corevms.Environment, block, cell uint32) error {
	mem, cpu := env.Memory(), env.CPU()

	for off := uint32(0); off < scanBlockBytes; off += 4 {
		if err := mem.StoreLongword(cpu, block+off, 0); err != nil {
			return err
		}
	}

	if err := layoutScanChain(env, block, nil); err != nil {
		return err
	}

	return mem.StoreLongword(cpu, cell, block)
}

// layoutScanChain sets the context's name blocks up as the $NAM macro
// would, each one's resultant string a name of names (none for slots past
// them) and its RLF the next one's NAM.
func layoutScanChain(env *corevms.Environment, block uint32, names []string) error {
	mem, cpu := env.Memory(), env.CPU()

	for i := range scanSlots {
		nam := block + uint32(i)*scanSlot + scanNAM
		buf := block + uint32(i)*scanSlot + scanRSA

		name := ""
		if i < len(names) {
			name = names[i]
		}

		name = name[:min(len(name), maxFileSpec)]

		for _, b := range []struct{ addr, v uint32 }{
			{nam + namBID, namBIDValue},
			{nam + namBLN, namBLNValue},
			{nam + namRSS, maxFileSpec},
			{nam + namRSL, uint32(len(name))},
		} {
			if err := mem.StoreByte(cpu, b.addr, byte(b.v)); err != nil {
				return err
			}
		}

		for j := range len(name) {
			if err := mem.StoreByte(cpu, buf+uint32(j), name[j]); err != nil {
				return err
			}
		}

		rlf := uint32(0)
		if i+1 < len(names) {
			rlf = nam + scanSlot
		}

		for _, l := range []struct{ addr, v uint32 }{{nam + namRSA, buf}, {nam + namRLF, rlf}} {
			if err := mem.StoreLongword(cpu, l.addr, l.v); err != nil {
				return err
			}
		}
	}

	return nil
}

// setScanName makes name the scan context's only name: how LIB$DELETE_FILE
// and LIB$RENAME_FILE leave the last file they acted on as the next call's
// related name.
func setScanName(env *corevms.Environment, block uint32, name string) error {
	st := scanContexts(env)[block]
	if st == nil {
		return nil
	}

	st.names, st.pending = []string{name}, ""

	return layoutScanChain(env, block, st.names)
}

// libFileScanEnd is LIB$FILE_SCAN_END: with fab, a $PARSE of the null
// string on it; with context, the context freed and its longword
// cleared; with neither, nothing.
func libFileScanEnd(env *corevms.Environment, argv []uint32) (uint32, error) {
	fab, cell := arg(argv, 0), arg(argv, 1)

	if fab != 0 {
		status, err := parseNull(env, fab)
		if err != nil || status == rmsFAB {
			return status, err
		}
	}

	if cell == 0 {
		return ssNormal, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	block, err := mem.LoadLongword(cpu, cell)
	if err != nil {
		return memoryFault(env, err)
	}

	if block == 0 {
		return ssNormal, nil
	}

	// A block that isn't one of this image's contexts isn't freed; the
	// longword is cleared all the same.
	if contexts := scanContexts(env); contexts[block] != nil {
		delete(contexts, block)
		env.FreeVM(block)
	}

	if err := mem.StoreLongword(cpu, cell, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// parseNull runs $PARSE on the program's FAB with its FNS and DNS 0, the
// null string, and its NAM's ESS 0, so that RMS frees any wildcard
// context its NAM names (all three stay 0, as VMS leaves them). It
// returns $PARSE's status, or RMS$_FAB
// without changing anything when the block isn't a FAB; err is a memory
// error, already signaled.
func parseNull(env *corevms.Environment, fab uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	bid, err := mem.LoadByte(cpu, fab+fabBID)
	if err != nil {
		return memoryFault(env, err)
	}

	if uint32(bid) != fabBIDValue {
		return rmsFAB, nil
	}

	sizes := []uint32{fab + fabFNS, fab + fabDNS}

	// The NAM's ESS too, so the parse makes no expanded string: VMS 7.1
	// leaves all three 0, and a LIB$FILE_SCAN of the FAB after is then
	// RMS$_ESL from $SEARCH (testdata/lib58's FS after FSE).
	nam, err := mem.LoadLongword(cpu, fab+fabNAM)
	if err != nil {
		return memoryFault(env, err)
	}

	if nam != 0 {
		sizes = append(sizes, nam+namESS)
	}

	for _, size := range sizes {
		if err := mem.StoreByte(cpu, size, 0); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	return env.CallService("SYS$PARSE", []uint32{fab})
}
