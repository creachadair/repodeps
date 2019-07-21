#!/bin/sh

# Configuration settings
: ${SERVER:?missing depserver address}
: ${LOGDIR:='/logs'}
: ${SLEEPTIME:=720} # seconds
: ${FRACTION:='0.1'}

export TZ=PST8PDT

now() { echo "$(date +'%F %T %z')" ; }

trap 'echo terminated by signal 1>&2; exit 3' TERM
set -e
while true ; do
    echo "\"-- CHECK $(now)\"" | tee /dev/fd/2

    ./jcall -T -c "$SERVER" \
	    Scan '{"logUpdates":true, "sampleRate": $FRACTION}' \
	    Rank '{"logUpdates":false, "update":true}'

    echo "\"-- DONE $(now)\""
    sleep "$SLEEPTIME"
done 1>>"$LOGDIR"/crawl.log
