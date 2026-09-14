//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     init_symbols.c
//
//  Purpose:    Initialized the internal (system) symbol table for the VAX
//
//  History:    07/28/97    New header format standardization
//
//              05/20/00    Added new XFC symbols for DCL callback and quiting
//                          the emulator.
//

#include "vax.h"
#include "asmproto.h"
#include "vaxinstr.h"

void init_system_symbols(void);

//  This is the table of symbol values we create.  This could be externally
//  acquired someday, like from a resouce fork or external file.

static struct SYMDATA {
    char *  name;
    LONGWORD    value;
} symdata[] = {

    //  XFC function codes
    
    {   "XFC$CONSOLE_WRITE",    1       },
    {   "XFC$CONSOLE_READ",     2       },
    {   "XFC$CONSOLE_CMD",      3       },
    {   "XFC$VMR",              0x7F    },
    {   "XFC$VMW",              0x7E    },
    {   "XFC$SHIM",             0x7D    },
    {   "XFC$HALT",             0x7C    },
    {   "XFC$HALT_SILENT",      0x7B    },
    {   "XFC$P1VECTOR",         0x7A    },
    {   "XFC$DCL",              0x79    },
    {   "XFC$QUIT_EMULATION",   0x78    },

    //  DCL console callbacks   

    {   "DCL$_QUALIFIER",       'Q'     },
    {   "DCL$_VERB",            'V'     },
    {   "DCL$_PARAMETER",       'P'     },

    //  PTE protection flags
    
    {   "PTE$K_NA",             0       },
    {   "PTE$K_RESERVED",       1       },
    {   "PTE$K_KW",             2       },
    {   "PTE$K_KR",             3       },
    {   "PTE$K_UW",             4       },
    {   "PTE$K_EW",             5       },
    {   "PTE$K_ERKW",           6       },
    {   "PTE$K_ER",             7       },
    {   "PTE$K_SW",             8       },
    {   "PTE$K_SREW",           9       },
    {   "PTE$K_SRKW",           10      },
    {   "PTE$K_SR",             11      },
    {   "PTE$K_URSW",           12      },
    {   "PTE$K_UREW",           13      },
    {   "PTE$K_URKW",           14      },
    {   "PTE$K_UR",             15      },
    {   "PTE$K_ALL",            4       },
    {   "PTE$K_NONE",           0       },
    {   "PTE$K_READONLY",       15      },
    
    //  Privileged register names
    
    {   "VAX$PR_KSP",           0       },
    {   "VAX$PR_ESP",           1       },
    {   "VAX$PR_SSP",           2       },
    {   "VAX$PR_USP",           3       },
    {   "VAX$PR_ISP",           4       },
    {   "VAX$PR_P0BR",          8       },
    {   "VAX$PR_P0LR",          9       },
    {   "VAX$PR_P1BR",          10      },
    {   "VAX$PR_P1LR",          11      },
    {   "VAX$PR_SBR",           12      },
    {   "VAX$PR_SLR",           13      },
    {   "VAX$PR_PCBB",          16      },
    {   "VAX$PR_SCBB",          17      },
    {   "VAX$PR_IPL",           18      },
    {   "VAX$PR_ASTLVL",        19      },
    {   "VAX$PR_SIRR",          20      },
    {   "VAX$PR_SISR",          21      },
    {   "VAX$PR_ICCS",          24      },
    {   "VAX$PR_NICR",          25      },
    {   "VAX$PR_ICR",           26      },
    {   "VAX$PR_TODR",          27      },
    {   "VAX$PR_RXCS",          32      },
    {   "VAX$PR_RXDB",          33      },
    {   "VAX$PR_TXCS",          34      },
    {   "VAX$PR_TXDB",          35      },
    {   "VAX$PR_TBDR",          36      },
    {   "VAX$PR_MAPEN",         56      },
    {   "VAX$PR_TBIA",          57      },
    {   "VAX$PR_TBIS",          58      },
    {   "VAX$PR_PMR",           61      },
    {   "VAX$PR_SID",           62      },
    {   "VAX$PR_TBCHK",         63      },

    //  EDITPC operators

    {   "EO$INSERT",        0x44    },
    {   "EO$STORE_SIGN",    0x04    },
    {   "EO$FILL",      0x80    },
    {   "EO$MOVE",      0x90    },
    {   "EO$FLOAT",     0xA0    },
    {   "EO$END_FLOAT",     0x01    },
    {   "EO$BLANK_ZERO",    0x45    },
    {   "EO$REPLACE_SIGN",  0x46    },
    {   "EO$LOAD_FILL",     0x40    },
    {   "EO$LOAD_SIGN",     0x41    },
    {   "EO$LOAD_PLUS",     0x42    },
    {   "EO$LOAD_MINUS",    0x43    },
    {   "EO$CLEAR_SIGNIF",  0x02    },
    {   "EO$SET_SIGNIF",    0x03    },
    {   "EO$ADJUST_INPUT",  0x47    },
    {   "EO$END",       0x00    },

    //  Arithmetic traps

    {   "SRM$K_INT_OVF_T",  0x01    },
    {   "SRM$K_INT_DIV_T",  0x02    },
    {   "SRM$K_FLT_OVF_T",  0x03    },
    {   "SRM$K_FLT_DIV_T",  0x04    },
    {   "SRM$K_FLT_UND_T",  0x05    },
    {   "SRM$K_DEC_OVF_T",  0x06    },
    {   "SRM$K_SUB_RNG_T",  0x07    },

    //  Arithmetic faults

    {   "SRM$K_FLT_OVF_F",  0x08    },
    {   "SRM$K_FLT_DIV_F",  0x09    },
    {   "SRM$K_FLT_UND_F",  0x0A    },

    //  Exception vectors

    {   "EXC$UNUSED",           EXC_UNUSED   },
    {   "EXC$CHECK",            EXC_CHECK    },
    {   "EXC$KSNV",             EXC_KSNV     },
    {   "EXC$POWER",            EXC_POWER    },
    {   "EXC$PRIV",             EXC_PRIV     },
    {   "EXC$CUSTOMER",         EXC_CUSTOMER },
    {   "EXC$RESOP",            EXC_RESOP    },
    {   "EXC$RESADDR",          EXC_RESADDR  },
    {   "EXC$ACCVIO",           EXC_ACCVIO   },
    {   "EXC$TNV",              EXC_TNV      },
    {   "EXC$TP",               EXC_TP       },
    {   "EXC$BPT",              EXC_BPT      },
    {   "EXC$COMPAT",           EXC_COMPAT   },
    {   "EXC$ARITH",            EXC_ARITH    },
    {   "EXC$CHMK",             EXC_CHMK     },
    {   "EXC$CHME",             EXC_CHME     },
    {   "EXC$CHMS",             EXC_CHMS     },
    {   "EXC$CHMU",             EXC_CHMU  },
    {   "EXC$SBI",              EXC_SBI   },
    {   "EXC$CMRD",             EXC_CMRD   },
    {   "EXC$INTERVAL",         EXC_INTERVAL },
    {   "EXC$SBIALERT",         EXC_SBIALERT   },
    {   "EXC$SBIFAULT",         EXC_SBIFAULT   },
    {   "EXC$MWT",              EXC_MWT   },
    {   "EXC$SOFTWARE1",        EXC_SOFTWARE1   },
    {   "EXC$SOFTWARE2",        EXC_SOFTWARE2   },
    {   "EXC$SOFTWARE3",        EXC_SOFTWARE3   },
    {   "EXC$SOFTWARE4",        EXC_SOFTWARE4   },
    {   "EXC$SOFTWARE5",        EXC_SOFTWARE5   },
    {   "EXC$SOFTWARE6",        EXC_SOFTWARE6   },
    {   "EXC$SOFTWARE7",        EXC_SOFTWARE7   },
    {   "EXC$SOFTWARE8",        EXC_SOFTWARE8   },
    {   "EXC$SOFTWARE9",        EXC_SOFTWARE9   },
    {   "EXC$SOFTWARE10",       EXC_SOFTWARE10  },
    {   "EXC$SOFTWARE11",       EXC_SOFTWARE11  },
    {   "EXC$SOFTWARE12",       EXC_SOFTWARE12  },
    {   "EXC$SOFTWARE13",       EXC_SOFTWARE13  },
    {   "EXC$SOFTWARE14",       EXC_SOFTWARE14  },
    {   "EXC$SOFTWARE15",       EXC_SOFTWARE15  },
    {   "EXC$CONREAD",          EXC_CONREAD     },
    {   "EXC$CONWRITE",         EXC_CONWRITE    },
    
    {   "CONSOLE$HANDLER",      0xFFFFFFFF      },
    
    {   "$STATUS",          0x0     },
    

    {   "",                     0       }

};




//  Do the initialization, driven from the above table.

void init_system_symbols(void)
{

    LONGWORD n, rc;
    char * p;
    LONGWORD value;
    char name[ 32 ];

    
//  Create all the predefined names from the table above.
    
    for( n = 0; n < 1000; n++ ) {
    
        p = symdata[ n ].name;
        value = symdata[ n ].value;
    
        if( *p == 0 )
            break;
        
        rc = set_symbol( &p, value, SYM_PERMANENT );
        if( rc != VAX_OK )
            break;  
    }

//  Also, we create a symbol for all the single-byte instruction values.

    for( n = 0; n < 256; n++ ) {
    
        strcpy( name, "OPC$_" );
        strcat( name, instruction[ n ].name );
        value = n;
        p = name;
        rc = set_symbol( &p, value, SYM_PERMANENT );
        if( rc != VAX_OK )
            break;
        
    }
    
//  While not really symbols, now is as good a time as any to build the default
//  logical name table.

    init_logicals();
    
    return;
}
