package rtl

import "github.com/tucats/govax/internal/rms"

// This file is internal/rtl's half of docs/PHASE-22.md's subtask 11: it
// wires internal/rms's six already-implemented, already-tested RMS service
// handlers (SysCreate/SysConnect/SysOpen/SysClose/SysGet/SysPut) into this
// package's own ServiceTable, so a running VAX program's SYS$CREATE/
// SYS$CONNECT/SYS$OPEN/SYS$CLOSE/SYS$GET/SYS$PUT calls actually reach them.
//
// # Why a wrapper closure per service, instead of registering the functions
// # directly
//
// ServiceTable.Register wants a ServiceFunc: func(*Environment, []uint32)
// (uint32, error). internal/rms's own handlers are func(*rms.Context,
// []uint32) (uint32, error) instead — same shape, different first-argument
// type, because internal/rms can't import internal/rtl to spell out
// *rtl.Environment itself (see internal/rms/context.go's own doc comment:
// this package registers rms's handlers, so the dependency has to run this
// direction, and Go refuses a two-way package import). Each wrapper below
// is exactly that adaptation: build a *rms.Context from the Environment
// this call was actually made against (env.rmsContext, environment.go),
// then forward argv to the real handler unchanged.
func registerRMSServices(t *ServiceTable) {
	t.Register("SYS$CREATE", func(env *Environment, argv []uint32) (uint32, error) {
		return rms.SysCreate(env.rmsContext(), argv)
	})
	t.Register("SYS$CONNECT", func(env *Environment, argv []uint32) (uint32, error) {
		return rms.SysConnect(env.rmsContext(), argv)
	})
	t.Register("SYS$OPEN", func(env *Environment, argv []uint32) (uint32, error) {
		return rms.SysOpen(env.rmsContext(), argv)
	})
	t.Register("SYS$CLOSE", func(env *Environment, argv []uint32) (uint32, error) {
		return rms.SysClose(env.rmsContext(), argv)
	})
	t.Register("SYS$GET", func(env *Environment, argv []uint32) (uint32, error) {
		return rms.SysGet(env.rmsContext(), argv)
	})
	t.Register("SYS$PUT", func(env *Environment, argv []uint32) (uint32, error) {
		return rms.SysPut(env.rmsContext(), argv)
	})
}
