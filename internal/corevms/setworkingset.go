package corevms

// WorkingSetChange is what DCL's SET WORKING_SET asks for: each field
// is nil when its qualifier is absent. Limit is /LIMIT (the default
// working set, DFWSCNT), Quota /QUOTA (WSQUOTA), Extent /EXTENT
// (WSEXTENT), and Adjust /[NO]ADJUST.
type WorkingSetChange struct {
	Limit, Quota, Extent *uint32
	Adjust               *bool
}

// WorkingSetValues are a process's working set values as SHOW
// WORKING_SET and SET WORKING_SET's message report them: the default
// (DFWSCNT, which SHOW calls the limit), the quota and extent, the
// authorized quota and extent, and whether automatic adjustment is on.
type WorkingSetValues struct {
	Limit, Quota, Extent uint32
	AuthQuota, AuthExt   uint32
	Adjust               bool
}

// WorkingSetValues returns the process's working set values.
func (env *Environment) WorkingSetValues() WorkingSetValues {
	p := env.Process

	return WorkingSetValues{
		Limit:     p.WSDefault,
		Quota:     p.WSQuota,
		Extent:    p.WSExtent,
		AuthQuota: p.WSAuth,
		AuthExt:   p.WSAuthExt,
		Adjust:    !p.WSAdjustOff,
	}
}

// SetWorkingSet makes the changes SET WORKING_SET asks for to the
// process's own values, within what it was authorized at creation
// (WSAuth, WSAuthExt), and returns the values that result and whether
// any value asked for was cut back to a bound.
//
// The rules (the DCL Dictionary, and the book's 16.4.1.2):
//
//   - /EXTENT is at most the authorized extent and must be more than
//     MINWSCNT; a smaller one is raised to MINWSCNT + 1 (govax's choice,
//     unconfirmed: the Dictionary says only that it must be greater).
//   - /QUOTA is at most the authorized quota, and at most the extent.
//   - /QUOTA and /LIMIT are at least MINWSCNT, the least a working set
//     may be (unconfirmed).
//   - /LIMIT (the default working set) above the quota raises the quota
//     too, as far as the authorized quota; the limit is then at most the
//     quota (unconfirmed: the Dictionary doesn't say what a limit above
//     the authorized quota does).
//   - A smaller extent or quota takes the values below it down with it.
//
// The current working-set limit (WSLIMIT, JPI$_WSSIZE) is then set to
// the default, as image exit sets it: DCL runs between images, when the
// limit is the default anyway (unconfirmed for VMS, whose DCL is an
// image of the process). With paging on, a smaller list gives up its
// oldest pages.
func (env *Environment) SetWorkingSet(change WorkingSetChange) (WorkingSetValues, bool) {
	p := env.Process
	clamped := false

	bound := func(v, lo, hi uint32) uint32 {
		switch {
		case v > hi:
			clamped = true

			return hi
		case v < lo:
			clamped = true

			return lo
		}

		return v
	}

	if change.Extent != nil {
		p.WSExtent = bound(*change.Extent, p.MinWSCount+1, max(p.WSAuthExt, p.MinWSCount+1))
	}

	if change.Quota != nil {
		p.WSQuota = bound(*change.Quota, p.MinWSCount, min(p.WSAuth, p.WSExtent))
	}

	if change.Limit != nil {
		limit := bound(*change.Limit, p.MinWSCount, p.WSAuthExt)
		if limit > p.WSQuota {
			p.WSQuota = bound(limit, 0, min(p.WSAuth, p.WSExtent))
		}

		p.WSDefault = min(limit, p.WSQuota)
	}

	// A lowered extent or quota brings the values under it down.
	p.WSQuota = min(p.WSQuota, p.WSExtent)
	p.WSDefault = min(p.WSDefault, p.WSQuota)

	if change.Adjust != nil {
		p.WSAdjustOff = !*change.Adjust
	}

	env.setWorkingSetLimit(int64(p.WSDefault))

	return env.WorkingSetValues(), clamped
}
