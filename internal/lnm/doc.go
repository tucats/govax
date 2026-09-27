// Package lnm is govax's VAX/VMS logical-name database: the shared store
// behind the console's DEFINE/ASSIGN/DEASSIGN/SHOW LOGICAL commands, RMS
// file-specification translation, and the $CRELNM/$DELLNM/$TRNLNM/$CRELNT
// system services (docs/PHASE-25.md).
//
// It models VMS 7.3's structure rather than a flat name map:
//
//   - Logical names live in named tables. A name may be defined once per
//     access mode in the same table, and may have up to 128 equivalence
//     strings (a search list), each with its own CONCEALED/TERMINAL
//     translation attributes.
//   - Tables are themselves found by name through two directory tables,
//     LNM$PROCESS_DIRECTORY and LNM$SYSTEM_DIRECTORY. A directory holds
//     table-name entries and ordinary logical names that translate to table
//     names, so LNM$PROCESS, LNM$GROUP, LNM$SYSTEM, and the LNM$FILE_DEV
//     search list are plain logical names that callers can redefine.
//   - Every table except a directory has a parent table. Deleting a table
//     deletes its descendants too.
//
// The behavioral reference is the VMS System Services Reference Manual's
// $CRELNM/$CRELNT/$DELLNM/$TRNLNM descriptions and the VSI OpenVMS User's
// Manual, chapter 11. Status codes are returned as vmserrors.VMSError
// values carrying the real $SSDEF numbers.
//
// govax has no privilege model yet, so the caller is treated as holding
// every privilege (SYSNAM, GRPNAM, SYSPRV): the privilege checks those
// services describe are not made. Structural rules that apply even to a
// privileged caller still are, such as a name's access mode never being
// more privileged than its table's, and the tables created at startup
// never being deleted.
//
// A Database is not safe for concurrent use.
package lnm
