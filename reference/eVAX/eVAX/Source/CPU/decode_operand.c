//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     decode_operand.c
//
//  Purpose:    Decodes a single operand from memory for the current instruction.
//              This is called from decode_opcode() and assumes that the virtual
//              cpu context in the console block has been set up to reflect the
//              current instruction being decoded (this is needed to determing
//              operand type, count, etc.).
//
//  History:    07/28/97    New header format standardization
//
//              09/24/98    Added support for short literal and immediate literal
//                          floating point values.
//
//              12/28/98    Add symbol formatting to disassembly.  If we are
//                          disassembling, then absolute and branch addresses
//                          are checked to see if a SYM_LABEL symbol exists
//                          for the value.
//
//              01/06/99    Make symbolic resolution conditional.  Also
//                          conditionally show real address as a comment.
//
//              06/11/99    Correct error in relative, relative deferred modes,
//                          where offset wasn't being made relative to PC after
//                          operand, but before it.  This meant offset was wrong
//                          by 1, 2, or 4 bytes.  Bad.
//
//              09/26/99    Removed disasm capability from this routine; it's now
//                          handled by disasm_opcode which is called from the
//                          decode_opcode routine if needed.
//

#define VMINLINE 1

#include "vax.h"
#include "vaxinstr.h"
#include "asmproto.h"       // for get_symbol()
#include "fpu.h"            // for fpu_store()
#include "pte.h"


extern char * rom;  /* ROM memory */


/*
 *  This table is used for S^#n references in instructions where the short type */
 /* is OP_TYPE_FLOAT.  In this case, the actual nibble in the instruction is an */
 /* index into this table.  This table is exported rather than static so it can */
 /* be shared by the assembler generating the instructions.                     */
 
double short_double[ 64 ] = 
{
/*  .           .           .           .           .           .           .           .       */
    1./2.,      9./16.,     5./8.,      11./16.,    3./4.,      13./16.,    7./8.,      15./16.,
    1.,         9./8.,      5./4.,      11./8.,     3./2.,      13./8.,     7./4.,      15./8.,
    2.,         9./4.,      5./2.,      11./4.,     3.,         13./4.,     7./2.,      15./4.,
    4.,         9./2.,      5.,         11./2.,     6.,         13./2.,     7.,         15./2.,
    8.,         9.,         10.,        11.,        12.,        13.,        14.,        15.,
    16.,        18.,        20.,        22.,        24.,        26.,        28.,        30.,
    32.,        36.,        40.,        44.,        48.,        52.,        56.,        60.,
    64.,        72.,        80.,        88.,        96.,        104.,       112.,       120.
};


struct SYMBOL * find_label( LONGWORD dest, LONGWORD flags );

LONGWORD mode_profile[ 256 ] = { 0 };

//
//  Parse a single operand out of memory.
//
//  May be called somewhat recursively in the case of indexed mode


LONGWORD decode_operand(          /* The virtual machine state    */
                    struct OPCODE * opcode,     /* The opcode structure         */
                    short opcount,              /* Which operand are we doing?  */
                    LONGWORD * the_pc,              /* Virtual PC address           */
                    short *Treg,                /* Index of temp regs           */
                    short src_scale,            /* Scale of operand             */
                    short idx )                 /* Are we in indexed mode       */
{

    unsigned short reg, mode, access;
    LONGWORD index, pc, rc, decode_rc;
    short treg;
#if VMINLINE
    short size;
    ULONGWORD paddr;
#endif

    char    byte_offset;
    short   word_offset;
    LONGWORD    long_offset;
    LONGWORD    off_addr;
    
    unsigned char optype, dtype;
    double sd;
    LONGWORD sdlong1, sdlong2;
    
    struct OPCODE local_opcode;    
        
    opcode-> address[ opcount ] = 0L;
    opcode-> is_register[ opcount ] = OP_REGISTER;  // Most common case
    opcode-> size[ opcount ] = ( char ) src_scale;

    decode_rc = VAX_OK;
    
    access = ( unsigned short ) opcode-> access[ opcount ];
    
    treg = *Treg;    

    pc = *the_pc;

/*
 *  Immediate and Branch operands get special handling, but they are not the most
 *  common modes.  Let's only do this if we know the access mode is BR or IM...
 */
 
    if( access >= OP_BR ) {
    
        /* If this is an OP_IM operand, the operand is a value and it's stored */
        /* directly in the instruction stream, using the size value of this opcode */
        
        if( access == OP_IM ) {
        
            decode_rc = load_register( treg, pc, src_scale );
            if( decode_rc == VAX_OK ) {
                SEXT( vax.reg[ treg ], src_scale );
                            
                opcode-> VAXaddr[ opcount ] = vax.reg[ treg ];
                opcode-> address[ opcount ] = ( unsigned char * ) &( vax.reg[ treg ]);
                opcode-> regnum[ opcount ] = treg;
                treg++;
                pc += src_scale;
                *Treg = treg;
                *the_pc = pc;
                    
                /* Show that this operand references a memory location */
                
                opcode-> is_register[ opcount ] = OP_TEMPORARY;

            }
            return decode_rc;
        }
        
        /* If this is a OP_BR operand, then it's a literal branch displacement */
        /* stored right in the instruction stream.   */
        
        if( access == OP_BR ) {
        
            decode_rc = load_register( treg, pc, src_scale );  /* Read the offset data */

            SEXT( vax.reg[ treg ], src_scale );
            
          
            /* Advance the PC and store the calculated address the displacement refers to */
            
            pc += src_scale;
            opcode-> VAXaddr[ opcount ] =  pc + vax.reg[ treg ];
                
            /* Update the caller and we're done. */
                
            *Treg = treg;
            *the_pc = pc;
        
            /* Show that this operand references a memory location */
            
            opcode-> is_register[ opcount ] = OP_MEMORY;
            
            return decode_rc;

        }
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
    
    reg = ( unsigned short ) ( optype & 0x0f );
    mode = ( unsigned short ) ( optype >> 4 );
    mode_profile[ optype ]++;

/*
 *  Experimental optimization.  The most common addressing mode is Rn register
 *  mode.  So let's special case it first before all the other junk, and sort
 *  of "inline" all the work here to see if it makes the system run faster...
 *
 *  Empirical testing says it runs a teeny bit (2-3%) faster, so we'll go with it.
 */
 
    if( mode == 5 ) {
        opcode-> address[ opcount ] = ( unsigned char * ) &( vax.reg[ reg ]);
        opcode-> regnum[ opcount ] = reg;

#if BIGENDIAN

        if( src_scale == 4 )
            ;
        else
        /* If it's a byte, point to the LSB of the register */
        if( src_scale == 2 )
            opcode-> address[ opcount ] += 2;
        
        /* If it's a word, point to the LSW of the register */
        else
        if( src_scale == 1 )
            opcode-> address[ opcount ] += 3;
#endif

        *Treg = treg;
        *the_pc = pc;
        return VAX_OK;    

    }
    
    
    // Handle literals by placing in a temp register and setting address to it.
    
    if( mode < 4 ) {

        //  If this is a by-address access, use of a short literal is illegal.
        //  Also, if we are supposed to write to this it's illegal.

        //   WAS: if( access == OP_AD || access == OP_MD || access == OP_WR )
            
        if( access != OP_RD )
            decode_rc = set_fault( EXC_RESADDR, 0 );
            
        vax.reg[ treg ] = optype;
        dtype = instruction[ opcode-> index ].type; /* For short indexing */

        /* If it's an integer instruction type, use the value as-is */
        
        if( dtype == OP_TYPE_INT ) {
            opcode-> address[ opcount ] = 
                              ( unsigned char * ) &( vax.reg[ treg ]);
            opcode-> regnum[ opcount ] = treg;
            treg++;
        }
        
        else {
        
            /* optype is an index into a table of values */
            
            sd = short_double[ optype ];
            
            rc = fpu_store( sd, &sdlong1, src_scale == 8 ? &sdlong2 : 0L /* F_FLOAT */ );
            
            /* Write 4 or 8 bytes to temporary storage */
            
            if( src_scale == 4 ) {
                vax.reg[ treg + 0 ] = sdlong1;
            }
            else {
                vax.reg[ treg + 0 ] = sdlong1;
                vax.reg[ treg + 1 ] = sdlong2;
            }
        
            opcode-> address[ opcount ]  = 
                ( unsigned char * ) &( vax.reg[ treg ]);
            opcode-> regnum[ opcount ] = treg;
            treg = ( short ) ( treg + ( src_scale ==  4 ? 1 : 2 ));
            
        }
        
        //  Show that this operand references a non-addressable location
        
        opcode-> is_register[ opcount ] = OP_TEMPORARY;
    }
    else
    
    //  If it's Program Counter addressing mode, handle differently.  These are all
    //  implicitly memory addresses.  We'll be updating the opcode-> memaddr[]
    //  element to a VAX address that can be resolved to a physical address later.
    
    if( mode >= 8 && reg == 0x0F ) {

        //  Since PC mode always means a memory location, set the status flag now.
            
        opcode-> is_register[ opcount ] = OP_MEMORY;

        switch( mode ) {
        
        case 0x08:  /*  Immediate, size based on scale  */
        
            //  If we are supposed to write to this it's illegal.
            
            if( access == OP_MD || access == OP_WR ) {
                decode_rc = set_fault( EXC_RESADDR, 0 );
            }
            
            load_register( treg, pc, src_scale );
            opcode-> address[ opcount ] = ( unsigned char * ) &( vax.reg[ treg ] );
            opcode-> regnum[ opcount ] = treg;
            opcode-> VAXaddr[ opcount ] = pc;
            
            treg ++;
            if( src_scale > 4 )
                treg++;
                
            pc = pc + src_scale;
            break;
        
        case 0x09:  /*  Absolute    */
            decode_rc = load_register( treg, pc, 4 );            
            opcode-> VAXaddr[ opcount ] =  vax.reg[ treg ];   /* Address is actual data from opcode */
            pc += 4;
            break;

        case 0x0A:  /*  Byte relative   */
        
            decode_rc = load_register( treg, pc, 1 );
            off_addr = pc;
            byte_offset = ( char ) vax.reg[ treg ];
            off_addr = off_addr + byte_offset;
            off_addr = off_addr + 1;
            opcode-> VAXaddr[ opcount ]  = off_addr;
            pc += 1;
            break;
        
        case 0x0B:  /*  Byte relative deferred */

            decode_rc = load_register( treg, pc, 1 );      //  Read the byte offset value
            byte_offset = ( char ) vax.reg[ treg ];           //  Convert to signed value
            load_register( treg, pc + ( signed char ) byte_offset + 1, 4 );//  Load it's contents
            opcode-> VAXaddr[ opcount ]  =  vax.reg[ treg ];  //  And return address of contents
            
            pc += 1;
            break;

        case 0x0C:  /*  Word relative   */
        
            decode_rc = load_register( treg, pc, 2 );
            word_offset = ( short ) vax.reg[ treg ];
            opcode-> VAXaddr[ opcount ] = pc + word_offset + 2;
            pc += 2;
            break;
        
        case 0x0D:  /*  Word relative deferred */

            decode_rc = load_register( treg, pc, 2 );      //  Read the word offset value
            word_offset = ( short ) vax.reg[ treg ];          //  Convert to signed value
            load_register( treg, pc + word_offset + 2, 4 );//  Load it's contents
            opcode-> VAXaddr[ opcount ] =  vax.reg[ treg ];   //  And return address of contents
            
            pc += 2;
            break;
        
        case 0x0E:  /*  Long relative   */
        
            decode_rc = load_register( treg, pc, 4 );
            long_offset = vax.reg[ treg ];
            opcode-> VAXaddr[ opcount ] = pc + long_offset + 4;
            pc += 4;
            break;
        
        case 0x0F:  /*  Long relative deferred */

            decode_rc = load_register( treg, pc, 4 );      //  Read the LONGWORD offset value
            long_offset = vax.reg[ treg ];                    //  Convert to signed value
            load_register( treg, pc + long_offset + 4, 4 );//  Load it's contents of address of value
            opcode-> VAXaddr[ opcount ] =  vax.reg[ treg ];   //  And return address of contents
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
        
        decode_rc = decode_operand( 
                            &local_opcode,
                            opcount,
                            &pc,
                            &treg,
                            src_scale, 1  /* , disasm */ );
           
        //  Indexed mode always implies a memory operand
        
        opcode-> is_register[ opcount ] = OP_MEMORY;

        
        // Finally, compute the indexed offset and return it
        
        opcode-> VAXaddr[ opcount ] = local_opcode.VAXaddr[ opcount ] + index;
        break;
        
        
    case 0x05:  /* Register:  R0 */
        opcode-> address[ opcount ] = ( unsigned char * ) &( vax.reg[ reg ]);
        opcode-> regnum[ opcount ] = reg;
        break;
    
    case 0x06:  /* Register deferred: (R0)  */

        opcode-> is_register[ opcount ] = OP_MEMORY;
        opcode-> VAXaddr[ opcount ] = vax.reg[ reg ];
        break;
    
    case 0x07:  /* Autodecrement: -(R0) */
        opcode-> is_register[ opcount ] = OP_MEMORY;
        vax.reg[ reg ] -= src_scale;
        opcode-> VAXaddr[ opcount ] = vax.reg[ reg ];
        break;
    
    case 0x08:  /* Autoincrement: (R0)+ */
    
        opcode-> is_register[ opcount ] = OP_MEMORY;
        opcode-> VAXaddr[ opcount ]  =  vax.reg[ reg ];
        vax.reg[ reg ] += src_scale;
        break;
    
    case 0x09:  /* Autoincrement deferred: @R0+ */
    
        // Register is address of memory, which contains address of operand
        
        opcode-> is_register[ opcount ] = OP_MEMORY;

        /* Get address of memory location pointed to by register into temp space */

        load_register( treg, vax.reg[ reg ], 4 );
        
        /* old:  lp = ( LONGWORD * ) &( vax.memory[ vax.reg[ reg ]]); */
        
        /* Dereference and use and new memory location which is operand address */
        /* Note that we re-use the temporary register                           */
        
        load_register( treg, vax.reg[ treg ], 4 );

        vax.reg[ reg ] += 4;

        opcode-> address[ opcount ] =  ( unsigned char * ) &( vax.reg[ treg ]);
        treg++;
        break;
    
    
    case 0x0A:  /* Byte displacement:  B^n(r0) */
    
        load_register( treg, pc, 1 );  // 1 = byte
        byte_offset = ( char ) vax.reg[ treg ];
        vax.reg[ treg ] = vax.reg[ reg ] + ( signed char ) byte_offset;
        opcode-> is_register[ opcount ] = OP_MEMORY;
        opcode-> VAXaddr[ opcount ] = vax.reg[ treg ];
        treg++;
        pc += 1;
        break;
        
    case 0x0B:  /* Deferred Byte displacement: @B^n(R0) */
    
        load_register( treg, pc, 1 );  // 1 = byte
        byte_offset = ( char ) vax.reg[ treg ];
        vax.reg[ treg ] = vax.reg[ reg ] + ( LONGWORD ) byte_offset;
        load_register( treg, vax.reg[ treg ], 4 );
        
        opcode-> is_register[ opcount ] = OP_MEMORY;
        opcode-> VAXaddr[ opcount ] =  vax.reg[ treg ];
        treg++;
        pc += 1;
        break;
        
    case 0x0C:  /* Word displacement: W^n(r0) */
    
        load_register( treg, pc, 2 );  // 2 = word
        word_offset = ( short ) vax.reg[ treg ];
        vax.reg[ treg ] = vax.reg[ reg ] + ( LONGWORD ) word_offset;
        opcode-> VAXaddr[ opcount ] = vax.reg[ treg ];
        treg++;
        opcode-> is_register[ opcount ] = OP_MEMORY;
        pc += 2;
        break;
        
    case 0x0D:  /* Deferred Word displacement */
    
        load_register( treg, pc, 2 );  // 2 = word
        word_offset = ( short ) vax.reg[ treg ];
        vax.reg[ treg ] = vax.reg[ reg ] + ( LONGWORD ) word_offset;
        load_register( treg, vax.reg[ treg ], 4 );
        
        opcode-> is_register[ opcount ] = OP_MEMORY;
        opcode-> VAXaddr[ opcount ] = vax.reg[ treg ];
        treg++;
        pc += 2;
        break;
        
    case 0x0E:  /* Longword displacement: L^n(r0) */
    
        load_register( treg, pc, 4 );  // 4 = longword
        long_offset = vax.reg[ treg ];
        vax.reg[ treg ] = vax.reg[ reg ] + ( LONGWORD ) long_offset;
        opcode-> VAXaddr[ opcount ] =  vax.reg[ treg ];
        treg++;
        opcode-> is_register[ opcount ] = OP_MEMORY;
        pc += 4;
        break;

    case 0x0F:  /* Deferred Longword displacement */
    
        load_register( treg,pc, 4 );  // 4 = longword
        long_offset = vax.reg[ treg ];
        vax.reg[ treg ] = vax.reg[ reg ] + ( LONGWORD ) long_offset;
        opcode-> is_register[ opcount ] = OP_MEMORY;
        load_register( treg, vax.reg[ treg ], 4 );
        opcode-> VAXaddr[ opcount ] = vax.reg[ treg ];
        treg++;
        pc += 4;
        break;
        
    
    }  // End of mode case


    /*  If we resolved the operand to a real memory address, that   */
    /*  means it's in a virtual VAX register.  We need to adjust    */
    /*  the pointer based on the expected size of the operand, so   */
    /*  word and byte addressing work correctly for data of that    */
    /*  size.                                                       */
    
    /*  Note that this is NOT the same as saying the operand itself */
    /*  is a register, lots of memory address operations result in  */
    /*  a temporary calculation in a virtual VAX temp register that */
    /*  really points to a memory location.                         */
    
    /*  NOTE:  THIS IS VERY NON-PORTABLE, and assumes that the      */
    /*  emulator is implemented on a big-endian machine (like PPC)  */

#if BIGENDIAN
    if( opcode-> address[ opcount ] ) {
    
        /* If it's a byte, point to the LSB of the register */
        
        if( src_scale == 1 )
            opcode-> address[ opcount ] += 3;
        
        /* If it's a word, point to the LSW of the register */
        else
        if( src_scale == 2 )
            opcode-> address[ opcount ] += 2;
    }
#endif

    *Treg = treg;
    *the_pc = pc;
    
    return decode_rc;
}




struct SYMBOL * find_label( LONGWORD dest, LONGWORD flags )
{
    struct SYMBOL * sp;
    LONGWORD found, is_label;

    found = 0;
    for( sp = vax.console.symbols; sp; sp = sp-> next ) {
        is_label = ( sp-> flags & flags );
        if( is_label && ( sp-> value == dest )) {
            found = 1;
            break;
        }
    }

    if( !found ) {
        for( sp = system_symbols; sp; sp = sp-> next ) {
            is_label = ( sp-> flags & flags );
            if( is_label && ( sp-> value == dest )) {
                found = 1;
                break;
            }
        }
    }

    return found ? sp : 0L;

}

