#!/bin/bash

readonly image=creachadair/deps-server:latest
. "$(dirname $0)/config.sh"

case "$1" in
    (reset)
	docker stop deps-server
	docker rm deps-server
	docker network rm ${net}
	docker network create --driver=bridge ${net}
	;;
    ("")
	# OK
	;;
    (*)
	echo "Usage: $(basename $0) [reset]" 1>&2
	exit 2
	;;
esac

# N.B. The write token is for safety, not security.
set -x
set -e
docker run \
       --detach \
       --init \
       --name deps-server \
       --network ${net} \
       -v ${graph_volume}:/data/graph-db \
       -v ${repo_volume}:/data/repo-db \
       -v ${work_volume}:/data/tmp \
       -p 127.0.0.1:${port}:${port} \
       --env GRAPH_DB=/data/graph-db \
       --env REPO_DB=/data/repo-db \
       --env WORKDIR=/data/tmp \
       --env ADDRESS=0.0.0.0:${port} \
       --env DEPSERVER_WRITE_TOKEN=${access_token} \
       ${image}
