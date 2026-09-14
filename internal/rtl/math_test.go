package rtl

import "testing"

func TestShimLibAdawi(t *testing.T) {
	cases := []struct {
		name      string
		sum, base uint32
		wantBase  uint32
		wantSign  uint32
	}{
		{"positive result", 5, 10, 15, 1},
		{"zero result", 0, 0, 0, 0},
		{"negative result", 0xFFFFFFFF /* -1 */, 0, 0xFFFFFFFF, 0xFFFFFFFF},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, _ := fixture()
			sumAddr, baseAddr, signAddr := uint32(0x1000), uint32(0x1004), uint32(0x1008)
			putLongword(t, env, sumAddr, c.sum)
			putLongword(t, env, baseAddr, c.base)

			r0, err := shimLibAdawi(env, []uint32{sumAddr, baseAddr, signAddr})
			if err != nil {
				t.Fatalf("shimLibAdawi: %v", err)
			}
			if r0 != 1 {
				t.Errorf("r0 = %d, want 1 (matches lib_adawi's own return, not VAX_OK)", r0)
			}

			sign, err := env.mem.LoadLongword(env.cpu, signAddr)
			if err != nil {
				t.Fatal(err)
			}
			if sign != c.wantSign {
				t.Errorf("sign = %#x, want %#x", sign, c.wantSign)
			}
		})
	}
}

func TestRegisterMathShims(t *testing.T) {
	env, _ := fixture()
	if _, ok := env.shims.Lookup(1); !ok {
		t.Error("shim code 1 (LIB$ADAWI) not registered")
	}
}
