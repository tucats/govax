//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX"
//
//  Author:     Tom Cole
//
//  Module:     emul_integer_math.c
//
//  Purpose:    Emulator handlers for integer math instructions
//
//              This single routine handles all integer math instructions.  Floating
//              and packed data will be handled elsewhere.  This routine examines the
//              opcode to determine what the data size and type is, and whether to
//              write back to the second or third operand (ADDL2 vs. ADDL3, etc.).
//
//  History:    07/28/97    New header format standardization
//              01/12/98    Added ADWC
//              09/27/99    Tweaks to make system 64-bit friendly.
//              01/11/00    Fix bug in masking of function code, which was
//                          preventing BIS/BIC instructions from decoding right.
//              12/06/01    Fixed dumb bug in implementation of XORnm (duh).
//


#include "vax.h"


/*
 *  Handler for most common integer math opcodes, such as
 *
 *      ADD, SUB, MUL, DIV, BIS, BIC
 */
 

EMULATOR_ENTRY( emul_integer_math )
{


    char * src1, *src2;
    
    LONGWORD data, d1, d2;
    short dataw;
    char datab;
    ULONGWORD udata = 0L;
    unsigned char op, dsize, opcnt, func;
    
    
    //  Get address of source data (from memory or from register).  Note
    //  that this might cause a page fault.  If so, then the get returns
    //  a null pointer.  In that case, return a fault return code.
    
    src1 = ( char * ) get_operand( opcode, 0, OP_RD );
    if( src1 == 0L )
        return VAX_FAULT;
    
    
    src2 = ( char * ) get_operand( opcode, 1, OP_RD );
    if( src2 == 0L )
        return VAX_FAULT;
    
    /*
        The instructions are structured like this:
    
         0 1 0 0  - - - -       ---F-
         0 1 1 0  - - - -       ---D-
         1 0 0 0  0 0 0 0       ADDB2 
         1 0 0 0  0 0 0 1       ADDB3 
         1 0 0 0  0 0 1 0       SUBB2
         1 0 0 0  0 0 1 1       SUBB3
         1 0 0 0  0 1 0 0       MULB2
         1 0 0 0  0 1 0 1       MULB3
         1 0 0 0  0 1 1 0       DIVB2
         1 0 0 0  0 1 1 1       DIVB3
         1 0 0 0  1 0 0 0       BISB2
         1 0 0 0  1 0 0 1       BISB3
         1 0 0 0  1 0 1 0       BICB2
         1 0 0 0  1 0 1 1       BICB3
         1 0 1 0  - - - -       ---W-
         1 1 0 0  - - - -       ---L-
    
         1 1 0 1  1 0 0 0       ADWC
         
        So let's decompose the opcode to find out what the operations
        are that we'll do, and we can overload this routine with all
        the operations.
     */

    op = opcode-> function;
    
    dsize = ( op >> 5 );        /*  2 = F, 3 = D, 4 = B, 5 = W, 6 = L */
    opcnt = ( op & 0x01 ) + 2;  /*  2 or 3 */
    func  = ( op >> 1 ) & 0x07; /*  0 = ADD, 1 = SUB, 2 = MUL, 3 = DIV  */
                                /*  4 = BIS, 5 = BIC                    */
    
    /* Some of the integer instructions are "outliers".  Let's deal */
    /* with them first by making them conform better.               */
    
    if( op == 0xD8 ) {  /* ADWC */
        dsize = 5;      /* Word */
        func = 0;       /* Add  */
    }
    else
    if( op == 0xD9 ) {  /* SBWC */
        dsize = 5;      /* Word */
        func = 1;       /* Sub  */
    }
    
    
    /* Based on data size, we must manipulate right-sized variables */
    
    switch( dsize ) {
    
    case 4: /* Byte */
    
        d1 = *(( char * ) src1 );   //  First operand
        d2 = *(( char * ) src2 );   //  Second operand
        
        switch( func ) {
        
        case 0:  data = d2 + d1; break;
        case 1:  data = d2 - d1; break;
        case 2:  data = d2 * d1; break;
        case 3:  data = d2 / d1; break;
        
        case 4:  data = d2 | d1;        break;
        case 5:  data = d2 & (~d1 );    break;
        
        }
        
        SETCONDITIONBITS( data, 0L );
        if(( func == 3 ) && ( d1 == 0 ))
            vax.pslw.v = 1;
        else
            vax.pslw.v = ( data > 255 ) || ( data < -256 );
            
        if( func == 4 )
            vax.pslw.c = 0;
        else
            vax.pslw.c = ( data & 0x00000100 ) >> 8;
        
        datab = ( char ) data;
        
        put_operand( opcode, opcode->count - 1, OP_WR, ( char * ) &datab );
        break;
    
    case 5: /* Word */
            
        d1 = *(( short * ) src1 );
        d2 = *(( short * ) src2 );
        
        switch( func ) {
        
        case 0:  data = d2 + d1; break;
        case 1:  data = d2 - d1; break;
        case 2:  data = d2 * d1; break;
        case 3:  data = d2 / d1; break;
        
        case 4:  data = d2 | d1;        break;
        case 5:  data = d2 & (~d1 );    break;
        
        }
        
        // If this is ADWC or SBWC then throw the carry bit into the mix
        
        if( op == 0xD8 )    /* ADWC */
            data = data + vax.pslw.c;
        else
        if( op == 0xD9 )    /* SBWC */
            data = data - vax.pslw.c;
            
            
        SETCONDITIONBITS( data, 0L );
        if(( func == 3 ) && ( d1 == 0 ))
            vax.pslw.v = 1;
        else
            vax.pslw.v = ( data > 32767 ) || ( data < -32768 );
            
        if( func == 4 )
            vax.pslw.c = 0;
        else
            vax.pslw.c = ( data & 0x00010000 ) >> 16;
        
        dataw = ( short ) data;
        put_operand( opcode, opcode->count - 1, OP_WR, ( char * ) &dataw );
        break;
    
    case 6: /* Longword */
    
    
    
        d1 = *(( LONGWORD * ) src1 );
        d2 = *(( LONGWORD * ) src2 );
        
        switch( func ) {
        
        case 0:  data = d2 + d1;
                 udata = ( ULONGWORD ) d1 + (ULONGWORD ) d2;
                 break;
                 
        case 1:  data = d2 - d1;
                 udata = ( ULONGWORD ) d2 - (ULONGWORD ) d1; 
                 break;
                 
        case 2:  data = d1 * d2; 
                 udata = ( ULONGWORD ) d1 * (ULONGWORD ) d2; 
                 break;
                 
        case 3:  data = d2 / d1; 
                 udata = ( ULONGWORD ) d2 / (ULONGWORD ) d1; 
                 break;
                 
        case 4:  data = d2 | d1;
                 udata = ( ULONGWORD ) d2 | ( ULONGWORD ) d1 ;
                 break;
                 
        case 5:  data = d2 & (~d1 );
                 udata = ( ULONGWORD ) d2 & ~(( ULONGWORD ) d1 );
                 break;
        }
        
        SETCONDITIONBITS( data, 0L );
        if( func == 4 )
            vax.pslw.c = 0;
        if(( func == 3 ) && ( d1 == 0 ))
            vax.pslw.v = 1;
        else
            vax.pslw.v =( ( ULONGWORD ) data == udata ) ? 0 : 1;
        put_operand( opcode, opcode->count - 1, OP_WR, ( char * ) &data );
        break;
    
    default:
    
        set_fault( EXC_PRIV, 0 );
        return VAX_FAULT;
    
    }
    


    return VAX_OK;
}


/*
 *  XORxn instructions
 */

EMULATOR_ENTRY( emul_xor )
{

    char    mask_b, src_b;
    short   mask_w, src_w;
    LONGWORD    mask_l, src_l;
    LONGWORD    kind, count, rc;
        
    kind = ( opcode-> function & 0xF0) >> 5;    /* 4 = B, 5 = W, 6 = L */
    count = ( opcode-> function & 0x0F ) == 0x0C ? 2 : 3;

    switch( kind ) {
    
    case    4:  /*  XORBn   */
    
            GET_OPERAND( mask_b, char, 0, OP_RD );
            GET_OPERAND( src_b,  char, 1, OP_RD );
            
            src_b = src_b ^ mask_b;
            
            vax.pslw.n = src_b < 0;
            vax.pslw.z = src_b == 0;
            vax.pslw.v = 0;
            
            rc = put_operand( opcode, count - 1, OP_WR, ( void * ) &src_b );
            if( rc )
                return rc;
            break;
    
    case    5:  /*  XORWn   */
            GET_OPERAND( mask_w, short, 0, OP_RD );
            GET_OPERAND( src_w,  short, 1, OP_RD );
            
            src_w = src_w ^ mask_w;
            
            vax.pslw.n = src_w < 0;
            vax.pslw.z = src_w == 0;
            vax.pslw.v = 0;
            
            rc = put_operand( opcode, count - 1, OP_WR, ( void * ) &src_w );
            if( rc )
                return rc;
            break;
    
    case    6:  /*  XORLn   */

            GET_OPERAND( mask_l, LONGWORD, 0, OP_RD );
            GET_OPERAND( src_l,  LONGWORD, 1, OP_RD );
            
            src_l = src_l ^ mask_l;
            
            vax.pslw.n = src_l < 0;
            vax.pslw.z = src_l == 0;
            vax.pslw.v = 0;
            
            rc = put_operand( opcode, count - 1, OP_WR, ( void * ) &src_l );
            if( rc )
                return rc;
            break;
    
    }
    

    return VAX_OK;
}

