package rms

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNoUncheckedAccess holds RMS to Decision 6 of docs/planning/PHASE-93 -
// file protection.md: every place that reaches a file or directory through
// ods2 (opening a file by ID, opening or resolving a directory, changing
// a directory's entries or a file's header, or searching with
// filespec.Glob) is in the checked layer, or decides the access itself,
// or has a reason not to. It scans this package's source, as
// internal/cpu's TestEveryInstructionImplemented scans the instruction
// table, and fails on a call in a function the list below doesn't name,
// and on a name in the list that no longer makes one, so that the list
// stays the truth.
func TestNoUncheckedAccess(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(files)

	found := map[string]bool{}

	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		for _, fn := range uncheckedCallers(t, name) {
			key := name + ":" + fn
			found[key] = true

			if _, ok := checkedAccess[key]; !ok {
				t.Errorf("%s reaches files through ods2 unchecked: move it onto the checked layer (access.go, utilaccess.go), or name it in checkedAccess with the reason", key)
			}
		}
	}

	for key := range checkedAccess {
		if !found[key] {
			t.Errorf("%s is in checkedAccess but makes none of the calls: remove it", key)
		}
	}
}

// uncheckedAccessCalls are the ods2 calls that reach a file or directory
// without a check: a method or function of these names (os.Remove and the
// like aside).
var uncheckedAccessCalls = map[string]bool{
	"OpenFID": true, "OpenDirectory": true, "ResolveDirectory": true, "Glob": true,
	"Insert": true, "Remove": true, "Rename": true, "UpdateHeader": true, "SetACL": true,
}

// uncheckedCallers are the functions in the Go file name that make one of
// uncheckedAccessCalls.
func uncheckedCallers(t *testing.T, name string) []string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	var callers []string

	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		calls := false

		ast.Inspect(fd, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !uncheckedAccessCalls[sel.Sel.Name] {
				return true
			}

			if pkg, ok := sel.X.(*ast.Ident); ok && (pkg.Name == "os" || pkg.Name == "strings" || pkg.Name == "slices") {
				return true
			}

			calls = true

			return true
		})

		if calls {
			callers = append(callers, fd.Name.Name)
		}
	}

	return callers
}

// checkedAccess are the functions allowed the calls, by file and name,
// each with why.
var checkedAccess = map[string]string{
	// The checked layer itself.
	"access.go:resolveChecked":     "the layer: a lookup check before each level",
	"access.go:mayOpenFID":         "the layer: reads the header it checks",
	"access.go:newVersionAllowed":  "the layer: reads the versions it checks",
	"utilaccess.go:globChecked":    "the layer: the directories checked first",
	"utilaccess.go:openChecked":    "the layer: reads the header it checks",
	"directory.go:readable":        "the layer's question for DIRECTORY: reads the header it checks",
	"recordfile.go:readRaw":        "an image's read (paging off): imageAllowed checks it; any other read is openChecked's",
	"profile.go:newFileProfile":    "reads the previous version's profile for the creator, not the creator's access",
	"createdir.go:deepestExisting": "finds the level CreateDirectory then checks (pathAllowed, needChange)",
	"copy.go:resolveVolumeDest":    "pathAllowed checks the levels first; createVersion the directory",
	"recordfile.go:CreateAllowed":  "pathAllowed checks the levels first; createAllowed the directory",

	// RMS's services and the ACP check in the service (subtask 8).
	"acp.go:ACPLookupAs":              "the ACP: needLookup on the directory",
	"acp.go:ACPAccessWith":            "the ACP: the access mode's credentials checked against the header",
	"acpattr.go:updateAttributes":     "the ACP: attributeChangeAllowed before it",
	"acpattr.go:ACPReadAttributesAs":  "the ACP: R or C checked",
	"acpattr.go:ACPWriteAttributesAs": "the ACP: attributeChangeAllowed",
	"acpcreate.go:createInDirectory":  "the ACP: the directory's R and W, a supersede's W and D",
	"acpcreate.go:ACPEnterAs":         "the ACP: the directory's R and W",
	"acpdelete.go:ACPDelete":          "the ACP: the directory's R and W, the file's D",
	"acpdelete.go:checkDeletable":     "the ACP: reads the header ACPDelete checks",
	"acpdelete.go:deleteNow":          "the ACP: a file deleted at its last close, checked when marked",
	"acpfilespec.go:ACPFileSpec":      "the back-link walk for a file already opened by ID (LIB$FID_TO_NAME)",
	"acpfilespec.go:entryIn":          "the back-link walk",
	"namfid.go:openByNAM":             "$OPEN by DID: needLookup on the directory, openFID's check on the file",
	"secobject.go:FindSecurityObject": "a security service's file: globChecked's directories; MayRead or MayChange before the profile is read or changed",
	"secobject.go:SetACL":             "a security service's change: MayChange asked first",
	"secobject.go:SecurityObjects":    "SET SECURITY's files: globChecked's directories; MayRead or MayChange before the profile is read or changed",
	"rename.go:renameSpecs":           "$RENAME: RMV and ENT checked before it",
	"search.go:current":               "$SEARCH's walk: searchAccess on each directory",
	"search.go:subdirectories":        "$SEARCH's walk: searchAccess on each directory",
	"search.go:searchPlain":           "$SEARCH's walk: searchAccess on each directory",
	"search.go:endAtTop":              "$SEARCH's walk, the existence of the level above a wildcard",
	"search.go:directoryExists":       "$SEARCH's existence test for RMS$_DNF; the walk checks access",
	"fop.go:makeTemporary":            "FOP TMP: the opener's own new file",
	"fop.go:deleteAtClose":            "FOP DLT: the opener's own file at its close",
	"create.go:markContiguity":        "$CREATE's new file",
	"xab.go:applyCreateXABs":          "$CREATE's new file",
	"xab.go:applyCloseXABs":           "a file the process opened for writing, at its close",
	"indexed.go:Extend":               "an indexed file the process opened for writing",
	"indexed.go:initIndexed":          "$CREATE's new indexed file",
	"convert.go:Close":                "CONVERT's output, opened by $CREATE",
	"convert.go:writeRaw":             "CONVERT's output, opened by $CREATE",

	// The DCL utilities, checking inline or on what they made themselves.
	"delete.go:deleteOnVolume":         "DELETE: the directory's R and W and the file's D checked inline",
	"purge.go:purgeOnVolume":           "PURGE: the directory's R and W and each version's D checked inline",
	"directory.go:matchAttributes":     "DIRECTORY: after readable",
	"directory.go:DirectoryMissing":    "DIRECTORY: whether a directory is there, for the message of a search that found nothing",
	"copy.go:createBinaryFile":         "COPY's new output file",
	"recordfile.go:RevisionDate":       "a file the utility found with globChecked and read: its header's date",
	"recordfile.go:SourceAttributesOf": "a file the utility found with globChecked and read: its header's facts",
	"recordfile.go:allocatedBlocks":    "RewriteRecordFile's file, checked for R and W first",
	"recordfile.go:versionLimit":       "RewriteRecordFile's file, checked for R and W first, and its new version",
	"renamecmd.go:searchRenameElement": "RENAME: whether the directory is there, then globChecked",
	"renamecmd.go:RenameTarget":        "RENAME: whether the target directory is there; $RENAME checks the access",
	"restore.go:restore":               "BACKUP's restore: pathAllowed first, the file made by createVersion",
	"restore.go:existing":              "BACKUP's restore: an overlaid file's W or a replaced one's D checked inline",
	"restore.go:fill":                  "BACKUP's restore: the file it made or may overlay",
	"restore.go:markBackupDirectory":   "BACKUP's restore: a directory it made",
	"save.go:Read":                     "BACKUP's save: R checked inline",
	"save.go:saveTops":                 "BACKUP's save: directories globChecked found",
	"save.go:directory":                "BACKUP's save: the walk, needList on each subdirectory",
	"save.go:writeSaveSet":             "BACKUP's save: the save set it made",
	"shareable.go:findVersion":         "LINK and RUN finding a shareable image: its read is checked (readRaw, AccessImage)",

	// Outside any process's protection (Decision 6's exceptions).
	"mount.go:DirectoryExists":       "MOUNT's and SET DEFAULT's existence test: VMS checks no access there",
	"pagefile.go:openVolumePageFile": "the page file: the system's, through the ACP as SYSTEM",
	"verify.go:VerifyVolume":         "volume verification, the system's",
}
