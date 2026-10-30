package mthrtl

import (
	"math/big"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
)

// TestNextSeed: the manual's formula, in integers wider than the seed, against
// nextSeed's uint32 arithmetic, and a sequence worked out apart from any of this.
func TestNextSeed(t *testing.T) {
	seed := uint32(0x12345678)

	for range 100000 {
		want := (uint64(seed)*69069 + 1) % (1 << 32)
		seed = nextSeed(seed)

		if uint64(seed) != want {
			t.Fatalf("seed %#x, want %#x", seed, want)
		}
	}

	seed = 1

	for _, want := range []uint32{0x00010DCE, 0x1C5983F7, 0xC35937CC, 0x2E130A5D, 0xE723057A} {
		seed = nextSeed(seed)
		if seed != want {
			t.Errorf("seed %#x, want %#x", seed, want)
		}
	}
}

// TestSeedValue: each result is its seed's high 24 bits over 2^24, exactly, kept
// by F_floating without rounding, and below 1.0 at the very top. The low 8 bits
// make no difference.
func TestSeedValue(t *testing.T) {
	one := big.NewRat(1, 1)
	seed := uint32(0x9ABCDEF0)

	check := func(seed uint32) {
		v := seedValue(seed)

		want := big.NewRat(int64(seed>>8), 1<<24)
		if v.Rat().Cmp(want) != 0 {
			t.Fatalf("seed %#x: %v, want %v", seed, v.Rat(), want)
		}

		if v.Sign() < 0 || v.Rat().Cmp(one) >= 0 {
			t.Fatalf("seed %#x: %v is out of range", seed, v.Rat())
		}

		rounded, err := vaxfloat.Round(vaxfloat.F, v)
		if err != nil || rounded.Cmp(v) != 0 {
			t.Fatalf("seed %#x: F_floating holds %v (%v), want %v", seed, rounded.Rat(), err, v.Rat())
		}
	}

	for range 100000 {
		seed = nextSeed(seed)
		check(seed)
	}

	for _, edge := range []uint32{0, 0xFF, 0x100, 0x7FFFFFFF, 0x80000000, 0xFFFFFF00, 0xFFFFFFFF} {
		check(edge)
	}
}

// TestRandomUniform: the sequence fills the range evenly. Sixteen bins of 160,000
// values hold about 10,000 each, so a bin off by 3% is more than 3 standard
// deviations out, and the mean is near a half.
func TestRandomUniform(t *testing.T) {
	const samples = 160000

	var (
		bins [16]int
		sum  float64
	)

	seed := uint32(1)

	for range samples {
		seed = nextSeed(seed)
		bins[seed>>28]++

		f, _ := seedValue(seed).Big().Float64()
		sum += f
	}

	for i, n := range bins {
		if n < 9700 || n > 10300 {
			t.Errorf("bin %d has %d values", i, n)
		}
	}

	if mean := sum / samples; mean < 0.49 || mean > 0.51 {
		t.Errorf("the mean is %v", mean)
	}
}

// useRandomSetting sets vax.runtime.random for one test.
func useRandomSetting(t *testing.T, value string) {
	t.Helper()

	old, had := settings.Get(randomSetting), settings.Exists(randomSetting)
	settings.Set(randomSetting, value)

	t.Cleanup(func() {
		if had {
			settings.Set(randomSetting, old)
		} else {
			_ = settings.Delete(randomSetting)
		}
	})
}

// TestRandomSetting calls the routine with the seed set to the same value each time.
// Off, every call has one answer, the formula's. On, the answers differ, and each is
// still the high bits of the seed left in memory, in range.
func TestRandomSetting(t *testing.T) {
	const addr, start = 0x2000, 0x12345678

	call := func(t *testing.T, env *corevms.Environment) (r0, seed uint32) {
		t.Helper()

		cell := seedCell{env, addr}
		cell.store(start)

		r0, err := random(env, []uint32{addr})
		if err != nil {
			t.Fatal(err)
		}

		return r0, cell.load()
	}

	for _, value := range []string{"", "false"} {
		t.Run("off "+value, func(t *testing.T) {
			if value != "" {
				useRandomSetting(t, value)
			}

			env := fixture(t)
			want := nextSeed(start)

			for range 8 {
				r0, seed := call(t, env)

				if seed != want || r0 != registerWords(vaxfloat.F, seedValue(want))[0] {
					t.Fatalf("R0 %#x, seed %#x; want the seed %#x and its value", r0, seed, want)
				}
			}
		})
	}

	t.Run("on", func(t *testing.T) {
		useRandomSetting(t, "true")

		env := fixture(t)
		seen := map[uint32]bool{}

		const calls = 64

		for range calls {
			r0, seed := call(t, env)

			if r0 != registerWords(vaxfloat.F, seedValue(seed))[0] {
				t.Fatalf("R0 %#x isn't the value of the seed left, %#x", r0, seed)
			}

			if seed == nextSeed(start) {
				t.Fatalf("the seed followed the formula")
			}

			seen[seed] = true
		}

		// 64 draws of 32 bits collide about once in two million runs, and one
		// collision doesn't fail this; a generator stuck on few values does.
		if len(seen) < calls-1 {
			t.Errorf("%d different seeds in %d calls", len(seen), calls)
		}
	})
}

// seedCell is the longword a test's seed is in.
type seedCell struct {
	env  *corevms.Environment
	addr uint32
}

func (c seedCell) store(v uint32) {
	if err := c.env.Memory().StoreLongword(c.env.CPU(), c.addr, v); err != nil {
		panic(err)
	}
}

func (c seedCell) load() uint32 {
	v, err := c.env.Memory().LoadLongword(c.env.CPU(), c.addr)
	if err != nil {
		panic(err)
	}

	return v
}
