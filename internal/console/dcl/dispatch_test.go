package dcl

import "testing"

func TestDispatch(t *testing.T) {
	g := loadEvaxGrammar(t)

	var gotID int64
	var gotWhat string
	g.Bind("SHOW_MEMORY", func(id int64, r *Result) error {
		gotID = id
		gotWhat = r.Active
		return nil
	})

	r, err := g.Parse("SHOW MEMORY")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := g.Dispatch(r); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if gotWhat != "SHOW_MEMORY" || gotID != r.ActiveID {
		t.Errorf("handler got id=%d active=%s, want id=%d active=SHOW_MEMORY", gotID, gotWhat, r.ActiveID)
	}
}

func TestDispatch_noHandlerBound(t *testing.T) {
	g := loadEvaxGrammar(t)
	r, err := g.Parse("SHOW MEMORY")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := g.Dispatch(r); err == nil {
		t.Error("expected error for unbound entry")
	}
}
