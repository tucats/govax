package isam

import (
	"bytes"
	"encoding/binary"
)

// RMS's two kinds of compression, the opposite direction of bucket.go's
// readKey and expand. Each must make what VMS makes, byte for byte, since
// how much a bucket holds, and so where it splits, follows from it. The
// rules here were fitted to every record of testdata/idx88's files
// (TestCompressionIsVMSs) and the local system disk's
// (TestRealIndexedFiles); the phase doc's "What the writer settled" has them.

// compressKey stores a key against the key before it in the bucket: a byte
// of how many bytes are stored, a byte of how many leading bytes it shares
// with the previous key (front compression), and the bytes that differ, the
// last standing for itself repeated to the key's end (rear compression).
// The first key in a bucket has no previous key, so nil.
func compressKey(key, previous []byte) []byte {
	front := 0
	for front < len(key) && front < len(previous) && key[front] == previous[front] {
		front++
	}

	// The stored part stops at the first byte of the run that ends the key,
	// but never before the front count: those bytes are spoken for already.
	end := len(key)
	for end > 1 && key[end-1] == key[end-2] {
		end--
	}

	if end < front {
		end = front
	}

	// A key that only differs in its run still has to store one byte of it,
	// so that expanding has a byte to repeat. A key the same as the last
	// stores nothing.
	if end == front && front < len(key) {
		end++
	}

	stored := make([]byte, 0, 2+end-front)
	stored = append(stored, byte(end-front), byte(front))

	return append(stored, key[front:end]...)
}

// The rules of a data record's compression. RMS looks for runs a longword
// at a time, counted from the start of the fragment it's building, and
// only then looks either way to see how long the run is. So a run of 6 is
// compressed or not according to where it falls: one that starts a byte
// past a longword's start holds no whole longword of the fragment.
const (
	runScanStep = 4   // the scan's stride
	shortestRun = 5   // a shorter run isn't worth a fragment's 3 bytes
	longestRun  = 256 // a run's first byte, and a byte's count of 255 more
)

// compressData turns a record's bytes after its key into fragments: each a
// word's length, that many bytes, and a byte of how many more times the
// last of them repeats. Bytes with no run to compress are one fragment
// with a repeat count of 0.
func compressData(rest []byte) []byte {
	out := make([]byte, 0, len(rest)+3)
	start := 0

	for at := 0; at+runScanStep <= len(rest); {
		from, to, found := runAt(rest, start, at)
		if !found {
			at += runScanStep

			continue
		}

		if to-from > longestRun {
			to = from + longestRun
		}

		out = appendFragment(out, rest[start:from+1], to-from-1)
		start = to
		at = to
	}

	if start < len(rest) {
		out = appendFragment(out, rest[start:], 0)
	}

	return out
}

// runAt looks at the longword at an offset. If its bytes are all alike and
// the run they're part of is long enough to compress, it returns the run's
// limits, going no further back than the fragment's start.
func runAt(rest []byte, start, at int) (int, int, bool) {
	c := rest[at]

	for i := 1; i < runScanStep; i++ {
		if rest[at+i] != c {
			return 0, 0, false
		}
	}

	from, to := at, at+runScanStep

	for from > start && rest[from-1] == c {
		from--
	}

	for to < len(rest) && rest[to] == c {
		to++
	}

	return from, to, to-from >= shortestRun
}

// appendFragment adds one fragment.
func appendFragment(out, literal []byte, repeat int) []byte {
	out = binary.LittleEndian.AppendUint16(out, uint16(len(literal)))
	out = append(out, literal...)

	return append(out, byte(repeat))
}

// recordRest is a record's bytes without its primary key's: the key is
// stored first in a data record whatever its position, and its segments'
// bytes aren't stored twice. assemble in key.go is the opposite.
func (k *Key) recordRest(record []byte) []byte {
	rest := make([]byte, 0, len(record))

	for at, b := range record {
		inKey := false

		for _, s := range k.Segments {
			if at >= s.Position && at < s.Position+s.Size {
				inKey = true

				break
			}
		}

		if !inKey {
			rest = append(rest, b)
		}
	}

	return rest
}

// CheckCompression compresses every record and key at a file's level 0
// again, each against the one before it, and reports the first that doesn't
// come out as stored, with the number it compared. A file RMS made always
// passes, so this is how the rules above are held to VMS's files.
func (f *File) CheckCompression() (int, error) {
	compared := 0

	for _, k := range f.Keys {
		if !k.HasIndex() {
			continue
		}

		for vbn := k.FirstData; ; {
			z, err := f.readLevelZero(k, vbn)
			if err != nil {
				return compared, err
			}

			n, err := f.checkBucket(k, z)
			compared += n

			if err != nil {
				return compared, err
			}

			if z.bucket.isLast() {
				break
			}

			vbn = z.bucket.next
		}
	}

	return compared, nil
}

// checkBucket does one level 0 bucket for CheckCompression.
func (f *File) checkBucket(k *Key, z *levelZero) (int, error) {
	b := z.bucket
	compared := 0

	var previous []byte

	compareKey := func(at int, key []byte) error {
		compared++

		if !bytes.Equal(compressKey(key, previous), b.data[at:at+2+int(b.data[at])]) {
			return b.corrupt(at, "a key isn't compressed as VMS compresses it")
		}

		return nil
	}

	for i := range z.primary {
		r := &z.primary[i]
		if r.isRRV() {
			continue
		}

		if k.Flags&FlagKeyCompress != 0 {
			at, _ := f.dataKeyPlace(b, k, r)
			if err := compareKey(at, r.key); err != nil {
				return compared, err
			}
		}

		previous = r.key

		if k.Flags&FlagRecordCompress != 0 && !r.isDeleted() {
			compared++

			rest, err := r.expand(k, b)
			if err != nil {
				return compared, err
			}

			if !bytes.Equal(compressData(rest), r.body) {
				return compared, b.corrupt(r.offset, "a record isn't compressed as VMS compresses it")
			}
		}
	}

	for i := range z.alts {
		s := &z.alts[i]

		if k.Flags&FlagKeyCompress != 0 {
			if err := compareKey(s.offset+2, s.key); err != nil {
				return compared, err
			}
		}

		previous = s.key
	}

	return compared, nil
}
