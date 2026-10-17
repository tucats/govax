package console_test

import (
	"testing"
)

// Phase 64's subtask 8 (docs/PHASE-64 - page file.md): the page file is
// installed as the system starts, after vax.init and before the user's
// commands (cmd/govax calls StartPageFile), so its blocks are taken on
// its volume at once and the volume can't be dismounted, before any page
// has been written; vax.sysgen.paging.defer keeps the old behavior, the
// file opened at the pager's first need.

func TestPageFileInstalledAtStartup(t *testing.T) {
	pageFileSettings(t)
	setSetting(t, "vax.sysgen.paging.file", "DUA1:[000000]PAGEFILE.SYS")
	setSetting(t, "vax.sysgen.paging.size", "200")

	c, dispatch := wsDispatcher(t)
	mountPageFileVolume(t, c, "DUA1")

	if c.RTL.PageFile() != nil {
		t.Fatal("the page file is open before the system has started")
	}

	c.StartPageFile()

	pf := c.RTL.PageFile()
	if pf == nil || !pf.Created() || pf.Name() != "DUA1:[000000]PAGEFILE.SYS;1" || pf.Size() < 200 {
		t.Fatalf("the page file on DUA1 wasn't installed: %+v", pf)
	}

	if pf.Map.UsedBlocks() != 0 {
		t.Errorf("%d page file blocks in use at startup, want none", pf.Map.UsedBlocks())
	}

	want := "%DISM-W-CANNOTDMT, DUA1: cannot be dismounted|%DISM-W-INSWPGFIL, 1 swap or page files installed on volume"
	if got := dispatch("DISMOUNT DUA1:"); got != want {
		t.Errorf("DISMOUNT DUA1: at startup:\n got %q\nwant %q", got, want)
	}

	if got := dispatch(`WRITE SYS$OUTPUT F$GETSYI("PAGEFILE_PAGE")`); got != "200" {
		t.Errorf("F$GETSYI(\"PAGEFILE_PAGE\") = %q, want 200", got)
	}

	c.EndSession()
}

func TestPageFileDeferred(t *testing.T) {
	pageFileSettings(t)
	setSetting(t, "vax.sysgen.paging.defer", "true")

	c, _ := wsDispatcher(t)
	c.StartPageFile()

	if c.RTL.PageFile() != nil {
		t.Error("the page file was installed with vax.sysgen.paging.defer")
	}

	if c.RTL.OpenPageFile() == nil {
		t.Error("the deferred page file didn't open at need")
	}

	c.EndSession()
}
