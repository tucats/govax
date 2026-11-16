package vmsdef

// JBCFacility is the job controller's facility number, the JBC facility's
// (JBC$_FACILITY, testdata/bat98/defined.txt): the queue services' and the
// queue commands' conditions.
const JBCFacility = 4

// jbcMessages are the JBC facility's own messages (those whose
// STS$V_FAC_SP bit is set), keyed as Messages is, without the severity
// bits. Each text is one VMS 7.1 printed for the condition in Phase 98's
// probe (testdata/bat98/vax: the queue commands' messages, and BAT98Q's
// IOSBs shown by $GETMSG), and the values are $JBCMSGDEF's
// (defined.txt). The probe's $GETMSG dump reached only message numbers 0
// to 1,023, the facility's shared part, where VMS gives the system
// messages under JBC's name (sharedMessage); the rest of $JBCMSGDEF's
// conditions have no text here until a run shows one.
var jbcMessages = map[uint32]Message{
	0x00048018: {Facility: "JBC", Ident: "INVQUENAM", Text: "invalid queue name"},
	0x00048020: {Facility: "JBC", Ident: "NOPRIV", Text: "insufficient privilege or queue protection violation"},
	0x00048038: {Facility: "JBC", Ident: "NOSUCHQUE", Text: "no such queue"},
	0x00048040: {Facility: "JBC", Ident: "NOSUCHJOB", Text: "no such job"},
	0x00048058: {Facility: "JBC", Ident: "STARTED", Text: "queue already started"},
	0x00048080: {Facility: "JBC", Ident: "JOBABORT", Text: "job aborted during execution"},
	0x00048090: {Facility: "JBC", Ident: "INVPARLEN", Text: "invalid parameter length"},
	0x000480C0: {Facility: "JBC", Ident: "NODSTQUE", Text: "no such destination queue"},
	0x000480D0: {Facility: "JBC", Ident: "JOBDELETE", Text: "job deleted before execution"},
	0x000480E8: {Facility: "JBC", Ident: "QUERESET", Text: "job aborted during execution because queue was reset"},
	0x000480F8: {Facility: "JBC", Ident: "EXECUTING", Text: "job executing on queue prevents requested operation"},
	0x00048108: {Facility: "JBC", Ident: "MISREQPAR", Text: "missing required parameter"},
	0x00048170: {Facility: "JBC", Ident: "NOSUCHFILE", Text: "no such file"},
	0x00048180: {Facility: "JBC", Ident: "QUENOTSTOP", Text: "queue must be stopped to perform operation"},
	0x000481A0: {Facility: "JBC", Ident: "NOMOREJOB", Text: "no more jobs found"},
	0x000481A8: {Facility: "JBC", Ident: "NOMOREQUE", Text: "no more queues found"},
	0x000481B0: {Facility: "JBC", Ident: "NOJOBCTX", Text: "no job context"},
	0x000481C0: {Facility: "JBC", Ident: "NOSUCHNODE", Text: "remote node is unknown"},
	0x000481D8: {Facility: "JBC", Ident: "NOSUCHENT", Text: "no such entry"},
	0x00048220: {Facility: "JBC", Ident: "QUENOTMOD", Text: "modifications not made to running queue"},
	0x00048248: {Facility: "JBC", Ident: "JOBNOTEXEC", Text: "specified job is not executing"},
	0x00048278: {Facility: "JBC", Ident: "PRIOSMALL", Text: "scheduling priority has smaller value than requested"},
}
