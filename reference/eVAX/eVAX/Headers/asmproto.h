//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS Computers
//
//  Author:     Tom Cole
//
//  Module:     asmproto.h
//
//  Purpose:    Prototypes, defines, etc. for the assembler and it's support routines.
//
//  History:    07/28/97    New header format standardization
//              01/05/99    Added ASM_BRANCHDEST flag defintiion
//


//  D E F I N E S

#define ASM_ADDRPROMPT      0x00000001      /*  Show address as part of ASM  */
                                            /*      prompt?                  */
#define ASM_WARNFORWARD     0x00000002      /*  Warn if there are unresolved */
                                            /*      symbols after ASM END    */
#define ASM_PARSING_IDX     0x00000004      /*  Are we parsing an indexed    */
                                            /*      mode expression? Used to */
                                            /*      control recursion        */
#define ASM_BRANCHDEST      0x00000008      /*  Do we display destination    */
                                            /*      of branch in disasm?     */
#define ASM_EXPRESSION      0x00000010      /*  Are we parsing an expression */
                                            /*      versus scalar value?     */
#define ASM_SYMBOLS         0x00000020      /*  Display symbols in disasm?   */

#define ASM_RESTMP          0x00000040      /*  Require that temp symbols    */
                                            /*      resolve in scope         */

#define ASM_SYMDEBUG        0x00000080      /*  Symbol table debug?          */

#define ASM_SCOPENAMES      0x00000100      /*  Do we use scope names?       */

#define ASM_ENTRY           0x00000200      /*  Assembly includes __ENTRY    */

#define ASM_UNIQUE          0x00000400      /*  Must define unique symbol    */

#define ASM_REGISTER_EXPR   0x00000800      /*  Allow @register in expressions */


//  P R O T O T Y P E S

LONGWORD asm_operand( char ** Buffptr );
LONGWORD asm_reg( char ch, char ** P, LONGWORD *Value );
LONGWORD asm_hex( char ** P, ULONGWORD *Value );
LONGWORD asm_dec( char ** P, LONGWORD *Value );
LONGWORD asm_opcode( char ** Buffptr, LONGWORD * count );
LONGWORD asm_pseudo( char * Buffer );

LONGWORD asm_value( char ** P, ULONGWORD * V, LONGWORD mode );
LONGWORD asm_mask( char ** P, LONGWORD * V );
LONGWORD asm_float( char ** P );
LONGWORD asm_expr( char ** P, LONGWORD * V );

LONGWORD get_symbol(  char ** symbol, LONGWORD * value );
LONGWORD set_symbol(  char ** symbol, LONGWORD value, int flags );
LONGWORD get_symbol_direct( char * Symbol, LONGWORD * value );
LONGWORD clear_system_symbols( void );
LONGWORD clear_symbol(  char * symbol );
LONGWORD clear_all_symbols( void );
LONGWORD clear_temp_symbols( void );

LONGWORD dump_symbols(  int flag );
LONGWORD dump_forward_references( struct SYMBOL * p );
LONGWORD check_unresolved_symbols( int flag );
LONGWORD dump_system_symbols( void );
LONGWORD scope_symbols( void );
LONGWORD set_symbol_direct( char * symbol, LONGWORD value, int flags  );
LONGWORD set_symbol_string_direct( char * symbol, char * strvalue, int flags  );

LONGWORD asm_symbol(  char ** P, LONGWORD *Value );
LONGWORD asm_label( char ** P );

LONGWORD asm_function( char * fname, char ** P, LONGWORD * value );
LONGWORD assemble_direct( char* buffer );

