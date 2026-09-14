package rtl

// Port of librtl_strings.c — CRTL string/character-classification shims.
//
// The decc_isXXX routines all return 1 for true / 0 for false. A real C
// library's ctype.h macros are only contractually "zero or nonzero" (many
// implementations return a nonzero bitmask, not literally 1), but nothing
// in this codebase (or, plausibly, in the small test programs this project
// targets) inspects the exact nonzero value rather than just its truth —
// compiled C code almost always writes `if (isalnum(c))`, never `if
// (isalnum(c) == 1)` — so a clean 0/1 encoding is used throughout rather
// than trying to replicate one specific real libc's internal magic numbers.

func isUpperASCII(ch byte) bool { return ch >= 'A' && ch <= 'Z' }
func isLowerASCII(ch byte) bool { return ch >= 'a' && ch <= 'z' }
func isDigitASCII(ch byte) bool { return ch >= '0' && ch <= '9' }
func isAlphaASCII(ch byte) bool { return isUpperASCII(ch) || isLowerASCII(ch) }
func isAlnumASCII(ch byte) bool { return isAlphaASCII(ch) || isDigitASCII(ch) }
func isCntrlASCII(ch byte) bool { return ch < 0x20 || ch == 0x7F }
func isPrintASCII(ch byte) bool { return ch >= 0x20 && ch < 0x7F }
func isGraphASCII(ch byte) bool { return isPrintASCII(ch) && ch != ' ' }
func isPunctASCII(ch byte) bool { return isGraphASCII(ch) && !isAlnumASCII(ch) }
func isSpaceASCII(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}
func isXDigitASCII(ch byte) bool {
	return isDigitASCII(ch) || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f')
}

func boolToR0(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

func classifyShim(fn func(byte) bool) ShimFunc {
	return func(env *Environment, argv []uint32) (uint32, error) {
		return boolToR0(fn(byte(argv[0] & 0xFF))), nil
	}
}

// shimStrUpcase is STR$UPCASE: upcases a string in place through a VAX
// string descriptor.
func shimStrUpcase(env *Environment, argv []uint32) (uint32, error) {
	addr := argv[0]
	length, err := env.mem.LoadWord(env.cpu, addr)
	if err != nil {
		return 0, err
	}
	daddr, err := env.mem.LoadLongword(env.cpu, addr+4)
	if err != nil {
		return 0, err
	}
	for n := uint16(0); n < length; n++ {
		ch, err := env.mem.LoadByte(env.cpu, daddr+uint32(n))
		if err != nil {
			return 0, err
		}
		if ch >= 'a' && ch <= 'z' {
			if err := env.mem.StoreByte(env.cpu, daddr+uint32(n), ch-32); err != nil {
				return 0, err
			}
		}
	}
	return 1, nil
}

// cAtoi replicates the C library atoi(): skip leading whitespace, an
// optional sign, then digits, stopping at the first non-digit (0 if none
// were found).
func cAtoi(s string) int32 {
	i := 0
	for i < len(s) && isSpaceASCII(s[i]) {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	var v int32
	for i < len(s) && isDigitASCII(s[i]) {
		v = v*10 + int32(s[i]-'0')
		i++
	}
	if neg {
		return -v
	}
	return v
}

// shimDeccAtoi is DECC$ATOI.
func shimDeccAtoi(env *Environment, argv []uint32) (uint32, error) {
	s, err := loadDString(env, argv[0])
	if err != nil {
		return 0, err
	}
	return uint32(cAtoi(s)), nil
}

// cStrcmp replicates strcmp's sign convention (only the sign is meaningful
// to any caller in this codebase, so this doesn't try to reproduce one
// specific libc's exact byte-difference magnitude).
func cStrcmp(a, b string) uint32 {
	switch {
	case a < b:
		return 0xFFFFFFFF // -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// shimDeccStrcmp is DECC$STRCMP.
func shimDeccStrcmp(env *Environment, argv []uint32) (uint32, error) {
	a, err := loadDString(env, argv[0])
	if err != nil {
		return 0, err
	}
	b, err := loadDString(env, argv[1])
	if err != nil {
		return 0, err
	}
	return cStrcmp(a, b), nil
}

// shimDeccStrncmp is DECC$STRNCMP.
func shimDeccStrncmp(env *Environment, argv []uint32) (uint32, error) {
	a, err := loadDString(env, argv[0])
	if err != nil {
		return 0, err
	}
	b, err := loadDString(env, argv[1])
	if err != nil {
		return 0, err
	}
	n := int(argv[2])
	if len(a) > n {
		a = a[:n]
	}
	if len(b) > n {
		b = b[:n]
	}
	return cStrcmp(a, b), nil
}

// shimDeccStrncpy is DECC$STRNCPY: copies min(argv[2], strlen(source)+1)
// bytes from the source string to the destination, including the source's
// own NUL terminator when the copy is short enough to reach it — matching
// decc_strncpy's own "if (alen < len) len = alen + 1" adjustment.
func shimDeccStrncpy(env *Environment, argv []uint32) (uint32, error) {
	s, err := loadDString(env, argv[1])
	if err != nil {
		return 0, err
	}
	length := int(argv[2])
	if len(s) < length {
		length = len(s) + 1
	}
	for i := 0; i < length; i++ {
		var ch byte
		if i < len(s) {
			ch = s[i]
		}
		if err := env.mem.StoreByte(env.cpu, argv[0]+uint32(i), ch); err != nil {
			return 0, err
		}
	}
	return uint32(length), nil
}

func registerStringShims(t *ShimTable) {
	t.Register(2, "STR$UPCASE", shimStrUpcase)
	t.Register(10, "DECC$STRCMP", shimDeccStrcmp)
	t.Register(11, "DECC$STRNCMP", shimDeccStrncmp)
	t.Register(12, "DECC$STRNCPY", shimDeccStrncpy)
	t.Register(13, "DECC$ATOI", shimDeccAtoi)
	t.Register(17, "DECC$ISALNUM", classifyShim(isAlnumASCII))
	t.Register(18, "DECC$ISALPHA", classifyShim(isAlphaASCII))
	t.Register(19, "DECC$ISCNTRL", classifyShim(isCntrlASCII))
	t.Register(20, "DECC$ISDIGIT", classifyShim(isDigitASCII))
	t.Register(21, "DECC$ISGRAPH", classifyShim(isGraphASCII))
	t.Register(22, "DECC$ISLOWER", classifyShim(isLowerASCII))
	t.Register(23, "DECC$ISPRINT", classifyShim(isPrintASCII))
	t.Register(24, "DECC$ISPUNCT", classifyShim(isPunctASCII))
	t.Register(25, "DECC$ISSPACE", classifyShim(isSpaceASCII))
	t.Register(26, "DECC$ISUPPER", classifyShim(isUpperASCII))
	t.Register(27, "DECC$ISXDIGIT", classifyShim(isXDigitASCII))
	t.Register(28, "DECC$ISASCII", func(env *Environment, argv []uint32) (uint32, error) { return 1, nil })
}
