package rms

// SysDisplay implements SYS$DISPLAY (docs/PHASE-33.md, subtask 5): it
// reports an open file's attributes again, as $OPEN did.
//
// From the RMS Reference Manual's $DISPLAY description. The file is the
// one FAB$W_IFI names (RMS$_IFI if none). Its attributes go into the FAB
// and every XAB of the chain FAB$L_XAB names; a NAM at FAB$L_NAM gets the
// resultant string, FNB, FID, DID, and DVI.
func SysDisplay(ctx *Context, argv []uint32) (uint32, error) {
	if len(argv) < 1 {
		return ssInsufficientArgs, nil
	}

	fab := argv[0]

	ifi, err := ctx.loadWord(fab + fabIFI)
	if err != nil {
		return 0, err
	}

	h, ok := ctx.Files.Lookup(ifi)
	if !ok || h.IsConsole() || h.IsRecordDevice() {
		return fabStatus(ctx, fab, rmsInvalidIFI, 0)
	}

	nam, sts, err := fabNAMBlock(ctx, fab)
	if err != nil {
		return 0, err
	}

	if sts != 0 {
		return fabStatus(ctx, fab, sts, 0)
	}

	chain, sts, stv, err := ctx.xabChain(fab)
	if err != nil {
		return 0, err
	}

	if sts != 0 {
		return fabStatus(ctx, fab, sts, stv)
	}

	if nam != 0 && h.Found != nil {
		sts, err := ctx.fillNAM(fab, nam, *h.Found, namOutputs{Resultant: true})
		if err != nil {
			return 0, err
		}

		if sts != 0 {
			return fabStatus(ctx, fab, sts, 0)
		}
	}

	if err := ctx.fillFABAttributes(fab, h.File); err != nil {
		return 0, err
	}

	if err := ctx.fillXABs(chain, h.File, xabDisplay); err != nil {
		return 0, err
	}

	return fabStatus(ctx, fab, rmsNormal, 0)
}
