package corevms

import (
	"slices"
	"testing"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 93's subtask 5: the process's default protection, $SETDFPROT, and
// the credentials RMS and the DCL utilities are given.

// wordAt reads the word at addr.
func wordAt(t *testing.T, env *Environment, addr uint32) uint16 {
	t.Helper()

	v, err := env.mem.LoadWord(env.cpu, addr)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

func TestSetdfprot(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process

	if p.DefaultProtection != DefaultFileProtection {
		t.Fatalf("default protection %#04x, want %%XFA00", p.DefaultProtection)
	}

	newWord, curWord := a.alloc(2), a.alloc(2)
	putWord(t, env, newWord, 0xAF00)

	// The old code to cur-def-prot-addr, the new from new-def-prot-addr.
	wantR0(t, callLNM(t, env, serviceSysSetdfprot, newWord, curWord), rmsNormal)

	if p.DefaultProtection != 0xAF00 || wordAt(t, env, curWord) != 0xFA00 {
		t.Errorf("default %#04x, old %#04x; want %%XAF00, %%XFA00", p.DefaultProtection, wordAt(t, env, curWord))
	}

	// One word for both trades the codes.
	putWord(t, env, newWord, 0xFA00)
	wantR0(t, callLNM(t, env, serviceSysSetdfprot, newWord, newWord), rmsNormal)

	if p.DefaultProtection != 0xFA00 || wordAt(t, env, newWord) != 0xAF00 {
		t.Errorf("the same word: default %#04x, word %#04x; want %%XFA00, %%XAF00", p.DefaultProtection, wordAt(t, env, newWord))
	}

	// Only reading it, and nothing at all.
	wantR0(t, callLNM(t, env, serviceSysSetdfprot, 0, curWord), rmsNormal)
	wantR0(t, callLNM(t, env, serviceSysSetdfprot), rmsNormal)

	if p.DefaultProtection != 0xFA00 || wordAt(t, env, curWord) != 0xFA00 {
		t.Errorf("reading: default %#04x, word %#04x", p.DefaultProtection, wordAt(t, env, curWord))
	}

	// A word it can't reach changes nothing.
	putWord(t, env, newWord, 0x1234)
	wantR0(t, callLNM(t, env, serviceSysSetdfprot, badAddr), rmsIAL)
	wantR0(t, callLNM(t, env, serviceSysSetdfprot, newWord, badAddr), rmsIAL)

	if p.DefaultProtection != 0xFA00 {
		t.Errorf("a failed call changed the default to %#04x", p.DefaultProtection)
	}
}

// TestLoginProtection: a login takes RMS_FILEPROT and MAXSYSGROUP from the
// Sysgen, and the credentials carry them with the process's UIC,
// privileges, and rights.
func TestLoginProtection(t *testing.T) {
	p := NewProcess()
	p.Job = newJob(p.PID)

	sg := DefaultSysgen()
	sg.LoginQuotas(p, SystemAccount())

	if p.DefaultProtection != 0xFA00 || p.MaxSysGroup != 0o10 {
		t.Errorf("defaults: protection %#04x, MAXSYSGROUP %o; want %%XFA00, 10", p.DefaultProtection, p.MaxSysGroup)
	}

	sg.FileProtection, sg.MaxSysGroup = 0xFF00, 0o20
	sg.LoginQuotas(p, SystemAccount())

	creds := p.Credentials()

	ids := []uint32{vmsdef.Symbols["KGB$K_INTERACTIVE_ID"], vmsdef.Symbols["KGB$K_LOCAL_ID"]}
	if !slices.Equal([]uint32{creds.Rights[0].Value, creds.Rights[1].Value}, ids) || len(creds.Rights) != 2 {
		t.Errorf("rights %+v, want INTERACTIVE and LOCAL", creds.Rights)
	}

	if p.DefaultProtection != 0xFF00 || creds.MaxSysGroup != 0o20 || creds.UIC != security.SystemUIC ||
		creds.Privileges != allPrivileges {
		t.Errorf("configured: protection %#04x, credentials %+v", p.DefaultProtection, creds)
	}

	// MAXSYSGROUP decides the system category: [20,1] is a system UIC
	// under 20 and not under the usual 10.
	p.UIC = uint32(security.NewUIC(0o20, 1))
	if !p.Credentials().InSystemGroup() {
		t.Error("[20,1] isn't a system UIC with MAXSYSGROUP 20")
	}

	p.MaxSysGroup = security.DefaultMaxSysGroup
	if p.Credentials().InSystemGroup() {
		t.Error("[20,1] is a system UIC with MAXSYSGROUP 10")
	}

	// The credentials are a copy: changing them changes nothing.
	creds.Rights[0].Value = 0
	if p.Rights[0].Value == 0 {
		t.Error("the credentials share the process's rights")
	}
}

// TestCredentialsReachRMS: the RMS context of each call, and the session a
// process is given, ask the process for its credentials each time, so a
// privilege turned off counts at once; with none, they are SYSTEM's.
func TestCredentialsReachRMS(t *testing.T) {
	env, _ := fixture()
	session := rms.NewSession(env.Mounts)

	if got := session.Identity(); got.UIC != security.SystemUIC || got.Privileges != security.AllPrivileges {
		t.Errorf("a session with no process: %+v, want SYSTEM's", got)
	}

	env.UseSession(session)
	env.Process.UIC = uint32(security.NewUIC(0o200, 1))
	env.Process.CurrentPrivileges &^= security.PrivBYPASS

	for name, got := range map[string]security.Credentials{
		"context": env.rmsContext().Identity(),
		"session": session.Identity(),
	} {
		if got.UIC != security.NewUIC(0o200, 1) || got.Has(security.PrivBYPASS) {
			t.Errorf("%s: %+v, want [200,1] without BYPASS", name, got)
		}
	}

	if (&rms.Context{}).Identity().UIC != security.SystemUIC {
		t.Error("a context with no credentials isn't SYSTEM's")
	}
}
