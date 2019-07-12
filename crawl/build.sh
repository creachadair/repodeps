#!/bin/sh

readonly img=creachadair/repodeps-crawl

set -e
cd "$(dirname $0)"
cd ..

docker build -t "$img" -f crawl/Dockerfile .
docker push "$img"
docker image rm "$img"
