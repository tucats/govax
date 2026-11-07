package help

import "strings"

// Split turns a request's text into its words, as VMS's HELP reads them
// (testdata/help90, round 1b): words are separated by blanks and tabs,
// and a "/" starts a new word, so "ALPHA/BRIEF" is "ALPHA" and "/BRIEF",
// and "Slash/Inside" is "Slash" and "/Inside". Text in quotation marks is
// part of one word, quotes and all ("\"ALPHA Parameters\"" is a single
// word, which matches no key).
//
// A word that is only "(" is dropped: "HELP ODD (" showed ODD (unconfirmed
// why; DCL may take it for an empty list of values).
func Split(text string) []string {
	var (
		words  []string
		word   strings.Builder
		quoted bool
	)

	end := func() {
		if word.Len() > 0 {
			if w := word.String(); w != "(" {
				words = append(words, w)
			}

			word.Reset()
		}
	}

	for i := 0; i < len(text); i++ {
		c := text[i]

		switch {
		case c == '"':
			quoted = !quoted
			word.WriteByte(c)
		case quoted:
			word.WriteByte(c)
		case c == ' ' || c == '\t':
			end()
		case c == '/':
			end()
			word.WriteByte(c)
		default:
			word.WriteByte(c)
		}
	}

	end()

	return words
}

// maxWords is how many words a request's keys can have: VMS 7.1's HELP
// keeps nine, and a tenth is taken as the first again ("DEEPER L2 ... L9
// L10" is "Sorry, no documentation on DEEPER L2 L3 L4 L5 L6 L7 L8 L9
// DEEPER"; unconfirmed why).
const maxWords = 9

// matches reports whether a request's word names key: as an abbreviation
// (the key starts with the word), or, if the word has the wildcards * (any
// characters) or % (exactly one character), as a pattern the whole key
// matches. Case doesn't matter. A word that is only "/" names nothing
// ("HELP ALPHA /" is "Sorry").
func matches(word, key string) bool {
	w, k := strings.ToUpper(word), strings.ToUpper(key)

	if strings.ContainsAny(w, "*%") {
		return wildMatch(w, k)
	}

	if w == "" || w == "/" {
		return false
	}

	return strings.HasPrefix(k, w)
}

// wildMatch matches s against pattern's * and % wildcards.
func wildMatch(pattern, s string) bool {
	// star is where the last * was, and mark how much of s it has taken.
	star, mark := -1, 0

	for p, i := 0, 0; i < len(s) || p < len(pattern); {
		switch {
		case p < len(pattern) && pattern[p] == '*':
			star, mark = p, i
			p++
		case p < len(pattern) && i < len(s) && (pattern[p] == '%' || pattern[p] == s[i]):
			p++
			i++
		case star >= 0 && mark < len(s):
			mark++
			p, i = star+1, mark
		default:
			return false
		}
	}

	return true
}
