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
// definition text (testdata/dcl/console.dcl) rather than being derived
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
// Phase 25 adds two more (docs/PHASE-25.md, subtask 5):
//
//   - A "/list" switch on a parameter or qualifier statement makes it take
//     a comma-separated list. A parameter list is written "A,B" or
//     "A, B"; a qualifier list is "/X=(A,B)", or a single "/X=A". Quoted
//     elements may contain commas. Result.List returns the elements (the
//     keyword names, for a keyword type); String/Keyword still return the
//     first.
//   - "disallow any2(A,B,C)", CDU's "at most one of these" form, alongside
//     the pairwise "disallow A and B" it expands into.
//
// Phase 37 (docs/PHASE-37.md) adds what the console's former fixed
// commands need:
//
//   - The $expression value type: a console address or value expression,
//     whose extent readExpression finds (blanks around operators,
//     parentheses, and quoted strings belong to it; a '/' after a blank
//     and before a letter starts a qualifier). The handler evaluates it.
//   - A parameter's /separator="c", which ends its value and is skipped
//     before the next parameter (DEPOSIT X=5).
//   - A verb's or syntax's /assignment=syntax: a line whose first
//     positional token is "name=" continues in that syntax (SET X=5).
//   - A keyword's /nonegatable, dclrtl.c's DCL_NONEGATE.
//   - A keyword's /value: KEYWORD=value, the value going to the first
//     parameter of the keyword's /syntax= (SET PROMPT="text").
//   - A leading '@' is a verb by itself ("@FILE").
//
// This is a from-scratch,
// behavior-preserving reimplementation appropriate to this project's "not a
// cross-compile" goal (see docs/PLAN.md), not a fidelity deviation — the DCL
// engine is the console's own parsing tool, not emulated VAX ISA behavior,
// so docs/DEVIATIONS.md's bug-fixing policy does not apply to it.
package dcl
