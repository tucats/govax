package isam

import (
	"fmt"
	"hash/crc32"
	"io"
	"strings"
)

// Dump writes a file's structure as text: the prolog's fields, then each
// key's buckets, level by level from the root, with what each holds. It
// shows only what the structures define and never a byte past a bucket's
// free offset, which VMS leaves as it was, so two files built alike dump
// alike however their unused bytes differ. That's the comparison the
// phase doc's answer 3 asks for: "the same records, IDs, RRVs, levels,
// pointers, split points, and allocations". A record's bytes show as a
// size and a checksum, to keep the lines short.
func (f *File) Dump(w io.Writer) error {
	fmt.Fprintf(w, "prolog %d, areas at VBN %d, data at VBN %d, EOF %d, global buffers %d\n",
		f.Version, f.AreaVBN, f.DataVBN, f.EOF, f.GlobalBufs)

	for _, a := range f.Areas {
		fmt.Fprintf(w, "area %d: bucket %d, extent %d+%d, used %d, next %d, avail %d, next extent %d+%d, deq %d, total %d, aln %d, aop %02X\n",
			a.ID, a.BucketSize, a.ExtentVBN, a.ExtentSize, a.Used, a.NextVBN, a.Available,
			a.NextExtent, a.NextExtentSize, a.ExtendSize, a.TotalAllocation, a.Alignment, a.Options)
	}

	for _, k := range f.Keys {
		dumpKey(w, k)

		if !k.HasIndex() {
			continue
		}

		if err := f.dumpLevels(w, k); err != nil {
			return err
		}
	}

	return nil
}

// dumpKey writes a key descriptor's fields.
func dumpKey(w io.Writer, k *Key) {
	var segments []string
	for _, s := range k.Segments {
		segments = append(segments, fmt.Sprintf("%d:%d", s.Position, s.Size))
	}

	fmt.Fprintf(w, "key %d: type %02X, flags %02X, size %d, segments %s, null %02X, min record %d, name %q\n",
		k.Ref, uint8(k.Type), k.Flags, k.Size, strings.Join(segments, ","), k.Null, k.MinRecord, k.Name)
	fmt.Fprintf(w, "  areas %d %d %d, buckets %d %d, fill %d %d, root %d at level %d, first data %d\n",
		k.IndexArea, k.Level1Area, k.DataArea, k.IndexBucketSize, k.DataBucketSize,
		k.IndexFill, k.DataFill, k.RootVBN, k.RootLevel, k.FirstData)
}

// dumpLevels writes each of a key's levels, top down. A level starts at
// the first pointer of the level above and follows the chain.
func (f *File) dumpLevels(w io.Writer, k *Key) error {
	start := k.RootVBN

	for level := k.RootLevel; level > 0; level-- {
		var first uint32

		for vbn := start; ; {
			b, err := f.readBucket(vbn, k.IndexBucketSize)
			if err != nil {
				return err
			}

			dumpBucket(w, b)

			list, err := b.indexEntries(k)
			if err != nil {
				return err
			}

			for _, e := range list {
				fmt.Fprintf(w, "    %04X key %X -> %d\n", e.offset, e.key, e.pointer)
			}

			if vbn == start && len(list) > 0 {
				first = list[0].pointer
			}

			if b.isLast() {
				break
			}

			vbn = b.next
		}

		start = first
	}

	for vbn := start; ; {
		z, err := f.readLevelZero(k, vbn)
		if err != nil {
			return err
		}

		dumpBucket(w, z.bucket)

		if err := dumpLevelZero(w, k, z); err != nil {
			return err
		}

		if z.bucket.isLast() {
			return nil
		}

		vbn = z.bucket.next
	}
}

// dumpBucket writes a bucket's header.
func dumpBucket(w io.Writer, b *bucket) {
	fmt.Fprintf(w, "  bucket %d: level %d, check %02X, key %d, free %04X, next ID %d, next %d, flags %02X\n",
		b.vbn, b.level, b.check, b.indexNo, b.free, b.nextID, b.next, b.control)
}

// dumpLevelZero writes a level 0 bucket's records.
func dumpLevelZero(w io.Writer, k *Key, z *levelZero) error {
	for i := range z.primary {
		r := &z.primary[i]

		if r.isRRV() {
			fmt.Fprintf(w, "    %04X %02X id %d: RRV to %d,%d\n", r.offset, r.control, r.id, r.rfa.VBN, r.rfa.ID)

			continue
		}

		rest, err := r.expand(k, z.bucket)
		if err != nil {
			return err
		}

		record := k.assemble(r.key, rest)
		fmt.Fprintf(w, "    %04X %02X id %d: rfa %d,%d key %X, size %d, sum %08X\n",
			r.offset, r.control, r.id, r.rfa.VBN, r.rfa.ID, r.key, len(record), crc32.ChecksumIEEE(record))
	}

	for i := range z.alts {
		s := &z.alts[i]
		fmt.Fprintf(w, "    %04X key %X:", s.offset, s.key)

		for _, p := range s.pointers {
			fmt.Fprintf(w, " %02X %d,%d", p.control, p.rfa.VBN, p.rfa.ID)
		}

		fmt.Fprintln(w)
	}

	return nil
}
