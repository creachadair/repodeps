#!/bin/bash

set -e
set -o pipefail
readonly port=9735
readonly root=/mnt/data/repodeps
readonly image=creachadair/repo-depserver:latest

set -x
docker stop repo-depserver || true
docker pull ${image}
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
