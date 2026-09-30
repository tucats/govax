package main

import (
	"log"
	"regexp"
	"strconv"
	"strings"
)

// Reading a C header's constants, such as VMS's fabdef.h, rabdef.h, and
// rmsdef.h.
//
// These headers only ever define a value constant as a plain, bare
// integer literal (decimal or 0x-prefixed hex), never an expression
// combining other macros, so a single-line regex per #define is
// sufficient; no C preprocessor or expression evaluation is needed. Each
// header also ends with a block of C-only field-access aliasing macros
// (e.g. "#define fab$w_ifi fab$r_ifi_overlay.fab$w_ifi") that this parser
// must not mistake for value constants. Every real value constant is
// spelled in upper case (FAB$C_BID, RMS$_NORMAL, ...) while every
// aliasing macro is spelled in lower case, so filtering by case alone
// (not by trying to detect a non-numeric value, which would still match
// some of these) cleanly separates the two without false positives.

// defineRE matches one "#define NAME value" line, capturing the name and
// the bare integer literal. It is anchored to upper-case names only (see
// above for why that's the correct, not merely convenient, filter).
var defineRE = regexp.MustCompile(`^#define\s+([A-Z][A-Z0-9_]*\$[A-Z0-9_]+)\s+(0[Xx][0-9A-Fa-f]+|[0-9]+)\b`)

// parseDefines returns every constant src defines whose name begins with
// prefix ("" for all of them).
func parseDefines(src, prefix string) map[string]uint32 {
	out := map[string]uint32{}

	for _, line := range strings.Split(src, "\n") {
		m := defineRE.FindStringSubmatch(line)
		if m == nil || !strings.HasPrefix(m[1], prefix) {
			continue
		}

		v, err := strconv.ParseUint(m[2], 0, 32)
		if err != nil {
			log.Fatalf("gen: bad integer in %q: %v", line, err)
		}

		name := m[1]
		if prev, ok := out[name]; ok && prev != uint32(v) {
			log.Fatalf("gen: %s redefined with a different value (%d, then %d)", name, prev, v)
		}

		out[name] = uint32(v)
	}

	return out
}
