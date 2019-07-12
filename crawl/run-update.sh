#!/bin/sh

# Configuration settings
: ${GRAPH_DB:?missing graph database path}
: ${REPO_DB:?missing repo database path}
: ${WORKDIR:='/data/tmp'}
: ${LOGDIR:='/logs'}
: ${SLEEPTIME:=720} # seconds

mkdir -p "$WORKDIR" || exit 1

backup() {
    local rd="${GRAPH_DB}".bak
    local pd="${REPO_DB}".bak
    echo "Backing up graph database"
    ./badger backup --backup-file "$rd" --dir "$GRAPH_DB" 2>/dev/null 1>&2
    echo "Backing up repository database"
    ./badger backup --backup-file "$pd" --dir "$REPO_DB" 2>/dev/null 1>&2
    mv "$rd" "${GRAPH_DB}".snap
    mv "$pd" "${REPO_DB}".snap
    echo "<done>"
} 1>&2

now() { echo "$(date +'%F %T %z')" ; }

while true ; do
    echo "\"-- CHECK $(now)\""   # for the JSON log
    ./checkrepo -repo-db "$REPO_DB" -graph-db "$GRAPH_DB" -clone-dir "$WORKDIR" \
		-log-filter EN -scan -sample 0.1 \
 		-update -interval 1h \
		-concurrency 4 \
	&& ./rankdeps -store "$GRAPH_DB" -iterations 40 -update -scale 6 \
	&& backup
    echo "\"-- DONE $(now)\""
    sleep "$SLEEPTIME"
done 2>>"$LOGDIR"/check.log 1>>"$LOGDIR"/update.log
