package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// printStatus dumps out stats if they are enabled to the console when the emulation finishes.
func printStats(c *console.Console, out io.Writer, flag bool) {
	if flag {
		instructionCount := c.Engine.InstructionCount()
		interruptCount := c.Engine.InterruptCount()
		translate, readCount, writeCount, mbReadCount, mbWriteCount := c.Engine.Memory().Stats()
		elapsed := time.Since(startTime)

		fmt.Fprintf(out, "Emulation Statistics:\n")
		fmt.Fprintf(out, "  Initialization Time:   %16s\n\n", formatDuration(c.InitTime))
		fmt.Fprintf(out, "  CPU:\n")
		fmt.Fprintf(out, "    Elapsed Time:        %16s\n", formatDuration(elapsed))
		fmt.Fprintf(out, "    Instructions:        %16s\n", formatLargeNumber(int64(instructionCount)))

		// Calcuate instructions per second, but avoid divide by zero if the elapsed time is less than a millisecond.
		// We don't show this value if the only executed instructions are the microkernel initialization, which is
		// usually < 20 instructions.
		if instructionCount > 20 {
			if elapsed < time.Millisecond {
				fmt.Fprintf(out, "    Instructions/Sec:    %16s\n", "N/A")
			} else {
				instructionsPerSecond := float64(instructionCount) / elapsed.Seconds()
				fmt.Fprintf(out, "    Instructions/Sec:    %16s\n", formatLargeNumber(int64(instructionsPerSecond)))
			}
		} else {
			fmt.Fprintf(out, "    Instructions/Sec:    %16s\n", "N/A")
		}

		fmt.Fprintf(out, "    Interrupts:          %16s\n", formatLargeNumber(int64(interruptCount)))

		fmt.Fprintf(out, "\n  Memory:\n")
		fmt.Fprintf(out, "    Page Translations:   %16s\n", formatLargeNumber(translate))
		fmt.Fprintf(out, "    Single-Byte Reads:   %16s\n", formatLargeNumber(readCount))
		fmt.Fprintf(out, "    Single-Byte Writes:  %16s\n", formatLargeNumber(writeCount))
		fmt.Fprintf(out, "    Multi-byte Reads:    %16s\n", formatLargeNumber(mbReadCount))
		fmt.Fprintf(out, "    Multi-byte Writes:   %16s\n", formatLargeNumber(mbWriteCount))

		stcTries, stcHits := c.Engine.Memory().STCStats()
		tbTries, tbHits, tbFlushes, _ := c.Engine.Memory().TBStats()

		fmt.Fprintf(out, "\n  Translation Buffer:\n")
		fmt.Fprintf(out, "    Sequential Cache Tries: %13s\n", formatLargeNumber(stcTries))
		fmt.Fprintf(out, "    Sequential Cache Hits:  %13s\n", formatLargeNumber(stcHits))
		fmt.Fprintf(out, "    TB Cache Tries:         %13s\n", formatLargeNumber(tbTries))
		fmt.Fprintf(out, "    TB Cache Hits:          %13s\n", formatLargeNumber(tbHits))
		fmt.Fprintf(out, "    TB Flushes:             %13s\n", formatLargeNumber(tbFlushes))

		// The instruction-fetch window (internal/vm/fetch.go): hits are
		// instruction-stream reads served straight from RAM, without a
		// translation (so they appear in none of the counts above); fills
		// are the times a fetch had to translate a new page into it.
		fetchHits, fetchFills := c.Engine.Memory().FetchStats()

		fmt.Fprintf(out, "    Fetch Window Hits:      %13s\n", formatLargeNumber(fetchHits))
		fmt.Fprintf(out, "    Fetch Window Fills:     %13s\n", formatLargeNumber(fetchFills))

		if s, ok := c.PagingStatistics(); ok {
			printPagingStats(out, s)
		}

		printGoStats(out)
	}
}

// printGoStats prints out the Go runtime statistics, to show the memory
// usage on the host system.
func printGoStats(out io.Writer) {
	var m runtime.MemStats

	line := func(label string, n uint64) {
		fmt.Fprintf(out, "    %-24s%13s\n", label+":", formatLargeNumber(int64(n)))
	}

	// Convert n (which is number of nanoseconds in Unix epoch) to a time.Time and format as a string. If n is 0, return "0" instead of the epoch time.
	lineTime := func(label string, n uint64) {
		// N is nanoseconds, need to convert to seconds and fractional nanoseconds for time.Unix.
		seconds := int64(n / 1e9)
		nanoseconds := int64(n % 1e9)

		// Format using time that includes milliseconds.
		fmt.Fprintf(out, "    %-24s%13s\n", label+":", time.Unix(seconds, nanoseconds).Format("15:04:05.000"))
	}

	//lineDuration formats a duration in nanoseconds to a human-readable string.
	lineDuration := func(label string, n uint64) {
		fmt.Fprintf(out, "    %-24s%13s\n", label+":", formatDuration(time.Duration(n)))
	}

	// lineBytes formats a byte count to a human-readable string with units.
	lineBytes := func(label string, n uint64) {
		fmt.Fprintf(out, "    %-24s%13s\n", label+":", formatByteCount(int64(n)))
	}

	runtime.ReadMemStats(&m)
	fmt.Fprintf(out, "\n  Host (Go) Runtime:\n")

	lineBytes("Currently Allocated", m.Alloc)
	lineBytes("Total Allocated", m.TotalAlloc)
	lineBytes("Total Memory Usage", m.Sys)
	line("Lookups", m.Lookups)
	line("Mallocs", m.Mallocs)
	line("Frees", m.Frees)
	lineBytes("Heap Allocated", m.HeapAlloc)
	lineBytes("Heap Size", m.HeapSys)
	lineBytes("Heap Idle", m.HeapIdle)
	lineBytes("Heap In Use", m.HeapInuse)
	lineBytes("Heap Released", m.HeapReleased)
	line("Heap Objects", m.HeapObjects)
	lineBytes("Stack In Use", m.StackInuse)
	lineBytes("Stack Size", m.StackSys)
	lineBytes("MSpan In Use", m.MSpanInuse)
	lineBytes("MSpan Size", m.MSpanSys)
	lineBytes("MCache In Use", m.MCacheInuse)
	lineBytes("MCache Size", m.MCacheSys)
	lineBytes("Bucket Hash Size", m.BuckHashSys)
	lineBytes("GC Metadata Size", m.GCSys)
	lineBytes("Other Sys Size", m.OtherSys)
	lineBytes("Next GC Target", m.NextGC)
	lineTime("Last GC Time", m.LastGC)
	lineDuration("Pause Total NS", m.PauseTotalNs)
	line("Num GC", uint64(m.NumGC))

}

// printPagingStats adds paging's section to the statistics, with paging
// on (VMINIT/PAGING): what SHOW MEMORY shows of it, the page frames on
// each list and in use, the page faults by how each was resolved, the
// pages the modified page writer wrote, the reclaimer's runs, and the
// page file's use.
func printPagingStats(out io.Writer, s machine.PagingStatistics) {
	line := func(label string, n int64) {
		fmt.Fprintf(out, "    %-24s%13s\n", label+":", formatLargeNumber(n))
	}

	f, p, r := s.Frames, s.Pages, s.PageFile

	fmt.Fprintf(out, "\n  Paging:\n")
	line("Free List Frames", int64(f.Free))
	line("Modified List Frames", int64(f.Modified))
	line("Active Frames", int64(f.Active()))
	line("  Process Pages", int64(f.Process))
	line("  Global Pages", int64(f.Global))
	line("  System Pages", int64(f.System))
	line("Page Faults", int64(p.Faults.Total()))
	line("  Free List", int64(p.Faults.FreeList))
	line("  Modified List", int64(p.Faults.ModifiedList))
	line("  Global Valid", int64(p.Faults.GlobalValid))
	line("  Demand Zero", int64(p.Faults.DemandZero))
	line("  Read", int64(p.Faults.Read))
	line("    Pages Read", int64(p.PagesRead))
	line("      From Page File", int64(p.PageFileReads))
	line("Pages Written", int64(p.Written))
	line("  Write I/Os", int64(p.WriteIOs))
	line("Reclaimer Runs", int64(p.Reclaims))
	line("  Pages Trimmed", int64(p.Trimmed))

	fmt.Fprintf(out, "\n  Page File:\n")

	switch r.Status {
	case corevms.PageFileNone:
		fmt.Fprintf(out, "    None\n")

		if r.Unused != "" {
			fmt.Fprintf(out, "    %s\n", r.Unused)
		}

		return
	case corevms.PageFileNotOpened:
		fmt.Fprintf(out, "    %s (not opened)\n", r.Name)
	case corevms.PageFileUnavailable:
		fmt.Fprintf(out, "    %s (not available)\n", r.Name)
	case corevms.PageFileInUse:
		fmt.Fprintf(out, "    %s\n", r.Name)
		line("Size (blocks)", int64(r.Size))
		line("Free", int64(r.Free))
		line("In Use", int64(r.Used))

		switch r.Warned {
		case paging.PageFrag:
			fmt.Fprintf(out, "    Passed PAGEFRAG (65%%)\n")
		case paging.PageCrit:
			fmt.Fprintf(out, "    Passed PAGECRIT (90%%)\n")
		}

		if r.Full {
			fmt.Fprintf(out, "    The writer found it full\n")
		}
	}

	line("Committed", int64(r.Charged))
}

func formatDuration(d time.Duration) string {
	text := d.String()

	result := strings.Builder{}
	decimal := false
	digits := 0

	for _, ch := range text {
		if ch == '.' {
			decimal = true
		}

		// If it's not a decimal, just copy it and continue
		if !unicode.IsDigit(ch) {
			result.WriteRune(ch)

			continue
		}

		// It is a digit. See if we've already seen the decimal and
		// enough places to ignore this value.
		if decimal && digits > 2 {
			continue
		}

		if decimal {
			digits++
		}

		result.WriteRune(ch)
	}

	return result.String()
}

// formatByteCount formats a byte count into a human-readable string, using KB, MB, GB, etc. as appropriate.
func formatByteCount(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// formatLargeNumber formats an int64 using commas for ease of readability.
// This uses the experimental text package.
func formatLargeNumber(v int64) string {
	p := message.NewPrinter(language.English)
	result := p.Sprintf("%d", v)

	return result
}

// historyFilePath returns a per-user location for readline's command
// history, or "" (disabling history persistence) if the home directory
// can't be determined.
func historyFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".govax", "history.txt")
}
