
//
//  memmap.h
//
//  This contains definitions used by the structure mapping logic that maps
//  data structures in the VAX address space into native structures.  This is
//  used for image loading, page table management, etc.
//

#include <stddef.h>   /* offsetof(), used by STROFF below */

struct MAPLIST {
    struct MAPLIST *    next;
    char                name[ 32 ];
    LONGWORD            p_offset;
    LONGWORD            v_offset;
    short               size;
    short               kind;
};
    
struct MAP {
    struct MAP      * next;
    struct MAPLIST  *list;
    long	    highwater;
    char            name[ 32 ];
};

#define MAPNEXT -1

GLOBALINIT( struct MAP * maps,  0L )


LONGWORD map_init( char * name );
LONGWORD map_add( char * name, char * member, LONGWORD vax_offset, LONGWORD struct_offset, short size, short kind );
struct MAP * find_map( char * name );
LONGWORD map_dump( void );
LONGWORD map( char * name, void * structure, ULONGWORD vax_addr );
LONGWORD map_int( char * name, char * member, LONGWORD offset, short size );
LONGWORD map_chr( char * name, char * member, LONGWORD offset, short size );
LONGWORD map_offset( char * name, char * member );
LONGWORD store_field( LONGWORD base, char * str_name, 
     char * field_name, void * addr, short len );


//  STROFF computes a byte offset via the classic null-pointer offsetof
//  trick: every call site passes a NULL-initialized typed pointer (e.g.
//  "struct FAB * fab = 0L;" in rms.c), so this must NOT round-trip the
//  offset through a pointer-width-dependent cast -- that only "happened"
//  to be safe before because LONGWORD equaled native pointer width; now
//  that LONGWORD is a true 32-bit type (AUDIT.md 5.1/V6), use the
//  standard, well-defined offsetof() instead of casting pointers at all.
#define STROFF( structure, member ) \
    (( LONGWORD ) offsetof( __typeof__( *( structure ) ), member ))
