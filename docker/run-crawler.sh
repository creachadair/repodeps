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

    ./jcall -v -c "$SERVER" \
	    Scan '{"logUpdates":true, "logErrors":true}' \
	    Rank '{"logUpdates":true, "logProgress":true}'

    echo "\"-- DONE $(now)\""
    sleep "$SLEEPTIME"
done 1>>"$LOGDIR"/crawl.log
