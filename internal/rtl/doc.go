// Package rtl simulates the VMS runtime-library/system-service calling
// convention: RMS file I/O, CLI, and the LIB$/SYS$ library routines.
// See docs/PHASE-10.md.
//
// Every dispatch-by-code-or-name subsystem here (SYS$ services, LIB$/CRTL
// shims) is a registry keyed by name or numeric code — the same shape as
// internal/cpu's instruction Table/Handler — rather than a switch statement,
// so later phases can register more entries without restructuring dispatch.
package rtl
