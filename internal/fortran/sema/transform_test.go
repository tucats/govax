package sema

import "testing"

// TestTransform checks the transformational functions' arguments
// (transform.go): what compiles, and each message.
func TestTransform(t *testing.T) {
	// unit is a subroutine with arrays and scalars, then the lines given.
	unit := func(lines ...string) []string {
		return append(append([]string{"subroutine s(n, k)", "integer n, k, v(6), m(3, 3), w(3)", "logical l(3, 3)",
			"real r(3)", "character*2 c(3, 3)"}, lines...), "end subroutine")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"SPREAD", unit("m = spread(w, 2, n)", "v(1:3) = sum(spread(w, 1, 2), dim=1)"), ""},
		{"SPREAD of a scalar", unit("w = spread(n, 1, 3)"), ""},
		{"CSHIFT and EOSHIFT", unit("v = cshift(v, n)", "m = cshift(m, w, 2)", "m = eoshift(m, -1, 9, dim=1)",
			"c = eoshift(c, 1)", "c = eoshift(c, w, (/ 'a', 'b', 'c' /), 2)"), ""},
		{"PACK and UNPACK", unit("w = pack(m, l)", "v = pack(m, .true., v)", "m = unpack(w, l, 0)",
			"m = unpack(w, l, m)"), ""},
		{"RESHAPE of values", unit("m = reshape(v, (/ 3, n /))", "m = reshape(w, (/ 3, 3 /), (/ 1 /), (/ 2, 1 /))"), ""},
		{"a reduction by a DIM= not constant", unit("w = sum(m, dim=k)", "w = count(l, k)", "w = maxval(m, k, m > 0)"), ""},

		{"SPREAD's DIM= out of range", unit("m = spread(w, 3, n)"),
			"DIMRANGE: SPREAD's DIM, 3, isn't a dimension of an array of rank 2"},
		{"DIM= not constant (Phase 113)", unit("m = spread(w, k, n)", "m = cshift(m, w, k)",
			"m = eoshift(m, w, dim=k)", "w = spread(n, k, 3)"), ""},
		{"DIM= an array", unit("m = cshift(m, 1, w)"),
			"NEEDSUBSCRIPT: the array W can't be used without subscripts here"},
		{"SHIFT's rank with a DIM= not constant", unit("m = cshift(m, m, k)"),
			"ARRAYRANK: CSHIFT's SHIFT must be of rank 0 or 1, not 2"},
		{"SPREAD's NCOPIES", unit("m = spread(w, 1, 2.0)"),
			"BADARG: the intrinsic function SPREAD's arguments must be an INTEGER scalar NCOPIES, and this one is REAL"},
		{"CSHIFT's SHIFT of the wrong rank", unit("m = cshift(m, m)"),
			"ARRAYRANK: CSHIFT's SHIFT must be of rank 0 or 1, not 2"},
		{"CSHIFT's SHIFT not INTEGER", unit("v = cshift(v, 1.0)"),
			"BADARG: the intrinsic function CSHIFT's arguments must be an INTEGER SHIFT, and this one is REAL"},
		{"EOSHIFT's BOUNDARY of another type", unit("v = eoshift(v, 1, 'x')"),
			"BADARG: the intrinsic function EOSHIFT's arguments must be a BOUNDARY of ARRAY's type, and this one is CHARACTER*1"},
		{"PACK's MASK", unit("w = pack(m, m)"),
			"BADARG: the intrinsic function PACK's arguments must be a LOGICAL MASK, and this one is INTEGER"},
		{"PACK's VECTOR of another rank", unit("w = pack(m, l, m)"),
			"ARRAYRANK: PACK's VECTOR must be of rank 1, not 2"},
		{"UNPACK's MASK a scalar", unit("m = unpack(w, .true., 0)"),
			"ARRAYARG: UNPACK's MASK must be an array"},
		{"RESHAPE's SHAPE of a size not known", unit("m = reshape(v, v(1:n))"),
			"BADRESHAPE: this RESHAPE can't be computed: SHAPE's size isn't known when compiling"},
		{"RESHAPE's ORDER not constant (Phase 113)", unit("m = reshape(v, (/ 3, 3 /), order=w(1:2))",
			"m = reshape(v, (/ 3, 3 /), order=w(k:k+1))"), ""},
		{"RESHAPE's ORDER of another size", unit("m = reshape(v, (/ 3, 3 /), order=w)"),
			"BADRESHAPE: this RESHAPE can't be computed: ORDER's size isn't SHAPE's"},
		{"RESHAPE's ORDER not INTEGER", unit("m = reshape(v, (/ 3, 3 /), order=r(1:2))"),
			"ARRAYARG: RESHAPE's ORDER, an INTEGER array of rank one, must be an array"},
		{"RESHAPE's ORDER not a permutation", unit("m = reshape(v, (/ 3, n /), order=(/ 1, 1 /))"),
			"BADRESHAPE: this RESHAPE can't be computed: ORDER isn't a permutation of 1 to the shape's rank"},
		{"a shape that doesn't conform", unit("w = spread(w, 1, 2)"),
			"NOTCONFORM: the shapes (3) and (2, 3) don't conform"},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
