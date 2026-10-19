package machine

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vm"
)

// How an image's sections are laid out in memory, for the debugger's
// SHOW IMAGE/FULL: one ImageSection per image section descriptor (ISD),
// with what its pages are mapped to now. With paging on, an image is
// mapped from its file (imagemap.go), so its pages move between forms as
// the program runs: not yet read, valid in the working set, in
// transition on the free or modified list, or written to the page file.
// With paging off, an image is copied, and every page is simply in
// memory.

// SectionKind is what an ISD describes.
type SectionKind uint8

const (
	// SectionReadOnly: code or read-only data, read from the image file.
	SectionReadOnly SectionKind = iota

	// SectionWritable: writable data, copy-on-reference: each page is
	// read from the image file once, then the process's own copy, backed
	// by the page file.
	SectionWritable

	// SectionFixup: the fixup section (G^ cells and the fixup lists), a
	// writable copy-on-reference section.
	SectionFixup

	// SectionDemandZero: pages that start as zeros, backed by the page
	// file once written.
	SectionDemandZero

	// SectionUserStack: the user stack the image asks for; govax's user
	// stack is the process's own, not the image's (not mapped).
	SectionUserStack

	// SectionGlobal: a reference to a shareable image's global section
	// (not mapped here: the shareable image is listed as an image of its
	// own, or is one of govax's built-in libraries).
	SectionGlobal
)

func (k SectionKind) String() string {
	switch k {
	case SectionReadOnly:
		return "read-only"
	case SectionWritable:
		return "writable"
	case SectionFixup:
		return "fixup"
	case SectionDemandZero:
		return "demand zero"
	case SectionUserStack:
		return "user stack"
	case SectionGlobal:
		return "global"
	}

	return "unknown"
}

// ImageSection is one of an image's sections, as SHOW IMAGE/FULL shows
// it.
type ImageSection struct {
	// Kind is what the ISD describes, and Name its global section name
	// (for SectionGlobal).
	Kind SectionKind
	Name string

	// First and Last are the addresses of its first and last bytes, and
	// Pages its size in pages. For SectionUserStack and SectionGlobal
	// only Pages means anything.
	First, Last uint32
	Pages       int

	// VBN is the image file block its first page comes from (none for a
	// demand-zero section).
	VBN uint32

	// Protection is the protection of its first page now.
	Protection vm.Protection

	// Section is the private section its pages are mapped to, when the
	// image was mapped and the section is still there (InSection).
	Section   corevms.SectionInfo
	InSection bool

	// Forms counts its pages by the form their PTEs are in now.
	Forms map[corevms.PageForm]int

	// Backing counts the pages in memory (valid or in transition) by
	// where their frames' contents go back to: the image file, the page
	// file, or nowhere yet (a demand-zero page never written out). With
	// paging off it's empty: a page never leaves memory.
	Backing map[paging.BackingKind]int

	// OnModified counts the pages in transition whose frames are on the
	// modified list (written to their backing store before reuse); the
	// rest of the pages in transition are on the free list.
	OnModified int
}

// ImageSections returns img's sections in the order its header lists
// them, with each one's pages as they're mapped now.
func (m *Machine) ImageSections(img ImageInfo) []ImageSection {
	icb := img.icb
	if icb == nil {
		return nil
	}

	env := m.images().process()

	out := make([]ImageSection, 0, len(icb.ISDList))

	for _, isd := range icb.ISDList {
		s := ImageSection{Pages: int(isd.Pages), VBN: isd.VBN}

		switch {
		case isdType(isd.Flags) == isdUsrStack:
			s.Kind = SectionUserStack
		case isd.Flags&isdGBL != 0:
			s.Kind, s.Name = SectionGlobal, isd.Name
		case isd.Flags&isdDZRO != 0:
			s.Kind, s.VBN = SectionDemandZero, 0
		case isd.Flags&isdFIXUPVEC != 0:
			s.Kind = SectionFixup
		case isd.Flags&isdWRT != 0:
			s.Kind = SectionWritable
		default:
			s.Kind = SectionReadOnly
		}

		if s.Kind == SectionUserStack || s.Kind == SectionGlobal || env == nil || s.Pages == 0 {
			out = append(out, s)

			continue
		}

		s.First = icb.Base + isd.VPN<<9
		s.Last = s.First + uint32(s.Pages)*512 - 1
		s.Forms = map[corevms.PageForm]int{}
		s.Backing = map[paging.BackingKind]int{}

		if icb.Mapped && s.Kind != SectionDemandZero {
			s.Section, s.InSection = env.SectionAt(s.First)
		}

		for n := range uint32(s.Pages) {
			info, err := env.PageInfoAt(s.First + n*512)
			if err != nil {
				continue
			}

			if n == 0 {
				s.Protection = info.PTE.Protection()
			}

			s.Forms[info.Form]++

			if info.HasBacking {
				s.Backing[info.Backing.Kind]++
			}

			if info.Form == corevms.PageTransition && info.List == paging.Modified {
				s.OnModified++
			}
		}

		out = append(out, s)
	}

	return out
}
