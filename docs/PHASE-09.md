# Phase 09: I/O & device support

## Goal

Port device abstraction and logical name tables — the layer that gives the emulator
symbolic device/file names and a pluggable device model, sitting under both the
console and the RTL.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/RTL/devices.c` — device abstraction (note: this file
  lives under the C tree's `Source/RTL/`, but `docs/PLAN.md` calls out I/O as its own
  concern distinct from the RTL library simulators — kept as a separate phase here per
  that plan, even though the C source doesn't draw the line at a separate directory).
- `reference/eVAX/eVAX/Source/RTL/logical_names.c` — VMS-style logical name tables.

## Deliverables

- `internal/io` package with a device abstraction and logical-name-table
  implementation, consumed by Phase 08 (console SHOW DEVICE, etc.) and Phase 10 (RTL
  file I/O resolving logical names).
- Unit tests: logical name define/translate/delete, device registration and lookup.

## Open questions / notes

- Confirm the device model's shape once RMS (Phase 10) requirements are clearer — file
  I/O and device abstraction are tightly coupled in VMS, so this phase may need minor
  revisiting once Phase 10 starts.

## Progress Log

_Not started._
