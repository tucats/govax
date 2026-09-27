// Package io implements the govax device abstraction (see
// docs/PHASE-09.md), the Go equivalent of
// reference/eVAX/eVAX/Source/RTL/devices.c. Phase 09 also ported
// logical_names.c here; Phase 25 replaced that with internal/lnm.
//
// Both C files are RTL/emulator-tooling support, not emulated VAX ISA
// behavior — like internal/console/dcl (Phase 08), so docs/DEVIATIONS.md's
// ISA-fidelity bug-fixing policy doesn't apply here; findings in the C
// source are just fixed or documented in this phase's own progress log.
//
// Scope: this package owns the data structures and lookup/definition logic
// (find_device/define_device/get_dev_class_name) that Phase 08's console
// commands (SHOW DEVICE, DEFINE/DEVICE — see internal/console/device.go)
// and Phase 10's RTL system services (SYS$GETDVIW, SYS$ASSIGN) build on. The actual system-service
// call marshaling (SSDEF's argc/argv-from-VAX-memory convention, item-list
// walking) belongs to Phase 10 (RTL simulators), since it needs the RTL
// calling-convention plumbing this phase doesn't implement — nothing here
// touches internal/vm or internal/cpu.
package io
