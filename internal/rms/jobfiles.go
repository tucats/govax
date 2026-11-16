package rms

import (
	"errors"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/ods2/volume"
)

// What SUBMIT asks of RMS (docs/planning/PHASE-98 - batch queues.md,
// subtask 7): the command procedures a job is to run. VMS's SUBMIT opens
// each file "as input" and hands the queue manager its name and file ID,
// so the job runs the file it was given even when a later version is made
// (the phase doc's A.1). A missing file is SUBMIT's OPENIN, naming the
// file as $PARSE expanded it:
//
//	%SUBMIT-F-OPENIN, error opening DUA1:[000000]NOSUCH98.COM; as input
//	-RMS-E-FNF, file not found

// JobFile is one file a job will run: its full name, as $SEARCH writes it
// ("DUA1:[000000]BAT98OK.COM;1"), its device, and its file ID.
type JobFile struct {
	Name string
	ID   FileID
}

// JobFiles finds the files spec names, ".COM" its default type, in
// $SEARCH's order: every match of a wildcard, each element of a search
// list. Each must be readable by the session's process. A spec that names
// none is a *DCLFileError with its expanded name (RMS$_FNF, or RMS$_DNF
// for a directory that isn't there); a file the process may not read is
// one with RMS$_PRV.
func (s *Session) JobFiles(spec string) ([]JobFile, error) {
	p, _, err := s.Parse(spec, ".COM", "", true, false)
	if err != nil {
		return nil, err
	}

	expanded := p.String()

	var files []JobFile

	err = s.eachSpec(expanded, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := s.globChecked(vol, r.Spec)
		if _, denied := IsDenied(err); denied {
			return err
		}

		if err != nil {
			return &NotFoundError{Spec: expanded}
		}

		n := len(files)

		for _, m := range matches {
			if (r.Spec.Name == "" && m.Name != "") || (r.Spec.Type == "" && m.Type != "") {
				continue
			}

			name := r.shownFile(m)

			if _, err := s.openChecked(vol, m, security.Read, name); err != nil {
				return err
			}

			id := FileID{Num: m.Fid.Num, Seq: m.Fid.Seq, Rvn: m.Fid.Rvn, Nmx: m.Fid.Nmx}
			files = append(files, JobFile{Name: name, ID: id})
		}

		if len(files) == n {
			return &NotFoundError{Spec: expanded}
		}

		return nil
	})

	var notFound *NotFoundError

	switch {
	case len(files) > 0 && err == nil:
		return files, nil
	case err == nil || errors.As(err, &notFound) || isNotFound(err):
		return nil, s.NotFound(expanded)
	}

	if e := s.DeniedFileError(err, expanded, false); e != nil {
		return nil, e
	}

	return nil, err
}
