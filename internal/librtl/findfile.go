package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// LIB$FIND_FILE and LIB$FIND_FILE_END (docs/PHASE-58, subtask 3; the
// survey's "FIND_FILE (155) and FIND_FILE_END (159)"), written from the
// 5.0 LIB$ manual (LIB-155, LIB-159, and section 2.7.4, "File Searching
// Routines"):
//
//	LIB$FIND_FILE filespec ,resultant-filespec ,context
//	              [,default-filespec] [,related-filespec]
//	              [,status-value] [,flags]
//	LIB$FIND_FILE_END context
//
// LIB$FIND_FILE returns one file a call: the next file a wildcard
// specification matches, until RMS$_NMF. It works as a VAX routine
// would, with RMS's own services. Its context, "the address of an
// internal FAB/NAM buffer", is a heap block (LIB$GET_VM's, so the image's
// end frees it) holding a real FAB and NAM, a second NAM that carries the
// related name (NAM$L_RLF), and their buffers; the routine calls $PARSE
// and $SEARCH on them (corevms.Environment.CallService), so RMS's name
// processing (default and related names, logical names, search lists)
// and its two kinds of wildcard context are $SEARCH's own. What it keeps
// besides, the specification being searched for and the last file found,
// is in Go, per image (corevms.Environment.ImageData), by the block's
// address, as the date and time contexts are (dtcontext.go).
//
// What the manuals leave open is as VMS 7.1 does it (testdata/lib58,
// docs/PHASE-58's subtask 8):
//
//   - A new search starts on the first call, when filespec's text isn't
//     the last call's (compared as given), and on every call with
//     LIB$M_FIL_NOWILD set. A specification with no wildcard (NAM$L_FNB's
//     NAM$V_WILDCARD clear after the $PARSE, a search list included) gets
//     one $SEARCH: the next call with it is RMS$_NMF, with nothing
//     written. RMS$_NMF ends the context: the block goes back to the
//     heap and the longword is cleared, so the call after starts the
//     search again; with LIB$M_FIL_MULTIPLE the context stays, for the
//     related name.
//   - The routine's success is $SEARCH's, RMS$_NORMAL, even when
//     resultant-filespec is too short for the name (no LIB$_STRTRU).
//   - When a service fails, resultant-filespec gets the NAM's resultant
//     string, else its expanded string, else (a $PARSE that failed) the
//     filespec as given.
//   - status-value gets FAB$L_STV after each $PARSE and after a $SEARCH
//     that fails.
//   - With LIB$M_FIL_MULTIPLE, related-filespec is ignored and the last
//     call's name (the file found, or what a failing call wrote to
//     resultant-filespec) is the related name of each new search:
//     temporary defaulting, "[SMITH]A,B" making B [SMITH]B.
//   - FNS and DNS are bytes, so a filespec of 256 characters is one of 0:
//     the length is taken modulo 256. Flag bits past MULTIPLE are
//     ignored; a context longword that holds no context's address is
//     RMS$_FAB. LIB$FIND_FILE_END's success is SS$_NORMAL.

// Statuses the file search routines return.
var (
	rmsNormal = vmsdef.Symbols["RMS$_NORMAL"]
	rmsFAB    = vmsdef.Symbols["RMS$_FAB"]
	libNoWild = vmsdef.LibrarySymbols["LIB$_NOWILD"]
)

// LIB$FIND_FILE's flags. The manuals give the bits, and govax's
// definitions have no LIB$M_FIL_ symbols (the survey's "Codes").
const (
	findNoWild   = 1 << 0 // LIB$M_FIL_NOWILD: a wildcard is LIB$_NOWILD
	findMultiple = 1 << 1 // LIB$M_FIL_MULTIPLE: temporary defaulting
)

// The FAB and NAM fields the routines set and read.
var (
	fabBID = vmsdef.Symbols["FAB$B_BID"]
	fabBLN = vmsdef.Symbols["FAB$B_BLN"]
	fabNAM = vmsdef.Symbols["FAB$L_NAM"]
	fabFNA = vmsdef.Symbols["FAB$L_FNA"]
	fabFNS = vmsdef.Symbols["FAB$B_FNS"]
	fabDNA = vmsdef.Symbols["FAB$L_DNA"]
	fabDNS = vmsdef.Symbols["FAB$B_DNS"]
	fabSTS = vmsdef.Symbols["FAB$L_STS"]
	fabSTV = vmsdef.Symbols["FAB$L_STV"]
	namBID = vmsdef.Symbols["NAM$B_BID"]
	namBLN = vmsdef.Symbols["NAM$B_BLN"]
	namESA = vmsdef.Symbols["NAM$L_ESA"]
	namESS = vmsdef.Symbols["NAM$B_ESS"]
	namRSA = vmsdef.Symbols["NAM$L_RSA"]
	namRSS = vmsdef.Symbols["NAM$B_RSS"]
	namRSL = vmsdef.Symbols["NAM$B_RSL"]
	namRLF = vmsdef.Symbols["NAM$L_RLF"]
	namFNB = vmsdef.Symbols["NAM$L_FNB"]

	fabBIDValue = vmsdef.Symbols["FAB$C_BID"]
	fabBLNValue = vmsdef.Symbols["FAB$C_BLN"]
	namBIDValue = vmsdef.Symbols["NAM$C_BID"]
	namBLNValue = vmsdef.Symbols["NAM$C_BLN"]
	fnbWildcard = vmsdef.Symbols["NAM$M_WILDCARD"]
)

// maxFileSpec is the longest file specification a FAB's FNS or DNS, or a
// NAM's ESS or RSS, can hold.
const maxFileSpec = 255

// fileBuffer is a buffer's size in a search block: a file specification,
// rounded up to a longword.
const fileBuffer = 256

// A search block's layout: the FAB, the NAM, the related NAM, then the
// buffers (expanded and resultant strings, the related name's resultant
// string, and the file and default names).
const (
	findFAB        = 0
	findNAM        = findFAB + 80
	findRelNAM     = findNAM + 96
	findESA        = findRelNAM + 96
	findRSA        = findESA + fileBuffer
	findRelRSA     = findRSA + fileBuffer
	findFileName   = findRelRSA + fileBuffer
	findDefault    = findFileName + fileBuffer
	findBlockBytes = findDefault + fileBuffer
)

// findContext is what LIB$FIND_FILE keeps in Go for one context: the
// filespec being searched for (searching false before a search has
// started, or after its $PARSE failed), whether it has a wildcard and
// has been searched once, and the last call's name, the related name of
// the next search with LIB$M_FIL_MULTIPLE.
type findContext struct {
	spec           string
	searching      bool
	wild, searched bool
	previous       string
}

// findContextsKey is the corevms.Environment.ImageData key of the
// image's find contexts, by their blocks' addresses.
type findContextsKey struct{}

// findContexts returns the running image's find contexts.
func findContexts(env *corevms.Environment) map[uint32]*findContext {
	return env.ImageData(findContextsKey{}, func() any {
		return map[uint32]*findContext{}
	}).(map[uint32]*findContext)
}

// findArgs are LIB$FIND_FILE's arguments, read.
type findArgs struct {
	spec, defaultSpec, related string
	result, cell, statusValue  uint32
	flags                      uint32
}

// libFindFile is LIB$FIND_FILE.
func libFindFile(env *corevms.Environment, argv []uint32) (uint32, error) {
	a, status, err := readFindArgs(env, argv)
	if err != nil || status != 0 {
		return status, err
	}

	mem, cpu := env.Memory(), env.CPU()

	block, err := mem.LoadLongword(cpu, a.cell)
	if err != nil {
		return memoryFault(env, err)
	}

	contexts := findContexts(env)

	var c *findContext

	if block == 0 {
		if block, status, err = newSearchBlock(env, a.cell); err != nil || status != 0 {
			return status, err
		}

		c = &findContext{}
		contexts[block] = c
	} else if c = contexts[block]; c == nil {
		return rmsFAB, nil
	}

	multiple := a.flags&findMultiple != 0

	switch {
	case !c.searching || a.flags&findNoWild != 0 || a.spec != c.spec:
		related := a.related
		if multiple {
			related = c.previous
		}

		status, err := startSearch(env, block, a, related)
		if err != nil {
			return memoryFault(env, err)
		}

		if err := copySTV(env, block, a.statusValue); err != nil {
			return memoryFault(env, err)
		}

		c.spec, c.searching, c.searched = a.spec, status&1 != 0, false
		if status&1 == 0 {
			return failedSearch(env, c, block, a, status)
		}

		fnb, err := mem.LoadLongword(cpu, block+findNAM+namFNB)
		if err != nil {
			return memoryFault(env, err)
		}

		c.wild = fnb&fnbWildcard != 0

		if a.flags&findNoWild != 0 && c.wild {
			return failedSearch(env, c, block, a, libNoWild)
		}

	case !c.wild && c.searched:
		// The one file a specification without a wildcard names has been
		// given (or wasn't there).
		return endSearch(env, c, block, a, rmsNMF)
	}

	status, err = env.CallService("SYS$SEARCH", []uint32{block + findFAB})
	if err != nil {
		return status, err
	}

	c.searched = true

	if status&1 == 0 {
		if err := copySTV(env, block, a.statusValue); err != nil {
			return memoryFault(env, err)
		}

		return failedSearch(env, c, block, a, status)
	}

	name, err := resultantName(env, block+findNAM)
	if err != nil {
		return memoryFault(env, err)
	}

	c.previous = name

	if _, _, err := writeString(env, a.result, name); err != nil {
		return stringFailure(env, err)
	}

	return status, nil
}

// failedSearch is the end of a call whose $PARSE or $SEARCH failed with
// status (or LIB$_NOWILD): resultant-filespec gets the NAM's resultant
// string, else its expanded string, else the filespec, which is also the next MULTIPLE
// search's related name, and RMS$_NMF ends the context.
func failedSearch(env *corevms.Environment, c *findContext, block uint32, a findArgs, status uint32) (uint32, error) {
	name, err := resultantName(env, block+findNAM)
	if err == nil && name == "" {
		name, err = expandedString(env, block+findNAM)
	}

	if err != nil {
		return memoryFault(env, err)
	}

	if name == "" {
		name = a.spec
	}

	c.previous = name

	if name != "" {
		if _, _, err := writeString(env, a.result, name); err != nil {
			return stringFailure(env, err)
		}
	}

	if status == rmsNMF {
		return endSearch(env, c, block, a, status)
	}

	return status, nil
}

// endSearch returns status (RMS$_NMF) after ending the context, unless
// LIB$M_FIL_MULTIPLE keeps it for the related name.
func endSearch(env *corevms.Environment, c *findContext, block uint32, a findArgs, status uint32) (uint32, error) {
	if a.flags&findMultiple != 0 {
		return status, nil
	}

	if _, err := endFindContext(env, block, a.cell); err != nil {
		return 0, err
	}

	return status, nil
}

// copySTV copies the block's FAB$L_STV to the longword at statusValue,
// when given. err is a memory error, for the caller to signal.
func copySTV(env *corevms.Environment, block, statusValue uint32) error {
	if statusValue == 0 {
		return nil
	}

	mem, cpu := env.Memory(), env.CPU()

	stv, err := mem.LoadLongword(cpu, block+findFAB+fabSTV)
	if err != nil {
		return err
	}

	return writing(mem.StoreLongword(cpu, statusValue, stv))
}

// expandedString reads the expanded string of the NAM at nam.
func expandedString(env *corevms.Environment, nam uint32) (string, error) {
	mem, cpu := env.Memory(), env.CPU()

	esl, err := mem.LoadByte(cpu, nam+namESL)
	if err != nil {
		return "", err
	}

	esa, err := mem.LoadLongword(cpu, nam+namESA)
	if err != nil {
		return "", err
	}

	return readText(env, esa, uint32(esl))
}

// readFindArgs reads LIB$FIND_FILE's arguments. status is nonzero for
// arguments the routine refuses (LIB$_INVSTRDES, LIB$_INVFILSPE), and
// err a memory error, already signaled.
func readFindArgs(env *corevms.Environment, argv []uint32) (a findArgs, status uint32, err error) {
	a = findArgs{result: arg(argv, 1), cell: arg(argv, 2), statusValue: arg(argv, 5)}

	for _, s := range []struct {
		i   int
		out *string
	}{{0, &a.spec}, {3, &a.defaultSpec}, {4, &a.related}} {
		addr := arg(argv, s.i)
		if addr == 0 && s.i > 0 {
			continue
		}

		if *s.out, err = readString(env, addr); err != nil {
			status, err = stringFailure(env, err)

			return a, status, err
		}
	}

	// FNS and DNS are bytes: what's past a multiple of 256 is all RMS sees.
	a.spec = a.spec[:len(a.spec)%256]
	a.defaultSpec = a.defaultSpec[:len(a.defaultSpec)%256]

	if a.flags, err = optionalLongword(env, arg(argv, 6), 0); err != nil {
		status, err = memoryFault(env, err)
	}

	return a, status, err
}

// newSearchBlock allocates a search block, makes its FAB and NAMs, and
// writes its address to the longword at cell. Its status is
// LIB$_INSVIRMEM when the heap has no room; err is a memory error,
// already signaled.
func newSearchBlock(env *corevms.Environment, cell uint32) (block, status uint32, err error) {
	block, err = env.AllocateVM(findBlockBytes)
	if err != nil {
		return 0, 0, err
	}

	if block == 0 {
		return 0, libInsVirMem, nil
	}

	if err := initSearchBlock(env, block, cell); err != nil {
		env.FreeVM(block)
		status, err = memoryFault(env, writing(err))

		return 0, status, err
	}

	return block, 0, nil
}

// initSearchBlock sets a search block up (layoutSearchBlock), then
// writes its address to the longword at cell.
func initSearchBlock(env *corevms.Environment, block, cell uint32) error {
	if err := layoutSearchBlock(env, block); err != nil {
		return err
	}

	return env.Memory().StoreLongword(env.CPU(), cell, block)
}

// layoutSearchBlock clears a search block and sets it up as the $FAB and
// $NAM macros would: the FAB's NAM is the block's, whose expanded and
// resultant strings are the block's buffers, and the related NAM's
// resultant string is the block's third buffer.
func layoutSearchBlock(env *corevms.Environment, block uint32) error {
	mem, cpu := env.Memory(), env.CPU()

	for off := uint32(0); off < findBlockBytes; off += 4 {
		if err := mem.StoreLongword(cpu, block+off, 0); err != nil {
			return err
		}
	}

	bytes := []struct{ addr, v uint32 }{
		{block + findFAB + fabBID, fabBIDValue},
		{block + findFAB + fabBLN, fabBLNValue},
		{block + findNAM + namBID, namBIDValue},
		{block + findNAM + namBLN, namBLNValue},
		{block + findNAM + namESS, maxFileSpec},
		{block + findNAM + namRSS, maxFileSpec},
		{block + findRelNAM + namBID, namBIDValue},
		{block + findRelNAM + namBLN, namBLNValue},
		{block + findRelNAM + namRSS, maxFileSpec},
	}

	for _, b := range bytes {
		if err := mem.StoreByte(cpu, b.addr, byte(b.v)); err != nil {
			return err
		}
	}

	longs := []struct{ addr, v uint32 }{
		{block + findFAB + fabNAM, block + findNAM},
		{block + findFAB + fabFNA, block + findFileName},
		{block + findFAB + fabDNA, block + findDefault},
		{block + findNAM + namESA, block + findESA},
		{block + findNAM + namRSA, block + findRSA},
		{block + findRelNAM + namRSA, block + findRelRSA},
	}

	for _, l := range longs {
		if err := mem.StoreLongword(cpu, l.addr, l.v); err != nil {
			return err
		}
	}

	return nil
}

// startSearch puts a's file and default names, and related as the
// related name, into the block's FAB and NAMs and calls $PARSE, returning
// its status. err is a memory error, for the caller to signal.
func startSearch(env *corevms.Environment, block uint32, a findArgs, related string) (uint32, error) {
	if len(related) > maxFileSpec {
		related = related[:maxFileSpec]
	}

	if err := setBlockNames(env, block, a.spec, a.defaultSpec, related); err != nil {
		return 0, writing(err)
	}

	return env.CallService("SYS$PARSE", []uint32{block + findFAB})
}

// setBlockNames writes the file, default, and related names into a
// search block, with their lengths in FNS, DNS, and the related NAM's
// RSL; the NAM's RLF points at the related NAM only when there is a
// related name.
func setBlockNames(env *corevms.Environment, block uint32, spec, defaultSpec, related string) error {
	mem, cpu := env.Memory(), env.CPU()

	for _, s := range []struct {
		buf, length uint32
		text        string
	}{
		{block + findFileName, block + findFAB + fabFNS, spec},
		{block + findDefault, block + findFAB + fabDNS, defaultSpec},
		{block + findRelRSA, block + findRelNAM + namRSL, related},
	} {
		for i := range len(s.text) {
			if err := mem.StoreByte(cpu, s.buf+uint32(i), s.text[i]); err != nil {
				return err
			}
		}

		if err := mem.StoreByte(cpu, s.length, byte(len(s.text))); err != nil {
			return err
		}
	}

	rlf := uint32(0)
	if related != "" {
		rlf = block + findRelNAM
	}

	return mem.StoreLongword(cpu, block+findNAM+namRLF, rlf)
}

// resultantName reads the resultant string of the NAM at nam.
func resultantName(env *corevms.Environment, nam uint32) (string, error) {
	mem, cpu := env.Memory(), env.CPU()

	rsl, err := mem.LoadByte(cpu, nam+namRSL)
	if err != nil {
		return "", err
	}

	rsa, err := mem.LoadLongword(cpu, nam+namRSA)
	if err != nil {
		return "", err
	}

	return readText(env, rsa, uint32(rsl))
}

// libFindFileEnd is LIB$FIND_FILE_END. It ends the search the context
// longword names (endFindContext). A longword of 0 is nothing to end; one
// that names no context is RMS$_FAB. Its success is SS$_NORMAL (the VSI
// manual's, and VMS 7.1's; the 5.0 manual says RMS$_NORMAL).
func libFindFileEnd(env *corevms.Environment, argv []uint32) (uint32, error) {
	cell := arg(argv, 0)

	block, err := env.Memory().LoadLongword(env.CPU(), cell)
	if err != nil {
		return memoryFault(env, err)
	}

	if block == 0 {
		return ssNormal, nil
	}

	if findContexts(env)[block] == nil {
		return rmsFAB, nil
	}

	return endFindContext(env, block, cell)
}

// endFindContext ends the context in block: a $PARSE of the null string
// frees RMS's search context ("parses the null string to deallocate any
// saved RMS context"), then the block goes back to the heap and the
// longword at cell is cleared (as VMS 7.1 does).
func endFindContext(env *corevms.Environment, block, cell uint32) (uint32, error) {
	if err := setBlockNames(env, block, "", "", ""); err != nil {
		return memoryFault(env, writing(err))
	}

	if _, err := env.CallService("SYS$PARSE", []uint32{block + findFAB}); err != nil {
		return 0, err
	}

	delete(findContexts(env), block)
	env.FreeVM(block)

	if err := env.Memory().StoreLongword(env.CPU(), cell, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}
