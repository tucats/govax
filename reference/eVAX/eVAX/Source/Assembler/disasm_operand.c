//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     disasm_operand.c
//
//  Purpose:    Disasembles a single operand from memory for the current instruction.
//              This can be called from decode_opcode when disassembly is required.
//
//
//  History:    09/26/99    Replicated from decode_operand.c
//
//              01/13/99    Fixed bug in formatting of indexing operands when they
//                          are not the first operation (too many commas!)

#define VMINLINE 1

#include "vax.h"
#include "vaxinstr.h"
#include "asmproto.h"       // for get_symbol()
#include "fpu.h"            // for fpu_store()
#include "pte.h"


/*
 *  This table is used for S^#n references in instructions where the short type */
 /* is OP_TYPE_FLOAT.  This is actually defined in decode_operand.c             */
 
extern double short_double[ 64 ];


static LONGWORD add_dest( LONGWORD dest );
struct SYMBOL * find_label( LONGWORD dest, LONGWORD flags );


extern char * rom;  /* ROM memory */


//
//  Parse a single operand out of memory.
//
//  May be called somewhat recursively in the case of indexed mode


LONGWORD disasm_operand(          /* The virtual machine state    */
                    struct OPCODE * opcode,     /* The opcode structure         */
                    short opcount,              /* Which operand are we doing?  */
                    LONGWORD * the_pc,              /* Virtual PC address           */
                    short *Treg,                /* Index of temp regs           */
                    short src_scale,            /* Scale of operand             */
                    short idx,                  /* Are we in indexed mode       */
                    char * disasm )             /* Disasm buffer if any         */
{

    unsigned short reg, mode, access;
    LONGWORD index, pc, rc, decode_rc;
    short treg;
#if VMINLINE
    short size;
    ULONGWORD paddr;
#endif

    char d[ 64 ];
    LONGWORD dest;
    struct SYMBOL * sp;
    
    char    byte_offset;

    LONGWORD    off_addr;
    
    char    *fmt, fmtbuff[ 64 ];
    unsigned char optype, dtype;
    double sd;

    
    struct OPCODE local_opcode;
    static char * regname[] = { "R0", "R1", "R2", "R3", "R4", "R5", "R6", "R7",
                         "R8", "R9", "R10", "R11", "AP", "FP", "SP", "PC" };
    
            
    decode_rc = VAX_OK;
    
    access = ( unsigned short ) opcode-> access[ opcount ];
    
    treg = *Treg;
    
    if( opcount > 0 && !idx ) {
        strcat( disasm, "," );
    }
    

    pc = *the_pc;

    /* If this is an OP_IM operand, the operand is a value and it's stored */
    /* directly in the instruction stream, using the size value of this opcode */
    
    if( access == OP_IM ) {
    
        decode_rc = load_register( treg, pc, src_scale );
        if( decode_rc == VAX_OK ) {
            SEXT( vax.reg[ treg ], src_scale );
            
        
            if( vax.console.radix == 16 ) {
                fmt = fmtbuff;
                sprintf( fmt, "#%%0%dlX", src_scale * 2 );
            }
            else
                fmt = "#%ld";
            sprintf( d, fmt, vax.reg[ treg ] );
            strcat( disasm, d );
            pc += src_scale;
            *Treg = treg;
            *the_pc = pc;
          
        }
        return decode_rc;
    }
    
    /* If this is a OP_BR operand, then it's a literal branch displacement */
    /* stored right in the instruction stream.   */
    
    if( access == OP_BR ) {
    
        decode_rc = load_register( treg, pc, src_scale );  /* Read the offset data */

        SEXT( vax.reg[ treg ], src_scale );
        
        if( vax.console.radix == 16 )
            fmt = "%08lX";
        else
            fmt = "%ld";
        
        dest = vax.reg[ treg ] + pc + src_scale;

        if( vax.assembler.flags & ASM_SYMBOLS ) {
            sp = find_label( dest, SYM_LABEL );
        }
        else
            sp = 0L;

        if( sp ) {
            sprintf( d, "%s", sp-> name );
            add_dest( dest );
        }
        else
            sprintf( d, fmt, vax.reg[ treg ] + pc + src_scale );

        strcat( disasm, d );

        
        /* Advance the PC and store the calculated address the displacement refers to */
        
        pc += src_scale;
            
        /* Update the caller and we're done. */
            
        *Treg = treg;
        *the_pc = pc;
            
        return decode_rc;

    }
    
    /* Read the operand addressing mode byte. Break it apart. */

#if !VMINLINE
    if( decode_rc = load_byte( pc, &optype ))
        return decode_rc;
#else

    paddr = pc;
    if( vax.MAPEN ) {
        size = 1;
        decode_rc = vm( &paddr, &size, VM_READ );
        if( decode_rc )
            return decode_rc;
    }


    {
        unsigned char *pb;
        pb = PHYADDR( paddr );
        if( pb == 0L )
            return set_fault( EXC_ACCVIO, 2, pc, 1 );
        optype = *pb;
    }

#endif

    pc++;
    
    reg = ( short) ( optype & 0x0f );
    mode = ( short ) ( optype >> 4 );
    
    
    // Handle literals by placing in a temp register and setting address to it.
    
    if( mode < 4 ) {

        //  If this is a by-address access, use of a short literal is illegal.
        //  Also, if we are supposed to write to this it's illegal.

        //   WAS: if( access == OP_AD || access == OP_MD || access == OP_WR )
            
        if( access != OP_RD )
            decode_rc = set_fault( EXC_RESADDR, 0 );
            
        dtype = instruction[ opcode-> index ].type; /* For short indexing */

        /* If it's an integer instruction type, use the value as-is */
        
        if( dtype == OP_TYPE_INT ) {
            strcat( disasm, "S^#" );
            sprintf( d, "%02X", optype );
            strcat( disasm, d );
            treg++;
        }
        
        else {
        
            /* optype is an index into a table of values */
            
            sd = short_double[ optype ];
            
            strcat( disasm, "S^#" );
            sprintf( d, "%f", sd );
            strcat( disasm, d );
            
        }
        
    }
    else
    
    //  If it's Program Counter addressing mode, handle differently.  These are all
    //  implicitly memory addresses.  We'll be updating the opcode-> memaddr[]
    //  element to a VAX address that can be resolved to a physical address later.
    
    if( mode >= 8 && reg == 0x0F ) {

        switch( mode ) {
        
        case 0x08:  /*  Immediate, size based on scale  */
        
            //  If we are supposed to write to this it's illegal.
            
            if( access == OP_MD || access == OP_WR ) {
                decode_rc = set_fault( EXC_RESADDR, 0 );
            }
            
            load_register( treg, pc, src_scale );
            

            switch ( src_scale ) {
            
            case 1: sprintf( d, "I^#%02X", vax.reg[ treg ] );
                    break;
            
            case 2: sprintf( d, "I^#%04X", vax.reg[ treg ] );
                    break;

            case 4: 
            
                    /* Find out if floating or integer */
                    
                    dtype = instruction[ opcode-> index ].type; /* For short indexing */

                    if( dtype == OP_TYPE_INT ) {

                        if( vax.assembler.flags & ASM_SYMBOLS ) {
                            sp = find_label( vax.reg[treg],
                                  SYM_ENTRY | SYM_LABEL );
                        }
                        else
                            sp = 0L;

                        if( sp ) {
                            sprintf( d, "I^#%s", sp-> name );
                            add_dest( vax.reg[ treg ]);
                        }
                        else
                            sprintf( d, "I^#%08X", vax.reg[ treg ]);
                    }
                    else {
                        rc = fpu_load( vax.reg[ treg ], 0L, &sd);
                        sprintf( d, "I^#%f", sd );
                    }
                    
                    break;
            
            case 8: 
            
                    /* Find out if floating or integer */
                    
                    dtype = instruction[ opcode-> index ].type; /* For short indexing */

                    if( dtype == OP_TYPE_INT ) 
                        sprintf( d, "I^#%08X", vax.reg[ treg ] );
                    else {
                        rc = fpu_load(  vax.reg[ treg ], vax.reg[ treg + 1 ], &sd );
                        sprintf( d, "I^#%f", sd );
                    }
                    
                    break;

            }
            
            strcat( disasm, d );
            pc = pc + src_scale;
            break;
        
        case 0x09:  /*  Absolute    */
            decode_rc = load_register( treg, pc, 4 );

            if( vax.assembler.flags & ASM_SYMBOLS ) {
                sp = find_label( vax.reg[treg],
                                  SYM_ENTRY | SYM_LABEL );
            }
            else
                sp = 0L;

            if( sp ) {
                sprintf( d, "@#%s", sp-> name );
                add_dest( vax.reg[ treg ]);
            }
            else
                sprintf( d, "@#%08X", vax.reg[ treg ]);
            strcat( disasm, d );
            
            pc += 4;
            break;

        case 0x0A:  /*  Byte relative   */
        
            decode_rc = load_register( treg, pc, 1 );
            off_addr = pc;
            byte_offset = ( char ) vax.reg[ treg ];
            off_addr = off_addr + byte_offset;
            off_addr = off_addr + 1;
            
            sprintf( d, "B^%02X", vax.reg[ treg ] );
            strcat( disasm, d );
            add_dest( off_addr );
            
            pc += 1;
            break;
        
        case 0x0B:  /*  Byte relative deferred */

            decode_rc = load_register( treg, pc, 1 );      //  Read the byte offset value
            sprintf( d, "@B^%02X", vax.reg[ treg ] );
            strcat( disasm, d );
            add_dest( pc + vax.reg[treg] + 1 );
            
            pc += 1;
            break;

        case 0x0C:  /*  Word relative   */
        
            decode_rc = load_register( treg, pc, 2 );
            sprintf( d, "W^%02X", vax.reg[ treg ] );
            strcat( disasm, d );
            add_dest( pc + vax.reg[ treg ] + 2 );
            pc += 2;
            break;
        
        case 0x0D:  /*  Word relative deferred */

            decode_rc = load_register( treg, pc, 2 );      //  Read the word offset value
            sprintf( d, "@W^%02X", vax.reg[ treg ] );
            strcat( disasm, d );
            add_dest( pc + vax.reg[ treg ] + 2 );
                        
            pc += 2;
            break;
        
        case 0x0E:  /*  Long relative   */
        
            decode_rc = load_register( treg, pc, 4 );
            sprintf( d, "L^%02X", vax.reg[ treg ] );
            strcat( disasm, d );
            add_dest( pc + vax.reg[ treg ] + 4 );
            pc += 4;
            break;
        
        case 0x0F:  /*  Long relative deferred */

            decode_rc = load_register( treg, pc, 4 );      //  Read the LONGWORD offset value
            sprintf( d, "@L^%02X", vax.reg[ treg ] );
            strcat( disasm, d );
            add_dest( pc + vax.reg[ treg ] + 4 );
            pc += 4;
            break;
        

        }
    }
    else
    switch( mode ) {
    
    case 0x04:  /* Indexed */
    
        // First, see if we're already resolving an index, if so booboo */
        
        if( idx )
            return VAX_ILLADDRFAULT;
        
        index = vax.reg[ reg ] * src_scale;
        
        // Parse the base operand address spec, and store in a temporary here */
        
        local_opcode = *opcode;
        
        decode_rc = disasm_operand( 
                            &local_opcode,
                            opcount,
                            &pc,
                            &treg,
                            src_scale, 1, disasm );
            

        sprintf( d, "[%s]", regname[ reg ] );
        strcat( disasm, d );
        

        break;
        
        
    case 0x05:  /* Register:  R0 */

        sprintf( d, "%s", regname[ reg ] );
        strcat( disasm, d );

        break;
    
    case 0x06:  /* Register deferred: (R0)  */


        sprintf( d, "(%s)", regname[ reg ] );
        strcat( disasm, d );
        
        break;
    
    case 0x07:  /* Autodecrement: -(R0) */

        sprintf( d, "-(%s)", regname[ reg ] );
        strcat( disasm, d );
        
        break;
    
    case 0x08:  /* Autoincrement: (R0)+ */
    
        sprintf( d, "(%s)+", regname[ reg ] );
        strcat( disasm, d );
        
        break;
    
    case 0x09:  /* Autoincrement deferred: @R0+ */
        
        sprintf( d, "@(%s)+", regname[ reg ] );
        strcat( disasm, d );
        break;
    
    
    case 0x0A:  /* Byte displacement:  B^n(r0) */
    
        load_register( treg, pc, 1 );  // 1 = byte
        sprintf( d, "B^%02X(%s)", vax.reg[ treg ], regname[ reg ] );
        strcat( disasm, d );
        pc += 1;
        break;
        
    case 0x0B:  /* Deferred Byte displacement: @B^n(R0) */
    
        load_register( treg, pc, 1 );  // 1 = byte
        sprintf( d, "@B^%02X(%s)", vax.reg[ treg ], regname[ reg ] );
        strcat( disasm, d );
        
        pc += 1;
        break;
        
    case 0x0C:  /* Word displacement: W^n(r0) */
    
        load_register( treg, pc, 2 );  // 2 = word

        sprintf( d, "W^%04X(%s)", vax.reg[ treg ], regname[ reg ] );
        strcat( disasm, d );
        
        pc += 2;
        break;
        
    case 0x0D:  /* Deferred Word displacement */
    
        load_register( treg, pc, 2 );  // 2 = word
        sprintf( d, "@W^%04X(%s)", vax.reg[ treg ], regname[ reg ] );
        strcat( disasm, d );
       pc += 2;
        break;
        
    case 0x0E:  /* Longword displacement: L^n(r0) */
    
        load_register( treg, pc, 4 );  // 4 = longword
        sprintf( d, "L^%08X(%s)", vax.reg[ treg ], regname[ reg ] );
        strcat( disasm, d );
        
        pc += 4;
        break;

    case 0x0F:  /* Deferred Longword displacement */
    
        load_register( treg,pc, 4 );  // 4 = longword
        sprintf( d, "@L^%08X(%s)", vax.reg[ treg ], regname[ reg ] );
        strcat( disasm, d );
        pc += 4;
        break;
        
    
    }  // End of mode case


    *Treg = treg;
    *the_pc = pc;
    
    return decode_rc;
}



static LONGWORD add_dest( LONGWORD dest )
{
    char dbuff[ 16 ];

    if( !( vax.assembler.flags & ASM_BRANCHDEST ))
        return VAX_OK;

    if( vax.assembler.comment[ 0 ] == 0 )
        strcpy( vax.assembler.comment, "; " );
    else
        strcat( vax.assembler.comment, ", " );

    sprintf( dbuff, "%08X", dest );
    strcat( vax.assembler.comment, dbuff );
    return VAX_OK;
}

