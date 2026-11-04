package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// runIdx88D runs round 4 of testdata/idx88, IDX88D.COM (DCL on indexed files;
// docs/PHASE-88 - keyed index files.md, subtask 9), on govax, on a fresh volume
// of VMS's 3-block clusters as DUA1 with node SIMVAX, as the VAX's was, and
// returns the log it wrote and what reached the terminal.
func runIdx88D(t *testing.T) (log, terminal string) {
	t.Helper()

	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	c.RTL.NodeName = "SIMVAX"
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	dispatch := func(line string) {
		t.Helper()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("%s: %v", line, err)
		}
	}

	dsk := filepath.Join(t.TempDir(), "idx88d.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 /CLUSTER=3 IDX88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	for _, name := range []string{"idx88d.mar", "idx88d.com"} {
		src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "idx88", name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST ` + strings.ToUpper(name) + "/QUIET")
	}

	buf.Reset()
	dispatch("@IDX88D/OUTPUT=IDX88D.LOG")

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]IDX88D.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	for _, r := range records {
		b.Write(r)
		b.WriteByte('\n')
	}

	return b.String(), strings.ReplaceAll(buf.String(), "\r", "")
}

// TestIdx88DProbe rehearses round 4 on govax, so that a mistake of ours can't
// cost the author a VAX run: IDX88D.MAR must assemble, link, and make its files,
// and the procedure must run through every section to its end. Until VMS's log
// is here, it also holds govax to what the DCL Dictionary says READ and WRITE
// do on an indexed file (each command, then the lines it showed, as the log
// has them). IDX88D_LOG=file keeps govax's log in the file.
func TestIdx88DProbe(t *testing.T) {
	log, _ := runIdx88D(t)

	if out := os.Getenv("IDX88D_LOG"); out != "" {
		if err := os.WriteFile(out, []byte(log), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, want := range []string{
		"$ RUN IDX88D\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X00010001\"\n",

		// WRITE in any order, and a duplicate primary key refused.
		"$ WRITE F \"K00010GRPBaaZZZ one\"\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X00010001\"\n",
		"$ WRITE F \"K00010GRPZzzZZZ a second K00010\"\n%RMS-F-DUP,",

		// READ in sequence is the primary key's order.
		"  R = \"K00010GRPBaaZZZ one\"\n$ GOTO D2LOOP\n$ D2LOOP:\n$ R = \"-\"\n$ READ/END_OF_FILE=D2END/ERROR=D2END F R\n" +
			"$ SHOW SYMBOL R\n  R = \"K00020GRPAbbMMM two\"\n",

		// /KEY, then on in sequence; generic; /MATCH=GT.
		"$ READ/KEY=\"K00030\" F R\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X00030001\"\n$ SHOW SYMBOL R\n" +
			"  R = \"K00030GRPBbbMMM three\"\n$ R = \"-\"\n$ READ F R\n$ SHOW SYMBOL R\n  R = \"K00040GRPAccAAA four\"\n",
		"$ READ/KEY=\"K0002\" F R\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X00030001\"\n$ SHOW SYMBOL R\n  R = \"K00020",
		"$ READ/KEY=\"K00030\"/MATCH=GT F R\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X00030001\"\n$ SHOW SYMBOL R\n  R = \"K00040",

		// /INDEX's key, its duplicates in the order written, and on in its order.
		"$ READ/INDEX=1/KEY=\"GRPB\" F R\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X00030001\"\n$ SHOW SYMBOL R\n" +
			"  R = \"K00010GRPBaaZZZ one\"\n$ R = \"-\"\n$ READ F R\n$ SHOW SYMBOL R\n  R = \"K00030GRPBbbMMM three\"\n",

		// A descending key's GT is the next lower value.
		"$ READ/INDEX=3/KEY=\"MMM\"/MATCH=GT F R\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X00030001\"\n$ SHOW SYMBOL R\n  R = \"K00040",

		// READ/DELETE, and the record gone.
		"$ READ/INDEX=0/DELETE/KEY=\"K00040\" F R\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X00030001\"\n",
		"$ READ/KEY=\"K00040\" F R\n%RMS-E-RNF,",

		// WRITE/UPDATE longer, and the primary key unchanged.
		"$ READ/KEY=\"K00020\" F R\n$ SHOW SYMBOL R\n  R = \"K00020GRPAbbMMM two, made longer\"\n",
		"$ WRITE/UPDATE F \"K00021GRPCbbMMM changed\"\n%RMS-F-CHG,",

		// A record another stream has locked, and /NOLOCK.
		"$ READ/KEY=\"K00050\" B R\n%RMS-E-RLK,",
		"$ READ/NOLOCK/KEY=\"K00050\" B R\n$ SHOW SYMBOL $STATUS\n  $STATUS == \"%X00030001\"\n",

		// TYPE and DIRECTORY/FULL.
		"$ TYPE D2.IDX\nAAAA the only AAAA..\nBBBB first of BBBB..\nBBBB second of BBBB.\n",
		"File organization:  Indexed, Prolog: 3, Using 4 keys\n",

		"  M = \"END\"\n",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("no %q in the log", want)
		}
	}
}
