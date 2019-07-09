#!/bin/sh

# Configuration settings
: ${DB:?missing deps database path}
: ${POLLDB:?missing poll database path}
: ${CLONEDIR:='/data/tmp'}
: ${LOGDIR:='/logs'}
: ${SLEEPTIME:=720} # seconds

mkdir -p "$CLONEDIR" || exit 1

backup() {
    local rd="${DB}".bak
    local pd="${POLLDB}".bak
    echo "Backing up dependency graph"
    ./badger backup --backup-file "$rd" --dir "$DB" 2>/dev/null 1>&2
    echo "Backing up poll database"
    ./badger backup --backup-file "$pd" --dir "$POLLDB" 2>/dev/null 1>&2
    mv "$rd" "${DB}".snap
    mv "$pd" "${POLLDB}".snap
    echo "<done>"
} 1>&2

now() { echo "$(date +'%F %T %z')" ; }

while true ; do
    echo "-- CHECK $(now)" 1>&2  # for the text log
    echo "\"-- CHECK $(now)\""   # for the JSON log
    ./checkrepo -polldb "$POLLDB" -store "$DB" -clone-dir "$CLONEDIR" \
		-log-filter EN -scan -sample 0.1 \
 		-update -interval 1h \
		-concurrency 4 \
	&& ./rankdeps -store "$DB" -iterations 40 -update -scale 6 \
	&& backup
    echo "\"-- DONE $(now)\""
    sleep "$SLEEPTIME"
done 2>>"$LOGDIR"/check.log 1>>"$LOGDIR"/update.log
