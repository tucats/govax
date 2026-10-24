package anl

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/vmsimage"
)

// TestLegacyImagePages matches VMS 7.1's ANALYZE/IMAGE of DUNGEON.EXE, a
// VMS V1 image (format 02.01; testdata/legacy72), page for page but for
// the time. The report shows only the header, so the image is its
// header block (dungeon-hdr.bin) and zeros, the file's 210 blocks.
func TestLegacyImagePages(t *testing.T) {
	header, err := os.ReadFile("../../testdata/legacy72/dungeon-hdr.bin")
	if err != nil {
		t.Fatal(err)
	}

	text, err := os.ReadFile("../../testdata/legacy72/vax/dungeon.ani")
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 210*vmsimage.BlockSize)
	copy(data, header)

	img, err := vmsimage.ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(text), "\n")
	file := lines[2]
	command := strings.TrimRight(lines[len(lines)-2], " ")

	var b bytes.Buffer

	when := time.Date(2026, time.October, 24, 10, 40, 3, 30_000_000, time.UTC)
	p := NewPager(&b, TitleImage, file, command)
	p.Now = func() time.Time { return when }

	if err := p.Write(AnalyzeImage(img, ImageOptions{}).Lines); err != nil {
		t.Fatal(err)
	}

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	want := imageTimeRE.ReplaceAllString(string(text), "${1}"+vmsTime(when))
	if diff := firstDiff(want, b.String()); diff != "" {
		t.Error(diff)
	}
}
