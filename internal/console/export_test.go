package console

import "testing"

// Helpers of this package's own tests that its external tests (package
// console_test) use too.

// ProbeSections, ProbeCompared, and JoinRecords are probeSections,
// probe50TCompared, and joinRecords, for TestProbe50TOracle.
var (
	ProbeSections = probeSections
	ProbeCompared = probe50TCompared
	JoinRecords   = joinRecords
)

// FrameOwners is frameOwners, for the paging tests with images
// (pagingframes_test.go): it returns how many process and global frames
// are tied.
func FrameOwners(t *testing.T, c *Console) (process, global int) {
	t.Helper()

	counts := frameOwners(t, c)

	return counts.process, counts.global
}
