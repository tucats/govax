package rms

// SysPut implements SYS$PUT: given a RAB's VAX address (argv[0] — SYS$PUT
// takes exactly one argument, like SYS$CONNECT and unlike SYS$CREATE which
// also takes a FAB), writes one sequential record to whatever file the RAB
// is currently connected to (via SYS$CONNECT — see connect.go), reading
// the record's bytes out of emulated VAX memory at the address and length
// the calling program set in the RAB's own RAB$L_RBF/RAB$W_RSZ fields.
//
// # A quick Go note for readers new to the language
//
// argv is a slice (Go's dynamically-sized array type) of uint32 VAX
// memory addresses — every service handler in this emulator is called
// with the same "(argv []uint32) (uint32, error)" shape (see
// internal/rtl's ServiceTable), so argv[0] is simply "the first, and in
// SYS$PUT's case only, argument the calling VAX program passed". The
// (uint32, error) the function returns is Go's ordinary two-result
// convention for "here's the value, and here's whether producing it
// failed": the uint32 becomes register R0 back in the emulated VAX
// program (the real system-service return-status convention), and a
// non-nil error means something went wrong at the Go/emulator level
// (bad memory access) rather than an ordinary RMS-level failure the VAX
// program itself is expected to check for and handle — see status.go's
// storeStatus doc comment for how those two very different kinds of
// "failure" are kept distinct throughout this package.
//
// # Why this only implements sequential access
//
// Real RMS's RAB$B_RAC field can ask for sequential, keyed (indexed
// files), or direct-by-RFA access, but docs/PHASE-22.md scopes this whole
// phase to sequential organization only (see fab.go's orgSeq and rab.go's
// racSeq) — so any other RAB$B_RAC value is a real RMS$_RAC error to
// report back to the calling program, not something to silently
// misinterpret as sequential anyway.
func SysPut(ctx *Context, argv []uint32) (uint32, error) {
	rabAddr := argv[0]

	// A RAB only ever refers to a file through the IFI SYS$CONNECT
	// already copied into its RAB$W_ISI field (rab.go's rabISI) — SYS$PUT
	// itself never touches the RAB's related FAB directly, unlike
	// SYS$CONNECT which has to chase rabFAB to find it.
	ifi, err := ctx.loadWord(rabAddr + rabISI)
	if err != nil {
		return 0, err
	}

	handle, ok := ctx.Files.Lookup(ifi)
	if !ok {
		// Lookup returning false means this RAB was never SYS$CONNECTed
		// at all (RAB$W_ISI is still whatever zero value newFAB-style
		// test fixtures or an uninitialized VAX program leave it at,
		// which falls in ifi.go's reserved-and-never-allocated 0-3
		// range) — a real, expected RMS$_IFI condition, not a bug in
		// this package.
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsInvalidIFI)
	}

	rac, err := ctx.loadByte(rabAddr + rabRAC)
	if err != nil {
		return 0, err
	}

	if rac != racSeq {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsInvalidRAC)
	}

	// A real ODS-2-backed file has to have been armed for writing by
	// SYS$CONNECT (connect.go's armForFAC) before SYS$PUT can use it —
	// armForFAC itself already reports RMS$_PRV at CONNECT time for a
	// FAB that never asked for FAB$V_PUT access, so a nil Writer here
	// means this RAB's FAB was instead armed for reading only (SYS$OPEN/
	// SYS$CONNECT with FAB$V_GET, docs/PHASE-22.md's later subtasks) and
	// a PUT was attempted through it anyway — the same "no write access"
	// condition, just discovered one step later. The console case needs
	// no such check: a FileHandle's Console field is ready to write to
	// the moment SYS$CREATE allocates it (see ifi.go's FileHandle doc
	// comment).
	if !handle.IsConsole() && handle.Writer == nil {
		return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsPrivilegeViolation)
	}

	// RAB$W_RSZ/RAB$L_RBF (rab.go) are, on SYS$PUT, both set by the
	// calling program before the call: RSZ says how many bytes long the
	// outgoing record is, and RBF is the VAX-memory address the record's
	// actual bytes live at. loadFixedString reads exactly that many bytes
	// verbatim — record data is arbitrary binary content, not a
	// NUL-terminated C string, so nothing shorter or longer than the
	// declared length would be correct (same reasoning as fabFNS's own
	// doc comment for file-specification strings).
	rsz, err := ctx.loadWord(rabAddr + rabRSZ)
	if err != nil {
		return 0, err
	}

	rbf, err := ctx.loadLongword(rabAddr + rabRBF)
	if err != nil {
		return 0, err
	}

	recordString, err := ctx.loadFixedString(rbf, int(rsz))
	if err != nil {
		return 0, err
	}

	// A Go string's bytes are already exactly what loadFixedString read
	// from VAX memory (Go strings are just immutable byte sequences —
	// converting one to []byte, as done here, never re-interprets or
	// validates the bytes as text the way it might in a language with a
	// stricter string type). record is what both the console-write and
	// the ods2 Writer.Put paths below actually consume.
	record := []byte(recordString)

	if handle.IsConsole() {
		if _, err := handle.Console.Write(record); err != nil {
			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsDeviceError)
		}

		// Real RMS has no notion of "line endings" for the terminal
		// pseudo-device the way this emulator's console actually
		// works — each SYS$PUT is logically one output line, so a
		// newline is appended after every record written this way,
		// exactly matching the deleted Phase 10 stopgap's own
		// console-output behavior (git history: internal/rtl/rms.go's
		// serviceSysPut, "if fabIfi <= 3").
		if _, err := handle.Console.Write([]byte{'\n'}); err != nil {
			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsDeviceError)
		}
	} else {
		if err := handle.Writer.Put(record); err != nil {
			// The sibling ods2 module's rms.Writer.Put only ever
			// fails this way for a record whose length doesn't fit
			// the file's own declared record format — exactly
			// matching real RMS's RMS$_RSZ ("record size is
			// invalid"): too long for a Fixed-format file's single
			// declared size, wrong length generally for one, or
			// over the 65535-byte length-prefix limit a
			// Variable/VFC-format file's framing can represent. See
			// status.go's own rmsRecordTooBig doc comment, which
			// anticipated exactly this SYS$PUT use.
			return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsRecordTooBig)
		}
	}

	return storeStatus(ctx, rabAddr, rabSTS, rabSTV, rmsNormal)
}
