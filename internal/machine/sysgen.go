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
	keys := []string{PagingSetting, minWSCountSetting}

	for key := range pqlSettings {
		keys = append(keys, key)
	}

	return keys
}

// SysgenCountKeys returns the keys whose value is a count of pages, which
// must be a positive integer (cmd/govax's audit warns about any other).
func SysgenCountKeys() []string {
	keys := []string{minWSCountSetting}

	for key := range pqlSettings {
		keys = append(keys, key)
	}

	return keys
}

// ParseSysgenCount reads a count key's text: a positive decimal integer,
// or %X followed by hex digits, as vax.init's F$INTEGER reads the
// vax.memory.* keys. It reports false for anything else.
func ParseSysgenCount(text string) (uint32, bool) {
	text = strings.TrimSpace(text)
	base := 10

	if len(text) > 2 && strings.EqualFold(text[:2], "%x") {
		text, base = text[2:], 16
	}

	v, err := strconv.ParseUint(text, base, 32)
	if err != nil || v == 0 {
		return 0, false
	}

	return uint32(v), true
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
