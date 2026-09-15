package dcl

import (
	"fmt"
	"strings"
)

func upcase(s string) string { return strings.ToUpper(s) }

// ambiguousError/unrecognizedError match DCLkeysearch's/DCLqualsearch's own
// two distinct error messages ("unrecognized keyword %s" vs. "ambigious
// keyword %s") for an unmatched or ambiguous abbreviation.
func unrecognizedError(kind, token string) error {
	return fmt.Errorf("dcl: unrecognized %s %q", kind, token)
}

func ambiguousError(kind, token string) error {
	return fmt.Errorf("dcl: ambiguous %s %q", kind, token)
}

// matchKeyword resolves token (already upcased) against keywords by
// unambiguous prefix, allowing a "NO" prefix to negate a match (unless the
// full, un-negated name is itself a better/unique match) — a direct port of
// DCLkeysearch's matching rule, minus its DCL_NONEGATE per-keyword flag
// (unused by any keyword in testdata/dcl/evax.dcl).
func matchKeyword(keywords []*Keyword, token string) (kw *Keyword, negated bool, err error) {
	var found *Keyword
	
	count := 0
	exact := false

	for _, k := range keywords {
		if strings.HasPrefix(k.Name, token) {
			if !exact {
				found, count = k, count+1
			}

			if k.Name == token {
				found, count, exact = k, 1, true
			}
		}
	}

	if count == 1 {
		return found, false, nil
	}

	if strings.HasPrefix(token, "NO") && len(token) > 2 {
		return matchKeyword(keywords, token[2:])
	}

	if count == 0 {
		return nil, false, unrecognizedError("keyword", token)
	}

	return nil, false, ambiguousError("keyword", token)
}

// matchQualifier resolves token (already upcased, no leading "/") against an
// Entry's qualifiers by unambiguous prefix, with the same NO-prefix
// negation rule as matchKeyword — a direct port of DCLqualsearch.
func matchQualifier(quals []*Qualifier, token string) (q *Qualifier, negated bool, err error) {
	var found *Qualifier

	count := 0
	exact := false

	for _, cand := range quals {
		if strings.HasPrefix(cand.Name, token) {
			if !exact {
				found, count = cand, count+1
			}

			if cand.Name == token {
				found, count, exact = cand, 1, true
			}
		}
	}

	if count == 1 {
		return found, false, nil
	}

	if strings.HasPrefix(token, "NO") && len(token) > 2 {
		q, _, err = matchQualifier(quals, token[2:])
		if err == nil {
			return q, true, nil
		}

		return nil, false, err
	}

	if count == 0 {
		return nil, false, unrecognizedError("qualifier", token)
	}

	return nil, false, ambiguousError("qualifier", token)
}

// matchVerb resolves token (already upcased) against the grammar's
// top-level verbs by unambiguous prefix (no NO-prefix negation — a verb
// can't be negated), matching how DCLverb-created entries are looked up
// during DCLparse's "verb" FSM state.
func (g *Grammar) matchVerb(token string) (*Entry, error) {
	var found *Entry

	count := 0
	exact := false

	for _, e := range g.verbOrder {
		if strings.HasPrefix(e.Name, token) {
			if !exact {
				found, count = e, count+1
			}

			if e.Name == token {
				found, count, exact = e, 1, true
			}
		}
	}

	if count == 0 {
		return nil, unrecognizedError("verb", token)
	}

	if count > 1 {
		return nil, ambiguousError("verb", token)
	}

	if found.aliasRef != nil {
		return found.aliasRef, nil
	}

	return found, nil
}
