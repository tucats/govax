package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
)

// $GETQUI's searches and contexts (docs/planning/PHASE-98 - batch
// queues.md, subtask 9) beyond what round 2 of the probe asked
// (internal/console's TestGetquiOracle holds those to VMS): DISPLAY_ENTRY
// and DISPLAY_FILE walks, FREEZE_CONTEXT, a job the caller may not read,
// TRANSLATE_QUEUE through a logical name, DISPLAY_MANAGER, and each access
// mode's context its own.

// quiText is an item's value as text, for comparing.
func quiText(info QueueInfo, name string) string {
	data, ok := info.Item(quiCode(name))
	if !ok {
		return "(none)"
	}

	return data
}

// quiLong is a longword item's value.
func quiLong(info QueueInfo, name string) uint32 {
	data, _ := info.Item(quiCode(name))

	return ItemLongword(data)
}

func TestGetquiSearches(t *testing.T) {
	env, a, path := jobServiceFixture(t)

	for _, step := range []struct {
		function string
		itmlst   uint32
	}{
		{"CREATE_QUEUE", a.items(sjcItem(a, "QUEUE", "BAT98_R"), sjcItem(a, "BATCH", nil))},
	} {
		if _, st := sndjbc(t, env, a, step.function, step.itmlst, true); st != jbcNormal {
			t.Fatalf("%s: %#x", step.function, st)
		}
	}

	submit := func(name string, files ...string) int {
		t.Helper()

		s, err := env.SubmitJob(JobRequest{Queue: "BAT98_Q", Name: name, Files: files, Hold: true, Priority: -1})
		if err != nil {
			t.Fatalf("SubmitJob %s: %v", name, err)
		}

		return s.Job.Entry
	}

	first := submit("FIRST", "DUA0:[000000]BAT98OK.COM", "DUA0:[000000]BAT98WRN.COM")
	second := submit("SECOND", "DUA0:[000000]BAT98OK.COM")
	other := submit("OTHER", "DUA0:[000000]BAT98OK.COM")

	// OTHER is another user's, in another group.
	if err := jobqueue.Update(path, func(d *jobqueue.Database) error {
		j := d.Job(other)
		j.User, j.Owner = "BAT98W", jobqueue.UIC(0o300<<16|0o10)

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ask := func(q QueueQuery) (QueueInfo, uint32) {
		t.Helper()

		return env.QueueInformation(q)
	}

	code := func(name string) uint16 { return quiCode(name) }

	// A wildcard DISPLAY_ENTRY: the caller's own jobs by entry number,
	// then NOMOREENT, then again from the start.
	for i, want := range []any{first, second, jbcNoMoreEnt, first} {
		info, st := ask(QueueQuery{Function: code("DISPLAY_ENTRY")})

		switch w := want.(type) {
		case int:
			if st != jbcNormal || int(quiLong(info, "ENTRY_NUMBER")) != w {
				t.Errorf("DISPLAY_ENTRY %d: %#x entry %d, want %d", i, st, quiLong(info, "ENTRY_NUMBER"), w)
			}
		case uint32:
			if st != w {
				t.Errorf("DISPLAY_ENTRY %d: %#x, want %#x", i, st, w)
			}
		}
	}

	// Its files, then NOMOREFILE, then none.
	for i, want := range []any{"_GOVAX$DUA0:[000000]BAT98OK.COM;1", "_GOVAX$DUA0:[000000]BAT98WRN.COM;1", jbcNoMoreFile, jbcNoSuchFile} {
		info, st := ask(QueueQuery{Function: code("DISPLAY_FILE")})

		switch w := want.(type) {
		case string:
			if st != jbcNormal || quiText(info, "FILE_SPECIFICATION") != w {
				t.Errorf("DISPLAY_FILE %d: %#x %q, want %q", i, st, quiText(info, "FILE_SPECIFICATION"), w)
			}
		case uint32:
			if st != w {
				t.Errorf("DISPLAY_FILE %d: %#x, want %#x", i, st, w)
			}
		}
	}

	// A DISPLAY_JOB with no queue context.
	ask(QueueQuery{Function: code("CANCEL_OPERATION")})

	if _, st := ask(QueueQuery{Function: code("DISPLAY_JOB")}); st != jbcNoQueCtx {
		t.Errorf("DISPLAY_JOB with no queue context: %#x", st)
	}

	// A wildcard DISPLAY_QUEUE: FREEZE_CONTEXT gives the same queue again.
	for i, want := range []string{"BAT98_Q", "BAT98_Q", "BAT98_R", ""} {
		flags := uint32(0)
		if i == 1 {
			flags = quiFreeze
		}

		info, st := ask(QueueQuery{Function: code("DISPLAY_QUEUE"), Name: "BAT98*", HasName: true, Flags: flags})
		if got := quiText(info, "QUEUE_NAME"); want == "" && st != jbcNoMoreQue || want != "" && got != want {
			t.Errorf("DISPLAY_QUEUE %d: %#x %q, want %q", i, st, got, want)
		}
	}

	// ALL_JOBS: every job in the queue, in SHOW QUEUE's order; without
	// it, the caller's own.
	ask(QueueQuery{Function: code("DISPLAY_QUEUE"), Name: "BAT98_Q", HasName: true})

	var names []string

	for {
		info, st := ask(QueueQuery{Function: code("DISPLAY_JOB"), Flags: quiAllJobs})
		if st != jbcNormal {
			break
		}

		names = append(names, quiText(info, "JOB_NAME"))
	}

	if got := len(names); got != 3 || names[2] != "OTHER" {
		t.Errorf("DISPLAY_JOB ALL_JOBS: %q", names)
	}

	// A job the caller may not read: only the few items, and
	// JOB_INACCESSIBLE.
	saved := *env.Process
	env.Process.Username, env.Process.UIC = "BAT98", 0o200<<16|0o10
	env.Process.CurrentPrivileges = 0

	info, st := ask(QueueQuery{Function: code("DISPLAY_ENTRY"), Number: first, HasNumber: true})
	if st != jbcNormal || quiLong(info, "JOB_STATUS")&quiMask("JOB_INACCESSIBLE") == 0 ||
		quiText(info, "JOB_NAME") != "(none)" || int(quiLong(info, "ENTRY_NUMBER")) != first {
		t.Errorf("another's job: %#x status %#x name %q", st, quiLong(info, "JOB_STATUS"), quiText(info, "JOB_NAME"))
	}

	// Its queue's owner and protection: not shown (SECURITY_INACCESSIBLE).
	info, _ = ask(QueueQuery{Function: code("DISPLAY_QUEUE"), Name: "BAT98_Q", HasName: true})
	if quiLong(info, "OWNER_UIC") != 0 || quiLong(info, "PROTECTION") != 0xFFFF ||
		quiLong(info, "QUEUE_FLAGS")&quiMask("SECURITY_INACCESSIBLE") == 0 {
		t.Errorf("the queue's security without read access: owner %#x protection %#x flags %#x",
			quiLong(info, "OWNER_UIC"), quiLong(info, "PROTECTION"), quiLong(info, "QUEUE_FLAGS"))
	}

	*env.Process = saved

	// TRANSLATE_QUEUE through a logical name.
	if _, err := env.Logicals.Define("LNM$PROCESS_TABLE", "MYQ", lnm.User, 0, []lnm.Equivalence{{Value: "BAT98_R"}}); err != nil {
		t.Fatal(err)
	}

	info, st = ask(QueueQuery{Function: code("TRANSLATE_QUEUE"), Name: "MYQ", HasName: true})
	if st != jbcNormal || quiText(info, "QUEUE_NAME") != "BAT98_R" {
		t.Errorf("TRANSLATE_QUEUE MYQ: %#x %q", st, quiText(info, "QUEUE_NAME"))
	}

	// DISPLAY_MANAGER: the one manager, once to a wildcard.
	info, st = ask(QueueQuery{Function: code("DISPLAY_MANAGER"), Name: "*", HasName: true})
	if st != jbcNormal || quiText(info, "MANAGER_NAME") != "SYS$QUEUE_MANAGER" ||
		quiLong(info, "MANAGER_STATUS") != quiMask("MANAGER_RUNNING") {
		t.Errorf("DISPLAY_MANAGER *: %#x %q %#x", st, quiText(info, "MANAGER_NAME"), quiLong(info, "MANAGER_STATUS"))
	}

	if _, st := ask(QueueQuery{Function: code("DISPLAY_MANAGER"), Name: "*", HasName: true}); st != jbcNoMoreQmgr {
		t.Errorf("DISPLAY_MANAGER * again: %#x", st)
	}

	if _, st := ask(QueueQuery{Function: code("DISPLAY_MANAGER"), Name: "OTHER", HasName: true}); st != jbcNoSuchQmgr {
		t.Errorf("DISPLAY_MANAGER OTHER: %#x", st)
	}

	// THIS_JOB from a process that isn't a batch job; forms left out.
	if _, st := ask(QueueQuery{Function: code("DISPLAY_JOB"), Flags: quiThisJob}); st != jbcNoSuchJob {
		t.Errorf("DISPLAY_JOB THIS_JOB: %#x", st)
	}

	if _, st := ask(QueueQuery{Function: code("DISPLAY_FORM"), Name: "*", HasName: true}); st != jbcNotSupported {
		t.Errorf("DISPLAY_FORM: %#x", st)
	}

	// An image's context (user mode) is apart from DCL's (supervisor).
	ask(QueueQuery{Function: code("CANCEL_OPERATION")})
	ask(QueueQuery{Function: code("DISPLAY_QUEUE"), Name: "BAT98*", HasName: true})

	user := env.quiContextFor(registers.User)

	info, _ = env.queueInformation(user, QueueQuery{Function: code("DISPLAY_QUEUE"), Name: "BAT98*", HasName: true})
	if got := quiText(info, "QUEUE_NAME"); got != "BAT98_Q" {
		t.Errorf("user mode's first wildcard queue: %q", got)
	}

	if info, _ := ask(QueueQuery{Function: code("DISPLAY_QUEUE"), Name: "BAT98*", HasName: true}); quiText(info, "QUEUE_NAME") != "BAT98_R" {
		t.Errorf("supervisor mode's second wildcard queue: %q", quiText(info, "QUEUE_NAME"))
	}
}
