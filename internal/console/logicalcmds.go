package console

import (
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/lnm"
)

// bindLogicalCommands binds the logical-name commands' grammar entries
// (internal/bootdata/files/console.dcl) to c.
func bindLogicalCommands(g *dcl.Grammar, c *Console) {
	define := func(assign bool) dcl.Handler {
		return func(id int64, r *dcl.Result) error {
			name := r.String("NAME")
			if assign {
				// ASSIGN removes one trailing colon; DEFINE keeps it.
				name = strings.TrimSuffix(name, ":")
			}

			var attrs uint32

			for _, a := range r.List("TRANSLATION_ATTRIBUTES") {
				switch a {
				case "CONCEALED":
					attrs |= lnm.AttrConcealed

				case "TERMINAL":
					attrs |= lnm.AttrTerminal
				}
			}

			return c.cmd().DefineLogicalName(logicalTableQualifier(r), name, r.List("VALUE"),
				logicalModeQualifier(r), attrs, !r.Negated("LOG"))
		}
	}

	g.Bind("DEFINE", define(false))
	g.Bind("ASSIGN", define(true))

	g.Bind("DEASSIGN", func(id int64, r *dcl.Result) error {
		// Like ASSIGN, DEASSIGN removes one trailing colon.
		name := strings.TrimSuffix(r.String("NAME"), ":")

		return c.cmd().DeassignLogicalName(logicalTableQualifier(r), name, logicalModeQualifier(r), r.Present("ALL"))
	})

	// A bare CREATE makes a text file from SYS$INPUT (createfile.go);
	// /DIRECTORY (create.go) and /NAME_TABLE are below.
	g.Bind("CREATE", func(id int64, r *dcl.Result) error {
		return c.cmd().CreateFile(r.String("FILE"), r.ParamPresent("FILE", "HOST"), r.Present("LOG") && !r.Negated("LOG"))
	})

	g.Bind("CREATE_DIRECTORY", func(id int64, r *dcl.Result) error {
		req := dclcmd.CreateDirectoryRequest{
			Directories: r.List("DIRECTORIES"),
			OwnerUIC:    r.String("OWNER_UIC"),
			Protection:  r.List("PROTECTION"),
			Allocation:  int(r.Int("ALLOCATION")),
			Log:         r.Present("LOG") && !r.Negated("LOG"),
		}

		if r.Present("VERSION_LIMIT") {
			n := int(r.Int("VERSION_LIMIT"))
			req.VersionLimit = &n
		}

		return c.cmd().CreateDirectory(req)
	})

	g.Bind("CREATE_NAME_TABLE", func(id int64, r *dcl.Result) error {
		parent := r.String("PARENT_TABLE")
		if parent == "" {
			parent = lnm.ProcessDirectoryName
		}

		return c.cmd().CreateNameTable(r.String("TABLE"), parent, logicalModeQualifier(r), !r.Negated("LOG"))
	})

	g.Bind("SHOW_LOGICAL", func(id int64, r *dcl.Result) error {
		if r.Present("STRUCTURE") {
			c.cmd().ShowLogicalStructure()

			return nil
		}

		var tables []string

		switch {
		case r.Present("PROCESS"):
			tables = []string{"LNM$PROCESS"}

		case r.Present("JOB"):
			tables = []string{"LNM$JOB"}

		case r.Present("GROUP"):
			tables = []string{"LNM$GROUP"}

		case r.Present("SYSTEM"):
			tables = []string{"LNM$SYSTEM"}

		case r.Present("TABLE"):
			tables = r.List("TABLE")
		}

		return c.cmd().ShowLogical(r.List("NAME"), tables, r.Present("FULL"))
	})

	g.Bind("SHOW_TRANSLATION", func(id int64, r *dcl.Result) error {
		return c.cmd().ShowTranslation(r.String("TABLE"), r.String("NAME"))
	})
}

// logicalTableQualifier returns the table a DEFINE/ASSIGN/DEASSIGN
// command names: /PROCESS, /JOB, /GROUP, /SYSTEM, or /TABLE= (the grammar
// lets only one through), defaulting to the process table.
func logicalTableQualifier(r *dcl.Result) string {
	switch {
	case r.Present("JOB"):
		return "LNM$JOB"

	case r.Present("GROUP"):
		return "LNM$GROUP"

	case r.Present("SYSTEM"):
		return "LNM$SYSTEM"

	case r.Present("TABLE"):
		return r.String("TABLE")

	default:
		return "LNM$PROCESS"
	}
}

// logicalModeQualifier returns the access mode /USER_MODE,
// /SUPERVISOR_MODE, or /EXECUTIVE_MODE selects; supervisor by default.
func logicalModeQualifier(r *dcl.Result) lnm.Mode {
	switch {
	case r.Present("USER_MODE"):
		return lnm.User

	case r.Present("EXECUTIVE_MODE"):
		return lnm.Executive

	default:
		return lnm.Supervisor
	}
}
