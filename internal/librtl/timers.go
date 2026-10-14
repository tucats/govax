package librtl

import (
	"fmt"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// The performance measurement routines (docs/PHASE-55, the survey's
// "Timers"; the 5.0 manual's section 2.5): LIB$INIT_TIMER records where
// the process stands -- the time, its CPU time, its buffered and direct
// I/O counts, and its page faults -- and LIB$SHOW_TIMER and
// LIB$STAT_TIMER report how far each has moved since. LIB$FREE_TIMER
// frees a block INIT_TIMER allocated.
//
// The figures are kept in a timer block: one static block, LIBRTL's own
// data (kept in Go per image, corevms.Environment.ImageData), or any
// number of blocks on the heap, which the program names by a handle, a
// longword holding the block's address. A heap block is in the program's
// memory, laid out as timerBlock* below, as VMS 7.1 laid one out
// (testdata/lib55): it starts with the tag "TIMR", which is how the
// routines tell a block from another address (LIB$_INVARG). They read
// the tag wherever the handle points, so a handle of 0 is an access
// violation, as on VMS.

var libBadBlockAddress = vmsdef.LibrarySymbols["LIB$_BADBLOADR"]

// A heap timer block's layout: the tag, the CPU time (10ms units),
// buffered I/O, direct I/O, and page faults, a longword each, then the
// time INIT_TIMER filled it (a VMS time, quadword). VMS 7.1's block had
// the tag, four counts, and the time at offset 20; which count is which
// is unconfirmed (the statistic codes' order is assumed).
const (
	timerBlockTag    = 0
	timerBlockCPU    = 4
	timerBlockBufIO  = 8
	timerBlockDirIO  = 12
	timerBlockFaults = 16
	timerBlockTime   = 20
	timerBlockSize   = 28
)

// timerTag is a heap timer block's first longword: "TIMR" in ASCII.
const timerTag = 0x524D4954

// The statistic codes SHOW_TIMER and STAT_TIMER take. SHOW_TIMER's 0 (or
// none) is all five.
const (
	timerElapsed = 1 // elapsed time
	timerCPU     = 2 // CPU time, in 10ms units
	timerBufIO   = 3 // buffered I/O operations
	timerDirIO   = 4 // direct I/O operations
	timerFaults  = 5 // page faults
)

// cpuTimeUnit is JPI$_CPUTIM's unit, 10 milliseconds, in VMS time units
// (100ns).
const cpuTimeUnit = 100_000

// timerSample is what a timer block holds: where the process stood.
type timerSample struct {
	time                      uint64 // VMS time, 100ns units
	cpu, bufIO, dirIO, faults uint32 // CPU time in 10ms units, and counts
}

// timerState is one image's timer data: the static block, nil until
// INIT_TIMER fills it.
type timerState struct {
	static *timerSample
}

// timersKey is the corevms.Environment.ImageData key of the image's
// timerState.
type timersKey struct{}

// timers returns the running image's timer data.
func timers(env *corevms.Environment) *timerState {
	return env.ImageData(timersKey{}, func() any {
		return &timerState{}
	}).(*timerState)
}

// sampleNow is where the process stands now: the system time and its
// $GETJPI figures (JPI$_CPUTIM, JPI$_BUFIO, JPI$_DIRIO, JPI$_PAGEFLTS).
func sampleNow(env *corevms.Environment) timerSample {
	p := env.Process

	return timerSample{
		time:   env.Clock(),
		cpu:    uint32(env.CPUTime(env) / cpuTimeUnit),
		bufIO:  p.BufferedIO,
		dirIO:  p.DirectIO,
		faults: p.PageFaults,
	}
}

// since is how far each figure has moved from s to now.
func (s timerSample) since(now timerSample) timerSample {
	return timerSample{
		time:   now.time - s.time,
		cpu:    now.cpu - s.cpu,
		bufIO:  now.bufIO - s.bufIO,
		dirIO:  now.dirIO - s.dirIO,
		faults: now.faults - s.faults,
	}
}

// storeTimerBlock writes s to the heap timer block at addr.
func storeTimerBlock(env *corevms.Environment, addr uint32, s timerSample) error {
	mem, cpu := env.Memory(), env.CPU()

	for _, f := range []struct {
		offset, v uint32
	}{
		{timerBlockTag, timerTag},
		{timerBlockTime, uint32(s.time)},
		{timerBlockTime + 4, uint32(s.time >> 32)},
		{timerBlockCPU, s.cpu},
		{timerBlockBufIO, s.bufIO},
		{timerBlockDirIO, s.dirIO},
		{timerBlockFaults, s.faults},
	} {
		if err := mem.StoreLongword(cpu, addr+f.offset, f.v); err != nil {
			return writing(err)
		}
	}

	return nil
}

// loadTimerBlock reads the heap timer block at addr.
func loadTimerBlock(env *corevms.Environment, addr uint32) (timerSample, error) {
	mem, cpu := env.Memory(), env.CPU()

	var w [timerBlockSize / 4]uint32

	for i := range w {
		v, err := mem.LoadLongword(cpu, addr+uint32(4*i))
		if err != nil {
			return timerSample{}, err
		}

		w[i] = v
	}

	return timerSample{
		time:   uint64(w[timerBlockTime/4+1])<<32 | uint64(w[timerBlockTime/4]),
		cpu:    w[timerBlockCPU/4],
		bufIO:  w[timerBlockBufIO/4],
		dirIO:  w[timerBlockDirIO/4],
		faults: w[timerBlockFaults/4],
	}, nil
}

// isTimerBlock reports whether a heap timer block is at block: whether
// its first longword is the tag. A block that can't be read (0, say) is
// a memory error, which the routines signal as an access violation.
func isTimerBlock(env *corevms.Environment, block uint32) (bool, error) {
	tag, err := env.Memory().LoadLongword(env.CPU(), block+timerBlockTag)

	return tag == timerTag, err
}

// timerStart reads the block a SHOW_TIMER or STAT_TIMER handle-address
// argument names: the static block when it's omitted (0), else the heap
// block whose address is the longword it points to. ok is false, for
// LIB$_INVARG, when that's not a timer block, or the static block was
// never filled.
func timerStart(env *corevms.Environment, handle uint32) (s timerSample, ok bool, err error) {
	t := timers(env)

	if handle == 0 {
		if t.static == nil {
			return timerSample{}, false, nil
		}

		return *t.static, true, nil
	}

	block, err := env.Memory().LoadLongword(env.CPU(), handle)
	if err != nil {
		return timerSample{}, false, err
	}

	if ok, err := isTimerBlock(env, block); err != nil || !ok {
		return timerSample{}, false, err
	}

	s, err = loadTimerBlock(env, block)

	return s, err == nil, err
}

// libInitTimer is LIB$INIT_TIMER:
//
//	LIB$INIT_TIMER [context.mlu.r]
//
// It records the time, CPU time, I/O counts, and page faults in a timer
// block: the static one when context is omitted; a new heap block when
// the longword context points to is 0, whose address it writes there
// (LIB$_INSVIRMEM with no room); or the heap block that longword names,
// LIB$_INVARG if it isn't one (isTimerBlock).
func libInitTimer(env *corevms.Environment, argv []uint32) (uint32, error) {
	t := timers(env)
	context := arg(argv, 0)

	if context == 0 {
		s := sampleNow(env)
		t.static = &s

		return ssNormal, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	block, err := mem.LoadLongword(cpu, context)
	if err != nil {
		return memoryFault(env, err)
	}

	if block != 0 {
		ok, err := isTimerBlock(env, block)
		if err != nil {
			return memoryFault(env, err)
		}

		if !ok {
			return libInvArg, nil
		}

		if err := storeTimerBlock(env, block, sampleNow(env)); err != nil {
			return memoryFault(env, err)
		}

		return ssNormal, nil
	}

	block, err = env.AllocateVM(timerBlockSize)
	if err != nil {
		return 0, err
	}

	if block == 0 {
		return libInsVirMem, nil
	}

	if err := storeTimerBlock(env, block, sampleNow(env)); err != nil {
		env.FreeVM(block)

		return memoryFault(env, err)
	}

	if err := mem.StoreLongword(cpu, context, block); err != nil {
		env.FreeVM(block)

		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// libFreeTimer is LIB$FREE_TIMER:
//
//	LIB$FREE_TIMER handle-address.mlu.r
//
// It frees the heap timer block whose address is the longword at
// handle-address, and zeroes that longword. LIB$_INVARG if it isn't a
// timer block (isTimerBlock; a handle holding 0 is an access violation,
// as on VMS 7.1), and LIB$_BADBLOADR if the heap no longer has it (the
// program freed it with LIB$FREE_VM). The tag is cleared first, so a
// copy of the handle no longer names a block (unconfirmed).
func libFreeTimer(env *corevms.Environment, argv []uint32) (uint32, error) {
	handle := arg(argv, 0)
	mem, cpu := env.Memory(), env.CPU()

	block, err := mem.LoadLongword(cpu, handle)
	if err != nil {
		return memoryFault(env, err)
	}

	ok, err := isTimerBlock(env, block)
	if err != nil {
		return memoryFault(env, err)
	}

	if !ok {
		return libInvArg, nil
	}

	if err := mem.StoreLongword(cpu, block+timerBlockTag, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, handle, 0); err != nil {
		return memoryFault(env, writing(err))
	}

	if !env.FreeVM(block) {
		return libBadBlockAddress, nil
	}

	return ssNormal, nil
}

// libStatTimer is LIB$STAT_TIMER:
//
//	LIB$STAT_TIMER code.rl.r, value-argument.wz.r [,handle-address.rlu.r]
//
// It writes one statistic since the block's INIT_TIMER: for code 1 the
// elapsed time, a quadword "in system time format": a delta time,
// negative, and at least 100ns long (VMS 7.1 wrote -1 for no time
// passed: testdata/lib55); for 2 to 5 the CPU
// time in 10ms units, buffered I/O, direct I/O, or page faults, a
// longword. LIB$_INVARG for another code, or for a handle that names no
// timer block (timerStart).
func libStatTimer(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	code, err := readArg(env, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if code < timerElapsed || code > timerFaults {
		return libInvArg, nil
	}

	start, ok, err := timerStart(env, arg(argv, 2))
	if err != nil {
		return memoryFault(env, err)
	}

	if !ok {
		return libInvArg, nil
	}

	d := start.since(sampleNow(env))
	value := arg(argv, 1)

	if code == timerElapsed {
		delta := -max(d.time, 1)

		err = mem.StoreLongword(cpu, value, uint32(delta))
		if err == nil {
			err = mem.StoreLongword(cpu, value+4, uint32(delta>>32))
		}
	} else {
		err = mem.StoreLongword(cpu, value, [...]uint32{timerCPU: d.cpu, timerBufIO: d.bufIO,
			timerDirIO: d.dirIO, timerFaults: d.faults}[code])
	}

	if err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// timerFields are SHOW_TIMER's lines for the statistics in d, by code,
// as VMS 7.1 wrote them (testdata/lib55). The whole line (code 0) is
//
//	 ELAPSED:    0 00:00:00.00  CPU: 0:00:00.00  BUFIO: 0  DIRIO: 1  FAULTS: 2
//
// with a blank at its end, and one statistic's is the manual's table's
// form (" ELAPSED TIME =    0 00:00:00.00", " CPU TIME = 0:00:00.00",
// " BUFFERED I/O COUNT = 0", ...). The elapsed time is written as $ASCTIM
// writes a delta time (the days four wide), the CPU time as
// hours:mm:ss.cc. Hundredths are truncated, and larger numbers aren't
// padded (unconfirmed: the run's were single digits).
func timerFields(d timerSample) [timerFaults + 1]string {
	cs := d.time / cpuTimeUnit // the elapsed time in hundredths
	elapsed := fmt.Sprintf("%4d %02d:%02d:%02d.%02d", cs/8_640_000, cs/360_000%24, cs/6_000%60, cs/100%60, cs%100)
	cpu := fmt.Sprintf("%d:%02d:%02d.%02d", d.cpu/360_000, d.cpu/6_000%60, d.cpu/100%60, d.cpu%100)

	return [...]string{
		0: fmt.Sprintf(" ELAPSED: %s  CPU: %s  BUFIO: %d  DIRIO: %d  FAULTS: %d ",
			elapsed, cpu, d.bufIO, d.dirIO, d.faults),
		timerElapsed: " ELAPSED TIME = " + elapsed,
		timerCPU:     " CPU TIME = " + cpu,
		timerBufIO:   fmt.Sprintf(" BUFFERED I/O COUNT = %d", d.bufIO),
		timerDirIO:   fmt.Sprintf(" DIRECT I/O COUNT = %d", d.dirIO),
		timerFaults:  fmt.Sprintf(" PAGE FAULT COUNT = %d", d.faults),
	}
}

// libShowTimer is LIB$SHOW_TIMER:
//
//	LIB$SHOW_TIMER [handle-address.rlu.r] [,code.rl.r]
//	               [,user-action-procedure.flc.rp] [,user-argument-value.rz.v]
//
// It writes a line of the statistics since the block's INIT_TIMER: all
// five with code omitted or 0, else the one code names (1 to 5, as
// LIB$STAT_TIMER's; another is LIB$_INVARG, as is a handle that names no
// timer block). The line goes to SYS$OUTPUT, as LIB$PUT_OUTPUT writes it,
// and SHOW_TIMER returns the write's status; or, with an action routine,
// to that routine, called with a class S descriptor of the line and
// user-argument-value (0 when omitted), whose status SHOW_TIMER returns.
//
// The routine is called as the shim's tail call (corevms.CallRequest's
// Tail): its argument list, descriptor, and text go below SP, in the
// stub's own frame, and it returns to the stub's RET, which returns its
// R0 to SHOW_TIMER's caller and takes them off the stack.
func libShowTimer(env *corevms.Environment, argv []uint32) (uint32, error) {
	code := int32(0)

	if addr := arg(argv, 1); addr != 0 {
		var err error
		if code, err = readArg(env, addr); err != nil {
			return memoryFault(env, err)
		}
	}

	if code < 0 || code > timerFaults {
		return libInvArg, nil
	}

	start, ok, err := timerStart(env, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if !ok {
		return libInvArg, nil
	}

	fields := timerFields(start.since(sampleNow(env)))

	line := fields[code]

	action := arg(argv, 2)
	if action == 0 {
		return env.PutOutput(line)
	}

	return callAction(env, action, line, arg(argv, 3))
}

// callAction calls the action routine at action as the shim's tail call
// (see libShowTimer), with the argument list (2, a class S descriptor of
// line, value), laid out below SP with the descriptor and the text.
func callAction(env *corevms.Environment, action uint32, line string, value uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()

	desc, err := stackString(env, line)
	if err != nil {
		return memoryFault(env, writing(err))
	}

	argList := cpu.GPR(registers.SP) - 12

	for i, v := range []uint32{2, desc, value} {
		if err := mem.StoreLongword(cpu, argList+uint32(4*i), v); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	cpu.SetGPR(registers.SP, argList)

	return 0, &corevms.CallRequest{Routine: action, ArgList: argList, Tail: true}
}
