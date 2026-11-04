package isam

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// analysisValues reads an FDL report's ANALYSIS_OF_KEY sections.
func analysisValues(text string) map[int]map[string]int {
	out := map[int]map[string]int{}
	var cur map[string]int
	line := regexp.MustCompile(`^\t([A-Z0-9_]+)\s+(-?\d+)$`)
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "ANALYSIS_OF_KEY ") {
			n, _ := strconv.Atoi(strings.TrimPrefix(l, "ANALYSIS_OF_KEY "))
			cur = map[string]int{}
			out[n] = cur
			continue
		}
		if m := line.FindStringSubmatch(l); m != nil && cur != nil {
			v, _ := strconv.Atoi(m[2])
			cur[m[1]] = v
		}
	}
	return out
}

// TestStatisticsAsVMS holds a walk's figures to the ANALYSIS_OF_KEY sections
// of VMS's FDL report on each of round 1's files.
func TestStatisticsAsVMS(t *testing.T) {
	compared := 0
	for _, v := range probeFiles(t) {
		if strings.Contains(v.name, ";") {
			continue
		}
		text, err := os.ReadFile(filepath.Join(vaxDir, "anl", strings.ToLower(v.name)+".fdl"))
		if err != nil {
			continue
		}
		want := analysisValues(string(text))
		in := NewInspector(v.blocks, v.attr, uint32(len(v.blocks)/BlockSize), func(f Finding) {
			t.Errorf("%s: %s", v.name, f.Text)
		})
		if in.Prolog.Version != 3 {
			continue
		}
		in.CheckSums()
		for _, k := range in.Keys {
			s := in.Walk(k)
			w := want[k.Ref]
			got := map[string]int{
				"DATA_FILL": s.DataFill(), "DATA_KEY_COMPRESSION": s.KeyCompression(),
				"DATA_RECORD_COUNT": s.Records, "DATA_SPACE_OCCUPIED": s.DataBlocks, "DEPTH": s.Depth,
				"INDEX_COMPRESSION": s.IndexCompression(), "INDEX_FILL": s.IndexFill(), "INDEX_SPACE_OCCUPIED": s.IndexBlocks,
				"LEVEL1_RECORD_COUNT": s.Level1Records, "MEAN_DATA_LENGTH": s.MeanDataLength(), "MEAN_INDEX_LENGTH": s.MeanIndexLength(),
			}
			if k.Ref == 0 {
				got["DATA_RECORD_COMPRESSION"] = s.RecordCompression()
			} else {
				got["DUPLICATES_PER_SIDR"] = s.DuplicatesPerSIDR()
			}
			if !s.Initialized {
				got = map[string]int{}
			}
			compared++
			for name, value := range w {
				if got[name] != value {
					t.Errorf("%s key %d %s: VMS %d, govax %d %s", v.name, k.Ref, name, value, got[name], fmt.Sprintf("%+v", *s))
				}
			}
		}
	}
	t.Logf("%d keys compared", compared)
}
