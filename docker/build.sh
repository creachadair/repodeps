#!/bin/sh

# Build the Docker images for the dependency server and crawler.

readonly df=docker/Dockerfile

set -e
cd "$(dirname $0)"
cd ..

docker build -t creachadair/deps-crawler --target=crawler -f $df .
docker build -t creachadair/deps-server --target=server -f $df .
docker image prune -f
