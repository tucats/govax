package rms

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// This file is how RMS reads a host file that a command names as its
// input: COPY's /HOST source, LIBRARY's library and input files, MACRO's
// sources, LINK's objects, and anything else read through ReadRawFile or
// ReadRecordFile.
//
// A host name with no directory in it ("starlet.mlb") that isn't in the
// current directory is also looked for through Session.HostFallback. The
// console points that at its search-path resolver (internal/respath): the
// -path directories, then govax's embedded files (internal/bootdata), so
// that "LIBRARY/LIST starlet.mlb" finds govax's own STARLET.MLB with no
// copy of it on the host. A name with a directory in it means exactly
// that file, and is never looked for anywhere else.

// readHostFile reads the host file path, or, when it doesn't exist and
// path is a bare name, the file of that name HostFallback finds. A
// directory is an error. If the fallback finds nothing either, the error
// is the one reading path itself gave.
func (s *Session) readHostFile(path string) ([]byte, error) {
	info, err := os.Stat(path)

	switch {
	case err == nil && info.IsDir():
		return nil, fmt.Errorf("%s: is a directory, not a file", path)

	case err == nil:
		return os.ReadFile(path)

	case errors.Is(err, fs.ErrNotExist):
		if data, ok := s.readHostFallback(path); ok {
			return data, nil
		}
	}

	return nil, err
}

// readHostFallback looks for the bare name through HostFallback. Since a
// VMS file name has no case, and govax's embedded files are all in lower
// case, a name typed in upper case ("STARLET.MLB") is tried in lower case
// too.
func (s *Session) readHostFallback(name string) ([]byte, bool) {
	if s.HostFallback == nil || name != filepath.Base(name) {
		return nil, false
	}

	for _, n := range []string{name, strings.ToLower(name)} {
		if data, err := s.HostFallback(n); err == nil {
			return data, true
		}
	}

	return nil, false
}
