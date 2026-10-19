package link

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// withoutGST returns a real shareable image without its global symbol
// table, which govax's LINK doesn't write yet (docs/PHASE-68, subtask 6):
// the file ends before the table's first block, and the header's IHS
// doesn't name it.
func withoutGST(t *testing.T, image []byte) []byte {
	t.Helper()

	le := binary.LittleEndian
	b := bytes.Clone(image)

	vbn := le.Uint32(b[ihsOffset+4:])
	if vbn == 0 {
		t.Fatal("the image has no global symbol table")
	}

	b = b[:(vbn-1)*blockSize]
	le.PutUint32(b[ihsOffset+4:], 0)
	le.PutUint16(b[ihsOffset+10:], 0)
	le.PutUint32(b[ihsOffset+24:], 0)

	return b
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
		},
	},
	{
		// No transfer vector, no GSMATCH= (the ident from the link time,
		// EQUAL), and /NOTRACEBACK (a first transfer address of 0).
		image:   "shr1a",
		objects: [][2]string{{"shr1", ""}},
		options: func(o *Options) { o.Traceback = false },
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
		},
	},
	{
		// PROTECT=YES around cluster PROT: its section is ISD$V_PROTECT.
		image:   "shr5",
		objects: [][2]string{{"xfr1", "PROT"}, {"shr1", ""}},
		options: func(o *Options) { o.Clusters = []Cluster{{Name: "PROT", Protect: true}} },
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
// each is real LINK's byte for byte, but for the global symbol table:
// the IHD$K_LIM header with LNKNOTFR, the GSMATCH in IHD$L_IDENT and the
// link flags' top byte, base 0, Table 6-1's shareable order and section
// types, no demand-zero runs split off, no LASTCLU, no stack, and a PRVFXD
// fixup section with IAF$V_SHR, whose .ADDRESS list for image 0 holds the
// image's own addresses. Each is linked from real MACRO's objects and
// from govax's.
func TestLinkShareableImages(t *testing.T) {
	for _, c := range shareableCases {
		real, opts := realImage(t, filepath.Join(link68Dir, "vax", c.image+".exe"))
		want := withoutGST(t, real)

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
