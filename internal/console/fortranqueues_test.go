package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/testproc"
)

// fortranQueues is a FORTRAN program that works the batch queues through
// the system services, with the definition modules Phase 98's close-out
// added ($QUIDEF, $SJCDEF, $JBCMSGDEF): it creates and starts a queue by
// $SNDJBCW, then lists every queue by $GETQUIW's wildcard search until
// JBC$_NOMOREQUE.
const fortranQueues = `      PROGRAM QUEUES
      INCLUDE '($QUIDEF)'
      INCLUDE '($SJCDEF)'
      INCLUDE '($JBCMSGDEF)'
      INCLUDE '($SSDEF)'
      INCLUDE '($SYSSRVNAM)'
      STRUCTURE /ITEM/
        INTEGER*2 LEN, CODE
        INTEGER*4 ADDR, RETLEN
      END STRUCTURE
      RECORD /ITEM/ LIST(5)
      INTEGER*4 IOSB(2), STATUS, LIMIT, FLAGS, NAMELEN
      CHARACTER*31 NAME, SEARCH
      CHARACTER*13 QUEUE
      DATA QUEUE /'FORTRAN_BATCH'/

C     Create the queue, a batch queue with a job limit of 3.
      LIMIT = 3
      LIST(1).LEN = LEN(QUEUE)
      LIST(1).CODE = SJC$_QUEUE
      LIST(1).ADDR = %LOC(QUEUE)
      LIST(1).RETLEN = 0
      LIST(2).LEN = 0
      LIST(2).CODE = SJC$_BATCH
      LIST(2).ADDR = 0
      LIST(2).RETLEN = 0
      LIST(3).LEN = 4
      LIST(3).CODE = SJC$_JOB_LIMIT
      LIST(3).ADDR = %LOC(LIMIT)
      LIST(3).RETLEN = 0
      LIST(4).LEN = 0
      LIST(4).CODE = 0
      STATUS = SYS$SNDJBCW(, %VAL(SJC$_CREATE_QUEUE), , LIST, IOSB, ,)
      IF (STATUS) STATUS = IOSB(1)
      IF (STATUS .NE. JBC$_NORMAL) CALL LIB$STOP(%VAL(STATUS))

C     Start it.
      LIST(2).CODE = 0
      STATUS = SYS$SNDJBCW(, %VAL(SJC$_START_QUEUE), , LIST, IOSB, ,)
      IF (STATUS) STATUS = IOSB(1)
      IF (STATUS .NE. JBC$_NORMAL) CALL LIB$STOP(%VAL(STATUS))

C     Every queue, its name and job limit.
      SEARCH = '*'
      FLAGS = QUI$M_SEARCH_WILDCARD
      LIST(1).LEN = 1
      LIST(1).CODE = QUI$_SEARCH_NAME
      LIST(1).ADDR = %LOC(SEARCH)
      LIST(2).LEN = 4
      LIST(2).CODE = QUI$_SEARCH_FLAGS
      LIST(2).ADDR = %LOC(FLAGS)
      LIST(3).LEN = 31
      LIST(3).CODE = QUI$_QUEUE_NAME
      LIST(3).ADDR = %LOC(NAME)
      LIST(3).RETLEN = %LOC(NAMELEN)
      LIST(4).LEN = 4
      LIST(4).CODE = QUI$_JOB_LIMIT
      LIST(4).ADDR = %LOC(LIMIT)
      LIST(4).RETLEN = 0
      LIST(5).LEN = 0
      LIST(5).CODE = 0
      DO WHILE (.TRUE.)
        STATUS = SYS$GETQUIW(, %VAL(QUI$_DISPLAY_QUEUE), , LIST,
     1    IOSB, ,)
        IF (STATUS) STATUS = IOSB(1)
        IF (STATUS .EQ. JBC$_NOMOREQUE) GO TO 10
        IF (.NOT. STATUS) CALL LIB$STOP(%VAL(STATUS))
        PRINT 100, NAME(1:NAMELEN), LIMIT
      END DO
   10 PRINT *, 'NO MORE QUEUES'
  100 FORMAT (1X, A, ' JOB LIMIT ', I2)
      END
`

// TestFortranQueues compiles and runs fortranQueues beside SYS$BATCH: the
// modules' names are the services' codes, and the queue it made is the
// manager's.
func TestFortranQueues(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	_, dispatch := batchConsole(t)

	src := filepath.Join(t.TempDir(), "queues.for")
	if err := os.WriteFile(src, []byte(fortranQueues), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{
		"INITIALIZE/QUEUE/BATCH/JOB_LIMIT=2 SYS$BATCH",
		`COPY "` + src + `"/HOST QUEUES.FOR/QUIET`,
		"FORTRAN QUEUES",
		"MACRO QUEUES",
		"LINK QUEUES",
	} {
		if got := dispatch(line); got != "" {
			t.Fatalf("%s: %s", line, got)
		}
	}

	got := dispatch("RUN QUEUES")
	want := "FORTRAN_BATCH JOB LIMIT  3\nSYS$BATCH JOB LIMIT  2\nNO MORE QUEUES\n"

	if got != want {
		t.Errorf("RUN QUEUES:\n%s\nwant:\n%s", got, want)
	}

	if got := dispatch("SHOW QUEUE FORTRAN_BATCH"); !strings.Contains(got, "Batch queue FORTRAN_BATCH, idle,") {
		t.Errorf("SHOW QUEUE FORTRAN_BATCH:\n%s", got)
	}
}
