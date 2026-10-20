package console

import (
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/vmserrors"
)

// backupCommand is the BACKUP command r parsed, line its text
// (console.dcl's BACKUP: the command qualifiers on the verb, the others
// on the specifier they follow). A command giving two of the input's
// date kinds, or two of the output's /NEW_VERSION, /OVERLAY, and
// /REPLACE, is BADQUALIFIERCOMBO: the grammar's DISALLOW knows only the
// verb's own qualifiers. What VMS says of them is unconfirmed.
func backupCommand(r *dcl.Result, line string) (dclcmd.BackupCommand, error) {
	in, out := "INPUT", "OUTPUT"

	inputs := r.List(in)

	cmd := dclcmd.BackupCommand{
		Command: line,
		Input: dclcmd.BackupSpecifier{
			Text:    inputs[0],
			Host:    r.ParamPresent(in, "HOST"),
			SaveSet: r.ParamPresent(in, "SAVE_SET") && !r.ParamNegated(in, "SAVE_SET"),
		},
		Inputs: inputs,

		List:     r.Present("LIST") && !r.Negated("LIST") && !r.Defaulted("LIST"),
		ListFile: r.String("LIST"),
		Full:     r.Present("FULL") && !r.Negated("FULL"),
		Brief:    r.Present("BRIEF") && !r.Negated("BRIEF"),
		Log:      r.Present("LOG") && !r.Negated("LOG"),
		Compare:  r.Present("COMPARE") && !r.Negated("COMPARE"),

		CheckCRC: r.ParamPresent(in, "CRC") && !r.ParamNegated(in, "CRC"),
		Select:   r.ParamList(in, "SELECT"),
		Exclude:  r.ParamList(in, "EXCLUDE"),
		Since:    r.ParamString(in, "SINCE"),
		Before:   r.ParamString(in, "BEFORE"),

		WriteCRC: r.ParamPresent(out, "CRC") && !r.ParamNegated(out, "CRC"),
		Comment:  r.ParamString(out, "COMMENT"),
	}

	for _, kw := range r.List("IGNORE") {
		switch kw {
		case "NOBACKUP":
			cmd.IgnoreNoBackup = true
		case "INTERLOCK":
			cmd.IgnoreInterlock = true
		}
	}

	if r.Present(out) {
		cmd.Output = &dclcmd.BackupSpecifier{
			Text:    r.String(out),
			Host:    r.ParamPresent(out, "HOST"),
			SaveSet: r.ParamPresent(out, "SAVE_SET") && !r.ParamNegated(out, "SAVE_SET"),
		}
	}

	cmd.InputOwner = paramOwner(r, in)
	cmd.OutputOwner = paramOwner(r, out)

	for _, size := range []struct {
		name string
		to   **int
	}{{"BLOCK_SIZE", &cmd.BlockSize}, {"GROUP_SIZE", &cmd.GroupSize}} {
		if r.ParamPresent(out, size.name) {
			n := int(r.ParamInt(out, size.name))
			*size.to = &n
		}
	}

	kind, err := oneOf(r, in, []string{"MODIFIED", "BACKUP", "CREATED", "EXPIRED"})
	if err != nil {
		return cmd, err
	}

	cmd.DateKind = dclcmd.BackupDateKind(kind)

	existing, err := oneOf(r, out, []string{"", "NEW_VERSION", "OVERLAY", "REPLACE"})
	if err != nil {
		return cmd, err
	}

	cmd.Existing = dclcmd.BackupExisting(existing)

	return cmd, nil
}

// paramOwner is /BY_OWNER on param: nil when it isn't given, else its
// value ("" when it has none).
func paramOwner(r *dcl.Result, param string) *string {
	if !r.ParamPresent(param, "BY_OWNER") || r.ParamDefaulted(param, "BY_OWNER") || r.ParamNegated(param, "BY_OWNER") {
		return nil
	}

	owner := r.ParamString(param, "BY_OWNER")

	return &owner
}

// oneOf is the index in names of the one qualifier of param given, 0
// when none is (names[0] is the default, "" for none), or
// BADQUALIFIERCOMBO for two.
func oneOf(r *dcl.Result, param string, names []string) (int, error) {
	chosen, found := 0, false

	for i, name := range names {
		if name == "" || !r.ParamPresent(param, name) || r.ParamNegated(param, name) {
			continue
		}

		if found {
			return 0, vmserrors.New(vmserrors.CLI_BADQUALIFIERCOMBO, "/"+names[chosen], "/"+name)
		}

		chosen, found = i, true
	}

	return chosen, nil
}
