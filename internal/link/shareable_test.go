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
// object name and cluster, and the options it was linked with.
type shareableCase struct {
	image   string
	objects [][2]string
	options func(*Options)
}

// shareableCases are the probe's shareable images that govax's LINK makes
// so far: position independent, not based, with .ADDRESS only in
// writable sections.
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

		opts.Sources = []SymbolSource{govaxTables()}
		opts.Shareable = true
		c.options(&opts)

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

// TestLinkShareableRefusals checks what LINK/SHAREABLE refuses until later
// subtasks: a based image, a position-dependent psect (SHR3), and a
// .ADDRESS in a read-only section (SHR6).
func TestLinkShareableRefusals(t *testing.T) {
	base := uint32(0x300000)

	for _, c := range []struct {
		object string
		base   *uint32
		want   string
	}{
		{"shr3", &base, "based shareable image"},
		{"shr3", nil, "position dependent"},
		{"shr6", nil, "read-only section"},
	} {
		_, err := Link([]Input{{File: c.object + ".obj", Module: link68Object(t, c.object)}}, Options{
			Shareable: true, Base: c.base, Sources: []SymbolSource{govaxTables()},
		})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want one about a %s", c.object, err, c.want)
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
		opts.Sources = []SymbolSource{govaxTables()}
		opts.Shareable = true
		c.options(&opts)

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
	for _, name := range []string{"shr1", "shr1a", "shr5"} {
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
