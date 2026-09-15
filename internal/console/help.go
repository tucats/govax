package console

import (
	"bufio"
	"os"
	"strings"
)

// Help holds parsed HELP text, matching help.c's own vax.help file format
// (documented in the file itself, testdata/dcl/vax.help): a "$"-prefixed
// line marks a topic key (comma-separated 4-character, space-padded/
// truncated tokens, one per HELP argument word); several consecutive "$"
// lines with no text between them share the following body text (letting
// synonyms like SHOW/SH point at the same section); body text runs until
// the next "$" line or end of file.
type Help struct {
	sections map[string]string // key (no leading '$', trimmed) -> body text
}

// ParseHelp parses HELP text in the format above.
func ParseHelp(text string) *Help {
	h := &Help{sections: map[string]string{}}

	var (
		pendingKeys []string
		body        strings.Builder
	)

	flush := func() {
		if len(pendingKeys) == 0 {
			return
		}
		text := body.String()
		for _, k := range pendingKeys {
			h.sections[k] = text
		}

		pendingKeys = nil

		body.Reset()
	}

	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 4096), 1<<20)

	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "$") {
			if body.Len() > 0 {
				flush()
			}

			pendingKeys = append(pendingKeys, strings.TrimSpace(line[1:]))

			continue
		}

		if len(pendingKeys) > 0 {
			body.WriteString(line)
			body.WriteString("\n")
		}
	}

	flush()

	return h
}

// LoadHelpFile reads and parses a HELP text file.
func LoadHelpFile(path string) (*Help, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ParseHelp(string(b)), nil
}

// helpKey builds the "$"-line key for a HELP command's argument words,
// matching help.c's read_verb-based key construction: each word is
// upcased and space-padded/truncated to exactly 4 characters, joined by
// commas; no arguments at all maps to the literal key "HELP".
func helpKey(words []string) string {
	if len(words) == 0 {
		return "HELP"
	}

	toks := make([]string, len(words))

	for i, w := range words {
		w = strings.ToUpper(w)
		if len(w) >= 4 {
			toks[i] = w[:4]
		} else {
			toks[i] = w + strings.Repeat(" ", 4-len(w))
		}
	}

	return strings.Join(toks, ",")
}

// Help implements the HELP console command against h (a nil Help, or a
// topic with no matching section, prints a "no help available" message
// rather than erroring — matching help.c's own VAX_NOHELPFILE/VAX_NOHELP
// being non-fatal console messages, not something that aborts the
// command loop).
func (c *Console) Help(h *Help, words []string) error {
	if h == nil {
		c.Printf("No help file available\n")
		return nil
	}

	key := helpKey(words)
	
	c.Printf("\n")

	body, ok := h.sections[key]
	if !ok {
		c.Printf("No help available for that topic\n")
		return nil
	}

	c.Printf("%s", body)

	return nil
}
