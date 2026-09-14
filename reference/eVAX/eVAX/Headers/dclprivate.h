
/*
 *      This is the storage used by the callbacks for the DCL language
 *      itself.  Each component of a command stores data in the appropriate
 *      part of this structure.  A final call to DCLdefine_element() processes the
 *      data stored in the structure.  This executes a single DCL definition
 *      call to the grammar the user is creating.
 */

struct DCLARGBLK {

        int             kind;
        long            id;
        char            name[ 32 ];
        int             typ;
        int             req;
        int             suffix;
        int             value;
        int             list;
        int             has_def;
        int             negate;
        char            keys[ 32 ];
        char            syntax[ 32 ];
        char            alias[ 32 ];
        char            imply[ 32 ];
        char            local[ 32 ];
        char            entry[ 64 ];
        long            min;
        long            max;
        long            i_def;
        char            c_def[ 128 ];
        char            prompt[ 128 ];

};

#ifndef DCLGLOBALS
extern
#endif

struct DCLARGBLK dcl_private_store;

long DCLdefine_element( void );

