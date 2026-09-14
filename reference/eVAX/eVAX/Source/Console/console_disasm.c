
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_disasm.c
//
//  Purpose:    This console command disassembles instructions in the virtual
//              VAX memory.
//
//
//  History:    07/28/97    New header format standardization
//
//              02/01/98    Change DISASM command to specify range of addresses, 
//                          not a count of instructions.
//
//              12/28/98    Added check for disassembly of a .ENTRY mask. 
//                          There's no way to know if an arbitrary word is
//                          really an entry mask, but if it was created
//                          by a .ENTRY in the mini-assembler, we can at 
//                          least show that.
//
//                          Additionally, added format_mask() which formats
//                          a mask work for disassembly.
//
//              01/06/99    Make the starting address specification able to
//                          be an expression instead of a single value.


#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"

LONGWORD format_operands( struct OPCODE * op );

LONGWORD console_disasm ( char ** P )
{
    LONGWORD rc;
    char * p;
    ULONGWORD value;
    LONGWORD full, verb;
    
    LONGWORD saved_pc, count;
    
/*
 *  If there is no active virtual we are done!
 */
 
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }

   
    p = *P;
    rc = VAX_OK;
    flush_blanks( &p );
    read_verb( &p, &verb );
    if( verb == CHAR4('/','F','U','L')) 
        full = 1;
    else {
        full = 0;
        p = *P;
    }
    
/*
 *  Get the starting address.
 */
        
    rc = asm_expr( &p, ( LONGWORD * ) &value );
    if( rc )
        goto exit;

    /* Get the ending address, if specified. Otherwise, value+1 */
    /* so we disassemble a single instruction.                  */
    
    flush_blanks( &p );
    if( !isend( *p )) {
        rc = asm_value( &p, ( void * ) &count, K_NOFORWARD );
        if( rc )
            goto exit;
        if( ( ULONGWORD ) count < value ) {
            rc = VAX_INVENDADDR;
            goto exit;
        }
    }
    else
        count = value;
        
    saved_pc = vax.PC;
    vax.PC = value;
    
    while( vax.PC <= ( ULONGWORD ) count ) {
    
        rc = decode_instruction( &vax.console.opcode, 1 );
        printf("%s\n", vax.console.opcode.name );
        if( rc == VAX_STEPENTRY )
            rc = VAX_OK;
            
        if( rc != VAX_OK )
            break;
        
        if( full ) {
            format_operands(&vax.console.opcode);
        }
        
    }
    vax.PC = saved_pc;

    
/*
 *  Store parameters back to caller.
 */
 
exit:

    /* *Vax = vax; -- use global vax instead -- */
    *P = p;
    return rc;
}


/*
 *  Shorthand routine to disassemble (and print) a single instruction
 *  to the console.  Must pass the address of the instruction to print.
 */

LONGWORD format_instruction( char * prefix, ULONGWORD addr )
{
    ULONGWORD saved_pc;
    LONGWORD rc;

    saved_pc = vax.PC;
    vax.PC = addr;
    rc = decode_instruction( &vax.console.opcode, 1 );
    printf("    %s %s\n", prefix, vax.console.opcode.name );

    vax.PC = saved_pc;

    return rc;
}

/*
 *  Format a mask word.
 */

char * format_mask( unsigned short mask )
{
    LONGWORD n, first;
    char * mp;
    static char fmtbuf[ 80 ];
    char m[ 8 ];

    strcpy( fmtbuf, "^M<" );
    first = 1;
    for( n = 0; n < 16; n++ ) {
        if(( mask >> n ) & 1 ) {

            if( n == 15 )
                mp = "IV";
            else
            if( n == 14 )
                mp = "DV";
            else {
                sprintf( m, "R%d", n );
                mp = m;
            }

            if( !first )
                strcat( fmtbuf, "," );
            strcat( fmtbuf, mp );
            first = 0;
        }
    }

    strcat( fmtbuf, ">" );
    return fmtbuf;
}

LONGWORD format_operands( struct OPCODE * op )
{
    int n;
    char * mp[] = {
        "none     ",
        "read     ",
        "write    ",
        "modify   ",
        "address  ",
        "bitfield ",
        "branch   ",
        "immediate"};
    
    for( n = 0; n < op->count; n++ ) {                
        printf( "            #%d %s  ", n, mp[op->access[n]] );
        if( op->is_register[ n ] ) {
            if( op-> regnum[ n ] > 15 )
                printf( "T%d = %08X\n", op-> regnum[ n ] - 15, vax.reg[ op-> regnum[n]]);
            else
                printf( "R%d = %08X\n", op->regnum[ n ],
                    vax.reg[ op-> regnum[n]]);
            }
        else
            printf( "%08X\n", op->VAXaddr[ n ]);
    }
    return VAX_OK;
}