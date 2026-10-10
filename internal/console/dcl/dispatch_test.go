package dcl

import "testing"

const (
	testShowQuantumCommand = "SHOW QUANTUM"
	testShowQuantumBinding = "SHOW_QUANTUM"
)

func TestDispatch(t *testing.T) {
	var (
		gotID   int64
		gotWhat string
	)

	g := loadDebugGrammar(t)

	g.Bind(testShowQuantumBinding, func(id int64, r *Result) error {
		gotID = id
		gotWhat = r.Active

		return nil
	})

	r, err := g.Parse(testShowQuantumCommand)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if err := g.Dispatch(r); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if gotWhat != testShowQuantumBinding || gotID != r.ActiveID {
		t.Errorf("handler got id=%d active=%s, want id=%d active=%s", gotID, gotWhat, r.ActiveID, testShowQuantumBinding)
	}
}

func TestDispatch_noHandlerBound(t *testing.T) {
	g := loadDebugGrammar(t)

	r, err := g.Parse(testShowQuantumCommand)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if err := g.Dispatch(r); err == nil {
		t.Error("expected error for unbound entry")
	}
}
