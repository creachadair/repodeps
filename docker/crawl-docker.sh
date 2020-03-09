#!/bin/sh

readonly image=creachadair/deps-crawler:latest
. "$(dirname $0)/config.sh"

case "$1" in
    (reset|stop)
	docker stop deps-crawler
	docker rm deps-crawler
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

set -x
set -e
docker run \
       --detach \
       --name deps-crawler \
       --network ${net} \
       --env SERVER=deps-server:${port} \
       --env INTERVAL=10m \
       --env SAMPLE_RATE=0.2 \
       --env DEPSERVER_WRITE_TOKEN=${access_token} \
       ${image}
