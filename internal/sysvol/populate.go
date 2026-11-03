package sysvol

import (
	"bytes"
	"fmt"
)

// Volume is what this package needs of the system volume. Specifications
// are a directory and a file name, with no device ("[SYSTEM.SYSLIB]FTNRTL.EXE"):
// the implementation knows which volume it writes. govax's is over RMS
// (subtask 3); the tests' is a map.
type Volume interface {
	// CreateDirectory makes the directory dir ("[SYSTEM.SYSLIB]"), whose
	// parent exists. A directory that exists already is not an error.
	CreateDirectory(dir string) error

	// WriteFile writes records as a new version of the file spec, laid out
	// as format f says.
	WriteFile(spec string, records [][]byte, f Format) error

	// ReadFile returns the records of the highest version of the file
	// spec, and true, or false (and no error) if there is no such file.
	ReadFile(spec string) (records [][]byte, found bool, err error)
}

// Action is what Refresh did with one file.
type Action int

const (
	// Current is a file that already holds govax's copy: nothing written.
	Current Action = iota

	// Written is a file that was missing, written from govax's copy (a
	// seeded file the first time only).
	Written

	// Updated is a supplied file the user hadn't changed, whose newer
	// govax copy was written as a new version.
	Updated

	// Kept is a supplied file the user has changed (its contents aren't
	// what the manifest says govax wrote), left alone though govax's copy
	// differs.
	Kept

	// Left is a seeded file left alone: there, or made once before and
	// since deleted by the user.
	Left
)

// String names an Action as the tests do.
func (a Action) String() string {
	switch a {
	case Current:
		return "current"
	case Written:
		return "written"
	case Updated:
		return "updated"
	case Kept:
		return "kept"
	case Left:
		return "left"
	}

	return fmt.Sprintf("Action(%d)", int(a))
}

// Change is what Refresh did with one file.
type Change struct {
	File   File
	Action Action

	// Notify is set on a Kept file whose govax copy the user hasn't been
	// told about yet: the caller says it wasn't installed (MOUNT's
	// SYSKEPT). The manifest then remembers that copy, so each new copy
	// is reported once.
	Notify bool
}

// Report is what Refresh did.
type Report struct {
	// Manifest is the volume's manifest as Refresh left it.
	Manifest *Manifest

	// Changes has an entry per file in Files(), in its order.
	Changes []Change

	// ManifestWritten is whether a new version of the manifest was
	// written; it isn't when nothing in it changed.
	ManifestWritten bool
}

// Count returns how many files Refresh did a with.
func (r *Report) Count(a Action) int {
	n := 0

	for _, c := range r.Changes {
		if c.Action == a {
			n++
		}
	}

	return n
}

// Refresh brings the system volume up to date with this govax's files
// (Decision 3), and is what fills a new one too (Populate). It makes any
// directory that's missing, then for each file in Files():
//
//   - a supplied file that is missing is written (Written);
//   - one whose contents are govax's copy already needs nothing (Current),
//     and if the manifest's digest is another, it is corrected: the volume
//     had the copy but not its record, as after a refresh that stopped
//     before it wrote the manifest;
//   - one whose contents match the manifest's digest is govax's, unchanged
//     by the user, and govax's newer copy is written as a new version
//     (Updated), the old one kept below it;
//   - any other is the user's (Kept): changed, or not in the manifest at
//     all, and never written. Notify says the user should be told, the
//     first time each new govax copy finds it changed;
//   - a seeded file is written only if the manifest doesn't say it was
//     made before and it isn't there (Written); otherwise it is Left.
//
// A manifest record for a file govax no longer supplies is kept, so a
// later govax that supplies the file again still knows whether the user
// changed it. The manifest is written last, and only if it changed.
//
// A failure ends Refresh with an error naming the file, and the manifest
// isn't written. That leaves no harm: a file written before the failure
// holds govax's copy, which the next refresh finds Current and records.
// A damaged manifest is an error before anything is written, so that
// files whose ownership can't be told are never replaced.
func Refresh(v Volume) (*Report, error) {
	return refreshFiles(v, Files())
}

// refreshFiles is Refresh for the files in files, which the tests choose,
// standing in for a later govax's copies.
func refreshFiles(v Volume, files []File) (*Report, error) {
	for _, dir := range Directories {
		if err := v.CreateDirectory(dir); err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
	}

	m, found, err := ReadManifest(v)
	if err != nil {
		return nil, err
	}

	// The manifest's records as they are, to tell at the end whether it
	// changed; a volume without one always gets one.
	var before []byte
	if found {
		before = bytes.Join(m.Records(), []byte{'\n'})
	}

	report := &Report{Manifest: m}

	for _, f := range files {
		change, err := refreshFile(v, m, f)
		if err != nil {
			return nil, err
		}

		report.Changes = append(report.Changes, change)
	}

	if !found || !bytes.Equal(before, bytes.Join(m.Records(), []byte{'\n'})) {
		if err := WriteManifest(v, m); err != nil {
			return nil, err
		}

		report.ManifestWritten = true
	}

	return report, nil
}

// refreshFile does Refresh's work for one file f, recording in m what it
// did, and returns the change.
func refreshFile(v Volume, m *Manifest, f File) (Change, error) {
	change := Change{File: f}
	spec := f.Spec()

	records, exists, err := v.ReadFile(spec)
	if err != nil {
		return change, fmt.Errorf("%s: %w", spec, err)
	}

	if f.Kind == Seeded {
		change.Action = Left

		if !exists && !m.Seeded(spec) {
			contents, err := f.Contents()
			if err != nil {
				return change, err
			}

			if err := writeFile(v, f, contents); err != nil {
				return change, err
			}

			change.Action = Written
		}

		m.SetSeeded(spec)

		return change, nil
	}

	contents, err := f.Contents()
	if err != nil {
		return change, err
	}

	ours := SumOf(contents)

	if !exists {
		if err := writeFile(v, f, contents); err != nil {
			return change, err
		}

		change.Action = Written
		m.Set(spec, ours)

		return change, nil
	}

	theirs := SumOf(Join(records, f.Format))
	recorded, listed := m.Sum(spec)

	switch {
	case theirs == ours:
		change.Action = Current

		if !listed || recorded != ours {
			m.Set(spec, ours)
		}

	case listed && theirs == recorded:
		if err := writeFile(v, f, contents); err != nil {
			return change, err
		}

		change.Action = Updated
		m.Set(spec, ours)

	default:
		change.Action = Kept

		// Decision 3: the user hears of it only when govax's copy has
		// changed since what it last wrote, and only once per copy.
		told, wasTold := m.Kept(spec)
		if (!listed || recorded != ours) && (!wasTold || told != ours) {
			change.Notify = true

			m.SetKept(spec, ours)
		}
	}

	return change, nil
}

// writeFile writes contents, govax's copy of f, as a new version.
func writeFile(v Volume, f File, contents []byte) error {
	if err := v.WriteFile(f.Spec(), Records(contents, f.Format), f.Format); err != nil {
		return fmt.Errorf("%s: %w", f.Spec(), err)
	}

	return nil
}

// Populate fills a newly made system volume: every directory, then every
// file, then the manifest. It is Refresh on a volume with nothing on it
// (a seeded file already there is left alone), and returns the manifest
// it wrote.
func Populate(v Volume) (*Manifest, error) {
	report, err := Refresh(v)
	if err != nil {
		return nil, err
	}

	return report.Manifest, nil
}

// ReadManifest reads the volume's manifest. A volume with none (one made
// by hand, or whose manifest was deleted) has an empty one, and found is
// false.
func ReadManifest(v Volume) (m *Manifest, found bool, err error) {
	records, found, err := v.ReadFile(ManifestSpec)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", ManifestSpec, err)
	}

	if !found {
		return NewManifest(), false, nil
	}

	m, err = ParseManifest(records)
	if err != nil {
		return nil, true, err
	}

	return m, true, nil
}

// WriteManifest writes m as a new version of the volume's manifest.
func WriteManifest(v Volume, m *Manifest) error {
	if err := v.WriteFile(ManifestSpec, m.Records(), Text); err != nil {
		return fmt.Errorf("%s: %w", ManifestSpec, err)
	}

	return nil
}
