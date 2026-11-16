package security

import "testing"

// The queue rule's particulars, each from the Guide to System Security
// (4.6.3, 4.8.7.2); corevms's TestBat98ProtectionMatrix holds the whole
// rule to VMS 7.1's runs.

// queueCode is a queue's code from a file code's letters: R read, W
// submit, E manage, D delete.
func queueCode(t *testing.T, text string) Protection {
	t.Helper()

	p, err := ParseProtection(text, NoAccess)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

func TestCheckQueueManageImplies(t *testing.T) {
	// (S:M,O:D,G:R,W:S), owned by [1,4]: the system category's M alone
	// gives read, submit, and delete too, but not control's changes.
	p := Profile{Owner: SystemUIC, Protection: queueCode(t, "(S:E,O:D,G:R,W:W)")}
	sysprv := Credentials{UIC: NewUIC(0o200, 1), Privileges: PrivSYSPRV}

	for _, want := range []Access{QueueRead, QueueSubmit, QueueManage, QueueDelete, QueueRead | QueueDelete} {
		if !CheckQueue(sysprv, p, want).Granted {
			t.Errorf("SYSPRV refused %s of (S:M)", QueueLetters(want))
		}
	}

	world := Credentials{UIC: NewUIC(0o200, 1)}
	if CheckQueue(world, p, QueueRead).Granted || !CheckQueue(world, p, QueueSubmit).Granted {
		t.Errorf("the world category should have submit alone")
	}
}

func TestCheckJobCodeOnlyOwnJobs(t *testing.T) {
	// (S,O:D,G:RD,W): a member of the group may delete its own jobs, and
	// not another member's, though the code's group field gives delete.
	p := Profile{Owner: NewUIC(0o200, 9), Protection: queueCode(t, "(S,O:D,G:RD,W)")}
	alice, bob := NewUIC(0o200, 1), NewUIC(0o200, 2)
	c := Credentials{UIC: alice}

	if !CheckJob(c, p, alice, QueueDelete).Granted {
		t.Error("refused delete of its own job")
	}

	if CheckJob(c, p, bob, QueueDelete).Granted || CheckJob(c, p, bob, QueueRead).Granted {
		t.Error("the code gave access to another's job")
	}
}

func TestCheckJobACLReachesEveryJob(t *testing.T) {
	// An ACE's read and delete reach every job in the queue.
	ace := ACE{Type: TypeIdentifier, Identifiers: []uint32{0x80010001}, Access: QueueRead | QueueDelete}
	p := Profile{Owner: NewUIC(0o200, 9), Protection: NoAccess, ACL: []ACE{ace}}
	c := Credentials{UIC: NewUIC(0o300, 1), Rights: []Identifier{{Value: 0x80010001}}}

	if !CheckJob(c, p, SystemUIC, QueueRead|QueueDelete).Granted {
		t.Error("the ACE's read and delete didn't reach SYSTEM's job")
	}

	if CheckQueue(c, p, QueueSubmit).Granted {
		t.Error("the ACE gave submit")
	}
}

func TestCheckJobManageReachesEveryJob(t *testing.T) {
	p := Profile{Owner: NewUIC(0o200, 9), Protection: queueCode(t, "(S,O,G,W:E)")}
	c := Credentials{UIC: NewUIC(0o300, 1)}

	if !CheckJob(c, p, SystemUIC, QueueDelete).Granted {
		t.Error("manage on the queue didn't reach another's job")
	}
}

func TestCheckJobOwnerSees(t *testing.T) {
	// Unconfirmed: a job's owner always sees it, whatever the code.
	p := Profile{Owner: SystemUIC, Protection: NoAccess}
	me := NewUIC(0o200, 1)

	if !CheckJob(Credentials{UIC: me}, p, me, QueueRead).Granted {
		t.Error("the owner couldn't see its own job")
	}

	if CheckJob(Credentials{UIC: me}, p, me, QueueDelete).Granted {
		t.Error("the owner could delete its job with no delete in the code")
	}
}

func TestQueueCodeText(t *testing.T) {
	if got := QueueCodeText(queueCode(t, "(S:E,O:D,G:R,W:W)")); got != "(S:M,O:D,G:R,W:S)" {
		t.Errorf("got %s", got)
	}

	if got := QueueCodeText(NoAccess); got != "(S,O,G,W)" {
		t.Errorf("got %s", got)
	}
}
