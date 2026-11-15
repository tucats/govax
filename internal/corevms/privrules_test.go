package corevms

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// TestNoUncheckedPrivileges holds govax to Decision 1 of
// docs/planning/PHASE-94 - privileges.md: no code decides whether an
// operation is allowed by reading a process's current privileges
// (CurrentPrivileges) itself; it asks the gate (privrules.go). It scans
// every Go file of the repository's internal and cmd trees but tests,
// and fails on a function that reads CurrentPrivileges and isn't in
// managedPrivileges, the places that manage the masks rather than
// check them, and on an entry there that no longer reads it, so that the
// list stays the truth.
func TestNoUncheckedPrivileges(t *testing.T) {
	found := map[string]bool{}

	for _, root := range []string{"../../internal", "../../cmd"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}

			rel, _ := filepath.Rel("../..", path)
			for _, fn := range privilegeReaders(t, path) {
				key := filepath.ToSlash(rel) + ":" + fn
				found[key] = true

				if _, ok := managedPrivileges[key]; !ok {
					t.Errorf("%s reads CurrentPrivileges: check through the gate (requirePrivilege, holdsPrivilege) with a rule, or name it in managedPrivileges with the reason", key)
				}
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	for key := range managedPrivileges {
		if !found[key] {
			t.Errorf("%s is in managedPrivileges but doesn't read CurrentPrivileges: remove it", key)
		}
	}
}

// privilegeReaders are the functions in the Go file path that mention a
// CurrentPrivileges field, and the package variables whose values do
// (a table of function literals, as $GETJPI's items are).
func privilegeReaders(t *testing.T, path string) []string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	var readers []string

	mentions := func(n ast.Node) bool {
		reads := false

		ast.Inspect(n, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "CurrentPrivileges" {
				reads = true
			}

			return true
		})

		return reads
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if mentions(d) {
				readers = append(readers, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok && mentions(vs) {
					readers = append(readers, vs.Names[0].Name)
				}
			}
		}
	}

	return readers
}

// managedPrivileges are the functions allowed to read CurrentPrivileges,
// by file and name, each with why.
var managedPrivileges = map[string]string{
	"internal/corevms/privrules.go:privilegeGranted":        "the gate",
	"internal/corevms/privilege.go:resetImagePrivileges":    "image rundown sets the mask",
	"internal/corevms/knownfiles.go:StartImage":             "image activation adds an installed image's privileges",
	"internal/corevms/privilege.go:serviceSysSetprv":        "$SETPRV returns the previous mask",
	"internal/corevms/privilege.go:setPrivileges":           "$SETPRV changes the mask",
	"internal/corevms/lexical.go:SetProcessPrivileges":      "SET PROCESS/PRIVILEGES returns the previous mask",
	"internal/corevms/creprc.go:CreateProcess":              "$CREPRC copies the creator's mask to the new process",
	"internal/corevms/spawn.go:Spawn":                       "LIB$SPAWN asks $CREPRC for the creator's mask",
	"internal/corevms/getjpi.go:jpiItemsByName":             "JPI$_CURPRIV returns the mask",
	"internal/corevms/uicprot.go:Credentials":               "the protection rule's credentials carry the privileges that override protection",
	"internal/dclcmd/privileges.go:processPrivilegesReport": "SHOW PROCESS/PRIVILEGES shows the mask",
}

// TestEveryServiceHasPrivilegeRules holds the table to Decision 2: every
// service govax registers says which rules it asks the gate about (or
// that it asks about none), and serviceRules names no service govax
// lacks.
func TestEveryServiceHasPrivilegeRules(t *testing.T) {
	env, _ := fixture()

	for name := range env.services.entries {
		if _, ok := serviceRules[name]; !ok {
			t.Errorf("%s isn't in serviceRules: name the privilege rules it checks, or put it in unprivilegedServices", name)
		}
	}

	for name := range serviceRules {
		if _, ok := env.services.entries[name]; !ok {
			t.Errorf("serviceRules names %s, which isn't registered", name)
		}
	}

	seen := map[string]bool{}

	for _, list := range [][]string{affectingServices, unprivilegedServices} {
		for _, name := range list {
			if seen[name] {
				t.Errorf("%s is listed twice", name)
			}

			seen[name] = true
		}
	}
}

// TestEveryPrivilegeRuleUsed holds the table to Decision 2 the other
// way: every rule is named by some code outside the table (a check), and
// by some service's list in serviceRules; and no two rules share a name.
func TestEveryPrivilegeRuleUsed(t *testing.T) {
	declared := ruleVariables(t)
	if len(declared) != len(privilegeRules) {
		t.Fatalf("privrules.go declares %d rule variables, privilegeRules has %d rules", len(declared), len(privilegeRules))
	}

	used := map[string]bool{}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "privrules.go" {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && declared[id.Name] {
				used[id.Name] = true
			}

			return true
		})
	}

	var unused []string

	for name := range declared {
		if !used[name] {
			unused = append(unused, name)
		}
	}

	sort.Strings(unused)

	for _, name := range unused {
		t.Errorf("rule %s is checked nowhere: use it, or remove it", name)
	}

	listed := map[*PrivilegeRule]bool{}

	for _, rules := range serviceRules {
		for _, r := range rules {
			listed[r] = true
		}
	}

	for _, r := range commandRules {
		listed[r] = true
	}

	names := map[string]bool{}

	for _, r := range privilegeRules {
		if !listed[r] {
			t.Errorf("rule %q is in no service's list in serviceRules, nor in commandRules", r.Name)
		}

		if names[r.Name] {
			t.Errorf("two rules are called %q", r.Name)
		}

		names[r.Name] = true
	}
}

// ruleVariables are the names of privrules.go's package variables made
// by newRule.
func ruleVariables(t *testing.T) map[string]bool {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "privrules.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}

	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}

		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)

			for i, value := range vs.Values {
				call, ok := value.(*ast.CallExpr)
				if !ok {
					continue
				}

				if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "newRule" {
					names[vs.Names[i].Name] = true
				}
			}
		}
	}

	return names
}

// TestPrivilegeRuleTable prints the rules as the phase doc's A.5 has
// them (go test -run TestPrivilegeRuleTable -v), and checks each row is
// whole.
func TestPrivilegeRuleTable(t *testing.T) {
	table := PrivilegeRuleTable()

	for _, line := range strings.Split(strings.TrimSpace(table), "\n") {
		if strings.Count(line, "|") != 6 || strings.Contains(line, "||") {
			t.Errorf("row %q", line)
		}
	}

	t.Log("\n" + table)
}

// TestPrivilegeGate: the gate grants by all of or any of a rule's
// privileges, lets an exempt mode through, returns the rule's status,
// and traces each decision under SET DEBUG PRIVILEGES.
func TestPrivilegeGate(t *testing.T) {
	env, _ := fixture()
	p := env.Process

	var trace bytes.Buffer

	env.cpu.SetDebugWriter(&trace)
	env.cpu.SetDebug(registers.DebugPrivileges)
	setMode(env, registers.User, registers.User, 0x9000)

	// Any of: SYSPRV alone sets another's mailbox's protection.
	p.CurrentPrivileges = privSYSPRV
	if st := env.requirePrivilege(ruleMailboxSetProt); st != 0 {
		t.Errorf("SYSPRV alone: %#x", st)
	}

	p.CurrentPrivileges = privTMPMBX
	if st := env.requirePrivilege(ruleMailboxSetProt); st != ssNoPriv {
		t.Errorf("neither: %#x, want SS$_NOPRIV", st)
	}

	// The rule's own status, and an exempt mode.
	if st := env.requirePrivilege(ruleEnqSystem); st != ruleEnqSystem.Status || st == ssNoPriv {
		t.Errorf("SYSLCK from user mode: %#x", st)
	}

	if st := env.requirePrivilegeAt(ruleEnqSystem, registers.Executive); st != 0 {
		t.Errorf("SYSLCK from executive mode: %#x", st)
	}

	if env.holdsPrivilege(ruleCmkrnl) {
		t.Error("CMKRNL held from user mode without it")
	}

	setMode(env, registers.Kernel, registers.User, 0x9000)

	if !env.holdsPrivilege(ruleCmkrnl) {
		t.Error("CMKRNL refused to a kernel-mode caller")
	}

	// The trace: a line a decision.
	lines := strings.Split(strings.TrimSpace(trace.String()), "\n")
	want := []string{
		"USER mode: mailbox IO$_SETPROTECTION, not the owner (needs BYPASS or SYSPRV): holds SYSPRV",
		"USER mode: mailbox IO$_SETPROTECTION, not the owner (needs BYPASS or SYSPRV): refused, SS$_NOPRIV",
		"USER mode: $ENQ system-wide resource (needs SYSLCK): refused, SS$_NOSYSLCK",
		"EXECUTIVE mode: $ENQ system-wide resource (needs SYSLCK): exempt in its mode",
		"USER mode: $CMKRNL (needs CMKRNL): refused, SS$_NOPRIV",
		"KERNEL mode: $CMKRNL (needs CMKRNL): exempt in its mode",
	}

	if len(lines) != len(want) {
		t.Fatalf("trace:\n%s", trace.String())
	}

	for i, line := range lines {
		if !strings.HasPrefix(line, "DEBUG(PRIVILEGES): ") || !strings.HasSuffix(line, want[i]) {
			t.Errorf("trace line %d: %q, want ...%q", i+1, line, want[i])
		}
	}

	// Off, nothing.
	trace.Reset()
	env.cpu.SetDebug(0)
	env.requirePrivilege(ruleLckpag)

	if trace.Len() != 0 {
		t.Errorf("traced with the switch off: %q", trace.String())
	}
}
