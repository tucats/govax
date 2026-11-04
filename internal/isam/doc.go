// Package isam is the on-disk format of RMS's indexed files (Phase 88;
// docs/PHASE-88 - keyed index files.md, whose Part A is where this layout
// was worked out, from VMS-made files and VMS's own reports of them).
//
// An indexed file keeps its records in order of one or more keys, so a
// program can ask for "the record whose part number is 1234" rather than
// "the 57th record". It's built like a library's card catalog:
//
//   - The file is divided into buckets. A bucket is RMS's unit of I/O: 1
//     to 63 512-byte blocks, its size chosen when the file is made.
//   - The prolog, the first few blocks, describes the file: one key
//     descriptor per key (where the key is in a record, its type, the
//     VBN, or virtual block number, of its root bucket), and one area
//     descriptor per area (a set of buckets of one size).
//   - Each key has a tree of buckets over it. Level 0 is the data level;
//     each level above it is an index, whose entries are "the highest key
//     in that bucket below". The top is one bucket, the root. Every
//     level's buckets are also chained in key order, so reading in
//     sequence never goes back up the tree.
//   - Key 0, the primary key, keeps the records themselves in its level
//     0. Any other key, an alternate, keeps SIDRs in its level 0
//     (secondary index data records): a key value and pointers to the
//     records having it.
//
// A record's address, its RFA, is the pair (VBN, ID): the bucket it was
// first written into, and an ID number unique in that bucket. It never
// changes. When a split moves the record to a new bucket, the old bucket
// keeps a small RRV (record reference vector) saying where it went, and
// the record carries its RFA with it. That's why a SIDR's pointers still
// work after a split, and why a program can keep an RFA.
//
// The package reads such a file (subtask 3): Open reads the prolog, Find
// searches a key's tree, First and Following read in key order, Walk reads a
// whole key, Record reads by RFA, and Dump shows the structure as text. It
// also makes one (subtask 4): Create lays out the prolog, and Put adds a
// record, making a key's tree at its first record. A record that doesn't fit
// its bucket splits it (split.go, subtask 5). Update and Delete change and take
// out records already there (change.go, subtask 6): an update in place, its RFA
// kept, or by a split if it outgrows its bucket; a delete closing up, but for a
// bucket's last record, kept as a deleted stub with its key. Check walks a whole
// file for what no operation may leave, which the tests run after them. An Inspector
// (inspect.go) is ANALYZE/RMS_FILE's walk, which reports what's wrong and goes on. It's plain Go that
// knows nothing of VAX memory, FABs, or RABs: internal/rms maps those
// onto it. Only Prolog 3 files are read, by the author's choice; Open
// reports anything else as an ErrPrologLevel.
//
// Names such as KEY$B_ROOTLEV in the comments are VMS's own, from the
// definitions real MACRO assembled in testdata/idx88 (defined.txt).
package isam
