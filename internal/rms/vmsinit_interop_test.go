package rms

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/volume"
)

// TestVMSInitializeFidelity checks ods2's volume.Initialize against volumes
// VMS 7.3 INITIALIZE made on the user's simh VAX (docs/PHASE-27.md): for
// each reference disk in testdata/disks/, it initializes a scratch volume
// of the same size with the same label and compares the two block by block.
// Everything must match except what VMS itself varies from volume to
// volume, or what mounting a volume on VMS changes:
//
//   - LBN 0, the boot block, which INIT fills with a PDP-11 program and
//     ods2 leaves zeroed;
//   - timestamps: home block creation dates, file header creation and
//     revision dates, and the storage control block's mount time and
//     volume lock name (the reference volumes were mounted once);
//   - checksums over those.
//
// The reference disks aren't committed (see testdata/disks/README.md), so
// this skips any that aren't present.
func TestVMSInitializeFidelity(t *testing.T) {
	refs := []struct {
		file   string
		blocks uint32
		label  string
	}{
		{"vms-init-rd51.dsk", 21600, "MARXCHG"},
		{"rq1-rx33.dsk", 2400, "RX33DSK"},
		{"rq3-rd54.dsk", 311200, "RD54DSK"},
	}

	for _, ref := range refs {
		t.Run(ref.file, func(t *testing.T) {
			refPath := filepath.Join(disksDir(t), ref.file)
			if _, err := os.Stat(refPath); err != nil {
				t.Skipf("%s not present (see testdata/disks/README.md)", refPath)
			}

			path := filepath.Join(t.TempDir(), "init.dsk")

			c, err := diskimage.Create(path, ref.blocks)
			if err != nil {
				t.Fatal(err)
			}

			if err := volume.Initialize(c, volume.InitializeOptions{Label: ref.label}); err != nil {
				t.Fatalf("Initialize: %v", err)
			}

			if err := c.Close(); err != nil {
				t.Fatal(err)
			}

			compareVolumes(t, refPath, path, ref.blocks)
		})
	}
}

// compareVolumes compares blocks 1 to blocks-1 of the reference and
// candidate volumes after masking the fields VMS varies (see
// TestVMSInitializeFidelity), reporting the first few differences.
func compareVolumes(t *testing.T, refPath, gotPath string, blocks uint32) {
	t.Helper()

	ref, err := os.Open(refPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ref.Close()

	got, err := os.Open(gotPath)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()

	// The storage control block is the first block of BITMAP.SYS, whose
	// map is in file 2's header, the second block after the index file
	// bitmap. Read its LBN from the reference.
	scbLBN := referenceSCB(t, ref)

	refBlock := make([]byte, 512)
	gotBlock := make([]byte, 512)

	var diffs []string

	for lbn := uint32(1); lbn < blocks; lbn++ {
		if _, err := ref.ReadAt(refBlock, int64(lbn)*512); err != nil && err != io.EOF {
			t.Fatal(err)
		}

		if _, err := got.ReadAt(gotBlock, int64(lbn)*512); err != nil && err != io.EOF {
			t.Fatal(err)
		}

		if bytes.Equal(refBlock, gotBlock) {
			continue
		}

		kind := maskVaryingFields(refBlock, lbn == scbLBN)
		maskVaryingFields(gotBlock, lbn == scbLBN)

		if bytes.Equal(refBlock, gotBlock) {
			continue
		}

		var offsets []string

		for i := range refBlock {
			if refBlock[i] != gotBlock[i] {
				offsets = append(offsets, fmt.Sprintf("%d: %02x/%02x", i, refBlock[i], gotBlock[i]))
			}
		}

		if len(offsets) > 12 {
			offsets = append(offsets[:12], "...")
		}

		diffs = append(diffs, fmt.Sprintf("LBN %d (%s) differs at [VMS/ods2] %s", lbn, kind, strings.Join(offsets, ", ")))
	}

	for i, d := range diffs {
		if i == 20 {
			t.Errorf("... and %d more differing blocks", len(diffs)-20)

			break
		}

		t.Error(d)
	}
}

// maskVaryingFields zeroes the fields of a block that VMS varies between
// volumes, by the kind of block it is, and returns that kind.
func maskVaryingFields(b []byte, isSCB bool) string {
	zero := func(from, to int) {
		for i := from; i < to; i++ {
			b[i] = 0
		}
	}

	switch {
	case string(b[496:506]) == "DECFILE11B":
		zero(58, 68)   // checksum 1, creation date
		zero(510, 512) // checksum 2

		return "home block"

	case isSCB:
		zero(32, 56) // write count, volume lock name, mount time
		zero(510, 512)

		return "storage control block"

	case b[0] == 40 && b[1] == 100 && binary.LittleEndian.Uint16(b[6:]) == 0x0201:
		zero(80+22, 80+38) // IDENT creation and revision dates
		zero(510, 512)

		return "file header"
	}

	return "data"
}

// referenceSCB finds the reference volume's storage control block: the
// first LBN of BITMAP.SYS, whose header is file 2.
func referenceSCB(t *testing.T, f *os.File) uint32 {
	t.Helper()

	home := make([]byte, 512)
	if _, err := f.ReadAt(home, 512); err != nil {
		t.Fatal(err)
	}

	ibmapLBN := binary.LittleEndian.Uint32(home[24:])
	ibmapSize := uint32(binary.LittleEndian.Uint16(home[32:]))

	hdr := make([]byte, 512)
	if _, err := f.ReadAt(hdr, int64(ibmapLBN+ibmapSize+1)*512); err != nil {
		t.Fatal(err)
	}

	// Format 1 pointer: count, high LBN bits, low LBN word; or format 2:
	// count word, then a longword LBN.
	m := int(hdr[1]) * 2
	w0 := binary.LittleEndian.Uint16(hdr[m:])

	if w0>>14 == 1 {
		return uint32((w0>>8)&0x3F)<<16 | uint32(binary.LittleEndian.Uint16(hdr[m+2:]))
	}

	return binary.LittleEndian.Uint32(hdr[m+2:])
}
