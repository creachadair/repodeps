#!/bin/bash

readonly image=creachadair/deps-server:latest
. "$(dirname $0)/config.sh"

case "$1" in
    (reset)
	docker stop deps-server
	docker rm deps-server
	docker network rm ${net}
	;;
    ("")
	# OK
	;;
    (*)
	echo "Usage: $(basename $0) [reset]" 1>&2
	exit 2
	;;
esac

set -x
set -e
docker network create --driver=bridge ${net}
docker run \
       --detach \
       --name deps-server \
       --network ${net} \
       -v ${root}:/data \
       -p 127.0.0.1:${port}:${port} \
       --env GRAPH_DB=/data/graph-db \
       --env REPO_DB=/data/repo-db \
       --env WORKDIR=/data/tmp \
       --env ADDRESS=0.0.0.0:${port} \
       ${image}
