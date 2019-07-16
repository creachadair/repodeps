#!/bin/bash

cd "$(dirname $0)"
. config.sh
readonly me="$(id -un):$(id -gn)"

set -e
set -x
docker stop deps-server
(cd "$root" &&
     sudo chown -R "$me" graph-db &&
     sudo chown -R "$me" repo-db &&
     badger backup -f graph-db.snap --dir graph-db &&
     badger backup -f repo-db.snap --dir repo-db)
./serve-docker.sh reset
