package rtl

// Port of librtl_utils.c's string/buffer marshaling helpers, used throughout
// this package's shims and services wherever the C source calls
// store_string/load_string/load_dstring/str_get/str_put.

// maxCStringLen mirrors load_dstring's own hardcoded scan bound (65535) for
// a string with no caller-supplied length limit.
const maxCStringLen = 65535

// storeString writes s to addr, stopping early at a literal NUL byte within
// s (matching store_string's own "if (ch==0) break") or after maxLen bytes,
// whichever comes first. maxLen <= 0 means "no limit beyond s itself" —
// store_string's own len==0 case (defaulting to 65535) never actually
// matters for any string this port constructs, so it isn't replicated
// exactly; every caller here passes an explicit maxLen.
func storeString(env *Environment, s string, addr uint32, maxLen int) error {
	if maxLen <= 0 || maxLen > len(s) {
		maxLen = len(s)
	}
	for i := 0; i < maxLen; i++ {
		ch := s[i]
		if err := env.mem.StoreByte(env.cpu, addr+uint32(i), ch); err != nil {
			return err
		}
		if ch == 0 {
			break
		}
	}
	return nil
}

// loadString reads a NUL-terminated string from addr, stopping at the first
// NUL byte or after maxLen bytes, matching load_string (bounded) and
// load_dstring (maxCStringLen, effectively unbounded).
func loadString(env *Environment, addr uint32, maxLen int) (string, error) {
	buf := make([]byte, 0, maxLen)
	for n := 0; n < maxLen; n++ {
		ch, err := env.mem.LoadByte(env.cpu, addr+uint32(n))
		if err != nil {
			return "", err
		}
		if ch == 0 {
			break
		}
		buf = append(buf, ch)
	}
	return string(buf), nil
}

// loadDString is load_dstring: a NUL-terminated string with no caller-
// supplied length limit.
func loadDString(env *Environment, addr uint32) (string, error) {
	return loadString(env, addr, maxCStringLen)
}

// strGet reads a VAX string descriptor at addr (a 16-bit length at +0, the
// string's own address at +4 — descriptor fields +2:+3, the type/class
// bytes, are never inspected by any caller in the C source either) into a
// Go string. ok reports whether the descriptor's length fit within maxLen;
// false replicates str_get's own "*retlen = -1" case (the descriptor is
// larger than the caller's buffer, nothing copied) as a normal outcome
// rather than an error — distinct from a genuine memory-access failure.
func strGet(env *Environment, addr uint32, maxLen int) (s string, ok bool, err error) {
	dlen, err := env.mem.LoadWord(env.cpu, addr)
	if err != nil {
		return "", false, err
	}
	if int(dlen) > maxLen {
		return "", false, nil
	}
	daddr, err := env.mem.LoadLongword(env.cpu, addr+4)
	if err != nil {
		return "", false, err
	}
	buf := make([]byte, dlen)
	for i := range buf {
		ch, err := env.mem.LoadByte(env.cpu, daddr+uint32(i))
		if err != nil {
			return "", false, err
		}
		buf[i] = ch
	}
	return string(buf), true, nil
}

// strPut is str_put: writes s into the string area a VAX string descriptor
// at addr points to (length at +0, address at +4), truncating or space-
// padding to the descriptor's own declared length exactly as str_put's
// "if (n > len) ch = ' '; else ch = buff[n]" loop does for a NUL-terminated
// source buffer — s's own implicit terminator (one byte past its content)
// counts as real data at n == len(s), matching str_put's "one past len is
// still buff[len], the C string's own NUL" behavior for a source that is
// (as every caller here provides) a true NUL-terminated C string, not an
// arbitrary length-prefixed buffer.
func strPut(env *Environment, addr uint32, s string) error {
	dlen, err := env.mem.LoadWord(env.cpu, addr)
	if err != nil {
		return err
	}
	daddr, err := env.mem.LoadLongword(env.cpu, addr+4)
	if err != nil {
		return err
	}
	for n := 0; n < int(dlen); n++ {
		var ch byte
		switch {
		case n < len(s):
			ch = s[n]
		case n == len(s):
			ch = 0
		default:
			ch = ' '
		}
		if err := env.mem.StoreByte(env.cpu, daddr+uint32(n), ch); err != nil {
			return err
		}
	}
	return nil
}
