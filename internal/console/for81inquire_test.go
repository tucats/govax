package console

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// inquireAnswers are the specifiers TestFor81InquireOracle asks, by the code
// VMS's FOR$INQUIRE knows each by, which the probe's lines show. CONVERT, 48,
// waits for subtask 11.
var inquireAnswers = []struct {
	code int
	name string
}{
	{35, "ACCESS"}, {36, "SEQUENTIAL"}, {37, "DIRECT"}, {38, "FORM"}, {39, "FORMATTED"},
	{40, "UNFORMATTED"}, {43, "BLANK"}, {44, "ORGANIZATION"}, {45, "RECORDTYPE"}, {46, "KEYED"},
	{47, "CARRIAGECONTROL"},
}

// inquireReport is a FORTRAN subroutine that asks everything of a unit, or of
// a file if the unit is negative, and prints a line for each answer, as the
// probe's lines look.
func inquireReport() []string {
	lines := []string{
		"      SUBROUTINE ASK (U, NAME)",
		"      INTEGER U, NUM, RL, NR, IOS",
		"      LOGICAL EX, OP, NM",
		"      CHARACTER*(*) NAME",
		"      CHARACTER*64 FN",
	}

	var names, list []string

	for _, a := range inquireAnswers {
		v := fmt.Sprintf("Q%d", a.code)
		names = append(names, v)
		list = append(list, fmt.Sprintf("%s=%s", a.name, v))
	}

	lines = append(lines, "      CHARACTER*64 "+strings.Join(names[:6], ", "), "      CHARACTER*64 "+strings.Join(names[6:], ", "))

	common := []string{
		"     1 EXIST=EX, OPENED=OP, NUMBER=NUM, NAMED=NM, NAME=FN,",
		"     2 RECL=RL, NEXTREC=NR, IOSTAT=IOS,",
		"     3 " + strings.Join(list[:3], ", ") + ",",
		"     4 " + strings.Join(list[3:6], ", ") + ",",
		"     5 " + strings.Join(list[6:9], ", ") + ",",
		"     6 " + strings.Join(list[9:], ", ") + ")",
	}

	lines = append(lines, "      IF (U .GE. -99) THEN", "      INQUIRE (U,")
	lines = append(lines, common...)
	lines = append(lines, "      ELSE", "      INQUIRE (FILE=NAME,")
	lines = append(lines, common...)
	lines = append(lines, "      END IF", "      WRITE (*,1) IOS, EX, OP, NUM, NM, FN, RL, NR",
		"    1 FORMAT (' IOS',I4/' q30',L2/' q31',L2/' q32',I5/' q33',L2/",
		"     1 ' q34 ',A/' q41',I6/' q42',I4)")

	for _, a := range inquireAnswers {
		lines = append(lines, fmt.Sprintf("      WRITE (*,'('' q%d '',A)') Q%d", a.code, a.code))
	}

	return append(lines, "      END")
}

// TestFor81InquireOracle holds FTNRTL's INQUIRE to VMS's FORRTL: each case is
// one of testdata/for81's, in FORTRAN, and each answer must be the one in
// vax/for81-vax.txt, the device's name aside (VMS's volume was DUA2). Relative
// files wait for subtask 10 (#491, #492), a direct unit's NEXTREC moving for
// subtask 8 (#507).
// testdata/fortran/inquire.for has the places of each width (#506) and unit 6
// once it's connected (#477).
func TestFor81InquireOracle(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", "for81-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	// The answers in order, of each INQUIRE in each case.
	want := map[int][]map[string]string{}
	line := regexp.MustCompile(`^ q(\d+) r=\w+ (?:v=(\w+)|\|(.*)\|)$`)
	number := 0

	for l := range strings.Lines(string(text)) {
		l = strings.TrimRight(l, "\n")

		if strings.HasPrefix(l, "#") {
			fmt.Sscanf(l, "#%d", &number)
		}

		m := line.FindStringSubmatch(l)
		if m == nil {
			continue
		}

		if m[1] == "29" || len(want[number]) == 0 {
			want[number] = append(want[number], map[string]string{})
		}

		answers := want[number][len(want[number])-1]
		answers[m[1]] = strings.TrimRight(m[3], " ")

		if m[2] != "" {
			var v uint32

			fmt.Sscanf(m[2], "%X", &v)
			answers[m[1]] = fmt.Sprint(int32(v))
		}
	}

	type inquireCase struct {
		number int
		before []string // what comes first
		asks   []string // each ASK's arguments
		after  []string // what comes last
	}

	// The probe gave no BLANK=, which to FORRTL is ZERO, and the compiler
	// passes the manual's NULL, so the cases ask for what the probe got.
	connected := func(n int, keys string) inquireCase {
		if !strings.Contains(keys, "BLANK") {
			keys += ", BLANK='ZERO'"
		}

		return inquireCase{n, []string{"OPEN (10, DISP='DELETE'", "     1" + keys + ")"}, []string{"10, ' '"}, nil}
	}

	cases := []inquireCase{
		{474, nil, []string{"19, ' '"}, nil},
		{475, nil, []string{"120, ' '"}, nil},
		{478, nil, []string{"-100, 'MISSING.DAT'"}, nil},
		{479, nil, []string{"-100, 'OLD1.DAT'"}, nil},
		{480, nil, []string{"-100, 'OLD*.DAT'"}, nil},
		{481, nil, []string{"-100, 'NOSUCH:[X]Y.DAT'"}, nil},
		{482, nil, []string{"-100, 'A[B'"}, nil},
		{483, []string{"OPEN (10, FILE='INQ1.DAT', BLANK='ZERO')"}, []string{"-100, 'INQ1.DAT'", "-100, '[]INQ1.DAT;'"}, nil},
		connected(486, ""),
		connected(487, ", FORM='UNFORMATTED'"),
		connected(488, ", ACCESS='DIRECT', RECL=4"),
		connected(489, ", ACCESS='DIRECT', RECL=40, FORM='FORMATTED'"),
		connected(493, ", RECORDTYPE='FIXED', RECL=10"),
		connected(494, ", RECORDTYPE='SEGMENTED', FORM='UNFORMATTED'"),
		connected(495, ", RECORDTYPE='STREAM'"),
		connected(496, ", RECORDTYPE='STREAM_CR'"),
		connected(497, ", RECORDTYPE='STREAM_LF'"),
		connected(498, ", CARRIAGECONTROL='LIST'"),
		connected(499, ", CARRIAGECONTROL='NONE'"),
		connected(500, ", BLANK='ZERO'"), // the codes the probe called NULL and ZERO are the other way round
		connected(501, ", BLANK='NULL'"),
		connected(502, ", STATUS='SCRATCH'"),
		connected(503, ", SHARED"),
		connected(504, ", RECL=133"),
		{505, []string{"OPEN (10, FILE='OLD1.DAT', STATUS='OLD', READONLY,", "     1 BLANK='ZERO')"}, []string{"10, ' '"}, nil},
		{508, []string{"WRITE (13) 1"}, []string{"13, ' '"}, []string{"CLOSE (13, DISP='DELETE')"}},
		{509, []string{"WRITE (13, '(A)') 'TEXT'"}, []string{"13, ' '"}, []string{"CLOSE (13, DISP='DELETE')"}},
	}

	d, c, buf := shareFindConsole(t, false)

	// OLD1.DAT is the probe's: FIX 20 longwords, unformatted (#407).
	fortranBuild(t, d, c, buf, "MKOLD", []string{
		"      OPEN (1, FILE='OLD1.DAT', RECORDTYPE='FIXED', RECL=20,",
		"     1 FORM='UNFORMATTED', STATUS='NEW')",
		"      CLOSE (1)",
		"      END",
	})
	shareRun(t, d, buf, "MKOLD")

	// One program of every case, each ending with the unit closed: the volume
	// is small.
	program := []string{"      PROGRAM ORACLE"}

	for _, ic := range cases {
		program = append(program, fmt.Sprintf("      WRITE (*,'('' case %d'')')", ic.number))

		for _, l := range ic.before {
			if !strings.HasPrefix(l, "     1") {
				l = "      " + l
			}

			program = append(program, l)
		}

		for _, a := range ic.asks {
			program = append(program, "      CALL ASK ("+a+")")
		}

		for _, l := range ic.after {
			program = append(program, "      "+l)
		}

		program = append(program, "      CLOSE (10)")
	}

	program = append(program, "      END")
	program = append(program, inquireReport()...)

	fortranBuild(t, d, c, buf, "ORACLE", program)

	out := shareRun(t, d, buf, "ORACLE")
	got := inquireLines(out)

	for _, ic := range cases {
		if len(got[ic.number]) != len(want[ic.number]) {
			t.Errorf("#%d: %d answers, VMS's %d", ic.number, len(got[ic.number]), len(want[ic.number]))

			continue
		}

		for i, answers := range got[ic.number] {
			for code, g := range answers {
				w, ok := want[ic.number][i][code]
				if !ok {
					t.Errorf("#%d: no q%s in VMS's", ic.number, code)
				}

				if code == "34" {
					w = strings.ReplaceAll(w, "DUA2:", "DUA1:")
				}

				// A logical prints as T or F.
				if code == "30" || code == "31" || code == "33" {
					w = map[string]string{"-1": "T", "0": "F"}[w]
				}

				if g != w {
					t.Errorf("#%d: q%s is %q, VMS's %q", ic.number, code, g, w)
				}
			}
		}
	}
}

// inquireLines reads what the program prints: each case's answers, by the code
// each line names.
func inquireLines(out string) map[int][]map[string]string {
	all := map[int][]map[string]string{}
	number := 0

	for l := range strings.Lines(out) {
		f := strings.Fields(l)

		switch {
		case len(f) == 0:
		case f[0] == "case":
			fmt.Sscanf(f[1], "%d", &number)
		case f[0] == "IOS":
			all[number] = append(all[number], map[string]string{})
		case strings.HasPrefix(f[0], "q") && len(all[number]) > 0:
			all[number][len(all[number])-1][f[0][1:]] = strings.Join(f[1:], " ")
		}
	}

	return all
}
