//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     initialization.c
//
//  Purpose:    initialization and termination routines for the virtual VAX.
//
//  History:    07/28/97    New header format standardization
//
//              01/06/99    Add initialization of assembler flags for
//                          new features (SYMBOLS, specifically).
//
//              06/03/99    Initialize debug field to set DBG_REGISTERS
//                          as the default state.
//
//              11/16/99    For some console commands, the dispatch
//                          initialization routine is (void*)-1L which
//                          means just use the DCL dispatcher.

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "shim.h"


/*
 *  This is the routine that creates the initial system symbol table
 */

void init_system_symbols( void );

/*
 *  This data structure handles command dispatch.  The table consists of
 *  the verb and up to two possible aliases for the verb, and a slot for
 *  a routine entry point.  The dispatch_init routine fills in the entry
 *  points at runtime.
 *
 *  To add a new command, add it to the table below and add a dispatch
 *  initialization.  Then just write the routine and you're ready to go.
 *
 *  Oh yeah, you've also got to put a prototype for it in the console_proto
 *  file as well.
 */


struct CONSOLE_DISPATCH_TABLE  console_dispatch_table[] = {

{{  CHAR4('I','N','I','T'), 
    CHAR4('I','N','I','T'), 
    CHAR4('I','N','I','T')      },  0L      },

{{  CHAR4('S','H','O','W'), 
    CHAR4('S','H','O',' '), 
    CHAR4('S','H',' ',' ')      },  0L      },
    
{{  CHAR4('S','E','T',' '), 
    CHAR4('S','E',' ',' '),  
    0               },  0L      },

{{  CHAR4('Q','U','I','T'), 
    CHAR4('E','X','I','T'),  
    0               },  0L      },

{{  CHAR4('A','S','M',' '), 
    CHAR4('A','S','S','E'),  
    0               },  0L      },

{{  CHAR4('D','I','S','A'), 
    CHAR4('D','I','S',' '),  
    0               },  0L      },

{{  CHAR4('E','X','E','C'), 
    CHAR4('G','O',' ',' '), 
    CHAR4('G',' ',' ',' ')      },  0L      },

{{  CHAR4('E','X','A','M'), 
    CHAR4('E','X',' ',' '), 
    CHAR4('D','U','M','P')      },  0L      },

{{  CHAR4('Z','E','R','O'),  
    0,      
    0,              },  0L      },

{{  CHAR4('H','E','L','P'), 
    CHAR4('?',' ',' ',' '),  
    0               },  0L      },

{{  CHAR4('S','A','V','E'), 
    CHAR4('?',' ',' ',' '),  
    0               },  0L      },

{{  CHAR4('C','L','E','A'), 
    CHAR4('C','L','R',' '),  
    0               },  0L      },

{{  CHAR4('S','T','E','P'), 
    CHAR4('S','T',' ',' '),  
    CHAR4('S',' ',' ',' ')  },  0L      },

{{  CHAR4('R','U','N',' '), 
    CHAR4('R',' ',' ',' '),  
    0               },  0L      },

{{  CHAR4('L','O','A','D'),  
    0,      
    0               },  0L      },

{{  CHAR4('T','I','M','E'),  
    0,      
    0               },  0L      },

{{  CHAR4('I','N','C','L'), 
    CHAR4('I','N','C',' '),  
    CHAR4('@',' ',' ',' ')  },  0L      },

{{  CHAR4('P','R','I','N'), 
    CHAR4('E','C','H','O'),  
    0               },  0L      },

{{  CHAR4('T','E','S','T'),  
    0,      
    0               },  0L      },

{{  CHAR4('V','M','I','N'),  
    CHAR4('V','M',' ',' '), 
    CHAR4('V','M','I',' ')  },  0L      },

{{  CHAR4('C','A','L','L'),  
    0,      
    0               },  0L      },

{{  CHAR4('.',' ',' ',' '),  
    CHAR4('D','O',' ',' '), 
    0               },  0L      },

{{  CHAR4('I','F',' ',' '),  
    CHAR4('?',' ',' ',' '), 
    0               },  0L      },

{{  CHAR4('B','O','O','T'),
    0,
    0               },  0L      },

{{  CHAR4('R','U','N',' '),
    0,
    0               },  0L      },

{{  CHAR4('R','O','M',' '),
    0,
    0               },  0L      },


    /* End-of-table marker is all zeroes */
{{   0,      0,      0      },  0L      }};


//  Initialize the dispatch vector for console commands

LONGWORD console_dispatch_init(void)
{
    
    console_dispatch_table[ 0 ].handler = console_init;
    console_dispatch_table[ 1 ].handler = ( console_handler )-1L;    /* Totally DCL-driven */
    console_dispatch_table[ 2 ].handler = console_set;
    console_dispatch_table[ 3 ].handler = ( console_handler )-1L;    /* Totally DCL-driven */
    console_dispatch_table[ 4 ].handler = console_asm;
    console_dispatch_table[ 5 ].handler = console_disasm;
    console_dispatch_table[ 6 ].handler = console_exec;
    console_dispatch_table[ 7 ].handler = console_exam;
    console_dispatch_table[ 8 ].handler = console_zero;
    console_dispatch_table[ 9 ].handler = help;
    console_dispatch_table[ 10 ].handler = console_save;
    console_dispatch_table[ 11 ].handler = ( console_handler )-1L;   /* Totally DCL-driven */
    console_dispatch_table[ 12 ].handler = console_step;
    console_dispatch_table[ 13 ].handler = console_run;
    console_dispatch_table[ 14 ].handler = console_load;
    console_dispatch_table[ 15 ].handler = console_time;
    console_dispatch_table[ 16 ].handler = console_include;
    console_dispatch_table[ 17 ].handler = console_print;
    console_dispatch_table[ 18 ].handler = ( console_handler )-1L;    /* Totally DCL-driven */
    console_dispatch_table[ 19 ].handler = ( console_handler) -1L;
    console_dispatch_table[ 20 ].handler = console_call;
    console_dispatch_table[ 21 ].handler = console_do;
    console_dispatch_table[ 22 ].handler = console_if;
    console_dispatch_table[ 23 ].handler = console_boot;
    console_dispatch_table[ 24 ].handler = console_run;
    console_dispatch_table[ 25 ].handler = console_rom;
    

    return VAX_OK;
}



//
//  ALLOC_VAX
//
//  Allocate a new VAX processor

LONGWORD alloc_vax( LONGWORD Physmem )
{
    LONGWORD i, physmem, n;
    extern char * rom, *rom_name, * nvram, * nvram_name, * microvax;
    extern char * page_map;
    extern long mapsize;
        
    if( microvax == 0L )
        microvax = getmem( 256 );
        
    /* Can't create less than 8K system.  Also, normalize memory to be a 512-byte boundary */
    
    
    physmem = ( Physmem & 0xFFFFFE00UL );
    if( physmem < 8192L )
        physmem = 8192L;
    
    if( physmem != Physmem ) 
        physmem = physmem + 0x000000200;
            
    vax.memory = ( unsigned char * ) getmem( physmem );
    if( vax.memory == 0L )
        return VAX_MEM;

    // printf( "ALLOC VAX MEMORY, %ld BYTES AT %08lX\n", physmem, ( long ) vax.memory );
    
    /*  Initialize the register file (0-15 plus 48 temporaries) */
    
    for( i = 0; i <= MAXREG; i++ )
        vax.reg[ i ] = 0L;
    
    /*  Initialize the privileged register array */
    
    for( i = 0; i <= MAXPRIVREG; i++ )
        vax.preg[ i ] = 0L;
        
    vax.treg = MAXREG;
    
    vax.console.symbols = 0L;
    vax.console.radix = 16;
    vax.console.disasm = 0;
    vax.console.assembler_mode = 0;
    vax.console.verify = 0;
    vax.console.running = 1;
    vax.console.deposit = 0x00000200;     /* Initial deposit/asm location */
    vax.console.p0_deposit = 0x00000200;
    vax.console.s0_deposit = 0x80000000;

    vax.PC = 0x00000200; /* Same as base deposit location */

    vax.console.breakpoint_list = 0L;     /* No breakpoints defined */
    vax.console.flags = CONSOLE_EXPAND | CONSOLE_VERBOSE;
    vax.console.tbcache = 1;              /* Initially enable TB caching */
    vax.console.vmtrace = 0;              /* Disable VM tracing */

    vax.console.CALL_active = 0;          /* Not running under CALL */
    vax.console.CALL_PC = 0L;

    vax.console.comment = CONSOLE_COMMENT;            /* Comment delimiter */
    vax.console.separator = CONSOLE_SEPARATOR;        /* Command separator */

    vax.console.prompt = 0L;              /* Use default prompt */
    vax.console.microkernel_valid = 0;    /* Microkernel not valid */
    vax.console.vminit_valid = 0;
    
    vax.console.min_deposit = 0x7FFFFFFFUL;           /* __FIRST */
    vax.console.max_deposit = 0x00000000UL;           /* __LAST  */

    vax.console.pid = 0x00000001;
    vax.console.uic = 0x00010001;
    
/*
 *  Default location for finding a sharable image
 */

    strcpy( vax.console.share_prefix, "share/" );

#ifdef VMS
    strcpy( vax.console.share_prefix, "EVAX$SHARE:" );
#endif

#ifdef macintosh
    strcpy( vax.console.share_prefix, ":share:" );
#endif

#ifdef WIN
    strcpy( vax.console.share_prefix, "share\\" );
#endif

    vax.assembler.cur_entry[ 0 ] = 0;    
    vax.assembler.location = 0L;
    vax.assembler.displacement = 0;
    vax.assembler.flags = ASM_WARNFORWARD | ASM_SYMBOLS | 
                                    ASM_REGISTER_EXPR | ASM_BRANCHDEST | ASM_SCOPENAMES;
                                    
    vax.assembler.dialect = ASM_DIALECT_ANY;
    
    vax.instruction_PC = 0L;
    
    vax.fault.code = 0;                   /* No exception occurred yet */
    vax.fault.pc = 0L;
    vax.fault.psl = 0L;
    vax.fault.signal_args[ 0 ] = 0;       /* No arguments to exception */
    vax.interrupt_pending = 0L;
    vax.interrupt_ipl = 0;
    vax.iqueue = 0L;
    vax.fault_pending = 0;
    
    vax.debug = DBG_REGISTERS |             /* Show changed registers at STEP */
                        DBG_USERHALT |             /* HALT in user mode is okay      */
                        DBG_LIBINIT;               /* LIB$INITIALIZE will be called  */
                        
    vax.halted = 1;                        /* CPU not running, etc. */
    vax.console.singlestep = STEP_NONE;    /* Not currently single stepping */
    vax.console.stepmode = STEP_INSTRUCTION; /* By default, STEP means STEP/IN */
    
#if macintosh
    vax.timebase = TickCount();
#else
    vax.timebase = 0L;
#endif

#if HAS_HARDWARE_CLOCK
    vax.quantum.current = 1;
    vax.quantum.initial = 1;	/* This is handled via ualarm() interrupt */
#else
    vax.quantum.current = 20;
    vax.quantum.initial = 20;	/* Or handled every 'n' instructions */
#endif

    vax.uiquantum.current = 0L;
    vax.uiquantum.initial = 3000;       /* Good for a 400mhz+ system, make smaller for  */
                                                /* significantly slower systems...              */
    
    
    vax.SISR = 0L;                /* Software interrupt status register */
    vax.SIRR = 0L;
    vax.IPL = 0x0000001FL;
    vax.SCBB = 0L;
    vax.TODR = 0L;
    vax.ASTLVL = 4;
    vax.TXCS = 0x00000080;        /* Show ready to send byte to terminal  */
    vax.SID = 0x13000202;         /* Let's be a VAXstation 4000-90 */
                                    
    vax.PC = 0L;
    vax.memory[ 0 ] = 0;
    vax.memsize = physmem;
    vax.rom_base = -1L;
    vax.rom_end = -1L;
    vax.nvram_base = -1L;
    vax.nvram_end = -1L;
    
    if( !rom_name )
        rom_name = getmem( 256 );
    if( !nvram_name )
        nvram_name = getmem( 256 );
        
    if( rom )
        freemem( rom );
    if( nvram )
        freemem( nvram );
        
    rom = 0L;
    nvram = 0L;
    
    vax.SP = physmem - 4; /* Stack pointer starts at high end of memory by default */
    vax.ISP = vax.SP;   /*  Interrupt stack points to same place, and so on */
    vax.KSP = vax.SP;
    vax.SSP = vax.SP;
    vax.ESP = vax.SP;
    vax.USP = vax.SP;
    
    vax.vm_initialized = 0;   /* VM has never been console initialized */

    vax.TBDR = 0;         /* translation buffer caching is NOT disabled */
     
    
    init_emulators();
    
    if( system_symbols == 0L ) {
        shim_init();
        init_system_symbols( );
    };
    
     if( page_map )
        freemem( page_map );
    mapsize = ( physmem >> 9 ) + 1;
    page_map = getmem( mapsize );
    for( n = 0; n < mapsize; n++ )
        page_map[ n ] = 0;
        
    vax_init = 1;

   
    vax.psl.reg = 0L;
    read_psl_bits();   /* Set up cache from initial bit state */

    vax.pslw.cur_mod = AM_KERNEL;
    vax.pslw.prv_mod = AM_KERNEL;
    vax.pslw.ipl = 31;
    
    write_psl_bits();  /* Update bits to match processor cache */
    

    return VAX_OK;
        
}

//
//  Free a VAX processor.

LONGWORD free_vax( struct VAX * theVAX )
{
    struct SYMBOL * sym, *next_sym;
    struct FSYMBOL * fp, *fnext;
    struct INTERRUPT * ip, *ipnext;
    struct BREAKSTR * breakpoint, *next_bp;

    // printf( "FREE  VAX MEMORY, %ld BYTES AT %08lX\n", vax.memsize, ( long ) vax.memory );
        
    if( vax.memory == 0L )
        return VAX_OK;
        
    for( ip = vax.iqueue; ip; ip = ipnext ) {
        ipnext = ip-> next;
        freemem( ip );
    }
    
    for( breakpoint = vax.console.breakpoint_list; breakpoint; breakpoint = next_bp ) {
        next_bp = breakpoint-> next;
        freemem( breakpoint );
        
    }
    vax.console.breakpoint_list = 0L;

    for( sym = vax.console.symbols; sym; sym = next_sym ) {
        next_sym = sym-> next;
        
        for( fp = sym-> forward; fp; fp = fnext ) {
            fnext = fp-> next;
            freemem( fp );
        }
        
        freemem( sym );
    }

    if( vax.console.prompt )
        freemem( vax.console.prompt );
     
    freemem( vax.memory );

    return VAX_OK;
    
}

