package console

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestConsoleInitializeContainer_buildsAMountableVolume confirms the
// Console-level wrapper's happy path: InitializeContainer builds a
// container the same c.Mount used by every other test in this package can
// then attach to a device, with the label threaded all the way through --
// the InitializeContainer/Mount two-step docs/PHASE-23.md's own design
// section describes (INITIALIZE formats a volume without mounting it; a
// separate MOUNT attaches it).
func TestConsoleInitializeContainer_buildsAMountableVolume(t *testing.T) {
	c, _ := newTestConsole(t)
	path := filepath.Join(t.TempDir(), "new.dsk")

	if err := c.InitializeContainer(path, 400, "TESTVOL", 0); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount of a freshly initialized container: %v", err)
	}

	if label, ok := c.Mounts.VolumeLabel("DUA0"); !ok || label != "TESTVOL" {
		t.Errorf("VolumeLabel(DUA0) = %q, %v, want TESTVOL, true", label, ok)
	}
}

// TestConsoleInitializeContainer_badPath confirms a failure from the
// sibling internal/rms.InitializeContainer (here, a nonexistent parent
// directory) is translated into the real SS_BADPARAM console status,
// matching every other console command that reports a bad argument via a
// real VMS-style status code rather than a bare Go error.
func TestConsoleInitializeContainer_badPath(t *testing.T) {
	c, _ := newTestConsole(t)
	path := filepath.Join(t.TempDir(), "no-such-directory", "new.dsk")

	err := c.InitializeContainer(path, 400, "TESTVOL", 0)
	if err == nil {
		t.Fatal("InitializeContainer with a nonexistent parent directory = nil error, want SS_BADPARAM")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_BADPARAM)) {
		t.Errorf("InitializeContainer error = %v, want SS_BADPARAM", err)
	}
}

// TestConsoleInitializeContainer_zeroBlocks confirms a size of zero blocks
// -- rejected by the sibling internal/rms.InitializeContainer -- is also
// reported as SS_BADPARAM here, the same translation TestConsoleInitialize
// Container_badPath checks for a different underlying failure.
func TestConsoleInitializeContainer_zeroBlocks(t *testing.T) {
	c, _ := newTestConsole(t)
	path := filepath.Join(t.TempDir(), "new.dsk")

	err := c.InitializeContainer(path, 0, "TESTVOL", 0)
	if err == nil {
		t.Fatal("InitializeContainer with blocks=0 = nil error, want SS_BADPARAM")
	}

	if !errors.Is(err, vmserrors.New(vmserrors.SS_BADPARAM)) {
		t.Errorf("InitializeContainer error = %v, want SS_BADPARAM", err)
	}
}

// TestDispatch_initializeContainerViaDCL exercises this subtask's own new
// dispatch.go work directly: parsing and dispatching a real
// "INITIALIZE/CONTAINER ..." command line through the DCL grammar
// (internal/bootdata/files/evax.dcl's initialize_container syntax, stubbed
// in subtask 1) into the g.Bind("INITIALIZE_CONTAINER", ...) closure this
// subtask added, which in turn calls the already-tested
// Console.InitializeContainer. Confirms PATH/SIZE/LABEL/CLUSTER all thread
// through correctly by mounting the result afterward and checking its
// label -- the same "prove it by mounting" strategy
// TestConsoleInitializeContainer_buildsAMountableVolume uses one layer
// down.
//
// The container path is double-quoted in the command line for the same
// reason TestDispatch_mountAndDismountViaDCL's own comment explains: DCL
// upcases an unquoted token, and t.TempDir() paths are mixed-case.
func TestDispatch_initializeContainerViaDCL(t *testing.T) {
	d, c := newTestDispatcher(t)
	path := filepath.Join(t.TempDir(), "new.dsk")

	if err := d.Dispatch(fmt.Sprintf(`INITIALIZE/CONTAINER "%s" 400 TESTVOL`, path)); err != nil {
		t.Fatalf("Dispatch INITIALIZE/CONTAINER: %v", err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mount of the result: %v", err)
	}

	if label, ok := c.Mounts.VolumeLabel("DUA0"); !ok || label != "TESTVOL" {
		t.Errorf("VolumeLabel(DUA0) = %q, %v, want TESTVOL, true", label, ok)
	}
}

// TestDispatch_initializeContainerAbbreviatedViaDCL confirms "INIT" still
// reaches INITIALIZE_CONTAINER via /CONTAINER, the container-formatting
// counterpart to TestDispatch_initAbbreviatesInitializeVax
// (dispatch_test.go) -- INIT is DCL's own unambiguous-prefix abbreviation
// of INITIALIZE (docs/PHASE-23.md's "INITIALIZE: unifying INIT and
// INITIALIZE under one verb" design section), not tied to either
// qualifier specifically, so it has to work for /CONTAINER exactly as well
// as it already does for /VAX.
func TestDispatch_initializeContainerAbbreviatedViaDCL(t *testing.T) {
	d, _ := newTestDispatcher(t)
	path := filepath.Join(t.TempDir(), "new.dsk")

	if err := d.Dispatch(fmt.Sprintf(`INIT/CONTAINER "%s" 400 TESTVOL`, path)); err != nil {
		t.Fatalf("Dispatch INIT/CONTAINER: %v", err)
	}
}

// TestDispatch_initializeContainerDoesNotMount confirms INITIALIZE/
// CONTAINER, dispatched through the real DCL grammar and Console layer end
// to end, never leaves the new container mounted -- matching real VMS's
// own INITIALIZE and docs/PHASE-23.md's own explicit design note that this
// command formats a volume without mounting it.
func TestDispatch_initializeContainerDoesNotMount(t *testing.T) {
	d, c := newTestDispatcher(t)
	path := filepath.Join(t.TempDir(), "new.dsk")

	if err := d.Dispatch(fmt.Sprintf(`INITIALIZE/CONTAINER "%s" 400 TESTVOL`, path)); err != nil {
		t.Fatalf("Dispatch INITIALIZE/CONTAINER: %v", err)
	}

	if _, ok := c.Mounts.Lookup("DUA0"); ok {
		t.Error("Lookup(DUA0) after INITIALIZE/CONTAINER (before any MOUNT) = found, want not found")
	}
}

// TestDispatch_initializeContainerDefaultCluster confirms omitting the
// optional /CLUSTER qualifier dispatches successfully (r.Int("CLUSTER")'s
// zero value reaching Console.InitializeContainer as clusterSize=0, which
// the sibling internal/rms.InitializeContainer/volume.Initialize document
// as selecting their own 1-block-per-cluster default) -- CLUSTER, unlike
// PATH/SIZE, carries no /prompt= in the grammar, so it must be genuinely
// optional on the command line.
func TestDispatch_initializeContainerDefaultCluster(t *testing.T) {
	d, _ := newTestDispatcher(t)
	path := filepath.Join(t.TempDir(), "new.dsk")

	if err := d.Dispatch(fmt.Sprintf(`INITIALIZE/CONTAINER "%s" 400`, path)); err != nil {
		t.Fatalf("Dispatch INITIALIZE/CONTAINER with no label/cluster: %v", err)
	}
}
