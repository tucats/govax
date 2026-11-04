package isam

import (
	"encoding/binary"
	"fmt"
)

// An Inspector is ANALYZE/RMS_FILE's reading of a file (subtask 10d). Where the
// rest of the package stops at the first thing wrong, an Inspector reports it and
// goes on as far as it can, because describing a damaged file is what ANALYZE is
// for. It reads the prolog whatever its sums say, walks each key's tree a level at
// a time, and counts what the statistics need as it goes.
//
// Everything here comes from VMS 7.1's reports (testdata/idx88's rounds 1 and 3):
// the order of the checks, which findings end the walk, and each statistic's
// formula. A finding's Status is the one VMS's $STATUS showed when it was first in
// a report. A status no report showed is inspectUnseen, and the phase doc lists them.

// The findings' status values, as $STATUS showed them.
const (
	statusCheckByte  = 0x103B877A
	statusLevel      = 0x103B8792
	statusSum        = 0x103B87B2
	statusKeySpill   = 0x103B87D2
	statusRecordFit  = 0x103B8812
	statusSIDR       = 0x103B887A
	statusChainLoop  = 0x103B88B2
	statusAreaVBN    = 0x103B8762
	statusRecordID   = 0x103B95F2
	statusNotCounted = 0 // a reserved bit's finding counts as an error, but sets no status

	// inspectUnseen stands for the findings no report's first error was. It's
	// govax's choice, unconfirmed: an error of the same facility.
	inspectUnseen = statusLevel
)

// Finding is one thing wrong with a file: a line of ANALYZE's report, its "***"
// line. VBN is 0 for a read that failed, whose text has its own VBN.
type Finding struct {
	VBN    uint32
	Text   string
	Status uint32
}

// FixedProlog is what VBN 1 says of the whole file. An indexed file's and a
// relative file's prologs have the same layout.
type FixedProlog struct {
	AreaCount int
	AreaVBN   uint32
	DataVBN   uint32
	MaxRecord uint32
	EOF       uint32
	Version   int
	NoExtend  bool
}

// ReadFixedProlog reads VBN 1's file-level fields.
func ReadFixedProlog(blocks Blocks) FixedProlog {
	first := make([]byte, BlockSize)
	if err := blocks.ReadBlock(1, first); err != nil {
		return FixedProlog{}
	}

	return FixedProlog{
		AreaCount: int(first[plgAreaCount]),
		AreaVBN:   uint32(first[plgAreaVBN]),
		DataVBN:   uint32(binary.LittleEndian.Uint16(first[plgDataVBN:])),
		MaxRecord: binary.LittleEndian.Uint32(first[plgMaxRecord:]),
		EOF:       binary.LittleEndian.Uint32(first[plgEOF:]),
		Version:   int(binary.LittleEndian.Uint16(first[plgVersion:])),
		NoExtend:  first[plgFlags]&1 != 0,
	}
}

// plgFlags is PLG$B_FLAGS, whose bit 0 is PLG$V_NOEXTEND.
const plgFlags = 0x10

// An area's AREA$B_AOP bits for FDL's AREA BEST_TRY_CONTIGUOUS and CONTIGUOUS, the
// same as XAB$M_CBT and XAB$M_CTG.
const (
	AreaBestTry    = 0x20
	AreaContiguous = 0x80
)

// Inspector walks one file, telling Report each finding as it comes. Stopped says a
// finding ended the walk: ANALYZE's "Unrecoverable error".
type Inspector struct {
	Report  func(Finding)
	Stopped bool
	Prolog  FixedProlog
	Keys    []*Key
	Areas   []Area

	blocks    Blocks
	attr      Attributes
	allocated uint32
	file      *File // for the decoders, which need only attr
}

// NewInspector starts on a file of allocated blocks.
func NewInspector(blocks Blocks, attr Attributes, allocated uint32, report func(Finding)) *Inspector {
	return &Inspector{
		Report:    report,
		Prolog:    ReadFixedProlog(blocks),
		blocks:    blocks,
		attr:      attr,
		allocated: allocated,
		file:      &File{blocks: blocks, attr: attr},
	}
}

func (in *Inspector) find(vbn uint32, status uint32, text string) {
	in.Report(Finding{VBN: vbn, Text: text, Status: status})
}

// CheckSums checks the prolog blocks' sums, each of the key descriptors' blocks in
// their chain's order and then the areas'. It's the first thing ANALYZE does: its
// findings come before the report's FIXED PROLOG. It also reads the descriptors.
func (in *Inspector) CheckSums() {
	seen := map[uint32]bool{}

	check := func(vbn uint32) []byte {
		block := make([]byte, BlockSize)
		if err := in.blocks.ReadBlock(vbn, block); err != nil {
			return nil
		}

		if !seen[vbn] {
			seen[vbn] = true

			if !blockSumRight(block) {
				in.find(vbn, statusSum, "Prolog block checksum is invalid.")
			}
		}

		return block
	}

	if in.Prolog.Version != 3 {
		return
	}

	vbn, offset := uint32(1), 0

	for vbn != 0 && offset+keyDescSize <= BlockSize && len(in.Keys) < 256 {
		block := check(vbn)
		if block == nil {
			break
		}

		k := parseKey(block[offset : offset+keyDescSize])
		k.DescriptorVBN, k.DescriptorOffset = vbn, offset
		in.Keys = append(in.Keys, k)

		vbn, offset = k.NextDescriptorVBN, k.NextDescriptorOffset
	}

	perBlock := BlockSize / areaDescSize

	for i := range in.Prolog.AreaCount {
		block := check(in.Prolog.AreaVBN + uint32(i/perBlock))
		if block == nil {
			break
		}

		in.Areas = append(in.Areas, parseArea(block[(i%perBlock)*areaDescSize:][:areaDescSize]))
	}
}

// fatal is a finding that ends the walk.
func (in *Inspector) fatal(vbn uint32, status uint32, text string) {
	in.find(vbn, status, text)
	in.Stopped = true
}

// CheckAreasLocated reports a prolog with nothing to say where its areas are: a
// file whose $CREATE failed (E85). A level other than 3 stops here too: govax
// doesn't read Prologs 1 and 2 (the author's answer 1).
func (in *Inspector) CheckAreasLocated() {
	if in.Prolog.Version != 3 || in.Prolog.AreaCount == 0 || in.Prolog.AreaVBN == 0 {
		in.fatal(1, statusAreaVBN, "Information for locating area descriptors from prolog is invalid.")
	}
}

// CheckAreasReadable is the same check as FDL's report finds it, reading the area
// descriptors at a VBN of 0 (E85.FDL).
func (in *Inspector) CheckAreasReadable() {
	if in.Prolog.AreaVBN == 0 {
		in.fatal(0, inspectUnseen, "Attempt to read block with invalid VBN 0.")

		return
	}

	in.CheckAreasLocated()
}

// KeyStats is what a key's walk counted. Every "mean" and percentage VMS shows is
// computed from these by the methods below.
type KeyStats struct {
	Initialized bool
	Depth       int // index levels

	Level1Records int
	IndexBlocks   int
	indexFull     int // bytes the entries' keys would take uncompressed
	indexStored   int // bytes they take
	indexUsed     int // bytes used of the index buckets
	indexBytes    int // the index buckets' bytes
	indexMean     int // an entry's uncompressed length

	DataBlocks int
	dataUsed   int
	dataBytes  int

	Records     int // live records, or SIDRs for an alternate key
	Pointers    int // an alternate's pointers to live records
	recordBytes int // the records' uncompressed bytes
	sidrBytes   int // the bytes an alternate's level 0 buckets hold, as stored
	keyFull     int
	keyStored   int
	bodyFull    int
	bodyStored  int
}

// percent is a part's share of a whole, truncated; 0 for no whole.
func percent(part, whole int) int {
	if whole == 0 {
		return 0
	}

	return part * 100 / whole
}

// compression is how much smaller stored is than full, in percent: negative when
// the overhead outweighs the savings, as the manual says.
func compression(full, stored int) int {
	if full == 0 {
		return 0
	}

	return (full - stored) * 100 / full
}

// IndexFill is the mean index bucket fill.
func (s *KeyStats) IndexFill() int { return percent(s.indexUsed, s.indexBytes) }

// IndexCompression is the mean index entry compression.
func (s *KeyStats) IndexCompression() int { return compression(s.indexFull, s.indexStored) }

// MeanIndexLength is the mean length of an index entry, as if uncompressed.
func (s *KeyStats) MeanIndexLength() int { return s.indexMean }

// DataFill is the mean data bucket fill.
func (s *KeyStats) DataFill() int { return percent(s.dataUsed, s.dataBytes) }

// KeyCompression is the mean data key compression.
func (s *KeyStats) KeyCompression() int { return compression(s.keyFull, s.keyStored) }

// RecordCompression is the mean data record compression.
func (s *KeyStats) RecordCompression() int { return compression(s.bodyFull, s.bodyStored) }

// MeanDataLength is the mean length of a data record, rounded: a primary key's as
// if uncompressed, an alternate's a SIDR's bytes as they're stored.
func (s *KeyStats) MeanDataLength() int {
	if s.Records == 0 {
		return 0
	}

	return (2*(s.sidrBytes+s.recordBytes) + s.Records) / (2 * s.Records)
}

// RecordBytes is the live records' uncompressed bytes, which the overall space
// efficiency is a share of the file's.
func (s *KeyStats) RecordBytes() int { return s.recordBytes }

// DuplicatesPerSIDR is the FDL's DUPLICATES_PER_SIDR: the extra pointers over the
// SIDRs, rounded.
func (s *KeyStats) DuplicatesPerSIDR() int {
	if s.Records == 0 {
		return 0
	}

	return ((s.Pointers-s.Records)*2 + s.Records) / (2 * s.Records)
}

// MeanDuplicates is the statistics report's mean duplicates per data record: the
// same, truncated.
func (s *KeyStats) MeanDuplicates() int {
	if s.Records == 0 {
		return 0
	}

	return (s.Pointers - s.Records) / s.Records
}

// Walk goes through one key's tree: each index level from the root down, then the
// level 0 chain from the descriptor's first data bucket.
func (in *Inspector) Walk(k *Key) *KeyStats {
	s := &KeyStats{Depth: k.RootLevel, Initialized: k.Flags&FlagNoIndexYet == 0}
	if !s.Initialized {
		return s
	}

	first := k.RootVBN

	for level := k.RootLevel; level > 0 && !in.Stopped; level-- {
		first = in.walkIndexLevel(k, s, first, level)
	}

	if !in.Stopped {
		in.walkLevelZero(k, s)
	}

	return s
}

// inspectBucket reads a bucket, reports what its header has wrong, and returns nil
// when it can't be gone on from.
func (in *Inspector) inspectBucket(k *Key, vbn uint32, size, level int, root bool) *bucket {
	if size <= 0 || vbn == 0 || vbn > in.allocated {
		in.fatal(0, inspectUnseen, fmt.Sprintf("Attempt to read block with invalid VBN %d.", vbn))

		return nil
	}

	data := make([]byte, size*BlockSize)

	for i := range size {
		if err := in.blocks.ReadBlock(vbn+uint32(i), data[i*BlockSize:(i+1)*BlockSize]); err != nil {
			in.fatal(0, inspectUnseen, fmt.Sprintf("Attempt to read block with invalid VBN %d.", vbn+uint32(i)))

			return nil
		}
	}

	b := &bucket{
		vbn:     vbn,
		data:    data,
		check:   data[bktCheck],
		indexNo: int(data[bktIndexNo]),
		sample:  binary.LittleEndian.Uint16(data[bktSample:]),
		free:    int(binary.LittleEndian.Uint16(data[bktFree:])),
		nextID:  binary.LittleEndian.Uint16(data[bktNextID:]),
		next:    binary.LittleEndian.Uint32(data[bktNext:]),
		level:   int(data[bktLevel]),
		control: data[bktControl],
	}

	if data[len(data)-1] != b.check {
		in.find(vbn, statusCheckByte, "Bucket check byte is out of phase.")
	}

	fatal := false

	if b.indexNo != k.Ref {
		in.find(vbn, inspectUnseen, "Invalid key of reference in bucket header.")

		fatal = true
	}

	if b.sample != uint16(vbn) {
		in.find(vbn, inspectUnseen, "Invalid bucket address sample in bucket header.")

		fatal = true
	}

	if b.free < bktOverhead || b.free > len(data) {
		in.find(vbn, inspectUnseen, "Invalid first free byte offset in bucket header.")

		fatal = true
	}

	if b.level != level {
		in.find(vbn, statusLevel, "Invalid level number in bucket header.")
	}

	switch {
	case level > 0 && (b.control&bktRoot != 0) != root:
		in.find(vbn, inspectUnseen, "BKT$V_ROOTBKT flag in index bucket header is not correct.")
	case level == 0:
		for bit := 1; bit < 8; bit++ {
			if b.control&(1<<bit) != 0 {
				in.find(vbn, statusNotCounted, fmt.Sprintf("Reserved flag bit %d is set.", bit))
			}
		}
	}

	if !b.isLast() && (b.next == 0 || b.next > in.allocated) {
		in.find(vbn, inspectUnseen, "Next-bucket pointer out of range of file.")

		fatal = true
	}

	if fatal {
		in.Stopped = true

		return nil
	}

	return b
}

// chainLimit is how many buckets of a size the file could hold: a chain that goes
// on past that loops. X11's chain of one bucket pointing to itself went round 7
// times in a file of 15 blocks, with buckets of 2.
func (in *Inspector) chainLimit(size int) int {
	return int(in.allocated) / size
}

// walkIndexLevel walks one index level, returning the VBN its first entry points to.
func (in *Inspector) walkIndexLevel(k *Key, s *KeyStats, first uint32, level int) uint32 {
	down := uint32(0)
	visits := 0

	for vbn := first; ; {
		b := in.inspectBucket(k, vbn, k.IndexBucketSize, level, level == k.RootLevel)
		if b == nil {
			return 0
		}

		visits++

		entries, err := b.indexEntries(k)
		if err != nil {
			in.fatal(vbn, statusKeySpill, "Key spills over into free key space of bucket.")

			return 0
		}

		if down == 0 && len(entries) > 0 {
			down = entries[0].pointer
		}

		s.IndexBlocks += k.IndexBucketSize
		s.indexBytes += len(b.data)
		s.indexUsed += b.free + len(entries)*b.pointerSize() + bktEndOffset
		s.indexMean = k.Size + b.pointerSize()

		if level == 1 {
			s.Level1Records += len(entries)
		}

		for at, e := range entries {
			end := b.free
			if at+1 < len(entries) {
				end = entries[at+1].offset
			}

			s.indexFull += k.Size + b.pointerSize()
			s.indexStored += end - e.offset + b.pointerSize()
		}

		if b.isLast() {
			return down
		}

		if visits >= in.chainLimit(k.IndexBucketSize) {
			in.fatal(vbn, statusChainLoop, fmt.Sprintf("The bucket chains for key #%d contain a loop.", k.Ref))

			return 0
		}

		vbn = b.next
	}
}

// walkLevelZero walks the data level, the records for the primary key and the SIDRs
// for an alternate.
func (in *Inspector) walkLevelZero(k *Key, s *KeyStats) {
	visits := 0

	for vbn := k.FirstData; ; {
		b := in.inspectBucket(k, vbn, k.DataBucketSize, 0, false)
		if b == nil {
			return
		}

		visits++

		// A bucket that says it's an index bucket isn't read as records: its own
		// end must say where its pointers start (X12's level 1 on a data bucket).
		if b.level != 0 {
			free := int(binary.LittleEndian.Uint16(b.data[len(b.data)-bktEndOffset:]))
			if free < b.free || free > len(b.data)-bktEndOffset {
				in.find(vbn, inspectUnseen, "VBN free space offset is invalid.")

				return
			}
		}

		s.DataBlocks += k.DataBucketSize
		s.dataBytes += len(b.data)
		s.dataUsed += b.free + 1 // the check byte at the bucket's end

		if k.Ref == 0 {
			in.walkRecords(k, s, b)
		} else {
			in.walkSIDRs(k, s, b)
		}

		if in.Stopped || b.isLast() {
			return
		}

		if visits >= in.chainLimit(k.DataBucketSize) {
			in.fatal(vbn, statusChainLoop, fmt.Sprintf("The bucket chains for key #%d contain a loop.", k.Ref))

			return
		}

		vbn = b.next
	}
}

// walkRecords goes through a primary key's data bucket.
func (in *Inspector) walkRecords(k *Key, s *KeyStats, b *bucket) {
	ids := map[uint16]bool{}

	var previous []byte

	for at := bktOverhead; at < b.free; {
		r, next, err := in.file.dataRecord(b, k, at, previous)
		if err != nil {
			in.fatal(b.vbn, statusRecordFit, "Key and/or data bytes do not fit in primary data record.")

			return
		}

		if ids[r.id] || r.id >= b.nextID {
			in.find(b.vbn, statusRecordID, fmt.Sprintf("Record id at offset %%X'%04X' is duplicate or too large.", at))
		}

		ids[r.id] = true

		if !r.isRRV() {
			previous = r.key

			if !r.isDeleted() {
				in.countRecord(k, s, b, &r)
			}
		}

		at = next
	}
}

// countRecord adds a live record to the primary key's counts.
func (in *Inspector) countRecord(k *Key, s *KeyStats, b *bucket, r *dataRecord) {
	body, err := r.expand(k, b)
	if err != nil {
		in.fatal(b.vbn, statusRecordFit, "Key and/or data bytes do not fit in primary data record.")

		return
	}

	stored := len(r.key)
	if k.Flags&FlagKeyCompress != 0 {
		stored = int(b.data[keyStart(r)]) + 2
	}

	s.Records++
	s.recordBytes += k.Size + len(body)
	s.keyFull += k.Size
	s.keyStored += stored
	s.bodyFull += len(body)
	s.bodyStored += len(r.body)
}

// keyStart is where a record's stored key starts, in its bucket: its body less the
// key's bytes can't say, so it's found from the header's form.
func keyStart(r *dataRecord) int {
	at := r.offset + 3

	if r.control&ircNoPtr == 0 {
		at += 2 + int(r.control&ircPtrSize) + 2
	}

	return at + 2
}

// walkSIDRs goes through an alternate key's level 0 bucket.
func (in *Inspector) walkSIDRs(k *Key, s *KeyStats, b *bucket) {
	records, err := in.file.sidrs(b, k)
	if err != nil {
		in.fatal(b.vbn, statusSIDR, "SIDR pointer extends past end of SIDR record.")

		return
	}

	s.sidrBytes += b.free - bktOverhead

	for _, r := range records {
		live := 0

		for _, p := range r.pointers {
			if !p.isDeleted() {
				live++
			}
		}

		stored := k.Size
		if k.Flags&FlagKeyCompress != 0 {
			stored = int(b.data[r.offset+2]) + 2
		}

		s.Records++
		s.Pointers += live
		s.keyFull += k.Size
		s.keyStored += stored
	}
}
