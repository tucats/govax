package listing

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/fortran/diag"
)

// Show is what /SHOW asks a listing to hold. The manuals to hand don't give the
// qualifier; the keywords and the default are as the phase's survey recalls VAX
// FORTRAN's, those that mean anything here.
type Show struct {
	// Include is every INCLUDE file's lines, whatever INCLUDE's own /LIST says.
	Include bool

	// Map is each program unit's storage map.
	Map bool
}

// DefaultShow is what a listing holds with no /SHOW.
func DefaultShow() Show {
	return Show{Map: true}
}

// showKeywords are /SHOW's keywords, each what it does to a setting.
var showKeywords = map[string]func(*Show){
	"ALL":       func(s *Show) { *s = Show{Include: true, Map: true} },
	"NONE":      func(s *Show) { *s = Show{} },
	"INCLUDE":   func(s *Show) { s.Include = true },
	"NOINCLUDE": func(s *Show) { s.Include = false },
	"MAP":       func(s *Show) { s.Map = true },
	"NOMAP":     func(s *Show) { s.Map = false },
}

// With is the setting with /SHOW's keywords applied in order. No keywords is
// /SHOW alone, ALL. A keyword may be cut short, while it stays one keyword's start.
func (s Show) With(keywords []string) (Show, error) {
	if len(keywords) == 0 {
		keywords = []string{"ALL"}
	}

	for _, word := range keywords {
		apply, err := showKeyword(word)
		if err != nil {
			return s, err
		}

		apply(&s)
	}

	return s, nil
}

// showKeyword finds the keyword word is, whole or cut short.
func showKeyword(word string) (func(*Show), error) {
	found, matches := diag.FindKeyword(showKeywords, word)

	switch matches {
	case 0:
		return nil, fmt.Errorf("/SHOW has no keyword %s; give ALL, NONE, [NO]INCLUDE, or [NO]MAP", strings.ToUpper(word))
	case 1:
		return found, nil
	}

	return nil, fmt.Errorf("/SHOW's keyword %s is ambiguous; give ALL, NONE, [NO]INCLUDE, or [NO]MAP", strings.ToUpper(word))
}
