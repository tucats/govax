package diff

import "bytes"

// Options are what the comparison takes from the command.
type Options struct {
	// Match is /MATCH: how many records must agree in a row for the files
	// to be in step again (3 by default). It is signed, as VMS compares
	// it: /MATCH=-1 (shown as 4294967295) acts as 1, a section ending at
	// the first pair of records that agree.
	Match int32

	// Window is /WINDOW: how deep a search for the place the files are in
	// step again goes before it moves a record on (resync); 0 is no limit.
	Window int

	// MaxDifferences is /MAXIMUM_DIFFERENCES: the comparison stops once
	// its sections hold this many records (Result.Stopped); 0 is no limit.
	MaxDifferences int

	// BothSides counts a section's records as both files' together, not
	// its larger side's, for /MAXIMUM_DIFFERENCES: /SLP's count. VMS's
	// L03 (/MAXIMUM_DIFFERENCES=3/SLP) stopped at a section of one record
	// and an insertion of two, its script replacing one record with one;
	// the merged L01_3 listed two of each. Fitted to that one case,
	// unconfirmed.
	BothSides bool
}

// Span is a run of records in one file: Start to End, counted from 0, End
// not included. An empty span (Start == End) is a place between records.
type Span struct {
	Start, End int
}

// Len is the number of records in the span.
func (s Span) Len() int { return s.End - s.Start }

// Section is one difference section: the master's records and the
// revision's that don't match, with any that agree between them.
type Section struct {
	Master, Revision Span
}

// Records is the section's count of difference records, as VMS counts
// them: its larger side's (testdata/dif116's A16).
func (s Section) Records() int { return max(s.Master.Len(), s.Revision.Len()) }

// Result is a comparison's outcome.
type Result struct {
	Sections []Section

	// Records is the sum of the sections' counts.
	Records int

	// Stopped says /MAXIMUM_DIFFERENCES ended the comparison.
	Stopped bool
}

// Compare compares master's records with revision's.
func Compare(master, revision [][]byte, opts Options) Result {
	c := comparison{a: master, b: revision, opts: opts}

	return c.run()
}

// comparison is one run of Compare: the two files, a and b, and the
// options.
type comparison struct {
	a, b [][]byte
	opts Options
}

func (c *comparison) run() Result {
	var res Result

	i, j := 0, 0
	n, m := len(c.a), len(c.b)
	counted := 0 // the records counted against /MAXIMUM_DIFFERENCES

	for i < n || j < m {
		if i < n && j < m && bytes.Equal(c.a[i], c.b[j]) {
			i++
			j++

			continue
		}

		p, q := c.resync(i, j)
		s := Section{Master: Span{i, p}, Revision: Span{j, q}}

		// The section that reaches /MAXIMUM_DIFFERENCES ends the comparison,
		// and is cut: it lists only as many records of each file, from its
		// start, as the limit had left, whether or not they are the section's
		// (testdata/dif116's L01_1 to L01_9: the master's "entry 10" after
		// an insertion).
		count := s.Records()
		if c.opts.BothSides {
			count = s.Master.Len() + s.Revision.Len()
		}

		if limit := c.opts.MaxDifferences; limit > 0 && counted+count >= limit {
			rest := limit - counted
			if c.opts.BothSides {
				rest = (rest + 1) / 2
			}

			s = Section{Master: Span{i, min(i+rest, n)}, Revision: Span{j, min(j+rest, m)}}
			res.Sections = append(res.Sections, s)
			res.Records += s.Records()
			res.Stopped = true

			break
		}

		res.Sections = append(res.Sections, s)
		res.Records += s.Records()
		counted += count

		i, j = p, q
	}

	return res
}

// resync finds where the files are in step again after records a[i] and
// b[j] differ, by the rule fitted to VMS 7.1's listings (doc.go's "The
// search"): a square of places around (i, j) growing a record at a time, at
// depth d first (i+d, j), then (i, j+d), (i+d, j+1), (i+1, j+d), and so on
// to (i+d, j+d); the first place where the files are in step ends the
// section. Without /WINDOW the square grows until it covers both files' ends,
// which are always in step. With /WINDOW=w it grows only to depth w; then
// the search starts again a record further on in both files, at depth 0,
// until the files end.
func (c *comparison) resync(i, j int) (int, int) {
	n, m := len(c.a), len(c.b)
	w := c.opts.Window

	for from := 1; ; from = 0 {
		depth := max(n-i, m-j)
		if w > 0 {
			depth = w
		}

		for d := from; d <= depth; d++ {
			if p, q, ok := c.square(i, j, d); ok {
				return p, q
			}
		}

		if w == 0 || (i >= n && j >= m) {
			return n, m
		}

		i, j = min(i+1, n), min(j+1, m)
	}
}

// square looks for a place in step on the edge of the square of depth d
// from (i, j), the master's side of each pair of places first.
func (c *comparison) square(i, j, d int) (int, int, bool) {
	n, m := len(c.a), len(c.b)

	for t := 0; t <= d; t++ {
		for _, at := range [2][2]int{{i + d, j + t}, {i + t, j + d}} {
			if p, q := at[0], at[1]; p <= n && q <= m && c.inStep(p, q) {
				return p, q, true
			}
		}
	}

	return 0, 0, false
}

// inStep says whether the files are in step at (p, q): both end there, or
// /MATCH records agree in a row from there (at least one, whatever /MATCH
// is), both files ending together counting as agreement.
func (c *comparison) inStep(p, q int) bool {
	n, m := len(c.a), len(c.b)

	need := max(int(c.opts.Match), 1)

	for t := 0; t < need; t++ {
		switch {
		case p+t == n && q+t == m:
			return true
		case p+t >= n || q+t >= m:
			return false
		case !bytes.Equal(c.a[p+t], c.b[q+t]):
			return false
		}
	}

	return true
}
