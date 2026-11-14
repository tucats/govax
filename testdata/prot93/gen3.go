//go:build ignore

// gen3 writes round 3 of Phase 93's probe (README.md's "Round 3"): the
// security services. sec93.mar calls $PARSE_ACL, $FORMAT_ACL, $CHANGE_ACL,
// $CHKPRO, $CHECK_ACCESS, $GET_SECURITY, $SET_SECURITY, and $SETDFPROT,
// case by case, each case a routine of its own under a handler, and prints
// what each returned. sec93.com is SYSTEM's run (the accounts again, the
// tree the cases use, the program's parts with and without privileges, and
// round 9 of the system service macro probes, testdata/mp/macros);
// sec93u.com is PROT93C's (the program as the world, and the execute-only
// procedures, A.4's first open question); sec93y.com the end. Run it from
// the repository root:
//
//	go run testdata/prot93/gen3.go
//
// Every ACE the program hands a service is written here in bytes, by the
// layouts round 1 found (the phase doc's A.4, "ACE layouts"), so no case
// depends on $PARSE_ACL but $PARSE_ACL's own.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const dir = "testdata/prot93"

// disk is the logical name sec93.com defines for the exchange volume, as
// rounds 1 and 2 did: the accounts' default device is it.
const disk = "PROT93$DISK:"

// ---------------------------------------------------------------------
// Definitions: the values round 1 brought back (defined.txt), emitted as
// assignments at the head of the program for each name a case uses, so
// the program needs no $ACLDEF, $CHPDEF, or $OSSDEF macro on either side.

var defined = map[string]int64{}

var used = map[string]bool{}

func readDefined() {
	f, err := os.Open(filepath.Join(dir, "defined.txt"))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		name, value, ok := strings.Cut(s.Text(), " = ")
		if !ok {
			continue
		}

		v, err := strconv.ParseInt(strings.TrimPrefix(value, "0x"), 16, 64)
		if err != nil {
			log.Fatalf("%s: %v", s.Text(), err)
		}

		defined[name] = v
	}
}

// sym is a defined name, recorded so its value is emitted.
func sym(name string) string {
	if _, ok := defined[name]; !ok {
		log.Fatalf("%s is not in defined.txt", name)
	}

	used[name] = true

	return name
}

// ---------------------------------------------------------------------
// Identifiers and ACEs.

const (
	idA     = 0x00800001 // [200,1], PROT93A
	idB     = 0x00800002 // [200,2], PROT93B
	idC     = 0x00C00001 // [300,1], PROT93C
	idSys   = 0x00010004 // [1,4], SYSTEM
	idG200  = 0x0080FFFF // [200,*]
	idAll   = 0x3FFFFFFF // [*,*]
	idAny1  = 0x3FFF0001 // [*,1]
	id310   = 0x00C80001 // [310,1], which no identifier names
	idZero  = 0x00000000 // [0,0]
	idTeam  = 0x80010001 // PROT93_TEAM, round 1's value
	idInter = 0x80000003 // INTERACTIVE
	idLocal = 0x80000004 // LOCAL
	idNone  = 0x8001FFFF // a general identifier that doesn't exist
)

// ACE types and option flags (A.4).
const (
	tKeyID   = 1
	tAudit   = 5
	tAlarm   = 6
	tInfo    = 7
	tDirDef  = 9
	tSubsys  = 11
	tCreator = 13

	oDefault = 0x100
	oProt    = 0x200
	oHidden  = 0x400
	oNoProp  = 0x800

	aR = 1
	aW = 2
	aE = 4
	aD = 8
	aC = 0x10
)

func le32(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

// ace is an ACE's bytes: its size, type, flags, then body.
func ace(typ byte, flags uint16, body ...[]byte) []byte {
	b := []byte{0, typ, byte(flags), byte(flags >> 8)}
	for _, p := range body {
		b = append(b, p...)
	}

	b[0] = byte(len(b))

	return b
}

// idACE is an identifier ACE.
func idACE(flags uint16, access uint32, ids ...uint32) []byte {
	body := [][]byte{le32(access)}
	for _, id := range ids {
		body = append(body, le32(id))
	}

	return ace(tKeyID, flags, body...)
}

// denied turns a category's letters into the bits a default protection ACE
// keeps for it: the ones denied, with C (A.4).
func denied(letters string) uint32 {
	m := uint32(0x1F)
	for _, c := range letters {
		switch c {
		case 'R':
			m &^= aR
		case 'W':
			m &^= aW
		case 'E':
			m &^= aE
		case 'D':
			m &^= aD
		}
	}

	return m
}

// dirdefACE is a default protection ACE for S, O, G, and W's letters.
func dirdefACE(flags uint16, s, o, g, w string) []byte {
	return ace(tDirDef, flags, le32(0), le32(denied(s)), le32(denied(o)), le32(denied(g)), le32(denied(w)))
}

func creatorACE(flags uint16, access uint32) []byte {
	return ace(tCreator, flags, le32(access))
}

// alarmACE is an alarm or audit ACE: the low flag bits SUCCESS 1 and
// FAILURE 2, the journal's name after the access, uncounted.
func alarmACE(typ byte, flags uint16, access uint32, name string) []byte {
	return ace(typ, flags, le32(access), []byte(name))
}

func cat(parts ...[]byte) []byte {
	var b []byte
	for _, p := range parts {
		b = append(b, p...)
	}

	return b
}

// ---------------------------------------------------------------------
// Cases.

// kase is one case: a routine of its own (C<n>), run when its part's
// letter is in SEC93_PART. Its data and code may use "%" for the case's
// label prefix.
type kase struct {
	part  byte
	title string
	data  []string
	code  []string
	files []object
}

// object is a file or directory sec93.com makes for the cases, with its
// owner, protection, and ACL (each ACE as SET SECURITY/ACL takes it).
type object struct {
	name, owner, prot string
	acl               []string
	dir               bool
}

var cases []*kase

func newCase(part byte, format string, args ...any) *kase {
	k := &kase{part: part, title: fmt.Sprintf(format, args...)}
	cases = append(cases, k)

	return k
}

func (k *kase) d(format string, args ...any) { k.data = append(k.data, fmt.Sprintf(format, args...)) }
func (k *kase) c(format string, args ...any) { k.code = append(k.code, fmt.Sprintf(format, args...)) }

// pct stands for a "%" in text until the label prefixes are substituted.
const pct = "\x01"

// delim is a delimiter for .ASCID that text doesn't contain.
func delim(text string) string {
	for _, d := range []string{`"`, "/", "|", "~", "^", "'"} {
		if !strings.Contains(text, d) {
			return d
		}
	}

	log.Fatalf("no delimiter for %q", text)

	return ""
}

// ascid is a class S descriptor and its text.
func (k *kase) ascid(label, text string) {
	d := delim(text)
	k.d("%s:\t.ASCID\t%s%s%s", label, d, strings.ReplaceAll(text, "%", pct), d)
}

// bytes is a label on b's bytes, 12 to a line.
func (k *kase) bytes(label string, b []byte) {
	if len(b) == 0 {
		k.d("%s:", label)

		return
	}

	for i := 0; i < len(b); i += 12 {
		var parts []string
		for _, x := range b[i:min(i+12, len(b))] {
			parts = append(parts, fmt.Sprintf("^X%02X", x))
		}

		lead := ""
		if i == 0 {
			lead = label + ":"
		}

		k.d("%s\t.BYTE\t%s", lead, strings.Join(parts, ","))
	}
}

// buffer is a zeroed buffer of n bytes and a descriptor for it, length dl.
func (k *kase) buffer(label string, n, dl int) {
	k.d("%s:\t.BLKB\t%d", label, n)
	k.d("%sD:\t.LONG\t%d", label, dl)
	k.d("\t.ADDRESS %s", label)
}

// byteDesc is a descriptor of length dl for b's bytes.
func (k *kase) byteDesc(label string, b []byte, dl int) {
	k.bytes(label, b)
	k.d("%sD:\t.LONG\t%d", label, dl)
	k.d("\t.ADDRESS %s", label)
}

// file is a file the case needs, which sec93.com makes first.
func (k *kase) file(o object) { k.files = append(k.files, o) }

// show reports a file's owner, protection, and ACL through RMS's $DISPLAY,
// as rounds 1 and 2 did, so what a service changed is seen apart from it.
func (k *kase) show(spec string) {
	k.ascid("%FS", spec)
	k.c("PUSHAQ\t%%FS")
	k.c("CALLS\t#1,SHOWFILE")
}

// ---------------------------------------------------------------------
// Item lists.

// item is one entry of an item_list_3: an input (data, its bytes) or an
// output (out bytes, shown by kind), or a buffer address of 0 (zero).
type item struct {
	code   string // a name from defined.txt, or a number
	data   []byte
	length int // the buffer length to give, if not len(data) or out
	out    int
	kind   int // an output's form: kHex, kText, kACL, kAll
	zero   bool
	noRet  bool // no return length address
	label  string
}

const (
	kHex  = 0 // the bytes the return length says
	kText = 1 // the text the return length says
	kACL  = 2 // the ACEs in the bytes the return length says
	kAll  = 3 // the whole buffer, in hex
)

func in(code string, b []byte) item         { return item{code: code, data: b} }
func inL(code string, v uint32) item        { return item{code: code, data: le32(v)} }
func out(code string, n, kind int) item     { return item{code: code, out: n, kind: kind} }
func outNR(code string, n, kind int) item   { return item{code: code, out: n, kind: kind, noRet: true} }
func zero(code string, n int) item          { return item{code: code, zero: true, length: n} }
func inN(code string, b []byte, n int) item { return item{code: code, data: b, length: n} }
func inQ(code string, lo, hi uint32) item {
	return item{code: code, data: append(le32(lo), le32(hi)...)}
}
func inW(code string, words ...uint16) item {
	var b []byte
	for _, w := range words {
		b = binary.LittleEndian.AppendUint16(b, w)
	}

	return item{code: code, data: b}
}

// codeOf is an item code's text in the program.
func codeOf(code string) string {
	if _, err := strconv.Atoi(code); err == nil {
		return code
	}

	return sym(code)
}

// items writes the item list %L and its buffers, and returns the code that
// clears the outputs before the call and the code that shows them after.
func (k *kase) items(list []item) (before, after []string) {
	for i := range list {
		it := &list[i]
		it.label = fmt.Sprintf("%%I%d", i+1)

		switch {
		case it.zero:
		case it.out > 0:
			k.d("%s:\t.BLKB\t%d", it.label, it.out)
			k.d("%sR:\t.WORD\t0,0", it.label)
			k.ascid(it.label+"N", strings.TrimPrefix(strings.TrimPrefix(it.code, "CHP$_"), "OSS$_"))
		default:
			k.bytes(it.label, it.data)
			k.d("\t.ALIGN\tLONG")
		}
	}

	k.d("\t.ALIGN\tLONG")

	for i, it := range list {
		lead := ""
		if i == 0 {
			lead = "%L:"
		}

		n := len(it.data)
		if it.out > 0 {
			n = it.out
		}

		if it.length != 0 {
			n = it.length
		}

		k.d("%s\t.WORD\t%d,%s", lead, n, codeOf(it.code))

		if it.zero {
			k.d("\t.LONG\t0")
		} else {
			k.d("\t.ADDRESS %s", it.label)
		}

		if it.out > 0 && !it.noRet {
			k.d("\t.ADDRESS %sR", it.label)
		} else {
			k.d("\t.LONG\t0")
		}
	}

	if len(list) == 0 {
		k.d("%%L:")
	}

	k.d("\t.LONG\t0")

	for _, it := range list {
		if it.out == 0 {
			continue
		}

		before = append(before, fmt.Sprintf("MOVC5\t#0,(SP),#0,#%d,%s", it.out, it.label),
			fmt.Sprintf("MOVW\t#^XFFFF,%sR", it.label))

		ret := "PUSHAW\t" + it.label + "R"
		if it.noRet {
			ret = "PUSHL\t#0"
		}

		after = append(after,
			fmt.Sprintf("PUSHL\t#%d", it.kind),
			fmt.Sprintf("PUSHL\t#%d", it.out),
			"PUSHAB\t"+it.label,
			ret,
			"PUSHAQ\t"+it.label+"N",
			"CALLS\t#5,SHOWOUT")
	}

	return before, after
}

// ---------------------------------------------------------------------
// $PARSE_ACL.

type parseOpts struct {
	bufLen   int  // aclent's descriptor length (256)
	noErrpos bool // errpos omitted
	accnam   bool // the probe's own access names
	dump     int  // bytes of the buffer to show whatever the size says
	args     int  // the CALLS count (6)
}

func parse(part byte, text string, o parseOpts) {
	if o.bufLen == 0 {
		o.bufLen = 256
	}

	if o.args == 0 {
		o.args = 6
	}

	title := text
	if o.bufLen != 256 {
		title += fmt.Sprintf(" [buffer %d]", o.bufLen)
	}

	if o.noErrpos {
		title += " [no errpos]"
	}

	if o.accnam {
		title += " [access names]"
	}

	k := newCase(part, "$PARSE_ACL %s", title)
	k.ascid("%S", text)
	k.buffer("%B", 256, o.bufLen)
	k.d("%%E:\t.WORD\t0,0")

	k.c("MOVC5\t#0,(SP),#0,#256,%%B")
	k.c("MOVW\t#^XFFFF,%%E")

	for range o.args - 4 {
		k.c("CLRL\t-(SP)")
	}

	if o.accnam {
		k.c("PUSHAQ\tACCNAM")
	} else {
		k.c("CLRL\t-(SP)")
	}

	if o.noErrpos {
		k.c("CLRL\t-(SP)")
	} else {
		k.c("PUSHAW\t%%E")
	}

	k.c("PUSHAQ\t%%BD")
	k.c("PUSHAQ\t%%S")
	k.c("CALLS\t#%d,G^SYS$PARSE_ACL", o.args)
	k.c("MOVL\tR0,R2")
	k.c("MOVZWL\t%%E,R3")
	k.c("MOVZBL\t%%B,R4")
	k.c("SAY\t</  $PARSE_ACL r0=!XL errpos=!UL size=!UL/>,R2,R3,R4")
	// What follows errpos, where the service stopped.
	k.c("MOVZWL\t%%S,R5")
	k.c("CMPL\tR3,R5")
	k.c("BGTRU\t10$")
	k.c("SUBL2\tR3,R5")
	k.c("ADDL3\tR3,%%S+4,R6")
	k.c("SAY\t</  rest !UL \"!AF\"/>,R5,R5,R6")
	k.c("10$:")

	if o.dump > 0 {
		k.c("PUSHL\t#%d", o.dump)
		k.c("PUSHAB\t%%B")
		k.c("CALLS\t#2,HEXLINES")
	}

	k.c("CMPL\tR4,#256")
	k.c("BLEQ\t20$")
	k.c("MOVL\t#256,R4")
	k.c("20$:")
	k.c("PUSHL\tR4")
	k.c("PUSHAB\t%%B")
	k.c("CALLS\t#2,ACLWALK")
}

// ---------------------------------------------------------------------
// $FORMAT_ACL.

type fmtOpts struct {
	width   int    // -1 omitted
	indent  int    // -1 omitted
	trm     []byte // nil omitted
	outLen  int    // aclstr's descriptor length (512)
	noLen   bool   // acllen omitted
	accnam  bool
	descLen int // aclent's descriptor length, if not the ACE's size
	note    string
}

func format(part byte, what string, b []byte, o fmtOpts) {
	if o.outLen == 0 {
		o.outLen = 512
	}

	k := newCase(part, "$FORMAT_ACL %s%s", what, o.note)

	dl := len(b)
	if o.descLen != 0 {
		dl = o.descLen
	}

	k.byteDesc("%A", b, dl)
	k.d("\t.BLKB\t32")
	k.buffer("%O", 512, o.outLen)
	k.d("%%N:\t.WORD\t0,0")
	k.d("%%W:\t.WORD\t%d", max(o.width, 0))
	k.d("%%X:\t.WORD\t%d", max(o.indent, 0))

	if o.trm != nil {
		k.bytes("%TR", o.trm)
		k.d("%%TRD:\t.LONG\t%d", len(o.trm))
		k.d("\t.ADDRESS %%TR")
	}

	k.c("MOVC5\t#0,(SP),#^A/./,#512,%%O")
	k.c("MOVW\t#^XFFFF,%%N")
	k.c("CLRQ\t-(SP)")

	if o.accnam {
		k.c("PUSHAQ\tACCNAM")
	} else {
		k.c("CLRL\t-(SP)")
	}

	if o.indent >= 0 {
		k.c("PUSHAW\t%%X")
	} else {
		k.c("CLRL\t-(SP)")
	}

	if o.trm != nil {
		k.c("PUSHAQ\t%%TRD")
	} else {
		k.c("CLRL\t-(SP)")
	}

	if o.width >= 0 {
		k.c("PUSHAW\t%%W")
	} else {
		k.c("CLRL\t-(SP)")
	}

	k.c("PUSHAQ\t%%OD")

	if o.noLen {
		k.c("CLRL\t-(SP)")
	} else {
		k.c("PUSHAW\t%%N")
	}

	k.c("PUSHAQ\t%%AD")
	k.c("CALLS\t#9,G^SYS$FORMAT_ACL")
	k.c("MOVL\tR0,R2")
	k.c("MOVZWL\t%%N,R3")
	k.c("SAY\t</  $FORMAT_ACL r0=!XL len=!UL/>,R2,R3")
	k.c("CMPL\tR3,#512")
	k.c("BLEQ\t10$")
	k.c("MOVL\t#%d,R3", o.outLen)
	k.c("10$:")
	k.c("SAY\t</  \"!AF\"/>,R3,#%%O")

	if o.trm != nil || o.noLen {
		// The bytes, to see the terminators (and, with no length, how far
		// the text went).
		n := 64
		if o.noLen {
			n = 96
		}

		k.c("PUSHL\t#%d", n)
		k.c("PUSHAB\t%%O")
		k.c("CALLS\t#2,HEXLINES")
	}
}

// ---------------------------------------------------------------------
// $CHANGE_ACL.

type changeOpts struct {
	objtyp string // a name, a number, or "none" (no objtyp)
	noName bool   // objnam omitted
	chan_  bool   // chan, from OPENCHAN of the file
	ctx    string // "", a value ("0", "-1"), or "keep" (the shared CONTEXT)
	acmode int    // -1 omitted
	note   string
	noShow bool
	nameAs string // objnam's text, if not the file's
}

func change(part byte, what, spec string, list []item, o changeOpts) *kase {
	k := newCase(part, "$CHANGE_ACL %s %s%s", spec, what, o.note)

	name := spec
	if o.nameAs != "" {
		name = o.nameAs
	}

	k.ascid("%NM", name)

	objtyp := "ACL$C_FILE"
	if o.objtyp != "" {
		objtyp = o.objtyp
	}

	if objtyp != "none" {
		k.d("%%OT:\t.LONG\t%s", codeOf(objtyp))
	}

	k.d("%%AM:\t.LONG\t%d", max(o.acmode, 0))
	before, after := k.items(list)

	switch o.ctx {
	case "":
	case "keep":
	default:
		k.c("MOVL\t#%s,CONTEXT", o.ctx)
	}

	if o.chan_ {
		k.c("PUSHL\t#<FAB$M_GET!FAB$M_PUT>")
		k.c("PUSHAQ\t%%NM")
		k.c("CALLS\t#2,OPENCHAN")
	}

	k.code = append(k.code, before...)
	k.c("CLRQ\t-(SP)")

	if o.ctx != "" {
		k.c("PUSHAL\tCONTEXT")
	} else {
		k.c("CLRL\t-(SP)")
	}

	k.c("CLRL\t-(SP)")

	if o.acmode >= 0 {
		k.c("PUSHAL\t%%AM")
	} else {
		k.c("CLRL\t-(SP)")
	}

	k.c("PUSHAB\t%%L")

	if o.noName || o.chan_ {
		k.c("CLRL\t-(SP)")
	} else {
		k.c("PUSHAQ\t%%NM")
	}

	if objtyp == "none" {
		k.c("CLRL\t-(SP)")
	} else {
		k.c("PUSHAL\t%%OT")
	}

	if o.chan_ {
		k.c("MOVZWL\tCHAN,-(SP)")
	} else {
		k.c("CLRL\t-(SP)")
	}

	k.c("CALLS\t#9,G^SYS$CHANGE_ACL")
	k.c("MOVL\tR0,R2")

	if o.ctx != "" {
		k.c("SAY\t</  $CHANGE_ACL r0=!XL contxt=!XL/>,R2,CONTEXT")
	} else {
		k.c("SAY\t</  $CHANGE_ACL r0=!XL/>,R2")
	}

	k.code = append(k.code, after...)

	if o.chan_ {
		k.c("CALLS\t#0,CLOSECHAN")
	}

	if !o.noShow {
		k.show(spec)
	}

	return k
}

// ---------------------------------------------------------------------
// $CHKPRO.

func chkpro(part byte, what string, list []item, args int) {
	k := newCase(part, "$CHKPRO %s", what)
	before, after := k.items(list)
	k.code = append(k.code, before...)

	for range args - 1 {
		k.c("CLRL\t-(SP)")
	}

	k.c("PUSHAB\t%%L")
	k.c("CALLS\t#%d,G^SYS$CHKPRO", args)
	k.c("MOVL\tR0,R2")
	k.c("SAY\t</  $CHKPRO r0=!XL/>,R2")
	k.code = append(k.code, after...)
}

// ---------------------------------------------------------------------
// $CHECK_ACCESS.

type checkOpts struct {
	note   string
	objtyp string // "none" for none
	clsnam string // the 8-argument form with this class name
	args   int
}

func checkAccess(part byte, spec, user string, list []item, o checkOpts) {
	what := ""
	if o.clsnam != "" {
		what = " class " + o.clsnam
	}

	if o.objtyp != "" && o.objtyp != "ACL$C_FILE" {
		what += " objtyp " + o.objtyp
	}

	k := newCase(part, "$CHECK_ACCESS %s user %q%s%s", spec, user, what, o.note)
	k.ascid("%NM", spec)
	k.ascid("%UN", user)

	if o.clsnam != "" {
		k.ascid("%CN", o.clsnam)
	}

	objtyp := "ACL$C_FILE"
	if o.objtyp != "" {
		objtyp = o.objtyp
	}

	if objtyp != "none" {
		k.d("%%OT:\t.LONG\t%s", codeOf(objtyp))
	}

	args := 4
	if o.clsnam != "" {
		args = 8
	}

	if o.args != 0 {
		args = o.args
	}

	before, after := k.items(list)
	k.code = append(k.code, before...)

	switch {
	case args == 8:
		k.c("CLRQ\t-(SP)")

		if o.clsnam != "" {
			k.c("PUSHAQ\t%%CN")
		} else {
			k.c("CLRL\t-(SP)")
		}

		k.c("CLRL\t-(SP)")
	case args > 4:
		for range args - 4 {
			k.c("CLRL\t-(SP)")
		}
	}

	k.c("PUSHAB\t%%L")
	k.c("PUSHAQ\t%%UN")
	k.c("PUSHAQ\t%%NM")

	if objtyp == "none" {
		k.c("CLRL\t-(SP)")
	} else {
		k.c("PUSHAL\t%%OT")
	}

	k.c("CALLS\t#%d,G^SYS$CHECK_ACCESS", args)
	k.c("MOVL\tR0,R2")
	k.c("SAY\t</  $CHECK_ACCESS r0=!XL/>,R2")
	k.code = append(k.code, after...)
}

// ---------------------------------------------------------------------
// $GET_SECURITY and $SET_SECURITY.

type secOpts struct {
	clsnam string // "-" for none
	objnam string // "-" for none
	handle bool   // objhan: a channel from OPENCHAN of the file
	flags  string
	ctx    string // "" none, "new" (CONTEXT cleared first), "keep"
	acmode int    // -1 none
	show   string // a file to show after
	note   string
	fac    string // OPENCHAN's access ("GET" unless given)
}

func security(part byte, service, spec string, list []item, o secOpts) {
	cls := o.clsnam
	if cls == "" {
		cls = "FILE"
	}

	name := spec
	if o.objnam != "" {
		name = o.objnam
	}

	what := fmt.Sprintf("$%s %s %s", service, cls, name)
	if o.handle {
		what = fmt.Sprintf("$%s %s handle of %s", service, cls, spec)
	}

	if o.flags != "" {
		what += " flags " + o.flags
	}

	if o.ctx != "" {
		what += " context " + o.ctx
	}

	k := newCase(part, "%s%s", what, o.note)

	if cls != "-" {
		k.ascid("%CN", cls)
	}

	if name != "-" {
		k.ascid("%NM", name)
	}

	if o.handle {
		k.ascid("%HN", spec)
	}

	k.d("%%AM:\t.LONG\t%d", max(o.acmode, 0))
	before, after := k.items(list)

	if o.ctx == "new" {
		k.c("CLRL\tCONTEXT")
	}

	if o.handle {
		fac := "GET"
		if o.fac != "" {
			fac = o.fac
		}

		k.c("PUSHL\t#FAB$M_%s", fac)
		k.c("PUSHAQ\t%%HN")
		k.c("CALLS\t#2,OPENCHAN")
	}

	k.code = append(k.code, before...)

	if o.acmode >= 0 {
		k.c("PUSHAL\t%%AM")
	} else {
		k.c("CLRL\t-(SP)")
	}

	if o.ctx != "" {
		k.c("PUSHAL\tCONTEXT")
	} else {
		k.c("CLRL\t-(SP)")
	}

	k.c("PUSHAB\t%%L")

	if o.flags != "" {
		for _, f := range strings.FieldsFunc(o.flags, func(r rune) bool { return r == '<' || r == '>' || r == '!' }) {
			sym(f)
		}

		k.c("PUSHL\t#%s", o.flags)
	} else {
		k.c("CLRL\t-(SP)")
	}

	if o.handle {
		k.c("PUSHAL\tCHAN")
	} else {
		k.c("CLRL\t-(SP)")
	}

	if name != "-" && (!o.handle || o.objnam != "") {
		k.c("PUSHAQ\t%%NM")
	} else {
		k.c("CLRL\t-(SP)")
	}

	if cls != "-" {
		k.c("PUSHAQ\t%%CN")
	} else {
		k.c("CLRL\t-(SP)")
	}

	k.c("CALLS\t#7,G^SYS$%s", service)
	k.c("MOVL\tR0,R2")

	if o.ctx != "" {
		k.c("SAY\t</  $%s r0=!XL contxt=!XL/>,R2,CONTEXT", service)
	} else {
		k.c("SAY\t</  $%s r0=!XL/>,R2", service)
	}

	k.code = append(k.code, after...)

	if o.handle {
		k.c("CALLS\t#0,CLOSECHAN")
	}

	if o.show != "" {
		k.show(o.show)
	}
}

func get(part byte, spec string, list []item, o secOpts) {
	security(part, "GET_SECURITY", spec, list, o)
}
func set(part byte, spec string, list []item, o secOpts) {
	security(part, "SET_SECURITY", spec, list, o)
}

// ---------------------------------------------------------------------
// The program's fixed parts.

const programHead = `	.TITLE	SEC93	Phase 93's security services probe
	.IDENT	/V1.0/
;
; Written by testdata/prot93/gen3.go: do not edit. Round 3 of Phase 93's
; probe (docs/planning/PHASE-93 - file protection.md): calls the security
; services case by case, each case a routine of its own under HANDLER,
; and prints what each returned. SEC93_PART names the parts to run, a
; letter each (all of them if it isn't defined):
;
;	P $PARSE_ACL	F $FORMAT_ACL	C $CHANGE_ACL	K $CHKPRO
;	H $CHECK_ACCESS	G $GET_SECURITY	S $SET_SECURITY	D $SETDFPROT
;	Q $CHKPRO with the caller's own rights and privileges
;	M SYSTEM's cases without privileges	U PROT93C's cases
;
	$SSDEF
	$LNMDEF
	$FABDEF
	$XABDEF
	$XABPRODEF

; SAY text, a1, ... a10: format text with $FAOL and the arguments (each
; a longword, pushed by value) and write the line.
	.MACRO	SAY	TEXT,A1,A2,A3,A4,A5,A6,A7,A8,A9,A10,?LBL,?OVER
	BRW	OVER
LBL:	.ASCID	TEXT
OVER:
	$$N = 1
	.IRP	ARG,<A10,A9,A8,A7,A6,A5,A4,A3,A2,A1>
	.IF	NOT_BLANK,<ARG>
	PUSHL	ARG
	$$N = $$N + 1
	.ENDC
	.ENDR
	PUSHAQ	LBL
	CALLS	#$$N,SAYR
	.ENDM	SAY

; RMS service, fab: an RMS service with no completion routines.
	.MACRO	RMS	SERVICE,FAB
	CLRQ	-(SP)
	PUSHAB	FAB
	CALLS	#3,G^SYS$'SERVICE
	.ENDM	RMS
`

const programData = `
	.PSECT	DATA,WRT,NOEXE,LONG
SIGCOND: .LONG	0		; the condition HANDLER was given
SIGARG1: .LONG	0		; an access violation's reason mask
SIGARG2: .LONG	0		; and address
CONTEXT: .LONG	0		; the context cases share
CHAN:	.LONG	0		; OPENCHAN's channel
PARTBUF: .BLKB	32
PARTLEN: .LONG	0
TABNAM:	.ASCID	/LNM$FILE_DEV/
PARTNAM: .ASCID	/SEC93_PART/
TRNLST:	.WORD	32,LNM$_STRING
	.ADDRESS PARTBUF,PARTLEN
	.LONG	0
DEFPART: .ASCII	/PFCKHGSDQ/
DEFLEN	= 9
OUTD:	.LONG	600
	.ADDRESS OUTBUF
OUTBUF:	.BLKB	600
OUTLEN:	.WORD	0,0
HEXTAB:	.ASCII	/0123456789ABCDEF/
LINE:	.BLKB	80
FMTBUF:	.BLKB	1024
FMTD:	.LONG	1024
	.ADDRESS FMTBUF
FMTLEN:	.WORD	0,0
ACED:	.LONG	0,0

; The probe's own names for the access bits, for the accnam argument:
; LOOK, TOUCH, RUN, KILL, OWN, then X5 to X31.
	.ALIGN	LONG
ACCNAM:
	.LONG	4
	.ADDRESS AN0
	.LONG	5
	.ADDRESS AN1
	.LONG	3
	.ADDRESS AN2
	.LONG	4
	.ADDRESS AN3
	.LONG	3
	.ADDRESS AN4
	$$I = 5
	.REPT	5
	.LONG	2
	.ADDRESS ANX+<<$$I-5>*2>
	$$I = $$I + 1
	.ENDR
	.REPT	22
	.LONG	3
	.ADDRESS ANY+<<$$I-10>*3>
	$$I = $$I + 1
	.ENDR
AN0:	.ASCII	/LOOK/
AN1:	.ASCII	/TOUCH/
AN2:	.ASCII	/RUN/
AN3:	.ASCII	/KILL/
AN4:	.ASCII	/OWN/
ANX:	.ASCII	/X5X6X7X8X9/
ANY:	.ASCII	/X10X11X12X13X14X15X16X17X18X19X20X21X22X23X24X25X26X27X28X29X30X31/

; SHOWFILE's blocks.
	.ALIGN	LONG
FABS:	$FAB	FAC=GET,SHR=<GET,PUT,UPD,DEL>
	.ALIGN	LONG
XABS:	$XABPRO	ACLBUF=ACLB,ACLSIZ=512
ACLB:	.BLKB	512
	.ALIGN	LONG
FABC:	$FAB	FOP=UFO,SHR=GET
`

const programCode = `
	.PSECT	CODE,NOWRT,EXE

	.ENTRY	START,^M<R2,R3,R4,R5,R6,R7,R8>
	$TRNLNM_S TABNAM=TABNAM,LOGNAM=PARTNAM,ITMLST=TRNLST
	BLBS	R0,2$
	MOVC3	#DEFLEN,DEFPART,PARTBUF
	MOVL	#DEFLEN,PARTLEN
2$:	MOVZWL	PARTLEN,PARTLEN
	SAY	</parts !AD/>,PARTLEN,#PARTBUF
	MOVAB	CASES,R6
10$:	MOVL	(R6)+,R7
	BEQL	90$
	MOVL	(R6)+,R8
	LOCC	R8,PARTLEN,PARTBUF
	BEQL	10$
	CLRL	SIGCOND
	CALLS	#0,(R7)
	TSTL	SIGCOND
	BEQL	10$
	SAY	</  signal !XL !XL !XL/>,SIGCOND,SIGARG1,SIGARG2
	BRB	10$
90$:	SAY	</end of cases/>
	MOVL	#SS$_NORMAL,R0
	RET

; HANDLER records the condition (and an access violation's reason mask
; and address), then unwinds to the caller of the routine that
; established it, which gets the condition in R0. It ignores SS$_UNWIND.
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0
	CMPL	4(R0),#SS$_UNWIND
	BEQL	9$
	MOVL	4(R0),SIGCOND
	CLRQ	SIGARG1
	CMPL	SIGCOND,#SS$_ACCVIO
	BNEQ	1$
	MOVQ	8(R0),SIGARG1
1$:	MOVL	8(AP),R1
	MOVL	SIGCOND,12(R1)
	$UNWIND_S
9$:	RET

; SAYR text, a1, ...: format and write one line.
	.ENTRY	SAYR,^M<R2,R3,R4,R5>
	MOVL	#600,OUTD
	$FAOL_S	CTRSTR=@4(AP),OUTLEN=OUTLEN,OUTBUF=OUTD,PRMLST=8(AP)
	MOVZWL	OUTLEN,OUTD
	PUSHAQ	OUTD
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#600,OUTD
	RET

; HEXLINES address, length: the bytes in hex, 16 to a line.
	.ENTRY	HEXLINES,^M<R2,R3,R4,R5,R6>
	MOVL	4(AP),R2
	MOVL	8(AP),R3
1$:	TSTL	R3
	BLEQ	9$
	MOVAB	LINE,R4
	MOVL	#^X20202020,(R4)+
	MOVL	#16,R5
2$:	MOVZBL	(R2)+,R6
	EXTZV	#4,#4,R6,R0
	MOVB	HEXTAB[R0],(R4)+
	EXTZV	#0,#4,R6,R0
	MOVB	HEXTAB[R0],(R4)+
	MOVB	#^A/ /,(R4)+
	DECL	R3
	BLEQ	3$
	SOBGTR	R5,2$
3$:	MOVAB	LINE,R0
	SUBL3	R0,R4,R0
	DECL	R0
	SAY	</!AD/>,R0,#LINE
	BRB	1$
9$:	RET

; ACLWALK address, length: each ACE in the bytes, in hex, then as
; $FORMAT_ACL writes it. It stops at an ACE of size 0, and says so if an
; ACE claims more bytes than are left.
	.ENTRY	ACLWALK,^M<R2,R3,R4>
	MOVL	4(AP),R2
	MOVL	8(AP),R3
1$:	TSTL	R3
	BLEQ	9$
	MOVZBL	(R2),R4
	BEQL	9$
	CMPL	R4,R3
	BGTR	8$
	PUSHL	R4
	PUSHL	R2
	CALLS	#2,HEXLINES
	PUSHL	R4
	PUSHL	R2
	CALLS	#2,FORMAT1
	ADDL2	R4,R2
	SUBL2	R4,R3
	BRB	1$
8$:	SAY	</    ACE size !UL with !UL bytes left/>,R4,R3
	PUSHL	R3
	PUSHL	R2
	CALLS	#2,HEXLINES
9$:	RET

; FORMAT1 address, length: one ACE as $FORMAT_ACL writes it, by default.
	.ENTRY	FORMAT1,^M<R2>
	MOVAB	HANDLER,(FP)
	MOVL	4(AP),ACED+4
	MOVL	8(AP),ACED
	MOVL	#1024,FMTD
	CLRL	FMTLEN
	CLRQ	-(SP)
	CLRQ	-(SP)
	CLRQ	-(SP)
	PUSHAQ	FMTD
	PUSHAW	FMTLEN
	PUSHAQ	ACED
	CALLS	#9,G^SYS$FORMAT_ACL
	MOVL	R0,R2
	MOVZWL	FMTLEN,R0
	BLBC	R2,5$
	SAY	</      !AD/>,R0,#FMTBUF
	RET
5$:	SAY	</      $FORMAT_ACL r0=!XL len=!UL/>,R2,R0
	RET

; SHOWOUT name, retlen, buffer, size, kind: an output item. A return
; length address of 0 means the size; a length past the size is cut to it.
	.ENTRY	SHOWOUT,^M<R2,R3,R4,R5>
	MOVL	12(AP),R3
	MOVL	16(AP),R4
	MOVL	8(AP),R2
	BEQL	1$
	MOVZWL	(R2),R5
	SAY	</  !AS retlen=!UL/>,4(AP),R5
	BRB	2$
1$:	MOVL	R4,R5
	SAY	</  !AS/>,4(AP)
2$:	CMPL	20(AP),#3
	BNEQ	3$
	MOVL	R4,R5
3$:	CMPL	R5,R4
	BLEQ	4$
	MOVL	R4,R5
4$:	CMPL	20(AP),#1
	BNEQ	5$
	SAY	</    "!AF"/>,R5,R3
	RET
5$:	CMPL	20(AP),#2
	BNEQ	6$
	PUSHL	R5
	PUSHL	R3
	CALLS	#2,ACLWALK
	RET
6$:	PUSHL	R5
	PUSHL	R3
	CALLS	#2,HEXLINES
	RET

; SHOWFILE name: the file's owner, protection, and ACL, by RMS's $OPEN
; and $DISPLAY with a XABPRO.
	.ENTRY	SHOWFILE,^M<R2,R3,R4,R5>
	MOVAB	HANDLER,(FP)
	MOVL	4(AP),R2
	MOVB	(R2),FABS+FAB$B_FNS
	MOVL	4(R2),FABS+FAB$L_FNA
	MOVC5	#0,(SP),#0,#512,ACLB
	CLRL	XABS+XAB$L_ACLCTX
	CLRW	XABS+XAB$W_ACLLEN
	CLRL	XABS+XAB$L_ACLSTS
	CLRL	XABS+XAB$L_UIC
	CLRW	XABS+XAB$W_PRO
	MOVAB	XABS,FABS+FAB$L_XAB
	RMS	OPEN,FABS
	BLBS	R0,10$
	MOVL	FABS+FAB$L_STS,R2
	MOVL	FABS+FAB$L_STV,R3
	SAY	</  file $OPEN sts=!XL stv=!XL/>,R2,R3
	RET
10$:	RMS	DISPLAY,FABS
	MOVL	XABS+XAB$L_UIC,R2
	MOVZWL	XABS+XAB$W_PRO,R3
	MOVZWL	XABS+XAB$W_ACLLEN,R4
	MOVL	XABS+XAB$L_ACLSTS,R5
	SAY	</  file owner=!XL pro=!XW acllen=!UL aclsts=!XL/>,R2,R3,R4,R5
	CMPL	R4,#512
	BLEQ	20$
	MOVL	#512,R4
20$:	PUSHL	R4
	PUSHAB	ACLB
	CALLS	#2,ACLWALK
	RMS	CLOSE,FABS
	RET

; OPENCHAN name, fac: open the file for user file open with the access
; given, and keep the channel RMS assigned in CHAN.
	.ENTRY	OPENCHAN,^M<R2,R3>
	MOVL	4(AP),R2
	MOVB	(R2),FABC+FAB$B_FNS
	MOVL	4(R2),FABC+FAB$L_FNA
	MOVB	8(AP),FABC+FAB$B_FAC
	CLRL	CHAN
	RMS	OPEN,FABC
	MOVL	FABC+FAB$L_STS,R2
	MOVL	FABC+FAB$L_STV,R3
	SAY	</  open for a channel sts=!XL stv=!XL/>,R2,R3
	BLBC	R2,9$
	MOVL	R3,CHAN
9$:	RET

; CLOSECHAN: deassign CHAN.
	.ENTRY	CLOSECHAN,^M<>
	TSTL	CHAN
	BEQL	9$
	$DASSGN_S CHAN=CHAN
	CLRL	CHAN
9$:	RET
`

func program() string {
	var b strings.Builder

	b.WriteString(programHead)

	// The definitions the cases use.
	var names []string
	for n := range used {
		names = append(names, n)
	}

	sort.Strings(names)
	b.WriteString("\n; Values from round 1 (testdata/prot93/defined.txt).\n")

	for _, n := range names {
		fmt.Fprintf(&b, "%s = ^X%X\n", n, defined[n])
	}

	b.WriteString(programData)

	for i, k := range cases {
		n := i + 1
		p := fmt.Sprintf("C%d_", n)
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ALIGN\tLONG\n", n, k.title)

		title := fmt.Sprintf("== %d %c %s", n, k.part, k.title)
		fmt.Fprintf(&b, "%sT:\t.ASCID\t%s%s%s\n", p, delim(title), title, delim(title))

		for _, l := range k.data {
			b.WriteString(strings.ReplaceAll(strings.ReplaceAll(l, "%", p), pct, "%") + "\n")
		}
	}

	b.WriteString("\n; The cases and their parts.\n\t.ALIGN\tLONG\nCASES:\n")

	for i, k := range cases {
		fmt.Fprintf(&b, "\t.ADDRESS C%d\n\t.LONG\t^A/%c/\n", i+1, k.part)
	}

	b.WriteString("\t.LONG\t0,0\n")
	b.WriteString(programCode)

	for i, k := range cases {
		n := i + 1
		p := fmt.Sprintf("C%d_", n)
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHANDLER,(FP)\n", n, k.title, n)
		fmt.Fprintf(&b, "\tSAY\t</!AS/>,#%sT\n", p)

		for _, l := range k.code {
			l = strings.ReplaceAll(l, "%", p)
			if strings.HasSuffix(l, ":") {
				b.WriteString(l + "\n")
			} else {
				b.WriteString("\t" + l + "\n")
			}
		}

		b.WriteString("\tRET\n")
	}

	b.WriteString("\n\t.END\tSTART\n")

	return b.String()
}

// ---------------------------------------------------------------------
// The cases themselves.

// The tree's directories, under [SEC93].
const (
	chgDir = disk + "[SEC93.CHG]"
	getDir = disk + "[SEC93.GET]"
	setDir = disk + "[SEC93.SET]"
	chkDir = disk + "[SEC93.CHK]"
	npDir  = disk + "[SEC93.NP]"
)

func parseCases() {
	texts := []string{
		"(IDENTIFIER=[200,1],ACCESS=READ)",
		"(IDENTIFIER=PROT93A,ACCESS=READ)",
		"(IDENTIFIER=[PROT93,PROT93A],ACCESS=READ)",
		"(IDENTIFIER=[200,*],ACCESS=READ)",
		"(IDENTIFIER=[*,*],ACCESS=READ)",
		"(IDENTIFIER=[*,1],ACCESS=READ)",
		"(IDENTIFIER=[310,1],ACCESS=READ)",
		"(IDENTIFIER=[1,4],ACCESS=READ)",
		"(IDENTIFIER=[SYSTEM],ACCESS=READ)",
		"(IDENTIFIER=SYSTEM,ACCESS=READ)",
		"(IDENTIFIER=PROT93,ACCESS=READ)",
		"(IDENTIFIER=PROT93_TEAM,ACCESS=READ)",
		"(IDENTIFIER=prot93_team,ACCESS=READ)",
		"(IDENTIFIER=INTERACTIVE+LOCAL,ACCESS=READ)",
		"(IDENTIFIER=[200,1]+PROT93_TEAM+INTERACTIVE,ACCESS=READ)",
		"(IDENTIFIER=[200,1]+[200,2]+[300,1]+PROT93_TEAM,ACCESS=READ)",
		"(IDENTIFIER=%X80010001,ACCESS=READ)",
		"(IDENTIFIER=[200,1],ACCESS=READ+WRITE+EXECUTE+DELETE+CONTROL)",
		"(IDENTIFIER=[200,1],ACCESS=CONTROL+READ)",
		"(IDENTIFIER=[200,1],ACCESS=NONE)",
		"(IDENTIFIER=[200,1])",
		"(IDENTIFIER=[200,1],ACCESS=READ+NONE)",
		"(IDENTIFIER=[200,1],ACCESS=BIT_5+BIT_31)",
		"(IDENTIFIER=[200,1],ACCESS=R)",
		"(IDENTIFIER=[200,1],ACCESS=RE+WR)",
		"(ID=[200,1],A=READ)",
		"(IDENT=[200,1],ACC=READ+WRI)",
		"(identifier=[200,1],access=read)",
		"( IDENTIFIER = [200,1] , ACCESS = READ )",
		"(IDENTIFIER=[200,1],ACCESS=READ)   ",
		"(IDENTIFIER=[200,1],OPTIONS=DEFAULT,ACCESS=READ)",
		"(IDENTIFIER=[200,1],OPTIONS=PROTECTED+NOPROPAGATE,ACCESS=READ)",
		"(IDENTIFIER=[200,1],OPTIONS=HIDDEN,ACCESS=READ)",
		"(IDENTIFIER=[200,1],OPTIONS=NONE,ACCESS=READ)",
		"(IDENTIFIER=[200,1],OPTIONS=DEFAULT+PROTECTED+NOPROPAGATE+HIDDEN,ACCESS=READ)",
		"(IDENTIFIER=[200,1],ACCESS=READ,OPTIONS=DEFAULT)",
		"(IDENTIFIER=[200,1],OPTIONS=DEFAULT,OPTIONS=PROTECTED,ACCESS=READ)",
		"(IDENTIFIER=[200,1],ACCESS=READ,ACCESS=WRITE)",
		"(DEFAULT_PROTECTION,S:RWED,O:RWED,G:RE,W:)",
		"(DEFAULT_PROTECTION,SYSTEM:RWED,OWNER:RWED,GROUP:R,WORLD:E)",
		"(DEFAULT_PROTECTION,OPTIONS=PROTECTED,S:RWED,O:RWED,G,W)",
		"(DEFAULT_PROTECTION,W:R)",
		"(DEFAULT_PROTECTION,S=RWED,O=RWED)",
		"(DEFAULT_PROTECTION,S:RWEDC)",
		"(DEFAULT_PROTECTION)",
		"(DEFAULT_PROTECTION,(S:RWED,O:RWED,G:RE,W))",
		"(DEFAULT_PROTECTION,S:RWED,S:R)",
		"(DEFAULT_PROTECTION,X:R)",
		"(CREATOR,ACCESS=READ+WRITE)",
		"(CREATOR,OPTIONS=DEFAULT,ACCESS=READ)",
		"(CREATOR)",
		"(ALARM_JOURNAL=SECURITY,ACCESS=READ+SUCCESS)",
		"(ALARM=SECURITY,ACCESS=READ+WRITE+SUCCESS+FAILURE)",
		"(AUDIT=SECURITY,ACCESS=DELETE+FAILURE)",
		"(ALARM=OTHER,ACCESS=READ+SUCCESS)",
		"(ALARM=SECURITY,ACCESS=READ)",
		"(ALARM=SECURITY,OPTIONS=PROTECTED,ACCESS=WRITE+FAILURE)",
		"(SUBSYSTEM,IDENTIFIER=PROT93_TEAM)",
		"(SUBSYSTEM,IDENTIFIER=PROT93_TEAM,ACCESS=READ)",
		"(IDENTIFIER=NOSUCH,ACCESS=READ)",
		"(IDENTIFIER=[200,1],ACCESS=READX)",
		"(IDENTIFIER=[200,1],ACCESS=READ",
		"",
		"()",
		"(IDENTIFIER=,ACCESS=READ)",
		"(IDENTIFIER=[200,1],ACCESS=READ)X",
		"(IDENTIFIER=[200,1],ACCESS=READ)(IDENTIFIER=[200,2],ACCESS=READ)",
		"(IDENTIFIER=[200,1],ACCESS=READ),(IDENTIFIER=[200,2],ACCESS=READ)",
		"(IDENTIFIER=[400000,1],ACCESS=READ)",
		"(IDENTIFIER=[1,99999],ACCESS=READ)",
		"(IDENTIFIER=[200],ACCESS=READ)",
		"(IDENTIFIER=[200,1,2],ACCESS=READ)",
		"(FOO=1)",
		"IDENTIFIER=[200,1],ACCESS=READ",
		"(IDENTIFIER=[200,1],FOO=BAR,ACCESS=READ)",
		"(IDENTIFIER=[200,1],OPTIONS=NOSUCH,ACCESS=READ)",
	}

	for _, t := range texts {
		parse('P', t, parseOpts{})
	}

	const one = "(IDENTIFIER=[200,1],ACCESS=READ)"
	parse('P', one, parseOpts{bufLen: 8, dump: 16})
	parse('P', one, parseOpts{bufLen: 12, dump: 16})
	parse('P', one, parseOpts{bufLen: 11, dump: 16})
	parse('P', one, parseOpts{bufLen: 0x10000 - 1})
	parse('P', one, parseOpts{noErrpos: true})
	parse('P', one, parseOpts{args: 4})
	parse('P', "(IDENTIFIER=[200,1],ACCESS=LOOK+TOUCH+OWN)", parseOpts{accnam: true})
	parse('P', "(IDENTIFIER=[200,1],ACCESS=READ)", parseOpts{accnam: true})
	parse('P', "(IDENTIFIER=[200,1],ACCESS=X5+X31)", parseOpts{accnam: true})
	parse('P', "(DEFAULT_PROTECTION,S:RWED,O:RWED,G:RE,W:)", parseOpts{accnam: true})
	parse('P', "(ALARM=SECURITY,ACCESS=LOOK+SUCCESS)", parseOpts{accnam: true})
}

func formatCases() {
	type f struct {
		what string
		b    []byte
	}

	plain := []f{
		{"[200,1] READ", idACE(0, aR, idA)},
		{"[200,1] all", idACE(0, 0x1F, idA)},
		{"[200,1] none", idACE(0, 0, idA)},
		{"[200,1] bit 5", idACE(0, 0x20, idA)},
		{"[200,1] bit 31", idACE(0, 0x80000000, idA)},
		{"[200,1] every bit", idACE(0, 0xFFFFFFFF, idA)},
		{"[200,2] READ", idACE(0, aR, idB)},
		{"[300,1] READ", idACE(0, aR, idC)},
		{"[1,4] READ", idACE(0, aR, idSys)},
		{"[310,1] READ", idACE(0, aR, id310)},
		{"[0,0] READ", idACE(0, aR, idZero)},
		{"[200,*] READ", idACE(0, aR, idG200)},
		{"[*,*] READ", idACE(0, aR, idAll)},
		{"[*,1] READ", idACE(0, aR, idAny1)},
		{"[377,*] READ", idACE(0, aR, 0x00FFFFFF)},
		{"PROT93_TEAM READ", idACE(0, aR, idTeam)},
		{"%X8001FFFF READ", idACE(0, aR, idNone)},
		{"INTERACTIVE READ", idACE(0, aR, idInter)},
		{"LOCAL READ", idACE(0, aR, idLocal)},
		{"%X40000001 READ", idACE(0, aR, 0x40000001)},
		{"three identifiers", idACE(0, aR|aW, idA, idTeam, idInter)},
		{"no identifier", idACE(0, aR)},
		{"DEFAULT", idACE(oDefault, aR, idA)},
		{"PROTECTED", idACE(oProt, aR, idA)},
		{"NOPROPAGATE", idACE(oNoProp, aR, idA)},
		{"HIDDEN", idACE(oHidden, aR, idA)},
		{"every option", idACE(oDefault|oProt|oHidden|oNoProp, aR, idA)},
		{"flag bit 12", idACE(0x1000, aR, idA)},
		{"one reserved longword", ace(tKeyID, 1, le32(aR), le32(0x12345678), le32(idA))},
		{"two reserved longwords", ace(tKeyID, 2, le32(aR), le32(0x12345678), le32(0x9ABCDEF0), le32(idA))},
		{"S:RWED,O:RWED,G:RE,W:", dirdefACE(0, "RWED", "RWED", "RE", "")},
		{"default protection all none", dirdefACE(0, "", "", "", "")},
		{"default protection all RWED", dirdefACE(0, "RWED", "RWED", "RWED", "RWED")},
		{"default protection PROTECTED", dirdefACE(oProt, "RWED", "RWED", "", "")},
		{"default protection without C", ace(tDirDef, 0, le32(0), le32(0), le32(0), le32(0x0A), le32(0x0F))},
		{"default protection spare 1", ace(tDirDef, 0, le32(1), le32(0x10), le32(0x10), le32(0x1A), le32(0x1F))},
		{"default protection short", ace(tDirDef, 0, le32(0), le32(0x10))},
		{"creator READ+WRITE", creatorACE(0, aR|aW)},
		{"creator none", creatorACE(0, 0)},
		{"creator DEFAULT", creatorACE(oDefault, aR)},
		{"alarm SUCCESS+FAILURE", alarmACE(tAlarm, 3, aR|aW, "SECURITY")},
		{"alarm SUCCESS", alarmACE(tAlarm, 1, aR, "SECURITY")},
		{"alarm FAILURE", alarmACE(tAlarm, 2, aR, "SECURITY")},
		{"alarm neither", alarmACE(tAlarm, 0, aR, "SECURITY")},
		{"alarm other name", alarmACE(tAlarm, 1, aR, "JOURNAL1")},
		{"alarm no name", alarmACE(tAlarm, 1, aR, "")},
		{"audit DELETE+FAILURE", alarmACE(tAudit, 2, aD, "SECURITY")},
		{"subsystem PROT93_TEAM", ace(tSubsys, 0, le32(aR), le32(idTeam))},
		{"subsystem no access word", ace(tSubsys, 0, le32(idTeam))},
		{"application customer", ace(tInfo, 1, le32(0x0000ABCD), []byte("DATA"))},
		{"application CSS", ace(tInfo, 2, le32(0x00001234), []byte("DATA"))},
		{"application VMS", ace(tInfo, 3, le32(0), []byte("DATAMORE"))},
		{"type 2", ace(2, 0, le32(0), le32(0))},
		{"type 3", ace(3, 0, le32(0), le32(0))},
		{"type 8", ace(8, 0, le32(0), le32(0))},
		{"type 10", ace(10, 0, le32(0), le32(0))},
		{"type 12", ace(12, 0, le32(0), le32(0))},
		{"type 14", ace(14, 0, le32(0), le32(0))},
		{"type 0", ace(0, 0, le32(0))},
		{"type 32", ace(32, 0, le32(0))},
		{"type 128", ace(0x80, 0, le32(0))},
		{"type 255", ace(0xFF, 0, le32(0))},
	}

	for _, c := range plain {
		format('F', c.what, c.b, fmtOpts{width: -1, indent: -1})
	}

	three := idACE(oProt|oNoProp, aR|aW|aE|aD|aC, idA, idTeam, idInter)

	// Malformed ACEs.
	format('F', "size 0", []byte{0, 1, 0, 0, 1, 0, 0, 0}, fmtOpts{width: -1, indent: -1})
	format('F', "size 4", []byte{4, 1, 0, 0, 1, 0, 0, 0}, fmtOpts{width: -1, indent: -1, descLen: 8})
	format('F', "size 2", []byte{2, 1, 0, 0}, fmtOpts{width: -1, indent: -1, descLen: 4})
	format('F', "identifier ACE of 8 bytes", []byte{8, 1, 0, 0, 1, 0, 0, 0}, fmtOpts{width: -1, indent: -1})
	format('F', "identifier ACE of 10 bytes", []byte{10, 1, 0, 0, 1, 0, 0, 0, 1, 0}, fmtOpts{width: -1, indent: -1})
	format('F', "[200,1] READ", idACE(0, aR, idA), fmtOpts{width: -1, indent: -1, descLen: 8, note: " [descriptor 8]"})
	format('F', "two ACEs", cat(idACE(0, aR, idA), idACE(0, aW, idB)), fmtOpts{width: -1, indent: -1, note: " [descriptor 24]"})
	format('F', "[200,1] READ", idACE(0, aR, idA), fmtOpts{width: -1, indent: -1, descLen: 0x100, note: " [descriptor 256]"})

	// Width, terminators, indentation.
	format('F', "three identifiers", three, fmtOpts{width: 30, indent: 4, trm: []byte("/"), note: " [width 30, trm /, indent 4]"})
	format('F', "three identifiers", three, fmtOpts{width: 20, indent: 0, trm: []byte("\r\n"), note: " [width 20, trm CRLF, indent 0]"})
	format('F', "three identifiers", three, fmtOpts{width: 5, indent: -1, trm: []byte("/"), note: " [width 5, trm /]"})
	format('F', "three identifiers", three, fmtOpts{width: 0, indent: -1, trm: []byte("/"), note: " [width 0, trm /]"})
	format('F', "three identifiers", three, fmtOpts{width: -1, indent: 8, note: " [indent 8]"})
	format('F', "three identifiers", three, fmtOpts{width: 30, indent: -1, note: " [width 30, no trm]"})
	format('F', "three identifiers", three, fmtOpts{width: 30, indent: 2, trm: []byte{}, note: " [width 30, empty trm, indent 2]"})
	format('F', "three identifiers", three, fmtOpts{width: 40, indent: 10, trm: []byte("<>"), note: " [width 40, trm <>, indent 10]"})
	format('F', "default protection", dirdefACE(0, "RWED", "RWED", "RE", ""), fmtOpts{width: 25, indent: 3, trm: []byte("/"), note: " [width 25, trm /, indent 3]"})
	format('F', "alarm", alarmACE(tAlarm, 3, aR|aW, "SECURITY"), fmtOpts{width: 20, indent: 3, trm: []byte("/"), note: " [width 20, trm /, indent 3]"})

	// The output buffer.
	format('F', "[200,1] READ", idACE(0, aR, idA), fmtOpts{width: -1, indent: -1, outLen: 10, note: " [aclstr 10]"})
	format('F', "[200,1] READ", idACE(0, aR, idA), fmtOpts{width: -1, indent: -1, outLen: 0, note: " [aclstr 0]"})
	format('F', "three identifiers", three, fmtOpts{width: 30, indent: 4, trm: []byte("/"), outLen: 40, note: " [aclstr 40, width 30]"})
	format('F', "[200,1] READ", idACE(0, aR, idA), fmtOpts{width: -1, indent: -1, noLen: true, note: " [no acllen]"})

	// The probe's own access names.
	format('F', "[200,1] all", idACE(0, 0x1F, idA), fmtOpts{width: -1, indent: -1, accnam: true, note: " [access names]"})
	format('F', "[200,1] bits 5 and 31", idACE(0, 0x80000020, idA), fmtOpts{width: -1, indent: -1, accnam: true, note: " [access names]"})
	format('F', "default protection", dirdefACE(0, "RWED", "RWED", "RE", ""), fmtOpts{width: -1, indent: -1, accnam: true, note: " [access names]"})
	format('F', "alarm", alarmACE(tAlarm, 3, aR|aW, "SECURITY"), fmtOpts{width: -1, indent: -1, accnam: true, note: " [access names]"})
}

// The $CHANGE_ACL cases' files, in [SEC93.CHG].
func changeCases() {
	aRead := idACE(0, aR, idA)
	bRead := idACE(0, aR, idB)
	cRead := idACE(0, aR, idC)
	aWrite := idACE(0, aW, idA)
	aDel := idACE(0, aD, idA)
	aProt := idACE(oProt, aR, idA)
	team := idACE(0, aR, idTeam)

	// CA1: an ACL built and taken apart, one call at a time.
	ca1 := chgDir + "CA1.DAT"
	k := change('C', "ACLLENGTH", ca1, []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1})
	k.file(object{name: ca1, owner: "[200,1]", prot: "(S:RWED,O:RWED,G:RE,W:)"})
	change('C', "ADDACLENT [200,1] READ, no contxt", ca1, []item{in("ACL$C_ADDACLENT", aRead)}, changeOpts{acmode: -1})
	change('C', "ADDACLENT [200,2] READ, contxt 0", ca1, []item{in("ACL$C_ADDACLENT", bRead)}, changeOpts{acmode: -1, ctx: "0"})
	change('C', "ADDACLENT [300,1] READ, contxt -1", ca1, []item{in("ACL$C_ADDACLENT", cRead)}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT [200,1] WRITE, contxt -1 (matches a higher ACE)", ca1, []item{in("ACL$C_ADDACLENT", aWrite)}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT [200,1] DELETE, contxt 0 (matches a lower ACE)", ca1, []item{in("ACL$C_ADDACLENT", aDel)}, changeOpts{acmode: -1, ctx: "0"})
	change('C', "ADDACLENT [200,1] PROTECTED READ, contxt 0", ca1, []item{in("ACL$C_ADDACLENT", aProt)}, changeOpts{acmode: -1, ctx: "0"})
	change('C', "ACLLENGTH", ca1, []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1})
	change('C', "READACL, contxt 0", ca1, []item{outNR("ACL$C_READACL", 512, kACL)}, changeOpts{acmode: -1, ctx: "0"})
	change('C', "READACL 20 bytes, contxt 0", ca1, []item{outNR("ACL$C_READACL", 20, kAll)}, changeOpts{acmode: -1, ctx: "0", noShow: true})
	change('C', "READACL 20 bytes, contxt kept", ca1, []item{outNR("ACL$C_READACL", 20, kAll)}, changeOpts{acmode: -1, ctx: "keep", noShow: true})
	change('C', "READACL 20 bytes, contxt kept", ca1, []item{outNR("ACL$C_READACL", 20, kAll)}, changeOpts{acmode: -1, ctx: "keep", noShow: true})
	change('C', "READACL 20 bytes, contxt kept", ca1, []item{outNR("ACL$C_READACL", 20, kAll)}, changeOpts{acmode: -1, ctx: "keep", noShow: true})
	change('C', "READACL 8 bytes, contxt 0", ca1, []item{outNR("ACL$C_READACL", 8, kAll)}, changeOpts{acmode: -1, ctx: "0", noShow: true})
	change('C', "READACL, no contxt", ca1, []item{outNR("ACL$C_READACL", 512, kACL)}, changeOpts{acmode: -1, noShow: true})
	change('C', "DELACLENT [200,2] READ", ca1, []item{in("ACL$C_DELACLENT", bRead)}, changeOpts{acmode: -1})
	change('C', "DELACLENT [200,2] READ again", ca1, []item{in("ACL$C_DELACLENT", bRead)}, changeOpts{acmode: -1})
	change('C', "DELACLENT [300,1] WRITE (identifier and flags match)", ca1, []item{in("ACL$C_DELACLENT", idACE(0, aW, idC))}, changeOpts{acmode: -1})
	change('C', "DELETEACL", ca1, []item{zero("ACL$C_DELETEACL", 0)}, changeOpts{acmode: -1})
	change('C', "DELACLENT the protected ACE", ca1, []item{in("ACL$C_DELACLENT", aProt)}, changeOpts{acmode: -1})
	change('C', "ACLLENGTH", ca1, []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1})

	// CA2: finding, then changing in the same call.
	ca2 := chgDir + "CA2.DAT"
	k = change('C', "FNDACLENT [200,2] READ, MODACLENT [300,1] WRITE", ca2,
		[]item{in("ACL$C_FNDACLENT", bRead), in("ACL$C_MODACLENT", idACE(0, aW, idC))}, changeOpts{acmode: -1, ctx: "0"})
	k.file(object{name: ca2, owner: "[200,1]", prot: "(S:RWED,O:RWED,G:RE,W:)",
		acl: []string{"IDENTIFIER=[200,1],ACCESS=READ", "IDENTIFIER=[200,2],ACCESS=READ", "IDENTIFIER=PROT93_TEAM,ACCESS=READ"}})
	change('C', "FNDACLENT PROT93_TEAM READ, DELACLENT (no buffer)", ca2,
		[]item{in("ACL$C_FNDACLENT", team), zero("ACL$C_DELACLENT", 0)}, changeOpts{acmode: -1, ctx: "0"})
	change('C', "FNDACLENT [300,1] WRITE, READACE", ca2,
		[]item{in("ACL$C_FNDACLENT", idACE(0, aW, idC)), outNR("ACL$C_READACE", 64, kACL)}, changeOpts{acmode: -1, ctx: "0", noShow: true})
	change('C', "FNDACLENT [300,1] READ (identifier and flags match)", ca2,
		[]item{in("ACL$C_FNDACLENT", cRead), outNR("ACL$C_READACE", 64, kACL)}, changeOpts{acmode: -1, ctx: "0", noShow: true})
	change('C', "FNDACLENT [200,2] READ (not there)", ca2,
		[]item{in("ACL$C_FNDACLENT", bRead)}, changeOpts{acmode: -1, ctx: "0", noShow: true})
	change('C', "FNDACETYP identifier, READACE", ca2,
		[]item{in("ACL$C_FNDACETYP", []byte{12, tKeyID, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}), outNR("ACL$C_READACE", 64, kACL)}, changeOpts{acmode: -1, ctx: "0", noShow: true})
	change('C', "FNDACETYP alarm (not there)", ca2,
		[]item{in("ACL$C_FNDACETYP", []byte{8, tAlarm, 0, 0, 0, 0, 0, 0})}, changeOpts{acmode: -1, ctx: "0", noShow: true})
	change('C', "FNDACLENT [300,1] WRITE, ADDACLENT [200,2] DELETE", ca2,
		[]item{in("ACL$C_FNDACLENT", idACE(0, aW, idC)), in("ACL$C_ADDACLENT", idACE(0, aD, idB))}, changeOpts{acmode: -1, ctx: "0"})
	change('C', "NEXT_ACE, READACE, contxt 0", ca2,
		[]item{out("ACL$C_NEXT_ACE", 4, kHex), outNR("ACL$C_READACE", 64, kACL)}, changeOpts{acmode: -1, ctx: "0", noShow: true})
	change('C', "GRANT_ACE", ca2, []item{outNR("ACL$C_GRANT_ACE", 64, kACL)}, changeOpts{acmode: -1, noShow: true})
	change('C', "RLOCK_ACL, READACL, UNLOCK_ACL", ca2,
		[]item{zero("ACL$C_RLOCK_ACL", 4), outNR("ACL$C_READACL", 512, kACL), zero("ACL$C_UNLOCK_ACL", 4)}, changeOpts{acmode: -1, ctx: "0", noShow: true})
	change('C', "WLOCK_ACL, UNLOCK_ACL", ca2,
		[]item{zero("ACL$C_WLOCK_ACL", 4), zero("ACL$C_UNLOCK_ACL", 4)}, changeOpts{acmode: -1, noShow: true})
	change('C', "DELETE_ALL", ca2, []item{zero("ACL$C_DELETE_ALL", 0)}, changeOpts{acmode: -1})

	// CA3: matching by type and by flags and access.
	ca3 := chgDir + "CA3.DIR"
	k = change('C', "ADDACLENT default protection S:RWED,O:RWED,G:R,W:", ca3,
		[]item{in("ACL$C_ADDACLENT", dirdefACE(0, "RWED", "RWED", "R", ""))}, changeOpts{acmode: -1, ctx: "-1"})
	k.file(object{name: ca3, owner: "[200,1]", prot: "(S:RWE,O:RWE,G:RE,W:)", dir: true,
		acl: []string{"IDENTIFIER=[200,1],ACCESS=READ"}})
	change('C', "ADDACLENT default protection W:RE (matches by type)", ca3,
		[]item{in("ACL$C_ADDACLENT", dirdefACE(0, "RWED", "RWED", "RE", "RE"))}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT default protection at the top", ca3,
		[]item{in("ACL$C_ADDACLENT", dirdefACE(0, "RWED", "RWED", "", ""))}, changeOpts{acmode: -1, ctx: "0"})
	change('C', "ADDACLENT [200,1] DEFAULT READ", ca3,
		[]item{in("ACL$C_ADDACLENT", idACE(oDefault, aR, idA))}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT creator", ca3,
		[]item{in("ACL$C_ADDACLENT", creatorACE(0, aR|aW))}, changeOpts{acmode: -1, ctx: "-1"})
	ca4 := chgDir + "CA4.DAT"
	k = change('C', "ADDACLENT alarm SUCCESS READ", ca4,
		[]item{in("ACL$C_ADDACLENT", alarmACE(tAlarm, 1, aR, "SECURITY"))}, changeOpts{acmode: -1, ctx: "-1"})
	k.file(object{name: ca4, owner: "[200,1]", prot: "(S:RWED,O:RWED,G:RE,W:)"})
	change('C', "ADDACLENT alarm SUCCESS READ again", ca4,
		[]item{in("ACL$C_ADDACLENT", alarmACE(tAlarm, 1, aR, "SECURITY"))}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT alarm FAILURE READ", ca4,
		[]item{in("ACL$C_ADDACLENT", alarmACE(tAlarm, 2, aR, "SECURITY"))}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT alarm SUCCESS WRITE", ca4,
		[]item{in("ACL$C_ADDACLENT", alarmACE(tAlarm, 1, aW, "SECURITY"))}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT audit FAILURE DELETE", ca4,
		[]item{in("ACL$C_ADDACLENT", alarmACE(tAudit, 2, aD, "SECURITY"))}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT alarm SUCCESS READ, other name", ca4,
		[]item{in("ACL$C_ADDACLENT", alarmACE(tAlarm, 1, aR, "JOURNAL1"))}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT [200,1] READ with a reserved longword", ca4,
		[]item{in("ACL$C_ADDACLENT", ace(tKeyID, 1, le32(aR), le32(0x12345678), le32(idA)))}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT [200,1] HIDDEN READ", ca4,
		[]item{in("ACL$C_ADDACLENT", idACE(oHidden, aR, idA))}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "READACL", ca4, []item{outNR("ACL$C_READACL", 512, kACL)}, changeOpts{acmode: -1, ctx: "0", noShow: true})

	// CA5: several ACEs in one buffer.
	ca5 := chgDir + "CA5.DAT"
	k = change('C', "ADDACLENT three ACEs in one buffer", ca5,
		[]item{in("ACL$C_ADDACLENT", cat(aRead, bRead, team))}, changeOpts{acmode: -1, ctx: "0"})
	k.file(object{name: ca5, owner: "[200,1]", prot: "(S:RWED,O:RWED,G:RE,W:)", acl: []string{"IDENTIFIER=[300,1],ACCESS=READ"}})
	change('C', "ADDACLENT [200,1] WRITE, PROT93_TEAM WRITE in one buffer (both match)", ca5,
		[]item{in("ACL$C_ADDACLENT", cat(aWrite, idACE(0, aW, idTeam)))}, changeOpts{acmode: -1, ctx: "0"})
	change('C', "two ADDACLENT items, contxt 0", ca5,
		[]item{in("ACL$C_ADDACLENT", idACE(0, aE, idLocal)), in("ACL$C_ADDACLENT", idACE(0, aE, idInter))}, changeOpts{acmode: -1, ctx: "0"})
	change('C', "ADDACLENT a buffer longer than its ACE", ca5,
		[]item{inN("ACL$C_ADDACLENT", cat(idACE(0, aR, idSys), []byte{0, 0, 0, 0}), 16)}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT a buffer shorter than its ACE", ca5,
		[]item{inN("ACL$C_ADDACLENT", idACE(0, aR, id310), 8)}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT an ACE of size 3", ca5,
		[]item{in("ACL$C_ADDACLENT", []byte{3, 1, 0, 0})}, changeOpts{acmode: -1, ctx: "-1"})
	change('C', "ADDACLENT an ACE of type 200", ca5,
		[]item{in("ACL$C_ADDACLENT", ace(200, 0, le32(0)))}, changeOpts{acmode: -1, ctx: "-1"})

	// Arguments.
	ca6 := chgDir + "CA6.DAT"
	k = change('C', "ACLLENGTH", ca6, []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1, objtyp: "none", note: " [no objtyp]"})
	k.file(object{name: ca6, owner: "[200,1]", prot: "(S:RWED,O:RWED,G:RE,W:)", acl: []string{"IDENTIFIER=[200,1],ACCESS=READ"}})
	change('C', "ACLLENGTH", ca6, []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1, objtyp: "99", note: " [objtyp 99]"})
	change('C', "ACLLENGTH", ca6, []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1, objtyp: "ACL$C_DEVICE", note: " [objtyp DEVICE]", noShow: true})
	change('C', "ACLLENGTH", ca6, []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1, noName: true, note: " [no objnam or chan]", noShow: true})
	change('C', "ACLLENGTH", ca6, []item{out("ACL$C_ACLLENGTH", 2, kAll)}, changeOpts{acmode: -1, note: " [a 2-byte buffer]", noShow: true})
	change('C', "item 99", ca6, []item{zero("99", 4)}, changeOpts{acmode: -1, noShow: true})
	change('C', "no items", ca6, nil, changeOpts{acmode: -1, noShow: true})
	change('C', "ACLLENGTH", ca6, []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: 3, note: " [acmode user]", noShow: true})
	change('C', "ADDACLENT [200,2] READ by channel", ca6, []item{in("ACL$C_ADDACLENT", bRead)}, changeOpts{acmode: -1, chan_: true, ctx: "0"})
	change('C', "READACL by channel", ca6, []item{outNR("ACL$C_READACL", 512, kACL)}, changeOpts{acmode: -1, chan_: true, ctx: "0", noShow: true})
	change('C', "ACLLENGTH", chgDir+"NOSUCH.DAT", []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1, noShow: true, note: " [no such file]"})
	change('C', "ACLLENGTH", disk+"[SEC93.NOSUCH]A.DAT", []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1, noShow: true, note: " [no such directory]"})
	change('C', "ACLLENGTH", chgDir+"CA*.DAT", []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1, noShow: true, note: " [wildcard]"})
	change('C', "ACLLENGTH", "SEC93$CHG:CA6", []item{out("ACL$C_ACLLENGTH", 4, kHex)}, changeOpts{acmode: -1, noShow: true, note: " [a logical name, no type]"})
}

// chkproCases are $CHKPRO's, a third party's check, entirely by its items.
func chkproCases() {
	prot := func(s, o, g, w string) []byte {
		// A file's protection word: each category's denied bits.
		v := uint16(denied(s)&0xF) | uint16(denied(o)&0xF)<<4 | uint16(denied(g)&0xF)<<8 | uint16(denied(w)&0xF)<<12

		return binary.LittleEndian.AppendUint16(nil, v)
	}

	std := prot("RWED", "RWED", "R", "")
	priv := func(bits ...int) item {
		var lo, hi uint32
		for _, b := range bits {
			if b < 32 {
				lo |= 1 << b
			} else {
				hi |= 1 << (b - 32)
			}
		}

		return inQ("CHP$_PRIV", lo, hi)
	}

	const (
		sysprv  = 28
		bypass  = 29
		grpprv  = 34
		readall = 35
	)

	type c struct {
		what  string
		items []item
	}

	base := func(uic, access uint32, p []byte, extra ...item) []item {
		return append([]item{inL("CHP$_ACCESS", access), in("CHP$_PROT", p), inL("CHP$_OWNER", idA), inL("CHP$_UIC", uic), priv()}, extra...)
	}

	list := []c{
		{"owner READ", base(idA, aR, std)},
		{"owner WRITE+DELETE", base(idA, aW|aD, std)},
		{"group READ", base(idB, aR, std)},
		{"group WRITE", base(idB, aW, std)},
		{"world READ", base(idC, aR, std)},
		{"system [1,4] READ", base(idSys, aR, std)},
		{"system [10,1] READ", base(0x00080001, aR, std)},
		{"[11,1] READ", base(0x00090001, aR, std)},
		{"no access asked", base(idC, 0, std)},
		{"world READ, SYSPRV", base(idC, aR, std, priv(sysprv))},
		{"world READ, BYPASS", base(idC, aR, std, priv(bypass))},
		{"world READ, READALL", base(idC, aR, std, priv(readall))},
		{"world WRITE, READALL", base(idC, aW, std, priv(readall))},
		{"world READ, READALL, FLAGS USEREADALL", base(idC, aR, std, priv(readall), inL("CHP$_FLAGS", 0x4))},
		{"world READ, READALL, FLAGS 0", base(idC, aR, std, priv(readall), inL("CHP$_FLAGS", 0))},
		{"group READ+WRITE, GRPPRV (no group access)", base(idB, aR|aW, prot("RWED", "RWED", "", ""), priv(grpprv))},
		{"world READ, GRPPRV", base(idC, aR, std, priv(grpprv))},
		{"group CONTROL, one protection word", base(idB, aC, std)},
		{"owner CONTROL, one protection word", base(idA, aC, std)},
		{"group CONTROL, control word denying it", base(idB, aC, cat(std, []byte{0x00, 0x0F}))},
		{"owner CONTROL, control word denying it", base(idA, aC, cat(std, []byte{0xF0, 0x00}))},
		{"world READ, owner [0,0]", []item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", prot("", "", "", "")), inL("CHP$_OWNER", 0), inL("CHP$_UIC", idC), priv()}},
		{"no PROT", []item{inL("CHP$_ACCESS", aR), inL("CHP$_OWNER", idA), inL("CHP$_UIC", idC), priv()}},
		{"no OWNER", []item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", std), inL("CHP$_UIC", idC), priv()}},
		{"world READ, ACL grants PROT93_TEAM, RIGHTS [300,1]+PROT93_TEAM",
			[]item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", std), inL("CHP$_OWNER", idA), priv(),
				in("CHP$_RIGHTS", cat(le32(idC), le32(0), le32(idTeam), le32(0))),
				in("CHP$_ACL", idACE(0, aR, idTeam)), out("CHP$_MATCHEDACE", 64, kACL), out("CHP$_PRIVUSED", 4, kHex)}},
		{"world READ, ACL grants PROT93_TEAM, UIC [300,1] alone",
			[]item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", std), inL("CHP$_OWNER", idA), inL("CHP$_UIC", idC), priv(),
				in("CHP$_ACL", idACE(0, aR, idTeam)), out("CHP$_MATCHEDACE", 64, kACL)}},
		{"RIGHTS, then UIC [200,2]",
			[]item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", prot("", "", "", "")), inL("CHP$_OWNER", idA), priv(),
				in("CHP$_RIGHTS", cat(le32(idC), le32(0), le32(idTeam), le32(0))), inL("CHP$_UIC", idB),
				in("CHP$_ACL", idACE(0, aR, idTeam)), out("CHP$_MATCHEDACE", 64, kACL)}},
		{"ADDRIGHTS PROT93_TEAM, UIC [300,1]",
			[]item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", prot("", "", "", "")), inL("CHP$_OWNER", idA), inL("CHP$_UIC", idC), priv(),
				in("CHP$_ADDRIGHTS", cat(le32(idTeam), le32(0))), in("CHP$_ACL", idACE(0, aR, idTeam)), out("CHP$_MATCHEDACE", 64, kACL)}},
		{"ACL denies the owner",
			base(idA, aR, std, in("CHP$_ACL", idACE(0, 0, idA)), out("CHP$_MATCHEDACE", 64, kACL), out("CHP$_PRIVUSED", 4, kHex))},
		{"ACL denies the owner, SYSPRV",
			base(idA, aR, prot("RWED", "RWED", "", ""), priv(sysprv), in("CHP$_ACL", idACE(0, 0, idA)), out("CHP$_MATCHEDACE", 64, kACL), out("CHP$_PRIVUSED", 4, kHex))},
		{"ACL grants CONTROL to the world",
			base(idC, aC, std, in("CHP$_ACL", idACE(0, aC, idC)), out("CHP$_MATCHEDACE", 64, kACL))},
		{"ACL in two segments",
			base(idC, aW, std, in("CHP$_ACL", idACE(0, aR, idB)), in("CHP$_ACL", idACE(0, aW, idC)), out("CHP$_MATCHEDACE", 64, kACL))},
		{"ACL with a wildcard [300,*] READ",
			base(idC, aR, std, in("CHP$_ACL", idACE(0, aR, 0x00C0FFFF)), out("CHP$_MATCHEDACE", 64, kACL))},
		{"ACL matches, MATCHEDACE buffer 4",
			base(idC, aR, std, in("CHP$_ACL", idACE(0, aR, idC)), out("CHP$_MATCHEDACE", 4, kAll))},
		{"ACL with an alarm, ALARMNAME and AUDITNAME",
			base(idC, aR, std, in("CHP$_ACL", cat(alarmACE(tAlarm, 3, aR, "SECURITY"), idACE(0, aR, idC))),
				out("CHP$_ALARMNAME", 32, kText), out("CHP$_AUDITNAME", 32, kText), out("CHP$_MATCHEDACE", 64, kACL))},
		{"ACL with an ACE of size 3", base(idC, aR, std, in("CHP$_ACL", []byte{3, 1, 0, 0}))},
		{"ACL DEFAULT ACE only", base(idC, aR, std, in("CHP$_ACL", idACE(oDefault, aR, idC)), out("CHP$_MATCHEDACE", 64, kACL))},
		{"PRIVUSED, world READ with SYSPRV and BYPASS", base(idC, aR, std, priv(sysprv, bypass), out("CHP$_PRIVUSED", 4, kHex))},
		{"PRIVUSED, world READ with READALL", base(idC, aR, std, priv(readall), out("CHP$_PRIVUSED", 4, kHex))},
		{"PRIVUSED, group READ+WRITE with GRPPRV", base(idB, aR|aW, prot("RWED", "RWED", "", ""), priv(grpprv), out("CHP$_PRIVUSED", 4, kHex))},
		{"ACMODE user, MODE kernel", base(idA, aR, std, item{code: "CHP$_ACMODE", data: []byte{3}}, item{code: "CHP$_MODE", data: []byte{0}})},
		{"ACMODE kernel, MODE user", base(idA, aR, std, item{code: "CHP$_ACMODE", data: []byte{0}}, item{code: "CHP$_MODE", data: []byte{3}})},
		{"ACMODE user, MODES all kernel", base(idA, aR, std, item{code: "CHP$_ACMODE", data: []byte{3}}, inQ("CHP$_MODES", 0, 0))},
		{"ACMODE user, MODES all user", base(idA, aR, std, item{code: "CHP$_ACMODE", data: []byte{3}}, inQ("CHP$_MODES", 0xFFFFFFFF, 0xFFFFFFFF))},
		{"no items", nil},
		{"item 99", []item{inL("99", 0)}},
		{"ACCESS only", []item{inL("CHP$_ACCESS", aR)}},
	}

	for _, c := range list {
		chkpro('K', c.what, c.items, 1)
	}

	var many []item
	for range 21 {
		many = append(many, in("CHP$_ACL", idACE(0, aR, idB)))
	}

	chkpro('K', "21 ACL items", append(base(idC, aR, std), many...), 1)
	chkpro('K', "owner READ, 3 arguments", base(idA, aR, std), 3)

	// The caller's own rights and privileges fill what the items don't
	// say: run with and without the caller's privileges.
	q := []c{
		{"world READ, no PRIV item", []item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", std), inL("CHP$_OWNER", idA), inL("CHP$_UIC", idC), out("CHP$_PRIVUSED", 4, kHex)}},
		{"world WRITE, no PRIV item", []item{inL("CHP$_ACCESS", aW), in("CHP$_PROT", std), inL("CHP$_OWNER", idA), inL("CHP$_UIC", idC), out("CHP$_PRIVUSED", 4, kHex)}},
		{"no UIC item, world protection none", []item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", prot("RWED", "RWED", "R", "")), inL("CHP$_OWNER", idA), priv()}},
		{"no UIC item, system protection none", []item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", prot("", "RWED", "R", "R")), inL("CHP$_OWNER", idA), priv()}},
		{"ACL grants INTERACTIVE, UIC [300,1], no RIGHTS", []item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", prot("", "", "", "")), inL("CHP$_OWNER", idA), inL("CHP$_UIC", idC), priv(),
			in("CHP$_ACL", idACE(0, aR, idInter)), out("CHP$_MATCHEDACE", 64, kACL)}},
		{"ADDRIGHTS PROT93_TEAM alone", []item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", prot("", "", "", "")), inL("CHP$_OWNER", idA), priv(),
			in("CHP$_ADDRIGHTS", cat(le32(idTeam), le32(0))), in("CHP$_ACL", idACE(0, aR, idTeam)), out("CHP$_MATCHEDACE", 64, kACL)}},
	}

	for _, c := range q {
		chkpro('Q', c.what, c.items, 1)
	}
}

// The $CHECK_ACCESS cases' files, in [SEC93.CHK].
func checkAccessCases() {
	h1 := chkDir + "H1.DAT"
	h2 := chkDir + "H2.DAT"
	h3 := chkDir + "H3.DAT"
	h4 := chkDir + "H4.DAT"

	first := true
	for _, f := range []struct {
		spec string
		o    object
	}{
		{h1, object{name: h1, owner: "[200,1]", prot: "(S:RWED,O:RWED,G:R,W:)"}},
		{h2, object{name: h2, owner: "[200,1]", prot: "(S:RWED,O:RWED,G:R,W:R)",
			acl: []string{"IDENTIFIER=[200,2],ACCESS=NONE", "IDENTIFIER=PROT93_TEAM,ACCESS=READ+WRITE"}}},
		{h3, object{name: h3, owner: "[0,0]", prot: "(S,O,G,W)"}},
		{h4, object{name: h4, owner: "[200,1]", prot: "(S,O,G,W)", acl: []string{"IDENTIFIER=INTERACTIVE,ACCESS=READ"}}},
	} {
		for _, u := range []string{"PROT93A", "PROT93B", "PROT93C", "SYSTEM"} {
			for _, a := range []uint32{aR, aW, aC} {
				checkAccess('H', f.spec, u, []item{inL("CHP$_ACCESS", a), out("CHP$_MATCHED_ACE", 64, kACL)},
					checkOpts{note: fmt.Sprintf(" access %X", a)})

				if first {
					cases[len(cases)-1].file(f.o)
					first = false
				}
			}
		}

		first = true
	}

	checkAccess('H', h1, "PROT93C", []item{inL("CHP$_ACCESS", aR), inL("CHP$_FLAGS", 1), out("CHP$_PRIVUSED", 4, kHex)}, checkOpts{})
	checkAccess('H', h1, "PROT93C", []item{inL("CHP$_ACCESS", aR), inL("CHP$_FLAGS", 0x4), out("CHP$_PRIVUSED", 4, kHex)}, checkOpts{})
	checkAccess('H', h1, "PROT93C", []item{out("CHP$_PRIVUSED", 4, kHex)}, checkOpts{})
	checkAccess('H', h1, "prot93b", []item{inL("CHP$_ACCESS", aR)}, checkOpts{})
	checkAccess('H', h1, "PROT93B   ", []item{inL("CHP$_ACCESS", aR)}, checkOpts{})
	checkAccess('H', h1, "NOSUCH", []item{inL("CHP$_ACCESS", aR)}, checkOpts{})
	checkAccess('H', h1, "", []item{inL("CHP$_ACCESS", aR)}, checkOpts{})
	checkAccess('H', chkDir+"NOSUCH.DAT", "PROT93A", []item{inL("CHP$_ACCESS", aR)}, checkOpts{})
	checkAccess('H', disk+"[SEC93]CHK.DIR", "PROT93C", []item{inL("CHP$_ACCESS", aR)}, checkOpts{})
	checkAccess('H', h1, "PROT93A", []item{inL("CHP$_ACCESS", aR)}, checkOpts{objtyp: "none"})
	checkAccess('H', h1, "PROT93A", []item{inL("CHP$_ACCESS", aR)}, checkOpts{objtyp: "ACL$C_DEVICE"})
	checkAccess('H', disk, "PROT93C", []item{inL("CHP$_ACCESS", aR)}, checkOpts{objtyp: "ACL$C_DEVICE"})
	checkAccess('H', h1, "PROT93A", []item{inL("CHP$_ACCESS", aR)}, checkOpts{objtyp: "none", clsnam: "FILE"})
	checkAccess('H', h1, "PROT93C", []item{inL("CHP$_ACCESS", aR)}, checkOpts{clsnam: "FILE"})
	checkAccess('H', h1, "PROT93C", []item{inL("CHP$_ACCESS", aR)}, checkOpts{objtyp: "none", clsnam: "NOSUCH"})
	checkAccess('H', h1, "PROT93A", []item{inL("CHP$_ACCESS", aR)}, checkOpts{args: 5})
}

// The $GET_SECURITY cases' files, in [SEC93.GET].
func getCases() {
	g1 := getDir + "G1.DAT"
	g2 := getDir + "G2.DAT"
	g3 := getDir + "G3.DIR"

	single := []item{
		out("OSS$_OWNER", 4, kHex),
		out("OSS$_PROTECTION", 2, kHex),
		out("OSS$_PROTECTION", 4, kAll),
		out("OSS$_SYS_PROT", 4, kAll),
		out("OSS$_OWN_PROT", 4, kAll),
		out("OSS$_GRP_PROT", 4, kAll),
		out("OSS$_WOR_PROT", 4, kAll),
		out("OSS$_ACL_LENGTH", 4, kHex),
		out("OSS$_ACL_READ", 512, kACL),
		out("OSS$_CLASS_NAME", 64, kText),
		out("OSS$_OBJECT_NAME", 255, kText),
		out("OSS$_ACCESS_NAMES_LENGTH", 4, kHex),
		out("OSS$_ACCESS_NAMES", 512, kHex),
		out("OSS$_DAMAGED_ACL", 4, kAll),
		out("OSS$_IN_TRANSITION", 4, kAll),
		out("OSS$_TRANQUILITY_COUNT", 4, kAll),
		out("OSS$_ACL_GRANT_ACE", 64, kACL),
		out("OSS$_NEXT_OBJECT", 64, kText),
		out("OSS$_FIRST_TEMPLATE", 64, kText),
		out("OSS$_ACCESS_CLASS_NAME", 64, kText),
	}

	for i, it := range single {
		get('G', g1, []item{it}, secOpts{acmode: -1})

		if i == 0 {
			cases[len(cases)-1].file(object{name: g1, owner: "[200,1]", prot: "(S:RWED,O:RWED,G:RE,W:)",
				acl: []string{"IDENTIFIER=[200,2],ACCESS=READ", "IDENTIFIER=PROT93_TEAM,OPTIONS=PROTECTED,ACCESS=READ+WRITE"}})
		}
	}

	all := []item{out("OSS$_OWNER", 4, kHex), out("OSS$_PROTECTION", 2, kHex), out("OSS$_ACL_LENGTH", 4, kHex), out("OSS$_ACL_READ", 512, kACL)}
	get('G', g2, all, secOpts{acmode: -1})
	cases[len(cases)-1].file(object{name: g2, owner: "[300,1]", prot: "(S:RWED,O:RWED,G:RE,W:RE)"})
	get('G', g3, append(all, out("OSS$_OBJECT_NAME", 255, kText)), secOpts{acmode: -1})
	cases[len(cases)-1].file(object{name: g3, owner: "[200,1]", prot: "(S:RWE,O:RWE,G:RE,W:)", dir: true,
		acl: []string{"DEFAULT_PROTECTION,S:RWED,O:RWED,G:R,W:", "IDENTIFIER=[200,2],OPTIONS=DEFAULT,ACCESS=READ"}})

	// The ACL a piece at a time, through a context.
	get('G', g1, []item{out("OSS$_ACL_READ", 16, kAll)}, secOpts{acmode: -1, ctx: "new"})
	get('G', g1, []item{out("OSS$_ACL_READ", 16, kAll)}, secOpts{acmode: -1, ctx: "keep"})
	get('G', g1, []item{out("OSS$_ACL_READ", 16, kAll)}, secOpts{acmode: -1, ctx: "keep"})
	get('G', g1, nil, secOpts{acmode: -1, ctx: "keep", flags: "OSS$M_RELCTX"})
	get('G', g1, []item{out("OSS$_ACL_READ", 8, kAll)}, secOpts{acmode: -1, note: " [8 bytes]"})
	get('G', g1, []item{out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "new"})
	get('G', g1, []item{out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "keep"})
	get('G', g1, []item{out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "keep"})
	get('G', g1, []item{zero("OSS$_ACL_POSITION_TOP", 0), out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "keep"})
	get('G', g1, []item{zero("OSS$_ACL_FIND_NEXT", 0), out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "keep"})
	get('G', g1, []item{zero("OSS$_ACL_POSITION_BOTTOM", 0), out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "keep"})
	get('G', g1, []item{in("OSS$_ACL_FIND_ENTRY", idACE(oProt, aR|aW, idTeam)), out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "keep"})
	get('G', g1, []item{in("OSS$_ACL_FIND_ENTRY", idACE(0, aR|aW, idTeam)), out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "keep", note: " [flags differ]"})
	get('G', g1, []item{in("OSS$_ACL_FIND_TYPE", []byte{tKeyID}), out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "keep"})
	get('G', g1, []item{in("OSS$_ACL_FIND_TYPE", []byte{8, tKeyID, 0, 0, 0, 0, 0, 0}), out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "keep", note: " [an ACE's header]"})
	get('G', g1, []item{in("OSS$_ACL_FIND_TYPE", []byte{tAlarm}), out("OSS$_ACL_READ_ENTRY", 64, kACL)}, secOpts{acmode: -1, ctx: "keep"})
	get('G', g1, []item{in("OSS$_ACL_READ_ENTRY", idACE(0, aR, idB))}, secOpts{acmode: -1, ctx: "keep", note: " [READ_ENTRY given an ACE]"})
	get('G', g1, nil, secOpts{acmode: -1, ctx: "keep", flags: "OSS$M_RELCTX"})

	// Names.
	for _, n := range []string{
		"file", "FIL", "DEVICE", "VOLUME", "NOSUCH", "SECURITY_CLASS",
	} {
		get('G', g1, []item{out("OSS$_OWNER", 4, kHex), out("OSS$_CLASS_NAME", 64, kText), out("OSS$_OBJECT_NAME", 255, kText)}, secOpts{acmode: -1, clsnam: n})
	}

	get('G', g1, []item{out("OSS$_OWNER", 4, kHex)}, secOpts{acmode: -1, clsnam: "-"})
	get('G', g1, []item{out("OSS$_OWNER", 4, kHex)}, secOpts{acmode: -1, objnam: "-"})

	for _, n := range []string{
		"SEC93$GET:G1.DAT", "SEC93$GET:G1", getDir + "G1.DAT;", getDir + "G1.DAT;1", getDir + "G1.DAT;0",
		getDir + "G1.DAT;-0", getDir + "G*.DAT", getDir + "NOSUCH.DAT", disk + "[SEC93.NOSUCH]G1.DAT",
		disk + "[SEC93]GET.DIR", disk + "[SEC93.GET]", "G1.DAT", "NOSUCHDEV:[SEC93.GET]G1.DAT",
	} {
		get('G', g1, []item{out("OSS$_OWNER", 4, kHex), out("OSS$_OBJECT_NAME", 255, kText)}, secOpts{acmode: -1, objnam: n})
	}

	for _, d := range []string{disk, "PROT93$DISK", "SYS$DISK:"} {
		get('G', g1, []item{out("OSS$_OWNER", 4, kHex), out("OSS$_PROTECTION", 2, kHex), out("OSS$_ACL_READ", 512, kACL),
			out("OSS$_OBJECT_NAME", 255, kText)}, secOpts{acmode: -1, clsnam: "DEVICE", objnam: d})
		get('G', g1, []item{out("OSS$_OWNER", 4, kHex), out("OSS$_PROTECTION", 2, kHex), out("OSS$_OBJECT_NAME", 255, kText)},
			secOpts{acmode: -1, clsnam: "VOLUME", objnam: d})
	}

	get('G', g1, []item{out("OSS$_OWNER", 4, kHex), out("OSS$_PROTECTION", 2, kHex), out("OSS$_OBJECT_NAME", 255, kText)},
		secOpts{acmode: -1, clsnam: "LOGICAL_NAME_TABLE", objnam: "LNM$SYSTEM_TABLE"})
	get('G', g1, []item{out("OSS$_FIRST_TEMPLATE", 64, kText), out("OSS$_NEXT_OBJECT", 64, kText)},
		secOpts{acmode: -1, clsnam: "SECURITY_CLASS", objnam: "FILE"})

	// By handle.
	get('G', g2, all, secOpts{acmode: -1, handle: true})
	get('G', g2, all, secOpts{acmode: -1, handle: true, clsnam: "-"})
	get('G', g2, all, secOpts{acmode: -1, handle: true, objnam: getDir + "G1.DAT", note: " [and a name]"})

	// Flags and modes.
	get('G', g1, []item{out("OSS$_OWNER", 4, kHex)}, secOpts{acmode: -1, flags: "OSS$M_WLOCK", ctx: "new"})
	get('G', g1, []item{out("OSS$_OWNER", 4, kHex)}, secOpts{acmode: -1, flags: "OSS$M_RELCTX", ctx: "keep"})
	get('G', g1, []item{out("OSS$_OWNER", 4, kHex)}, secOpts{acmode: -1, flags: "OSS$M_RELCTX"})
	get('G', g1, []item{out("OSS$_OWNER", 4, kHex)}, secOpts{acmode: 3})
	get('G', g1, []item{zero("99", 4)}, secOpts{acmode: -1})
	get('G', g1, nil, secOpts{acmode: -1})
}

// The $SET_SECURITY cases' files, in [SEC93.SET], each a fresh file.
func setCases() {
	n := 0
	newFile := func(acl ...string) string {
		n++
		spec := fmt.Sprintf("%sS%d.DAT", setDir, n)
		cases[len(cases)-1].file(object{name: spec, owner: "[200,1]", prot: "(S:RWED,O:RWED,G:RE,W:)", acl: acl})

		return spec
	}

	two := []string{"IDENTIFIER=[200,2],ACCESS=READ", "IDENTIFIER=PROT93_TEAM,OPTIONS=PROTECTED,ACCESS=READ+WRITE", "IDENTIFIER=[300,1],ACCESS=READ"}

	type c struct {
		what  string
		acl   []string
		items []item
	}

	list := []c{
		{"OWNER [300,1]", nil, []item{inL("OSS$_OWNER", idC)}},
		{"OWNER PROT93_TEAM", nil, []item{inL("OSS$_OWNER", idTeam)}},
		{"OWNER [0,0]", nil, []item{inL("OSS$_OWNER", 0)}},
		{"PROTECTION %XFF00", nil, []item{inW("OSS$_PROTECTION", 0xFF00)}},
		{"PROTECTION as a longword %X0000FF00", nil, []item{inL("OSS$_PROTECTION", 0xFF00)}},
		{"SYS_PROT 2, WOR_PROT 0", nil, []item{inL("OSS$_SYS_PROT", 2), inL("OSS$_WOR_PROT", 0)}},
		{"GRP_PROT %X10", nil, []item{inL("OSS$_GRP_PROT", 0x10)}},
		{"OWNER and PROTECTION", nil, []item{inL("OSS$_OWNER", idC), inW("OSS$_PROTECTION", 0x0000)}},
		{"ACL_ADD_ENTRY [200,1] READ", two, []item{in("OSS$_ACL_ADD_ENTRY", idACE(0, aR, idA))}},
		{"ACL_ADD_ENTRY [200,2] WRITE (matches)", two, []item{in("OSS$_ACL_ADD_ENTRY", idACE(0, aW, idB))}},
		{"ACL_ADD_ENTRY [300,1] WRITE (matches a lower ACE)", two, []item{in("OSS$_ACL_ADD_ENTRY", idACE(0, aW, idC))}},
		{"ACL_POSITION_BOTTOM, ACL_ADD_ENTRY [200,1] READ", two, []item{zero("OSS$_ACL_POSITION_BOTTOM", 0), in("OSS$_ACL_ADD_ENTRY", idACE(0, aR, idA))}},
		{"ACL_POSITION_TOP, ACL_ADD_ENTRY [200,1] READ", two, []item{zero("OSS$_ACL_POSITION_TOP", 0), in("OSS$_ACL_ADD_ENTRY", idACE(0, aR, idA))}},
		{"ACL_ADD_ENTRY two ACEs in one buffer", two, []item{in("OSS$_ACL_ADD_ENTRY", cat(idACE(0, aR, idA), idACE(0, aE, idInter)))}},
		{"ACL_FIND_ENTRY [300,1] READ, ACL_ADD_ENTRY [200,1] READ", two, []item{in("OSS$_ACL_FIND_ENTRY", idACE(0, aR, idC)), in("OSS$_ACL_ADD_ENTRY", idACE(0, aR, idA))}},
		{"ACL_DELETE_ENTRY [200,2] READ", two, []item{in("OSS$_ACL_DELETE_ENTRY", idACE(0, aR, idB))}},
		{"ACL_DELETE_ENTRY [200,1] READ (not there)", two, []item{in("OSS$_ACL_DELETE_ENTRY", idACE(0, aR, idA))}},
		{"ACL_DELETE_ENTRY the protected ACE", two, []item{in("OSS$_ACL_DELETE_ENTRY", idACE(oProt, aR|aW, idTeam))}},
		{"ACL_FIND_ENTRY [300,1] READ, ACL_DELETE_ENTRY no buffer", two, []item{in("OSS$_ACL_FIND_ENTRY", idACE(0, aR, idC)), zero("OSS$_ACL_DELETE_ENTRY", 0)}},
		{"ACL_DELETE", two, []item{zero("OSS$_ACL_DELETE", 0)}},
		{"ACL_DELETE_ALL", two, []item{zero("OSS$_ACL_DELETE_ALL", 0)}},
		{"ACL_FIND_ENTRY [200,2] READ, ACL_MODIFY_ENTRY [200,2] DELETE", two, []item{in("OSS$_ACL_FIND_ENTRY", idACE(0, aR, idB)), in("OSS$_ACL_MODIFY_ENTRY", idACE(0, aD, idB))}},
		{"ACL_MODIFY_ENTRY with no position", two, []item{in("OSS$_ACL_MODIFY_ENTRY", idACE(0, aD, idB))}},
		{"ACL_FIND_TYPE identifier, ACL_DELETE_ENTRY no buffer", two, []item{in("OSS$_ACL_FIND_TYPE", []byte{tKeyID}), zero("OSS$_ACL_DELETE_ENTRY", 0)}},
		{"ACL_ADD_ENTRY default protection on a file", nil, []item{in("OSS$_ACL_ADD_ENTRY", dirdefACE(0, "RWED", "RWED", "R", ""))}},
		{"ACL_ADD_ENTRY HIDDEN", nil, []item{in("OSS$_ACL_ADD_ENTRY", idACE(oHidden, aR, idA))}},
		{"ACL_ADD_ENTRY an ACE of size 3", nil, []item{in("OSS$_ACL_ADD_ENTRY", []byte{3, 1, 0, 0})}},
		{"PROTECTION 1 byte", nil, []item{in("OSS$_PROTECTION", []byte{0x0F})}},
		{"OWNER 2 bytes", nil, []item{inN("OSS$_OWNER", le32(idC), 2)}},
		{"item 99", nil, []item{zero("99", 4)}},
		{"no items", nil, nil},
	}

	for _, c := range list {
		// The case's file is made before its spec is known: a placeholder
		// case takes it, then the real one.
		spec := fmt.Sprintf("%sS%d.DAT", setDir, n+1)
		set('S', spec, c.items, secOpts{acmode: -1, note: " " + c.what, show: spec})
		newFile(c.acl...)
	}

	// Through a context: LOCAL changes, then one that commits them.
	spec := fmt.Sprintf("%sS%d.DAT", setDir, n+1)
	set('S', spec, []item{inW("OSS$_PROTECTION", 0xF000)}, secOpts{acmode: -1, ctx: "new", flags: "<OSS$M_LOCAL!OSS$M_WLOCK>", show: spec})
	newFile()
	set('S', spec, []item{in("OSS$_ACL_ADD_ENTRY", idACE(0, aR, idC))}, secOpts{acmode: -1, ctx: "keep", flags: "OSS$M_LOCAL", show: spec})
	set('S', spec, nil, secOpts{acmode: -1, ctx: "keep", flags: "OSS$M_RELCTX", show: spec})

	spec = fmt.Sprintf("%sS%d.DAT", setDir, n+1)
	set('S', spec, []item{inW("OSS$_PROTECTION", 0xF000)}, secOpts{acmode: -1, ctx: "new", flags: "OSS$M_WLOCK", show: spec})
	newFile()
	set('S', spec, []item{inL("OSS$_OWNER", idB)}, secOpts{acmode: -1, ctx: "keep", show: spec})
	set('S', spec, nil, secOpts{acmode: -1, ctx: "keep", flags: "OSS$M_RELCTX", show: spec})

	// By handle.
	spec = fmt.Sprintf("%sS%d.DAT", setDir, n+1)
	set('S', spec, []item{inW("OSS$_PROTECTION", 0x0F00), in("OSS$_ACL_ADD_ENTRY", idACE(0, aR, idC))}, secOpts{acmode: -1, handle: true, show: spec})
	newFile()
	set('S', spec, []item{inW("OSS$_PROTECTION", 0x00F0)}, secOpts{acmode: -1, handle: true, fac: "PUT", show: spec, note: " [opened for write]"})

	// A directory, and the class.
	spec = disk + "[SEC93.SET]SD1.DIR"
	set('S', spec, []item{in("OSS$_ACL_ADD_ENTRY", dirdefACE(0, "RWED", "RWED", "RE", ""))}, secOpts{acmode: -1, show: spec})
	cases[len(cases)-1].file(object{name: spec, owner: "[200,1]", prot: "(S:RWE,O:RWE,G:RE,W:)", dir: true})
	set('S', spec, []item{inW("OSS$_PROTECTION", 0x0000)}, secOpts{acmode: -1, clsnam: "DEVICE", objnam: spec})
	set('S', spec, []item{inW("OSS$_PROTECTION", 0x0000)}, secOpts{acmode: -1, clsnam: "NOSUCH", objnam: spec})
}

// setdfprotCases are $SETDFPROT's: the process's default protection.
func setdfprotCases() {
	k := newCase('D', "$SETDFPROT query")
	k.d("%%P:\t.WORD\t^XAAAA")
	k.d("%%Q:\t.WORD\t^X5555")
	k.c("PUSHAW\t%%P")
	k.c("CLRL\t-(SP)")
	k.c("CALLS\t#2,G^SYS$SETDFPROT")
	k.c("MOVL\tR0,R2")
	k.c("MOVZWL\t%%P,R3")
	k.c("SAY\t</  $SETDFPROT r0=!XL cur=!XW/>,R2,R3")
	k.c("MOVW\t#^XFF00,%%Q")
	k.c("PUSHAW\t%%P")
	k.c("PUSHAW\t%%Q")
	k.c("CALLS\t#2,G^SYS$SETDFPROT")
	k.c("MOVL\tR0,R2")
	k.c("MOVZWL\t%%P,R3")
	k.c("SAY\t</  set FF00: r0=!XL previous=!XW/>,R2,R3")
	k.c("PUSHAW\t%%Q")
	k.c("CLRL\t-(SP)")
	k.c("CALLS\t#2,G^SYS$SETDFPROT")
	k.c("MOVL\tR0,R2")
	k.c("MOVZWL\t%%Q,R3")
	k.c("SAY\t</  query: r0=!XL cur=!XW/>,R2,R3")
	k.c("PUSHL\t#0")
	k.c("PUSHAW\t%%P")
	k.c("CALLS\t#2,G^SYS$SETDFPROT")
	k.c("MOVL\tR0,R2")
	k.c("SAY\t</  set back: r0=!XL/>,R2")
	k.c("CLRQ\t-(SP)")
	k.c("CALLS\t#2,G^SYS$SETDFPROT")
	k.c("MOVL\tR0,R2")
	k.c("SAY\t</  no arguments: r0=!XL/>,R2")
	k.c("PUSHL\t#4")
	k.c("CLRL\t-(SP)")
	k.c("CALLS\t#2,G^SYS$SETDFPROT")
	k.c("MOVL\tR0,R2")
	k.c("SAY\t</  cur at address 4: r0=!XL/>,R2")
	k.c("CLRL\t-(SP)")
	k.c("PUSHL\t#4")
	k.c("CALLS\t#2,G^SYS$SETDFPROT")
	k.c("MOVL\tR0,R2")
	k.c("SAY\t</  new at address 4: r0=!XL/>,R2")
}

// identityCases are the cases a process without privileges runs, SYSTEM's
// (part M) or PROT93C's (part U), each on files of its own (suffix).
func identityCases(part byte, suffix string) {
	nf := func(name, owner, prot string, acl ...string) object {
		return object{name: npDir + name + suffix + ".DAT", owner: owner, prot: prot, acl: acl}
	}

	files := []object{
		nf("NR", "[200,1]", "(S:R,O:RWED,G:R,W:R)"),
		nf("NN", "[200,1]", "(S,O:RWED,G,W)"),
		nf("NC", "[200,1]", "(S,O:RWED,G,W)", "IDENTIFIER=[300,1],ACCESS=CONTROL"),
		nf("NZ", "[0,0]", "(S,O,G,W)", "IDENTIFIER=[1,4],ACCESS=READ", "IDENTIFIER=[300,1],ACCESS=READ"),
		nf("NO", "[300,1]", "(S,O:RWED,G,W)"),
		nf("NS", "[1,4]", "(S:RWED,O:RWED,G,W)"),
	}

	shown := []item{out("OSS$_OWNER", 4, kHex), out("OSS$_PROTECTION", 2, kHex), out("OSS$_ACL_READ", 512, kACL)}

	for i, f := range files {
		get(part, f.name, shown, secOpts{acmode: -1})

		if i == 0 {
			for _, g := range files {
				cases[len(cases)-1].file(g)
			}
		}
	}

	for _, f := range files {
		set(part, f.name, []item{inW("OSS$_PROTECTION", 0x0000)}, secOpts{acmode: -1, note: " PROTECTION %X0000"})
		get(part, f.name, shown, secOpts{acmode: -1})
	}

	for _, f := range files {
		set(part, f.name, []item{in("OSS$_ACL_ADD_ENTRY", idACE(0, aR, idB))}, secOpts{acmode: -1, note: " ACL_ADD_ENTRY"})
		change(part, "ADDACLENT [200,2] WRITE", f.name, []item{in("ACL$C_ADDACLENT", idACE(0, aW, idB))}, changeOpts{acmode: -1, noShow: true})
		change(part, "READACL", f.name, []item{outNR("ACL$C_READACL", 512, kACL)}, changeOpts{acmode: -1, ctx: "0", noShow: true})
	}

	set(part, files[2].name, []item{inL("OSS$_OWNER", idC)}, secOpts{acmode: -1, note: " OWNER [300,1] (control by an ACE)"})
	set(part, files[4].name, []item{inL("OSS$_OWNER", idC)}, secOpts{acmode: -1, note: " OWNER [300,1] (already [300,1])"})
	set(part, files[4].name, []item{inL("OSS$_OWNER", idSys)}, secOpts{acmode: -1, note: " OWNER [1,4]"})
	set(part, files[5].name, []item{inL("OSS$_OWNER", idSys)}, secOpts{acmode: -1, note: " OWNER [1,4] (already [1,4])"})

	std := binary.LittleEndian.AppendUint16(nil, uint16(denied("RWED")&0xF)|uint16(denied("RWED")&0xF)<<4|uint16(denied("R")&0xF)<<8|uint16(denied("")&0xF)<<12)

	for _, a := range []uint32{aR, aW} {
		chkpro(part, fmt.Sprintf("world access %d, no PRIV or UIC item", a),
			[]item{inL("CHP$_ACCESS", a), in("CHP$_PROT", std), inL("CHP$_OWNER", idA), out("CHP$_PRIVUSED", 4, kHex)}, 1)
	}

	chkpro(part, "owner READ, PRIV item none", []item{inL("CHP$_ACCESS", aR), in("CHP$_PROT", std), inL("CHP$_OWNER", idA), inL("CHP$_UIC", idA), inQ("CHP$_PRIV", 0, 0)}, 1)
	checkAccess(part, files[0].name, "PROT93A", []item{inL("CHP$_ACCESS", aR)}, checkOpts{})
	checkAccess(part, files[1].name, "PROT93B", []item{inL("CHP$_ACCESS", aR)}, checkOpts{})

	parse(part, "(IDENTIFIER=PROT93A,ACCESS=READ)", parseOpts{})
	parse(part, "(IDENTIFIER=[PROT93,PROT93B],ACCESS=READ)", parseOpts{})
	parse(part, "(IDENTIFIER=PROT93_TEAM,ACCESS=READ)", parseOpts{})
	format(part, "[200,1] READ", idACE(0, aR, idA), fmtOpts{width: -1, indent: -1})
	format(part, "PROT93_TEAM READ", idACE(0, aR, idTeam), fmtOpts{width: -1, indent: -1})
	format(part, "[200,*] READ", idACE(0, aR, idG200), fmtOpts{width: -1, indent: -1})

	k := newCase(part, "$SETDFPROT query, set %%XEE00, query")
	k.d("%%P:\t.WORD\t^XAAAA")
	k.d("%%Q:\t.WORD\t^XEE00")
	k.c("PUSHAW\t%%P")
	k.c("CLRL\t-(SP)")
	k.c("CALLS\t#2,G^SYS$SETDFPROT")
	k.c("MOVL\tR0,R2")
	k.c("MOVZWL\t%%P,R3")
	k.c("SAY\t</  $SETDFPROT r0=!XL cur=!XW/>,R2,R3")
	k.c("PUSHAW\t%%P")
	k.c("PUSHAW\t%%Q")
	k.c("CALLS\t#2,G^SYS$SETDFPROT")
	k.c("MOVL\tR0,R2")
	k.c("MOVZWL\t%%P,R3")
	k.c("SAY\t</  set: r0=!XL previous=!XW/>,R2,R3")
	k.c("PUSHAW\t%%Q")
	k.c("CLRL\t-(SP)")
	k.c("CALLS\t#2,G^SYS$SETDFPROT")
	k.c("MOVL\tR0,R2")
	k.c("MOVZWL\t%%Q,R3")
	k.c("SAY\t</  query: r0=!XL cur=!XW/>,R2,R3")
}

// ---------------------------------------------------------------------
// The procedures.

type proc struct{ strings.Builder }

func (p *proc) line(format string, args ...any) { fmt.Fprintf(p, format+"\n", args...) }

func (p *proc) section(title string) {
	p.line("$ !")
	p.line("$ WRITE SYS$OUTPUT \"######## %s\"", title)
}

func (p *proc) header(name, who, logName string) {
	p.line("$ ! %s.COM - Phase 93's probe, round 3: %s.", name, who)
	p.line("$ ! Written by testdata/prot93/gen3.go (README.md's \"Round 3\").")
	p.line("$ !")
	p.line("$ IF P1 .EQS. \"LOGGED\" THEN GOTO BEGIN")
	p.line("$ HERE = F$ENVIRONMENT(\"PROCEDURE\")")
	p.line("$ DEV = F$PARSE(HERE,,,\"DEVICE\")")

	if name == "SEC93" {
		p.line("$ DEFINE/SYSTEM/EXECUTIVE_MODE PROT93$DISK 'DEV'")
	}

	p.line("$ LOGFILE = DEV + \"[PROT93LOG]\" + %s", logName)
	p.line("$ @'HERE'/OUTPUT='LOGFILE' LOGGED")
	p.line("$ WRITE SYS$OUTPUT \"%s finished; the log is \", LOGFILE", name)
	p.line("$ EXIT")
	p.line("$ BEGIN:")
	p.line("$ SET NOON")
	p.line("$ SET VERIFY")
}

// run runs the program's parts.
func (p *proc) run(parts string) {
	p.line("$ DEFINE SEC93_PART \"%s\"", parts)
	p.line("$ RUN %s[SEC93]SEC93", disk)
	p.line("$ SHOW SYMBOL $STATUS")
	p.line("$ DEASSIGN SEC93_PART")
}

const (
	dropPrivs    = "$ SET PROCESS/PRIVILEGES=(NOREADALL,NOGRPPRV,NOSYSPRV,NOBYPASS)"
	restorePrivs = "$ SET PROCESS/PRIVILEGES=(READALL,GRPPRV,SYSPRV,BYPASS)"
	showPrivs    = `$ WRITE SYS$OUTPUT "READALL=", F$PRIVILEGE("READALL"), " GRPPRV=", F$PRIVILEGE("GRPPRV"), ` +
		`" SYSPRV=", F$PRIVILEGE("SYSPRV"), " BYPASS=", F$PRIVILEGE("BYPASS")`
)

func account(name, uic, privs string) string {
	return fmt.Sprintf("ADD %s/UIC=%s/PASSWORD=PROT93PW/NOPWDEXPIRED/PWDLIFETIME=NONE/FLAGS=NODISUSER"+
		"/DEVICE=%s/DIRECTORY=[PROT93LOG]/PRIVILEGES=(%s)/DEFPRIVILEGES=(TMPMBX,NETMBX)/ACCOUNT=PROT93"+
		"/OWNER=\"PHASE 93 PROBE\"", name, uic, disk, privs)
}

// The execute-only procedures (A.4's first open question), in [SEC93.EXO],
// owned by PROT93A. Each line is written with WRITE, so the procedure
// holds exactly these lines.
var exoProcs = []struct {
	name, prot string
	lines      []string
}{
	{"EXONLY", "(S:E,O:RWED,G:E,W:E)", []string{
		`$ WRITE SYS$OUTPUT "EXONLY LINE 1 RAN"`,
		`$ WRITE SYS$OUTPUT "VERIFY_PROCEDURE=", F$ENVIRONMENT("VERIFY_PROCEDURE"), " VERIFY_IMAGE=", F$ENVIRONMENT("VERIFY_IMAGE")`,
		`$ WRITE SYS$OUTPUT "F$VERIFY=", F$VERIFY()`,
		`$ SET VERIFY`,
		`$ WRITE SYS$OUTPUT "EXONLY AFTER SET VERIFY"`,
		`$ WRITE SYS$OUTPUT "PROCEDURE=", F$ENVIRONMENT("PROCEDURE")`,
		`$ EXONLY_SYMBOL == "SET BY EXONLY"`,
		`$ WRITE SYS$OUTPUT "EXONLY LAST LINE"`,
		`$ EXIT 1`,
	}},
	{"EXNEST", "(S:E,O:RWED,G:E,W:E)", []string{
		`$ WRITE SYS$OUTPUT "EXNEST LINE 1 RAN"`,
		`$ @PROT93$DISK:[SEC93.EXO]EXONLY`,
		`$ WRITE SYS$OUTPUT "EXNEST AFTER EXONLY, VERIFY_PROCEDURE=", F$ENVIRONMENT("VERIFY_PROCEDURE")`,
		`$ @PROT93$DISK:[SEC93.EXO]EXREAD`,
		`$ WRITE SYS$OUTPUT "EXNEST LAST LINE"`,
	}},
	{"EXREAD", "(S:R,O:RWED,G:R,W:R)", []string{
		`$ WRITE SYS$OUTPUT "EXREAD LINE 1 RAN, VERIFY_PROCEDURE=", F$ENVIRONMENT("VERIFY_PROCEDURE")`,
		`$ WRITE SYS$OUTPUT "EXREAD LAST LINE"`,
	}},
	{"EXDATA", "(S:E,O:RWED,G:E,W:E)", []string{
		`$ WRITE SYS$OUTPUT "EXDATA LINE 1 RAN"`,
		`$ READ SYS$INPUT LINE`,
		`A LINE OF DATA`,
		`$ WRITE SYS$OUTPUT "EXDATA READ: ", LINE`,
		`$ TYPE SYS$INPUT`,
		`DATA TYPED FROM AN EXECUTE-ONLY PROCEDURE`,
		`$ WRITE SYS$OUTPUT "EXDATA LAST LINE"`,
	}},
	{"EXERR", "(S:E,O:RWED,G:E,W:E)", []string{
		`$ WRITE SYS$OUTPUT "EXERR LINE 1 RAN"`,
		`$ NOSUCHVERB`,
		`$ WRITE SYS$OUTPUT "EXERR NOT REACHED"`,
	}},
}

// writeProcedure writes a procedure's lines with OPEN/WRITE, each quote
// doubled.
func (p *proc) writeProcedure(spec string, lines []string) {
	p.line("$ OPEN/WRITE EXOF %s", spec)

	for _, l := range lines {
		p.line("$ WRITE EXOF \"%s\"", strings.ReplaceAll(l, `"`, `""`))
	}

	p.line("$ CLOSE EXOF")
}

func (p *proc) makeObject(o object) {
	if o.dir {
		// A directory: [SEC93.SET]SD1.DIR is [SEC93.SET.SD1].
		i := strings.Index(o.name, "]")
		parent := o.name[:i]
		base := strings.TrimSuffix(o.name[i+1:], ".DIR")
		p.line("$ CREATE/DIRECTORY %s.%s]", parent, base)
	} else {
		p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, o.name)
	}

	p.line("$ SET SECURITY/OWNER=%s/PROTECTION=%s %s", o.owner, o.prot, o.name)

	if len(o.acl) > 0 {
		var aces []string
		for _, a := range o.acl {
			aces = append(aces, "("+a+")")
		}

		acl := strings.Join(aces, ",")
		if len(aces) > 1 {
			acl = "(" + acl + ")"
		}

		p.line("$ SET SECURITY/ACL=%s %s", acl, o.name)
	}
}

func buildSystem() string {
	var p proc

	p.header("SEC93", "SYSTEM's run. Run it as SYSTEM, with the round 3 volume mounted /SYSTEM:\n$ !\n$ !     @DUA1:[000000]SEC93\n$ !\n$ ! Its log is PROT93$DISK:[PROT93LOG]SEC93.LOG",
		`"SEC93.LOG"`)

	p.section("ENVIRONMENT")
	p.line("$ WRITE SYS$OUTPUT F$GETSYI(\"VERSION\"), \" \", F$GETSYI(\"HW_NAME\"), \" \", F$GETSYI(\"NODENAME\")")
	p.line("$ SHOW LOGICAL PROT93$DISK")
	p.line("$ WRITE SYS$OUTPUT F$GETJPI(\"\",\"USERNAME\"), \" \", F$GETJPI(\"\",\"UIC\")")
	p.line("$ SHOW PROCESS/PRIVILEGES")
	p.line("$ SHOW PROTECTION")
	p.line("$ SAVED_PROTECTION = F$ENVIRONMENT(\"PROTECTION\")")

	p.section("ACCOUNTS")
	p.line("$ HOME = F$ENVIRONMENT(\"DEFAULT\")")
	p.line("$ SET DEFAULT SYS$SYSTEM")
	p.line("$ RUN SYS$SYSTEM:AUTHORIZE")

	for _, l := range []string{
		account("PROT93A", "[200,1]", "TMPMBX,NETMBX"),
		account("PROT93B", "[200,2]", "TMPMBX,NETMBX,READALL,GRPPRV,SYSPRV,BYPASS"),
		account("PROT93C", "[300,1]", "TMPMBX,NETMBX,READALL,GRPPRV,SYSPRV,BYPASS"),
		"ADD/IDENTIFIER PROT93_TEAM/VALUE=IDENTIFIER=65537",
		"GRANT/IDENTIFIER PROT93_TEAM PROT93C",
		"SHOW/IDENTIFIER/FULL PROT93",
		"SHOW/IDENTIFIER/FULL PROT93_TEAM",
		"SHOW PROT93A/BRIEF",
		"SHOW PROT93B/BRIEF",
		"SHOW PROT93C/BRIEF",
		"EXIT",
	} {
		p.line("%s", l)
	}

	p.line("$ SET DEFAULT 'HOME'")
	p.line("$ WRITE SYS$OUTPUT F$IDENTIFIER(\"PROT93_TEAM\",\"NAME_TO_NUMBER\")")
	p.line("$ WRITE SYS$OUTPUT F$IDENTIFIER(\"PROT93\",\"NAME_TO_NUMBER\")")

	p.section("BUILD")
	p.line("$ SET DEFAULT %s[SEC93]", disk)
	p.line("$ MACRO/LIST=%s[SEC93]SEC93.LIS/OBJECT=%s[SEC93]SEC93.OBJ %s[000000]SEC93", disk, disk, disk)
	p.line("$ LINK/MAP=%s[SEC93]SEC93.MAP/FULL SEC93", disk)
	p.line("$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) %s[SEC93]SEC93.EXE", disk)
	p.line("$ SHOW SECURITY %s[SEC93]SEC93.EXE", disk)
	p.line("$ SEARCH %s[SEC93]SEC93.MAP SECURESHR,SYS$PARSE_ACL,SYS$FORMAT_ACL,SYS$CHANGE_ACL,SYS$CHECK_ACCESS,SYS$CHKPRO,SYS$GET_SECURITY,SYS$SET_SECURITY,SYS$SETDFPROT", disk)
	p.line("$ SET DEFAULT 'HOME'")

	p.section("VOLUME")
	p.line("$ SET VOLUME/PROTECTION=(S:RWCD,O:RWCD,G:RWCD,W:RWCD) %s", disk)
	p.line("$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) %s[000000]000000.DIR", disk)
	p.line("$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) %s[000000]PROT93LOG.DIR", disk)
	// govax's copies have SYSTEM's default protection (round 2's first
	// try): what the account reads is opened here, on the VAX.
	p.line("$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) %s[000000]SEC93U.COM", disk)
	p.line("$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RWED,W:RWED) %s[PROT93LOG]T.DAT", disk)
	p.line("$ SHOW SECURITY/CLASS=VOLUME %s", disk)
	p.line("$ DIRECTORY/SECURITY %s[000000]", disk)

	p.section("THE TREE")
	p.line("$ DEFINE SEC93$CHG %s", chgDir)
	p.line("$ DEFINE SEC93$GET %s", getDir)

	for _, d := range []string{chgDir, getDir, setDir, chkDir, npDir, disk + "[SEC93.EXO]"} {
		p.line("$ CREATE/DIRECTORY %s", d)
	}

	p.line("$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) %s[000000]SEC93.DIR", disk)
	p.line("$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) %s[SEC93]NP.DIR", disk)
	p.line("$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) %s[SEC93]EXO.DIR", disk)
	p.line("$ SET SECURITY/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE) %s[SEC93]CHK.DIR", disk)

	for _, k := range cases {
		for _, o := range k.files {
			p.makeObject(o)
		}
	}

	for _, e := range exoProcs {
		spec := disk + "[SEC93.EXO]" + e.name + ".COM"
		p.writeProcedure(spec, e.lines)
		p.line("$ SET SECURITY/OWNER=[200,1]/PROTECTION=%s %s", e.prot, spec)
	}

	p.line("$ DIRECTORY/SECURITY %s[SEC93...]*.*;*", disk)

	p.section("PROGRAM, ALL PRIVILEGES")
	p.line(showPrivs)
	p.run("PFCKHGSDQ")

	p.section("PROGRAM, NO PRIVILEGES")
	p.line(dropPrivs)
	p.line(showPrivs)
	p.run("QM")
	p.line(restorePrivs)
	p.line(showPrivs)

	p.section("THE TREE AFTER")
	p.line("$ DIRECTORY/SECURITY %s[SEC93...]*.*;*", disk)
	p.line("$ SHOW PROTECTION")

	p.section("EXECUTE-ONLY PROCEDURES AS SYSTEM")
	p.line("$ @%s[SEC93.EXO]EXONLY", disk)
	p.line("$ SET VERIFY")
	p.line("$ SHOW SYMBOL EXONLY_SYMBOL")

	p.section("MACROS ROUND 9")
	p.line("$ SET DEFAULT %s[SEC93MAC]", disk)
	p.line("$ @MACROS9/OUTPUT=MACROS9.LOG")
	p.line("$ SET VERIFY")
	p.line("$ DIRECTORY %s[SEC93MAC]*.OBJ,*.ANL", disk)
	p.line("$ SET DEFAULT 'HOME'")

	p.section("END")
	p.line("$ WRITE SYS$OUTPUT \"SEC93 FINISHED\"")
	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

// buildUser is PROT93C's run: the program's U part, then the
// execute-only procedures.
func buildUser() string {
	var p proc

	p.header("SEC93U", "PROT93C's run. Log in as PROT93C (a new login), then:\n$ !\n$ !     @PROT93$DISK:[000000]SEC93U\n$ !\n$ ! Its log is PROT93$DISK:[PROT93LOG]SEC93U.LOG",
		`"SEC93U.LOG"`)

	// The first lines run before the log's SET VERIFY: they ask what an
	// execute-only procedure does with SET VERIFY off, as the account
	// logged in.
	p.section("ENVIRONMENT")
	p.line("$ WRITE SYS$OUTPUT F$GETJPI(\"\",\"USERNAME\"), \" \", F$GETJPI(\"\",\"UIC\")")
	p.line("$ SHOW PROCESS/PRIVILEGES")
	p.line(showPrivs)

	p.section("PROGRAM, NO PRIVILEGES")
	p.run("U")

	exo := disk + "[SEC93.EXO]"

	p.section("EXECUTE-ONLY, VERIFY ON")
	p.line("$ WRITE SYS$OUTPUT \"VERIFY_PROCEDURE=\", F$ENVIRONMENT(\"VERIFY_PROCEDURE\")")
	p.line("$ @%sEXONLY", exo)
	p.line("$ SHOW SYMBOL $STATUS")
	p.line("$ SHOW SYMBOL EXONLY_SYMBOL")
	p.line("$ WRITE SYS$OUTPUT \"VERIFY_PROCEDURE=\", F$ENVIRONMENT(\"VERIFY_PROCEDURE\")")
	p.line("$ DELETE/SYMBOL/GLOBAL EXONLY_SYMBOL")

	p.section("EXECUTE-ONLY, VERIFY OFF")
	p.line("$ SET NOVERIFY")
	p.line("$ WRITE SYS$OUTPUT \"VERIFY_PROCEDURE=\", F$ENVIRONMENT(\"VERIFY_PROCEDURE\")")
	p.line("$ @%sEXONLY", exo)
	p.line("$ WRITE SYS$OUTPUT \"$STATUS=\", $STATUS")
	p.line("$ WRITE SYS$OUTPUT \"VERIFY_PROCEDURE=\", F$ENVIRONMENT(\"VERIFY_PROCEDURE\")")
	p.line("$ SET VERIFY")

	p.section("EXECUTE-ONLY, NESTED")
	p.line("$ @%sEXNEST", exo)
	p.line("$ SET NOVERIFY")
	p.line("$ @%sEXNEST", exo)
	p.line("$ SET VERIFY")

	p.section("EXECUTE-ONLY, DATA AND AN ERROR")
	p.line("$ @%sEXDATA", exo)
	p.line("$ @%sEXERR", exo)
	p.line("$ SHOW SYMBOL $STATUS")

	p.section("EXECUTE-ONLY, OUTPUT TO A FILE")
	p.line("$ @%sEXONLY/OUTPUT=%s[PROT93LOG]EXO.OUT", exo, disk)
	p.line("$ TYPE %s[PROT93LOG]EXO.OUT", disk)

	p.section("EXECUTE-ONLY, READ AS A FILE")
	p.line("$ TYPE %sEXONLY.COM", exo)
	p.line("$ OPEN/READ EXOF %sEXONLY.COM", exo)
	p.line("$ CLOSE EXOF")
	p.line("$ WRITE SYS$OUTPUT F$FILE_ATTRIBUTES(\"%sEXONLY.COM\",\"EOF\")", exo)
	p.line("$ WRITE SYS$OUTPUT F$SEARCH(\"%sEXONLY.COM\")", exo)
	p.line("$ DIRECTORY/SECURITY %s", exo)

	p.section("END")
	p.line("$ WRITE SYS$OUTPUT \"SEC93U FINISHED\"")
	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

func buildFinal() string {
	var p proc

	p.header("SEC93Y", "the end. Run it as SYSTEM after PROT93C's run:\n$ !\n$ !     @PROT93$DISK:[000000]SEC93Y\n$ !\n$ ! Its log is PROT93$DISK:[PROT93LOG]SEC93Y.LOG",
		`"SEC93Y.LOG"`)

	p.section("THE TREE")
	p.line("$ DIRECTORY/SECURITY %s[SEC93...]*.*;*", disk)

	p.section("REMOVING THE ACCOUNTS")
	p.line("$ HOME = F$ENVIRONMENT(\"DEFAULT\")")
	p.line("$ SET DEFAULT SYS$SYSTEM")
	p.line("$ RUN SYS$SYSTEM:AUTHORIZE")

	for _, l := range []string{"REMOVE PROT93A", "REMOVE PROT93B", "REMOVE PROT93C", "REMOVE/IDENTIFIER PROT93_TEAM", "REMOVE/IDENTIFIER PROT93", "EXIT"} {
		p.line("%s", l)
	}

	p.line("$ SET DEFAULT 'HOME'")
	p.line("$ WRITE SYS$OUTPUT \"SEC93Y FINISHED\"")
	p.line("$ DEASSIGN/SYSTEM/EXECUTIVE_MODE PROT93$DISK")
	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

// ---------------------------------------------------------------------
// The govax console scripts.

// macroFiles are round 9 of the system service macro probes
// (testdata/mp/macros), assembled on the same volume.
var macroFiles = []string{"r9_change_acl", "r9_parse_acl", "r9_format_acl", "r9_chkpro", "r9_check_access",
	"r9_get_security", "r9_set_security", "r9_setdfprot"}

func exchange() string {
	var b strings.Builder

	b.WriteString(`! EXCHANGE-R3.CMD - builds round 3 of Phase 93's probe on a new volume.
! Written by testdata/prot93/gen3.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange-r3.cmd
!
INITIALIZE/CONTAINER "testdata/disks/sec93-exchange.dsk" /DEVICE=RD53 SEC93
MOUNT/WRITE DUA5 "testdata/disks/sec93-exchange.dsk"
CREATE/DIRECTORY DUA5:[PROT93LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[SEC93]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[SEC93MAC]
`)

	for _, f := range []string{"sec93.mar", "sec93.com", "sec93u.com", "sec93y.com"} {
		fmt.Fprintf(&b, "COPY \"%s/%s\"/HOST DUA5:[000000]%s\n", dir, f, strings.ToUpper(f))
	}

	b.WriteString("COPY \"" + dir + "/sec93t.dat\"/HOST DUA5:[PROT93LOG]T.DAT\n")

	for _, f := range macroFiles {
		fmt.Fprintf(&b, "COPY \"testdata/mp/macros/%s.mar\"/HOST DUA5:[SEC93MAC]%s.MAR\n", f, strings.ToUpper(f))
	}

	b.WriteString("COPY \"testdata/mp/macros/macros9.com\"/HOST DUA5:[SEC93MAC]MACROS9.COM\n")
	b.WriteString("DIRECTORY/OWNER/PROTECTION DUA5:[000000],[PROT93LOG],[SEC93MAC]\n")
	b.WriteString("DISMOUNT DUA5\n")

	return b.String()
}

func copyout() string {
	var b strings.Builder

	b.WriteString(`! COPYOUT-R3.CMD - copies round 3's results off sec93-exchange.dsk:
! the logs, the map, and the program's image into testdata/prot93/vax/round3,
! and round 9 of the macro probes into testdata/mp/macros/vax. Written by
! testdata/prot93/gen3.go. Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout-r3.cmd
!
MOUNT DUA5 "testdata/disks/sec93-exchange.dsk"
`)

	for _, l := range []string{"SEC93.LOG", "SEC93U.LOG", "SEC93Y.LOG"} {
		fmt.Fprintf(&b, "COPY DUA5:[PROT93LOG]%s \"%s/vax/round3/%s\"/HOST/QUIET\n", l, dir, strings.ToLower(l))
	}

	fmt.Fprintf(&b, "COPY DUA5:[SEC93]SEC93.MAP \"%s/vax/round3/sec93.map\"/HOST/QUIET\n", dir)
	fmt.Fprintf(&b, "COPY/BINARY DUA5:[SEC93]SEC93.EXE \"%s/vax/round3/sec93.exe\"/HOST/QUIET\n", dir)
	fmt.Fprintf(&b, "COPY/BINARY DUA5:[SEC93]SEC93.OBJ \"%s/vax/round3/sec93.obj\"/HOST/QUIET\n", dir)

	for _, f := range macroFiles {
		up := strings.ToUpper(f)
		fmt.Fprintf(&b, "COPY/BINARY DUA5:[SEC93MAC]%s.OBJ \"testdata/mp/macros/vax/%s.obj\"/HOST/QUIET\n", up, f)
		fmt.Fprintf(&b, "COPY DUA5:[SEC93MAC]%s.ANL \"testdata/mp/macros/vax/%s.anl\"/HOST/QUIET\n", up, f)
	}

	b.WriteString("COPY DUA5:[SEC93MAC]MACROS9.LOG \"testdata/mp/macros/vax/macros9.log\"/HOST/QUIET\n")
	b.WriteString("DISMOUNT DUA5\n")

	return b.String()
}

func main() {
	readDefined()

	parseCases()
	formatCases()
	changeCases()
	chkproCases()
	checkAccessCases()
	getCases()
	setCases()
	setdfprotCases()
	identityCases('M', "_M")
	identityCases('U', "_U")

	// The program is written last: the procedures' tree comes from the
	// cases, and the program's definitions from every case's use.
	files := map[string]string{
		"sec93.com":       buildSystem(),
		"sec93u.com":      buildUser(),
		"sec93y.com":      buildFinal(),
		"sec93t.dat":      "SEC93 TEST FILE\n",
		"exchange-r3.cmd": exchange(),
		"copyout-r3.cmd":  copyout(),
	}

	files["sec93.mar"] = program()

	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	counts := map[byte]int{}
	for _, k := range cases {
		counts[k.part]++
	}

	var parts []string
	for _, p := range "PFCKHGSDQMU" {
		parts = append(parts, fmt.Sprintf("%c %d", p, counts[byte(p)]))
	}

	fmt.Printf("%d cases (%s)\n", len(cases), strings.Join(parts, ", "))
}
