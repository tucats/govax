package machine

import (
	"strconv"
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"

	"github.com/tucats/govax/internal/corevms"
)

// The system parameters' keys (docs/PHASE-62 - VMS paging.md, Decision
// 11; HELP CONFIG KEYS). Each is vax.sysgen. and the VMS SYSGEN
// parameter's name in lowercase; the paging ones are govax's own. Each
// must also be in cmd/govax/main.go's validConfigs.
const (
	PagingSetting     = "vax.sysgen.paging.enable"
	minWSCountSetting = "vax.sysgen.minwscnt"
	tbSkipWSLSetting  = "vax.sysgen.tbskipwsl"
	growLimSetting    = "vax.sysgen.growlim"
	mpwHiLimitSetting = "vax.sysgen.mpw_hilimit"
	mpwLoLimitSetting = "vax.sysgen.mpw_lolimit"
	freeLimSetting    = "vax.sysgen.freelim"
	freeGoalSetting   = "vax.sysgen.freegoal"
	pageFileSetting   = "vax.sysgen.paging.file"
	pageSizeSetting   = "vax.sysgen.paging.size"
	pageDeferSetting  = "vax.sysgen.paging.defer"

	// WriteClusterSetting is MPW_WRTCLUSTER's key, whose value has a
	// rule of its own (ParseWriteCluster).
	WriteClusterSetting = "vax.sysgen.mpw_wrtcluster"
)

// pqlSettings are the PQL_Dxxx keys, by the quota each sets the default
// of (corevms.Sysgen.PQL).
var pqlSettings = map[string]string{
	"vax.sysgen.pql_dwsdefault": "WSDEFAULT",
	"vax.sysgen.pql_dwsquota":   "WSQUOTA",
	"vax.sysgen.pql_dwsextent":  "WSEXTENT",
	"vax.sysgen.pql_dpgflquota": "PGFLQUOTA",
}

// SysgenKeys returns every vax.sysgen.* key the console reads, for
// cmd/govax's validConfigs and its audit.
func SysgenKeys() []string {
	keys := []string{
		PagingSetting, minWSCountSetting, tbSkipWSLSetting, growLimSetting,
		mpwHiLimitSetting, mpwLoLimitSetting, freeLimSetting, freeGoalSetting,
		pageFileSetting, pageSizeSetting, pageDeferSetting, WriteClusterSetting,
	}

	for key := range pqlSettings {
		keys = append(keys, key)
	}

	return keys
}

// SysgenCountKeys returns the keys whose value is a count of pages, which
// must be a positive integer (cmd/govax's audit warns about any other).
func SysgenCountKeys() []string {
	keys := []string{minWSCountSetting, pageSizeSetting}

	for key := range pqlSettings {
		keys = append(keys, key)
	}

	return keys
}

// SysgenNumberKeys returns the keys whose value is a number that may be
// 0, which must be a nonnegative integer (cmd/govax's audit warns about
// any other): TBSKIPWSL, which 0 turns off; GROWLIM, which 0 lets a
// working set grow past its quota while any page is free; and the
// modified page writer's MPW_HILIMIT and MPW_LOLIMIT, which 0 has write
// whenever any page is modified, and write every page it can; and the
// reclaimer's FREELIM, which 0 turns off, and FREEGOAL.
func SysgenNumberKeys() []string {
	return []string{
		tbSkipWSLSetting, growLimSetting, mpwHiLimitSetting, mpwLoLimitSetting,
		freeLimSetting, freeGoalSetting,
	}
}

// ParseSysgenCount reads a count key's text: a positive decimal integer,
// or %X followed by hex digits, as vax.init's F$INTEGER reads the
// vax.memory.* keys. It reports false for anything else.
func ParseSysgenCount(text string) (uint32, bool) {
	v, ok := ParseSysgenNumber(text)

	return v, ok && v != 0
}

// ParseSysgenNumber reads a number key's text: ParseSysgenCount's forms,
// or 0.
func ParseSysgenNumber(text string) (uint32, bool) {
	text = strings.TrimSpace(text)
	base := 10

	if len(text) > 2 && strings.EqualFold(text[:2], "%x") {
		text, base = text[2:], 16
	}

	v, err := strconv.ParseUint(text, base, 32)
	if err != nil {
		return 0, false
	}

	return uint32(v), true
}

// ParseWriteCluster reads MPW_WRTCLUSTER's text: a number in
// ParseSysgenNumber's forms from 16 to 120 and a multiple of 8, SYSGEN's
// range for the parameter on a VAX. It reports false for anything else.
func ParseWriteCluster(text string) (uint32, bool) {
	v, ok := ParseSysgenNumber(text)

	return v, ok && v >= 16 && v <= 120 && v%8 == 0
}

// sysgenSettings reads the vax.sysgen.* keys into the parameters a new
// System gets, each key that isn't set, or isn't a positive count,
// keeping its default (cmd/govax warns about a bad value at startup).
// paging is the mode the last VMINIT chose, not the key: VMINIT's
// /[NO]PAGING may have overridden it.
func sysgenSettings(paging bool) corevms.Sysgen {
	sg := corevms.DefaultSysgen()
	sg.Paging = paging

	if v, ok := ParseSysgenCount(settings.Get(minWSCountSetting)); ok {
		sg.MinWSCount = v
	}

	if v, ok := ParseSysgenNumber(settings.Get(tbSkipWSLSetting)); ok {
		sg.TBSkipWSL = v
	}

	if v, ok := ParseSysgenNumber(settings.Get(growLimSetting)); ok {
		sg.GrowLimit = v
	}

	if v, ok := ParseSysgenNumber(settings.Get(mpwHiLimitSetting)); ok {
		sg.MPWHiLimit = v
	}

	if v, ok := ParseSysgenNumber(settings.Get(mpwLoLimitSetting)); ok {
		sg.MPWLoLimit = v
	}

	if v, ok := ParseWriteCluster(settings.Get(WriteClusterSetting)); ok {
		sg.MPWWriteCluster = v
	}

	if v, ok := ParseSysgenNumber(settings.Get(freeLimSetting)); ok {
		sg.FreeLimit = v
	}

	if v, ok := ParseSysgenNumber(settings.Get(freeGoalSetting)); ok {
		sg.FreeGoal = v
	}

	sg.PageFileName = strings.TrimSpace(settings.Get(pageFileSetting))

	if v, ok := ParseSysgenCount(settings.Get(pageSizeSetting)); ok {
		sg.PageFileSize = v
	}

	sg.PageFileDefer = settings.Exists(pageDeferSetting) && settings.GetBool(pageDeferSetting)

	for key, quota := range pqlSettings {
		if v, ok := ParseSysgenCount(settings.Get(key)); ok {
			if sg.PQL == nil {
				sg.PQL = map[string]uint32{}
			}

			sg.PQL[quota] = v
		}
	}

	return sg
}

// pagingDefault is VMINIT's paging mode when its command doesn't say:
// vax.sysgen.paging.enable, off when unset.
func pagingDefault() bool {
	return settings.Exists(PagingSetting) && settings.GetBool(PagingSetting)
}
