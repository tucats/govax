package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMountDefaultVolume_noFile confirms an unset container file does
// nothing at all: no mount, no message, no default.
func TestMountDefaultVolume_noFile(t *testing.T) {
	c, buf := newTestConsole(t)
	before := c.ContainerSession.DefaultString()

	c.MountDefaultVolume(DefaultVolume{})

	if buf.Len() != 0 {
		t.Errorf("output = %q, want none", buf.String())
	}

	if _, mounted := c.Mounts.Lookup("DUA0"); mounted {
		t.Error("DUA0 is mounted, want nothing mounted")
	}

	if got := c.ContainerSession.DefaultString(); got != before {
		t.Errorf("default = %q, want it unchanged (%q)", got, before)
	}
}

// TestMountDefaultVolume_creates confirms a container that doesn't exist
// is created (in a directory that doesn't exist yet either), mounted
// writable on the default device, given the default directory, and made
// the default.
func TestMountDefaultVolume_creates(t *testing.T) {
	c, buf := newTestConsole(t)
	c.Verbose = true

	t.Cleanup(func() { _ = c.Mounts.DismountAll() })

	path := filepath.Join(t.TempDir(), "sub", "work.dsk")

	c.MountDefaultVolume(DefaultVolume{File: path, Type: "RX50"})

	out := buf.String()
	for _, want := range []string{
		"%MOUNT-I-CREATED, container " + path + " created as an RX50 volume labeled WORK\n",
		"%MOUNT-I-MOUNTED, WORK mounted on _DUA0:\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}

	if strings.Contains(out, "-W-") || strings.Contains(out, "-E-") {
		t.Errorf("output %q has a warning or error", out)
	}

	if !c.Mounts.Writable("DUA0") {
		t.Error("DUA0 isn't mounted writable")
	}

	if got := c.ContainerSession.DefaultString(); got != "DUA0:[WORK]" {
		t.Errorf("default = %q, want DUA0:[WORK]", got)
	}

	if d, ok := c.Devices.Find("DUA0"); !ok || d.MaxBlock != 800 || d.DevType == 0 {
		t.Errorf("device DUA0 = %+v, %v; want an RX50 (800 blocks, a device type)", d, ok)
	}
}

// TestMountDefaultVolume_existing confirms an existing container is
// mounted as it is, its label checked when one is configured, and a
// configured directory that isn't on it falls back to the MFD.
func TestMountDefaultVolume_existing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.dsk")

	setup, _ := newTestConsole(t)
	if err := setup.InitializeContainer(path, 800, "OLDVOL", 0, ""); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	t.Run("matching label, missing directory", func(t *testing.T) {
		c, buf := newTestConsole(t)
		c.Verbose = true

		t.Cleanup(func() { _ = c.Mounts.DismountAll() })

		c.MountDefaultVolume(DefaultVolume{File: path, Label: "oldvol", Device: "DUA1:", Directory: "SRC"})

		out := buf.String()
		if strings.Contains(out, "CREATED") {
			t.Errorf("output %q says the existing container was created", out)
		}

		for _, want := range []string{
			"%MOUNT-I-MOUNTED, OLDVOL mounted on _DUA1:\n",
			"%MOUNT-W-NODIR, directory [SRC] not found on DUA1:; using [000000]\n",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("output %q lacks %q", out, want)
			}
		}

		if got := c.ContainerSession.DefaultString(); got != "DUA1:[000000]" {
			t.Errorf("default = %q, want DUA1:[000000]", got)
		}
	})

	t.Run("wrong label", func(t *testing.T) {
		c, buf := newTestConsole(t)

		c.MountDefaultVolume(DefaultVolume{File: path, Label: "WORK"})

		if out := buf.String(); !strings.Contains(out, "%MOUNT-W-NODEFAULT, default volume "+path+" not mounted: volume label is OLDVOL, not WORK") {
			t.Errorf("output = %q, want a NODEFAULT warning about the label", out)
		}

		if _, mounted := c.Mounts.Lookup("DUA0"); mounted {
			t.Error("DUA0 is still mounted after a label mismatch")
		}
	})
}

// TestMountDefaultVolume_readOnly confirms a container the host won't let
// govax write is mounted read-only, and says so.
func TestMountDefaultVolume_readOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only file")
	}

	path := filepath.Join(t.TempDir(), "ro.dsk")

	setup, _ := newTestConsole(t)
	if err := setup.InitializeContainer(path, 800, "WORK", 0, ""); err != nil {
		t.Fatalf("InitializeContainer: %v", err)
	}

	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}

	c, buf := newTestConsole(t)
	c.Verbose = true

	t.Cleanup(func() { _ = c.Mounts.DismountAll() })

	c.MountDefaultVolume(DefaultVolume{File: path})

	out := buf.String()
	if !strings.Contains(out, "%MOUNT-I-WRITELOCK, volume is write locked\n%MOUNT-I-MOUNTED, WORK mounted on _DUA0:\n") {
		t.Errorf("output = %q, want WRITELOCK then MOUNTED", out)
	}

	if _, mounted := c.Mounts.Lookup("DUA0"); !mounted || c.Mounts.Writable("DUA0") {
		t.Error("DUA0 isn't mounted read-only")
	}
}

// TestMountDefaultVolume_quiet confirms a console that isn't verbose (a
// one-shot command) shows no informational messages.
func TestMountDefaultVolume_quiet(t *testing.T) {
	c, buf := newTestConsole(t)
	c.Verbose = false

	t.Cleanup(func() { _ = c.Mounts.DismountAll() })

	c.MountDefaultVolume(DefaultVolume{File: filepath.Join(t.TempDir(), "q.dsk"), Type: "RX50"})

	if buf.Len() != 0 {
		t.Errorf("output = %q, want none", buf.String())
	}

	if _, mounted := c.Mounts.Lookup("DUA0"); !mounted {
		t.Error("DUA0 isn't mounted")
	}
}

// TestMountDefaultVolume_badType confirms an unknown device type is a
// warning, and nothing is created.
func TestMountDefaultVolume_badType(t *testing.T) {
	c, buf := newTestConsole(t)
	path := filepath.Join(t.TempDir(), "x.dsk")

	c.MountDefaultVolume(DefaultVolume{File: path, Type: "RZ99"})

	if !strings.Contains(buf.String(), "unknown device type RZ99") {
		t.Errorf("output = %q, want an unknown-type warning", buf.String())
	}

	if _, err := os.Stat(path); err == nil {
		t.Error("the container was created anyway")
	}
}
