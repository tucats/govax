package rms

import "strings"

// Indirect access to process-permanent files (ppf.go): what each RMS
// service does with a FAB or RAB whose IFI or ISI is an indirect
// accessor's, from the Guide to OpenVMS File Applications, 6.6, and VMS
// 7.1's answers to testdata/ppf71 (docs/PHASE-71 - process-permanent
// files.md, Part A).

// openIndirect is $OPEN's and $CREATE's work once name processing has led
// to a process-permanent file (p.Permanent), the expanded string and FNB
// written: no file is opened. The FAB gets an indirect IFI, the file's
// attributes, and the device's characteristics; FAB$B_FAC and FAB$L_FOP
// stay as the program gave them. A NAM's resultant string is the
// expanded one, its DVI the logical name, its FID and DID zero. FOP's UFO
// and NFS are RMS$_FOP. A name whose file has been closed is RMS$_IFI
// (unconfirmed).
func (ctx *Context) openIndirect(fab, nam uint32, p parsedName, fop uint32) (uint32, error) {
	if fop&(fopUFO|fopNFS) != 0 {
		return fabStatus(ctx, fab, rmsFileOptionError, 0)
	}

	e, ok := ctx.Files.Permanent(p.Permanent.IFI)
	if !ok {
		return fabStatus(ctx, fab, rmsInvalidIFI, 0)
	}

	// The program's own record attributes, before the file's replace
	// them, are the ones its $PUTs carry (FAB$V_PPF_RAT).
	rat, err := ctx.loadByte(fab + fabRAT)
	if err != nil {
		return 0, err
	}

	if nam != 0 {
		rss, err := ctx.loadByte(nam + namRSS)
		if err != nil {
			return 0, err
		}

		rsa, err := ctx.loadLongword(nam + namRSA)
		if err != nil {
			return 0, err
		}

		overflow, err := ctx.storeNameString(nam, rsa, rss, namRSL, p.fileName)
		if err != nil {
			return 0, err
		}

		if overflow {
			return fabStatus(ctx, fab, rmsRSSError, 0)
		}

		if err := ctx.storeDVI(nam, p.Lookup); err != nil {
			return 0, err
		}
	}

	if err := ctx.storeIndirectAttributes(fab, e, false); err != nil {
		return 0, err
	}

	if err := ctx.storeWord(fab+fabIFI, indirectWord(e.index, rat)); err != nil {
		return 0, err
	}

	return fabStatus(ctx, fab, rmsNormal, 0)
}

// storeIndirectAttributes writes an indirect accessor's FAB: the device's
// characteristics in FAB$L_SDC, and in FAB$L_DEV as a record device's
// (VMS 7.1: a disk's without RND, FOD, and DIR, with REC and CCL); then a
// volume file's attributes, or for another stream the fixed control size
// ($OPEN; at the terminal VMS 7.1 left the rest of the FAB as given) or
// the stream's record attributes ($DISPLAY, which full asks for).
func (ctx *Context) storeIndirectAttributes(fab uint32, e *PermanentEntry, full bool) error {
	sdc := e.characteristics
	dev := sdc&^(devRND|devFOD|devDIR) | devREC | devCCL

	if err := ctx.storeLongword(fab+fabDEV, dev); err != nil {
		return err
	}

	if err := ctx.storeLongword(fab+fabSDC, sdc); err != nil {
		return err
	}

	h := e.handle
	if h.File != nil {
		return ctx.fillFABAttributes(fab, h.File)
	}

	// The terminal's process-permanent files are VFC print files with
	// two bytes of fixed control (VMS 7.1's $DISPLAY); a record device's
	// govax reports as variable records with carriage return
	// (unconfirmed).
	rfm, rat, fsz := byte(rfmVFC), byte(ratPRN), byte(2)
	if h.IsRecordDevice() {
		rfm, rat, fsz = rfmVAR, ratCR, 0
	}

	if !full {
		return ctx.storeByte(fab+fabOffset("FSZ"), fsz)
	}

	for _, s := range []func() error{
		func() error { return ctx.storeLongword(fab+fabOffset("ALQ"), 0) },
		func() error { return ctx.storeByte(fab+fabORG, orgSeq) },
		func() error { return ctx.storeByte(fab+fabRFM, rfm) },
		func() error { return ctx.storeByte(fab+fabRAT, rat) },
		func() error { return ctx.storeWord(fab+fabMRS, 0) },
		func() error { return ctx.storeByte(fab+fabOffset("FSZ"), fsz) },
	} {
		if err := s(); err != nil {
			return err
		}
	}

	return nil
}

// displayIndirect is $DISPLAY of an indirect accessor's FAB: the file
// as the open that made it opened it. The FAB gets the attributes, that
// open's FAB$B_FAC, and in FAB$L_STV an address in P1 on VMS (VMS 7.1:
// ^X7FF9C000 plus the stream's number; govax writes the same, meaning
// nothing); a NAM the file's resultant string, FID, DID, and DVI, as for
// any open file. The terminal's NAM gets the device's DVI, and for an
// output file a resultant string of its own (storeTerminalResultant).
func (ctx *Context) displayIndirect(fab uint32, e *PermanentEntry) (uint32, error) {
	nam, sts, err := fabNAMBlock(ctx, fab)
	if err != nil {
		return 0, err
	}

	if sts != 0 {
		return fabStatus(ctx, fab, sts, 0)
	}

	h := e.handle

	if nam != 0 {
		if h.Found != nil {
			sts, err := ctx.fillNAM(fab, nam, *h.Found, namOutputs{Resultant: true})
			if err != nil {
				return 0, err
			}

			if sts != 0 {
				return fabStatus(ctx, fab, sts, 0)
			}
		} else {
			if err := ctx.storeDVI(nam, strings.TrimSuffix(e.options.Device, ":")); err != nil {
				return 0, err
			}

			if err := ctx.storeTerminalResultant(nam, e); err != nil {
				return 0, err
			}
		}
	}

	if err := ctx.storeIndirectAttributes(fab, e, true); err != nil {
		return 0, err
	}

	if err := ctx.storeByte(fab+fabFAC, h.Access); err != nil {
		return 0, err
	}

	if h.File != nil {
		chain, sts, stv, err := ctx.xabChain(fab)
		if err != nil {
			return 0, err
		}

		if sts != 0 {
			return fabStatus(ctx, fab, sts, stv)
		}

		if err := ctx.fillXABs(chain, h.File, xabDisplay); err != nil {
			return 0, err
		}
	}

	return fabStatus(ctx, fab, rmsNormal, ppfDisplaySTV|uint32(e.stream))
}

// storeTerminalResultant is $DISPLAY's resultant string for a terminal
// that is a process-permanent output file: the device of the expanded
// string the accessor's open wrote (the logical name used), then "[].LOG;",
// as VMS 7.1 wrote "SYS$OUTPUT:[].LOG;" (testdata/ppf71's terminal run;
// the name and type left from LOGINOUT's open of the file, it seems). An
// input file's, or another stream's, is left as the open wrote it
// (unconfirmed).
func (ctx *Context) storeTerminalResultant(nam uint32, e *PermanentEntry) error {
	if !e.IsTerminal() || e.handle.Access&facPut == 0 {
		return nil
	}

	esa, err := ctx.loadLongword(nam + namESA)
	if err != nil {
		return err
	}

	esl, err := ctx.loadByte(nam + namESL)
	if err != nil {
		return err
	}

	expanded, err := ctx.loadFixedString(esa, int(esl))
	if err != nil {
		return err
	}

	device, _, found := strings.Cut(expanded, ":")
	if esa == 0 || !found {
		return nil
	}

	return ctx.storeResultant(nam, fileName{Dev: device + ":", Dir: "[]", Type: ".LOG", Ver: ";"})
}

// ppfDisplaySTV is the base of $DISPLAY's FAB$L_STV for an indirect
// accessor (see displayIndirect).
const ppfDisplaySTV = 0x7FF9C000

// parseIndirect is the rest of $PARSE once name processing has led to a
// process-permanent file: the NAM's DVI is the logical name, its DID
// zero, and FAB$L_DEV and FAB$L_SDC both the device's characteristics
// (VMS 7.1). A name whose file has been closed is RMS$_DEV, as a device
// that isn't there (unconfirmed).
func (ctx *Context) parseIndirect(fab, nam uint32, p parsedName) (sts, stv uint32, err error) {
	e, ok := ctx.Files.Permanent(p.Permanent.IFI)
	if !ok {
		return rmsDeviceError, ssNoSuchDevice, nil
	}

	if err := ctx.storeDVI(nam, p.Lookup); err != nil {
		return 0, 0, err
	}

	for _, off := range []uint32{fabDEV, fabSDC} {
		if err := ctx.storeLongword(fab+off, e.characteristics); err != nil {
			return 0, 0, err
		}
	}

	return rmsNormal, 0, nil
}

// indirectAccess checks an indirect accessor's record operation against
// the access of the open that made the file (FAB$B_FAC is ignored for
// an indirect accessor): need is the access the operation takes. It
// returns 0 when the operation may go on, which it always may on a
// stream that isn't indirect, or the RMS$_FAC it stored in the RAB.
func (ctx *Context) indirectAccess(rabAddr uint32, need byte) (uint32, error) {
	isi, err := ctx.loadWord(rabAddr + rabISI)
	if err != nil {
		return 0, err
	}

	e, ok := ctx.Files.indirectStream(isi)
	if !ok || e.handle.Access&need != 0 {
		return 0, nil
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsFACNotAllowed)
}

// indirectControl is the fixed control an indirect accessor's $PUT gives
// a record of a VFC file, isi its RAB$W_ISI: a print file's carriage
// control from the record attributes the accessor's FAB had at $OPEN
// (FAB$V_PPF_RAT): carriage return, "a line feed before, a carriage
// return after" (01 8D, as DCL's own records); otherwise none, zeros
// (unconfirmed). It is nil for a stream that isn't indirect or a file
// without fixed control.
func (ctx *Context) indirectControl(isi uint16, h *FileHandle) []byte {
	if !isIndirect(isi) || h.File == nil {
		return nil
	}

	ra := h.File.Header.RecordAttributes

	control := make([]byte, ra.VfcSize)
	if ra.VfcSize == 2 && ra.Attributes&ratPRN != 0 && wordRAT(isi)&ratCR != 0 {
		copy(control, dclRecordControl)
	}

	return control
}
