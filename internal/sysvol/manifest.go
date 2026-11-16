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

// UpdateLogSpec is the update log's specification on the volume: a text
// file of a line for each file or help library module a refresh of an
// existing volume wrote (Report.Updates), made at the first such refresh
// and added to at each later one. It isn't one of Files(): govax only
// ever adds lines to it, and the user may delete it.
const UpdateLogSpec = Root + "GOVAX$UPDATE.LOG"

// The words a manifest record may have after its specification, beside a
// digest: keptWord starts the digest of a newer govax copy the user has
// been told about (KEPT=3f5a...), seededWord marks a seeded file, and
// ownerWord and protectionWord, always together and last, are the profile
// govax gave the file or directory (profile.go).
const (
	keptWord       = "KEPT="
	seededWord     = "SEEDED"
	ownerWord      = "OWNER="
	protectionWord = "PROTECTION="
)

// manifestHeader is the comment the manifest starts with, for a user who
// TYPEs it. A record starting with "!" is a comment, and ParseManifest
// passes over it.
var manifestHeader = []string{
	"! GOVAX$FILES.DAT - the files govax supplied to this system volume, each",
	"! with a SHA-256 digest of what govax wrote. When govax starts, a file whose",
	"! contents still match its digest is replaced by govax's own copy if that",
	"! has changed; a file that doesn't match was changed here, and is kept.",
	"! KEPT= is the digest of the newer copy govax last said it didn't install;",
	"! SEEDED marks a file govax made once and leaves to you. OWNER= and",
	"! PROTECTION= are the owner and protection govax gave a file or directory;",
	"! govax sets them again only when it wants others, so SET SECURITY's stay.",
	"! govax keeps this file up to date; changing it changes which files govax",
	"! treats as yours.",
}

// entry is what the manifest knows of one file.
type entry struct {
	// sum is the digest of what govax last wrote to a supplied file, if
	// hasSum.
	sum    Sum
	hasSum bool

	// kept is the digest of govax's copy of a supplied file the user had
	// changed, which govax has said it didn't install, if hasKept: the
	// same copy isn't reported again.
	kept    Sum
	hasKept bool

	// seeded is whether this is a seeded file govax has made (or found
	// made), so it is never made again.
	seeded bool

	// profile is the owner and protection govax last gave the file or
	// directory, if hasProfile.
	profile    Profile
	hasProfile bool
}

// Manifest is the volume's record of the files govax supplied (Decision 3):
// each file's specification, and the digest of the contents govax last
// wrote there. A later govax compares a file's contents with its digest to
// tell whether the user has changed it since.
//
// On the volume it is a text file, ManifestSpec, one record per file,
// after a few comment records:
//
//	[SYSTEM.SYSLIB]FTNRTL.EXE 3f5a...                  (64 hexadecimal digits)
//	[SYSTEM.SYSMGR]CONSOLE.DCL 81c0... KEPT=d2e4...    (a newer copy not installed)
//	[SYSTEM.SYSMGR]SYSTARTUP_VMS.COM SEEDED            (made once, the user's)
//
// and each record, a directory's too ([SYSTEM.SYSLIB], with nothing else),
// ends with the profile govax gave it (profile.go):
//
//	[SYSTEM.SYSLIB]FTNRTL.EXE 3f5a... OWNER=[1,4] PROTECTION=(S:RWED,O:RWED,G:RE,W:RE)
type Manifest struct {
	entries map[string]entry
}

// NewManifest returns an empty manifest.
func NewManifest() *Manifest {
	return &Manifest{entries: map[string]entry{}}
}

// key is the form a specification is kept under: VMS names are the same
// in any case.
func key(spec string) string {
	return strings.ToUpper(strings.TrimSpace(spec))
}

// Sum returns the digest recorded for the file spec, and whether there
// is one.
func (m *Manifest) Sum(spec string) (Sum, bool) {
	e := m.entries[key(spec)]

	return e.sum, e.hasSum
}

// Set records sum as the digest of what govax wrote to the file spec.
// govax's copy is now the file, so any KEPT note goes.
func (m *Manifest) Set(spec string, sum Sum) {
	e := m.entries[key(spec)]
	e.sum, e.hasSum = sum, true
	e.kept, e.hasKept = Sum{}, false
	m.entries[key(spec)] = e
}

// Kept returns the digest of govax's copy of spec that the user was last
// told wasn't installed, and whether there is one.
func (m *Manifest) Kept(spec string) (Sum, bool) {
	e := m.entries[key(spec)]

	return e.kept, e.hasKept
}

// SetKept records that the user has been told govax's copy of spec, whose
// digest is sum, wasn't installed over their changed file.
func (m *Manifest) SetKept(spec string, sum Sum) {
	e := m.entries[key(spec)]
	e.kept, e.hasKept = sum, true
	m.entries[key(spec)] = e
}

// Seeded returns whether the seeded file spec has been made (or found).
func (m *Manifest) Seeded(spec string) bool {
	return m.entries[key(spec)].seeded
}

// SetSeeded records that the seeded file spec has been made (or found),
// so govax never makes it again.
func (m *Manifest) SetSeeded(spec string) {
	e := m.entries[key(spec)]
	e.seeded = true
	m.entries[key(spec)] = e
}

// Profile returns the profile govax last gave the file or directory spec,
// and whether there is one.
func (m *Manifest) Profile(spec string) (Profile, bool) {
	e := m.entries[key(spec)]

	return e.profile, e.hasProfile
}

// SetProfile records that govax gave the file or directory spec the
// profile p.
func (m *Manifest) SetProfile(spec string, p Profile) {
	e := m.entries[key(spec)]
	e.profile, e.hasProfile = p, true
	m.entries[key(spec)] = e
}

// Delete forgets the file spec: govax no longer supplies it.
func (m *Manifest) Delete(spec string) {
	delete(m.entries, key(spec))
}

// Specs returns the specifications the manifest has, sorted.
func (m *Manifest) Specs() []string {
	specs := make([]string, 0, len(m.entries))
	for spec := range m.entries {
		specs = append(specs, spec)
	}

	sort.Strings(specs)

	return specs
}

// Records returns the manifest as its file's records: the header's
// comments, then a record per file, sorted by specification so that the
// same manifest is always the same file.
func (m *Manifest) Records() [][]byte {
	records := make([][]byte, 0, len(manifestHeader)+len(m.entries))

	for _, line := range manifestHeader {
		records = append(records, []byte(line))
	}

	for _, spec := range m.Specs() {
		e := m.entries[spec]
		fields := []string{spec}

		if e.hasSum {
			fields = append(fields, e.sum.String())
		}

		if e.hasKept {
			fields = append(fields, keptWord+e.kept.String())
		}

		if e.seeded {
			fields = append(fields, seededWord)
		}

		if e.hasProfile {
			fields = append(fields, e.profile.String())
		}

		records = append(records, []byte(strings.Join(fields, " ")))
	}

	return records
}

// ParseManifest reads a manifest from its file's records. Blank records
// and comments are passed over; any other record that isn't a
// specification followed by a digest, a KEPT= digest, or both, or by
// SEEDED, is an error, naming its record number, so that a damaged
// manifest isn't taken for one that lists fewer files.
func ParseManifest(records [][]byte) (*Manifest, error) {
	m := NewManifest()

	for i, r := range records {
		line := strings.TrimSpace(string(r))
		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}

		fields := strings.Fields(line)

		e, err := parseEntry(fields)
		if err != nil {
			return nil, fmt.Errorf("%s, record %d: %w", ManifestName, i+1, err)
		}

		m.entries[key(fields[0])] = e
	}

	return m, nil
}

// parseEntry reads one manifest record's fields: the specification,
// then a digest, a KEPT= digest, or both, in that order, or SEEDED; then,
// last, the OWNER= and PROTECTION= words of its profile, which may stand
// alone (a directory's record). A KEPT= note alone is a file never govax's
// (one there before the manifest was), whose newer govax copy the user has
// been told about.
func parseEntry(fields []string) (entry, error) {
	var e entry

	bad := fmt.Errorf("%q is not a file and a digest", strings.Join(fields, " "))

	if n := len(fields); n >= 3 && hasWord(fields[n-2], ownerWord) && hasWord(fields[n-1], protectionWord) {
		p, err := parseProfile(fields[n-2], fields[n-1])
		if err != nil {
			return e, err
		}

		e.profile, e.hasProfile = p, true
		fields = fields[:n-2]

		if len(fields) == 1 {
			return e, nil
		}
	}

	if len(fields) == 2 && strings.EqualFold(fields[1], seededWord) {
		e.seeded = true

		return e, nil
	}

	if len(fields) < 2 || len(fields) > 3 {
		return e, bad
	}

	rest := fields[1:]

	if !hasKeptWord(rest[0]) {
		sum, err := ParseSum(rest[0])
		if err != nil {
			return e, err
		}

		e.sum, e.hasSum = sum, true
		rest = rest[1:]
	}

	if len(rest) == 0 {
		return e, nil
	}

	if !hasKeptWord(rest[0]) || len(rest) > 1 {
		return e, bad
	}

	kept, err := ParseSum(rest[0][len(keptWord):])
	if err != nil {
		return e, err
	}

	e.kept, e.hasKept = kept, true

	return e, nil
}

// hasKeptWord returns whether a manifest field is a KEPT= note.
func hasKeptWord(field string) bool {
	return hasWord(field, keptWord)
}

// hasWord returns whether a manifest field starts with the keyword word
// ("KEPT=", "OWNER="), in any case.
func hasWord(field, word string) bool {
	return len(field) >= len(word) && strings.EqualFold(field[:len(word)], word)
}
