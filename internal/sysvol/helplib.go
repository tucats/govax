package sysvol

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/lbr"
)

// This file is the refresh of a help library (docs/PHASE-90 - VMS help.md,
// Decision 7). HELPLIB.HLB and DBG$HELP.HLB are refreshed module by module
// rather than as whole files, since a user adds help of their own to them
// with LIBRARY/HELP, and a newer govax's help should reach the modules the
// user hasn't touched. The manifest records, beside the library's own
// digest, a digest of each module govax wrote, under the library's
// specification and the module's name ("[SYSTEM.SYSHLP]HELPLIB.HLB(COPY)"),
// and for each module of govax's copy:
//
//   - one missing from the volume's library is inserted, unless the
//     manifest says govax wrote it before (the user deleted it, and it
//     stays deleted);
//   - one whose records are govax's copy already needs nothing;
//   - one whose records match the manifest's digest is govax's, unchanged,
//     and is replaced by the newer copy;
//   - any other is the user's (changed, or inserted by them under a name
//     govax's help now also has), kept, and reported once per new copy
//     with SYSKEPT naming the module.
//
// A module of the volume's library that govax no longer supplies is taken
// out if it's what govax wrote (the manifest's digest), and kept if it's
// the user's. When anything changes, the library is written as a new
// version, as LIBRARY/REPLACE would leave it. A volume library that can't
// be read as a library is the user's file, kept whole.

// moduleSpec is the manifest's name for the module name of the library
// spec.
func moduleSpec(spec, name string) string {
	return spec + "(" + strings.ToUpper(name) + ")"
}

// moduleSum is a module's digest: of its records, one per line.
func moduleSum(records [][]byte) Sum {
	var b strings.Builder

	for _, r := range records {
		b.Write(r)
		b.WriteByte('\n')
	}

	return SumOf([]byte(b.String()))
}

// libraryModules reads a library's modules: its entries in index order,
// with their records and insertion times.
func libraryModules(data []byte) (*lbr.Library, map[string]*lbr.Entry, []string, error) {
	l, err := lbr.Open(data)
	if err != nil {
		return nil, nil, nil, err
	}

	if l.Type != lbr.TypeHelp || len(l.Indexes) == 0 {
		return nil, nil, nil, fmt.Errorf("not a help library")
	}

	modules := map[string]*lbr.Entry{}

	var names []string

	for _, k := range l.Indexes[0].Keys {
		m, err := l.Module(k.RFA)
		if err != nil {
			return nil, nil, nil, err
		}

		key := strings.ToUpper(k.Name)
		modules[key] = &lbr.Entry{Name: k.Name, Records: m.Records, Inserted: m.Header.Inserted}
		names = append(names, key)
	}

	return l, modules, names, nil
}

// recordModules records in m the digest of each module of ours, govax's
// copy of the library spec, which the volume now has as govax wrote it,
// and forgets the records of modules ours doesn't have.
func recordModules(m *Manifest, spec string, ours []byte) error {
	_, modules, _, err := libraryModules(ours)
	if err != nil {
		return fmt.Errorf("%s: govax's copy: %w", spec, err)
	}

	prefix := strings.ToUpper(spec) + "("

	for _, s := range m.Specs() {
		if name, ok := strings.CutPrefix(s, prefix); ok {
			if _, kept := modules[strings.TrimSuffix(name, ")")]; !kept {
				m.Delete(s)
			}
		}
	}

	for key, e := range modules {
		m.Set(moduleSpec(spec, key), moduleSum(e.Records))
	}

	return nil
}

// refreshHelpLibrary is refreshFile's work for a help library the volume
// has and whose contents aren't govax's copy: theirs is the volume's
// library, ours govax's.
func refreshHelpLibrary(v Volume, m *Manifest, f File, theirs, ours []byte) (Change, error) {
	change := Change{File: f, Action: Kept}
	spec := f.Spec()

	theirLib, theirModules, theirNames, err := libraryModules(theirs)
	if err != nil {
		// Not a library govax can read: the user's file, whole.
		return keptWhole(m, f, ours), nil
	}

	_, ourModules, ourNames, err := libraryModules(ours)
	if err != nil {
		return change, fmt.Errorf("%s: govax's copy: %w", spec, err)
	}

	b, err := lbr.Edit(theirLib)
	if err != nil {
		return keptWhole(m, f, ours), nil
	}

	changed := false

	for _, key := range ourNames {
		mine := ourModules[key]
		ms := moduleSpec(spec, key)
		sum := moduleSum(mine.Records)
		recorded, listed := m.Sum(ms)
		there, has := theirModules[key]

		switch {
		case !has && listed:
			// govax wrote it before, and the user deleted it.
		case !has:
			if err := b.Insert(copyEntry(mine)); err != nil {
				return change, fmt.Errorf("%s: %w", ms, err)
			}

			changed = true
			m.Set(ms, sum)
		case moduleSum(there.Records) == sum:
			if !listed || recorded != sum {
				m.Set(ms, sum)
			}
		case listed && moduleSum(there.Records) == recorded:
			if _, err := b.Replace(copyEntry(mine)); err != nil {
				return change, fmt.Errorf("%s: %w", ms, err)
			}

			changed = true
			m.Set(ms, sum)
		default:
			told, wasTold := m.Kept(ms)
			if (!listed || recorded != sum) && (!wasTold || told != sum) {
				change.KeptModules = append(change.KeptModules, there.Name)

				m.SetKept(ms, sum)
			}
		}
	}

	for _, key := range theirNames {
		if _, ok := ourModules[key]; ok {
			continue
		}

		ms := moduleSpec(spec, key)
		if recorded, listed := m.Sum(ms); listed && recorded == moduleSum(theirModules[key].Records) {
			if err := b.Delete(theirModules[key].Name); err != nil {
				return change, fmt.Errorf("%s: %w", ms, err)
			}

			changed = true

			m.Delete(ms)
		}
	}

	sort.Strings(change.KeptModules)

	if !changed {
		if len(change.KeptModules) == 0 && !hasUserModules(theirModules, ourModules) {
			change.Action = Current
		}

		return change, nil
	}

	b.Updated = latestInsertion(b)

	data := b.Bytes()
	if err := writeFile(v, f, Canonical(data, f.Format)); err != nil {
		return change, err
	}

	change.Action = Updated
	m.Set(spec, SumOf(Canonical(data, f.Format)))

	return change, nil
}

// hasUserModules reports whether the volume's library holds a module that
// isn't govax's as govax supplies it now: one the user changed or added.
func hasUserModules(theirs, ours map[string]*lbr.Entry) bool {
	for key, e := range theirs {
		mine, ok := ours[key]
		if !ok || moduleSum(mine.Records) != moduleSum(e.Records) {
			return true
		}
	}

	return false
}

// keptWhole is a help library the refresh can't read as one, left alone
// as the user's file, reported as refreshFile reports any file it keeps.
func keptWhole(m *Manifest, f File, ours []byte) Change {
	change := Change{File: f, Action: Kept}
	spec := f.Spec()
	sum := SumOf(ours)

	recorded, listed := m.Sum(spec)
	told, wasTold := m.Kept(spec)

	if (!listed || recorded != sum) && (!wasTold || told != sum) {
		change.Notify = true

		m.SetKept(spec, sum)
	}

	return change
}

// copyEntry is a copy of e, so that a Builder's changes to its entry
// don't reach govax's copy of the library.
func copyEntry(e *lbr.Entry) *lbr.Entry {
	c := *e

	return &c
}

// latestInsertion is the latest insertion time of b's modules: the
// librarian's revision time after it inserts them.
func latestInsertion(b *lbr.Builder) uint64 {
	latest := b.Updated

	for _, name := range b.Names() {
		if e, ok := b.Module(name); ok && e.Inserted > latest {
			latest = e.Inserted
		}
	}

	return latest
}
