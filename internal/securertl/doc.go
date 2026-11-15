// Package securertl is SECURESHR.EXE's routines: the security services VMS
// keeps in a shareable image rather than in the system service vector
// (docs/planning/PHASE-93 - file protection.md, subtask 10, and A.4's
// "Round 3", where VMS 7.1's map of a program calling them showed where
// each service is).
//
// A program calls SYS$PARSE_ACL, SYS$FORMAT_ACL, SYS$CHANGE_ACL, or
// SYS$CHECK_ACCESS as it calls any system service, with CALLS or CALLG,
// but LINK finds them in SECURESHR (through IMAGELIB on VMS, through
// govax's own tables here), so the program's image names SECURESHR and an
// offset in its transfer vector. govax stands for the image as it does for
// LIBRTL (internal/librtl) and LBRSHR (internal/lbrrtl): each entry is a
// stub whose XFC$SHIM code runs a Go routine.
//
//   - $PARSE_ACL turns an ACE's text into its bytes, and $FORMAT_ACL its
//     bytes into text, by internal/security's grammar (acetext.go), which
//     DCL's SET SECURITY/ACL and SHOW SECURITY share.
//   - $CHANGE_ACL reads and changes a file's ACL, found by name or by the
//     channel it is accessed on (rms.SecurityObject), by an item list
//     that works on one position in the ACL (change.go).
//   - $CHECK_ACCESS asks whether a user, named, may access a file or a
//     device, by the user's account (corevms.UserAccount) and $CHKPRO's
//     item list (checkaccess.go).
//
// VMS 7.1 also linked SECURESHRP, a protected image SECURESHR uses; govax
// stands for it too (machine's runtimeLibraries), so an image VMS's LINK
// made activates.
package securertl
