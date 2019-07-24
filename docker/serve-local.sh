#!/bin/bash
#
# Usage: serve-local.sh
#
readonly port=9736
readonly root="$HOME/software/sourced/data"
readonly data='la-experiments:/mnt/data/repodeps'

update() {
  local base="${1:?missing database name}"
  if [[ -f "${base}.snap" ]] ; then
    badger restore -f "${base}.snap" --dir "${base}.new"
    mv "$base" "${base}.old"
    mv "${base}.new" "${base}"
    rm -fr "${base}.old"
  fi
}

set -e
pid="$(lsof -Fp -i4tcp:${port} | grep ^p | cut -c2-)"
set -x
if [[ "$pid" != "" ]] ; then
    kill -INT "$pid"
    sleep 2
fi

set -o pipefail
cd "$root"
rsync -vzt -e 'ssh -o ClearAllForwardings=yes' "${data}/*.snap" .
update graph-db
update repo-db
sync
case "$1" in
    (""|serve)
	depserver -address "localhost:$port" \
		  -graph-db graph-db \
		  -repo-db repo-db \
		  -read-only &
	echo "Database updated; service restarted." 1>&2
	;;
    (sync)
	echo "Database updated; service not restarted." 1>&2
	;;
    (*)
	echo "Unknown argument '$1'" 1>&2
	exit 1
	;;
esac
