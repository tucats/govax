

#define FAB_C_BID 3                     /* code for fab                     */
#define FAB_M_PPF_RAT 16320
#define FAB_M_PPF_IND 16384
#define FAB_M_ASY 1
#define FAB_M_MXV 2
#define FAB_M_SUP 4
#define FAB_M_TMP 8
#define FAB_M_TMD 16
#define FAB_M_DFW 32
#define FAB_M_SQO 64
#define FAB_M_RWO 128
#define FAB_M_POS 256
#define FAB_M_WCK 512
#define FAB_M_NEF 1024
#define FAB_M_RWC 2048
#define FAB_M_DMO 4096
#define FAB_M_SPL 8192
#define FAB_M_SCF 16384
#define FAB_M_DLT 32768
#define FAB_M_NFS 65536
#define FAB_M_UFO 131072
#define FAB_M_PPF 262144
#define FAB_M_INP 524288
#define FAB_M_CTG 1048576
#define FAB_M_CBT 2097152
#define FAB_M_SYNCSTS 4194304
#define FAB_M_RCK 8388608
#define FAB_M_NAM 16777216
#define FAB_M_CIF 33554432
#define FAB_M_ESC 134217728
#define FAB_M_TEF 268435456
#define FAB_M_OFP 536870912
#define FAB_M_KFO 1073741824
#define FAB_M_PUT 1
#define FAB_M_GET 2
#define FAB_M_DEL 4
#define FAB_M_UPD 8
#define FAB_M_TRN 16
#define FAB_M_BIO 32
#define FAB_M_BRO 64
#define FAB_M_EXE 128
#define FAB_M_SHRPUT 1
#define FAB_M_SHRGET 2
#define FAB_M_SHRDEL 4
#define FAB_M_SHRUPD 8
#define FAB_M_MSE 16
#define FAB_M_NIL 32
#define FAB_M_UPI 64
#define FAB_M_ORG 240
#define FAB_C_SEQ 0                     /* sequential                       */
#define FAB_C_REL 16                    /* relative                         */
#define FAB_C_IDX 32                    /* indexed                          */
#define FAB_C_HSH 48                    /* hashed                           */
#define FAB_M_FTN 1
#define FAB_M_CR 2
#define FAB_M_PRN 4
#define FAB_M_BLK 8
#define FAB_C_RFM_DFLT 2                /* var len is default               */
#define FAB_C_UDF 0                     /* undefined (also stream binary)   */
#define FAB_C_FIX 1                     /* fixed length records             */
#define FAB_C_VAR 2                     /* variable length records          */
#define FAB_C_VFC 3                     /* variable fixed control           */
#define FAB_C_STM 4                     /* RMS-11 stream (valid only for sequential org)  */
#define FAB_C_STMLF 5                   /* LF stream (valid only for sequential org)  */
#define FAB_C_STMCR 6                   /* CR stream (valid only for sequential org)  */
#define FAB_C_MAXRFM 6                  /* maximum rfm supported            */
#define FAB_M_ONLY_RU 1
#define FAB_M_RU 2
#define FAB_M_BI 4
#define FAB_M_AI 8
#define FAB_M_AT 16
#define FAB_M_NEVER_RU 32
#define FAB_M_JOURNAL_FILE 64
#define FAB_M_RCF_RU 1
#define FAB_M_RCF_AI 2
#define FAB_M_RCF_BI 4
#define FAB_K_BLN 80                    /* length of fab                    */
#define FAB_C_BLN 80                    /* length of fab                    */
struct FAB {
    unsigned char fab_b_bid;            /* block id                         */
    unsigned char fab_b_bln;            /* block len                        */
    union  {
        unsigned short int fab_w_ifi;   /* internal file index              */
        struct  {
            unsigned int fabdef___fill_1 : 6; /* move to bit 6                  */
            unsigned int fab_v_ppf_rat : 8; /* rat value for process-permanent files  */
            unsigned int fab_v_ppf_ind : 1; /* indirect access to process-permanent file  */
                                        /* (i.e., restricted operations)              */
            unsigned int fab_v_fill_0 : 1;
            } fab_r_ifi_bits;
        } fab_r_ifi_overlay;
    union  {
        unsigned long int fab_l_fop;    /* file options                     */
        struct  {
            unsigned int fab_v_asy : 1;     /* asynchronous operations          */
            unsigned int fab_v_mxv : 1;     /* maximize version number          */
            unsigned int fab_v_sup : 1;     /* supersede existing file          */
            unsigned int fab_v_tmp : 1;     /* create temporary file            */
            unsigned int fab_v_tmd : 1;     /* create temp file marked for delete  */
            unsigned int fab_v_dfw : 1;     /* deferred write (rel and idx)     */
            unsigned int fab_v_sqo : 1;     /* sequential access only           */
            unsigned int fab_v_rwo : 1;     /* rewind mt on open                */
            unsigned int fab_v_pos : 1;     /* use next magtape position        */
            unsigned int fab_v_wck : 1;     /* write checking                   */
            unsigned int fab_v_nef : 1;     /* inhibit end of file positioning  */
            unsigned int fab_v_rwc : 1;     /* rewind mt on close               */
            unsigned int fab_v_dmo : 1;     /* dismount mt on close (not implemented)  */
            unsigned int fab_v_spl : 1;     /* spool file on close              */
            unsigned int fab_v_scf : 1;     /* submit command file on close     */
            unsigned int fab_v_dlt : 1;     /* delete sub-option                */
            unsigned int fab_v_nfs : 1;     /* non-file structured operation    */
            unsigned int fab_v_ufo : 1;     /* user file open - no rms operations  */
            unsigned int fab_v_ppf : 1;     /* process permanent file (pio segment)  */
            unsigned int fab_v_inp : 1;     /* process-permanent file is 'input'  */
            unsigned int fab_v_ctg : 1;     /* contiguous extension             */
            unsigned int fab_v_cbt : 1;     /* contiguous best try              */
            unsigned int fab_v_syncsts : 1; /* Synchronous status notification for asynchronous routines. */
            unsigned int fab_v_rck : 1;     /* read checking                    */
            unsigned int fab_v_nam : 1;     /* use name block dvi, did, and/or fid fields for open  */
            unsigned int fab_v_cif : 1;     /* create if non-existent           */
            unsigned int fabdef___fill_3 : 1; /* reserved (was UFM bitfield)    */
            unsigned int fab_v_esc : 1;     /* 'escape' to non-standard function (_modify)  */
            unsigned int fab_v_tef : 1;     /* truncate at eof on close (write-accessed seq. disk file only)  */
            unsigned int fab_v_ofp : 1;     /* output file parse (only name type sticky)  */
            unsigned int fab_v_kfo : 1;     /* known file open (image activator only release 1)  */
            unsigned int fabdef___fill_4 : 1; /* reserved (not implemented)     */
            } fab_r_fop_bits;
        } fab_r_fop_overlay;
    unsigned long int fab_l_sts;        /* status                           */
    unsigned long int fab_l_stv;        /* status value                     */
    unsigned long int fab_l_alq;        /* allocation quantity              */
    unsigned short int fab_w_deq;       /* default allocation quantity      */
    union  {
        unsigned char fab_b_fac;        /* file access                      */
        struct  {
            unsigned int fab_v_put : 1;     /* put access                       */
            unsigned int fab_v_get : 1;     /* get access                       */
            unsigned int fab_v_del : 1;     /* delete access                    */
            unsigned int fab_v_upd : 1;     /* update access                    */
            unsigned int fab_v_trn : 1;     /* truncate access                  */
            unsigned int fab_v_bio : 1;     /* block i/o access                 */
            unsigned int fab_v_bro : 1;     /* block and record i/o access      */
            unsigned int fab_v_exe : 1;     /* execute access (caller must be exec or kernel mode,  */
/*  ufo must also be set)                                                   */
            } fab_r_fac_bits;
        } fab_r_fac_overlay;
    union  {
        unsigned char fab_b_shr;        /* file sharing                     */
        struct  {
            unsigned int fab_v_shrput : 1;  /* put access                       */
            unsigned int fab_v_shrget : 1;  /* get access                       */
            unsigned int fab_v_shrdel : 1;  /* delete access                    */
            unsigned int fab_v_shrupd : 1;  /* update access                    */
            unsigned int fab_v_mse : 1;     /* multi-stream connects enabled    */
            unsigned int fab_v_nil : 1;     /* no sharing                       */
            unsigned int fab_v_upi : 1;     /* user provided interlocking (allows multiple  */
/*  writers to seq. files)                                                  */
            unsigned int fab_v_fill_1 : 1;
            } fab_r_shr_bits;
        } fab_r_shr_overlay;
    unsigned long int fab_l_ctx;        /* user context                     */
/*-----*****                                                                */
    char fab_b_rtv;                     /* retrieval window size            */
    union  {
        unsigned char fab_b_org;        /* file organization                */
        struct  {
            unsigned int fabdef___fill_5 : 4;
            unsigned int fab_v_org : 4;
            } fab_r_org_bits;
        } fab_r_org_overlay;
    union  {
        unsigned char fab_b_rat;        /* record format                    */
        struct  {
            unsigned int fab_v_ftn : 1;     /* fortran carriage-ctl             */
            unsigned int fab_v_cr : 1;      /* lf-record-cr carriage ctl        */
            unsigned int fab_v_prn : 1;     /* print-file carriage ctl          */
            unsigned int fab_v_blk : 1;     /* records don't cross block boundaries  */
            unsigned int fab_v_fill_2 : 4;
            } fab_r_rat_bits;
        } fab_r_rat_overlay;
    unsigned char fab_b_rfm;            /* record format                    */
    union  {
	unsigned int fab_l_jnl;		/* lcb address */
	struct {
	    union  {
		unsigned char fab_b_journal;    /* journaling options (from FH2_B_JOURNAL) */
		struct  {               /* note: only one of RU, ONLY_RU, NEVER_RU */
/* may be set at a time                                                     */
		    unsigned int fab_v_only_ru : 1; /* file is accessible only in recovery unit  */
	            unsigned int fab_v_ru : 1;      /* enable recovery unit journal     */
	            unsigned int fab_v_bi : 1;      /* enable before image journal      */
	            unsigned int fab_v_ai : 1;      /* enable after image journal       */
	            unsigned int fab_v_at : 1;      /* enable audit trail journal       */
	            unsigned int fab_v_never_ru : 1; /* file is never accessible in recovery unit */
	            unsigned int fab_v_journal_file : 1; /* this is a journal file      */
	            unsigned int fab_v_fill_3 : 1;
	            } fab_r_journal_bits;
	        } fab_r_journal_overlay;
	    unsigned char fab_b_ru_facility;    /* recoverable facility id number   */
	    short int fabdef___fill_7;          /* (spare)                          */
	    } fab_l_jnl_real_stuff;
	} fab_l_jnl_overlay;
    LONGWORD fab_l_xab;		        /* xab address (VAX address)        */
    LONGWORD fab_l_nam;      	        /* nam block address (VAX address)  */
    LONGWORD fab_l_fna;			/* file name string address (VAX addr) */
    LONGWORD fab_l_dna;			/* default file name string addr (VAX addr) */
    unsigned char fab_b_fns;            /* file name string size            */
    unsigned char fab_b_dns;            /* default name string size         */
    unsigned short int fab_w_mrs;       /* maximum record size              */
    unsigned long int fab_l_mrn;        /* maximum record number            */
    unsigned short int fab_w_bls;       /* blocksize for tape               */
    unsigned char fab_b_bks;            /* bucket size                      */
    unsigned char fab_b_fsz;            /* fixed header size                */
    unsigned long int fab_l_dev;        /* device characteristics           */
    unsigned long int fab_l_sdc;        /* spooling device characteristics  */
    unsigned short int fab_w_gbc;       /* Global buffer count              */
    union  {
        unsigned char fab_b_acmodes;    /* agent access modes               */
        struct  {
            unsigned int fab_v_lnm_mode : 2; /* ACMODE for log nams             */
            unsigned int fab_v_chan_mode : 2; /* ACMODE for channel             */
            unsigned int fab_v_file_mode : 2; /* ACMODE to use for determining file accessibility */
            unsigned int fab_v_callers_mode : 2; /* ACMODE for user structure probing; */
/* maximized with actual mode of caller                                     */
            } fab_r_acmodes_bits;
        } fab_r_acmodes_overlay;
    union  {                    /* recovery control flags           */
        unsigned char fab_b_rcf;        /* (only for use by RMS Recovery)   */
        struct  {
            unsigned int fab_v_rcf_ru : 1;  /* recovery unit recovery           */
            unsigned int fab_v_rcf_ai : 1;  /* after image recovery             */
            unsigned int fab_v_rcf_bi : 1;  /* before image recovery            */
            unsigned int fab_v_fill_4 : 5;
            } fab_r_rcf_bits;
        } fab_r_rcf_overlay;
    long int fabdef___fill_9;           /* (spare)                          */
    } ;
 
/* These are tradtional macros that should be hand-maintained for compatibility */

#define FAB_V_PPF_RAT	6		/* rat value for process-permanent files */
#define FAB_S_PPF_RAT	8
#define FAB_V_PPF_IND	14		/* indirect access to process-permanent file */
#define FAB_V_MXV	1		/* maximize version number */
#define FAB_V_SUP	2		/* supersede existing file */
#define FAB_V_TMP	3		/* create temporary file */
#define FAB_V_TMD	4		/* temporary file marked for delete */
#define FAB_V_DFW	5		/* deferred write (rel and idx) */
#define FAB_V_SQO	6		/* sequential access only */
#define FAB_V_RWO	7		/* rewind magnetic tape on open */
#define FAB_V_POS	8		/* use next magnetic tape position */
#define FAB_V_WCK	9		/* write checking */
#define FAB_V_NEF	10		/* not end of file, inihibit eof positioning */
#define FAB_V_RWC	11		/* rewind magnetic tape on close */
#define FAB_V_DMO	12		/* dismount mt on close (not implemented) */
#define FAB_V_SPL	13		/* spool file on close */
#define FAB_V_SCF	14		/* submit command file on close */
#define FAB_V_DLT	15		/* delete file */
#define FAB_V_NFS	16		/* non-file-structured operation */
#define FAB_V_UFO	17		/* user file open - no rms operation */
#define FAB_V_PPF	18		/* process permanent file (pio segment) */
#define FAB_V_INP	19		/* process permanent file is 'input' */
#define FAB_V_CTG	20		/* contiguous extension */
#define FAB_V_CBT	21		/* contiguous best try */
#define FAB_V_JNL	22		/* explicit logging (not implemented) */
#define FAB_M_JNL	(1 << FAB_V_JNL)
#define FAB_V_RCK	23		/* read checking */
#define FAB_V_NAM	24		/* use NAM block device, file and/or directory id */
#define FAB_V_CIF	25		/* create if non-existent */
#define FAB_V_UFM	26		/* user file open mode (user if 1, super if 0) enable only if esc and (ufo or nfs) are also on */
#define FAB_M_UFM	(1 << FAB_V_UFM)
#define FAB_V_ESC	27		/* 'escape' to non-standard functions (_modify) */
#define FAB_V_TEF	28		/* truncate at end-of-file on close (write-accessed seq. disk file only) */
#define FAB_V_OFP	29		/* output file parse (only name type sticky) */
#define FAB_V_KFO	30		/* known file open (image activator only release 1) */
#define FAB_V_PUT	0		/* put access */
#define FAB_V_GET	1		/* get access */
#define FAB_V_DEL	2		/* delete access */
#define FAB_V_UPD	3		/* update access */
#define FAB_V_TRN	4		/* truncate access */
#define FAB_V_BIO	5		/* block i/o access */
#define FAB_V_BRO	6		/* block and record i/o access */
#define FAB_V_EXE	7		/* execute access (caller must be exec or kernel mode, ufo must also be set) */
#define FAB_V_SHRPUT	0		/* put access */
#define FAB_V_SHRGET	1		/* get access */
#define FAB_V_SHRDEL	2		/* delete access */
#define FAB_V_SHRUPD	3		/* update access */
#define FAB_V_MSE	4		/* multi-stream connects enabled */
#define FAB_V_NIL	5		/* no sharing */
#define FAB_V_UPI	6		/* user provided interlocking (allows multiple */
#define FAB_V_ORG	4		/* file organization */
#define FAB_S_ORG	4
#define FAB_V_FTN	0		/* FORTRAN carriage control character */
#define FAB_V_CR	1		/* line feed - record -carriage return */
#define FAB_V_PRN	2		/* print-file carriage control */
#define FAB_V_BLK	3		/* records don't cross block boundaries */
#define fab_b_dsbmsk	fab_b_acmodes	/* saved for backwards compatibility */
#define FAB_S_LNM_MODE	2		/* logical names */
#define FAB_V_LNM_MODE	0
#define FAB_S_CHAN_MODE	2		/* channel */
#define FAB_V_CHAN_MODE	2
#define FAB_S_FILE_MODE	2		/* files accessability */
#define FAB_V_FILE_MODE	4
/* The following defines were scrambled before VAX C V3.1 */
#define FAB_V_RU	1		/* (was 0) recovery unit recovery */
#define FAB_V_BI	2		/* (was 2) before image recovery */
#define FAB_V_AI	3		/* (was 1) after image recovery */

