//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_xfc.c
//
//  Purpose:    Emulator handlers for XFC instructions which provide exits to
//              operating system like functions.
//
//  History:    07/28/97    New header format standardization
//
//              08/16/98    Added XFC XFC$VM to test VM translations
//
//              11/01/99    Added HALT and HALT_SILENT for microkernel support.
//
//              11/08/99    Added XFC XFC$P1VECTOR for system services
//
//              05/20/00    Added XFC for DCL callback, and quiting the emulator.
//                          Also fixed bug where bogus XFC was ignored.
//


#include "vax.h"
#include "shim.h"
#include "dclrtl.h"
#include "asmproto.h"

LONGWORD console_dispatch( char * cmd );




EMULATOR_ENTRY( emul_xfc )
{
    char ch;
    short loop;
    short slen;
    LONGWORD saved_mapen;
    LONGWORD code;
    LONGWORD addr, rc;
    char * buff;
    
//  The XFC instruction takes a single implicit immediate instruction argument.  Get it.

    code = opcode-> VAXaddr[ 0 ];

//  Based on the code, do some work.


    switch( code ) {
    
    case    0x01:   /* #XFC$CONSOLE_WRITE       R0 = byte to write */
    
            ch = ( char ) ( vax.R0 & 0x000000FFL );
            putchar( ch );
            break;
    
    case    0x02:   /*  #XFC$CONSOLE_READ       R0 = byte read */
    
            ch = ( char ) getchar();
            vax.R0 &= 0xFFFFFF00L;
            vax.R0 |= ch;
            break;
    
    case    0x03:   /*  #XFC$CONSOLE_CMD        R0 = address of command text */
    
            /* Get addressability to data, and fetch the length value */
            
            addr = vax.R0;
            load_memory( addr, ( void * ) &slen, 2 );
            addr += 2;
            
            /* If the string is empty or badly formed, don't use it. */
            
            if( slen <= 0 )
                break;
            
            /* Allocate a buffer and copy the string to the buffer */
            
            buff = ( char * ) getmem( slen + 2 );
            
            for( loop = 0; loop < slen; loop++ ) {
                load_memory( addr + loop, ( void * ) &ch, 1 );
                buff[ loop ] = ch;
            }
            
            /* Slap a null terminator and execute as a console command */
            
            buff[ loop ] = '\0';
            
            rc = console_dispatch( buff );
            
            /* Store the return code back to the program */
            
            vax.R0 = rc;
            
            
            /* Free the buffer now that we're done with it. */
            
            freemem( buff );

            break;

    case    0x78:   /*  #XFC$QUIT_EMULATION exit from eVAX */
    
            if( vax.pslw.cur_mod > 0 ) {
                return set_fault( EXC_PRIV, 0 );
            }
            
            vax.halted = VAX_USERHALT;
            vax.console.running = 0;
            
            return VAX_OK;
            
    case    0x79:   /*  #XFC$DCL  parse DCL info from console */

            if( !MKVALID )
                return set_fault( EXC_PRIV, 0 );
                
            switch( vax.R0 ) {

            case 1:

                vax.R0 = DCLpresent( vax.R1, vax.R2 );
                break;

            case 2:

                vax.R0 = DCLgetkeyword( vax.R1, vax.R2, vax.R3 );
                break;

            case 3:

                rc = get_symbol_direct( "EXE$DCLSTRING", &addr );
                if( rc ) {
                    printf( "EXE$DCLSTRING buffer not set up in microkernel\n" );
                    break;
                }  

                buff = DCLgetstring( vax.R1, vax.R2 );
                if( buff == 0L ) 
                    vax.R0 = 0L;
                else {
                    store_string( buff, addr, (int) strlen( buff ) + 1 );
                    vax.R0 = addr;
                }

                break;

            case 4:
                vax.R0 = DCLgetinteger( vax.R1, vax.R2 );
                break;

            default:
                printf( "Invalid XFC$DCL parse callback at %08X\n", vax.PC );
                vax.halted = 1;
                return VAX_FAULT;
            }

            return VAX_OK;

    case    0x7A:   /*  #XFC$P1VECTOR invokes a system service */

            return call_service( vax.PC - 4 );
    
    case    0x7B:   /*  #XFC$HALT_SILENT Halts without error _or_ message */
    
            vax.halted = VAX_USERHALT;
            return VAX_OK;
            
    case    0x7C:   /*  #XFC$HALT       Halts emulation without error */
    
            printf( "Emulation halted under program control at %08X\n", vax.PC - 2 );
            
            vax.halted = VAX_USERHALT;
            return VAX_OK;
            
    
    case    0x7D:   /*  #XFC$SHIM       R0 = function dispatch code */
    
            return shim();
            
    case    0x7F:   /*  #XFC$VMR        R0 = address to translate   */
                    /*                  R0 receives updated address */
                    /*                  V  bit set if TNV fault     */
            
    case    0x7E:   /*  #XFC$VMW        R0 = address to translate   */
                    /*                  R0 receives updated address */
                    /*                  V  bit set if TNV fault     */
            
            addr = vax.R0;
            slen = 4;
            
            saved_mapen = vax.MAPEN;
            vax.MAPEN = 1;
            if( vm( ( ULONGWORD * ) &addr, &slen, ( short ) ( code == 0x7E )) 
                == VAX_FAULT )
                         vax.pslw.v = 1;
            else
                vax.R0 = addr;
            vax.MAPEN = saved_mapen;
            
            
            break;

            
    default:
            return set_fault( EXC_RESOP, 0 );
    }

    return VAX_OK;
    
}
