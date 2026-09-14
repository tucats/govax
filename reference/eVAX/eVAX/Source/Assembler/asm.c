//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     asm.c
//
//  Purpose:    Pseudoassembler for the console interface to the VAX.
//
//
//  History:    07/28/97    New header format standardization
//
//              07/24/97    Changed so all memory stores are done via store_memory so the assembler
//                          is compatable with virtual memory when it's available.
//
//              02/01/98    Moved register name parsing, numeric value parsing, opcode
//                          parsing, and label handling into their own source files.
//


#include "vax.h"
#include "vaxinstr.h"

//  Local prototypes

#include "asmproto.h"
#include "console_proto.h"

//  Short hand function for assembling a single statement, doesn't require parse
//  support by advancing a character pointer.

LONGWORD assemble_direct( char* buffer )
{
    char * p;
    char buff[ 256 ];
    strcpy( buff, buffer );
    uppercase( buff );
    p = buff;
    return assemble( &p );
}


//  Assemble a single statement, storing at the current PC value and
//  advancing the PC.


LONGWORD assemble( char ** Buffer )
{
    char * p, *sp;
    LONGWORD rc, count, n, verb;
    char * buffer;
    
    /* vax = *Vax;  -- now use global vax */
    buffer = *Buffer;
    
    if( !vax_init ) {
        return VAX_NOVAX;
    }
    
/*
 *  See if we are turning on ASM mode, done by a blank ASM command.
 */
 
    p = buffer;
    flush_blanks( &p );
    if( isend( *p )) {
        vax.console.assembler_mode = 1;
        *Buffer = p;
        return VAX_OK;
    }

/*
 *  For compatability with GNU assemblers, accept a "#" in the first column as a comment.
 */
 
    if( *p == '#' ) {
    
        while( !isend( *p ))
            p++;
        *Buffer = p;
        return VAX_OK;
    }
    
/*
 *  See if it's the END pseudo command in the assembler, which turns off
 *  the ASM command mode.
 */
    
    read_verb( &p, &verb );
    if( verb == CHAR4('E','N','D',' ') ) {
        vax.console.assembler_mode = 0;
        vax.assembler.case_base = 0L;        
        // See if the optional entry point name is placed here.
        
        flush_blanks( &p );
        if( !isend( *p )) {
        
            rc = asm_hex( &p, ( void * ) &verb );
            if( rc == VAX_OK ) {
            
                rc = set_symbol_direct( "__ENTRY", verb, SYM_NONE );
            }
            
            vax.PC = verb;
            
        
        }
        
        *Buffer = p;
        
        if(( vax.assembler.flags & ASM_WARNFORWARD ) &&
             check_unresolved_symbols( 0 ) != VAX_OK )
                    printf( "%s\n", vaxmsg( VAX_ASMFORWARD ));
            
        return VAX_OK;
    }

/*
 *  No mode changing stuff, so reset to the start of the buffer and do a normal
 *  assembly statement.
 */
 
    p = buffer;
    
    //  See if there's a label.  If so, set it's value and keep trucking
    
    rc = asm_label( &p );
    if( rc ) {
        *Buffer = p;
        return rc;
    }

    //  See if it's a pseudo operator.  If so, we're done.
    
    rc = asm_pseudo( p );
    if( rc != VAX_ASMNOTPSEUDO ) {
        *Buffer = p;
        return rc;
    }
    
    
    //  Otherwise, it must be a real opcode.  We must empty out the
    //  base for .CASE blocks now since we're interposing some other
    //  instructions so it resets to case block.

    vax.assembler.case_base = 0L;        
    rc = asm_opcode( &p, &count );
    if( rc ) {
        *Buffer = p;
        if( rc == VAX_ASMNOOPCODE )
            rc = VAX_OK;        /* We allow just labels on a line */
        return rc;
    }
    
    if( vax.console.deposit < vax.console.min_deposit ) {
        vax.console.min_deposit = vax.console.deposit - 1L;
        sp = "__FIRST";
        rc = set_symbol_direct( "__FIRST", vax.console.min_deposit, 0 );
    }
    
    for( n = 0; n < count; n++ ) {
    
        vax.assembler.opcount = n;
        
        vax.assembler.scale = vax.console.scale[ n ];

        /* Disable use of @register syntax in expressions for operands */
        
        vax.assembler.flags &= ~ASM_REGISTER_EXPR;
        
        /* Assemble a single operand */
        
        rc = asm_operand( &p );
        
        vax.assembler.flags |= ASM_REGISTER_EXPR;
        
        if( rc ) {
            *Buffer = p;
            return rc;
        }
        

        if(( n < (count - 1 )) && isend( *p )) {
            return VAX_ASMINSFOPERANDS;
        }
        if( *p == ',' )
            p++;
    }
    
    if( vax.console.deposit >= vax.console.max_deposit ) {
        vax.console.max_deposit = vax.console.deposit - 1L;
        rc = set_symbol_direct( "__LAST", vax.console.max_deposit, 0 );
    }

    *Buffer = p;
    return VAX_OK;
}


//
//  The . or DO command
//

LONGWORD console_do( char ** P )
{

    char * p;
        LONGWORD value, rc;
    ULONGWORD saved_deposit, saved_PC;
    char sname[ 32 ], *sp;

    /* vax = *Vax;  -- now use global vax */
    if( !vax_init )
        return VAX_NOVAX;

    p = *P;
    strcpy( sname, "CONSOLE$SCRATCH" );
    sp = sname;
    rc = get_symbol( &sp, &value );
    if( rc )
        return rc;

    saved_deposit = vax.console.deposit;
    vax.console.deposit = value;
    rc = assemble( &p );
    vax.console.deposit = saved_deposit;
    if( rc )
        return rc;
    
    *P = p;

    vax.console.singlestep = STEP_INSTRUCTION;
    saved_PC = vax.PC;        
    vax.PC = value;
    rc = execute_vax( ( short ) vax.console.disasm );
    vax.PC = saved_PC;
    return rc;
}

