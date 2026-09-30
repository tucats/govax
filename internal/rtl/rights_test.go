package rtl

import "testing"

func TestAsctoid(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	id, attrib := a.alloc(4), a.alloc(4)

	cases := []struct {
		name  string
		want  uint32
		value uint32
	}{
		{"SYSTEM", ssNormal, NominalUIC},
		{"interactive", ssNormal, 0x80000003},
		{" LOCAL ", ssNormal, 0x80000004},
		{"NOBODY", ssNoSuchID, 0},
		{"", ssIvIdent, 0},
		{"12345", ssIvIdent, 0},
		{"BAD NAME", ssIvIdent, 0},
		{"A23456789012345678901234567890123", ssIvIdent, 0},
	}

	for _, tc := range cases {
		putLongword(t, env, id, 0xFFFF)
		putLongword(t, env, attrib, 0xFFFF)

		got := callLNM(t, env, serviceSysAsctoid, a.desc(tc.name), id, attrib)
		if got != tc.want {
			t.Errorf("%q: %#x, want %#x", tc.name, got, tc.want)

			continue
		}

		if tc.want == ssNormal && (a.readLong(id) != tc.value || a.readLong(attrib) != 0) {
			t.Errorf("%q: id %#x attrib %#x", tc.name, a.readLong(id), a.readLong(attrib))
		}
	}

	wantR0(t, callLNM(t, env, serviceSysAsctoid, a.desc("SYSTEM")), ssNormal) // outputs optional
	wantR0(t, callLNM(t, env, serviceSysAsctoid, badAddr), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysAsctoid, a.desc("SYSTEM"), badAddr), ssAccVio)
}

func TestIdtoasc(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	namlen, resid := a.alloc(2), a.alloc(4)
	nambuf, buf := a.outDesc(31)

	wantR0(t, callLNM(t, env, serviceSysIdtoasc, 0x80000002, namlen, nambuf, resid), ssNormal)

	if n := uint16(a.readLong(namlen)); a.readString(buf, n) != "NETWORK" || a.readLong(resid) != 0x80000002 {
		t.Errorf("name %q resid %#x", a.readString(buf, n), a.readLong(resid))
	}

	wantR0(t, callLNM(t, env, serviceSysIdtoasc, 0x12345, namlen, nambuf), ssNoSuchID)

	// Too small a buffer: truncated.
	small, sbuf := a.outDesc(3)
	wantR0(t, callLNM(t, env, serviceSysIdtoasc, NominalUIC, 0, small), ssBufferOvf)

	if a.readString(sbuf, 3) != "SYS" {
		t.Errorf("truncated name %q", a.readString(sbuf, 3))
	}

	// A listing, in name order, then SS$_NOSUCHID with the context
	// cleared.
	ctx := a.alloc(4)
	putLongword(t, env, ctx, 0)

	var names []string

	for {
		r0 := callLNM(t, env, serviceSysIdtoasc, 0xFFFFFFFF, namlen, nambuf, 0, 0, ctx)
		if r0 == ssNoSuchID {
			break
		}

		wantR0(t, r0, ssNormal)
		
		names = append(names, a.readString(buf, uint16(a.readLong(namlen))))

		if len(names) > 20 {
			t.Fatal("the listing doesn't end")
		}
	}

	want := []string{"BATCH", "DIALUP", "INTERACTIVE", "LOCAL", "NETWORK", "REMOTE", "SYSTEM"}
	if len(names) != len(want) {
		t.Fatalf("listing %v, want %v", names, want)
	}

	for i := range want {
		if names[i] != want[i] {
			t.Errorf("listing %v, want %v", names, want)
		}
	}

	if a.readLong(ctx) != 0 {
		t.Error("the context should be cleared after the last identifier")
	}

	// $FINISH_RDB ends a listing early.
	callLNM(t, env, serviceSysIdtoasc, 0xFFFFFFFF, 0, 0, 0, 0, ctx)
	wantR0(t, callLNM(t, env, serviceSysFinishRdb, ctx), ssNormal)

	if a.readLong(ctx) != 0 {
		t.Error("$FINISH_RDB should clear the context")
	}

	wantR0(t, callLNM(t, env, serviceSysIdtoasc, 0xFFFFFFFF), ssAccVio) // no context
	wantR0(t, callLNM(t, env, serviceSysFinishRdb, badAddr), ssAccVio)
}
