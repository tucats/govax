package diff

import "bytes"

// Options are what the comparison takes from the command.
type Options struct {
	// Match is /MATCH: how many records must agree in a row for the files
	// to be in step again (3 by default). It is signed, as VMS compares
	// it: /MATCH=-1 (shown as 4294967295) acts as 1, a section ending at
	// the first pair of records that agree.
	Match int32

	// Window is /WINDOW: how far past a section's start, in either file,
	// to look for the place the files are in step again; 0 is no limit.
	Window int

	// MaxDifferences is /MAXIMUM_DIFFERENCES: the comparison stops once
	// its sections hold this many records; 0 is no limit.
	MaxDifferences int
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

	for i < n || j < m {
		if i < n && j < m && bytes.Equal(c.a[i], c.b[j]) {
			i++
			j++

			continue
		}

		p, q := c.resync(i, j)
		s := Section{Master: Span{i, p}, Revision: Span{j, q}}
		res.Sections = append(res.Sections, s)
		res.Records += s.Records()

		if c.opts.MaxDifferences > 0 && res.Records >= c.opts.MaxDifferences {
			res.Stopped = true

			break
		}

		i, j = p, q
	}

	return res
}

// resync finds where the files are in step again after records a[i] and b[j]
// differ: the place (p, q), past i and j, that leaves the fewest records
// before it, both files' together, and among those the one furthest into
// the master. With none, the section runs to both files' ends.
func (c *comparison) resync(i, j int) (int, int) {
	n, m := len(c.a), len(c.b)

	for k := 1; k <= (n-i)+(m-j); k++ {
		for dp := k; dp >= 0; dp-- {
			dq := k - dp
			p, q := i+dp, j+dq

			if p > n || q > m {
				continue
			}

			if c.opts.Window > 0 && (dp > c.opts.Window || dq > c.opts.Window) {
				continue
			}

			if c.inStep(p, q) {
				return p, q
			}
		}
	}

	return n, m
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
