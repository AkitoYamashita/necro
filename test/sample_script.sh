#!/usr/bin/env bash
#!/bin/bash
BASE="$(dirname "$(readlink -f "${BASH_SOURCE:-0}")")"
DIRNAME="$(cd "$(dirname "${BASH_SOURCE:-0}")"; pwd)"
FILENAME="$(basename "${BASH_SOURCE:-0}")"
DATEID=$(date +%Y%m%d%H%M%S)
[ -e $BASE/_.sh ] && source $BASE/_.sh
cd $BASE

./dist/necro conf/sample_task1.yml --dry-run

echo "complete"
