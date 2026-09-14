
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX"
//
//  Author:     Tom Cole
//
//  Module:     console_set.c
//
//  Purpose:    This module implements the SET commands.  These commands are
//              used to modify the state of the virtual VAX processor or the
//              console processor.
//
//  History:    08/06/97    New header format standardization
//
//              02/01/98    Added SET [NO]ADDRESS command, which sets/clears
//                          the vax.assembler.flags[ASM_ADDRPROMPT] bit
//                          flags.
//
//              01/05/99    Fix annoying problem with SET PC=value giving
//                          bogus error about text after command.
//
//              05/03/99    Make SET MODE and SET PSL MODE statements
//                          switch the stacks as well as the mode bits.
//                          Support the (scary) SET MODE INTERRUPT command.
//
//              05/07/99    Allow SET PTE to have a range of addresses using
//                          syntax like SET PTE 0200 TO 0400 ...
//
//              05/25/99    Allow SET to accept qualifiers for symbols, such
//                          as SET/ENTRY or SET/LABEL or SET/PERM.
//
//              06/03/99    Added SET DEBUG REGISTERS
//
//              06/10/99    Make sure SET commands that change IPL or mode also
//                          check to see if pending AST delivery is needed.
//
//              11/08/99    Added SET BREAK/TEMP to allow creating one-shot
//                          breakpoints.  Moved breakpoint creation to a
//                          shared routine set_break().
//
//              01/10/00    SET NOSHARE explicitly disables sharable image loads
//
//		06/29/00    Added SET DEBUG [NO]COMMAND_EXPANSION, SET [NO]EXPAND,
//                          SET [NO]VERBOSE
//
//		02/20/02    Added SET FAULT n to set size of the FAULT history
//			    buffer.
//

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "pte.h"
#include "vaxinstr.h"

#include <ctype.h>


/*
 *  Define some useful macros to make the code more readable (I hope).
 */

//  Given a verb and two switch settings, define a case block that sets or
//  clears the flag.  Use for SET DEBUG <flag>
//

#define SETDBG( verb, switch_on, switch_off, switch_flag ) \
        case switch_on :                             \
        case switch_off:                             \
            if( verb == switch_on )                  \
                vax.debug |= switch_flag;          \
            else                                     \
                vax.debug &= ~switch_flag;         \
            break
            
LONGWORD set_fault_history( int fsize );
LONGWORD parse_pte_changes( char ** P, union PTE * pte );
LONGWORD setpte( char ** P);
LONGWORD setpte_multiple( char ** P );

extern char * rom;  /* ROM memory */

/*
 *       The SET command
 */

LONGWORD console_set( char ** P )
{
    LONGWORD rc, n, parsing;
    char * p, * saved_p;
    LONGWORD verb, verb2, perm;
    char b[ 32 ], * bp;
    ULONGWORD value;
    struct BREAKSTR * bptr;
    LONGWORD after;
    extern char * pr_names[];
    struct SYMBOL * sp;
    LONGWORD size;
    char * fn;
    
/*
 *  Get local addressability to virtual VAX and command buffer
 */
 
    /* vax = *Vax;  -- now use global vax */
    p = *P;

    
/*
 *  If there is no active virtual we are done!
 */
 
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }
    
/*
 *  See if it's a SET <register>=<value
 */

    rc = set_reg( &p );
    if( rc == VAX_OK ) {
        *P = p;
        return rc;
    }
    rc = VAX_OK;

    perm = 0;

    //  Look for qualifiers that affect the symbol.

    /*  /PERMANENT      the symbol cannot be deleted    */

    bp = p;
    rc = read_verb( &p, &verb2 );
    if( rc )
        return rc;
    if( verb2 == CHAR4( '/','P','E','R' ) ||
        verb2 == CHAR4( '/','P','R','M' )) {
        perm = perm | SYM_PERMANENT;
    }
    else {
        p = bp;
    }
    flush_blanks( &p );


    /*  /ENTRY          the symbol is an entry          */

    bp = p;
    rc = read_verb( &p, &verb2 );
    if( rc )
        return rc;
    if( verb2 == CHAR4( '/','E','N','T' )) {
        perm = perm | SYM_ENTRY;
    }
    else {
        p = bp;
    }
    flush_blanks( &p );

    /*  /LABEL          the symbol is a label           */

    bp = p;
    rc = read_verb( &p, &verb2 );
    if( rc )
        return rc;
    if( verb2 == CHAR4( '/','L','A','B' ) ||
        verb2 == CHAR4( '/','L','B','L' )) {
        perm = perm | SYM_LABEL;
    }
    else {
        p = bp;
    }
    flush_blanks( &p );


/*
 *  See if it's a symbol defintion NAME=value
 */
    
    flush_blanks( &p );
    if( isend( *p ))
        return VAX_INCOMPLETE;
        
    saved_p = p;
    
    for( n = 0; n < 31; n++ ) {
        b[ n ]  = *saved_p;
        
        if( b[ n ] != '_' && b[ n ] != '$' && !isalnum( b[ n ]))
            break;
            
        if( isend( b[ n ]) || is_blank( b[ n ]))
            break;
        
        saved_p++;
    }
    
    b[ n ] = 0;
    
    flush_blanks( &saved_p );
    if( *saved_p == '=' ) {
        saved_p++;
        rc = asm_expr( &saved_p, ( LONGWORD * ) &value );
        if( rc )
            goto exit;
        
        bp = b;
        
        /* At this point, it could be a symbol or a privileged register name.  Let's see */
        /* if it's in the privileged name list                                          */
        
        for( n = 0; n <= MAXPRIVREG; n++ ) {
            if( strcmp( pr_names[ n ], b ) == 0 ) {

                rc =  set_priv_reg( ( short ) n, value );

                p = saved_p;
                goto exit;
            }
        }
        
        /* Could be PSL which is a special case name */
        
        if( strcmp( b, "PSL" ) == 0 ) {
            vax.psl.reg = value;
            read_psl_bits();   /* Update cache from new register value */
            p = saved_p;
            goto exit;
        }
        
        /* Not a privileged register name, so assume it's a symbol set. */
        
        rc = set_symbol( &bp, value, (int) perm );
        p = saved_p;
        goto exit;
    }
    

/*
 *  Not a symbol set, so process regular command syntax
 */
 
    
    read_verb( &p, &verb );
    parsing = 1;

    switch( verb ) {

    case CHAR4('P','S','L',' '):

        while( parsing ) {

            flush_blanks( &p );
            if( isend( *p )) {
                parsing = 0;
                break;
            }

            if( *p == ',' ) {
                p++;
                flush_blanks( &p );
            }

            rc = read_verb( &p, &verb );
            if( rc )
                goto exit;

            flush_blanks( &p );
            if( *p == '=' ) {
                p++;
                flush_blanks( &p );
            }

            rc = asm_expr( &p, ( LONGWORD * ) &value );
            if( rc )
                goto exit;

            switch( verb ) {

#define SETPSL( field, v, min, max ) \
    if( v < min || v > max ) {       \
        rc = VAX_INVSETPSL;          \
        goto exit;                   \
    }                                \
    vax.pslw.field = v


            case CHAR4('C','M',' ',' '):  SETPSL( cm, value, 0, 1 );      break;

            case CHAR4('T','P',' ',' '):  SETPSL( tp, value, 0, 1 );      break;

            case CHAR4('F','P','D',' '):  SETPSL( fpd, value, 0, 1 );     break;

            case CHAR4('I','S',' ',' '):  SETPSL( is, value, 0, 1 );      break;

            case CHAR4('C','U','R','_'):
            case CHAR4('C','U','R',' '):
            case CHAR4('M','O','D',' '):
            case CHAR4('C','U','R','M'):  set_mode_stack( value );
                                          SETPSL( cur_mod, value, 0, 3 ); 
                                          invalidate_tb_prot();

                                          /* See if new mode means AST */
                                          /* interrupt is called for.  */

                                            if( ! vax.pslw.is ) {
                                                if( vax.pslw.cur_mod >= vax.ASTLVL ) {
                                                    interrupt( 0x00000088, 02, 0 );
                                                }
                                            }

break;

            case CHAR4('P','R','V','_'):
            case CHAR4('P','R','V',' '):
            case CHAR4('P','R','V','M'):  SETPSL( prv_mod, value, 0, 3 ); break;

            case CHAR4('I','P','L',' '):  SETPSL( ipl, value, 0, 31 );    
                                          vax.IPL = value;
                                          for( n = 15; ( ULONGWORD ) n > value; n-- ) {
                                            if( vax.SISR & ( 1 << n )) {
                                                vax.SISR &= ~( 1 << n );
                                                interrupt( ( 0x000000080 + ( n << 2 )), n, 0 );
                                            }
                                          }
                                          break;


            case CHAR4('D','V',' ',' '):  SETPSL( dv, value, 0, 1 );      break;

            case CHAR4('F','U',' ',' '):  SETPSL( fu, value, 0, 1 );      break;

            case CHAR4('I','V',' ',' '):  SETPSL( iv, value, 0, 1 );      break;

            case CHAR4('T',' ',' ',' '):  SETPSL( t, value, 0, 1 );       break;

            case CHAR4('N',' ',' ',' '):  SETPSL( n, value, 0, 1 );       break;

            case CHAR4('Z',' ',' ',' '):  SETPSL( z, value, 0, 1 );       break;

            case CHAR4('V',' ',' ',' '):  SETPSL( v, value, 0, 1 );       break;

            case CHAR4('C',' ',' ',' '):  SETPSL( c, value, 0, 1 );       break;

            default:
                rc = VAX_INVSETPSL;
                goto exit;
            }

        }

        rc = VAX_OK;
        break;

    case CHAR4('E','X','P','A'):
        vax.console.flags |= CONSOLE_EXPAND;
        break;
    
    case CHAR4('N','O','E','X'):
        vax.console.flags &= ~CONSOLE_EXPAND;
        break;
        
    case CHAR4('N','O','S','H'):
    
        vax.console.share_prefix[ 0 ] = 0x1B;
        break;
        
    case CHAR4('S','H','A','R'):
    case CHAR4('S','H','R',' '):
        flush_blanks( &p );
        
        if( *p == '"' )
            p++;
        
        n = strlen( p );
        if( n > 79 )
            n = 79;
            
        strncpy( vax.console.share_prefix, p, 79 );
        if( vax.console.share_prefix[ n-1 ] == '"' )
            vax.console.share_prefix[ n-1 ] = 0;
        vax.console.share_prefix[ 79 ] = 0;
        
        while( !isend( *p ))
            p++;
        break;
        
    case CHAR4('M','O','D','E'):
        rc = read_verb( &p, &verb );
        if( rc )
            goto exit;

        switch( verb ) {

        case CHAR4('I',' ',' ',' '):
        case CHAR4('I','N','T','E'):
            set_mode_stack( 4 );
            break;

        case CHAR4('U',' ',' ',' '):
        case CHAR4('U','S','E','R'):
            set_mode_stack( 3 );
            break;

        case CHAR4('S',' ',' ',' '):
        case CHAR4('S','U','P','E'):
            set_mode_stack( 2 );
            break;

        case CHAR4('E',' ',' ',' '):
        case CHAR4('E','X','E','C'):
            set_mode_stack( 1 );
            break;

        case CHAR4('K',' ',' ',' '):
        case CHAR4('K','E','R','N'):
            set_mode_stack( 0 );
            break;

        default:
            rc = VAX_UNKPARM;

        }

       /* Since we may have just changed mode, flush the cached */
       /* translation buffer page protection status.            */
       
       invalidate_tb_prot();

                                          
//  At this point we should check to see if ASTLVL tells us there are 
//  pending AST's.  We don't do this if on the interrupt stack, though
//  since AST's are handled in process space typically.  ASTLVL is used
//  to prevent AST delivery to a process not in a suitable mode; i.e.
//  don't deliver USER mode AST's when we are in KERNEL mode, etc.

        if( ! vax.pslw.is ) {
            if( vax.pslw.cur_mod >= vax.ASTLVL ) {
                interrupt( 0x00000088, 02, 0 );
            }
        }
       
        break;

    case CHAR4('S','T','E','P'):
        rc = read_verb( &p, &verb );
        if( rc )
            goto exit;

        switch( verb ) {
        
        case CHAR4('/','O','V','E' ):
        case CHAR4('O','V','E','R' ):
        
            vax.console.stepmode = STEP_OVER;
            break;
        
        case CHAR4('/','I','N','T' ):
        case CHAR4('/','I','N',' ' ):
        case CHAR4('/','I','N','S' ):
        case CHAR4('I','N','T','O' ):
        case CHAR4('I','N',' ',' ' ):
        case CHAR4('I','N','S','T' ):
        
            vax.console.stepmode = STEP_INSTRUCTION;
            break;
        
        case CHAR4('/','R','E','T' ):
        case CHAR4('R','E','T',' ' ):
        case CHAR4('R','E','T','U' ):
        
            vax.console.stepmode = STEP_RETURN;
            break;

       default:
            rc = VAX_UNKPARM;
            
        }
        break;  
    
    
    case CHAR4('M','K','V','A'):
        vax.console.microkernel_valid = 1;
        break;
    
    case CHAR4('N','O','M','K'):
        if( MKVALID )
            printf( "%s\n", vaxmsg( VAX_DELMK ));
        vax.console.microkernel_valid = 0;
        break;
    
    
    case CHAR4('P','T','E',' '):
    case CHAR4('P','A','G','E'):

        rc = setpte_multiple( &p );
        break;
            
    case CHAR4('A','S','M',' '):
    case CHAR4('A','S','S','E'):
    
        flush_blanks( &p );
        rc = read_verb( &p, &verb );
        if( rc != VAX_OK )
            goto exit;
            
        switch( verb ) {
        
        case CHAR4('F','O','R','W'):
            vax.assembler.flags |= ASM_WARNFORWARD;
            break;
        
        case CHAR4('N','O','F','O'):
            vax.assembler.flags &= ~ASM_WARNFORWARD;
            break;
            
        case CHAR4('A','D','D','R'):
            vax.assembler.flags |= ASM_ADDRPROMPT;
            break;
        
        case CHAR4('N','O','A','D'):
            vax.assembler.flags &= ~ASM_ADDRPROMPT;
            break;
        
        case CHAR4('B','R','A','N'):
            vax.assembler.flags |= ASM_BRANCHDEST;
            break;

        case CHAR4('N','O','B','R'):
            vax.assembler.flags &= ~ASM_BRANCHDEST;
            break;
        
        case CHAR4('S','Y','M','B'):
        case CHAR4('S','Y','M',' '):
            vax.assembler.flags |= ASM_SYMBOLS;
            break;

        case CHAR4('N','O','S','Y'):
            vax.assembler.flags &= ~ASM_SYMBOLS;
            break;

        case CHAR4('R','E','S','O'):
        case CHAR4('R','E','S','T'):
            vax.assembler.flags |= ASM_RESTMP;
            break;

        case CHAR4('N','O','R','E'):
            vax.assembler.flags &= ~ASM_RESTMP;
            break;

        case CHAR4('D','E','B','U'):
            vax.assembler.flags |= ASM_SYMDEBUG;
            break;

        case CHAR4('N','O','D','E'):
            vax.assembler.flags &= ~ASM_SYMDEBUG;
            break;

        case CHAR4('S','C','O','P'):
            vax.assembler.flags |= ASM_SCOPENAMES;
            break;

        case CHAR4('N','O','S','C'):
            vax.assembler.flags &= ~ASM_SCOPENAMES;
            break;

        default:    
            rc = VAX_INVSETASM;
            break;
        }
        
        break;



    case CHAR4('D','E','B','U'):
    case CHAR4('D','B','G',' '):
    
        flush_blanks( &p );
        if( isend( *p ))
            verb = CHAR4('D','E','B','U');
        else {
            rc = read_verb( &p, &verb );
            if( rc != VAX_OK )
                goto exit;
        }

        while( 1 ) {
        
            rc = VAX_OK;
            switch( verb ) {
            
            SETDBG( verb, CHAR4('R','M','S',' '), CHAR4('N','O','R','M'), DBG_RMS );
            SETDBG( verb, CHAR4('F','U','L','L'), CHAR4('N','O','F','U'), DBG_FULLDISASM );
            SETDBG( verb, CHAR4('D','E','B','U'), CHAR4('N','D','B','G'), DBG_DEBUG )     ;
            SETDBG( verb, CHAR4('K','E','Y','B'), CHAR4('N','O','K','E'), DBG_KBD )       ;
            SETDBG( verb, CHAR4('K','B','D',' '), CHAR4('N','O','K','B'), DBG_KBD )       ;
            SETDBG( verb, CHAR4('D','E','V','I'), CHAR4('N','O','D','E'), DBG_DEVICES )   ;
            SETDBG( verb, CHAR4('V','M',' ',' '), CHAR4('N','O','V','M'), DBG_VM    )     ;
            SETDBG( verb, CHAR4('T','B',' ',' '), CHAR4('N','O','T','B'), DBG_TB    )     ;
            SETDBG( verb, CHAR4('M','E','M','O'), CHAR4('N','O','M','E'), DBG_MEMORY )    ;
            SETDBG( verb, CHAR4('S','Y','M','B'), CHAR4('N','O','S','Y'), DBG_SYMBOLS )   ;
            SETDBG( verb, CHAR4('I','N','T','E'), CHAR4('N','O','I','N'), DBG_INTERRUPTS );
            SETDBG( verb, CHAR4('E','X','C','E'), CHAR4('N','O','E','X'), DBG_EXCEPTIONS );
            SETDBG( verb, CHAR4('P','1',' ',' '), CHAR4('N','O','P','1'), DBG_P1         );
            SETDBG( verb, CHAR4('P','2',' ',' '), CHAR4('N','O','P','2'), DBG_P2         );
            SETDBG( verb, CHAR4('P','3',' ',' '), CHAR4('N','O','P','3'), DBG_P3         );
            SETDBG( verb, CHAR4('P','4',' ',' '), CHAR4('N','O','P','4'), DBG_P4         );
            SETDBG( verb, CHAR4('R','E','G','I'), CHAR4('N','O','R','E'), DBG_REGISTERS  );
            SETDBG( verb, CHAR4('I','M','A','G'), CHAR4('N','O','I','M'), DBG_IMAGES     );
            SETDBG( verb, CHAR4('U','S','E','R'), CHAR4('N','O','U','S'), DBG_USERHALT   );
            SETDBG( verb, CHAR4('S','E','R','V'), CHAR4('N','O','S','E'), DBG_SERVICES   );
            SETDBG( verb, CHAR4('D','C','L',' '), CHAR4('N','O','D','C'), DBG_DCL        );
            SETDBG( verb, CHAR4('C','H','M',' '), CHAR4('N','O','C','H'), DBG_CHM        );
            SETDBG( verb, CHAR4('C','O','M','M'), CHAR4('N','O','C','O'), DBG_EXPAND     );
            SETDBG( verb, CHAR4('L','O','G','I'), CHAR4('N','O','L','O'), DBG_LOGICALS   );
            SETDBG( verb, CHAR4('L','I','B','I'), CHAR4('N','O','L','I'), DBG_LIBINIT    );
            SETDBG( verb, CHAR4('P','R','O','C'), CHAR4('N','O','P','R'), DBG_PROCESS    );
            default:    
                rc = VAX_INVSETDBG;
                break;
            }
            flush_blanks( &p );
            if( isend(*p))
                break;
            if( *p == ',' ) {
                p++;
                flush_blanks( &p );
            }
            read_verb( &p, &verb );
        }
        
        break;

        
    case CHAR4('V','M',' ',' '):
    case CHAR4('M','A','P','E'):

        if( vax.pslw.cur_mod ) {
            set_fault( EXC_PRIV, 0 );
            return handle_fault();
        }
    
        vax.MAPEN = 1;
        break;
    
    case CHAR4('N','O','V','M'):
    case CHAR4('N','O','M','A'):
        if( vax.pslw.cur_mod ) {
            set_fault( EXC_PRIV, 0 );
            return handle_fault();
        }
    
        vax.MAPEN = 0;
        break;
       
    case CHAR4('W','A','T','C'):

        size = 4;
        /* See if there is a size qualifier */

        flush_blanks(&p);
        saved_p = p;
        rc = read_verb( &p, &verb );
        if( rc )
            return rc;

        if( verb == CHAR4('/','B','Y','T') ||
            verb == CHAR4('/','B',' ',' '))
            size = 1;
        else
        if( verb == CHAR4('/','W','O','R') ||
            verb == CHAR4('/','W',' ',' '))
            size = 2;
        else
        if( verb == CHAR4('/','L','O','N') ||
            verb == CHAR4('/','L',' ',' '))
            size = 4;
        else 
            p = saved_p;

        /* Get the address */

        rc = asm_value( &p, (ULONGWORD*) &n, K_NOFORWARD );
        if( rc )
            break;
            
        flush_blanks( &p );
        if( isend( *p ))
            fn = 0L;
        else {
            fn = p;
            for( fn = p; *p; p++ )
                if( *p == '\n' )
                    *p = 0;
        }

        add_watchpoint( n, size, fn );
        break;
 
    case CHAR4('F','A','U','L'):
    case CHAR4('H','I','S','T'):
    
        rc = asm_value( &p, ( ULONGWORD * ) &n, K_NOFORWARD );
        if( rc )
            return rc;
        
        set_fault_history( (int) n );
        break;

    case CHAR4('B','R','E','A'):
    case CHAR4('B','R',' ',' '):

        /* See if it's a /FAULT qualified instruction */
        
        flush_blanks( &p );
        saved_p = p;
        rc = read_verb( &p, &verb );
        if( rc )
            return rc;
        
        if( verb == CHAR4('/','F','A','U' ) ||
            verb == CHAR4('/','F',' ',' ' )) {
            verb = BREAK_FAULT;
        }
        else
        if( verb == CHAR4('/','T','E','M') ||
            verb == CHAR4('/','T','M','P')) {
            verb = BREAK_ADDRESS | BREAK_TEMPORARY;
        }
        else
        if( verb == CHAR4('/','I','N','S')) {
            verb = BREAK_INSTRUCTION;
        }
        else {
            p = saved_p;
            verb = BREAK_ADDRESS;
        }
        
        /* Get the PC address or the OPCODE to break on */

        if( verb == BREAK_INSTRUCTION ) {
            char opname[ 32 ];
            flush_blanks(  &p );
            strcpy( opname, "OPC$_" );
            strcat( opname, p );
            saved_p = p;
            p = opname;
            rc = asm_value( &p, ( ULONGWORD * ) &n, K_NOFORWARD );
            p = saved_p;
            while( !isend(*p))
                p++;
        }
        else
            rc = asm_value( &p, ( ULONGWORD * ) &n, K_NOFORWARD );
        if( rc )
            break;

        /* If it's here, get the optional count of times to skip breaking */

        flush_blanks( &p );
        if( !isend( *p )) {
           rc = asm_dec( &p, &after );
            if( rc )
                break;
        }
        else
            after = 0;

        if( verb == BREAK_FAULT && after ) {
            printf( "SET BREAK/FAULT cannot include a count; zero assumed.\n" );
            after = 0;
        }

        /* If the breakpoint is for an instruction, find it. */
        
        if( verb == BREAK_INSTRUCTION ) {
             int opc;
             opc = (int) n; 
             for( n = 0; n < 512; n++ ) {
                if( instruction[ n ].name[ 0 ] == 0 )
                    break;
                if( instruction[ n ].opcode == opc ) {
                    instruction[ n ].debugdata |= OP_DBG_BREAK;
                    printf( "Breakpoint set on instruction %02X %s\n", 
                        instruction[ n ].opcode,
                        instruction[ n ].name );
                    goto exit;
                }
            }
            printf( "Nonexistent instruction opcode %02X\n", n );
            goto exit;
        }
        
        /* If the value given is an entry point, we must advance it past the */
        /* mask word to a "real" instruction.  Check the value in both the   */
        /* user and system symbol tables.                                    */

        for( sp = vax.console.symbols; sp; sp = sp-> next ) {
            if(( sp-> flags & SYM_ENTRY ) &&
               ( sp-> value == n ))
                   break; 
        }

        if( !sp )  /* Not in user table, try system table */
        for( sp = system_symbols; sp; sp = sp-> next ) {
            if(( sp-> flags & SYM_ENTRY ) &&
               ( sp-> value == n ))
                   break; 
        }
        if( sp ) {
            n = n + 2;
            printf("%%VAX-I-BRKENTRY, breakpoint is an entry mask,\n" );
            printf("\tchanged to %08X\n", n );
        }

        /* See if it already exists.  If so, just update the after value */

        rc = 0;
        for( bptr = vax.console.breakpoint_list; bptr; bptr = bptr-> next ) {

            if(( bptr-> kind == verb ) && 
               (  bptr-> pc == ( ULONGWORD ) n )) {
                bptr-> after = after;
                rc = 1;
                break;
            }
        }

        if( rc ) {
            rc = VAX_OK;
            break;
        }

        /* Didn't exist, so make a new one */

        set_break( n, (int) after, (int) verb );
        
        break;
    
    case CHAR4('Q','U','A','N'):
    
        rc = asm_dec( &p, &n );
        if( rc )
            break;

        if( vax.quantum.initial == 0 && ( n > 0 ))
            printf( "%%VAX-I-INTERRUPTS, interrupt delivery resumed (quantum>0)\n" );
                    
        if( vax.quantum.initial > 0 && ( n == 0 ))
            printf( "%%VAX-I-NOINTERRUPTS, interrupt delivery suspended (quantum=0)\n");
            
        vax.quantum.initial = n;
        vax.quantum.current = n;
        
        break;
            
    case CHAR4('U','I','Q','U'):
    
        rc = asm_dec( &p, &n );
        if( rc )
            break;

        if( vax.uiquantum.initial == 0 && ( n > 0 ))
            printf( "%%VAX-I-UISLICE, UI time slicing resumed (uiquantum>0)\n" );
                    
        if( vax.uiquantum.initial > 0 && ( n == 0 ))
            printf( "%%VAX-I-NOUISLICE, UI time slicing suspended (uiquantum=0)\n");
            
        vax.uiquantum.initial = n;
        vax.uiquantum.current = n;
        
        break;
            
    case CHAR4('B','A','S','E'):
        rc = asm_expr( &p, ( LONGWORD * ) &n );
        if( rc )
            break;
        
        vax.console.deposit = n;
        break;
    
    case CHAR4('V','E','R','B'):
        vax.console.flags |= CONSOLE_VERBOSE;
        break;
        
    case CHAR4('V','E','R','I'):
        vax.console.verify = 1;
        break;
    
    case CHAR4('N','O','V','E'):
        vax.console.verify = 0;
        vax.console.flags &= ~CONSOLE_VERBOSE;
        break;
        
    case CHAR4('R','A','D',' '):
    case CHAR4('R','A','D','I'):

        rc = read_verb( &p, &verb );
        if( rc != VAX_OK )
            return rc;
            
        switch( verb ) {
        
        case CHAR4('H','E','X',' '):
        case CHAR4('H','E','X','A'):
        case CHAR4('1','6',' ',' '):
            vax.console.radix = 16;
            break;
        
        case CHAR4('D','E','C',' '):
        case CHAR4('D','E','C','I'):
        case CHAR4('1','0',' ',' '):
            vax.console.radix = 10;
            break;
            
        default:
            rc = VAX_UNKPARM;
        }
        
        break;
        
    case CHAR4('T','R','A','C'):
    case CHAR4('D','I','S','A'):

        
        vax.console.disasm = 1;
        break;
    
    case CHAR4('N','O','T','R'):
    case CHAR4('N','O','D','I'):
        vax.console.disasm = 0;
        break;
        
                
    default:

        rc = VAX_UNKPARM;
        
    }
    
    
    
/*
 *  Store parameters back to caller.
 */

exit:

    /* *Vax = vax; -- use global vax instead -- */
    *P = p;
    return rc;
}


LONGWORD setpte_multiple( char ** P )
{
    LONGWORD rc;
    LONGWORD paddr;
    ULONGWORD addr1, addr2, n;
    char * pt, * cmdp, * bp;
    char cmd[ 256 ];
    LONGWORD saved_radix;

    pt = *P;
    rc = asm_expr( &pt, &paddr );
    if( rc )
        return rc;

    addr1 = ( ULONGWORD ) paddr;

    flush_blanks( &pt );
    if( strncmp( pt, "TO ", 3 ) == 0 ) {

        pt += 3;
        rc = asm_expr( &pt, &paddr );
        if( rc )
            return rc;
        addr2 = ( ULONGWORD ) paddr;
                
        cmdp = pt;

        /* Loop over each page in the range */

        addr1 = addr1 & 0xFFFFFE00L;
        addr2 = addr2 & 0xFFFFFE00L;

        for( n = addr1; n <= addr2; n += 512 ) {
            sprintf( cmd, "%08X %s", n, cmdp );
            bp = cmd;
            /* printf( "SET PTE %s\n", bp ); */
            saved_radix = vax.console.radix;
            vax.console.radix = 16;
            rc = setpte( &bp );
            vax.console.radix = saved_radix;
            if( rc )
                return rc;
        }

        /* Eat the rest of this command */
        pt = *P;
        while( !isend( *pt ))
            pt++;

        /* Call it quits */
        *P = pt;
        return VAX_OK;
     }

     /* Not a multipage thing, so do the regular operation */

     return setpte( P );
}



//  This handles the SET PAGE command which for a given address will set up
//  characteristics of the PTE entry.

LONGWORD setpte( char ** P)
{

    LONGWORD region, page, byte;
    ULONGWORD addr, va, pteaddr;
    LONGWORD the_lr, the_br, rc;
    short   n, fault;
    unsigned char * r;
    unsigned char * p;
    short tb_idx;
    char * pt;

    LONGWORD paddr;
    
    union PTE pte;

/*
 *  Parse the address we are changing.
 */

    pt = *P;
    rc = asm_expr( &pt, &paddr );
    if( rc ) {
        *P = pt;
        return rc;
    }
    addr = paddr;

/*
 *  First, if virtual memory isn't on then we have no work to do.
 */
 
    if( vax.MAPEN == 0L ) {
        printf( "Cannot SET PAGE when virtual memory is disabled.\n" );
        return VAX_OK;
    }
 
/*
 *  Okay, work to do.  First, let's break apart the address.
 */

    va = addr;
    
    region = ( addr >> 30 ) & 0x00000003L;
    page   = ( addr & 0x3FFFFFFFL ) >> 9;
    byte   = ( addr & 0x1FF );

/*
 *  Let's do a little translation buffer caching.  Since the work of
 *  doing a translation is VERY costly, let's see if we already have
 *  done this particular translation.  If we ever get a translation
 *  fault then we flush the buffer and do the next translations the
 *  hard way.
 */

    tb_idx =  ( short ) (( region << 4 ) + ( page & 0x0000FL ));    /* Index into TB cache */
    
/*
 *  Depending on which region it's in, do the work.
 */

    switch( region ) {
    
    case 0: /* P0 */
    case 1: /* P1 */

            /* Depending on which region, get the LR and BR values */
            
            if( region == 0 ) {
                the_br = vax.P0BR;
                the_lr = vax.P0LR;
            }
            else {
                the_br = vax.P1BR;
                the_lr = vax.P1LR;
            }

    /*  See if the page number violates the base register length.  Note     */
    /*  that for P1 space, the base register is the list of INVALID page    */
    /*  entries, so the sense of the test must be inverted.                 */
    
            fault = 0;
            if( region == 0 && page > the_lr ) {
                fault = 1;
            }
            else
            if( region == 1 && page <= the_lr )
                fault = 1;
                
            if( fault ) {
                printf( "ACCVIO, page table P%d length violation\n", region );
                return VAX_OK;
            }

    /*  Get the address of the page table entry.  This is actually a */
    /*  system virtual address, so we must recursively translate it, */
    /*  accessing the page in kernel mode.                           */
    
                        
            paddr = pteaddr = the_br + ( page * 4 );

            n = sizeof( pte );
            rc = vm( ( ULONGWORD * ) &paddr, &n, 0 );
            if( rc ) {
                return rc;
            }
            
    /*  Now we have the actual physical address of the page table entry */

            p = PHYADDR( paddr );
            r = ( unsigned char * ) &pte.longword;
            

#if BIGENDIAN
            for( n = 1; n < 5; n++ ) {
                r[ 4 - n ]  = *(p++);
            }
#else
            for( n = 0; n < 4; n++ ) {
                r[ n ]  = *(p++);
            }
#endif

            rc = parse_pte_changes( &pt, &pte );

            if( rc == VAX_OK ) {

                p = PHYADDR( paddr );
                r = ( unsigned char * ) &pte.longword;
            

#if BIGENDIAN
                for( n = 1; n < 5; n++ ) {
                    *(p++) = r[ 4-n ];
                }
#else
                for( n = 0; n < 4; n++ ) {
                    *(p++) = r[ n ];
                }
#endif

                invalidate_page( addr );
            }
            *P = pt;

            break;  
    
    
    
    case 2: /* S0 */

    /*  See if the page number violates the SBR length */
    
            if( ( ULONGWORD ) page > vax.SLR ) {
                printf( "ACCVIO, page table S0 length violation\n" );
                return VAX_FAULT;
            }
    
    /*  Find the VAXaddr that is the base address of the page table */
    /*  Each PTE is four bytes LONGWORD, so scale appropriately         */
    
            paddr = vax.SBR + ( page * 4 );
            p = PHYADDR( paddr );

            r = ( unsigned char * ) &pte.longword;

#if BIGENDIAN
            for( n = 1; n < 5; n++ ) {
                r[ 4 - n ]  = *(p++);
            }
#else
            for( n = 0; n < 4; n++ ) {
                r[ n ]  = *(p++);
            }
#endif

            rc = parse_pte_changes( &pt, &pte );

            if( rc == VAX_OK ) {
                p = PHYADDR( paddr );
                r = ( unsigned char * ) &pte.longword;
            

#if BIGENDIAN
                for( n = 1; n < 5; n++ ) {
                    *(p++) = r[ 4-n ];
                }
#else
                for( n = 0; n < 4; n++ ) {
                    *(p++) = r[ n ];
                }
#endif

            invalidate_page( addr );

            }
            *P = pt;

    
            
            break;

    default:  /* S1 */
    
            /* ERROR */
            printf( "ACCVIO, invalid S1 region\n" );
                
            return VAX_FAULT;
    }
    
    return VAX_OK;
}
    

LONGWORD parse_pte_changes( char ** P, union PTE * pte )
{
    LONGWORD verb, value, rc;
    char * p;
    LONGWORD parsing;

    p = *P;
    parsing = 1;
    flush_blanks( &p );

    while( !isend( *p )) {

        rc = read_verb( &p, &verb );
        if( rc )
            return rc;


        flush_blanks( &p );
        if( *p == '=' ) {
            p++;
            flush_blanks( &p );
        }


        rc = asm_expr( &p, &value );
        if( rc )
            return rc;

        switch( verb ) {

        case CHAR4('V',' ',' ',' '):
        case CHAR4('V','A','L','I'):
            pte-> bit.v = (unsigned int) value;
            break;

        case CHAR4('P','R','O','T'):
            pte-> bit.prot = (unsigned int) value;
            break;

        case CHAR4('M',' ',' ',' '):
        case CHAR4('M','O','D','I'):
            pte-> bit.m = (unsigned int) value;
            break;

        case CHAR4('O','W','N',' '):
        case CHAR4('O','W','N','E'):
            pte-> bit.own = (unsigned int) value;
            break;

        case CHAR4('S',' ',' ',' '):
        case CHAR4('S','O','F','T'):
            pte-> bit.s = (unsigned int) value;
            break;

        case CHAR4('P','F','N',' '):
        case CHAR4('P','A','G','E'):
            pte-> bit.pfn = (unsigned int) value;

        default:
            rc = VAX_FAULT;
        }

    }
    
    *P = p;
    return VAX_OK;
}

