

/*                                                                          */
/*         record access block (rab) definitions                            */
/*                                                                          */
/*  there is one rab per connected stream                                   */
/*  it is used for all communications between the user                      */
/*  and rms concerning operations on the stream                             */
/*                                                                          */

#define RAB_C_BID 1                     /* code for rab                     */
#define RAB_M_PPF_RAT 16320
#define RAB_M_PPF_IND 16384
#define RAB_M_ASY 1
#define RAB_M_TPT 2
#define RAB_M_REA 4
#define RAB_M_RRL 8
#define RAB_M_UIF 16
#define RAB_M_MAS 32
#define RAB_M_FDL 64
#define RAB_M_HSH 128
#define RAB_M_EOF 256
#define RAB_M_RAH 512
#define RAB_M_WBH 1024
#define RAB_M_BIO 2048
#define RAB_M_CDK 4096
#define RAB_M_LOA 8192
#define RAB_M_LIM 16384
#define RAB_M_SYNCSTS 32768
#define RAB_M_LOC 65536
#define RAB_M_WAT 131072
#define RAB_M_ULK 262144
#define RAB_M_RLK 524288
#define RAB_M_NLK 1048576
#define RAB_M_KGE 2097152
#define RAB_M_KGT 4194304
#define RAB_M_NXR 8388608
#define RAB_M_RNE 16777216
#define RAB_M_TMO 33554432
#define RAB_M_CVT 67108864
#define RAB_M_RNF 134217728
#define RAB_M_ETO 268435456
#define RAB_M_PTA 536870912
#define RAB_M_PMT 1073741824
#define RAB_M_CCO -2147483648
#define RAB_M_EQNXT 2097152
#define RAB_M_NXT 4194304
#define RAB_C_SEQ 0                     /* sequential access                */
#define RAB_C_KEY 1                     /* keyed access                     */
#define RAB_C_RFA 2                     /* rfa access                       */
#define RAB_C_STM 3                     /* stream access (valid only for sequential org)  */
#define RAB_C_MAXRAC 2                  /* Maximum RAC value currently supported by RMS */
#define RAB_K_BLN 68                    /* length of rab                    */
#define RAB_C_BLN 68                    /* length of rab                    */

struct RAB {
    unsigned char rab_b_bid;            /* block id                         */
    unsigned char rab_b_bln;            /* block length                     */
    union  {
        unsigned short int rab_w_isi;   /* internal stream index            */
/* (ifi in fab)                                                             */
        struct  {
            unsigned int rabdef___fill_1 : 6; /* move to bit 6                  */
            unsigned int rab_v_ppf_rat : 8; /* rat value for process-permanent files  */
            unsigned int rab_v_ppf_ind : 1; /* indirect access to process-permanent file  */
/* (i.e., restricted operations)                                            */
            unsigned int rab_v_fill_5 : 1;
            } rab_r_isi_bits;
        } rab_r_isi_overlay;
    union  {
        unsigned long int rab_l_rop;    /* record options                   */
        struct  {
            unsigned int rab_v_asy : 1;     /* asynchronous operations          */
            unsigned int rab_v_tpt : 1;     /* truncate put - allow sequential put not at  */
/*   eof, thus truncating file (seq. org only)                              */
/*                                                                          */
/* these next two should be in the byte for bits                            */
/* input to _find or _get, but there is no room there                       */
/*                                                                          */
            unsigned int rab_v_rea : 1;     /* lock record for read only, allow other readers  */
            unsigned int rab_v_rrl : 1;     /* read record regardless of lock   */
/*                                                                          */
            unsigned int rab_v_uif : 1;     /* update if existent               */
            unsigned int rab_v_mas : 1;     /* mass-insert mode                 */
            unsigned int rab_v_fdl : 1;     /* fast record deletion             */
            unsigned int rab_v_hsh : 1;     /* use hash code in bkt             */
/*                                                                          */
            unsigned int rab_v_eof : 1;     /* connect to eof                   */
            unsigned int rab_v_rah : 1;     /* read ahead                       */
            unsigned int rab_v_wbh : 1;     /* write behind                     */
            unsigned int rab_v_bio : 1;     /* connect for bio only             */
            unsigned int rab_v_cdk : 1;     /* check for duplicate keys on _GET */
            unsigned int rab_v_loa : 1;     /* use bucket fill percentage       */
            unsigned int rab_v_lim : 1;     /* compare for key limit reached on _get/_find seq. (idx only)  */
            unsigned int rab_v_syncsts : 1; /* Synchronous status notification for asynchronous routines. */
/*                                                                          */
/* the following bits are input to                                          */
/* _find or _get, (see above also REA and RRL)                              */
/* (separate byte)                                                          */
/*                                                                          */
            unsigned int rab_v_loc : 1;     /* use locate mode                  */
            unsigned int rab_v_wat : 1;     /* wait if record not available     */
            unsigned int rab_v_ulk : 1;     /* manual unlocking                 */
            unsigned int rab_v_rlk : 1;     /* allow readers for this locked record  */
            unsigned int rab_v_nlk : 1;     /* do not lock record               */
            unsigned int rab_v_kge : 1;     /* key > or =                       */
            unsigned int rab_v_kgt : 1;     /* key greater than                 */
            unsigned int rab_v_nxr : 1;     /* get non-existent record          */
/*                                                                          */
/*  the following bits are terminal qualifiers only                         */
/*  (separate byte)                                                         */
/*                                                                          */
            unsigned int rab_v_rne : 1;     /* read no echo                     */
            unsigned int rab_v_tmo : 1;     /* use time-out period              */
            unsigned int rab_v_cvt : 1;     /* convert to upper case            */
            unsigned int rab_v_rnf : 1;     /* read no filter                   */
            unsigned int rab_v_eto : 1;     /* extended terminal operation      */
            unsigned int rab_v_pta : 1;     /* purge type ahead                 */
            unsigned int rab_v_pmt : 1;     /* use prompt buffer                */
            unsigned int rab_v_cco : 1;     /* cancel control o on output       */
            } rab_r_rop_bits0;
        struct  {
            unsigned int rabdef___fill_6 : 21;
            unsigned int rab_v_eqnxt : 1;   /* Synonyms for KGE and             */
            unsigned int rab_v_nxt : 1;     /*   KGT                            */
            unsigned int rab_v_fill_6 : 1;
            } rab_r_rop_bits1;
/* the following bits may be                                                */
/* input to various rab-related                                             */
/* operations                                                               */
/*                                                                          */
        struct  {
            char rabdef___fill_3;
            unsigned char rab_b_rop1;   /* various options                  */
            unsigned char rab_b_rop2;   /* get/find options (use of this field discouraged  */
/* due to REA and RRL being in a different byte)                            */
            unsigned char rab_b_rop3;   /* terminal read options            */
/*                                                                          */
            } rab_r_rop_fields;
        } rab_r_rop_overlay;
    unsigned long int rab_l_sts;        /* status                           */
    union  {
        unsigned long int rab_l_stv;    /* status value                     */
        struct  {
            unsigned short int rab_w_stv0; /* low word of stv               */
            unsigned short int rab_w_stv2; /* high word of stv              */
            } rab_r_stv_fields;
        } rab_r_stv_overlay;
    union  {
        unsigned short int rab_w_rfa [3]; /* record's file address          */
        struct  {
            unsigned long int rab_l_rfa0;
            unsigned short int rab_w_rfa4;
            } rab_r_rfa_fields;
        } rab_r_rfa_overlay;
    short int rabdef___fill_4;          /* (reserved - rms release 1 optimizes stores  */
/*  to the rfa field to be a move quad, overwriting                         */
/*  this reserved word)                                                     */
    unsigned long int rab_l_ctx;        /* user context                     */
/*-----*****                                                                */
    short int rabdef___fill_5;          /* (spare)                          */
    unsigned char rab_b_rac;            /* record access                    */
    unsigned char rab_b_tmo;            /* time-out period                  */
    unsigned short int rab_w_usz;       /* user buffer size                 */
    unsigned short int rab_w_rsz;       /* record buffer size               */
    LONGWORD rab_l_ubf;			/* user buffer address (VAX address) */
    LONGWORD rab_l_rbf;			/* record buffer address (VAX address) */
    LONGWORD rab_l_rhb;			/* record header buffer addr (VAX address) */
    union  {
        LONGWORD rab_l_kbf;		/* key buffer address (VAX address)  */
        LONGWORD rab_l_pbf;		/* prompt buffer addr (VAX address)  */
        } rab_r_kbf_overlay;
    union  {
        unsigned char rab_b_ksz;        /* key buffer size                  */
        unsigned char rab_b_psz;        /* prompt buffer size               */
        } rab_r_ksz_overlay;
    unsigned char rab_b_krf;            /* key of reference                 */
    char rab_b_mbf;                     /* multi-buffer count               */
    unsigned char rab_b_mbc;            /* multi-block count                */
    union  {
        unsigned long int rab_l_bkt;    /* bucket hash code, vbn, or rrn    */
        unsigned long int rab_l_dct;    /* duplicates count on key accessed on alternate key  */
        } rab_r_bkt_overlay;
    LONGWORD rab_l_fab;			/* related fab for connect (VAX address) */
    LONGWORD rab_l_xab;			/* XAB address (VAX address)         */
    } ;
 
/* These are tradtional macros that should be hand-maintained for compatibility */
#define RAB_V_PPF_RAT	6		/* rat value for process-permanent files */
#define RAB_S_PPF_RAT	8
#define RAB_V_PPF_IND	14		/* indirect access to process-permanent file */
#define RAB_V_ASY	0		/* asynchronous operations */
#define RAB_V_TPT	1		/* truncate-on-put - allow sequential put not at eof */
#define RAB_V_REA	2		/* lock record for read only, allow other readers */
#define RAB_V_RRL	3		/* read record regardless of lock */
#define RAB_V_UIF	4		/* update if existent */
#define RAB_V_MAS	5		/* mass-insert mode */
#define RAB_V_FDL	6		/* fast record deletion */
#define RAB_V_HSH	7		/* use hash code in bkt */
#define RAB_V_EOF	8		/* connect to end-of-file */
#define RAB_V_RAH	9		/* read ahead */
#define RAB_V_WBH	10		/* write behind */
#define RAB_V_BIO	11		/* connect for block I/O only */
#define RAB_V_LV2	12		/* level 2 RU lock consistency */
#define RAB_M_LV2	(1 << RAB_V_LV2)
#define RAB_V_LOA	13		/* load buckets according to the file size */
#define RAB_V_LIM	14		/* compare for key limit reached on _get/_find seq.(idx only) */
#define RAB_V_LOC	16		/* use locate mode */
#define RAB_V_WAT	17		/* wait if record not available */
#define RAB_V_ULK	18		/* manual unlocking */
#define RAB_V_RLK	19		/* allow readers for this locked record */
#define RAB_V_NLK	20		/* do not lock record */
#define RAB_V_KGE	21		/* key is greater than or equal to */
#define RAB_V_KGT	22		/* key is greater than */
#define RAB_V_NXR	23		/* non-existent record processing */
#define RAB_V_RNE	24		/* read no echo */
#define RAB_V_TMO	25		/* use time-out period */
#define RAB_V_CVT	26		/* convert to upper case */
#define RAB_V_RNF	27		/* read no filter */
#define RAB_V_ETO	28		/* extended terminal operation */
#define RAB_V_PTA	29		/* purge type ahead */
#define RAB_V_PMT	30		/* use prompt buffer */
#define RAB_V_CCO	31		/* cancel control O on output */
#define RAB_V_EQNXT 	21		/*  Synonym for KGE */
#define RAB_V_NXT  	22		/*  Synonym for KGT */

