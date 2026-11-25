package sumslp

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

// Merging (subtask 5). The probe's MERGE and CLASH2 sections are
// TestProbeMerged's; these are the merge's own properties, and the choices
// no case settled, pinned so a change to them is seen.

// splitScript deals a script's edits (each command with its text) among n
// update files at random, each file keeping its edits in order and ending
// with "/".
func splitScript(rng *rand.Rand, script [][]byte, n int) []Update {
	files := make([][][]byte, n)
	f := 0

	for _, line := range script {
		switch {
		case bytes.Equal(line, []byte("/")):
		case len(line) > 0 && line[0] == '-':
			f = rng.IntN(n)
			files[f] = append(files[f], line)
		default:
			files[f] = append(files[f], line)
		}
	}

	updates := make([]Update, n)
	for i := range files {
		updates[i] = Update{Name: fmt.Sprintf("PART%d", i), Records: append(files[i], []byte("/"))}
	}

	return updates
}

// TestSplitScriptsMerge deals each DIFFERENCES/SLP script's edits among two
// or three update files: as the edits don't meet, merged they give the
// revision, with no message, whatever the files' order.
func TestSplitScriptsMerge(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(117, 5))

	pairs := append(dif116Pairs(t), sum117Pairs(t)...)

	for i := range 2000 {
		master, revision := randomPair(rng)
		pairs = append(pairs, slpPair{ID: fmt.Sprintf("random %d", i), Master: master, Revision: revision})
	}

	for _, p := range pairs {
		updates := splitScript(rng, govaxScript(p.Master, p.Revision), 2+rng.IntN(2))
		r := Apply(p.Master, updates)

		if m := r.Messages(); len(m) > 0 {
			t.Fatalf("%s: messages:\n%s", p.ID, bytes.Join(m, []byte("\n")))
		}

		if !equalRecords(r.Output, p.Revision) {
			t.Fatalf("%s: the output\n%s\nisn't the revision\n%s", p.ID,
				bytes.Join(r.Output, []byte("\n")), bytes.Join(p.Revision, []byte("\n")))
		}
	}
}

// TestMergeChoices pins what govax does where the probe had no case: a range
// the input ends in, with other files' edits after it (the phase doc's A.5,
// "what stays unconfirmed"). Each case is ten lines "1" to "10".
func TestMergeChoices(t *testing.T) {
	t.Parallel()

	ten := make([][]byte, 10)
	for i := range ten {
		ten[i] = fmt.Appendf(nil, "%d", i+1)
	}

	for _, c := range []struct {
		name  string
		files []string // each update file, its lines joined by "|"
		want  string   // the output, joined by " "
	}{
		{
			// A's range runs out; the input is read again, numbered on, and
			// B's insert after 12 goes after the second reading's line 2.
			"an insert after the range ran out",
			[]string{"-9,11|A", "-12|B"},
			"1 2 3 4 5 6 7 8 A 1 2 B 3 4 5 6 7 8 9 10",
		},
		{
			// B's range starts inside A's open range: dropped, as in one
			// file (O02).
			"a second range inside one that ran out",
			[]string{"-9,11|A", "-10,12|B"},
			"1 2 3 4 5 6 7 8 A 1 2 3 4 5 6 7 8 9 10",
		},
		{
			// B's range starts after A's ran out, on the second reading.
			"a range on the second reading",
			[]string{"-9,11|A", "-13,14|B"},
			"1 2 3 4 5 6 7 8 A 1 2 B 5 6 7 8 9 10",
		},
		{
			// B's range runs out on the second reading: it closes at the
			// second end, and the run ends.
			"a range at the second end",
			[]string{"-9,11|A", "-19,25|B"},
			"1 2 3 4 5 6 7 8 A 1 2 3 4 5 6 7 8 B",
		},
	} {
		var updates []Update

		for i, f := range c.files {
			var records [][]byte
			for l := range strings.SplitSeq(f+"|/", "|") {
				records = append(records, []byte(l))
			}

			updates = append(updates, Update{Name: fmt.Sprintf("F%d", i), Records: records})
		}

		r := Apply(ten, updates)

		if got := string(bytes.Join(r.Output, []byte(" "))); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}
