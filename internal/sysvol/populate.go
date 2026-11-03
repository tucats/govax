package sysvol

import (
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

// Populate fills a newly made system volume: every directory, then every
// file (a seeded file only if it isn't there), then the manifest of the
// supplied files. It returns the manifest it wrote.
func Populate(v Volume) (*Manifest, error) {
	for _, dir := range Directories {
		if err := v.CreateDirectory(dir); err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
	}

	m := NewManifest()

	for _, f := range Files() {
		if f.Kind == Seeded {
			if _, found, err := v.ReadFile(f.Spec()); err != nil {
				return nil, fmt.Errorf("%s: %w", f.Spec(), err)
			} else if found {
				continue
			}
		}

		contents, err := f.Contents()
		if err != nil {
			return nil, err
		}

		if err := v.WriteFile(f.Spec(), Records(contents, f.Format), f.Format); err != nil {
			return nil, fmt.Errorf("%s: %w", f.Spec(), err)
		}

		if f.Kind == Supplied {
			m.Set(f.Spec(), SumOf(contents))
		}
	}

	if err := WriteManifest(v, m); err != nil {
		return nil, err
	}

	return m, nil
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
