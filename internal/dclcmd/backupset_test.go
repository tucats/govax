package dclcmd

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/rms"
)

// probeSets is where the probe's save sets are (testdata/backup69).
const probeSets = "../../testdata/backup69/vax/"

// newSaveSetContext is a Context on a volume, DUA0: and the default,
// holding B69.BCK and B69CRCD.BCK as COPY/BINARY puts them there, and
// PI.MAR, a text file.
func newSaveSetContext(t *testing.T) *Context {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sets.dsk")
	if err := rms.InitializeContainer(path, 2000, "SETS", 0, ""); err != nil {
		t.Fatal(err)
	}

	mounts := rms.NewMountTable()
	if err := mounts.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = mounts.Dismount("DUA0") })

	s := rms.NewSession(mounts)
	if err := s.SetDefault("DUA0:"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"b69.bck", "b69crcd.bck", "pi.mar"} {
		if _, err := s.Copy(probeSets+name, true, strings.ToUpper(name), false, rms.CopyOptions{Binary: true}); err != nil {
			t.Fatalf("Copy %s: %v", name, err)
		}
	}

	return &Context{ContainerSession: s}
}

// TestReadSaveSet: a save set on a volume or the host is read whole,
// named as VMS's messages name it, and /CRC reaches the reader.
func TestReadSaveSet(t *testing.T) {
	c := newSaveSetContext(t)

	in, err := c.ReadSaveSet(BackupSpecifier{Text: "B69.BCK", SaveSet: true}, backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if in.Name != "DUA0:[000000]B69.BCK;1" || in.Expanded != "DUA0:[000000]B69.BCK;" {
		t.Errorf("Name %q, Expanded %q", in.Name, in.Expanded)
	}

	if len(in.Set.Files) == 0 || in.RecordsMatch {
		t.Errorf("%d files, RecordsMatch %v (COPY/BINARY's undefined records)", len(in.Set.Files), in.RecordsMatch)
	}

	// The same save set on the host, by a name with a "/".
	host, err := c.ReadSaveSet(BackupSpecifier{Text: probeSets + "b69.bck", SaveSet: true}, backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if host.Name != probeSets+"b69.bck" || host.Expanded != host.Name || !host.RecordsMatch {
		t.Errorf("host: Name %q, Expanded %q, RecordsMatch %v", host.Name, host.Expanded, host.RecordsMatch)
	}

	if len(host.Set.Files) != len(in.Set.Files) || host.Set.Blocks != in.Set.Blocks {
		t.Errorf("host: %d files in %d blocks, the volume's %d in %d",
			len(host.Set.Files), host.Set.Blocks, len(in.Set.Files), in.Set.Blocks)
	}

	// The damaged CRC is found only with /CRC, as VMS V7.1 finds it.
	plain, err := c.ReadSaveSet(BackupSpecifier{Text: "B69CRCD.BCK", SaveSet: true}, backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	checked, err := c.ReadSaveSet(BackupSpecifier{Text: "B69CRCD.BCK", SaveSet: true}, backup.Options{CheckCRC: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(plain.Set.Recovered) != 0 || len(checked.Set.Recovered) != 1 {
		t.Errorf("recovered %v without /CRC and %v with it, want none and one", plain.Set.Recovered, checked.Set.Recovered)
	}
}

// TestReadSaveSetErrors: OPENIN for a save set that isn't there, with
// its expanded name and RMS's error, and NOTSAVESET for a file that
// isn't one, with its full name.
func TestReadSaveSetErrors(t *testing.T) {
	c := newSaveSetContext(t)

	for _, spec := range []struct{ text, name string }{
		{"NONE.BCK", "DUA0:[000000]NONE.BCK;"},
		{"DUA0:[NOSUCH]NONE.BCK", "DUA0:[NOSUCH]NONE.BCK;"},
		{"*.BCK", "DUA0:[000000]*.BCK;"},
	} {
		_, err := c.ReadSaveSet(BackupSpecifier{Text: spec.text, SaveSet: true}, backup.Options{})

		var openErr *SaveSetOpenError
		if !errors.As(err, &openErr) || openErr.Name != spec.name {
			t.Errorf("%s: %v, want OPENIN of %s", spec.text, err, spec.name)

			continue
		}

		var fileErr *rms.DCLFileError
		if !errors.As(err, &fileErr) {
			t.Errorf("%s: cause %v, want an RMS status", spec.text, openErr.Cause)
		}
	}

	missing := filepath.Join(t.TempDir(), "none.bck")

	var openErr *SaveSetOpenError
	if _, err := c.ReadSaveSet(BackupSpecifier{Text: missing, SaveSet: true}, backup.Options{}); !errors.As(err, &openErr) || openErr.Name != missing {
		t.Errorf("missing host file: %v", err)
	}

	for _, spec := range []struct {
		text string
		host bool
		name string
	}{
		{"PI.MAR", false, "DUA0:[000000]PI.MAR;1"},
		{probeSets + "pi.exe", true, probeSets + "pi.exe"},
	} {
		_, err := c.ReadSaveSet(BackupSpecifier{Text: spec.text, Host: spec.host, SaveSet: true}, backup.Options{})

		var notErr *NotSaveSetError
		if !errors.As(err, &notErr) || notErr.Name != spec.name || !errors.Is(err, backup.ErrNotSaveSet) {
			t.Errorf("%s: %v, want NOTSAVESET of %s", spec.text, err, spec.name)
		}
	}
}

// TestBackupOperationFor: the /SAVE_SET rule.
func TestBackupOperationFor(t *testing.T) {
	files := BackupSpecifier{Text: "[B69...]"}
	set := BackupSpecifier{Text: "B69.BCK", SaveSet: true}

	for _, c := range []struct {
		name          string
		input         BackupSpecifier
		output        *BackupSpecifier
		list, compare bool
		want          BackupOperation
		err           error
	}{
		{"list", set, nil, true, false, BackupList, nil},
		{"list of a file", files, nil, true, false, 0, ErrListNeedsSaveSet},
		{"no output", set, nil, false, false, 0, ErrListNeedsSaveSet},
		{"restore", set, &files, false, false, BackupRestore, nil},
		{"restore and list", set, &files, true, false, BackupRestore, nil},
		{"compare", set, &files, false, true, BackupCompare, nil},
		{"save", files, &set, false, false, BackupSave, nil},
		{"save and list", files, &set, true, false, BackupSave, nil},
		{"two save sets", set, &set, false, false, 0, ErrBothSaveSets},
		{"no save set", files, &files, false, false, 0, ErrNoSaveSet},
		{"compare to a save set", files, &set, false, true, 0, ErrCompareToSaveSet},
	} {
		got, err := BackupOperationFor(c.input, c.output, c.list, c.compare)
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("%s: %v, %v; want %v, %v", c.name, got, err, c.want, c.err)
		}
	}
}
