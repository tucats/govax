package rms

// searchState is one NAM block's wildcard context: what $PARSE found and
// how far $SEARCH has got through it (docs/PHASE-33.md, subtask 3).
type searchState struct {
	// WCC is the context number in the NAM's NAM$L_WCC.
	WCC uint32

	// Names is the parse's expanded names, one per search list element.
	Names []parsedName
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
