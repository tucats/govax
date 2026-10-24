package machine

import (
	"errors"
	"strings"
)

// A VMS V1 image's shareable images (docs/PHASE-72 - legacy images.md,
// subtask 4, Decision 4).
//
// An image linked by VMS V1's linker (format 02.01) has no fixup section
// and no shareable image list. It names the shareable images it uses
// only through global section ISDs, NAME_001, NAME_002, ..., at the
// addresses the linker mapped them at, and calls them at those absolute
// addresses (DUNGEON.EXE's VMSRTL_001 to VMSRTL_003, from ^X1A600). VMS
// 7.1 maps the image NAME whole at the first such ISD's address and
// passes over the image's other ISDs naming it (testdata/legacy72's
// answers 1 and 10): VMSRTL.EXE, now a jacket with a transfer vector
// and a fixup section of its own, goes at ^X1A600, so the old vector
// offsets land on its vector, and the images it uses follow it by Phase
// 68's rules (answer 2).

// legacyImageNames are the shareable images the global section ISDs of
// icb, a V1 image, name: each ISD's name without its "_nnn", in the
// order of their first ISDs.
func legacyImageNames(icb *ICB) []string {
	var names []string

	seen := map[string]bool{}

	for _, isd := range icb.ISDList {
		if isd.Flags&isdGBL == 0 {
			continue
		}

		name, ok := legacyImageName(isd.Name)
		if !ok || seen[name] {
			continue
		}

		seen[name] = true
		names = append(names, name)
	}

	return names
}

// legacyImageName is the shareable image a V1 global section named
// section maps: its name before a final "_" and digits, uppercased. It
// reports false for a name without that suffix.
func legacyImageName(section string) (string, bool) {
	at := strings.LastIndexByte(section, '_')
	if at <= 0 || at == len(section)-1 {
		return "", false
	}

	for _, c := range section[at+1:] {
		if c < '0' || c > '9' {
			return "", false
		}
	}

	return strings.ToUpper(section[:at]), true
}

// activateLegacyImages activates the shareable images icb, a V1 image,
// names through its global section ISDs, in the order of their first
// ISDs. Each goes at its first ISD's address: that ISD is given to
// imageBase as a based section, which a V1 section is, the linker having
// placed it. The ident it was linked with is checked as for any
// shareable image. An image govax's shims stand for has no file to map,
// and gets nothing (no V1 image on hand names one).
func (p *ImageProcess) activateLegacyImages(icb *ICB) error {
	for _, name := range legacyImageNames(icb) {
		linked := linkedSection(icb, name)

		placed := *linked
		placed.Flags |= isdBASED

		dep, err := p.loadImage(name, icbSecondary, &placed)
		if errors.Is(err, errEmulated) {
			continue
		}

		if err != nil {
			return err
		}

		if !identMatches(linked, dep) {
			return &IdentMismatchError{Image: dep.Name, File: dep.Spec.Name}
		}
	}

	return nil
}
