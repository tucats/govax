//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_show.c
//
//  Purpose:    This module implements the SHOW commands.  These commands
//              display the state of the virtual VAX processor or data about
//              the console processor.
//
//  History:    08/06/97    New header format standardization
//
//              02/01/98    Added SHOW ASM command.  Added printbit formatting routine.
//
//              01/06/99    Add additional ASM mode displays.  Make the
//                          breakpoint display resolve symbols.
//
//              01/07/99    Added SHOW CALLS to display call frames
//
//              01/08/99    Handle "console generated" frames better
//
//              01/11/99    Added SHOW ASM attributes.
//
//              02/01/99    Added SHOW UNIMPLEMENTED INSTRUCTIONS as a tool
//                          to help development.
//
//              02/12/99    Added SHOW PAGE command
//
//              05/03/99    Fix up SHOW STACK so that it stops when it
//                          gets to a page that's protected.  We now
//                          mark the high-address page of each stack as
//                          an inaccessible page.
//
//              05/11/99    Added SHOW STRINGS command that dumps the
//                          assembler's string pool.
//
//              05/25/99    Modified format of SHOW SYS SYM and SHOW SYM
//                          to include more symbol format modifiers.
//
//              05/25/99    Added SHOW INTERRUPT
//
//              06/03/99    Added DBG_REGISTERS to the SHOW DEBUG display.
//
//              06/14/99    Added fault breakpoints to SHOW BREAKPOINTS
//
//              07/13/99    Added SHOW MMAP command to dump struct maps.
//
//              09/21/99    Added sequential translation cache to SHOW TB
//
//              09/26/99    Added SHOW ROM and SHOW NVRAM.
//
//              11/08/99    Updated SHOW BREAK to know about temporary breaks.
//
//              11/14/99    Changed parsing to use DCLRTL.
//
//              12/06/99    Fixed bugs in SHOW INSTR command's handling of
//                          a specific opcode as a parameter.
//
//              12/15/99    Added SHOW MEMORY/RUNTIME.  Fixed bug in SHOW STACK.
//
//              01/10/00    Made SHOW SHARE aware of NOSHARE case.
//
//		04/24/01    Added SHOW COMMAND_ARGS
//
//		10/10/01    Added SHOW WATCHPOINTS
//
//		11/02/01    Added SHOW BREAK/INSTRUCTIONS
//

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "vaxinstr.h"
#include "pte.h"
#include "shim.h"
#include "memmap.h"
#include "dclrtl.h"

extern char * pr_names[];

static char * printbit( LONGWORD flag, LONGWORD mask, LONGWORD normal, 
                                    char * name, char * desc );
static void show_modes( void );
void show_faults( void );
                
static void show_instructions( LONGWORD match );
LONGWORD tracevm( ULONGWORD Addr, short mode );
static LONGWORD showscb( LONGWORD all );
void dump_tb(void);
struct SYMBOL * find_label( LONGWORD dest, LONGWORD flags );
LONGWORD mapped_pages( void );

extern LONGWORD tb_hit, tb_try, tb_flush, tb_pflush;
void dump_icb_list(int);



//  The SHOW command DCL handler.

LONGWORD console_show_dcl( long id )
{
    LONGWORD rc, opmatch;
    char * p, *pn, *ps, *tmp;
    LONGWORD n, i, j, value, mode, match;
    LONGWORD all, unimplemented;
    struct BREAKSTR * breakpoint;
    struct SYMBOL * sym, *sp;
    char buff[ 256 ], m[ 32 ], *bp;
    ULONGWORD addr;
    float ratio;
    LONGWORD desc_len, count;
    ULONGWORD desc_addr, desc;
    char attr[ 256 ];

    LONGWORD stack_id;
    ULONGWORD stringpool_base, stringpool_size, stringpool, stringaddr;
    LONGWORD saved_mode, saved_is;

    static char * mode_names[ 5 ] = {"KERNEL", "EXEC", "SUPER", "USER", 
                                       "INTERRUPT" };

/*
 *  If there is no active virtual we are done!
 */
 
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }

/*
 *  Many of the SHOW commands can display the TODR register explicilty
 *  or implicitly, so let's update it now.  We only do this if the register
 *  is already non-zero in value (VAX 11/750 standard).
 */
    
#if 0  /* millisecond timer now handles this */
    if( vax.TODR != 0L ) {
            /* Difference is 60th's of a second, make seconds them 1000's */
        vax.TODR = (( TickCount() - vax.timebase ) * 100 ) /60;
    }
#endif
                
    vax.ICR = vax.clock;
    count = 1;
    rc = VAX_OK;
        
/*
 *  Dispatch based on the parameter verb.
 */
 
    switch( id ) {

    case 411:       /* SHOW WATCHPOINTS */

        show_watchpoints();
        break;
    
    case 412:	    /* SHOW BREAK/INSTR */
    
        count = 0; 
        for( n = 0; n < 512; n++ ) {
            if( instruction[n].name[0] == 0 )
                break;
            if( instruction[ n ].debugdata & OP_DBG_BREAK ) {
                count++;
                printf( "    %02X  %s\n" ,
                    instruction[n].opcode,
                    instruction[n].name );
            }
        }
        
        if( count == 0 )
            printf( "No instruction breakpoints set\n" );
        else
            printf( "%d instruction breakpoint%s set\n",
                count, count == 1 ? "" : "s" );
        break;
        
    case 120:       /* SHOW <register> */
    
            //  Search the list of privileged registers to see if it's a match
            
        strcpy( buff, DCLgetstring( DCL_CALLBACK_PARAMETER, 160 ));
        match = 0;
        for( n = 5; n < MAXPRIVREG; n++ ) { /* Note we start after xSP registers */
        
        
            if( strcmp( buff, pr_names[ n ] ) == 0 ) {
                match = 1;
                break;
            }
        }

        //  If a match was found, it's a privileged register and we're done.
            
        if( match ) {
            printf( "   PREG %6s = %08X (hex)  %12u (dec)\n", pr_names[ n ], 
                vax.preg[ n ], vax.preg[ n ] );
            return VAX_OK;
        }
/*
 *  See if it's a request to examine a register by name 
 */
     
        rc = exam_reg_string( buff );
        if( rc == VAX_OK )
            return rc;
        printf( "DEBUG: unexpected SHOW keyword failure\n" );
        return VAX_SYNTAX;


    case 121:       /* SHOW STRING */
        if( !MKVALID )
            return VAX_NOMK;
            
        strcpy( buff, "CONSOLE$STRINGPOOL" );
        bp = buff;
        rc = get_symbol( &bp, ( LONGWORD * ) &stringpool );
        if( rc )
            return rc;

        strcpy( buff, "CONSOLE$STRINGPOOL_BASE" );
        bp = buff;
        rc = get_symbol( &bp, ( LONGWORD * ) &stringpool_base );
        if( rc )
            return rc;

        strcpy( buff, "CONSOLE$STRINGPOOL_SIZE" );
        bp = buff;
        rc = get_symbol( &bp, ( LONGWORD * ) &stringpool_size );
        if( rc )
            return rc;

        printf( "STRING POOL\n" );
        printf( "    CONSOLE$STRINGPOOL_BASE       %08X\n", stringpool_base );
        printf( "    CONSOLE$STRINGPOOL_SIZE       %08X\n", stringpool_size );
        printf( "    CONSOLE$STRINGPOOL (Current)  %08X\n", stringpool );

        rc = load_memory( stringpool_base, ( void * ) &stringaddr, 4 );

        if( stringaddr )
            printf( "\n" );

        while( stringaddr ) {

            rc = load_memory( stringaddr, ( void * ) &desc_len, 4 );
            if( rc )
                return rc;
            desc_len = desc_len & 0x0000FFFF;

            rc = load_memory( stringaddr+4, ( void * ) &desc_addr, 4 );
            if( rc )
                return rc;

            desc = stringaddr;
            rc = load_memory( stringaddr+8, ( void * ) &stringaddr, 4 );
            if( rc )
                return rc;

            if( desc_len <= 250 )
                buff[ desc_len ] = 0;

            for( n = 0; n < desc_len; n++ ) {
                if( n > 250 ) {
                    buff[ 250 ] = 0;
                    strcat( buff, "..." );
                    break;
                }
                rc = load_memory( desc_addr+n, 
                          ( void *) &( buff[ n ]), 1 );
                if( rc )
                    return rc;
            }

            printf( "    Descriptor %08X [%3d bytes]  \"%s\"\n",
                    desc, desc_len, buff );
        }

        printf( "\n" );

        break;
        

    case 122:   /* SHOW NVRAM */
        {
            extern char * nvram, * nvram_name;
            
            if( nvram == 0L ) {
                printf( "No NVRAM initialized\n" );
                break;
            }
            
            printf( "    NVRAM  FILE=\"%s\"\n", nvram_name );
            printf( "        CONSOLE$NVRAM_BASE = %08X\n", vax.nvram_base );
            printf( "        CONSOLE$NVRAM_END  = %08X\n", vax.nvram_end );
            printf( "        CONSOLE$NVRAM_SIZE = %08X (%uK)\n", 
                 ((vax.nvram_end+1)-vax.nvram_base),
                  ((vax.nvram_end+1)-vax.nvram_base) /1024 );
            break;
        }
        
    case 154:   /* SHOW ROM */
    
        {
            extern char * rom_name;
            extern char * rom;
            
            if( rom == 0L ) {
                printf( "No ROM loaded\n" );
                break;
            }
            
            printf( "    ROM FILE=\"%s\"\n", rom_name );
            printf( "        CONSOLE$ROM_BASE  = %08X\n", vax.rom_base );
            printf( "        CONSOLE$ROM_END   = %08X\n", vax.rom_end );
            printf( "        CONSOLE$ROM_SIZE  = %08X (%dK)\n",
                ( LONGWORD ) (( vax.rom_end + 1 ) - vax.rom_base ),
                ( LONGWORD ) (( vax.rom_end + 1 ) - vax.rom_base ) / 1024 );
            break;
        }
        
    case 123:   /* SHOW ERROR */
    
        /* If the parameter is present, use the value */
        
        if( DCLpresent( DCL_CALLBACK_PARAMETER, 1005 ) == 1) {
            p = DCLgetstring( DCL_CALLBACK_PARAMETER, 1005 );
            rc = asm_expr( &p, &value );
            if( rc )
                return rc;
            printf( "Error code %08X, \"%s\"\n", value, vaxmsg( value ));
        }
        
        /*  Or use last command return code */
        get_symbol_direct( "$STATUS", &value );
        printf( "Last error code was %08X, \"%s\"\n", value, vaxmsg( value ));
        break;
        
    case 124:   /* SHOW MODE */

        printf( "Current MODE is %s\n", mode_names[ vax.pslw.cur_mod ] );
        break;

    case 125:   /* SHOW SHIM */

        if( !MKVALID )
            return VAX_NOMK;
            
    
        rc = shim_dump( );
        break;
        
    case 126:   /* SHOW PAGE */

        p = DCLgetstring( DCL_CALLBACK_PARAMETER, 1008 );
        rc = asm_expr( &p, &value );
        if( rc )
            return rc;

        mode = 0;   /* Assume /READ mode */
        
        /* See if /WRITE was given instead */
        if( DCLpresent( DCL_CALLBACK_QUALIFIER, 1006 ) == 1)
            mode = 1;
            
        rc = tracevm( ( ULONGWORD ) value, ( short ) mode );
        break;
        
    case 155:   /* SHOW TB */

        {
            extern LONGWORD cached_page_hit;
            extern LONGWORD cached_page_try;
            
            if( cached_page_try == 0L )
                ratio = 0.0;
            else {
                ratio = ( float ) cached_page_hit;
                ratio = ( float ) ( ratio / ( float ) cached_page_try * 100.0 );
            }
            printf( "Sequential Translation Cache\n" );
            printf( "    Tries=%d    Hits=%d    Misses=%d    Ratio = %d%%\n",
                    cached_page_try, cached_page_hit,
                    ( cached_page_try - cached_page_hit ), ( LONGWORD ) ratio );
        };

        printf( "\nTranslation buffer caching is %s\n",
          vax.TBDR ? "disabled" : "enabled" );

        if( tb_try == 0L ) 
            ratio = 0.0;
        else {
            ratio = ( float ) tb_hit;
            ratio = ( float ) ( ratio / ( float ) tb_try * 100.0 );
        }
        printf( "    Tries=%d    Hits=%d    Misses=%d    Ratio = %d%%\n",
            tb_try, tb_hit, tb_try - tb_hit, ( LONGWORD ) ratio );
        printf( "    Flushes=%d   PFlushes=%d\n", tb_flush, tb_pflush );
        
        dump_tb();

        
        break;

    case 127:   /* SHOW CALL_FRAMES */

        /* See if a count was given */
        
        if( DCLpresent( DCL_CALLBACK_PARAMETER, 1009 )) {
            p = DCLgetstring( DCL_CALLBACK_PARAMETER, 1009 );
            rc = asm_expr( &p, &value );
            if( rc )
                return rc;
        }
        else
            value = 1;

        rc = show_calls( value );
        break;

    case 128:   /* SHOW QUANTUM */

        printf( "QUANTUM\n    INTERRUPTS  Initial=%d  Current=%d\n",
                vax.quantum.initial, vax.quantum.current );
        
        printf( "    USER INTF   Initial=%d  Current=%d\n",
                vax.uiquantum.initial, vax.uiquantum.current );

        break;

    case 156:   /* SHOW MAP */    
        map_dump();
        break;
        
    case 129:    /* SHOW DEBUG */
    
        printf( "DEBUG SETTINGS:\n" );
        
        printf( "    %s\n", printbit( vax.debug,
                                    DBG_DEBUG,
                                    0, 
                                    "DEBUG",
                                    "Invoke native debugger?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_VM,
                                    0, 
                                    "VM",
                                    "Debug virtual memory translations?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_TB,
                                    0, 
                                    "TB",
                                    "Debug translation buffer caching?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_SYMBOLS,
                                    0, 
                                    "SYMBOLS",
                                    "Debug symbol table handling?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_EXCEPTIONS,
                                    0, 
                                    "EXCEPTIONS",
                                    "Debug exception handling?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_INTERRUPTS,
                                    0,
                                    "INTERRUPTS",
                                    "Debug interrupt handling?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_CHM,
                                    0,
                                    "CHM",
                                    "Debug change-mode operations?" ));
        
        printf( "    %s\n", printbit( vax.debug,
                                    DBG_REGISTERS,
                                    0, 
                                    "REGISTERS",
                                    "Display changed registers on STEP?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_FULLDISASM,
                                    0, 
                                    "FULLDISASM",
                                    "Display operand values on disasm?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_USERHALT,
                                    0,
                                    "USERHALT",
                                    "HALT in user mode halts CPU?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_KBD,
                                    0,
                                    "KEYBOARD",
                                    "Debug console keyboard input?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_IMAGES,
                                    0, 
                                    "IMAGES",
                                    "Display image info on RUN command?" ));
        
        printf( "    %s\n", printbit( vax.debug,
                                    DBG_SERVICES,
                                    0,
                                    "SERVICES",
                                    "Debug P1 system service calls?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_DCL,
                                    0,
                                    "DCL",
                                    "Debug DCL parsing?" ));

        printf( "    %s\n", printbit( vax.debug,
                                     DBG_EXPAND,
                                     0,
                                     "COMMAND",
                                     "Display command line expansions?" ));

        printf( "    %s\n", printbit( vax.debug,
                                     DBG_LOGICALS,
                                     0,
                                     "LOGICALS",
                                     "Debug logical name operations?" ));

        printf( "    %s\n", printbit( vax.debug,
                                     DBG_DEVICES,
                                     0,
                                     "DEVICES",
                                     "Debug device operations?" ));

        printf( "    %s\n", printbit( vax.debug,
                                     DBG_PROCESS,
                                     0,
                                     "PROCESSES",
                                     "Debug process operations?" ));

        printf( "    %s\n", printbit( vax.debug,
                                     DBG_LIBINIT,
                                     0,
                                     "LIBINIT",
                                     "Invoke LIB$INITIALIZE for images?" ));

        printf( "    %s\n", printbit( vax.debug,
                                    DBG_RMS,
                                    0, 
                                    "RMS",
                                    "Debug RMS operations?" ));
        

        break;

        
    case 130:   /* SHOW ASSEMBLER_SETTINGS */
    
        printf( "ASSEMBLER SETTINGS:\n" );
        
        printf( "    %s\n", printbit( vax.assembler.flags,
                                    ASM_ADDRPROMPT, 
                                    0, 
                                    "ADDRESS_PROMPT",
                                    "Include address in ASM prompt?" ));
        
        printf( "    %s\n", printbit( vax.assembler.flags,
                                    ASM_WARNFORWARD,
                                    0,
                                    "FORWARD_WARNINGS",
                                    "Warn if unresolved references after ASM END" ));
        
        printf( "    %s\n", printbit( vax.assembler.flags,
                                    ASM_BRANCHDEST,
                                    0,
                                    "BRANCH_DESTINATION",
                                    "Display branch destination addresses" ));

        printf( "    %s\n", printbit( vax.assembler.flags,
                                    ASM_SYMBOLS,
                                    0,
                                    "SYMBOLS",
                                    "Display addresses symbolically" ));

        printf( "    %s\n", printbit( vax.assembler.flags,
                                    ASM_RESTMP,
                                    0,
                                    "RESOLVE_TEMP",
                                    "Require temporary sym resolution in scope"));

        printf( "    %s\n", printbit( vax.assembler.flags,
                                    ASM_SCOPENAMES,
                                    0,
                                    "SCOPENAMES",
                                    "Use scope names in temp symbols" ));

        printf( "\n" );
        
        break;
        
            
    case 131:   /* SHOW INSTRUCTION */
    

        /* SHOW INST/PROF */
        
        if( DCLpresent( DCL_CALLBACK_QUALIFIER, 1010 )) {
            show_instructions( -2 );
            return VAX_OK;
        }
        
        /*  SHOW INSTR/MODE */
        
        if( DCLpresent( DCL_CALLBACK_QUALIFIER, 1009 )) {
            show_modes();
            return VAX_OK;
        }
        
        p = DCLgetstring( DCL_CALLBACK_PARAMETER, 1013 );
        opmatch = -1;
        if( p != 0L ) {
            rc = asm_hex( &p, ( void * ) &opmatch );
            if( rc != VAX_OK )
                return rc;
        }

        all = ( DCLpresent( DCL_CALLBACK_QUALIFIER, 1012 ) == 1 );
        unimplemented = ( DCLpresent( DCL_CALLBACK_QUALIFIER, 1011 ) == 1 );

        if( !all || opmatch == -1 )
            printf( "INSTRUCTION OPCODES:\n\n" );
        
        /* The default is to just show the opcode values */
        count = 0;
        if( !all && opmatch == -1 ) {
            n = 0;
            for( i = 0;  i < 500; i++ ) {
                pn = instruction[ i ].name;
                
                if( *pn == 0L )
                    break;

                /* IF the 'unimplemented' flag is set, then the routine */
                /* must not be implemented, else loop again.            */

                if( unimplemented && instruction[ i ].routine != emul_unimplemented )
                    continue;

                /* Likewise if the flag is not set then the routine MUST */
                /* be implemented or we loop again.                      */

                if( !unimplemented && instruction[ i ].routine == 0L )
                    continue;

                /* If it's not an instruction, skip it... */

                if( strncmp( instruction[ i ].name, "RSVD_", 5 ) == 0 )
                    continue;
                if( strncmp( instruction[ i ].name, "EXT_", 4 ) == 0 )
                    continue;

                count++;
                if( instruction[ i ].extended )
                    printf( "   %02X %02X %-8s  ",
                        instruction[ i ].extended,
                        instruction[ i ].opcode,
                        instruction[ i ].name );
                else
                    printf( "      %02X %-8s  ",
                        instruction[ i ].opcode,
                        instruction[ i ].name );
                n++;
                if( n == 4 ) {
                    printf( "\n" );
                    n = 0;
                }
            }
            if( n )
                printf( "\n" );
            printf( "\n%d Instructions.\n", count );

        }
        else
            show_instructions( opmatch );
            
        break;
                
    case 132:   /* SHOW BREAK */
    
        if( !vax.console.breakpoint_list ) {
            printf( "No breakpoints defined\n" );
            break;
        }
        
        printf( "    Breakpoints:\n" );
        for( breakpoint = vax.console.breakpoint_list; breakpoint; breakpoint = breakpoint-> next ) {

            buff[ 0 ] = 0;
            if( breakpoint-> kind == BREAK_FAULT ) {
                sprintf( buff, "%s", vaxexcept(( short ) breakpoint-> pc ));
            }
            else {
                for( sp = vax.console.symbols; sp; sp = sp-> next ) {
    
                    if((( ULONGWORD ) sp-> value == ( ULONGWORD ) breakpoint-> pc - 2  ) &&
                        ( sp-> flags & SYM_ENTRY ))
                        break;
    
                    if((( ULONGWORD ) sp-> value == ( ULONGWORD ) breakpoint-> pc ) &&
                       ( sp-> flags & SYM_LABEL ))
                        break;
                }
    
                if( !sp )
                for( sp = system_symbols; sp; sp = sp-> next ) {
    
                    if((( ULONGWORD ) sp-> value == ( ULONGWORD ) breakpoint-> pc - 2  ) &&
                        ( sp-> flags & SYM_ENTRY ))
                        break;
    
                    if((( ULONGWORD ) sp-> value == ( ULONGWORD ) breakpoint-> pc ) &&
                       ( sp-> flags & SYM_LABEL ))
                        break;
                }
                
                if( sp )
                    strcpy( buff, sp-> name );
            }
            
            tmp = "";
            if( breakpoint-> kind & BREAK_STEP )
                tmp = "<step>";
            else
            if( breakpoint-> kind & BREAK_TEMPORARY )
                tmp = "<temporary>";
                
            if( breakpoint-> after )
                printf( "  %c %08X (%3d) %s %s\n", 
                    (( BREAK_TYPE & breakpoint-> kind ) == BREAK_FAULT) ? 'F' : ' ',
                    breakpoint-> pc,
                        breakpoint-> after, buff, tmp );
            else
                printf( "  %c %08X       %s %s\n", 
                    (( BREAK_TYPE & breakpoint-> kind )== BREAK_FAULT) ? 'F' : ' ',
                        breakpoint-> pc, buff, tmp );

        }
        break;
        
    case 133:   /* SHOW REGISTERS */

        rc = dump_registers();
        break;
    
    case 134:   /* SHOW STEP */
    
        switch( vax.console.stepmode ) {
                
        case STEP_OVER:
            ps = "OVER";
            break;
        
        case STEP_RETURN:
            ps = "RETURN";
            break;
        
        default:
            ps = "INTO";
        }
        
        printf( "Default is STEP/%s\n", ps );
        rc = VAX_OK;
        break;
        
    case 135:   /* SHOW PSL */

        rc = dump_psl();
        break;
        
    case 136:   /* SHOW CPU */
    
        rc = dump_registers();
        if( rc )
            break;
        
        printf( "\n    Stack pointers:\n\n" );
        printf( "    USP: %08X   SSP: %08X   ESP: %08X   KSP: %08X\n    ISP: %08X\n\n",
                    vax.USP, vax.SSP, vax.ESP, vax.KSP,
                    vax.ISP );
        
        //  Print the privileged registers as a block.  Each time the output buffer
        //  line gets LONGWORD enough, dump it out.
        
        strcpy( buff, "    " );
        
        printf( "    Privileged registers:\n\n" );
        for( n = 5; n < MAXPRIVREG; n++ ) {
            pn = pr_names[ n ];
            if( *pn == '_' )
                continue;
            
            sprintf( m, "    %-6s:  %08X", pn, vax.preg[ n ] );
            strcat( buff, m );
            if( strlen( buff ) > 60 ) {
                strcat( buff, "\n" );
                printf( "%s", buff );
                strcpy( buff, "    " );
            }   
        }
        
        //  If there's stuff left in the output buffer, flush it now.  We check the
        //  [1] character because it would be null if the buffer was only initialized
        //  to a tab character.
        if( buff[ 1 ] ) {
            strcat( buff, "\n" );       
            printf( "%s", buff );
        }
    
         
        //  Fall through to SHOW BASE ? */
        
    case 137:   /* SHOW BASE */
    
        printf( "\n        Next storage address is %08X\n", vax.console.deposit );
        break;
    
    case 138:   /* SHOW MEMORY */

        if( DCLpresent( DCL_CALLBACK_QUALIFIER, 1016 ) == 1 ) {
            printmem();
            return VAX_OK;
        }

        if( DCLpresent( DCL_CALLBACK_QUALIFIER, 1010 ) == 1 ) {
            n = DCLpresent( DCL_CALLBACK_QUALIFIER, 1011 ) == 1;
            if( MKVALID )
                return decc_dump_memory( (int) n );
            else
                return VAX_NOMK;
        }
        
        show_regions( );
            
        break;
    
    case 144:   /* SHOW ISP */
    case 143:   /* SHOW KSP */
    case 142:   /* SHOW ESP */
    case 141:   /* SHOW SSP */
    case 140:   /* SHOW USP */
    case 139:   /* SHOW STACK */
        
        saved_mode = 0L;
        count = ( vax.preg[ vax.pslw.cur_mod ] - vax.SP ) >> 2;

        if( id != 139 /* STACK */ ) {
            saved_mode = vax.pslw.cur_mod;
            saved_is   = vax.pslw.is;

            /* Because a mode switch can happen, dump TB protection cache */
            invalidate_tb_prot(); 
            switch( id ) {

            case 143 /* KSP */ :  set_mode_stack( 0 ); break;
            case 142 /* ESP */ :  set_mode_stack( 1 ); break;
            case 141 /* SSP */ :  set_mode_stack( 2 ); break;
            case 140 /* USP */ :  set_mode_stack( 3 ); break;
            case 144 /* ISP */ :  set_mode_stack( 4 ); break;
            }
        }

        all = ( DCLpresent( DCL_CALLBACK_QUALIFIER, 1030 ) == 1 );
        if( !all ) {
            if( count < 1 || count > 255 )
                count = 1;
            if( DCLpresent( DCL_CALLBACK_PARAMETER,1031 ) == 1 ) {
                p = DCLgetstring(  DCL_CALLBACK_PARAMETER, 1031 );
                rc = asm_hex( &p, ( ULONGWORD * ) &count );
                if( rc )
                    goto stack_exit;
            }
                
        }
        
        if( vax.pslw.is )
            stack_id = 4;
        else
            stack_id = vax.pslw.cur_mod;

        printf( "    %s MODE SP = %08X:\n", 
                   mode_names[ stack_id ], vax.SP );
        
        count = count * 4;
        
        for( n = 0; n < count; n += 4 ) {
        
            addr = vax.SP + n;
            
            /* If virtual memory is on, then the stack top is end of P1 */
            /* if we are in USER mode.  If virtual memory is off, then  */
            /* it's the end of physical memory. */
            
            if(( vax.pslw.cur_mod == 3 && vax.MAPEN && addr >= 0x7FFFFFFF ) || 
               ( vax.MAPEN == 0 && ( addr >= ( ULONGWORD ) vax.memsize ))) {
                printf( "      <end of memory>\n" );
                goto stack_exit;
            }

            if( probe( addr, 4, VM_READ | VM_NOSIGNAL )) {
                printf( "      <end of stack>\n" );
                goto stack_exit;
            }
            
            rc = load_memory( addr, ( unsigned char * ) &value, 4 );
            if( rc )
                goto stack_exit;
                
            if( vax.console.radix == 16 ) 
                printf( "      SP+%04X [%08X]:  %08X\n", n, addr, value );
            else
                printf( "      SP+%04d [%08X]:  %d\n", n, addr, value );

        }
        
stack_exit: if( id != 139 /* STACK */ ) {
                set_mode_stack( saved_mode );
            }

        break;
        
    case 145:   /* SHOW FAULT */
    
#if 0  // Replaced with show_faults()
        
        if( vax.fault.code == 0 ) {
            printf( "    No exceptions or interrupts have occurred yet\n" );
        }
        else {        
            printf( "    Last exception/interrupt code was %08lX, %s\n        %ld argument%s,", 
                    vax.fault.code, vaxexcept(( short ) vax.fault.code ),
                        vax.fault.signal_args[ 0 ],
                        vax.fault.signal_args[ 0 ] == 1 ? "" : "s" );
                        
            printf( "    PC=%08lX   PSL=%08lX\n",
                    vax.fault.pc, vax.fault.psl );
            
            
            for( n = 1; ( ULONGWORD ) n <= vax.fault.signal_args[ 0 ]; n++ ) {
                printf( "        Argument %ld is %08lX\n", n, vax.fault.signal_args[ n ] );
            }
        }
#else
        show_faults();
#endif

        if( vax.interrupt_pending ) {
            printf( "    There is a pending interrupt code %s at ipl %02X\n",
                        vaxexcept(( short ) vax.interrupt_pending) , 
                        vax.interrupt_ipl );
        }
        
        if( vax.iqueue ) {
        
            struct INTERRUPT * ip;
            
            printf( "Pending interrupts:\n" );
            printf( "    IPL  Quantum  Age Code\n" );

            for( ip = vax.iqueue; ip; ip = ip-> next ) {
                printf( "    %02X   %4d   %4d  %02X %s\n",
                        ip-> ipl, ip-> quantum, ip-> age, ip-> code, 
                        vaxexcept((short) ip-> code ));
            }
        }
        
        break;
        
    case 146:   /* SHOW RADIX */

        printf( "    Default radix is %d\n", vax.console.radix );
        break;
    
    case 147:   /* SHOW TRACE */

        printf( "    Execution trace disassembly is %s\n",
                 vax.console.disasm ? "enabled" : "disabled" );
        if( vax.console.disasm ) 
            printf( "    Register tracking is %s\n",
                 vax.debug & DBG_REGISTERS ? "enabled":"disabled" );

        break;

    case 148:   /* SHOW SCB */
    
        all = DCLpresent( DCL_CALLBACK_QUALIFIER, 1032 ) == 1;
        return showscb( all );

    case 160:   /* SHOW IMAGES */
        if( !MKVALID )
            return VAX_NOMK;
            
        dump_icb_list( (int) DCLpresent( DCL_CALLBACK_QUALIFIER, 100 ));
        break;
    
    case 161:   /* SHOW SHARE_PREFIX */
    
        if( !MKVALID )
            return VAX_NOMK;
        
        if( vax.console.share_prefix[0] == 0x1B )
            printf( "Recursive sharable image loading disabled.\n" );
        else
        if( vax.console.share_prefix[0] == 0 )
            printf( "No sharable image prefix defined\n" );
        else
            printf( "Sharable images loaded from \"%s\"\n", vax.console.share_prefix );
        break;
    
    case 162:   /* SHOW REGIONS */

        if( !MKVALID )
            return VAX_NOMK;
        else {
            static char * region_name[3] = {"P0", "P1", "S0" };
            printf( "REGION LIMITS:\n" );
            for( i = 0; i < 3; i++ ) { 
                get_region_size( (int) i, &n );
                printf( "\t%s   %08X\n", region_name[ i ], n );
            }
            printf( "\n" );
        }
        break;
        
    case 151:   /* SHOW SYMBOL/SYSTEM */
    
        rc = dump_system_symbols();
        break;
    
    case 150:   /* SHOW SYMBOL/ALL */
    
        rc = dump_symbols( 0 ); /* 0 means /NOUNRESOLVED */
        break;
    
    case 153:   /* SHOW SYMBOL/UNRESOLVED */
    
        rc = dump_symbols( 1 ); /* 1 means /UNRESOLVED */
        break;
    
    case 159:   /* SHOW COMMAND_ARGS */
    
        if( !MKVALID )
            return VAX_NOMK;
        
        rc = get_symbol_direct( "CONSOLE$ARG_FILE", &value );
        if( rc == VAX_OK )
            printf( "Executing file %s\n",  vax.console.last_symbol-> svalue );
        else
            printf( "No command line file specified.\n" );
            
        rc = get_symbol_direct( "CONSOLE$ARG_COUNT", ( LONGWORD * ) &value );
        if( rc )
            break;
        
        n = value;
        printf( "There %s %d command line argument%s.\n", 
            n == 1 ? "is" : "are", n, n == 1 ? "" : "s" );
            
        for( j = 1; j <= n; j++ ) {
             sprintf( buff, "CONSOLE$ARG_%d", j );
             rc = get_symbol_direct( buff, &value );
             if( rc )
                  break;
            printf( "Arg %d: %s\n", j, vax.console.last_symbol-> svalue );
        }
        rc = VAX_OK;
        break;
        
    case 149:   /* SHOW SYMBOL  */

        p = DCLgetstring( DCL_CALLBACK_PARAMETER, 1021 );
        rc = get_symbol_direct( p, ( LONGWORD * ) &value );
        if( rc )
            break;
        
        sym = vax.console.last_symbol;

        attr[ 0 ] = 0;

        if( sym != 0L ) {
            if( sym-> flags & SYM_PERMANENT )
                strcat( attr, "perm " );
            if( sym-> flags & SYM_ENTRY )
                strcat( attr, "entry " );
            if( sym-> flags & SYM_LABEL )
                strcat( attr, "label " );
            if( sym-> flags & SYM_LOCAL )
                strcat( attr, "local " );
            if( sym-> flags & SYM_STRING ) {
                strcat( attr, "string=\"" );
                strcat( attr, sym-> svalue );
                strcat( attr, "\" " );
             }
        }
        
        /* Dump the forward references.  If there are none print the value normally */
        
        if( dump_forward_references( sym ) == 0 )
            printf( "    %s = %08X (hex)   %12d (dec) %s\n", 
                sym-> name, value, value, attr );
            
        break;
    
    case 188:
        printf( "Command line expansion is %s\n",
            ( vax.console.flags & CONSOLE_EXPAND ) ? "enabled" : "disabled" );
        break;
    
    case 500:	/* SHOW CLOCK */
    
           //  Show the clock state
        
        printf( "   CPU CLOCK STATES:\n" );
        printf( "      HARDWARE CLOCK=%simplemented",
               HAS_HARDWARE_CLOCK ? "" : "not " );
        if( HAS_HARDWARE_CLOCK )
            printf( ", clock is currently %srunning",
                             vax.clock_running ? "" : "not " );
        printf( "\n" );
        printf( "      ICR  = %08X (%4u)  [Current clock value]\n", vax.clock, vax.clock );
        printf( "      NICR = %08X (%4u)  [Reload  clock value]\n", vax.NICR,  vax.NICR );
        printf( "  QUANTUM\n    INITIAL=%d\n    CURRENT=%d\n",
                            vax.quantum.initial, vax.quantum.current );
        break;

    default:
        rc = VAX_UNKPARM;
        break;
        
    }
    

    return rc;
}




//  Utility routine.  Given a flag and mask bit, format for output.

char * printbit( LONGWORD flag, LONGWORD mask, LONGWORD no_value, char * name, char * desc )
{
    static char * msg = 0L;
    
    char n[ 32 ];
    LONGWORD value;
    
    if( msg == 0L )
        msg = getmem( 256 );
    
    if( msg == 0L )
        return "<insufficient memory to format data>";

    value = flag & mask;
    if( value )
        value = 1;
    
    if( value != no_value )
        strcpy( n, name );
    else {
        strcpy( n, "NO" );
        strcat( n, name );
    }
    
    if( desc != 0L )
        sprintf( msg, "%-20s    %s", n, desc );
    else
        sprintf( msg, "%s", n );
        
    return msg;
}



/*
 *  show_instructions()
 *
 *  This does the SHOW ALL INSTRUCTION command, which dumps each opcode with
 *  lots of formatted information about how the instruction works.
 */

void show_instructions( LONGWORD match )
{

    short i, size, j;
    char * pn;
    char m[ 200 ];
    char b[ 32 ];
    LONGWORD * access, accmode;
    LONGWORD found;
    LONGWORD count;

/*  Loop over the instruction data store */

    found = 0;
    count = 0;
    for( i = 0; i < 500; i++ ) {
    
        /* If the name is empty then we've hit the end of the list */
        
        pn = instruction[ i ].name;
        if( *pn == 0 )
            break;
    
        /* If it's not implemented, skip it */
        
        if( instruction[ i ].routine == 0L )
            continue;
        
        /* If we want a particular one, test for it */
        
        if( match >= 0 && instruction[ i ].opcode != match )
            continue;
        else
            found = 1;
            
        
        /*  Start the output buffer with the usage count data */
        /*  if we are asking for profile data (match = -2)    */
        /*  If we show profile data and the count is zero     */
        /*  then don't display it.                            */

        if( match == -2 ) {
            if( instruction[ i ].use_count )
                sprintf( m, "(%5d) ", instruction[ i ].use_count );
            else
                continue;
        }
        else
            m[ 0 ] = 0;


        /*  Format the opcode values and the name at the start of the buffer */
        
        if( instruction[ i ].extended )
            sprintf( b, "%02X %02X %-8s",   instruction[ i ].extended,
                                            instruction[ i ].opcode,
                                            pn );
        else
            sprintf( b, "   %02X %-8s",     instruction[ i ].opcode,
                                            pn );
        
        strcat( m, b );
        
        /*  Loop over the operand list */
        
        for( j = 0; j < instruction[ i ].operand_count; j++ ) {
        
            access = instruction[ i ].access;
            accmode = GETMODE( access, j );
            
            size = instruction[ i ].scale[ j ];
                
            if( j > 0 )
                strcat( m, ", " );
            
            /* Build the operand mode and type */
            
            switch( accmode ) {
            
            case OP_RD: pn = "src";     break;
            case OP_WR: pn = "dst";     break;
            case OP_MD: pn = "mod";     break;
            case OP_AD: pn = "addr";    break;
            case OP_VA: pn = "var";     break;
            case OP_BR: pn = "br";      break;
            default:    pn = "x";       break;
            }
            strcat( m, pn );
            
            switch( size ) {
            case 1: pn = ".b";  break;
            case 2: pn = ".w";  break;
            case 4: pn = ".l";  break;
            case 8: pn = ".q";  break;
            default:    pn = ".x";  break;
            }
            
            strcat( m, pn );
            
        }
        printf( "    %s\n", m );
        count++;
    }
    
    if( match == -1 && count > 1 )
        printf( "%d instructions\n", count );
        
    if( match >= 0 && !found ) 
        printf( "No implemented instruction for opcode %02X\n", match );

    return;
}




/*
 *  SHOW_REGIONS
 *
 *  This implements the code that dumps out the VM table data.  It's called from VMINIT
 *  to show what it did, and from SHOW VM when virtual memory is enabled.
 */

LONGWORD show_regions(void)
{

    LONGWORD n, base, len, count;

    if( !vax_init )
        return VAX_NOVAX;

//  Dump what we know about physical memory.

    len = vax.memsize / 512;

    printf( "\n    Physical Memory\n        %08x (%d decimal) pages\n        Addresses  00000000 - %08X\n\n",
            len, len, vax.memsize - 1 );
            
//  If the VMINIT command has never been issued, then we don't have console-specific
//  knowledge of the memory layout, so we can't report on it.  Note that VAX software
//  may well have set up VM status, but _we_ don't know about it.

    printf( "    Virtual Memory (currently %s)\n",
        vax.MAPEN ? "ENABLED" : "DISABLED" );

    if( !vax.vm_initialized || !VMVALID ) {
        printf( "        Virtual memory configuration is unknown.\n" );
        
        return VAX_OK;
    }

    count = mapped_pages();
    
    printf( "    There are %d physical pages mapped, %d free.\n\n", count, len - count );
    
//  Loop over each of the regions and tell what we know about it.

    base = 0L;
    len = 0L;

    for( n = 0; n < 3; n++ ) {
    
        if( n == 0 ) {
            base = vax.P0BR;
            len  = vax.P0LR;
        }
        else
        if( n == 1 ) {
            base = vax.P1BR;
            len  = vax.P1LR;
        }
        else
        if( n == 2 ) {
            base = vax.SBR;
            len  = vax.SLR;
        }

        printf( "        %s Region\n            Region size = %08X (%5d decimal) pages\n            PFN database = %d page%s",
            vax.region[ n ].name, 
            vax.region[ n ].size, 
            vax.region[ n ].size, 
            vax.region[ n ].pte_count,
            ( vax.region[ n ].pte_count == 1 ) ? " " : "s" );

        printf( "  %sBR = %08X    %sLR = %08X\n",
            vax.region[ n ].name, base, vax.region[ n ].name, len );
            
        printf( "            Physical addresses = %08X-%08X\n            Virtual addresses  = %08X-%08X\n\n",
            vax.region[ n ].p_start, vax.region[ n ].p_end -1,
            vax.region[ n ].v_start, vax.region[ n ].v_end -1 );
        
    }

    
    return VAX_OK;
}

//
//  Utility routine, formats the stack and shows the call frame(s)
//

LONGWORD show_calls( LONGWORD count )
{

        ULONGWORD fp;
        LONGWORD v;

        union MASKREG {
            LONGWORD longword;
            struct {
                int   spa: 2 ;
                unsigned int calltype: 1;
                int   mbz: 1 ;
                int   mask: 12 ;
                int   psw: 16 ;
            } bits;
        } mask_union;
        ULONGWORD mask, addr, ap, n, saved_ap, rc, argc;


        fp = vax.FP;
        ap = vax.AP;

        if( fp == 0L || ap == 0L ) {
            rc = VAX_NOFRAMES;
            return rc;
        }

        while( count > 0 ) {

            count--;
            printf( "    FRAME: %08X", fp );
            addr = fp;

            /* Get the condition handler address */

            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;

            if( v != 0L )
                printf( ", Handler: %08X", v );

            /* Get the mask longword that defines the frame format */

            addr += 4;
            rc = load_memory( addr, ( void * ) &mask_union.longword, 4 );
            if( rc )
                return rc;

            printf( ", SPA: %1X, %s, MASK: %04X, PSW: %04X\n",
                      mask_union.bits.spa,
                      mask_union.bits.calltype == 0 ? "CALLG" : "CALLS",
                      mask_union.bits.mask,
                      mask_union.bits.psw );


            /* Get the PC, FP, and PC registers */

            addr += 4;
            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;
            printf( "        Saved AP : %08X\n", v );
            saved_ap = v;

            addr += 4;
            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;

            if( v == 0xFFFFDEAF )
                printf( "        Saved FP : <console>\n" );
            else
                printf( "        Saved FP : %08X\n", v );
            fp = v;

            addr += 4;
            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;

            if( v == 0xFFFFDEAF )
                printf( "        Saved PC : <console>\n" );
            else
                printf( "        Saved PC : %08X\n", v );

            /* Get the saved registers, if any */

            addr += 4;
            mask = mask_union.bits.mask;
            for( n = 0; n <= 11; n++ ) {

                if( mask & ( 1 << n )) {
                    rc = load_memory( addr, ( void * ) &v, 4 );
                    if( rc )
                        return rc;
                    addr += 4;
                    printf( "        Saved R%-2u: %08X\n", n, v );
                }
            }

            /* Get the argument count */

            if( ap >= 0x200 && ap < 0x7FFFFFFC ) {
                rc = load_memory( ap, ( void * ) &argc, 4 );
                if( rc )
                    return rc;
            }
            else {
                printf( "        (AP) is not an argument list\n" );
                argc = 0;
            }
            
            if( argc > 255 )
                printf( "        (AP) is not an argument list\n" );
            else {
                if( argc > 0 )
                    printf( "        There %s %u argument%s:\n",
                         argc == 1 ? "is" : "are",
                         argc,
                         argc == 1 ? "" : "s" );

                if( argc > 0 ) {
                    addr = ap;
                    for( n = 1; n <= argc; n++ ) {
                        if( n > 15 ) {
                            printf( "            ...and %u more...\n", argc - n );
                            break;
                        }
                        addr += 4;
                        rc = load_memory( addr, ( void * ) &v, 4 );
                        if( rc )
                            return rc;
                        printf( "            [%2u]: %08X\n", n, v );
                    }
                    if( argc > 64 )
                        printf( "            Note: AP is probably bogus for this call frame\n" );
                } 
            }
            ap = saved_ap;

        /* The value of FP was captured above and will be the new */
        /* frame pointer now.  However, if the FP is zero then we */
        /* are done. Also if the FP is the magic value 'fdef' we  */
        /* are looking at a frame created by the console and are  */
        /* unable to continue.                                    */

            if( fp == 0L || fp == 0xFFFFDEAF )
                break;

            printf( "\n" );

        }

        return VAX_OK;
}

//
//  Utility routine, find the return address of the current call frame
//

LONGWORD get_return( LONGWORD * Addr )
{

        ULONGWORD fp;
        LONGWORD v;
        LONGWORD count = 1;

        union MASKREG {
            LONGWORD longword;
            struct {
                int   spa: 2 ;
                unsigned int calltype: 1;
                int   mbz: 1 ;
                int   mask: 12 ;
                int   psw: 16 ;
            } bits;
        } mask_union;
        ULONGWORD mask, addr, ap, n, saved_ap, rc;

        *Addr = 0L;
        
        fp = vax.FP;
        ap = vax.AP;

        if( fp == 0L || ap == 0L ) {
            rc = VAX_NOFRAMES;
            return rc;
        }

        while( count > 0 ) {

            count--;
            addr = fp;

            /* Get the condition handler address */

            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;


            /* Get the mask longword that defines the frame format */

            addr += 4;
            rc = load_memory( addr, ( void * ) &mask_union.longword, 4 );
            if( rc )
                return rc;


            /* Get the PC, FP, and PC registers */

            addr += 4;
            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;
            saved_ap = v;

            addr += 4;
            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;

            fp = v;

            addr += 4;
            rc = load_memory( addr, ( void * ) &v, 4 );
            if( rc )
                return rc;

            *Addr = v;

            /* Get the saved registers, if any */

            addr += 4;
            mask = mask_union.bits.mask;
            for( n = 0; n <= 11; n++ ) {

                if( mask & ( 1 << n )) {
                    rc = load_memory( addr, ( void * ) &v, 4 );
                    if( rc )
                        return rc;
                    addr += 4;
                }
            }

            ap = saved_ap;

        /* The value of FP was captured above and will be the new */
        /* frame pointer now.  However, if the FP is zero then we */
        /* are done. Also if the FP is the magic value 'fdef' we  */
        /* are looking at a frame created by the console and are  */
        /* unable to continue.                                    */

            if( fp == 0L || fp == 0xFFFFDEAF )
                break;

        }

        return VAX_OK;
}

//
//   Show the SCB entries that are non-zero
//

static LONGWORD showscb( LONGWORD all )
{

    ULONGWORD addr, vector, stack;
    LONGWORD n, rc, count, saved_mapen;

    struct SYMBOL * sp;

    static char * vector_name[] = {
    /* 00  */  "UNUSED",
    /* 04  */  "CHECK",
    /* 08  */  "KSNV",
    /* 0C  */  "POWER",
    /* 10  */  "PRIV",
    /* 14  */  "CUSTOMER",
    /* 18  */  "RESOP",
    /* 1C  */  "RESADDR",
    /* 20  */  "ACCVIO",
    /* 24  */  "TNV",
    /* 28  */  "TP",
    /* 2C  */  "BPT",
    /* 20  */  "COMPAT",
    /* 34  */  "ARITH",
    /* 38  */  "RESERVED38",
    /* 3C  */  "RESERVED3C",
    /* 40  */  "CHMK",
    /* 44  */  "CHME",
    /* 48  */  "CHMS",
    /* 4C  */  "CHMU",
    /* 50  */  "SBI",
    /* 54  */  "CMRD",
    /* 58  */  "SBIALERT",
    /* 5C  */  "SBIFAULT",
    /* 60  */  "MWT",
    /* 64  */  "RESERVED64",
    /* 68  */  "RESERVED68",
    /* 6C  */  "RESERVED6C",
    /* 70  */  "RESERVED70",
    /* 74  */  "RESERVED74",
    /* 78  */  "RESERVED78",
    /* 7C  */  "RESERVED7C",
    /* 80  */  "RESERVED80",
    /* 84  */  "SOFTWARE1",
    /* 88  */  "SOFTWARE2",
    /* 8C  */  "SOFTWARE3",
    /* 90  */  "SOFTWARE4",
    /* 94  */  "SOFTWARE5",
    /* 98  */  "SOFTWARE6",
    /* 9C  */  "SOFTWARE7",
    /* A0  */  "SOFTWARE8",
    /* A4  */  "SOFTWARE9",
    /* A8  */  "SOFTWARE10",
    /* AC  */  "SOFTWARE11",
    /* B0  */  "SOFTWARE12",
    /* B4  */  "SOFTWARE13",
    /* B8  */  "SOFTWARE14",
    /* BC  */  "SOFTWARE15",
    /* C0  */  "RESERVEDC0",
    /* C4  */  "RESERVEDC4",
    /* C8  */  "RESERVEDC8",
    /* CC  */  "RESERVEDCC",
    /* D0  */  "RESERVEDD0",
    /* D4  */  "RESERVEDD4",
    /* D8  */  "RESERVEDD8",
    /* DC  */  "RESERVEDDC",
    /* E0  */  "RESERVEDE0",
    /* E4  */  "RESERVEDE4",
    /* E8  */  "RESERVEDE8",
    /* EC  */  "RESERVEDEC",
    /* F0  */  "RESERVEDF0",
    /* F4  */  "RESERVEDF4",
    /* F8  */  "CONREAD",
    /* FC  */  "CONWRITE" };

    if( !vax_init )
        return VAX_NOVAX;

    addr = vax.SCBB;
    if( addr == 0L ) {
        printf( "No SCB established (SCBB=0)\n" );
        return VAX_OK;
    }

    count = 0;
    printf( "SYSTEM CONTROL BLOCK\n    SCBB register = %08X\n", vax.SCBB );
    saved_mapen = vax.MAPEN;
    vax.MAPEN = 0;

    for( n = 0; n < 64; n ++ ) {

        rc = load_memory( addr + ( n * 4 ), ( void * ) &vector, 4 );
        if( rc ) {
           vax.MAPEN = saved_mapen;
           return rc;
        }

        if( vector || all ) {

            if( MKVALID )
                sp = find_label( vector, SYM_LABEL );
            else
                sp = 0L;
                
            if( count == 0 ) 
                printf( "    Slot  Name            Address  ISP?  Label\n" );
            count++;
            
            if( vector == 0xFFFFFFFF ) 
                printf( "     %02X   EXC$%-10s  <console handler>\n", 
                  n, vector_name[ n ] );
            
            else {
                stack = vector & 0x00000003UL;
                vector = vector & 0xFFFFFFFCUL;
                printf( "     %02X   EXC$%-10s  %08X %s %s\n", 
                  n, vector_name[ n ], vector, stack ? "(ISP)" : "     ",
                      sp == 0L ? "" : sp-> name );
            }
        }
    }

    if( count == 0 )
        printf( "No exception vector entries defined.\n" );

    vax.MAPEN = saved_mapen;

    return VAX_OK;
}



void show_modes( void )
{
    LONGWORD n, count;
    extern LONGWORD mode_profile[ 256 ];

/*
 *  Dump the immediate mode constants.
 */
    count = 0;
    for( n = 0; n < 0x40; n++ ) {
        if( mode_profile[ n ] == 0 )
            continue;
        count++;
        if( count == 1 )
            printf( "Immediate mode constants:\n" );
        printf( "\t %02X                %8d time%s\n", n, mode_profile[ n ],
            mode_profile[ n ] == 1 ? "" : "s" );
    }

/*
 *  PC addressing mode cases.
 */

    printf( "\nPC addressing modes:\n" );
    
    if( mode_profile[ 0x8F ] )
        printf( "\t 8F  I^#n          %8d time%s\n", mode_profile[ 0x8F ], mode_profile[ 0x8F ] == 1 ? "" : "s" );

    if( mode_profile[ 0x9F ] )
        printf( "\t 9F  @#n           %8d time%s\n", mode_profile[ 0x9F ], mode_profile[ 0x9F ] == 1 ? "" : "s" );

    if( mode_profile[ 0xAF ] )
        printf( "\t AF  B^n           %8d time%s\n", mode_profile[ 0xAF ], mode_profile[ 0xAF ] == 1 ? "" : "s" );

    if( mode_profile[ 0xBF ] )
        printf( "\t BF  @B^n          %8d time%s\n", mode_profile[ 0xBF ], mode_profile[ 0xBF ] == 1 ? "" : "s" );

    if( mode_profile[ 0xCF ] )
        printf( "\t CF  W^n           %8d time%s\n", mode_profile[ 0xCF ], mode_profile[ 0xCF ] == 1 ? "" : "s" );

    if( mode_profile[ 0xDF ] )
        printf( "\t DF  @W^n          %8d time%s\n", mode_profile[ 0xDF ], mode_profile[ 0xDF ] == 1 ? "" : "s" );

    if( mode_profile[ 0xEF ] )
        printf( "\t EF  L^n           %8d time%s\n", mode_profile[ 0xEF ], mode_profile[ 0xEF ] == 1 ? "" : "s" );

    if( mode_profile[ 0xFF ] )
        printf( "\t FF  @L^n          %8d time%s\n", mode_profile[ 0xFF ], mode_profile[ 0xFF ] == 1 ? "" : "s" );

/*
 *  Register addressing modes.
 */
 
    printf( "\nRegister addressing modes:\n" );
    
    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0x40 + n ];
    if( count )
        printf( "\t 4n  [Rn]          %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0x50 + n ];
    if( count )
        printf( "\t 5n  Rn            %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0x60 + n ];
    if( count )
        printf( "\t 6n  (Rn)          %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0x70 + n ];
    if( count )
        printf( "\t 7n  -(Rn)         %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0x80 + n ];
    if( count )
        printf( "\t 8n  (Rn)+         %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0x90 + n ];
    if( count )
        printf( "\t 9n  @(Rn)+        %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0xA0 + n ];
    if( count )
        printf( "\t An  B^d(Rn)       %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0xB0 + n ];
    if( count )
        printf( "\t Bn  @B^d(Rn)      %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0xC0 + n ];
    if( count )
        printf( "\t Cn  W^d(Rn)       %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0xD0 + n ];
    if( count )
        printf( "\t Dn  @W^d(Rn)      %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0xE0 + n ];
    if( count )
        printf( "\t En  L^d(Rn)       %8d time%s\n", count, count == 1 ? "" : "s" );

    count = 0;
    for( n = 0; n < 0x0F; n++ )
        count += mode_profile[ 0xF0 + n ];
    if( count )
        printf( "\t Fn  @L^d(Rn)      %8d time%s\n", count, count == 1 ? "" : "s" );
        
    return;
}


