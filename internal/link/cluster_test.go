package link

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmsimage"
)

// Two modules for the cluster tests: MAIN calls SUB, and each has a
// local psect named DATA and a global one named COMMON.
const (
	clusterMain = "\t.TITLE\tMAIN\n\t.PSECT\tDATA,WRT,NOEXE\n\t.LONG\t1\n" +
		"\t.PSECT\tCOMMON,GBL,WRT,NOEXE\n\t.LONG\t2\n" +
		"\t.PSECT\tCODE,NOWRT,EXE\n\t.ENTRY\tSTART,^M<>\n\tCALLS\t#0,SUB\n\tRET\n\t.END\tSTART\n"
	clusterSub = "\t.TITLE\tSUB\n\t.PSECT\tDATA,WRT,NOEXE\n\t.LONG\t3\n" +
		"\t.PSECT\tCOMMON,GBL,WRT,NOEXE\n\t.LONG\t4\n" +
		"\t.PSECT\tSUBCODE,NOWRT,EXE\n\t.ENTRY\tSUB,^M<>\n\tRET\n\t.END\n"
)

// linkClusters links MAIN in the default cluster and SUB in cluster
// SUBS, with opts, and returns the image and its decoded header.
func linkClusters(t *testing.T, opts Options) (*Image, *vmsimage.Image) {
	t.Helper()

	img, err := Link([]Input{
		{File: "MAIN.OBJ", Module: macroModule(t, clusterMain)},
		{File: "SUB.OBJ", Module: macroModule(t, clusterSub), Cluster: "SUBS"},
	}, opts)
	if err != nil {
		t.Fatal(err)
	}

	hdr, err := vmsimage.ReadImage(img.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	return img, hdr
}

// psectsNamed returns the image's psects named name.
func psectsNamed(img *Image, name string) []PsectInfo {
	var found []PsectInfo

	for _, p := range img.Psects {
		if p.Name == name {
			found = append(found, p)
		}
	}

	return found
}

// TestClusterPsects checks where the modules' psects go when SUB is in a
// cluster of its own, which has no base: the cluster comes before the
// default cluster in the list, so SUB is read first and placed at the
// image's start, and the default cluster after it. Each cluster has its
// own local DATA; the global COMMON is the first cluster's, which both
// modules contribute to.
func TestClusterPsects(t *testing.T) {
	img, hdr := linkClusters(t, Options{Clusters: []Cluster{{Name: "SUBS"}}})

	if got := img.l.modules[0].name; got != "SUB" {
		t.Errorf("first module %s, want SUB", got)
	}

	data := psectsNamed(img, "DATA")
	if len(data) != 2 {
		t.Fatalf("%d psects named DATA, want 2: %+v", len(data), img.Psects)
	}

	common := psectsNamed(img, "COMMON")
	if len(common) != 1 || common[0].Length != 8 {
		t.Errorf("COMMON is %+v, want one psect of 8 bytes", common)
	}

	// SUBS: COMMON and DATA (WRT NOEXE) at ^X200, SUBCODE at ^X400; the
	// default cluster from ^X600.
	if p := psectsNamed(img, "SUBCODE"); len(p) != 1 || p[0].Base != 0x400 {
		t.Errorf("SUBCODE is %+v, want at ^X400", p)
	}

	if p := psectsNamed(img, "CODE"); len(p) != 1 || p[0].Base != 0x800 {
		t.Errorf("CODE is %+v, want at ^X800", p)
	}

	// Without a base anywhere, the image is position independent, and the
	// default cluster, at the highest address, is the last.
	if hdr.LinkFlags&ihdPICIMG == 0 {
		t.Error("PICIMG is clear")
	}

	for _, d := range hdr.ISDs {
		last := d.Flags&isdLASTCLU != 0
		want := d.Address() >= 0x600 && d.Flags&isdFIXUPVEC == 0

		if last != want {
			t.Errorf("ISD at %08X: LASTCLU %v, want %v", d.Address(), last, want)
		}
	}
}

// TestCollectAfterDefault checks a COLLECT= of a cluster nothing else
// makes, which follows the default cluster in the list: the default
// cluster's module defines the psect first, so it stays there (the
// Linker manual's warning, 6.3.2), though it's made global. A COLLECT=
// into a cluster before the module's moves it.
func TestCollectAfterDefault(t *testing.T) {
	img, _ := linkClusters(t, Options{
		Clusters: []Cluster{{Name: "SUBS"}},
		Collects: []Collect{{Cluster: "LATER", Psects: []string{"CODE"}}},
	})

	if code := psectsNamed(img, "CODE"); len(code) != 1 || code[0].Flags&obj.PsectGBL == 0 || code[0].Base != 0x800 {
		t.Errorf("CODE is %+v, want global, at ^X800", code)
	}

	for _, p := range img.l.order {
		if p.name == "CODE" && p.cluster.name != defaultClusterName {
			t.Errorf("CODE is in cluster %s", p.cluster.name)
		}
	}

	img, _ = linkClusters(t, Options{
		Clusters: []Cluster{{Name: "SUBS"}},
		Collects: []Collect{{Cluster: "SUBS", Psects: []string{"CODE"}}},
	})

	for _, p := range img.l.order {
		if p.name == "CODE" && p.cluster.name != "SUBS" {
			t.Errorf("CODE is in cluster %s, want SUBS", p.cluster.name)
		}
	}
}

// TestClusterSectionFlags checks what a cluster gives its image sections:
// CLUSTER='s page fault cluster and PROTECT=YES's ISD$V_PROTECT, on its
// sections only, and IOSEGMENT=n,NOP0BUFS in the header.
func TestClusterSectionFlags(t *testing.T) {
	_, hdr := linkClusters(t, Options{
		Clusters:  []Cluster{{Name: "SUBS", PFC: 8, Protect: true}},
		IOSegment: &IOSegment{Pages: 12},
	})

	if hdr.IOPages != 12 || hdr.LinkFlags&ihdNOP0BUFS == 0 {
		t.Errorf("I/O pages %d, link flags %08X", hdr.IOPages, hdr.LinkFlags)
	}

	for _, d := range hdr.ISDs {
		sub := d.Address() < 0x600

		if got := d.Flags&isdPROTECT != 0; got != sub {
			t.Errorf("ISD at %08X: PROTECT %v", d.Address(), got)
		}

		if want := map[bool]byte{true: 8, false: 0}[sub]; d.PFC != want {
			t.Errorf("ISD at %08X: pfc %d, want %d", d.Address(), d.PFC, want)
		}
	}
}

// TestConflictingClusters checks that two clusters with overlapping bases
// are real LINK's CONFMEM, for the second in the list, and no image.
func TestConflictingClusters(t *testing.T) {
	base := uint32(0x10000)

	_, err := Link([]Input{
		{File: "MAIN.OBJ", Module: macroModule(t, clusterMain)},
		{File: "SUB.OBJ", Module: macroModule(t, clusterSub), Cluster: "SUBS"},
	}, Options{Base: &base, Clusters: []Cluster{{Name: "SUBS", Base: &base}}})

	var fe *FatalError
	if !errors.As(err, &fe) {
		t.Fatalf("err = %v, want a FatalError", err)
	}

	want := "%LINK-E-CONFMEM, conflicting virtual memory requirement\n\tat %X00010000 for 2. pages in cluster DEFAULT_CLUSTER\n" +
		"%LINK-E-NOIMGFIL, image file not created"
	if fe.Error() != want || fe.Status() != 0x006482B2 {
		t.Errorf("error %q, status %X", fe.Error(), fe.Status())
	}
}

// TestISDMax checks that demand-zero compression stops at ISD_MAX=: a
// section with an untouched run in its middle splits into three image
// sections only while the limit allows two more.
func TestISDMax(t *testing.T) {
	src := "\t.PSECT\tDATA,WRT,NOEXE,PAGE\n\t.LONG\t1\n\t.BLKB\t^X1000\n\t.LONG\t2\n" +
		"\t.PSECT\tCODE,NOWRT,EXE\n\t.ENTRY\tSTART,^M<>\n\tRET\n\t.END\tSTART\n"

	sections := func(limit uint32) int {
		img, err := Link([]Input{{File: "A.OBJ", Module: macroModule(t, src)}}, Options{ISDMax: &limit})
		if err != nil {
			t.Fatal(err)
		}

		hdr, err := vmsimage.ReadImage(img.Bytes)
		if err != nil {
			t.Fatal(err)
		}

		return len(hdr.ISDs)
	}

	// DATA, CODE, the fixup section, and the stack are 4; splitting DATA
	// makes 6.
	if n := sections(96); n != 6 {
		t.Errorf("%d image sections with ISD_MAX=96, want 6", n)
	}

	if n := sections(5); n != 4 {
		t.Errorf("%d image sections with ISD_MAX=5, want 4", n)
	}
}

// TestImageOrder checks the shareable image list's order: an image an
// options file names comes before one found in a library, whichever the
// code refers to first (the probe's MAIN1: SHR1, then LIBRTL).
func TestImageOrder(t *testing.T) {
	src := "\t.PSECT\tCODE,NOWRT,EXE\n\t.ENTRY\tSTART,^M<>\n" +
		"\tCALLS\t#0,G^LIB$PUT_OUTPUT\n\tCALLS\t#0,G^MY_ROUTINE\n\tRET\n\t.END\tSTART\n"

	mine := &TableSource{
		Symbols: map[string]Definition{"MY_ROUTINE": {Image: "MYSHR", Value: 8}},
		Images:  map[string]SharedImage{"MYSHR": {Pages: 1, MajorID: 1, MinorID: 1, Match: MatchLEQ}},
	}

	img, err := Link([]Input{{File: "A.OBJ", Module: macroModule(t, src)}}, Options{
		Sources:  []SymbolSource{mine, librtl},
		Clusters: []Cluster{{Shareable: true, Image: "MYSHR"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	hdr, err := vmsimage.ReadImage(img.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	if got := hdr.Fixups.Shared; len(got) != 3 || got[1] != "MYSHR" || got[2] != "LIBRTL" {
		t.Errorf("shareable image list %q, want this image, MYSHR, LIBRTL", got)
	}
}
