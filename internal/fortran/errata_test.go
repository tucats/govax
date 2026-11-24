package fortran_test

import (
	"strings"
	"testing"
)

// Phase 113's errata (docs/planning/PHASE-113 - Fortran errata.md), subtask 2:
// what a module's text says of what it couldn't before.

// erratumModule has a VAX structure with initial values of several types, in a
// union too, and a derived type with a CHARACTER component of length zero.
const erratumModule = `module errata
  structure /stamp/
    integer*2 code /7/
    real*8 when /2.5d0/
    character*4 tag /'ABCD'/
    logical*1 flags(3) /.true., .false., .true./
    union
      map
        integer*4 whole /-1/
      end map
      map
        integer*2 halves(2)
      end map
    end union
    complex both /(1.0, -2.0)/
  end structure
  type hollow
    character(len=0) :: nothing
    integer :: n = 3
  end type
  record /stamp/ last
end module`

// TestErrataModuleText holds the module's text: each field's values after its
// name, VAX FORTRAN's form, and the component of length zero; and a program
// using the module compiles.
func TestErrataModuleText(t *testing.T) {
	r := compileFree("ERRATA", erratumModule, nil)
	if r.Errors != 0 {
		t.Fatalf("messages: %v", messageList(r))
	}

	text := strings.Join(moduleText(t, r, "ERRATA"), "\n")
	for _, want := range []string{
		"    INTEGER*2 CODE /7_2/",
		"    REAL*8 WHEN /2.5E+00_8/",
		"    CHARACTER*4 TAG /'ABCD'/",
		"    LOGICAL*1 FLAGS(1:3) /.TRUE._1, .FALSE._1, .TRUE._1/",
		"        INTEGER*4 WHOLE /-1_4/",
		"        INTEGER*2 HALVES(1:2)\n",
		"    COMPLEX*8 BOTH /(1.0E+00_4, -2.0E+00_4)/",
		"    CHARACTER(LEN=0) :: NOTHING\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no\n%s\nin\n%s", want, text)
		}
	}

	user := compileFree("USER", `program user
  use errata
  record /stamp/ s
  type(hollow) :: h
  print *, s.code, s.when, s.tag, s.flags, s.whole, s.both, len(h%nothing), h%n
end program`, moduleFiles{"ERRATA": moduleText(t, r, "ERRATA")})
	if user.Errors != 0 {
		t.Fatalf("messages: %v", messageList(user))
	}
}
