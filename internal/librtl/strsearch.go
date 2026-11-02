package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
)

// The STR$ routines that compare and search (docs/PHASE-83, subtask 9).

// ordered is -1, 0, or 1 as a longword: a comparison routine's R0.
func ordered(less, more bool) uint32 {
	switch {
	case less:
		return 0xFFFFFFFF
	case more:
		return 1
	}

	return 0
}

// twoSources reads a routine's first two arguments, both input strings.
func twoSources(env *corevms.Environment, argv []uint32) (a, b string, err error) {
	if a, _, err = strSource(env, strArg(env, argv, 0)); err != nil {
		return "", "", err
	}

	b, _, err = strSource(env, strArg(env, argv, 1))

	return a, b, err
}

// compareBlankFilled compares two strings a byte at a time by key, the shorter
// filled with blanks, as CMPC5 with a blank fill does when key is the byte's
// value.
func compareBlankFilled(a, b string, key func(c byte) uint32) uint32 {
	at := func(s string, i int) byte {
		if i < len(s) {
			return s[i]
		}

		return ' '
	}

	for i := range max(len(a), len(b)) {
		if x, y := key(at(a, i)), key(at(b, i)); x != y {
			return ordered(x < y, x > y)
		}
	}

	return 0
}

// strCompare is STR$COMPARE: the shorter string blank filled, bytes unsigned.
func strCompare(env *corevms.Environment, argv []uint32) (uint32, error) {
	a, b, err := twoSources(env, argv)
	if err != nil {
		return strFailure(env, err)
	}

	return compareBlankFilled(a, b, func(c byte) uint32 { return uint32(c) }), nil
}

// strCompareEql is STR$COMPARE_EQL: 0 when the two have one length and the same
// bytes, otherwise 1, whichever is the greater.
func strCompareEql(env *corevms.Environment, argv []uint32) (uint32, error) {
	a, b, err := twoSources(env, argv)
	if err != nil {
		return strFailure(env, err)
	}

	if a == b {
		return 0, nil
	}

	return 1, nil
}

// strCaseBlindCompare is STR$CASE_BLIND_COMPARE: compared in capitals, and past
// the shorter's end not as STR$COMPARE does: what's left of the longer is equal
// if it's all blanks, and otherwise the longer is the greater, even if what's left is
// below a blank (the probe's AB and AB<tab>, which STR$COMPARE orders the other
// way).
func strCaseBlindCompare(env *corevms.Environment, argv []uint32) (uint32, error) {
	a, b, err := twoSources(env, argv)
	if err != nil {
		return strFailure(env, err)
	}

	a, b = upcased(a), upcased(b)
	common := min(len(a), len(b))

	for i := range common {
		if a[i] != b[i] {
			return ordered(a[i] < b[i], a[i] > b[i]), nil
		}
	}

	if strings.Trim(a[common:], " ") == "" && strings.Trim(b[common:], " ") == "" {
		return 0, nil
	}

	return ordered(len(a) < len(b), len(a) > len(b)), nil
}

// multinationalKey is a character's place for STR$COMPARE_MULTI: by its capital
// and, when case matters, a lowercase letter just before its capital (VMS's run:
// abc is less than ABC, and also than ABD). The Multinational table's other
// choices, and the five foreign languages' tables, aren't in the manual or the
// probe: every language has this order, unconfirmed.
func multinationalKey(c byte, blind bool) uint32 {
	capital := upcase[c]
	key := uint32(capital) << 1

	if !blind && c == capital {
		key |= 1
	}

	return key
}

// strCompareMulti is STR$COMPARE_MULTI: first-source-string, second-source-string,
// and by value [flags-value] (bit 0 is case blind) and [foreign-language] (1 to 6,
// 1 when omitted, anything else LIB$_INVARG).
func strCompareMulti(env *corevms.Environment, argv []uint32) (uint32, error) {
	blind := arg(argv, 2)&1 != 0

	if len(argv) > 3 && (argv[3] < 1 || argv[3] > 6) {
		return env.SignalFailure([]uint32{libInvArg}, libInvArg)
	}

	a, b, err := twoSources(env, argv)
	if err != nil {
		return strFailure(env, err)
	}

	return compareBlankFilled(a, b, func(c byte) uint32 { return multinationalKey(c, blind) }), nil
}

// strFind is STR$POSITION's work: where sub first occurs in s at or after start
// (below 1, 1), or 0. An empty sub is found where the search starts.
func strFind(s, sub string, start int32) int {
	start = max(start, 1)
	if int(start)-1 > len(s) {
		return 0
	}

	at := strings.Index(s[start-1:], sub)
	if at < 0 {
		return 0
	}

	return at + int(start)
}

// strPosition is STR$POSITION: source-string, substring, [start-position], by
// reference.
func strPosition(env *corevms.Environment, argv []uint32) (uint32, error) {
	s, sub, err := twoSources(env, argv)
	if err != nil {
		return strFailure(env, err)
	}

	start := int32(1)

	if addr := arg(argv, 2); addr != 0 {
		if start, err = signedAt(env, addr); err != nil {
			return strFailure(env, err)
		}
	}

	return uint32(strFind(s, sub, start)), nil
}

// strPositionR6 is STR$POSITION_R6: R0 the source, R1 the substring, R2 the
// start, by value. It returns the place in R0 and R3; R1 is past the substring's
// text, R2 the source's bytes after the match (MATCHC's R1 and R2), R4 and R5 the
// source's text and length, and R6 the substring's length.
func strPositionR6(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	s, stext, err := strSource(env, regs[registers.R0])
	if err != nil {
		status, err := strFailure(env, err)

		return jsbStatus(regs, status, err)
	}

	sub, subtext, err := strSource(env, regs[registers.R1])
	if err != nil {
		status, err := strFailure(env, err)

		return jsbStatus(regs, status, err)
	}

	found := strFind(s, sub, int32(regs[registers.R2]))
	remaining := 0

	if found != 0 {
		remaining = len(s) - (found - 1 + len(sub))
	}

	regs[registers.R0], regs[registers.R3] = uint32(found), uint32(found)
	regs[registers.R1] = subtext + uint32(len(sub))
	regs[registers.R2] = uint32(remaining)
	regs[registers.R4], regs[registers.R5] = stext, uint32(len(s))
	regs[registers.R6] = uint32(len(sub))

	return nil
}

// strFindFirstInSet is STR$FIND_FIRST_IN_SET: source-string, set-of-characters.
// The place of the first character in the set, or 0.
func strFindFirstInSet(env *corevms.Environment, argv []uint32) (uint32, error) {
	s, set, err := twoSources(env, argv)
	if err != nil {
		return strFailure(env, err)
	}

	for i := range len(s) {
		if strings.IndexByte(set, s[i]) >= 0 {
			return uint32(i + 1), nil
		}
	}

	return 0, nil
}

// strFindFirstNotInSet is STR$FIND_FIRST_NOT_IN_SET: source-string,
// set-of-characters. The place of the first character not in the set, or 0. Two
// answers are VMS's own, from its run, and not what the words suggest: an empty
// source gives 1, and an empty set 0. (Both empty: unconfirmed, here 1.)
func strFindFirstNotInSet(env *corevms.Environment, argv []uint32) (uint32, error) {
	s, set, err := twoSources(env, argv)
	if err != nil {
		return strFailure(env, err)
	}

	if s == "" {
		return 1, nil
	}

	if set == "" {
		return 0, nil
	}

	for i := range len(s) {
		if strings.IndexByte(set, s[i]) < 0 {
			return uint32(i + 1), nil
		}
	}

	return 0, nil
}

// strFindFirstSubstring is STR$FIND_FIRST_SUBSTRING: source-string, index,
// substring-index, then the substrings. Of those found, the one that starts first
// wins, and of two starting together the one named first. index and
// substring-index are longwords given its place and its number from 1, both 0 if
// none is found; R0 is 1 for found and 0 for not.
func strFindFirstSubstring(env *corevms.Environment, argv []uint32) (uint32, error) {
	s, _, err := strSource(env, strArg(env, argv, 0))
	if err != nil {
		return strFailure(env, err)
	}

	best, which := 0, 0

	for i := 3; i < len(argv); i++ {
		sub, _, err := strSource(env, argv[i])
		if err != nil {
			return strFailure(env, err)
		}

		if found := strFind(s, sub, 1); found != 0 && (which == 0 || found < best) {
			best, which = found, i-2
		}
	}

	mem, cpu := env.Memory(), env.CPU()

	if err := mem.StoreLongword(cpu, strArg(env, argv, 1), uint32(best)); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(cpu, strArg(env, argv, 2), uint32(which)); err != nil {
		return memoryFault(env, writing(err))
	}

	if which == 0 {
		return 0, nil
	}

	return 1, nil
}

// wildMatch reports whether candidate matches pattern: * is any characters, %
// exactly one, and no character escapes them. Case matters. It backs up to the
// latest * on a mismatch, moving that * on by a character, which is enough: an
// earlier * could only do worse.
func wildMatch(candidate, pattern string) bool {
	c, p := 0, 0
	starP, starC := -1, 0

	for c < len(candidate) {
		switch {
		case p < len(pattern) && pattern[p] == '*':
			starP, starC = p, c
			p++
		case p < len(pattern) && (pattern[p] == '%' || pattern[p] == candidate[c]):
			p++
			c++
		case starP >= 0:
			starC++
			c, p = starC, starP+1
		default:
			return false
		}
	}

	return strings.Trim(pattern[p:], "*") == ""
}

// strMatchWild is STR$MATCH_WILD: candidate-string, pattern-string. It returns
// STR$_MATCH or STR$_NOMATCH. (The probe's patterns with % went through its
// generator's own use of %, so they don't test it: % is the manual's.)
func strMatchWild(env *corevms.Environment, argv []uint32) (uint32, error) {
	candidate, pattern, err := twoSources(env, argv)
	if err != nil {
		return strFailure(env, err)
	}

	if wildMatch(candidate, pattern) {
		return strMatch, nil
	}

	return strNoMatch, nil
}
