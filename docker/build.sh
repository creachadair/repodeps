#!/bin/sh

readonly df=docker/Dockerfile

set -e
cd "$(dirname $0)"
cd ..

docker build -t creachadair/deps-crawler --target=crawler -f $df .
docker build -t creachadair/deps-server --target=server -f $df .
