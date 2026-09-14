package rtl

import (
	"fmt"
	"math"
	"strings"
)

// Port of librtl_print.c's decc_printf/decc_sprintf and the format engine
// they share.
//
// decc_apply_format's own approach is "parse just enough of the directive to
// know its type and argument count, then hand the original format text
// straight to the host's own sprintf()" — this port does the same thing
// using Go's fmt.Sprintf instead of re-implementing width/precision/flag
// parsing from scratch: after stripping the one syntax host C has that Go's
// fmt doesn't ('l', the long-size modifier — Go has no equivalent and
// doesn't need one, since the value's own type already says its width), a
// %[-][0][width][.precision]{d,x,X,c,s,f} directive means the same thing to
// both. This is RTL tooling, not emulated VAX ISA behavior, so there's no
// fidelity reason to hand-rebuild a printf clone Go already has.

// formatCodes are the directive-terminating characters decc_apply_format's
// own scan recognizes (its "instring(ch, \"%dxXfsc\")" check).
const formatCodes = "%dxXfsc"

// loadDFloat converts VAX D_floating bits (lo/hi longwords, matching
// fpu_load's own argument order) to a float64. This is a self-contained
// copy of internal/cpu's unexported fpuLoad algorithm (docs/PHASE-05.md's
// design notes explain its derivation) rather than a shared call, since
// that function isn't exported and this package has no other reason to
// depend on internal/cpu. A reserved encoding (fpuLoad's own
// ExcReservedOp case) yields 0 here rather than an error, matching
// decc_apply_format's own "fpu_load(...); *pos += 2;" — the C source never
// checks fpu_load's return code at all.
func loadDFloat(lo, hi uint32) float64 {
	wordSwap := func(v uint32) uint32 { return v<<16 | v>>16 }

	lowLong := wordSwap(lo)
	sign := lowLong >> 31
	biasedExp := lowLong >> 23 & 0xFF
	frac23 := lowLong & 0x7FFFFF
	if biasedExp == 0 {
		return 0
	}

	ieeeExp := uint32(int(biasedExp) - 129 + 1023)
	hi32 := sign<<31 | ieeeExp<<20 | frac23>>3
	highLong := wordSwap(hi)
	lo32 := frac23&0x7<<29 | highLong>>3

	bits := uint64(hi32)<<32 | uint64(lo32)
	return math.Float64frombits(bits)
}

// deccFormat is decc_format: scans the NUL-terminated format string at
// argv[startPos], consuming subsequent argv entries for each directive,
// and returns the assembled output.
func deccFormat(env *Environment, argv []uint32, startPos int) (string, error) {
	fmtStr, err := loadDString(env, argv[startPos])
	if err != nil {
		return "", err
	}

	var out strings.Builder
	pos := startPos + 1
	i := 0
	for i < len(fmtStr) {
		ch := fmtStr[i]
		switch ch {
		case 0:
			return out.String(), nil

		case '\\':
			i++
			if i >= len(fmtStr) {
				break
			}
			switch fmtStr[i] {
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 't':
				out.WriteByte('\t')
			case '0':
				out.WriteByte(0)
			default:
				out.WriteByte(fmtStr[i])
			}
			i++

		case '%':
			start := i
			i++
			for i < len(fmtStr) && !strings.ContainsRune(formatCodes, rune(fmtStr[i])) {
				i++
			}
			if i >= len(fmtStr) {
				// Malformed directive with no terminator; nothing sensible
				// to substitute, matching decc_apply_format's own
				// unrecognized-format diagnostic path in spirit (no crash,
				// no output for it).
				return out.String(), nil
			}
			i++ // include the terminating code character
			spec := fmtStr[start:i]
			code := spec[len(spec)-1]

			if code == '%' {
				out.WriteByte('%')
				continue
			}

			cleaned := strings.ReplaceAll(spec, "l", "")

			switch code {
			case 'd':
				if pos >= len(argv) {
					return out.String(), nil
				}
				fmt.Fprintf(&out, cleaned, int32(argv[pos]))
				pos++
			case 'x', 'X':
				if pos >= len(argv) {
					return out.String(), nil
				}
				fmt.Fprintf(&out, cleaned, argv[pos])
				pos++
			case 'c':
				if pos >= len(argv) {
					return out.String(), nil
				}
				fmt.Fprintf(&out, cleaned, rune(argv[pos]&0xFF))
				pos++
			case 'f':
				if pos+1 >= len(argv) {
					return out.String(), nil
				}
				fmt.Fprintf(&out, cleaned, loadDFloat(argv[pos], argv[pos+1]))
				pos += 2
			case 's':
				if pos >= len(argv) {
					return out.String(), nil
				}
				s, err := loadDString(env, argv[pos])
				if err != nil {
					s = ""
				}
				fmt.Fprintf(&out, cleaned, s)
				pos++
			}

		default:
			out.WriteByte(ch)
			i++
		}
	}
	return out.String(), nil
}

// shimDeccPrintf is DECC$PRINTF: formats and writes to the console (RMS
// internal file index 1's own target — the real C source's "printf" writes
// to the host's actual stdout, which in this port is Environment's
// consoleOut).
func shimDeccPrintf(env *Environment, argv []uint32) (uint32, error) {
	s, err := deccFormat(env, argv, 0)
	if err != nil {
		return 0, err
	}
	if env.consoleOut != nil {
		if _, err := env.consoleOut.Write([]byte(s)); err != nil {
			return 0, err
		}
	}
	return uint32(len(s)), nil
}

// shimDeccSprintf is DECC$SPRINTF: formats into a caller-supplied VAX
// buffer, NUL-terminated.
func shimDeccSprintf(env *Environment, argv []uint32) (uint32, error) {
	s, err := deccFormat(env, argv, 1)
	if err != nil {
		return 0, err
	}
	if err := storeString(env, s+"\x00", argv[0], len(s)+1); err != nil {
		return 0xFFFFFFFF, nil
	}
	return 0, nil
}

func registerPrintShims(t *ShimTable) {
	t.Register(8, "DECC$PRINTF", shimDeccPrintf)
	t.Register(9, "DECC$SPRINTF", shimDeccSprintf)
}
