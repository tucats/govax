package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// runWithStats runs govax with --stats on a script and returns its
// output.
func runWithStats(t *testing.T, script string) string {
	t.Helper()

	old := stats
	stats = true

	t.Cleanup(func() { stats = old })

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, io.NopCloser(strings.NewReader(script)), nil); err != nil {
		t.Fatalf("run: %v\n%s", err, buf.String())
	}

	return buf.String()
}

// TestRun_statsPaging: with paging on, --stats ends with paging's section
// (docs/PHASE-64 - page file.md, subtask 10), what SHOW MEMORY shows of
// it: frames, faults, the writer and reclaimer, and the page file, here
// a host file installed at startup.
func TestRun_statsPaging(t *testing.T) {
	pageFile := filepath.Join(t.TempDir(), "pagefile.sys")

	setConfig(t, "vax.sysgen.paging.enable", "true")
	setConfig(t, "vax.sysgen.paging.file", `"`+pageFile+`"`)
	setConfig(t, "vax.sysgen.paging.size", "2048")

	out := runWithStats(t, "SHOW TIME\n")

	stats := out[strings.Index(out, "Emulation Statistics:"):]

	for _, want := range []string{
		"\n  Paging:\n",
		"    Free List Frames:",
		"    Active Frames:",
		"    Page Faults:",
		"      Demand Zero:",
		"    Pages Written:",
		"    Reclaimer Runs:",
		"\n  Page File:\n    " + pageFile + "\n",
		"    Size (blocks):                  2,048\n",
		"    Committed:",
	} {
		if !strings.Contains(stats, want) {
			t.Errorf("statistics lack %q:\n%s", want, stats)
		}
	}
}

// TestRun_statsNoPaging: with paging off, --stats has no paging section.
func TestRun_statsNoPaging(t *testing.T) {
	setConfig(t, "vax.sysgen.paging.enable", "false")

	out := runWithStats(t, "SHOW TIME\n")

	if !strings.Contains(out, "Emulation Statistics:") {
		t.Fatalf("no statistics:\n%s", out)
	}

	if strings.Contains(out, "Paging:") || strings.Contains(out, "Page File:") {
		t.Errorf("paging's section with paging off:\n%s", out)
	}
}
