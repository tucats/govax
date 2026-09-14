//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     asm_register.c
//
//  Purpose:    Parse register specifications for the seudoassembler of
//              the console interface to the VAX.
//
//
//  History:    02/01/98    Created by removing from asm.c
//


#include "vax.h"
#include "vaxinstr.h"

//  Local prototypes

#include "asmproto.h"

int delimiter( char ch );

    
//  Determine if the character passed is a valid parse delimiter

int delimiter( char ch )
{

    if( ch >= 'A' && ch <= 'Z' )
        return 0;
    
    if( ch >= '0' && ch <= '9' )
        return 0;
    if( ch == '$' )
        return 0;
    if( ch == '_' )
        return 0;
    
    return 1;
}


//  Parse out a register specification and return the register number.
//  Note that the first character of the register is passed in ch, assuming
//  it was already plucked off the string.  If ch contains zero, then it
//  DOES NOT already contain the first character and this routine fetches it.

LONGWORD asm_reg( char ch, char ** P, LONGWORD *Value )
{
    LONGWORD value;
    char * p = *P;
    LONGWORD rc;
    
    if( ch == 0 ) {
        flush_blanks( &p );
        ch = *(p++);
    }
    
    if( ch == 'R' ) {
        rc = asm_dec( &p, &value );
        if( value < 0 || value > 15 )
            return VAX_ASMINVREG;
        if( !delimiter(*p))
            return VAX_ASMINVREG;
            
        *Value = value;
        *P = p;
        return VAX_OK;
    }       
    
    if( ch == 'A' ) {
        if( *(p++) != 'P' )
            return VAX_ASMINVREG;
        if( !delimiter(*p))
            return VAX_ASMINVREG;
        *Value = 12;     /* AP = R12 */
        *P = p;
        return VAX_OK;
    }
    
    if( ch == 'F' ) {
        if( *(p++) != 'P' )
            return VAX_ASMINVREG;
        if( !delimiter(*p))
            return VAX_ASMINVREG;
        *Value = 13;     /* FP = R13 */
        *P = p;
        return VAX_OK;
    }
    
    if( ch == 'S' ) {
        if( *(p++) != 'P' )
            return VAX_ASMINVREG;
        if( !delimiter(*p))
            return VAX_ASMINVREG;
        *Value = 14;     /* SP = R14 */
        *P = p;
        return VAX_OK;
    }
    
    if( ch == 'P' ) {
        if( *(p++) != 'C' )
            return VAX_ASMINVREG;
        if( !delimiter(*p))
            return VAX_ASMINVREG;
        *Value = 15;     /* PC = R15 */
        *P = p;
        return VAX_OK;
    }
    return VAX_ASMINVREG;
}


    
