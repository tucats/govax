package vaxfloat

import "testing"

// TestModel checks each format's model against the values Phase 100's survey
// (A.1's table) gives, and that HUGE, TINY, and EPSILON are values of the
// format.
func TestModel(t *testing.T) {
	cases := []struct {
		f                   Format
		digits, emin, emax  int
		huge, tiny, epsilon string
	}{
		{F, 24, -127, 127, "1.70141173e+38", "2.93873588e-39", "1.19209290e-07"},
		{D, 56, -127, 127, "1.70141183e+38", "2.93873588e-39", "2.77555756e-17"},
		{G, 53, -1023, 1023, "8.98846567e+307", "5.56268465e-309", "2.22044605e-16"},
		{H, 113, -16383, 16383, "5.94865748e+4931", "8.40525786e-4933", "1.92592994e-34"},
	}

	for _, c := range cases {
		if c.f.Digits() != c.digits || c.f.MinExponent() != c.emin || c.f.MaxExponent() != c.emax {
			t.Errorf("%v: model %d, %d, %d", c.f, c.f.Digits(), c.f.MinExponent(), c.f.MaxExponent())
		}

		for _, v := range []struct {
			name, want string
			x          Value
		}{{"huge", c.huge, c.f.Huge()}, {"tiny", c.tiny, c.f.Tiny()}, {"epsilon", c.epsilon, c.f.Epsilon()}} {
			if got := v.x.Text('e', 8); got != v.want {
				t.Errorf("%v %s = %s, want %s", c.f, v.name, got, v.want)
			}

			if r, err := Round(c.f, v.x); err != nil || r.Cmp(v.x) != 0 {
				t.Errorf("%v %s isn't the format's: %v", c.f, v.name, err)
			}
		}
	}
}

// TestModelFunctions checks the model's functions against the standard's
// examples (13.13.33 and the rest, for REAL*4) and each format's edges.
func TestModelFunctions(t *testing.T) {
	must := func(v Value, err error) Value {
		t.Helper()

		if err != nil {
			t.Fatal(err)
		}

		return v
	}

	text := func(v Value) string { return v.Text('g', 10) }

	for _, c := range []struct {
		name string
		got  Value
		want string
	}{
		{"FRACTION(3.0)", Fraction(FromInt(3)), "0.75"},
		{"FRACTION(-10.0)", Fraction(FromInt(-10)), "-0.625"},
		{"SCALE(3.0, 2)", must(F.Scale(FromInt(3), 2)), "12"},
		{"SET_EXPONENT(3.0, 1)", must(F.SetExponent(FromInt(3), 1)), "1.5"},
		{"SPACING(3.0)", F.Spacing(FromInt(3)), "2.384185791e-07"},
		{"RRSPACING(-3.0)", F.RRSpacing(FromInt(-3)), "12582912"},
		{"NEAREST(3.0, 2.0)", must(F.Nearest(FromInt(3), true)), "3.000000238"},
		{"NEAREST(1.0, -1.0)", must(F.Nearest(FromInt(1), false)), "0.9999999404"},
		{"NEAREST(-1.0, 1.0)", must(F.Nearest(FromInt(-1), true)), "-0.9999999404"},
		{"NEAREST(0.0, -1.0)", must(F.Nearest(Value{}, false)), "-2.938735877e-39"},
		{"NEAREST(TINY, -1.0)", must(F.Nearest(F.Tiny(), false)), "0"},
		{"SPACING(0.0)", F.Spacing(Value{}), "2.938735877e-39"},
		{"SPACING(TINY)", F.Spacing(F.Tiny()), "2.938735877e-39"},
		{"SCALE(TINY, -1)", must(F.Scale(F.Tiny(), -1)), "0"},
		{"SPACING(1D0)", D.Spacing(FromInt(1)), "2.775557562e-17"},
		{"SPACING(1G0)", G.Spacing(FromInt(1)), "2.220446049e-16"},
		{"RRSPACING(1Q0)", H.RRSpacing(FromInt(1)), "5.192296859e+33"},
	} {
		if got := text(c.got); got != c.want {
			t.Errorf("%s = %s, want %s", c.name, got, c.want)
		}
	}

	if e := Exponent(FromInt(10)); e != 4 {
		t.Errorf("EXPONENT(10.0) = %d, want 4", e)
	}

	if e := Exponent(F.Tiny()); e != -127 {
		t.Errorf("EXPONENT(TINY) = %d, want -127", e)
	}

	for _, f := range []Format{F, D, G, H} {
		if _, err := f.Nearest(f.Huge(), true); err != ErrOverflow {
			t.Errorf("%v NEAREST(HUGE, 1) = %v, want overflow", f, err)
		}

		if _, err := f.Scale(f.Huge(), 1); err != ErrOverflow {
			t.Errorf("%v SCALE(HUGE, 1) = %v, want overflow", f, err)
		}

		if v := must(f.Nearest(f.Huge(), false)); v.Cmp(f.Huge()) >= 0 || must(f.Nearest(v, true)).Cmp(f.Huge()) != 0 {
			t.Errorf("%v NEAREST below HUGE and back", f)
		}

		if v := must(f.Nearest(FromInt(1), false)); must(Round(f, v)).Cmp(v) != 0 {
			t.Errorf("%v NEAREST(1, -1) isn't the format's", f)
		}
	}
}
