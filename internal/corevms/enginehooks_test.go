package corevms

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/cpu"
)

func TestEngineError(t *testing.T) {
	if !errors.Is(EngineError(ErrHalt), cpu.ErrHalted) {
		t.Error("EngineError(ErrHalt) does not unwrap to cpu.ErrHalted")
	}

	other := errors.New("boom")
	if !errors.Is(EngineError(other), other) {
		t.Error("EngineError should pass through any other error unchanged")
	}

	if EngineError(nil) != nil {
		t.Error("EngineError(nil) should be nil")
	}
}
