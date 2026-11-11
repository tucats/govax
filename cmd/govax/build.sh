#!/bin/bash
#
# Simple build script for govax. It reads the version from
# cmd/govax/tools/buildver.txt (form MAJOR.MINOR-BUILD) and injects it, plus
# a UTC build timestamp, into the binary with -ldflags, so that
# "govax --version" and the VMS version string report the build.
#
# Usage: cmd/govax/build.sh [-i] [-o OUTPUT]
#   -i         increment the build number in buildver.txt first
#   -o OUTPUT  name of the binary to write (default: ./govax in the repo root)
#
# It may be run from any directory; it always builds the repository it lives in.

set -e

# Find the repository root from this script's own location (cmd/govax/..).
SCRIPTDIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPTDIR/../.." && pwd)"
VERFILE="$SCRIPTDIR/tools/buildver.txt"
OUTPUT="$ROOT/govax"
INCREMENT=""

while [ $# -gt 0 ]; do
    case "$1" in
    -i|--increment) INCREMENT=1 ;;
    -o|--output)    shift; OUTPUT="$1" ;;
    -h|--help)
        sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//'
        exit 0
        ;;
    *)  echo "Unknown option: $1" >&2; exit 1 ;;
    esac
    shift
done

[ -f "$VERFILE" ] || { echo "Missing $VERFILE" >&2; exit 1; }
VERSION="$(tr -d '[:space:]' < "$VERFILE")"

if [ -n "$INCREMENT" ]; then
    VERSION="${VERSION%-*}-$(( ${VERSION##*-} + 1 ))"
    echo "$VERSION" > "$VERFILE"
fi

BUILDTIME="$(date -u +%Y%m%d%H%M%S)"

echo "Building govax $VERSION -> $OUTPUT"
cd "$ROOT"
go build -ldflags "-X main.BuildVersion=$VERSION -X main.BuildTime=$BUILDTIME" -o "$OUTPUT" ./cmd/govax
