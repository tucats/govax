package fortran

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Debug is what /DEBUG asks MACRO to write for the debugger (docs/PHASE-85 - FORTRAN
// compiler usability.md). The compiler writes no debugger records itself (Phase 74's
// Decision 3): it says so in the MACRO file, so that a plain MACRO does as it's told.
// The zero value is the default, MACRO's own: traceback records alone.
type Debug struct {
	// Symbols is .ENABLE DEBUG: debugger records.
	Symbols bool

	// NoTraceback is .DISABLE TRACEBACK: no traceback records.
	NoTraceback bool
}

// debugKeywords are /DEBUG's keywords, each what it does to a setting.
var debugKeywords = map[string]func(*Debug){
	"ALL":         func(d *Debug) { *d = Debug{Symbols: true} },
	"NONE":        func(d *Debug) { *d = Debug{NoTraceback: true} },
	"SYMBOLS":     func(d *Debug) { d.Symbols = true },
	"NOSYMBOLS":   func(d *Debug) { d.Symbols = false },
	"TRACEBACK":   func(d *Debug) { d.NoTraceback = false },
	"NOTRACEBACK": func(d *Debug) { d.NoTraceback = true },
}

// With is the setting /DEBUG's keywords ask for. No keywords is /DEBUG alone, ALL. As
// with MACRO's /DEBUG, naming what's wanted means only that, so /DEBUG=SYMBOLS has no
// traceback records; a list of only NO forms takes those from the default (ours).
func (d Debug) With(keywords []string) (Debug, error) {
	if len(keywords) == 0 {
		keywords = []string{"ALL"}
	}

	for _, word := range keywords {
		if !strings.HasPrefix(strings.ToUpper(word), "NO") {
			d = Debug{NoTraceback: true}

			break
		}
	}

	const list = "ALL, NONE, [NO]SYMBOLS, or [NO]TRACEBACK"

	for _, word := range keywords {
		apply, matches := diag.FindKeyword(debugKeywords, word)

		switch matches {
		case 1:
			apply(&d)
		case 0:
			return d, fmt.Errorf("/DEBUG has no keyword %s; give %s", strings.ToUpper(word), list)
		default:
			return d, fmt.Errorf("/DEBUG's keyword %s is ambiguous; give %s", strings.ToUpper(word), list)
		}
	}

	return d, nil
}

// writeDirectives writes the directives that say so, in the MACRO file's opening.
func (d Debug) writeDirectives(w *codegen.Writer) {
	if d.Symbols {
		w.Statement("", ".ENABLE", "DEBUG", "")
	}

	if d.NoTraceback {
		w.Statement("", ".DISABLE", "TRACEBACK", "")
	}
}
