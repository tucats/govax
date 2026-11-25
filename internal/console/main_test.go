package console

import (
	"os"
	"testing"

	"github.com/tucats/govax/internal/testproc"
)

// TestMain runs the package's tests through testproc.Main, which gives the
// tests that run in processes of their own (testproc.Isolate) a place to
// share what's costly to make (testproc.Shared).
func TestMain(m *testing.M) {
	os.Exit(testproc.Main(m))
}
