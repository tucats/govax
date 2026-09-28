package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// blissLiteralRE matches one entry of a BLISS "LITERAL" declaration as
// laid out in reference/vms/ssdef.txt: "NAME, I, value" with an optional
// trailing separator. The VEST listings (reference/vms/syidef.txt) write
// the type as I4, a four-byte integer, which is accepted too, and a few
// of their names are in mixed case (dvidef.txt's DVI$_SHDW_spare_bit_1).
var blissLiteralRE = regexp.MustCompile(`^\s*([A-Za-z0-9_$]+),\s*I4?,\s*(-?[0-9]+)\s*[,;]?\s*$`)

// parseBlissLiterals extracts every "NAME, I, value" literal whose name
// starts with prefix from a BLISS LITERAL listing such as
// reference/vms/ssdef.txt (the VAX/VMS 7.3 $SSDEF definitions). Blank
// lines, ";" comment lines, and the "LITERAL" keyword are skipped; any
// other line is an error, so an unexpected format can't be silently
// half-parsed. Literals with other prefixes (ssdef.txt also defines
// SYSTEM$_FACILITY) are accepted but left out.
func parseBlissLiterals(src, prefix string) (map[string]uint32, error) {
	out := map[string]uint32{}

	for n, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.EqualFold(trimmed, "LITERAL") {
			continue
		}

		m := blissLiteralRE.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("line %d: unrecognised BLISS literal %q", n+1, line)
		}

		if !strings.HasPrefix(m[1], prefix) {
			continue
		}

		v, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: bad value %q", n+1, m[2])
		}

		if prev, ok := out[m[1]]; ok && prev != uint32(v) {
			return nil, fmt.Errorf("%s redefined with a different value (%d, then %d)", m[1], prev, v)
		}

		out[m[1]] = uint32(v)
	}

	return out, nil
}
