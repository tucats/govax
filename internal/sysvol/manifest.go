package sysvol

import (
	"fmt"
	"sort"
	"strings"
)

// ManifestName is the manifest's file name, in Root: ManifestSpec,
// [SYSTEM]GOVAX$FILES.DAT.
const ManifestName = "GOVAX$FILES.DAT"

// ManifestSpec is the manifest's specification on the volume.
const ManifestSpec = Root + ManifestName

// manifestHeader is the comment the manifest starts with, for a user who
// TYPEs it. A record starting with "!" is a comment, and ParseManifest
// passes over it.
var manifestHeader = []string{
	"! GOVAX$FILES.DAT - the files govax supplied to this system volume, each",
	"! with a SHA-256 digest of what govax wrote. When govax starts, a file whose",
	"! contents still match its digest is replaced by govax's own copy if that",
	"! has changed; a file that doesn't match was changed here, and is kept.",
	"! govax keeps this file up to date; changing it changes which files govax",
	"! treats as yours.",
}

// Manifest is the volume's record of the files govax supplied (Decision 3):
// each file's specification, and the digest of the contents govax last
// wrote there. A later govax compares a file's contents with its digest to
// tell whether the user has changed it since.
//
// On the volume it is a text file, ManifestSpec, one record per file:
//
//	[SYSTEM.SYSLIB]FTNRTL.EXE 3f5a...  (64 hexadecimal digits)
//
// after a few comment records.
type Manifest struct {
	sums map[string]Sum
}

// NewManifest returns an empty manifest.
func NewManifest() *Manifest {
	return &Manifest{sums: map[string]Sum{}}
}

// key is the form a specification is kept under: VMS names are the same
// in any case.
func key(spec string) string {
	return strings.ToUpper(strings.TrimSpace(spec))
}

// Sum returns the digest recorded for the file spec, and whether there
// is one.
func (m *Manifest) Sum(spec string) (Sum, bool) {
	s, ok := m.sums[key(spec)]

	return s, ok
}

// Set records sum as the digest of what govax wrote to the file spec.
func (m *Manifest) Set(spec string, sum Sum) {
	m.sums[key(spec)] = sum
}

// Delete forgets the file spec: govax no longer supplies it.
func (m *Manifest) Delete(spec string) {
	delete(m.sums, key(spec))
}

// Specs returns the specifications the manifest has, sorted.
func (m *Manifest) Specs() []string {
	specs := make([]string, 0, len(m.sums))
	for spec := range m.sums {
		specs = append(specs, spec)
	}

	sort.Strings(specs)

	return specs
}

// Records returns the manifest as its file's records: the header's
// comments, then a record per file, sorted by specification so that the
// same manifest is always the same file.
func (m *Manifest) Records() [][]byte {
	records := make([][]byte, 0, len(manifestHeader)+len(m.sums))

	for _, line := range manifestHeader {
		records = append(records, []byte(line))
	}

	for _, spec := range m.Specs() {
		records = append(records, []byte(spec+" "+m.sums[spec].String()))
	}

	return records
}

// ParseManifest reads a manifest from its file's records. Blank records
// and comments are passed over; any other record that isn't a
// specification and a digest is an error, naming its record number, so
// that a damaged manifest isn't taken for one that lists fewer files.
func ParseManifest(records [][]byte) (*Manifest, error) {
	m := NewManifest()

	for i, r := range records {
		line := strings.TrimSpace(string(r))
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s, record %d: %q is not a file and a digest", ManifestName, i+1, line)
		}

		sum, err := ParseSum(fields[1])
		if err != nil {
			return nil, fmt.Errorf("%s, record %d: %w", ManifestName, i+1, err)
		}

		m.Set(fields[0], sum)
	}

	return m, nil
}
