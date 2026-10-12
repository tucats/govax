package console

// Helpers of this package's own tests that its external tests (package
// console_test) use too.

// ProbeSections, ProbeCompared, and JoinRecords are probeSections,
// probe50TCompared, and joinRecords, for TestProbe50TOracle.
var (
	ProbeSections = probeSections
	ProbeCompared = probe50TCompared
	JoinRecords   = joinRecords
)
