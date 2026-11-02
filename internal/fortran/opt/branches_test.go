package opt

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/ir"
	"github.com/tucats/govax/internal/fortran/sema"
)

// listing makes a routine's code from lines of text: "NAME:" a compiler's label,
// "=NAME:" a FORTRAN statement's, and otherwise an instruction. "FILL n" is n bytes
// of CLRL R0, two at a time, which a test uses for a distance.
func listing(lines ...string) *codegen.Code {
	c := &codegen.Code{
		Outside:       map[string]bool{},
		Displacements: map[string]int{"T.A": 4, "T.B": 8},
		Cells:         map[string]*ir.Cell{},
		Params:        map[string]*ir.Param{},
	}

	for i, name := range []string{"T.I", "T.N", "T.X"} {
		c.Cells[name] = &ir.Cell{Name: name, T: ir.Long, Size: 4, Offset: 4 * i}
	}

	c.Cells["T.SAME"] = &ir.Cell{Name: "T.SAME", T: ir.Long, Size: 4, Offset: 0}
	c.Cells["T.V"] = &ir.Cell{Name: "T.V", T: ir.Long, Size: 4, Offset: 40, Sym: &sema.Symbol{Volatile: true}}
	c.Params["@T.A(AP)"] = &ir.Param{Name: "T.A", Sym: &sema.Symbol{}}
	c.Params["@T.B(AP)"] = &ir.Param{Name: "T.B", Sym: &sema.Symbol{}}

	for _, line := range lines {
		op, rest, _ := strings.Cut(line, " ")

		switch {
		case op == "FILL":
			for n := 0; n < atoi(rest); n += 2 {
				c.Items = append(c.Items, &codegen.Item{Kind: codegen.Instruction, Opcode: "CLRL", Operands: []string{"R0"}})
			}

		case strings.HasPrefix(line, "="):
			c.Items = append(c.Items, &codegen.Item{Kind: codegen.Label, Name: strings.TrimSuffix(line[1:], ":"), Named: true})

		case strings.HasSuffix(line, ":"):
			c.Items = append(c.Items, &codegen.Item{Kind: codegen.Label, Name: strings.TrimSuffix(line, ":")})

		default:
			it := &codegen.Item{Kind: codegen.Instruction, Opcode: op}
			if rest != "" {
				it.Operands = strings.Split(rest, ",")
			}

			c.Items = append(c.Items, it)
		}
	}

	return c
}

// atoi is the number in text.
func atoi(text string) int {
	n := 0
	for _, d := range text {
		n = 10*n + int(d-'0')
	}

	return n
}

// shown is the code as listing's lines, FILL's instructions counted back into one.
func shown(c *codegen.Code) string {
	var lines []string

	fill := 0

	flush := func() {
		if fill > 0 {
			lines = append(lines, "FILL "+itoa(fill))
			fill = 0
		}
	}

	for _, it := range c.Items {
		switch {
		case it.Kind == codegen.Label:
			flush()

			if it.Named {
				lines = append(lines, "="+it.Name+":")
			} else {
				lines = append(lines, it.Name+":")
			}

		case it.Opcode == "CLRL" && it.Operands[0] == "R0":
			fill += 2

		default:
			flush()
			lines = append(lines, strings.TrimSpace(it.Opcode+" "+strings.Join(it.Operands, ",")))
		}
	}

	flush()

	return strings.Join(lines, "; ")
}

// itoa is n as text.
func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}

	return itoa(n/10) + string(rune('0'+n%10))
}

// TestBranchRules runs Branches on small routines.
func TestBranchRules(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"the opposite branch, when the target is near",
			[]string{"T:", "FILL 10", "BLEQ S", "BRW T", "S:"},
			"T:; FILL 10; BGTR T"},
		{"around a BRW stays, when it isn't near",
			[]string{"T:", "FILL 130", "BLEQ S", "BRW T", "S:"},
			"T:; FILL 130; BLEQ S; BRW T; S:"},
		{"the low bit's branches turn over too",
			[]string{"T:", "BLBC T.X,S", "BRW T", "S:"},
			"T:; BLBS T.X,T"},
		{"a jump that reaches is a BRB",
			[]string{"BNEQ L", "BRW T", "M:", "FILL 2", "L:", "BEQL M", "FILL 100", "T:", "FILL 2", "RET"},
			"BNEQ L; BRB T; M:; FILL 2; L:; BEQL M; FILL 100; T:; FILL 2; RET"},
		{"127 bytes forward reaches",
			[]string{"BEQL L", "BRW T", "L:", "FILL 126", "NOP", "T:", "RET"},
			"BNEQ T; FILL 126; NOP; T:; RET"},
		{"128 bytes forward doesn't",
			[]string{"BEQL L", "BRW T", "L:", "FILL 126", "NOP", "NOP", "T:", "FILL 2", "RET"},
			"BEQL L; BRW T; L:; FILL 126; NOP; NOP; T:; FILL 2; RET"},
		{"128 bytes back reaches, from the branch's end",
			[]string{"T:", "FILL 126", "BRW T"},
			"T:; FILL 126; BRB T"},
		{"130 bytes back doesn't",
			[]string{"T:", "FILL 128", "BRW T"},
			"T:; FILL 128; BRW T"},
		{"a branch to what comes next goes",
			[]string{"BRW S", "S:", "FILL 2", "BEQL U", "U:", "RET"},
			"FILL 2; RET"},
		{"a low bit's branch to what comes next reads its operand, and stays",
			[]string{"BLBC T.V,U", "U:", "RET"},
			"BLBC T.V,U; U:; RET"},
		{"a branch to a jump goes where the jump goes",
			[]string{"BEQL A", "FILL 8", "BRW Z", "A:", "BRW B", "B:", "FILL 2", "RET", "Z:", "FILL 2", "RET"},
			"BEQL B; FILL 8; BRB Z; B:; FILL 2; RET; Z:; FILL 2; RET"},
		{"a chain goes to its end",
			[]string{"BRW A", "FILL 2", "A:", "BRW B", "FILL 2", "B:", "BRW C", "FILL 2", "C:", "FILL 2", "RET"},
			"FILL 2; RET"},
		{"a branch isn't sent where it wouldn't reach",
			[]string{"BEQL A", "FILL 8", "BNEQ M", "RET", "A:", "BRW Z", "M:", "FILL 300", "Z:", "FILL 2", "RET"},
			"BEQL A; FILL 8; BNEQ M; RET; A:; BRW Z; M:; FILL 300; Z:; FILL 2; RET"},
		{"a jump to a RET is a RET",
			[]string{"FILL 2", "BRW Z", "FILL 2", "Z:", "RET"},
			"FILL 2; RET"},
		{"a loop of jumps ends the search",
			[]string{"A:", "BRW B", "B:", "BRW A"},
			"A:; BRB A"},
		{"a jump to itself stays",
			[]string{"A:", "BRW A"},
			"A:; BRB A"},
		{"code after a jump, with no label anything goes to, goes",
			[]string{"BRW Z", "FILL 4", "=T.L5:", "FILL 2", "RET", "Z:", "FILL 2", "RET"},
			"=T.L5:; FILL 2; RET"},
		{"a label outside code goes to stays, and what it starts",
			[]string{"RET", "OUT:", "FILL 2", "RET"},
			"RET; OUT:; FILL 2; RET"},
		{"a label only a table names stays",
			[]string{"CASEL T.X,#1,#0", "W:", ".WORD A-W", "RET", "A:", "RET"},
			"CASEL T.X,#1,#0; W:; .WORD A-W; RET; A:; RET"},
		{"a label only MOVAB names stays",
			[]string{"MOVAB A,R0", "JMP (R0)", "A:", "RET"},
			"MOVAB A,R0; JMP (R0); A:; RET"},

		{"DECL and BGTR is SOBGTR",
			[]string{"T:", "FILL 4", "DECL T.X", "BLEQ S", "BRW T", "S:"},
			"T:; FILL 4; SOBGTR T.X,T"},
		{"DECL and BGEQ is SOBGEQ",
			[]string{"T:", "DECL T.X", "BLSS S", "BRW T", "S:"},
			"T:; SOBGEQ T.X,T"},
		{"a DECL and another condition stays",
			[]string{"T:", "DECL T.X", "BGEQ S", "BRW T", "S:"},
			"T:; DECL T.X; BLSS T"},
		{"a far DECL stays",
			[]string{"T:", "FILL 130", "DECL T.X", "BLEQ S", "BRW T", "S:"},
			"T:; FILL 130; DECL T.X; BLEQ S; BRW T; S:"},
		{"a label between them keeps them",
			[]string{"BEQL S", "T:", "DECL T.X", "S:", "BGTR T"},
			"BEQL S; T:; DECL T.X; S:; BGTR T"},

		{"INCL, CMPL, and BLEQ is AOBLEQ",
			[]string{"T:", "FILL 4", "INCL T.I", "CMPL T.I,T.N", "BGTR S", "BRW T", "S:"},
			"T:; FILL 4; AOBLEQ T.N,T.I,T"},
		{"and BLSS is AOBLSS",
			[]string{"T:", "INCL T.I", "CMPL T.I,#10", "BLSS T"},
			"T:; AOBLSS #10,T.I,T"},
		{"the comparison the other way round",
			[]string{"T:", "INCL T.I", "CMPL T.N,T.I", "BGEQ T"},
			"T:; AOBLEQ T.N,T.I,T"},
		{"dummies by the standard's rule",
			[]string{"T:", "INCL @T.A(AP)", "CMPL @T.A(AP),@T.B(AP)", "BLEQ T"},
			"T:; AOBLEQ @T.B(AP),@T.A(AP),T"},
		{"a limit that overlaps the index",
			[]string{"T:", "INCL T.I", "CMPL T.I,T.SAME", "BLEQ T"},
			"T:; INCL T.I; CMPL T.I,T.SAME; BLEQ T"},
		{"a VOLATILE index",
			[]string{"T:", "INCL T.V", "CMPL T.V,#10", "BLEQ T"},
			"T:; INCL T.V; CMPL T.V,#10; BLEQ T"},
		{"a VOLATILE limit",
			[]string{"T:", "INCL T.I", "CMPL T.I,T.V", "BLEQ T"},
			"T:; INCL T.I; CMPL T.I,T.V; BLEQ T"},
		{"an operand nothing's known of",
			[]string{"T:", "INCL T.I", "CMPL T.I,T.Z[R0]", "BLEQ T"},
			"T:; INCL T.I; CMPL T.I,T.Z[R0]; BLEQ T"},
		{"a test of the condition codes after it, where it would go",
			[]string{"T:", "BEQL Z", "INCL T.I", "CMPL T.I,T.N", "BLEQ T", "Z:", "RET"},
			"T:; BEQL Z; INCL T.I; CMPL T.I,T.N; BLEQ T; Z:; RET"},
		{"a test of the condition codes after it, where it wouldn't",
			[]string{"T:", "INCL T.I", "CMPL T.I,T.N", "BLEQ T", "BEQL T", "RET"},
			"T:; INCL T.I; CMPL T.I,T.N; BLEQ T; BEQL T; RET"},

		{"a constant step up is ACBL",
			[]string{"T:", "ADDL2 #3,T.I", "CMPL T.I,T.N", "BLEQ T"},
			"T:; ACBL T.N,#3,T.I,T"},
		{"a constant step down, by SUBL2",
			[]string{"T:", "SUBL2 #3,T.I", "CMPL T.I,T.N", "BGEQ T"},
			"T:; ACBL T.N,#-3,T.I,T"},
		{"a constant step down, by DECL",
			[]string{"T:", "DECL T.I", "CMPL T.I,T.N", "BGEQ T"},
			"T:; ACBL T.N,#-1,T.I,T"},
		{"a condition that isn't the step's sign's",
			[]string{"T:", "ADDL2 #3,T.I", "CMPL T.I,T.N", "BGEQ T"},
			"T:; ADDL2 #3,T.I; CMPL T.I,T.N; BGEQ T"},
		{"BLSS has no ACBL",
			[]string{"T:", "ADDL2 #3,T.I", "CMPL T.I,T.N", "BLSS T"},
			"T:; ADDL2 #3,T.I; CMPL T.I,T.N; BLSS T"},
		{"a step that isn't a constant",
			[]string{"T:", "ADDL2 T.X,T.I", "CMPL T.I,T.N", "BLEQ T"},
			"T:; ADDL2 T.X,T.I; CMPL T.I,T.N; BLEQ T"},
		{"a far loop closes with ACBL, whose displacement is a word",
			[]string{"T:", "FILL 200", "INCL T.I", "CMPL T.I,T.N", "BGTR S", "BRW T", "S:"},
			"T:; FILL 200; ACBL T.N,#1,T.I,T"},
		{"too far for a word",
			[]string{"T:", "FILL 33000", "BRW T"},
			"T:; FILL 33000; BRW T"},
		{"a DECL of a register is shorter than ACBL's immediate",
			[]string{"T:", "DECL R6", "CMPL R6,R7", "BGEQ T"},
			"T:; DECL R6; CMPL R6,R7; BGEQ T"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code := listing(c.in...)

			if strings.Contains(strings.Join(c.in, " "), "OUT:") {
				code.Outside["OUT"] = true
			}

			Branches(code)

			if got := shown(code); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}

			if err := code.Check(); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestPeephole runs Peephole on each kind of instruction it replaces.
func TestPeephole(t *testing.T) {
	cases := map[string]string{
		"MOVL #0,R1":     "CLRL R1",
		"MOVW #0,T.X":    "CLRW T.X",
		"MOVB #0,T.X":    "CLRB T.X",
		"ADDL2 #1,T.X":   "INCL T.X",
		"ADDW2 #1,T.X":   "INCW T.X",
		"SUBL2 #1,T.X":   "DECL T.X",
		"SUBB2 #1,T.X":   "DECB T.X",
		"CMPL T.X,#0":    "TSTL T.X",
		"CMPW T.X,#0":    "TSTW T.X",
		"MOVL #1,R1":     "MOVL #1,R1",
		"ADDL2 #2,T.X":   "ADDL2 #2,T.X",
		"ADDF2 #1,T.X":   "ADDF2 #1,T.X",
		"MOVF T.X,T.N":   "MOVF T.X,T.N",
		"CMPL #0,T.X":    "CMPL #0,T.X",
		"CMPC3 #1,A,B":   "CMPC3 #1,A,B",
		"MOVC3 #0,A,B":   "MOVC3 #0,A,B",
		"MOVAB #0,R0":    "MOVAB #0,R0",
		"MOVZWL #0,R0":   "MOVZWL #0,R0",
		"ADDL3 #1,R0,R1": "ADDL3 #1,R0,R1",
	}

	for in, want := range cases {
		code := listing(in)
		Peephole(code)

		if got := shown(code); got != want {
			t.Errorf("%s became %s, want %s", in, got, want)
		}
	}
}
