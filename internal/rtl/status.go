package rtl

// VMS system-service status codes this package's handlers actually return,
// matching ss_def.h. Not the full ~1000-entry VMS status code set — just the
// subset the ported C source uses; add more here as later handlers need them
// (the same table-driven-extensibility spirit as p1Vector/the shim/service
// registries, just as plain constants rather than a registry).
const (
	ssNormal      = 1
	ssWasClr      = 1
	ssWasSet      = 9
	ssAccVio      = 12
	ssBadParam    = 20
	ssIvChan      = 316
	ssIvDevNam    = 324
	ssIvLogNam    = 340
	ssIvLogTab    = 348
	ssNoLogNam    = 444
	ssInsfMem     = 292
	ssInsfArg     = 276
	ssNoSuchDev   = 2312
	ssNoSuchFac   = 9276
	ssNoSuchFile  = 2320
	ssInvArg      = 4042
	ssTooManyArgs = 10060
	ssNoLogTab    = 8852
)
