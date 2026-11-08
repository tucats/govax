package lbr

import "strings"

// Match returns index 1's keys that pattern matches, in index order. The
// pattern may hold the wildcards * (any characters) and % (any one
// character). Unless the index compares keys as they are
// (IDD$V_NOCASECMP), the pattern is upper-cased first, as LBR$LOOKUP_KEY
// upper-cases a key it looks up; if the index upper-cases its entries to
// compare them (IDD$V_UPCASNTRY), so is each key.
func (l *Library) Match(pattern string) []Key {
	if len(l.Indexes) == 0 {
		return nil
	}

	x := l.Indexes[0]

	var out []Key

	for _, k := range x.Keys {
		if matchKey(pattern, k.Name, x.Flags) {
			out = append(out, k)
		}
	}

	return out
}

// Match returns the names of the modules pattern matches, in index order,
// by Library.Match's rules.
func (b *Builder) Match(pattern string) []string {
	var out []string

	for _, name := range b.Names() {
		if matchKey(pattern, name, b.flags[0]) {
			out = append(out, name)
		}
	}

	return out
}

func matchKey(pattern, key string, flags uint16) bool {
	if flags&IndexNoCaseCmp == 0 {
		pattern = upcase(pattern)
	}

	if flags&IndexUpcase != 0 {
		key = upcase(key)
	}

	if !strings.ContainsAny(pattern, "*%") {
		return pattern == key
	}

	return wildMatch(pattern, key)
}

// wildMatch matches s against pattern's * and % wildcards.
func wildMatch(pattern, s string) bool {
	// star is where the last * was, and mark the text it has taken up to.
	star, mark := -1, 0

	for p, i := 0, 0; i < len(s) || p < len(pattern); {
		switch {
		case p < len(pattern) && pattern[p] == '*':
			star, mark = p, i
			p++

			continue
		case p < len(pattern) && i < len(s) && (pattern[p] == '%' || pattern[p] == s[i]):
			p++
			i++

			continue
		case star >= 0 && mark < len(s):
			mark++
			p, i = star+1, mark

			continue
		}

		return false
	}

	return true
}

// Find returns where the module key names starts, by LBR$LOOKUP_KEY's rule
// (testdata/help90's LBR90, round 1b): unless the index compares keys as
// they are (IDD$V_NOCASECMP), the key is upper-cased, and if the index
// upper-cases its entries to compare them (IDD$V_UPCASNTRY), so is each
// entry. A help library's index does the second, so "alpha" and
// "MIXEDCASE" find ALPHA and MixedCase. Abbreviations find nothing.
func (x *Index) Find(key string) (RFA, bool) {
	if r, ok := x.byKey[key]; ok {
		return r, true
	}

	if x.Flags&IndexNoCaseCmp == 0 {
		key = upcase(key)
	}

	for _, k := range x.Keys {
		name := k.Name
		if x.Flags&IndexUpcase != 0 {
			name = upcase(name)
		}

		if name == key {
			return k.RFA, true
		}
	}

	return RFA{}, false
}

// Select returns the index's keys that pattern matches, in index order, by
// LBR$GET_INDEX's rule: the pattern, with the wildcards * and %, is taken
// as it is, and each entry upper-cased if the index upper-cases its
// entries to compare them. VMS 7.1's GET_INDEX found nothing in a help
// library for "alpha*", where LBR$LOOKUP_KEY found "alpha" (testdata/help90's
// LBR90, round 1b).
func (x *Index) Select(pattern string) []Key {
	var out []Key

	for _, k := range x.Keys {
		name := k.Name
		if x.Flags&IndexUpcase != 0 {
			name = upcase(name)
		}

		if name == pattern || strings.ContainsAny(pattern, "*%") && wildMatch(pattern, name) {
			out = append(out, k)
		}
	}

	return out
}
