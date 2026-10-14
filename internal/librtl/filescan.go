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
// for each failure, each with one argument, the FAB's address. The
// routines read what they need from the FAB and its NAM (the resultant
// name, FAB$L_STS and FAB$L_STV, the program's own FAB$L_CTX). It is the
// first routine that calls back into the program, so it runs on a
// goroutine of its own (corevms.Environment.Callback; corevms's
// callback.go), and reads as the loop the manual describes.
//
// The context, "the address of a longword" in which the routine keeps
// the "name blocks and file specification strings" it allocates for
// temporary defaulting across "multiple input files" (DCL's
// COPY [SMITH]A,B,C *), is a heap block holding a NAM and a buffer: the
// last file found, the related name of the next call's $PARSE. What's
// kept in Go is only which blocks are contexts, per image, as for
// LIB$FIND_FILE's (findfile.go).
//
// The choices the manuals leave open are the survey's probe cases, each
// unconfirmed until the probe (subtask 8) runs on VMS:
//
//   - A success or error routine of 0 isn't called. What the routines
//     return in R0 is ignored: the search goes on after every file and
//     every error ("allows the search sequence to continue even if an
//     error occurs").
//   - The routine's status is that of the last RMS service: RMS$_NMF
//     after a search that ran out, the error when $PARSE failed (the
//     error routine is called first) or when $SEARCH failed in a way it
//     can't go on from. A failed $SEARCH that leaves NAM$L_WCC as it was
//     (a missing directory or device, in a search without a wildcard
//     context) would fail the same way again, so the scan ends there; one
//     that moved it on (RMS$_FNF, after which the next $SEARCH says
//     RMS$_NMF) is followed by another $SEARCH.
//   - With a context, the last file found is the related name of the
//     next call: the program's NAM$L_RLF points at the context's NAM for
//     the call, when the program gave no related NAM of its own, and is
//     put back to 0 when the call ends. Without one, the routine does no
//     defaulting of its own: "inherited from the previous file
//     specification processed" is read as the program's own related NAM,
//     if it has one.
//   - A context longword that holds no context's address is LIB$_INVARG,
//     to both routines.
//   - LIB$FILE_SCAN_END's fab is parsed with FNS and DNS 0 (the null
//     string), which frees RMS's wildcard context, and both are put back
//     after; its status is SS$_NORMAL (the VSI manual's), or RMS$_FAB for
//     a block that isn't a FAB. Its context is freed and the longword
//     cleared ("replacing the context value passed to it with a
//     temporary context value"), which a longword of 0 needs nothing for.

// Statuses and fields LIB$FILE_SCAN uses beyond LIB$FIND_FILE's.
var (
	rmsNMF = vmsdef.Symbols["RMS$_NMF"]
	namWCC = vmsdef.Symbols["NAM$L_WCC"]
)

// A scan context's layout: a NAM whose resultant string is the last
// file found, then its buffer.
const (
	scanNAM        = 0
	scanRSA        = scanNAM + 96
	scanBlockBytes = scanRSA + fileBuffer
)

// scanContextsKey is the corevms.Environment.ImageData key of the image's
// scan contexts, by their blocks' addresses.
type scanContextsKey struct{}

// scanContexts returns the running image's scan contexts.
func scanContexts(env *corevms.Environment) map[uint32]bool {
	return env.ImageData(scanContextsKey{}, func() any {
		return map[uint32]bool{}
	}).(map[uint32]bool)
}

// libFileScan is LIB$FILE_SCAN.
func libFileScan(env *corevms.Environment, argv []uint32) (uint32, error) {
	fab, success, failure, cell := arg(argv, 0), arg(argv, 1), arg(argv, 2), arg(argv, 3)
	mem, cpu := env.Memory(), env.CPU()

	// The context, made on the first call that passes one.
	var block uint32

	if cell != 0 {
		var err error
		if block, err = mem.LoadLongword(cpu, cell); err != nil {
			return memoryFault(env, err)
		}

		switch {
		case block == 0:
			status, err := newScanBlock(env, cell, &block)
			if err != nil || status != 0 {
				return status, err
			}
		case !scanContexts(env)[block]:
			return libInvArg, nil
		}
	}

	// call calls routine, when there is one, with the FAB's address.
	call := func(routine uint32) error {
		if routine == 0 {
			return nil
		}

		_, err := env.Callback(routine, fab)

		return err
	}

	// With a context holding a name, it is the related name, unless the
	// program gave its own.
	nam, related, err := scanRelated(env, fab, block)
	if err != nil {
		return memoryFault(env, err)
	}

	if related {
		defer func() {
			// The deferred put-back may run as the routine is abandoned
			// in a callback; a memory error is then nobody's to report.
			_ = mem.StoreLongword(cpu, nam+namRLF, 0)
		}()
	}

	status, err := env.CallService("SYS$PARSE", []uint32{fab})
	if err != nil {
		return status, err
	}

	if status&1 == 0 {
		if err := call(failure); err != nil {
			return memoryFault(env, err)
		}

		return status, nil
	}

	var last string

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

		if status&1 != 0 {
			if last, err = resultantName(env, nam); err != nil {
				return memoryFault(env, err)
			}

			if err := call(success); err != nil {
				return memoryFault(env, err)
			}

			continue
		}

		if err := call(failure); err != nil {
			return memoryFault(env, err)
		}

		after, err := searchPlace(env, nam)
		if err != nil {
			return memoryFault(env, err)
		}

		if after == wcc {
			break
		}
	}

	if block != 0 && last != "" {
		if err := setScanName(env, block, last); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	return status, nil
}

// searchPlace returns the NAM's NAM$L_WCC, where its search is: 0 when
// there is no NAM (whose $SEARCH fails the same way every time).
func searchPlace(env *corevms.Environment, nam uint32) (uint32, error) {
	if nam == 0 {
		return 0, nil
	}

	return env.Memory().LoadLongword(env.CPU(), nam+namWCC)
}

// newScanBlock allocates a scan context, sets up its NAM, and writes its
// address to the longword at cell and to *block. Its status is
// LIB$_INSVIRMEM when the heap has no room; err is a memory error,
// already signaled.
func newScanBlock(env *corevms.Environment, cell uint32, block *uint32) (uint32, error) {
	addr, err := env.AllocateVM(scanBlockBytes, 0)
	if err != nil {
		return 0, err
	}

	if addr == 0 {
		return libInsVirMem, nil
	}

	if err := initScanBlock(env, addr, cell); err != nil {
		env.FreeVM(addr)

		return memoryFault(env, writing(err))
	}

	scanContexts(env)[addr] = true
	*block = addr

	return 0, nil
}

// initScanBlock clears a scan context and sets its NAM up as the $NAM
// macro would, its resultant string the block's buffer (empty), then
// writes the block's address to the longword at cell.
func initScanBlock(env *corevms.Environment, block, cell uint32) error {
	mem, cpu := env.Memory(), env.CPU()

	for off := uint32(0); off < scanBlockBytes; off += 4 {
		if err := mem.StoreLongword(cpu, block+off, 0); err != nil {
			return err
		}
	}

	for _, b := range []struct{ addr, v uint32 }{
		{block + scanNAM + namBID, namBIDValue},
		{block + scanNAM + namBLN, namBLNValue},
		{block + scanNAM + namRSS, maxFileSpec},
	} {
		if err := mem.StoreByte(cpu, b.addr, byte(b.v)); err != nil {
			return err
		}
	}

	if err := mem.StoreLongword(cpu, block+scanNAM+namRSA, block+scanRSA); err != nil {
		return err
	}

	return mem.StoreLongword(cpu, cell, block)
}

// scanRelated returns the program's NAM (FAB$L_NAM) and, when the scan
// context block holds a name and the NAM has no related NAM of its own,
// points the NAM's RLF at the context's NAM, returning related true: the
// caller puts RLF back to 0 when it's done.
func scanRelated(env *corevms.Environment, fab, block uint32) (nam uint32, related bool, err error) {
	mem, cpu := env.Memory(), env.CPU()

	if nam, err = mem.LoadLongword(cpu, fab+fabNAM); err != nil || nam == 0 || block == 0 {
		return nam, false, err
	}

	rsl, err := mem.LoadByte(cpu, block+scanNAM+namRSL)
	if err != nil || rsl == 0 {
		return nam, false, err
	}

	rlf, err := mem.LoadLongword(cpu, nam+namRLF)
	if err != nil || rlf != 0 {
		return nam, false, err
	}

	if err := mem.StoreLongword(cpu, nam+namRLF, block+scanNAM); err != nil {
		return nam, false, writing(err)
	}

	return nam, true, nil
}

// setScanName makes name (at most a file specification's length) the
// scan context's related name.
func setScanName(env *corevms.Environment, block uint32, name string) error {
	mem, cpu := env.Memory(), env.CPU()

	if len(name) > maxFileSpec {
		name = name[:maxFileSpec]
	}

	for i := range len(name) {
		if err := mem.StoreByte(cpu, block+scanRSA+uint32(i), name[i]); err != nil {
			return err
		}
	}

	return mem.StoreByte(cpu, block+scanNAM+namRSL, byte(len(name)))
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

	contexts := scanContexts(env)
	if !contexts[block] {
		return libInvArg, nil
	}

	delete(contexts, block)
	env.FreeVM(block)

	if err := mem.StoreLongword(cpu, cell, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// parseNull runs $PARSE on the program's FAB with its FNS and DNS 0, the
// null string, so that RMS frees any wildcard context its NAM names,
// then puts FNS and DNS back. It returns $PARSE's status, or RMS$_FAB
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

	fns, err := mem.LoadByte(cpu, fab+fabFNS)
	if err != nil {
		return memoryFault(env, err)
	}

	dns, err := mem.LoadByte(cpu, fab+fabDNS)
	if err != nil {
		return memoryFault(env, err)
	}

	sizes := func(f, d byte) error {
		if err := mem.StoreByte(cpu, fab+fabFNS, f); err != nil {
			return err
		}

		return mem.StoreByte(cpu, fab+fabDNS, d)
	}

	if err := sizes(0, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	status, err := env.CallService("SYS$PARSE", []uint32{fab})
	if err != nil {
		return status, err
	}

	if err := sizes(fns, dns); err != nil {
		return memoryFault(env, writing(err))
	}

	return status, nil
}
