package lbrrtl

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The NAM blocks LBR$INI_CONTROL and LBR$OPEN take (subtask 11d). As the
// manual says, LBR$OPEN fills in a NAM block given to LBR$INI_CONTROL, and
// a later LBR$OPEN with a NAM holding a file ID opens the file by it
// (RMS's open by NAM block), whatever name it's given. What VMS 7.1's did
// (testdata/help90's round 3, case 35):
//
//   - the NAM's resultant length (NAM$B_RSL) and file ID (NAM$W_FID) were
//     set, but not the resultant string in the NAM's own buffer
//     (NAM$L_RSA's bytes stayed as they were);
//   - a later INI_CONTROL with the same NAM opened the file by its ID with
//     no name at all, and with the name of a file that wasn't there;
//   - that NAM as LBR$OPEN's related name (rlfna) gave the name and type of
//     the file it was filled in for.
//
// So VMS's LBRSHR kept the name somewhere of its own; govax keeps it in Go,
// by the NAM's address, for the image's life (namFill). A NAM govax didn't
// fill in is a related name by its own resultant (or expanded) string, as
// RMS reads one. What else VMS writes into the NAM (the directory ID, the
// device) isn't known, and govax writes only what case 35 showed.

// NAM block offsets.
var (
	namRSL = vmsdef.Symbols["NAM$B_RSL"]
	namRSA = vmsdef.Symbols["NAM$L_RSA"]
	namESL = vmsdef.Symbols["NAM$B_ESL"]
	namESA = vmsdef.Symbols["NAM$L_ESA"]
	namFID = vmsdef.Symbols["NAM$W_FID"]
)

// namFill is what LBR$OPEN put in a NAM block: the file it opened, its
// resultant name, and the file ID it wrote.
type namFill struct {
	loc  rms.FileLocation
	name string
	fid  [3]uint16
}

// readFID reads a NAM block's file ID.
func readFID(env *corevms.Environment, nam uint32) ([3]uint16, error) {
	var fid [3]uint16

	for i := range fid {
		w, err := env.Memory().LoadWord(env.CPU(), nam+namFID+uint32(2*i))
		if err != nil {
			return fid, err
		}

		fid[i] = w
	}

	return fid, nil
}

// byNAM returns the file the NAM at nam names by its file ID, if LBR$OPEN
// filled it in and its file ID is still the one written.
func byNAM(env *corevms.Environment, nam uint32) (rms.FileLocation, bool, error) {
	if nam == 0 {
		return rms.FileLocation{}, false, nil
	}

	fid, err := readFID(env, nam)
	if err != nil || fid == [3]uint16{} {
		return rms.FileLocation{}, false, err
	}

	f, ok := stateOf(env).nams[nam]
	if !ok || f.fid != fid {
		return rms.FileLocation{}, false, nil
	}

	return f.loc, true, nil
}

// fillNAM writes the file LBR$OPEN opened into the NAM at nam: its
// resultant length and its file ID (none for a host file).
func fillNAM(env *corevms.Environment, nam uint32, loc rms.FileLocation) error {
	if nam == 0 {
		return nil
	}

	f := namFill{loc: loc, name: loc.Name}

	if env.Session != nil && !loc.Host {
		if attrs, err := env.Session.FileAttributes(loc.Name); err == nil {
			_, _ = fmt.Sscanf(attrs.FID, "(%d,%d,%d)", &f.fid[0], &f.fid[1], &f.fid[2])
		}
	}

	mem, cpu := env.Memory(), env.CPU()

	if err := mem.StoreByte(cpu, nam+namRSL, byte(min(len(f.name), 255))); err != nil {
		return err
	}

	for i, w := range f.fid {
		if err := mem.StoreWord(cpu, nam+namFID+uint32(2*i), w); err != nil {
			return err
		}
	}

	stateOf(env).nams[nam] = f

	return nil
}

// relatedName is the name, type, and version a related NAM block gives
// (the manual: "only the file name, type, and version fields of the NAM
// block are used"): from the name LBR$OPEN filled it in with, or from its
// resultant string, or its expanded string without one.
func relatedName(env *corevms.Environment, nam uint32) (string, error) {
	if nam == 0 {
		return "", nil
	}

	if f, ok := stateOf(env).nams[nam]; ok {
		return fileOnly(f.name), nil
	}

	mem, cpu := env.Memory(), env.CPU()

	for _, at := range [][2]uint32{{namRSL, namRSA}, {namESL, namESA}} {
		n, err := mem.LoadByte(cpu, nam+at[0])
		if err != nil {
			return "", err
		}

		if n == 0 {
			continue
		}

		addr, err := mem.LoadLongword(cpu, nam+at[1])
		if err != nil {
			return "", err
		}

		b := make([]byte, n)
		for i := range b {
			if b[i], err = mem.LoadByte(cpu, addr+uint32(i)); err != nil {
				return "", err
			}
		}

		return fileOnly(string(b)), nil
	}

	return "", nil
}

// fileOnly is a file specification's name, type, and version: what follows
// its device and directory.
func fileOnly(spec string) string {
	if i := strings.LastIndexAny(spec, "]>:"); i >= 0 {
		return spec[i+1:]
	}

	return spec
}

// withRelated is the default specification dns with the related name's
// name and type for the fields dns doesn't have (RMS's order: the file
// specification, then the default, then the related name). A related
// version isn't used (unconfirmed).
func withRelated(dns, related string) string {
	if related == "" {
		return dns
	}

	prefix, file := dns[:len(dns)-len(fileOnly(dns))], fileOnly(dns)
	if i := strings.IndexByte(file, ';'); i >= 0 {
		file = file[:i]
	}

	name, typ, _ := strings.Cut(file, ".")

	rel := related
	if i := strings.IndexByte(rel, ';'); i >= 0 {
		rel = rel[:i]
	}

	relName, relType, _ := strings.Cut(rel, ".")

	if name == "" {
		name = relName
	}

	if typ == "" {
		typ = relType
	}

	if typ == "" {
		return prefix + name
	}

	return prefix + name + "." + typ
}
