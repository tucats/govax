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
		}

		for _, q := range e.Qualifiers {
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
				target, _, err := e.qualifier(q.Alias)
				if err != nil {
					return vmserrors.New(vmserrors.CLI_QUALALIASNOTFOUND, q.Name, q.Alias)
				}
				q.aliasRef = target
			}
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
