package rms

import (
	"errors"
	"testing"

	"github.com/tucats/ods2/ondisk"
)

// recAttrOf decodes an ACPAttributes' record attribute area.
func recAttrOf(t *testing.T, a ACPAttributes) ondisk.RecAttr {
	t.Helper()

	ra, err := ondisk.DecodeRecAttr(a.RecordAttributes[:])
	if err != nil {
		t.Fatal(err)
	}

	return ra
}

func TestACPReadAttributes(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	a, err := m.ACPReadAttributes("DUA0", fid)
	if err != nil {
		t.Fatal(err)
	}

	ra := recAttrOf(t, a)
	if ra.Format != ondisk.RecordFormatStreamLF || ra.HighestBlock == 0 {
		t.Errorf("record attributes %+v", ra)
	}

	if a.Name != "DATA.TXT;1" || a.Backlink != mfd || a.Created == 0 {
		t.Errorf("name %q, back link %v, created %#x", a.Name, a.Backlink, a.Created)
	}

	if h, err := ondisk.DecodeFileHeader(a.Header[:]); err != nil || fileIDFrom(h.Fid) != fid {
		t.Errorf("the header block decodes to %v, %v", h.Fid, err)
	}

	if _, err := m.ACPReadAttributes("DUA1", fid); !errors.Is(err, ErrACPNotMounted) {
		t.Errorf("unmounted: %v", err)
	}

	if _, err := m.ACPReadAttributes("DUA0", FileID{Num: fid.Num, Seq: fid.Seq + 1}); !errors.Is(err, ErrACPNoSuchFile) {
		t.Errorf("stale FID: %v", err)
	}
}

// TestACPWriteAttributes writes the attributes by file ID: the writable
// ones change, the allocation and the protected characteristics don't.
func TestACPWriteAttributes(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")
	before, _ := m.ACPReadAttributes("DUA0", fid)

	err := m.ACPWriteAttributes("DUA0", fid, func(a *ACPAttributes) {
		ra := recAttrOf(t, *a)
		ra.Format = ondisk.RecordFormatFixed
		ra.RecordSize = 80
		ra.HighestBlock = 9999
		copy(a.RecordAttributes[:], ondisk.EncodeRecAttr(ra))

		a.Characteristics = ondisk.FchContig | ondisk.FchDirectory
		a.Protection = 0xEE00
		a.Owner = 0o200<<16 | 0o3
		a.Expires = 0x00AB000000000000
		a.Name = "IGNORED.TXT"
	})
	if err != nil {
		t.Fatal(err)
	}

	after, _ := m.ACPReadAttributes("DUA0", fid)
	ra := recAttrOf(t, after)

	if ra.Format != ondisk.RecordFormatFixed || ra.RecordSize != 80 {
		t.Errorf("record attributes %+v weren't written", ra)
	}

	if ra.HighestBlock != recAttrOf(t, before).HighestBlock {
		t.Errorf("the allocation changed to %d", ra.HighestBlock)
	}

	if after.Characteristics != ondisk.FchContig {
		t.Errorf("characteristics %#x: want CONTIG, and not DIRECTORY", after.Characteristics)
	}

	if after.Protection != 0xEE00 || after.Owner != 0o200<<16|0o3 || after.Expires != 0x00AB000000000000 {
		t.Errorf("protection %#x, owner %#x, expires %#x", after.Protection, after.Owner, after.Expires)
	}

	if after.Name != "DATA.TXT;1" || after.Created != before.Created {
		t.Errorf("name %q, created %#x: a read-only or untouched attribute changed", after.Name, after.Created)
	}

	ro := NewMountTable()
	if err := ro.Mount("DUA0", newTestVolumeFile(t, "ROVOL"), false); err != nil {
		t.Fatal(err)
	}

	if err := ro.ACPWriteAttributes("DUA0", mfd, func(*ACPAttributes) {}); !errors.Is(err, ErrACPWriteLocked) {
		t.Errorf("read-only mount: %v", err)
	}
}

// TestACPDeaccessWithAttributes sets the end of file to a partial block
// on deaccess, after writing past the old one: the attributes win.
func TestACPDeaccessWithAttributes(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")

	a, err := m.ACPAccess("DUA0", fid, true)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := a.Extend(1); err != nil {
		t.Fatal(err)
	}

	if err := a.WriteVirtual(2, []byte("second block")); err != nil {
		t.Fatal(err)
	}

	err = a.DeaccessWithAttributes(func(attrs *ACPAttributes) {
		ra := recAttrOf(t, *attrs)
		ra.EndOfFileBlock, ra.FirstFreeByte = 2, 12
		copy(attrs.RecordAttributes[:], ondisk.EncodeRecAttr(ra))
	})
	if err != nil {
		t.Fatal(err)
	}

	after, _ := m.ACPReadAttributes("DUA0", fid)
	if ra := recAttrOf(t, after); ra.EndOfFileBlock != 2 || ra.FirstFreeByte != 12 {
		t.Errorf("end of file %d/%d, want 2/12", ra.EndOfFileBlock, ra.FirstFreeByte)
	}

	// Read access can't write attributes; the file is closed anyway.
	r, _ := m.ACPAccess("DUA0", fid, false)
	if err := r.DeaccessWithAttributes(func(*ACPAttributes) {}); !errors.Is(err, ErrACPReadOnly) {
		t.Errorf("read access: %v", err)
	}

	if err := r.WriteAttributes(func(*ACPAttributes) {}); !errors.Is(err, ErrACPReadOnly) {
		t.Errorf("WriteAttributes on read access: %v", err)
	}
}

// TestACPDeaccessKeepsPartialEndOfFile: accessing a file for writing and
// only overwriting its first block leaves its end of file where it was,
// mid-block, rather than rounding it up to a whole block.
func TestACPDeaccessKeepsPartialEndOfFile(t *testing.T) {
	m := newACPFixture(t)
	fid, _, _ := m.ACPLookup("DUA0", mfd, "DATA.TXT")
	before := recAttrOf(t, mustAttributes(t, m, fid))

	a, err := m.ACPAccess("DUA0", fid, true)
	if err != nil {
		t.Fatal(err)
	}

	if err := a.WriteVirtual(1, []byte("overwritten")); err != nil {
		t.Fatal(err)
	}

	if err := a.Deaccess(); err != nil {
		t.Fatal(err)
	}

	after := recAttrOf(t, mustAttributes(t, m, fid))
	if after.EndOfFileBlock != before.EndOfFileBlock || after.FirstFreeByte != before.FirstFreeByte {
		t.Errorf("end of file %d/%d, was %d/%d", after.EndOfFileBlock, after.FirstFreeByte, before.EndOfFileBlock, before.FirstFreeByte)
	}
}

func mustAttributes(t *testing.T, m *MountTable, fid FileID) ACPAttributes {
	t.Helper()

	a, err := m.ACPReadAttributes("DUA0", fid)
	if err != nil {
		t.Fatal(err)
	}

	return a
}
