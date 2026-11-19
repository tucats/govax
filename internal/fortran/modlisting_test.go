package fortran_test

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/listing"
	"github.com/tucats/govax/internal/fortran/sema"
)

// The listings of modules and their users (docs/planning/PHASE-106 - Fortran 90
// modules.md, subtask 6, finding 13): each use-associated name's origin in the
// storage map and the cross reference, a row for each module used, only the names a
// unit mentions, a module procedure's path beside its hashed global name, and the
// /DEBUG table's module variables.

// listingModules are two modules, the second passing on some of the first's
// entities, one of them under a long name with a long procedure.
const listingModules = `module m
  integer, parameter :: pub = 1
  integer, parameter, private :: priv = 2
  integer :: shared, spare
end module m
module n
  use m, only: pub, shared
  integer, parameter :: twice = 2 * pub
end module n
module very_long_module_name_here
  integer :: counter_of_things = 0
contains
  subroutine a_long_procedure_name_too(k)
    integer :: k
    counter_of_things = counter_of_things + k
  end subroutine a_long_procedure_name_too
end module very_long_module_name_here`

// listingUser uses the modules every way: a whole module, a rename of an entity
// one module has from another, an ONLY list, and a renamed module procedure.
const listingUser = `program p
  use n, only: twice, renamed => shared
  use m, only: pub
  use very_long_module_name_here, only: bump => a_long_procedure_name_too
  call bump(twice)
  print *, renamed
end program p
subroutine s
  use m
  spare = 1
end subroutine s`

// withListing asks for a listing with a cross reference.
func withListing(o *fortran.Options) {
	o.List, o.CrossReference, o.Show = true, true, listing.DefaultShow()
}

// squeezed is a line with its runs of blanks made one, so that a test doesn't
// depend on the columns' widths.
func squeezed(line string) string { return strings.Join(strings.Fields(line), " ") }

// listingHas reports each of want that no line of the listing is.
func listingHas(t *testing.T, listing []string, want ...string) {
	t.Helper()

	var lines []string
	for _, l := range listing {
		lines = append(lines, squeezed(l))
	}

	for _, w := range want {
		if !slices.Contains(lines, squeezed(w)) {
			t.Errorf("no line %q in the listing:\n%s", w, strings.Join(listing, "\n"))
		}
	}
}

// listingLacks reports each line of the listing that has any of the texts.
func listingLacks(t *testing.T, listing []string, texts ...string) {
	t.Helper()

	for _, l := range listing {
		for _, text := range texts {
			if strings.Contains(squeezed(l), text) {
				t.Errorf("line %q has %q", l, text)
			}
		}
	}
}

// userListing is what the listing says of the user's units, the same whether
// they are compiled with their modules or from the modules' files, its source
// starting on line at+1. In the cross references, {n} is line n of listingUser.
func userListing(at int) []string {
	lines := []string{
		"Storage map for P, the main program",
		"  RENAMED  INTEGER  $MOD$M+0  P.RENAMED  from N as SHARED, defined in M",
		"  " + sema.LongName("VERY_LONG_MODULE_NAME_HERE.A_LONG_PROCEDURE_NAME_TOO") +
			" is VERY_LONG_MODULE_NAME_HERE.A_LONG_PROCEDURE_NAME_TOO, BUMP here",
		"Cross reference for P (D defined, A assigned, a blank is used)",
		"  RENAMED  INTEGER from N as SHARED, defined in M  {2}D  {6}",
		"  PUB  constant from M  {3}D",
		"  TWICE  constant from N  {2}D  {5}",
		"  BUMP  subroutine from VERY_LONG_MODULE_NAME_HERE as A_LONG_PROCEDURE_NAME_TOO  {4}D  {5}",
		"  M  module  {3}",
		"  N  module  {2}",
		"  VERY_LONG_MODULE_NAME_HERE  module  {4}",
		// S uses all of M, and mentions only SPARE.
		"Storage map for S, a subroutine",
		"  SPARE  INTEGER  $MOD$M+4  S.SPARE  from M",
		"Cross reference for S (D defined, A assigned, a blank is used)",
		"  SPARE  INTEGER from M  {9}D  {10}A",
		"  M  module  {9}",
	}

	number := regexp.MustCompile(`\{(\d+)\}`)

	for i, l := range lines {
		lines[i] = number.ReplaceAllStringFunc(l, func(m string) string {
			n, _ := strconv.Atoi(m[1 : len(m)-1])

			return strconv.Itoa(n + at)
		})
	}

	return lines
}

func TestModuleListings(t *testing.T) {
	r := compileFree("P", listingModules+"\n"+listingUser, nil, withListing)
	if r.Errors > 0 {
		t.Fatal(messageList(r))
	}

	long := sema.LongName("VERY_LONG_MODULE_NAME_HERE.A_LONG_PROCEDURE_NAME_TOO")

	listingHas(t, r.Listing,
		// A module's own names have no origin; N's names from M have.
		"Storage map for M, a module",
		"  SHARED  INTEGER  $MOD$M+0  M.SHARED",
		"  SPARE  INTEGER  $MOD$M+4  M.SPARE",
		"Storage map for N, a module",
		"  SHARED  INTEGER  $MOD$M+0  N.SHARED  from M",
		"  SHARED  INTEGER from M  7D",
		"  PUB  constant from M  7D  8",
		"  M  module  7",
		// A module procedure is M.P in the listing, its global name beside it.
		"Storage map for VERY_LONG_MODULE_NAME_HERE.A_LONG_PROCEDURE_NAME_TOO, a subroutine",
		"  "+long+" is VERY_LONG_MODULE_NAME_HERE.A_LONG_PROCEDURE_NAME_TOO, its global name",
		"Cross reference for VERY_LONG_MODULE_NAME_HERE.A_LONG_PROCEDURE_NAME_TOO (D defined, A assigned, a blank is used)",
	)
	listingHas(t, r.Listing, userListing(strings.Count(listingModules, "\n")+1)...)

	// What a unit never mentions of a whole module isn't in its map or cross
	// reference: S has none of M's SHARED.
	listingLacks(t, r.Listing, "S.SHARED", "Cross reference for "+long)

	// The user apart from its modules, through their files, says the same.
	found := moduleFiles{}
	for _, name := range []string{"M", "N", "VERY_LONG_MODULE_NAME_HERE"} {
		found[name] = moduleText(t, r, name)
	}

	apart := compileFree("P", listingUser, found, withListing)
	if apart.Errors > 0 {
		t.Fatal(messageList(apart))
	}

	listingHas(t, apart.Listing, userListing(0)...)
}

// TestModuleUnusedWarnings is /WARNINGS=UNUSED beside modules: a module's public
// names are its users', its private ones its own; a name a whole module's USE gave
// is the module's, one an ONLY list or rename named the unit's.
func TestModuleUnusedWarnings(t *testing.T) {
	r := compileFree("P", listingModules+"\n"+listingUser+"\nsubroutine q\n  use m, only: spare\nend subroutine q", nil,
		func(o *fortran.Options) { o.Warnings.Unused = true })

	want := []string{
		"3:34 UNUSEDNAME PRIV is declared but never used",
		"20:16 UNUSEDNAME PUB is declared but never used",
		"30:16 UNUSEDNAME SPARE is declared but never used",
	}

	if got := messageList(r); !slices.Equal(got, want) {
		t.Errorf("messages\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestModuleDebugTable is the /DEBUG table of a module, which has its variables
// and no entry point, and of a user, which has the module's variables under its own
// names.
func TestModuleDebugTable(t *testing.T) {
	r := compileFree("P", listingModules+"\n"+listingUser, nil, func(o *fortran.Options) { o.Debug.Symbols = true })
	if r.Errors > 0 {
		t.Fatal(messageList(r))
	}

	macro := strings.Join(r.MACRO, "\n")

	for _, want := range []string{
		";DBG$ V M SHARED I 4 S:M.SHARED - -",
		";DBG$ V M SPARE I 4 S:M.SPARE - -",
		";DBG$ V P RENAMED I 4 S:P.RENAMED - -",
		";DBG$ E P P",
	} {
		if !strings.Contains(macro, want+"\n") {
			t.Errorf("no %q in\n%s", want, macro)
		}
	}

	if strings.Contains(macro, ";DBG$ E M ") || strings.Contains(macro, ";DBG$ E N ") {
		t.Errorf("an entry point for a module in\n%s", macro)
	}
}
