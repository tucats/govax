package paging

import "testing"

// checkMap fails the test if m breaks one of its rules.
func checkMap(t *testing.T, m *PageFileMap) {
	t.Helper()

	if err := m.Check(); err != nil {
		t.Fatal(err)
	}
}

// allocateBlocks allocates want blocks from m, failing the test if it can't.
func allocateBlocks(t *testing.T, m *PageFileMap, want uint32) Run {
	t.Helper()

	run, _, ok := m.Allocate(want)
	if !ok {
		t.Fatalf("Allocate(%d) failed with %d blocks free", want, m.FreeBlocks())
	}

	checkMap(t, m)

	return run
}

func TestPageFileMapAllocatesFromTheStart(t *testing.T) {
	m := NewPageFileMap(1000, 120)
	checkMap(t, m)

	if got := allocateBlocks(t, m, 120); got != (Run{First: 1, Count: 120}) {
		t.Errorf("first run %+v, want blocks 1 to 120", got)
	}

	if got := allocateBlocks(t, m, 5); got != (Run{First: 121, Count: 5}) {
		t.Errorf("second run %+v, want blocks 121 to 125", got)
	}

	if m.UsedBlocks() != 125 || m.FreeBlocks() != 875 {
		t.Errorf("used %d, free %d; want 125 and 875", m.UsedBlocks(), m.FreeBlocks())
	}

	if !m.Allocated(1) || !m.Allocated(125) || m.Allocated(126) || m.Allocated(0) || m.Allocated(1001) {
		t.Error("Allocated disagrees with the runs taken")
	}
}

func TestPageFileMapAskingForMoreThanTheClusterGetsTheCluster(t *testing.T) {
	m := NewPageFileMap(1000, 64)

	if got := allocateBlocks(t, m, 500); got.Count != 64 {
		t.Errorf("run %+v, want the cluster size, 64 blocks", got)
	}
}

func TestPageFileMapFirstFitAfterAFree(t *testing.T) {
	m := NewPageFileMap(400, 120)
	a := allocateBlocks(t, m, 120)
	allocateBlocks(t, m, 120)

	if err := m.Free(a); err != nil {
		t.Fatal(err)
	}

	checkMap(t, m)

	// The scan starts at the file's start, so the hole is used again.
	if got := allocateBlocks(t, m, 120); got.First != 1 {
		t.Errorf("run %+v, want the freed run at block 1", got)
	}
}

// fragment fills m, then frees every other run of hole blocks, so the
// longest free run is hole blocks.
func fragment(t *testing.T, m *PageFileMap, hole uint32) {
	t.Helper()

	for m.FreeBlocks() > 0 {
		if _, _, ok := m.Allocate(1); !ok {
			t.Fatal("filling the map failed")
		}
	}

	for first := uint32(1); first+hole-1 <= m.Size(); first += 2 * hole {
		if err := m.Free(Run{First: first, Count: hole}); err != nil {
			t.Fatal(err)
		}
	}

	checkMap(t, m)
}

func TestPageFileMapAllocationSizeDropsBy16(t *testing.T) {
	m := NewPageFileMap(2000, 120)
	fragment(t, m, 50)

	// The largest free run is 50 blocks: 120, 104, 88, 72, and 56 find
	// nothing, and 40 does.
	got := allocateBlocks(t, m, 120)
	if got.Count != 40 || m.AllocationSize() != 40 {
		t.Errorf("run %+v at allocation size %d; want 40 blocks at size 40", got, m.AllocationSize())
	}
}

func TestPageFileMapSpecialCaseTakesShortRuns(t *testing.T) {
	m := NewPageFileMap(500, 120)
	fragment(t, m, 3)

	// Nothing of 16 is free: the special case takes the first free run,
	// 3 blocks at block 1.
	got := allocateBlocks(t, m, 120)
	if got != (Run{First: 1, Count: 3}) || m.AllocationSize() != 16 {
		t.Errorf("run %+v at allocation size %d; want blocks 1 to 3 at size 16", got, m.AllocationSize())
	}

	// A small request is first fit at any size.
	if got := allocateBlocks(t, m, 2); got != (Run{First: 7, Count: 2}) {
		t.Errorf("run %+v, want blocks 7 and 8", got)
	}
}

func TestPageFileMapAllocationSizeRegrows(t *testing.T) {
	m := NewPageFileMap(2000, 120)
	fragment(t, m, 50)
	allocateBlocks(t, m, 120) // the size drops to 40

	// Freeing next to a free run (101 to 150) so that 60 are free
	// together lets the size grow by 8.
	if err := m.Free(Run{First: 151, Count: 10}); err != nil {
		t.Fatal(err)
	}

	checkMap(t, m)

	if m.AllocationSize() != 48 {
		t.Errorf("allocation size %d after the free, want 48", m.AllocationSize())
	}

	// A free that leaves a short run doesn't.
	if err := m.Free(Run{First: 175, Count: 1}); err != nil {
		t.Fatal(err)
	}

	if m.AllocationSize() != 48 {
		t.Errorf("allocation size %d after a short free, want 48", m.AllocationSize())
	}
}

func TestPageFileMapFull(t *testing.T) {
	m := NewPageFileMap(20, 16)
	allocateBlocks(t, m, 16)
	allocateBlocks(t, m, 16) // the last 4

	if _, _, ok := m.Allocate(1); ok {
		t.Error("Allocate succeeded on a full file")
	}

	if m.FreeBlocks() != 0 {
		t.Errorf("%d blocks free, want 0", m.FreeBlocks())
	}
}

func TestPageFileMapWarnsOncePerThreshold(t *testing.T) {
	m := NewPageFileMap(100, 16)

	var seen []Warning

	for m.FreeBlocks() > 0 {
		_, w, ok := m.Allocate(1)
		if !ok {
			t.Fatal("Allocate failed")
		}

		if w != NoWarning {
			seen = append(seen, w)

			// The warning comes with the block that passed the
			// threshold: the 66th and the 91st.
			want := map[Warning]uint32{PageFrag: 66, PageCrit: 91}[w]
			if m.UsedBlocks() != want {
				t.Errorf("%s at %d blocks used, want %d", w, m.UsedBlocks(), want)
			}
		}
	}

	if len(seen) != 2 || seen[0] != PageFrag || seen[1] != PageCrit {
		t.Errorf("warnings %v, want PAGEFRAG then PAGECRIT", seen)
	}

	// Freeing and filling again warns no more.
	if err := m.Free(Run{First: 1, Count: 50}); err != nil {
		t.Fatal(err)
	}

	for m.FreeBlocks() > 0 {
		if _, w, _ := m.Allocate(16); w != NoWarning {
			t.Errorf("%s again", w)
		}
	}
}

func TestPageFileMapFreeErrors(t *testing.T) {
	m := NewPageFileMap(100, 16)
	allocateBlocks(t, m, 10)

	for _, run := range []Run{{First: 0, Count: 1}, {First: 95, Count: 10}, {First: 5, Count: 10}} {
		if err := m.Free(run); err == nil {
			t.Errorf("Free(%+v) succeeded", run)
		}
	}

	// A refused free frees nothing.
	if m.UsedBlocks() != 10 {
		t.Errorf("%d blocks used after refused frees, want 10", m.UsedBlocks())
	}

	checkMap(t, m)
}

func TestPageFileMapSkipsFullWords(t *testing.T) {
	// A map larger than a word, full but for one block in its third word.
	m := NewPageFileMap(200, 16)
	for m.FreeBlocks() > 0 {
		allocateBlocks(t, m, 16)
	}

	if err := m.Free(Run{First: 150, Count: 1}); err != nil {
		t.Fatal(err)
	}

	if got := allocateBlocks(t, m, 1); got != (Run{First: 150, Count: 1}) {
		t.Errorf("run %+v, want block 150", got)
	}
}
