package cpu

// shortDouble is decode_operand.c's short_double[64] table, verbatim: the
// value a floating-point short-literal operand (addressing modes 0-3, on an
// instruction whose short-literal Type is ShortLiteralFloat) encodes when
// its 6-bit field is used as a table index rather than a raw integer.
var shortDouble = [64]float64{
	1. / 2., 9. / 16., 5. / 8., 11. / 16., 3. / 4., 13. / 16., 7. / 8., 15. / 16.,
	1., 9. / 8., 5. / 4., 11. / 8., 3. / 2., 13. / 8., 7. / 4., 15. / 8.,
	2., 9. / 4., 5. / 2., 11. / 4., 3., 13. / 4., 7. / 2., 15. / 4.,
	4., 9. / 2., 5., 11. / 2., 6., 13. / 2., 7., 15. / 2.,
	8., 9., 10., 11., 12., 13., 14., 15.,
	16., 18., 20., 22., 24., 26., 28., 30.,
	32., 36., 40., 44., 48., 52., 56., 60.,
	64., 72., 80., 88., 96., 104., 112., 120.,
}

// ShortFloat returns the short-literal float value at index i (0-63),
// exported for internal/asm's assembler/disassembler, which encodes and
// decodes S^#n float literals against this same table rather than a
// duplicate copy.
func ShortFloat(i int) float64 { return shortDouble[i] }

// FindShortFloat returns the index of value in the short-literal float
// table, and whether it was found — the encoding-side counterpart to
// ShortFloat, used when assembling an "S^#n" (or a bare "#n" small enough
// to use short-literal form) floating operand.
func FindShortFloat(value float64) (int, bool) {
	for i, v := range shortDouble {
		if v == value {
			return i, true
		}
	}
	return 0, false
}
