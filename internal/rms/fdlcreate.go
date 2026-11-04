package rms

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/fdl"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// CREATE/FDL's file (docs/PHASE-88 - keyed index files.md, subtask 10b).
//
// FDL$CREATE fills a FAB, a NAM, and XABs from an FDL file's definition and
// calls $CREATE, then $CLOSE (the Record Management Utilities manual's
// chapter 5), so the file is what RMS makes of them, every refusal RMS's own
// status: VMS 7.1's CREATE/FDL said "-RMS-F-NPK" for an indexed file with
// no KEY section, and "-RMS-F-POS ... for key = 0" for a key past the
// record (round 3 of the probe, testdata/idx88's IDX88C.COM, C3). govax's
// does the same: CreateFDL lays the blocks out in a small memory of its own,
// as DCL's READ and WRITE do their RAB (dclkeyed.go), and calls SysCreate
// and SysClose with them.
//
// Each attribute's field is the manual's chapter 4, which names it:
//
//   - FILE: ORGANIZATION is FAB$B_ORG, ALLOCATION ALQ, EXTENSION DEQ,
//     BUCKET_SIZE BKS, GLOBAL_BUFFER_COUNT GBC, MAX_RECORD_NUMBER MRN,
//     DEFAULT_NAME DNA; the switches FOP's bits (CONTIGUOUS CTG,
//     BEST_TRY_CONTIGUOUS CBT, MAXIMIZE_VERSION MXV, which is on unless NO,
//     SUPERSEDE SUP, CREATE_IF CIF, and the others); OWNER and PROTECTION a
//     XABPRO.
//   - RECORD: FORMAT is RFM, SIZE MRS, CONTROL_FIELD_SIZE FSZ, and
//     CARRIAGE_CONTROL (CARRIAGE_RETURN unless given), BLOCK_SPAN NO, and
//     MSB_RECORD_LENGTH RAT's bits.
//   - DATE: a XABDAT's CDT, RDT, EDT, and BDT.
//   - AREA n: a XABALL, AID n; BUCKET_SIZE is the FILE's when the AREA
//     doesn't give one (the manual: an AREA's attributes override the
//     FILE's).
//   - KEY n: a XABKEY, REF n. DUPLICATES is NO for key 0 and YES for the
//     others unless given, the compressions are on unless NO, and DATA_FILL
//     and INDEX_FILL are percentages of the area's bucket, which the XABKEY
//     takes in bytes (C3I's 600 percent of a 1-block bucket was RMS$_DFL);
//     LENGTH and POSITION are segment 0 when no SEG0_ is given. PROLOG is
//     KEY 0's.
//
// The name is the command's when it gives one, else FILE NAME's; DNA is
// DEFAULT_NAME. VMS 7.1 named C2BOTHER.XYZ from a command's C2BOTHER and a
// DEFAULT_NAME of .XYZ though NAME was C2BNAME.DAT (C2.B), so NAME isn't a
// default for the command's name (unconfirmed for a name the command gives
// only in part, its device or directory missing). The ACCESS, SHARING,
// CONNECT, NETWORK, SYSTEM, and ANALYSIS sections aren't used.

// The layout of CreateFDL's memory.
const (
	fdlFABAddr  uint32 = 0x200
	fdlNAMAddr  uint32 = 0x300
	fdlESAAddr  uint32 = 0x400
	fdlRSAAddr  uint32 = 0x500
	fdlFNAAddr  uint32 = 0x600
	fdlDNAAddr  uint32 = 0x700
	fdlPROAddr  uint32 = 0x800
	fdlDATAddr  uint32 = 0x880
	fdlAllAddr  uint32 = 0x1000 // 256 XABALLs of 0x40
	fdlKeyAddr  uint32 = 0x5000 // 256 XABKEYs of 0x80
	fdlKNMAddr  uint32 = 0xD000 // 256 names of 32
	fdlMemSize         = 0xF000
	fdlXABAlign        = 0x40
	fdlKeyStep         = 0x80
	fdlNameSize        = 255
)

// FAB and XAB values CreateFDL sets that the services don't name elsewhere.
var (
	ratBLK = byte(vmsConst("FAB$M_BLK"))
	ratMSB = byte(vmsConst("FAB$M_MSB"))

	xabHRD = byte(vmsConst("XAB$M_HRD"))
)

// fdlOrganizations, fdlFormats, and fdlCarriage are FDL's keywords' FAB
// values.
var (
	fdlOrganizations = map[string]byte{"SEQUENTIAL": orgSeq, "RELATIVE": orgRel, "INDEXED": orgIdx}

	fdlFormats = map[string]byte{
		"UNDEFINED": byte(vmsConst("FAB$C_UDF")), "FIXED": byte(vmsConst("FAB$C_FIX")),
		"VARIABLE": byte(vmsConst("FAB$C_VAR")), "VFC": byte(vmsConst("FAB$C_VFC")),
		"STREAM": byte(vmsConst("FAB$C_STM")), "STREAM_LF": byte(vmsConst("FAB$C_STMLF")),
		"STREAM_CR": byte(vmsConst("FAB$C_STMCR")),
	}

	fdlCarriage = map[string]byte{"CARRIAGE_RETURN": ratCR, "FORTRAN": byte(ratFTN), "PRINT": ratPRN, "NONE": 0}
)

// fdlKeyTypes are KEY TYPE's keywords' XAB$B_DTP values.
var fdlKeyTypes = map[string]string{
	"STRING": "STG", "DSTRING": "DSTG", "BIN2": "BN2", "BIN4": "BN4", "BIN8": "BN8",
	"INT2": "IN2", "INT4": "IN4", "INT8": "IN8", "DECIMAL": "PAC", "DBIN2": "DBN2",
	"DBIN4": "DBN4", "DBIN8": "DBN8", "DINT2": "DIN2", "DINT4": "DIN4", "DINT8": "DIN8",
	"DDECIMAL": "DPAC", "COLLATED": "COL", "DCOLLATED": "DCOL",
}

// fdlFileOptions are FILE's switches and their FOP bits. MAXIMIZE_VERSION
// is on unless NO (the manual's default).
var fdlFileOptions = []struct {
	name string
	bit  uint32
	def  bool
}{
	{"CONTIGUOUS", fopCTG, false},
	{"BEST_TRY_CONTIGUOUS", fopCBT, false},
	{"MAXIMIZE_VERSION", fopMXV, true},
	{"SUPERSEDE", vmsConst("FAB$M_SUP"), false},
	{"CREATE_IF", fopCIF, false},
	{"TEMPORARY", fopTMP, false},
	{"DELETE_ON_CLOSE", fopDLT, false},
	{"PRINT_ON_CLOSE", fopSPL, false},
	{"SUBMIT_ON_CLOSE", fopSCF, false},
	{"TRUNCATE_ON_CLOSE", vmsConst("FAB$M_TEF"), false},
	{"READ_CHECK", vmsConst("FAB$M_RCK"), false},
	{"WRITE_CHECK", vmsConst("FAB$M_WCK"), false},
	{"DEFERRED_WRITE", vmsConst("FAB$M_DFW"), false},
}

// FDLCreate is what CreateFDL makes a file from.
type FDLCreate struct {
	// Definition is the FDL file's.
	Definition *fdl.Definition

	// Name is the command's file name, "" for none.
	Name string

	// Owner turns FILE OWNER's text, "[200,201]" or "[SYSTEM]", into a
	// UIC; false leaves the file the creator's (unconfirmed: what VMS
	// does with an identifier it doesn't know).
	Owner func(text string) (uint32, bool)
}

// FDLCreated is what CreateFDL made, or why it didn't.
type FDLCreated struct {
	// Name is the file's resultant name, "DUA1:[R3]C23.DAT;1", when
	// Status is a success; otherwise its expanded name,
	// "DUA1:[R3]C34.IDX;", as CREATE/FDL's message shows it.
	Name string

	// Status and STV are $CREATE's (or $CLOSE's).
	Status, STV uint32
}

// fdlBlocks is a CreateFDL's memory, built in Go before it's stored.
type fdlBlocks struct {
	mem   []byte
	chain []uint32 // the XABs, in chain order
}

func (b *fdlBlocks) byteAt(addr uint32, v byte) { b.mem[addr] = v }
func (b *fdlBlocks) word(addr uint32, v uint16) { binary.LittleEndian.PutUint16(b.mem[addr:], v) }
func (b *fdlBlocks) long(addr uint32, v uint32) { binary.LittleEndian.PutUint32(b.mem[addr:], v) }
func (b *fdlBlocks) quad(addr uint32, v uint64) { binary.LittleEndian.PutUint64(b.mem[addr:], v) }

// xab starts a XAB of code cod and length bln at addr, and adds it to the
// chain.
func (b *fdlBlocks) xab(addr uint32, cod byte, bln uint32) {
	b.byteAt(addr+xabCOD, cod)
	b.byteAt(addr+xabBLN, byte(bln))
	b.chain = append(b.chain, addr)
}

// CreateFDL makes the file c describes, as CREATE/FDL does, and closes it.
func (s *Session) CreateFDL(c FDLCreate) FDLCreated {
	ctx := &Context{
		Mounts: s.Mounts, Logicals: s.Logicals, Session: s, Files: NewFileTable(nil),
		Mem: vm.NewMemory(fdlMemSize), CPU: registers.New(),
	}

	b := &fdlBlocks{mem: make([]byte, fdlMemSize)}
	def := c.Definition
	file := def.Section("FILE")

	name := c.Name
	if name == "" {
		name, _ = file.String("NAME")
	}

	dna, _ := file.String("DEFAULT_NAME")

	s.fdlFAB(b, def, name, dna)
	s.fdlProtection(ctx, b, file, name, dna, c.Owner)
	fdlDates(b, def.Section("DATE"))
	buckets := fdlAreas(b, def, file)
	fdlKeys(b, def, buckets, file)

	// The chain, in the order built.
	next := fdlFABAddr + fabOffset("XAB")
	for _, x := range b.chain {
		b.long(next, x)
		next = x + xabNXT
	}

	if err := ctx.storeBytes(0, b.mem); err != nil {
		return FDLCreated{Status: rmsDeviceError}
	}

	if _, err := SysCreate(ctx, []uint32{fdlFABAddr}); err != nil {
		return FDLCreated{Status: rmsDeviceError}
	}

	sts, _ := ctx.loadLongword(fdlFABAddr + fabSTS)
	stv, _ := ctx.loadLongword(fdlFABAddr + fabSTV)

	if sts&1 == 0 {
		return FDLCreated{Name: fdlString(ctx, namESL, fdlESAAddr), Status: sts, STV: stv}
	}

	created := FDLCreated{Name: fdlString(ctx, namRSL, fdlRSAAddr), Status: sts, STV: stv}

	if _, err := SysClose(ctx, []uint32{fdlFABAddr}); err != nil {
		return FDLCreated{Name: created.Name, Status: rmsDeviceError}
	}

	if sts, _ := ctx.loadLongword(fdlFABAddr + fabSTS); sts&1 == 0 {
		created.Status = sts
		created.STV, _ = ctx.loadLongword(fdlFABAddr + fabSTV)
	}

	return created
}

// fdlString reads the NAM's string at addr whose length is at lengthField.
func fdlString(ctx *Context, lengthField, addr uint32) string {
	n, err := ctx.loadByte(fdlNAMAddr + lengthField)
	if err != nil {
		return ""
	}

	text, err := ctx.loadBytes(addr, int(n))
	if err != nil {
		return ""
	}

	return string(text)
}

// fdlFAB lays out the FAB and NAM: FILE's and RECORD's attributes, the
// names.
func (s *Session) fdlFAB(b *fdlBlocks, def *fdl.Definition, name, dna string) {
	file, record := def.Section("FILE"), def.Section("RECORD")
	fab := fdlFABAddr

	b.byteAt(fab+fabOffset("BID"), fabBIDValue)
	b.byteAt(fab+fabOffset("BLN"), fabBLNValue)
	b.byteAt(fab+fabFAC, facPut)
	b.byteAt(fab+fabORG, fdlOrganizations[file.Keyword("ORGANIZATION", "SEQUENTIAL")])
	b.long(fab+fabOffset("ALQ"), uint32(file.Number("ALLOCATION", 0)))
	b.word(fab+fabOffset("DEQ"), uint16(file.Number("EXTENSION", 0)))
	b.byteAt(fab+fabOffset("BKS"), byte(file.Number("BUCKET_SIZE", 0)))
	b.word(fab+fabOffset("GBC"), uint16(file.Number("GLOBAL_BUFFER_COUNT", 0)))
	b.long(fab+fabOffset("MRN"), uint32(file.Number("MAX_RECORD_NUMBER", 0)))

	var fop uint32

	for _, o := range fdlFileOptions {
		if file.Switch(o.name, o.def) {
			fop |= o.bit
		}
	}

	b.long(fab+fabFOP, fop)

	b.byteAt(fab+fabRFM, fdlFormats[record.Keyword("FORMAT", "VARIABLE")])
	b.word(fab+fabMRS, uint16(record.Number("SIZE", 0)))
	b.byteAt(fab+fabOffset("FSZ"), byte(record.Number("CONTROL_FIELD_SIZE", 0)))

	rat := fdlCarriage[record.Keyword("CARRIAGE_CONTROL", "CARRIAGE_RETURN")]
	if !record.Switch("BLOCK_SPAN", true) {
		rat |= ratBLK
	}

	if record.Switch("MSB_RECORD_LENGTH", false) {
		rat |= ratMSB
	}

	b.byteAt(fab+fabRAT, rat)

	fdlText(b, fab+fabOffset("FNA"), fab+fabOffset("FNS"), fdlFNAAddr, name)
	fdlText(b, fab+fabOffset("DNA"), fab+fabOffset("DNS"), fdlDNAAddr, dna)

	nam := fdlNAMAddr
	b.long(fab+fabOffset("NAM"), nam)
	b.byteAt(nam+namBID, namBIDValue)
	b.byteAt(nam+namBLN, namBLNValue)
	b.long(nam+namESA, fdlESAAddr)
	b.byteAt(nam+namESS, fdlNameSize)
	b.long(nam+namRSA, fdlRSAAddr)
	b.byteAt(nam+namRSS, fdlNameSize)
}

// fdlText puts text at addr and its address and length in a FAB's fields.
func fdlText(b *fdlBlocks, addrField, sizeField, addr uint32, text string) {
	text = text[:min(len(text), fdlNameSize)]
	copy(b.mem[addr:], text)
	b.long(addrField, addr)
	b.byteAt(sizeField, byte(len(text)))
}

// fdlProtection lays out a XABPRO for FILE's OWNER and PROTECTION. With an
// owner and no protection the volume's default protection goes in it, the
// one a file gets without a XABPRO.
func (s *Session) fdlProtection(ctx *Context, b *fdlBlocks, file *fdl.Section, name, dna string, owner func(string) (uint32, bool)) {
	pro, hasPro := file.Get("PROTECTION")

	var uic uint32

	hasOwner := false

	if v, ok := file.Get("OWNER"); ok && owner != nil {
		uic, hasOwner = owner(v.Text)
	}

	if !hasPro && !hasOwner {
		return
	}

	mask := pro.Protection
	if !hasPro {
		names, sts := ctx.expandName(nameInputs{Primary: name, Default: dna})
		if sts != 0 || len(names) == 0 {
			return
		}

		vol, ok := s.Mounts.Lookup(names[0].spec().Device)
		if !ok {
			return
		}

		mask = vol.Devices[0].Home.FileProtection
	}

	b.xab(fdlPROAddr, xabPRO, uint32(xabLengths[xabPRO]))
	b.word(fdlPROAddr+xabPROw, mask)
	b.long(fdlPROAddr+xabUIC, uic)
}

// fdlDates lays out a XABDAT for the DATE section.
func fdlDates(b *fdlBlocks, date *fdl.Section) {
	if date == nil {
		return
	}

	b.xab(fdlDATAddr, xabDAT, uint32(xabLengths[xabDAT]))

	for _, d := range []struct {
		name string
		off  uint32
	}{{"CREATION", xabCDT}, {"REVISION", xabRDTq}, {"EXPIRATION", xabEDT}, {"BACKUP", xabBDT}} {
		if v, ok := date.Get(d.name); ok {
			b.quad(fdlDATAddr+d.off, v.Time)
		}
	}
}

// fdlAreas lays out a XABALL for each AREA section, and returns each area's
// bucket size, which key fills are percentages of. A file with no AREA
// section has one area, of FILE's bucket size.
func fdlAreas(b *fdlBlocks, def *fdl.Definition, file *fdl.Section) map[int]int64 {
	fileBucket := file.Number("BUCKET_SIZE", 0)
	buckets := map[int]int64{0: fileBucket}

	for i, area := range def.Numbered("AREA") {
		addr := fdlAllAddr + uint32(i)*fdlXABAlign
		bucket := area.Number("BUCKET_SIZE", fileBucket)
		buckets[area.ID] = bucket

		b.xab(addr, xabALL, uint32(xabLengths[xabALL]))
		b.byteAt(addr+xabAllAID, byte(area.ID))
		b.byteAt(addr+xabAllBKZ, byte(bucket))
		b.long(addr+xabALQ, uint32(area.Number("ALLOCATION", 0)))
		b.word(addr+xabDEQ, uint16(area.Number("EXTENSION", 0)))
		b.word(addr+xabAllVOL, uint16(area.Number("VOLUME", 0)))

		var aop byte

		for _, o := range []struct {
			name string
			bit  byte
		}{{"CONTIGUOUS", xabContiguous}, {"BEST_TRY_CONTIGUOUS", xabContiguousBestTry}, {"EXACT_POSITIONING", xabHRD}} {
			if area.Switch(o.name, false) {
				aop |= o.bit
			}
		}

		b.byteAt(addr+xabAOP, aop)
	}

	return buckets
}

// fdlKeys lays out a XABKEY for each KEY section.
func fdlKeys(b *fdlBlocks, def *fdl.Definition, buckets map[int]int64, file *fdl.Section) {
	fill := func(percent int64, area int64) uint16 {
		bucket, ok := buckets[int(area)]
		if !ok {
			bucket = file.Number("BUCKET_SIZE", 0)
		}

		return uint16(percent * bucket * 512 / 100)
	}

	for i, key := range def.Numbered("KEY") {
		addr := fdlKeyAddr + uint32(i)*fdlKeyStep
		b.xab(addr, xabKEY, vmsConst("XAB$C_KEYLEN"))
		b.byteAt(addr+xabKeyREF, byte(key.ID))
		b.byteAt(addr+xabKeyDTP, byte(vmsConst("XAB$C_"+fdlKeyTypes[key.Keyword("TYPE", "STRING")])))

		var flg byte

		for _, f := range []struct {
			name string
			bit  byte
			def  bool
			on   bool // the bit is set when the switch is on
		}{
			{"DUPLICATES", xabFlagDUP, key.ID != 0, true},
			{"CHANGES", xabFlagCHG, false, true},
			{"NULL_KEY", xabFlagNUL, false, true},
			{"DATA_KEY_COMPRESSION", xabFlagNoKeyCmpr, true, false},
			{"DATA_RECORD_COMPRESSION", xabFlagNoRecordCmpr, true, false},
			{"INDEX_COMPRESSION", xabFlagNoIndexCmpr, true, false},
		} {
			if key.Switch(f.name, f.def) == f.on {
				flg |= f.bit
			}
		}

		b.byteAt(addr+xabKeyFLG, flg)

		if v, ok := key.Get("NULL_VALUE"); ok {
			b.byteAt(addr+xabKeyNUL, v.Char)
		}

		dan := key.Number("DATA_AREA", 0)
		ian := key.Number("INDEX_AREA", 0)
		b.byteAt(addr+xabKeyDAN, byte(dan))
		b.byteAt(addr+xabKeyIAN, byte(ian))
		b.byteAt(addr+xabKeyLAN, byte(key.Number("LEVEL1_INDEX_AREA", 0)))

		if v, ok := key.Get("DATA_FILL"); ok {
			b.word(addr+xabKeyDFL, fill(v.Number, dan))
		}

		if v, ok := key.Get("INDEX_FILL"); ok {
			b.word(addr+xabKeyIFL, fill(v.Number, ian))
		}

		if key.ID == 0 {
			b.byteAt(addr+xabKeyPROLOG, byte(key.Number("PROLOG", 0)))
		}

		fdlSegments(b, addr, key)

		// A name is padded with blanks (the manual says nulls, but VMS 7.1
		// wrote blanks), and NAME "" is 32 of them, where a key with no
		// NAME has zeros (testdata/idx88's round 3: C33, C36, C51).
		if text, ok := key.String("NAME"); ok {
			knm := fdlKNMAddr + uint32(i)*keyNameSize
			field := b.mem[knm : knm+keyNameSize]

			for j := range field {
				field[j] = ' '
			}

			copy(field, text)
			b.long(addr+xabKeyKNM, knm)
		}
	}
}

// fdlSegments puts a key's segments in its XABKEY: SEGn_LENGTH and
// SEGn_POSITION, or LENGTH and POSITION as segment 0 when no SEG0_ is
// given.
func fdlSegments(b *fdlBlocks, addr uint32, key *fdl.Section) {
	for n := range fdl.MaxSegments {
		size := key.Number(segmentName(n, "LENGTH"), 0)
		pos := key.Number(segmentName(n, "POSITION"), 0)

		if n == 0 && !key.Has("SEG0_LENGTH") && !key.Has("SEG0_POSITION") {
			size = key.Number("LENGTH", 0)
			pos = key.Number("POSITION", 0)
		}

		b.byteAt(addr+xabKeySIZ+uint32(n), byte(size))
		b.word(addr+xabKeyPOS+2*uint32(n), uint16(pos))
	}
}

// segmentName is SEGn_LENGTH's or SEGn_POSITION's name.
func segmentName(n int, part string) string {
	return "SEG" + string(rune('0'+n)) + "_" + part
}
