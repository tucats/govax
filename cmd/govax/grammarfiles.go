package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/debugger"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/sysvol"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file reads the grammars govax starts with
// (docs/planning/PHASE-87 - system volume.md, Decision 4). Each is found through
// the resolver: a host copy (the current directory, then --path's
// directories), the system volume's, then govax's own. The volume's copy
// is the user's, and may be older than this govax, or edited: one that
// can't be read, or a grammar that doesn't parse or lacks a command this
// govax binds, is VAX_SYSGRAMMAR's warning, and govax uses its own copy
// instead, so a bad file on the volume never leaves govax without a
// command line. A bad host copy is still an error, as it was: that is a
// developer's, given on purpose.

// grammarFiles reads govax's start-up files through a resolver, warning
// on out of a system volume's copy it doesn't use.
type grammarFiles struct {
	resolver *respath.Resolver
	out      io.Writer

	// device is the system volume's device ("DUA0"), for the warnings'
	// file names, or "" with no system volume.
	device string
}

// volumeName returns the system volume's specification of the file the
// resolver calls name: DUA0:[SYSTEM.SYSMGR]CONSOLE.DCL.
func (g grammarFiles) volumeName(name string) string {
	return g.device + ":" + sysvol.Resolved[name]
}

// warn says the system volume's copy of name isn't used, and why.
func (g grammarFiles) warn(name string, cause error) {
	fmt.Fprintf(g.out, "%%%s\n", vmserrors.Wrap(vmserrors.VAX_SYSGRAMMAR, cause, g.volumeName(name)))
}

// grammar reads and parses the grammar name, and says where it came
// from. A system volume's copy that can't be read or doesn't parse is
// warned of, and govax's own is returned.
func (g grammarFiles) grammar(name string) (*dcl.Grammar, respath.Origin, error) {
	src, origin, err := g.resolver.ReadFileFrom(name)
	if err == nil {
		parsed, parseErr := dcl.ParseGrammar(string(src))
		if parseErr == nil {
			return parsed, origin, nil
		}

		err = parseErr
	}

	if origin != respath.Volume {
		return nil, origin, vmserrors.Wrap(vmserrors.VAX_GRAMMAR, err)
	}

	g.warn(name, err)

	builtin, err := g.builtinGrammar(name)

	return builtin, respath.Embedded, err
}

// builtinGrammar parses govax's own copy of the grammar name.
func (g grammarFiles) builtinGrammar(name string) (*dcl.Grammar, error) {
	src, err := g.resolver.ReadEmbedded(name)
	if err != nil {
		return nil, vmserrors.Wrap(vmserrors.VAX_GRAMMAR, err)
	}

	parsed, err := dcl.ParseGrammar(string(src))
	if err != nil {
		return nil, vmserrors.Wrap(vmserrors.VAX_GRAMMAR, err)
	}

	return parsed, nil
}

// unusable reports whether the grammar name, from origin and now bound
// to govax's handlers, lacks any of them: a system volume's copy from an
// older govax, or one a command was taken out of. It warns of one that
// does, which the caller replaces with govax's own. A host copy is used
// as it is.
func (g grammarFiles) unusable(name string, origin respath.Origin, bound *dcl.Grammar) bool {
	missing := bound.Missing()
	if origin != respath.Volume || len(missing) == 0 {
		return false
	}

	shown := missing
	if len(shown) > 5 {
		shown = shown[:5]
	}

	cause := "it has no " + strings.Join(shown, ", ")
	if more := len(missing) - len(shown); more > 0 {
		cause += fmt.Sprintf(", and %d more of govax's commands", more)
	}

	g.warn(name, fmt.Errorf("%s", cause))

	return true
}

// installConsoleGrammar reads the console's grammar and makes the
// console's dispatcher on it. A system volume's grammar that lacks one
// of govax's commands is replaced by govax's own, the dispatcher made
// again.
func installConsoleGrammar(c *console.Console, files grammarFiles) (*console.Dispatcher, error) {
	grammar, origin, err := files.grammar("console.dcl")
	if err != nil {
		return nil, err
	}

	d := console.NewDispatcher(c, grammar)

	if files.unusable("console.dcl", origin, grammar) {
		if grammar, err = files.builtinGrammar("console.dcl"); err != nil {
			return nil, err
		}

		// NewDispatcher makes the console's own dispatcher only when it
		// has none.
		c.Dispatcher = nil
		d = console.NewDispatcher(c, grammar)
	}

	return d, nil
}

// installDebuggerGrammar reads the debugger's grammar and
// installs the debugger on it, as installConsoleGrammar does the
// console's (docs/planning/PHASE-42.md: the DEBUG command and the DBG> prompt need
// them).
func installDebuggerGrammar(c *console.Console, files grammarFiles) error {
	grammar, origin, err := files.grammar("debug.dcl")
	if err != nil {
		return err
	}

	debugger.Install(c, grammar)

	if files.unusable("debug.dcl", origin, grammar) {
		if grammar, err = files.builtinGrammar("debug.dcl"); err != nil {
			return err
		}

		debugger.Install(c, grammar)
	}

	return nil
}
