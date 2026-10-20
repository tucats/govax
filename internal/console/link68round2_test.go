package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// docs/PHASE-68 - link shareable images.md, subtask 13's second round:
// VMS V7.1 ran testdata/link68/round2's RUN2.COM on a volume govax had
// run LINK68.COM on. It analyzed govax's images (RUN2's NAME.AN2), ran
// govax's programs against govax's shareable images, linked VMS's
// MACRO's objects against them (MAIN1V, MAIN2V) and through govax's
// library (MAIN1V.EXE;2, MAIN1W.MAP), and made a library of govax's
// images (MYSHRV.OLB). testdata/link68/vax/round2/govax holds govax's
// images and library from that volume.
//
// TestLink68Round2 runs RUN2.COM on govax with those same images, and
// holds what it writes to VMS's: the log, ANALYZE/IMAGE's reports of
// govax's images, the maps and images of the links, and the library
// listings.

// link68Round2Dir holds round 2's results, and govax's files VMS used.
var link68Round2Dir = filepath.Join(link68Dir, "vax", "round2")

// link68Round2Images are govax's images on round 2's volume, each with
// the version it had there (VMS's reports name it).
var link68Round2Images = map[string]int{
	"shr1": 8, "shr1a": 2, "shr2": 1, "shr3": 1, "shr3p": 1, "shr4": 2, "shr5": 1, "shr6": 1,
	"main1": 1, "main1a": 1, "main1n": 1, "main1l": 1, "main1u": 1, "main2": 1, "main3": 1,
	"main3p": 1, "main4": 1, "main6": 1, "exe1": 1,
}

// link68Round2Files are round 2's results compared, as copied from the
// volume: VMS's file name, and the volume file govax's run left.
var link68Round2Files = map[string]string{
	"main1v-1.exe": "MAIN1V.EXE;1",
	"main1v.exe":   "MAIN1V.EXE;2", // the link through MYSHR.OLB, which wanted MAIN1W
	"main2v.exe":   "MAIN2V.EXE;1",
	"main1v.map":   "MAIN1V.MAP;1",
	"main2v.map":   "MAIN2V.MAP;1",
	"main1w.map":   "MAIN1W.MAP;1",
	"myshrv.lis":   "MYSHRV.LIS;1",
	"myshrg.lis":   "MYSHRG.LIS;1",
}

// TestLink68Round2 runs RUN2.COM on govax, in both paging modes.
func TestLink68Round2(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, buf := link68Govax(t, paging)

		// What round 2's volume held: govax's images, each copied until it
		// has its version there (COPY takes the next version), govax's
		// library, and RUN2.COM.
		for name, version := range link68Round2Images {
			for range version {
				shareFindRun(t, d, `COPY "`+filepath.Join(link68Round2Dir, "govax", name+".exe")+`"/HOST/BINARY DUA1:[000000]`+strings.ToUpper(name)+".EXE")
			}
		}

		shareFindRun(t, d, `COPY "`+filepath.Join(link68Round2Dir, "govax", "myshr.olb")+`"/HOST/BINARY DUA1:[000000]MYSHR.OLB`)
		shareFindRun(t, d, `COPY "`+filepath.Join(link68Dir, "round2", "run2.com")+`"/HOST DUA1:[000000]RUN2.COM`)
		buf.Reset()

		if err := d.Dispatch("@RUN2/OUTPUT=RUN2.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("@RUN2: %v\n%s", err, buf)
		}

		out := t.TempDir()
		copyOut := func(volume, host string, binary bool) {
			t.Helper()

			line := "COPY DUA1:[000000]" + volume + ` "` + filepath.Join(out, host) + `"/HOST/QUIET`
			if binary {
				line += "/BINARY"
			}

			shareFindRun(t, d, line)
		}

		copyOut("RUN2.LOG", "run2.log", false)

		for name := range link68Round2Images {
			copyOut(strings.ToUpper(name)+".AN2", name+".an2", false)
		}

		for host, volume := range link68Round2Files {
			copyOut(volume, host, strings.HasSuffix(host, ".exe"))
		}

		shareFindRun(t, d, "DISMOUNT DUA1")

		t.Run("log", func(t *testing.T) { link68Round2Log(t, out) })

		for name := range link68Round2Images {
			t.Run(name+".an2", func(t *testing.T) { link68Round2Compare(t, out, name+".an2") })
		}

		for host := range link68Round2Files {
			t.Run(host, func(t *testing.T) { link68Round2Compare(t, out, host) })
		}
	})
}

// link68Round2Log holds govax's RUN2.LOG to VMS's, masked as
// LINK68.LOG is, up to the closing DIRECTORY, whose listing is the
// volume's (round 2's held the error links' images and VMS's own
// files).
func link68Round2Log(t *testing.T, out string) {
	cut := func(lines []string) []string {
		for i, line := range lines {
			if strings.HasPrefix(line, "$ DIRECTORY/SIZE=ALL/DATE") {
				return lines[:i+1]
			}
		}

		return lines
	}

	want := cut(link68MaskLog(link68Read(t, filepath.Join(link68Round2Dir, "run2.log"))))
	got := cut(link68MaskLog(link68Read(t, filepath.Join(out, "run2.log"))))

	link68CompareLines(t, "run2.log", got, want, link68LogDifferences)
}

// link68Round2Compare holds govax's copy of round 2's file name, in out,
// to VMS's: an image as link68CompareImage holds it, text as the round
// 1 oracle holds its text. The images MAIN1V and MAIN2V hold no
// link-time idents of their own images: they're executables, whose
// header's ident link68ImageMask passes over.
func link68Round2Compare(t *testing.T, out, name string) {
	vaxPath, govaxPath := filepath.Join(link68Round2Dir, name), filepath.Join(out, name)
	base := strings.TrimSuffix(name, filepath.Ext(name))

	if _, err := os.Stat(govaxPath); err != nil {
		t.Fatalf("govax's run made no %s", name)
	}

	if strings.HasSuffix(name, ".exe") {
		link68CompareImage(t, base, link68ReadBytes(t, vaxPath), link68ReadBytes(t, govaxPath))

		return
	}

	// A report names its image's version, the same on both volumes,
	// and its type is AN2, not ANI: link68MaskText cuts by type, and
	// an analysis is compared whole.
	link68CompareLines(t, name, link68MaskText(link68Read(t, govaxPath), name, base),
		link68MaskText(link68Read(t, vaxPath), name, base), link68TextDifferences)
}
