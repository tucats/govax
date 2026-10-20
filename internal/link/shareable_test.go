package link

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// padded returns a real image padded to a whole block: real LINK ends a
// shareable image at its global symbol table's last record, mid-block,
// where govax pads it (docs/PHASE-29.md, Decision 7).
func padded(image []byte) []byte {
	return append(bytes.Clone(image), make([]byte, int(pageUp(uint32(len(image))))-len(image))...)
}

// shareableCase is one of the probe's shareable images: its inputs, by
// object name and cluster, the options it was linked with, and the
// shareable images an options file named, which it is linked against
// (VMS's, from testdata/link68/vax).
type shareableCase struct {
	image   string
	objects [][2]string
	options func(*Options)
	uses    []string
}

// shareableCases are the probe's shareable images.
var shareableCases = []shareableCase{
	{
		// SHR1.OPT's transfer vector cluster first, then SHR1's
		// routines; GSMATCH=LEQUAL,1,1000.
		image:   "shr1",
		objects: [][2]string{{"xfr1", "SHR1_VECTORS"}, {"shr1", ""}},
		options: func(o *Options) {
			o.Clusters = []Cluster{{Name: "SHR1_VECTORS"}}
			o.GSMatch = &GSMatch{Match: MatchLEQ, Major: 1, Minor: 1000}
			o.Universals = []string{"SHR1_COUNT", "SHR1_TABLE", "SHR1_ORIGIN"}
		},
	},
	{
		// No transfer vector, so every routine is universal by
		// UNIVERSAL=, with its own address; no GSMATCH= (the ident from
		// the link time, EQUAL), and /NOTRACEBACK (a first transfer
		// address of 0).
		image:   "shr1a",
		objects: [][2]string{{"shr1", ""}},
		options: func(o *Options) {
			o.Traceback = false
			o.Universals = []string{
				"SHR1_BASE", "SHR1_ADD", "SHR1_JSB", "SHR1_SHOW",
				"SHR1_COUNT", "SHR1_TABLE", "SHR1_ORIGIN",
			}
		},
	},
	{
		// COLLECT= moves SHR1_RO into cluster FIRST, PSECT_ATTR= makes it
		// NOSHR (a read-only PRVPIC section after XFR1's SHRPIC one), and
		// DZRO_MIN=1 splits nothing; GSMATCH=ALWAYS,0,0.
		image:   "shr4",
		objects: [][2]string{{"xfr1", "FIRST"}, {"shr1", ""}},
		options: func(o *Options) {
			one := uint32(1)
			o.Clusters = []Cluster{{Name: "FIRST"}}
			o.Collects = []Collect{{Cluster: "FIRST", Psects: []string{"SHR1_RO"}}}
			o.PsectAttrs = []PsectAttr{{Name: "SHR1_RO", Attributes: []string{"NOSHR"}}}
			o.DZROMin = &one
			o.GSMatch = &GSMatch{Match: MatchAlways}
			o.Universals = []string{"SHR1_COUNT", "SHR1_TABLE", "SHR1_ORIGIN"}
		},
	},
	{
		// PROTECT=YES around cluster PROT: its section is ISD$V_PROTECT.
		image:   "shr5",
		objects: [][2]string{{"xfr1", "PROT"}, {"shr1", ""}},
		options: func(o *Options) {
			o.Clusters = []Cluster{{Name: "PROT", Protect: true}}
			o.Universals = []string{"SHR1_COUNT"}
		},
	},
	{
		// E1: the only universal symbol is one nothing defines, so the
		// global symbol table lists none, and its end of module record
		// carries the link's warning.
		image:   "e1",
		objects: [][2]string{{"shr1", ""}},
		options: func(o *Options) { o.Universals = []string{"NOSUCH"} },
	},
	{
		// SHR2 uses SHR1 (SHR1/SHAREABLE): G^ cells for SHR1 in
		// descending name order, a .ADDRESS list for it, LIBRTL in the
		// shareable image list and a global section ISD for it, as SHR1
		// uses it, and SHR1's ident ("V1.0"), the first module read;
		// GSMATCH=EQUAL,2,5.
		image:   "shr2",
		objects: [][2]string{{"shr2", ""}},
		uses:    []string{"shr1"},
		options: func(o *Options) {
			o.GSMatch = &GSMatch{Match: MatchEqual, Major: 2, Minor: 5}
			o.Universals = []string{"SHR2_TWICE", "SHR2_COUNT", "SHR2_ADDR"}
		},
	},
	{
		// SHR3, position dependent and based (BASE=%X300000): SHRFXD and
		// PRVFXD sections at their addresses, PICIMG clear, absolute
		// universal symbols, and no fixup section, since its .ADDRESS
		// needs no fixing.
		image:   "shr3",
		objects: [][2]string{{"shr3", ""}},
		options: func(o *Options) {
			base := uint32(0x300000)
			o.Base = &base
			o.Universals = []string{"SHR3_GET", "SHR3_DATA"}
		},
	},
	{
		// SHR3P, the same objects not based: PICIMG set, and a .ADDRESS
		// list for image 0.
		image:   "shr3p",
		objects: [][2]string{{"shr3", ""}},
		options: func(o *Options) { o.Universals = []string{"SHR3_GET", "SHR3_DATA"} },
	},
	{
		// SHR6, a .ADDRESS in a shared, read-only psect: its section is
		// CRF+WRT (still SHRPIC), and a second change-protection entry
		// makes it PRT$C_UR once fixed up.
		image:   "shr6",
		objects: [][2]string{{"shr6", ""}},
		options: func(o *Options) { o.Universals = []string{"SHR6_GET"} },
	},
}

// link68Uses returns VMS's shareable images names, from the probe, as
// symbol sources (each file named as the maps of the images linked
// against it name it), and the clusters the options files that named
// them make.
func link68Uses(t *testing.T, names []string) ([]SymbolSource, []Cluster) {
	t.Helper()

	var (
		sources  []SymbolSource
		clusters []Cluster
	)

	for _, n := range names {
		data, err := os.ReadFile(filepath.Join(link68Dir, "vax", n+".exe"))
		if err != nil {
			t.Fatal(err)
		}

		src, image, err := ReadShareableImage(data)
		if err != nil {
			t.Fatal(err)
		}

		src.File = "DUA1:[000000]" + strings.ToUpper(n) + ".EXE;" + link68ImageVersions[n]
		sources = append(sources, src)
		clusters = append(clusters, Cluster{Shareable: true, Image: image})
	}

	return sources, clusters
}

// shareableOptions returns c's options, added to realImage's opts: the
// shareable images it uses, first, then govax's tables, as sources, and
// their clusters before any its options make.
func (c shareableCase) shareableOptions(t *testing.T, opts Options) Options {
	t.Helper()

	sources, clusters := link68Uses(t, c.uses)

	opts.Sources = append(sources, govaxTables())
	opts.Shareable = true
	c.options(&opts)
	opts.Clusters = append(clusters, opts.Clusters...)

	return opts
}

// link68Source assembles one of the probe's sources with govax's MACRO.
func link68Source(t *testing.T, name string) *obj.Module {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(link68Dir, name+".mar"))
	if err != nil {
		t.Fatal(err)
	}

	return macroModule(t, string(src))
}

// TestLinkShareableImages links the probe's shareable images and checks
// each is real LINK's byte for byte (padded to a whole block): the
// IHD$K_LIM header with LNKNOTFR, the GSMATCH in IHD$L_IDENT and the
// link flags' top byte, base 0, Table 6-1's shareable order and section
// types, no demand-zero runs split off, no LASTCLU, no stack, and a PRVFXD
// fixup section with IAF$V_SHR, whose .ADDRESS list for image 0 holds the
// image's own addresses, and the global symbol table of the universal
// symbols, a .TRANSFER'd one's value its transfer vector entry. Each is
// linked from real MACRO's objects and from govax's.
func TestLinkShareableImages(t *testing.T) {
	for _, c := range shareableCases {
		real, opts := realImage(t, filepath.Join(link68Dir, "vax", c.image+".exe"))
		want := padded(real)
		opts = c.shareableOptions(t, opts)

		for _, from := range []string{"real", platformName} {
			t.Run(c.image+"/"+from, func(t *testing.T) {
				var inputs []Input

				for _, o := range c.objects {
					m := link68Object(t, o[0])
					if from == platformName {
						m = link68Source(t, o[0])
					}

					inputs = append(inputs, Input{File: "DUA1:[000000]" + strings.ToUpper(o[0]) + ".OBJ;1", Module: m, Cluster: o[1]})
				}

				img, err := Link(inputs, opts)
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(img.Bytes, want) {
					t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
				}

				if img.HasTransfer {
					t.Error("a shareable image has a user transfer address")
				}
			})
		}
	}
}

// linkShareableCase links one of shareableCases from real MACRO's
// objects.
func linkShareableCase(t *testing.T, name string) (*Image, []byte) {
	t.Helper()

	for _, c := range shareableCases {
		if c.image != name {
			continue
		}

		real, opts := realImage(t, filepath.Join(link68Dir, "vax", c.image+".exe"))
		opts = c.shareableOptions(t, opts)

		var inputs []Input
		for _, o := range c.objects {
			inputs = append(inputs, Input{File: "DUA1:[000000]" + strings.ToUpper(o[0]) + ".OBJ;1", Module: link68Object(t, o[0]), Cluster: o[1]})
		}

		img, err := Link(inputs, opts)
		if err != nil {
			t.Fatal(err)
		}

		return img, real
	}

	t.Fatalf("no case %s", name)

	return nil, nil
}

// TestLinkSymbolTableFile checks LINK/SYMBOL_TABLE's file for SHR1: the
// image's global symbol table as an object module file, byte for byte
// real LINK's SHR1.STB.
func TestLinkSymbolTableFile(t *testing.T) {
	img, _ := linkShareableCase(t, "shr1")

	records, err := img.SymbolTable()
	if err != nil {
		t.Fatal(err)
	}

	var got bytes.Buffer
	if err := obj.WriteRecords(&got, records); err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile(filepath.Join(link68Dir, "vax", "shr1.stb"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("symbol table file differs from real LINK's:\n%x\nwant\n%x", got.Bytes(), want)
	}
}

// TestReadGovaxShareableImage checks that ReadShareableImage reads the
// shareable images govax's LINK makes as it reads real LINK's: the same
// symbols, values, and image.
func TestReadGovaxShareableImage(t *testing.T) {
	for _, name := range []string{"shr1", "shr1a", "shr2", "shr3", "shr3p", "shr5", "shr6"} {
		img, real := linkShareableCase(t, name)

		got, gotName, err := ReadShareableImage(img.Bytes)
		if err != nil {
			t.Fatal(err)
		}

		want, wantName, err := ReadShareableImage(real)
		if err != nil {
			t.Fatal(err)
		}

		if gotName != wantName || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: read as %s %+v, want %s %+v", name, gotName, got, wantName, want)
		}
	}
}

// TestLinkUndefinedUniversal checks E1's messages: a UNIVERSAL= symbol
// nothing defines is an undefined symbol, a warning, and the image is
// still made.
func TestLinkUndefinedUniversal(t *testing.T) {
	img, _ := linkShareableCase(t, "e1")

	var got []string
	for _, m := range img.Messages {
		got = append(got, m.String())
	}

	want := []string{"%LINK-W-NUDFSYMS, 1 undefined symbol:", "%LINK-I-UDFSYM, \tNOSUCH "}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("messages %q, want %q", got, want)
	}
}

// TestUniversalInExecutable checks that an executable link takes
// UNIVERSAL= and .TRANSFER without effect: the probe's E2, with a
// UNIVERSAL= option added, is still real LINK's image.
func TestUniversalInExecutable(t *testing.T) {
	want, opts := realImage(t, filepath.Join(link68Dir, "vax", "e2.exe"))
	opts.Sources = []SymbolSource{govaxTables()}
	opts.Universals = []string{"SHR1_COUNT", "NOSUCH"}

	img, err := Link([]Input{
		{File: "DUA1:[000000]XFR1.OBJ;1", Module: link68Object(t, "xfr1")},
		{File: "DUA1:[000000]SHR1.OBJ;1", Module: link68Object(t, "shr1")},
	}, opts)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(img.Bytes, want) {
		t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
	}
}
