package rms

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/isam"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// AnalyzedFile is a file as ANALYZE/RMS_FILE reads it (subtask 10d): what its header
// says, and for a volume file its blocks, which ANALYZE reads directly rather than
// through the record services, so that a damaged file can be described.
type AnalyzedFile struct {
	Attr FileAttributes

	// Cluster is the volume's cluster size, FDL's CLUSTER_SIZE.
	Cluster int

	// Blocks reads the file; nil for a host file. Isam is what isam needs of the
	// header.
	Blocks isam.Blocks
	Isam   isam.Attributes
}

// Indexed and Relative say what the file's organization is.
func (a *AnalyzedFile) Indexed() bool { return a.Attr.Organization == 2 }

// Relative says it's a relative file.
func (a *AnalyzedFile) Relative() bool { return a.Attr.Organization == 1 }

// AnalyzeSearch calls visit for each file spec names, wildcards and all, and with no
// version given only each name's newest: ANALYZE's C81*.FIX. A name with no file is
// a *DCLFileError, as OPEN's. host is an explicit /HOST.
func (s *Session) AnalyzeSearch(spec string, host bool, visit func(*AnalyzedFile) error) error {
	loc, err := s.Locate(spec, host)
	if err != nil {
		return err
	}

	if loc.Host {
		attrs, err := hostFileAttributes(loc.Name)
		if err != nil {
			return s.NotFound(spec)
		}

		return visit(&AnalyzedFile{Attr: attrs, Cluster: 1})
	}

	found := false

	err = s.firstSpec(spec, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := filespec.Glob(vol, r.Spec)
		if err != nil {
			return &NotFoundError{Spec: spec}
		}

		seen := map[string]bool{}

		for _, m := range matches {
			if (r.Spec.Name == "" && m.Name != "") || (r.Spec.Type == "" && m.Type != "") {
				continue
			}

			name := strings.Join(m.Dirs, ".") + "]" + m.Name + "." + m.Type
			if r.Spec.Version == "" && seen[name] {
				continue
			}

			seen[name] = true

			f, err := vol.OpenFID(m.Fid)
			if err != nil {
				return err
			}

			attrs := headerAttributes(f.Header)
			attrs.Name = r.Display + ":" + dirSpec{Elems: m.Dirs}.String() + m.ShortName(';')
			attrs.Device = r.Spec.Device
			attrs.FID = m.Fid.String()
			found = true

			if err := visit(&AnalyzedFile{
				Attr:    attrs,
				Cluster: max(int(f.Device.Home.ClusterSize), 1),
				Blocks:  readOnlyBlocks{f},
				Isam:    indexedAttributes(f),
			}); err != nil {
				return err
			}
		}

		if !found {
			return &NotFoundError{Spec: spec}
		}

		return nil
	})

	var notFound *NotFoundError

	switch {
	case found:
		return err
	case err == nil || errors.As(err, &notFound):
		return s.NotFound(spec)
	default:
		return err
	}
}
