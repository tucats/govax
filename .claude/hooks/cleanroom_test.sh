#!/bin/bash
#
# cleanroom_test.sh: checks that cleanroom.sh refuses what it should and
# allows what it should. Run from the repository root:
#
#	.claude/hooks/cleanroom_test.sh
#
# It prints each case and exits nonzero if any case gets the wrong answer.

hook="$(dirname "$0")/cleanroom.sh"
failed=0

# check WANT DESCRIPTION JSON: WANT is 2 (refused) or 0 (allowed).
check() {
	printf '%s' "$3" | "$hook" 2>/dev/null
	got=$?

	if [ "$got" = "$1" ]; then
		echo "ok    $2"
	else
		echo "WRONG $2: exit $got, want $1"
		failed=1
	fi
}

check 2 "Read of the real macro library" '{"tool_name":"Read","tool_input":{"file_path":"/r/testdata/vmslib/STARLET.MLB"}}'
check 2 "strings of it" '{"tool_name":"Bash","tool_input":{"command":"strings testdata/vmslib/starlet.mlb | head"}}'
check 2 "a library extract" '{"tool_name":"Bash","tool_input":{"command":"govax library /extract=$FAB /output=x.mar STARLET.MLB"}}'
check 2 "an extract file" '{"tool_name":"Read","tool_input":{"file_path":"/r/starlet.ext"}}'
check 2 "listing the directory" '{"tool_name":"Bash","tool_input":{"command":"ls testdata/vmslib/"}}'
check 2 "the directory by a relative name" '{"tool_name":"Bash","tool_input":{"command":"cd testdata && cat vmslib/*"}}'
check 2 "a Glob of the directory" '{"tool_name":"Glob","tool_input":{"pattern":"**/*","path":"testdata/vmslib"}}'
check 2 "a Grep of reference/vms" '{"tool_name":"Grep","tool_input":{"pattern":"FAB","path":"reference/vms"}}'

check 2 "the oracle's error log" '{"tool_name":"Read","tool_input":{"file_path":"/r/testdata/mar/rms/vax/errors.log"}}'
check 2 "all of the oracle's output" '{"tool_name":"Bash","tool_input":{"command":"cat testdata/mar/rms/vax/*"}}'
check 2 "a recursive grep of it" '{"tool_name":"Bash","tool_input":{"command":"grep -r FAB testdata/mar/rms/vax"}}'
check 0 "an audited Phase 28 listing" '{"tool_name":"Read","tool_input":{"file_path":"/r/testdata/mar/macros/vax/rmscopy.lis"}}'
check 0 "an oracle object analysis" '{"tool_name":"Read","tool_input":{"file_path":"/r/testdata/mar/rms/vax/def_fab.anl"}}'

check 0 "gen reading STARLET.OLB's values" '{"tool_name":"Bash","tool_input":{"command":"go run ./internal/vmsdef/gen -n -into symbols -olb testdata/vmslib/starlet.olb"}}'
check 0 "govax's own macro source" '{"tool_name":"Read","tool_input":{"file_path":"/r/internal/bootdata/files/starlet.mar"}}'
check 0 "an object analysis" '{"tool_name":"Read","tool_input":{"file_path":"/r/testdata/mar/macros/vax/rmscopy.anl"}}'
check 0 "a test helper named vmsLibFile" '{"tool_name":"Bash","tool_input":{"command":"grep -rn vmsLibFile internal"}}'
check 0 "go test" '{"tool_name":"Bash","tool_input":{"command":"go test ./internal/asm/"}}'

exit $failed
