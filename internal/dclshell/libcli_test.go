package dclshell

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// TestShellCLI_symbols: the LIB$ symbol routines' view of a Shell's
// tables. Symbol finds a local symbol of an outer command level (table
// 1) before a global one (table 2), and takes abbreviations; SetSymbol
// writes the current level's local table or the global one, an "*"
// marking the abbreviation; DeleteSymbol deletes from the table named.
func TestShellCLI_symbols(t *testing.T) {
	shell := &Shell{}
	cli := shellCLI{shell}

	if st := cli.SetSymbol("DIR*ECTORY", "global value", corevms.CLIGlobalSymbol); st != ssNormal {
		t.Fatalf("SetSymbol global: %#x", st)
	}

	if st := cli.SetSymbol("OUTER", "outer", corevms.CLILocalSymbol); st != ssNormal {
		t.Fatalf("SetSymbol local: %#x", st)
	}

	shell.dclSymbols.push() // a command procedure's level

	for _, c := range []struct {
		name, value string
		table       uint32
	}{
		{"OUTER", "outer", corevms.CLILocalSymbol},
		{"DIR", "global value", corevms.CLIGlobalSymbol},
		{"DIRECT", "global value", corevms.CLIGlobalSymbol},
	} {
		value, table, st := cli.Symbol(c.name)
		if st != ssNormal || value != c.value || table != c.table {
			t.Errorf("Symbol(%s) = %q, %d, %#x; want %q, %d", c.name, value, table, st, c.value, c.table)
		}
	}

	if _, _, st := cli.Symbol("DI"); st != libNoSuchSym {
		t.Errorf("Symbol(DI) = %#x, want LIB$_NOSUCHSYM: shorter than DIR*ECTORY allows", st)
	}

	// OUTER is in an outer level's table, not the current one's.
	if st := cli.DeleteSymbol("OUTER", corevms.CLILocalSymbol); st != libNoSuchSym {
		t.Errorf("DeleteSymbol(OUTER) at the inner level = %#x, want LIB$_NOSUCHSYM", st)
	}

	if st := cli.DeleteSymbol("DIREC", corevms.CLIGlobalSymbol); st != ssNormal {
		t.Errorf("DeleteSymbol(DIREC) global = %#x", st)
	}

	if _, _, st := cli.Symbol("DIRECTORY"); st != libNoSuchSym {
		t.Errorf("DIRECTORY is still there: %#x", st)
	}

	shell.dclSymbols.pop()

	if value, _, _ := cli.Symbol("OUTER"); value != "outer" {
		t.Errorf("OUTER = %q after the inner level ended", value)
	}
}

// TestShellCLI_ambiguous: a symbol whose abbreviations would overlap
// another's in the same table is LIB$_AMBSYMDEF; one of the same full
// name is replaced, and the other table isn't looked at.
func TestShellCLI_ambiguous(t *testing.T) {
	cli := shellCLI{&Shell{}}

	if st := cli.SetSymbol("AB*CDEF", "1", corevms.CLILocalSymbol); st != ssNormal {
		t.Fatalf("AB*CDEF: %#x", st)
	}

	for _, c := range []struct {
		name  string
		table uint32
		want  uint32
	}{
		{"ABC", corevms.CLILocalSymbol, libAmbSymDef},  // ABC would mean AB*CDEF too
		{"A*BX", corevms.CLILocalSymbol, libAmbSymDef}, // so would AB
		{"AX*Y", corevms.CLILocalSymbol, ssNormal},     // AX, AXY: neither is AB*CDEF's
		{"ABCDEFG", corevms.CLILocalSymbol, ssNormal},  // not an abbreviation of ABCDEF
		{"ABCDE*F", corevms.CLILocalSymbol, ssNormal},  // AB*CDEF itself, replaced
		{"ABC", corevms.CLIGlobalSymbol, ssNormal},     // another table
	} {
		if st := cli.SetSymbol(c.name, "v", c.table); st != c.want {
			t.Errorf("SetSymbol(%s, table %d) = %#x, want %#x", c.name, c.table, st, c.want)
		}
	}
}
