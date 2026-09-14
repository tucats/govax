/*
 *      Module:         PUBLIC header for DCL parser.
 *
 *      Author:         Tom Cole, Aug93
 */



/****************************************************************************\
 *                                                                          *
 *                          PUBLIC DEFINITIONS                              *
 *                                                                          *
\****************************************************************************/


/*      SYMBOLIC DATA           */


/*      Debugger flags.                                         */

#define DCL_NODEBUG     0               /* No debug messages during parse    */
#define DCL_DUMP        1               /* Dump diagnostic data about parse  */
#define DCL_DEBUG       2               /* Debug messages during parse       */


/*      Return codes.                                           */

#define DCL_NOERROR     1               /* No error, but parse incomplete    */
#define DCL_DONE        101             /* No error, parse complete          */
#define DCL_IVERB       102             /* Invalid verb                      */
#define DCL_UNKNOWN     103             /* Unknown keyword                   */
#define DCL_TYPERR      104             /* Wrong value type                  */
#define DCL_SYNTAX      105             /* General syntax error              */
#define DCL_MISSPARM    106             /* Missing required parameter        */
#define DCL_MISSQUAL    107             /* Missing required qualifier        */
#define DCL_AMBKEY      108             /* Ambigious keyword                 */
#define DCL_CANTNEG     109             /* Invalid negated keyword or qual   */
#define DCL_INVCOMB     110             /* Invalid combination of qualifiers */
#define DCL_EOF         997             /* End of file on input stream       */
#define DCL_NODISPATCH  998             /* Can't dispatch, no bound routine  */
#define DCL_INTERNAL    999             /* Internal programmer boo-boo       */


/*      Type of return operation in callback.                   */

#define DCL_CALLBACK_VERB       'V'     /* Callback to store verb            */
#define DCL_CALLBACK_QUALIFIER  'Q'     /* Callback to store qualifier       */
#define DCL_CALLBACK_PARAMETER  'P'     /* Callback to store parameter       */


/*      Return data types given to call in callbacks            */

#define DCL_ANY         '*'             /* Datatype can be anything          */
#define DCL_NAME        'N'             /* Datatype must be a name (ident)   */
#define DCL_INTEGER     'I'             /* Datatype must be an integer       */
#define DCL_STRING      'S'             /* Datatype must be quoted string    */
#define DCL_KEYWORD     'K'             /* Datatype must be a keyword        */
#define DCL_FILENAME    'F'             /* Datatype must be a filename       */
#define DCL_DATE        'D'             /* Datatype must be a date           */

#define DCL_NONE        0               /* No value, type, or flag           */

/*      Bind operator types.                                    */

#define DCL_BIND_VERB      1            /* DCLbind is called to bind verb    */
#define DCL_BIND_SYNTAX    2            /* DCLbind is called to bind syntax  */
#define DCL_BIND_PARAMETER 3            /*    "          "        parameter  */
#define DCL_BIND_QUALIFIER 4            /*    "          "        qualifier  */

/*      This is a predefined callback value that tells DCL$LIB to read       */
/*      input from the terminal using a host routine.                        */

#define DCL_INPUT       (( long (*)(void)) -1L )


/*      STRUCTURE DEFINITIONS   */

/*      This is the structure (actually returned as an array of structures)  */
/*      containing the parse results.  This can be used as an alternative    */
/*      to the dump routine(s) to get the results of a parse back.           */

struct DCL_RESULT_ELEMENT {
        int kind;                       /* DCL_CALLBACK_xxx callback type    */
        char * name;                    /* Name of parameter/qualifier       */
        long id;                        /* ID of value being returned        */
        long datatype;                  /* Datatype expected.                */
        long i_value;                   /* Numeric value or zero             */
        char * c_value;                 /* Character value or emptystring    */
};

struct DCL_RESULTS {
        long    count;                  /* Number of elements */
        char    * char_data;            /* pointer to character pool */
        struct DCL_RESULT_ELEMENT element[ 1 ];
                                        /* Parse elements (count elements) */
};


/*      ROUTINE PROTOTYPES      */


/*      Read a file that contains grammar statements and process it.  This   */
/*      handles the I/O and calls the DCLdefine() processor for each stmt.   */

long DCLread(
        char * fname                    /* Name of file to read              */
        );


/*      Parse a grammar statement and store it away.  Grammar statements     */
/*      must be sent in order according to language requirements or an       */
/*      error will result.                                                   */

long DCLdefine(
        char * statement                /* Grammar statement to store away   */
        );


/*      Parse a command using the stored named grammar.  The text pointer    */
/*      may be null, in which case DCLparse() will prompt for a string.      */

long DCLparse( 
        char * grammar,                 /* Name of grammar to use for parse  */
        char * text,                    /* Text to parse                     */
        long (*callback)(void)          /* Routine to call to get more text  */
        );

/*      This is a sample callback that simply outputs the data to the user   */
/*      terminal.  It is typically used for debugging.                       */

long DCLcallback_debug( 
        int kind,                       /* DCL_CALLBACK_xxx callback type    */
        char * name,                    /* Name of parameter/qualifier       */
        long id,                        /* ID of value being returned        */
        long dtyp,                      /* Datatype expected.                */
        long i_value,                   /* Numeric value or zero             */
        char * c_value                  /* Character value or emptystring    */
        );

/*      This can be used instead of the callbacks.  It allocates a struct    */
/*      that contains the individual results of the parse.  Pass the         */
/*      address of a null pointer the first time you call this routine to    */
/*      ensure that memory is allocated correctly.  The storage is re-used   */
/*      if the same pointer is passed back repeatedly.                       */

long DCLresults( struct DCL_RESULTS ** result_array );


/*      After DCLdump() has been called, this routine should be called to    */
/*      release storage and reset attributes of parse elements before the    */
/*      next parse call is made.  After this call, the grammar has no        */
/*      context and a new DCLparse() call must be made.                      */

long DCLreset( void );



/*      This routine can be used to diagnose a return code value and print   */
/*      it out as text.  This is for debugging purposes only.                */

long DCLdebugrc(
        char *  routine,                /* Name of routine, used in message  */
        long    rc                      /* Return code to print in message   */
        );


/*      This lets the user declare a routine to be used to format and put    */
/*      messages to the user.  By default, messages are printed directly     */
/*      on the user's terminal.  This overrides them.  The callback routine  */
/*      must accept a single null-terminated string pointer as a parameter   */
/*      and process it properly.                                             */

long DCLsignal(
               long (*callback)(void)              /* Callback routine to signal with   */
        );


/*      This calls the bound routine associated with the current verb.       */

long DCLdispatch( void );


/*      This binds a routine to a named verb or alternate syntax definition  */
/*      in the specified grammar.                                            */

long DCLbind(
        int  kind,                      /* DCL_BIND_xxx type code            */
        char * grammar,                 /* Name of grammer containing verb   */
        char * verb,                    /* Name of item to bind to           */
        void * addr                     /* Address of item to bind           */
        );


/*      Validate that the current grammar is valid and consistent, i.e check */
/*      for unresolved forward references, dangling structures, etc. that    */
/*      might be caused by an incomplete or incorrect grammar spec.  This    */
/*      is called automatically from DCLread() and should be called by the   */
/*      user directly if DCLdefine() calls are made to modify a grammar.     */

long DCLvalidate( void );


/*      Diagnostic routine that displays memory use by DCL code.             */

long DCLmemstats( 
        int reset                       /* If nonzero, resets statistics     */
        );


/*      Set DCL and FSM debugging flags.  Typically not used...              */

long DCLsetdebug(
        long    dcldebug,               /* DCL_NODEBUG or DCL_DEBUG          */
        long    fsmdebug                /* DCL_NODEBUG or DCL_DEBUG          */
        );


/*      Query the current grammar to see the state of a specific item by id  */

long DCLpresent(
        long    kind,                   /* DCL_CALLBACK_xxx                  */
        long    id                      /* Id of the item to check on        */
        );

/*      Get the value of something that's a string or integer.               */

char * DCLgetstring(
        long    kind,                   /* DCL_CALLBACK_xxx                  */
        long    id                      /* ID of item to return              */
        );

long DCLgetinteger(
        long    kind,                   /* DCL_CALLBACK_xxx                  */
        long    id                      /* ID of item to return              */
        );

/*      For items that support keywords, query to see if the keyword was     */
/*      specified (i.e. get it's state).                                     */

long DCLgetkeyword(
        long    kind,                   /* DCL_CALLBACK_xxx                  */
        long    id,                     /* ID of parm or qualifier           */
        long    keyid                   /* ID of keyword being tested for    */
        );

/*      Given a grammar, write out a header file for all the /ID values      */
/*      associated with the grammar.                                         */

long DCLwriteheader(
        char    * fname,                /* Name of file to write.            */
        char    * grammar               /* Name of grammar to write          */
        );

/*      Return the /ENTRY= value for the active verb, if any.               */

char * DCLgetentry( void );

/*      Write the last-parsed data out environment variables/symbols         */

long DCLexport( void );

/*      Set the journalling file for storing processed grammar data or       */
/*      a null pointer to turn journalling off again.                        */

long DCLjournal( char * journal_name, char * routine_name );

/*      MACRO DEFINITIONS       */


/*      A macro to test a return value to see if it's okay from the DCL     */
/*      routines.  Checks for "done" and "noerror", anything else is error  */

#define DCLERROR( rc )          (!(((rc) == DCL_DONE)||((rc) == DCL_NOERROR)))

#define DCLABS( x )  (( (x) < 0 ) ? -(x) : ( x ))
