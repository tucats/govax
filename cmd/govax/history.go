package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
)

// The prompt's command history, one for each of its modes: the console's
// VAX>, the debugger's DBG>, and the assembler's ASM>. Each mode recalls
// its own commands, so the debugger's don't come back at the console's
// prompt after EXIT, nor the console's at DBG>; each is kept between
// sessions in a file of its own (modeHistory's names).
//
// readline keeps one history, in memory, which it adds each line read to.
// It can switch history files (SetHistoryPath), but then starts the new
// history empty, and switching whole configurations reloads a file over
// what it has. So govax keeps the files itself: readline has no history
// file, each mode's lines are kept here and appended to its file as they
// are read, and when the mode changes, readline's history is emptied and
// filled again with the new mode's lines.

// historyLimit is how many of a mode's commands are kept, as readline's
// own default limit.
const historyLimit = 500

// modeHistories are the prompt modes' histories. With no console history
// file (a test, or no home directory) there are none, and readline keeps
// its single in-memory history as it always has.
type modeHistories struct {
	console string
	lines   map[string][]string
	loaded  map[string]bool
	current string
	started bool
}

// newModeHistories returns the histories whose console file is console.
func newModeHistories(console string) *modeHistories {
	return &modeHistories{console: console, lines: map[string][]string{}, loaded: map[string]bool{}}
}

// recaller is the part of readline's Instance the histories fill: its
// in-memory history.
type recaller interface {
	ResetHistory()
	SaveHistory(content string) error
}

var _ recaller = (*readline.Instance)(nil)

// use makes mode's history the one rl recalls, if it isn't already.
func (h *modeHistories) use(rl recaller, mode string) {
	if h.console == "" || (h.started && mode == h.current) {
		return
	}

	h.started, h.current = true, mode

	rl.ResetHistory()

	for _, line := range h.load(mode) {
		_ = rl.SaveHistory(line)
	}
}

// load returns mode's lines, read from its file the first time.
func (h *modeHistories) load(mode string) []string {
	if h.loaded[mode] {
		return h.lines[mode]
	}

	h.loaded[mode] = true

	data, err := os.ReadFile(modeHistory(h.console, mode))
	if err != nil {
		return nil
	}

	var lines []string

	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}

	// A file grown past the limit is cut back to its last lines.
	if len(lines) > historyLimit {
		lines = lines[len(lines)-historyLimit:]
		_ = os.WriteFile(modeHistory(h.console, mode), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	}

	h.lines[mode] = lines

	return lines
}

// add records line, just read at the current mode's prompt (readline has
// added it to its own history already), and appends it to the mode's
// file.
func (h *modeHistories) add(line string) {
	line = strings.TrimSpace(line)
	if h.console == "" || line == "" {
		return
	}

	lines := append(h.load(h.current), line)
	if len(lines) > historyLimit {
		lines = lines[len(lines)-historyLimit:]
	}

	h.lines[h.current] = lines

	path := modeHistory(h.console, h.current)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}

	_, _ = f.WriteString(line + "\n")
	_ = f.Close()
}
