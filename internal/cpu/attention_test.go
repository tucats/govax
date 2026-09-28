package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// attentionFake is fakeServices plus an AttentionHandler that takes the
// key when take is set, recording the keys it's offered.
type attentionFake struct {
	fakeServices

	take bool
	keys []byte
}

func (f *attentionFake) HandleAttention(key byte) bool {
	f.keys = append(f.keys, key)

	return f.take
}

func TestAttention_handlerTakesKey(t *testing.T) {
	e := newEngine()
	c := e.cpu

	putBytes(t, c, e.mem, 0x1000, 0x01, 0x01) // NOP, NOP
	c.SetGPR(vax.PC, 0x1000)

	fake := &attentionFake{take: true}
	e.SetSystemServices(fake)

	// A taken CTRL/Y: the instruction runs, and the key is gone.
	e.AttentionKey(AttentionCtrlY)

	if err := e.Step(); err != nil {
		t.Fatalf("Step with a taken key = %v, want nil", err)
	}

	if len(fake.keys) != 1 || fake.keys[0] != AttentionCtrlY || e.AttentionRequested() {
		t.Errorf("offered %v, still requested %v; want [CTRL/Y], false", fake.keys, e.AttentionRequested())
	}

	if c.GPR(vax.PC) != 0x1001 {
		t.Errorf("PC = %#x, want the NOP executed (0x1001)", c.GPR(vax.PC))
	}

	// Without a pending key the handler isn't asked.
	if err := e.Step(); err != nil || len(fake.keys) != 1 {
		t.Errorf("Step = %v with %d offers; want nil, still 1", err, len(fake.keys))
	}
}

func TestAttention_handlerDeclines(t *testing.T) {
	e := newEngine()
	fake := &attentionFake{}
	e.SetSystemServices(fake)

	// Attention is CTRL/C. Declined, it stops the machine, and stays
	// pending until BeginRun.
	e.Attention()

	for i := 0; i < 2; i++ {
		if err := e.Step(); !errors.Is(err, ErrAttention) {
			t.Fatalf("Step %d = %v, want ErrAttention", i, err)
		}
	}

	if len(fake.keys) == 0 || fake.keys[0] != AttentionCtrlC {
		t.Errorf("offered %v, want CTRL/C", fake.keys)
	}

	e.BeginRun()

	if e.AttentionRequested() {
		t.Error("BeginRun left the key pending")
	}
}

// TestAttention_noHandler: services without an AttentionHandler always
// stop, as before.
func TestAttention_noHandler(t *testing.T) {
	e := newEngine()
	e.SetSystemServices(&fakeServices{})
	e.AttentionKey(AttentionCtrlY)

	if err := e.Step(); !errors.Is(err, ErrAttention) {
		t.Errorf("Step = %v, want ErrAttention", err)
	}
}
