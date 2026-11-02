package vmsdef

// strMessages are the STR facility's messages, as VMS 7.1's $GETMSG returned
// them for each STR$_ status (testdata/str83, the GETMSG cases), keyed as Messages
// is, without the severity bits. STR$_NORMAL is under its facility's code, ^X240001;
// the symbol's own value is 1, SS$_NORMAL's, which is what a routine returns.
var strMessages = map[uint32]Message{
	0x00240000: {Facility: "STR", Ident: "NORMAL", Text: "normal successful completion"},
	0x00248040: {Facility: "STR", Ident: "FATINTERR", Text: "fatal internal error"},
	0x00248048: {Facility: "STR", Ident: "DIVBY_ZER", Text: "division by zero"},
	0x00248050: {Facility: "STR", Ident: "ILLSTRCLA", Text: "illegal string class"},
	0x00248058: {Facility: "STR", Ident: "STRIS_INT", Text: "string is interlocked"},
	0x00248060: {Facility: "STR", Ident: "WRONUMARG", Text: "wrong number of arguments, !UB, to !AS", FAOCount: 2},
	0x00248068: {Facility: "STR", Ident: "INSVIRMEM", Text: "insufficient virtual memory"},
	0x00248070: {Facility: "STR", Ident: "STRTOOLON", Text: "string is too long (greater than 65535)"},
	0x00248078: {Facility: "STR", Ident: "ERRFREDYN", Text: "error freeing dynamic string when returned to LIB$FREE_VM"},
	0x00248200: {Facility: "STR", Ident: "TRU", Text: "truncation"},
	0x00248208: {Facility: "STR", Ident: "NOMATCH", Text: "no match found against input string"},
	0x00248210: {Facility: "STR", Ident: "INVDELIM", Text: "invalid delimiter"},
	0x00248218: {Facility: "STR", Ident: "NOELEM", Text: "no element found"},
	0x00248400: {Facility: "STR", Ident: "NEGSTRLEN", Text: "negative string length"},
	0x00248408: {Facility: "STR", Ident: "ILLSTRPOS", Text: "illegal string position"},
	0x00248410: {Facility: "STR", Ident: "ILLSTRSPE", Text: "illegal string specification"},
	0x00248418: {Facility: "STR", Ident: "MATCH", Text: "match found against input string"},
}
