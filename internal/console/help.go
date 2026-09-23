package console

import (
	"bufio"
	"os"
	"strings"
)

// Help holds parsed HELP text, matching help.c's own vax.help file format
// (documented in the file itself, internal/bootdata/files/vax.help -- the
// single copy of this file; see that file's own History comment for why
// there is no longer a separate testdata/dcl/vax.help): a "$"-prefixed
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

			pendingKeys = append(pendingKeys, normalizeHelpKey(line[1:]))

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

// normalizeHelpToken upcases and pads/truncates a single key component --
// one HELP argument word, or one comma-separated piece of a "$"-line key
// read from the help file -- to exactly four characters, the on-disk
// format's own documented rule (vax.help's own preamble: "if the token is
// less than four characters long, it must be blank padded").
func normalizeHelpToken(tok string) string {
	tok = strings.ToUpper(strings.TrimSpace(tok))
	if len(tok) >= 4 {
		return tok[:4]
	}

	return tok + strings.Repeat(" ", 4-len(tok))
}

// helpKey builds the "$"-line key for a HELP command's argument words,
// matching help.c's read_verb-based key construction: each word is
// normalized (normalizeHelpToken) and joined by commas; no arguments at
// all maps to the literal key "HELP".
func helpKey(words []string) string {
	if len(words) == 0 {
		return "HELP"
	}

	toks := make([]string, len(words))

	for i, w := range words {
		toks[i] = normalizeHelpToken(w)
	}

	return strings.Join(toks, ",")
}

// normalizeHelpKey applies normalizeHelpToken to each comma-separated
// component of a raw "$"-line key straight from the help file, so a key
// can be written there as a plain, unpadded word (e.g. "$DIR") without
// its author having to remember to hand-pad it with trailing spaces --
// which, in practice, a text editor that trims trailing whitespace on
// save will silently destroy (confirmed by auditing vax.help itself: many
// pre-existing short bare-command keys -- RUN, GO, DO, VM, ASM, PSL, XFC,
// SH, ST, EX among them -- had lost their padding this way and could
// never actually be looked up, since helpKey always builds a fully
// four-character-padded query key to compare against). Using the exact
// same normalization on both the file-parsing side and the query-building
// side (helpKey) guarantees they can never drift apart like this again.
func normalizeHelpKey(raw string) string {
	toks := strings.Split(raw, ",")

	for i, t := range toks {
		toks[i] = normalizeHelpToken(t)
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
