package corevms

import "github.com/tucats/govax/internal/rms"

// This file is internal/rtl's half of docs/PHASE-22.md's subtask 11: it
// wires internal/rms's already-implemented, already-tested RMS service
// handlers (SysCreate/SysConnect/SysOpen/SysClose/SysGet/SysPut, and
// SysRename, subtask 17) into this package's own ServiceTable, so a
// running VAX program's SYS$CREATE/SYS$CONNECT/SYS$OPEN/SYS$CLOSE/SYS$GET/
// SYS$PUT/SYS$RENAME calls actually reach them.
//
// # Why a wrapper closure per service, instead of registering the functions
// # directly
//
// ServiceTable.Register wants a ServiceFunc: func(*Environment, []uint32)
// (uint32, error). internal/rms's own handlers are func(*rms.Context,
// []uint32) (uint32, error) instead — same shape, different first-argument
// type, because internal/rms can't import internal/rtl to spell out
// *corevms.Environment itself (see internal/rms/context.go's own doc comment:
// this package registers rms's handlers, so the dependency has to run this
// direction, and Go refuses a two-way package import). Each wrapper below
// is exactly that adaptation: build a *rms.Context from the Environment
// this call was actually made against (env.rmsContext, environment.go),
// then forward argv to the real handler unchanged.
func registerRMSServices(t *ServiceTable) {
	for name, fn := range rmsServices {
		t.Register(name, func(env *Environment, argv []uint32) (status uint32, err error) {
			// The volume blocks the service read and wrote are the
			// process's direct I/O (iocount.go).
			env.countVolumeIO(func() { status, err = fn(env.rmsContext(), argv) })

			return status, err
		})
	}
}

// rmsServices are the RMS services, by name: Phase 22's and 33's, and
// Phase 49's record operations past $GET and $PUT ($FIND, $UPDATE,
// $TRUNCATE, $DELETE, $REWIND).
var rmsServices = map[string]func(*rms.Context, []uint32) (uint32, error){
	"SYS$CREATE":   rms.SysCreate,
	"SYS$CONNECT":  rms.SysConnect,
	"SYS$OPEN":     rms.SysOpen,
	"SYS$CLOSE":    rms.SysClose,
	"SYS$GET":      rms.SysGet,
	"SYS$PUT":      rms.SysPut,
	"SYS$PARSE":    rms.SysParse,
	"SYS$SEARCH":   rms.SysSearch,
	"SYS$DISPLAY":  rms.SysDisplay,
	"SYS$RENAME":   rms.SysRename,
	"SYS$WAIT":     rms.SysWait,
	"SYS$FLUSH":    rms.SysFlush,
	"SYS$ERASE":    rms.SysErase,
	"SYS$FREE":     rms.SysFree,
	"SYS$RELEASE":  rms.SysRelease,
	"SYS$FIND":     rms.SysFind,
	"SYS$UPDATE":   rms.SysUpdate,
	"SYS$TRUNCATE": rms.SysTruncate,
	"SYS$DELETE":   rms.SysDelete,
	"SYS$REWIND":   rms.SysRewind,
}
