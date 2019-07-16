#!/bin/sh

readonly image=creachadair/deps-crawler:latest
readonly net=deps
readonly port=9735

case "$1" in
    (reset)
	docker stop deps-crawler
	docker rm deps-crawler
	docker volume create crawler-log
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
       -v crawler-log:/logs \
       --env SERVER=deps-server:${port} \
       ${image}
