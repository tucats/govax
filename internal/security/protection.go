package security

import (
	"errors"
	"fmt"
	"strings"
)

// Protection codes as text (docs/planning/PHASE-93 - file protection.md,
// A.4, "Forms and their errors" and "Display").
//
// A protection code is written as a list of categories, each with the
// letters of the access it gives:
//
//	(SYSTEM:RWED, OWNER:RWED, GROUP:RE, WORLD)
//
// A category's name may be shortened to its first letter (or any
// leading part), the separator may be ":" or "=", case doesn't matter,
// and letters may repeat (S:RRR). A category with no letters gives no
// access. A category left out of the list keeps what it had: a code is
// always merged into a base, the code it changes.

// categoryNames are the categories' names, in mask order.
var categoryNames = [...]string{"SYSTEM", "OWNER", "GROUP", "WORLD"}

// ErrUnknownCategory is the error ParseProtection gives for an item whose
// category isn't SYSTEM, OWNER, GROUP, or WORLD: DCL refuses it with
// %DCL-W-IVKEYW, naming Name.
type ErrUnknownCategory struct{ Name string }

func (e ErrUnknownCategory) Error() string {
	return fmt.Sprintf("security: %q is not SYSTEM, OWNER, GROUP, or WORLD", e.Name)
}

// ErrNoCategories is ParseProtection's error for a code with no
// categories at all, "()" or "": DCL's %DCL-W-VALREQ.
var ErrNoCategories = errors.New("security: no categories given")

// ErrRepeatedCategory is ParseProtection's error for a code naming a
// category twice, (S:R,S:W): SET's %SET-F-SYNTAX with -LIB-F-INVARG.
var ErrRepeatedCategory = errors.New("security: a category given twice")

// ErrBadAccess is the error ParseProtection gives for an item whose
// letters aren't R, W, E, and D: SET's %SET-F-SYNTAX with
// -LIB-F-SYNTAXERR, naming Item.
type ErrBadAccess struct{ Item string }

func (e ErrBadAccess) Error() string {
	return fmt.Sprintf("security: %q has a letter other than R, W, E, or D", e.Item)
}

// ParseProtection reads a protection code's text, with or without its
// parentheses, and returns base with each category the text names set to
// what the text gives it.
func ParseProtection(text string, base Protection) (Protection, error) {
	s := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(text), " ", ""))
	s = strings.TrimSuffix(strings.TrimPrefix(s, "("), ")")

	if s == "" {
		return 0, ErrNoCategories
	}

	result := base

	var seen [len(categoryNames)]bool

	for _, item := range strings.Split(s, ",") {
		name, letters, _ := strings.Cut(strings.Replace(item, "=", ":", 1), ":")

		c, ok := categoryNamed(name)
		if !ok {
			return 0, ErrUnknownCategory{Name: name}
		}

		if seen[c] {
			return 0, ErrRepeatedCategory
		}

		seen[c] = true

		access, ok := accessFromLetters(letters)
		if !ok {
			return 0, ErrBadAccess{Item: item}
		}

		result = result.With(c, access)
	}

	return result, nil
}

// categoryNamed returns the category whose name starts with name.
func categoryNamed(name string) (Category, bool) {
	if name == "" {
		return 0, false
	}

	for c, full := range categoryNames {
		if strings.HasPrefix(full, name) {
			return Category(c), true
		}
	}

	return 0, false
}

// accessFromLetters reads a category's letters, R, W, E, and D, in any
// order and repeated.
func accessFromLetters(letters string) (Access, bool) {
	var a Access

	for _, l := range letters {
		switch l {
		case 'R':
			a |= Read
		case 'W':
			a |= Write
		case 'E':
			a |= Execute
		case 'D':
			a |= Delete
		default:
			return 0, false
		}
	}

	return a, true
}

// Text writes p as DCL's SHOW PROTECTION and F$ENVIRONMENT("PROTECTION")
// write a process's default protection: "SYSTEM=RWED, OWNER=RWED,
// GROUP=RE, WORLD=NO ACCESS". A category with no access is followed by
// "=" and none, or is its name alone when none is "" (F$ENVIRONMENT's
// "WORLD").
func (p Protection) Text(none string) string {
	parts := make([]string, len(categoryNames))

	for c, name := range categoryNames {
		letters := p.Allows(Category(c)).String()

		switch {
		case letters != "":
			parts[c] = name + "=" + letters
		case none != "":
			parts[c] = name + "=" + none
		default:
			parts[c] = name
		}
	}

	return strings.Join(parts, ", ")
}
