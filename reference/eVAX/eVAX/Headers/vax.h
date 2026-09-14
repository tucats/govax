//
//  Copyright � 1997,1998,1999 Forest Edge Software, All Rights Reserved
//
//  Program:    VAX Emulator, a "Virtual VAX" for Mac OS Computers
//
//  Author:     Tom Cole
//
//  Module:     vax.pch
//
//  Purpose:    This header file contains or links to the most commonly used
//              structure and symbol definitions for the emulator.  This is
//              included in every single compilation unit of the emulator.
//
//              Note that this is a ".pch" or pre-compiled header.  If changes
//              are made to this, it will be recompiled automatically for use
//              in subsequent builds.  Note that this is also the first compilation
//              unit in the "order of compilation" list in the project manager!
//
//
//  History:    02/23/98        Added uniform header
//              02/24/98        Rearranged elements, added blocking comments
//              09/24/98        Support floating parsing, change comment to ";"
//                                  and command separator to "\".
//              11/20/98        Added vm section to vax structure.
//              01/27/99        Added numerous small elements to the VAX
//                                  structure to support compiling CASE
//                                  pseudo-opcodes, and allowing forward
//                                  references in those expressions.
//              02/01/99        Rearranged struct VAX elements to create
//                                  natural alignments.
//              05/27/99        Fixed problem where PSL was not stored in
//                                  correct order on BIGENDIAN hosts.
//              06/01/99        Added pending interrupt queue
//              09/20/99        Added info on ROM and NVRAM address spaces;
//                                  extend privileged register array size.
//              11/08/99        Created BREAK_* symbols to define breakpoint 
//                                  types.
//              11/09/99        Added debug flag for system services.
//              11/29/99        Added debug flag for DCL debugging
//              12/09/99        Added share_prefix storage area for image loads
//              04/24/01        Updates for support of string symbols, Mac OS X.
//		06/26/01	Added flag to console for command line expansion.
//              07/08/01        Added several additional DBG_ flags for RTL use.

/*
 *  I N C L U D E S   F R O M   N A T I V E   E N V I R O N M E N T
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>




/*
 *  U T I L I T Y   A N D   D E B U G G I N G   M A C R O S
 */


//  Determine if this is a debug build or not.  Currently we do this
//  by seeing if the compiler is doing profiling, which is normally
//  turned on for DEBUG mode.

#if defined(__profile__)
#if __profile__ == 1 
#define DEBUG
#endif
#endif


//  Macro to set condition bits based on comparing two values, done after
//  most instructions set the destination.

#define SETCONDITIONBITS( v1, v2 ) \
    { vax.pslw.n = ((LONGWORD)(v1) < (LONGWORD)(v2));  \
     vax.pslw.z = ( v1 == v2 );  \
     vax.pslw.c = ((ULONGWORD)(v1) < (ULONGWORD)(v2)); }


//  Specify the behavior of the ASSERT macro, which is empty in an
//  optimized build, and defined in a debug build. 


#ifdef DEBUG
#define ASSERT( cond, msg ) \
    if(!(cond)) { \
        printf( "!! Assertion failure %s\n", msg ); \
        return VAX_ASSERT; \
    }
#else
#define ASSERT( cond, msg )

#endif


//  Read an operand.  Used in emulator entries.  Usage:
//
//      GET_OPERAND( loc, LONGWORD, 0, OP_RD );
//
//      Loads a longword from operand 0 into variable loc.  If
//      an error occurs (ACCVIO, etc.) then a RETURN is done.
//

#define GET_OPERAND( data, kind, pos, mode )                    \
    {                                                           \
        kind *p;                                                \
        p = ( kind * ) get_operand( opcode, pos, mode );    \
        if( p == 0L ) return VAX_FAULT;                         \
        data = *p;                                              \
    }

//  Macro for testing to see if overflow occurs in a simple expression.


#define OVERFLOW( a, b, expr ) \
( a < 0 && b < 0 && expr > 0 ) ? 1 : \
( ( b > 0 && a > 0 && expr < 0 ) ? 1 : 0 )


//  Macro for creating global definitions.  In only one module (typically
//  the driver, the symbol GLOBAL is defined.  This makes GLOBALINIT macros
//  create the defining instance.  Otherwise, GLOBALINIT creates external
//  references instead.

#ifdef GLOBAL
#define GLOBALINIT( data_structure, init_value ) \
data_structure = init_value;
#else 
#define GLOBALINIT( data_structure, init_value ) \
extern data_structure;
#endif


//  Sign extend a data item in place.  Similar to calling sext() but assumes
//  no side effects as a tradeoff for speed.

#define SEXT( data, size )                      \
    if( size == 1 && ( ULONGWORD ) data >= 128 )                \
        data = data - 256L;                     \
    else                                        \
    if( size == 2 && ( ULONGWORD ) data >= 32768 )          \
        data = data - 65536L

//  Determine if a physical memory address is in the ROM.

#define IN_ROM( addr ) \
     ( rom != 0L && ( addr >= vax.rom_base ) && ( addr <= vax.rom_end ))

/*
 *  Macro that takes a VAX physical address and returns the native address
 *  of that storage.  For most addresses, it just maps to the contiguous
 *  memory array.  If it's above that then call the more expensive get_address()
 *  routine which knows the maps of all the disjoint storage areas.
 */
 
#define PHYADDR( addr ) \
     (( (ULONGWORD) addr < (ULONGWORD) vax.memsize ) ? \
         &(vax.memory[addr]) : get_address( addr ))
    

         
//  Determine if the microkernel is valid.  This means that VMINIT and a "blessed"
//  microkernel are present in the machine.  Commands that damage or delete the
//  microkernel turn this off.  Some commands and pseudo-opcodes rely on this.


#define VMVALID  ( vax.console.vminit_valid )
#define MKVALID  ( VMVALID & vax.console.microkernel_valid )

//  Architecture definitions go here.
//

#include "arch.h"

//  This specifies that the console will dynamically bind pages to 
//  physical memory.  Note that this does NOT mean it pages, but it
//  does mean that pages are bound to VM PTE's as they are needed
//  when a VMINIT has been given.

#define DYNVM 1

/*
 *  D E F I N E D   C O N S T A N T S
 */

//  Assembler dialect codes.  Used in assembler decoding based on MACRO-32 or GAS

#define ASM_DIALECT_ANY     0
#define ASM_DIALECT_M32     1
#define ASM_DIALECT_GAS     2

//  Displacement size masks and names

#define K_NOFORWARD   -1        /* Flag means no forward reference allowed */

#define K_ADDR_SIZE   0x000F
#define K_ADDR_MODE   0x00F0

#define K_ADDR_BASE   0x0000
#define K_ADDR_B      0x0001
#define K_ADDR_W      0x0002
#define K_ADDR_L      0x0004
#define K_ADDR_Q      0x0008

#define K_DISP_BASE   0x0010

#define K_DISP_B      0x0011
#define K_DISP_W      0x0012
#define K_DISP_L      0x0014
#define K_DISP_Q      0x0018

#define K_BRANCH_BASE 0x0020

#define K_BRANCH_B    0x0021
#define K_BRANCH_W    0x0022
#define K_BRANCH_L    0x0024
#define K_BRANCH_Q    0x0028

#define K_CASE_BASE   0x0040
#define K_CASE_W      0x0042


//  Operand decoded locations, used to determine accessability for operands

#define OP_MEMORY       0x00            //  Operand reference memory location (MBZ!)
#define OP_REGISTER     0x01            //  Operand references VAX register
#define OP_TEMPORARY    0x03            //  Operand references temporary register 
                                        //      which means readonly and non-addressable
                                    
//  Machine exception names

#define EXC_UNUSED      0x00
#define EXC_CHECK       0x04        //  Machine check
#define EXC_KSNV        0x08        //  Kernel stack not valid
#define EXC_POWER       0x0C        //  Power failure
#define EXC_PRIV        0x10        //  Priveleged or reserved fault
#define EXC_CUSTOMER    0x14        //  Customer reserved instruction
#define EXC_RESOP       0x18        //  Reserved operand fault
#define EXC_RESADDR     0x1C        //  Reserved addressing mode fault
#define EXC_ACCVIO      0x20        //  Access control violation
#define EXC_TNV         0x24        //  Translation not valid
#define EXC_TP          0x28        //  Trace pending fault
#define EXC_BPT         0x2C        //  Breakpoint fault
#define EXC_COMPAT      0x30        //  Compatability mode fault (PDP-11)
#define EXC_ARITH       0x34        //  Arithmetic fault
#define EXC_CHMK        0x40        //  Change mode to kernel
#define EXC_CHME        0x44        //  Change mode to executive
#define EXC_CHMS        0x48        //  Change mode to supervisor
#define EXC_CHMU        0x4C        //  Change mode to user
#define EXC_SBI         0x50        //  SBI SILO COMPARE
#define EXC_CMRD        0x54        //  Corrected Memory read data
#define EXC_SBIALERT    0x58        //  SBI Alert
#define EXC_SBIFAULT    0x5C        //  SBI fault
#define EXC_MWT         0x60        //  Memory write timeout
#define EXC_SOFTWARE1   0x84        //  Software level 1
#define EXC_SOFTWARE2   0x88        //  Software level 2
#define EXC_SOFTWARE3   0x8C        //  Software level 3
#define EXC_SOFTWARE4   0x90        //  Software level 4
#define EXC_SOFTWARE5   0x94        //  Software level 5
#define EXC_SOFTWARE6   0x98        //  Software level 6
#define EXC_SOFTWARE7   0x9C        //  Software level 7
#define EXC_SOFTWARE8   0xA0        //  Software level 8
#define EXC_SOFTWARE9   0xA4        //  Software level 9
#define EXC_SOFTWARE10  0xA8        //  Software level 10
#define EXC_SOFTWARE11  0xAC        //  Software level 11
#define EXC_SOFTWARE12  0xB0        //  Software level 12
#define EXC_SOFTWARE13  0xB4        //  Software level 13
#define EXC_SOFTWARE14  0xB8        //  Software level 14
#define EXC_SOFTWARE15  0xBC        //  Software level 15
#define EXC_INTERVAL    0xC0        //  Interval timer
#define EXC_CONREAD     0xF8        //  Console read
#define EXC_CONWRITE    0xFC        //  Console write

//  Flags to describe misc console functions

#define CONSOLE_EXPAND  0x0001      //  Expand symbol substitutions in command strings
#define CONSOLE_VERBOSE 0x0002      //  Prattle on about things as we do them?

//  Flags to describe symbols

#define SYM_NONE      0          /* No flags set                        */
#define SYM_ENTRY     0x0001     /* This symbol is an entry point       */
#define SYM_FREFS     0x0002     /* This symbol has forward refs        */
#define SYM_LABEL     0x0004     /* This symbols is a label             */
#define SYM_LOCAL     0x0008     /* Symbol has local scope to entry     */
#define SYM_PERMANENT 0x0010     /* Symbol is permanent.                */
#define SYM_SYSTEM    0x0020     /* Symbol is a system symbol           */
#define SYM_STRING    0x0040     /* Symbol is a string, value is addr   */

//  Characteristics of the virtual VAX machine state

#define MAXREG      63          /* Number of registers, 0-15 architecturally */
                                /* defined, the rest are temporaries, etc.   */

#define MAXPRIVREG 128          /* Number of privileged registers */


//  Addressing mode values, used in handling opcode operands and
//  privileged register access modes.

#define OP_NL       0       // Operand not used
#define OP_RD       1       // Operand is readonly
#define OP_WR       2       // Operand is writeonly
#define OP_MD       3       // Operand is updated in place
#define OP_AD       4       // Operand is an address
#define OP_VA       5       // Operand is a variable bitfield address
#define OP_BR       6       // Operand is a displacement branch
#define OP_IM       7       // Operand is an implicit immediate operand

//  Access mode definitions for PSL, page tables, etc.

#define AM_KERNEL       0
#define AM_EXECUTIVE    1
#define AM_SUPERVISOR   2
#define AM_USER         3

//  Misc debug flags

#define DBG_SYMBOLS      0x00000001        // Symbol table debugging
#define DBG_DEBUG        0x00000002        // Invoke the native debugger if possible
#define DBG_MEMORY       0x00000004        // Debug memory
#define DBG_INTERRUPTS   0x00000008
#define DBG_EXCEPTIONS   0x00000010
#define DBG_VM           0x00000020        // Debug virtual memory mapping
#define DBG_TB           0x00000040        // Debug translation caching
#define DBG_FUNCTIONS    0x00000080
#define DBG_REGISTERS    0x00000100
#define DBG_IMAGES       0x00000200        // Display image info on RUN command?
#define DBG_USERHALT     0x00000400        // Will HALT halt or RESOP when not in kernel mode?
#define DBG_KBD          0x00000800        // Debug console keyboard input
#define DBG_SERVICES     0x00001000        // Debug system services
#define DBG_DCL          0x00002000        // Debug DCL parsing
#define DBG_UNIMP        0x00004000        // Debug unimplemented instructions?
#define DBG_CHM          0x00008000        // Debug change mode operations
#define DBG_EXPAND	 0x00010000        // Command line expansion enabled?
#define DBG_LOGICALS     0x00020000        // Debug logical name operations?
#define DBG_DEVICES      0x00040000        // Debug device operations?
#define DBG_LIBINIT      0x00080000        // Run LIB$INITIALIZE by default?
#define DBG_PROCESS      0x00100000        // Debug process services?
#define DBG_FULLDISASM   0x00200000	   // Disasm includes operands addresses?
#define DBG_RMS          0x00400000        // Debug RMS calls?
#define DBG_P1           0x01000000        // Misc debugging flag 1
#define DBG_P2           0x02000000        // Misc debugging flag 1
#define DBG_P3           0x04000000        // Misc debugging flag 1
#define DBG_P4           0x08000000        // Misc debugging flag 1



//  Define the register mnemonics.  These are used as "vax.NAME" where "vax" is
//  the handle to the virtual machine and "NAME" is the name of the register.

#define KSP             preg[ 0 ]
#define ESP             preg[ 1 ]
#define SSP             preg[ 2 ]
#define USP             preg[ 3 ]
#define ISP             preg[ 4 ]
#define P0BR            preg[ 8 ]
#define P0LR            preg[ 9 ]
#define P1BR            preg[ 10 ]
#define P1LR            preg[ 11 ]
#define SBR             preg[ 12 ]
#define SLR             preg[ 13 ]
#define PCBB            preg[ 16 ]
#define SCBB            preg[ 17 ]
#define IPL             preg[ 18 ]
#define ASTLVL          preg[ 19 ]
#define SIRR            preg[ 20 ]
#define SISR            preg[ 21 ]
#define ICCS            preg[ 24 ]
#define NICR            preg[ 25 ]
#define ICR             preg[ 26 ]
#define TODR            preg[ 27 ]
#define RXCS            preg[ 32 ]
#define RXDB            preg[ 33 ]
#define TXCS            preg[ 34 ]
#define TXDB            preg[ 35 ]
#define TBDR            preg[ 36 ]  /* 11/730 specific */
#define MAPEN           preg[ 56 ]
#define TBIA            preg[ 57 ]
#define TBIS            preg[ 58 ]
#define PMR             preg[ 61 ]
#define SID             preg[ 62 ]
#define TBCHK           preg[ 63 ]


//  Definitions for accessing the VAX register file

#define VAXREG( n )     reg[ n ]


//  Mnemonics for the first 6 temporary registers, used for
//  character instructions.

#define T0  VAXREG(16)
#define T1  VAXREG(17)
#define T2  VAXREG(18)
#define T3  VAXREG(19)
#define T4  VAXREG(20)
#define T5  VAXREG(21)


//  Mnemonics for the standard register file

#define R15 VAXREG(15)
#define R14 VAXREG(14)
#define R13 VAXREG(13)
#define R12 VAXREG(12)
#define R11 VAXREG(11)
#define R10 VAXREG(10)
#define R9  VAXREG(9)
#define R8  VAXREG(8)
#define R7  VAXREG(7)
#define R6  VAXREG(6)
#define R5  VAXREG(5)
#define R4  VAXREG(4)
#define R3  VAXREG(3)
#define R2  VAXREG(2)
#define R1  VAXREG(1)
#define R0  VAXREG(0)

//  Overloaded mnemonics for the pre-defined registers

#define PC  VAXREG(15)
#define SP  VAXREG(14)
#define FP  VAXREG(13)
#define AP  VAXREG(12)

//  Types of breakpoints

#define BREAK_ADDRESS          0          /*  Break on an address */
#define BREAK_FAULT            1          /*  Break on exception/interrupt */
#define BREAK_INSTRUCTION      2          /*  Break on a specific opcode */
#define BREAK_TYPE          0x0F          /*  Mask for finding type code */
#define BREAK_TEMPORARY     0x80          /*  Temporary breakpoint */
#define BREAK_STEP          0x40          /*  STEP-based breakpoint */

//  Miscellaneous constants

#define CONSOLE_COMMENT         ';'         /*  Default comment delimiter   */
#define CONSOLE_SEPARATOR       0xFFUL      /*  No command separator   */

//  Types of STEP modes

#define STEP_NONE           0
#define STEP_INSTRUCTION    1
#define STEP_OVER           2
#define STEP_RETURN         3


/*
 *  S T R U C T U R E   D E F I N I T I O N S
 */

//  Forward branch resolutions.  This list is empty for symbols that are
//  resolved, and non-empty for forward references during an ASM block.

struct FSYMBOL { 
    struct FSYMBOL *    next;
    LONGWORD            location;
    LONGWORD            displacement;   /* 8, 16, or 32 */
};


//  This structure defines the symbol table used by the console

struct SYMBOL {
    struct SYMBOL *     next;       /* Linked list of all symbols */
    struct FSYMBOL *    forward;    /* Linked list of forward resolutions */
    LONGWORD            flags;
    LONGWORD            value;
    char *              svalue;     /* String value when SYM_STRING is set --
                                        native host pointer, kept separate from
                                        `value` since it can't fit in a 32-bit
                                        LONGWORD.  See AUDIT.md N1. */
    char                name[ 1 ];
};

//  This structure is used to create a list of break points

struct BREAKSTR {
    struct BREAKSTR *   next;
    ULONGWORD           pc;         /* PC of place to stop          */
    LONGWORD            kind;       /* 0 = instruction, 1 = fault   */
    LONGWORD            after;      /* Number of times to skip      */
};


//  This is the structure into which an instruction is decoded.  After decode
//  phase, the address list contains the real memory locations of each of the
//  operands, whether in the register file or in the physical memory array.

struct OPCODE {
    unsigned char   extended;           // Extended opcode value (normally zero)
    unsigned char   function;           // Opcode value
    short           index;              // Index into instruction array (normally same as function)
    LONGWORD        access[ 6 ];        // Operand access mode array
    unsigned char   count;              // Count of operands
    unsigned char * address[ 6 ];       // Resolved addresses of each operand
    LONGWORD        VAXaddr[ 6 ];       // VAX memory address for each operand
    LONGWORD        size[ 6 ];          // Natural data size for each operand
    LONGWORD        is_register[ 6 ];   // Flag indicating if register (vs. memory) operand
    LONGWORD        regnum[ 6 ];        // If it's a register, which one?
    char            name[ 128 ];        // Name of the instruction for disassembly/etc.
};


//  This is the register mask data pushed on a stack call frame.  Note
//  that the fields are based on the endian-ness of the machine.

union MASKREG {
    
    LONGWORD        longword;
    struct {

#if 1 /* BIGENDIAN */  /* This isn't working right on Alpha... */

        int     spa: 2;
        unsigned int calltype: 1;
        int     mbz: 1;
        int     mask: 12;
        int     psw: 16;
#else
        int     psw: 16;
        int     mask: 12;
        int     mbz: 1;
        unsigned int calltype: 1;
        int     spa: 2;
#endif

    } bits;
};



//  The processor status longword bit fields.

struct PSL_BITS {
#if BIGENDIAN
    unsigned int        cm:1;       //  Compatibility mode (PDP-11)
    unsigned int        tp:1;       //  Trace pending
    unsigned int        mbz3:2;     
    unsigned int        fpd:1;      //  First part done
    unsigned int        is:1;       //  Interrupt stack
    unsigned int        cur_mod:2;  //  Current access mode
    unsigned int        prv_mod:2;  //  Previous access mode
    unsigned int        mbz2:1;
    unsigned int        ipl:5;      //  Interrupt priority level
    unsigned int        mbz1:8;
    unsigned int        dv:1;       //  Decimal overflow enable
    unsigned int        fu:1;       //  Floating underflow enable
    unsigned int        iv:1;       //  Integer overflow enable
    unsigned int        t:1;        //  Trace enable
    unsigned int        n:1;        //  Negative result
    unsigned int        z:1;        //  Zero result
    unsigned int        v:1;        //  Overflow result
    unsigned int        c:1;        //  Carry bit result
#else
    unsigned int        c:1;        //  Carry bit result
    unsigned int        v:1;        //  Overflow result
    unsigned int        z:1;        //  Zero result
    unsigned int        n:1;        //  Negative result
    unsigned int        t:1;        //  Trace enable
    unsigned int        iv:1;       //  Integer overflow enable
    unsigned int        fu:1;       //  Floating underflow enable
    unsigned int        dv:1;       //  Decimal overflow enable
    unsigned int        mbz1:8;
    unsigned int        ipl:5;      //  Interrupt priority level
    unsigned int        mbz2:1;
    unsigned int        prv_mod:2;  //  Previous access mode
    unsigned int        cur_mod:2;  //  Current access mode
    unsigned int        is:1;       //  Interrupt stack
    unsigned int        fpd:1;      //  First part done
    unsigned int        mbz3:2;     
    unsigned int        tp:1;       //  Trace pending
    unsigned int        cm:1;       //  Compatibility mode (PDP-11)
#endif
};

//  This is information used to timeslice; i.e. it helps the emulator know how
//  often to let other things run on the Mac so the emulator doesn't lock up
//  the system.  The cost of this, of course, is speed.  The quantum values
//  are expressed in VAX instructions; i.e. how many instructions do we let
//  execute before yielding?  We also yield on any console I/O operation.

struct QUANTUM {
    LONGWORD    initial;        //  Initial setting, zero means no yeild
    LONGWORD    current;        //  The current setting.
};

//  The processor status longword bit fields re-expressed as SHORT values for speed

struct PSL_W {
    ULONGWORD      cm;         //  Compatibility mode (PDP-11)
    ULONGWORD      tp;         //  Trace pending
    ULONGWORD      fpd;        //  First part done
    ULONGWORD      is;         //  Interrupt stack
    ULONGWORD      cur_mod;    //  Current access mode
    ULONGWORD      prv_mod;    //  Previous access mode
    ULONGWORD      ipl;        //  Interrupt priority level
    ULONGWORD      dv;         //  Decimal overflow enable
    ULONGWORD      fu;         //  Floating underflow enable
    ULONGWORD      iv;         //  Integer overflow enable
    ULONGWORD      t;          //  Trace enable
    ULONGWORD      n;          //  Negative result
    ULONGWORD      z;          //  Zero result
    ULONGWORD      v;          //  Overflow result
    ULONGWORD      c;          //  Carry bit result
};

//  Virtual memory accounting structure.  We have one of these for each region. 
//  They are set up by the VMINIT command, and are stored in the virtual machine
//  so a SHOW VM command can display them.

struct VMREGION {
    char            name[ 4 ];
    LONGWORD            pte_count, size;
    ULONGWORD   v_start, v_end;
    ULONGWORD   p_start, p_end;
};

//  Definition of a pending interrupt

struct INTERRUPT {
    struct INTERRUPT *  next;       //  Pointer to next entry in list
    LONGWORD                code;       //  Interrupt code number
    LONGWORD                ipl;        //  IPL to set process for interrupt
    LONGWORD                age;        //  Age in quantum of interrupt
    LONGWORD                quantum;    //  Requested quantum delay when posted
};

//  Define what we know bout a fault.

struct FAULT {
        LONGWORD    code;           //  Exception code
        ULONGWORD   pc;             //  Faulting PC
        ULONGWORD   psl;            //  PSL at the time
        ULONGWORD   r0;             //  R0 at the time
        ULONGWORD   r1;             //  R1 at the time
        ULONGWORD   signal_args[7]; //  count, and up to 6 parameters
        ULONGWORD   history_id;     //  sequence number
        struct FAULT *next;	    //  when a linked list...
};

//  The virtual machine state.  This is passed as the first parameter
//  to nearly every routine, allowing the virtual VAX to be stateless, 
//  and ultimately rednerable as a self-contained runtime library.

struct VAX {
    
    ULONGWORD reg[ MAXREG + 1];         // Registers, i.e. R0 is r[0], plus 48 temporaries
    ULONGWORD preg[ MAXPRIVREG + 1 ];   // Privileged registers 0-63
    
    LONGWORD            debug;          //  Misc debug flags go here.    


    struct PSL_W pslw;              //  Copy of psl bits as longs for speedy access.
    
    ULONGWORD   instruction_PC; //  The PC of the currently executing instruction
    LONGWORD    exception;              //  Last exception processed
    LONGWORD    timebase;               //  Used with TickCount to create TODR values.
    LONGWORD    treg;                   //  Index into reusable temporary registers

    LONGWORD    halted;                 //  Flagged when HALT occurs
    unsigned char * memory;         //  Address of physical memory, normally at first_word
    ULONGWORD    memsize;                //  Size of physical memory
    ULONGWORD   rom_base;               //  Base addr of ROM memory
    ULONGWORD   rom_end;                //  Last addr of ROM memory
    ULONGWORD   nvram_base;             //  Base address of non-volatile RAM
    ULONGWORD   nvram_end;              //  Last addres of non-volatile RAM
    
    struct FAULT        fault;
    LONGWORD            fault_pending;
    
    LONGWORD            clock_running;      //  Copy of ICCS<0>
    ULONGWORD   clock;              //  Copy of ICR
        
    LONGWORD            interrupt_pending;  //  Interrupt that needs servicing?
    LONGWORD            interrupt_ipl;      //  IPL of interrupt
    struct INTERRUPT * iqueue;          //  List of pending interrupts
    
    struct QUANTUM  quantum;            //  Info on interrupt handling intervals
    struct QUANTUM  uiquantum;          //  Info on UI time slicing 
    
    struct CONSOLE {
    
        LONGWORD        radix;          //  Default radix for constants
        LONGWORD        disasm;         //  Disassembly flag on instruction execution
        LONGWORD        running;        //  Accepting commands?
        LONGWORD        verify;         //  Display .com files as executed?
        LONGWORD        assembler_mode; //  Active ASM mode?
        LONGWORD        singlestep;     //  Flagged when we single step
        LONGWORD        tbcache;        //  Do we do VM TB caching?
        LONGWORD        vmtrace;        //  Trace VM operations?
        LONGWORD        CALL_active;    //  Are we running under CALL console cmd?
        LONGWORD        CALL_PC;        //  Saved PC from CALL command
        LONGWORD        CALL_FP;        //  Saved FP from CALL command
        struct OPCODE   opcode;         //  Used for DISASM command
        struct SYMBOL * symbols;        //  Symbol table for console
        struct SYMBOL * last_symbol;    //  last symbol found or set
        ULONGWORD       deposit;        //  Address of next deposit/assemble
        char          * scale;             //  Array of scale factors for each operand
        struct BREAKSTR * breakpoint_list;  // List of active breakpoints.
        LONGWORD        flags;          //  Misc console flags
        ULONGWORD       min_deposit;  //  Lowest address data stored in
        ULONGWORD       max_deposit;  //  Highest address data stored in
        ULONGWORD       instruction_count; // Instruction count for TIME command
        LONGWORD        image_load;     //  Next available address for image loading in P0
        char            comment;        //  Comment character
        char            separator;      //  Command separator character
        char          * prompt;         //  The command prompt string.
        LONGWORD        p0_deposit; 
        LONGWORD        s0_deposit;
        char            stepmode;       //  STEP/OVER or STEP/INTO by default 
        char            microkernel_valid;  //  Indicates that the microkernel has built
                                        //  successfully.
        char            vminit_valid;   // Indicates that the required support for the
                                        // VMINIT memory layout is in place.
        long		pid;		// Fake PID for runtime support
        long		uic;		// Fake UIC for runtime support
        char            share_prefix[80];
    } console;

    struct ASM {
        LONGWORD        index;          //  Index into the instruction array we're working on.
        LONGWORD        opcount;        //  Which operand are we working on now?
        LONGWORD        scale;          //  Scale of item we are working on.
        LONGWORD        displacement;   //  For symbols, what displacement size are we using?
        ULONGWORD location;             //  For symbols, where will be put the data?
        LONGWORD        was_forward;    //  Last symbol was forward reference?
        LONGWORD        flags;          //  Misc assembler flags
        LONGWORD        dialect;        //  Dialect code for assembler syntax
        LONGWORD        case_base;      //  Start of .CASE blocks else zero
        char        cur_entry[64];      //  Current .entry we're using
        char        comment[ 64 ];      //  Buffer for disassembly comment, if any
    } assembler;


    struct VMREGION     region[ 3 ];    //  Info about each VM region.
    LONGWORD            vm_initialized; //  Have we initialized VM?
    
    double              last_float;     //  Last floating value parsed
    LONGWORD            in_debugger;    //  Is the host debugger active from SET DEBUG?
    struct INSTRUCTION * inst_table;    //  Instruction table.
    union PSL {
        struct PSL_BITS bit;            // references are vax.psl.bit.v
        ULONGWORD   reg;                // or entire reg  vax.psl.reg
    } psl;

    LONGWORD            first_word;             //  physical memory starts here
};


/*
 *  Global storage here.  This is declared using GLOBALINIT which creates a defining
 *  instance when GLOBAL is defined, else just an extern.  The GLOBAL instance is
 *  defined in vax.c
 */
 
GLOBALINIT( struct VAX vax, { 0L } )
GLOBALINIT( long vax_init, 0 )

GLOBALINIT( struct SYMBOL * system_symbols, 0L )


/*
 *  A D D I T I O N A L   I N C L U D E S
 */

//  Return code values

#include "vaxrc.h"

//  Routine prototypes


#include "vaxproto.h"
