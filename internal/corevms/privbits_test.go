package corevms

// The privileges' bits, for the tests: the code names a privilege only
// through a rule (privrules.go).
var (
	privCMKRNL  = privilegeBit("CMKRNL")
	privCMEXEC  = privilegeBit("CMEXEC")
	privSYSNAM  = privilegeBit("SYSNAM")
	privGRPNAM  = privilegeBit("GRPNAM")
	privSYSPRV  = privilegeBit("SYSPRV")
	privPRMCEB  = privilegeBit("PRMCEB")
	privPRMMBX  = privilegeBit("PRMMBX")
	privTMPMBX  = privilegeBit("TMPMBX")
	privPSWAPM  = privilegeBit("PSWAPM")
	privALTPRI  = privilegeBit("ALTPRI")
	privOPER    = privilegeBit("OPER")
	privWORLD   = privilegeBit("WORLD")
	privGROUP   = privilegeBit("GROUP")
	privDETACH  = privilegeBit("DETACH")
	privNETMBX  = privilegeBit("NETMBX")
	privNOACNT  = privilegeBit("ACNT")
	privLOGIO   = privilegeBit("LOG_IO")
	privPHYIO   = privilegeBit("PHY_IO")
	privBYPASS  = privilegeBit("BYPASS")
	privGRPPRV  = privilegeBit("GRPPRV")
	privREADALL = privilegeBit("READALL")
)
