package lnm

import (
	"strconv"
	"time"
)

// TimeZoneName is the logical name VMS V6 and later define in the system
// table for the local time's difference from UTC: a signed decimal count of
// seconds east of Greenwich, "-18000" for US Eastern standard time (VMS 7.1
// on the simh VAX; docs/planning/PHASE-108 - Fortran 90 intrinsic
// procedures.md, finding 9). FORTRAN's DATE_AND_TIME reads it for its ZONE
// and VALUES(4).
const TimeZoneName = "SYS$TIMEZONE_DIFFERENTIAL"

// TimeZoneDifferential is TimeZoneName's equivalence string for a time zone
// offset in seconds east of UTC. A negative one has its "-", as VMS's; a
// positive one has no "+" (unconfirmed: the VAX showed a negative one).
func TimeZoneDifferential(seconds int) string {
	return strconv.Itoa(seconds)
}

// defineTimeZone defines TimeZoneName in the system table, executive mode,
// from the host's offset now: govax's local time is the host's, so its
// difference from UTC is the host's too. A change of the host's offset during
// a session (summer time) isn't followed, as VMS's name isn't until a
// procedure redefines it.
func (db *Database) defineTimeZone() {
	_, offset := time.Now().Zone()

	for _, t := range db.shared.tables {
		if t.Name == SystemTableName {
			t.add(&Entry{Name: TimeZoneName, Mode: Executive,
				Equivalences: []Equivalence{{Value: TimeZoneDifferential(offset)}}})
		}
	}
}
