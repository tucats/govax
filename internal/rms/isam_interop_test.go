package rms

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/isam"
	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// TestRealIndexedFiles reads every indexed file on a real VMS system disk
// through internal/isam, by every key, when that disk is here (it's local:
// see testdata/disks/README.md). These are files nobody made for a test:
// long-lived ones with RRVs and deleted records. Nothing of their contents
// shows, and LMF$LICENSE.LDB, which the survey left alone, stays unopened.
func TestRealIndexedFiles(t *testing.T) {
	path := skipUnlessDiskPresent(t, "rq0-ra92.dsk")

	container, err := diskimage.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	vol, err := volume.Mount(container)
	if err != nil {
		_ = container.Close()

		t.Fatal(err)
	}

	defer func() { _ = vol.Dismount() }()

	files, records, dangling := 0, 0, 0

	var walk func(fid ondisk.Fid, depth int)

	walk = func(fid ondisk.Fid, depth int) {
		dir, err := vol.OpenDirectory(fid)
		if err != nil {
			return
		}

		entries, err := dir.List()
		if err != nil {
			return
		}

		for _, entry := range entries {
			// The master file directory lists itself.
			if strings.HasSuffix(entry.Name, ".DIR") && entry.Fid != fid && depth < 12 {
				walk(entry.Fid, depth+1)

				continue
			}

			if strings.HasPrefix(entry.Name, "LMF$LICENSE") {
				continue
			}

			f, err := vol.OpenFID(entry.Fid)
			if err != nil {
				continue
			}

			attr := f.Header.RecordAttributes
			if attr.Format.Organization() != ondisk.OrganizationIndexed || attr.EndOfFileBlock == 0 {
				continue
			}

			read, gone := readIndexed(t, entry.Name, f, attr)
			if read >= 0 {
				files++
				records += read
				dangling += gone
			}
		}
	}

	walk(ondisk.MasterFileDirectoryFid, 0)

	t.Logf("%d files, %d records, %d pointers to deleted records", files, records, dangling)

	if files == 0 {
		t.Error("found no indexed file")
	}
}

// readIndexed reads one file by every key, returning its record count and
// how many alternate pointers led to deleted records: -1 if the file isn't
// one isam reads.
func readIndexed(t *testing.T, name string, f *volume.File, attr ondisk.RecAttr) (int, int) {
	t.Helper()

	want := isam.Attributes{Size: int(attr.MaxRecordSize)}
	if attr.Format.Record() == ondisk.RecordFormatFixed {
		want.Format = isam.Fixed
		want.Size = int(attr.RecordSize)
	}

	file, err := isam.Open(f, want)
	if err != nil {
		var level *isam.ErrPrologLevel
		if errors.As(err, &level) || errors.Is(err, isam.ErrBadProlog) {
			t.Logf("%s: %v", name, err)

			return -1, 0
		}

		t.Errorf("%s: %v", name, err)

		return -1, 0
	}

	count, gone := 0, 0

	for _, k := range file.Keys {
		entries := 0

		err := file.Walk(k.Ref, func(e isam.Entry) bool {
			entries++

			record, err := file.Record(e.RFA)

			switch {
			case errors.Is(err, isam.ErrDeleted) && k.Ref != 0:
				gone++
			case err != nil:
				t.Errorf("%s key %d: %v: %v", name, k.Ref, e.RFA, err)
			default:
				value, ok := k.Value(record)
				if !ok || !sameBytes(value, e.Key) {
					t.Errorf("%s key %d: the record at %v doesn't hold its key", name, k.Ref, e.RFA)
				}
			}

			return true
		})
		if err != nil {
			t.Errorf("%s key %d: %v", name, k.Ref, err)
		}

		if k.Ref == 0 {
			count = entries
		}
	}

	return count, gone
}

// sameBytes compares two values.
func sameBytes(a, b []byte) bool { return string(a) == string(b) }
