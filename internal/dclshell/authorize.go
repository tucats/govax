package dclshell

import (
	"strings"

	"github.com/tucats/govax/internal/authorize"
)

// This file is how DCL runs AUTHORIZE (docs/planning/PHASE-95 -
// authentication.md, Decision 10): the AUTHORIZE verb, and MCR and RUN of
// SYS$SYSTEM:AUTHORIZE, which on VMS run AUTHORIZE.EXE. govax's AUTHORIZE
// is Go (internal/authorize), run for the process whose DCL this is: its
// credentials ask for the authorization file, its SYS$OUTPUT gets the
// messages, and a session's commands come as HELP's responses do, a
// procedure's data lines or the terminal after the prompt.

// Authorize runs AUTHORIZE with command, the rest of the line: one
// command, or a session at the UAF> prompt. Its $STATUS is the one
// AUTHORIZE leaves, its messages already shown.
func (c *Interpreter) Authorize(command string) error {
	if err := c.machine.RequireInit(); err != nil {
		return err
	}

	env := c.RTL
	if env == nil {
		return nil
	}

	defer c.EndHelpInput()

	u := &authorize.Utility{
		Store:       env.System,
		Credentials: env.Credentials(),
		Out:         c.Out,
		Read:        c.ReadHelpResponse,
	}

	c.SetCommandStatus(u.Run(command), nil)

	return nil
}

// builtinUtility reports whether spec, an image MCR, RUN, or a foreign
// command names, is one of govax's utilities written in Go, which it
// runs in place of an image: AUTHORIZE, as SYS$SYSTEM:AUTHORIZE[.EXE]
// names it, or bare ("MCR AUTHORIZE") where bare is allowed, since MCR's
// default directory is SYS$SYSTEM.
func builtinUtility(spec string, bare bool) bool {
	s := strings.ToUpper(strings.Trim(strings.TrimSpace(spec), `"`))

	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = s[:i]
	}

	s = strings.TrimSuffix(s, ".EXE")

	switch s {
	case "SYS$SYSTEM:AUTHORIZE":
		return true
	case "AUTHORIZE":
		return bare
	}

	return false
}
