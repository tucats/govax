// Package dcl implements a grammar-driven command-language parser for the
// govax console, in the spirit of dclrtl.c/dclrtl.h (see docs/PHASE-08.md).
//
// The C source's dclrtl.c is a fully generic, self-hosting engine: its own
// grammar-definition language is itself defined as a $$$DCL$$$ grammar and
// parsed by a hand-built finite-state-machine byte-code interpreter
// (FSMdefine/FSMrun) that DCLparse drives one token/state-transition at a
// time. This package deliberately does not port that FSM interpreter or the
// self-hosting bootstrap — see docs/PHASE-08.md's progress log for the
// rationale. Instead it implements the same observable grammar language and
// command-parsing semantics (verb/syntax/type/keyword/qualifier/parameter
// statements; unambiguous abbreviation matching; NO-prefix negation;
// qualifier/keyword /syntax= redirection into an alternate parameter/
// qualifier set; $rest_of_line's greedy consume-to-end-of-line behavior;
// required-parameter/qualifier and DISALLOW-combination checks) directly
// against a Grammar data structure, built once by parsing the grammar
// definition text (testdata/dcl/evax.dcl) rather than being derived
// statement-by-statement through the FSM.
//
// Phase 23 adds one grammar-language feature with no dclrtl.c counterpart at
// all: a parameter-scoped qualifier. Writing "qualifier host/parameter=source"
// nested under a "parameter source" statement attaches that qualifier to the
// Parameter itself (Parameter.Qualifiers) instead of to the enclosing
// verb/syntax (Entry.Qualifiers) — needed by commands like COPY, whose
// SOURCE and DESTINATION parameters can each independently carry their own
// /HOST. Parse resolves a "/name" token against whichever parameter was most
// recently filled in first, falling back to the entry's own qualifiers only
// if that lookup misses, and Result.ParamPresent/ParamString/etc. read the
// matched values back out by (parameter name, qualifier name) pair. This is
// additive: a grammar that declares no parameter-scoped qualifiers (every
// verb/syntax before Phase 23) behaves identically to before.
//
// This is a from-scratch,
// behavior-preserving reimplementation appropriate to this project's "not a
// cross-compile" goal (see docs/PLAN.md), not a fidelity deviation — the DCL
// engine is the console's own parsing tool, not emulated VAX ISA behavior,
// so docs/DEVIATIONS.md's bug-fixing policy does not apply to it.
package dcl
