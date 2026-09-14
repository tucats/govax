/*
 *  instruction_table.h
 *
 *
 *  This header file contains the table of VAX instructions and what the
 *  Emulator must know about each of them.  This table was originally 
 *  created by hand, but is now generated _by_ the emulator itself with
 *  a TEST 11001 console command.  This allows us to modify the structure
 *  of the table easily without hand-editing the entire file.
 *
 *  THEREFORE, DO NOT CHANGE THIS FILE BY HAND, BUT INSTEAD MODIFY THE
 *  BEHAVIOR OF THE console_test.c MODULE TO GENERATE THE NEW DATA!
 */

struct INSTRUCTION instruction[] = {

{   /* HALT     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x00,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "HALT"
},

{   /* NOP      */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x01,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "NOP"
},

{   /* REI      */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x02,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "REI"
},

{   /* BPT      */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x03,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BPT"
},

{   /* RET      */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x04,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "RET"
},

{   /* RSB      */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x05,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "RSB"
},

{   /* LDPCTX   */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x06,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "LDPCTX"
},

{   /* SVPCTX   */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x07,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SVPCTX"
},

{   /* CVTPS    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x08,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTPS"
},

{   /* CVTSP    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x09,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTSP"
},

{   /* INDEX    */
        {  4,  4,  4,  4,  4,  4 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x0A,     /* Opcode (extended+normal)      */
        6,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_RD, OP_RD, OP_WR ),
        0L, 0L,          /* Profiling data here           */
        "INDEX"
},

{   /* CRC      */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x0B,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CRC"
},

{   /* PROBER   */
        {  1,  2,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x0C,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_AD, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "PROBER"
},

{   /* PROBEW   */
        {  1,  2,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x0D,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_AD, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "PROBEW"
},

{   /* INSQUE   */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x0E,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_AD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "INSQUE"
},

{   /* REMQUE   */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x0F,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_AD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "REMQUE"
},

{   /* BSBB     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x10,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BSBB"
},

{   /* BRB      */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x11,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BRB"
},

{   /* BNEQ     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x12,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BNEQ"
},

{   /* BEQL     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x13,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BEQL"
},

{   /* BGTR     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x14,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BGTR"
},

{   /* BLEQ     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x15,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BLEQ"
},

{   /* JSB      */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x16,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_AD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "JSB"
},

{   /* JMP      */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x17,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_AD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "JMP"
},

{   /* BGEQ     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x18,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BGEQ"
},

{   /* BLSS     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x19,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BLSS"
},

{   /* BGTRU    */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x1A,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BGTRU"
},

{   /* BLEQU    */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x1B,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BLEQU"
},

{   /* BVC      */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x1C,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BVC"
},

{   /* BVS      */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x1D,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BVS"
},

{   /* BCC      */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x1E,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BCC"
},

{   /* BCS      */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x1F,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BCS"
},

{   /* ADDP4    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x20,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDP4"
},

{   /* ADDP6    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x21,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDP6"
},

{   /* SUBP4    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x22,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBP4"
},

{   /* SUBP6    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x23,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBP6"
},

{   /* CVTPT    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x24,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTPT"
},

{   /* MULP     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x25,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULP"
},

{   /* CVTTP    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x26,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTTP"
},

{   /* DIVP     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x27,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVP"
},

{   /* MOVC3    */
        {  2,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x28,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_AD, OP_AD, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVC3"
},

{   /* CMPC3    */
        {  2,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x29,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_AD, OP_AD, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPC3"
},

{   /* SCANC    */
        {  2,  1,  1,  1,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x2A,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_AD, OP_AD, OP_RD, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SCANC"
},

{   /* SPANC    */
        {  2,  1,  1,  1,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x2B,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_AD, OP_AD, OP_RD, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SPANC"
},

{   /* MOVC5    */
        {  2,  1,  1,  2,  1,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x2C,     /* Opcode (extended+normal)      */
        5,              /* Number of operands            */
        ACCESS( OP_RD, OP_AD, OP_RD, OP_RD, OP_AD, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVC5"
},

{   /* CMPC5    */
        {  2,  1,  1,  2,  1,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x2D,     /* Opcode (extended+normal)      */
        5,              /* Number of operands            */
        ACCESS( OP_RD, OP_AD, OP_RD, OP_RD, OP_AD, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPC5"
},

{   /* MOVTC    */
        {  2,  1,  1,  1,  2,  1 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x2E,     /* Opcode (extended+normal)      */
        6,              /* Number of operands            */
        ACCESS( OP_RD, OP_AD, OP_RD, OP_AD, OP_RD, OP_AD ),
        0L, 0L,          /* Profiling data here           */
        "MOVTC"
},

{   /* MOVTUC   */
        {  2,  1,  1,  1,  2,  1 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x2F,     /* Opcode (extended+normal)      */
        6,              /* Number of operands            */
        ACCESS( OP_RD, OP_AD, OP_RD, OP_AD, OP_RD, OP_AD ),
        0L, 0L,          /* Profiling data here           */
        "MOVTUC"
},

{   /* BSBW     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x30,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BSBW"
},

{   /* BRW      */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x31,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_BR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BRW"
},

{   /* CVTWL    */
        {  2,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x32,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTWL"
},

{   /* CVTWB    */
        {  2,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x33,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTWB"
},

{   /* MOVP     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x34,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVP"
},

{   /* CMPP3    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x35,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPP3"
},

{   /* CVTPL    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x36,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTPL"
},

{   /* CMPP4    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x37,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPP4"
},

{   /* EDITPC   */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x38,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EDITPC"
},

{   /* MATCHC   */
        {  2,  1,  2,  1,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x39,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_AD, OP_RD, OP_AD, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MATCHC"
},

{   /* LOCC     */
        {  1,  2,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x3A,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_AD, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "LOCC"
},

{   /* SKPC     */
        {  1,  2,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x3B,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_AD, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SKPC"
},

{   /* MOVZWL   */
        {  2,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x3C,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVZWL"
},

{   /* ACBW     */
        {  2,  2,  2,  2,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x3D,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_MD, OP_BR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ACBW"
},

{   /* MOVAW    */
        {  2,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x3E,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVAW"
},

{   /* PUSHAW   */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x3F,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_AD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "PUSHAW"
},

{   /* ADDF2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x40,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDF2"
},

{   /* ADDF3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x41,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDF3"
},

{   /* SUBF2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x42,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBF2"
},

{   /* SUBF3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x43,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBF3"
},

{   /* MULF2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x44,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULF2"
},

{   /* MULF3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x45,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULF3"
},

{   /* DIVF2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x46,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVF2"
},

{   /* DIVF3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x47,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVF3"
},

{   /* CVTFB    */
        {  4,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x48,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTFB"
},

{   /* CVTFW    */
        {  4,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x49,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTFW"
},

{   /* CVTFL    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x4A,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTFL"
},

{   /* CVTRFL   */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x4B,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTRFL"
},

{   /* CVTBF    */
        {  1,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x4C,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTBF"
},

{   /* CVTWF    */
        {  2,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x4D,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTWF"
},

{   /* CVTLF    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x4E,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTLF"
},

{   /* ACBF     */
        {  4,  4,  4,  2,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x4F,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_MD, OP_BR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ACBF"
},

{   /* MOVF     */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x50,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVF"
},

{   /* CMPF     */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x51,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPF"
},

{   /* MNEGF    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x52,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MNEGF"
},

{   /* TSTF     */
        {  4,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x53,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "TSTF"
},

{   /* EMODF    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x54,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EMODF"
},

{   /* POLYF    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x55,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "POLYF"
},

{   /* CVTFD    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x56,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTFD"
},

{   /* RSVD_57  */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x57,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "RSVD_57"
},

{   /* ADAWI    */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x58,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADAWI"
},

{   /* RSVD_59  */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x59,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "RSVD_59"
},

{   /* RSVD_5A  */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x5A,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "RSVD_5A"
},

{   /* RSVD_5B  */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x5B,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "RSVD_5B"
},

{   /* INSQHI   */
        {  1,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x5C,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_AD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "INSQHI"
},

{   /* INSQTI   */
        {  1,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x5D,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_AD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "INSQTI"
},

{   /* REMQHI   */
        {  1,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x5E,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_AD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "REMQHI"
},

{   /* REMQTI   */
        {  1,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x5F,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_AD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "REMQTI"
},

{   /* ADDD2    */
        {  8,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x60,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDD2"
},

{   /* ADDD3    */
        {  8,  8,  8,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x61,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDD3"
},

{   /* SUBD2    */
        {  8,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x62,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBD2"
},

{   /* SUBD3    */
        {  8,  8,  8,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x63,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBD3"
},

{   /* MULD2    */
        {  8,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x64,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULD2"
},

{   /* MULD3    */
        {  8,  8,  8,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x65,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULD3"
},

{   /* DIVD2    */
        {  8,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x66,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVD2"
},

{   /* DIVD3    */
        {  8,  8,  8,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x67,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVD3"
},

{   /* CVTDB    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x68,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTDB"
},

{   /* CVTDW    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x69,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTDW"
},

{   /* CVTDL    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x6A,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTDL"
},

{   /* CVTRDL   */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x6B,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTRDL"
},

{   /* CVTBD    */
        {  1,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x6C,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTBD"
},

{   /* CVTWD    */
        {  2,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x6D,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTWD"
},

{   /* CVTLD    */
        {  4,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x6E,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTLD"
},

{   /* ACBD     */
        {  8,  8,  8,  2,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x6F,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_MD, OP_BR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ACBD"
},

{   /* MOVD     */
        {  8,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x70,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVD"
},

{   /* CMPD     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x71,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPD"
},

{   /* MNEGD    */
        {  8,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x72,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MNEGD"
},

{   /* TSTD     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x73,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "TSTD"
},

{   /* EMODD    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x74,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EMODD"
},

{   /* POLYD    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x75,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "POLYD"
},

{   /* CVTDF    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x76,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTDF"
},

{   /* RSVD_77  */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x77,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "RSVD_77"
},

{   /* ASHL     */
        {  1,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x78,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ASHL"
},

{   /* ASHQ     */
        {  1,  8,  8,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x79,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ASHQ"
},

{   /* EMUL     */
        {  4,  4,  4,  8,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x7A,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_WR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EMUL"
},

{   /* EDIV     */
        {  4,  8,  4,  4,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x7B,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_WR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EDIV"
},

{   /* CLRQ     */
        {  8,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x7C,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_WR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CLRQ"
},

{   /* MOVQ     */
        {  8,  8,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x7D,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVQ"
},

{   /* MOVAQ    */
        {  8,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x7E,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVAQ"
},

{   /* PUSHAQ   */
        {  8,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x7F,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_AD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "PUSHAQ"
},

{   /* ADDB2    */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x80,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDB2"
},

{   /* ADDB3    */
        {  1,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x81,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDB3"
},

{   /* SUBB2    */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x82,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBB2"
},

{   /* SUBB3    */
        {  1,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x83,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBB3"
},

{   /* MULB2    */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x84,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULB2"
},

{   /* MULB3    */
        {  1,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x85,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULB3"
},

{   /* DIVB2    */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x86,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVB2"
},

{   /* DIVB3    */
        {  1,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x87,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVB3"
},

{   /* BISB2    */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x88,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BISB2"
},

{   /* BISB3    */
        {  1,  1,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x89,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BISB3"
},

{   /* BICB2    */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x8A,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BICB2"
},

{   /* BICB3    */
        {  1,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x8B,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BICB3"
},

{   /* XORB2    */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x8C,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "XORB2"
},

{   /* XORB3    */
        {  1,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x8D,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "XORB3"
},

{   /* MNEGB    */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x8E,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MNEGB"
},

{   /* CASEB    */
        {  1,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x8F,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CASEB"
},

{   /* MOVB     */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x90,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVB"
},

{   /* CMPB     */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x91,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPB"
},

{   /* MCOMB    */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x92,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MCOMB"
},

{   /* BITB     */
        {  1,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x93,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BITB"
},

{   /* CLRB     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x94,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_WR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CLRB"
},

{   /* TSTB     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x95,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "TSTB"
},

{   /* INCB     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x96,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_MD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "INCB"
},

{   /* DECB     */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x97,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_MD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DECB"
},

{   /* CVTBL    */
        {  1,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x98,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTBL"
},

{   /* CVTBW    */
        {  1,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x99,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTBW"
},

{   /* MOVZBL   */
        {  1,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x9A,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVZBL"
},

{   /* MOVZBW   */
        {  1,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x9B,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVZBW"
},

{   /* ROTL     */
        {  1,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x9C,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ROTL"
},

{   /* ACBB     */
        {  1,  1,  1,  2,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x9D,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_MD, OP_BR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ACBB"
},

{   /* MOVAB    */
        {  1,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x9E,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVAB"
},

{   /* PUSHAB   */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0x9F,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_AD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "PUSHAB"
},

{   /* ADDW2    */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xA0,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDW2"
},

{   /* ADDW3    */
        {  2,  2,  2,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xA1,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDW3"
},

{   /* SUBW2    */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xA2,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBW2"
},

{   /* SUBW3    */
        {  2,  2,  2,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xA3,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBW3"
},

{   /* MULW2    */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xA4,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULW2"
},

{   /* MULW3    */
        {  2,  2,  2,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xA5,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULW3"
},

{   /* DIVW2    */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xA6,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVW2"
},

{   /* DIVW3    */
        {  2,  2,  2,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xA7,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVW3"
},

{   /* BISW2    */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xA8,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BISW2"
},

{   /* BISW3    */
        {  2,  2,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xA9,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BISW3"
},

{   /* BICW2    */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xAA,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BICW2"
},

{   /* BICW3    */
        {  2,  2,  2,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xAB,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BICW3"
},

{   /* XORW2    */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xAC,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "XORW2"
},

{   /* XORW3    */
        {  2,  2,  2,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xAD,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "XORW3"
},

{   /* MNEGW    */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xAE,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MNEGW"
},

{   /* CASEW    */
        {  2,  2,  2,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xAF,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CASEW"
},

{   /* MOVW     */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xB0,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVW"
},

{   /* CMPW     */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xB1,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPW"
},

{   /* MCOMW    */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xB2,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MCOMW"
},

{   /* BITW     */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xB3,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BITW"
},

{   /* CLRW     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xB4,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_WR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CLRW"
},

{   /* TSTW     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xB5,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "TSTW"
},

{   /* INCW     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xB6,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_MD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "INCW"
},

{   /* DECW     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xB7,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_MD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DECW"
},

{   /* BISPSW   */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xB8,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BISPSW"
},

{   /* BICPSW   */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xB9,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BICPSW"
},

{   /* POPR     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xBA,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "POPR"
},

{   /* PUSHR    */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xBB,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "PUSHR"
},

{   /* CHMK     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xBC,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CHMK"
},

{   /* CHME     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xBD,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CHME"
},

{   /* CHMS     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xBE,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CHMS"
},

{   /* CHMU     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xBF,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CHMU"
},

{   /* ADDL2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xC0,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDL2"
},

{   /* ADDL3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xC1,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDL3"
},

{   /* SUBL2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xC2,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBL2"
},

{   /* SUBL3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xC3,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBL3"
},

{   /* MULL2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xC4,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULL2"
},

{   /* MULL3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xC5,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULL3"
},

{   /* DIVL2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xC6,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVL2"
},

{   /* DIVL3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xC7,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVL3"
},

{   /* BISL2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xC8,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BISL2"
},

{   /* BISL3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xC9,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BISL3"
},

{   /* BICL2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xCA,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BICL2"
},

{   /* BICL3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xCB,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BICL3"
},

{   /* XORL2    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xCC,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "XORL2"
},

{   /* XORL3    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xCD,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_WR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "XORL3"
},

{   /* MNEGL    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xCE,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MNEGL"
},

{   /* CASEL    */
        {  4,  4,  4,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xCF,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CASEL"
},

{   /* MOVL     */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xD0,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVL"
},

{   /* CMPL     */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xD1,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPL"
},

{   /* MCOML    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xD2,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MCOML"
},

{   /* BITL     */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xD3,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BITL"
},

{   /* CLRL     */
        {  4,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xD4,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_WR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CLRL"
},

{   /* TSTL     */
        {  4,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xD5,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "TSTL"
},

{   /* INCL     */
        {  4,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xD6,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_MD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "INCL"
},

{   /* DECL     */
        {  4,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xD7,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_MD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DECL"
},

{   /* ADWC     */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xD8,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADWC"
},

{   /* SBWC     */
        {  2,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xD9,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SBWC"
},

{   /* MTPR     */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xDA,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MTPR"
},

{   /* MFPR     */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xDB,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MFPR"
},

{   /* MOVPSL   */
        {  4,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xDC,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_WR, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVPSL"
},

{   /* PUSHL    */
        {  4,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xDD,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_RD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "PUSHL"
},

{   /* MOVAL    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xDE,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_WR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVAL"
},

{   /* PUSHAL   */
        {  4,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xDF,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_AD, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "PUSHAL"
},

{   /* BBS      */
        {  4,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xE0,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_BR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BBS"
},

{   /* BBC      */
        {  4,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xE1,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_BR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BBC"
},

{   /* BBSS     */
        {  4,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xE2,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_BR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BBSS"
},

{   /* BBCS     */
        {  4,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xE3,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_BR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BBCS"
},

{   /* BBSC     */
        {  4,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xE4,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_BR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BBSC"
},

{   /* BBCC     */
        {  4,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xE5,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_BR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BBCC"
},

{   /* BBSSI    */
        {  4,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xE6,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_BR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BBSSI"
},

{   /* BBCCI    */
        {  4,  1,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xE7,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_BR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BBCCI"
},

{   /* BLBS     */
        {  4,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xE8,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_BR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BLBS"
},

{   /* BLBC     */
        {  4,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xE9,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_BR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BLBC"
},

{   /* FFS      */
        {  4,  1,  1,  4,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xEA,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_WR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "FFS"
},

{   /* FFC      */
        {  4,  1,  1,  4,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xEB,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_WR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "FFC"
},

{   /* CMPV     */
        {  4,  1,  1,  4,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xEC,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_RD, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPV"
},

{   /* CMPZV    */
        {  4,  1,  1,  4,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xED,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_RD, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPZV"
},

{   /* EXTV     */
        {  4,  1,  1,  4,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xEE,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_WR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EXTV"
},

{   /* EXTZV    */
        {  4,  1,  1,  4,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xEF,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_WR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EXTZV"
},

{   /* INSV     */
        {  4,  4,  1,  1,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xF0,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_RD, OP_WR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "INSV"
},

{   /* ACBL     */
        {  4,  4,  4,  2,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xF1,     /* Opcode (extended+normal)      */
        4,              /* Number of operands            */
        ACCESS( OP_RD, OP_RD, OP_MD, OP_BR, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ACBL"
},

{   /* AOBLSS   */
        {  4,  4 , 1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xF2,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_BR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "AOBLSS"
},

{   /* AOBLEQ   */
        {  4,  4,  1,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xF3,     /* Opcode (extended+normal)      */
        3,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_BR, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "AOBLEQ"
},

{   /* SOBGEQ   */
        {  4,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xF4,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_MD, OP_BR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SOBGEQ"
},

{   /* SOBGTR   */
        {  4,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xF5,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_MD, OP_BR, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SOBGTR"
},

{   /* CVTLB    */
        {  4,  1,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xF6,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTLB"
},

{   /* CVTLW    */
        {  4,  2,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xF7,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_MD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTLW"
},

{   /* ASHP     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xF8,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ASHP"
},

{   /* CVTLP    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xF9,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTLP"
},

{   /* CALLG    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xFA,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_AD, OP_AD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CALLG"
},

{   /* CALLS    */
        {  4,  4,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xFB,     /* Opcode (extended+normal)      */
        2,              /* Number of operands            */
        ACCESS( OP_RD, OP_AD, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CALLS"
},

{   /* XFC      */
        {  1,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xFC,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_IM, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "XFC"
},

{   /* EXT_FD   */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xFD,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EXT_FD"
},

{   /* EXT_FE   */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xFE,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EXT_FE"
},

{   /* EXT_FF   */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0x00, 0xFF,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EXT_FF"
},

{   /* CVTDH    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x32,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTDH"
},

{   /* ADDG2    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x40,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDG2"
},

{   /* ADDG3    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x41,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ADDG3"
},

{   /* SUBG2    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x42,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBG2"
},

{   /* SUBG3    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x43,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "SUBG3"
},

{   /* MULG2    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x44,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULG2"
},

{   /* MULG3    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x45,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MULG3"
},

{   /* DIVG2    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x46,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVG2"
},

{   /* DIVG3    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x47,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "DIVG3"
},

{   /* CVTGB    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x48,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTGB"
},

{   /* CVTGW    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x49,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTGW"
},

{   /* CVTGL    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x4A,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTGL"
},

{   /* CVTRGL   */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x4B,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTRGL"
},

{   /* CVTBG    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x4C,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTBG"
},

{   /* CVTWG    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x4D,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTWG"
},

{   /* CVTLG    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x4E,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTLG"
},

{   /* ACBG     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x4F,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "ACBG"
},

{   /* MOVG     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x50,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MOVG"
},

{   /* CMPG     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x51,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CMPG"
},

{   /* MNEGG    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x52,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "MNEGG"
},

{   /* TSTG     */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x53,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "TSTG"
},

{   /* EMODG    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x54,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "EMODG"
},

{   /* POLYG    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x55,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "POLYG"
},

{   /* CVTGH    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x56,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTGH"
},

{   /* CVTFH    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0x98,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTFH"
},

{   /* CVTHF    */
        {  0,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_FLOAT,  /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFD, 0xF6,     /* Opcode (extended+normal)      */
        0,              /* Number of operands            */
        ACCESS( OP_NL, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "CVTHF"
},

{   /* BUGL     */
        {  4,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFF, 0xFD,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_IM, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BUGL"
},

{   /* BUGW     */
        {  2,  0,  0,  0,  0,  0 },   /*  operand scales */
        OP_TYPE_INT,    /* data type for short literals  */
        0L,             /* Entry point for emulator      */
        0xFF, 0xFE,     /* Opcode (extended+normal)      */
        1,              /* Number of operands            */
        ACCESS( OP_IM, OP_NL, OP_NL, OP_NL, OP_NL, OP_NL ),
        0L, 0L,          /* Profiling data here           */
        "BUGW"
},

/* End of array denoted by null name */
{   { 0, 0, 0, 0, 0, 0 },   0,  0L, 0X00,   0X00,   0, { 0,0,0,0,0,0 },0, 0, ""  }};
