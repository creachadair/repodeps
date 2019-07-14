#!/bin/sh

# The directory where the databases and logs should go.
readonly datadir=/mnt/data/repodeps

# The image tag to use for the container.
readonly image=creachadair/repodeps-crawl:latest

set -e

case "$1" in
    (update)
	docker stop repo-crawler || true
	docker pull "$image"
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
       --env GRAPH_DB=/data/graph-db \
       --env REPO_DB=/data/repo-db \
       ${image}


       
