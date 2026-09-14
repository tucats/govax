
/****************************************************************************\
 ****************************************************************************
 **                                                                        **
 **                          D C L R T L . C                               **
 **                                                                        **
 ****************************************************************************
\****************************************************************************/


/*
 *      Module:         Command Line Parser
 *
 *      Author:         Tom Cole, Aug93
 *
 *      Description:    This module supports definition of a grammar and 
 *                      parsing statements in that grammar.  The grammar
 *                      language and the command line language are based
 *                      on Digital Equipment Corporation's DCL (Digital
 *                      Command Language) used in VMS systems.
 *
 *                      This module differs from the VMS CLI runtime 
 *                      routines in the following major ways:
 *
 *                      o  DCLRTL does not "pre-compile" the grammar, the
 *                         grammar definition is read in at runtime and
 *                         converted to data structures that are used to
 *                         manage the command line parsing.
 *
 *                      o  DCLRTL can be used to "query" for specific
 *                         qualifiers and parameters just as VMS CLI is.
 *                         However, DCLRTL can also return just the 
 *                         elements parsed to the caller via a call-back
 *                         mechanism, which means the caller doesn't have
 *                         to query every qualifier and parameter to learn
 *                         what was found during the parse.
 *
 *                      o  Query and callback operations use numeric 
 *                         identifiers instead of strings containing the
 *                         parse item.  These identifiers are defined in
 *                         the grammar language definitions.  These can
 *                         be any arbitrary longword, but are often used
 *                         as table indexes, etc. to make the callback
 *                         mechanism store qualifier and parameter values
 *                         efficiently for use by the calling program.
 *
 *                      o  DCLRTL permits ambigious keyword and qualifier
 *                         names to any character length; the CLI routines
 *                         require unambigious keyword names in the first
 *                         four characters.
 *
 *                      o  DCLRTL permits aliasing of verbs and qualifiers,
 *                         so a qualifier can have alternate spellings that
 *                         are defined in the grammar.  Defaults and other
 *                         attributes can be unique to each spelling, but
 *                         the qualifier returned is the "root" qualifier
 *                         for the alias.
 *
 *                      o  There are numerous other differences in the way
 *                         the grammar language works and how parsing is
 *                         done.  There are also some limitations, i.e the
 *                         CLI routines are capable of more complex parsing
 *                         than the DCLRTL routines in the current version.
 *
 *      History:
 *                      xxAUG93         Tom Cole
 *                      Initial implementation for VMS on AXP.  Ported to
 *                      VAX as well.
 *
 *                      08SEP93         Tom Cole
 *                      Restructure module to make it clearer what parts
 *                      do what.  Added header block comments to sections
 *                      to define major areas.  Added structured header
 *                      comment to top of file.
 *
 *                      11NOV99         Tom Cole
 *                      I want to reuse this code to improve the quality of
 *                      the command line for eVAX.  Ported initially to the
 *                      Mac.  Ports to Unix and Windows should follow shortly.
 *
 *                      16NOV99         Tom Cole
 *                      Fixed bug in detection of ambiguous abbreviation of
 *                      keywords so SHOW <register> would work right.
 *
 *                      20MAY00         Tom Cole
 *                      Added support for /ENTRY name on commands, that can
 *                      map to a named procedure entry point.  Added the
 *                      DCLgetentry() call that fetches this for the current
 *                      verb or syntax.
 *
 */



/****************************************************************************\
 *                                                                          *
 *                          PUBLIC DEFINITIONS                              *
 *                                                                          *
\****************************************************************************/

#define DCLGLOBALS

#include "vax.h"
#include "dclprivate.h"
#include "dclrtl.h"


/****************************************************************************\
 *                                                                          *
 *                          PRIVATE DEFINITIONS                             *
 *                                                                          *
\****************************************************************************/

/*
 *      Host-specific and portable include files.
 */


#include <stdlib.h>
#include <stdio.h>
#include <ctype.h>
#include <string.h>


/*
 *      Symbolic data used by DCL.
 */

#define DCL_MINDEF      (-(0x7fffffff)) /* Smallest numeric value       */
#define DCL_MAXDEF      (0x7fffffff)    /* Largest numeric value        */

#define DCLFLAG( n ) ( 1 << n )

#define DCL_REQ         DCLFLAG( 0 )    /* Item is required             */
#define DCL_LIST        DCLFLAG( 1 )    /* Item can be a list           */
#define DCL_NONEGATE    DCLFLAG( 2 )    /* Item cannot be negated       */
#define DCL_DEFAULT     DCLFLAG( 3 )    /* Item has a default           */
#define DCL_TMPDEFAULT  DCLFLAG( 4 )    /* Item has alias'd default     */
#define DCL_HASVALUE    DCLFLAG( 5 )    /* A value given for qualifier  */
#define DCL_ALLOWSUFFIX DCLFLAG( 6 )    /* Multiplier suffixes allowed  */
#define DCL_LOCAL       DCLFLAG( 7 )    /* Item local to one parameter  */

#define DCL_DEFAULTED           1
#define DCL_DEFWITHVALUE        2

#define DCL_PRESENT     1               /* Item was specified/defaulted */
#define DCL_ABSENT      0               /* Item was not specified       */
#define DCL_NEGATED     -1              /* Item explicitly negated      */

#ifndef DCL_UNDEFINED
#define DCL_UNDEFINED   DCL_MINDEF         /* Undefined item ID         */
#define DCL_NO_ID       (DCL_MINDEF + 1L ) /* Indicates item created by us */
#endif

#define DCL_ANY         '*'
#define DCL_NAME        'N'
#define DCL_INTEGER     'I'
#define DCL_STRING      'S'
#define DCL_KEYWORD     'K'
#define DCL_FILENAME    'F'
#define DCL_DATE        'D'
#define DCL_RESTOFLINE  'R'

/*
 *      Symbolic data used by FSM.
 */

#define FSM_MAXTOKEN 32767

#define FSM_NULL 0
#define FSM_LITERAL 1
#define FSM_STRING 2
#define FSM_ANYCHAR 3
#define FSM_EXCEPTCHAR 4
#define FSM_QUOTE 5
#define FSM_EOS 6
#define FSM_FILENAME 7
#define FSM_DATE 8
#define FSM_RESTOFLINE 9

#define FSM_NODEBUG 0
#define FSM_DEBUG 1

#define FSM_NOTFOUND 0
#define FSM_FOUND 1

#define FSM_NOCONCAT 0
#define FSM_CONCAT 1

#define FSM_NOSKIPBLANKS 0
#define FSM_SKIPBLANKS 1

#define FSM_UNDEFINED -1
#define FSM_ERROR 44
#define FSM_NOERROR 1
#define FSM_NEEDMORE 2
#define FSM_ACTION 3
#define FSM_STOP DCL_DONE
#define FSM_NOTRANS 5


#define DCLSIGN( x ) (( x < 0 ) ? -1 : 1 )


/****************************************************************************\
 *                                                                          *
 *                        DCL GRAMMAR IDENTIFIERS                           *
 *                                                                          *
\****************************************************************************/


#define DCL_QUALIFIER_ENTRY                                      16
#define DCL_KEYWORD_AND                                          42
#define DCL_VERB_DISALLOW                                         9
#define DCL_PARAMETER_AND                                        99
#define DCL_PARAMETER_NAME2                                       8
#define DCL_VERB_END                                              8
#define DCL_VERB_SYNTAX                                           7
#define DCL_QUALIFIER_IDENT                                       1
#define DCL_VERB_TYPE                                             6
#define DCL_VERB_QUALIFIER                                        5
#define DCL_QUALIFIER_LOCAL                                      15
#define DCL_QUALIFIER_MULTIPLIER                                 14
#define DCL_QUALIFIER_IMPLY                                      12
#define DCL_QUALIFIER_DEFAULT                                    11
#define DCL_QUALIFIER_MAXIMUM                                    10
#define DCL_QUALIFIER_MINIMUM                                     9
#define DCL_QUALIFIER_ALIAS_FOR                                   8
#define DCL_QUALIFIER_SYNTAX                                      7
#define DCL_QUALIFIER_TYPE                                        5
#define DCL_QUALIFIER_LIST                                        4
#define DCL_QUALIFIER_NONEGATABLE                                 3
#define DCL_QUALIFIER_REQUIRED                                    2
#define DCL_QUALIFIER_IDENT                                       1
#define DCL_VERB_KEYWORD                                          4
#define DCL_QUALIFIER_SYNTAX                                      7
#define DCL_QUALIFIER_NONEGATABLE                                 3
#define DCL_QUALIFIER_IDENT                                       1
#define DCL_VERB_PARAMETER                                        3
#define DCL_PARAMETER_NAME                                        1
#define DCL_QUALIFIER_MULTIPLIER                                 14
#define DCL_QUALIFIER_PROMPT                                     13
#define DCL_QUALIFIER_TYPE                                        5
#define DCL_QUALIFIER_REQUIRED                                    2
#define DCL_QUALIFIER_IDENT                                       1
#define DCL_VERB_VERB                                             2
#define DCL_QUALIFIER_ALIAS_FOR                                   8
#define DCL_QUALIFIER_IDENT                                       1
#define DCL_VERB_GRAMMAR                                          1


/****************************************************************************\
 *                                                                          *
 *                            FSM STRUCTURES                                *
 *                                                                          *
\****************************************************************************/


/*
 *      Table of states.  There is a node in this list for each state
 *      that can exist.  It links transition lists to each state.
 */

struct STATE {
        struct STATE *          next;
        struct TRANSITION *     transitions;
        long                    error;
        char                    name[ 1 ];
};

/*
 *      This is a generic argument list entry.  The actual value is
 *      sized based on the data stored for each entry.
 */

struct ARG {
        struct ARG *            next;
        char                    value[ 1 ];
};

/*
 *      Transition lists.  There are zero or more transitions for 
 *      each state.  A state transition specifies the conditions
 *      under which the FSM switches to another state.  There is
 *      an optional action that is performed before switching 
 *      state.  The action is specified by the list of arguments,
 *      which if non-zero are returned to the caller as an action.
 */

struct TRANSITION {
        struct TRANSITION *     next;
        long                    kind;
        union {
                struct STRING * string;
                char *          literal;
                char            onechar;
        } token;
        struct STATE *          state;
        struct ARG *            args;
};

/*
 *      A string is a list of characters that match a specific
 *      set of allowed characters, by name.  For example, "identifiers"
 *      might be a valid string, consisting of letters and numbers.
 *      This list keeps up with all named strings, which can be
 *      referenced in state transitions.
 */

struct STRING {
        struct STRING *         next;
        char *                  initial;
        char *                  secondary;
        char *                  delimiters;
        char                    name[ 1 ];
};

/*
 *      Errors are coded strings.  A state may specify an error code
 *      to be signalled to the caller when no transitions occur for a
 *      given state.  Because multiple states may signal the same error,
 *      these are indicated by numerical code, and translated using this
 *      list.
 */

struct ERROR {
        struct ERROR *          next;
        long                    code;
        char                    text[ 1 ];
};
/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                            DCL STRUCTURES                                *
 *                                                                          *
\****************************************************************************/


/*
 *      This is a list of valid keyword values for a given parameter
 *      or item value.
 */

struct DCL_KEYWORDS {
        struct DCL_KEYWORDS     *next;
        long                    id;
        int                     flags;
        struct DCL_VERB         *syntax;
        char                    name[ 1 ];
};

struct DCL_TYPE {
        struct DCL_TYPE         *next;
        struct DCL_KEYWORDS     *keylist;
        char                    name[ 1 ];
};

/*
 *      This is the list of qualifier values found for a particular
 *      verb.
 */

struct DCL_VALUE {
        struct DCL_VALUE        *next;
        long                    id;
        long                    i_value;
        int                     kind;
        char                    state;
        char                    c_value[ 1 ];
};


/*
 *      This is the list of qualifiers permitted by a verb.
 */

struct DCL_QUAL {
        struct DCL_QUAL         *next;
        long                    id;
        struct DCL_KEYWORDS     *keywords;
        struct DCL_VALUE        *value;
        struct DCL_VERB         *syntax;
        struct DCL_QUAL         *alias;
        struct DCL_QUAL         *imply;
        struct DCL_VALUE        *defvalue;
        struct DCL_PARM         *local;
        long                    min;
        long                    max;
        int                     kind;
        int                     flags;
        char                    state;
        char                    imply_state;
        char                    name[ 1 ];
};

/*
 *      This is the list of parameters allowed by a verb.
 */

struct DCL_PARM {
        struct DCL_PARM         *next;
        long                    id;
        int                     kind;
        int                     flags;
        char                    state;
        struct DCL_KEYWORDS     *keywords;
        struct DCL_VALUE        *defvalue;
        long                    i_value;
        char                    *c_value;
        char                    *prompt;
        char                    name[ 1 ];
};

/*
 *      This is the list of verb names allowed.
 */

struct DCL_VERB {
        struct DCL_VERB         *next;
        long                    id;
        long                    (*routine)(long);
        struct DCL_QUAL         *qualifier;
        struct DCL_PARM         *parameter;
        struct DCL_PARM         *nextparm;
        struct DCL_PARM         *lastparm;
        struct DCL_VERB         *alias;
        struct DISALLOW         *disallow;
        int                     flags;
        char                    state;
        char                    *entry;
        char                    name[ 1 ];
};

struct GRAMMAR {
        struct GRAMMAR          * next;
        struct DCL_VERB         * verbs;
        struct DCL_TYPE         * typenames;
        struct DCL_VERB         * current_verb;
        struct DCL_PARM         * current_parameter;
        struct DCL_QUAL         * current_qualifier;
        struct DCL_TYPE         * current_typenames;
        struct DCL_TYPE         * the_types;
        struct DISALLOW         * disallow;
        long                      valid;
        char                      name[ 1 ];
};

struct DISALLOW {
        struct DISALLOW         * next;
        struct DCL_QUAL         * qual1;
        struct DCL_QUAL         * qual2;
        char                    state1;
        char                    state2;
};

/*<PAGE>*/


/****************************************************************************\
 *                                                                          *
 *                           STORAGE INSTANCES                              *
 *                                                                          *
\****************************************************************************/

/*
 *      These are memory management counters used to track how DCLRTL
 *      is using memory for a given grammar.
 */

long FSM_get_count = 0L;
long FSM_get_size = 0L;
long FSM_free_count = 0L;
long FSM_free_size = 0L;

/*
 *      This is the callback used to send out all error messages.
 */

static long (*error_callback)( char * msg ) = 0L;

/*
 *      This is the callback used to fetch more text from the user.
 */

static long (*read_callback)( char *buff, long size ) = 0L;

/*
 *      This is the list of defined grammars, and the one the is the
 *      current active grammar from that list.
 */

static struct GRAMMAR * grammars = 0L;
static struct GRAMMAR * current_grammar = 0L;

/*
 *      This is context information about the current grammar.  This
 *      information is copied from the current grammar structure when
 *      a grammar is made active, and saved again when the grammar is
 *      switched out.
 */

static struct DCL_VERB *verbs = 0L;
static struct DCL_VERB *current_verb = 0L;
static struct DCL_PARM *current_parameter = 0L;
static struct DCL_QUAL *current_qualifier = 0L;
static struct DISALLOW *disallow = 0L;
static struct DCL_TYPE * typenames = 0L;
static struct DCL_TYPE * current_typenames = 0L;
static struct DCL_TYPE * the_types = 0L;

/*
 *      This is the debugging state for the DCL component.  This is
 *      set by the debugger flag parameter to DCLparse().
 */

static int dcl_debug_state = DCL_NODEBUG;


/*
 *      These are globally addressable variables used in the DCLresults()
 *      routine that creates and loads an array of DCLdump callback data.
 */

static long result_count = 0L;
static long result_chars = 0L;

static struct DCL_RESULTS * result_data = 0;

/*
 *      These are the list heads for the table of defined strings,
 *      states, and error codes for the finite-state machine.
 */

static struct STRING *          fsm_strings;
static struct STATE *           fsm_states;
static struct ERROR *           fsm_errors;

/*
 *      These two pointers define the "state" of the FSM.  The current
 *      state is where the machine will begin on the next FSMrun()
 *      call.  The initial state is stored away by an explicit call and
 *      "primes" the FSM.
 */

static struct STATE *           fsm_current_state;
static struct STATE *           fsm_init_state;

/*
 *      This pointer shows the current parse location from an FSMrun()
 *      call when an action is returned.  It points to the first character
 *      of the current token.  Also, a flag that indicates if the FSM_ALLOWANY
 *      transition can work or not.
 */

static char *                   fsm_curpos = 0L;
static long                     fsm_allow_rest = 0;

/*
 *      This flag indicates if debugging is enabled.  When turned on, the
 *      FSM dumps diagnostic data about the state/transition traversal.
 *      The normal state is FSM_NODEBUG, which means no messages.
 */

static long     fsm_debug_state = FSM_NODEBUG;
/*<PAGE>*/

/****************************************************************************\
 *                                                                          *
 *                          PRIVATE PROTOTYPES                              *
 *                                                                          *
\****************************************************************************/


static long DCLentry( char * name );
static long DCLgrammar( char * name );
static long DCLendgrammar( void );
static long DCLverb (char *name , long id );
static long DCLparameter ( char * name, long id , int kind, int flags );
static long DCLprompt( char * prompt );
static long DCLqualifier( char * name, long id, int kind, int flags );
static long DCLkeyword (char *name , long id, int flags, char * syntax );
static long DCLlocal( char * name );
static long DCLqual_max( long max );
static long DCLqual_min( long min );
static long DCLgrammar_error( char * fmt, char * arg );
static long DCLinternal_error( char * fmt, char * arg );
static long DCLerror( char * fmt, char * arg );
static long DCLaction( char * action, char * token );
static long DCLdebug( char * msg, char * arg );
static long DCLdebugn( char * msg, long arg );
static long DCLkeysearch( struct DCL_KEYWORDS * keywords, char * token, char ** tp );
static struct DCL_QUAL * DCLqualsearch( struct DCL_QUAL * quals, char * token );
static long DCLkeylist( char * name );
static long DCLdefine_internal_grammar( void );
static long DCLpromptparm( char * gname, struct DCL_PARM * parm );
static long DCLjournal_element( void );
static long DCLjournal_write( char * buff );
static long DCLjournal_int( char * name, long value );
static long DCLjournal_chr( char * name, char * value );
static long DCLcallback_dcl( int kind, char * name, long id, long dtype, long i_value, char * c_value );
static long DCLalias( char * name );
static long DCLcheckid( char * kind, char * name, long id );
static long DCLaddverb( char * name, struct DCL_VERB ** v );
static long DCLsyntax( char * name );
static long DCLusekeylist( char * name );
static long DCLatoi( char * token, long * value, long flags );
static long DCLdefault( long typ, long i_value, char * c_value );
static long DCLdisallow( char * qual1, char * qual2 );
static long DCLupcase( char * In, char * Out );
static long DCLwritedefine( FILE * f, char * kind, char * name, long id );
static long DCLdiscard( char * name );
static long DCLexport_symbol( char * prefix, char * name, char * value );
static long DCLresolvetype( void );
static long DCLcheck_requirements( char * name );
static long DCLresults_counter( int kind, char * name, long id, long dtyp, long i_value, char * c_value );
static long DCLresults_load( int kind, char * name, long id, long dtyp, long i_value, char * c_value );
static long dcldeferr( char * msg );
static long userdeferr( char * msg );
static long DCLdump( long (*callback)(int, char*, long, long, long, char*));


static long FSMdefine (void );
static char *FSMgetmem (long size );
static long FSMfreemem( void * data );
static long FSMgetstring( char * prompt, char *dbuff );
static long FSMskipblanks (void );
static long FSMnotransition (long code );
static long FSMerror (long code , char *text );
static long FSMstring (char *name , char *initial , char *secondary ,
                        char * delimiters, int concat );
static long FSMstate (char *name , int blanks );
static long FSMstop (char *name );
static long FSMtransition (long kind , char *literal , char *state , char *arg );
static long FSMinitial_state (char *state );
static long FSMputerror (int code , char *text );
static long FSMrun (char **string , char **state , char **arg , char **token , int *toklen );
static long FSMdebugtrans (struct TRANSITION *t );
static long FSMdebug (char *msg , char *arg );
static long FSMinlist (char ch , char *list );
static long FSMisfilename( char * p, int * lenptr );
static long FSMisdate( char * p, int * lenptr, char * fmtdate );
/*<PAGE>*/

/****************************************************************************\
 *                                                                          *
 *                           CALLBACK STORAGE                               *
 *                                                                          *
\****************************************************************************/


static char * grammar_name = 0L;

static long journal = 0;
static FILE * journal_file = 0L;

/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                       EXTERNAL CALLBACK MANAGERS                         *
 *                                                                          *
\****************************************************************************/

/*
 *      This sets the journaling flag.  When set, the routine(s) that 
 *      execute a parsed *grammar* line cause C code to be written instead
 *      of actually storing the grammar in memory.
 */

long DCLjournal( char * name, char * rtn )
{

        char buff[ 80 ];

        if( journal ) {
                DCLjournal_write( "\treturn 0L;" );
                DCLjournal_write( "}" );

                fclose( journal_file );
                journal_file = 0L;
        }

        if( name == 0L ) {
                journal = 0;
                return DCL_NOERROR;
        }

        journal_file = xfopen( name, "w" );
        if( journal_file == 0L ) {
                DCLinternal_error( "Can't write file %s", name );
                journal = 0;
                return DCL_INTERNAL;
        }

        DCLjournal_write( "" );
        DCLjournal_write( "/*" );

        sprintf( buff, " *\tGrammar definition function %s", rtn );
        DCLjournal_write( buff );

        DCLjournal_write( " */" );
        DCLjournal_write( "" );
        DCLjournal_write( "#include \"dclprivate.h\"" );
        DCLjournal_write( "" );

        sprintf( buff, "long %s()", rtn );
        DCLjournal_write( buff );
        DCLjournal_write( "{" );

        journal = 1;
        return DCL_NOERROR;
}


/*
 *      This is called to set the default callback routine.  By default,
 *      messages are directly output to the user via printf() calls.  If
 *      the error callback is set, then messages are output by formatting
 *      them as text and calling this routine.
 */

long DCLsignal( long (*callback)(void))
{

        error_callback = (long(*)(char*)) callback;
        return DCL_NOERROR;
}

/*
 *      Set the debug states for the DCL processor and FSM machine.
 */

long DCLsetdebug( long dcldebug, long fsmdebug )
{

        dcl_debug_state = (int) dcldebug;
        fsm_debug_state = fsmdebug;
        return DCL_NOERROR;
}

/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                        MAIN USER PARSE ROUTINE                           *
 *                                                                          *
\****************************************************************************/

/*
 *      This is the main driver module.  It is given a grammar name to use, 
 *      an optional text buffer to parse, and an optional routine to use to
 *      fetch additional text as needed.
 */


long DCLparse( char * name, char * buff, long (*callback)(void) )
{

        char * p;
        char * s;
        char * state;
        char * arg;
        int rc, qflag;
        char * token;
        long sts;
        int toklen, kind;
        static int FSM_defined = 0;
		int running;

        char tbuff[ 256 ];
        char dbuff[ 256 ];

		rc = 0;
		sts = 0;
/*
 *      Store the read callback given to us.  The FSM uses this to fetch
 *      more text if it needs it.
 */

        read_callback = (long(*)(char*, long))callback;

/*
 *      Identify the current grammar to use for processing.
 */

        DCLgrammar( name );
        if( verbs == 0L ) {

                DCLinternal_error( "can't find grammar %s", name );
                return DCL_INTERNAL;
        }

        if( !current_grammar-> valid ) {
                sts = DCLvalidate();
                if( DCLERROR( sts ))
                        return sts;
        }

/*
 *      Set the DCL debug flag correctly, and initialize the current context
 *      of the grammar.
 */

        current_verb = 0L;
        current_parameter = 0L;
        current_qualifier = 0L;

/*
 *      Define the FSM.
 */

        if( !FSM_defined ) {
                FSM_defined = 1;
                FSMdefine();
        }


/*
 *      If the user gave us a buffer, use it, else try to use the callback.
 */

        if( buff != 0L ) {
                strcpy( dbuff, buff );
                DCLdebug( "processing >> ", buff );
        }
        else {
                rc = (int) FSMgetstring( "DCL> ", dbuff );
                if( rc == DCL_EOF )
                        return rc;
        }


/*
 *      Upcase anything not in quotation marks.
 */

        qflag = 0;
        DCLupcase( dbuff, dbuff );
        p = dbuff;
        s = p;

/*
 *      Define the initial state for the machine to run with.  This sets
 *      the "current" state to the named state.
 */

        FSMinitial_state( "verb" );


/*
 *      Run the machine, in a loop.  We pass a buffer to the data buffer
 *      to scan, and a pointer to a char * variable.  This is set to the
 *      name of the state we're in when we finish, if the action routine
 *      needs to know the current state.  We also get a pointer to the
 *      argument data if there is any.  Finally, the last parsed token
 *      and it's length are returned to us.  This is a pointer in to the
 *      data area that "s" points to.  Note that after a normal call, the
 *      value of "s" is updated to point to the next character to parse,
 *      and "token" points to an area of memory _before_ "s" where the
 *      last token came from.  The value of "toklen" is typically the
 *      difference between "token" and "s", though this is not guaranteed.
 */

		running = 1;
        while( running ) {

        /*
         *      If the next parameter is "REST OF LINE" then allow the
         *      FSM to accept parm state transitions that eat the rest
         *      of the command line. 
         */

                if( current_verb && current_verb-> nextparm ) {
                        kind = current_verb-> nextparm-> kind;
                        fsm_allow_rest = kind == DCL_RESTOFLINE ? 1 : 0;
                }

        /*
         *      Run the FSM until it finds another token of some kind.
         */

                rc = (int) FSMrun( &s, &state, &arg, &token, &toklen );

        /*
         *      Reset the flag that allows REST_OF_LINE processing so there
         *      can be no ambiguity if we re-entrantly execute the FSM.
         */

                fsm_allow_rest = 0;

        /*
         *      If the FSM needs more input, i.e. the string is empty
         *      but the "stop" state hasn't been hit, then the command
         *      is incomplete.
         */

                if( rc == FSM_NEEDMORE ) {
                        rc = (int) FSMgetstring( "DCL> ", dbuff );
                        if( rc == DCL_EOF )
                                break;
                        p = dbuff;
                        s = p;
                        continue;
                }

        /*
         *      A transition occurred with a non-zero action string.  The
         *      action string is stored in the transition structure, and
         *      in our case is a format string.  The token string is moved
         *      to a null terminated buffer, and printed using the format
         *      string.  In a more normal (non test) environment, the
         *      arg string would contain coded commands to the caller that
         *      describe what to do with the token.  A common action would
         *      be to look up the token in a table of valid tokens that
         *      indicates where the return value is to be stored, and putting
         *      the token there (in the case of a qualifier switch, for
         *      example.
         */

                if( rc == FSM_ACTION ) {

                        strncpy( tbuff, token, toklen );
                        tbuff[ toklen ] = 0;
                        if( arg == 0 )
                                arg = "";

                        rc = (int) DCLaction( arg, tbuff );
                        if( rc == DCL_NOERROR )
                                continue;
                        else
                                break;
                }

        /*
         *      The "stop" state was hit.  In this case, the caller is
         *      done and the FSM has halted.
         */

                if( rc == FSM_STOP ) {
                        rc = FSM_NOERROR;
                        break;
                }

        /*
         *      Otherwise, the return code is an error signalled from
         *      the FSM machine.  The error text from a transition
         *      failure will already have been output.
         */
                break;
        }

        /*
         *      See if there were any required parameters or
         *      qualifiers for the current verb we didn't see,
         *      and we're done.
         */

        if( rc == FSM_NOERROR ) {
                rc = (int) DCLcheck_requirements( name );
                if( rc == DCL_NOERROR )
                        rc = DCL_DONE;
        }

        /*
         *      Handle outputing the return code from the run.
         */

        DCLdebugrc( "DCLparse", rc );
        return rc;
}
/*<PAGE>*/

/****************************************************************************\
 *                                                                          *
 *                       GRAMMAR DEFINITION ROUTINES                        *
 *                                                                          *
\****************************************************************************/

/*
 *      Read in a file that contains grammar statements, and process
 *      them as a grammar definition.
 */

long DCLread( char * fname )
{
        char record[ 128 ];
        char buff[ 512 ];
        FILE *in;
        char * bp;
        long sts, len;
		int running;
        char *fnp;
        fnp = fname;
		sts = 0;
        in = xfopen( fnp, "r" );
        if( in == 0L ) {
            DCLinternal_error( "can't read file %s", fname );
            return DCL_INTERNAL;
        }
        buff[ 0 ] = 0;
        
		running = 1;
        while( running ) {
                sts = DCL_DONE;
                bp = fgets( record, 128, in );
                if( bp == 0L )
                        break;
                
                /* Required for Mac OS X, files may have returns or line feeds... */
                
                while(*bp == '\r') bp++;
                
                len = strlen( bp );
                if( len > 0 ) {
                    
                    if( bp[ len - 1 ] == '\n' || bp[ len -1 ] == '\r' ) {
                        len--;
                    }
                    
                    if( len > 0 && bp[ len - 1 ] == '-' ) {
                        bp[ len - 1 ] = 0;
                        strcat( buff, " " );
                        strcat( buff, bp );
                        continue;
                    }
                }
                
                strcat( buff, bp );
                sts = DCLdefine( buff );
                if( sts != DCL_DONE ) {
                        DCLdebugrc( "DCL", sts );
                        break;
                }
                buff[ 0 ] = 0;
        }

        fclose( in );

        /* If a grammar was being built, make sure it's valid  */

        if( grammar_name ) {
                DCLgrammar( grammar_name );
                sts = DCLvalidate();
                DCLendgrammar();
        }

        return sts;
}
/*<PAGE>*/

/*
 *      This is called with a text string that is a single statement
 *      in the grammar definition language.  We make sure the DCL
 *      language itself is defined, then process the statement.  The
 *      callbacks store away information about the command the user
 *      gave us, and the DCLdefine_element() call actually processes the user's
 *      statement and defines a language element for the user.
 */

long DCLdefine( char * cmd )
{
        long sts, sts2;
        int len, n, all_blanks;
        static int DCL_defined = 0;
        long (*saved_callback)( char * msg );

/*
 *      See if this is an empty string -- we don't do anything with these.
 */

        len = (int) strlen( cmd );
        all_blanks = 1;
        for( n = 0; n < len; n++ ) 
                if( !isspace( cmd[ n ] )) {
                        all_blanks = 0;
                        break;
                }

        if( all_blanks )
                return DCL_DONE;

/*
 *      If we've never defined the DCL grammar itself, do that now.
 */

        if( !DCL_defined ) {
                DCL_defined = 1;
                DCLdefine_internal_grammar();
        }


        saved_callback = error_callback;
        error_callback = userdeferr;

/*
 *      Process the grammar statement using the internally defined DCL
 *      grammar.
 */

        sts = DCLparse( "$$$DCL$$$", cmd, 0L );

//      printf( "DCL:    %s\n", cmd );
        
/*
 *      If the statement was syntactically okay, then process the
 *      callback data and perform the requested define operation.
 */

        dcl_private_store.kind = 0;
        dcl_private_store.keys[ 0 ] = 0;

        if( sts == DCL_DONE || sts == DCL_NOERROR ) {
                sts2 = DCLdump( DCLcallback_dcl ); /* Load the arguments     */
                if( sts2 != DCL_IVERB )            /* If a verb was given... */
                        sts = DCLdefine_element();         /* ...define the element  */

                if( sts == DCL_NOERROR )        /* Single return code for ok */
                        sts = DCL_DONE;
        }

/*
 *      Reset the message writer to what the user want's now that we are
 *      done with outputting messages...
 */

        error_callback = saved_callback;
/*
 *      Reset back to the DCL grammar (which was probably superceded during
 *      processing the user's grammar request) and free up the storage.
 */

        DCLgrammar( "$$$DCL$$$" );
        DCLreset();

        return sts;
}
/*<PAGE>*/
/*
 *      Check over all datastructures and make sure there are no dangling
 *      loose ends...  This is called internally before a grammar is
 *      used if it has been changed or loaded.  It can also be explicitly
 *      called by the user who is using the DCLdefine() interface.
 */

long DCLvalidate(void)
{

        struct DCL_TYPE * k;
        struct DCL_VERB * v;

        int rc;

        rc = DCL_NOERROR;

/*
 *      Make sure there is a current, active grammar.
 */

        if( current_grammar == 0L ) {
                DCLinternal_error( "no active grammar to validate", "" );
                return DCL_INTERNAL;
        }

/*
 *      Optionally, put out a debug message showing what we're doing.
 */

        DCLdebug( "validating grammar", current_grammar-> name );

/*
 *      Make sure there is at least one defined verb.
 */

        if( verbs == 0L ) {
                DCLgrammar_error( "no verbs defined for this grammar", "" );
                rc = DCL_INTERNAL;
        }

/*
 *      Search for verbs created by /SYNTAX references that never got
 *      defined.
 */

        for( v = verbs; v; v = v-> next ) {
                if( v-> id == DCL_UNDEFINED  ) {
                        if( strncmp( v-> name, "SYNTAX$", 7 ) == 0 )
                                DCLgrammar_error( "syntax %s referenced but not defined",
                                        v-> name );
                        else
                                DCLgrammar_error( "verb %s referenced but not defined",
                                        v-> name );

                        rc = DCL_INTERNAL;
                }
        }

/*
 *      Search for keylists that are empty, and are probably the
 *      result of a reference to an undefined list.
 */

        for( k = typenames; k; k = k-> next ) {
                if( k-> keylist == 0L ) {
                        DCLgrammar_error( "empty or undefined TYPE of %s",
                                k-> name );
                        rc = DCL_INTERNAL;
                }
        }

/*
 *      All done, indicate if this grammar can be used...
 */

        current_grammar-> valid = ( rc == DCL_NOERROR ) ? 1 : 0;
        return rc;
}

/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                         DATA EXPORT ROUTINES                             *
 *                                                                          *
\****************************************************************************/

/*
 *      Write out a header file containing grammar definitions for
 *      a named grammar.
 */

long DCLwriteheader( char * fname, char * grammar )
{

        struct GRAMMAR * g;
        struct DCL_VERB * v;
        struct DCL_PARM * p;
        struct DCL_QUAL * q;
        struct DCL_TYPE * t;
        struct DCL_KEYWORDS * k;
        FILE * f;

        char gbuff[ 32 ];

/*
 *      Search for the matching grammar.
 */
        DCLupcase( grammar, gbuff );
        for( g = grammars; g; g = g-> next ) {
                if( strcmp( gbuff, g-> name ) == 0 ) 
                        break;
        }

        if( g == 0L ) {
                DCLinternal_error( "can't write headers, grammar %s not found", gbuff );
                return DCL_INTERNAL;
        }

/*
 *      Open the file...
 */

        f = xfopen( fname, "w" );
        if( f == 0L ) {
                DCLinternal_error( "cannot open output header file %s", fname );
                return DCL_INTERNAL;
        }

/*
 *      Write out a header.
 */

        fprintf( f, "/*\n" );
        fprintf( f, " *  Parse element symbols for grammar %s\n", gbuff );
        fprintf( f, " */\n" );
        fprintf( f, "\n" );
        fprintf( f, "#ifndef DCL_UNDEFINED\n" );
        fprintf( f, "#define DCL_UNDEFINED   0x%08lX\n", (long) DCL_UNDEFINED );
        fprintf( f, "#define DCL_NO_ID       0x%08lX\n", DCL_NO_ID );
        fprintf( f, "#endif\n" );
        fprintf( f, "\n" );

/*
 *      For each keyword...
 */

        for( t = g-> typenames; t; t = t-> next ) 
                for( k = t-> keylist; k; k = k-> next ) 
                        DCLwritedefine( f, "KEYWORD", k-> name, k-> id );


                
/*
 *      For each verb...
 */

        for( v = g-> verbs; v; v = v-> next ) {

                DCLwritedefine( f, "VERB", v-> name, v-> id );

                for( p = v-> parameter; p; p = p-> next )
                        DCLwritedefine( f, "PARAMETER", p-> name, p-> id );
                for( q = v-> qualifier; q; q = q-> next )
                        DCLwritedefine( f, "QUALIFIER", q-> name, q-> id );
        }

        fclose( f );
        return DCL_NOERROR;
}
/*<PAGE>*/

/*
 *      Local routine to write out a symbol to a file given it's kind,
 *      name, and id.  The "kind" is a string like "PARAMETER" or "VERB"
 *      and is made part of the #define symbol name.
 */

static long DCLwritedefine( FILE * f, char * kind, char * name, long id )
{
        int len, n;
        char xbuff[ 64 ];
        char idb[ 12 ];
        char * ip;


/*
 *      Format the name of the symbol into a buffer.  Uppercase the name,
 *      and convert "illegal" characters to underscores.
 */

        strcpy( xbuff, "DCL_" );
        strcat( xbuff, kind );
        strcat( xbuff, "_" );
        strcat( xbuff, name );
        DCLupcase( xbuff, xbuff );
        len = (int) strlen( xbuff );
        for( n = 0; n < len; n++ ) 
                if( FSMinlist( xbuff[ n ], " $" ))
                        xbuff[ n ] = '_';

/*
 *      Format the id.  For special cases, we use the name of the
 *      special case symbol.  Otherwise, format the id as a string.
 */

        if( id == DCL_UNDEFINED )
                ip = "DCL_UNDEFINED";
        else
        if( id == DCL_NO_ID )
                ip = "DCL_NO_ID";
        else {
                sprintf( idb, "%8ld", id );
                ip = idb;
        }

/*
 *      Write the #define to the output file.  Line up the columns
 *      neatly with width specification on the varible component
 *      for the name.
 */

        fprintf( f, "#define %-50s %s\n", xbuff, ip );
        return DCL_NOERROR;
}
/*<PAGE>*/
/*
 *      Export the current grammar settings.  This is done via host-specific
 *      means to create external environmental data that can be used to
 *      reflect the results of a parse.
 */

long DCLexport(void)
{
        char b[ 256 ], db[ 256 ];
        struct DCL_PARM * p;
        struct DCL_QUAL * q;
        struct DCL_VALUE * v;
        char * cp;

        if( current_verb == 0L ) {
                DCLinternal_error( "no current parse to export", "" );
                return DCL_INTERNAL;
        }

        DCLexport_symbol( "VERB", 0L, current_verb-> name );

        for( p = current_verb-> parameter; p; p = p-> next ) {

                if( p-> kind == DCL_INTEGER ) {
                        sprintf( b, "%ld", p-> i_value );
                        cp = b;
                }
                else
                if( p-> kind == DCL_KEYWORD ) {
                        cp = "";
                        if( p-> i_value > 0 )
                                cp = p-> c_value;
                        else
                        if( p-> i_value < 0 ) {
                                strcpy( b, "NO" );
                                strcat( b, p-> c_value );
                                cp = b;
                        }
                }
                else {
                        cp = p-> c_value;
                        if( cp == 0L ) 
                                cp = "";
                }
                DCLexport_symbol( "P_", p-> name, cp );
        }

        for( q = current_verb-> qualifier; q; q = q-> next ) {
                if( q-> flags & DCL_HASVALUE ) {
                        db[ 0 ] = 0;
                        for( v = q-> value; v; v = v-> next ) {
                                if( q-> kind == DCL_INTEGER ) {
                                        sprintf( b, "%ld", v-> i_value );
                                        cp = b;
                                }
                                else {
                                        cp = v-> c_value;
                                        if( cp == 0L ) 
                                                cp = "";
                                }
                                if( db[ 0 ] )
                                        strcat( db, "," );
                                strcat( db, cp );
                        }
                        DCLexport_symbol( "Q_", q-> name, db );
                }
                else {
                        if( q-> state == DCL_NEGATED )
                                cp = "FALSE";
                        else
                        if( q-> state == DCL_ABSENT )
                                cp = "";
                        else
                                cp = "TRUE";
                        DCLexport_symbol( "Q_", q-> name, cp );
                }

        }
        return DCL_NOERROR;
}
/*<PAGE>*/

/*
 *      Given the information for a single symbol, export it.  The prefix
 *      is used to build the symbol name, and may contain the name if it
 *      was explicitly given.  The value is the value of the exported 
 *      symbol.  THIS IS A HOST SPECIFIC ROUTINE.  It may export a DCL
 *      symbol, a Unix environment variable, etc...
 */

static long DCLexport_symbol( char * prefix, char * name, char * value )
{
		char * dummy2;

		dummy2 = name;
		dummy2 = prefix;
		dummy2 = value;


        printf( "DCLexport_symbol not supported\n" );
        

        return DCL_NOERROR;
}

/*<PAGE>*/

/****************************************************************************\
 *                                                                          *
 *                           VERB DISPATCHING                               *
 *                                                                          *
\****************************************************************************/

/*
 *      Routine that binds a routine address to a verb in a grammar, for
 *      dispatch purposes.
 */

long DCLbind( int kind, char * grammar, char * verb, void * addr )
{

        char gname[ 32 ];
        char vname[ 64 ], vbuff[ 64 ];
        int len;

        struct GRAMMAR * g;
        struct DCL_VERB * v;

/*
 *      Make sure we are asking to bind correctly!
 */

        if( kind != DCL_BIND_VERB && kind != DCL_BIND_SYNTAX ) {
                DCLinternal_error( "%s", "invalid DCLbind() type" );
                return DCL_INTERNAL;
        }

/*
 *      Upcase the parameters.
 */

        len = (int) strlen( grammar );
        DCLupcase( grammar, gname );

        len = (int) strlen( verb );
        DCLupcase( verb, vname );


/*
 *      If the bind type is for a syntax, then munge the name...
 */

        if( kind == DCL_BIND_SYNTAX ) {
                strcpy( vbuff, "SYNTAX$" );
                strcat( vbuff, vname );
                strcpy( vname, vbuff );
        }

/*
 *      Find the matching grammar.
 */

        for( g = grammars; g; g = g-> next )
                if( strcmp( gname, g-> name ) == 0 )
                        break;

        if( g == 0L ) {
                DCLinternal_error( "bind error, can't find grammar %s",
                        gname );
                return DCL_INTERNAL;
        }


/*
 *      Find the matching verb
 */

        for( v = g-> verbs; v; v = v-> next ) 
                if( strcmp( vname, v-> name ) == 0 )
                        break;

/*
 *      If it's not found, blow the user off.  It's too dangerous to let
 *      the caller bind to grammar entries that don't exist at all.
 */

        if( v == 0L ) {
                g-> valid = 0;
                DCLinternal_error( "bind error, grammar/verb %s not found",
                                vname );
                return DCL_INTERNAL;
        }

/*
 *      Store the address value.
 */
        DCLdebug( "binding address to verb", v-> name );

        v-> routine = (long (*)(long)) addr;
        return DCL_NOERROR;
}

/*<PAGE>*/
/*
 *      Routine that dispatches to the correct verb handling routine.
 */

long DCLdispatch(void)
{

        struct DCL_VERB * v;

/*
 *      Find the active verb.
 */

        for( v = verbs; v; v = v-> next ) 
                if( v-> state == DCL_PRESENT )
                        break;

        if( v == 0L ) {
                DCLinternal_error( "no active verb to dispatch %s", "" );
                return DCL_INTERNAL;
        }

        if( v-> routine == 0L )
                return DCL_NODISPATCH;

        DCLdebug( "dispatching routine for verb", v-> name );

        return (*(v-> routine))( v-> id );
}
/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                        PARSE DELIVERY CALLBACKS                          *
 *                                                                          *
\****************************************************************************/

/*
 *      Routine that returns all data from the currently processed parameter
 *      as an array of elements that contain each callback record.
 */

long DCLresults( struct DCL_RESULTS ** ResultData )
{
        long size;
        long sts;
        char * charpool;

        if( ResultData == 0L ) {
                DCLinternal_error( "DCLresults passed a NIL parameter", "" );
                return DCL_INTERNAL;
        }

        result_data= *ResultData;
        if( result_data )
                FSMfreemem( result_data );

        result_count = 0;
        result_chars = 0;

        DCLdump( DCLresults_counter );


        size = sizeof( struct DCL_RESULTS ) + 
                        result_chars +
                        sizeof( struct DCL_RESULT_ELEMENT ) * result_count;

        result_data = ( struct DCL_RESULTS * ) FSMgetmem( size );

        result_data-> count = result_count;
        charpool = ( char * ) result_data +
                        sizeof( struct DCL_RESULTS ) + 
                        sizeof( struct DCL_RESULT_ELEMENT ) * result_count;

        result_data-> char_data = charpool;
        result_count = 0;

        sts = DCLdump( DCLresults_load );
        result_data-> char_data = charpool;

        *ResultData= result_data;
        return sts;
}

/*
 *      Internal routine used just to count the callbacks.
 */

long DCLresults_counter( int kind, char * name, long id, long dtyp, long i_value, char * c_value ) 
{
		int n;

		n = (int) (dtyp + i_value + kind + id);	/* Dummy code */

        result_count++;

        if( name )
                result_chars += strlen( name ) + 1;

        if( c_value )
                result_chars += strlen( c_value ) + 1;

        return DCL_NOERROR;
}

long DCLresults_load( int kind, char * name, long id, long dtyp, long i_value, char * c_value ) 
{

        result_data-> element[ result_count ].kind      = kind;
        result_data-> element[ result_count ].id        = id;
        result_data-> element[ result_count ].datatype  = dtyp;
        result_data-> element[ result_count ].i_value   = i_value;

        if( name ) {
                strcpy( result_data-> char_data, name );
                result_data-> element[ result_count ].name = result_data-> char_data;
                result_data-> char_data += strlen( name ) + 1;
        }
        else
                result_data-> element[ result_count ].name = 0L;

        if( c_value ) {
                strcpy( result_data-> char_data, c_value );
                result_data-> element[ result_count ].c_value = result_data-> char_data;
                result_data-> char_data += strlen( c_value ) + 1;
        }
        else
                result_data-> element[ result_count ].c_value = 0L;

        result_count++;
        return DCL_NOERROR;
}

/*<PAGE>*/
/*
 *      Routine that dumps out all the data there is for the currently
 *      processed grammer.  It accepts a single parameter, which is a
 *      pointer to a call-back routine, which is called for each item
 *      found.
 */

static long DCLdump( long (*callback)(int, char*, long, long, long, char*))
{
        long sts;
        struct DCL_VERB * v;
        struct DCL_QUAL * q;
        struct DCL_PARM * p;
        struct DCL_VALUE * i;
        char qbuff[ 64 ];

/*
 *      First, locate the verb that is active.
 */

        for( v = verbs; v; v = v-> next )
                if( v-> state != DCL_ABSENT )
                        break;

        if( v == 0L ) {
                DCLdebug( "no verb found to dump","!!" );
                return DCL_IVERB;
        }

        sts = (*callback)( DCL_CALLBACK_VERB, 
                v-> name, v-> id, DCL_NONE, v-> id, v-> name );
        if( sts != DCL_NOERROR )
                return sts;

/*
 *      Now, all the parameters.
 */

        for( p = v-> parameter; p; p = p-> next ) {

                if( p-> state != DCL_ABSENT ) {
                        sts = (*callback)( DCL_CALLBACK_PARAMETER, 
                                        p-> name, p-> id,
                                        p-> kind,
                                        p-> state * p-> i_value, p-> c_value );
                        if( sts != DCL_NOERROR )
                                return sts;
                }
        }

/*
 *      Now, all the qualifier values.
 */

        for( q = v-> qualifier; q; q = q-> next ) {

                if( q-> state != DCL_ABSENT ) {
                        if( q-> state < 0 ) 
                                strcpy( qbuff, "NO" );
                        else
                                qbuff[ 0 ] = 0;
                        strcat( qbuff, q-> name );
                        if(!( q-> flags & DCL_HASVALUE )) {
                                sts = (*callback)( DCL_CALLBACK_QUALIFIER,
                                                qbuff, q-> id, DCL_NONE,
                                                ( long ) q-> id * q-> state,
                                                qbuff );
                        }
                        else
                        for( i = q-> value; i; i = i-> next ) {
                                sts = (*callback)( DCL_CALLBACK_QUALIFIER,
                                                qbuff, i-> id, q-> kind,
                                                i-> i_value, i-> c_value );
                                if( sts != DCL_NOERROR )
                                        return sts;
                        }
                }
        }

        return DCL_NOERROR;
}
/*<PAGE>*/

/*
 *      Routine called back by the dump routine by default from the 
 *      driver module -- this simply displays everything we find.
 */

long DCLcallback_debug( int kind, char * name, long id, long dtyp, long i_value, char * c_value ) 
{
        char * ivp, * idp;
        char ibuff[ 32 ], idbuff[ 32 ];
        char * msg, *cv, *dt;

        if( id == DCL_NO_ID || id == DCL_UNDEFINED ) {
                idp = "<none>";
        }
        else {
                sprintf( idbuff, "%ld", id );
                idp = idbuff;
        }

        switch( dtyp ) {
        case DCL_NONE:
                dt = "none";
                break;

        case DCL_ANY:
                dt = "any";
                break;

        case DCL_NAME:
                dt = "name";
                break;

        case DCL_INTEGER:
                dt = "integer";
                break;

        case DCL_STRING:
                dt = "string";
                break;

        case DCL_KEYWORD:
                dt = "keyword";
                break;

        case DCL_FILENAME:
                dt = "filename";
                break;

        case DCL_DATE:
                dt = "date";
                break;
        default:
                dt = "<none>";
                break;
        }


        switch( kind ) {
        case DCL_CALLBACK_VERB:
                msg = "verb";
                break;

        case DCL_CALLBACK_PARAMETER:
                msg = "parameter";
                break;

        case DCL_CALLBACK_QUALIFIER:
                msg = "qualifier";
                break;

        default:
                msg = "<internal error>";
                break;
        }

        if( i_value == DCL_UNDEFINED || i_value == DCL_NO_ID ) {
                ivp = "<none>";
        }
        else {
                sprintf( ibuff, "%ld", i_value );
                ivp = ibuff;
        }

        cv = ( c_value == 0L ) ? "<emptystring>" : c_value;
        printf( "DCLdump: %-12s  %-8s %-6s  %-12s %-8s  %s\n", 
                        msg, name, idp, dt, ivp, cv );

        return DCL_NOERROR;
}

/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                            QUERY ROUTINES                                *
 *                                                                          *
\****************************************************************************/

/*
 *      Routine that given a value type and id will return the state of
 *      the item.
 */

long DCLpresent( long kind, long id )
{

        struct DCL_QUAL * q;
        struct DCL_PARM * p;

        if( current_verb == 0L ) {
                DCLinternal_error( "no context to return DCLpresent() info","");
                return DCL_INTERNAL;
        }

        if( kind == DCL_CALLBACK_QUALIFIER ) {

                for( q = current_verb-> qualifier; q; q = q-> next ) {

                        if( q-> id == id )
                                return q-> state;
                }
                DCLinternal_error( "attempt to query nonexistent qualifier","");
                return DCL_INTERNAL;
        }

        if( kind == DCL_CALLBACK_PARAMETER ) {

                for( p = current_verb-> parameter; p; p = p-> next ) {

                        if( p-> id == id )
                                return p-> state;
                }
                DCLinternal_error( "attempt to query nonexistent parameter","");
                return DCL_INTERNAL;
        }

        DCLinternal_error( "invalid query type for DCLpresent() call", "" );
        return DCL_INTERNAL;
}
/*<PAGE>*/

/*
 *      Routine that given a value type and id will return the state of
 *      the item.
 */

long DCLgetkeyword( long kind, long id, long keyid )
{

        struct DCL_QUAL * q;
        struct DCL_PARM * p;
        struct DCL_VALUE * v;
        long n;

        if( current_verb == 0L ) {
                DCLinternal_error( "no context to return DCLpresent() info","");
                return DCL_INTERNAL;
        }

        if( kind == DCL_CALLBACK_QUALIFIER ) {

                for( q = current_verb-> qualifier; q; q = q-> next ) {

                        if( q-> id == id ) {
                                if( q-> kind != DCL_KEYWORD ) {
                                        DCLinternal_error( 
                                                "attempt to perform keyword query on non-keyword qualifier",
                                                q-> name );
                                        return DCL_INTERNAL;
                                }

                                for( v = q-> value; v; v = v-> next ) {
                                        n = DCLABS( v-> i_value );
                                        if( n == keyid )
                                                return DCLSIGN( v-> i_value );
                                }
                                return DCL_ABSENT;
                        }

                }
                DCLinternal_error( "attempt to query nonexistent qualifier","");
                return DCL_INTERNAL;
        }

        if( kind == DCL_CALLBACK_PARAMETER ) {

                for( p = current_verb-> parameter; p; p = p-> next ) {

                        if( p-> id == id ) {
                                if( p-> kind != DCL_KEYWORD ) {
                                        DCLinternal_error( 
                                                "attempt to perform keyword query on non-keyword parameter",
                                                p-> name );
                                        return DCL_INTERNAL;
                                }

                                n = DCLABS( p-> i_value );
                                if( n == keyid )
                                        return DCLSIGN( p-> i_value );
                                else
                                        return DCL_ABSENT;
                        }
                }
                DCLinternal_error( "attempt to query nonexistent parameter","");
                return DCL_INTERNAL;
        }

        DCLinternal_error( "invalid query type for DCLpresent() call", "" );
        return DCL_INTERNAL;
}

/*<PAGE>*/

/*
 *      Return the "entry" string for the current verb, if any.
 */

char * DCLgetentry(void)
{
        return current_verb-> entry;
}


/*
 *      Routine that given a value type and id will return the character
 *      value the item.
 */

char * DCLgetstring( long kind, long id )
{

        struct DCL_QUAL * q;
        struct DCL_PARM * p;

        if( current_verb == 0L ) {
                DCLinternal_error( "no context to return DCLgetstring() info","");
                return 0L;
        }

        if( kind == DCL_CALLBACK_QUALIFIER ) {

                for( q = current_verb-> qualifier; q; q = q-> next ) {

                        if( q-> id == id )
                                return q-> value-> c_value;
                }
                DCLinternal_error( "attempt to query nonexistent qualifier","");
                return 0L;
        }

        if( kind == DCL_CALLBACK_PARAMETER ) {

                for( p = current_verb-> parameter; p; p = p-> next ) {

                        if( p-> id == id )
                                return p-> c_value;
                }
                DCLinternal_error( "attempt to query nonexistent parameter","");
                return 0L;
        }

        DCLinternal_error( "invalid query type for DCLgetstring() call", "" );
        return 0L;
}

/*<PAGE>*/

/*
 *      Routine that given a value type and id will return the integer
 *      value of the item.
 */

long DCLgetinteger( long kind, long id )
{

        struct DCL_QUAL * q;
        struct DCL_PARM * p;

        if( current_verb == 0L ) {
                DCLinternal_error( "no context to return DCLgetinteger() info","");
                return DCL_INTERNAL;
        }

        if( kind == DCL_CALLBACK_QUALIFIER ) {

                for( q = current_verb-> qualifier; q; q = q-> next ) {

                        if( q-> id == id ) {
                                if( q-> value )
                                    return q-> value-> i_value;
                                else
                                    return 0;
                        }
                }
                DCLinternal_error( "attempt to query nonexistent qualifier","");
                return DCL_INTERNAL;
        }

        if( kind == DCL_CALLBACK_PARAMETER ) {

                for( p = current_verb-> parameter; p; p = p-> next ) {

                        if( p-> id == id )
                                return p-> i_value;
                }
                DCLinternal_error( "attempt to query nonexistent parameter","");
                return DCL_INTERNAL;
        }

        DCLinternal_error( "invalid query type for DCLgetinteger() call", "" );
        return DCL_INTERNAL;
}



/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                           STORAGE MANAGEMENT                             *
 *                                                                          *
\****************************************************************************/

/*
 *      Release storage and reset pointers and flags for the currently
 *      defined grammar.
 */

long DCLreset(void)
{

        struct DCL_VERB * v;
        struct DCL_PARM * p;
        struct DCL_QUAL * q;
        struct DCL_VALUE * d, *dn;

        DCLdebug( "resetting storage for grammar", current_grammar-> name );

        for( v = verbs; v; v = v-> next ) {

                v-> state = DCL_ABSENT;

                for( p = v-> parameter; p; p = p-> next ) {

                        p-> state = DCL_ABSENT;
                        if( p-> c_value )
                                FSMfreemem( p-> c_value );
                        p-> c_value = 0L;
                        p-> i_value = 0;
                }

                for( q = v-> qualifier; q; q = q-> next ) {

                        q-> state = DCL_ABSENT;
                        q-> flags &= ~DCL_HASVALUE;
                        for( d = q-> value; d; d = dn ) {
                                dn = d-> next;
                                FSMfreemem( d );
                        }
                        q-> value = 0L;
                }
        }

        return DCL_NOERROR;
}

/*<PAGE>*/

/*
 *      Memory allocator.  This is typically not portable, and is
 *      encapsulated as an FSM routine.  The default is to simply
 *      call malloc().
 */

static char * FSMgetmem( long size )
{
        char * p;
        int n;
        
        FSM_get_count++;
        FSM_get_size += size;

        p = malloc( size );
        for( n = 0; n < size; n++ )
            p[ n ] = 0;
        return p;
        
}

static long FSMfreemem( void * data )
{

        char * p;

        p = data;
        free( p );

        FSM_free_count++;

        return FSM_NOERROR;
}

long DCLmemstats( int reset )
{

        printf( "DCLmemstats: There are %ld bytes allocated by %ld requests.\n",
                        FSM_get_size, FSM_get_count );
        if( reset ) {
                FSM_get_size = 0;
                FSM_get_count = 0;
        }

        return FSM_NOERROR;
}

/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                       INTERNAL DEFINTION ROUTINES                        *
 *                                                                          *
\****************************************************************************/


/*
 *      Define the grammar used to define other grammars... This is done
 *      with explicit statements to the grammar definition routines.  The
 *      end user doesn't typically use these, but instead sends in statements
 *      defining the grammar that we parse ourselves and form the right 
 *      grammar definition.
 */

static long DCLdefine_internal_grammar(void)
{

        long (*saved_callback)( char * msg );

        saved_callback = error_callback;
        error_callback = dcldeferr;

        DCLgrammar( "$$$DCL$$$" );

        DCLkeylist( "AND" );
        DCLkeyword( "AND",      DCL_ANY,        DCL_NONEGATE, 0L );

        DCLverb(        "GRAMMAR",      DCL_VERB_GRAMMAR );
        DCLparameter(   "NAME",         DCL_PARAMETER_NAME,             DCL_ANY,        DCL_REQ );

        DCLverb(        "VERB",         DCL_VERB_VERB );
        DCLparameter(   "NAME",         DCL_PARAMETER_NAME,             DCL_ANY,        DCL_REQ );
        DCLqualifier(   "IDENT",        DCL_QUALIFIER_IDENT,            DCL_INTEGER,    DCL_REQ );
        DCLqualifier(   "ALIAS_FOR",    DCL_QUALIFIER_ALIAS_FOR,        DCL_ANY,        DCL_REQ );
        DCLqualifier(   "ENTRY",        DCL_QUALIFIER_ENTRY,            DCL_ANY,        DCL_NONE );

        DCLverb(        "PARAMETER",    DCL_VERB_PARAMETER );
        DCLparameter(   "NAME",         DCL_PARAMETER_NAME,             DCL_ANY,        DCL_REQ );
        DCLqualifier(   "IDENT",        DCL_QUALIFIER_IDENT,            DCL_INTEGER,    DCL_REQ );
        DCLqualifier(   "REQUIRED",     DCL_QUALIFIER_REQUIRED,         DCL_NONE,       DCL_NONE );
        DCLqualifier(   "TYPE",         DCL_QUALIFIER_TYPE,             DCL_ANY,        DCL_NONE );
        DCLqualifier(   "PROMPT",       DCL_QUALIFIER_PROMPT,           DCL_ANY,        DCL_NONE );
        DCLqualifier(   "MULTIPLIER",   DCL_QUALIFIER_MULTIPLIER,       DCL_NONE,       DCL_NONE );

        DCLverb(        "KEYWORD",      DCL_VERB_KEYWORD );
        DCLparameter(   "NAME",         DCL_PARAMETER_NAME,             DCL_ANY,        DCL_REQ );
        DCLqualifier(   "IDENT",        DCL_QUALIFIER_IDENT,            DCL_INTEGER,    DCL_REQ );
        DCLqualifier(   "NONEGATABLE",  DCL_QUALIFIER_NONEGATABLE,      DCL_NONE,       DCL_NONEGATE );
        DCLqualifier(   "SYNTAX",       DCL_QUALIFIER_SYNTAX,           DCL_ANY,        DCL_NONEGATE );

        DCLverb(        "QUALIFIER",    DCL_VERB_QUALIFIER );
        DCLparameter(   "NAME",         DCL_PARAMETER_NAME,             DCL_ANY,        DCL_REQ );
        DCLqualifier(   "IDENT",        DCL_QUALIFIER_IDENT,            DCL_INTEGER,    DCL_REQ );
        DCLqualifier(   "REQUIRED",     DCL_QUALIFIER_REQUIRED,         DCL_NONE,       DCL_NONE );
        DCLqualifier(   "NONEGATABLE",  DCL_QUALIFIER_NONEGATABLE,      DCL_NONE,       DCL_NONEGATE );
        DCLqualifier(   "LIST",         DCL_QUALIFIER_LIST,             DCL_NONE,       DCL_NONE );
        DCLqualifier(   "TYPE",         DCL_QUALIFIER_TYPE,             DCL_ANY,        DCL_NONE );
        DCLqualifier(   "SYNTAX",       DCL_QUALIFIER_SYNTAX,           DCL_ANY,        DCL_NONEGATE );
        DCLqualifier(   "ALIAS_FOR",    DCL_QUALIFIER_ALIAS_FOR,        DCL_ANY,        DCL_REQ );
        DCLqualifier(   "MINIMUM",      DCL_QUALIFIER_MINIMUM,          DCL_INTEGER,    DCL_REQ );
        DCLqualifier(   "MAXIMUM",      DCL_QUALIFIER_MAXIMUM,          DCL_INTEGER,    DCL_REQ );
        DCLqualifier(   "DEFAULT",      DCL_QUALIFIER_DEFAULT,          DCL_ANY,        DCL_NONE );
        DCLqualifier(   "IMPLY",        DCL_QUALIFIER_IMPLY,            DCL_ANY,        DCL_REQ );
        DCLqualifier(   "MULTIPLIER",   DCL_QUALIFIER_MULTIPLIER,       DCL_NONE,       DCL_NONE );
        DCLqualifier(   "LOCAL_TO",     DCL_QUALIFIER_LOCAL,            DCL_ANY,        DCL_REQ );

        DCLverb(        "TYPE",         DCL_VERB_TYPE );
        DCLparameter(   "NAME",         DCL_PARAMETER_NAME,             DCL_ANY,        DCL_REQ );

        DCLverb(        "SYNTAX",       DCL_VERB_SYNTAX );
        DCLparameter(   "NAME",         DCL_PARAMETER_NAME,             DCL_ANY,        DCL_REQ );
        DCLqualifier(   "IDENT",        DCL_QUALIFIER_IDENT,            DCL_INTEGER,    DCL_REQ );
        DCLqualifier(   "ENTRY",        DCL_QUALIFIER_ENTRY,            DCL_ANY,        DCL_NONE );

        DCLverb(        "END",          DCL_VERB_END );

        DCLverb(        "DISALLOW",     DCL_VERB_DISALLOW );
        DCLparameter(   "NAME",         DCL_PARAMETER_NAME,             DCL_ANY,        DCL_REQ );
        DCLparameter(   "AND",          DCL_PARAMETER_AND,              DCL_KEYWORD,    DCL_REQ );
        DCLusekeylist(  "AND" );
        DCLparameter(   "NAME2",        DCL_PARAMETER_NAME2,            DCL_ANY,        DCL_REQ );

        DCLvalidate();
        DCLendgrammar();

        error_callback = saved_callback;
        return DCL_NOERROR;

}
/*<PAGE>*/
/*
 *      Define the finite-state-machine attributes for the DCL parser.
 */

static long FSMdefine(void)
{

        static char * delims = " /,+)\n\t";
        static char * qdelims = " /=,+)\n\t";

/*
 *      Initialize the debugger to OFF.
 */

        fsm_debug_state = FSM_NODEBUG;


/*
 *      Define the error messages.
 */

        FSMerror( 101, "expected verb not found" );
        FSMerror( 102, "missing qualifier name after /" );
        FSMerror( 103, "no value after '='");
        FSMerror( 104, "expected list value not found" );
        FSMerror( 105, "list not ended with ')' or missing comma" );
        FSMerror( 106, "invalid hexadecimal constant" );
        FSMerror( 107, "invalid parameter syntax" );

/*
 *      Define some strings.   The "_space_" string is implicitly used
 *      by FSMstate when the second flag parameter is set to FSM_SKIPBLANKS.
 *      The "_name_" string indicates works defined by the user entry.
 *      The "_integer_" refers to a positive decimal integer.
 */

        FSMstring( "_space_", " \n\t", 0L, 0L, FSM_NOCONCAT );
        FSMstring( "_name_", 
                "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ$_",
                "0123456789", delims, FSM_CONCAT );
        FSMstring( "_qname_", 
                "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ$_",
                "0123456789", qdelims, FSM_CONCAT );
        FSMstring( "_integer_", "0123456789", "KMG", delims, FSM_CONCAT );
        FSMstring( "_quotes_", "\"'", 0L, 0L, FSM_NOCONCAT );
        FSMstring( "_hex_", "0123456789ABCDEFabcdef", 0L, 0L, FSM_NOCONCAT );

        FSMstring( "_list_", ", ", 0L, 0L, FSM_NOCONCAT );
/*
 *      VERB STATE
 *
 *      This is the initial state.  It ensures that the next token is an
 *      identifier, and passes that back to the caller.  The next state
 *      is a check for a parameter.  If there is no verb, then error.
 */

        FSMstate( "verb", FSM_SKIPBLANKS );
        FSMtransition( FSM_LITERAL, "!", "stop", 0L );
        FSMtransition( FSM_STRING, "_name_", "parm", "V" );
        FSMnotransition( 101 );

/*
 *      PARM STATE
 *
 *      This state checks for a parameter, or a qualifier.  In the case of
 *      a parameter entry (identifier or number) it transfers back to itself
 *      to search for another parameter.  If it's a "/" then it indicates a
 *      qualifier and we switch to a qualifier processing state.  If none
 *      of the above apply (FSM_NULL transition) then we automatically go
 *      to the "stop" state to terminate processing.
 */

        FSMstate( "parm", FSM_SKIPBLANKS );
        FSMtransition( FSM_LITERAL, "!", "stop", 0L );
        FSMtransition( FSM_LITERAL, "/", "qualifier", 0L );
        FSMtransition( FSM_RESTOFLINE, 0L, "stop", "PR" );
        FSMtransition( FSM_DATE, 0L, "parm", "PD" );
        FSMtransition( FSM_STRING, "_name_", "parm", "PN" );
        FSMtransition( FSM_STRING, "_integer_", "parm",     "PI" );
        FSMtransition( FSM_QUOTE,  "_quotes_", "parm",     "PS" );
        FSMtransition( FSM_LITERAL, "%X", "hexparm", 0L );
        FSMtransition( FSM_FILENAME, 0L, "parm", "PF" );
        FSMtransition( FSM_EOS, 0L, "stop", 0L );
        FSMnotransition( 107 );

/*
 *      HEXPARM STATE
 *
 *      This state is hit when a parameter starts with "%X", which means
 *      a hexadecimal constant.
 */

        FSMstate( "hexparm", FSM_NOSKIPBLANKS );
        FSMtransition( FSM_STRING, "_hex_", "parm", "PH" );
        FSMnotransition( 106 );

/*
 *      QUALIFIER STATE
 *
 *      This state is hit when the parameter parser encounters a slash
 *      character.  The state causes the qualifier to be signalled to 
 *      the caller.  We then transfer to the "chkval" state which tests
 *      for a value on the qualifier.  If the next token *wasn't* an
 *      identifier, then there's a syntax error.
 */

        FSMstate( "qualifier", FSM_SKIPBLANKS );
        FSMtransition( FSM_STRING, "_qname_", "chkval", "Q" );
        FSMnotransition( 102 );

/*
 *      CHKVAL STATE
 *
 *      Check for a value.  This is done by testing for an equals sign
 *      literal.  If found, we transfer to the state that checks for a
 *      list.  Otherwise, we take the "null" transition to check for
 *      another parameter or qualifier.
 */

        FSMstate( "chkval", FSM_SKIPBLANKS );
        FSMtransition( FSM_LITERAL, "!", "stop", 0L );
        FSMtransition( FSM_LITERAL, "=", "chklist", 0 );
        FSMtransition( FSM_NULL, 0L, "parm", 0L );

/*
 *      CHKLIST STATE
 *
 *      Checks to see if this is a list of values instead of a
 *      single value.  If the next token is an open parenthesis,
 *      then transfer to a state to handle lists of values.  Otherwise,
 *      we handle the possible transitions for numbers or identifiers.
 *      In either case, resume at the "parm" state when done.  If there
 *      is no transition then there was no value after the "=" character.
 */

        FSMstate( "chklist", FSM_SKIPBLANKS );
        FSMtransition( FSM_LITERAL, "(", "getlist",        "L" );
        FSMtransition( FSM_DATE, 0L, "parm", "ID" );
        FSMtransition( FSM_STRING, "_integer_", "parm",     "II" );
        FSMtransition( FSM_STRING, "_name_", "parm", "IN" );
        FSMtransition( FSM_QUOTE,  "_quotes_", "parm",     "IS" );
        FSMtransition( FSM_LITERAL, "%X", "hexvalue", 0L );
        FSMtransition( FSM_FILENAME, 0L, "parm", "IF" );
        FSMnotransition( 103 );

/*
 *      HEXVALUE STATE
 *
 *      This state is hit when a qualifier starts with "%X", which means
 *      a hexadecimal constant.
 */

        FSMstate( "hexvalue", FSM_NOSKIPBLANKS );
        FSMtransition( FSM_STRING, "_hex_", "parm", "IH" );
        FSMnotransition( 106 );


/*
 *      GETLIST STATE
 *
 *      This state handles reading values from a list.  Get a valid value
 *      from the list.  If found, transfer to the "getcomma" state which
 *      checks for delimiters.  If there is no value, then signal an error.
 */

        FSMstate( "getlist", FSM_SKIPBLANKS );
        FSMtransition( FSM_DATE, 0L, "getcomma", "ID" );
        FSMtransition( FSM_STRING, "_integer_", "getcomma",     "II" );
        FSMtransition( FSM_STRING, "_name_", "getcomma", "IN" );
        FSMtransition( FSM_QUOTE,  "_quotes_", "getcomma",     "IS" );
        FSMtransition( FSM_LITERAL, "%X", "hexlistvalue", 0L );
        FSMtransition( FSM_FILENAME, 0L, "getcomma", "IF" );
        FSMnotransition( 104 );

/*
 *      HEXLISTVALUE STATE
 *
 *      This state is hit when a value list item starts with "%X", which
 *      means a hexadecimal constant.
 */

        FSMstate( "hexlistvalue", FSM_NOSKIPBLANKS );
        FSMtransition( FSM_STRING, "_hex_", "getcomma", "IH" );
        FSMnotransition( 106 );


/*
 *      GETCOMMA STATE
 *
 *      This checks to see if we're at the end of a value list, or if there
 *      is another parameter.  If it's a closing parenthesis then the list
 *      is done and we resume searching for the next parameter or qualifier.
 *      If it's a comma, then go get another list entry.  Otherwise, it's
 *      a syntax error in the list.
 */

        FSMstate( "getcomma", FSM_SKIPBLANKS );
        FSMtransition( FSM_LITERAL, ")", "parm", 0L );
        FSMtransition( FSM_STRING, "_list_", "getlist", 0L );
        FSMnotransition( 105 );

/*
 *      This defines the name of the stop state.  Any transition to this
 *      state name will result in the FSM stopping, and returning the
 *      return code FSM_STOP.
 */

        FSMstop( "stop" );


        return FSM_NOERROR;
}
/*<PAGE>*/

/****************************************************************************\
 *                                                                          *
 *                         DCL DEFINITION ROUTINES                          *
 *                                                                          *
\****************************************************************************/

/*
 *      Store the entry name string for the current verb.
 */

static long DCLentry( char * name )
{

        current_verb-> entry = FSMgetmem( strlen( name ) + 2 );
        strcpy( current_verb-> entry, name );
	return DCL_NOERROR;
}

/*
 *      Define an alias to the current qualifier or verb.  This may
 *      be called immediately after a DCLverb() call to alias a verb,
 *      or after a DCLqualifier() call to alias a qualifier.  It links
 *      the current item to a specific existing item.  References to
 *      the alias in the grammar cause processing to occur on the aliased
 *      item.
 */

static long DCLalias( char * name )
{

        struct DCL_QUAL * q;
        struct DCL_VERB * v;

/*
 *      See if we are doing a verb, where there is no current qualifier
 *      or parameter yet.
 */

        if( current_parameter == 0L && current_qualifier == 0L ) {

                for( v = verbs; v; v = v-> next ) {
                        if( strcmp( v-> name, name ) == 0L )
                                break;
                }

                if( v == 0L ) {
                        DCLaddverb( name, &v );
                }
                current_verb-> alias = v;
                current_verb = v;
                return DCL_NOERROR;
        }

/*
 *      See if we are doing a qualifier.
 */

        if( current_qualifier != 0L ) {

                for( q = current_verb-> qualifier; q; q = q-> next ) {
                        if( strcmp( q-> name, name ) == 0L )
                                break;
                }

                if( q == 0L ) {
                        DCLgrammar_error( "can't find alias qualifier %s",
                                name );
                        return DCL_INTERNAL;
                }

                current_qualifier-> alias = q;

                /* DESIGN: do we want to switch qualifiers?  I don't */
                /* think so.  This does mean that there can be */
                /* conflicting attributes between a qualifier and it's */
                /* alias, but I need to attach defaults to the current */
                /* qualifier, etc. */

                /* current_qualifier = q; */

                return DCL_NOERROR;
        }

/*
 *      Else we were called at the wrong time!
 */

        DCLinternal_error( "no context to set alias %s", name );
        return DCL_INTERNAL;

}

/*<PAGE>*/

/*
 *      This routine is called when a sequence of changes are completed
 *      for a grammar.  It moves the current grammar definition out of
 *      the way by referencing a dummy (null) grammar.  Call this after
 *      any sequence of grammar definitions that might be interrupted.
 */

static long DCLendgrammar(void)
{

        return DCLgrammar( "$$$NULL$$$" );
}

/*<PAGE>*/

/*
 *      Create a new grammar header.  These are used to set up the
 *      list roots for a specific grammar loaded into memory.
 */

static long DCLgrammar( char * name )
{
        char buff[ 64 ];
        int len;
        long size;
        long new_grammar;
        struct GRAMMAR * g;

/*
 *      Make an up-cased copy of the name.
 */

        len = (int) strlen( name );
        DCLupcase( name, buff );

        DCLdebug( "switch to grammar", buff );

/*
 *      If there's a current grammar active, save the current state
 *      of affairs for that grammar again.
 */

        if( current_grammar != 0L ) {

                current_grammar-> verbs = verbs;
                current_grammar-> the_types = the_types;
                current_grammar-> typenames = typenames;
                current_grammar-> current_verb = current_verb;
                current_grammar-> current_parameter = current_parameter;
                current_grammar-> current_qualifier = current_qualifier;
                current_grammar-> current_typenames = current_typenames;
                current_grammar-> disallow = disallow;
        }

/*
 *      Search for the named grammar.
 */

        for( g = grammars; g; g = g-> next ) {
                if( strcmp( g-> name, buff ) == 0 ) {
                        break;
                }
        }


/*
 *      If it wasn't found, then we need to create a new grammar.
 */

        if( g == 0L ) {

                size = sizeof( struct GRAMMAR ) + strlen( name );
                g = ( struct GRAMMAR * ) FSMgetmem( size );

                g-> next = grammars;
                grammars = g;

                strcpy( g-> name, buff );
                g-> verbs = 0L;
                g-> current_verb = 0L;
                g-> typenames = 0L;
                g-> current_typenames = 0L;
                g-> current_parameter = 0L;
                g-> current_qualifier = 0L;
                g-> disallow = 0L;
                g-> valid = 0;
                g-> the_types = 0L;
                new_grammar = 1;
        }
        else
                new_grammar = 0;

/*
 *      Set the current state to the (possibly newly defined) new grammar.
 */

        current_grammar = g;
        verbs = g-> verbs;
        typenames = g-> typenames;
        current_verb = g-> current_verb;
        current_parameter = g-> current_parameter;
        current_qualifier = g-> current_qualifier;
        current_typenames = g-> current_typenames;
        the_types = g-> the_types;
        disallow = g-> disallow;

/*
 *      If this is a new grammar, create the predefined types for this
 *      grammar (shared by all grammars we generate).
 */

        if( new_grammar ) {
                DCLkeylist( "$$$TYPES" );
                DCLkeyword( "$ANY",         DCL_ANY,        DCL_NONEGATE, 0L );
                DCLkeyword( "$NAME",        DCL_NAME,       DCL_NONEGATE, 0L );
                DCLkeyword( "$STRING",      DCL_STRING,     DCL_NONEGATE, 0L );
                DCLkeyword( "$INTEGER",     DCL_INTEGER,    DCL_NONEGATE, 0L );
                DCLkeyword( "$FILENAME",    DCL_FILENAME,   DCL_NONEGATE, 0L );
                DCLkeyword( "$DATETIME",    DCL_DATE,       DCL_NONEGATE, 0L );
                DCLkeyword( "$REST_OF_LINE",DCL_RESTOFLINE, DCL_NONEGATE, 0L );
        }

        return DCL_NOERROR;
}


/*<PAGE>*/
/*<PAGE>*/

/*
 *      Declare a verb.  This can be followed by DCLparameter() and
 *      DCLqualifier() declarations as needed.
 */

static long DCLverb( char * name, long id )
{
        long size;
        struct DCL_VERB * v;


        DCLdebug( "defining verb", name );

        if( DCLcheckid( "verb", name, id ) != DCL_NOERROR )
                return DCL_INTERNAL;


        /* See if there is already a verb of this name. */

        for( v = verbs; v; v = v-> next )
                if( strcmp( name, v-> name ) == 0 )
                        break;

        /* If it existed, it must be undefined or boo-boo. */

        if( v ) {
                if( v-> id != DCL_UNDEFINED ) {
                        DCLgrammar_error( "duplicate verb/syntax definition for %s",
                                        name );
                        return DCL_INTERNAL;
                }
        }
        else {
                size = sizeof( struct DCL_VERB ) + strlen( name );
                v = ( struct DCL_VERB * ) FSMgetmem( size );

                v-> next = verbs;
                verbs = v;
        }

        /* Define the new current state based on new verb creation */

        current_verb = v;
        current_qualifier = 0L;
        current_parameter = 0L;

        /* Fill in the verb info... */

        strcpy( v-> name, name );
        v-> id = id;
        v-> qualifier = 0L;
        v-> routine = 0L;
        v-> parameter = 0L;
        v-> nextparm = 0L;
        v-> alias = 0L;
        v-> entry = 0L;
        v-> state = DCL_ABSENT;
        return DCL_NOERROR;

}
/*<PAGE>*/

/*
 *      Declare a qualifier.  This call can be followed by DCLkeyword()
 *      declarations bounds to the current qualifier.
 */


static long DCLqualifier( char * name, long id, int kind, int flags )
{

        struct DCL_QUAL * q;
        long size;


        DCLdebug( "defining qualifier", name );

        if( DCLcheckid( "qualifier", name, id ) != DCL_NOERROR ) 
                return DCL_INTERNAL;


        current_parameter = 0L;
        current_typenames = 0L;

        size = sizeof( struct DCL_QUAL ) + strlen( name );
        q = ( struct DCL_QUAL * ) FSMgetmem( size );

        q-> next = current_verb-> qualifier;
        current_verb-> qualifier = q;
        current_qualifier = q;
        q-> id = id;
        q-> kind = kind;
        q-> value = 0L;
        q-> defvalue = 0L;
        q-> keywords = 0L;
        q-> flags = flags;
        q-> alias = 0L;
        q-> state = DCL_ABSENT;
        q-> min = DCL_MINDEF;
        q-> max = DCL_MAXDEF;
        q-> local = 0L;

        strcpy( q-> name, name );
        return DCL_NOERROR;

}
/*<PAGE>*/

/*
 *      Declare that the most-recently assembled qualifier is bound to a
 *      particular parameter, by name.
 */


static long DCLlocal( char * name )
{

        struct DCL_PARM * p;


/*
 *      Make sure we are currently defining a qualifier...
 */

        if( !current_qualifier || current_parameter ) {
                DCLinternal_error( "no current qualifier to set LOCAL state", "" );
                return DCL_INTERNAL;
        }

/*
 *      Search for a parameter with the given name...
 */

        for( p = current_verb-> parameter; p; p = p-> next )
                if( strcmp( p-> name, name ) == 0 )
                        break;

        if( p == 0L ) {
                DCLgrammar_error( "no parameter %s to make /LOCAL_TO", name );
                return DCL_INTERNAL;
        }

/*
 *      Link the qualifier to the parameter.
 */

        current_qualifier-> flags |= DCL_LOCAL;
        current_qualifier-> local = p;

        return DCL_NOERROR;

}

/*<PAGE>*/

static long DCLdisallow( char * qual1, char * qual2 )
{
        char q1[ 32 ], q2[ 32 ];
        struct DCL_QUAL * qp1, * qp2, *qlist;
        struct DISALLOW * d;
        long state1, state2;


        if( current_verb == 0L ) {
                DCLinternal_error( "no current verb for DISALLOW context","" );
                return DCL_INTERNAL;
        }

        DCLupcase( qual1, q1 );
        DCLupcase( qual2, q2 );
        DCLdebug( "disallow qualifier 1 ", q1 );
        DCLdebug( "disallow qualifier 2 ", q2 );

        /*      Search for the qualifiers       */

        qlist = current_verb-> qualifier;

        state1 = DCL_PRESENT;
        qp1 = DCLqualsearch( qlist, q1 );
        if( strncmp( q1, "NO", 2 ) == 0 && qp1 == 0L ) {
                state1 = DCL_NEGATED;
                qp1 = DCLqualsearch( qlist, &( q1[ 2 ]));
        }
        if( qp1 == 0L ) {
                DCLgrammar_error( "DISALLOW can't find qualifier %s", q1 );
                return DCL_INTERNAL;
        }

        state2 = DCL_PRESENT;
        qp2 = DCLqualsearch( qlist, q2 );
        if( strncmp( q2, "NO", 2 ) == 0 && qp2 == 0L ) {
                state2 = DCL_NEGATED;
                qp2 = DCLqualsearch( qlist, &( q2[ 2 ]));
        }
        if( qp2 == 0L ) {
                DCLgrammar_error( "DISALLOW can't find qualifier %s", q2 );
                return DCL_INTERNAL;
        }

        d = ( struct DISALLOW * ) FSMgetmem( sizeof( struct DISALLOW ));
        d-> next = current_verb-> disallow;
        current_verb-> disallow = d;
        d-> state1 = ( char ) state1;
        d-> qual1 = qp1;
        d-> state2 = ( char ) state2;
        d-> qual2 = qp2;
        return DCL_NOERROR;
}

/*<PAGE>*/
/*
 *      Set up a default value for the current qualifier or parameter.
 */

static long DCLdefault( long typ, long i_value, char * c_value )
{
        struct DCL_VALUE * v;
        long size;

        if( current_parameter == 0L && current_qualifier == 0L ) {
                DCLinternal_error( "no current context to set default", "" );
                return DCL_INTERNAL;
        }

        if( current_qualifier ) {
                if( current_qualifier-> defvalue ) {
                        DCLinternal_error( "qualifier %s already has default value",
                                current_qualifier-> name );
                        return DCL_INTERNAL;
                }
        }
        else {
                if( current_parameter-> defvalue ) {
                        DCLinternal_error( "parameter %s already has default value",
                                current_parameter-> name );
                        return DCL_INTERNAL;
                }
        }

        if( typ == DCL_DEFWITHVALUE ) {
                size = sizeof( struct DCL_VALUE ) + strlen( c_value );
                v = ( struct DCL_VALUE * ) FSMgetmem( size );
        
                v-> next = 0L;
                v-> id = 0;
                v-> state = 0;
                v-> kind = 0;
                v-> i_value = i_value;
                strcpy( v-> c_value, c_value );
        }
        else
                v = 0L;

        if( current_qualifier ) {
                current_qualifier-> flags |= DCL_DEFAULT;
                current_qualifier-> defvalue = v;
        }
        else {
                current_parameter-> flags |= DCL_DEFAULT;
                current_parameter-> defvalue = v;
        }

        return DCL_NOERROR;
}
/*<PAGE>*/

/*
 *      Set qualifier minimum and maximum values.
 */

static long DCLqual_min( long min )
{

        if( current_qualifier == 0L ) {
                DCLinternal_error( "no current qualifier to set min","" );
                return DCL_INTERNAL;
        }

        if( current_qualifier-> kind != DCL_INTEGER ) {
                DCLinternal_error( "can't set minimum on non-INTEGER qualifier %s",
                        current_qualifier-> name );
                return DCL_INTERNAL;
        }

        current_qualifier-> min = min;
        return DCL_NOERROR;
}


static long DCLqual_max( long max )
{

        if( current_qualifier == 0L ) {
                DCLinternal_error( "no current qualifier to set max","" );
                return DCL_INTERNAL;
        }

        if( current_qualifier-> kind != DCL_INTEGER ) {
                DCLinternal_error( "can't set maximum on non-INTEGER qualifier %s",
                        current_qualifier-> name );
                return DCL_INTERNAL;
        }

        current_qualifier-> max = max;
        return DCL_NOERROR;
}

/*<PAGE>*/

/*
 *      Declare a syntax name, bound to the current qualifier.
 */

static long DCLsyntax( char * name )
{
        char buff[ 64 ];
        struct DCL_VERB * v;

        strcpy( buff, "SYNTAX$" );
        strcat( buff, name );

        if( current_qualifier == 0L ) {
                DCLinternal_error( "no current qualifier to bind syntax to", "" );
                return DCL_INTERNAL;
        }

        if( current_qualifier-> syntax ) {
                DCLinternal_error( "duplicate syntax binding for %s",
                        current_qualifier-> name );
                return DCL_INTERNAL;
        }

        for( v = verbs; v; v = v-> next ) {
                if( strcmp( v-> name, buff ) == 0 )
                        break;
        }

        if( v == 0L ) {
                DCLaddverb( buff, &v );
        }

        current_qualifier-> syntax = v;
        return DCL_NOERROR;
}
/*<PAGE>*/

/*
 *      Declare a parameter.  This call can be followed by
 *      DCLkeyword() declarations bound to the current parameter.
 */

static long DCLparameter( char * name, long id, int kind, int flags )
{

        struct DCL_PARM * p;
        long size;

        DCLdebug( "defining parameter", name );

        if( DCLcheckid( "parameter", name, id ) != DCL_NOERROR )
                return DCL_INTERNAL;


        current_qualifier= 0L;
        current_typenames = 0L;

        size = sizeof( struct DCL_PARM ) + strlen( name );
        p = ( struct DCL_PARM * ) FSMgetmem( size );

        /* Link it onto the list.  If the list is empty, make it */
        /* the first entry.  Else link it to the last parameter  */
        /* we created.                                           */

        if( current_verb-> nextparm == 0L )
                current_verb-> parameter = p;
        else
                current_verb-> nextparm-> next = p;

        /* Now identify this on as the "last one we created".   */

        current_verb-> nextparm = p;

        p-> next = 0;
        current_parameter = p;
        p-> id = id;
        p-> kind = kind;
        p-> c_value = 0L;
        p-> i_value = 0;
        p-> keywords = 0L;
        p-> flags = flags;
        p-> prompt = 0L;
        p-> state = DCL_ABSENT;
        strcpy( p-> name, name );
        return DCL_NOERROR;

}
/*<PAGE>*/

/*
 *      Set a prompt string for the current parameter.
 */

static long DCLprompt( char * p )
{
        long size;

        if( current_parameter == 0L ) {
                DCLinternal_error( "no context to set prompt string","" );
                return DCL_INTERNAL;
        }

        size = strlen( p ) + 1;
        current_parameter-> prompt = FSMgetmem( size );
        current_parameter-> flags |= DCL_REQ;
        strcpy( current_parameter-> prompt, p );
        return DCL_NOERROR;
}

        
/*<PAGE>*/

/*
 *      Define a new keyword list to add stuff to.
 */

static long DCLkeylist( char * name )
{

        long size;
        struct DCL_TYPE * k;

        /* Search for matching key list to see if it already exists */
        /* and we are just adding to it... */

        for( k = typenames; k; k = k-> next ) 
                if( strcmp( name, k-> name ) == 0 ) 
                        break;

        if( k == 0L ) {
                size = sizeof( struct DCL_TYPE ) + strlen( name );
                k = ( struct DCL_TYPE * ) FSMgetmem( size );
                k-> next = typenames;
                typenames = k;
                k-> keylist = 0L;
                strcpy( k-> name, name );
        }

        current_typenames = k;
        return DCL_NOERROR;
}
/*<PAGE>*/

/*
 *      Specify a keyword list to bind to the current parameter or
 *      qualifier being defined.
 */

long DCLusekeylist( char * name )
{

        struct DCL_TYPE * k;

/*
 *      Find the keylist.
 */

        for( k = typenames; k; k = k-> next ) 
                if( strcmp( name, k-> name ) == 0 )
                        break;

/*
 *      If not found, then create a new one.  Note that we assume that
 *      keylists are added to the head of the list (simple insertion)
 *      so after creating the new list we can just grab it from the
 *      listhead.
 */

        if( k == 0L ) {
                DCLkeylist( name );
                k = typenames;
        }

/*
 *      Make sure there is a current parameter or qualifier defined
 *      to attach the keyword list to.
 */

        if( current_parameter == 0L && current_qualifier == 0L ) {
                DCLinternal_error( "keylist %s used without context", name );
                return DCL_INTERNAL;
        }

        if( current_parameter )
                current_parameter-> keywords = k-> keylist;
        else
                current_qualifier-> keywords = k-> keylist;

        return DCL_NOERROR;
}

/*<PAGE>*/

/*
 *      This stores keywords away bound to the current keyword list
 *      being defined.
 */

static long DCLkeyword( char * name, long id, int flags, char * Syntax )
{
        long size;
        char * syntax;
        struct DCL_KEYWORDS * k;
        struct DCL_VERB * v;
        char buff[ 80 ];

        syntax = Syntax;
        if( Syntax != 0L )
                if( Syntax[ 0 ] == 0 )
                        syntax = 0L;

        DCLdebug( "defining keyword", name );

        if( DCLcheckid( "keyword", name, id ) != DCL_NOERROR )
                return DCL_INTERNAL;

        if( syntax ) {
                strcpy( buff, "SYNTAX$" );
                strcat( buff, syntax );
                for( v = verbs; v; v = v-> next )
                        if( strcmp( v-> name, buff ) == 0 ) 
                                break;
                if( v == 0L ) {
                        DCLaddverb( buff, &v );
                }
        }
        else
                v = 0L;

        /* Make sure it isn't a duplicate... */

        for( k = current_typenames-> keylist; k; k = k-> next ) {
                if( strcmp( name, k-> name ) == 0 ) {
                        DCLgrammar_error( "duplicate keyword definition for %s",
                                name );
                        return DCL_INTERNAL;
                }
        }

        /* Create a new entry... */

        size = sizeof( struct DCL_KEYWORDS ) + strlen( name );
        k = ( struct DCL_KEYWORDS * ) FSMgetmem( size );

        if( current_typenames == 0L ) {
                DCLgrammar_error( "keyword %s defined with no context",
                        name );
                return DCL_INTERNAL;
        }

        k-> next = current_typenames-> keylist;
        current_typenames-> keylist = k;

        strcpy( k-> name, name );
        k-> id = id;
        k-> flags = flags;
        k-> syntax = v;

        return DCL_NOERROR;
}

/*<PAGE>*/

/*
 *      Debugging routine that formats a return code.
 */

long DCLdebugrc( char * kind, long rc ) 
{

        int i;
        char buff[ 64 ];

        static struct Errors {
                long    rc;
                char    *msg;
        } errors[] = {
        {       FSM_ERROR,      "FSM error"                             },
        {       FSM_NEEDMORE,   "need more input text"                  },
        {       FSM_ACTION,     "need to execute FSM action"            },
        {       FSM_STOP,       "grammer successfully completed"        },
        {       FSM_NOTRANS,    "FSM error, no transition found"        },
        {       DCL_NOERROR,    "no error"                              },
        {       DCL_IVERB,      "unknown verb"                          },
        {       DCL_UNKNOWN,    "unknown or ambigious keyword"          },
        {       DCL_TYPERR,     "value is wrong type"                   },
        {       DCL_SYNTAX,     "DCL syntax error"                      },
        {       DCL_MISSPARM,   "missing required parameter value"      },
        {       DCL_MISSQUAL,   "missing required qualifier value"      },
        {       DCL_AMBKEY,     "ambigious keyword"                     },
        {       DCL_CANTNEG,    "cannot negate qualifier"               },
        {       DCL_INTERNAL,   "DCL internal programming error"        },
        {       DCL_INVCOMB,    "invalid combination of qualifiers"     },
        {       0,              ""                                      }};

        if( dcl_debug_state != DCL_DEBUG ) 
                return DCL_NOERROR;

        sprintf( buff, "return code from %s:", kind );
        for( i = 0; errors[ i ].msg[ 0 ]; i++ ) {

                if( errors[ i ].rc == rc ) {
                        DCLdebug( buff, errors[ i ].msg );
                        return DCL_NOERROR;
                }
        }

        DCLdebugn( buff, rc );
        return DCL_NOERROR;
}


/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                     TYPE/DATE CONVERSION ROUTINES                        *
 *                                                                          *
\****************************************************************************/

/*
 *      This looks in the 'dclargblk' structure that has been parsed, and
 *      handles the case where /TYPE= can be a predefined or user-defined
 *      name.  We search for the known types which are defined by the type
 *      of "TYPES".  If it's found there, we are done.  If not, then it must
 *      be a user-defined list and we must handle that elsewhere.
 */

static long DCLresolvetype(void)
{

        struct DCL_TYPE * t;
        long kind;
        char * p;

        /* If there's no type name here, then we're done... */

        if( dcl_private_store.keys[ 0 ] == 0 ) {
                dcl_private_store.typ = DCL_NONE;
                return DCL_NOERROR;
        }

        /* First, locate the list of type keywords we accept.  These are */
        /* bound to the "$$$DCL$$$" grammar, so switch there before doing */
        /* the search... */

        if( the_types == 0L ) {
                DCLdebug( "locating type keyword list", "" );
                p = current_grammar-> name;
                for( t = typenames; t; t = t-> next ) {
                        if( strcmp( t-> name, "$$$TYPES" ) == 0 )
                                break;
                }
                if( t == 0L ) {
                        DCLinternal_error( "required TYPE of '$$$TYPES' never defined", "" );
                        return DCL_INTERNAL;
                }
                the_types = t;
        }

        /* See if the key type given is known to us already */

        DCLdebug( "searching for type keyword ", dcl_private_store.keys );
        kind = DCLkeysearch( the_types-> keylist, dcl_private_store.keys, 0L );

        if( kind > 0 ) {

                dcl_private_store.typ = (int) kind;
                dcl_private_store.keys[ 0 ] = 0;
                return DCL_NOERROR;
        }

        /* It's not a known type, so indicate that it must be a keyword */
        /* type that the user provides.                                 */

        dcl_private_store.typ = DCL_KEYWORD;
        return DCL_NOERROR;
}

/*<PAGE>*/

/*
 *      Convert a string representation of an integer to a real longword.
 *      Accepts standard signed integers, and also handles multiplier
 *      suffixes like K, as in 8K for 8192.
 */

static long DCLatoi( char * token, long * n, long flags )
{

        int len;
        char ch;
        long m, v, k;
        char * p;
        char pbuff[ 32 ];
        
        *n = 0;
        len = (int) strlen( token );
        if( len == 0 ) 
            return DCL_NOERROR;
            
        ch = token[ len - 1 ];
        if( islower( ch )) 
                ch = ( char ) toupper( ch );

        if( FSMinlist( ch, "KMG" ) && !( flags & DCL_ALLOWSUFFIX )) {
                DCLerror( "invalid integer %s", token );
                return DCL_SYNTAX;
        }


        switch( ch ) {

        case 'K':
                m = 1024;
                break;

        case 'M':
                m = 1024000;
                break;

        case 'G':
                m = 1024000000;
                break;

        default:
                m = 1;
                break;

        }

        if( m > 1 ) {
                strcpy( pbuff, token );
                pbuff[ len - 1 ] = 0;
                p = pbuff;
        }
        else {
                p = token;
        }

/*
 *      Use actual ascii-to-integer converter.  This *DEPENDS* on using
 *      ASCII collating sequences!!!
 */

        v = 0;

        len = (int) strlen( p );
        for( k = 0; k < len; k++ ) {
                if( !FSMinlist( p[ k ], "-0123456789" )) {
                        DCLerror( "invalid integer %s", token );
                        return DCL_SYNTAX;
                }

                if( p[ k ] == '-' )
                        m = -m;
                else
                        v = ( v * 10 ) + ( p[ k ] - '0' );
        }
        *n = v * m;
        return DCL_NOERROR;

}
/*<PAGE>*/

/*
 *      Given a pointer to an input buffer, move it to the output buffer
 *      and up-case all characters in the string not enclosed in quotes.
 */

static long DCLupcase( char * In, char * Out )
{
        char * in, * out;
        long    in_quote;

        in_quote = 0;
        for( in = In, out = Out; *in; in++,out++ ) {
                if( *in == '"' )
                        in_quote = 1 - in_quote;
                if( in_quote )
                        *out = *in;
                else
                        *out = ( char ) ( ( islower( *in )) ? toupper( *in ) : *in );
        }
        *out = 0;
        return DCL_NOERROR;
}
/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                      CONSISTENCY CHECK ROUTINES                          *
 *                                                                          *
\****************************************************************************/

/*
 *      Check to see if an id number is valid.  It must be positive or
 *      one of the predefined values for undefined or generated id's.
 */

static long DCLcheckid( char * kind, char * name, long id )
{

        char buff[ 80 ];

        if( id == DCL_UNDEFINED || id == DCL_NO_ID || id > 0 )
                return DCL_NOERROR;

        strcpy( buff, "non-positive id for " );
        strcat( buff, kind );
        strcat( buff, " %s" );

        DCLinternal_error( buff, name );
        return DCL_INTERNAL;
}
/*<PAGE>*/
/*
 *      For the current verb, scans over the list and determines if
 *      any of the qualifiers or parameters are required but were
 *      not found.
 */

static long DCLcheck_requirements( char * name )
{

        struct DCL_PARM * p;
        struct DCL_QUAL * q;
        struct DCL_VALUE * v;
        struct DISALLOW * d;

        long size, sts;

        if( current_verb == 0L )
                return DCL_NOERROR;

        /* Scan to see if there are missing parameters */

        for( p = current_verb-> parameter; p; p = p-> next ) {
                if(( p-> flags & DCL_REQ ) && ( p-> state == DCL_ABSENT )
                  && ( p-> prompt != 0L )) {
                        sts = DCLpromptparm( name, p );
                        if( sts != DCL_NOERROR )
                                return sts;
                }

                if(( p-> flags & DCL_REQ ) && ( p-> state == DCL_ABSENT )) {
                        DCLerror( "required parameter %s not found", 
                                p-> name );
                        return DCL_MISSPARM;
                }
        }

        /* Handle default qualifiers */

        for( q = current_verb-> qualifier; q; q = q-> next ) {

                /* If not aliased, and not set, but has a default */
                /* value then mark as present instead. */

                if( q-> alias == 0L && 
                    q-> state == DCL_ABSENT && 
                    q-> flags & DCL_DEFAULT ) {
                        DCLdebug( "default qualifier marked PRESENT: ", 
                                q-> name );
                        q-> state = DCL_PRESENT;
                }

                /* If it doesn't have a value, but is present and there */
                /* is a default, then move the default to the value list */

                if( q-> value == 0L && 
                    q-> state == DCL_PRESENT && 
                    q-> defvalue) {
                        DCLdebug( "default value set for qualifier",
                                        q-> name );
                        size = sizeof( struct DCL_VALUE ) +
                                        strlen( q-> defvalue-> c_value );
                        v = ( struct DCL_VALUE * ) FSMgetmem( size );
                        v-> next = 0;
                        v-> id = q-> id;
                        v-> kind = q-> kind;
                        v-> state = DCL_PRESENT;
                        v-> i_value = q-> defvalue-> i_value;
                        strcpy( v-> c_value, q-> defvalue-> c_value );
                        q-> value = v;
                        q-> flags |= DCL_HASVALUE;
                }

                /* If an alias temporarily attached a default, remove */
                /* the default (leaving value used this time, if any) */

                if( q-> flags & DCL_TMPDEFAULT ) {
                        DCLdebug( "removing temporary default from ",
                                q-> name );
                        q-> flags &= ~DCL_TMPDEFAULT;
                        q-> defvalue = 0L;
                }

                /* If it's required to have a value and there isn't one */
                /* and the qualifier is marked present, then there was no */
                /* default and no value and so it's a boo-boo by the user */

                if(( q-> flags & DCL_REQ ) && 
                   !( q-> flags & DCL_HASVALUE ) &&
                   ( q-> state == DCL_PRESENT )) {

                        DCLerror( "required value for qualifier %s not found", 
                                        q-> name );
                        return DCL_MISSQUAL;
                }

        }

        /* Check for illegal combinations of qualifiers */

        for( d = current_verb-> disallow; d; d = d-> next ) {

                if( d-> qual1-> state == d-> state1 &&
                    d-> qual2-> state == d-> state2 ) {
                        DCLerror( "invalid combination of qualifiers specified",
                                "" );
                        return DCL_INVCOMB;
                }
        }

        return DCL_NOERROR;

}
/*<PAGE>*/

/*
 *      For the current verb, scans over the list and resets the
 *      "required" flag to indicate that the value hasn't been
 *      seen.
 */

static long DCLresetreq(void)
{

        struct DCL_PARM * p;
        struct DCL_QUAL * q;

        for( p = current_verb-> parameter; p; p = p-> next ) {
                p-> state = DCL_ABSENT;
        }

        for( q = current_verb-> qualifier; q; q = q-> next ) {
                q-> state = DCL_ABSENT;
        }

        return DCL_NOERROR;

}
/*<PAGE>*/
/*
 *      Search to see if there are any parameters or qualifiers attached
 *      to the current verb that are being discarded.  This is significant
 *      because we are in the middle of a syntax change, which will discard
 *      these values.  Tell the user...
 */

static long DCLdiscard( char * name )
{

        struct DCL_QUAL * q;
        struct DCL_PARM * p;
        struct DCL_VALUE * d, *dn;
        int msg;

        msg = 0;

        /* Search the parameters... */

        for( p = current_verb-> parameter; p; p = p-> next ) {
                if( p-> state != DCL_ABSENT ) {
                        DCLdebug( "discarding parameter", p-> name );
                        msg = 1;
                        p-> state = DCL_ABSENT;
                        if( p-> c_value ) {
                                FSMfreemem( p-> c_value );
                                p-> c_value = 0;
                        }
                }
        }

        /* Search the qualifiers */

        for( q = current_verb-> qualifier; q; q = q-> next ) {
                if( q-> state != DCL_ABSENT ) {
                        DCLdebug( "discarding qualifier", q-> name );
                        msg = 1;
                        q-> state = DCL_ABSENT;
                        q-> flags &= ~DCL_HASVALUE;
                        for( d = q-> value; d; d = dn ) {
                                dn = d-> next;
                                FSMfreemem( d );
                        }
                        q-> value = 0L;
                }
        }

        if( msg ) {
                DCLerror( 
                   "qualifiers and parameters specified before %s are ignored", 
                    name );
        }
        return DCL_NOERROR;
}
/*<PAGE>*/

/*
 *      Routine called to create a verb when one doesn't already exist.
 */

static long DCLaddverb( char * name, struct DCL_VERB ** verb )
{
        long sts;
        struct DCL_VERB * v;
        struct DCL_QUAL * q;
        struct DCL_PARM * p;

        v = current_verb;
        q = current_qualifier;
        p = current_parameter;

        sts = DCLverb( name, DCL_UNDEFINED );
        current_verb-> state = ( char ) DCL_UNDEFINED;
        *verb = current_verb;

        current_verb = v;
        current_qualifier = q;
        current_parameter = p;

        return sts;
}

/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                  GRAMMAR DESCRIPTION SEARCH ROUTINES                     *
 *                                                                          *
\****************************************************************************/


/*
 *      Given a list of keywords and a token, see if the token is
 *      an un-ambigious keyword from the list.  The search also
 *      allows for the prefix "NO" infront of the keyword as long
 *      as it isn't ambigious and the keyword permits negated forms.
 *
 *      Returns the keyword ID as it's result.  If the keyword was
 *      negated, it returns the negative of the id.
 *
 *      Also returns the full name of the token found in the third
 *      parameter.  If this parameter is passed as 0L (null pointer)
 *      then not only is the name not returned, but the routine does
 *      not signal an error when it can't find a match!  This is
 *      used internally to search for predefined keywords versus user
 *      defined keywords in the /TYPE= qualifier in the grammar.
 */


static long DCLkeysearch( struct DCL_KEYWORDS * keywords, 
                          char * token, 
                          char ** tp )
{
        struct DCL_KEYWORDS * k, *last;
        int found, fcount, len, neg, len2;
        char * np;
        len = (int) strlen( token );
        neg = strncmp( token, "NO", 2 ) == 0 ? 1 : 0;
        np = token + 2;
        len2 = len - 2;

        found = 0;
        last = 0;
        fcount = 0;
        
        len = (int) strlen( token );
        for( k = keywords; k; k = k-> next ) {
                if( strncmp( k-> name, token, len ) == 0 ) {

                        if( strcmp( k-> name, token ) == 0 ) {
                                if( tp )
                                        *tp = k-> name;
                                found =  (int) k-> id;
                                last = k;
                                fcount = 1;
                                break;
                        }
                        fcount++;
                        if( tp )
                                *tp = k-> name;
                        found = (int) k-> id;
                        last = k;
                }
                if( neg && len > 2 && ( strncmp( k-> name, np, len - 2 ) == 0 )) {
                        if( k-> flags & DCL_NONEGATE ) {
                                DCLerror( "cannot negate keyword \"%s\"",
                                        k-> name );
                                return 0;
                        }

                        if( strcmp( k-> name, np ) == 0 ) {
                                if( tp )
                                        *tp = k-> name;
                                found =(int)  -( k-> id );
                                last = k;
                                fcount = 1;
                                break;
                        }

                        fcount++;
                        if( tp )
                                *tp = k-> name;
                        found = (int)  -( k-> id );
                        last = k;
                }

        }

        if(( tp ) && ( fcount != 1 )) {
                *tp = 0L;
                DCLerror( fcount == 0 ?
                        "unrecognized keyword \"%s\"" :
                        "ambigious keyword \"%s\"", token );
                return 0L;
        }

        if( found && last-> syntax ) {
                DCLdebug( "switching to syntax", last-> syntax-> name );
                DCLdiscard( last-> name );
                current_verb-> state = DCL_ABSENT;
                current_verb = last-> syntax;
                current_verb-> nextparm = current_verb-> parameter;
                DCLresetreq();
                current_verb-> state = DCL_PRESENT;
        }

        return found;
}
/*<PAGE>*/

/*
 *      Match a token name with the associated qualifier that it
 *      refers to.  Requires the they token be unambigious and
 *      a valid subset of the qualifier name.  Permits the prefix
 *      "NO" to mean a negated qualifier.
 *
 *      Returns the structure pointing to the qualifier if it's
 *      found.  Sets the state value in the structure to indicate
 *      DCL_PRESENT or DCL_NEGATED.
 *
 */

static struct DCL_QUAL * DCLqualsearch( struct DCL_QUAL * quals, char * token )
{
        struct DCL_QUAL * q, *found;
        int len;

        found = 0L;
        len = (int) strlen( token );
        for( q = quals; q; q = q-> next ) {

        /* If there's a current parameter defined, and it doesn't match the local */
        /* root of this qualifier, then skip it... */

                if( current_verb && q-> local && q-> local != current_verb-> lastparm )
                        continue;

                if( strncmp( q-> name, token, len ) == 0 ) {

                        if( strcmp( q-> name, token ) == 0 ) {
                                found = q;
                                break;
                        }
                        if( found ) {
                                return 0L;
                        }
                        found = q;
                }
        }

        /* If there wasn't one found, let the caller know... */

        if( found == 0L )
                return found;

        /* If this qualifier is aliased, switch to the real qualifier */

        while( found-> alias ) {
                if( found-> defvalue ) {
                        found-> alias-> flags |= DCL_TMPDEFAULT;
                        found-> alias-> defvalue = found-> defvalue;
                }
                found-> state = DCL_ABSENT;
                found = found-> alias;
                DCLdebug( "alias selected for qualifier", found-> name );
        }

        /* If this qualifier triggers a syntax change, handle that now */

        if( found-> syntax != 0L ) {
                DCLdebug( "switching to syntax", found-> syntax-> name );
                DCLdiscard( found-> name );
                current_verb-> state = DCL_ABSENT;
                current_verb = found-> syntax;
                current_verb-> nextparm = current_verb-> parameter;
                DCLresetreq();
                current_verb-> state = DCL_PRESENT;
        }

        return found;
}
/*<PAGE>*/

/****************************************************************************\
 *                                                                          *
 *                      GRAMMAR PARSING ACTION ROUTINES                     *
 *                                                                          *
\****************************************************************************/

/*
 *      This is called when a parameter is missing and we can try to prompt
 *      for more data.  We actually restart the FSM using the string the
 *      user gave us, with the "parm" state.  Additional parameters,
 *      qualifiers, etc. can be parsed at this point.
 */

static long DCLpromptparm( char * gname, struct DCL_PARM * parm )
{



        char * p;
        char * s;
        char * state;
        char * arg;
        int rc, qflag;
        char * token;
        int toklen, kind;
        // static int FSM_defined = 0;
        int running;

        char tbuff[ 256 ];
        char dbuff[ 256 ];
        char pbuff[ 80 ];

/*
 *      Set up the context showing we are working on the current
 *      verb and that we are working on a specific parameter.
 */

        if( parm-> prompt == 0L )
                return DCL_NOERROR;
        strcpy( pbuff, "$_" );
        strcat( pbuff, parm-> prompt );
        strcat( pbuff, ": " );

        current_verb-> nextparm = parm;

        DCLgrammar( gname );

/*
 *      Try to use the callback to get more data.  There is no
 *      "current buffer".
 */

        rc = (int) FSMgetstring( pbuff, dbuff );
        if( rc == DCL_EOF )
                return rc;


/*
 *      Upcase anything not in quotation marks.
 */

        qflag = 0;
        DCLupcase( dbuff, dbuff );
        p = dbuff;
        s = p;

/*
 *      Define the initial state for the machine to run with.  This sets
 *      the "current" state to the named state.
 */

        FSMinitial_state( "parm" );


/*
 *      Run the machine, in a loop.  We pass a buffer to the data buffer
 *      to scan, and a pointer to a char * variable.  This is set to the
 *      name of the state we're in when we finish, if the action routine
 *      needs to know the current state.  We also get a pointer to the
 *      argument data if there is any.  Finally, the last parsed token
 *      and it's length are returned to us.  This is a pointer in to the
 *      data area that "s" points to.  Note that after a normal call, the
 *      value of "s" is updated to point to the next character to parse,
 *      and "token" points to an area of memory _before_ "s" where the
 *      last token came from.  The value of "toklen" is typically the
 *      difference between "token" and "s", though this is not guaranteed.
 */

		running = 1;
        while( running ) {

        /*
         *      If the next parameter is "REST OF LINE" then allow the
         *      FSM to accept parm state transitions that eat the rest
         *      of the command line. 
         */

                if( current_verb && current_verb-> nextparm ) {
                        kind = current_verb-> nextparm-> kind;
                        fsm_allow_rest = kind == DCL_RESTOFLINE ? 1 : 0;
                }

        /*
         *      Run the FSM until it finds another token of some kind.
         */

                rc = (int) FSMrun( &s, &state, &arg, &token, &toklen );

        /*
         *      Reset the flag that allows REST_OF_LINE processing so there
         *      can be no ambiguity if we re-entrantly execute the FSM.
         */

                fsm_allow_rest = 0;

        /*
         *      If the FSM needs more input, i.e. the string is empty
         *      but the "stop" state hasn't been hit, then the command
         *      is incomplete.
         */

                if( rc == FSM_NEEDMORE ) {
                        rc = (int) FSMgetstring( pbuff, dbuff );
                        if( rc == DCL_EOF )
                                break;
                        p = dbuff;
                        s = p;
                        continue;
                }

        /*
         *      A transition occurred with a non-zero action string.  The
         *      action string is stored in the transition structure, and
         *      in our case is a format string.  The token string is moved
         *      to a null terminated buffer, and printed using the format
         *      string.  In a more normal (non test) environment, the
         *      arg string would contain coded commands to the caller that
         *      describe what to do with the token.  A common action would
         *      be to look up the token in a table of valid tokens that
         *      indicates where the return value is to be stored, and putting
         *      the token there (in the case of a qualifier switch, for
         *      example.
         */

                if( rc == FSM_ACTION ) {

                        strncpy( tbuff, token, toklen );
                        tbuff[ toklen ] = 0;
                        if( arg == 0 )
                                arg = "";

                        rc = (int) DCLaction( arg, tbuff );
                        if( rc == DCL_NOERROR )
                                continue;
                        else
                                break;
                }

        /*
         *      The "stop" state was hit.  In this case, the caller is
         *      done and the FSM has halted.
         */

                if( rc == FSM_STOP ) {
                        rc = FSM_NOERROR;
                        break;
                }

        /*
         *      Otherwise, the return code is an error signalled from
         *      the FSM machine.  The error text from a transition
         *      failure will already have been output.
         */
                break;
        }

        DCLdebugrc( "DCLpromtparm", rc );
        return rc;
}
/*<PAGE>*/
/*
 *      This routine handles the DCL actions from the finite-state machine.
 *      This is called in a loop from repeated invocations of FSMrun() over
 *      the data to be parsed.  The argument is a two character code that
 *      tells what kind of action is being processed, and what the data type
 *      is, if derived from the FSM context.
 */

static long DCLaction( char * arg, char * token )
{
        struct DCL_VERB * v, *lv;
        struct DCL_VALUE * value, *lvalue;
        char tbuff[ 64 ];
        long n, flag, sts;
        long size;
        char * tp;
        char action, kind, parmkind;

        action = arg[ 0 ];
        kind = arg[ 1 ];

        switch( action ) {

        case 'V':
                lv = 0L;
                n = strlen( token );
                for( v = verbs; v; v = v-> next ) {
                        if( strncmp( v-> name, token, n ) == 0 ) {
                                if( lv ) {
                                        DCLerror( "ambigious command \"%s\"",
                                                token );
                                        return DCL_IVERB;
                                }
                                lv = v;
                        }
                }
                v = lv;
                if( v == 0 ) {
                        DCLerror( "unrecognized command \"%s\"", token );
                        return DCL_IVERB;
                }

                DCLdebug( "identified verb", token );
                while( v-> alias ) {
                        v = v-> alias;
                        DCLdebug( "alias for verb", v-> name );
                }

                current_verb = v;
                v-> lastparm = 0L;
                v-> nextparm = v-> parameter;
                DCLresetreq();
                v-> state = DCL_PRESENT;

                return DCL_NOERROR;

        case 'L':
                v = current_verb;
                if( v == 0L ) {
                        DCLinternal_error( "list without verb","" );
                        return DCL_INTERNAL;
                }
                if( current_qualifier == 0L ) {
                        DCLinternal_error( "list without qualifier","");
                        return DCL_INTERNAL;
                }
                if( current_qualifier-> flags & DCL_LIST ) {
                        DCLdebug( "start of list for qualifier",
                                current_qualifier-> name );

                        return DCL_NOERROR;
                }
                DCLerror( "qualifier \"%s\" does not permit a list of values",
                                current_qualifier-> name );
                return DCL_SYNTAX;

        case 'Q':
                v = current_verb;
                flag = DCL_PRESENT;
                if( v == 0L ) {
                        DCLinternal_error( "qualifier without verb","" );
                        return DCL_INTERNAL;
                }
                current_qualifier = DCLqualsearch( v-> qualifier, token );

                if( current_qualifier == 0L && strncmp( token, "NO", 2 ) == 0 ) {
                        flag = DCL_NEGATED;
                        current_qualifier = DCLqualsearch( v-> qualifier, token + 2 );
                }

                if( current_qualifier == 0L ) {
                        DCLerror( "unrecognized or ambigious qualifier \"%s\"",
                                token );
                        return DCL_UNKNOWN;
                }
                DCLdebug( "found qualifier", current_qualifier-> name );

                if(( flag == DCL_NEGATED ) &&
                   ( current_qualifier-> flags & DCL_NONEGATE )) {
                        DCLerror( "qualifier \"%s\" cannot be negated",
                                current_qualifier-> name );
                        return DCL_CANTNEG;
                }

                current_qualifier-> state = ( char ) flag;
                current_qualifier-> flags &= ~DCL_HASVALUE;
                return DCL_NOERROR;

        case 'I':

                v = current_verb;
                if( v == 0L ) {
                        DCLinternal_error( "value without qualifier","");
                        return DCL_INTERNAL;
                }

                if( current_qualifier-> kind == DCL_NONE ) {
                        DCLerror( "qualifier \"%s\" does not permit a value",
                                current_qualifier-> name );
                        return DCL_SYNTAX;
                }

                if( kind == 'I' ) {
                        sts = DCLatoi( token, &n, current_qualifier-> flags );
                        if( sts != DCL_NOERROR )
                                return sts;
                }
                else
                if( kind == 'H' ) {
                        kind = 'I';
                        sscanf( token, "%lx", &n );
                }
                else
                        n = 0;

                if( kind == 'I' ) {
                        if( n < current_qualifier-> min ) {
                                DCLerror( "qualifier %s value too small",
                                        current_qualifier-> name );
                                return DCL_SYNTAX;
                        }
                        if( n > current_qualifier-> max ) {
                                DCLerror( "qualifier %s value too large",
                                        current_qualifier-> name );
                                return DCL_SYNTAX;
                        }
                }


                tp = token;

                /* If we're looking for a filename, make integers be strings */

                if( current_qualifier-> kind == DCL_FILENAME && kind == 'I' ) {
                        sprintf( tbuff, "%ld", n );
                        tp = tbuff;
                        n = 0;
                        kind = 'F';
                }

                /* If we are looking for a filename, accept names also  */

                if( current_qualifier-> kind == DCL_FILENAME && kind == 'N' )
                        kind = 'F';

                if( current_qualifier-> kind != DCL_ANY ) {

                        if( current_qualifier-> kind == DCL_KEYWORD ) {

                                n = DCLkeysearch( current_qualifier-> keywords,
                                                token, &tp );
                                if( n == 0 ) {
                                        return DCL_UNKNOWN;
                                }

                        }
                        else
                        if( current_qualifier-> kind != kind ) {
                                DCLerror( "qualifier value \"%s\" is incorrect type",
                                                token );
                                return DCL_TYPERR;
                        }
                }

                if( kind == 'I' || kind == 'H' )
                        tp = "";

                /* If this qualifier doesn't permit a list and there's a */
                /* value there already, replace it!                      */

                if(!( current_qualifier-> flags & DCL_LIST ) &&
                    ( current_qualifier-> value != 0L )) {
                        FSMfreemem( current_qualifier-> value );
                        current_qualifier-> value = 0L;
                        lvalue = 0L;
                }
                else {

                /* Find the end of the list of values for this qualifier */

                        for( lvalue = 0, value = current_qualifier-> value;
                             value; 
                             value = value-> next )
                                lvalue = value;
                }

                size = sizeof( struct DCL_VALUE ) + strlen( tp );
                value = ( struct DCL_VALUE * ) FSMgetmem( size );

                /* Make it the first entry in the list if the list is   */
                /* empty, else attach it to the last entry found.       */

                if( lvalue == 0L )
                        current_qualifier-> value = value;
                else
                        lvalue-> next = value;
                value-> next = 0L;

                /* Fill in the value elements. */

                value-> id = current_qualifier-> id;
                strcpy( value-> c_value, tp );
                value-> i_value = n;
                current_qualifier-> state = DCL_PRESENT;
                current_qualifier-> flags |= DCL_HASVALUE;

                DCLdebug( "processed qualifier", current_qualifier-> name );
                if( current_qualifier-> kind == DCL_KEYWORD )
                        DCLdebugn( "stored keyword value", n );
                else
                if( current_qualifier-> kind == DCL_INTEGER )
                        DCLdebugn( "stored value", n );
                else
                        DCLdebug( "stored value", tp );


                return DCL_NOERROR;

        case 'P':

                tp = token;
                v = current_verb;
                if( v-> nextparm == 0L ) {
                        DCLerror( "too many parameters%s\n", "" );
                        return DCL_SYNTAX;
                }

                if( kind == 'I' ) {
                        sts = DCLatoi( token, &n, v-> nextparm-> flags );
                        if( sts != DCL_NOERROR )
                                return sts;
                        tp = "";
                }
                else
                if( kind == 'H' ) {
                        kind = 'I';
                        sscanf( token, "%lx", &n );
                        tp = "";
                }
                else
                        n = 0;

                parmkind = ( char ) v-> nextparm-> kind;

                if( parmkind == DCL_RESTOFLINE ) {
                        tp = fsm_curpos;
                }

                /* If we're looking for a filename, make integers be strings */

                if( parmkind == DCL_FILENAME && kind == 'I' ) {
                        sprintf( tbuff, "%ld", n );
                        tp = tbuff;
                        n = 0;
                        kind = 'F';
                }

                /* If we are looking for a filename, accept names also  */

                if( parmkind == DCL_FILENAME && kind == 'N' )
                        kind = 'F';

                if( parmkind != DCL_RESTOFLINE && parmkind != DCL_ANY ) {

                        if( parmkind == DCL_KEYWORD ) {

                                n = DCLkeysearch( v->  nextparm-> keywords,
                                                token, &tp );
                                if( n == 0 )
                                        return DCL_UNKNOWN;
                        }
                        else
                        if( parmkind != kind ) {
                                DCLerror( "parameter \"%s\" is incorrect type",
                                                token );
                                return DCL_TYPERR;
                        }
                }
                
                v-> nextparm-> c_value = FSMgetmem( strlen( tp ) + 1 );
                strcpy( v-> nextparm-> c_value, tp );
                v-> nextparm-> i_value = n;
                v-> nextparm-> state = DCL_PRESENT;

                DCLdebugn( "processed parameter id", v-> nextparm-> id );
                if( v-> nextparm-> kind == DCL_KEYWORD )
                        DCLdebugn( "stored keyword value", n );
                else
                if( v-> nextparm-> kind == DCL_INTEGER )
                        DCLdebugn( "stored value", n );
                else
                        DCLdebug( "stored value", tp );


                v-> lastparm = v-> nextparm;
                v-> nextparm = v-> nextparm-> next;
                return ( parmkind == DCL_RESTOFLINE ) ? FSM_STOP : DCL_NOERROR;

        default:
                DCLinternal_error( "unable to process action %s", arg );
                return DCL_NOERROR;
        }

}
/*<PAGE>*/

/*
 *      This is called if a successfull parse is made of a DCL
 *      format statement.  The data is stored in the DCLARGBLK,
 *      and tells how to form a call to define a DCL element.
 *
 *      The switch statement triggers off of the verb id name in
 *      the $$$DCL$$$ grammar definition.  Assign a unique value
 *      to each _verb_ in that grammer.  Each case block below
 *      handles the operation of the specific verb.  Usually, the
 *      data for the operation has been stored in the DCLARGBLK
 *      structure for use by these routines.  Empty elements of
 *      that argblock are zeroed so the action can determine if
 *      a value was ever given for a specific data element (usually
 *      matched to a qualifier in the $$$DCL$$$ grammar).
 */

long DCLdefine_element(void)
{
        char buff[ 80 ];
        long sts;

        DCLdebugn( "processing statement type", dcl_private_store.kind );

        if( journal )
                return DCLjournal_element();

        if( dcl_private_store.kind != 1 && grammar_name == 0L ) {
                DCLgrammar_error( "missing GRAMMAR name before statements", "" );
                return DCL_INTERNAL;
        }

        switch( dcl_private_store.kind ) {

        case DCL_NONE:
                DCLinternal_error( "DCL callbacks failed", "" );
                return DCL_INTERNAL;
                
        case DCL_VERB_GRAMMAR:

                /* If there's an old one, finish it up now. */

                if( grammar_name ) {
                        DCLgrammar( grammar_name );
                        sts = DCLvalidate();
                        DCLendgrammar();
                        if( sts == DCL_INTERNAL )
                                return sts;
                }

                DCLgrammar( dcl_private_store.name );

                grammar_name = FSMgetmem( strlen( dcl_private_store.name ) + 1 );
                strcpy( grammar_name, dcl_private_store.name );
                current_grammar-> valid = 0;
                break;

        case DCL_VERB_VERB:

                DCLgrammar( grammar_name );
                current_grammar-> valid = 0;
                DCLverb( dcl_private_store.name, dcl_private_store.id );

                if( dcl_private_store.alias[ 0 ] )
                        DCLalias( dcl_private_store.alias );

                if( dcl_private_store.entry[ 0 ] )
                        DCLentry( dcl_private_store.entry );

                break;

        case DCL_VERB_PARAMETER:

                DCLgrammar( grammar_name );
                current_grammar-> valid = 0;

                sts = DCLresolvetype();
                if( sts != DCL_NOERROR )
                        return sts;

                if( dcl_private_store.keys[ 0 ] )
                        dcl_private_store.typ = DCL_KEYWORD;

                DCLparameter( dcl_private_store.name, dcl_private_store.id,
                              dcl_private_store.typ,
                              dcl_private_store.req ? DCL_REQ : DCL_NONE );

                if( dcl_private_store.typ == DCL_KEYWORD ) {
                        if( dcl_private_store.keys[ 0 ] == 0 ) {
                                DCLgrammar_error( "%s", "/TYPE=KEYWORD with no /KEYLIST" );
                                return DCL_INTERNAL;
                        }
                        DCLusekeylist( dcl_private_store.keys );
                }
                if( dcl_private_store.prompt[ 0 ] )
                        DCLprompt( dcl_private_store.prompt );

                break;


        case DCL_VERB_KEYWORD:

                DCLgrammar( grammar_name );
                current_grammar-> valid = 0;
                DCLkeyword( dcl_private_store.name,
                            dcl_private_store.id,
                            dcl_private_store.negate ? DCL_NONEGATE : DCL_NONE,
                            dcl_private_store.syntax );
                break;

        case DCL_VERB_QUALIFIER:

                DCLgrammar( grammar_name );
                current_grammar-> valid = 0;

                sts = DCLresolvetype();
                if( sts != DCL_NOERROR )
                        return sts;

                if( dcl_private_store.keys[ 0 ] )
                        dcl_private_store.typ = DCL_KEYWORD;

                DCLqualifier( dcl_private_store.name,
                              dcl_private_store.id,
                              dcl_private_store.typ,
                              ( dcl_private_store.suffix ? DCL_ALLOWSUFFIX : 0 ) +
                              ( dcl_private_store.req    ? DCL_REQ         : 0 ) +
                              ( dcl_private_store.list   ? DCL_LIST        : 0 ) +
                              ( dcl_private_store.negate ? DCL_NONEGATE    : 0 ));

                if( dcl_private_store.min != DCL_MINDEF )
                        DCLqual_min( dcl_private_store.min );

                if( dcl_private_store.max != DCL_MAXDEF )
                        DCLqual_max( dcl_private_store.max );

                if( dcl_private_store.syntax[ 0 ] ) 
                        DCLsyntax( dcl_private_store.syntax );

                if( dcl_private_store.local[ 0 ] )
                        DCLlocal( dcl_private_store.local );

                if( dcl_private_store.alias[ 0 ] )
                        DCLalias( dcl_private_store.alias );

                if( dcl_private_store.typ == DCL_KEYWORD ) {
                        if( dcl_private_store.keys[ 0 ] == 0 ) {
                                DCLgrammar_error( "%s", "/TYPE=KEYWORD with no /KEYLIST" );
                                return DCL_INTERNAL;
                        }
                        DCLusekeylist( dcl_private_store.keys );
                }

                if( dcl_private_store.has_def ) 
                        DCLdefault( dcl_private_store.has_def,
                                dcl_private_store.i_def, dcl_private_store.c_def );
                break;

        case DCL_VERB_TYPE:
                DCLgrammar( grammar_name );
                current_grammar-> valid = 0;

                /*
                 * Switch to the "grammar" grammar, and add the
                 * type as a valid /TYPE keyword.
                 */

                DCLkeylist( "$$$TYPES" );

                DCLkeyword( dcl_private_store.name,
                            DCL_NO_ID,
                            DCL_NONEGATE, 0L );

                DCLkeylist( dcl_private_store.name );

                break;

        case DCL_VERB_SYNTAX:
                DCLgrammar( grammar_name );
                current_grammar-> valid = 0;
                strcpy( buff, "SYNTAX$" );
                strcat( buff, dcl_private_store.name );
                DCLverb( buff, dcl_private_store.id );

                if( dcl_private_store.entry[ 0 ] )
                        DCLentry( dcl_private_store.entry );

                break;

        case DCL_VERB_END:
                DCLgrammar( grammar_name );
                sts = DCLvalidate();
                DCLendgrammar();
                return sts;

        case DCL_VERB_DISALLOW:

                DCLgrammar( grammar_name );
                current_grammar-> valid = 0;
                sts = DCLdisallow( dcl_private_store.name, dcl_private_store.alias );
                break;

        }

/*
 *      We are done changing the grammar being currently built. 
 */

        DCLendgrammar();

        return DCL_NOERROR;
}
/*<PAGE>*/

/*
 *      Given an instance of the data parsed in a DCLARGBLK, write out
 *      the correct code that will cause the structure to be re-assembled
 *      correctly and then a call to DCLdefine_element() made.
 */

static long DCLjournal_element(void)
{

        DCLjournal_int( "kind",         dcl_private_store.kind          );
        DCLjournal_int( "id",           dcl_private_store.id            );
        DCLjournal_chr( "name",         dcl_private_store.name          );
        DCLjournal_int( "typ",          dcl_private_store.typ           );
        DCLjournal_int( "req",          dcl_private_store.req           );
        DCLjournal_int( "suffix",       dcl_private_store.suffix        );
        DCLjournal_int( "value",        dcl_private_store.value         );
        DCLjournal_int( "list",         dcl_private_store.list          );
        DCLjournal_int( "has_def",      dcl_private_store.has_def       );
        DCLjournal_int( "negate",       dcl_private_store.negate        );
        DCLjournal_chr( "keys",         dcl_private_store.keys          );
        DCLjournal_chr( "syntax",       dcl_private_store.syntax        );
        DCLjournal_chr( "alias",        dcl_private_store.alias         );
        DCLjournal_chr( "imply",        dcl_private_store.imply         );
        DCLjournal_chr( "local",        dcl_private_store.local         );

        if( dcl_private_store.min > DCL_MINDEF )
                DCLjournal_int( "min",          dcl_private_store.min           );

        if( dcl_private_store.max < DCL_MAXDEF )
                DCLjournal_int( "max",          dcl_private_store.max           );

        DCLjournal_int( "i_def",        dcl_private_store.i_def         );
        DCLjournal_chr( "c_def",        dcl_private_store.c_def         );
        DCLjournal_chr( "prompt",       dcl_private_store.prompt        );

        DCLjournal_write( "\tDCLdefine_element();" );
        return DCL_NOERROR;
}



static long DCLjournal_int( char * name, long value )
{

        char buff[ 80 ];

        if( value == 0L )
                return DCL_NOERROR;

        sprintf( buff, "\tdcl_private_store.%s = %ld;", name, value );
        DCLjournal_write( buff );
        return DCL_NOERROR;
}

static long DCLjournal_chr( char * name, char * value )
{

        char buff[ 80 ];

        if( value == 0L )
                return DCL_NOERROR;
        if( *value == '\0' )
                return DCL_NOERROR;

        sprintf( buff, "\tstrcpy( dcl_private_store.%s, \"%s\" );", 
                name, value );
        DCLjournal_write( buff );
        return DCL_NOERROR;
}

static long DCLjournal_write( char * buff )
{

        if( journal_file )
                fprintf( journal_file, "%s\n", buff );
        else
                printf( "%s\n", buff );

        return DCL_NOERROR;
}

/*<PAGE>*/

/*
 *      This is the callback that is used to move the data from the
 *      DCL grammar storage to the DCLARGBLK when the user is defining
 *      a language element of his own.
 *
 *      Each case entry type for qualifiers matches the id of the qualifier
 *      in the $$$DCL$$$ grammar definition.  When adding a new qualifier
 *      type, choose a unique number for all qualifiers, since this is
 *      driven through a single switch statement in this routine.  Typically,
 *      the id 'case' block puts data into the DCLARGBLK structure for
 *      use by the actual defining routine, so each qualifier type has a
 *      slot in the DCLARGBLK structure even if it isn't used for all of
 *      the qualifier types.
 */

static long DCLcallback_dcl( int kind, char * name, long id, long dtype, long i_value, char * c_value ) 
{
		char * dummy;

		dummy = name;

        switch( kind ) {

        case DCL_CALLBACK_VERB:
                dcl_private_store.kind = (int) id;
                dcl_private_store.id = DCL_NO_ID;
                dcl_private_store.name[ 0 ] = 0;
                dcl_private_store.typ = 0;
                dcl_private_store.req = 0;
                dcl_private_store.suffix = 0;
                dcl_private_store.list = 0;
                dcl_private_store.has_def = 0;
                dcl_private_store.i_def = id;
                dcl_private_store.c_def[ 0 ] = 0;
                dcl_private_store.prompt[ 0 ] = 0;
                dcl_private_store.negate = 0;
                dcl_private_store.keys[ 0 ] = 0;
                dcl_private_store.local[ 0 ] = 0;
                dcl_private_store.syntax[ 0 ] = 0;
                dcl_private_store.alias[ 0 ] = 0;
                dcl_private_store.entry[ 0 ] = 0;
                dcl_private_store.imply[ 0 ] = 0;
                dcl_private_store.min = DCL_MINDEF;
                dcl_private_store.max = DCL_MAXDEF;
                break;

        case DCL_CALLBACK_PARAMETER:
                switch( id ) {
                case DCL_PARAMETER_NAME:
                        strcpy( dcl_private_store.name, c_value );
                        strcpy( dcl_private_store.c_def, c_value );
                        break;
                case DCL_PARAMETER_NAME2:
                        strcpy( dcl_private_store.alias, c_value );
                        break;
                default:
                        break;
                }
                break;

        case DCL_CALLBACK_QUALIFIER:
                switch( id ) {

                case DCL_QUALIFIER_IDENT:
                        dcl_private_store.id = i_value;
                        break;

                case DCL_QUALIFIER_REQUIRED:
                        dcl_private_store.req = DCL_REQ;
                        break;

                case DCL_QUALIFIER_NONEGATABLE:
                        dcl_private_store.negate = DCL_NONEGATE;
                        break;

                case DCL_QUALIFIER_LIST:
                        dcl_private_store.list = DCL_LIST;
                        break;

                case DCL_QUALIFIER_TYPE:

                        strcpy( dcl_private_store.keys, c_value );
                        break;

                case DCL_QUALIFIER_SYNTAX:
                        strcpy( dcl_private_store.syntax, c_value );
                        break;

                case DCL_QUALIFIER_ENTRY:
                        strcpy( dcl_private_store.entry, c_value );
                        break;

                case DCL_QUALIFIER_ALIAS_FOR:
                        strcpy( dcl_private_store.alias, c_value );
                        break;

                case DCL_QUALIFIER_MINIMUM:
                        dcl_private_store.min = i_value;
                        break;

                case DCL_QUALIFIER_MAXIMUM:
                        dcl_private_store.max = i_value;
                        break;

                case DCL_QUALIFIER_DEFAULT:
                        if( dtype == DCL_NONE ) {
                                dcl_private_store.has_def = DCL_DEFAULTED;
                                dcl_private_store.i_def = dcl_private_store.id;
                                strcpy( dcl_private_store.c_def, dcl_private_store.name );
                        }
                        else {
                                dcl_private_store.has_def = DCL_DEFWITHVALUE;
                                dcl_private_store.i_def = i_value;
                                strcpy( dcl_private_store.c_def, c_value );
                        }
                        break;

                case DCL_QUALIFIER_IMPLY:
                        strcpy( dcl_private_store.imply, c_value );
                        break;

                case DCL_QUALIFIER_PROMPT:
                        strcpy( dcl_private_store.prompt, c_value );
                        break;

                case DCL_QUALIFIER_MULTIPLIER:
                        dcl_private_store.suffix = 1;
                        break;

                case DCL_QUALIFIER_LOCAL:
                        strcpy( dcl_private_store.local, c_value );
                        break;

                default:
                        DCLinternal_error( "unrecognized argblk element id","" );
                        return DCL_INTERNAL;

                }
                break;
        }

        return DCL_NOERROR;

}

/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                         DEBUG MESSAGE ROUTINES                           *
 *                                                                          *
\****************************************************************************/


/*
 *      Put out a debugging message to the user that incorporates a
 *      longword integer.  Formats the integer and then passes the
 *      code to the normal DCLdebug() routine.
 */

static long DCLdebugn( char * msg, long arg )
{

        char nbuff[ 32 ];
        sprintf( nbuff, "%ld", arg );
        return DCLdebug( msg, nbuff );
}

/*<PAGE>*/
/*
 *      Put out debugging text to the user, if appropriate.  If not
 *      appropriate, then no work is done.
 */

static long DCLdebug( char * msg, char * arg )
{

        if( dcl_debug_state == DCL_DEBUG ) 
                printf( "DCLdebug: %s %s\n", msg, arg );
        return DCL_NOERROR;
}
/*<PAGE>*/

/*
 *      Signal a DCL error.
 */

static long DCLerror( char * fmt, char * arg )
{
        char buff[ 256 ];

        sprintf( buff, fmt, arg );
        if( error_callback == 0L ) {
                if( islower( buff[ 0 ]))
                        buff[ 0 ] = ( char ) toupper( buff[ 0 ]);
                        printf( "%%DCL-E-ERROR, %s\n", buff );
        }
        else
                (*error_callback)( buff );

        return DCL_NOERROR;
}

/*
 *      Signal an internal or grammar error.
 */

static long DCLinternal_error( char * fmt, char * arg )
{

        char buff[ 256 ];

        strcpy( buff, "internal error; " );
        strcat( buff, fmt );
        return DCLerror( buff, arg );
}

static long DCLgrammar_error( char * fmt, char * arg )
{

        char buff[ 256 ];

        strcpy( buff, "grammar error; " );
        strcat( buff, fmt );
        return DCLerror( buff, arg );
}
/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                           FSM I/O ROUTINES                               *
 *                                                                          *
\****************************************************************************/

/*
 *      This routine gets a string from the user.  It is separated out
 *      because in a normal callable case this routine wouldn't be used,
 *      and it's generally not portalble.
 */

static long FSMgetstring( char * prompt, char * dbuff )
{
        
        if( (void*)read_callback == (void*)DCL_INPUT ) {
        
                printf( "%s", prompt );
                fgets( dbuff, 255, stdin );
 
                return FSM_NOERROR;
        }

        if( read_callback ) {
                return (*read_callback)( dbuff, 255 );
        }

        dbuff[ 0 ] = 0;
        DCLerror( "no text to parse", "" );
        return DCL_INTERNAL;
}

/*<PAGE>*/
/****************************************************************************\
 *                                                                          *
 *                 INTERNAL FINITE STATE MACHINE ROUTINES                   *
 *                                                                          *
\****************************************************************************/


/*
 *      Add a transition to the current state that skips past blanks.
 *      This is normally done automatically when a state is created,
 *      using the skipflag (second) parameter to FSMstate().  An
 *      explicit blank skip transition may be encoded using this
 *      routine.  The transition does no work, but advances the token
 *      pointer to the first character not in the "_space_" string.
 */

static long FSMskipblanks(void)
{
        struct STRING * s;
        static int string_found = 0;

/*
 *      See if the user has already defined a preferred blanks string. 
 *      If so, use that one.
 */

        if( !string_found ) {

                for( s = fsm_strings; s; s = s-> next ) {

                        if( strcmp( s-> name, "_space_" ) == 0 ) {
                                string_found = 1;
                                break;
                        }
                }
        }

/*
 *      If not, then create the default, which is blanks, tabs, and newlines.
 */

        if( !string_found ) {
                FSMstring( "_space_", " \t\n", 0L, 0L, FSM_NOCONCAT );
                string_found = 1;
        }

/*
 *      Create a transition that eats blanks and does no other work.
 */

        FSMtransition( FSM_STRING, "_space_", 0L, 0L );
        return FSM_NOERROR;
}
/*<PAGE>*/

/*
 *      Create an entry in the state structure that specifies that if no
 *      transition has occurred yet then signal an error.  This is the
 *      normal means of identifying syntax errors, i.e. nothing has been
 *      found that we recognize, so error.  This is stored in the current
 *      state being defined, so an FSMstate() must be active.
 */

static long FSMnotransition( long code )
{

        fsm_current_state-> error = code + 1024;
        return FSM_NOERROR;
}
/*<PAGE>*/

/*
 *      This entry stores an error code in the error list.  The error code
 *      has 1k added to it, since error codes less than 1024 are reserved
 *      to the FSM for internal use or for return codes to the caller.
 */

static long FSMerror( long code, char * text )
{

        struct ERROR * p;

        if( code < 1 ) {
                FSMputerror( 0, "Invalid non-positive error code used" );
                return FSM_ERROR;
        }

        p = ( struct ERROR * ) FSMgetmem( sizeof( struct ERROR ) + strlen( text ));
        p-> next = fsm_errors;
        fsm_errors = p;
        p-> code = code + 1024;
        strcpy( p-> text, text );
        return FSM_NOERROR;
}

/*<PAGE>*/

/*
 *      Store a string definition.  A string match occurs when the first
 *      character of the token buffer is from the list of valid initial
 *      characters, and all remaining characters are from the list of
 *      valid secondary characters.  Typically (but not always) the list
 *      of secondary characters includes the initial characters as well.
 *
 *      The secondary string can be a null pointer.  In this case, the
 *      initial and secondary strings are assumed to be identical.  
 *
 *      The "concat" flag indicates if the secondary string is to be
 *      constructed from the initial string concatenated with the contents
 *      of the secondary string.  This makes it easier to handle the normal
 *      case where the secondary string is a superset of the initial string.
 *
 */

static long FSMstring( char * name, char * initial, char * secondary, 
                        char * delimiters, int concat )
{

        struct STRING * p;
        long size;

/*
 *      Allocate the string header structure.
 */

        p = ( struct STRING * ) FSMgetmem( sizeof( struct STRING ) + strlen( name ));

/*
 *      Create memory for the initial string, and copy the argument to the
 *      new storage area.
 */

        p-> initial = FSMgetmem( strlen( initial ) + 1 );
        strcpy( p-> initial, initial );

/*
 *      If a secondary string exists, create that.  If the concat flag
 *      indicates the secondary string implicitly includes the initial
 *      string, then calculate the size based on both initial and
 *      secondary strings.  Otherwise, the size is just that of the
 *      secondary string.   Move the data to the secondary string buffer.
 */

        if( secondary ) {
                if ( concat == FSM_CONCAT )
                        size = strlen( initial ) + strlen( secondary ) + 1;
                else
                        size = strlen( secondary ) + 1;
                p-> secondary = FSMgetmem( size );
                p-> secondary[ 0 ] = 0;

                if( concat == FSM_CONCAT )
                        strcpy( p-> secondary, initial );

                strcat( p-> secondary, secondary );

        }
        else {
                p-> secondary = 0;
        }

/*
 *      If the delimiter string exists, copy that.
 */

        if( delimiters ) {
                p-> delimiters = FSMgetmem( strlen( delimiters ) + 1 );
                strcpy( p-> delimiters, delimiters );
        }
        else
                p-> delimiters = 0L;

/*
 *      Fill in the remainder of the string header structure.
 */

        strcpy( p-> name, name );
        p-> next = fsm_strings;
        fsm_strings = p;

        return FSM_NOERROR;
}
/*<PAGE>*/

/*
 *      Define a new state.  
 *
 *      This creates a new state entry.  It also makes it "current" so all
 *      subsequent transition definitions are bound to the current state
 *      until another state definition or the state machine runs.
 */


static long FSMstate( char * name, int blanks )
{

        struct STATE * p;

/*
 *      See if there is already a state of this name.  If so, we are done,
 *      and just make it current.  This allows the caller to create states
 *      and transitions out of synch by redefining the state each time a
 *      new set of transitions is to be bound.
 */

        for( p = fsm_states; p; p = p->next ) {
                if( strcmp( p-> name, name ) == 0 ) {
                        break;
                }
        }

/*
 *      If the state didn't already exist, create the storage now, and link
 *      it to the list of defined states.  We also initialize the state data.
 */

        if( p == 0 ) {
                p = ( struct STATE * ) FSMgetmem( sizeof( struct STATE ) + strlen( name ));
                p-> next = fsm_states;
                fsm_states = p;
                strcpy( p-> name, name );

                p-> transitions = 0;
                p-> error = 0;
        }

/*
 *      Make the state current.  If this declaration also included a request
 *      to skip blanks, handle that now.
 */

        fsm_current_state = p;

        if( blanks == FSM_SKIPBLANKS )
                        FSMskipblanks();

        return FSM_NOERROR;
}
/*<PAGE>*/

/*
 *      Define the state that stops processing.  If the state does not
 *      exist, it is created.  The state must not already have any
 *      transitions bound to it since it is the terminal state of the
 *      FSM.  For the same reason, there cannot be a declared non-transition
 *      error code.
 */

static long FSMstop( char * name )
{

        FSMstate( name, 0 );
        if( fsm_current_state-> transitions ) {
                FSMputerror( 0, "Cannot stop on a state with transitions" );
                return FSM_ERROR;
        }

        if( fsm_current_state-> error == FSM_UNDEFINED )
                fsm_current_state-> error = 0;

        if( fsm_current_state-> error ) {
                FSMputerror( 0, "Cannot stop on a state with error code" );
                return FSM_ERROR;
        }

        fsm_current_state-> error = FSM_STOP;
        return FSM_NOERROR;
}
/*<PAGE>*/

/*
 *      Define a transition for the current state.  
 *
 *      kind            The transition kind, from a predefined list.
 *
 *      literal         A literal string or a name, may be 0L if the
 *                      transition doesn't require data.
 *
 *      state           The name of the state to transfer to when the
 *                      transition occurs.  If 0L is passed, then no
 *                      state change occurs and the next transition
 *                      will be processed.
 *
 *      arg             The transition action argument.  If not 0L, this
 *                      data is passed back to the caller with a return
 *                      code indicating that an action is required.  If
 *                      set to 0L, then no action is done and the next
 *                      state is selected.
 */

static long FSMtransition( long kind, char * literal, char * state, char * arg )
{

        struct TRANSITION * p, *last;
        struct STATE * s, * tmp;
        struct STRING * st;
        struct ARG * a;
        char * c;

/*
 *      Currently, transition lists are singly linked lists.  Eeech.  But
 *      for now, step through the transition list for the current state,
 *      and locate the end of the list.
 */

        for( last = 0, p = fsm_current_state-> transitions; p; last = p, p = p-> next )
                continue;

/*
 *      Create storage for the new transition data structure.
 */

        p = ( struct TRANSITION * ) FSMgetmem( sizeof( struct TRANSITION ));

/*
 *      If the list is non-empty, attach the new transition to the end of
 *      the list.  Otherwise, make it the first entry on the list.
 */

        if( last )
                last-> next = p;
        else
                fsm_current_state-> transitions = p;

/*
 *      Initialize the transition data.  Close the linked list, store the
 *      transition kind, and initialize the next-state value to null.
 */

        p-> next = 0;
        p-> kind = kind;
        p-> state = 0;

/*
 *      See if the state name given exists already...
 */

        if( state ) {

/*
 *      Search for a matching state.  If one's found, then we
 *      are done and we can bind it to the state. 
 */

                s = 0;
                for( s = fsm_states; s; s = s-> next )
                        if( strcmp( state, s-> name ) == 0 ) {
                                break;
                        }

/*
 *      If there's no such state created yet, then make one we can
 *      bind to.  We must temporarily save the "fsm_current_state" since
 *      this is established by the call to FSMstate.
 */

                if( s == 0 ) {
                        tmp = fsm_current_state;
                        FSMstate( state, 0 );
                        fsm_current_state-> error = FSM_UNDEFINED;
                        s = fsm_current_state;
                        fsm_current_state = tmp;
                }

/*
 *      Attach the state pointer to the transition.
 */

                p-> state = s;

        }

/*
 *      If the action argument in non-zero, then allocate an argument block
 *      for it that can contain the null-terminated argument data.
 */


        if( arg ) {

                a = ( struct ARG * ) FSMgetmem( sizeof( struct ARG ) + strlen( arg ));
                p-> args = a;
                strcpy( a-> value, arg );
                a-> next = 0;
        }

/*
 *      If the transition is for a quoted string, accept the name
 *      of the string set that can be used for quoting.  The next
 *      character must be in that set, and we consume all characters
 *      up to the next occurance of the same character.
 */

        if( kind == FSM_QUOTE ) {
                p-> token.string = 0;
                for( st = fsm_strings; st; st = st-> next ) {
                        if( strcmp( st-> name, literal ) == 0 ) {
                                p-> token.string = st;
                                break;
                        }
                }
                if( p-> token.string == 0 ) {
                        FSMputerror( 0, "Attempt to bind unknown quote list to transition" );
                        return FSM_ERROR;
                }
        }
        else

/*
 *      No work for dates...
 */

        if( kind == FSM_DATE ) {
                p-> token.literal = "<date>";
        }
        else
/*
 *      If the transition is for a filename, we don't have any additional
 *      data.
 */

        if( kind == FSM_FILENAME ) {
                p-> token.literal = "<filename>";
        }
        else
/*
 *      If the transition is for the end-of-string, do nothing special.
 */

        if( kind == FSM_EOS ) {
                p-> token.literal = "<eos>";
        }
        else

/*
 *      If the transition is for any valid character, then we don't
 *      need to do anything special.
 */
        if( kind == FSM_ANYCHAR ) {

                p-> token.literal = "<anychar>";
        }
        else

/*
 *      If the transition is for any character except the named
 *      literal character, store that away as the token value.
 */

        if( kind == FSM_EXCEPTCHAR ) {
                p-> token.onechar = literal[ 0 ];
        }
        else
/*
 *      If the transition is a test for a literal string, then allocate
 *      space for the literal and copy it to that space.  Attach a pointer
 *      to the storage to the union data in the transition.
 */

        if( kind == FSM_LITERAL ) {
                c = FSMgetmem( strlen( literal ) + 1 );
                p-> token.literal = c;
                strcpy( p-> token.literal, literal );
        }
        else

/*
 *      If the transition is a null transition, store a string constant
 *      in the transition data for debugging purposes.
 */

        if( kind == FSM_NULL ) {
                p-> token.literal = "<null>";
        }

/*
 *      The literal data must be a name of some kind, such as the name
 *      of an FSMstring() entry.  Find the matching string pattern data
 *      and bind it's address to the transition.
 */

        else
        if( kind == FSM_STRING ) {
                p-> token.string = 0;
                for( st = fsm_strings; st; st = st-> next ) {
                        if( strcmp( st-> name, literal ) == 0 ) {
                                p-> token.string = st;
                                break;
                        }
                }
                if( p-> token.string == 0 ) {
                        FSMputerror( 0, "Attempt to bind unknown string to transition" );
                        return FSM_ERROR;
                }
        }

        return FSM_NOERROR;
}
/*<PAGE>*/

/*
 *      Set the initial state of the FSM.  This is different than the
 *      "current state" which stores the state that subsequent FSMxxx()
 *      calls will add definitions to.  The "initial state" is the one
 *      that will be executed next when the FSMrun() call is made.
 */

static long FSMinitial_state( char * state )
{

        struct STATE * s;

        fsm_init_state = 0;

/*
 *      Make sure the state given exists.  If it doesn't exist yet,
 *      then we create a new one.
 */

        for( s = fsm_states; s; s = s-> next )
                if( strcmp( state, s-> name ) == 0 ) {
                        fsm_init_state = s;
                        break;
                }

        if( fsm_init_state == 0 ) {
                s = fsm_current_state;
                FSMstate( state, 0 );
                fsm_init_state = fsm_current_state;
                fsm_current_state = s;
        }

        return FSM_NOERROR;
}
/*<PAGE>*/

/*
 *      This is an internal handling routine.  It is signalled when a
 *      problem is encountered during execution of the FSM.  If the code
 *      value is zero, then the text is printed as is.  If the code is
 *      non-zero, the value is also printed.  In either case, the FSM
 *      initial state is set to void.
 */

static long FSMputerror( int code, char * text )
{
        char buff[ 80 ];
		int n;

		n = code;
        fsm_init_state = ( struct STATE * ) -1;

        if( error_callback == 0L ) {
                strcpy( buff, text );
                if( islower( buff[ 0 ]))
                        buff[ 0 ] = ( char ) toupper( buff[ 0 ]);
                printf( "%s\n", buff );
        }
        else
                (*error_callback)( text );
        return FSM_NOERROR;
}

/*<PAGE>*/

/*
 *      Run the FSM for a while.  This returns when the data is exhausted,
 *      an error occurs, or an action must be performed before continuing.
 */

static long FSMrun( char ** string,     /* Pointer into buffer being parsed     */
             char ** state,     /* Name of last state processed         */
             char ** arg,       /* Argument string of last action       */
             char ** token,     /* Pointer to first char of last token  */
             int * toklen )     /* Number of bytes in last token        */
{
        char * pt, *tst;
        struct TRANSITION * t;
        int n, r;
        struct ERROR * e;
        struct STRING * str;
        char qchar;
        char tbuff[ 32 ];
		int running;

        pt = "<none>";
        *state = pt;
        *arg = pt;
        *token = pt;
        *toklen = 0;

/*
 *      If FSMinitial_state() was never called, then boo-boo.
 */

        if( fsm_init_state == 0 ) {
                FSMputerror( 0, "No initial state specified" );
                return FSM_ERROR;
        }

/*
 *      If an error has occurred and FSMinitial_state() is not
 *      called again to reinitialize the FSM, then boo-boo.
 */

        if( fsm_init_state == ( struct STATE * ) -1 ) {
                FSMputerror( 0, "Cannot resume from error" );
                return  FSM_ERROR;
        }

/*
 *      If the current (next) state is a terminal state in the FSM,
 *      then we are done anyway.
 */

        if( fsm_init_state-> error == FSM_STOP ) {
                        return FSM_STOP;
                }

/*
 *      Scan over the string...
 */

        pt = *string;

/*
 *      While the current character is non-zero...
 */
		running = 1;
        while( running ) {
                
                r = 1;

        /*
         *      If we've gotten to a terminal state during the last FSM
         *      iteration, then we're done.
         */

                if( fsm_init_state-> error == FSM_STOP ) {
                        return FSM_STOP;
                }

        /*
         *      Check each possible transition for this state.
         */

                for( t = fsm_init_state-> transitions; t; t = t-> next ) {

                /*
                 *      Put out a debugging message if appropriate.
                 */

                        FSMdebugtrans( t );

                /*
                 *      If the transition is for EOS, see if we're there, by
                 *      testing for a non-zero character under the pointer.
                 */

                        if( t-> kind == FSM_EOS ) {
                                r = *pt;
                                break;
                        }

                /*
                 *      If the transition is null, then it means it
                 *      is always taken, and we are done.
                 */

                        if( t-> kind == FSM_NULL ) {
                                r = 0;
                                break;
                        }

                /*
                 *      If the transition is for "rest of line", make
                 *      sure that's okay, and return the stuff we have
                 *      with no further check.
                 */

                        if( t-> kind == FSM_RESTOFLINE ) {

                                if( !fsm_allow_rest )
                                        continue;
                                if( *pt == 0 )
                                        continue;

                                r = 0;
                                *token = pt;
                                *toklen = 1;
                                pt++;
                                break;
                        }

                /*
                 *      If the transition is for any character, return
                 *      the character to process as an action item.  We
                 *      always take the transition, no matter what.
                 */

                        if( t-> kind == FSM_ANYCHAR ) {
                                r = 0;
                                *token = pt;
                                *toklen = 1;
                                pt++;
                                break;
                        }

                /*
                 *      If the transition is for any character but the
                 *      specified one, handle that.  If it's the "except"
                 *      character, then we DON'T take the transition.
                 */

                        if( t-> kind == FSM_EXCEPTCHAR ) {
                                if( *pt != t-> token.onechar ) {
                                        r = 0;
                                        *token = pt;
                                        *toklen = 1;
                                        pt++;
                                        break;
                                }
                                continue;
                        }

                /*
                 *      If the transition is for a date, see if the
                 *      token stream contains a valid filename...
                 */

                        if( t-> kind == FSM_DATE ) {

                                r = (int)FSMisdate( pt, &n, tbuff );
                                if( r == 0 ) {
                                        *token = tbuff;
                                        *toklen = (int) strlen( tbuff );
                                        pt+= *toklen;
                                        break;
                                }
                                continue;
                        }

                /*
                 *      If the transition is for a filename, see if the
                 *      token stream contains a valid filename...
                 */

                        if( t-> kind == FSM_FILENAME ) {

                                r = (int)FSMisfilename( pt, &n );
                                if( r == 0 ) {
                                        *token = pt;
                                        *toklen = n;
                                        pt+= *toklen;
                                        break;
                                }
                                continue;
                        }

                /*
                 *      If the transition is for a literal, compare for
                 *      as many characters as the literal calls for, and
                 *      see if there's a match.  If so, then return the
                 *      token pointer and length to the caller and skip 
                 *      past the token.
                 */

                        if( t-> kind == FSM_LITERAL ) {

                                n = (int) strlen( t-> token.literal );
                                r = strncmp( pt, t-> token.literal, n );

                                if( r == 0 ) {
                                        *toklen = n;
                                        *token = pt;
                                        pt += n;
                                        break;
                                }
                                continue;
                        }


                /*
                 *      See if it's a quote request.  This checks the next
                 *      character to see if it's in the string list.  If so,
                 *      we each characters until we get a matching closing
                 *      quote character.
                 */

                        if( t-> kind == FSM_QUOTE ) {
                                str = t-> token.string;
                                if( FSMinlist( *pt, str-> initial)) {
                                        qchar = *pt;
                                        *token = pt + 1;
                                        if( str-> secondary )
                                                tst = str-> secondary;
                                        else
                                                tst = str-> initial;
                                        
                                        for( n = 0; n < FSM_MAXTOKEN; n++ ) {
                                                pt++;
                                                if( *pt == 0 )
                                                        break;
                                                if( *pt == qchar )
                                                        break;
                                        }
                                        r = 0;
                                        *toklen = n;
                                        pt++;
                                        break;
                                }
                                continue;
                        }


                /*      See if it's a predefined string.  This requires
                 *      checking the first character to see if it's in
                 *      the initial list, and each following character to
                 *      see if it's in the secondary list.  If there is
                 *      no secondary list, then use the initial list for 
                 *      all following characters.  If the first character
                 *      is not in the initial list then the transition
                 *      is not taken.
                 */

                        if( t-> kind == FSM_STRING ) {
                                str = t-> token.string;
                                if( FSMinlist( *pt, str-> initial)) {
                                        *token = pt;
                                        n = 1;
                                        if( str-> secondary )
                                                tst = str-> secondary;
                                        else
                                                tst = str-> initial;
                                        
                                        for( n = 1; n < FSM_MAXTOKEN; n++ ) {
                                                pt++;
                                                if( *pt == 0 )
                                                        break;
                                                if( !FSMinlist( *pt, tst ))
                                                        break;
                                        }

                                        /* If there are no required delims */
                                        /* or the next character is EOS    */
                                        /* then we're done with this token */

                                        if( *pt == 0 || str-> delimiters == 0L ) {
                                                r = 0;
                                                *toklen = n;
                                                break;
                                        }

                                        /* If the delimiting character is    */
                                        /* from the valid set of delimiters, */
                                        /* then we're okay, else bail out... */

                                        if( FSMinlist( *pt, str-> delimiters )) {
                                                r = 0;
                                                *toklen = n;
                                                break;
                                        }
                                        else {
                                                r = 1;
                                                pt = *token;
                                                *token = 0L;
                                        }
                                }
                                continue;
                        }

                /*      Uh-oh.  It wasn't a known transition type.  We bail
                 *      out here. 
                 */

                        FSMputerror( 0, "Unsupported transition type found" );
                        return FSM_ERROR;

                }

        /*
         *      If the return code is non-zero still, then no transition
         *      was taken.  See if there is a matching error for this state
         *      to be displayed when no transition is found.  If there isn't
         *      a matching error, put out a generic one.
         */

                if( r ) {
                        for( e = fsm_errors; e; e = e-> next ) {
                                if( fsm_init_state-> error == e-> code ) {
                                        FSMputerror( (int) e-> code, e-> text );
                                        return FSM_ERROR;
                                }
                        }
                        FSMputerror( (int) fsm_init_state-> error, 
                                "Transition error" );
                        return FSM_ERROR;
                }

        /*
         *      Okay, we've decided on a transition (identified by the pointer
         *      "t".  If the transition has an argument, then we've got to let
         *      the caller handle that.
         */

                if( t-> args ) {
                        *arg = t-> args-> value;
                        *state = fsm_init_state-> name;
                        fsm_curpos = *token;

        /*
         *      If we will transfer to a new state on the next FSMrun(),
         *      then put that out as a debug message.  This way the caller
         *      knows what's going to happen next.
         */

                        if( t-> state ) {
                                FSMdebug( "anticipated transition to state ", 
                                        t-> state-> name );

                                fsm_init_state = t-> state;
                        }
                        *string = pt;
                        return FSM_ACTION;
                }

        /*
         *      If we got here, there's no action to be performed by the
         *      caller, so we can just go ahead and take the transition
         *      immediately.  Put out a debug message and move to the next
         *      state.
         */

                if( t-> state ) {
                        FSMdebug( "immediate transition to state ", 
                                        t-> state-> name );
                        fsm_init_state = t-> state;
                }
        }

/*
 *      If we fell out of the loop, we need more text and have not yet
 *      hit a terminal state.  This lets the caller acquire more data and
 *      call FSMrun() again.  The initial state will still point to the
 *      last state transition-ed to.
 */

        return FSM_NEEDMORE;
}
/*<PAGE>*/

/*
 *      If appropriate, put out debug messages describing the current
 *      transition.  Since each transition type has a different format,
 *      we've got a single centralized formatter routine here.
 *
 */

static long FSMdebugtrans( struct TRANSITION * t )
{

        char * m, * a;
        char b[ 256 ];
        char d[ 4 ];

        if( fsm_debug_state ) {
                m = "testing transition based on";

                if( t-> kind == FSM_FILENAME ) {
                        a = "<filename>";
                }
                else
                if( t-> kind == FSM_DATE ) {
                        a = "<date>";
                }
                else
                if( t-> kind == FSM_EOS ) {
                        a = "<end of string>";
                }
                else
                if( t-> kind == FSM_QUOTE ) {
                        strcpy( b, "quoted value delimited by string " );
                        strcat( b, t-> token.string-> name );
                        a = b;
                }
                else
                if( t-> kind == FSM_RESTOFLINE )
                        a = "rest of line";
                else
                if( t-> kind == FSM_ANYCHAR )
                        a = "any character";
                else
                if( t-> kind == FSM_EXCEPTCHAR ) {
                        strcpy( b, "any character except \"" );
                        d[ 0 ] = t-> token.onechar;
                        d[ 1 ] = 0;
                        strcat( b, d );
                        strcat( b, "\"" );
                        a = b;
                }
                else
                if( t-> kind == FSM_NULL ) {
                        a = "";
                        m = "null (always true) transition test";
                }
                else
                if( t-> kind == FSM_LITERAL ) {
                        strcpy( b, "literal \"" );
                        strcat( b, t-> token.literal );
                        strcat( b, "\"" );
                        a = b;
                }
                else
                if( t-> kind == FSM_STRING ) {
                        strcpy( b, "string set " );
                        strcat( b, t-> token.string-> name );
                        a = b;
                }
                else {
                        sprintf( b, "unknown transition code %ld", t-> kind );
                        a = b;
                }

                FSMdebug( m, a );
        }

        return FSM_NOERROR;
}

/*<PAGE>*/
/*
 *      Put out debugging text to the user, if appropriate.  If not
 *      appropriate, then no work is done.
 */

static long FSMdebug( char * msg, char * arg )
{

        if( fsm_debug_state ) 
                printf( "FSMdebug: %s %s\n", msg, arg );
        return FSM_NOERROR;
}
/*<PAGE>*/

/*
 *      Utility used to check a character to see if it's in a list,
 *      as part of an FSM_STRING transition.  Returns FSM_FOUND if
 *      the character is in the string, else FSM_NOTFOUND.
 */

static long FSMinlist( char ch, char * list )
{
        int n, j;

        n = (int) strlen( list );
        for( j = 0; j < n; j++ ) {
                if( ch == list[ j ] )
                        return FSM_FOUND;
        }
        return FSM_NOTFOUND;
}
/*<PAGE>*/

/*
 *      Given a pointer to a string area, determine if the next item
 *      in the string is a valid filename.  If so, then parse it until
 *      you find the end of the string, and return the length.  Returns
 *      zero if found, else nonzero if not a filename.
 *
 *      THIS IS A HOST-SPECIFIC ROUTINE AND MUST BE IMPLEMENTED CORRECTLY
 *      FOR THE GIVEN HOST.
 */

static long FSMisfilename( char * p, int * lenptr )
{

        int len, dot, colon;
        int reqnode, pastnode, innode, indir, inquote, inname, inext, inver;
        char lastch;
        char * c;

        static char * chset =
        "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz$_-*";

        static char * delims = " /,+)";

        *lenptr = 0;
        len = 0;
        colon = 0;
        indir = 0;
        inquote = 0;
        dot = 0;
        inname = 0;
        inext = 0;
        inver = 0;
        innode = 0;
        lastch = 0;
        pastnode = 0;
        reqnode = 0;

        for( c = p; *c; len++,c++ ) {

                lastch = *(c-1);

                /* See if it's in the list of terminating characters */

                if( FSMinlist( *c, delims ) && !inquote )
                        break;

                /* See if it's a quoted string.  This is illegal if  */
                /* we've moved past the node.  It also can't be the  */
                /* first character of the name, and requires a node  */

                if( *c == '"' ) {
                        if( c == p )
                                return 1;
                        if( pastnode )
                                return 1;
                        inquote = 1 - inquote;
                        reqnode = 1;
                        continue;
                }

                /* If we're in a quoted string, advance to next char */

                if( inquote )
                        continue;

                /* If it's a colon, then if we're already into the name */
                /* part we've messed up.  We are also past the node name */
                /* part so quotes aren't valid any more.  Also, if there */
                /* are now two colons then we're past the node name.  If */
                /* there are too many colons then it's not a file name */

                if( *c == ':' ) {
                        if( inname || inext || inver || indir )
                                return 1;
                        pastnode = 1;
                        colon++;
                        if( colon == 2 )
                                reqnode = 0;
                        if( colon > 2 )
                                return 1;
                        continue;
                }

                /* Not a colon, so reset the counter. */

                colon = 0;

                /* If it's a semicolon, we can't be in a directory part.  */
                /* The semicolon advances us to the version part of the   */
                /* string.  If already doing versions, then bad filename  */

                if( *c == ';' ) {
                        if( indir )
                                return 1;
                        if( inname ) {
                                inname = 0;
                                inver = 1;
                        }
                        else
                        if( inext ) {
                                inext = 0;
                                inver = 1;
                        }
                        else
                        if( inver ) {
                                return 1;
                        }
                        continue;
                }

                /* If dot, it could advance to type or version.  If in  */
                /* directory scanning then booboo.  If multiple dots,   */
                /* must be three of them in directory part for wildcard */

                if( *c == '.' ) {
                        dot++;
                        if( indir && dot > 3 )
                                return 1;
                        if( !indir && dot > 1 )
                                return 1;
                        if( indir )
                                continue;
                        if( inver )
                                return 1;
                        if( inext ) {
                                inext = 0;
                                inver = 1;
                        }
                        else {
                                inext = 1;
                                inname = 0;
                        }
                        continue;
                }
                dot = 0;


                /* If dir start and in name parts, then booboo.  If in name */
                /* and last char was closedir, then okay. */

                if( * c == '[' ) {
                        if( inname || inext || inver || indir ) {
                                if( inname && lastch == ']' ) {
                                        lastch = 0;
                                }
                                else
                                        return 1;
                        }
                        indir++;
                        if( indir > 1 )
                                return 1;
                        continue;
                };

                if( *c == ']' ) {
                        if( indir != 1 )
                                return 1;
                        indir = 0;
                        inname = 1;
                        continue;
                }
                

                if( FSMinlist( *c, chset ))
                        continue;

                return 1;
        }

/*
 *      See if any "rules" were broken by not completing the string
 *      correctly.
 */

        if( indir )
                return 1;

        if( inquote )
                return 1;

        if( reqnode )
                return 1;

        *lenptr = len;
        return len ? 0 : 1;
}
/*<PAGE>*/

/*
 *      Given a pointer to a string area, determine if the next item
 *      in the string is a valid date/time.  If so, then parse it until
 *      you find the end of the string, and return the length.  Returns
 *      zero if found, else nonzero if not a date.
 *
 *      THIS IS A HOST-SPECIFIC ROUTINE AND MUST BE IMPLEMENTED CORRECTLY
 *      FOR THE GIVEN HOST.
 */

#define DAY 1
#define MONTH 2
#define YEAR 3

static long FSMisdate( char * p, int * lenptr, char * fmt )
{

        static char * monthchars = "JANFEBMRPYUNLGSOCTVD";
        static char * digitchars = "0123456789";
        static char * delims = " /\n\t,)";

        static char * months[] = {
                "JAN", "FEB", "MAR", "APR", "MAY", "JUN",
                "JUL", "AUG", "SEP", "OCT", "NOV", "DEC" };
        static int  mlens[] = {
                31,    28,    31,    30,    31,    30,
                31,    31,    30,    31,    30,    31 };

        int state;
        char m[ 32 ];
        char t[ 32 ];
        char b[ 4 ];
        long day, year, n, len;

        char * c;

        state = DAY;
        t[ 0 ] = 0;
        m[ 0 ] = 0;
        b[ 1 ] = 0;
        day = 0;
        year = 0;
        len = 0;
        *lenptr = 0;

        for( c = p; *c; len++, c++ ) {

                if( FSMinlist( *c, delims ))
                        break;

                switch( state ) {

                case DAY:
                case YEAR:

                        if( FSMinlist( *c, monthchars ) || *c == '-' ) {
                                if( state == YEAR )
                                        return 1;
                                DCLatoi( t, &day, DCL_NONE );
                                t[ 0 ] = 0;
                                state = MONTH;
                                if( *c != '-' )
                                        c--;
                                continue;
                        }

                        if( FSMinlist( *c, digitchars )) {
                                b[ 0 ] = *c;
                                strcat( t, b );
                                continue;
                        }

                        return 1;

                case MONTH:
                        if( FSMinlist( *c, digitchars ) || *c == '-' ) {
                                state = YEAR;
                                if( *c != '-' )
                                        c--;
                                continue;
                        }

                        if( FSMinlist( *c, monthchars )) {
                                if( strlen( m ) < 3 ) {
                                        b[ 0 ] = *c;
                                        strcat( m, b );
                                }
                                continue;
                        }
                        return 1;
                }

        }

        if( state != YEAR )
                return 1;
        DCLatoi( t, &year, DCL_NONE );

        if( year < 0 )
                return 1;

        if( year < 100 ) {
                if( year < 10 )
                        year = 2000 + year;
                else
                        year = 1900 + year;
        }

        if( year < 1900 || year > 2099 )
                return 1;

        for( n = 0; n < 12; n++ ) {

                if( strcmp( m, months[ n ] ) == 0 )
                        break;
        }

        if( n >= 12 )
                return 1;

        if( day < 1 || day > mlens[ n ] )
                return 1;

        sprintf( fmt, "%ld-%s-%ld", day, m, year );
        *lenptr = (int) len;
        return 0;
        
}
/*<PAGE>*/

/*
 *      Callbacks for error messages encountered during specific parts of
 *      the processing.  "dcldeferr" should never be called, it indicates
 *      that defining the grammar language failed.  "userdeferr" indicates
 *      that an error occurred while processing the grammar for the
 *      statements we really care about.
 */

static long dcldeferr( char * msg )
{

        printf( "DCL FATAL INTERNAL ERROR -- %s\n", msg );
        return DCL_INTERNAL;
}

static long userdeferr( char * msg )
{

        printf( "Error in user defined grammar: %s\n", msg );
        return DCL_NOERROR;
}

