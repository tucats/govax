package rms

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
)

// Trace describes an RMS service's control block after the service ran,
// for SET DEBUG RMS (corevms's rms.go prints it): service is the name
// (SYS$CREATE), block the FAB's or RAB's address, and r0 the status the
// service returned. A FAB shows its file and default names, access,
// sharing, organization, record format and size, and IFI; a RAB its
// stream, access mode, options, record (or, for RAB$C_KEY, the record
// number), and the stream's kind, terminal, record device, indirect
// (process-permanent), or volume or host file. Fields that can't be read
// are left out.
func Trace(ctx *Context, service string, block, r0 uint32) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s %08X", service, block)

	if isRABService(service) {
		ctx.traceRAB(&b, block)
	} else {
		ctx.traceFAB(&b, block)
	}

	fmt.Fprintf(&b, ", %s", StatusName(r0))

	return b.String()
}

// rabServices are the services whose argument is a RAB; the others' is a
// FAB.
var rabServices = map[string]bool{
	"SYS$CONNECT": true, "SYS$GET": true, "SYS$PUT": true, "SYS$FIND": true,
	"SYS$UPDATE": true, "SYS$DELETE": true, "SYS$TRUNCATE": true,
	"SYS$REWIND": true, "SYS$FLUSH": true, "SYS$FREE": true,
	"SYS$RELEASE": true, "SYS$WAIT": true, "SYS$READ": true, "SYS$WRITE": true,
	"SYS$DISCONNECT": true,
}

func isRABService(service string) bool { return rabServices[service] }

// StatusName is a status code's message identifier with its severity
// letter and value, RMS$_FAC/F (00018514), or its hex value when no
// message has the code.
func StatusName(sts uint32) string {
	// RMS$_NORMAL, the RMS facility's message 0, has no message text.
	if sts == rmsNormal {
		return fmt.Sprintf("RMS$_NORMAL/S (%08X)", sts)
	}

	msg, ok := vmsdef.LookupMessage(sts)
	if !ok {
		return fmt.Sprintf("%08X", sts)
	}

	facility := msg.Facility
	if facility == "SYSTEM" {
		facility = "SS"
	}

	return fmt.Sprintf("%s$_%s/%c (%08X)", facility, msg.Ident, "WSEIF???"[sts&7], sts)
}

func (ctx *Context) traceFAB(b *strings.Builder, fab uint32) {
	str := func(addrOff, sizeOff uint32) string {
		addr, err := ctx.loadLongword(fab + addrOff)
		if err != nil || addr == 0 {
			return ""
		}

		n, err := ctx.loadByte(fab + sizeOff)
		if err != nil {
			return ""
		}

		s, err := ctx.loadFixedString(addr, int(n))
		if err != nil {
			return "?"
		}

		return s
	}

	fmt.Fprintf(b, " name %q", str(fabFNA, fabFNS))

	if dn := str(fabDNA, fabDNS); dn != "" {
		fmt.Fprintf(b, " default %q", dn)
	}

	for _, f := range []struct {
		label string
		off   uint32
	}{{"FAC", fabFAC}, {"SHR", fabSHR}, {"ORG", fabORG}, {"RFM", fabRFM}, {"RAT", fabRAT}} {
		if v, err := ctx.loadByte(fab + f.off); err == nil {
			fmt.Fprintf(b, " %s %02X", f.label, v)
		}
	}

	if v, err := ctx.loadWord(fab + fabMRS); err == nil {
		fmt.Fprintf(b, " MRS %d", v)
	}

	if v, err := ctx.loadLongword(fab + fabFOP); err == nil {
		fmt.Fprintf(b, " FOP %08X", v)
	}

	// The fields a program rarely sets are shown only when they aren't
	// 0: allocation and extension, bucket and fixed-control sizes, the
	// relative file's maximum record number, and the NAM and XAB chains.
	for _, f := range []struct {
		label string
		off   uint32
		size  int
	}{
		{"ALQ", fabOffset("ALQ"), 4}, {"DEQ", fabOffset("DEQ"), 2},
		{"BKS", fabOffset("BKS"), 1}, {"FSZ", fabOffset("FSZ"), 1},
		{"MRN", fabOffset("MRN"), 4}, {"NAM", fabNAM, 4}, {"XAB", fabOffset("XAB"), 4},
	} {
		var v uint32

		var err error

		switch f.size {
		case 1:
			var x uint8
			x, err = ctx.loadByte(fab + f.off)
			v = uint32(x)
		case 2:
			var x uint16
			x, err = ctx.loadWord(fab + f.off)
			v = uint32(x)
		default:
			v, err = ctx.loadLongword(fab + f.off)
		}

		if err == nil && v != 0 {
			if f.size == 4 && (f.label == "NAM" || f.label == "XAB") {
				fmt.Fprintf(b, " %s %08X", f.label, v)
			} else {
				fmt.Fprintf(b, " %s %d", f.label, v)
			}
		}
	}

	ctx.traceXABs(b, fab)

	if v, err := ctx.loadWord(fab + fabIFI); err == nil {
		fmt.Fprintf(b, " IFI %04X", v)
	}

	if v, err := ctx.loadLongword(fab + fabSTV); err == nil && v != 0 {
		fmt.Fprintf(b, " STV %08X", v)
	}
}

func (ctx *Context) traceRAB(b *strings.Builder, rab uint32) {
	isi, err := ctx.loadWord(rab + rabISI)
	if err != nil {
		return
	}

	fmt.Fprintf(b, " ISI %04X", isi)

	switch e, h := ctx.traceStream(isi); {
	case e != nil:
		fmt.Fprintf(b, " (indirect, access %02X)", e.handle.Access)
	case h == nil:
		b.WriteString(" (no stream)")
	case h.IsConsole():
		fmt.Fprintf(b, " (terminal, access %02X)", h.Access)
	case h.IsRecordDevice():
		fmt.Fprintf(b, " (record device, access %02X)", h.Access)
	default:
		fmt.Fprintf(b, " (file, access %02X)", h.Access)
	}

	rac, _ := ctx.loadByte(rab + rabRAC)
	rop, _ := ctx.loadLongword(rab + rabROP)
	fmt.Fprintf(b, " RAC %d ROP %08X", rac, rop)

	// Multiblock and multibuffer counts, shown when a program sets them.
	if v, err := ctx.loadByte(rab + rabOffset("MBC")); err == nil && v != 0 {
		fmt.Fprintf(b, " MBC %d", v)
	}

	if v, err := ctx.loadByte(rab + rabOffset("MBF")); err == nil && v != 0 {
		fmt.Fprintf(b, " MBF %d", int8(v))
	}

	if rac == racKey {
		if kbf, err := ctx.loadLongword(rab + rabKBF); err == nil {
			if n, err := ctx.loadLongword(kbf); err == nil {
				fmt.Fprintf(b, " record %d", n)
			}
		}
	}

	rsz, err := ctx.loadWord(rab + rabRSZ)
	if err != nil {
		return
	}

	fmt.Fprintf(b, " RSZ %d", rsz)

	if rbf, err := ctx.loadLongword(rab + rabRBF); err == nil && rbf != 0 && rsz > 0 {
		if s, err := ctx.loadFixedString(rbf, int(min(rsz, 60))); err == nil {
			fmt.Fprintf(b, " %q", s)
		}
	}

	if v, err := ctx.loadLongword(rab + rabSTV); err == nil && v != 0 {
		fmt.Fprintf(b, " STV %08X", v)
	}
}

// traceStream is the stream a RAB's ISI names: an indirect accessor's
// process-permanent entry, or a file handle.
func (ctx *Context) traceStream(isi uint16) (*PermanentEntry, *FileHandle) {
	if ctx.Files == nil {
		return nil, nil
	}

	if e, ok := ctx.Files.indirectStream(isi); ok {
		return e, nil
	}

	if h, ok := ctx.Files.LookupStream(isi); ok {
		return nil, h
	}

	return nil, nil
}

// xabNames are the XAB types the trace names; any other shows its code.
var xabNames = map[byte]string{
	xabALL: "ALL", xabDAT: "DAT", xabFHC: "FHC", xabITM: "ITM", xabKEY: "KEY",
	xabPRO: "PRO", xabRDT: "RDT", xabSUM: "SUM", xabTRM: "TRM",
}

// traceXABs shows the types in a FAB's XAB chain, in brackets, and an
// XABALL's allocation and extension quantities, which a program sets
// there rather than in the FAB.
func (ctx *Context) traceXABs(b *strings.Builder, fab uint32) {
	addr, err := ctx.loadLongword(fab + fabOffset("XAB"))
	if err != nil || addr == 0 {
		return
	}

	b.WriteString(" [")

	for n := 0; addr != 0 && n < maxXABs; n++ {
		cod, err := ctx.loadByte(addr + xabCOD)
		if err != nil {
			b.WriteString("?")

			break
		}

		if n > 0 {
			b.WriteString(" ")
		}

		if name, ok := xabNames[cod]; ok {
			b.WriteString(name)
		} else {
			fmt.Fprintf(b, "%d", cod)
		}

		if cod == xabALL {
			alq, _ := ctx.loadLongword(addr + xabALQ)
			deq, _ := ctx.loadWord(addr + xabDEQ)
			fmt.Fprintf(b, " ALQ %d DEQ %d", alq, deq)
		}

		if addr, err = ctx.loadLongword(addr + xabNXT); err != nil {
			break
		}
	}

	b.WriteString("]")
}
