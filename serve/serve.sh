#!/bin/bash

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
set -o pipefail
readonly port=9735
readonly root="$HOME/software/sourced/data"
readonly image=creachadair/repo-depserver:latest

set -x
docker stop repo-depserver || true
(cd "$root" \
     && rsync -vzt -e 'ssh -o ClearAllForwardings=yes' \
	      la-experiments:/mnt/data/repodeps/'*.snap' . \
     && update graph-db \
     && update repo-db)
docker rm repo-depserver || true
docker run \
       --detach \
       --name repo-depserver \
       -v ${root}:/data \
       -p 127.0.0.1:${port}:${port} \
       --env GRAPH_DB=/data/graph-db \
       --env REPO_DB=/data/repo-db \
       --env ADDRESS=0.0.0.0:${port} \
       ${image}
