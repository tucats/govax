package mthrtl

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The random number generator is a linear congruential one, the manual's:
//
//	SEED = (69069 * SEED + 1) modulo 2^32
//
// A multiplication of 32-bit numbers keeps its low 32 bits by itself, so Go's
// uint32 arithmetic is the modulo. The caller keeps the seed, so two seeds are two
// independent sequences, and the library holds no state.
const (
	randomMultiplier = 69069
	randomIncrement  = 1
)

// randomFractionBits is how many of the seed's high-order bits make the result.
// They're the generator's best: its low-order bits repeat far sooner.
const randomFractionBits = 24

// randomSetting is the configuration key that makes the generator unpredictable
// (govax's own; HELP CONFIG KEYS). Off, the default, a sequence follows from its
// seed by the manual's formula, so a run can be repeated and compared with VMS's, as
// vax.hardware.clock's default does for time. On, each new seed is 32 bits from the
// host's cryptographic generator, and the old seed plays no part. The rest is as
// before: the seed is read and rewritten, and the result is its high bits, so a
// program that looks at its seed or scales the result can't tell the difference. A
// program that sets a seed again to replay a sequence gets a new sequence: that's
// what the setting asks for.
const randomSetting = "vax.runtime.random"

// hostSeed is a seed from the host's cryptographic generator. crypto/rand's Read
// never returns an error: if the host can't supply random bytes, it ends the program
// rather than hand out predictable ones.
func hostSeed() uint32 {
	var b [4]byte

	_, _ = rand.Read(b[:])

	return binary.LittleEndian.Uint32(b[:])
}

// updatedSeed is what the seed becomes, by the configuration.
func updatedSeed(seed uint32) uint32 {
	if settings.GetBool(randomSetting) {
		return hostSeed()
	}

	return nextSeed(seed)
}

// nextSeed is the seed after seed, by the manual's formula.
func nextSeed(seed uint32) uint32 {
	return seed*randomMultiplier + randomIncrement
}

// seedValue is the number a seed stands for: its high 24 bits as a fraction, in
// the range 0.0 up to but not including 1.0. F_floating has 24 bits of precision,
// so every such fraction is exact and none rounds up to 1.0. (How VMS turns the
// bits into a number is a question for the probe; the manual says only
// "converted".)
func seedValue(seed uint32) vaxfloat.Value {
	high := int64(seed >> (32 - randomFractionBits))
	x := new(big.Float).SetInt64(high)

	return vaxfloat.FromBig(x.SetMantExp(x, -randomFractionBits))
}

// random is MTH$RANDOM: it updates the seed the argument points to, and returns
// the new seed's number in R0. The seed has modify access, so a seed the caller may
// read and not write is an access violation.
func random(env *corevms.Environment, argv []uint32) (uint32, error) {
	addr := arg(argv, 0)

	seed, err := env.Memory().LoadLongword(env.CPU(), addr)
	if err != nil {
		return fail(env, err)
	}

	seed = updatedSeed(seed)

	if err := env.Memory().StoreLongword(env.CPU(), addr, seed); err != nil {
		return fail(env, librtl.Writing(err))
	}

	return registerWords(vaxfloat.F, seedValue(seed))[0], nil
}
