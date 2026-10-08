package lck

// Mode is a lock mode, numbered as VMS numbers them (LCK$K_NLMODE is 0
// through LCK$K_EXMODE, 5).
type Mode uint8

// The six lock modes.
const (
	NL Mode = iota // null: no access; holds the resource and its value block
	CR             // concurrent read: read, sharing with readers and writers
	CW             // concurrent write: write, sharing with readers and writers
	PR             // protected read: read, sharing with readers only
	PW             // protected write: write, sharing with CR readers only
	EX             // exclusive: no sharing
)

// compatible is the System Services manual's lock compatibility table:
// compatible[a][b] is whether a lock at mode a may be granted while
// another is held at mode b. The table is symmetric.
var compatible = [6][6]bool{
	//   NL    CR     CW     PR     PW     EX
	NL: {true, true, true, true, true, true},
	CR: {true, true, true, true, true, false},
	CW: {true, true, true, false, false, false},
	PR: {true, true, false, true, false, false},
	PW: {true, true, false, false, false, false},
	EX: {true, false, false, false, false, false},
}

// Valid reports whether m is one of the six modes.
func (m Mode) Valid() bool {
	return m <= EX
}

// Compatible reports whether a lock at mode m can be held while another
// lock on the same resource is held at mode other.
func (m Mode) Compatible(other Mode) bool {
	return compatible[m][other]
}

// noMoreRestrictive reports whether converting a lock from mode from to
// mode m can't conflict with anything from didn't: every mode compatible
// with from is compatible with m. Such a conversion (a "down" conversion,
// or one to the same mode) is always granted at once.
func (m Mode) noMoreRestrictive(from Mode) bool {
	for other := NL; other <= EX; other++ {
		if from.Compatible(other) && !m.Compatible(other) {
			return false
		}
	}

	return true
}

// writes reports whether m is a mode whose holder may write the value
// block back: PW or EX.
func (m Mode) writes() bool {
	return m == PW || m == EX
}

// String is the mode's two-letter name.
func (m Mode) String() string {
	if !m.Valid() {
		return "??"
	}

	return [...]string{"NL", "CR", "CW", "PR", "PW", "EX"}[m]
}
