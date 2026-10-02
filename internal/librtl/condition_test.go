package librtl

import "testing"

func TestLibMatchCond(t *testing.T) {
	env := fixture(t)

	const base = 0x6000

	values := []uint32{
		0x0000000C | 0x10000000, // SS$_ACCVIO, inhibit-message bit set
		0x00000870,              // SS$_ENDOFFILE
		0x0000000A,              // SS$_ACCVIO's ID, severity ERROR
		0x0000000C,
	}

	for i, v := range values {
		putLongword(t, env, base+uint32(4*i), v)
	}

	ref := func(i int) uint32 { return base + uint32(4*i) }

	cases := []struct {
		argv []uint32
		want uint32
	}{
		{[]uint32{ref(0), ref(1), ref(2), ref(3)}, 2}, // severity and control bits ignored
		{[]uint32{ref(1), ref(0), ref(1)}, 2},
		{[]uint32{ref(1), ref(0)}, 0},
		{[]uint32{ref(0), 0x7FFF0000, ref(3)}, 2}, // unreadable: no match
		{[]uint32{ref(0)}, 0},
	}

	for _, tc := range cases {
		if got, err := libMatchCond(env, tc.argv); err != nil || got != tc.want {
			t.Errorf("LIB$MATCH_COND%v = %d, %v; want %d", tc.argv, got, err, tc.want)
		}
	}
}
