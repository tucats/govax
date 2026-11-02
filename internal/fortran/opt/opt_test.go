package opt

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/ir"
)

// routine is the smallest correct routine: one block that returns.
func routine(name string) *ir.Routine {
	return &ir.Routine{Name: name, Kind: ir.Subroutine, Blocks: []*ir.Block{{Name: "B1", End: &ir.Return{}}}}
}

// TestRunOrder checks that every pass sees every routine, in the list's order.
func TestRunOrder(t *testing.T) {
	var seen []string

	note := func(pass string) Pass {
		return Pass{Name: pass, Run: func(r *ir.Routine) { seen = append(seen, pass+":"+r.Name) }}
	}

	run([]*ir.Routine{routine("A"), routine("B")}, []Pass{note("one"), note("two")})

	want := "one:A two:A one:B two:B"
	if got := strings.Join(seen, " "); got != want {
		t.Errorf("ran %s, want %s", got, want)
	}
}

// TestBrokenRoutine checks that a pass that breaks a routine is named.
func TestBrokenRoutine(t *testing.T) {
	defer func() {
		text, _ := recover().(string)

		for _, want := range []string{"SUB", "after breaks it", "terminator"} {
			if !strings.Contains(text, want) {
				t.Errorf("the panic %q doesn't say %q", text, want)
			}
		}
	}()

	breaks := Pass{Name: "breaks it", Run: func(r *ir.Routine) { r.Blocks[0].End = nil }}
	run([]*ir.Routine{routine("SUB")}, []Pass{breaks})
	t.Error("no panic")
}

// TestBrokenByLowering checks that a routine wrong before any pass isn't blamed
// on one.
func TestBrokenByLowering(t *testing.T) {
	defer func() {
		if text, _ := recover().(string); !strings.Contains(text, "after lowering") {
			t.Errorf("the panic was %q", text)
		}
	}()

	r := routine("SUB")
	r.Blocks[0].End = nil
	run([]*ir.Routine{r}, nil)
	t.Error("no panic")
}

// TestNames checks that the list's names come back in order.
func TestNames(t *testing.T) {
	if got, want := len(Names()), len(Passes); got != want {
		t.Fatalf("%d names of %d passes", got, want)
	}

	for i, p := range Passes {
		if Names()[i] != p.Name {
			t.Errorf("name %d is %s, want %s", i, Names()[i], p.Name)
		}
	}
}
