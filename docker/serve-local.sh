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
set -x
pid="$(lsof -Fp -i4tcp:${port} | grep ^p | cut -c2-)"
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
depserver -address "localhost:$port" \
	  -graph-db graph-db \
	  -repo-db repo-db \
	  -rank-iter 40 \
	  -rank-scale 6 \
	  -sample-rate 0.1 &
