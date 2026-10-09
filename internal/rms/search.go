package rms

import (
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// This file is $SEARCH (docs/PHASE-33.md, subtask 3), written from the RMS
// Reference Manual and checked against VMS 7.3 (testdata/mar/rms3, the
// SEARCH probe). VMS searches in one of two ways:
//
//   - A specification whose directory has no wildcard, and isn't a search
//     list, keeps no context. NAM$L_WCC holds the position, counting from
//     1, of the directory entry last returned; each $SEARCH starts again
//     from the expanded string and the directory NAM$W_DID names, and
//     returns the next entry past that position that matches.
//   - A wildcard directory ("[*]", "[TEST...]") or a search list keeps a
//     context, which $PARSE makes. WCC is ^X10000 plus its number, the
//     lowest from 2 not in use; it's freed when the search runs out.
//
// When nothing matched at all, the status is RMS$_FNF; after the last
// match, RMS$_NMF. Either way WCC becomes ^X40000000, and the resultant
// string is the expanded string.

// wccDone is NAM$L_WCC once a search has run out.
const wccDone = 0x40000000

// wccContext marks a WCC that names a context.
const wccContext = 0x00010000

// fnbDirLvlsMask is NAM$V_DIR_LVLS's three bits.
var fnbDirLvlsMask = uint32(7) << fnbDirLvls

// searchState is one wildcard context: the parse's expanded names, and
// how far $SEARCH has got through them.
type searchState struct {
	// ID is the context's number.
	ID uint32

	// Names is the parse's expanded names, one per search list element,
	// and Elem the one being searched.
	Names []parsedName
	Elem  int

	// Dirs is the current element's directories, nil before it's been
	// started, and Dir the next one to look in. Last is the entry the
	// search last returned from it (HaveLast false: none yet); the search
	// goes on with the entry after it in directory order, not by its
	// place in the list, since the directory may have changed between
	// calls (Phase 47: another process creating or deleting files in it).
	Dirs     []searchDir
	Dir      int
	Last     ondisk.DirEntry
	HaveLast bool

	// Visited is the directories (by path) the current element's search
	// has finished. The walk is made again as each directory is
	// finished (nextDir), so that a subdirectory made meanwhile, ahead
	// of the search's place, is searched too, as VMS's walk, which
	// reads each directory as it reaches it, would find it.
	Visited map[string]bool

	// Found is whether any file has been returned.
	Found bool
}

// saveSearch makes a context for names, numbering it in NAM$L_WCC.
func (ctx *Context) saveSearch(nam uint32, names []parsedName) error {
	t := ctx.Files
	if t == nil {
		return nil
	}

	if t.searches == nil {
		t.searches = map[uint32]*searchState{}
	}

	id := uint32(2)
	for t.searches[id] != nil {
		id++
	}

	t.searches[id] = &searchState{ID: id, Names: names}

	return ctx.storeLongword(nam+namWCC, wccContext|id)
}

// searchDir is one directory a search looks in: its path, the directory,
// and which of the path's levels came from a wildcard or an ellipsis;
// Vol is the volume it's on.
type searchDir struct {
	Vol  *volume.Volume
	Path []string
	Dir  *volume.Directory
	Wild []bool
}

// current lists d's entries as the directory has them now: it opens the
// directory again by its file ID, since a wildcard context keeps d across
// calls, and Dir's header and map are those of when the search began.
func (d *searchDir) current() ([]ondisk.DirEntry, error) {
	if d.Vol == nil {
		return d.Dir.List()
	}

	dir, err := d.Vol.OpenDirectory(d.Dir.Header.Fid)
	if err != nil {
		return nil, err
	}

	d.Dir = dir

	return dir.List()
}

// entriesAfter is the index of the first of entries (in directory order:
// names ascending, each name's versions descending) that comes after
// last.
func entriesAfter(entries []ondisk.DirEntry, last ondisk.DirEntry) int {
	return sort.Search(len(entries), func(i int) bool {
		e := entries[i]
		if c := strings.Compare(e.Name, last.Name); c != 0 {
			return c > 0
		}

		return e.Version < last.Version
	})
}

// searchDirs returns the directories a search of d looks in, in the
// order VMS looks in them: depth first, each directory's subdirectories
// in directory order.
func searchDirs(vol *volume.Volume, d dirSpec) []searchDir {
	mfd, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		return nil
	}

	elems := d.Elems
	if len(elems) > 0 && elems[0] == "000000" {
		elems = elems[1:]
	}

	return walkSearch(vol, searchDir{Vol: vol, Dir: mfd}, elems)
}

// walkSearch returns the directories below node that elems lead to.
func walkSearch(vol *volume.Volume, node searchDir, elems []string) []searchDir {
	if len(elems) == 0 {
		return []searchDir{node}
	}

	e, rest := elems[0], elems[1:]

	var out []searchDir

	switch {
	case e == ellipsis:
		for _, below := range descendants(vol, node) {
			out = append(out, walkSearch(vol, below, rest)...)
		}

	default:
		wild := strings.ContainsAny(e, "*%")

		for _, child := range subdirectories(vol, node) {
			if wildMatch(e, child.Path[len(child.Path)-1]) {
				child.Wild[len(child.Wild)-1] = wild
				out = append(out, walkSearch(vol, child, rest)...)
			}
		}
	}

	return out
}

// descendants returns node and every directory below it, depth first,
// the levels below node marked as wild.
func descendants(vol *volume.Volume, node searchDir) []searchDir {
	out := []searchDir{node}

	for _, child := range subdirectories(vol, node) {
		child.Wild[len(child.Wild)-1] = true
		out = append(out, descendants(vol, child)...)
	}

	return out
}

// subdirectories returns node's subdirectories in directory order: the
// highest version of each .DIR entry, but not the MFD's entry for itself.
func subdirectories(vol *volume.Volume, node searchDir) []searchDir {
	entries, err := node.Dir.List()
	if err != nil {
		return nil
	}

	out := make([]searchDir, 0)

	last := ""

	for _, e := range entries {
		name, typ := splitEntryName(e.Name)
		if typ != "DIR" || name == last || e.Fid.Equal(node.Dir.Header.Fid) {
			continue
		}

		last = name

		dir, err := vol.OpenDirectory(e.Fid)
		if err != nil {
			continue
		}

		out = append(out, searchDir{
			Vol:  vol,
			Path: append(append([]string{}, node.Path...), name),
			Dir:  dir,
			Wild: append(append([]bool{}, node.Wild...), false),
		})
	}

	return out
}

// wildMatch reports whether s matches pattern, where "*" matches any run
// of characters and "%" any one.
func wildMatch(pattern, s string) bool {
	for pattern != "" {
		switch pattern[0] {
		case '*':
			for i := len(s); i >= 0; i-- {
				if wildMatch(pattern[1:], s[i:]) {
					return true
				}
			}

			return false

		case '%':
			if s == "" {
				return false
			}

		default:
			if s == "" || s[0] != pattern[0] {
				return false
			}
		}

		pattern, s = pattern[1:], s[1:]
	}

	return s == ""
}

// selected reports which of a directory's entries p matches, by name,
// type, and version: none or 0 is a name's highest version, n that
// version, -n the one n below the highest, -0 the lowest, and * all.
func selected(entries []ondisk.DirEntry, p parsedName) []bool {
	out := make([]bool, len(entries))
	name, typ := p.Name, strings.TrimPrefix(p.Type, ".")
	ver := strings.TrimPrefix(p.Ver, ";")
	n, _ := strconv.Atoi(ver)

	for i := 0; i < len(entries); {
		j := i
		for j < len(entries) && entries[j].Name == entries[i].Name {
			j++
		}

		en, et := splitEntryName(entries[i].Name)

		if wildMatch(name, en) && wildMatch(typ, et) {
			switch {
			case ver == "*":
				for k := i; k < j; k++ {
					out[k] = true
				}

			case ver == "-0":
				out[j-1] = true

			case n > 0:
				for k := i; k < j; k++ {
					out[k] = int(entries[k].Version) == n
				}

			case i-n < j:
				out[i-n] = true
			}
		}

		i = j
	}

	return out
}

// SysSearch implements SYS$SEARCH: it returns the next file the NAM's
// parsed specification matches, as described at the top of this file.
// Each file found is reported in the NAM: the resultant string in RSA
// (RMS$_RSS if it doesn't fit) and its length in RSL, the component
// pointers into it, and FID and DID.
func SysSearch(ctx *Context, argv []uint32) (uint32, error) {
	if len(argv) < 1 {
		return ssInsufficientArgs, nil
	}

	fab := argv[0]

	if sts, err := checkFAB(ctx, fab); err != nil || sts != 0 {
		return sts, err
	}

	ifi, err := ctx.loadWord(fab + fabIFI)
	if err != nil {
		return 0, err
	}

	if ifi != 0 {
		return fabStatus(ctx, fab, rmsInvalidIFI, 0)
	}

	nam, sts, err := fabNAMBlock(ctx, fab)
	if err != nil {
		return 0, err
	}

	if sts == 0 && nam == 0 {
		sts = rmsInvalidNAM
	}

	if sts != 0 {
		return fabStatus(ctx, fab, sts, 0)
	}

	wcc, err := ctx.loadLongword(nam + namWCC)
	if err != nil {
		return 0, err
	}

	var stv uint32

	if wcc&0xffff0000 == wccContext {
		st := ctx.Files.searches[wcc&0xffff]
		if st == nil {
			return fabStatus(ctx, fab, rmsInvalidWCC, 0)
		}

		sts, stv, err = ctx.searchContext(nam, st)
	} else {
		sts, stv, err = ctx.searchPlain(nam, wcc)
	}

	if err != nil {
		return 0, err
	}

	return fabStatus(ctx, fab, sts, stv)
}

// searchPlain is a search with no context: the next entry past position
// wcc of the directory the NAM's DID names that matches the expanded
// string.
func (ctx *Context) searchPlain(nam, wcc uint32) (sts, stv uint32, err error) {
	esl, err := ctx.loadByte(nam + namESL)
	if err != nil {
		return 0, 0, err
	}

	esa, err := ctx.loadLongword(nam + namESA)
	if err != nil {
		return 0, 0, err
	}

	if esl == 0 || esa == 0 {
		return rmsInvalidESL, 0, nil
	}

	text, err := ctx.loadFixedString(esa, int(esl))
	if err != nil {
		return 0, 0, err
	}

	names, sts := ctx.expandName(nameInputs{Primary: text})
	if sts != 0 {
		return sts, 0, nil
	}

	p := names[0]

	vol, ok := ctx.Mounts.Lookup(p.Lookup)
	if !ok {
		return rmsDeviceError, ssNoSuchDevice, nil
	}

	did, err := ctx.loadFid(nam + namDID)
	if err != nil {
		return 0, 0, err
	}

	var dir *volume.Directory

	if did.IsZero() {
		dirs := searchDirs(vol, p.DirSpec)
		if len(dirs) > 0 {
			dir = dirs[0].Dir
		}
	} else if dir, err = vol.OpenDirectory(did); err != nil {
		dir = nil
	}

	if dir == nil {
		return rmsDirNotFound, ssNoSuchFile, nil
	}

	if wcc != wccDone {
		entries, err := dir.List()
		if err != nil {
			return rmsDeviceError, 0, nil
		}

		sel := selected(entries, p)

		for i := int(wcc); i < len(entries); i++ {
			if !sel[i] {
				continue
			}

			if err := ctx.storeLongword(nam+namWCC, uint32(i+1)); err != nil {
				return 0, 0, err
			}

			return ctx.reportMatch(nam, p.Dev+p.Dir, entries[i], dir.Header.Fid)
		}
	}

	if err := ctx.endSearch(nam, p); err != nil {
		return 0, 0, err
	}

	if wcc == 0 {
		return rmsFileNotFound, ssNoSuchFile, nil
	}

	return rmsNoMoreFiles, ssNoMoreFiles, nil
}

// searchContext continues a context's search.
func (ctx *Context) searchContext(nam uint32, st *searchState) (sts, stv uint32, err error) {
	for st.Elem < len(st.Names) {
		p := st.Names[st.Elem]

		if st.Dirs == nil {
			vol, ok := ctx.Mounts.Lookup(p.Lookup)
			if ok {
				st.Dirs = searchDirs(vol, p.DirSpec)
			}

			if st.Dirs == nil {
				st.Dirs = []searchDir{}
			}

			st.Dir, st.HaveLast, st.Visited = 0, false, map[string]bool{}

			// A later search list element's expanded string replaces
			// the first's (the oracle's SEARCH case 6).
			if st.Elem > 0 {
				if err := ctx.storeExpanded(nam, p); err != nil {
					return 0, 0, err
				}
			}
		}

		for st.Dir < len(st.Dirs) {
			d := &st.Dirs[st.Dir]

			// The directory as it is now: its header and map read
			// again, in case it has grown or moved since the last call.
			entries, err := d.current()
			if err != nil {
				entries = nil
			}

			sel := selected(entries, p)

			start := 0
			if st.HaveLast {
				start = entriesAfter(entries, st.Last)
			}

			for i := start; i < len(entries); i++ {
				if !sel[i] {
					continue
				}

				st.Last, st.HaveLast = entries[i], true
				st.Found = true

				if err := ctx.storeLongword(nam+namFNB, matchFNB(p.FNB, *d)); err != nil {
					return 0, 0, err
				}

				return ctx.reportMatch(nam, p.Dev+dirSpec{Elems: d.Path}.String(), entries[i], d.Dir.Header.Fid)
			}

			st.HaveLast = false
			ctx.nextDir(st, p)
		}

		st.Elem++
		st.Dirs = nil
	}

	delete(ctx.Files.searches, st.ID)

	last := st.Names[len(st.Names)-1]
	if err := ctx.endSearch(nam, last); err != nil {
		return 0, 0, err
	}

	if err := ctx.storeFid(nam+namFID, ondisk.Fid{}); err != nil {
		return 0, 0, err
	}

	stv = 0

	if last.FNB&fnbSearchList != 0 {
		// A search list's search clears the device and directory too.
		stv = ssNoMoreFiles

		// Only the count: the name's characters stay (the oracle's
		// SEARCH case 6).
		if err := ctx.storeByte(nam+namDVI, 0); err != nil {
			return 0, 0, err
		}

		if err := ctx.storeFid(nam+namDID, ondisk.Fid{}); err != nil {
			return 0, 0, err
		}
	}

	if !st.Found {
		return rmsFileNotFound, ssNoSuchFile, nil
	}

	return rmsNoMoreFiles, stv, nil
}

// nextDir moves st's search to its next directory once the current one
// is finished: the first not yet searched after the current one in the
// walk as the tree is now (searchDirs again), so a directory made since
// the search began is searched if it comes later in the walk, and one
// made behind the search's place isn't. If the current directory has
// gone from the tree, the search goes on through the walk it had.
func (ctx *Context) nextDir(st *searchState, p parsedName) {
	cur := st.Dirs[st.Dir].key()
	st.Visited[cur] = true
	st.Dir++

	if vol, ok := ctx.Mounts.Lookup(p.Lookup); ok {
		fresh := searchDirs(vol, p.DirSpec)

		for i, d := range fresh {
			if d.key() == cur {
				st.Dirs, st.Dir = fresh, i+1

				break
			}
		}
	}

	for st.Dir < len(st.Dirs) && st.Visited[st.Dirs[st.Dir].key()] {
		st.Dir++
	}
}

// key names d for a search's list of directories visited: its path.
func (d *searchDir) key() string { return strings.Join(d.Path, ".") }

// matchFNB is FNB for a file found in d: the parse's bits with the
// directory's level count, and the wildcard bit of each level a
// wildcard or an ellipsis gave.
func matchFNB(fnb uint32, d searchDir) uint32 {
	fnb &^= fnbDirLvlsMask

	if len(d.Path) > 1 {
		fnb |= uint32(min(len(d.Path)-1, 7)) << fnbDirLvls
	}

	for level, wild := range d.Wild {
		switch {
		case !wild:
		case level == 0:
			fnb |= fnbWildUFD
		case level <= 7:
			fnb |= fnbWildSFD1 << (level - 1)
		}
	}

	return fnb
}

// storeExpanded writes p as the NAM's expanded string, its components,
// and FNB.
func (ctx *Context) storeExpanded(nam uint32, p parsedName) error {
	ess, err := ctx.loadByte(nam + namESS)
	if err != nil {
		return err
	}

	esa, err := ctx.loadLongword(nam + namESA)
	if err != nil {
		return err
	}

	if _, err := ctx.storeNameString(nam, esa, ess, namESL, p.fileName); err != nil {
		return err
	}

	return ctx.storeLongword(nam+namFNB, p.FNB)
}

// expandedOrESS writes p as the NAM's expanded string and FNB, returning
// RMS$_ESS when it doesn't fit (FNB then left clear, as $PARSE leaves
// it).
func (ctx *Context) expandedOrESS(nam uint32, p parsedName) (uint32, error) {
	ess, err := ctx.loadByte(nam + namESS)
	if err != nil {
		return 0, err
	}

	esa, err := ctx.loadLongword(nam + namESA)
	if err != nil {
		return 0, err
	}

	overflow, err := ctx.storeNameString(nam, esa, ess, namESL, p.fileName)
	if err != nil || overflow {
		return rmsESSError, err
	}

	return 0, ctx.storeLongword(nam+namFNB, p.FNB)
}

// endSearch writes what a search that has run out leaves in the NAM: the
// expanded string p as the resultant string, and WCC's "done" value.
func (ctx *Context) endSearch(nam uint32, p parsedName) error {
	rss, err := ctx.loadByte(nam + namRSS)
	if err != nil {
		return err
	}

	rsa, err := ctx.loadLongword(nam + namRSA)
	if err != nil {
		return err
	}

	if _, err := ctx.storeNameString(nam, rsa, rss, namRSL, p.fileName); err != nil {
		return err
	}

	return ctx.storeLongword(nam+namWCC, wccDone)
}

// reportMatch writes a file $SEARCH found into nam: the resultant string
// (where is its device and directory) and its components, FID, and DID.
func (ctx *Context) reportMatch(nam uint32, where string, e ondisk.DirEntry, did ondisk.Fid) (sts, stv uint32, err error) {
	dev, dir, _ := strings.Cut(where, "[")
	name, typ := splitEntryName(e.Name)

	f := fileName{
		Dev:  dev,
		Dir:  "[" + dir,
		Name: name,
		Type: "." + typ,
		Ver:  ";" + strconv.Itoa(int(e.Version)),
	}

	rss, err := ctx.loadByte(nam + namRSS)
	if err != nil {
		return 0, 0, err
	}

	rsa, err := ctx.loadLongword(nam + namRSA)
	if err != nil {
		return 0, 0, err
	}

	overflow, err := ctx.storeNameString(nam, rsa, rss, namRSL, f)
	if err != nil {
		return 0, 0, err
	}

	if overflow {
		return rmsRSSError, 0, nil
	}

	if err := ctx.storeFid(nam+namFID, e.Fid); err != nil {
		return 0, 0, err
	}

	if err := ctx.storeFid(nam+namDID, did); err != nil {
		return 0, 0, err
	}

	return rmsNormal, 0, nil
}
