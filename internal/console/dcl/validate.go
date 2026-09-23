package dcl

import "github.com/tucats/govax/internal/vmserrors"

// validate resolves every named cross-reference a grammar statement could
// only record as a string at definition time (a verb's /alias=, a
// qualifier's /syntax= or /alias=, a keyword's /syntax=, and any parameter/
// qualifier's /type= naming a user-defined Type) — matching DCLvalidate's
// role of catching dangling/forward references once a whole grammar has
// been read.
func (g *Grammar) validate() error {
	for _, e := range g.entries {
		if e.Alias != "" {
			target, ok := g.entries[e.Alias]
			if !ok {
				return vmserrors.New(vmserrors.CLI_ALIASNOTFOUND, e.Name, e.Alias)
			}

			e.aliasRef = target
		}

		for _, p := range e.Parameters {
			if p.Type == TypeKeyword {
				t, ok := g.types[p.TypeName]
				if !ok {
					return vmserrors.New(vmserrors.CLI_TYPENOTFOUND, "Parameter", p.Name, p.TypeName)
				}

				p.typeRef = t
			}

			// A parameter's own private qualifier list (Phase 23's
			// parameter-scoped qualifiers, e.g. COPY's /HOST nested under
			// SOURCE and again under DESTINATION) needs exactly the same
			// cross-reference resolution as the entry's own top-level
			// qualifier list below — just resolved against that one
			// parameter's list instead of the whole entry's. validateQualifiers
			// is shared between both call sites so the two stay in lockstep.
			if err := validateQualifiers(g, p.Qualifiers); err != nil {
				return err
			}
		}

		if err := validateQualifiers(g, e.Qualifiers); err != nil {
			return err
		}

		for _, d := range e.Disallows {
			if _, _, err := e.qualifier(d.Qual1); err != nil {
				return vmserrors.New(vmserrors.CLI_DISALLOWNOTFOUND, e.Name, d.Qual1)
			}

			if _, _, err := e.qualifier(d.Qual2); err != nil {
				return vmserrors.New(vmserrors.CLI_DISALLOWNOTFOUND, e.Name, d.Qual2)
			}
		}
	}

	for _, t := range g.types {
		for _, kw := range t.Keywords {
			if kw.Syntax != "" {
				if _, ok := g.entries[kw.Syntax]; !ok {
					return vmserrors.New(vmserrors.CLI_KEYWORDSYNTAXNOTFOUND, t.Name, kw.Name, kw.Syntax)
				}
			}
		}
	}

	return nil
}

// validateQualifiers resolves the named cross-references (/type=, /syntax=,
// /alias=) carried by one flat list of qualifiers — a helper shared by
// validate() between an Entry's own top-level Qualifiers list and each of
// its Parameters' private, parameter-scoped Qualifiers list (Phase 23),
// so both kinds of qualifier get identical validation. An /alias= is
// resolved against this same list (quals), not the whole entry: an
// entry-level qualifier can only alias another entry-level qualifier, and a
// parameter-scoped qualifier can only alias another qualifier declared on
// that same parameter — aliasing across scopes isn't supported, and no
// command this project defines needs it.
func validateQualifiers(g *Grammar, quals []*Qualifier) error {
	for _, q := range quals {
		if q.Type == TypeKeyword {
			t, ok := g.types[q.TypeName]
			if !ok {
				return vmserrors.New(vmserrors.CLI_TYPENOTFOUND, "Qualifier", q.Name, q.TypeName)
			}

			q.typeRef = t
		}

		if q.Syntax != "" {
			if _, ok := g.entries[q.Syntax]; !ok {
				return vmserrors.New(vmserrors.CLI_SYNTAXNOTFOUND, q.Name, q.Syntax)
			}
		}

		if q.Alias != "" {
			target, _, err := matchQualifier(quals, q.Alias)
			if err != nil {
				return vmserrors.New(vmserrors.CLI_QUALALIASNOTFOUND, q.Name, q.Alias)
			}

			q.aliasRef = target
		}
	}

	return nil
}
