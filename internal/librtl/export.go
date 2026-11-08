package librtl

import "github.com/tucats/govax/internal/corevms"

// What another RTL package's routines (internal/lbrrtl) use of this one's
// string handling: the same descriptor rules LIBRTL's routines follow.

// ReadString reads the string the descriptor at addr describes, of any
// class the RTL accepts (readString).
func ReadString(env *corevms.Environment, addr uint32) (string, error) {
	return readString(env, addr)
}

// WriteString returns s in the string the descriptor at addr describes, by
// its class (writeString): a fixed string padded with blanks, a dynamic
// one given s's length.
func WriteString(env *corevms.Environment, addr uint32, s string) (stored int, truncated bool, err error) {
	return writeString(env, addr, s)
}

// Buffer returns the storage the descriptor at addr describes, for a
// routine that fills a caller's buffer itself: its address and how many
// bytes it holds (a varying string's maximum length).
func Buffer(env *corevms.Environment, addr uint32) (text, size uint32, err error) {
	d, err := readDescriptor(env, addr)
	if err != nil {
		return 0, 0, err
	}

	return d.text, d.limit, nil
}

// StringFailure returns the status, or signals the condition, for an
// error from ReadString, WriteString, or Buffer (stringFailure):
// LIB$_INVSTRDES for a descriptor of no string class, an access violation
// for memory that can't be reached.
func StringFailure(env *corevms.Environment, err error) (uint32, error) {
	return stringFailure(env, err)
}
