// Package vmsimage decodes a VAX/VMS executable image file: its header
// blocks (the fixed header, activation, symbol table and debug,
// identification, and patch blocks), its image section descriptors, and
// the image activator's fixup section. It neither maps nor runs the
// image; the console's image loader does that.
//
// The layout constants (IHD..., ISD..., IAF..., SHL...) are exported for
// the packages that read or build images: ANALYZE/IMAGE (internal/anl),
// the debug symbol reader (internal/dbgsym), and their tests.
//
// It began as part of internal/anl (Phase 40) and moved here in Phase 41
// so internal/dbgsym could use it without importing ANALYZE. (It isn't
// called "image" so as not to be confused with Go's standard image
// package.)
package vmsimage
