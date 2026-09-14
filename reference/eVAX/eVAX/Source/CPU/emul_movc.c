//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     emul_movc.c
//
//  Purpose:    Emulator handlers for MOVC3 and MOVC5 instructions
//
//  History:    07/28/97    New header format standardization
//
//              07/23/99    Added additional misc character instructions
//
//              09/28/99    Bug fix in MOVC5 from Sergey Tikhonov <tsv@excom.spb.su>




#include "vax.h"


EMULATOR_ENTRY( emul_movtuc )
{

    unsigned short tmp1, tmp3;
    ULONGWORD tmp2, tmp4, tbladdr;
    unsigned char esc, ch;
    LONGWORD rc;
    
    
    GET_OPERAND( tmp1, unsigned short, 0, OP_RD );  /* srclen.rw    */
    
    tmp2 = opcode-> VAXaddr[ 1 ];                   /* srcaddr.ab   */
    
    GET_OPERAND( esc, unsigned char, 2, OP_RD );    /* esc.rb       */
    
    tbladdr = opcode-> VAXaddr[ 3 ];                /* tbladdr.ab   */
    
    GET_OPERAND( tmp3, unsigned short, 4, OP_RD );  /* dstlen.rw    */
    
    tmp4 = opcode-> VAXaddr[ 5 ];                   /* dstaddr.ab   */
    
    
    vax.pslw.n = ( signed short ) tmp1 < ( signed short ) tmp3;
    vax.pslw.z = tmp1 == tmp3;
    vax.pslw.c = tmp1 < tmp3;
    vax.pslw.v = 0;
    
    if( tmp1 > 0 && tmp3 > 0 ) {
    
        while( tmp1 & tmp3 ) {

            rc = load_byte( tmp2, &ch );
            if( rc )
                return rc;
            
            rc = load_byte( tbladdr + ch, &ch );
            if( rc )
                return rc;
            
            if( ch == esc ) {
                vax.pslw.v = 1;
                break;
            }
            
            rc = store_memory( tmp4, &ch, 1 );
            if( rc )
                return rc;
            
            tmp1--;
            tmp2++;
            tmp3--;
            tmp4++;
        }
    }
    
    vax.R0 = tmp1;
    vax.R1 = tmp2;
    vax.R3 = 0;
    vax.R4 = tmp3;
    vax.R5 = tmp4;
    
    return VAX_OK;
}




EMULATOR_ENTRY( emul_movtc )
{
    unsigned char ch, fill;
    unsigned short tmp1, tmp3, srclen, dstlen;
    ULONGWORD tmp2, tmp4, tbladdr, tmp5, tmp6;
    
    LONGWORD rc;
    
    GET_OPERAND( tmp1, unsigned short, 0, OP_RD );      /* srclen.rw        */
    srclen = tmp1;
    
    tmp2 = opcode-> VAXaddr[ 1 ];                       /* srcaddr.ab       */

    GET_OPERAND( fill, unsigned char, 2, OP_RD );       /* fill.rb          */
    
    tbladdr = opcode-> VAXaddr[ 3 ];                    /* tbladdr.ab       */

    GET_OPERAND( tmp3, unsigned short, 4, OP_RD );      /* dstlen.rw        */
    dstlen = tmp3;
            
    tmp4 = opcode-> VAXaddr[ 5 ];                       /* dstaddr.ab       */
    
    
    
    if( tmp2 > tmp4 ) {
    
        while(( tmp1 != 0 ) & ( tmp3 != 0 )) {
            
            /* Get the source data byte */
            
            rc = load_byte( tmp2, &ch  );
            if( rc )
                return rc;
            
            /* Use it to index into table to get translated byte */
            
            rc = load_byte( ( LONGWORD ) tbladdr + ch, &ch );
            if( rc )
                return rc;
            
            /* Store translated byte in destination */
            
            rc = store_memory( tmp4, &ch, 1 );
            if( rc )
                return rc;
            
            /* Update pointers and lengths */
            tmp1--;
            tmp2++;
            tmp3--;
            tmp4++;
        }
        
        while( tmp3 ) {
            rc = store_memory( tmp4, &fill, 1 );
            if( rc )
                return rc;
            tmp3--;
            tmp4++;
        }
        vax.R1 = tmp2;
        vax.R5 = tmp4;
    }
    
    else {
        if( tmp1 < tmp3 )
            tmp5 = tmp1;
        else
            tmp5 = tmp3;
        
        tmp6 = tmp3;
        tmp2 = tmp2 + tmp5;
        tmp4 = tmp4 + tmp6;
        
        while( tmp3 > tmp1 ) {
            tmp3--;
            tmp4--;
            rc = store_memory( tmp4, &fill, 1 );
            if( rc )
                return rc;
        }
        
        while( tmp3 ) {
            tmp1--;
            tmp2--;
            tmp3--;
            tmp4--;
            
            rc = load_byte( tmp2, &ch );
            if( rc )
                return rc;
            
            rc = load_byte( tbladdr + tmp2, &ch );
            if( rc )
                return rc;
            
            rc = store_memory( tmp4, &ch, 1 );
            if( rc )
                return rc;
        }
        vax.R1 = tmp2 + tmp5;
        vax.R5 = tmp4 + tmp6;
    }
    
    vax.R0 = tmp1;
    vax.R2 = 0;
    vax.R3 = tbladdr;
    vax.R4 = 0;
    
    vax.pslw.n = ( signed short ) srclen < ( signed short ) dstlen;
    vax.pslw.z = srclen == dstlen;
    vax.pslw.v = 0;
    vax.pslw.c = srclen < dstlen;
    
    return VAX_OK;
}





EMULATOR_ENTRY( emul_scanc )
{
    unsigned char test, mask, ch;
    short len;
    LONGWORD n, rc;
    LONGWORD addr, tbladdr;
    
    GET_OPERAND( len, short, 0, OP_RD );
    GET_OPERAND( mask, char, 3, OP_RD );
    
    addr = opcode-> VAXaddr[ 1 ];
    tbladdr = opcode-> VAXaddr[ 2 ];
    
    test = ( opcode-> function == 0x2A ) ? 0xff : 0;
    
    vax.pslw.n = 0;
    vax.pslw.z = 0;
    vax.pslw.v = 0;
    vax.pslw.c = 0;
    
    for( n = 0; n < len; n++ ) {
    
        /* Get the byte from the data area */
        
        rc = load_byte( addr + n, &ch  );
        if( rc )
            return rc;
        
        /* Use it to read the indexed entry in the table */
        
        rc = load_byte( tbladdr + ch, &ch  );
        
        /* Mask it.  */
        ch = ch & mask;
        
        if(( test && ch ) || (!test && !ch)) {
            vax.pslw.z = 1;
            break;
        }
    }
    
    vax.R0 = ( len - n );
    vax.R1 = addr + n;
    vax.R2 = 0;
    vax.R3 = tbladdr;
    
    return VAX_OK;
}

    



EMULATOR_ENTRY( emul_movc3 )
{

    char    src1;

    short   len, *len_a;

    LONGWORD    tmp1, tmp2, tmp3, tmp4;

//  See if the addresses are registers

    if( opcode-> is_register[ 1 ] || opcode-> is_register[ 2 ])
        return set_fault( EXC_RESOP, 0 );

//  Get addressability to the length.

    len_a = ( short * ) get_operand( opcode, 0, OP_RD );
    if( !len_a )
        return VAX_FAULT;
    len = *len_a;

//  Load up the working values

    tmp1 = len;
    tmp2 = opcode-> VAXaddr[ 1 ];
    tmp3 = opcode-> VAXaddr[ 2 ];

//  We have to handle overlap correctly, so there are really two versions of the loop.

    if( tmp2  > tmp3 ) {
    
        while( tmp1 > 0 ) {
            
            load_memory( tmp2, ( void * ) &src1, 1 );
            store_memory( tmp3, ( void * ) &src1, 1 );
            tmp1--;
            tmp2++;
            tmp3++;
        }
        
        vax.R1 = tmp2;
        vax.R3 = tmp3;
        
    }
    else {
    
        tmp4 = tmp1;
        tmp2 = tmp2 + tmp1;
        tmp3 = tmp3 + tmp1;
        
        while( tmp1 > 0 ) {
            tmp1--;
            tmp2--;
            tmp3--;
            load_memory( tmp2, ( void * ) &src1, 1 );
            store_memory( tmp3, ( void * ) &src1, 1 );
        }
        
        vax.R1 = tmp2 + tmp4;
        vax.R3 = tmp3 + tmp4;
    }
    
    vax.R0 = 0L;
    vax.R2 = 0L;
    vax.R4 = 0L;
    vax.R5 = 0L;

    vax.pslw.v = 0;
    vax.pslw.n = 0;
    vax.pslw.z = 1;
    vax.pslw.c = 0;
    
    return VAX_OK;
}




EMULATOR_ENTRY( emul_movc5 )
{

    char    src1, fill, *fill_a;

    short   len1, len2, *len_a;

    LONGWORD    tmp1, tmp2, tmp3, tmp4, tmp5, tmp6;

//  See if the addresses are registers

    if( opcode-> is_register[ 1 ] || opcode-> is_register[ 4 ])
        return set_fault( EXC_RESOP, 0 );

//  Get addressability to the source length.

    len_a = ( short * ) get_operand( opcode, 0, OP_RD );
    if( !len_a )
        return VAX_FAULT;
    len1 = *len_a;

//  Get addressability to the destination length.

    len_a = ( short * ) get_operand( opcode, 3, OP_RD );
    if( !len_a )
        return VAX_FAULT;
    len2 = *len_a;

//  Get the fill byte

    fill_a = ( char * ) get_operand( opcode, 2, OP_RD );
    if( !fill_a )
        return VAX_FAULT;
    fill = *fill_a;

    
//  Load up working registers

    tmp1 = len1;
    tmp2 = opcode-> VAXaddr[ 1 ];
    tmp3 = len2;
    tmp4 = opcode-> VAXaddr[ 4 ];

//  We have to handle overlap correctly, so there are really two versions of the loop.

    if( tmp2  > tmp4 ) {
    
        while( tmp1 != 0 && tmp3 != 0 ) {
            
            load_memory( tmp2, ( void * ) &src1, 1 );
            store_memory( tmp4, ( void * ) &src1, 1 );
            tmp1--;
            tmp2++;
            tmp3--;
            tmp4++;
        }
        
        while( tmp3 != 0 ) {
            store_memory( tmp4, ( void * ) &fill, 1 );
            tmp3--;
            tmp4++;
        }
    
        vax.R1 = tmp2;
        vax.R3 = tmp4;
    }
    else {
    
        tmp5 = ( ( ULONGWORD ) tmp1 < ( ULONGWORD ) tmp3 ) ? tmp1 : tmp3;
                /* MINU( tmp1, tmp3 ) */
                
        tmp6 = tmp3;
        tmp2 = tmp2 + tmp5;
        tmp4 = tmp4 + tmp6;
        
        while( tmp3 > tmp1 ) {
            tmp3--;
            tmp4--;
            store_memory( tmp4, ( void * ) &fill, 1 );
        }
        
        while( tmp3 != 0 ) {
            tmp1--;
            tmp2--;
            tmp3--;
            tmp4--;
            load_memory( tmp2, ( void * ) &src1, 1 );
            store_memory( tmp4, ( void * ) &src1, 1 );
        }
        
        vax.R1 = tmp2 + tmp5;
        vax.R3 = tmp4 + tmp6;
    }
    
    vax.R0 = tmp1;
    vax.R2 = 0;
    vax.R4 = 0;
    vax.R5 = 0;
    
    vax.pslw.n = ( len1 < len2 );
    vax.pslw.z = ( len1 == len2 );
    vax.pslw.v = 0;
    vax.pslw.c = ( ( unsigned short ) len1 < ( unsigned short ) len2 );
        
    return VAX_OK;
}


