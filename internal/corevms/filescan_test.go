package corevms

import "testing"

// TestScanFileSpec: where the parts of a file specification are, and where one
// ends (VSI's $FILESCAN page's rules).
func TestScanFileSpec(t *testing.T) {
	cases := []struct {
		text                                   string
		end                                    int
		node, device, root, directory, name    string
		kind, version, primary, access, second string
	}{
		{text: "", end: 0},
		{text: "A.B;1", end: 5, name: "A", kind: ".B", version: ";1"},
		{text: "A.B.1", end: 5, name: "A", kind: ".B", version: ".1"},
		{text: "DEV:[DIR]A.B;1 X", end: 14, device: "DEV:", directory: "[DIR]", name: "A", kind: ".B", version: ";1"},
		{text: "<DIR>A", end: 6, directory: "<DIR>", name: "A"},
		{text: "[ROOT.][DIR]A", end: 13, root: "[ROOT.]", directory: "[DIR]", name: "A"},
		{text: "[A.B", end: 0},
		{text: "NODE::A", end: 7, node: "NODE::", primary: "NODE", name: "A"},
		{text: `NODE"U P"::A`, end: 12, node: `NODE"U P"::`, primary: "NODE", access: `"U P"`, name: "A"},
		{text: "N1::N2::A", end: 9, node: "N1::N2::", primary: "N1", second: "N2::", name: "A"},
		{text: `NODE::"FILE SPEC"`, end: 17, node: "NODE::", primary: "NODE", name: `"FILE SPEC"`},
		{text: "A B", end: 1, name: "A"},
		{text: "*.*;*", end: 5, name: "*", kind: ".*", version: ";*"},
		{text: "A.B;-1", end: 6, name: "A", kind: ".B", version: ";-1"},
	}

	for _, c := range cases {
		p := ScanFileSpec(c.text)
		if p.End != c.end {
			t.Errorf("%q: end = %d, want %d", c.text, p.End, c.end)
		}

		for _, f := range []struct {
			name string
			part FileSpecPart
			want string
		}{
			{"node", p.Node, c.node}, {"device", p.Device, c.device}, {"root", p.Root, c.root},
			{"directory", p.Directory, c.directory}, {"name", p.Name, c.name}, {"type", p.Type, c.kind},
			{"version", p.Version, c.version}, {"primary", p.Primary, c.primary},
			{"access", p.Access, c.access}, {"secondary", p.Secondary, c.second},
		} {
			if got := c.text[f.part.Start:f.part.End]; got != f.want {
				t.Errorf("%q: %s = %q, want %q", c.text, f.name, got, f.want)
			}
		}
	}
}
