package sysdef

import (
	"time"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/vmsdef"
)

// Library lays every module out as a text library, as LIBRARY/CREATE/TEXT would,
// made at the time given, so that a user can read them with LIBRARY/LIST and
// LIBRARY/EXTRACT or put a changed copy where INCLUDE looks first.
func Library(when time.Time) ([]byte, error) {
	b, err := lbr.Create(lbr.TypeText)
	if err != nil {
		return nil, err
	}

	stamp := vmsdef.Time(when)
	b.Created, b.Updated = stamp, stamp

	for _, name := range Names() {
		lines, _ := Module(name)

		records := make([][]byte, len(lines))
		for i, line := range lines {
			records[i] = []byte(line)
		}

		entry := lbr.TextModule(name, records)
		entry.Inserted = stamp

		if err := b.Insert(entry); err != nil {
			return nil, err
		}
	}

	return b.Bytes(), nil
}
