//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     asm_opcode.c
//
//  Purpose:    Pseudoassembler for the console interface to the VAX.
//
//
//  History:    02/01/98    Initially created from asm.c
//
//              09/24/98    Signal error if attempting to assemble opcode that
//                          has no implemented routine address.
//
//              10/07/99    Added "alias" mechanism to allow fake opcode names to
//                          be mapped to real ones.  Helps support dialects of 
//                          assembler.


#include "vax.h"
#include "vaxinstr.h"

//  Local prototypes

#include "asmproto.h"

static int alias_opcode( char * name );

static struct ALIAS_NAMES {
    char *  alias;
    char *  actual;
    int     dialect;
} alias_names[] = {
{   "JBR",      "JMP",      ASM_DIALECT_GAS },
{   "JNEQU",    ".JNEQ",    ASM_DIALECT_GAS },
{   "JNEQ",     ".JNEQ",    ASM_DIALECT_GAS },
{   "JEQL",     ".JEQL",    ASM_DIALECT_GAS },
{   "JEQLU",    ".JEQL",    ASM_DIALECT_GAS },
{   "BNEQU",    "BNEQ",     ASM_DIALECT_ANY },
{   "BEQLU",    "BEQL",     ASM_DIALECT_ANY },
{   "BGEQU",    "BCC",      ASM_DIALECT_ANY },
{   "BLSSU",    "BCS",      ASM_DIALECT_ANY },
{   "CLRD",     "CLRQ",     ASM_DIALECT_ANY },
{   "CLRF",     "CLRL",     ASM_DIALECT_ANY },
{   "MOVAF",    "MOVAL",    ASM_DIALECT_ANY },
{   "PUSHAF",   "PUSHAL",   ASM_DIALECT_ANY },

{   "",         "",         ASM_DIALECT_ANY }};


//  From current position, find the opcode and remove it.

LONGWORD asm_opcode( char ** Buffptr, LONGWORD * count )
{

    char    b[ 16 ], ch;
    LONGWORD    n, j;
    char    * p;
    LONGWORD    rc;
    
//  Skip spaces
    p = *Buffptr;
    
    flush_blanks( &p );
    
    if( isend( *p ))
        return VAX_ASMNOOPCODE;
    
    n = 0;

//  Copy until end of opcode

    while( !is_blank( *p ) && !isend( *p )) {
    
        ch = *(p++);
        b[ n++ ] = ch;
        
    }
    b[ n ] = 0;

//  Convert opcodes to requird alias (opcode, or in some cases, pseudo-opcode )

    alias_opcode( b );
   
//  Save where we scanned to

    *Buffptr = p;

//  Search the instruction array looking for a matching name

    for( j = 0; instruction[ j ].name[ 0 ]; j++ ) {
    
        if( strcmp( instruction[ j ].name, b ) == 0 ) {

            if( instruction[ j ].extended ) {
                rc = store_memory( vax.console.deposit++, ( void * ) &(instruction[ j ].extended ), 1 );
                if( rc )
                    return rc;
            }

            rc = store_memory( vax.console.deposit++, ( void * ) &(instruction[ j ].opcode ), 1 );
            if( rc )
                return rc;
            
            vax.console.scale = instruction[ j ].scale;

            vax.assembler.index = j;
            vax.assembler.opcount = 0;
            vax.assembler.scale = instruction[ j ].scale[ 0 ];
            
            *count = instruction[ j ].operand_count;
            
            if( instruction[ j ].routine == 0L )
                return VAX_ASMNOTIMP;
                
            return VAX_OK;
        }
    }
    
    return VAX_ASMINVOPCODE;
}


/*
 *  Routine that handles "aliased" opcode names.  Some assemblers allow mnemonic
 *  aliases for instructions.  This routine will handle converting them to the
 *  "real" instruction name.  This is also used to smooth over some wierd stuff
 *  like the GAS instruction "jbr" which means "use a JMP, BRB, or BRW depending
 *  on the destination."  For now, we just convert it to a JMP so it always works.
 */

static int alias_opcode( char * b )
{

    int n;
    
    for( n = 0; n < 100; n++ ) {
    
        if( alias_names[ n ].alias[ 0 ] == 0 )
            break;
        
        //  If we are using a dialect and this is a dialect specific alias,
        //  then handle that.
        
        if( vax.assembler.dialect != ASM_DIALECT_ANY )
            if( alias_names[ n ].dialect != ASM_DIALECT_ANY )
                if( alias_names[ n ].dialect != vax.assembler.dialect )
                    continue;
                    
        if( strcmp( b, alias_names[ n ].alias ) == 0 ) {
            strcpy( b, alias_names[ n ].actual );
            return 1;
        }
    }
    
    return 0;
}

        
            
    
