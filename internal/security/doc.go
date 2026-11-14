// Package security is VMS's protection rule: given who is asking (a
// process's Credentials) and what is being asked about (an object's
// Profile: its owner, protection code, and access control list), it
// decides which kinds of access the asker has. Files, directories,
// mailboxes, global sections, and common event flag clusters are all
// protected by this one rule; each caller says which access it needs
// for the operation it is about to do.
//
// The rule is DIGITAL's, from the OpenVMS VAX Guide to System Security
// (VMS 6.0), section 4.3 and Figure 4-1, as VMS 7.1 was found to apply it
// (docs/planning/PHASE-93 - file protection.md, A.1a and A.4). It is
// plain Go: it knows nothing of processes, files, or emulated memory, so
// RMS, corevms, and the tests can all use it.
//
// # Who is asking
//
// A process is identified by its UIC (user identification code), a
// group number and a member number, written in octal as [group,member].
// SYSTEM is [1,4]. A process may also hold rights identifiers: general
// identifiers (named values with bit 31 set, such as the environmental
// identifier INTERACTIVE) that an access control list can name. And it
// has privileges, four of which bear on protection: BYPASS, SYSPRV,
// GRPPRV, and READALL.
//
// # What is asked about
//
// Every protected object has an owner UIC and a protection code. The
// code is a 16-bit mask of four 4-bit fields, one per category of user:
//
//	bits  0-3   SYSTEM   processes in a system group (group number up to
//	                     the SYSGEN parameter MAXSYSGROUP, usually octal
//	                     10), with SYSPRV, with GRPPRV in the owner's
//	                     group, or (for a file) owning the file's volume
//	bits  4-7   OWNER    the process whose UIC is the owner's
//	bits  8-11  GROUP    processes in the owner's UIC group
//	bits 12-15  WORLD    every process
//
// Within a field, bit 0 is read, bit 1 write, bit 2 execute (a device's
// physical I/O), and bit 3 delete (a device's logical I/O). A *set* bit
// denies that access to that category, so a mask of 0 lets everyone do
// everything. A process belongs to every category it qualifies for, and
// the categories add: it has every access any of its categories allows
// (VMS 7.1 opened FROT3.DAT, (S:E,O:D,G:R,W:W), for read and write by a
// process in the group and world categories).
//
// An object may also have an access control list (ACL): a list of
// access control entries (ACEs). Only identifier ACEs bear on access;
// each names one or more identifiers and the access their holder gets.
//
// # The check (Figure 4-1)
//
//  1. The ACL. The first identifier ACE whose identifiers the process
//     holds all of decides, and the scan stops. If it doesn't give the
//     access wanted, only the protection code's SYSTEM and OWNER fields,
//     and privileges, can still give it. An ACE with the DEFAULT option
//     is ignored: it is only a pattern for files created in a directory.
//  2. The protection code, if the ACL didn't decide and the owner UIC
//     isn't [0,0]: every field whose category the process belongs to.
//  3. Privileges: BYPASS gives everything; READALL gives a request for
//     read alone, even if an ACE denied it, but adds nothing to the rest
//     (a write-only file is still refused it for read and write). SYSPRV
//     and GRPPRV work through the SYSTEM category, step 2, so no
//     privilege but BYPASS overrules an ACE's denial except through the
//     SYSTEM field.
//
// An object owned by [0,0] has no protection code check: everyone has
// every access but control, unless the ACL has an identifier ACE, when
// only the ACL or privilege gives access.
//
// Control access, the right to change an object's protection and ACL, is
// never in a protection code. A process has it if it is the owner, if it
// is in the SYSTEM category (and the owner isn't [0,0]), if an ACE gives
// it CONTROL, or if it holds BYPASS. Changing a file's owner needs more:
// CanChangeOwner.
//
// What isn't settled by the guide or the probe is marked "unconfirmed"
// where it is decided.
package security
