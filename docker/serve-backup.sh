#!/bin/bash

cd "$(dirname $0)"
. config.sh

set -e
set -x
docker stop deps-server
(cd "$root" &&
     badger backup -f graph-db.snap --dir graph-db &&
     badger backup -f repo-db.snap --dir repo-db)
./serve-docker.sh reset
