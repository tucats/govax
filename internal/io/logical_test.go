package io

import "testing"

func TestLogicalNameTableSetAndGet(t *testing.T) {
	lt := NewLogicalNameTable()
	lt.Set("LNM_PROCESS", "SYS$SCRATCH", "DKA0:[SCRATCH]", 0)

	ln, ok := lt.Get("LNM_PROCESS", "SYS$SCRATCH", 0)
	if !ok {
		t.Fatalf("Get: not found")
	}

	if ln.Value != "DKA0:[SCRATCH]" {
		t.Errorf("Get.Value = %q, want DKA0:[SCRATCH]", ln.Value)
	}

	if _, ok := lt.Get("LNM_PROCESS", "NOSUCH", 0); ok {
		t.Errorf("Get(NOSUCH) unexpectedly found")
	}
	
	if _, ok := lt.Get("NOSUCHTABLE", "SYS$SCRATCH", 0); ok {
		t.Errorf("Get with unknown table unexpectedly found")
	}
}

func TestLogicalNameTableGetDefaultsToLNMRoot(t *testing.T) {
	lt := NewLogicalNameTable()
	lt.Set("LNM$ROOT", "FOO", "BAR", 0)

	ln, ok := lt.Get("", "FOO", 0)
	if !ok || ln.Value != "BAR" {
		t.Errorf("Get(\"\", FOO) = %v, %v, want BAR, true", ln, ok)
	}
}

func TestLogicalNameTableSetRedefineKeepsOriginalAttr(t *testing.T) {
	lt := NewLogicalNameTable()
	lt.Set("T", "NAME", "first", 0x1234)
	lt.Set("T", "NAME", "second", 0x9999)

	ln, ok := lt.Get("T", "NAME", 0)
	if !ok {
		t.Fatalf("Get: not found")
	}

	if ln.Value != "second" {
		t.Errorf("Value = %q, want second", ln.Value)
	}

	if ln.Attr != 0x1234 {
		t.Errorf("Attr = %#x, want %#x (redefinition must not change attr)", ln.Attr, 0x1234)
	}
}

func TestLogicalNameTableGetAttrFilter(t *testing.T) {
	lt := NewLogicalNameTable()
	lt.Set("T", "NAME", "value", LNMTable)

	if _, ok := lt.Get("T", "NAME", LNMTerminal); ok {
		t.Errorf("Get with mismatched attr filter unexpectedly found")
	}

	if _, ok := lt.Get("T", "NAME", LNMTable); !ok {
		t.Errorf("Get with matching attr filter not found")
	}
}

func TestLogicalNameTableDelete(t *testing.T) {
	lt := NewLogicalNameTable()
	lt.Set("T", "NAME", "value", 0)

	if !lt.Delete("T", "NAME") {
		t.Errorf("Delete: reported not-found for an existing entry")
	}

	if _, ok := lt.Get("T", "NAME", 0); ok {
		t.Errorf("Get after Delete unexpectedly found")
	}

	if lt.Delete("T", "NAME") {
		t.Errorf("Delete of an already-deleted entry should report false")
	}

	if lt.Delete("NOSUCHTABLE", "NAME") {
		t.Errorf("Delete against an unknown table should report false")
	}
}

func TestLogicalNameTableInitLogicals(t *testing.T) {
	lt := NewLogicalNameTable()
	lt.InitLogicals()

	for _, name := range []string{"SYS$COMMAND", "SYS$INPUT", "SYS$OUTPUT", "SYS$ERROR"} {
		ln, ok := lt.Get("LNM$FILE_DEV", name, 0)
		if !ok {
			t.Errorf("Get(LNM$FILE_DEV, %s) not found", name)

			continue
		}

		if ln.Value != "TTA0:" {
			t.Errorf("Get(LNM$FILE_DEV, %s).Value = %q, want TTA0:", name, ln.Value)
		}
	}

	ln, ok := lt.Get("LNM$TABLE", "LNM$FILE_DEV", 0)
	if !ok || ln.Value != "<TABLE>" {
		t.Errorf("Get(LNM$TABLE, LNM$FILE_DEV) = %v, %v, want <TABLE>, true", ln, ok)
	}
}

func TestLogicalNameTableAllMatching(t *testing.T) {
	lt := NewLogicalNameTable()
	lt.Set("T1", "A", "va", 0)
	lt.Set("T1", "B", "vb", 0)
	lt.Set("T2", "A", "va2", 0)

	all := lt.AllMatching("", "")
	if len(all) != 3 {
		t.Fatalf("AllMatching(\"\",\"\") len = %d, want 3", len(all))
	}

	onlyT1 := lt.AllMatching("T1", "")
	if len(onlyT1) != 2 {
		t.Errorf("AllMatching(T1, \"\") len = %d, want 2", len(onlyT1))
	}

	onlyA := lt.AllMatching("", "A")
	if len(onlyA) != 2 {
		t.Errorf("AllMatching(\"\", A) len = %d, want 2", len(onlyA))
	}

	exact := lt.AllMatching("T2", "A")
	if len(exact) != 1 || exact[0].Name.Value != "va2" {
		t.Errorf("AllMatching(T2, A) = %+v, want one entry with value va2", exact)
	}
}
