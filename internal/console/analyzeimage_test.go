package console

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// The fixtures ANALYZE/IMAGE's tests use: real LINK's ADDR.EXE and
// EXTERN.EXE, the objects EXTERN.EXE was linked from, and real ANALYZE's
// analyses of the images.
const (
	imageFixtureDir = "../../testdata/link/vax"
	imageFixtureExe = imageFixtureDir + "/addr.exe"
	imageFixtureAni = imageFixtureDir + "/addr.ani"
)

// imageTimeRE matches an image analysis's page header time and file
// line, imageLinkTimeRE its link time and linker, and imageCmdRE its
// command.
var (
	imageTimeRE     = regexp.MustCompile(`(?m)^(Analyze Image +)[ 0-9]{2}-[A-Z]{3}-[0-9]{4} [0-9:.]{11}(   Page [0-9]+\n)[^\n]*\n`)
	imageLinkTimeRE = regexp.MustCompile(`(?m)^(\t\t(link date/time|linker identification): )[^\n]*$`)
	imageCmdRE      = regexp.MustCompile(`(?m)^ANALYZE/IMAGE[^\n]*\n\z`)
)

// normalizeImageAnalysis masks what differs between two runs of
// ANALYZE/IMAGE on the same image: the times, the file's name, and the
// command. With linked (an image govax linked, compared with one real
// LINK linked), the link time and the linker's identification ("govax
// Vn" against "V11-39") are masked too.
func normalizeImageAnalysis(text string, linked bool) string {
	text = imageTimeRE.ReplaceAllString(text, "${1}TIME${2}FILE\n")

	if linked {
		text = imageLinkTimeRE.ReplaceAllString(text, "${1}MASKED")
	}

	return imageCmdRE.ReplaceAllString(text, "COMMAND\n")
}

// copyFixture copies a fixture into dir, so an output file written beside
// it can't overwrite the fixtures.
func copyFixture(t *testing.T, from, dir string) string {
	t.Helper()

	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}

	to := filepath.Join(dir, filepath.Base(from))
	if err := os.WriteFile(to, data, 0o644); err != nil {
		t.Fatal(err)
	}

	return to
}

func TestAnalyzeImage_hostFile(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	dir := t.TempDir()
	exe := copyFixture(t, imageFixtureExe, dir)

	// /OUTPUT with no file: ADDR.ANI beside the image.
	if err := d.Dispatch(`ANALYZE/IMAGE/OUTPUT "` + exe + `"`); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("ANALYZE/OUTPUT wrote to the console: %q", buf.String())
	}

	got, err := os.ReadFile(filepath.Join(dir, "addr.ani"))
	if err != nil {
		got, err = os.ReadFile(filepath.Join(dir, "ADDR.ANI"))
	}

	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile(imageFixtureAni)
	if err != nil {
		t.Fatal(err)
	}

	if normalizeImageAnalysis(string(got), false) != normalizeImageAnalysis(string(want), false) {
		t.Errorf("the analysis differs from VMS's:\n%s", got)
	}

	abs, _ := filepath.Abs(exe)
	if !strings.Contains(string(got), "\n"+abs+"\nANALYZ V07-04\n") {
		t.Errorf("the page header doesn't name %s", abs)
	}
}

func TestAnalyzeImage_header(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	for _, tt := range []struct {
		line   string
		fixups bool
	}{
		{`ANALYZE/IMAGE "` + imageFixtureExe + `"`, true},
		{`ANALYZE/IMAGE/HEADER "` + imageFixtureExe + `"`, false},
		{`ANALYZE/HEADER/IMAGE "` + imageFixtureExe + `"`, false},
		{`ANALYZE/IMAGE/HEADER/FIXUP_SECTION "` + imageFixtureExe + `"`, true},
		{`ANALYZE/IMAGE/FIXUP_SECTION "` + imageFixtureExe + `"`, true},
	} {
		buf.Reset()

		if err := d.Dispatch(tt.line); err != nil {
			t.Fatalf("%s: %v", tt.line, err)
		}

		out := buf.String()

		if !strings.Contains(out, "\tImage Section Descriptors (ISD)\n") {
			t.Errorf("%s: no image header", tt.line)
		}

		if got := strings.Contains(out, "IMAGE ACTIVATOR FIXUP SECTION\n"); got != tt.fixups {
			t.Errorf("%s: fixup section shown %v, want %v", tt.line, got, tt.fixups)
		}
	}
}

func TestAnalyzeImage_volume(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	data, err := os.ReadFile(imageFixtureExe)
	if err != nil {
		t.Fatal(err)
	}

	s := c.ContainerSession

	blocks := make([][]byte, 0, len(data)/512)
	for i := 0; i < len(data); i += 512 {
		blocks = append(blocks, data[i:i+512])
	}

	if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]ADDR.EXE"}, rms.ImageBlocks, blocks); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch("ANALYZE/IMAGE/OUTPUT DUA0:[000000]ADDR"); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	lines, _, err := s.ReadRecordFile(rms.FileLocation{Name: "DUA0:[000000]ADDR.ANI"}, rms.TextRecords)
	if err != nil {
		t.Fatalf("reading the analysis: %v", err)
	}

	if len(lines) < 4 || string(lines[0]) != "\f" || string(lines[2]) != "DUA0:[000000]ADDR.EXE;1" {
		t.Errorf("the analysis begins %q", lines[:min(4, len(lines))])
	}

	want, err := os.ReadFile(imageFixtureAni)
	if err != nil {
		t.Fatal(err)
	}

	got := joinLines(lines) + "\n"
	if normalizeImageAnalysis(got, false) != normalizeImageAnalysis(string(want), false) {
		t.Errorf("the analysis differs from VMS's:\n%s", got)
	}
}

// TestAnalyzeImage_linked links EXTERN.EXE from the objects real LINK
// linked it from, and checks govax's analysis of govax's image against
// VMS's analysis of VMS's, all but the times.
func TestAnalyzeImage_linked(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	exe := filepath.Join(t.TempDir(), "extern.exe")

	link := `LINK "` + imageFixtureDir + `/extern.obj","` + imageFixtureDir + `/defs.obj"/EXECUTABLE="` + exe + `"`
	if err := d.Dispatch(link); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	buf.Reset()

	if err := d.Dispatch(`ANALYZE/IMAGE "` + exe + `"`); err != nil {
		t.Fatalf("ANALYZE: %v", err)
	}

	want, err := os.ReadFile(imageFixtureDir + "/extern.ani")
	if err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if normalizeImageAnalysis(got, true) != normalizeImageAnalysis(string(want), true) {
		t.Errorf("the analysis differs from VMS's:\n%s", got)
	}
}

func TestAnalyzeImage_errors(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	// A damaged image: the first ISD's section type undefined.
	data, err := os.ReadFile(imageFixtureExe)
	if err != nil {
		t.Fatal(err)
	}

	data[0xB0+11] = 7
	bad := filepath.Join(dir, "bad.exe")

	if err := os.WriteFile(bad, data, 0o644); err != nil {
		t.Fatal(err)
	}

	err = d.Dispatch(`ANALYZE/IMAGE "` + bad + `"`)
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_ANALYZEERRORS)) {
		t.Errorf("ANALYZE of a bad image: %v, want CLI_ANALYZEERRORS", err)
	}

	if !strings.Contains(buf.String(), "The analysis uncovered 1 error.") {
		t.Error("the report of the bad image wasn't written")
	}

	// A file too short to be an image.
	short := filepath.Join(dir, "short.exe")
	if err := os.WriteFile(short, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch(`ANALYZE/IMAGE "` + short + `"`); !errors.Is(err, vmserrors.New(vmserrors.CLI_ANALYZE)) {
		t.Errorf("ANALYZE of a short file: %v, want CLI_ANALYZE", err)
	}

	if err := d.Dispatch(`ANALYZE/IMAGE "` + filepath.Join(dir, "missing.exe") + `"`); !errors.Is(err, vmserrors.New(vmserrors.SS_NOSUCHFILE)) {
		t.Errorf("ANALYZE of a missing file: %v, want SS_NOSUCHFILE", err)
	}
}
