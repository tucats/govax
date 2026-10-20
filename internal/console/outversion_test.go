package console

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOutputVersion: a command that makes a file on a volume makes it at
// the version its output specification gives. COPY to "Z.TXT;7" makes
// Z.TXT;7, a version that exists is OPENOUT and RMS$_FEX (COPY never
// supersedes), no version is the next, and ";*" keeps the input's
// version; a host file copied in takes the version given too. CREATE,
// OPEN/WRITE, @/OUTPUT=, DIRECTORY/OUTPUT=, MACRO/OBJECT=,
// LINK/EXECUTABLE=, and LIBRARY/CREATE keep a version given as well.
func TestOutputVersion(t *testing.T) {
	host := filepath.Join(t.TempDir(), "host.txt")
	if err := os.WriteFile(host, []byte("from the host\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, _ := runFileProcedure(t, false, "",
		`$ SET NOON`,
		`$ OPEN/WRITE OUT T.MAR;5`,
		`$ WRITE OUT "	.TITLE T"`,
		`$ WRITE OUT "	.ENTRY START,0"`,
		`$ WRITE OUT "	RET"`,
		`$ WRITE OUT "	.END START"`,
		`$ CLOSE OUT`,
		`$ COPY T.MAR Z.TXT;7`,
		`$ COPY T.MAR Z.TXT;7`,
		`$ COPY T.MAR Z.TXT`,
		`$ COPY T.MAR;5 Y.TXT;*`,
		`$ COPY "`+host+`"/HOST H.TXT;2`,
		`$ MACRO/OBJECT=T.OBJ;9 T`,
		`$ LINK/EXECUTABLE=T.EXE;11 T.OBJ;9`,
		`$ CREATE C.TXT;3`,
		`hello`,
		`$ DIRECTORY/OUTPUT=D.LIS;4 T.MAR`,
		`$ LIBRARY/CREATE L.OLB;6 T.OBJ;9`,
		`$ OPEN/WRITE OUT E.COM`,
		`$ WRITE OUT "$ SHOW DEFAULT"`,
		`$ CLOSE OUT`,
		`$ @E/OUTPUT=E.LOG;12`,
		`$ WRITE SYS$OUTPUT F$SEARCH("T.MAR;5")`,
		`$ WRITE SYS$OUTPUT F$SEARCH("Z.TXT;7")`,
		`$ WRITE SYS$OUTPUT F$SEARCH("Z.TXT;8")`,
		`$ WRITE SYS$OUTPUT F$SEARCH("Y.TXT;5")`,
		`$ WRITE SYS$OUTPUT F$SEARCH("H.TXT;2")`,
		`$ WRITE SYS$OUTPUT F$SEARCH("T.OBJ;9")`,
		`$ WRITE SYS$OUTPUT F$SEARCH("T.EXE;11")`,
		`$ WRITE SYS$OUTPUT F$SEARCH("C.TXT;3")`,
		`$ WRITE SYS$OUTPUT F$SEARCH("D.LIS;4")`,
		`$ WRITE SYS$OUTPUT F$SEARCH("L.OLB;6")`,
		`$ WRITE SYS$OUTPUT F$SEARCH("E.LOG;12")`,
	)

	want := "%COPY-S-COPIED, T.MAR;5 copied to DUA0:[000000]Z.TXT;7\n" +
		"%COPY-E-OPENOUT, error opening DUA0:[000000]Z.TXT;7 as output\n" +
		"-RMS-E-FEX, file already exists, not superseded\n" +
		"%COPY-S-COPIED, T.MAR;5 copied to DUA0:[000000]Z.TXT;8\n" +
		"%COPY-S-COPIED, T.MAR;5 copied to DUA0:[000000]Y.TXT;5\n" +
		"%COPY-S-COPIED, " + host + " copied to DUA0:[000000]H.TXT;2\n" +
		"DUA0:[000000]T.MAR;5\n" +
		"DUA0:[000000]Z.TXT;7\n" +
		"DUA0:[000000]Z.TXT;8\n" +
		"DUA0:[000000]Y.TXT;5\n" +
		"DUA0:[000000]H.TXT;2\n" +
		"DUA0:[000000]T.OBJ;9\n" +
		"DUA0:[000000]T.EXE;11\n" +
		"DUA0:[000000]C.TXT;3\n" +
		"DUA0:[000000]D.LIS;4\n" +
		"DUA0:[000000]L.OLB;6\n" +
		"DUA0:[000000]E.LOG;12\n"
	if out != want {
		t.Errorf("the procedure wrote:\n%s\nwant:\n%s", out, want)
	}
}
