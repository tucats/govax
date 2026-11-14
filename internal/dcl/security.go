package dcl

import (
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// The grammar types $PROTECTION and $UIC (docs/planning/PHASE-93 - file
// protection.md, Decision 9, and A.4's "Forms and their errors").
//
// VMS splits the work of reading a protection code or a UIC between DCL
// and the command that takes it, and the messages show the split. DCL
// knows a value's shape: a protection code's category names and whether
// any value is there, a UIC's brackets and list. Those errors are DCL's
// own, %DCL-W-IVKEYW, %DCL-W-VALREQ, %DCL-W-DIRECT, %DCL-W-NOLIST, and the
// command never runs. What the value means is the command's: a
// protection code's access letters, a UIC's numbers and names (looked up
// in the rights database), each refused with the command's own message,
// %SET-F-SYNTAX or %CREATE-F-SYNTAX. So these types check only the shape,
// and hand the command the text, uppercased and without blanks.
//
// A $PROTECTION value is always a list, one element per category, as typed
// ("S:RWED", "WORLD", "G=RE"); read it with Result.List. A $UIC value is
// one string, "[200,1]", "[SMITH]", or "SMITH"; read it with
// Result.String.

// protectionCategories are the categories a protection code may name, in
// mask order; any leading part of one names it.
var protectionCategories = [...]string{"SYSTEM", "OWNER", "GROUP", "WORLD"}

// isSecurityType reports whether typ is $PROTECTION, $UIC, or $ACL, the
// types this file reads.
func isSecurityType(typ ValueType) bool {
	return typ == TypeProtection || typ == TypeUIC || typ == TypeACL
}

// readSecurityValue reads a $PROTECTION or $UIC value from the front of s,
// returning its elements (one for a UIC) and what follows it.
func readSecurityValue(typ ValueType, s string) ([]Value, string, error) {
	switch typ {
	case TypeProtection:
		return readProtection(s)
	case TypeACL:
		v, rest, err := readACL(s)
		if err != nil {
			return nil, "", err
		}

		return []Value{v}, rest, nil
	}

	v, rest, err := readUIC(s)
	if err != nil {
		return nil, "", err
	}

	return []Value{v}, rest, nil
}

// readProtection reads a protection code: a list in parentheses, which may
// have blanks in it ("(S: RWED, W)"), or a single category without them
// ("S:R"). Each category's name must be SYSTEM, OWNER, GROUP, or WORLD or
// a leading part of one, else %DCL-W-IVKEYW naming it; an empty code or an
// empty element, "()" or "(S:RWED,)", is %DCL-W-VALREQ, with no segment,
// as VMS 7.1 showed them. A "(" with no ")" is %DCL-W-NOPAREN.
func readProtection(s string) ([]Value, string, error) {
	text, rest, err := readParenthesized(s)
	if err != nil {
		return nil, "", err
	}

	text = strings.TrimSuffix(strings.TrimPrefix(text, "("), ")")
	if strings.TrimSpace(text) == "" {
		return nil, "", vmserrors.NewSegment(vmserrors.CLI_VALREQ, "")
	}

	var vals []Value

	for _, item := range strings.Split(text, ",") {
		item = strings.ToUpper(strings.Join(strings.Fields(item), ""))
		if item == "" {
			return nil, "", vmserrors.NewSegment(vmserrors.CLI_VALREQ, "")
		}

		name, _, _ := strings.Cut(strings.Replace(item, "=", ":", 1), ":")
		if !isProtectionCategory(name) {
			return nil, "", vmserrors.NewSegment(vmserrors.CLI_IVKEYW, name)
		}

		vals = append(vals, Value{IsString: true, Str: item})
	}

	return vals, rest, nil
}

// isProtectionCategory reports whether name is a category's name or a
// leading part of one.
func isProtectionCategory(name string) bool {
	if name == "" {
		return false
	}

	for _, full := range protectionCategories {
		if strings.HasPrefix(full, name) {
			return true
		}
	}

	return false
}

// readParenthesized reads a value that is either a parenthesized list,
// running to its matching ")" whatever is inside, or a plain token,
// running to a blank or "/". The value is returned with its parentheses.
// A "(" never closed is %DCL-W-NOPAREN, its segment the rest of the
// line from the "(" and a blank after it, as VMS 7.1 showed it.
func readParenthesized(s string) (value, rest string, err error) {
	s = strings.TrimLeft(s, " \t")
	if !strings.HasPrefix(s, "(") {
		return readBareValue(s)
	}

	depth := 0

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return s[:i+1], s[i+1:], nil
			}
		}
	}

	return "", "", vmserrors.NewSegment(vmserrors.CLI_NOPAREN, strings.TrimRight(s, " \t")+" ")
}

// readBareValue reads a token up to a blank or "/", the end of a value
// that isn't quoted or parenthesized.
func readBareValue(s string) (value, rest string, err error) {
	if i := strings.IndexAny(s, " \t/"); i >= 0 {
		return s[:i], s[i:], nil
	}

	return s, "", nil
}

// readUIC reads a UIC: "[g,m]" or "<g,m>", "[name]", or a bare name. In
// brackets there may be one element or two, and the bracket must close
// before the value ends; otherwise it is %DCL-W-DIRECT, its segment the
// value as far as DCL read it ("[1,4" of "[1,4,5]", "[200,1" of an
// unclosed one). A bare value can't be a list: "1,4" is %DCL-W-NOLIST. A
// value with no text is %DCL-W-VALREQ. Whether the numbers are in range
// and the names exist is the command's to say.
func readUIC(s string) (Value, string, error) {
	s = strings.TrimLeft(s, " \t")

	if s == "" || s[0] == '/' {
		return Value{}, "", vmserrors.NewSegment(vmserrors.CLI_VALREQ, "")
	}

	closer := map[byte]byte{'[': ']', '<': '>'}[s[0]]
	if closer == 0 {
		token, rest, _ := readBareValue(s)
		if strings.Contains(token, ",") {
			return Value{}, "", vmserrors.NewSegment(vmserrors.CLI_NOLIST, strings.ToUpper(token))
		}

		return Value{IsString: true, Str: strings.ToUpper(token)}, rest, nil
	}

	commas := 0

	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case c == closer:
			token, rest := s[:i+1], s[i+1:]
			if rest != "" && !strings.ContainsRune(" \t/", rune(rest[0])) {
				break
			}

			return Value{IsString: true, Str: strings.ToUpper(token)}, rest, nil
		case c == ',':
			if commas++; commas == 2 {
				return Value{}, "", vmserrors.NewSegment(vmserrors.CLI_DIRECT, strings.ToUpper(s[:i]))
			}

			continue
		case c != ' ' && c != '\t' && c != '/':
			continue
		}

		return Value{}, "", vmserrors.NewSegment(vmserrors.CLI_DIRECT, strings.ToUpper(s[:i]))
	}

	return Value{}, "", vmserrors.NewSegment(vmserrors.CLI_DIRECT, strings.ToUpper(s))
}

// readACL reads an $ACL value: one entry, "(IDENTIFIER=X,ACCESS=READ)",
// or a list of them, "((...),(...))", running to its matching ")"
// whatever is inside, uppercased as DCL uppercases what isn't quoted. What
// the entries say is the command's to read ($PARSE_ACL's grammar), so a
// bad keyword is its message; only a "(" never closed is DCL's,
// %DCL-W-NOPAREN, its segment the rest of the line, as VMS 7.1 showed it.
// A value with no parentheses is taken as it stands, for the command to
// refuse.
func readACL(s string) (Value, string, error) {
	text, rest, err := readParenthesized(s)
	if err != nil {
		return Value{}, "", err
	}

	return Value{IsString: true, Str: strings.ToUpper(text)}, rest, nil
}
