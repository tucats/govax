package sumslp

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/diff"
)

// The round trip (Decision 5): DIFFERENCES/SLP's script for a master and a
// revision, applied to the master, gives the revision back. The scripts are
// VMS 7.1's own (testdata/dif116's /SLP cases and testdata/sum117's
// ROUNDTRIP section) and govax's internal/diff's, for the same pairs and for
// random ones.

// slpPair is a master, a revision, and the script VMS's DIFFERENCES/SLP
// wrote for them.
type slpPair struct {
	ID               string
	Master, Revision [][]byte
	Script           [][]byte
}

// dif116Pairs reads testdata/dif116's cases that are a plain
// "DIFFERENCES/SLP master revision", with VMS's script for each.
func dif116Pairs(t *testing.T) []slpPair {
	t.Helper()

	dir := filepath.Join("..", "..", "testdata", "dif116")

	data, err := os.ReadFile(filepath.Join(dir, "cases.txt"))
	if err != nil {
		t.Fatal(err)
	}

	files := map[string][][]byte{}

	var (
		pairs []slpPair
		name  string
		id    string
	)

	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "file "):
			// A name again is its next version: DIFFERENCES reads the last.
			name, id = strings.Fields(line)[1], ""
			files[name] = [][]byte{}
		case strings.HasPrefix(line, "|") && name != "":
			files[name] = append(files[name], unescape(line[1:]))
		case strings.HasPrefix(line, "case "):
			f := strings.Fields(line)
			name, id = "", ""

			if f[len(f)-1] == "file" {
				id = f[1]
			}
		case id != "" && strings.HasPrefix(line, "  command DIFFERENCES/SLP "):
			f := strings.Fields(strings.TrimPrefix(line, "  command DIFFERENCES/SLP "))
			if len(f) != 2 || strings.ContainsAny(line[len("  command DIFFERENCES/SLP"):], "/[") {
				continue
			}

			pairs = append(pairs, slpPair{ID: id, Master: files[f[0]], Revision: files[f[1]]})
		}
	}

	for i := range pairs {
		pairs[i].Script = scriptFile(t, filepath.Join(dir, "vax", "out", pairs[i].ID+".DIF"))
	}

	return pairs
}

// sum117Pairs reads testdata/sum117's ROUNDTRIP pairs (ID.A and ID.B), with
// VMS's script for each.
func sum117Pairs(t *testing.T) []slpPair {
	t.Helper()

	files := probeFiles(t)

	var pairs []slpPair

	for name, master := range files {
		id, ok := strings.CutSuffix(name, ".A")
		if !ok || (id[0] != 'R' && id[0] != 'T') {
			continue
		}

		pairs = append(pairs, slpPair{ID: id, Master: master, Revision: files[id+".B"],
			Script: scriptFile(t, filepath.Join(probeDir, "vax", "out", id+".DIF"))})
	}

	return pairs
}

// scriptFile reads a script VMS wrote, a record a line.
func scriptFile(t *testing.T, path string) [][]byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
}

// roundTrip applies script to master and says what went wrong, if anything:
// the output differs from revision, or the listing has a message.
func roundTrip(master, revision, script [][]byte) string {
	r := Apply(master, []Update{{Name: "SCRIPT", Records: script}})

	if m := r.Messages(); len(m) > 0 {
		return "messages:\n" + string(bytes.Join(m, []byte("\n")))
	}

	if !equalRecords(r.Output, revision) {
		return fmt.Sprintf("the output\n%s\nisn't the revision\n%s",
			bytes.Join(r.Output, []byte("\n")), bytes.Join(revision, []byte("\n")))
	}

	return ""
}

// govaxScript is internal/diff's /SLP script for a pair, as VMS's
// DIFFERENCES/SLP compares (testdata/sum117's ROUNDTRIP).
func govaxScript(master, revision [][]byte) [][]byte {
	res := diff.Compare(master, revision, diff.Options{Match: 3, BothSides: true})

	var script [][]byte
	for _, l := range diff.SLP(diff.Input{Records: master}, diff.Input{Records: revision}, res) {
		script = append(script, []byte(l))
	}

	return script
}

// TestVMSScriptsRoundTrip applies every script VMS 7.1's DIFFERENCES/SLP
// wrote in the two probes to its master: each gives its revision, with no
// message.
func TestVMSScriptsRoundTrip(t *testing.T) {
	t.Parallel()

	pairs := append(dif116Pairs(t), sum117Pairs(t)...)
	if len(pairs) < 136+60 {
		t.Fatalf("only %d pairs", len(pairs))
	}

	for _, p := range pairs {
		// A record holding a line feed (R14's) can't be told apart in the
		// host copy of VMS's script, a record a line: govax's script for the
		// pair is TestGovaxScriptsRoundTrip's.
		if hasLineFeed(p.Master) || hasLineFeed(p.Revision) {
			continue
		}

		if why := roundTrip(p.Master, p.Revision, p.Script); why != "" {
			t.Errorf("%s: %s", p.ID, why)
		}
	}
}

// TestGovaxScriptsRoundTrip does the same with internal/diff's scripts, for
// the probes' pairs and for seeded random ones: small files of a few
// distinct records, so that the sections meet, start and end the files, and
// hold records that start with SLP's operators.
func TestGovaxScriptsRoundTrip(t *testing.T) {
	t.Parallel()

	for _, p := range append(dif116Pairs(t), sum117Pairs(t)...) {
		if why := roundTrip(p.Master, p.Revision, govaxScript(p.Master, p.Revision)); why != "" {
			t.Errorf("%s: %s", p.ID, why)
		}
	}

	rng := rand.New(rand.NewPCG(117, 4))
	words := []string{"alpha", "beta", "gamma", "-minus", "/slash", "<less", `\back`, "%pct", "", " /"}

	file := func() [][]byte {
		n := rng.IntN(30)
		f := make([][]byte, n)

		for i := range f {
			f[i] = []byte(words[rng.IntN(len(words))])
		}

		return f
	}

	for i := range 3000 {
		master := file()

		// The revision is the master edited: some records dropped, changed,
		// or added, so the two share runs.
		var revision [][]byte

		for _, r := range master {
			switch rng.IntN(6) {
			case 0:
			case 1:
				revision = append(revision, []byte(words[rng.IntN(len(words))]))
			case 2:
				revision = append(revision, r, []byte(words[rng.IntN(len(words))]))
			default:
				revision = append(revision, r)
			}
		}

		if why := roundTrip(master, revision, govaxScript(master, revision)); why != "" {
			t.Fatalf("pair %d: %s", i, why)
		}
	}
}

// hasLineFeed says a record holds a line feed.
func hasLineFeed(records [][]byte) bool {
	for _, r := range records {
		if bytes.IndexByte(r, '\n') >= 0 {
			return true
		}
	}

	return false
}
