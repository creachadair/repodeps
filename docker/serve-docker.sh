#!/bin/bash

readonly image=creachadair/deps-server:latest
. "$(dirname $0)/config.sh"

case "$1" in
    (reset|stop)
	docker stop deps-server
	docker rm deps-server
	if [[ "$1" = stop ]] ; then exit 0 ; fi
	;;
    (""|start)
	# OK
	;;
    (*)
	echo "Usage: $(basename $0) [start|stop|reset]" 1>&2
	exit 2
	;;
esac
docker network create --driver=bridge ${net} 2>/dev/null &&
    echo "NOTE: Created bridge network ${net}"

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
