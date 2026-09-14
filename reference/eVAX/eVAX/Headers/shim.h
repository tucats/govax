//
//  Module:     shim.h
//
//  Purpose:    Define prototypes and datastructures for software shim handlers.
//
//  History:    07/14/99    Initial creation
//


typedef LONGWORD (*CALLV)( LONGWORD, LONGWORD * );

struct RTL_ENTRY {
    CALLV       addr;
    char *      name;
};


long call_service( LONGWORD pc );
long declare_service( char * name, CALLV handler );

LONGWORD shim( void );
LONGWORD shim_declare( LONGWORD code, CALLV entry );
LONGWORD shim_init( void );
LONGWORD shim_dump( void );
LONGWORD lib_initialize( void );

int instring( char ch, char * str );
void set_errno( int code );
LONGWORD load_string( LONGWORD addr, char * buff, LONGWORD len );
char * load_dstring( LONGWORD addr );
int store_string( char * str, ULONGWORD addr, int len );
LONGWORD str_get( LONGWORD addr, int * retlen, char * buff );
LONGWORD str_put( LONGWORD addr, int len, char * buff );

LONGWORD lib_adawi( LONGWORD count, LONGWORD * argv );
LONGWORD str_upcase( LONGWORD count, LONGWORD * argv );
LONGWORD exe_input( LONGWORD count, LONGWORD * argv );

LONGWORD exe_open( LONGWORD argc, LONGWORD * argv );
LONGWORD exe_read( LONGWORD argc, LONGWORD * argv );
LONGWORD exe_write( LONGWORD argc, LONGWORD * argv );
LONGWORD exe_close( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_printf( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_fprintf( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_sprintf( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_format( char * buff, LONGWORD argc, LONGWORD * argv, LONGWORD pos );
LONGWORD decc_strcmp( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_strncmp( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_strncpy( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_atoi( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_gets( LONGWORD argc, LONGWORD *argv );

LONGWORD decc_isalnum( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_isalpha( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_iscntrl( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_isdigit( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_isgraph( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_islower( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_isprint( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_ispunct( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_isspace( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_isupper( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_isxdigit( LONGWORD argc, LONGWORD * argv );
LONGWORD decc_isascii( LONGWORD argc, LONGWORD * argv );

LONGWORD decc_init_memory( void );
LONGWORD decc_malloc( LONGWORD argc, LONGWORD *argv );
LONGWORD decc_free( LONGWORD argc, LONGWORD *argv );

LONGWORD lib_get_vm( LONGWORD argc, LONGWORD * argv );
LONGWORD lib_free_vm( LONGWORD argc, LONGWORD * argv );
LONGWORD lib_delete_vm_zone( LONGWORD argc, LONGWORD * argv );

LONGWORD decc_time( LONGWORD argc, LONGWORD *argv );

