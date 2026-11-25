package corevms

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoUncheckedHostAccess holds govax to Decision 8 of
// docs/planning/PHASE-97 - host file access.md: with vax.host.sysprv on,
// only a process holding SYSPRV may use a host file, so every place that
// opens, creates, or runs one on the host must either ask the gate
// (rms.Session.Locate and LocateOutput, Session.MayUseHost,
// Environment.HostAllowed or HostCommandAllowed) or be given a location
// that something which asked has already placed, or be one of the files
// govax reaches for itself (the plan's A.4: configuration, the command
// line, the directories the configuration names).
//
// It scans every package under internal, as rms's TestNoUncheckedAccess
// scans RMS, and fails on a function making one of hostOpenCalls that
// hostAccessReasons doesn't name, and on a name there that no longer
// makes one, so the list stays the truth. Build tools (package main, run
// by go generate, never by govax) aren't scanned.
func TestNoUncheckedHostAccess(t *testing.T) {
	found := map[string]bool{}

	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel := filepath.ToSlash(strings.TrimPrefix(path, ".."+string(filepath.Separator)))

		for _, fn := range hostOpeners(t, path) {
			key := rel + ":" + fn
			found[key] = true

			if _, ok := hostAccessReasons[key]; !ok {
				t.Errorf("%s reaches a host file unchecked: ask the gate first (rms.Session.MayUseHost, corevms.Environment.HostAllowed), or name it in hostAccessReasons with the reason", key)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for key := range hostAccessReasons {
		if !found[key] {
			t.Errorf("%s is in hostAccessReasons but makes none of the calls: remove it", key)
		}
	}
}

// hostOpenCalls are the calls that open, create, or run something on the
// host, by package and function name.
var hostOpenCalls = map[string]map[string]bool{
	"os":   {"Open": true, "OpenFile": true, "Create": true, "ReadFile": true, "WriteFile": true},
	"exec": {"Command": true, "CommandContext": true},
}

// hostOpeners are the functions in the Go file path ("Type.Method" for a
// method) that make one of hostOpenCalls; none for a package main.
func hostOpeners(t *testing.T, path string) []string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	if file.Name.Name == "main" {
		return nil
	}

	var openers []string

	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		makesCall := false

		ast.Inspect(fd, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if pkg, ok := sel.X.(*ast.Ident); ok && hostOpenCalls[pkg.Name][sel.Sel.Name] {
				makesCall = true
			}

			return true
		})

		if makesCall {
			openers = append(openers, funcName(fd))
		}
	}

	return openers
}

// funcName is a function's name, or a method's as Type.Method.
func funcName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return fd.Name.Name
	}

	typ := fd.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}

	if id, ok := typ.(*ast.Ident); ok {
		return id.Name + "." + fd.Name.Name
	}

	return fd.Name.Name
}

// hostAccessReasons are the functions allowed to open host files, each
// with why it may. "Placed" means its caller passes a location that
// rms.Session.Locate or LocateOutput gave, which asked the gate.
var hostAccessReasons = map[string]string{
	// Asking the gate itself.
	"corevms/file.go:shimExeOpen":                 "EXE$OPEN asks HostAllowed first",
	"machine/rom.go:Machine.SaveROM":              "asks mayCreateHostFile first",
	"machine/rom.go:Machine.SaveNVRAM":            "asks mayCreateHostFile first",
	"dclshell/spawnhost.go:Interpreter.spawnHost": "asks HostCommandAllowed first",

	// Placed by Locate or LocateOutput (subtask 3's audit).
	"dclshell/spawnhost.go:Interpreter.hostInput":           "SPAWN/HOST's /INPUT, placed by Locate",
	"dclshell/spawnhost.go:Interpreter.hostOutput":          "SPAWN/HOST's /OUTPUT, placed by LocateOutput",
	"dclshell/procedure.go:Interpreter.openProcedureOutput": "@'s /OUTPUT, placed by LocateOutput",
	"machine/imagesource.go:openHostImage":                  "RUN's image, placed by Locate; a shareable image from the locator's configured directories",
	"rms/compare.go:compareWithHost":                        "BACKUP/COMPARE's target, placed",
	"rms/copy.go:copyOneFileToHost":                         "COPY's destination, placed by LocateOutput",
	"rms/dclfile.go:openHostDCLFile":                        "OPEN and the debugger's log, placed",
	"rms/edit.go:openHostForEdit":                           "EDIT of a host file, placed by Locate",
	"rms/hostfile.go:Session.readHostFile":                  "a placed host file, or HostFallback's configured places",
	"rms/recordfile.go:createTemp":                          "beside a placed host file being rewritten",
	"rms/restore.go:restoreToHost":                          "BACKUP's restore target, placed by LocateOutput",
	"rms/save.go:Session.CreateSaveSet":                     "BACKUP's save set, placed by LocateOutput",
	"rms/saveset.go:openHostSaveSet":                        "BACKUP's save set input, placed by Locate",
	"dclcmd/linksource.go:Context.openLibraryImage":         "an image beside a placed shareable image library",

	// govax's own files (the plan's A.4, and the review's answers 3 and 4).
	"dcl/define.go:LoadGrammarFile":                "the grammars, named by the command line",
	"dclcmd/edit.go:Context.Edit":                  "the host editor, vax.editor.command (the review, 3)",
	"dclcmd/edit.go:writeScratch":                  "the editor's scratch copy, vax.editor.scratch",
	"dclcmd/edit.go:readScratch":                   "the editor's scratch copy, vax.editor.scratch",
	"dclcmd/linksource.go:Context.openSharedImage": "the locator's host step: SET SHARE_PREFIX and vax.library (the review, 4)",
	"dclcmd/syslib.go:Context.readLibraryFile":     "vax.library's STARLET.MLB, IMAGELIB.OLB, STARLET.OLB",
	"hostlock/hostlock.go:TryLock":                 "lock files beside the system's own files",
	"hostlock/hostlock.go:Wait":                    "lock files beside the system's own files",
	"respath/respath.go:Resolver.search":           "--path directories; callers ask MayUseHost for the current directory (ReadConfigured)",
	"rms/pagefile.go:openHostPageFile":             "the page file, vax.sysgen.paging.file",
	"jobqueue/file.go:Load":                        "the queue database, vax.queue.file",
	"jobqueue/file.go:LockManager":                 "the queue manager's lock beside the queue database",
	"uaf/file.go:Load":                             "the authorization file, vax.authorization.file",

	// Test support, which nothing in a govax build imports.
	"testproc/testproc.go:Isolate": "tests only: runs the test binary again as a child process",
	"testproc/testproc.go:Shared":  "tests only: a result shared by a go test run's processes",
}
