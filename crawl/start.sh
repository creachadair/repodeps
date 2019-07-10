#!/bin/sh

# The directory where the databases and logs should go.
readonly datadir=/mnt/data/repodeps

# The image tag to use for the container.
readonly image=creachadair/repodeps-crawl:latest

docker run \
       --detach \
       --name repo-crawler \
       -v ${datadir}:/data \
       -v ${datadir}:/logs \
       --env DB=/data/godeps-db \
       --env POLLDB=/data/poll-db \
       ${image}


       
