#!/bin/sh

readonly base=github.com/creachadair/repodeps

cd "$(dirname $0)"
go install $base
ls -1 tools \
    | grep -v 'tools\.go' \
    | xargs -t -I@ go install "${base}/tools/@"
