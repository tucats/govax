// Package anl is VMS's ANALYZE utility: ANALYZE/OBJECT now, and
// ANALYZE/IMAGE later. See docs/PHASE-38.md.
//
// An analyzer turns a file's contents into Lines, the text of a report
// with the hints the page layout needs; a Pager lays the lines out on
// pages as VMS 7.3's ANALYZE (ANALYZ V07-04) does, with its page headers
// and closing line. The two are separate so each analyzer can be tested
// for content alone, and so every analyzer shares one page layout.
//
// The output is matched to real ANALYZE's, which is the only source for
// it (the project is clean room): testdata holds 54 object modules with
// VMS's analysis of each beside them. The text is 8-bit: a hex dump shows
// DEC Multinational characters as they are, so a report's strings hold
// ISO Latin-1 bytes, not UTF-8.
//
// The package reads records already split from their file; finding the
// file, on the host or on a mounted volume, is the console's job.
package anl
