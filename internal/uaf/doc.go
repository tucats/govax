// Package uaf is govax's authorization file: the accounts a user may log
// in as, and the rights identifiers they hold (docs/planning/PHASE-95 -
// authentication.md, Decisions 1 to 5).
//
// # What VMS keeps
//
// On VMS each account is a record of SYS$SYSTEM:SYSUAF.DAT, an indexed
// file whose primary key is the username: the account's UIC, its home
// device and directory, its privileges, its priority, its quotas, its
// password's hash, and much more. The rights identifiers (named 32-bit
// values an access control list can grant access to) are records of a
// second file, RIGHTSLIST.DAT, with records saying which accounts hold
// which. The AUTHORIZE utility edits both, and LOGINOUT reads them when
// a user logs in, giving the new process what the record says.
//
// # What govax keeps
//
// govax keeps both in one host file of its own format, JSON, so a person
// can read it and, carefully, edit it: by default ~/.govax/sysuaf.json,
// or the file vax.authorization.file names. A File is the whole of it:
// its accounts (Record) and its identifiers (Identifier), each identifier
// with its holders (Holder). A holder is a UIC, as RIGHTSLIST.DAT's holder
// records are: every account with the UIC holds the identifier (HeldBy).
// Names are names in the file (privileges, flags, attributes), a UIC is
// "[group,member]" in octal as VMS writes it, and an identifier's value
// is "%X80010001". Load reads a file whole
// and checks it, naming the line of anything wrong; Save writes it whole,
// through a temporary file and a rename, so a reader never sees half of
// one; Update does a read, a change, and a write under a lock
// (internal/hostlock) so two govax sessions sharing the file don't lose
// each other's changes; Ensure makes the file the first time, with
// Default's two accounts.
//
// The fields govax keeps are those it has a use for, or will have
// (the phase doc's A.3): no access times, password lifetimes, expiration
// dates, or login failure counts yet. Passwords are hashed with Argon2id
// (HashPassword, CheckPassword); an empty hash means no password.
//
// The package knows nothing of processes, RMS, or DCL: corevms reads it
// when a process logs in and when a service asks about an account.
package uaf
