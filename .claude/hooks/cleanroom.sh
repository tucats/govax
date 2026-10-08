#!/bin/bash
#
# cleanroom.sh: a Claude Code PreToolUse hook that keeps licensed VMS text
# out of Claude's sessions in this repository (docs/PHASE-32.md, "The
# clean-room method").
#
# govax's own system macros (internal/bootdata/files/starlet.mar) are
# written from DIGITAL's published manuals and from real VAX MACRO's
# output, never from the text of VMS's STARLET.MLB. This hook refuses any
# tool call whose input names a file that holds such text:
#
#   - STARLET.MLB, or an extract or listing made from it;
#   - testdata/vmslib/, except the files Phase 31's gen reads for their
#     values (starlet.olb, imagelib.olb, librtl.exe);
#   - reference/vms/, VMS's definition files;
#   - real-MACRO listings that may show STARLET macro expansions and
#     haven't been audited (the list below; remove an entry once the
#     author has checked or regenerated the file).
#
# go test, go build, and go vet don't name these files, so they run: the
# local-only tests that compare govax's macros with the real library read
# it themselves, and report differences as object data, never as macro
# text.
#
# Claude Code passes the tool call as JSON on standard input. Exit status 2
# refuses the call, and what's written to standard error tells Claude why.

input=$(cat)

# Everything the tool was given, as one lowercase string: a command, a
# path, a search pattern, or a glob.
text=$(printf '%s' "$input" | jq -r '.tool_input | tostring' 2>/dev/null | tr '[:upper:]' '[:lower:]')

# The library files gen reads for their values alone are allowed by name.
text=$(printf '%s' "$text" | sed -E 's#testdata/vmslib/(starlet\.olb|imagelib\.olb|librtl\.exe)##g')

refuse() {
	echo "Clean-room barrier (.claude/hooks/cleanroom.sh, docs/PHASE-32.md): $1. govax's macros are written from the manuals and from real MACRO's output, not from STARLET.MLB's text." >&2
	exit 2
}

case "$text" in
*starlet.mlb* | *starlet.ext* | *starlet.lis*)
	refuse "this names VMS's STARLET macro library or something made from it" ;;
*reference/vms*)
	refuse "reference/vms holds licensed VMS definition files" ;;
esac

# vmslib as a name of its own (the directory), not inside an identifier
# such as the tests' vmsLibFile.
if printf '%s' "$text" | grep -Eq '(^|[^a-z0-9_])vmslib([^a-z0-9_]|$)'; then
	refuse "testdata/vmslib holds licensed VMS files; only starlet.olb, imagelib.olb, and librtl.exe may be named, for their values"
fi

# Real-MACRO output not yet audited for STARLET macro expansion text goes
# here, refused until the author has checked it. Phase 28's rmscopy.lis,
# fabalign.lis, and qiow.lis, and the Phase 32 oracle's logs
# (testdata/mar/rms/vax), were audited by the author on 2026-09-30 and
# hold no expansion text. Waiting (2026-10-08): the logs of the macro
# probe's rounds 5 and 6 and of Phase 46's definition probes
# (testdata/mp/run46/README.md), whose error messages may quote a line of
# an expansion.
unaudited='mp/macros/vax/macros[56][.]log|mp/defs/vax/defs46[.]log'
if [ -n "$unaudited" ] && printf '%s' "$text" | grep -Eq "$unaudited"; then
	refuse "this names real-MACRO output that may show STARLET macro text and hasn't been audited"
fi

exit 0
