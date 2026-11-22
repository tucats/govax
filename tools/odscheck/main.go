// Command odscheck checks an ODS-2 container's structure from its raw blocks, the
// way VMS's ANALYZE/DISK_STRUCTURE does, without mounting it through govax. It was
// written for Phase 111's simh round (docs/planning/PHASE-111 - Fortran 90
// milestone.md, subtask 11), when VMS's file system crashed on a volume govax made,
// and may become the basis of an ANALYZE/DISK_STRUCTURE/REPAIR for govax.
//
//	cd tools/odscheck
//	GOWORK=off go run . [-free] [-dirs] [-blocks N] CONTAINER
//
// It is a module of its own, outside govax's build, since only internal/rms may
// import ods2 (CLAUDE.md); GOWORK=off keeps the repository's go.work from claiming it.
//
// What a volume holds, briefly, for a reader new to ODS-2: block 1 is the home block,
// which says where the index file, INDEXF.SYS, is. INDEXF.SYS holds the index file
// bitmap (a bit per file number, set when it's in use) and then one 512-byte file
// header per file number, file n's header at a fixed virtual block of INDEXF.SYS. A
// header names its file (the FID: a number, a sequence number bumped when the slot
// is reused, and a volume number), and its map area lists the file's extents
// (retrieval pointers: a count of blocks and a starting logical block). A file with
// too many extents for one header goes on in extension headers, each linked from the
// one before by its extension FID. INDEXF.SYS describes itself the same way, so its
// own extension headers must lie in blocks it already maps. BITMAP.SYS is the storage
// bitmap, a bit per cluster of blocks. A directory is a file of 512-byte blocks of
// records, each a name and its versions' FIDs, sorted by name, and ending in a 0xFFFF
// word; VMS requires directories to be contiguous.
//
// It checks: each header's checksum, file number, structure level, area offsets,
// and map; that no two files claim a block; INDEXF.SYS's extension chain and every
// other file's; back links to directories; each directory's contiguity, records,
// order, and the FIDs it names; and BITMAP.SYS against every header's map (ods2's
// volume.AnalyzeDisk). -free shows the free header slots INDEXF.SYS maps (VMS reads
// one when it makes a file), and -dirs each directory's header and how full each of
// its blocks is (a full block and no spare block means VMS must extend the directory
// to insert a name).
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"slices"
	"sort"

	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// checker holds the container being checked and what the checks have found.
type checker struct {
	disk     *os.File
	blocks   uint32 // the container's size in blocks
	cluster  uint32
	problems int
}

// block reads one logical block.
func (c *checker) block(lbn uint32) []byte {
	b := make([]byte, 512)
	if _, err := c.disk.ReadAt(b, int64(lbn)*512); err != nil {
		fmt.Fprintf(os.Stderr, "odscheck: reading LBN %d: %v\n", lbn, err)
		os.Exit(2)
	}

	return b
}

// bad reports a problem.
func (c *checker) bad(format string, args ...any) {
	c.problems++
	if c.problems <= 200 {
		fmt.Printf("PROBLEM: "+format+"\n", args...)
	}
}

// extents is a file's map, in virtual block order.
type extents []ondisk.Extent

// lbn is the logical block of virtual block vbn (1 is the first), if mapped.
func (e extents) lbn(vbn uint32) (uint32, bool) {
	v := vbn - 1
	for _, x := range e {
		if v < x.Count {
			return x.StartLBN + v, true
		}

		v -= x.Count
	}

	return 0, false
}

// size is how many blocks the map describes.
func (e extents) size() (n uint32) {
	for _, x := range e {
		n += x.Count
	}

	return n
}

func main() {
	showFree := flag.Bool("free", false, "show the free header slots INDEXF.SYS maps")
	showDirs := flag.Bool("dirs", false, "show each directory's header and its blocks' fill")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: odscheck [-free] [-dirs] CONTAINER")
		os.Exit(2)
	}

	disk, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "odscheck:", err)
		os.Exit(2)
	}

	info, err := disk.Stat()
	if err != nil {
		fmt.Fprintln(os.Stderr, "odscheck:", err)
		os.Exit(2)
	}

	c := &checker{disk: disk, blocks: uint32(info.Size() / 512)}
	c.check(*showFree, *showDirs)
	c.checkBitmap(flag.Arg(0))

	fmt.Printf("%d problems\n", c.problems)

	if c.problems > 0 {
		os.Exit(1)
	}
}

// check runs every check but the storage bitmap's.
func (c *checker) check(showFree, showDirs bool) {
	home, err := ondisk.DecodeHomeBlock(c.block(1))
	if err != nil {
		c.bad("home block: %v", err)

		return
	}

	c.cluster = uint32(home.ClusterSize)
	fmt.Printf("home block: %d blocks, cluster %d, index bitmap VBN %d LBN %d size %d, max files %d\n",
		c.blocks, home.ClusterSize, home.IndexBitmapVBN, home.IndexBitmapLBN, home.IndexBitmapSize, home.MaxFiles)

	// File n's header is at this virtual block of INDEXF.SYS.
	headerVBN := func(n uint32) uint32 {
		return uint32(home.IndexBitmapVBN) + uint32(home.IndexBitmapSize) + n - 1
	}

	index := c.indexMap(home, headerVBN)

	bitmap := make([]byte, 0, int(home.IndexBitmapSize)*512)
	for i := range uint32(home.IndexBitmapSize) {
		bitmap = append(bitmap, c.block(home.IndexBitmapLBN+i)...)
	}

	headers := c.readHeaders(home, index, headerVBN, bitmap, showFree)
	c.checkChains(headers)
	c.checkDirectories(headers, showDirs)
}

// indexMap reads INDEXF.SYS's primary header, which follows the index file bitmap,
// and each of its extension headers, found through the map read so far.
func (c *checker) indexMap(home ondisk.HomeBlock, headerVBN func(uint32) uint32) extents {
	primary, err := ondisk.DecodeFileHeader(c.block(home.IndexBitmapLBN + uint32(home.IndexBitmapSize)))
	if err != nil {
		c.bad("INDEXF.SYS's header: %v", err)

		return nil
	}

	pointers, err := primary.RetrievalPointers()
	if err != nil {
		c.bad("INDEXF.SYS's map: %v", err)
	}

	index := extents(pointers)
	describe("INDEXF.SYS", primary)

	for segment := primary; !segment.ExtensionFid.IsZero(); {
		n := segment.ExtensionFid.Number()

		lbn, ok := index.lbn(headerVBN(n))
		if !ok {
			c.bad("INDEXF.SYS's extension header %d, VBN %d, isn't in what comes before it", n, headerVBN(n))

			break
		}

		next, err := ondisk.DecodeFileHeader(c.block(lbn))
		if err != nil {
			c.bad("INDEXF.SYS's extension header %d: %v", n, err)

			break
		}

		more, _ := next.RetrievalPointers()
		index = append(index, more...)
		describe(fmt.Sprintf("INDEXF.SYS extension at LBN %d", lbn), next)

		segment = next
	}

	fmt.Printf("INDEXF.SYS maps %d blocks in %d extents\n", index.size(), len(index))

	return index
}

// readHeaders reads and checks every header the index file bitmap says is in use,
// and reports a block two files claim.
func (c *checker) readHeaders(home ondisk.HomeBlock, index extents, headerVBN func(uint32) uint32,
	bitmap []byte, showFree bool,
) map[uint32]ondisk.FileHeader {
	headers := map[uint32]ondisk.FileHeader{}
	owner := map[uint32]uint32{}
	clusterEnd := (c.blocks + c.cluster - 1) / c.cluster * c.cluster // a partial last cluster is whole

	for n := uint32(1); n <= home.MaxFiles; n++ {
		inUse := bitmap[(n-1)/8]&(1<<((n-1)%8)) != 0

		lbn, ok := index.lbn(headerVBN(n))
		if !ok {
			if inUse {
				c.bad("file %d is in use, but INDEXF.SYS doesn't reach its header, VBN %d", n, headerVBN(n))
			}

			continue
		}

		raw := c.block(lbn)

		if !inUse {
			if showFree {
				fmt.Printf("free slot %d: VBN %d, LBN %d, first bytes % x\n", n, headerVBN(n), lbn, raw[:16])
			}

			continue
		}

		if sum, _ := ondisk.Checksum(raw); binary.LittleEndian.Uint16(raw[510:]) != sum {
			c.bad("file %d (LBN %d): checksum %04x, computed %04x", n, lbn, binary.LittleEndian.Uint16(raw[510:]), sum)
		}

		h, err := ondisk.DecodeFileHeader(raw)
		if err != nil {
			c.bad("file %d: %v", n, err)

			continue
		}

		switch {
		case h.Fid.Number() != n:
			c.bad("file %d's header says it is file %d", n, h.Fid.Number())
		case h.StructureLevel != 0x0201:
			c.bad("file %d's structure level is %04x", n, h.StructureLevel)
		case h.IdentOffset > h.MapOffset || h.MapOffset > h.AclOffset || h.AclOffset > h.EndOffset:
			c.bad("file %d's areas are out of order: ident %d, map %d, ACL %d, end %d",
				n, h.IdentOffset, h.MapOffset, h.AclOffset, h.EndOffset)
		case int(h.MapOffset)+int(h.MapWordsInUse) > int(h.AclOffset):
			c.bad("file %d's %d map words run past its ACL area", n, h.MapWordsInUse)
		}

		headers[n] = h

		pointers, err := h.RetrievalPointers()
		if err != nil {
			c.bad("file %d's map: %v", n, err)
		}

		for _, x := range pointers {
			if x.StartLBN+x.Count > clusterEnd {
				c.bad("file %d's extent %d blocks at %d runs past the volume's end", n, x.Count, x.StartLBN)

				continue
			}

			for b := x.StartLBN; b < x.StartLBN+x.Count; b++ {
				if other, taken := owner[b]; taken {
					c.bad("LBN %d belongs to files %d and %d", b, other, n)

					break
				}

				owner[b] = n
			}
		}
	}

	fmt.Printf("%d headers in use\n", len(headers))

	return headers
}

// checkChains follows each file's extension headers, and checks each file's back
// link names a directory. Files 1 to 16 are the volume's own, linked to the MFD.
func (c *checker) checkChains(headers map[uint32]ondisk.FileHeader) {
	for n, h := range headers {
		if h.SegmentNumber != 0 {
			continue
		}

		for segment := h; !segment.ExtensionFid.IsZero(); {
			next, ok := headers[segment.ExtensionFid.Number()]
			if !ok {
				c.bad("file %d's extension header %v isn't in use", n, segment.ExtensionFid)

				break
			}

			if next.SegmentNumber != segment.SegmentNumber+1 {
				c.bad("file %d's extension header %d is segment %d after %d",
					n, segment.ExtensionFid.Number(), next.SegmentNumber, segment.SegmentNumber)
			}

			if next.Fid.Seq != segment.ExtensionFid.Seq {
				c.bad("file %d's extension header has sequence %d, its link %d", n, next.Fid.Seq, segment.ExtensionFid.Seq)
			}

			segment = next
		}

		if n > 16 || n == 4 {
			if parent, ok := headers[h.Backlink.Number()]; !ok || !parent.IsDirectory() {
				c.bad("file %d's back link %v isn't a directory", n, h.Backlink)
			}
		}
	}
}

// entryKey is a directory entry's place in VMS's order: by name, then versions
// highest first.
type entryKey struct {
	name    string
	version uint16
}

// checkDirectories checks each directory's contiguity, records, order, and FIDs.
func (c *checker) checkDirectories(headers map[uint32]ondisk.FileHeader, show bool) {
	numbers := make([]uint32, 0, len(headers))
	for n := range headers {
		numbers = append(numbers, n)
	}

	slices.Sort(numbers)

	count := 0

	for _, n := range numbers {
		h := headers[n]
		if !h.IsDirectory() || h.SegmentNumber != 0 {
			continue
		}

		count++

		pointers, _ := h.RetrievalPointers()
		dir := extents(pointers)

		if !h.ExtensionFid.IsZero() {
			c.bad("directory %d has an extension header", n)
		}

		for i := 1; i < len(dir); i++ {
			if dir[i].StartLBN != dir[i-1].StartLBN+dir[i-1].Count {
				c.bad("directory %d isn't contiguous: %v", n, dir)

				break
			}
		}

		if h.FileCharacteristics&0x80 == 0 { // FCH$M_CONTIG
			c.bad("directory %d isn't marked contiguous (characteristics %08x)", n, h.FileCharacteristics)
		}

		// The blocks in use: up to the end of file block, which with a first free
		// byte of 0 is the block after the last.
		used := h.RecordAttributes.EndOfFileBlock
		if h.RecordAttributes.FirstFreeByte == 0 && used > 0 {
			used--
		}

		if show {
			describe(fmt.Sprintf("directory %d", n), h)
			fmt.Printf("    %d blocks in use of %d allocated\n", used, dir.size())
		}

		var names []entryKey

		for vbn := uint32(1); vbn <= used; vbn++ {
			lbn, ok := dir.lbn(vbn)
			if !ok {
				c.bad("directory %d's end of file, block %d, is past its %d blocks", n, used, dir.size())

				break
			}

			raw := c.block(lbn)

			entries, err := ondisk.DecodeDirectoryBlock(raw)
			if err != nil {
				c.bad("directory %d, VBN %d: %v", n, vbn, err)

				continue
			}

			if len(entries) == 0 {
				c.bad("directory %d's VBN %d, within its end of file, is empty", n, vbn)
			}

			if show {
				fmt.Printf("    VBN %d: %d bytes in records, %d entries\n", vbn, recordBytes(raw), len(entries))
			}

			for _, d := range entries {
				names = append(names, entryKey{d.Name, d.Version})

				target, ok := headers[d.Fid.Number()]

				switch {
				case !ok:
					c.bad("directory %d: %s;%d names file %d, not in use", n, d.Name, d.Version, d.Fid.Number())
				case target.Fid.Seq != d.Fid.Seq:
					c.bad("directory %d: %s;%d has sequence %d, its header %d", n, d.Name, d.Version, d.Fid.Seq, target.Fid.Seq)
				}
			}
		}

		if !sort.SliceIsSorted(names, func(i, j int) bool {
			if names[i].name != names[j].name {
				return names[i].name < names[j].name
			}

			return names[i].version > names[j].version
		}) {
			c.bad("directory %d's entries are out of order", n)
		}
	}

	fmt.Printf("%d directories\n", count)
}

// recordBytes is how many bytes of a directory block its records use, up to the
// 0xFFFF word that ends them (or the block's end). Each record starts with its
// length in bytes, less the length word itself.
func recordBytes(raw []byte) int {
	at := 0
	for at+2 <= len(raw) {
		n := binary.LittleEndian.Uint16(raw[at:])
		if n == 0xFFFF {
			break
		}

		at += int(n) + 2
	}

	return at
}

// checkBitmap holds BITMAP.SYS to every header's map, by ods2's own analysis.
func (c *checker) checkBitmap(path string) {
	container, err := diskimage.Open(path)
	if err != nil {
		c.bad("opening for ods2: %v", err)

		return
	}

	vol, err := volume.Mount(container)
	if err != nil {
		c.bad("ods2 can't mount it: %v", err)

		return
	}

	report, err := volume.AnalyzeDisk(vol.Devices[0])
	if err != nil {
		c.bad("the storage bitmap: %v", err)

		return
	}

	for _, d := range report.Discrepancies {
		c.bad("the storage bitmap: %s", d)
	}
}

// describe shows a header's fields, for comparing one volume's with another's.
func describe(what string, h ondisk.FileHeader) {
	fmt.Printf("%s: FID %v, extension %v, segment %d, areas ident %d map %d ACL %d end %d, %d map words,\n"+
		"    characteristics %08x, back link %v, high water %d, owner %v, protection %04x, record protection %04x\n",
		what, h.Fid, h.ExtensionFid, h.SegmentNumber, h.IdentOffset, h.MapOffset, h.AclOffset, h.EndOffset,
		h.MapWordsInUse, h.FileCharacteristics, h.Backlink, h.HighWaterMark, h.Owner, h.FileProtection,
		h.RecordProtection)

	ra := h.RecordAttributes
	fmt.Printf("    end of file block %d, first free byte %d, highest block %d\n",
		ra.EndOfFileBlock, ra.FirstFreeByte, ra.HighestBlock)
}
