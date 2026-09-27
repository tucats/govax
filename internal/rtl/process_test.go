package rtl

import "testing"

func TestNewEnvironmentProcess(t *testing.T) {
	env, _ := fixture()
	p := env.Process

	if p == nil {
		t.Fatal("Environment.Process = nil, want the default emulated process")
	}

	if p.PID != nominalPID || p.Username != "SYSTEM" || p.UIC != NominalUIC {
		t.Errorf("process = PID %#x user %q UIC %#x, want %#x SYSTEM %#x",
			p.PID, p.Username, p.UIC, nominalPID, NominalUIC)
	}

	if p.UICGroup() != 1 || p.UICMember() != 4 {
		t.Errorf("UIC = [%o,%o], want [1,4]", p.UICGroup(), p.UICMember())
	}

	if p.WSLimit != p.WSDefault {
		t.Errorf("WSLimit = %d, want WSDEFAULT %d", p.WSLimit, p.WSDefault)
	}

	if !(p.MinWSCount <= p.WSDefault && p.WSDefault <= p.WSQuota && p.WSQuota <= p.WSExtent) {
		t.Errorf("working-set quotas out of order: MINWSCNT %d, WSDEFAULT %d, WSQUOTA %d, WSEXTENT %d",
			p.MinWSCount, p.WSDefault, p.WSQuota, p.WSExtent)
	}
}

func TestOptArg(t *testing.T) {
	argv := []uint32{7, 8}

	if got := optArg(argv, 1); got != 8 {
		t.Errorf("optArg(argv, 1) = %d, want 8", got)
	}

	if got := optArg(argv, 2); got != 0 {
		t.Errorf("optArg(argv, 2) = %d, want 0 for an omitted trailing argument", got)
	}
}
