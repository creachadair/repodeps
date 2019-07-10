#!/bin/sh

# The directory where the databases and logs should go.
readonly datadir=/mnt/data/repodeps

# The image tag to use for the container.
readonly image=creachadair/repodeps-crawl:latest

set -e

case "$1" in
    (update)
	docker pull "$image"
	docker stop repo-crawler || true
	docker rm repo-crawler || true
	;;
    ("")
	;;
    (*)
	echo "Unknown argument '$1'; usage is '$(basename $0) [update]'" 1>&2
	exit 1
	;;
esac
docker run \
       --detach \
       --name repo-crawler \
       -v ${datadir}:/data \
       -v ${datadir}:/logs \
       --env DB=/data/godeps-db \
       --env POLLDB=/data/poll-db \
       ${image}


       
