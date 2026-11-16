# The Phase 98 probe: batch queues

These fixtures are `docs/planning/PHASE-98 - batch queues.md`'s subtask
2 (its Part D). They ask VMS 7.1 four things:

- **The queue commands' words**: INITIALIZE/QUEUE, START, STOP (with
  `/NEXT`, `/RESET`, `/ENTRY`, `/REQUEUE`), SET QUEUE, DELETE/QUEUE, SHOW
  QUEUE in each form, SUBMIT with each qualifier govax will keep, SHOW
  ENTRY, SET ENTRY, DELETE/ENTRY, SYNCHRONIZE, SET and SHOW
  SECURITY/CLASS=QUEUE, with their errors and `$STATUS`; on queues of the
  probe's own, `BAT98_*`.
- **What a batch job is**: its process, rights, names, parameters,
  F$GETJPI's and F$GETQUI's answers, its log file (where it goes, what is
  in it), and how it ends (several files, an error, `/CPUTIME`, aborted,
  deleted, stopped, requeued with `/RESTART`).
- **Who may do what**: the queue commands as three probe accounts against
  a queue in six protection states, one of them once per privilege.
- **The services**: `$SNDJBC` and `$GETQUI` called directly, their item
  codes and statuses, the JBC facility's messages, and the values of
  `$SJCDEF`, `$QUIDEF`, and `$JBCMSGDEF`.

| File | What it holds |
| ---- | ------------- |
| `gen.go` | Writes everything below but this README and `decode.go`: `go run testdata/bat98/gen.go` from the repository root |
| `bat98.com` | SYSTEM's run: every case, in sections (QUEUES, JOBS ON A STOPPED QUEUE, RUNNING JOBS, ENDINGS, THE JOB LIMIT, RETAINED JOBS, SYNCHRONIZE, DELETE, AFTER ON A RUNNING QUEUE, LOG FILES, ORDER, RESTART, THE JOB'S VIEW, SERVICES, PROTECTION) |
| `bat98acct.com` | Makes the probe's accounts in the VAX's SYSUAF and their homes |
| `bat98drop.com` | The end: deletes the probe's queues, removes the accounts and identifiers |
| `bat98z.com` | Deletes one of the probe's queues (P1), or all of them |
| `bat98j.com` | A batch job's view of itself, written to its log |
| `bat98p.com` | The protection matrix: the queue commands, as whoever runs it, once per privilege pass |
| `bat98ok.com`, `bat98wrn.com`, `bat98err.com`, `bat98fat.com`, `bat98bad.com` | Procedures that end with success, a warning, an error, a severe error, and an unknown command |
| `bat98w.com` | A job that writes a marker file and runs for P2 seconds at most |
| `bat98c.com` | A job that uses CPU time, 15 seconds at most |
| `bat98o.com` | A job that writes its name at the end of `[BAT98SCR]ORDER.TXT` |
| `bat98r.com` | A job that can be restarted (SET RESTART_VALUE) |
| `login_*.com` | Each account's LOGIN.COM, in its home |
| `bat98q.mar` | `$SNDJBCW` and `$GETQUIW` called directly, 59 cases, then the JBC messages by `$GETMSG` |
| `def_sjc.mar`, `def_qui.mar`, `def_jbc.mar` | `$SJCDEF`, `$QUIDEF`, `$JBCMSGDEF` each called `GLOBAL`: their values from the object, no listing |
| `decode.go` | Turns `vax/def_*.obj` into `defined.txt`, names and values only |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts that build the exchange volume and bring the results back |

Every wait in the run is bounded. A job SYSTEM waits for with
SYNCHRONIZE always ends by itself (the longest runs a minute), and a wait
for a job to *start* looks for its marker file 30 times, 2 seconds apart,
then goes on. Nothing waits on a held job or a stopped queue.

## The accounts

`BAT98ACCT.COM` makes them with AUTHORIZE, in the VAX's own SYSUAF, and
`BAT98DROP.COM` removes them. Each has the password `BAT98PW` (nothing
logs in with it: a batch job's login asks for none, so it may expire),
owner `GOVAX PROBE`, account `GOVAX98`, device `BAT98$DISK:`, a home
directory of its own, no LGICMD (so its login runs `SYS$LOGIN:LOGIN.COM`),
and TMPMBX and NETMBX as its default privileges.

| Username | UIC | Home | What differs |
| -------- | --- | ---- | ------------ |
| BAT98 | `[200,10]` | `[BAT98]` | Holds the identifier BAT98_ID |
| BAT98O | `[200,11]` | `[BAT98O]` | Authorized for every privilege, so BAT98P.COM can turn each on |
| BAT98W | `[300,10]` | `[BAT98W]` | Another group |

UIC members are octal. To make them by hand instead, as SYSTEM, with the
volume mounted and `BAT98$DISK` defined (BAT98.COM defines it):

    $ SET DEFAULT SYS$SYSTEM
    $ MCR AUTHORIZE
    UAF> ADD BAT98/UIC=[200,10]/PASSWORD=BAT98PW/FLAGS=NODISUSER
    UAF> MODIFY BAT98/DEVICE=BAT98$DISK:/DIRECTORY=[BAT98]/ACCOUNT=GOVAX98/OWNER="GOVAX PROBE"
    UAF> MODIFY BAT98/PRIVILEGES=(TMPMBX,NETMBX)/DEFPRIVILEGES=(TMPMBX,NETMBX)/LGICMD=""
    UAF> COPY BAT98 BAT98O/UIC=[200,11]/PASSWORD=BAT98PW
    UAF> MODIFY BAT98O/DIRECTORY=[BAT98O]/PRIVILEGES=ALL/DEFPRIVILEGES=(TMPMBX,NETMBX)
    UAF> COPY BAT98 BAT98W/UIC=[300,10]/PASSWORD=BAT98PW
    UAF> MODIFY BAT98W/DIRECTORY=[BAT98W]
    UAF> ADD/IDENTIFIER BAT98_ID
    UAF> GRANT/IDENTIFIER BAT98_ID BAT98
    UAF> EXIT

Each home must then be owned by its account, with protection
(S:RWE,O:RWE,G:RE,W:RE), and its LOGIN.COM owned by the account too;
`BAT98ACCT.COM`'s SET SECURITY lines do that. If the VAX uses UICs
`[200,10]`, `[200,11]`, or `[300,10]`, or a username or identifier
BAT98..., say so before running. `ADD` may make a group identifier,
GOVAX98, for group 300 if the group has none; `BAT98DROP.COM` removes it.

## What the run changes on the VAX

All undone by `BAT98DROP.COM`: the three accounts in SYSUAF, BAT98_ID
(and maybe GOVAX98) in RIGHTSLIST, the system logical name `BAT98$DISK`,
and the probe's queues, `BAT98_A`, `BAT98_B`, `BAT98_P`, `BAT98_D1` to
`BAT98_D13`, and any of `BAT98_D`, `BAT98_E`, `BAT98_X`, `BAT98_Q`,
`BAT98_Q2` a case left. It never changes SYS$BATCH or any other queue of
the VAX's. It uses the VAX's queue manager, and its entry numbers.

The log file cases with the default `/PRINTER` (`BAT98G13` to `BAT98G15`)
run only when the VAX has no `SYS$PRINT` queue, so the probe never
prints. A few cases write a log in SYSTEM's own login directory
(SYS$LOGIN) by default; `FINDLOG` deletes each one it finds after typing
it into the log.

## The VAX run

1. **Build the exchange volume**, from the repository root on the host:

       rm -f testdata/disks/bat98-exchange.dsk
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/bat98/exchange.cmd

   This makes `testdata/disks/bat98-exchange.dsk` (RD53, label BAT98,
   gitignored).

2. **Attach it** to the simh VAX on a free disk unit, at the `sim>`
   prompt:

       attach rq1 testdata/disks/bat98-exchange.dsk

   The steps below say `DUA1:`; use the unit you attached.

3. **As SYSTEM**, check that the queue manager runs:

       $ SHOW QUEUE/MANAGER

   If it isn't running, start it (`START/QUEUE/MANAGER`). The probe makes
   its own queues and needs no others. Then mount the volume for every
   user, and run SYSTEM's part:

       $ MOUNT/SYSTEM DUA1: BAT98
       $ @DUA1:[000000]BAT98

   It takes a while: perhaps half an hour, most of it the protection
   matrix's 18 batch jobs. Its log is `BAT98$DISK:[BAT98LOG]BAT98.LOG`,
   whose last line is `BAT98 FINISHED`; it writes only that line to the
   terminal when it ends, **except** for `/NOTIFY`'s messages (case R7 and
   any others VMS sends): keep everything the terminal shows during the
   run, and paste it into `testdata/bat98/vax/terminal.log` on the host.

4. **By hand**, as SYSTEM, at the terminal, after BAT98.COM has finished
   and before the end (step 5), keeping the terminal's text from the
   first command to the last, to add to `terminal.log`:

   - **SYNCHRONIZE and Ctrl/Y**:

         $ SUBMIT/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME=HAND1/LOG_FILE=BAT98$DISK:[BAT98LOG]HAND1.LOG/PARAMETERS=(HAND1,60) BAT98$DISK:[000000]BAT98W
         $ SYNCHRONIZE/ENTRY='$ENTRY'

     wait a few seconds, press Ctrl/Y, then:

         $ SHOW SYMBOL $STATUS
         $ SHOW QUEUE/ALL BAT98_A

   - **A reboot** (optional; only if the VAX may be restarted). It shows
     what a job executing at a system's stop becomes, with and without
     `/RESTART` (Decision 11):

         $ SUBMIT/QUEUE=BAT98_A/RESTART/NOPRINTER/KEEP/NAME=BOOTR/LOG_FILE=BAT98$DISK:[BAT98LOG]BOOTR.LOG/PARAMETERS=BOOTR BAT98$DISK:[000000]BAT98R
         $ SUBMIT/QUEUE=BAT98_A/NOPRINTER/KEEP/NAME=BOOTN/LOG_FILE=BAT98$DISK:[BAT98LOG]BOOTN.LOG/PARAMETERS=(BOOTN,600) BAT98$DISK:[000000]BAT98W
         $ SHOW QUEUE/FULL/ALL BAT98_A

     Shut the VAX down the usual way, boot it, log in as SYSTEM, then:

         $ MOUNT/SYSTEM DUA1: BAT98
         $ DEFINE/SYSTEM/EXECUTIVE_MODE BAT98$DISK DUA1:
         $ SHOW QUEUE/FULL/ALL BAT98_A
         $ START/QUEUE BAT98_A
         $ SHOW QUEUE/FULL/ALL BAT98_A

     and a minute later `SHOW QUEUE/FULL/ALL BAT98_A` again. (BAT98R ends
     a minute after it starts, BAT98W's BOOTN ten minutes; DROP aborts
     either.)

5. **As SYSTEM**, run the end:

       $ @BAT98$DISK:[000000]BAT98DROP

   Its log is `[BAT98LOG]BAT98DROP.LOG`, ending `BAT98DROP FINISHED` (it
   deassigns `BAT98$DISK` after the log closes). Then dismount and
   detach:

       $ DISMOUNT DUA1:

       sim> detach rq1

6. **Copy the results back**, on the host, from the repository root:

       mkdir -p testdata/bat98/vax
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/bat98/copyout.cmd
       go run testdata/bat98/decode.go

   This writes the logs (`bat98.log`, `bat98drop.log`, each job's log
   that the run kept, and the steps by hand's), the definition probes'
   objects (`def_*.obj`), and `defined.txt`. A file the run didn't make
   is a message, and the rest are copied.

If a run stops partway, run `@BAT98$DISK:[000000]BAT98DROP` as SYSTEM (it
can run more than once), then start again from step 1 with a new volume.
BAT98.COM can also be run again as it is: it deletes its queues before
making them.

## The rehearsal

`TestBat98Probe` (`internal/console/bat98_test.go`) runs BAT98J.COM and
BAT98P.COM at govax's terminal and BAT98.COM to its end, so the probe
can't lose a VAX round to a mistake in its own DCL: every symbol it uses
is defined, every label it goes to is there, and BAT98ACCT.COM's
AUTHORIZE commands are govax's as well as VMS's. govax has no queues yet,
so each queue command is a message there, and each wait returns at once
when its SUBMIT made no entry. BAT98Q.MAR was assembled and linked by
govax's MACRO and LINK with stand-in values for the three definition
macros, which govax lacks until subtask 3, so it has no mistake MACRO
can find.
