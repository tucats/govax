//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, All Rights Reserved
//
//  Program:    eVAX, a "Virtual VAX"
//
//  Author:     Tom Cole
//
//  Module:     p1_vector.c
//
//  Purpose:    This handles the P1 system service vector for running VMS
//              programs.  It can generate the P1 vector, and generates the
//              symbol table entries required to allow direct calls.
//
//  History:    11/08/99        Created.
//

#include "vax.h"
#include "vaxinstr.h"
#include "asmproto.h"
#include "shim.h"
#include "services.h"

void declare_services( void );

long service_count = 0;

struct P1_VECTOR {
    char * name;
    LONGWORD addr;
    CALLV service;
    int	jmp;
} p1_vector[] = {

{ "SYS$ABORT_RU",           0x7FFEE640, 0L, 0 },
{ "SYS$ABORT_TRANS",        0x7FFEE6E0, 0L, 0 },
{ "SYS$ABORT_TRANSW",       0x7FFEE738, 0L, 0 },
{ "SYS$ACK_EVENT",          0x7FFEE838, 0L, 0 },
{ "SYS$ADD_BRANCH",         0x7FFEE708, 0L, 0 },
{ "SYS$ADD_BRANCHW",        0x7FFEE760, 0L, 0 },
{ "SYS$ADJSTK",             0x7FFEDE20, 0L, 0 },
{ "SYS$ADJWSL",             0x7FFEDE28, 0L, 0 },
{ "SYS$ALCDNP",             0x7FFEDE30, 0L, 0 },
{ "SYS$ALLOC",              0x7FFEDE38, 0L, 0 },
{ "SYS$ASCEFC",             0x7FFEDE40, 0L, 0 },
{ "SYS$ASCTIM",             0x7FFEDE48, 0L, 0 },
{ "SYS$ASCTOID",            0x7FFEE4B0, 0L, 0 },
{ "SYS$ASCUTC",             0x7FFEE590, 0L, 0 },
{ "SYS$ASSIGN",             0x7FFEDE50, 0L, 0 },
{ "SYS$AUDIT_EVENT",        0x7FFEE8A8, 0L, 0 },
{ "SYS$AUDIT_EVENTW",       0x7FFEE8B0, 0L, 0 },
{ "SYS$AUDIT_EVENTW_2",     0x7FFEE8B8, 0L, 0 },
{ "SYS$AUDIT_EVENTW_3",     0x7FFEE8C0, 0L, 0 },
{ "SYS$BINTIM",             0x7FFEDE58, 0L, 0 },
{ "SYS$BINUTC",             0x7FFEE588, 0L, 0 },
{ "SYS$BRDCST",             0x7FFEE098, 0L, 0 },
{ "SYS$BRKTHRU",            0x7FFEE4C8, 0L, 0 },
{ "SYS$BRKTHRUW",           0x7FFEE4E8, 0L, 0 },
{ "SYS$BRKTHRUW_2",         0x7FFEE4F0, 0L, 0 },
// { "SYS$CALL_HANDL",         0x80000010, 0L, 0 },
{ "SYS$CALL_HANDL_JACKET",  0x7FFEE568, 0L, 0 },
{ "SYS$CANCEL",             0x7FFEDE60, 0L, 0 },
{ "SYS$CANCEL_SELECTIVE",   0x7FFEE5E8, 0L, 0 },
{ "SYS$CANEVTAST",          0x7FFEE3B0, 0L, 0 },
{ "SYS$CANEXH",             0x7FFEE0C0, 0L, 0 },
{ "SYS$CANRNH",             0x7FFEE528, 0L, 0 },
{ "SYS$CANRUH",             0x7FFEE650, 0L, 0 },
{ "SYS$CANTIM",             0x7FFEDE68, 0L, 0 },
{ "SYS$CANWAK",             0x7FFEDE70, 0L, 0 },
{ "SYS$CFS_SERVICE",        0x7FFEE5F8, 0L, 0 },
{ "SYS$CHECK_AUDIT",        0x7FFEE800, 0L, 0 },
{ "SYS$CHECK_PRIVILEGE",    0x7FFEE8C8, 0L, 0 },
{ "SYS$CHECK_PRIVILEGE2_2", 0x7FFEE8D8, 0L, 0 },
{ "SYS$CHECK_PRIVILEGE2_3", 0x7FFEE8E0, 0L, 0 },
{ "SYS$CHECK_PRIVILEGEW",   0x7FFEE8D0, 0L, 0 },
{ "SYS$CHKPRO",             0x7FFEE4E0, 0L, 0 },
{ "SYS$CLI",                0x7FFEDE18, 0L, 0 },
{ "SYS$CLOSE",              0x7FFEE1B8, 0L, 0 },
{ "SYS$CLRAST",             0x7FFEE108, 0L, 0 },
{ "SYS$CLRAST_2",           0x7FFEE110, 0L, 0 },
{ "SYS$CLRCLUEVT",          0x7FFEE820, 0L, 0 },
{ "SYS$CLREF",              0x7FFEDE98, 0L, 0 },
{ "SYS$CLRPAR",             0x7FFEDE80, 0L, 0 },
{ "SYS$CMEXEC",             0x7FFEDE88, 0L, 0 },
{ "SYS$CMKRNL",             0x7FFEDE90, 0L, 0 },
{ "SYS$CNTREG",             0x7FFEDEA0, 0L, 0 },
{ "SYS$COMMIT_RU",          0x7FFEE638, 0L, 0 },
{ "SYS$CONNECT",            0x7FFEE1C0, 0L, 0 },
{ "SYS$CREATE",             0x7FFEE1C8, 0L, 0 },
{ "SYS$CREATE_BRANCH",      0x7FFEE840, 0L, 0 },
{ "SYS$CREATE_BRANCHW",     0x7FFEE848, 0L, 0 },
{ "SYS$CREATE_BRANCHW_1",   0x7FFEE850, 0L, 0 },
{ "SYS$CREATE_BUFOBJ",      0x7FFEE550, 0L, 0 },
{ "SYS$CREATE_UID",         0x7FFEE788, 0L, 0 },
{ "SYS$CRELNM",             0x7FFEE480, 0L, 0 },
{ "SYS$CRELNT",             0x7FFEE478, 0L, 0 },
{ "SYS$CRELOG",             0x7FFEDEB0, 0L, 0 },
{ "SYS$CREMBX",             0x7FFEDEB8, 0L, 0 },
{ "SYS$CREPRC",             0x7FFEDEC0, 0L, 0 },
{ "SYS$CRETVA",             0x7FFEDEC8, 0L, 0 },
{ "SYS$CRMPSC",             0x7FFEDE78, 0L, 0 },
{ "SYS$DACEFC",             0x7FFEDED0, 0L, 0 },
{ "SYS$DALLOC",             0x7FFEDED8, 0L, 0 },
{ "SYS$DASSGN",             0x7FFEDEE0, 0L, 0 },
{ "SYS$DCLAST",             0x7FFEDEE8, 0L, 0 },
{ "SYS$DCLCMH",             0x7FFEE0A0, 0L, 0 },
{ "SYS$DCLEVT",             0x7FFEE388, 0L, 0 },
{ "SYS$DCLEXH",             0x7FFEDEF0, 0L, 0 },
{ "SYS$DCLRNH",             0x7FFEE520, 0L, 0 },
{ "SYS$DCLRUH",             0x7FFEE648, 0L, 0 },
{ "SYS$DECLARE_RM",         0x7FFEE6E8, 0L, 0 },
{ "SYS$DECLARE_RMW",        0x7FFEE740, 0L, 0 },
{ "SYS$DELETE",             0x7FFEE168, 0L, 0 },
{ "SYS$DELETE_BUFOBJ",      0x7FFEE558, 0L, 0 },
{ "SYS$DELLNM",             0x7FFEE488, 0L, 0 },
{ "SYS$DELLOG",             0x7FFEDEF8, 0L, 0 },
{ "SYS$DELMBX",             0x7FFEDF00, 0L, 0 },
{ "SYS$DELPRC",             0x7FFEDF08, 0L, 0 },
{ "SYS$DELTVA",             0x7FFEDF10, 0L, 0 },
{ "SYS$DEQ",                0x7FFEE3C8, 0L, 0 },
{ "SYS$DERLMB",             0x7FFEE0B8, 0L, 0 },
{ "SYS$DEVICE_SCAN",        0x7FFEE518, 0L, 0 },
{ "SYS$DGBLSC",             0x7FFEDF18, 0L, 0 },
{ "SYS$DIAGNOSE",           0x7FFEE560, 0L, 0 },
{ "SYS$DISABLE_VP_USE",     0x7FFEE7B0, 0L, 0 },
{ "SYS$DISABLE_VP_USE_INT", 0x7FFEE7B8, 0L, 0 },
{ "SYS$DISCONNECT",         0x7FFEE1D0, 0L, 0 },
{ "SYS$DISPLAY",            0x7FFEE1D8, 0L, 0 },
{ "SYS$DLCDNP",             0x7FFEDF20, 0L, 0 },
{ "SYS$DLCEFC",             0x7FFEDF28, 0L, 0 },
{ "SYS$DNS",                0x7FFEE5A0, 0L, 0 },
{ "SYS$DNSW",               0x7FFEE5A8, 0L, 0 },
{ "SYS$EMAA",               0x7FFEE790, 0L, 0 },
{ "SYS$ENABLE_VP_USE",      0x7FFEE7A0, 0L, 0 },
{ "SYS$ENABLE_VP_USE_INT",  0x7FFEE7A8, 0L, 0 },
{ "SYS$END_BRANCH",         0x7FFEE5B0, 0L, 0 },
{ "SYS$END_BRANCHW",        0x7FFEE5B8, 0L, 0 },
{ "SYS$END_BRANCHW_2",      0x7FFEE5C0, 0L, 0 },
{ "SYS$END_RU",             0x7FFEE678, 0L, 0 },
{ "SYS$END_RU_2",           0x7FFEE680, 0L, 0 },
{ "SYS$END_TRANS",          0x7FFEE6D8, 0L, 0 },
{ "SYS$END_TRANSW",         0x7FFEE730, 0L, 0 },
{ "SYS$ENQ",                0x7FFEE3C0, 0L, 0 },
{ "SYS$ENQW",               0x7FFEE3D0, 0L, 0 },
{ "SYS$ENQW_2",             0x7FFEE3D8, 0L, 0 },
{ "SYS$ENQW_3",             0x7FFEE3E0, 0L, 0 },
{ "SYS$ENTER",              0x7FFEE228, 0L, 0 },
{ "SYS$ERAPAT",             0x7FFEE470, 0L, 0 },
{ "SYS$ERASE",              0x7FFEE1E0, 0L, 0 },
{ "SYS$EVDPOSTEVENT",       0x7FFEE798, 0L, 0 },
{ "SYS$EXCMSG",             0x7FFEE0E8, 0L, 0 },
{ "SYS$EXIT",               0x7FFEDF40, 0L, 0 },
{ "SYS$EXPREG",             0x7FFEDF48, 0L, 0 },
{ "SYS$EXTEND",             0x7FFEE1E8, 0L, 0 },
{ "SYS$EXTRNH",             0x7FFEE530, 0L, 0 },
{ "SYS$FAO",                0x7FFEDF50, 0L, 0 },
{ "SYS$FAOL",               0x7FFEDF58, 0L, 0 },
{ "SYS$FILESCAN",           0x7FFEE278, 0L, 0 },
{ "SYS$FIND",               0x7FFEE170, 0L, 0 },
{ "SYS$FINISH_RDB",         0x7FFEE4B8, 0L, 0 },
{ "SYS$FINISH_RMOP",        0x7FFEE700, 0L, 0 },
{ "SYS$FINISH_RMOPW",       0x7FFEE758, 0L, 0 },
{ "SYS$FLUSH",              0x7FFEE1F0, 0L, 0 },
{ "SYS$FORCEX",             0x7FFEDF60, 0L, 0 },
{ "SYS$FORGET_RM",          0x7FFEE6F0, 0L, 0 },
{ "SYS$FORGET_RMW",         0x7FFEE748, 0L, 0 },
{ "SYS$FORGE_WORD",         0x7FFEE900, 0L, 0 },
{ "SYS$FREE",               0x7FFEE178, 0L, 0 },
{ "SYS$GET",                0x7FFEE180, 0L, 0 },
{ "SYS$GETCHN",             0x7FFEE0C8, 0L, 0 },
{ "SYS$GETDEV",             0x7FFEE0D0, 0L, 0 },
{ "SYS$GETDVI",             0x7FFEE410, 0L, 0 },
{ "SYS$GETDVIW",            0x7FFEE418, 0L, 0 },
{ "SYS$GETEVI",             0x7FFEE3B8, 0L, 0 },
{ "SYS$GETJPI",             0x7FFEE0D8, 0L, 0 },
{ "SYS$GETJPIW",            0x7FFEE420, 0L, 0 },
{ "SYS$GETJPIW_2",          0x7FFEE428, 0L, 0 },
{ "SYS$GETLKI",             0x7FFEE498, 0L, 0 },
{ "SYS$GETLKIW",            0x7FFEE4A0, 0L, 0 },
{ "SYS$GETLKIW_2",          0x7FFEE4A8, 0L, 0 },
{ "SYS$GETLUI",             0x7FFEE690, 0L, 0 },
{ "SYS$GETMSG",             0x7FFEE0B0, 0L, 0 },
{ "SYS$GETPTI",             0x7FFEDEA8, 0L, 0 },
{ "SYS$GETQUI",             0x7FFEE4F8, 0L, 0 },
{ "SYS$GETQUIW",            0x7FFEE500, 0L, 0 },
{ "SYS$GETQUIW_2",          0x7FFEE508, 0L, 0 },
{ "SYS$GETSECI",            0x7FFEE6B8, 0L, 0 },
{ "SYS$GETSYI",             0x7FFEE3F8, 0L, 0 },
{ "SYS$GETSYIW",            0x7FFEE430, 0L, 0 },
{ "SYS$GETTIM",             0x7FFEDF78, 0L, 0 },
{ "SYS$GETUTC",             0x7FFEE578, 0L, 0 },
{ "SYS$GET_DEFAULT_TRANS", 0x7FFEE858, 0L, 0 },
{ "SYS$GET_RUID", 0x7FFEE670, 0L, 0 },
{ "SYS$GET_SECURITY", 0x7FFEE8F0, 0L, 0 },
{ "SYS$GL_ASTRET", 0x7FFEE110, 0L, 0 },
{ "SYS$GL_COMMON", 0x7FFEE114, 0L, 0 },
{ "SYS$GRANTID", 0x7FFEE4D0, 0L, 0 },
{ "SYS$GRANT_LICENSE", 0x7FFEE698, 0L, 0 },
{ "SYS$HASH_PASSWORD", 0x7FFEE538, 0L, 0 },
{ "SYS$HIBER", 0x7FFEDF88, 0L, 0 },
{ "SYS$IDTOASC", 0x7FFEE4C0, 0L, 0 },
{ "SYS$IMGACT", 0x7FFEDF90, 0L, 0 },
{ "SYS$IMGFIX", 0x7FFEE400, 0L, 0 },
{ "SYS$IMGFIX_2", 0x7FFEE408, 0L, 0 },
{ "SYS$IMGSTA", 0x7FFEDF68, 0L, 0 },
{ "SYS$IPC", 0x7FFEE6C8, 0L, 0 },
{ "SYS$IPCW", 0x7FFEE770, 0L, 0 },
{ "SYS$IPCW_2", 0x7FFEE778, 0L, 0 },
{ "SYS$IPCW_3", 0x7FFEE780, 0L, 0 },
{ "SYS$JOIN_RM", 0x7FFEE6F8, 0L, 0 },
{ "SYS$JOIN_RMW", 0x7FFEE750, 0L, 0 },
{ "SYS$JOIN_RU", 0x7FFEE658, 0L, 0 },
{ "SYS$LCKPAG", 0x7FFEDF98, 0L, 0 },
{ "SYS$LKWSET", 0x7FFEDFA0, 0L, 0 },
{ "SYS$LOOKUP_LICENSE", 0x7FFEE6C0, 0L, 0 },
{ "SYS$MAKE_REF", 0x7FFEE540, 0L, 0 },
{ "SYS$MGBLSC", 0x7FFEDFA8, 0L, 0 },
{ "SYS$MODIFY", 0x7FFEE1F8, 0L, 0 },
{ "SYS$MTACCESS", 0x7FFEE688, 0L, 0 },
{ "SYS$NETWORK_LOGIN", 0x7FFEE828, 0L, 0 },
{ "SYS$NUMTIM", 0x7FFEDFB8, 0L, 0 },
{ "SYS$NUMUTC", 0x7FFEE580, 0L, 0 },
{ "SYS$NXTVOL", 0x7FFEE200, 0L, 0 },
{ "SYS$OPEN", 0x7FFEE208, 0L, 0 },
{ "SYS$PARSE", 0x7FFEE230, 0L, 0 },
{ "SYS$POSIX_FORK_CONTROL", 0x7FFEE5E0, 0L, 0 },
{ "SYS$POSIX_IOSERVICE", 0x7FFEE5D8, 0L, 0 },
{ "SYS$POSIX_SERVICE", 0x7FFEE5D0, 0L, 0 },
{ "SYS$POSIX_UMSERVICE", 0x7FFEE5F0, 0L, 0 },
{ "SYS$PRCTERM", 0x7FFEE818, 0L, 0 },
{ "SYS$PREPARE_RU", 0x7FFEE630, 0L, 0 },
{ "SYS$PROCESS_SCAN", 0x7FFEE510, 0L, 0 },
{ "SYS$PURGWS", 0x7FFEDFB0, 0L, 0 },
{ "SYS$PUT", 0x7FFEE188, 0L, 0 },
{ "SYS$PUTMSG", 0x7FFEE0E0, 0L, 0 },
{ "SYS$QIO", 0x7FFEDFC8, 0L, 0 },
{ "SYS$QIOW", 0x7FFEDE00, 0L, 0 },
{ "SYS$QIOW_2", 0x7FFEDE08, 0L, 0 },
{ "SYS$QIOW_3", 0x7FFEDE10, 0L, 0 },
{ "SYS$READ", 0x7FFEE190, 0L, 0 },
{ "SYS$READEF", 0x7FFEDFD0, 0L, 0 },
{ "SYS$RECOVER", 0x7FFEE860, 0L, 0 },
{ "SYS$RECOVERW", 0x7FFEE868, 0L, 0 },
{ "SYS$RECOVERW_1", 0x7FFEE870, 0L, 0 },
{ "SYS$RELEASE", 0x7FFEE198, 0L, 0 },
{ "SYS$RELEASE_LICENSE", 0x7FFEE6A0, 0L, 0 },
{ "SYS$RELEASE_VP", 0x7FFEE7C0, 0L, 0 },
{ "SYS$RELEASE_VP_INT", 0x7FFEE7C8, 0L, 0 },
{ "SYS$REMOVE", 0x7FFEE238, 0L, 0 },
{ "SYS$REMOVE_REF", 0x7FFEE548, 0L, 0 },
{ "SYS$RENAME", 0x7FFEE240, 0L, 0 },
{ "SYS$REPORT_EVENT", 0x7FFEE808, 0L, 0 },
{ "SYS$RESCHED", 0x7FFEE6B0, 0L, 0 },
{ "SYS$RESTORE_VP_EXCEPTION", 0x7FFEE7D0, 0L, 0 },
{ "SYS$RESTORE_VP_EXC_INT", 0x7FFEE7D8, 0L, 0 },
{ "SYS$RESTORE_VP_STATE", 0x7FFEE7F0, 0L, 0 },
{ "SYS$RESUME", 0x7FFEDFD8, 0L, 0 },
{ "SYS$REVOKID", 0x7FFEE4D8, 0L, 0 },
{ "SYS$REWIND", 0x7FFEE210, 0L, 0 },
{ "SYS$RMSRUNDWN", 0x7FFEE268, 0L, 0 },
{ "SYS$RUNDOWN_RU", 0x7FFEE668, 0L, 0 },
{ "SYS$RUNDWN", 0x7FFEDFE0, 0L, 0 },
{ "SYS$SAVE_VP_EXCEPTION", 0x7FFEE7E0, 0L, 0 },
{ "SYS$SAVE_VP_EXC_INT", 0x7FFEE7E8, 0L, 0 },
{ "SYS$SCHDWK", 0x7FFEDFF0, 0L, 0 },
{ "SYS$SCHED", 0x7FFEE7F8, 0L, 0 },
{ "SYS$SEARCH", 0x7FFEE248, 0L, 0 },
{ "SYS$SETAST", 0x7FFEDFF8, 0L, 0 },
{ "SYS$SETCLUEVT", 0x7FFEE810, 0L, 0 },
{ "SYS$SETDDIR", 0x7FFEE250, 0L, 0 },
{ "SYS$SETDFPROT", 0x7FFEE258, 0L, 0 },
{ "SYS$SETEF", 0x7FFEE000, 0L, 0 },
{ "SYS$SETEVTAST", 0x7FFEE390, 0L, 0 },
{ "SYS$SETEVTASTW", 0x7FFEE398, 0L, 0 },
{ "SYS$SETEVTASTW_2", 0x7FFEE3A0, 0L, 0 },
{ "SYS$SETEVTASTW_3", 0x7FFEE3A8, 0L, 0 },
{ "SYS$SETEXV", 0x7FFEE008, 0L, 0 },
{ "SYS$SETIME", 0x7FFEE0F8, 0L, 0 },
{ "SYS$SETIMR", 0x7FFEE020, 0L, 0 },
{ "SYS$SETPFM", 0x7FFEE0A8, 0L, 0 },
{ "SYS$SETPRA", 0x7FFEE018, 0L, 0 },
{ "SYS$SETPRI", 0x7FFEE028, 0L, 0 },
{ "SYS$SETPRN", 0x7FFEE010, 0L, 0 },
{ "SYS$SETPRT", 0x7FFEE030, 0L, 0 },
{ "SYS$SETPRV", 0x7FFEE100, 0L, 0 },
{ "SYS$SETRWM", 0x7FFEE038, 0L, 0 },
{ "SYS$SETSFM", 0x7FFEE040, 0L, 0 },
{ "SYS$SETSHLV", 0x7FFEE908, 0L, 0 },
{ "SYS$SETSSF", 0x7FFEE3E8, 0L, 0 },
{ "SYS$SETSTK", 0x7FFEE3F0, 0L, 0 },
{ "SYS$SETSWM", 0x7FFEE048, 0L, 0 },
{ "SYS$SET_DEFAULT_TRANS", 0x7FFEE878, 0L, 0 },
{ "SYS$SET_DEFAULT_TRANSW", 0x7FFEE880, 0L, 0 },
{ "SYS$SET_DEF_TRAN_DUMMY", 0x7FFEE888, 0L, 0 },
{ "SYS$SET_RESOURCE_DOMAIN", 0x7FFEE8F8, 0L, 0 },
{ "SYS$SET_SECURITY", 0x7FFEE8E8, 0L, 0 },
{ "SYS$SIGPRC", 0x7FFEE6A8, 0L, 0 },
{ "SYS$SNDACC", 0x7FFEE0F0, 0L, 0 },
{ "SYS$SNDERR", 0x7FFEDF38, 0L, 0 },
{ "SYS$SNDJBC", 0x7FFEDF70, 0L, 0 },
{ "SYS$SNDJBCW", 0x7FFEE438, 0L, 0 },
{ "SYS$SNDOPR", 0x7FFEDFC0, 0L, 0 },
{ "SYS$SNDSMB", 0x7FFEDFE8, 0L, 0 },
{ "SYS$SPACE", 0x7FFEE218, 0L, 0 },
{ "SYS$SPARE_VECTOR_1", 0x7FFEE270, 0L, 0 },
{ "SYS$SRCHANDLER", 0x7FFEE118, 0L, 1 }, /* Flag indicates JMP not CALL */
{ "SYS$SSVEXC", 0x7FFEE260, 0L, 0 },
{ "SYS$SS_VECTOR_DUMMY_3_1", 0x7FFEE5C8, 0L, 0 },
{ "SYS$SS_VECTOR_DUMMY_3_10", 0x7FFEE610, 0L, 0 },
{ "SYS$SS_VECTOR_DUMMY_3_11", 0x7FFEE618, 0L, 0 },
{ "SYS$SS_VECTOR_DUMMY_3_12", 0x7FFEE620, 0L, 0 },
{ "SYS$SS_VECTOR_DUMMY_3_8", 0x7FFEE600, 0L, 0 },
{ "SYS$SS_VECTOR_DUMMY_3_9", 0x7FFEE608, 0L, 0 },
{ "SYS$SS_VECTOR_SPARE", 0x7FFEE918, 0L, 0 },
{ "SYS$START_BRANCH", 0x7FFEE710, 0L, 0 },
{ "SYS$START_BRANCHW", 0x7FFEE768, 0L, 0 },
{ "SYS$START_RU", 0x7FFEE628, 0L, 0 },
{ "SYS$START_TRANS", 0x7FFEE6D0, 0L, 0 },
{ "SYS$START_TRANSW", 0x7FFEE718, 0L, 0 },
{ "SYS$START_TRANSW_2", 0x7FFEE720, 0L, 0 },
{ "SYS$START_TRANSW_3", 0x7FFEE728, 0L, 0 },
{ "SYS$SUBSYSTEM", 0x7FFEE598, 0L, 0 },
{ "SYS$SUSPND", 0x7FFEE050, 0L, 0 },
{ "SYS$SYNCH", 0x7FFEE440, 0L, 0 },
{ "SYS$SYNCH_INT", 0x7FFEE910, 0L, 0 },
{ "SYS$TIMCON", 0x7FFEE570, 0L, 0 },
{ "SYS$TRANS_EVENT", 0x7FFEE890, 0L, 0 },
{ "SYS$TRANS_EVENTW", 0x7FFEE898, 0L, 0 },
{ "SYS$TRANS_EVENTW_1", 0x7FFEE8A0, 0L, 0 },
{ "SYS$TRNLNM", 0x7FFEE490, 0L, 0 },
{ "SYS$TRNLOG", 0x7FFEE058, 0L, 0 },
{ "SYS$TRUNCATE", 0x7FFEE220, 0L, 0 },
{ "SYS$TSTCLUEVT", 0x7FFEE830, 0L, 0 },
{ "SYS$ULKPAG", 0x7FFEE060, 0L, 0 },
{ "SYS$ULWSET", 0x7FFEE068, 0L, 0 },
{ "SYS$UNJOIN_RU", 0x7FFEE660, 0L, 0 },
{ "SYS$UNWIND", 0x7FFEE070, 0L, 0 },
{ "SYS$UPDATE", 0x7FFEE1A0, 0L, 0 },
{ "SYS$UPDSEC", 0x7FFEDF30, 0L, 0 },
{ "SYS$UPDSECW", 0x7FFEDF80, 0L, 0 },
{ "SYS$WAIT", 0x7FFEE1A8, 0L, 0 },
{ "SYS$WAITFR", 0x7FFEE078, 0L, 0 },
{ "SYS$WAIT_FORM", 0x7FFEE120, 0L, 0 },
{ "SYS$WAKE", 0x7FFEE080, 0L, 0 },
{ "SYS$WFLAND", 0x7FFEE088, 0L, 0 },
{ "SYS$WFLOR", 0x7FFEE090, 0L, 0 },
{ "SYS$WRITE", 0x7FFEE1B0, 0L, 0 },
{ 0L, 0L, 0L, 0}};


/*
 *  Called by XFC #7A, which invokes a system service.
 */

long call_service( LONGWORD pc )
{
    int n;
    CALLV callv;
    LONGWORD rc, argc, size, * argv;

    char msg[ 256 ], mbuff[ 10 ];

    /*
     *  Figure out which service it is.
     */

    for( n = 0; n < service_count; n++ ) {

        if( !p1_vector[ n ].jmp && ( p1_vector[ n ].addr == pc )) 
            break;
        
        if( p1_vector[ n ].jmp && ( p1_vector[n].addr == pc + 2 ))
            break;
            
    }

    if( n >= service_count ) {
        printf( "Attempt to execute at non-existant P1 vector %08X\n", pc );
        vax.halted = 1;
        return VAX_NOSERVICE;
    }


    callv = p1_vector[ n ].service;
    sprintf( msg, "%08X: [%s] %s( ", vax.PC, p1_vector[ n ].jmp ? "JMP" : "CALL",
             p1_vector[ n ].name );

/*
 *  Find out how many arguments there are
 */

    rc = load_memory( vax.AP, ( void * ) &argc, 4 );
    if( rc )
        return rc;

/*
 *  Allocate an argument array natively for them.
 */

    size = sizeof( LONGWORD ) * argc;
    argv = ( LONGWORD * ) getmem( size );
    if( argv == 0L )
        return VAX_MEM;

/*
 *  Get the arguments locally in native format
 */

    for( n = 0; n < argc; n++ ) {
    
        rc = load_memory( ( vax.AP ) + (( n + 1 ) * 4 ), ( void * ) &( argv[ n ]), 4 );
        if( rc )
            return rc;

        if( n > 0 )
            strcat( msg, ", " );
        sprintf( mbuff, "%08X", argv[ n ] );
        strcat( msg, mbuff );
        
    }
    strcat( msg, n ? " )" : ")" );

/*
 *  If at this point we know there's no place to go, give it up.
 */

    if( callv == 0L ) {
        printf( "Unimplemented native service for %s\n", msg );
        return VAX_NOSERVICE;
    }

    rc = (*callv)( argc, argv );

    if( vax.debug & DBG_SERVICES ) {
        sprintf( mbuff, "%08X", rc );
        strcat( msg, ", returns " );
        strcat( msg, mbuff );
        printf( "DEBUG(SERVICES): %s\n", msg );
    }
    
    vax.R0 = rc;
    freemem( argv );

    return VAX_OK;
}


/*
 *  Initialize the P1 space for the current micro-kernel
 *  based system.  Fill in the supported service handlers
 *  where known.
 */

long p1_init(void)
{
    LONGWORD setpte_multiple( char ** P );
    long n, min, max;
    long rc;
    LONGWORD addr;
    short mask;
    char byte;
    int saved_mode, jmp;
    char buff[ 64 ], *bp;
    
    saved_mode = (int) vax.pslw.cur_mod;
    set_mode_stack( 0 );
    
    max = 0;
    min = 0x7FFFFFFF;

    mask = 0x0;          /* Procedure mask is empty */

    for( n = 0; n < 1000; n++ ) {

        if( p1_vector[ n ].name == 0L )
            break;

        addr = p1_vector[ n ].addr;

    /* If the service field is non-empty it is a flag that means this */
    /* entry is accessed via JMP rather than CALL, such as SYS$SRCHANDLER */
    
        jmp = p1_vector[ n ].jmp;
        
        rc = set_symbol_direct( p1_vector[ n ].name, addr, 
            ( jmp ? SYM_LABEL : SYM_ENTRY ) | SYM_PERMANENT );
        if( rc )
            return rc;

        /*  Write the procedure entry mask to memory */

        if( !jmp ) {
            rc = store_memory( addr, ( void * ) &mask, 2 );
            if( rc )
                return rc;
        }
        else {
        
        /* Because we dont' write a mask, fake out the address */
        /* so subsequent offsets will go to the right memory   */
        /* addresses.   */
        
            addr = addr - 2;
        }
        

        /*  Write the XFC opcode */

        byte = 0xFC;
        rc = store_memory( addr+2, ( void * ) &byte, 1 );
        if( rc )
            return rc;


        /*  Write the XFC$P1VECTOR selector code */

        byte = 0x7A;
        rc = store_memory( addr+3, ( void * ) &byte, 1 );
        if( rc )
            return rc;


        /*  Write the RET instruction */

        byte = 0x04;
        rc = store_memory( addr+4, ( void * ) &byte, 1 );
        if( rc )
            return rc;



        /*  Update the min and max values */

        if( ( unsigned long ) p1_vector[ n ].addr >= 0x80000000UL )
            continue;

        if( p1_vector[ n ].addr > max )
            max = p1_vector[ n ].addr;

        if( p1_vector[ n ].addr < min )
            min = p1_vector[ n ].addr;

    }

    service_count = n;

    set_symbol_direct( "EXE$P1_VECTOR_BASE", min, SYM_NONE );
    set_symbol_direct( "EXE$P1_VECTOR_END",  max, SYM_NONE );
    
    declare_services();

    bp = buff;
    strcpy( bp, "EXE$P1_VECTOR_BASE TO EXE$P1_VECTOR_END PROT=PTE$K_UR" );
    
    setpte_multiple( &bp );
    
    set_mode_stack( saved_mode );
    
    return VAX_OK;
}

   
    
/*
 *  Declare a handler for a system service.
 */

long declare_service( char * name, CALLV handler )
{

    int n;

    for( n = 0; n < service_count; n++ ) {
        if( strcmp( name, p1_vector[ n ].name ) == 0 ) {
            p1_vector[ n ].service = handler;
            return VAX_OK;
        }
    }

    return VAX_NOSERVICE;
}


/*
 *  Declare all services we know about
 */

void declare_services( void )
{

    declare_service( "SYS$CLREF",           sys_clref );
    declare_service( "SYS$SETEF",           sys_setef );
    declare_service( "SYS$READEF",          sys_readef );
    declare_service( "SYS$EXPREG",          sys_expreg );
    declare_service( "SYS$TRNLNM",          sys_trnlnm );
    declare_service( "SYS$DCLEXH",          sys_dclexh );
    declare_service( "SYS$ASSIGN",	    sys_assign );
    declare_service( "SYS$GETDVIW",	    sys_getdviw );
    declare_service( "SYS$GETJPIW",         sys_getjpiw );
    declare_service( "SYS$SETAST",          sys_setast );
    declare_service( "SYS$CLI",             sys_cli );
    
    declare_service( "SYS$CREATE",          rms_create );
    declare_service( "SYS$CONNECT",         rms_connect );
    declare_service( "SYS$PUT",             rms_put );
    
}

