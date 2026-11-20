package fortran_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/diag"
)

// warned is what a free-form program compiled with the /WARNINGS keywords says, each
// message "line:column IDENT text | the line quoted".
func warned(t *testing.T, keywords []string, lines ...string) []string {
	t.Helper()

	w, err := diag.Warnings{}.With(keywords)
	if err != nil {
		t.Fatal(err)
	}

	r := compileFree("P", strings.Join(lines, "\n"), nil, func(o *fortran.Options) { o.Warnings = w })

	said := []string{}
	for _, m := range r.Messages {
		said = append(said, fmt.Sprintf("%d:%d %s %s | %s", m.Pos.Line, m.Pos.Column, m.Ident, m.Text, strings.TrimSpace(m.Pos.Text)))
	}

	return said
}

func sameWarnings(t *testing.T, name string, got, want []string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Errorf("%s: got\n  %s\nwant\n  %s", name, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestWarningsIntentOutPosition: INTENTOUTREF points at the use and quotes its line,
// not the SUBROUTINE statement's (Phase 110, finding 5).
func TestWarningsIntentOutPosition(t *testing.T) {
	got := warned(t, []string{"GENERAL"},
		"subroutine s(a)",
		"  real, intent(out) :: a",
		"  print *, a",
		"  a = 1",
		"end subroutine s")

	sameWarnings(t, "INTENTOUTREF", got, []string{
		"3:12 INTENTOUTREF A is an INTENT(OUT) dummy argument, used here before any statement gives it a value | print *, a",
	})
}

// TestWarningsModulePrivate: a module's private variables, by attribute, by a PRIVATE
// statement's list, and by PRIVATE alone, are reported if nothing uses them; a public
// one isn't, nor one a module procedure uses. Before Phase 110 the module's block of
// its variables counted as COMMON, and hid them all.
func TestWarningsModulePrivate(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"by attribute", []string{"module m", "  integer, private :: a", "  integer :: b", "end module m"},
			[]string{"2:23 UNUSEDNAME A is declared but never used | integer, private :: a"}},
		{"by a list", []string{"module m", "  integer :: a, b", "  private :: a", "end module m"},
			[]string{"2:14 UNUSEDNAME A is declared but never used | integer :: a, b"}},
		{"by default", []string{"module m", "  private", "  integer :: a", "  integer, public :: b", "end module m"},
			[]string{"3:14 UNUSEDNAME A is declared but never used | integer :: a"}},
		{"used by a procedure", []string{"module m", "  private", "  public :: s", "  integer :: a, c", "contains",
			"  subroutine s", "    c = 1", "  end subroutine s", "end module m"},
			[]string{"4:14 UNUSEDNAME A is declared but never used | integer :: a, c"}},
	}

	for _, c := range cases {
		sameWarnings(t, c.name, warned(t, []string{"UNUSED"}, c.lines...), c.want)
	}
}

// TestWarningsHostHidden: under DECLARATIONS, an internal procedure's own name that
// its host has too is HOSTHIDDEN, a dummy argument among them; its own name and a
// name it only uses (the host's) aren't.
func TestWarningsHostHidden(t *testing.T) {
	program := []string{
		"module consts",
		"  integer :: limit = 3",
		"end module consts",
		"program p",
		"  use consts",
		"  implicit none",
		"  integer :: i, total",
		"  total = 0",
		"  do i = 1, limit",
		"    call add(i)",
		"  end do",
		"  print *, total",
		"contains",
		"  subroutine add(i)",
		"    integer :: i, limit",
		"    limit = 1",
		"    total = total + i * limit",
		"  end subroutine add",
		"end program p",
	}

	sameWarnings(t, "DECLARATIONS", warned(t, []string{"DECLARATIONS"}, program...), []string{
		"14:18 HOSTHIDDEN I is declared here, hiding P's I | subroutine add(i)",
		"15:19 HOSTHIDDEN LIMIT is declared here, hiding P's LIMIT (from CONSTS) | integer :: i, limit",
	})

	sameWarnings(t, "the default", warned(t, []string{"GENERAL"}, program...), nil)
}

// TestWarningsUnread: under UNUSED, a variable or array given a value that nothing
// reads is UNREAD, at its first assignment; a DO variable, one passed as an argument,
// one read on any line (in statement order, not flow), a dummy argument, a NAMELIST
// item, a COMMON variable, and a TARGET aren't.
func TestWarningsUnread(t *testing.T) {
	program := []string{
		"subroutine s(d)",
		"  integer :: d, i, w, n, m, k, c, g",
		"  real :: a(3)",
		"  real, target :: tg",
		"  common /blk/ c",
		"  namelist /grp/ g",
		"  w = 2",
		"  a(2) = 1",
		"  w = 3",
		"  do i = 1, 3",
		"  end do",
		"  n = 1",
		"  call t(n)",
		"  m = 1",
		"  m = m + 1",
		"  d = 1",
		"  c = 1",
		"  g = 1",
		"  write (*, nml=grp)",
		"  tg = 1",
		"  k = 5",
		"  print *, k",
		"end subroutine s",
	}

	sameWarnings(t, "UNUSED", warned(t, []string{"UNUSED"}, program...), []string{
		"7:3 UNREAD W is given a value here but never used | w = 2",
		"8:3 UNREAD A is given a value here but never used | a(2) = 1",
	})

	sameWarnings(t, "the default", warned(t, []string{"GENERAL"}, program...), nil)
}
