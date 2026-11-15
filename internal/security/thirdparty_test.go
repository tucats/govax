package security

import "testing"

// TestThirdPartyInterpreted holds ThirdParty's Interpreted rules to
// Check: every file of the probe's matrix, every identity with each
// privilege that matters, every access, must get the same answer.
func TestThirdPartyInterpreted(t *testing.T) {
	privs := []uint64{0, PrivSYSPRV, PrivGRPPRV, PrivBYPASS, PrivREADALL, PrivSYSPRV | PrivBYPASS}
	uics := []UIC{uicSystem, uicA, uicB, uicC, NewUIC(0o10, 1), NewUIC(0o11, 1)}
	wants := []Access{0, Read, Write, Execute, Delete, Control, Read | Write, All}

	for _, f := range probeFiles {
		code, err := ParseProtection(f.prot, NoAccess)
		if err != nil {
			t.Fatal(err)
		}

		p := Profile{Owner: f.owner, Protection: code, ACL: f.acl}

		for _, u := range uics {
			for _, pv := range privs {
				c := Credentials{UIC: u, Privileges: pv, Rights: []Identifier{{Value: idInteract}}}
				if u == uicC {
					c.Rights = append(c.Rights, Identifier{Value: idTeam})
				}

				for _, w := range wants {
					want := Check(c, p, w)
					got := ThirdParty(c, p, w, Interpreted, false)

					if got.Granted != want.Granted || got.ACE != want.ACE {
						t.Errorf("%s %v privs %#x want %v: got %v ACE %d, Check %v ACE %d",
							f.name, u, pv, w, got.Granted, got.ACE, want.Granted, want.ACE)
					}

					if !got.Granted && got.Used != 0 {
						t.Errorf("%s %v: refused with privileges used %v", f.name, u, got.Used)
					}
				}
			}
		}
	}
}

// TestThirdPartyCompared is $CHKPRO's rules, from round 3's part K
// (testdata/prot93/vax/round3/sec93.log, cases 235 to 285 and 503).
func TestThirdPartyCompared(t *testing.T) {
	std, _ := ParseProtection("(S:RWED,O:RWED,G:R,W)", NoAccess)
	open, _ := ParseProtection("(S:RWED,O:RWED,G,W)", NoAccess)

	cases := []struct {
		name    string
		c       Credentials
		p       Profile
		want    Access
		readAll bool
		granted bool
		used    Used
		ace     int
	}{
		{"235 owner READ", Credentials{UIC: uicA}, Profile{Owner: uicA, Protection: std}, Read, false, true, 0, -1},
		{"238 group WRITE", Credentials{UIC: uicB}, Profile{Owner: uicA, Protection: std}, Write, false, false, 0, -1},
		{"239 world READ", Credentials{UIC: uicC}, Profile{Owner: uicA, Protection: std}, Read, false, false, 0, -1},
		{"241 system [10,1]", Credentials{UIC: NewUIC(0o10, 1)}, Profile{Owner: uicA, Protection: std}, Read, false, true, UsedSYSPRV, -1},
		{"242 [11,1]", Credentials{UIC: NewUIC(0o11, 1)}, Profile{Owner: uicA, Protection: std}, Read, false, false, 0, -1},
		{"246 READALL", Credentials{UIC: uicC, Privileges: PrivREADALL}, Profile{Owner: uicA, Protection: std}, Read, false, false, 0, -1},
		{"248 READALL, USEREADALL", Credentials{UIC: uicC, Privileges: PrivREADALL}, Profile{Owner: uicA, Protection: std}, Read, true, true, UsedREADALL, -1},
		{"250 GRPPRV", Credentials{UIC: uicB, Privileges: PrivGRPPRV}, Profile{Owner: uicA, Protection: open}, Read | Write, false, true, UsedGRPPRV, -1},
		{"251 world GRPPRV", Credentials{UIC: uicC, Privileges: PrivGRPPRV}, Profile{Owner: uicA, Protection: std}, Read, false, false, 0, -1},
		{"252 group CONTROL", Credentials{UIC: uicB}, Profile{Owner: uicA, Protection: std}, Control, false, true, 0, -1},
		{"256 owner [0,0]", Credentials{UIC: uicC}, Profile{Protection: NoAccess}, Read, false, true, 0, -1},
		{"263 ACL denies the owner", Credentials{UIC: uicA}, Profile{Owner: uicA, Protection: std, ACL: grant(uint32(uicA), 0)}, Read, false, true, 0, 0},
		{"265 ACL CONTROL", Credentials{UIC: uicC}, Profile{Owner: uicA, Protection: std, ACL: grant(uint32(uicC), Control)}, Control, false, true, 0, 0},
		{"271 SYSPRV and BYPASS", Credentials{UIC: uicC, Privileges: PrivSYSPRV | PrivBYPASS}, Profile{Owner: uicA, Protection: std}, Read, false, true, UsedSYSPRV, -1},
		{"245 BYPASS", Credentials{UIC: uicC, Privileges: PrivBYPASS}, Profile{Owner: uicA, Protection: std}, Read, false, true, UsedBYPASS, -1},
		{"270 a DEFAULT ACE", Credentials{UIC: uicC}, Profile{Owner: uicA, Protection: std,
			ACL: []ACE{NewIdentifierACE([]uint32{uint32(uicC)}, Read, OptionDefault)}}, Read, false, false, 0, -1},
	}

	for _, tc := range cases {
		got := ThirdParty(tc.c, tc.p, tc.want, Compared, tc.readAll)
		if got.Granted != tc.granted || got.Used != tc.used || got.ACE != tc.ace {
			t.Errorf("%s: got %+v, want granted %v used %v ACE %d", tc.name, got, tc.granted, tc.used, tc.ace)
		}
	}
}
