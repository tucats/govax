package link

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// link68Program is one of the probe's programs, linked against a
// shareable image an options file named.
type link68Program struct {
	image, object string
	// uses is the shareable image, with the version of its file the
	// program's map names ("shr1;6").
	uses string
	// text is the image's name as /EXECUTABLE= gave it, which the map's
	// first page shows ("" without the qualifier).
	text string
	// gsmatch is the GSMATCH= the shareable image had been linked with
	// when the program was, if not its case's (shareableCases): MAIN1N
	// was linked while SHR1 was EQUAL,1,1001.
	gsmatch *GSMatch
}

// link68Programs are the probe's programs that use its shareable images.
var link68Programs = []link68Program{
	// SHR1 named in an options file, then LIBRTL, which SHR1 uses, in
	// the cluster right after it, though MAIN1 calls LIB$PUT_OUTPUT
	// itself.
	{image: "main1", object: "main1", uses: "shr1;1"},
	{image: "main1a", object: "main1", uses: "shr1a;1", text: "MAIN1A"},
	{image: "main1n", object: "main1", uses: "shr1;6", text: "MAIN1N", gsmatch: &GSMatch{Match: MatchEqual, Major: 1, Minor: 1001}},
	// SHR2, which uses SHR1, which uses LIBRTL: SHR1's global section
	// ISD has its ident and first section's pages, read from SHR1.EXE.
	{image: "main2", object: "main2", uses: "shr2;1"},
	// SHR3 is based: a global section ISD for each of its sections, at
	// its address, BASED, and LASTCLU, since it's the cluster at the
	// highest address (so the default cluster's sections and the stack
	// aren't), no cells for it, and PICIMG clear.
	{image: "main3", object: "main3", uses: "shr3;1"},
	// SHR3P's first section is position dependent, so its global
	// section ISD is ISD$K_SHRFXD.
	{image: "main3p", object: "main3", uses: "shr3p;1", text: "MAIN3P"},
	{image: "main4", object: "main1", uses: "shr4;1", text: "MAIN4"},
	// SHR6's first section is fixed up by the activator, so its global
	// section ISD is CRF+WRT.
	{image: "main6", object: "main6", uses: "shr6;1"},
}

// link68Opener is the source of the images the program's shareable image
// uses itself (MAIN2's SHR1), as LINK finds their files: VMS's, at the
// version the maps name, or, with govax, those govax's LINK makes.
func link68Opener(t *testing.T, govax bool) *ImageFileSource {
	t.Helper()

	return &ImageFileSource{Open: func(image string) (SymbolSource, error) {
		name := strings.ToLower(image)

		if govax {
			src, _ := govaxShareableImage(t, name, nil)

			return src, nil
		}

		sources, _ := link68Uses(t, []string{name})

		return sources[0], nil
	}}
}

// govaxShareableImage links one of shareableCases with govax's MACRO and
// LINK, its GSMATCH= gsmatch if that isn't nil, and reads the image as a
// program linked against it would: its source (its file named as
// link68Uses names VMS's) and its name.
func govaxShareableImage(t *testing.T, uses string, gsmatch *GSMatch) (*TableSource, string) {
	t.Helper()

	name, version, ok := strings.Cut(uses, ";")
	if !ok {
		version = link68ImageVersions[name]
	}

	for _, c := range shareableCases {
		if c.image != name {
			continue
		}

		_, opts := realImage(t, filepath.Join(link68Dir, "vax", c.image+".exe"))
		opts = c.shareableOptions(t, opts)

		if gsmatch != nil {
			opts.GSMatch = gsmatch
		}

		var inputs []Input
		for _, o := range c.objects {
			inputs = append(inputs, Input{File: "DUA1:[000000]" + strings.ToUpper(o[0]) + ".OBJ;1", Module: link68Source(t, o[0]), Cluster: o[1]})
		}

		img, err := Link(inputs, opts)
		if err != nil {
			t.Fatal(err)
		}

		src, image, err := ReadShareableImage(img.Bytes)
		if err != nil {
			t.Fatal(err)
		}

		src.File = "DUA1:[000000]" + strings.ToUpper(name) + ".EXE;" + version

		return src, image
	}

	t.Fatalf("no shareable image case %s", name)

	return nil, ""
}

// options returns the program's link options, added to realImage's opts:
// its shareable image's cluster first, and as sources the image (VMS's
// file, or with govax the image govax's LINK makes), then rest, then the
// opener of the images the image uses.
func (p link68Program) options(t *testing.T, opts Options, govax bool, rest ...SymbolSource) Options {
	t.Helper()

	var (
		src   *TableSource
		image string
	)

	if govax {
		src, image = govaxShareableImage(t, p.uses, p.gsmatch)
	} else {
		sources, _ := link68Uses(t, []string{p.uses})
		src = sources[0].(*TableSource)

		for name := range src.Images {
			image = name
		}

		if p.gsmatch != nil {
			i := src.Images[image]
			i.Match, i.MajorID, i.MinorID = p.gsmatch.Match, p.gsmatch.Major, p.gsmatch.Minor
			src.Images[image] = i
		}
	}

	opts.Clusters = []Cluster{{Shareable: true, Image: image}}
	opts.Sources = append(append([]SymbolSource{src}, rest...), link68Opener(t, govax))

	return opts
}

// TestLinkProgramsAgainstShareableImages links the probe's programs
// against its shareable images, VMS's and govax's own, from real MACRO's
// objects and govax's, and checks each is real LINK's image byte for
// byte: the global section ISD for each image from its header (its name,
// _001, ident, match control, and its first section's pages, type, and
// attributes), the images in the order of their clusters (the options
// file's image, then the images it uses), a shareable image that uses
// another (MAIN2), and a based one (MAIN3).
func TestLinkProgramsAgainstShareableImages(t *testing.T) {
	for _, p := range link68Programs {
		want, opts := realImage(t, filepath.Join(link68Dir, "vax", p.image+".exe"))

		for _, images := range []string{"vms", platformName} {
			for _, objects := range []string{"real", platformName} {
				t.Run(p.image+"/"+images+"-images/"+objects+"-objects", func(t *testing.T) {
					opts := p.options(t, opts, images == platformName, govaxTables())

					m := link68Object(t, p.object)
					if objects == platformName {
						m = link68Source(t, p.object)
					}

					img, err := Link([]Input{{File: "DUA1:[000000]" + strings.ToUpper(p.object) + ".OBJ;1", Module: m}}, opts)
					if err != nil {
						t.Fatal(err)
					}

					if !bytes.Equal(img.Bytes, want) {
						t.Errorf("image differs from real LINK's:\n%s", diffBlocks(img.Bytes, want))
					}
				})
			}
		}
	}
}

// TestLinkBasedOverBasedImage checks the probe's E5: an executable based
// where the based SHR3 already is. Real LINK warned BASSHRBEL, then
// failed with CONFMEM for the default cluster, and made no image.
func TestLinkBasedOverBasedImage(t *testing.T) {
	_, opts := realImage(t, filepath.Join(link68Dir, "vax", "main3.exe"))
	opts = link68Program{uses: "shr3;1"}.options(t, opts, false, govaxTables())

	base := uint32(0x300000)
	opts.Base = &base

	_, err := Link([]Input{{File: "DUA1:[000000]MAIN3.OBJ;1", Module: link68Object(t, "main3")}}, opts)

	var fe *FatalError
	if !errors.As(err, &fe) {
		t.Fatalf("error %v, want a FatalError", err)
	}

	want := "%LINK-W-BASSHRBEL, shareable image DUA1:[000000]SHR3.EXE;1 has base of %X00300000\n" +
		"\twhich is below image base of %X00300000\n" +
		"%LINK-E-CONFMEM, conflicting virtual memory requirement\n" +
		"\tat %X00300000 for 2. pages in cluster DEFAULT_CLUSTER\n" +
		"%LINK-E-NOIMGFIL, image file not created"
	if fe.Error() != want || fe.Status() != 0x006482B2 {
		t.Errorf("error %q, status %X", fe.Error(), fe.Status())
	}
}
