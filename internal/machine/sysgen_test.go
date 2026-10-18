package machine

import (
	"path/filepath"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"

	"github.com/tucats/govax/internal/corevms"
)

// TestParseSysgenCount: a count is a positive decimal integer or %X hex,
// and anything else is refused.
func TestParseSysgenCount(t *testing.T) {
	for _, tc := range []struct {
		text string
		want uint32
		ok   bool
	}{
		{"588", 588, true},
		{" 20 ", 20, true},
		{"%X400", 0x400, true},
		{"%x1f", 0x1F, true},
		{"0", 0, false},
		{"-3", 0, false},
		{"lots", 0, false},
		{"", 0, false},
		{"%X", 0, false},
	} {
		got, ok := ParseSysgenCount(tc.text)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseSysgenCount(%q) = %d, %v; want %d, %v", tc.text, got, ok, tc.want, tc.ok)
		}
	}
}

// TestParseSysgenNumber: a number key takes 0 too.
func TestParseSysgenNumber(t *testing.T) {
	for _, tc := range []struct {
		text string
		want uint32
		ok   bool
	}{
		{"0", 0, true},
		{"%X0", 0, true},
		{"8", 8, true},
		{"-1", 0, false},
		{"", 0, false},
	} {
		got, ok := ParseSysgenNumber(tc.text)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseSysgenNumber(%q) = %d, %v; want %d, %v", tc.text, got, ok, tc.want, tc.ok)
		}
	}
}

// TestSysgenTBSkipWSL: vax.sysgen.tbskipwsl reaches the System, 0
// included; unset, or bad, it's the book's 8.
func TestSysgenTBSkipWSL(t *testing.T) {
	for _, tc := range []struct {
		text string
		want uint32
	}{{"0", 0}, {"3", 3}, {"many", 8}, {"", 8}} {
		values := map[string]string{tbSkipWSLSetting: tc.text}
		withSettings(t, values)

		if tc.text == "" {
			_ = settings.Delete(tbSkipWSLSetting)
		}

		c, _ := newTestMachine(t)
		if got := c.ProcessOne().Sysgen.TBSkipWSL; got != tc.want {
			t.Errorf("vax.sysgen.tbskipwsl %q: TBSkipWSL = %d, want %d", tc.text, got, tc.want)
		}
	}
}

// TestSysgenGrowLimit: vax.sysgen.growlim reaches the System, 0
// included; unset, or bad, it's 63.
func TestSysgenGrowLimit(t *testing.T) {
	for _, tc := range []struct {
		text string
		want uint32
	}{{"0", 0}, {"%X100", 256}, {"-1", 63}, {"", 63}} {
		values := map[string]string{growLimSetting: tc.text}
		withSettings(t, values)

		if tc.text == "" {
			_ = settings.Delete(growLimSetting)
		}

		c, _ := newTestMachine(t)
		if got := c.ProcessOne().Sysgen.GrowLimit; got != tc.want {
			t.Errorf("vax.sysgen.growlim %q: GrowLimit = %d, want %d", tc.text, got, tc.want)
		}
	}
}

// TestSysgenSettings: the vax.sysgen.* keys reach the System, and process
// 1 starts from them; a bad value keeps the default.
func TestSysgenSettings(t *testing.T) {
	withSettings(t, map[string]string{
		"vax.sysgen.pql_dwsdefault": "300",
		"vax.sysgen.pql_dwsquota":   "%X258", // 600
		"vax.sysgen.pql_dwsextent":  "9000",
		"vax.sysgen.pql_dpgflquota": "20000",
		minWSCountSetting:           "bad",
	})

	c, _ := newTestMachine(t)
	env := c.ProcessOne()

	want := map[string]uint32{"WSDEFAULT": 300, "WSQUOTA": 600, "WSEXTENT": 9000, "PGFLQUOTA": 20000}
	for quota, v := range want {
		if got := env.Sysgen.PQL[quota]; got != v {
			t.Errorf("Sysgen.PQL[%s] = %d, want %d", quota, got, v)
		}
	}

	p := env.Process
	if p.WSDefault != 300 || p.WSLimit != 300 || p.WSQuota != 600 || p.WSExtent != 9000 {
		t.Errorf("process 1's working set = default %d, limit %d, quota %d, extent %d; want 300, 300, 600, 9000",
			p.WSDefault, p.WSLimit, p.WSQuota, p.WSExtent)
	}

	if p.Job.Pooled.PGFLQUOTA != 20000 {
		t.Errorf("process 1's job PGFLQUOTA = %d, want 20000", p.Job.Pooled.PGFLQUOTA)
	}

	if p.MinWSCount != 20 {
		t.Errorf("with a bad vax.sysgen.minwscnt, MinWSCount = %d, want the default 20", p.MinWSCount)
	}
}

// TestSysgenSettings_unset: with no keys, process 1 has the SYSTEM
// account's values, as before the keys existed.
func TestSysgenSettings_unset(t *testing.T) {
	withSettings(t, map[string]string{"vax.sysgen.pql_dwsquota": "1"})

	for _, key := range SysgenKeys() {
		_ = settings.Delete(key)
	}

	c, _ := newTestMachine(t)
	p := c.ProcessOne().Process

	if p.WSDefault != 512 || p.WSQuota != 1024 || p.WSExtent != 16400 || p.Job.Pooled.PGFLQUOTA != 40960 || p.MinWSCount != 20 {
		t.Errorf("process 1 = WSDEFAULT %d, WSQUOTA %d, WSEXTENT %d, PGFLQUOTA %d, MINWSCNT %d; want 512, 1024, 16400, 40960, 20",
			p.WSDefault, p.WSQuota, p.WSExtent, p.Job.Pooled.PGFLQUOTA, p.MinWSCount)
	}
}

// TestVMInitPaging: VMINIT's paging mode comes from the setting unless
// the command chooses, reaches the System's Sysgen, and lasts until the
// next VMINIT, INIT, or ZERO.
func TestVMInitPaging(t *testing.T) {
	withSettings(t, map[string]string{PagingSetting: "true"})

	c, _ := newTestMachine(t)

	if c.Paging() || c.ProcessOne().Sysgen.Paging {
		t.Fatal("paging is on after INIT, before any VMINIT")
	}

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	if !c.Paging() || !c.ProcessOne().Sysgen.Paging {
		t.Error("VMINIT with vax.sysgen.paging.enable true left paging off")
	}

	if err := c.VMInitPaging(false, 20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	if c.Paging() || c.ProcessOne().Sysgen.Paging {
		t.Error("VMINIT/NOPAGING left paging on")
	}

	if err := c.VMInitPaging(true, 20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInitPaging: %v", err)
	}

	if err := c.Zero(); err != nil {
		t.Fatalf("Zero: %v", err)
	}

	if c.Paging() {
		t.Error("ZERO left paging on")
	}

	_ = settings.Delete(PagingSetting)

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	if c.Paging() {
		t.Error("VMINIT with no setting turned paging on")
	}
}

// TestSysgenModifiedPageWriter: vax.sysgen.mpw_hilimit and mpw_lolimit
// reach the System, 0 included; unset, or bad, they're 500 and 32.
func TestSysgenModifiedPageWriter(t *testing.T) {
	for _, tc := range []struct {
		hi, lo         string
		wantHi, wantLo uint32
	}{{"0", "0", 0, 0}, {"%X40", "16", 64, 16}, {"lots", "-3", 500, 32}, {"", "", 500, 32}} {
		values := map[string]string{mpwHiLimitSetting: tc.hi, mpwLoLimitSetting: tc.lo}
		withSettings(t, values)

		if tc.hi == "" {
			_ = settings.Delete(mpwHiLimitSetting)
			_ = settings.Delete(mpwLoLimitSetting)
		}

		c, _ := newTestMachine(t)
		if sg := c.ProcessOne().Sysgen; sg.MPWHiLimit != tc.wantHi || sg.MPWLoLimit != tc.wantLo {
			t.Errorf("vax.sysgen.mpw_hilimit %q, mpw_lolimit %q: %d and %d, want %d and %d",
				tc.hi, tc.lo, sg.MPWHiLimit, sg.MPWLoLimit, tc.wantHi, tc.wantLo)
		}
	}
}

// TestSysgenWriteCluster: vax.sysgen.mpw_wrtcluster reaches the System
// when it's a multiple of 8 from 16 to 120; unset, or anything else, it's
// 120.
func TestSysgenWriteCluster(t *testing.T) {
	for _, tc := range []struct {
		text string
		want uint32
	}{{"16", 16}, {"%X40", 64}, {"120", 120}, {"8", 120}, {"128", 120}, {"20", 120}, {"0", 120}, {"many", 120}, {"", 120}} {
		withSettings(t, map[string]string{WriteClusterSetting: tc.text})

		if tc.text == "" {
			_ = settings.Delete(WriteClusterSetting)
		}

		c, _ := newTestMachine(t)
		if got := c.ProcessOne().Sysgen.MPWWriteCluster; got != tc.want {
			t.Errorf("vax.sysgen.mpw_wrtcluster %q: %d, want %d", tc.text, got, tc.want)
		}
	}
}

// TestSysgenPFCDefault: vax.sysgen.pfcdefault reaches the System from 0
// to 127; unset, or anything else, it's 32.
func TestSysgenPFCDefault(t *testing.T) {
	for _, tc := range []struct {
		text string
		want uint32
	}{{"0", 0}, {"1", 1}, {"%X10", 16}, {"127", 127}, {"128", 32}, {"-1", 32}, {"many", 32}, {"", 32}} {
		withSettings(t, map[string]string{PFCDefaultSetting: tc.text})

		if tc.text == "" {
			_ = settings.Delete(PFCDefaultSetting)
		}

		c, _ := newTestMachine(t)
		if got := c.ProcessOne().Sysgen.PFCDefault; got != tc.want {
			t.Errorf("vax.sysgen.pfcdefault %q: %d, want %d", tc.text, got, tc.want)
		}
	}
}

// TestSysgenReclaimer: vax.sysgen.freelim and freegoal reach the System,
// 0 included; unset, or bad, they're 32 and 200.
func TestSysgenReclaimer(t *testing.T) {
	for _, tc := range []struct {
		lim, goal         string
		wantLim, wantGoal uint32
	}{{"0", "0", 0, 0}, {"%X40", "300", 64, 300}, {"many", "-1", 32, 200}, {"", "", 32, 200}} {
		values := map[string]string{freeLimSetting: tc.lim, freeGoalSetting: tc.goal}
		withSettings(t, values)

		if tc.lim == "" {
			_ = settings.Delete(freeLimSetting)
			_ = settings.Delete(freeGoalSetting)
		}

		c, _ := newTestMachine(t)
		if sg := c.ProcessOne().Sysgen; sg.FreeLimit != tc.wantLim || sg.FreeGoal != tc.wantGoal {
			t.Errorf("vax.sysgen.freelim %q, freegoal %q: %d and %d, want %d and %d",
				tc.lim, tc.goal, sg.FreeLimit, sg.FreeGoal, tc.wantLim, tc.wantGoal)
		}
	}
}

// TestSysgenPageFile: vax.sysgen.paging.file and paging.size reach the
// System; unset, there's no page file, of the default size.
func TestSysgenPageFile(t *testing.T) {
	c, _ := newTestMachine(t)
	if sg := c.ProcessOne().Sysgen; sg.PageFileName != "" || sg.PageFileSize != corevms.DefaultPageFileSize {
		t.Errorf("unset: page file %q of %d blocks, want none of %d", sg.PageFileName, sg.PageFileSize, corevms.DefaultPageFileSize)
	}

	withSettings(t, map[string]string{pageFileSetting: ` "./pagefile.sys" `, pageSizeSetting: "%X1000"})

	c, _ = newTestMachine(t)
	if sg := c.ProcessOne().Sysgen; sg.PageFileName != `"./pagefile.sys"` || sg.PageFileSize != 0x1000 {
		t.Errorf("page file %q of %d blocks, want \"./pagefile.sys\" of 4096", sg.PageFileName, sg.PageFileSize)
	}
}

// TestVMInitClosesPageFile: a new System (VMINIT here) closes the old
// one's page file first (the phase doc's finding 16).
func TestVMInitClosesPageFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pagefile.sys")
	withSettings(t, map[string]string{pageFileSetting: `"` + path + `"`, pageSizeSetting: "50"})

	c, out := newTestMachine(t)
	if err := c.VMInitPaging(true, 64, 64, 64, 4, 0, 0, 0, 0); err != nil {
		t.Fatal(err)
	}

	old := c.ProcessOne()
	if old.OpenPageFile() == nil {
		t.Fatalf("the page file didn't open: %q", out)
	}

	if err := c.VMInitPaging(true, 64, 64, 64, 4, 0, 0, 0, 0); err != nil {
		t.Fatal(err)
	}

	if old.PageFile() != nil {
		t.Error("the old System's page file is still open")
	}

	if c.ProcessOne().PageFile() != nil {
		t.Error("the new System's page file is open before it's needed")
	}

	if c.ProcessOne().OpenPageFile() == nil {
		t.Errorf("the new System's page file didn't open: %q", out)
	}
}

// TestStartPageFile: StartPageFile installs process 1's page file, and
// from then on each new System (VMINIT here) installs its own as it's
// made; vax.sysgen.paging.defer leaves it for the pager's first need,
// and with paging off there's none.
func TestStartPageFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pagefile.sys")
	withSettings(t, map[string]string{pageFileSetting: `"` + path + `"`, pageSizeSetting: "50"})

	c, out := newTestMachine(t)
	if err := c.VMInitPaging(true, 64, 64, 64, 4, 0, 0, 0, 0); err != nil {
		t.Fatal(err)
	}

	if c.ProcessOne().PageFile() != nil {
		t.Fatal("the page file is open before the system has started")
	}

	c.StartPageFile()

	if pf := c.ProcessOne().PageFile(); pf == nil || pf.Size() != 50 {
		t.Fatalf("StartPageFile didn't install a 50-block page file: %q", out)
	}

	if err := c.VMInitPaging(true, 64, 64, 64, 4, 0, 0, 0, 0); err != nil {
		t.Fatal(err)
	}

	if c.ProcessOne().PageFile() == nil {
		t.Errorf("VMINIT's new System didn't install its page file: %q", out)
	}

	if err := c.VMInitPaging(false, 64, 64, 64, 4, 0, 0, 0, 0); err != nil {
		t.Fatal(err)
	}

	if c.ProcessOne().PageFile() != nil {
		t.Error("a page file was installed with paging off")
	}

	withSettings(t, map[string]string{pageDeferSetting: "true"})

	if err := c.VMInitPaging(true, 64, 64, 64, 4, 0, 0, 0, 0); err != nil {
		t.Fatal(err)
	}

	if !c.ProcessOne().Sysgen.PageFileDefer || c.ProcessOne().PageFile() != nil {
		t.Error("vax.sysgen.paging.defer didn't defer the page file")
	}

	if c.ProcessOne().OpenPageFile() == nil {
		t.Errorf("the deferred page file didn't open at need: %q", out)
	}
}
