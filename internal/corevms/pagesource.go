package corevms

import (
	"errors"
	"io"
)

// A section's page source (docs/PHASE-66 - mapped image activation.md,
// subtask 2).
//
// A private section's pages come in from its file when they're first
// touched (filesec.go). The file is a PageSource: a $CRMPSC section's is
// the channel's accessed file (an rms.ACPFile), and an image section's,
// once images are mapped rather than copied, is the image file, which
// the image activator holds open while the image runs: an rms.ACPFile
// for an image on a mounted volume, or a host file (HostPageSource).
//
// Only a section that writes its pages back to its file needs to write
// one, and only an accessed volume file can: such a source is also a
// pageWriter. An image section never writes back (its modified pages
// take the page file as backing store), so a host image needs no writer.

// PageSource is a file a section's pages are read from, one 512-byte
// block per page.
type PageSource interface {
	// ReadPage reads virtual block vbn (from 1) into buf, a page long.
	ReadPage(vbn uint32, buf []byte) error

	// Deaccess lets the file go, once no section uses it (holdFile,
	// dropFile).
	Deaccess() error
}

// pageWriter is a PageSource a section's modified pages can be written
// back to (rms.ACPFile's WritePage).
type pageWriter interface {
	WritePage(vbn uint32, data []byte) error
}

// errReadOnlySource is writePage's error for a source that can't be
// written.
var errReadOnlySource = errors.New("corevms: section file can't be written")

// errPastSource is a HostPageSource's error for a block past the file's
// last.
var errPastSource = errors.New("corevms: block past the end of the file")

// HostPageSource is a host file as a section's page source: an image run
// from the host (RUN/HOST, or a shareable image found there). Its blocks
// are the file's bytes 512 at a time, the last one, if the file isn't a
// whole number of blocks long, filled out with zeros. Nothing stops a
// host program writing the file while it's held; the file is read as it
// is when each page comes in.
type HostPageSource struct {
	r      io.ReaderAt
	closer io.Closer
	size   int64
}

// NewHostPageSource makes a page source of r, size bytes long; Deaccess
// closes closer, if it isn't nil.
func NewHostPageSource(r io.ReaderAt, size int64, closer io.Closer) *HostPageSource {
	return &HostPageSource{r: r, closer: closer, size: size}
}

// Blocks is how many blocks the file has, its last one perhaps partly
// used.
func (h *HostPageSource) Blocks() uint32 {
	return uint32((h.size + pageSize - 1) / pageSize)
}

// ReadPage reads block vbn into buf: errPastSource for block 0 or one past
// the file's last.
func (h *HostPageSource) ReadPage(vbn uint32, buf []byte) error {
	if vbn == 0 || vbn > h.Blocks() {
		return errPastSource
	}

	n, err := h.r.ReadAt(buf[:pageSize], int64(vbn-1)*pageSize)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	clear(buf[n:pageSize])

	return nil
}

// Deaccess closes the file. Closing again does nothing.
func (h *HostPageSource) Deaccess() error {
	c := h.closer
	h.closer = nil

	if c == nil {
		return nil
	}

	return c.Close()
}

// ReleaseSource is the image activator letting go of src, the image file
// it opened, once the image's sections are mapped (at image reset, step
// 8, VMS deassigns the image file's channel; here the activator gives it
// up at once, as $CRMPSC's caller may deassign its channel): sections
// that still use it keep it accessed until the last of them goes
// (dropFile); with none, it's let go now.
func (sys *System) ReleaseSource(src PageSource) error {
	if sys.orphanFile(src) {
		return nil
	}

	return src.Deaccess()
}
