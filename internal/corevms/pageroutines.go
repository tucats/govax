package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmheap"
)

// A zone's page routines: LIB$CREATE_VM_ZONE's get-page and free-page
// arguments, procedures of the program's that the zone calls for its
// areas instead of LIB$GET_VM_PAGE and LIB$FREE_VM_PAGE, with the same
// arguments (the VSI LIB$ manual's LIB$CREATE_VM_ZONE; docs/PHASE-59 -
// LIB VM zones.md, subtask 6). The zone's allocator asks for pages in
// the middle of an allocation, so each call is a callback
// (callback.go's Callback): the RTL routine allocating must be running
// on a goroutine of its own (RunCallingBack).

// PageRoutineFailure is a zone's get-page or free-page routine returning
// a failure status (its low bit clear). The RTL routine that called it
// returns that status.
type PageRoutineFailure struct {
	Status uint32
}

func (e *PageRoutineFailure) Error() string {
	return fmt.Sprintf("corevms: page routine returned %08X", e.Status)
}

// zonePages returns where a zone with settings s takes its areas from:
// the image's page pool, or, when s names a get-page or a free-page
// routine, a vmheap.Borrowed that calls them. A routine not given is the
// pool's code: a zone with only a get-page routine gives its pages to
// the pool when it's deleted (which refuses, silently, pages that
// aren't the pool's), and one with only a free-page routine takes them
// from the pool as LIB$GET_VM_PAGE does, and gives them to the routine,
// as VMS 7.1 does (testdata/lib59).
func (env *Environment) zonePages(s ZoneSettings) vmheap.Pages {
	pool := env.pagePool()

	if s.GetPage == 0 && s.FreePage == 0 {
		return pool
	}

	return &vmheap.Borrowed{
		Get: func(n uint32) (uint32, error) {
			if s.GetPage == 0 {
				return pool.Allocate(n)
			}

			return env.callPageRoutine(s.GetPage, n, 0)
		},
		Put: func(addr, n uint32) error {
			if s.FreePage == 0 {
				_ = pool.Free(n, addr)

				return nil
			}

			_, err := env.callPageRoutine(s.FreePage, n, addr)

			return err
		},
	}
}

// callPageRoutine calls the page routine at routine with the arguments
// LIB$GET_VM_PAGE and LIB$FREE_VM_PAGE take: the addresses of a page
// count, n, and of a base address, addr (two longwords it puts below
// SP, under the argument list). It returns the base address as the
// routine left it (a get-page routine's pages), or a
// PageRoutineFailure.
func (env *Environment) callPageRoutine(routine, n, addr uint32) (uint32, error) {
	c, mem := env.cpu, env.mem

	sp := c.GPR(registers.SP)
	cells := sp - 8

	if err := mem.StoreLongword(c, cells, n); err != nil {
		return 0, err
	}

	if err := mem.StoreLongword(c, cells+4, addr); err != nil {
		return 0, err
	}

	c.SetGPR(registers.SP, cells)

	r0, err := env.Callback(routine, cells, cells+4)

	c.SetGPR(registers.SP, sp)

	if err != nil {
		return 0, err
	}

	if r0&1 == 0 {
		return 0, &PageRoutineFailure{Status: r0}
	}

	// The cells are below SP now, but nothing has run since the routine
	// returned that could have written there.
	return mem.LoadLongword(c, cells+4)
}
