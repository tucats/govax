package rtl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestServiceSysTrnlnmDebugLogicalsTrace(t *testing.T) {
	env, _ := fixture()

	tabAddr, tabStr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, tabAddr, tabStr, "LNM$FILE_DEV")
	nameAddr, nameStr := uint32(0x1200), uint32(0x1300)
	putDescriptor(t, env, nameAddr, nameStr, "SYS$OUTPUT")

	var buf bytes.Buffer
	env.cpu.SetDebugWriter(&buf)
	env.cpu.SetDebug(vax.DebugLogicals)

	if _, err := serviceSysTrnlnm(env, []uint32{0, tabAddr, nameAddr, 0, 0}); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, `DEBUG: $TRNLNM(LNM$FILE_DEV,SYS$OUTPUT), value="TTA0:"`) {
		t.Errorf("output = %q, want a $TRNLNM value trace", out)
	}
}

func TestServiceSysTrnlnmNoDebugTraceWhenFlagClear(t *testing.T) {
	env, _ := fixture()

	tabAddr, tabStr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, tabAddr, tabStr, "LNM$FILE_DEV")
	nameAddr, nameStr := uint32(0x1200), uint32(0x1300)
	putDescriptor(t, env, nameAddr, nameStr, "SYS$OUTPUT")

	var buf bytes.Buffer
	env.cpu.SetDebugWriter(&buf)
	env.cpu.SetDebug(0)

	if _, err := serviceSysTrnlnm(env, []uint32{0, tabAddr, nameAddr, 0, 0}); err != nil {
		t.Fatal(err)
	}

	if buf.Len() != 0 {
		t.Errorf("output = %q, want no trace output with DebugLogicals clear", buf.String())
	}
}

func TestServiceSysTrnlnm(t *testing.T) {
	env, _ := fixture()

	tabAddr, tabStr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, tabAddr, tabStr, "LNM$FILE_DEV")
	nameAddr, nameStr := uint32(0x1200), uint32(0x1300)
	putDescriptor(t, env, nameAddr, nameStr, "SYS$OUTPUT")

	itemList, buf := uint32(0x2000), uint32(0x3000)
	putWord(t, env, itemList, 32)
	putWord(t, env, itemList+2, lnmString)
	putLongword(t, env, itemList+4, buf)
	putLongword(t, env, itemList+8, 0)
	putLongword(t, env, itemList+12, 0)

	r0, err := serviceSysTrnlnm(env, []uint32{0, tabAddr, nameAddr, 0, itemList})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}

	got, err := loadString(env, buf, 32)
	if err != nil {
		t.Fatal(err)
	}
	if got != "TTA0:" {
		t.Errorf("value = %q, want \"TTA0:\" (InitLogicals' own default)", got)
	}
}

func TestServiceSysTrnlnmNoItemList(t *testing.T) {
	env, _ := fixture()
	tabAddr, tabStr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, tabAddr, tabStr, "LNM$FILE_DEV")
	nameAddr, nameStr := uint32(0x1200), uint32(0x1300)
	putDescriptor(t, env, nameAddr, nameStr, "SYS$INPUT")

	r0, err := serviceSysTrnlnm(env, []uint32{0, tabAddr, nameAddr, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNormal {
		t.Errorf("r0 = %d, want ssNormal", r0)
	}
}

func TestServiceSysTrnlnmNoSuchName(t *testing.T) {
	env, _ := fixture()
	tabAddr, tabStr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, tabAddr, tabStr, "LNM$FILE_DEV")
	nameAddr, nameStr := uint32(0x1200), uint32(0x1300)
	putDescriptor(t, env, nameAddr, nameStr, "NOSUCHNAME")

	r0, err := serviceSysTrnlnm(env, []uint32{0, tabAddr, nameAddr, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNoLogNam {
		t.Errorf("r0 = %d, want ssNoLogNam", r0)
	}
}

func TestServiceSysTrnlnmNoSuchTable(t *testing.T) {
	env, _ := fixture()
	tabAddr, tabStr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, tabAddr, tabStr, "NOSUCHTABLE")
	nameAddr, nameStr := uint32(0x1200), uint32(0x1300)
	putDescriptor(t, env, nameAddr, nameStr, "SYS$INPUT")

	r0, err := serviceSysTrnlnm(env, []uint32{0, tabAddr, nameAddr, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNoLogTab {
		t.Errorf("r0 = %d, want ssNoLogTab", r0)
	}
}

func TestServiceSysTrnlnmCaseBlind(t *testing.T) {
	env, _ := fixture()
	env.Logicals.Set("MYTABLE", "MYNAME", "hello", 0)

	attrAddr := uint32(0x0900)
	putLongword(t, env, attrAddr, lnmCaseBlind)

	tabAddr, tabStr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, tabAddr, tabStr, "MYTABLE")
	nameAddr, nameStr := uint32(0x1200), uint32(0x1300)
	putDescriptor(t, env, nameAddr, nameStr, "myname")

	r0, err := serviceSysTrnlnm(env, []uint32{attrAddr, tabAddr, nameAddr, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNormal {
		t.Errorf("r0 = %d, want ssNormal (case-blind lookup should have upcased \"myname\")", r0)
	}
}

func TestServiceSysTrnlnmAttributesAndTable(t *testing.T) {
	env, _ := fixture()

	tabAddr, tabStr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, tabAddr, tabStr, "LNM$FILE_DEV")
	nameAddr, nameStr := uint32(0x1200), uint32(0x1300)
	putDescriptor(t, env, nameAddr, nameStr, "SYS$OUTPUT")

	itemList := uint32(0x2000)
	attrBuf, tableDesc, tableStr := uint32(0x3000), uint32(0x3100), uint32(0x3200)

	putWord(t, env, itemList, 4)
	putWord(t, env, itemList+2, lnmAttributes)
	putLongword(t, env, itemList+4, attrBuf)
	putLongword(t, env, itemList+8, 0)

	putWord(t, env, itemList+12, 0)
	putWord(t, env, itemList+14, lnmTable)
	putLongword(t, env, itemList+16, tableDesc)
	putLongword(t, env, itemList+20, 0)

	putLongword(t, env, itemList+24, 0)

	// LNM__TABLE writes through a descriptor (str_put), not a plain buffer.
	putWord(t, env, tableDesc, 20) // generous descriptor length
	putLongword(t, env, tableDesc+4, tableStr)

	r0, err := serviceSysTrnlnm(env, []uint32{0, tabAddr, nameAddr, 0, itemList})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}

	tableName, err := loadString(env, tableStr, 20)
	if err != nil {
		t.Fatal(err)
	}
	if tableName != "LNM$FILE_DEV" {
		t.Errorf("table name = %q, want \"LNM$FILE_DEV\"", tableName)
	}
}
