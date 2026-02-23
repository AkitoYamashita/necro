#!/usr/bin/env bash
#!/bin/bash
BASE="$(dirname "$(readlink -f "${BASH_SOURCE:-0}")")"
DIRNAME="$(cd "$(dirname "${BASH_SOURCE:-0}")"; pwd)"
FILENAME="$(basename "${BASH_SOURCE:-0}")"
DATEID=$(date +%Y%m%d%H%M%S)
[ -e $BASE/_.sh ] && source $BASE/_.sh
cd $BASE

./dist/necro version
./dist/necro conf/task1_s3-list_sample.yml --dry-run

echo "complete"
