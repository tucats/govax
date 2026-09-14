//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_clear.c
//
//  Purpose:    This module implements the CLEAR commands.  These are used to
//              clear out states in the console, such as the break point list
//              or the symbol table.  Additionally, CLEAR MEMORY is mapped to
//              the ZERO command.
//
//  History:    08/06/97    New header format standardization
//
//              01/20/99    Added CLEAR PROFILE instruction to clear out
//                          instruction profile counter.
//
//              10/15/99    Added CLEAR TEMP SYMBOLS to clear out non-permanent
//                          user symbols from the symbol table.
//
//              11/12/99    Modified to start using the DCL command parser instead
//                          of the brute force mechanisms.
//
//		11/02/01    Added CLEAR BREAK/INSTR family


#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "vaxinstr.h"
#include "dclrtl.h"

extern LONGWORD mode_profile[ 256 ];




LONGWORD console_clear_dcl( long id )
{

    LONGWORD rc;
    char * p;
    LONGWORD count, n;
    struct BREAKSTR * bp, * nextbp, *lastbp;
    struct INTERRUPT * ip, *nextip;
    char opcode[ 32 ];
    
    char * pn;
    char zero = 0;
    extern LONGWORD tb_hit, tb_try, tb_pflush;
    ULONGWORD stringpool_addr, stringpool_len;
    
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }

               
    rc = VAX_OK;
    
    switch( id ) {

    case 553:   /*  CLEAR BREAK/INSTR/ALL */
    
        count = 0;
        for( n = 0; n < 512; n++ ) {
        
            if( instruction[n].name[0] == 0 )
                break;
            if( instruction[ n ].debugdata & OP_DBG_BREAK ) {
                count++;
                instruction[ n ].debugdata &= ~OP_DBG_BREAK;
                printf( "    %02X %s\n", 
                    instruction[ n ].opcode,
                    instruction[ n ].name );
            }
        }
        
        if( count == 0 )
            printf( "No instruction breakpoints were set.\n" );
        else
            printf( "Cleared %d instruction breakpoint%s\n",
                count, count == 1 ? "" : "s" );
                
        break;
    
    case 551:	/*  CLEAR BREAK/INSTR <opcode> */
    
        strcpy( opcode, "OPC$_" ); /* Prefix so it is a known symbolic value */
        
        strcat( opcode, DCLgetstring( DCL_CALLBACK_PARAMETER, 552 ));
        p = opcode;
        rc = asm_hex( &p, ( ULONGWORD * ) &n );
        if( rc )
            break;
        
        for( count = 0; count < 512; count++ ) {
        
            if( instruction[count].name[0] == 0 )
                break;
        
            if( instruction[count].opcode == n ) {
                instruction[n].debugdata &= ~OP_DBG_BREAK;
                printf( "Removed breakpoint on instruction %02X %s\n",
                    instruction[count].opcode,
                    instruction[ count ].name );
                break;
            }
        }
        

    case 105:   /*  CLEAR STRINGS */

        rc = get_symbol_direct( "CONSOLE$STRINGPOOL_BASE", ( LONGWORD * ) &stringpool_addr );
        if( rc )
            return rc;

        rc = get_symbol_direct( "CONSOLE$STRINGPOOL_SIZE", ( LONGWORD * ) &stringpool_len );
        if( rc )
             return rc;

        rc = set_symbol_direct( "CONSOLE$STRINGPOOL", stringpool_addr, 0 );
 
        for( n = 0; ( ULONGWORD ) n < stringpool_len; n++ ) {
            rc = store_memory( stringpool_addr + n, ( void * ) &zero, 1 );
            if( rc )
                return rc;
        }
       break;

    case 107:   /*  CLEAR TB */
        
        invalidate_tb();
        tb_try = tb_hit = tb_pflush = 0L;
        { extern LONGWORD cached_page_hit, cached_page_try;
          cached_page_hit = 0;
          cached_page_try = 0;
        }
        
        break;

    case 113:   /*  CLEAR ERROR */
    
        set_symbol_direct( "$STATUS", 0, SYM_PERMANENT );
        break;
 
    case 104:   /*  CLEAR PROFILES */       

        count = 0;
        for( n = 0; n < 500; n++ ) {
            pn = instruction[ n ].name;
            if( *pn == 0 )
                break;

            if( instruction[ n ].use_count ) {
                count++;
                instruction[ n ].use_count = 0;
            }
        }

        printf( "Reset %d instruction%s\n", count,
                  count == 1 ? "" : "s"  );

        count = 0;
        for( n = 0; n < 256; n++ )
            if( mode_profile[ n ] ) {
                count++;
                mode_profile[ n ] = 0;
            }
        printf( "Reset %d addressing mode%s\n", count,
                  count == 1 ? "" : "s"  );
            

        break;

            
    case 103:   /* CLEAR MEMORY */
    
        rc = console_zero( &p );
        break;

    case 114:   /* CLEAR MEMORY/STATISTICS */

        {
            extern LONGWORD total_allocated;
            extern LONGWORD count_allocated;
            extern LONGWORD total_freed;
            extern LONGWORD count_freed;

            total_allocated = vax.memsize;
            count_allocated = 0L;
            total_freed = count_freed = 0L;
            break;
    }
  
    case 111:   /* CLEAR SYMBOLS/ALL */
    
            rc = clear_all_symbols();
            break;
    
    case 115:   /* CLEAR SYMBOLS/TEMP */
            rc = clear_temp_symbols();
            break;
    
    case 106:   /* CLEAR SYMBOL <name> */
          
        printf( "\tUnable to clear symbol %s\n", DCLgetstring( DCL_CALLBACK_PARAMETER, 1003 ));
        printf( "\tClearing individual symbols is not currently supported.\n" );
        
        break;
    
    case 102:   /* CLEAR INTERRUPT <id> */
    
        p = DCLgetstring( DCL_CALLBACK_QUALIFIER, 1002 );
        rc = asm_hex( &p, ( ULONGWORD * ) &n );
        if( rc )
            break;
        
        nextip = 0L;
        n = 0;
        for( ip = vax.iqueue; ip; ip = nextip ) {
            
            nextip = ip-> next;
            if( ip-> code ==  n ) {
               if( nextip == 0L ) 
                  vax.iqueue = vax.iqueue -> next;
               else
                  nextip-> next = ip-> next;
               n++;    
               freemem( ip );
            }
            
        }

        printf( "\tCleared %d pending interrupt%s\n", n,
                n == 1 ? "" : "s" );       
        break;
 
        
    case 110:   /* CLEAR INTERRUPT/ALL */

            n = 0;
            for( ip = vax.iqueue; ip; ip = nextip ) {
                n++;
                nextip = ip-> next;
                freemem( ip );
            }
            vax.iqueue = 0L;
            vax.interrupt_pending = 0;
            printf( "\tCleared %d pending interrupt%s\n", n,
                n == 1 ? "" : "s" );
            break;
 
           
 
    case 112:   /* CLEAR BREAK/ALL */    
            
            n = 0;
            for( bp = vax.console.breakpoint_list; bp; bp = nextbp ) {
                n++;
                nextbp = bp-> next;
                freemem( bp );
                
            }
            
            printf( "\tCleared %d breakpoint entr%s\n", n,
                n == 1 ? "y" : "ies" );
            
            vax.console.breakpoint_list = 0L;
            break;

    case 108:   /* CLEAR BREAK/FAULT <addr> */

        p = DCLgetstring( DCL_CALLBACK_PARAMETER, 2 );
        rc = asm_hex( &p, ( ULONGWORD * ) &n );
        if( rc )
            break;
        
        nextbp = 0L;
        for( bp = vax.console.breakpoint_list; bp; bp = bp-> next ) {
            if( bp-> kind == BREAK_FAULT && bp-> pc == ( ULONGWORD ) n )
                break;
            nextbp = bp;
            
        }
        
        if( bp == 0L ) {
            printf( "Breakpoint not defined.\n" );
            break;
        }
                    
        if( nextbp == 0L ) 
            vax.console.breakpoint_list = vax.console.breakpoint_list -> next;
        else
            nextbp-> next = bp-> next;
        
        freemem( bp );
        
        break;

    case 109:   /* CLEAR BREAK/FAULT/ALL */

        
        nextbp = 0L;
        lastbp = 0L;
        n = 0;
        
        for( bp = vax.console.breakpoint_list; bp; bp = nextbp ) {
            nextbp = bp-> next;
            if( bp-> kind == BREAK_FAULT ) {
                if( lastbp == 0L )
                    vax.console.breakpoint_list = nextbp;
                else
                    lastbp-> next = nextbp;
                freemem( bp );
                n++;
            }
            else
                 lastbp = bp;
            
        }
        
        printf( "    Cleared %d breakpoint%s\n",
            n, ( n == 1 ? "" : "s" ));
                    
        break;



    case 101:   /* CLEAR BREAK <addr> */
            
        p = DCLgetstring( DCL_CALLBACK_PARAMETER, 1 );
        rc = asm_hex( &p, ( ULONGWORD * ) &n );
        if( rc )
            break;
        

        nextbp = 0L;
        for( bp = vax.console.breakpoint_list; bp; bp = bp-> next ) {
            if( bp-> kind == BREAK_ADDRESS && bp-> pc == ( ULONGWORD ) n )
                break;
            nextbp = bp;
            
        }
        
        if( bp == 0L ) {
            printf( "Breakpoint not defined.\n" );
            break;
        }
                    
        if( nextbp == 0L ) 
            vax.console.breakpoint_list = vax.console.breakpoint_list -> next;
        else
            nextbp-> next = bp-> next;
        
        freemem( bp );
        
        break;
    
    default:
        rc = VAX_UNKPARM;
        break;
    
    }

    return rc;
}
