package opt

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/ir"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/parse"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/fortran/source"
)

// probed is one instruction whose bound a test checks: its text, its bound, and
// the labels just before and just after it.
type probed struct {
	text          string
	bound         int
	before, after string
}

// compileProbed compiles a FORTRAN file with every instruction between two labels
// of the test's own, which assemble to nothing, so that MACRO's symbol table says
// how long each instruction came out. It reports false for a file the compiler
// can't do alone, one with an INCLUDE, say.
func compileProbed(t *testing.T, path string, optimized bool) ([]string, []probed, bool) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var messages diag.List

	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	program := source.Read(source.Form{}, &source.File{Spec: path, Lines: lines}, nil, &messages)
	tree := parse.Parse(lex.Lex(program.Statements, &messages), &messages)
	prog := sema.Analyze(tree, "T$MAIN", sema.Options{}, &messages)

	if messages.Failed() {
		return nil, nil, false
	}

	var (
		list  []probed
		count int
	)

	probe := func(c *codegen.Code) {
		if optimized {
			Improve(c)
		}

		items := make([]*codegen.Item, 0, 3*len(c.Items))

		for _, it := range c.Items {
			if it.Kind != codegen.Instruction {
				items = append(items, it)

				continue
			}

			count++
			before, after := "Z$"+strconv.Itoa(count)+"A", "Z$"+strconv.Itoa(count)+"B"
			list = append(list, probed{it.Opcode + " " + strings.Join(it.Operands, ","), c.Bound(it), before, after})
			items = append(items,
				&codegen.Item{Kind: codegen.Label, Name: before}, it,
				&codegen.Item{Kind: codegen.Label, Name: after})
		}

		c.Items = items
	}

	var w codegen.Writer

	w.Statement("", ".TITLE", "TEST", "")
	codegen.Generate(&w, ir.Lower(prog), program.Lines, codegen.Options{Improve: probe})

	return w.Lines(), list, true
}

// TestBoundsHold holds Code.Bound to what MACRO assembles, instruction by
// instruction, in every program to hand, both as the generator wrote them and as
// the passes left them: a pass that adds bounds up believes a branch reaches, and a
// bound too small would be a program MACRO refuses. It logs how close the bounds
// are, since one too big costs a branch that could have been short.
func TestBoundsHold(t *testing.T) {
	var files []string

	for _, pattern := range []string{"../codegen/testdata/*.for", "../../../testdata/fortran/*.for", "../../../testdata/opt84/kernels/*.for"} {
		found, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}

		files = append(files, found...)
	}

	checked, instructions, bytesBound, bytesActual := 0, 0, 0, 0

	for _, path := range files {
		for _, optimized := range []bool{false, true} {
			lines, list, ok := compileProbed(t, path, optimized)
			if !ok {
				continue
			}

			checked++

			a := asm.New(false)
			a.SetDialect(asm.DialectMACRO)
			a.SetMacroLibraries(systemMacros(t))

			if _, err := a.Assemble(strings.Join(lines, "\n") + "\n"); err != nil {
				var all *asm.Errors
				if errors.As(err, &all) {
					err = errors.Join(all.List...)
				}

				t.Errorf("%s (optimized %v) doesn't assemble: %v", path, optimized, err)

				continue
			}

			symbols := a.Symbols()

			for _, p := range list {
				before, ok1 := symbols.Get(p.before)
				after, ok2 := symbols.Get(p.after)

				if !ok1 || !ok2 {
					t.Fatalf("%s: no symbol %s or %s", path, p.before, p.after)
				}

				actual := int(after.Value) - int(before.Value)

				if actual > p.bound {
					t.Errorf("%s: %s assembled to %d bytes, but its bound is %d", path, p.text, actual, p.bound)
				}

				instructions++
				bytesBound += p.bound
				bytesActual += actual
			}
		}
	}

	if checked < 100 {
		t.Errorf("only %d compilations checked", checked)
	}

	t.Logf("%d compilations, %d instructions: bounds total %d bytes, MACRO's %d", checked, instructions, bytesBound, bytesActual)
}

// systemMacros is govax's own system macro library, which the compiler's
// MACRO files call ($DSCDEF).
func systemMacros(t *testing.T) asm.MacroLibrary {
	t.Helper()

	lib, err := asm.SystemMacros()
	if err != nil {
		t.Fatal(err)
	}

	return lib
}
