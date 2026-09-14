//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     errors.c
//
//  Purpose:    This module implements vaxexcept() and vaxmsg() routine, which
//              given a numeric integer value will return the name of the exception
//              or error message text.
//
//  History:    08/06/97    New header format standardization
//              12/10/98    Added BIGENDIAN sensitivity
//


#include "vax.h"


static char * facility[] = { "EVAX", "EMUL", "CPU", "ASM", 0L };
static char * severity[] = { "W", "S", "E", "I", "F", "?", "?", "?" };


static struct MESSAGES {
    LONGWORD    id;
    char *  code;
    char *  text;
} messages[] = {
{   VAX_OK,                 "OK",           "Normal successfull completion"                     },
{   VAX_MEM,                "MEM",          "Insufficent memory for emulator to continue"       },
{   VAX_UNIMP,              "UNIMP",        "Unimplemented emulation of VAX instruction"        },
{   VAX_UNKPARM,            "UNKPARM",      "Unknown command parameter"                         },
{   VAX_UNKCMD,             "UNKCMD",       "Unknown command"                                   },
{   VAX_ILLADDRFAULT,       "ILLADDRFAULT", "Illegal addressing mode fault"                     },
{   VAX_TBIT,               "TBIT",         "Trace bit exception"                               },
{   VAX_HALT,               "HALT",         "Processor halted"                                  },
{   VAX_ACCVIO,             "ACCVIO",       "Access violation"                                  },
{   VAX_ACCVIORD,           "ACCVIO",       "Access violation (reason mask=read)"               },
{   VAX_ACCVIOWR,           "ACCVIO",       "Access violation (reason mask=write)"              },
{   VAX_ACCVIOMD,           "ACCVIO",       "Access violation (reason mask=read+write)"         },
{   VAX_ASSERT,             "ASSERT",       "Assertion failure"                                 },
{   VAX_ASMINVMODE,         "INVMODE",      "Invalid operand or mode specification"             },
{   VAX_ASMINVREG,          "INVREG",       "Invalid register specification"                    },
{   VAX_ASMINVCONST,        "INVCONST",     "Invalid constant specification"                    },
{   VAX_ASMINSFOPERANDS,    "INSFOPERANDS", "Insufficient operands"                             },
{   VAX_ASMINVOPCODE,       "INVOPCODE",    "Invalid instruction opcode"                        },
{   VAX_ASMINVPSEUDO,       "INVPSEUDO",    "Invalid pseudo instruction"                        },
{   VAX_ASMNOTPSEUDO,       "NOTPSEUDO",    "Opcode is not a pseudo-opcode"                     },
{   VAX_ASMUNDSYM,          "UNDSYM",       "Undefined symbol"                                  },
{   VAX_NOHELPFILE,         "NOHELPFILE",   "Help file VAX.HELP not found"                      },
{   VAX_NOHELP,             "NOHELP",       "No help available for that topic"                  },
{   VAX_NOVAX,              "NOVAX",        "There is no virtual VAX to operate on"             },
{   VAX_SYNTAX,             "SYNTAX",       "Syntax error in console command"                   },
{   VAX_BREAK,              "BREAK",        "Processor halted at break point location"          },
{   VAX_ASMINVPR,           "INVPR",        "Invalid privileged register number"                },
{   VAX_FAULT,              "FAULT",        "A processor exception/fault occurred"              },
{   VAX_ASMNOOPCODE,        "NOOPCODE",     "No opcode on line"                                 },
{   VAX_ASMUNDSYMS,         "UNDSYMS",      "Cannot execute when there are undefined symbols"   },
{   VAX_ASMFDISPBYTE,       "FDISPBYTE",    "Invalid forward reference with byte displacement"  },
{   VAX_ASMFDISPWORD,       "FDISPWORD",    "Invalid forward reference with word displacement"  },
{   VAX_ASMINVBYTE,         "INVBYTE",      "Invalid or out-of-range byte value"                },
{   VAX_ASMINVWORD,         "INVWORD",      "Invalid or out-of-range word value"                },
{   VAX_ASMFORWARD,         "FORWARD",      "There are unresolved forward references"           },
{   VAX_INVSETASM,          "INVSETASM",    "Invalid SET ASSEMBLER parameter"                   },
{   VAX_UNKSTRUCT,          "UNKSTRUCT",    "Unknown or unsupported structured memory type"     },
{   VAX_INVENDADDR,         "INVENDADDR",   "Invalid ending address or negative range"          },
{   VAX_INCOMPLETE,         "INCOMPLETE",   "Incomplete or missing command parameter"           },
{   VAX_FNF,                "FNF",          "File not found"                                    },
{   VAX_UNKTEST,            "UNKTEST",      "Unknown or invalid TEST operation"                 },
{   VAX_INVINSTR,           "INVINSTR",     "Invalid VAX instruction"                           },
{   VAX_EXTRACMD,           "EXTRACMD",     "Syntax error; extra text after command or statement"   },
{   VAX_ASMINVPTE,          "INVPTE",       "Invalid PTE specification"                         },
{   VAX_INVVMSIZE,          "INVVMSIZE",    "Invalid VM size (more pages than physical memory)" },
{   VAX_INVVMSYN,           "INVVMSYN",     "Invalid VMINIT region specification"               },
{   VAX_INVS0SIZE,          "INVS0SIZE",    "Invalid S0 size; not enough room for stacks and PFN database" },
{   VAX_ASMFLOAT,           "FLOAT",        "Unexpected floating point constant"                },
{   VAX_ASMINVFLOAT,        "INVFLOAT",     "Invalid floating point constant"                   },
{   VAX_ASMNOTIMP,          "NOTIMP",       "Cannot assemble unimplemented instruction"         },
{   VAX_ASMINVMASK,         "INVMASK",      "Invalid register mask syntax"                      },
{   VAX_EXPENTRY,           "EXPENTRY",     "Expected entry address not found"                  },

{   VAX_UNHANDLED,          "UNHANDLED",    
"Unhandled exception halts processor;\n\t\targuments are left on KSP" },

{   VAX_INVARGLIST,         "INVARGLIST",   
"Invalid CALL argument list"                        },

{   VAX_ASMINVFEX,          "INVFEX",       
"Invalid forward reference expression"              },

{   VAX_ASMUNRESTMP,        "UNRESTMP",     
"Unresolved forward references to temporary labels" },

{   VAX_NOFRAMES,           "NOFRAMES", 
"No call frames on stack"                           },

{   VAX_ASMDUPSYM,          "DUPSYM",
"Invalid duplicate symbol definition"               },

{   VAX_INVSETDBG,          "INVSETDBG",
"Invalid SET DEBUG flag"                            },

{   VAX_INVSETPSL,          "INVSETPSL",
"Invalid SET PSL field or value"                    },

{   VAX_ASMINVSCB,          "INVSCB",
"Invalid SCB vector"                                },

{   VAX_ASMINVRGN,          "INVRGN",
"Invalid REGION name"                               },

{   VAX_ASMSPOVF,           "SPOVF",
"String pool overflow; use CLEAR STRING to reset"   },

{   VAX_ASMINVFARG,         "INVFARG",
"Invalid number or type of function arguments"      },

{   VAX_ASMNUMFARG,         "NUMFARG",
"Incorrect number of function arguments"            },

{   VAX_ASMMMFARG,          "MMFARG",
"Mismatched function argument types"                },

{   VAX_ASMNOTFUN,          "NOTFUN",
"Not a function name"                               },

{   VAX_NOTKERNEL,          "NOTKERNEL",
"Command not valid when not in KERNEL mode"         },

{   VAX_ATTENTION,          "ATTENTION",
"VAX halted by console attention"                   },

{   VAX_STEPENTRY,          "STEPENTRY",
"STEP over .ENTRY mask"                             },

{   VAX_NOMK,               "NOMK",
"Command not valid except when using microkernel" },

{   VAX_NOVMINIT,           "NOVMINIT",
"Command not valid unless VMINIT executed" },

{   VAX_DELMK,              "DELMK",
"Deleting microkernel"  },

{   VAX_BADROM,             "BADROM",
"Unabled to load ROM" },

{   VAX_BADNVR,             "BADNVR",
"Unable to load NVRAM" },

{   VAX_NOSERVICE,          "NOSERVICE",
"Invocation of unimplemented VMS system service" },

{   VAX_ASMINVIND,          "INVIND",
"Invalid use of '@' indirection operator" },

{   -1, 0, 0L }};


static struct EXCEPTIONS {
    short   code;
    char *  name;
} exceptions[] = {
    { 0x04, "Machine Check"                             },
    { 0x08, "Kernel Stack Not Valid"                    },
    { 0x0C, "Power Fail"                                },
    { 0x10, "Reserved or Privileged Instruction"        },
    { 0x14, "Customer Reserved Instruction XFC"         },
    { 0x18, "Reserved Operand"                          },
    { 0x1C, "Reserved Addressing Mode"                  },
    { 0x20, "Access Control Violation"                  },
    { 0x24, "Translation Not Valid"                     },
    { 0x28, "Trace Pending (TP)"                        },
    { 0x2C, "Breakpoint Instruction"                    },
    { 0x30, "Compatability"                             },
    { 0x34, "Arithmetic"                                },
    { 0x40, "CHMK"                                      },
    { 0x44, "CHME"                                      },
    { 0x48, "CHMS"                                      },
    { 0x4C, "CHMU"                                      },
    { 0x50, "SBI SILO Compare"                          },
    { 0x54, "Corrected Memory Read Data"                },
    { 0x58, "SBI Alert"                                 },
    { 0x5C, "SBI Fault"                                 },
    { 0x60, "Memory Write Timeout"                      },
    { 0x84, "Software Level 1"                          },
    { 0x88, "Software Level 2"                          },
    { 0x8C, "Software Level 3"                          },
    { 0x90, "Software Level 4"                          },
    { 0x94, "Software Level 5"                          },
    { 0x98, "Software Level 6"                          },
    { 0x9C, "Software Level 7"                          },
    { 0xA0, "Software Level 8"                          },
    { 0xA4, "Software Level 9"                          },
    { 0xA8, "Software Level 10"                         },
    { 0xAC, "Software Level 11"                         },
    { 0xB0, "Software Level 12"                         },
    { 0xB4, "Software Level 13"                         },
    { 0xB8, "Software Level 14"                         },
    { 0xBC, "Software Level 15"                         },
    { 0xC0, "Internal Timer"                            },
    { 0xF8, "Console Terminal Receive"                  },
    { 0xFC, "Console Terminal Transmit"                 },
    { 0, 0L }};



//  Return a text description for an exception vector code


char * vaxexcept( short code )
{
    static char b[ 32 ];
    short n;
    
    for( n = 0; exceptions[ n ].name; n++ ) {
    
        if( code == exceptions[ n ].code )
            return exceptions[ n ].name;
    }
    
    sprintf( b, "Unused/Undefined %02X", code );
    return b;

}


char * vaxmsg( LONGWORD rc )
{

    static char msgbuff[ 256 ];

    unsigned int fac, num, sev;
    
    LONGWORD n, match;
    LONGWORD rc_x, id_x;
    
    match = 0;
    rc_x = rc & 0xFFFFFFF8;
    for( n = 0; messages[ n ].id != -1; n++ ) {
        id_x = messages[ n ].id & 0xFFFFFFF8;
        if( id_x == rc_x ) {
            match = 1;
            break;
        }
    }
    
    if( !match ) {
        sprintf( msgbuff, "%%EVAX-?-UNRECOGNIZED, Unrecognized return code %08X", rc );
    }
    else {

        sev = ( rc >>  0 ) & 0x00000007;
        num = ( rc >>  3 ) & 0x0000FFFF;
        fac = ( rc >> 18 ) & 0x00003FFF;

        sprintf( msgbuff, "%%%s-%s-%s, %s",
            facility[ fac ],
            severity[ sev ],
            messages[ n ].code,
            messages[ n ].text );
    }
    return msgbuff;
}
