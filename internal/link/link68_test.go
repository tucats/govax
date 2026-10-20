package link

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// link68Dir is Phase 68's probe (testdata/link68), whose vax directory
// holds real MACRO's objects and real LINK's images.
var link68Dir = filepath.Join("..", "..", "testdata", "link68")

// link68Object reads the object real MACRO made of one of the probe's
// sources.
func link68Object(t *testing.T, name string) *obj.Module {
	t.Helper()

	f, err := os.Open(filepath.Join(link68Dir, "vax", name+".obj"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	records, err := obj.ReadRecords(f)
	if err != nil {
		t.Fatal(err)
	}

	m, err := obj.Decode(records)
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// TestLinkTransferVectorExecutable links the probe's E2, a transfer
// vector (XFR1, whose .TRANSFERs are OPR_REDEF commands) and SHR1's
// routines as an executable: the redefinitions mean nothing there, and
// the image, which has no user transfer address, is real LINK's byte
// for byte. It is linked from real MACRO's objects and from govax's.
func TestLinkTransferVectorExecutable(t *testing.T) {
	want, opts := realImage(t, filepath.Join(link68Dir, "vax", "e2.exe"))
	opts.Sources = []SymbolSource{govaxTables()}

	link := func(xfr1, shr1 *obj.Module) {
		t.Helper()

		img, err := Link([]Input{
			{File: "DUA1:[000000]XFR1.OBJ;1", Module: xfr1},
			{File: "DUA1:[000000]SHR1.OBJ;1", Module: shr1},
		}, opts)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(img.Bytes, want) {
			t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
		}
	}

	link(link68Object(t, "xfr1"), link68Object(t, "shr1"))

	var govax []*obj.Module

	for _, name := range []string{"xfr1", "shr1"} {
		src, err := os.ReadFile(filepath.Join(link68Dir, name+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		govax = append(govax, macroModule(t, string(src)))
	}

	link(govax[0], govax[1])
}

// exe1Options are the options the probe's R11 linked EXE1 with, from
// SYS$INPUT: BASE=, a based cluster with EXE1B in it, COLLECT=,
// PSECT_ATTR=, IOSEGMENT=, DZRO_MIN=, and ISD_MAX=.
func exe1Options(opts Options) Options {
	u32 := func(v uint32) *uint32 { return &v }

	opts.Sources = []SymbolSource{govaxTables()}
	opts.Base = u32(0x10000)
	opts.Clusters = []Cluster{{Name: "SECOND", Base: u32(0x40000)}}
	opts.Collects = []Collect{{Cluster: "SECOND", Psects: []string{"EXE1_RO"}}}
	opts.PsectAttrs = []PsectAttr{{Name: "EXE1_DATA", Attributes: []string{"NOWRT"}}}
	opts.IOSegment = &IOSegment{Pages: 40, P0Bufs: true}
	opts.DZROMin = u32(2)
	opts.ISDMax = u32(40)

	return opts
}

// TestLinkClustersExecutable links the probe's EXE1, an executable laid
// out by options, and checks the image is real LINK's byte for byte: the
// default cluster at BASE=, cluster SECOND at its own base with EXE1B's
// module (read first) and the collected EXE1_RO, EXE1_DATA made
// read-only, EXE1_ZERO's four untouched pages a demand-zero section of
// their own (DZRO_MIN=2), LIBRTL placed at link time in the gap after the
// default cluster, with no fixup cells, the fixup section after SECOND,
// LASTCLU on SECOND's sections, and the I/O segment in the header. It is
// linked from real MACRO's objects and from govax's.
func TestLinkClustersExecutable(t *testing.T) {
	want, opts := realImage(t, filepath.Join(link68Dir, "vax", "exe1.exe"))
	opts = exe1Options(opts)

	link := func(exe1, exe1b *obj.Module) *Image {
		t.Helper()

		img, err := Link([]Input{
			{File: "DUA1:[000000]EXE1.OBJ;1", Module: exe1},
			{File: "DUA1:[000000]EXE1B.OBJ;1", Module: exe1b, Cluster: "SECOND"},
		}, opts)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(img.Bytes, want) {
			t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
		}

		return img
	}

	img := link(link68Object(t, "exe1"), link68Object(t, "exe1b"))

	// The modules are read cluster by cluster: SECOND's first.
	if got := img.l.modules[0].name; got != "EXE1B" {
		t.Errorf("first module %s, want EXE1B", got)
	}

	var govax []*obj.Module

	for _, name := range []string{"exe1", "exe1b"} {
		src, err := os.ReadFile(filepath.Join(link68Dir, name+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		govax = append(govax, macroModule(t, string(src)))
	}

	link(govax[0], govax[1])
}

// TestMapClustersExecutable checks the lines of EXE1's default map that
// real LINK's /FULL map of it shares: the psect synopsis, by address,
// with EXE1_DATA read-only and EXE1_RO global, and the image synopsis's
// layout lines. (The counts of files, modules, psects, and symbols
// include STARLET's SYS$P1_VECTOR, which govax's tables stand in for.)
func TestMapClustersExecutable(t *testing.T) {
	_, opts := realImage(t, filepath.Join(link68Dir, "vax", "exe1.exe"))

	img, err := Link([]Input{
		{File: "DUA1:[000000]EXE1.OBJ;1", Module: link68Object(t, "exe1")},
		{File: "DUA1:[000000]EXE1B.OBJ;1", Module: link68Object(t, "exe1b"), Cluster: "SECOND"},
	}, exe1Options(opts))
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(link68Dir, "vax", "exe1.map"))
	if err != nil {
		t.Fatal(err)
	}

	vms := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		vms[strings.TrimRight(line, " \r")] = true
	}

	lines := img.Map(MapOptions{ImageFile: "DUA1:[000000]EXE1.EXE;1", MapFile: "DUA1:[000000]EXE1.MAP;1"})

	checked := 0

	for _, line := range lines {
		line = strings.TrimRight(line, " ")

		for _, prefix := range []string{
			"EXE1", "SAY_DATA", "                EXE1",
			"Virtual memory allocated:", "Stack size:", "Image header", "Image binary",
			"Image name", "Number of image sections:", "User transfer", "Debugger transfer", "Image type:",
		} {
			if strings.HasPrefix(line, prefix) && !strings.HasPrefix(line, "EXE1 ") {
				checked++

				if !vms[line] {
					t.Errorf("map line %q isn't real LINK's", line)
				}
			}
		}
	}

	if checked < 25 {
		t.Errorf("only %d lines checked", checked)
	}
}

// link68VMSSources are real LINK's default sources, as vmsSources, named
// as on the probe's VAX, after the shareable images an options file
// named (VMS's, from testdata/link68/vax, under the names the maps show).
func link68VMSSources(t *testing.T, uses []string) []SymbolSource {
	t.Helper()

	sources, _ := link68Uses(t, uses)

	vms := vmsSources(t)
	lib := vms[0].(*ImageLibrarySource)
	open := lib.Open
	lib.Open = func(image string) (SymbolSource, error) {
		src, err := open(image)
		if ts, ok := src.(*TableSource); ok {
			ts.File = "SYS$COMMON:[SYSLIB]" + image + ".EXE;1"
		}

		return src, err
	}
	vms[1].(*ObjectLibrarySource).File = "SYS$COMMON:[SYSLIB]STARLET.OLB;1"

	return append(sources, vms...)
}

// link68ImageVersions are the versions of the probe's shareable images
// that the other links read, as their maps name them.
var link68ImageVersions = map[string]string{"shr1": "8", "shr3p": "1", "shr6": "1"}

// TestMapShareableImagesFull links the probe's shareable images from real
// MACRO's objects with real LINK's libraries, and checks each /FULL map
// line for line against real LINK's, up to its run statistics: the
// shareable images and library modules among the modules, the
// relocatable references, the image sections (types, the fixup section,
// the images used), every psect's contributions, the symbols by name with
// their U flags and those of the images and libraries the image uses,
// the symbols by value, and the image synopsis's SHAREABLE line.
func TestMapShareableImagesFull(t *testing.T) {
	for _, c := range shareableCases {
		if c.image == "e1" {
			continue // the probe kept no map of E1
		}

		t.Run(c.image, func(t *testing.T) {
			want, _, image, mapFile := realMap(t, filepath.Join(link68Dir, "vax", c.image+".map"))

			_, opts := realImage(t, filepath.Join(link68Dir, "vax", c.image+".exe"))
			opts = c.shareableOptions(t, opts)
			opts.Sources = link68VMSSources(t, c.uses)

			var inputs []Input
			for _, o := range c.objects {
				inputs = append(inputs, Input{File: "DUA1:[000000]" + strings.ToUpper(o[0]) + ".OBJ;1", Module: link68Object(t, o[0]), Cluster: o[1]})
			}

			img, err := Link(inputs, opts)
			if err != nil {
				t.Fatal(err)
			}

			got := img.Map(MapOptions{ImageFile: image, ImageText: strings.ToUpper(c.image), MapFile: mapFile, Full: true})

			for i := range max(len(got), len(want)) {
				var g, w string
				if i < len(got) {
					g = got[i]
				}

				if i < len(want) {
					w = want[i]
				}

				if g != w {
					t.Errorf("line %d:\n got %q\nwant %q", i+1, g, w)
				}
			}
		})
	}
}

// TestMapExecutablesFull links the probe's programs from real MACRO's
// objects, against VMS's shareable images named as in their options files
// and real LINK's libraries, and checks each /FULL map line for line
// against real LINK's, up to its run statistics: the shareable image
// among the modules and its sections in the image section synopsis
// (each a global section of its own, relocatable), the user stack with
// the default cluster, the .ADDRESS references to shareable images, and
// each symbol in a shareable image at its cell. EXE1 adds a based
// cluster, a demand-zero section split off, and LIBRTL placed at link
// time. MAIN1, MAIN2, and MAIN4, whose LIBRTL real LINK put right after
// the shareable image that uses it, are subtask 9's.
func TestMapExecutablesFull(t *testing.T) {
	cases := []struct {
		image, object string
		uses          string
		// text is the image's name as /EXECUTABLE= gave it, which the
		// first page's heading shows ("" without the qualifier).
		text string
	}{
		{image: "main3p", object: "main3", uses: "shr3p", text: "MAIN3P"},
		{image: "main6", object: "main6", uses: "shr6"},
		{image: "exe1"},
	}

	for _, c := range cases {
		t.Run(c.image, func(t *testing.T) {
			want, _, image, mapFile := realMap(t, filepath.Join(link68Dir, "vax", c.image+".map"))
			_, opts := realImage(t, filepath.Join(link68Dir, "vax", c.image+".exe"))

			var (
				inputs []Input
				uses   []string
			)

			if c.image == "exe1" {
				opts = exe1Options(opts)
				inputs = []Input{
					{File: "DUA1:[000000]EXE1.OBJ;1", Module: link68Object(t, "exe1")},
					{File: "DUA1:[000000]EXE1B.OBJ;1", Module: link68Object(t, "exe1b"), Cluster: "SECOND"},
				}
			} else {
				uses = []string{c.uses}
				_, opts.Clusters = link68Uses(t, uses)
				inputs = []Input{{File: "DUA1:[000000]" + strings.ToUpper(c.object) + ".OBJ;1", Module: link68Object(t, c.object)}}
			}

			opts.Sources = link68VMSSources(t, uses)

			img, err := Link(inputs, opts)
			if err != nil {
				t.Fatal(err)
			}

			got := img.Map(MapOptions{ImageFile: image, ImageText: c.text, MapFile: mapFile, Full: true})

			for i := range max(len(got), len(want)) {
				var g, w string
				if i < len(got) {
					g = got[i]
				}

				if i < len(want) {
					w = want[i]
				}

				if g != w {
					t.Errorf("line %d:\n got %q\nwant %q", i+1, g, w)
				}
			}
		})
	}
}

// TestMapShareableImagesGovaxTables links the probe's shareable images
// with govax's own symbol tables, which TestMapShareableImagesFull's
// libraries stand in for where VMS's aren't at hand, and checks that each
// /FULL map's lines are real LINK's but for what those tables don't know:
// LIBRTL's ident, file, and date, STARLET.OLB's SYS$P1_VECTOR module, and
// the counts and the length estimate that follow from them.
func TestMapShareableImagesGovaxTables(t *testing.T) {
	differ := []string{
		"LIBRTL   ", "Number of files:", "Number of modules:", "Number of program sections:",
		"Number of global symbols:", "Estimated map length:",
	}

	for _, c := range shareableCases {
		if c.image == "e1" {
			continue
		}

		t.Run(c.image, func(t *testing.T) {
			img, _ := linkShareableCase(t, c.image)
			want, _, image, mapFile := realMap(t, filepath.Join(link68Dir, "vax", c.image+".map"))

			vms := map[string]bool{}
			for _, line := range want {
				vms[line] = true
			}

			got := img.Map(MapOptions{ImageFile: image, ImageText: strings.ToUpper(c.image), MapFile: mapFile, Full: true})

		lines:
			for i, line := range got {
				for _, d := range differ {
					if strings.HasPrefix(line, d) {
						continue lines
					}
				}

				if !vms[line] {
					t.Errorf("line %d %q isn't real LINK's", i+1, line)
				}
			}

			// Real LINK's map has SYS$P1_VECTOR's line too, if it used
			// STARLET.OLB.
			if n := len(want) - len(got); n != 0 && n != 1 {
				t.Errorf("%d lines, real LINK's %d", len(got), len(want))
			}
		})
	}
}
