package uaf

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Passwords (the phase doc's Decision 4).
//
// A password is never kept, only a hash of it: a value from which the
// password can't be worked out, but which the same password always gives
// again. VMS's SYSUAF.DAT keeps a hash of its own (Purdy's polynomial);
// govax keeps Argon2id's (RFC 9106), a hash made to be slow and to need
// much memory, so that guessing passwords from a stolen file is costly.
// Each hash has a random salt, so two accounts with one password have
// different hashes.
//
// The hash is kept as a "PHC string", which names its own parameters:
//
//	$argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
//
// (m the memory in KiB, t the passes, p the lanes; salt and hash in
// base64 without padding). So the parameters can be raised later and old
// hashes still checked with the ones they were made with.

// The parameters new hashes are made with: RFC 9106's second recommended
// option (64 MiB, 3 passes), 4 lanes, a 16-byte salt and a 32-byte hash.
const (
	hashMemory  = 64 * 1024
	hashPasses  = 3
	hashLanes   = 4
	saltLength  = 16
	hashLength  = 32
	hashVariant = "argon2id"
)

// ErrBadHash is CheckPassword's error for a stored hash it can't read.
var ErrBadHash = errors.New("the stored password hash is not an Argon2id PHC string")

// HashPassword returns password's hash, as a Record's Password field
// keeps it. VMS's passwords aren't case-sensitive (AUTHORIZE and LOGINOUT
// uppercase them), so neither are govax's: the hash is of the uppercased
// password, with blanks around it removed.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	key := argon2.IDKey([]byte(normalPassword(password)), salt, hashPasses, hashMemory, hashLanes, hashLength)

	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s", hashVariant, argon2.Version,
		hashMemory, hashPasses, hashLanes,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// CheckPassword reports whether password is the one hash was made from.
// An empty hash is an account with no password, which no password
// matches: logging in with --user asks for none, and a later remote login
// will refuse such an account.
func CheckPassword(hash, password string) (bool, error) {
	if hash == "" {
		return false, nil
	}

	fields := strings.Split(hash, "$")
	if len(fields) != 6 || fields[0] != "" || fields[1] != hashVariant {
		return false, ErrBadHash
	}

	var version int
	if _, err := fmt.Sscanf(fields[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrBadHash
	}

	var (
		memory, passes uint32
		lanes          uint8
	)

	if _, err := fmt.Sscanf(fields[3], "m=%d,t=%d,p=%d", &memory, &passes, &lanes); err != nil || passes == 0 || lanes == 0 {
		return false, ErrBadHash
	}

	salt, err1 := base64.RawStdEncoding.DecodeString(fields[4])
	want, err2 := base64.RawStdEncoding.DecodeString(fields[5])

	if err1 != nil || err2 != nil || len(want) == 0 {
		return false, ErrBadHash
	}

	got := argon2.IDKey([]byte(normalPassword(password)), salt, passes, memory, lanes, uint32(len(want)))

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// normalPassword is the password as it's hashed: uppercased, without
// blanks around it.
func normalPassword(password string) string {
	return strings.ToUpper(strings.TrimSpace(password))
}
