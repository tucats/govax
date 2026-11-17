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
