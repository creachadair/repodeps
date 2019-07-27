#!/bin/sh

readonly image=creachadair/deps-crawler:latest
. "$(dirname $0)/config.sh"

case "$1" in
    (reset)
	docker stop deps-crawler
	docker rm deps-crawler
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
docker run \
       --detach \
       --name deps-crawler \
       --network ${net} \
       -v ${root}:/logs \
       --env SERVER=deps-server:${port} \
       --env SLEEPTIME=600 \
       --env FRACTION=0.2 \
       --env DEPSERVER_WRITE_TOKEN=37F23ABF-0D81-4B51-8D14-BE8A01ACDDE0 \
       ${image}
