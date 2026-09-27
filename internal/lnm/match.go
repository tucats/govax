package lnm

// Match reports whether name matches pattern using VMS wildcards: "*"
// matches any run of characters (including none) and "%" matches exactly
// one. Any other character must match exactly; case is significant, since
// DCL has already upper-cased an unquoted name. SHOW LOGICAL uses this to
// select names.
func Match(pattern, name string) bool {
	p, n := 0, 0

	// star and starN remember the most recent "*" and the name position
	// it was tried against, so a mismatch can retry with the "*"
	// absorbing one more character.
	star, starN := -1, 0

	for n < len(name) {
		switch {
		case p < len(pattern) && (pattern[p] == '%' || pattern[p] == name[n]):
			p++
			n++

		case p < len(pattern) && pattern[p] == '*':
			star, starN = p, n
			p++

		case star >= 0:
			starN++
			p, n = star+1, starN

		default:
			return false
		}
	}

	for p < len(pattern) && pattern[p] == '*' {
		p++
	}

	return p == len(pattern)
}

// HasWildcards reports whether pattern contains "*" or "%".
func HasWildcards(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '*' || pattern[i] == '%' {
			return true
		}
	}

	return false
}
