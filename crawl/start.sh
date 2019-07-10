#!/bin/sh

# The directory where the databases and logs should go.
readonly datadir=/mnt/data/repodeps

# The image tag to use for the container.
readonly image=creachadair/repodeps-crawl:latest

set -e

if [[ "$1" = "update" ]] ; then
    docker pull "$image"
    docker stop repo-crawler || true
    docker rm repo-crawler || true
fi
docker run \
       --detach \
       --name repo-crawler \
       -v ${datadir}:/data \
       -v ${datadir}:/logs \
       --env DB=/data/godeps-db \
       --env POLLDB=/data/poll-db \
       ${image}


       
