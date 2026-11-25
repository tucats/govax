package console_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/testproc"
	"github.com/tucats/govax/internal/uaf"
)

// TestShowProcessRightsAndQuotas holds SHOW PROCESS/RIGHTS and /QUOTAS to
// VMS 7.1's (the probe's round 3, UAF95I.COM run by UAF95 after logging
// in, testdata/uaf95/vax/round3), after the heading's first two lines
// (the time, PID, and node differ). UAF95 holds UAF95_ID there, as the
// probe's UAF95ACCT.COM granted it. govax has no "System rights" (VMS's
// SYS$NODE_ identifier), and its quota report shows a limit where it
// doesn't charge the quota (quotaDifferences).
func TestShowProcessRightsAndQuotas(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	s := newLoginSession(t)

	err := s.c.RTL.System.UpdateAuthorization(func(f *uaf.File) error {
		f.Identifiers = append(f.Identifiers, uaf.Identifier{
			Name: "UAF95_ID", Value: f.NextGeneral(),
			Holders: []uaf.Holder{{UIC: uaf.NewUIC(0o200, 5)}},
		})

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	s.logIn("UAF95")

	// The VAX's PFCDEFAULT was 16 (testdata/pg64/vax/pg64.log's SYSGEN
	// SHOW), and its DFPFC the same.
	s.c.RTL.Sysgen.PFCDefault = 16

	// What VMS's SHOW showed that govax shows otherwise: the line, and
	// govax's. The byte count quota is the account's BYTLM (govax charges
	// none of it, and VMS's 32256 is its own BYTLM less what login took),
	// the open files FILLM (VMS's two open process-permanent files
	// charged it), the paging file quota what govax's job has left (all
	// of PGFLQUOTA with paging off; VMS's SHOW, an image, had charged its
	// own pages), and the AST quota all of ASTLM (VMS's DCL had one AST
	// outstanding, F$GETJPI's ASTCNT 39).
	quotaDifferences := map[string]string{
		" Buffered I/O byte count quota:     32256  Buffered I/O limit:      40": " Buffered I/O byte count quota:     32768  Buffered I/O limit:      40",
		" Timer queue entry quota:              40  Open file quota:        298": " Timer queue entry quota:              40  Open file quota:        300",
		" Paging file quota:                 31278  Subprocess quota:         2": " Paging file quota:                 32768  Subprocess quota:         2",
		" Default page fault cluster:           16  AST quota:               39": " Default page fault cluster:           16  AST quota:               40",
	}

	for _, tc := range []struct{ command, section string }{
		{"SHOW PROCESS/RIGHTS", "$ SHOW PROCESS/RIGHTS"},
		{"SHOW PROCESS/QUOTAS", "$ SHOW PROCESS/QUOTAS"},
	} {
		got := strings.Split(strings.TrimRight(s.dispatch(tc.command), "\n"), "\n")
		want := probeSection(t, "uaf95i_uaf95_interactive.log", tc.section)

		// VMS's rights end with the "System rights" section govax lacks.
		if i := indexOf(want, "System rights:"); i > 0 {
			want = want[:i-1]
		}

		for i, w := range want {
			if d, ok := quotaDifferences[w]; ok {
				want[i] = d
			}
		}

		if len(got) != len(want) {
			t.Errorf("%s: %d lines, want %d:\n%s", tc.command, len(got), len(want), strings.Join(got, "\n"))

			continue
		}

		for i := range want {
			if i == 1 || i == 2 {
				continue // the heading's time, PID, and node
			}

			if got[i] != want[i] {
				t.Errorf("%s line %d:\n  got  %q\n  want %q", tc.command, i, got[i], want[i])
			}
		}
	}
}
