#!/bin/sh

# Configuration settings
: ${SERVER:?missing depserver address}
: ${LOGDIR:='/logs'}
: ${SLEEPTIME:=720} # seconds

now() { echo "$(date +'%F %T %z')" ; }

trap 'echo terminated by signal 1>&2; exit 3' TERM
set -e
while true ; do
    echo "\"-- CHECK $(now)\""

    ./jcall -c "$SERVER" \
	    Scan '{"logUpdates":true}' \
	    Rank '{"logUpdates":false}'

    echo "\"-- DONE $(now)\""
    sleep "$SLEEPTIME"
done 1>>"$LOGDIR"/crawl.log
