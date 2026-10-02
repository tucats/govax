package librtl

import "github.com/tucats/govax/internal/rtl"

// strUpcase is STR$UPCASE (ported from eVAX's librtl_strings.c), as eVAX
// has it: it upcases, in place, the string the descriptor at its first
// argument describes, and returns 1.
func strUpcase(env *rtl.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	addr := arg(argv, 0)

	length, err := mem.LoadWord(cpu, addr)
	if err != nil {
		return 0, err
	}

	daddr, err := mem.LoadLongword(cpu, addr+4)
	if err != nil {
		return 0, err
	}

	for n := uint16(0); n < length; n++ {
		ch, err := mem.LoadByte(cpu, daddr+uint32(n))
		if err != nil {
			return 0, err
		}

		if ch >= 'a' && ch <= 'z' {
			if err := mem.StoreByte(cpu, daddr+uint32(n), ch-32); err != nil {
				return 0, err
			}
		}
	}

	return 1, nil
}
