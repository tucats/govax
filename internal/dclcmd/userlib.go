package dclcmd

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is LINK's default user libraries (docs/PHASE-68 - link
// shareable images.md, subtask 12; the Linker manual's /USERLIBRARY): the
// libraries the logical names LNK$LIBRARY, LNK$LIBRARY_1, ...,
// LNK$LIBRARY_999 name, searched after the command's own input and before
// IMAGELIB.OLB and STARLET.OLB. Each may be an object library or a
// shareable image library. /USERLIBRARY=(table,...) picks the logical name
// tables looked in, of PROCESS, GROUP, and SYSTEM, in that order; ALL (the
// default) is all three, and NONE, like /NOUSERLIBRARY, none. In each
// table the names are taken in turn until one isn't defined there, as the
// manual says.

// userLibraryTables are /USERLIBRARY's tables, in the order searched, and
// the logical name table each is.
var userLibraryTables = []struct{ keyword, table string }{
	{"PROCESS", "LNM$PROCESS"},
	{"GROUP", "LNM$GROUP"},
	{"SYSTEM", "LNM$SYSTEM"},
}

// maxUserLibrary is the last LNK$LIBRARY_n.
const maxUserLibrary = 999

// userLibraryTableNames returns the logical name tables /USERLIBRARY's
// keywords name, in the order they are searched. No keywords is ALL.
func userLibraryTableNames(keywords []string) ([]string, error) {
	if len(keywords) == 0 {
		keywords = []string{"ALL"}
	}

	want := map[string]bool{}

	for _, k := range keywords {
		k = strings.ToUpper(strings.TrimSpace(k))

		switch {
		case k == "" || abbreviates(k, "ALL", 1):
			for _, t := range userLibraryTables {
				want[t.keyword] = true
			}
		case abbreviates(k, "NONE", 1):
			return nil, nil
		default:
			matched := false

			for _, t := range userLibraryTables {
				if abbreviates(k, t.keyword, 1) {
					want[t.keyword], matched = true, true
				}
			}

			if !matched {
				return nil, vmserrors.New(vmserrors.CLI_IVKEYW, k)
			}
		}
	}

	var tables []string

	for _, t := range userLibraryTables {
		if want[t.keyword] {
			tables = append(tables, t.table)
		}
	}

	return tables, nil
}

// abbreviates reports whether s is keyword or an abbreviation of it at
// least least characters long.
func abbreviates(s, keyword string, least int) bool {
	return len(s) >= least && len(s) <= len(keyword) && keyword[:len(s)] == s
}

// userLibraries returns the default user libraries the logical name
// tables name, in order, as symbol sources.
func (c *Context) userLibraries(tables []string) ([]link.SymbolSource, error) {
	db := c.Logicals
	if db == nil && c.ContainerSession != nil {
		db = c.ContainerSession.Logicals
	}

	if db == nil {
		return nil, nil
	}

	var sources []link.SymbolSource

	for _, table := range tables {
		for n := 0; n <= maxUserLibrary; n++ {
			name := "LNK$LIBRARY"
			if n > 0 {
				name = fmt.Sprintf("LNK$LIBRARY_%d", n)
			}

			e, err := db.Translate(table, name, lnm.User, 0)
			if err != nil || len(e.Equivalences) == 0 {
				break
			}

			src, err := c.userLibrary(e.Equivalences[0].Value)
			if err != nil {
				return nil, err
			}

			sources = append(sources, src)
		}
	}

	return sources, nil
}

// userLibrary reads the library spec names, a default user library, as a
// symbol source.
func (c *Context) userLibrary(spec string) (link.SymbolSource, error) {
	s := c.ContainerSession

	loc, err := s.Locate(spec, false)
	if err != nil {
		return nil, FileFailure(err, spec)
	}

	data, found, err := s.ReadRawFile(loc.WithDefaultType("OLB"))
	if err != nil {
		return nil, FileFailure(err, loc.Name)
	}

	lib, err := lbr.Open(data)
	if err != nil {
		return nil, vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s: %w", found.Name, err), found.Name)
	}

	switch lib.Type {
	case lbr.TypeShareable:
		return c.imageLibrary(lib, found, GovaxSymbols(c.ShimLinks)), nil
	case lbr.TypeObject:
		return &link.ObjectLibrarySource{File: found.Name, Library: lib}, nil
	}

	return nil, vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s is a %s library, not an object or shareable image library", found.Name, lib.Type), found.Name)
}
