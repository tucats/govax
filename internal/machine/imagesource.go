package machine

import (
	"errors"
	"io"
	"os"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// OpenImage is ReadImage for an image that's mapped rather than copied
// (docs/PHASE-66 - mapped image activation.md, subtask 2): it finds the
// image file fn by the same rules, and returns its bytes (for the
// header, the ISDs, and the debug symbols) with the file held open as
// the page source its sections read their pages from. An image on a
// mounted volume is accessed for reading, others allowed to read it but
// not write it (rms.Session.AccessImage); a host image is an open host
// file (corevms.HostPageSource), its writers not stopped.
//
// The caller hands the source to the sections it maps, then lets it go
// with corevms.System.ReleaseSource, which leaves it open for as long as
// a section uses it.
func (m *Machine) OpenImage(fn string, main bool) ([]byte, corevms.PageSource, error) {
	data, src, _, err := m.openImage(fn, main)

	return data, src, err
}

// openImage is OpenImage, also returning where the image was found.
func (m *Machine) openImage(fn string, main bool) ([]byte, corevms.PageSource, imageSpec, error) {
	if s := m.Hooks.Session(); main && s != nil {
		loc, err := s.Locate(fn, m.runHost)
		if err != nil {
			return nil, nil, imageSpec{}, err
		}

		if !loc.Host {
			a, data, found, err := s.AccessImage(loc.WithDefaultType("EXE"))

			var notFound *rms.NotFoundError
			if errors.As(err, &notFound) {
				return nil, nil, imageSpec{}, vmserrors.New(vmserrors.RMS_IMAGENOTFOUND, fn)
			}

			if err != nil {
				return nil, nil, imageSpec{}, err
			}

			return data, a, imageSpec{Name: found.Physical}, nil
		}
	}

	path, ok := m.findImage(fn)
	if !ok {
		return nil, nil, imageSpec{}, vmserrors.New(vmserrors.RMS_IMAGENOTFOUND, fn)
	}

	data, src, err := openHostImage(path)

	return data, src, hostImageSpec(path), err
}

// openHostImage opens the host file at path as an image's page source,
// returning its contents too.
func openHostImage(path string) ([]byte, corevms.PageSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		_ = f.Close()

		return nil, nil, err
	}

	return data, corevms.NewHostPageSource(f, int64(len(data)), f), nil
}
