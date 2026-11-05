#!/bin/sh
set -eu
project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"
export PATH="$project_root/.tools/go/bin:$project_root/.tools:$PATH"
export GOMODCACHE="$project_root/.cache/gomod"
export GOCACHE="$project_root/.cache/go-build"
if [ "$#" -eq 0 ]; then set -- dev; fi
exec wails3 "$@"
