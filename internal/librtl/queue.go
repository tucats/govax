package librtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmsdef"
)

// The queue and interlock routines (docs/PHASE-53, subtask 9): LIB$INSQHI,
// LIB$INSQTI, LIB$REMQHI, LIB$REMQTI, LIB$BBSSI, and LIB$BBCCI make the
// VAX's interlocked instructions callable.
//
// A self-relative queue is a doubly linked ring of quadword-aligned
// entries around a quadword header, each link a displacement from the
// header or entry it's in (a zeroed header is an empty queue). The work
// is the CPU's own (cpu.InsertInterlocked, cpu.RemoveInterlocked), so the
// routines and the instructions follow one set of rules. What the
// routines add is the retry: when the queue's secondary interlock (bit 0
// of the header) is set, the instruction changes nothing, and the routine
// tries again, retry-count times in all (10 when the argument is
// omitted), before returning LIB$_SECINTFAI. With one processor nothing
// but a program sets that bit, so in practice it never fails.
//
// A misaligned header or entry, an entry that is its header, or a
// remque-address that is the header, is the instruction's reserved
// operand fault: signaled as SS$_ROPRAND, the queue unchanged. The manual
// lists SS$_ROPRAND among REMQHI's and REMQTI's returned statuses but
// calls it a fault; govax signals it, for all four (unconfirmed).

var (
	libOneEntQue = vmsdef.Symbols["LIB$_ONEENTQUE"]
	libQueWasEmp = vmsdef.Symbols["LIB$_QUEWASEMP"]
	libSecIntFai = vmsdef.Symbols["LIB$_SECINTFAI"]
)

// defaultRetryCount is the queue routines' retry-count when the call
// omits it.
const defaultRetryCount = 10

// retryCount reads the optional retry-count argument (a longword by
// reference) at argv[i]: the number of attempts in all, 1 meaning no
// retries. A count of 0 still makes one attempt (unconfirmed).
func retryCount(env *corevms.Environment, argv []uint32, i int) (uint32, error) {
	addr := arg(argv, i)
	if addr == 0 {
		return defaultRetryCount, nil
	}

	n, err := env.Memory().LoadLongword(env.CPU(), addr)

	return max(n, 1), err
}

// queueFault is what a queue routine does with an error from the CPU's
// queue code: the reserved operand fault is signaled as SS$_ROPRAND, a
// memory error as SS$_ACCVIO (memoryFault). The queue instructions both
// read and write, so a memory error is marked as a write's.
func queueFault(env *corevms.Environment, err error) (uint32, error) {
	var f *cpu.Fault
	if errors.As(err, &f) && f.Code == cpu.ExcReservedOp {
		return reservedOperand(env)
	}

	return memoryFault(env, writing(err))
}

// libInsqhi is LIB$INSQHI (LIB-248): insert entry at the queue's head.
//
//	LIB$INSQHI entry ,header [,retry-count]
func libInsqhi(env *corevms.Environment, argv []uint32) (uint32, error) {
	return insertQueue(env, argv, false)
}

// libInsqti is LIB$INSQTI (LIB-251): insert entry at the queue's tail.
//
//	LIB$INSQTI entry ,header [,retry-count]
func libInsqti(env *corevms.Environment, argv []uint32) (uint32, error) {
	return insertQueue(env, argv, true)
}

// insertQueue is LIB$INSQHI and LIB$INSQTI: SS$_NORMAL when the queue now
// has more than one entry, LIB$_ONEENTQUE when entry is its only one, and
// LIB$_SECINTFAI when every attempt found the interlock set.
func insertQueue(env *corevms.Environment, argv []uint32, tail bool) (uint32, error) {
	tries, err := retryCount(env, argv, 2)
	if err != nil {
		return memoryFault(env, err)
	}

	for range tries {
		r, err := cpu.InsertInterlocked(env.CPU(), env.Memory(), arg(argv, 0), arg(argv, 1), tail)
		if err != nil {
			return queueFault(env, err)
		}

		switch {
		case r.C:
			continue
		case r.Z:
			return libOneEntQue, nil
		default:
			return ssNormal, nil
		}
	}

	return libSecIntFai, nil
}

// libRemqhi is LIB$REMQHI (LIB-315): remove the queue's first entry.
//
//	LIB$REMQHI header ,remque-address [,retry-count]
func libRemqhi(env *corevms.Environment, argv []uint32) (uint32, error) {
	return removeQueue(env, argv, false)
}

// libRemqti is LIB$REMQTI (LIB-317): remove the queue's last entry.
//
//	LIB$REMQTI header ,remque-address [,retry-count]
func libRemqti(env *corevms.Environment, argv []uint32) (uint32, error) {
	return removeQueue(env, argv, true)
}

// removeQueue is LIB$REMQHI and LIB$REMQTI. remque-address gets the entry
// removed, or the header's address when the queue was empty. The status
// is SS$_NORMAL when entries remain, LIB$_ONEENTQUE when the queue is now
// empty, LIB$_QUEWASEMP when it was empty already, and LIB$_SECINTFAI
// (remque-address unwritten) when every attempt found the interlock set.
func removeQueue(env *corevms.Environment, argv []uint32, tail bool) (uint32, error) {
	tries, err := retryCount(env, argv, 2)
	if err != nil {
		return memoryFault(env, err)
	}

	result := arg(argv, 1)

	for range tries {
		r, err := cpu.RemoveInterlocked(env.CPU(), env.Memory(), arg(argv, 0), result, tail)
		if err != nil {
			return queueFault(env, err)
		}

		if r.C {
			continue
		}

		if err := env.Memory().StoreLongword(env.CPU(), result, r.Removed); err != nil {
			return memoryFault(env, writing(err))
		}

		switch {
		case r.V:
			return libQueWasEmp, nil
		case r.Z:
			return libOneEntQue, nil
		default:
			return ssNormal, nil
		}
	}

	return libSecIntFai, nil
}

// libBbssi is LIB$BBSSI (LIB-21): set the bit at position from
// bit-zero-address, returning its state before, 1 or 0. The position is
// any signed longword, as for the bit field routines; one processor has
// no interlock to model.
//
//	LIB$BBSSI position ,bit-zero-address
func libBbssi(env *corevms.Environment, argv []uint32) (uint32, error) {
	return testAndChange(env, argv, 1)
}

// libBbcci is LIB$BBCCI (LIB-19): LIB$BBSSI, clearing the bit.
//
//	LIB$BBCCI position ,bit-zero-address
func libBbcci(env *corevms.Environment, argv []uint32) (uint32, error) {
	return testAndChange(env, argv, 0)
}

// testAndChange is LIB$BBSSI and LIB$BBCCI: the bit's old state is the
// value, and the bit becomes bit.
func testAndChange(env *corevms.Environment, argv []uint32, bit uint32) (uint32, error) {
	mem, c := env.Memory(), env.CPU()

	p, err := mem.LoadLongword(c, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	position, base := int32(p), arg(argv, 1)

	// The bit is modified, so a failure to reach it is a write's.
	old, err := cpu.MemoryField(c, mem, position, 1, base, false)
	if err != nil {
		return memoryFault(env, writing(err))
	}

	if err := cpu.SetMemoryField(c, mem, position, 1, base, bit); err != nil {
		return memoryFault(env, writing(err))
	}

	return old, nil
}
