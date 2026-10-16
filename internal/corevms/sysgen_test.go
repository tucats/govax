package corevms

import (
	"bytes"
	"testing"
)

// TestSysgenPQLDefault: $CREPRC's default for a quota is the configured
// PQL parameter when there is one, else quotaRules' own; a list item
// still wins, and the minimum still applies.
func TestSysgenPQLDefault(t *testing.T) {
	creator := NewProcess()
	creator.Job = newJob(creator.PID)
	creator.WSQuota, creator.WSDefault, creator.WSExtent = 5000, 5000, 20000

	sg := DefaultSysgen()
	sg.PQL = map[string]uint32{"WSQUOTA": 2000, "WSDEFAULT": 1500}

	values, _, st := resolveQuotas(&sg, creator, nil, false, false)
	if st != 0 {
		t.Fatalf("resolveQuotas: status %#x", st)
	}

	if got := values[pql("WSQUOTA")]; got != 2000 {
		t.Errorf("WSQUOTA = %d, want the configured 2000", got)
	}

	if got := values[pql("WSDEFAULT")]; got != 1500 {
		t.Errorf("WSDEFAULT = %d, want the configured 1500", got)
	}

	if got := values[pql("WSEXTENT")]; got != 16400 {
		t.Errorf("WSEXTENT = %d, want the rule's default 16400", got)
	}

	values, _, _ = resolveQuotas(&sg, creator, []QuotaItem{{pql("WSQUOTA"), 3000}}, false, false)
	if got := values[pql("WSQUOTA")]; got != 3000 {
		t.Errorf("with a list item, WSQUOTA = %d, want 3000", got)
	}

	sg.PQL["WSQUOTA"] = 10 // below PQL_MWSQUOTA, 1024

	values, _, _ = resolveQuotas(&sg, creator, nil, false, false)
	if got := values[pql("WSQUOTA")]; got != 1024 {
		t.Errorf("with a configured default below the minimum, WSQUOTA = %d, want 1024", got)
	}

	values, _, _ = resolveQuotas(nil, creator, nil, false, false)
	if got := values[pql("WSQUOTA")]; got != 1024 {
		t.Errorf("with no Sysgen, WSQUOTA = %d, want the rule's 588 raised to its minimum, 1024", got)
	}
}

// TestLoginQuotas: a logging-in process gets its account's values, with
// configured PQL parameters laid over them; the default can't exceed the
// quota, the limit starts at the default, and MINWSCNT is the system's.
func TestLoginQuotas(t *testing.T) {
	p := NewProcess()
	p.Job = newJob(p.PID)

	sg := DefaultSysgen()
	sg.LoginQuotas(p, SystemAccount())

	if p.WSDefault != 512 || p.WSLimit != 512 || p.WSQuota != 1024 || p.WSExtent != 16400 || p.Job.Pooled.PGFLQUOTA != 40960 {
		t.Errorf("SYSTEM with nothing configured = default %d, limit %d, quota %d, extent %d, PGFLQUOTA %d",
			p.WSDefault, p.WSLimit, p.WSQuota, p.WSExtent, p.Job.Pooled.PGFLQUOTA)
	}

	sg = Sysgen{MinWSCount: 40, PQL: map[string]uint32{"WSQUOTA": 400, "PGFLQUOTA": 9000}}
	sg.LoginQuotas(p, SystemAccount())

	if p.WSQuota != 400 || p.WSDefault != 400 || p.WSLimit != 400 {
		t.Errorf("with WSQUOTA 400 configured: quota %d, default %d, limit %d; want 400 each (the default held to the quota)",
			p.WSQuota, p.WSDefault, p.WSLimit)
	}

	if p.WSAuth != 400 || p.WSAuthExt != 16400 {
		t.Errorf("WSAUTH %d, WSAUTHEXT %d; want the quota and extent, 400 and 16400", p.WSAuth, p.WSAuthExt)
	}

	if p.Job.Pooled.PGFLQUOTA != 9000 || p.MinWSCount != 40 || p.WSExtent != 16400 {
		t.Errorf("PGFLQUOTA %d, MinWSCount %d, WSEXTENT %d; want 9000, 40, 16400",
			p.Job.Pooled.PGFLQUOTA, p.MinWSCount, p.WSExtent)
	}
}

// TestNewEnvironmentLogsIn: process 1 starts from the System's Sysgen.
func TestNewEnvironmentLogsIn(t *testing.T) {
	base, _ := fixture()
	sys := NewSystem(base.cpu, base.mem, base.Devices, base.Mounts)
	sys.SetSysgen(Sysgen{MinWSCount: 30, PQL: map[string]uint32{"WSDEFAULT": 256}})

	env, err := NewEnvironment(sys, base.Logicals, bytes.NewReader(nil), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}

	if p := env.Process; p.WSDefault != 256 || p.WSLimit != 256 || p.MinWSCount != 30 {
		t.Errorf("process 1: WSDEFAULT %d, WSLIMIT %d, MINWSCNT %d; want 256, 256, 30", p.WSDefault, p.WSLimit, p.MinWSCount)
	}
}
