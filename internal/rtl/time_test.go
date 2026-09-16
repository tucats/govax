package rtl

import (
	"testing"
	"time"
)

func TestShimDeccTime(t *testing.T) {
	env, _ := fixture()
	timeAddr := uint32(0x1000)

	before := uint32(time.Now().Unix())
	r0, err := shimDeccTime(env, []uint32{timeAddr})
	after := uint32(time.Now().Unix())

	if err != nil {
		t.Fatalf("shimDeccTime: %v", err)
	}
	
	if r0 < before || r0 > after {
		t.Errorf("r0 = %d, want within [%d,%d]", r0, before, after)
	}

	stored, err := env.mem.LoadLongword(env.cpu, timeAddr)
	if err != nil {
		t.Fatal(err)
	}

	if stored != r0 {
		t.Errorf("stored time = %d, want %d (same as r0)", stored, r0)
	}
}

func TestShimDeccTimeNoAddress(t *testing.T) {
	env, _ := fixture()
	if _, err := shimDeccTime(env, []uint32{0}); err != nil {
		t.Fatalf("shimDeccTime: %v", err)
	}
}
