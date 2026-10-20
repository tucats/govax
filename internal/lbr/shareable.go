package lbr

import (
	"encoding/binary"
	"fmt"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmsimage"
)

// This file is LIBRARY/SHARE's input: a shareable image as a module of a
// shareable image symbol table library (docs/PHASE-68 - link shareable
// images.md, subtask 12). VMS 7.1's librarian (testdata/link68's
// MYSHR.OLB, made by LIBRARY/CREATE/SHARE MYSHR SHR1.EXE,SHR2.EXE) keeps
// little of the image:
//
//   - the module is named for the image, and holds two records of its
//     global symbol table: its module header and its end of module
//     record. The symbol table's other records (its LNM, the psect, and
//     the universal symbols with their values) aren't kept: a link that
//     finds a symbol in the library reads the image itself for them.
//   - index 2 has the image's universal symbols, each naming the module.
//   - the module header's user data holds 0, then 4, then the image's
//     global section ident (IHD$L_IDENT, major ident in the top byte):
//     SHR1's is 00 04 E8 03 00 01, which LIBRARY/LIST/FULL shows as
//     "Ident 01,0003E8". The match control isn't kept. What the 4 is
//     (the ident's length, as an object module's is the length of its
//     text) is unconfirmed; every module has it.

// shareableIdentLength is the second byte of a shareable image module's
// user data.
const shareableIdentLength = 4

// ShareableModule reads a shareable image file's bytes as LIBRARY/SHARE
// does: the module of a shareable image library that stands for it.
func (b *Builder) ShareableModule(image []byte) (*Entry, error) {
	if b.Type != TypeShareable {
		return nil, fmt.Errorf("lbr: shareable images go only in a shareable image library")
	}

	img, err := vmsimage.ReadImage(image)
	if err != nil {
		return nil, err
	}

	if img.Type != vmsimage.TypeShareable {
		return nil, fmt.Errorf("not a shareable image")
	}

	if len(img.GST) == 0 {
		return nil, fmt.Errorf("shareable image %s has no global symbol table", img.Name)
	}

	m, err := obj.Decode(img.GST)
	if err != nil {
		return nil, fmt.Errorf("shareable image %s's global symbol table: %w", img.Name, err)
	}

	name := m.Name()
	if name == "" || len(name) > b.KeySize {
		return nil, fmt.Errorf("image name %q isn't 1 to %d characters", name, b.KeySize)
	}

	// The table's first record is its module header, and its last its
	// end of module record.
	first, last := img.GST[0], img.GST[len(img.GST)-1]
	if obj.RecordType(first[0]) != obj.RecHDR {
		return nil, fmt.Errorf("shareable image %s's global symbol table doesn't start with a module header", name)
	}

	if t := obj.RecordType(last[0]); t != obj.RecEOM && t != obj.RecEOMW {
		return nil, fmt.Errorf("shareable image %s's global symbol table doesn't end with an end of module record", name)
	}

	e := &Entry{Name: name, Records: [][]byte{first, last}, UserData: make([]byte, b.userSize)}

	if b.userSize >= 2+4 {
		e.UserData[1] = shareableIdentLength
		binary.LittleEndian.PutUint32(e.UserData[2:], img.Ident)
	}

	seen := map[string]bool{}

	for _, s := range m.Symbols() {
		if !indexedSymbol(s) || seen[s.Name] {
			continue
		}

		if s.Name == "" || len(s.Name) > b.KeySize {
			return nil, fmt.Errorf("image %s: symbol name %q isn't 1 to %d characters", name, s.Name, b.KeySize)
		}

		seen[s.Name] = true
		e.Symbols = append(e.Symbols, s.Name)
	}

	return e, nil
}
