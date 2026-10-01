package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Reading a list of values: lines of "NAME = value", such as
// testdata/mar/rms/defined.txt, which records the names VMS's own
// definition macros define, as real MACRO assembled them (docs/
// PHASE-32.md). Any other line (a comment, or "NAME undefined") is
// skipped. A value is decimal, or hexadecimal with 0x.

var valueRE = regexp.MustCompile(`^\s*([A-Za-z0-9_$.]+)\s*=\s*(0[xX][0-9A-Fa-f]+|[0-9]+)\s*$`)

// parseValues returns every value src lists whose name begins with prefix
// ("" for all of them). A name listed twice must have the same value both
// times.
func parseValues(src, prefix string) (map[string]uint32, error) {
	out := map[string]uint32{}

	for n, line := range strings.Split(src, "\n") {
		m := valueRE.FindStringSubmatch(line)
		if m == nil || !strings.HasPrefix(m[1], prefix) {
			continue
		}

		v, err := strconv.ParseUint(m[2], 0, 32)
		if err != nil {
			return nil, fmt.Errorf("line %d: bad value %q", n+1, m[2])
		}

		if prev, ok := out[m[1]]; ok && prev != uint32(v) {
			return nil, fmt.Errorf("line %d: %s is %#x here, but %#x earlier", n+1, m[1], v, prev)
		}

		out[m[1]] = uint32(v)
	}

	return out, nil
}
