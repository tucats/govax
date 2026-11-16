// Package authorize is AUTHORIZE, VMS's utility for the authorization
// file, as govax has it (docs/planning/PHASE-95 - authentication.md,
// Decisions 5 and 10): the accounts and identifiers internal/uaf keeps,
// changed by commands typed at a "UAF> " prompt or given one at a time
// ("AUTHORIZE ADD SMITH/UIC=[200,1]", "MCR AUTHORIZE ...").
//
// The commands are a grammar of their own, authorize.dcl in
// internal/bootdata, parsed by internal/dcl as the debugger's is; this
// package binds a handler to each verb. A Utility is one run: where the
// file is (a Store, corevms's System in govax), whose credentials ask
// for it, where its output goes, and where a session's commands come from.
//
// What AUTHORIZE prints and leaves in $STATUS is VMS 7.1's, from
// testdata/uaf95's runs (the phase doc's A.5): each message is
// "%UAF-s-IDENT, text", a value the message is about between backslashes
// after it, and a system status under it as "-SYSTEM-F-DUPIDENT, ...".
// The status a one-command run leaves follows no one rule on VMS (some
// errors leave SS$_NORMAL, some AUTHORIZE's own condition, some another
// facility's with the inhibit bit), so each command says its own.
//
// The authorization file is a host file outside the volumes, so its
// protection is given here in Go, SYSUAF.DAT's on VMS: owner [1,4],
// (S:RWE,O:RWE,G,W). A command that reads the file needs read access,
// one that changes it write access too, by internal/security's rule
// with the process's credentials (Decision 5). Without it AUTHORIZE says
// NAOFIL with RMS's PRV, as VMS 7.1 did for an account without SYSPRV.
//
// This subtask (5) has the account commands: ADD, COPY, DEFAULT,
// MODIFY, REMOVE, RENAME, and EXIT. SHOW, LIST, and the identifier
// commands are subtask 6's.
package authorize
