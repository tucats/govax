
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     asm_operand.c
//
//  Purpose:    Assemble a single operand from the input stream... called
//              from asm() for as many operands as required for the current
//              instruction.  The operand data is written directly to the
//              current deposit location.
//
//
//  Return codes:
//
//      VAX_ASMINVMODE          A syntax error in specifying the mode made it unparsable.
//      VAX_ASMINVCONST         An invalid hex constant was found
//      VAX_ASMINVREG           And invalid register spec was found
//
//  History:    07/28/97    New header format standardization
//
//              07/24/97    Changed so all memory stores are done via store_memory
//                          so the assembler is compatable with virtual memory when
//                          it's available.
//
//              09/28/98    Added support for short literal and immediate literal
//                          floating point values, like #3 (short) and #19.3 (immediate).


#include "vax.h"
#include "vaxinstr.h"
#include "fpu.h"

//  Local prototypes

#include "asmproto.h"

#define ASM_LIT_NONE        0           //  No literal
#define ASM_LIT_SHORT       1           //  Short literal constant flag
#define ASM_LIT_IMMEDIATE   2           //  Immediate constant flag

/* Table used for short literal floating values */

extern double short_double[ 64 ];

/*
 *  Assemble a single operand, and store to memory.
 */
 
LONGWORD asm_operand( char ** Buffptr )
{

    char * p, *saved_p;
    LONGWORD parsing, access, * access_array, disp, n;
    char ch;
    union VALUE {
        ULONGWORD longword;
        unsigned short word[ 2 ];
        unsigned char byte[ 4 ];
    } value;
    int deferred;
    
    LONGWORD rc;
    unsigned char mode, dtype;
    LONGWORD constant;
    LONGWORD disp_l;
    short disp_w;
    char disp_b;
    
    p = *Buffptr;
    parsing = 1;
    constant = ASM_LIT_NONE;
    rc = VAX_OK;
    
    n = vax.assembler.opcount;
    access_array =  instruction[ vax.assembler.index ].access;
    dtype =         instruction[ vax.assembler.index ].type;
    
    access = GETMODE( access_array, vax.assembler.opcount );

/*
 *  The operand syntax for index mode is horrible, the index value goes AFTER the
 *  base operand (which can be anything).  However, the index coding must be stored
 *  in the memory buffer first.  So scan ahead in the string buffer to see if there
 *  is an index.  If so, we've got ugly ugly work to do.
 *
 *  Note that since we call ourselves recursively, we don't scan for an index on
 *  the recursions (indictated by the parsing_index flag) but rather forge ahead
 *  and parse the base address instead.
 */

    flush_blanks( &p );
    if( ! ( vax.assembler.flags & ASM_PARSING_IDX )) { 

		int running;

		running = 1;
        while( running ) {
        
            ch = *(p++);
            
            /* If we get to EOS with no bracket, we're safe.  We must reset th pointer */
            /* back to the start of the string so we can parse normally from here.     */
            if( isend( ch ) || ch == ',' ) {
                p = *Buffptr;
                break;
            }
            
            /* If we find a bracket, work to do... */
            
            if( ch == '[' ) {
            
                ch = 0; /* Tells asm_reg to start with the 'R' of Rn rather than what's in ch */
                
                rc = asm_reg( ch, &p, ( LONGWORD * ) &value.longword );
                if( rc )
                    return rc;
                mode = ( unsigned char ) ( 0x40 + ( char ) value.longword );
                rc = store_memory( vax.console.deposit++, &mode, 1 );
                if( rc )
                    return rc;

                /* Now that we've dealt with the [Rx] part, recurse and handle the prefix */
                /* Note that we bracket the call with a flag that tells us we are recursing */
                
                vax.assembler.flags |= ASM_PARSING_IDX;
                rc = asm_operand( Buffptr );
                vax.assembler.flags &= ~ASM_PARSING_IDX;

                return rc;
            }
            
        }
    }


/*
 *  Assuming we've either recursively handling an index base (parsing_index was set)
 *  or there was no [Rx] syntax in the string, parse the operand normally.
 */
 
    while( parsing ) {
    
        ch = *(p++);
        
        if( isend( ch ) || ch == ',' ) {
            *Buffptr = p-1;  /* Don't shoot past the end! */
            return VAX_OK;
        }
        
        if( is_blank( ch ))
            continue;
    
    //  If this is a '[' character, skip over the index, we've already handled it.  
    
        if( ch == '[' ) {
			int running;
			running = 1;
            while( running ) {
            
                ch = *(p++);
                if( isend( ch ) || ch == ']' || ch == ',' ) {
                    break;
                }
            }
            
            *Buffptr = p;
            return VAX_OK;
        }



    //  #value          Immediate implicit.  This is used for CHMK, XFC, etc.
    //                  Parse the "#" and then do what OP_BR does, less offsetting.
    //                  Note that we REQUIRE the #constant notation for this type.
    
        if( access == OP_IM ) {
        
            if( ch != '#' )
                return VAX_ASMINVCONST;
            
            p++;        // Set up so branch can decrement again to get constant value
        }
        
    //  address         Branch addressing mode
    //  #value          OR immediate implicit
    
        if( access == OP_BR || access == OP_IM ) {
        
            /* Get the numeric value */
            
            p--;    /* Back up to the "ch" character */
            
            /* Figure disp mode based on data scale */
            
            if( access == OP_BR )
                disp = K_BRANCH_BASE + vax.assembler.scale;
            else
                disp = K_ADDR_L;

            rc = asm_value( &p, &value.longword, disp );
            if( rc )
                return VAX_ASMINVCONST;
            
            /*
                By convention, even though this is a branch mode, you specify it
                by naming the actual location to branch to.  As such, let's convert
                the "absolute" location to a relative distance.  We must account for
                the size since it's an offset from the byte following the displacement.
                
                Obviously, we only do this for branch mode, not implicit immediate.
             */
            
            
            if( access == OP_BR )
                value.longword = value.longword - vax.console.deposit 
                                - vax.assembler.scale;
            
            switch( vax.assembler.scale ) {
            
            case 1:

#if BIGENDIAN
                rc = store_memory( vax.console.deposit, 
                    ( void * ) &value.byte[ 3 ], 1 );
#else
                rc = store_memory( vax.console.deposit, 
                    ( void * ) &value.byte[ 0 ], 1 );
#endif
                if( rc )
                    return rc;
                break;

            case 2:

#if BIGENDIAN
                rc = store_memory( vax.console.deposit, 
                    ( void * ) &value.word[ 1 ], 2 );
#else
                rc = store_memory( vax.console.deposit, 
                    ( void * ) &value.word[ 0 ], 2 );
#endif
                if( rc )
                    return rc;

                break;

            case 4:
                rc = store_memory( vax.console.deposit, 
                     ( void * ) &value.longword, 4 );
                if( rc )
                    return rc;

                break;
            }
            
            vax.console.deposit += vax.assembler.scale;
            continue;
            
        }
        
    //  #literal, based on size we either handle it as a S^# or I^# modes.  It could also be a
    //  floating literal, so we do some odd stuff here.  Either way, we try to set up the variable
    //  "constant" to determine if the value is a literal or short literal value.
    
        if( ch == '#' ) {
        
            saved_p = p;
            disp = K_ADDR_BASE + vax.console.scale[ vax.assembler.opcount ];
            vax.assembler.was_forward = 0;

        /* If the short type is integer, just parse it.  Must be < 64 */
            if( dtype == OP_TYPE_INT ) {
                rc = asm_value( &p, &value.longword, disp ); /* was asm_hex() */
                if( rc )
                    return rc;
            }
        
        /* Otherwise it's float.  Parse the value and look it up in the */
        /* table of value short floats.  If a match, store the index    */
        
            else {
                rc = asm_float( &p );
                if( rc )
                    return rc;
                value.longword = 0xFFFFFFFFUL;
                for( n = 0; n < 64; n++ ) {
                    if( short_double[ n ] == vax.last_float ) {
                        value.longword = 0L;

#if BIGENDIAN
                        value.byte[ 3 ] = ( char ) n;
#else
                        value.byte[ 0 ] = ( char ) n;
#endif


                        break;
                    }
                }
                
                if( value.longword == -1 )
                    value.longword = 0xffff;
                
            }
        
        //  If it was a forward reference, we must assume it's big.  Or, if it's just a big number,
        //  then it must be a literal immediate rather than a short literal.  Note that if it's a
        //  floating value that wasn't in the short_double[] table, then value.longword is set to
        //  a big number.  However, later we must know that it's a bogus value and go get the
        //  vax.last_float number instead.
        
            if( vax.assembler.was_forward || value.longword >= 64 )
                constant = ASM_LIT_IMMEDIATE;
            else
                constant = ASM_LIT_SHORT;
        
        //  If it's an immediate, the displacement for forward references
        //  will be off for one byte (the addressing mode in the instruction).
        
            if( constant == ASM_LIT_IMMEDIATE && vax.assembler.was_forward ) {
                vax.console.last_symbol-> forward-> location += 1;
            }
    
        }
        
        
    //  S^#literal      Literal Mode
    
    //  We may already know it's a short literal by use of # followed by a valid
    //  value.  In that case, we already have the value.  It may also be explicitly
    //  set up as a short literal by using the S^# construct.
    
        if(( constant == ASM_LIT_SHORT ) || (ch == 'S' && *p == '^' )) {
        
            /* If we haven't already parsed the value, do so now */
            
            if( constant != ASM_LIT_SHORT ) {
                if( *(p++) != '^' )
                    return VAX_ASMINVMODE;
                if( *(p++) != '#' )
                    return VAX_ASMINVMODE;
            
            /* If the short type is integer, just parse it.  Must be < 64 */
                if( dtype == OP_TYPE_INT ) {
                    rc = asm_hex( &p, &value.longword );
                    if( rc )
                        return rc;
                    if( value.longword >= 64 )
                        return VAX_ASMINVCONST;
                }
            
            /* Otherwise it's float.  Parse the value and look it up in the */
            /* table of value short floats.  If a match, store the index    */
            
                else {
                    rc = asm_float( &p );
                    if( rc )
                        return rc;
                    value.longword = 0xFFFFFFFFUL;
                    for( n = 0; n < 64; n++ ) {
                        if( short_double[ n ] == vax.last_float ) {
                            value.longword = 0L;
#if BIGENDIAN
                            value.byte[ 3 ] = ( char ) n;
#else
                            value.byte[ 0 ] = ( char ) n;
#endif
                            break;
                        }
                    }
                    
                    if( value.longword == -1 )
                        return VAX_ASMINVCONST;
                
                }
            }
            
            /*  Now store the value away */

#if BIGENDIAN            
            rc = store_memory( vax.console.deposit++, 
                ( void * ) &value.byte[ 3 ], 1 );
#else
            rc = store_memory( vax.console.deposit++, 
                ( void * ) &value.byte[ 0 ], 1 );
#endif
            if( rc )
                return rc;

            continue;
        }
    
        
    //  Rn              Register Mode
    
        if( ch == 'R' || ch == 'S' || ch == 'A' || ch == 'F' || ch == 'P' ) {
            saved_p = p;
            rc = asm_reg( ch, &p, ( LONGWORD * ) &value.longword );
            
            // If it was a valid register, then use it.
            
            if( rc == VAX_OK ) {
                mode = ( unsigned char ) ( 0x50 + ( char ) value.longword );
                rc = store_memory( vax.console.deposit++, &mode, 1 );
                if( rc )
                    return rc;

                continue;
            }
            
            // Nope, not a valid register.  Back up and try something else.
            p = saved_p;
            rc = VAX_OK;
        }
    
    //  -(Rn)           Autodecrement Mode
    
        if( ch == '-' ) {
            flush_blanks( &p );
            if( *(p++) != '(')
                return VAX_ASMINVMODE;
            rc = asm_reg( 0, &p, ( LONGWORD * ) &value.longword );     /* zero indicates no leading character */
            if( rc )
                return rc;
                
            mode = ( unsigned char ) ( 0x70 + ( char ) value.longword );
            rc = store_memory( vax.console.deposit++, &mode, 1 );
            if( rc )
                return rc;

            flush_blanks( &p );
            if( *(p++) != ')' )
                return VAX_ASMINVMODE;
            continue;
        }
  
    //  @(Rn)+          Autoincrement deferred

    
        if( ch == '@' && *p == '(' ) {
        
            p++;
            rc = asm_reg( 0, &p, ( LONGWORD * ) &value.longword );
            if( rc )
                return rc;
            
            flush_blanks( &p );
            if( *(p++) != ')' )
                return VAX_ASMINVMODE;
            
            mode = 0x90;
            if( *p != '+' ) {
                mode = 0xB0;
            }
            else
                p++;
            
            mode = ( unsigned char ) ( mode + ( char ) value.longword );
            rc = store_memory( vax.console.deposit++, &mode, 1 );
            if( rc )
                return rc;

            if( mode >= 0xB0 ) {
                disp_b = 0;
                rc = store_memory( vax.console.deposit++, ( unsigned char * ) &disp_b, 1 );
                if( rc )
                    return rc;
            }
            
            continue;
        }
    
  
    //  (Rn)            Register Deferred Mode
    //  (Rn)+           Autoincrement Mode
    
        if( ch == '(' ) {
            rc = asm_reg( 0, &p, ( LONGWORD * ) &value.longword );
            if( rc )
                return rc;
            
            flush_blanks( &p );
            if( *(p++) != ')' )
                return VAX_ASMINVMODE;
            
            if( *p == '+' ) {
                mode = 0x80;
                p++;
            }
            else
                mode = 0x60;

            mode = ( unsigned char ) ( mode + ( char ) value.longword );
            rc = store_memory( vax.console.deposit++, &mode, 1 );
            if( rc )
                return rc;

            continue;
        }
    
    
    //  I^#constant     Immediate
    
    //  We may already know it's an immediate from parsing the # token by itself.  Is it's not
    //  already known we must read the value (floating or double).
    
        if(( constant == ASM_LIT_IMMEDIATE ) || ( ch == 'I' && *p == '^' )) {
        
        /*  If we haven't parsed the operand prefix already, do so now */
        
            if( constant != ASM_LIT_IMMEDIATE ) {
                if( *(p++) != '^' )
                    return VAX_ASMINVMODE;
                if( *(p++) != '#' )
                    return VAX_ASMINVMODE;
            }
            
        /*  Write the mode value to memory */
        
            mode = 0x8FU;
            rc = store_memory( vax.console.deposit++, &mode, 1 );
            if( rc )
                return rc;

        /*  If we haven't already gotten the value (from previous "#" parsing) then we need */
        /*  to get it now.  It may be a floating value.                                     */
        
            if( constant != ASM_LIT_IMMEDIATE ) {
                disp = K_ADDR_BASE + vax.console.scale[ vax.assembler.opcount ];
                
                if( dtype == OP_TYPE_INT ) {    
                    rc = asm_value( &p, &value.longword, disp );
					if( rc )
                        return rc;
                }
                else {
                    rc = asm_float( &p );
					if( rc )
                        return rc;
                }
            }
            
        /*  At this point, we have the data.  Based on size of the data, write the correct */
        /*  thing.  Note that for 1 and 2 byte values, we know it's an integer.  For 4 byte */
        /*  values, pay attention to the dtype again.  For 8 byte values, we currently are  */
        /*  assuming a dfloat value.                                                        */
        
            if( vax.console.scale[ vax.assembler.opcount ] == 1 ) {

#if BIGENDIAN
                rc = store_memory( vax.console.deposit, 
                       ( void * ) &value.byte[ 3 ], 1 );
#else
                rc = store_memory( vax.console.deposit, 
                       ( void * ) &value.byte[ 0 ], 1 );
#endif

                if( rc )
                    return rc;

            }
            else
            if( vax.console.scale[ vax.assembler.opcount ] == 2 ) {

#if BIGENDIAN
                rc = store_memory( vax.console.deposit, 
                       ( void * ) &value.word[ 1 ], 2 );
#else
                rc = store_memory( vax.console.deposit, 
                       ( void * ) &value.word[ 0 ], 2 );
#endif
                if( rc )
                    return rc;
            }
            else
            if( vax.console.scale[ vax.assembler.opcount ] == 4 ) {
    
                if( dtype == OP_TYPE_FLOAT )
                    rc = fpu_store( vax.last_float, ( LONGWORD * ) &value.longword, 0L /* F_FLOAT */ );
                    
                rc = store_memory( vax.console.deposit, ( void * ) &value.longword, 4 );
                if( rc )
                    return rc;
            }
            else {
                if( dtype == OP_TYPE_FLOAT )
                    rc = fpu_store( vax.last_float, ( LONGWORD * ) &value.longword, &n );
                    
                rc = store_memory( vax.console.deposit, ( void * ) &value.longword, 4 );
				if( rc )
                    return rc;
                    
                vax.console.deposit += 4;
                rc = store_memory( vax.console.deposit, ( void * ) &n, 4 );
				if( rc )
                    return rc;
            }
            
            vax.console.deposit += vax.console.scale[ vax.assembler.opcount ];
            
            continue;
        }
        
        
    //  @#address       Absolute
    
        if( ch == '@' && *p == '#' ) {
            p++;
            mode = 0x9F;
            rc = store_memory( vax.console.deposit++, &mode, 1 );
            if( rc )
                return rc;

            rc = asm_value( &p, &value.longword, K_ADDR_L );
            if( rc )
                return rc;

            rc = store_memory( vax.console.deposit, ( void * ) &value.longword, 4 );
            if( rc )
                return rc;
            vax.console.deposit += 4;
            
            continue;
        }
    
    //  It may be the deferred flavor of addressing.  Check to see if we still have a leading "@"
    //  which tells us to remember it and parse like regular.  The deferred flag will be used to
    //  modify the mode byte written.
    
        if( ch == '@' ) {
        
            deferred = 0x10;
            ch = *p;
            p++;
        }
        else
            deferred = 0;

        
    //  @(rn)               Byte displacement deferred.  The displacement
    //                      is missing, so we assume byte displacement of zero
    
        if( deferred && ( ch == '(' )) {

            disp_b = 0;
            rc = asm_reg( 0, &p, ( LONGWORD * ) &value.longword );
            if( rc )
                return rc;
                
            mode = ( unsigned char ) ( 0x00B0 + ( char ) value.longword );
            if( *(p++) != ')' )
                return VAX_ASMINVMODE;

            rc = store_memory( vax.console.deposit++, &mode, 1 );
            if( rc )
                return rc;

            rc = store_memory( vax.console.deposit++, ( void * ) &disp_b, 1 );
            if( rc )
                return rc;

            continue;
        }
    
        
    //  B^address               Byte relative
    //  B^displacement(Rn)      Byte displacement
    
        if( ch == 'B' && *p == '^' ) {
            p++;
            rc = asm_value( &p, &value.longword, K_DISP_B );
            if( value.longword > 255 )
                return VAX_ASMINVMODE;
            disp_b = ( char ) value.longword;
            
            mode = ( unsigned char ) ( 0xAF + deferred ); /* Add 0x10 if needed */
            
            if( *p == '(' ) {
            
                p++;
                rc = asm_reg( 0, &p, ( LONGWORD * ) &value.longword );
                if( rc )
                    return rc;
                mode = ( unsigned char ) ( 0x00A0 + deferred + ( char ) value.longword );
                if( *(p++) != ')' )
                    return VAX_ASMINVMODE;
            }

            rc = store_memory( vax.console.deposit++, &mode, 1 );
            if( rc )
                return rc;

            rc = store_memory( vax.console.deposit++, ( void * ) &disp_b, 1 );
            if( rc )
                return rc;

            continue;
        }
        
    
    //  W^address               Word relative
    //  W^displacement(Rn)      Word displacement
    
        if( ch == 'W' && *p == '^' ) {
            p++;
            rc = asm_value( &p, &value.longword, K_DISP_W );
            if(value.longword > 65535 )
                return VAX_ASMINVMODE;
            disp_w = ( short ) value.longword;
            
            mode = ( unsigned char ) ( 0xCF + deferred );
            if( *p == '(' ) {
            
                p++;
                rc = asm_reg( 0, &p, ( LONGWORD * ) &value.longword );
                if( rc )
                    return rc;
                mode = ( unsigned char ) ( 0x00C0 + deferred + ( char ) value.longword );
                if( *(p++) != ')' )
                    return VAX_ASMINVMODE;
            }
            
            rc = store_memory( vax.console.deposit++, &mode, 1 );
            if( rc )
                return rc;

            rc = store_memory( vax.console.deposit, ( void * ) &disp_w, 2 );
            if( rc )
                return rc;

            vax.console.deposit += 2;
    
            continue;
        }

    
    //  L^address               Long relative
    //  L^displacement(Rn)      Long displacement
    
        if( ch == 'L' && *p == '^' ) {
            p++;
            rc = asm_value( &p, &value.longword, K_DISP_L );
            if(value.longword > 255 )
                return VAX_ASMINVMODE;
            disp_l = value.longword;
            
            mode = ( unsigned char ) ( 0xEF + deferred );
            if( *p == '(' ) {
            
                p++;
                rc = asm_reg( 0, &p, ( LONGWORD * ) &value.longword );
                if( rc )
                    return rc;
                mode = ( unsigned char ) ( 0x00E0 + deferred + ( char ) value.longword );
                flush_blanks( &p );
                if( *(p++) != ')' )
                    return VAX_ASMINVMODE;
            }
            
            rc = store_memory( vax.console.deposit++, &mode, 1 );
            if( rc )
                return rc;

            rc = store_memory( vax.console.deposit, ( void * ) &disp_l, 4 );
            if( rc )
                return rc;

            vax.console.deposit += 4;
            continue;
        }   

    //  If we got here, it's just a value (probably a symbol).  Treat it like
    //  an absolute value at this point.
    

        p--;    /* Back up so we don't loose first character */
        
        if( rc == VAX_OK ) {
        
            /* Assume we mean @# mode */
            
            mode = 0x9F;
            rc = store_memory( vax.console.deposit++, &mode, 1 );
            if( rc )
                return rc;

            rc = asm_value( &p, &value.longword, K_ADDR_L );
            if( rc )
                return rc;
            disp_l = value.longword;
            
            /* Might be DISP(Rn) format, with no leading size for displacement */

            if( *p == '(' ) {
            
                p++;
                rc = asm_reg( 0, &p, ( LONGWORD * ) &value.longword );
                if( rc )
                    return rc;
                mode = ( unsigned char ) ( 0x00E0 + deferred + ( char ) value.longword );
                flush_blanks( &p );
                if( *(p++) != ')' )
                    return VAX_ASMINVMODE;

                /* Mode byte is changed, so write it using previous addr */
                
                rc = store_memory( vax.console.deposit - 1, &mode, 1 );
                if( rc )
                    return rc;
                    
                /* Also, if it was a forward reference, the fixup must */
                /* be changed from ADDR_L to DISP_L.  We can find the  */
                /* fixup because they are always linked to the head of */
                /* the fixup list when new ones are created.           */
                
                
                if( vax.assembler.was_forward ) 
                    vax.console.last_symbol-> forward-> displacement = K_DISP_L;
            }
            


            rc = store_memory( vax.console.deposit, ( void * ) &disp_l, 4 );
            if( rc )
                return rc;
            vax.console.deposit += 4;
            
            continue;
        }


        return VAX_ASMINVMODE;
    }
    
    return VAX_OK;
}
            



    
