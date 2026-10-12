package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// selfRelForward resolves node's forward link (stored at node) to the
// absolute address it points to.
func selfRelForward(t *testing.T, cpu *registers.CPU, mem *vm.Memory, node uint32) uint32 {
	t.Helper()

	off, err := mem.LoadLongword(cpu, node)
	if err != nil {
		t.Fatalf("LoadLongword(%#x): %v", node, err)
	}

	return node + off
}

// selfRelBackward resolves node's backward link (stored at node+4, but
// -- like every self-relative link -- relative to node itself, not node+4)
// to the absolute address it points to.
func selfRelBackward(t *testing.T, cpu *registers.CPU, mem *vm.Memory, node uint32) uint32 {
	t.Helper()

	off, err := mem.LoadLongword(cpu, node+4)
	if err != nil {
		t.Fatalf("LoadLongword(%#x): %v", node+4, err)
	}

	return node + off
}

func insqhi(t *testing.T, e *Engine, entry, header uint32) {
	t.Helper()

	bytes := []byte{0x5C}
	bytes = append(bytes, absoluteMode(entry)...)
	bytes = append(bytes, absoluteMode(header)...)
	stepInstruction(t, e, bytes...)
}

func insqti(t *testing.T, e *Engine, entry, header uint32) {
	t.Helper()

	bytes := []byte{0x5D}
	bytes = append(bytes, absoluteMode(entry)...)
	bytes = append(bytes, absoluteMode(header)...)
	stepInstruction(t, e, bytes...)
}

func TestEmulInsqhi(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	header := uint32(0x5000)
	putBytes(t, cpu, mem, header, 0, 0, 0, 0, 0, 0, 0, 0)

	insqhi(t, e, 0x6000, header)

	if !cpu.PSL().Z() {
		t.Error("Z = false after inserting the first entry, want true")
	}

	insqhi(t, e, 0x7000, header)

	if cpu.PSL().Z() {
		t.Error("Z = true after inserting a second entry, want false " +
			"(regression: emul_insqhi.c's hf==hb emptiness test is also " +
			"true for a genuine one-entry queue)")
	}

	insqhi(t, e, 0x8000, header)

	if cpu.PSL().Z() {
		t.Error("Z = true after inserting a third entry, want false")
	}

	// INSQHI inserts at the head, so the most recently inserted entry comes
	// first: header -> 0x8000 -> 0x7000 -> 0x6000 -> header.
	forward := []uint32{0x8000, 0x7000, 0x6000, header}
	cur := header

	for i, want := range forward {
		next := selfRelForward(t, cpu, mem, cur)
		if next != want {
			t.Fatalf("forward step %d: %#x -> %#x, want %#x", i, cur, next, want)
		}

		cur = next
	}

	backward := []uint32{0x6000, 0x7000, 0x8000, header}
	cur = header

	for i, want := range backward {
		prev := selfRelBackward(t, cpu, mem, cur)
		if prev != want {
			t.Fatalf("backward step %d: %#x <- %#x, want %#x", i, cur, prev, want)
		}

		cur = prev
	}
}

func TestEmulInsqti(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	header := uint32(0x5000)
	putBytes(t, cpu, mem, header, 0, 0, 0, 0, 0, 0, 0, 0)

	insqti(t, e, 0x6000, header)

	if !cpu.PSL().Z() {
		t.Error("Z = false after inserting the first entry, want true")
	}

	insqti(t, e, 0x7000, header)

	if cpu.PSL().Z() {
		t.Error("Z = true after inserting a second entry, want false")
	}

	insqti(t, e, 0x8000, header)

	// INSQTI inserts at the tail, so entries come out in insertion order:
	// header -> 0x6000 -> 0x7000 -> 0x8000 -> header.
	forward := []uint32{0x6000, 0x7000, 0x8000, header}
	cur := header

	for i, want := range forward {
		next := selfRelForward(t, cpu, mem, cur)
		if next != want {
			t.Fatalf("forward step %d: %#x -> %#x, want %#x", i, cur, next, want)
		}

		cur = next
	}

	// Backward links are maintained independently of the forward chain
	// checked above (see docs/DEVIATIONS.md's INSQTI finding, which the
	// forward-walk check already regresses); confirm they're consistent
	// too.
	backward := []uint32{0x8000, 0x7000, 0x6000, header}
	cur = header

	for i, want := range backward {
		prev := selfRelBackward(t, cpu, mem, cur)
		if prev != want {
			t.Fatalf("backward step %d: %#x <- %#x, want %#x", i, cur, prev, want)
		}

		cur = prev
	}
}

func TestEmulRemqhi(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	header := uint32(0x5000)

	putBytes(t, cpu, mem, header, 0, 0, 0, 0, 0, 0, 0, 0)
	insqhi(t, e, 0x6000, header) // head -> 0x6000
	insqhi(t, e, 0x7000, header) // head -> 0x7000 -> 0x6000

	remove := func() uint32 {
		t.Helper()

		bytes := []byte{0x5E}
		bytes = append(bytes, absoluteMode(header)...)
		bytes = append(bytes, regMode(registers.R0))

		stepInstruction(t, e, bytes...)

		return cpu.GPR(registers.R0)
	}

	if got := remove(); got != 0x7000 {
		t.Errorf("first removal = %#x, want 0x7000 (most recently inserted)", got)
	}

	if cpu.PSL().Z() || cpu.PSL().V() {
		t.Errorf("after first removal: Z=%v V=%v, want both false (queue non-empty, removal succeeded)",
			cpu.PSL().Z(), cpu.PSL().V())
	}

	if got := remove(); got != 0x6000 {
		t.Errorf("second removal = %#x, want 0x6000", got)
	}

	if !cpu.PSL().Z() || cpu.PSL().V() {
		t.Errorf("after second removal: Z=%v V=%v, want Z=true V=false (queue now empty)",
			cpu.PSL().Z(), cpu.PSL().V())
	}

	remove() // queue already empty

	if !cpu.PSL().Z() || !cpu.PSL().V() {
		t.Errorf("removing from an empty queue: Z=%v V=%v, want both true",
			cpu.PSL().Z(), cpu.PSL().V())
	}
}

func TestEmulRemqti(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	header := uint32(0x5000)
	putBytes(t, cpu, mem, header, 0, 0, 0, 0, 0, 0, 0, 0)
	insqti(t, e, 0x6000, header) // 0x6000 -> tail
	insqti(t, e, 0x7000, header) // 0x6000 -> 0x7000 -> tail

	bytes := []byte{0x5F}
	bytes = append(bytes, absoluteMode(header)...)
	bytes = append(bytes, regMode(registers.R0))
	stepInstruction(t, e, bytes...)

	if got := cpu.GPR(registers.R0); got != 0x7000 {
		t.Errorf("removal = %#x, want 0x7000 (the tail entry)", got)
	}
	
	if cpu.PSL().Z() || cpu.PSL().V() {
		t.Errorf("Z=%v V=%v, want both false", cpu.PSL().Z(), cpu.PSL().V())
	}
}

func TestEmulInsqueRemque(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	header := uint32(0x5000)
	// A self-referential (empty) absolute-queue header.
	putBytes(t, cpu, mem, header, byte(header), byte(header>>8), byte(header>>16), byte(header>>24))
	putBytes(t, cpu, mem, header+4, byte(header), byte(header>>8), byte(header>>16), byte(header>>24))

	insque := func(entry, pred uint32) {
		t.Helper()
		
		bytes := []byte{0x0E}
		bytes = append(bytes, absoluteMode(entry)...)
		bytes = append(bytes, absoluteMode(pred)...)
		stepInstruction(t, e, bytes...)
	}

	insque(0x6000, header)

	if !cpu.PSL().Z() {
		t.Error("Z = false after inserting the first entry, want true")
	}

	insque(0x7000, 0x6000) // insert after 0x6000

	if cpu.PSL().Z() {
		t.Error("Z = true after inserting a second entry, want false")
	}

	// header -> 0x6000 -> 0x7000 -> header
	if got, err := mem.LoadLongword(cpu, header); err != nil || got != 0x6000 {
		t.Errorf("header forward link = %#x, want 0x6000", got)
	}

	if got, err := mem.LoadLongword(cpu, 0x6000); err != nil || got != 0x7000 {
		t.Errorf("0x6000's forward link = %#x, want 0x7000", got)
	}

	if got, err := mem.LoadLongword(cpu, 0x7000); err != nil || got != header {
		t.Errorf("0x7000's forward link = %#x, want header (%#x)", got, header)
	}

	bytes := []byte{0x0F}
	bytes = append(bytes, absoluteMode(0x6000)...)
	bytes = append(bytes, regMode(registers.R1))
	stepInstruction(t, e, bytes...)

	if cpu.GPR(registers.R1) != 0x6000 {
		t.Errorf("R1 = %#x, want 0x6000 (the removed entry's own address)", cpu.GPR(registers.R1))
	}

	if got, err := mem.LoadLongword(cpu, header); err != nil || got != 0x7000 {
		t.Errorf("header forward link after removal = %#x, want 0x7000", got)
	}
}

// TestInterlockedQueueSecondaryInterlock: with bit 0 of the header set,
// each instruction changes nothing and sets C (and V for a removal).
func TestInterlockedQueueSecondaryInterlock(t *testing.T) {
	cpu, mem := fixture()
	header := uint32(0x5000)
	putLongword(t, cpu, mem, header, 1)
	putLongword(t, cpu, mem, header+4, 0)

	for _, tail := range []bool{false, true} {
		r, err := InsertInterlocked(cpu, mem, 0x6000, header, tail)
		if err != nil || r != (QueueResult{C: true}) {
			t.Errorf("insert (tail %v) = %+v, %v; want C alone", tail, r, err)
		}

		r, err = RemoveInterlocked(cpu, mem, header, 0, tail)
		if err != nil || r != (QueueResult{V: true, C: true}) {
			t.Errorf("remove (tail %v) = %+v, %v; want V and C", tail, r, err)
		}
	}

	if got, _ := mem.LoadLongword(cpu, header); got != 1 {
		t.Errorf("header = %#x, want 1 (unchanged)", got)
	}
}

// TestInterlockedQueueReservedOperand: misaligned headers and entries, an
// entry that is its header, a header link with bits 2:1 set, and a
// removal whose addr is its header are reserved operand faults.
func TestInterlockedQueueReservedOperand(t *testing.T) {
	cpu, mem := fixture()
	header := uint32(0x5000)
	putLongword(t, cpu, mem, header, 0)
	putLongword(t, cpu, mem, header+4, 0)

	for _, c := range []struct {
		name string
		err  error
	}{
		{"entry misaligned", func() error { _, err := InsertInterlocked(cpu, mem, 0x6004, header, false); return err }()},
		{"header misaligned", func() error { _, err := InsertInterlocked(cpu, mem, 0x6000, header+2, true); return err }()},
		{"entry is header", func() error { _, err := InsertInterlocked(cpu, mem, header, header, false); return err }()},
		{"remove misaligned", func() error { _, err := RemoveInterlocked(cpu, mem, header+4, 0, false); return err }()},
		{"addr is header", func() error { _, err := RemoveInterlocked(cpu, mem, header, header, true); return err }()},
	} {
		if !isReservedOp(c.err) {
			t.Errorf("%s: %v, want a reserved operand fault", c.name, c.err)
		}
	}

	putLongword(t, cpu, mem, header, 4) // bits 2:1 set

	if _, err := RemoveInterlocked(cpu, mem, header, 0, false); !isReservedOp(err) {
		t.Errorf("header link 4: %v, want a reserved operand fault", err)
	}
}

// TestRemqtiEmpty: REMQTI of an empty queue stores the header's address,
// as REMQHI does.
func TestRemqtiEmpty(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	header := uint32(0x5000)
	putBytes(t, cpu, mem, header, 0, 0, 0, 0, 0, 0, 0, 0)
	cpu.SetGPR(registers.R0, 0xDEAD)

	bytes := []byte{0x5F}
	bytes = append(bytes, absoluteMode(header)...)
	bytes = append(bytes, regMode(registers.R0))
	stepInstruction(t, e, bytes...)

	if got := cpu.GPR(registers.R0); got != header {
		t.Errorf("R0 = %#x, want the header, %#x", got, header)
	}

	if psl := cpu.PSL(); !psl.Z() || !psl.V() || psl.C() {
		t.Errorf("Z=%v V=%v C=%v, want Z and V", psl.Z(), psl.V(), psl.C())
	}
}

// TestInterlockedQueueMixed: entries inserted at both ends come out in
// order from both ends, the links staying consistent.
func TestInterlockedQueueMixed(t *testing.T) {
	cpu, mem := fixture()
	header := uint32(0x5000)
	putLongword(t, cpu, mem, header, 0)
	putLongword(t, cpu, mem, header+4, 0)

	// 0x7000 0x6000 0x8000
	for _, step := range []struct {
		entry uint32
		tail  bool
		z     bool
	}{{0x6000, false, true}, {0x7000, false, false}, {0x8000, true, false}} {
		r, err := InsertInterlocked(cpu, mem, step.entry, header, step.tail)
		if err != nil || r.Z != step.z {
			t.Fatalf("insert %#x: %+v, %v", step.entry, r, err)
		}
	}

	for _, step := range []struct {
		tail bool
		want uint32
		z    bool
	}{{true, 0x8000, false}, {false, 0x7000, false}, {true, 0x6000, true}, {false, header, true}} {
		r, err := RemoveInterlocked(cpu, mem, header, 0, step.tail)
		if err != nil || r.Removed != step.want || r.Z != step.z {
			t.Errorf("remove (tail %v) = %+v, %v; want %#x, Z %v", step.tail, r, err, step.want, step.z)
		}
	}

	if f, b := selfRelForward(t, cpu, mem, header), selfRelBackward(t, cpu, mem, header); f != header || b != header {
		t.Errorf("empty header links to %#x, %#x; want itself", f, b)
	}
}
