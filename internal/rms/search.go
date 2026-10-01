package rms

import (
	"strconv"
	"strings"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// searchState is one NAM block's wildcard context: what $PARSE found and
// how far $SEARCH has got through it (docs/PHASE-33.md, subtask 3).
type searchState struct {
	// WCC is the context number in the NAM's NAM$L_WCC.
	WCC uint32

	// Names is the parse's expanded names, one per search list element,
	// and Elem the one being searched.
	Names []parsedName
	Elem  int

	// Matches is the current element's matching files, or nil before
	// it's been searched; Next is the next one to return.
	Matches []filespec.Match
	Next    int

	// Found is whether any file has been returned, and Done whether the
	// search has run out.
	Found, Done bool
}

// saveSearch keeps names as nam's wildcard context, replacing any it had,
// and stores the context's number in NAM$L_WCC.
func (ctx *Context) saveSearch(nam uint32, names []parsedName) error {
	t := ctx.Files
	if t == nil {
		return nil
	}

	if t.searches == nil {
		t.searches = map[uint32]*searchState{}
	}

	t.lastWCC++
	t.searches[nam] = &searchState{WCC: t.lastWCC, Names: names}

	return ctx.storeLongword(nam+namWCC, t.lastWCC)
}

// SysSearch implements SYS$SEARCH (docs/PHASE-33.md, subtask 3): it
// returns the next file the NAM's parsed specification matches.
//
// From the RMS Reference Manual's $SEARCH description. It continues the
// wildcard context $PARSE left (NAM$L_WCC); with none, it searches for the
// expanded string in ESA/ESL. Each file found is reported in the NAM: the
// resultant string in RSA (when RSA and RSS are both nonzero, RMS$_RSS if
// it doesn't fit) and its length in RSL, the component pointers into it,
// and FID and DID. A search list's elements are searched in turn. When
// no file matched at all the status is RMS$_FNF; after the last match,
// RMS$_NMF.
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

	st, sts, err := ctx.searchState(nam)
	if err != nil {
		return 0, err
	}

	if sts != 0 {
		return fabStatus(ctx, fab, sts, 0)
	}

	sts, stv, err := ctx.searchNext(nam, st)
	if err != nil {
		return 0, err
	}

	return fabStatus(ctx, fab, sts, stv)
}

// searchState returns nam's wildcard context: $PARSE's, when NAM$L_WCC
// still names it, or a new one for the expanded string otherwise
// (RMS$_ESA when there's none).
func (ctx *Context) searchState(nam uint32) (*searchState, uint32, error) {
	wcc, err := ctx.loadLongword(nam + namWCC)
	if err != nil {
		return nil, 0, err
	}

	if st, ok := ctx.Files.searches[nam]; ok && wcc != 0 && st.WCC == wcc {
		return st, 0, nil
	}

	esl, err := ctx.loadByte(nam + namESL)
	if err != nil {
		return nil, 0, err
	}

	esa, err := ctx.loadLongword(nam + namESA)
	if err != nil {
		return nil, 0, err
	}

	if esl == 0 || esa == 0 {
		return nil, rmsESAError, nil
	}

	text, err := ctx.loadFixedString(esa, int(esl))
	if err != nil {
		return nil, 0, err
	}

	f, sts := scanName(text)
	if sts != 0 {
		return nil, sts, nil
	}

	if err := ctx.saveSearch(nam, []parsedName{finishName(f, "", 0, false)}); err != nil {
		return nil, 0, err
	}

	return ctx.Files.searches[nam], 0, nil
}

// searchNext finds st's next file and reports it in nam, returning the
// status and STV.
func (ctx *Context) searchNext(nam uint32, st *searchState) (sts, stv uint32, err error) {
	for !st.Done {
		if st.Elem >= len(st.Names) {
			st.Done = true

			break
		}

		p := st.Names[st.Elem]

		if st.Matches == nil {
			vol, ok := ctx.Mounts.Lookup(p.Lookup)
			if !ok {
				st.Elem++

				continue
			}

			st.Matches = searchMatches(vol, p)
			st.Next = 0
		}

		if st.Next >= len(st.Matches) {
			st.Elem++
			st.Matches = nil

			continue
		}

		m := st.Matches[st.Next]
		st.Next++
		st.Found = true

		return ctx.reportMatch(nam, p, m)
	}

	if !st.Found {
		st.Found = true // a further call is RMS$_NMF

		return rmsFileNotFound, 0, nil
	}

	return rmsNoMoreFiles, 0, nil
}

// searchMatches returns the files on vol that p matches, in the order
// $SEARCH returns them. The version is applied here, as VMS reads it:
// none or 0 is the highest, -n the one n below it.
func searchMatches(vol *volume.Volume, p parsedName) []filespec.Match {
	spec := p.spec()
	ver := spec.Version
	spec.Version = "*"

	if spec.Name == "" {
		spec.Name = "\x00" // no name matches nothing but a null name
	}

	all, err := filespec.Glob(vol, spec)
	if err != nil {
		return []filespec.Match{}
	}

	if ver == "*" {
		return all
	}

	n, _ := strconv.Atoi(ver)

	var out []filespec.Match

	// all lists each name's versions together, highest first.
	for i := 0; i < len(all); {
		j := i
		for j < len(all) && sameFile(all[i], all[j]) {
			j++
		}

		group := all[i:j]

		switch {
		case n > 0:
			for _, m := range group {
				if int(m.Version) == n {
					out = append(out, m)
				}
			}

		case -n < len(group):
			out = append(out, group[-n])
		}

		i = j
	}

	if out == nil {
		out = []filespec.Match{}
	}

	return out
}

// sameFile reports whether a and b are versions of one file.
func sameFile(a, b filespec.Match) bool {
	return a.Name == b.Name && a.Type == b.Type && strings.Join(a.Dirs, ".") == strings.Join(b.Dirs, ".")
}

// reportMatch writes a file $SEARCH found into nam: the resultant string
// and its components, FID, and DID.
func (ctx *Context) reportMatch(nam uint32, p parsedName, m filespec.Match) (sts, stv uint32, err error) {
	f := fileName{
		Dev:  p.Dev,
		Dir:  dirSpec{Elems: m.Dirs}.String(),
		Name: m.Name,
		Type: "." + m.Type,
		Ver:  ";" + strconv.Itoa(int(m.Version)),
	}
	text := f.Dev + f.Dir + f.Name + f.Type + f.Ver

	rss, err := ctx.loadByte(nam + namRSS)
	if err != nil {
		return 0, 0, err
	}

	rsa, err := ctx.loadLongword(nam + namRSA)
	if err != nil {
		return 0, 0, err
	}

	overflow, err := ctx.storeString(rsa, rss, text)
	if err != nil {
		return 0, 0, err
	}

	if overflow {
		return rmsRSSError, 0, nil
	}

	if rsa != 0 && rss != 0 {
		if err := ctx.storeByte(nam+namRSL, byte(len(text))); err != nil {
			return 0, 0, err
		}

		if err := ctx.storeComponents(nam, rsa, f); err != nil {
			return 0, 0, err
		}
	}

	if err := ctx.storeFid(nam+namFID, m.Fid); err != nil {
		return 0, 0, err
	}

	if vol, ok := ctx.Mounts.Lookup(p.Lookup); ok {
		if dir, err := filespec.ResolveDirectory(vol, m.Dirs); err == nil {
			if err := ctx.storeFid(nam+namDID, dir.Header.Fid); err != nil {
				return 0, 0, err
			}
		}
	}

	return rmsNormal, 0, nil
}
