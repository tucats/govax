package diag

import (
	"fmt"
	"strings"
)

// Warnings is what /WARNINGS asks the compiler to say (docs/PHASE-85 - FORTRAN compiler
// usability.md). The zero value is the default: the general warnings and nothing else,
// the one thing here the language manual speaks of being DECLARATIONS (the note in its
// section 4.8). It has a NoGeneral, not a General, so that a program leaving it out gets
// the default, as with every other qualifier's setting.
type Warnings struct {
	// Declarations warns of each name that takes its type from its first letter,
	// which is what IMPLICIT NONE would refuse.
	Declarations bool

	// NoGeneral holds back the warnings and informational messages no keyword of
	// their own asks for. Errors can't be held back.
	NoGeneral bool

	// Unused warns of the names only declared and the labels nothing refers to.
	Unused bool

	// Alignment is CDEC$ OPTIONS /WARN=ALIGNMENT from the start of each unit.
	Alignment bool
}

// warningKeywords are /WARNINGS's keywords, each what it does to a setting.
var warningKeywords = map[string]func(*Warnings){
	"ALL":            func(w *Warnings) { *w = Warnings{Declarations: true, Unused: true, Alignment: true} },
	"NONE":           func(w *Warnings) { *w = Warnings{NoGeneral: true} },
	"DECLARATIONS":   func(w *Warnings) { w.Declarations = true },
	"NODECLARATIONS": func(w *Warnings) { w.Declarations = false },
	"GENERAL":        func(w *Warnings) { w.NoGeneral = false },
	"NOGENERAL":      func(w *Warnings) { w.NoGeneral = true },
	"UNUSED":         func(w *Warnings) { w.Unused = true },
	"NOUNUSED":       func(w *Warnings) { w.Unused = false },
	"ALIGNMENT":      func(w *Warnings) { w.Alignment = true },
	"NOALIGNMENT":    func(w *Warnings) { w.Alignment = false },
}

// warningKeywordList says what the keywords are, for a message.
const warningKeywordList = "ALL, NONE, [NO]ALIGNMENT, [NO]DECLARATIONS, [NO]GENERAL, or [NO]UNUSED"

// With is the setting with /WARNINGS's keywords applied in order, so that
// /WARNINGS=UNUSED adds to the default and /WARNINGS=(NONE,UNUSED) replaces it. No
// keywords is /WARNINGS alone, which is ALL, as /CHECK alone is (ours, unconfirmed).
func (w Warnings) With(keywords []string) (Warnings, error) {
	if len(keywords) == 0 {
		keywords = []string{"ALL"}
	}

	for _, word := range keywords {
		apply, matches := FindKeyword(warningKeywords, word)

		switch matches {
		case 1:
			apply(&w)
		case 0:
			return w, fmt.Errorf("/WARNINGS has no keyword %s; give %s", strings.ToUpper(word), warningKeywordList)
		default:
			return w, fmt.Errorf("/WARNINGS's keyword %s is ambiguous; give %s", strings.ToUpper(word), warningKeywordList)
		}
	}

	return w, nil
}

// askedFor are the messages a keyword of their own asked for, so that NOGENERAL doesn't
// hold them back: /WARNINGS=(NOGENERAL,UNUSED) is how to see only those. A source's
// own CDEC$ OPTIONS /WARN=ALIGNMENT asks for the two ALIGN messages too, so they appear
// even under /WARNINGS=NONE, which gives only the setting each unit starts with (ours).
var askedFor = map[string]bool{
	"UNDECLARED": true, "UNUSEDNAME": true, "UNUSEDDUMMY": true, "UNUSEDLABEL": true,
	"ALIGNFIELD": true, "ALIGNCOMMON": true,
}

// Held reports whether the setting keeps the message m from being shown.
func (w Warnings) Held(m Message) bool {
	return w.NoGeneral && m.Severity <= Warning && !askedFor[m.Ident]
}

// Drop takes out the messages held says to, so that what a compilation reports, counts,
// and lists is only what's shown.
func (l *List) Drop(held func(Message) bool) {
	kept := l.messages[:0]

	for _, m := range l.messages {
		if !held(m) {
			kept = append(kept, m)
		}
	}

	l.messages = kept
}

// FindKeyword finds the keyword word is in a qualifier's table of keywords, ignoring
// case, whole or cut short, and says how many it could be: 1 if the result is good, 0 if
// none, and more if it's ambiguous. A whole keyword is itself, even if it starts another.
func FindKeyword[T any](keywords map[string]T, word string) (T, int) {
	word = strings.ToUpper(word)

	if found, ok := keywords[word]; ok {
		return found, 1
	}

	var found T

	matches := 0

	for name, value := range keywords {
		if word != "" && strings.HasPrefix(name, word) {
			found = value
			matches++
		}
	}

	return found, matches
}
