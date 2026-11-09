package dclcmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is DCL's EDIT command, govax's own: there is no EDT or TPU to run, so
// EDIT hands a copy of the file to an editor on the host. The file goes to a
// private directory in the scratch area, the editor runs on it, and what it wrote
// comes back as a new version of the file (rms/edit.go has how lines become the
// file's records). A file comes back only if the editor ended successfully and
// wrote it: quitting vi with :q! leaves the file alone, even though vi then says
// it succeeded.
//
// What the user typed isn't thrown away because govax can't use it. If the editor
// wrote the file, but it can't become a new version (a line too long for a fixed
// record, say) or the editor then reported a failure, the scratch file stays, and
// the message says where.
//
// The machine doesn't run while the editor does: the command holds the only
// goroutine that would run it, so every process waits, and the editor has the
// terminal to itself.

// The configuration keys SET EDITOR and SHOW EDITOR work on.
const (
	EditorCommandSetting = "vax.editor.command"
	EditorScratchSetting = "vax.editor.scratch"
)

// defaultEditorCommand is the editor with nothing configured.
const defaultEditorCommand = "vi %file"

// editorFilePattern finds %file in an editor's command. A command typed in DCL
// without quotes is in uppercase, so case doesn't matter.
var editorFilePattern = regexp.MustCompile(`(?i)%file`)

// editorCommand is the configured editor's command.
func editorCommand() string {
	if command := strings.TrimSpace(settings.Get(EditorCommandSetting)); command != "" {
		return command
	}

	return defaultEditorCommand
}

// editorScratch is the configured directory for the editor's files.
func editorScratch() string {
	if scratch := strings.TrimSpace(settings.Get(EditorScratchSetting)); scratch != "" {
		return scratch
	}

	return os.TempDir()
}

// SplitCommand splits a host command into its program and arguments: words
// separated by white space, where single or double quotes keep a word's white
// space. No shell is involved, so a file's name can't mean anything but itself.
func SplitCommand(command string) ([]string, error) {
	var (
		words   []string
		word    strings.Builder
		inWord  bool
		inQuote rune
	)

	for _, r := range command {
		switch {
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				word.WriteRune(r)
			}

		case r == '"' || r == '\'':
			inQuote, inWord = r, true

		case unicode.IsSpace(r):
			if inWord {
				words = append(words, word.String())
				word.Reset()

				inWord = false
			}

		default:
			word.WriteRune(r)

			inWord = true
		}
	}

	if inQuote != 0 {
		return nil, fmt.Errorf("a quote isn't closed")
	}

	if inWord {
		words = append(words, word.String())
	}

	if len(words) == 0 {
		return nil, fmt.Errorf("there is no command")
	}

	return words, nil
}

// editorArguments is the command to run to edit path: %file in any word of
// command stands for it, and without one it goes last.
func editorArguments(command, path string) ([]string, error) {
	words, err := SplitCommand(command)
	if err != nil {
		return nil, err
	}

	named := false

	for i, word := range words {
		if editorFilePattern.MatchString(word) {
			named = true
			words[i] = editorFilePattern.ReplaceAllLiteralString(word, path)
		}
	}

	if !named {
		words = append(words, path)
	}

	return words, nil
}

// EditorOptions is SET EDITOR's qualifiers. A nil field wasn't given. An empty
// value goes back to the default.
type EditorOptions struct {
	Command *string
	Scratch *string
}

// SetEditor is SET EDITOR: it changes the configuration, which govax writes back
// when it exits, so that later sessions keep it (as SET PROMPT does). Nothing
// changes if either value can't be used.
func (c *Context) SetEditor(opts EditorOptions) error {
	if opts.Command != nil && *opts.Command != "" {
		if _, err := SplitCommand(*opts.Command); err != nil {
			return vmserrors.New(vmserrors.EDIT_COMMAND, *opts.Command, err.Error())
		}
	}

	if opts.Scratch != nil && *opts.Scratch != "" {
		if err := checkScratch(*opts.Scratch); err != nil {
			return vmserrors.New(vmserrors.EDIT_SCRATCH, *opts.Scratch, err.Error())
		}
	}

	setOrDelete(EditorCommandSetting, opts.Command)
	setOrDelete(EditorScratchSetting, opts.Scratch)

	return nil
}

// setOrDelete changes one configuration key, if a value was given.
func setOrDelete(key string, value *string) {
	switch {
	case value == nil:
	case *value == "":
		_ = settings.Delete(key)
	default:
		settings.Set(key, *value)
	}
}

// checkScratch reports what's wrong with a scratch directory.
func checkScratch(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return errors.Unwrap(err)
	}

	if !info.IsDir() {
		return errors.New("it is not a directory")
	}

	return nil
}

// ShowEditor is SHOW EDITOR.
func (c *Context) ShowEditor() error {
	c.Printf("  Editor command:     %s%s\n", editorCommand(), defaultNote(EditorCommandSetting))
	c.Printf("  Scratch directory:  %s%s\n", editorScratch(), defaultNote(EditorScratchSetting))

	return nil
}

// defaultNote says a value shown isn't one the configuration has.
func defaultNote(key string) string {
	if strings.TrimSpace(settings.Get(key)) == "" {
		return " (the default)"
	}

	return ""
}

// Edit is the EDIT command.
func (c *Context) Edit(name string, host bool) error {
	file, err := c.ContainerSession.OpenForEdit(name, host)
	if err != nil {
		return editFailure(err, name)
	}

	scratch := editorScratch()
	if err := checkScratch(scratch); err != nil {
		return vmserrors.New(vmserrors.EDIT_SCRATCH, scratch, err.Error())
	}

	dir, err := os.MkdirTemp(scratch, "govax-edit-")
	if err != nil {
		return vmserrors.New(vmserrors.EDIT_SCRATCH, scratch, errors.Unwrap(err).Error())
	}

	// keep is set when the directory holds what the user typed, and a message
	// says where it is.
	keep := false

	defer func() {
		if !keep {
			_ = os.RemoveAll(dir)
		}
	}()

	path := filepath.Join(dir, file.ScratchName())

	before, err := writeScratch(path, file)
	if err != nil {
		return vmserrors.New(vmserrors.EDIT_SCRATCH, scratch, err.Error())
	}

	command := editorCommand()

	words, err := editorArguments(command, path)
	if err != nil {
		return vmserrors.New(vmserrors.EDIT_COMMAND, command, err.Error())
	}

	failed := c.runEditor(exec.Command(words[0], words[1:]...)) //nolint:gosec // running it is the point

	edited, written := readScratch(path, file, before)

	switch {
	case failed != nil && written:
		keep = true

		return keptText(vmserrors.New(vmserrors.EDIT_EDITOR, command, failed.Error(), file.Spec), path)

	case failed != nil:
		return vmserrors.New(vmserrors.EDIT_EDITOR, command, failed.Error(), file.Spec)

	case !written:
		c.Printf("%%EDIT-I-NOCHANGE, %s not %s\n", file.Spec, map[bool]string{true: "created", false: "changed"}[file.New])

		return nil
	}

	created, lines, err := c.ContainerSession.SaveEdit(file, edited)
	if err != nil {
		keep = true

		return keptText(saveFailure(err, file.Spec), path)
	}

	if file.MixedControl {
		c.Printf("%%EDIT-W-VFCCONTROL, the records of %s had different control bytes, and the new version's are all the same\n", file.Spec)
	}

	c.Printf("%%EDIT-S-CREATED, %s created, %d %s\n", created.Name, lines, map[bool]string{true: "line", false: "lines"}[lines == 1])

	return nil
}

// scratchState is what a scratch file was before the editor ran, to tell whether
// it wrote the file.
type scratchState struct {
	exists   bool
	modified time.Time
}

// writeScratch puts the text to edit in the scratch file. A new file has no
// scratch file, so that the editor can say it's new.
func writeScratch(path string, file *rms.EditFile) (scratchState, error) {
	if file.New {
		return scratchState{}, nil
	}

	if err := os.WriteFile(path, file.Text, 0o600); err != nil {
		return scratchState{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return scratchState{}, err
	}

	return scratchState{exists: true, modified: info.ModTime()}, nil
}

// readScratch returns what the editor left in the scratch file, and whether it
// wrote the file at all. The modification time says so, even if it wrote what was
// there already, and the contents say so on a file system whose clock is too
// coarse to tell. An editor that removed the file wrote nothing.
func readScratch(path string, file *rms.EditFile, before scratchState) ([]byte, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}

	edited, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	written := !before.exists || !info.ModTime().Equal(before.modified) || !bytes.Equal(edited, file.Text)

	return edited, written
}

// runEditor runs the editor at the terminal, or with no terminal to lend it, on
// the DCL's output and no input.
func (c *Context) runEditor(cmd *exec.Cmd) error {
	if c.LendTerminal != nil {
		return c.LendTerminal(cmd)
	}

	cmd.Stdout, cmd.Stderr = c.Out, c.Out

	return cmd.Run()
}

// keptText adds to err the place the user's text is, in the second line a VMS
// message has for what follows from the first.
func keptText(err vmserrors.VMSError, path string) error {
	return vmserrors.ConditionError{
		Status: err.Status,
		Text:   err.Error() + "\n-EDIT-I-KEPT, the edited text is in " + path,
	}
}

// saveFailure is the message for an edited file that can't become a new version.
func saveFailure(err error, spec string) vmserrors.VMSError {
	var tooLong *rms.TooLongError
	if errors.As(err, &tooLong) {
		return vmserrors.New(vmserrors.EDIT_TOOLONG, fmt.Sprint(tooLong.Line), fmt.Sprint(tooLong.Length), spec, fmt.Sprint(tooLong.Limit))
	}

	return vmserrors.New(vmserrors.EDIT_NOTSAVED, spec, err.Error())
}

// editFailure reports a file that couldn't be opened for editing: no editor has
// started.
func editFailure(err error, name string) error {
	if lnmErr := logicalNameFailure(err); lnmErr != nil {
		return lnmErr
	}

	var (
		notMounted *rms.NotMountedError
		notFound   *rms.NotFoundError
		ambiguous  *rms.AmbiguousError
		notText    *rms.NotTextError
		readOnly   *rms.ReadOnlyError
	)

	switch {
	case errors.As(err, &notMounted):
		return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
	case errors.As(err, &notFound):
		return vmserrors.Wrap(vmserrors.SS_NOSUCHFILE, err, notFound.Spec)
	case errors.As(err, &ambiguous):
		return vmserrors.Wrap(vmserrors.CLI_AMBIGUOUS, err, "file specification", name)
	case errors.As(err, &notText):
		return vmserrors.New(vmserrors.EDIT_NOTTEXT, notText.Spec, notText.Reason)
	case errors.As(err, &readOnly):
		return vmserrors.New(vmserrors.EDIT_READONLY, readOnly.Spec)
	}

	return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, name)
}
